# decimald wire protocol

`adapter/cmd/decimald` reads one JSON object per line from stdin and writes one
JSON object per line to stdout. Nothing else is written to stdout; diagnostics go
to stderr.

The shape is deliberately close to the upstream fuzzer server at
`tests/original/hypothesis/evaluate.mjs`, which already speaks
`{"func": ..., "args": [...], "config": {...}}` one object per line. The
differences are an `id` for pairing, an explicit success flag so a failure is
data rather than a crash, and a response that carries a value's whole internal
state rather than only its string form.

## Request

```json
{"id": 1, "op": "cmp", "args": ["1.5", "2"], "config": {"precision": 20}}
```

| field    | type            | required | meaning |
| -------- | --------------- | -------- | ------- |
| `id`     | integer         | yes      | Echoed in the response. A caller that pipelines requests uses it to pair them up. |
| `op`     | string          | yes      | Operation name; see the table below. |
| `args`   | array           | yes      | Operands. Arity is checked. |
| `config` | object          | no       | Settings for this request only. Absent fields keep the decimal.js defaults. |

### Statelessness

Every request is evaluated from the default configuration with `config` applied
on top. The process keeps nothing between lines. Two consequences:

- A caller may spawn one process per request or stream a million requests through
  one process and get identical answers.
- `Decimal.clone()`, which the test suite uses to run several independently
  configured constructors side by side, needs no server-side bookkeeping: each
  constructor simply sends its own `config`.

The upstream fuzzer's `"defaults": true` flag is accepted and ignored, since
every request already starts from the defaults.

### Config fields

decimal.js's names and ranges. A value that is out of range, fractional, `null`,
or of the wrong type is a `DecimalError`, exactly where decimal.js's
`Decimal.set` throws one.

| field | type | range |
| ----- | ---- | ----- |
| `precision` | integer | 1 … 1e9 |
| `rounding` | integer | 0 … 8 |
| `modulo` | integer | 0 … 9 |
| `toExpNeg` | integer | -9e15 … 0 |
| `toExpPos` | integer | 0 … 9e15 |
| `minE` | integer | -9e15 … 0 |
| `maxE` | integer | 0 … 9e15 |
| `crypto` | boolean, or 0/1 | — |

Unknown fields are ignored, as decimal.js ignores properties outside its own
list.

## Response

Success:

```json
{"id": 1, "ok": true, "value": {"d": [1, 5000000], "e": 0, "s": 1, "str": "1.5"}}
```

Failure:

```json
{"id": 1, "ok": false, "value": null, "error": "DecimalError: invalid argument: hello", "kind": "decimal"}
```

`value` is always present and is `null` on failure. `error` and `kind` appear
only on failure.

### Failure kinds

`kind` exists because the test suite treats a `DecimalError` as a *passing*
outcome — `T.assertException` counts any thrown `Error` whose message matches
`/DecimalError/` as a pass. An operation the port has not written yet must
therefore never be reported with that wording, or a missing feature would show up
as a green test.

| kind | meaning | how the shim surfaces it |
| ---- | ------- | ------------------------ |
| `decimal` | A condition decimal.js signals by throwing a `DecimalError`: an unparseable operand, an out-of-range setting. The message begins with `DecimalError`. | `throw new Error('[DecimalError] …')` — may legitimately satisfy `assertException`. |
| `unimplemented` | The operation is not in the port yet. The message never contains `DecimalError`. | `throw new Error('not implemented in port: …')` — always a test failure. |
| `protocol` | The request was malformed: bad JSON, wrong arity, an operand that is not a number, string or decimal object. A harness bug. | `throw new Error('harness failure: …')` — always a test failure. |

## Encoding a Decimal

```json
{"d": [123456, 7891011], "e": 5, "s": 1, "str": "123456.7891011"}
```

| field | meaning |
| ----- | ------- |
| `d`   | The base-1e7 coefficient limbs, most significant first, or `null` for a non-finite value. This is decimal.js's `.d`, limb for limb, including its alignment rule that limb boundaries fall on multiples of seven digits counting from the decimal point. |
| `e`   | The base-10 exponent of the most significant digit — decimal.js's `.e` — or `"NaN"` for a non-finite value. |
| `s`   | The sign: `1`, `-1`, or `"NaN"` for NaN. Note that this is the *stored* sign, so `-0` has `s: -1`. |
| `str` | The value as `Decimal.prototype.toString` would render it under the request's `config`, which is what makes `toExpNeg` and `toExpPos` relevant to a response. |

The suite's `T.assertEqualProps(digits, exponent, sign, n)` reads `n.d`, `n.e`
and `n.s` directly, which is why the whole internal state is on the wire and not
just the string.

### Numbers JSON cannot spell

JSON has no literal for `NaN`, `Infinity`, `-Infinity` or `-0`, and all four
matter here: the first three are values decimal.js represents, and `-0` is a
value whose sign lives only in `.s` because `toString` prints it as `"0"`. A
protocol that shipped only strings would silently turn `-0` into `0`.

So **wherever the protocol expects a number, these four are written as strings
instead**, in both directions:

| JavaScript | wire |
| ---------- | ---- |
| `NaN` | `"NaN"` |
| `Infinity` | `"Infinity"` |
| `-Infinity` | `"-Infinity"` |
| `-0` | `"-0"` |
| anything else | a plain JSON number |

This applies to `e`, `s`, the result of `cmp` (which is `NaN` when either operand
is NaN), and the result of `sign` (which is `-0` for a negative zero). The
helpers `encodeNumber` and `decodeNumber` in `adapter/shim/transport.cjs`
implement the mapping on the JavaScript side.

Round trips are exact. Feeding a response's decimal object straight back in as an
operand reproduces the value bit for bit — including `-0`, both infinities, NaN,
and exponents at the ±9e15 limits — because reconstruction rebuilds the value
from `d`, `e` and `s` under wide-open exponent limits rather than reparsing
`str`.

## Operands

An element of `args` may be:

- **a decimal object**, as above, which is reconstructed exactly;
- **a string**, parsed the way decimal.js parses a string operand;
- **a JSON number**, parsed from its literal text.

The shim never sends bare JSON numbers, because `JSON.stringify(-0)` is `0` and
NaN and the infinities have no literal at all; it converts JavaScript numbers to
strings on its side. The number case exists so that a request typed by hand, or
borrowed from the upstream fuzzer, still works.

## Operations

Only what the port implements is registered; every other name returns
`kind: "unimplemented"`. Send `{"id": 0, "op": "ops", "args": []}` for the live
list.

### Construction

| op | args | value |
| -- | ---- | ----- |
| `new` | 1 operand | decimal |
| `fromInt` | 1 integer | decimal |
| `fromFloat` | 1 number | decimal |
| `nan` | none | decimal |
| `inf` | 1 sign (`1` or `-1`) | decimal |

### State

| op | args | value |
| -- | ---- | ----- |
| `d` | 1 operand | limb array or `null` |
| `e` | 1 operand | number or `"NaN"` |
| `s` | 1 operand | number or `"NaN"` |
| `str` | 1 operand | string |
| `echo` | 1 operand | decimal (used to test the round trip) |

### Predicates

`isNaN`, `isFinite`, `isInf`, `isZero`, `isInteger`, `isNegative`, `isPositive` —
1 operand, boolean value.

### Sign

| op | args | value |
| -- | ---- | ----- |
| `sign` | 1 operand | decimal.js's `Decimal.sign`: `1`, `-1`, `0`, `"-0"` or `"NaN"` |
| `signum` | 1 operand | `-1`, `0` or `1`; `0` for NaN |

### Comparison

| op | args | value |
| -- | ---- | ----- |
| `cmp` | 2 operands | `-1`, `0`, `1`, or `"NaN"` |
| `eq`, `gt`, `gte`, `lt`, `lte` | 2 operands | boolean |

### Meta

| op | args | value |
| -- | ---- | ----- |
| `config` | none | the effective settings, normalised |
| `ops` | none | sorted array of registered operation names |

## Adding an operation

Add one line to the `ops` table in `adapter/cmd/decimald/main.go`. The `unary`,
`binary`, `predicate` and `comparison` adapters cover the common shapes; a value
that is a Decimal goes through `encode`, and a value that is a JavaScript number
goes through the string convention above.

Then remove the operation's name from `UNIMPLEMENTED_METHODS` or
`UNIMPLEMENTED_STATICS` in `adapter/shim/decimal.cjs` and wire it to `call`.
