# tests/port

The port's own tests, and where to find them.

## The tests themselves live in `src/`

Go's convention is that a test sits next to the code it exercises, in the same
package: `src/round.go` is tested by `src/round_test.go`. Moving them here would
mean either giving up access to the unexported internals — the limb array, the
exponent, `finalise` itself — or exporting things purely to make a directory
layout match, which is a worse trade than a pointer in a README.

So: **the new tests added by this port are `src/*_test.go`**, about 200,000
assertions of them. Run them with:

```bash
go test ./src/
```

## What is in this directory

The generators that produce those assertions' expected values, by running
decimal.js itself:

| Script | Produces |
|---|---|
| `gen_rounding_cases.mjs` | `src/testdata/rounding.txt` — 5508 cases over 50 values, 12 digit counts, 9 rounding modes |
| `gen_binary_cases.mjs` | `plus/minus/times/div/divToInt/mod.txt` — the two-operand arithmetic |
| `gen_unary_cases.mjs` | `abs/neg/round/floor/ceil/trunc/toDP/toSD.txt` |
| `gen_string_cases.mjs` | `strings.txt` — every formatting method across six configurations |
| `gen_pow_cases.mjs` | `pow.txt` — integer exponentiation |
| `gen_misc_cases.mjs` | `misc.txt` — dp, sd, toNearest, clamp, min, max, sum |
| `gen_fraction_cases.mjs` | `fraction.txt` — toFraction with and without a denominator limit |

Each takes the path to a decimal.js build and writes tab-separated expectations
to stdout:

```bash
node tests/port/gen_rounding_cases.mjs ./decimal.js/decimal.mjs > src/testdata/rounding.txt
```

Expectations are generated rather than written by hand deliberately. A
hand-written expectation encodes a reading of the source; a generated one
encodes its behaviour, which is the thing this port is supposed to match. That
distinction has already paid for itself: a hand-written expectation for
`NewFromInt(-9007199254740991)` claimed four limbs, and the port's three were
right.

## The committed files are a sample, not the sweep

The generators produce exhaustive sweeps — every value against every precision
and every rounding mode — which came to 215334 lines. That is the right thing to
check against while porting and the wrong thing to carry in a repository: it is
this port's own output, not an upstream fixture.

What is committed is a stride sample, about 500 lines per file, produced by:

```bash
node tests/port/sample_testdata.mjs --max 500
```

Every Nth line rather than the first N, because each file is ordered by
configuration block: a prefix would keep one precision and discard the rest,
while a stride keeps every configuration, every rounding mode and the whole
spread of values.

The tests read whatever is in the file, so regenerating a full sweep simply
makes them stronger — no code changes, no flags:

```bash
node tests/port/gen_binary_cases.mjs ./decimal.js/decimal.mjs mod > src/testdata/mod.txt
go test ./src/ -run TestMod
```

The real proof is not these files in any case. It is decimal.js's own suite,
unmodified, at 22658 of 22658 — see `results/parity.txt`.
