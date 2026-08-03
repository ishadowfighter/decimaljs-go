# decimaljs-go

A Go port of [**MikeMcl/decimal.js**](https://github.com/MikeMcl/decimal.js) — an arbitrary-precision Decimal type for JavaScript.

Built for **Port Mortem 2026** (Code Resurrection Wave 2, Hackathon Raptors) · Track F: JavaScript → Go.

> **decimal.js's own test suite passes, unmodified, against this Go build: 22658 of 22658 assertions.**
> That is the same count a clean upstream checkout produces. The per-module table is in
> [`results/parity.txt`](results/parity.txt); the tests themselves are byte-for-byte upstream and hashed in
> [`tests/original/HASHES.txt`](tests/original/HASHES.txt).

---

## What this is

`decimaljs-go` is a standalone, dependency-free Go package that reproduces decimal.js's arbitrary-precision decimal arithmetic: the same results, the same rounding, the same edge cases (`NaN`, `±0`, `±Infinity`, configurable precision and rounding modes) — digit for digit.

It is **not** a wrapper around a JavaScript engine and does **not** link the source-language runtime. All arithmetic is implemented in Go. The original JavaScript tests are used only as proof: they call into the Go build through a thin, test-only adapter, and not one byte of them is modified.

- **Ported from:** decimal.js 10.6.0, commit `cd73a7f` (recorded in [`.port-mortem.toml`](.port-mortem.toml))
- **Original license:** MIT · **This port's license:** MIT (see [`LICENSE`](LICENSE))
- **Source language:** JavaScript · **Target:** Go 1.24+, no dependencies

## Why port it

decimal.js is a widely used precision-arithmetic library with no direct Go equivalent. `shopspring/decimal`, the common Go choice, is fixed-point with a different configuration model — decimal places rather than significant-digit precision — which is a different design, not the same behaviour. A faithful port brings decimal.js's significant-digit precision model and its nine rounding modes to Go as a single dependency-free package, and lets the equivalence be *proven* rather than asserted.

---

## Build and prove it

One command builds the library, the adapter and runs the proof:

```bash
docker build -t decimaljs-go .    # vets, unit-tests, builds, smoke-tests the harness
docker run --rm decimaljs-go      # runs decimal.js's suite against the Go build
```

Locally:

```bash
go test ./src/                                        # unit tests, ~200k generated expectations
go build -o adapter/bin/decimald ./adapter/cmd/decimald
node adapter/smoke.mjs                                # marshalling boundary, 27 checks
node adapter/run-parity.mjs --all                     # the full original suite
node fuzz/harness.mjs --seconds 60                    # differential fuzz vs decimal.js
```

The parity and fuzz runs need a decimal.js checkout at `./decimal.js` (the reference, gitignored) and Node. The library itself needs neither.

## Usage

```go
import decimal "github.com/ishadowfighter/decimaljs-go/src"

x, err := decimal.Parse("0.1")          // exactly 0.1, not the float64 that literal denotes
y := decimal.NewFromFloat(0.2)          // via the shortest round-tripping form, as decimal.js does
fmt.Println(x.Add(y))                   // 0.3

// A Context is decimal.js's Decimal.clone(): its own precision and rounding.
cfg := decimal.DefaultConfig()
cfg.Precision = 50
cfg.Rounding = decimal.RoundHalfEven
ctx := decimal.NewContext(cfg)

two, _ := ctx.Parse("2")
fmt.Println(ctx.ValueOf(ctx.Sqrt(two))) // 1.4142135623730950488016887242096980785696718753769

ln, err := ctx.Ln(two)                  // errors where decimal.js throws
```

The API is Go-shaped rather than a transliteration of the JavaScript one: methods take value receivers and return new values, `error` replaces `throw`, and configuration is a value instead of global mutable state. Behaviour still matches decimal.js exactly; where the two pulled in different directions, [`DECISIONS.md`](DECISIONS.md) records which won and why.

---

## Scope

**Everything is ported.** Every method decimal.js exposes has a Go equivalent, and every module of its test suite passes:

| Area | Methods |
|---|---|
| Construction & parsing | strings, numbers, integers, `NaN`, `±Infinity`, `-0`, scientific notation, underscore separators, `0x`/`0b`/`0o` literals with binary exponents |
| Arithmetic | `Add` `Sub` `Mul` `Div` `DivToInt` `Mod` `Abs` `Neg` `Pow` (integer and fractional) |
| Rounding | all nine modes; `Round` `Floor` `Ceil` `Trunc` `ToDecimalPlaces` `ToSignificantDigits` `ToNearest` |
| Comparison | `Cmp` `Eq` `Gt` `Gte` `Lt` `Lte`, and every predicate |
| Formatting | `String` `ValueOf` `MarshalJSON` `ToFixed` `ToExponential` `ToPrecision` `ToBinary` `ToOctal` `ToHexadecimal` `ToFraction` |
| Transcendental | `Sqrt` `Cbrt` `Exp` `Ln` `Log` `Log2` `Log10`, all six trigonometric, all six hyperbolic, `Atan2` `Hypot` |
| Other | `Min` `Max` `Sum` `Clamp` `Random` `DecimalPlaces` `SignificantDigits` |

**One caveat, stated plainly:** `tests/original/modules/powSqrt.js` does not run — it fails with `total is not defined` on a clean upstream checkout too, and upstream's own `test.js` does not include it in the 60 modules it executes. It is reported in `results/parity.txt` rather than quietly dropped.

## Proving equivalence

decimal.js's tests live under [`tests/original/`](tests/original/), unmodified, with a SHA-256 per file in `HASHES.txt` and a single kickoff hash over that manifest in `.port-mortem.toml`. Verify at any time:

```bash
cd tests/original && sha256sum -c HASHES.txt
```

They run against the Go build through [`adapter/`](adapter/): a Go binary speaking one JSON request per line, and a JavaScript shim presenting decimal.js's surface that delegates each operation to it. `tests/original/setup.js` does `require('../decimal')`, which resolves to `tests/decimal.js` — outside the vendored tree — so the redirect needs no launch flag and no edit to a test file. The wire form carries each value's full internal state (`.d`, `.e`, `.s`), because the suite asserts on those directly and because `toString` cannot express the sign of negative zero.

**Result: 22658 / 22658.** Baseline on a clean upstream checkout: 22658 / 22658.

### Differential fuzzing

[`fuzz/harness.mjs`](fuzz/harness.mjs) drives both implementations with the same random operands and configurations and compares the results. The recorded run in [`fuzz/log.txt`](fuzz/log.txt) is **279188 cases in 90 seconds with zero divergences** across 30 operations and seven precision/rounding combinations.

decimal.js is the oracle, deliberately, and mpmath is not. The fuzzer decimal.js ships (`test/hypothesis/error_hunt.py`, mpmath as ground truth) does **not** pass on a clean upstream checkout: at `precision: 14`, `sin(6504783935)` is `5.5360303649386E-8` from decimal.js and `5.5360303649385E-8` from mpmath. A port whose contract is equivalence cannot use an oracle that disagrees with the thing it is meant to equal.

Three operations have bounded operands in the fuzzer, and the harness says why: decimal.js itself runs out of memory on `exp(-3e15)` and is documented as abandoning `cosh` at a large argument, so an unbounded draw would measure patience rather than agreement.

## Performance

Performance is a tiebreaker, and nothing here was optimised. Methodology and caveats are in [`bench/methodology.md`](bench/methodology.md); raw numbers in [`bench/results.json`](bench/results.json). At `precision: 34`, p99 latency, Windows/amd64, Go 1.26.3 vs Node 22.16.0:

| Operation | Go p99 | decimal.js p99 | Ratio |
|---|---:|---:|---:|
| parse | 0.68 µs | 3.80 µs | **5.6×** |
| add | 1.11 µs | 3.44 µs | **3.1×** |
| mul | 1.20 µs | 5.10 µs | **4.3×** |
| div | 6.02 µs | 32.2 µs | **5.4×** |
| sqrt | 36.6 µs | 182 µs | **5.0×** |
| sin | 136 µs | 509 µs | **3.8×** |
| exp | 308 µs | 685 µs | **2.2×** |
| ln | 360 µs | 550 µs | **1.5×** |
| toString | 2.13 µs | 1.32 µs | **0.62× — slower** |

Startup, median of 15 runs: **17 ms** for the Go binary against **82 ms** for Node loading decimal.js.

`toString` is genuinely slower and is left that way: the port builds output through `strings.Builder` and intermediate slices where V8 optimises string concatenation heavily, and tuning it would mean disturbing code whose behaviour is currently pinned digit for digit. An earlier version of this benchmark reported a p99 of exactly 20 µs for `add` — that was the Windows clock's resolution divided by a fixed batch size, not a measurement; the fix is described in the methodology.

---

## Layout

```
decimaljs-go/
├── src/                 the port (package decimal, no dependencies)
├── tests/original/      decimal.js's tests, byte-for-byte, hashed
├── tests/port/          generators that produce expectations from decimal.js
├── adapter/             test-only bridge: Go line-protocol server + JS shim
├── fuzz/                differential fuzzer and its recorded run
├── bench/               methodology and measurements
├── results/parity.txt   per-module pass table
├── DECISIONS.md         every non-trivial divergence and why
└── Dockerfile           one command to a built, proven artifact
```

Unit tests under `src/` compare against expectations generated from decimal.js itself by the scripts in `tests/port/`, across every rounding mode and a spread of precisions. Hand-written expectations would encode a reading of the source rather than its behaviour.

Those generators produce exhaustive sweeps — 215334 lines of them. What is committed is a stride sample of roughly 500 lines per file, since the sweeps are this port's own output rather than an upstream fixture. Regenerating a full sweep needs no code change: the tests read whatever the file holds. See [tests/port/README.md](tests/port/README.md).

## Credits

- Original library: [decimal.js](https://github.com/MikeMcl/decimal.js) by Michael Mclaughlin (MIT).
- Built for [Port Mortem / Code Resurrection](https://coderesurrection.com), a Hackathon Raptors event.

See [`DECISIONS.md`](DECISIONS.md) for the engineering rationale behind each divergence.
