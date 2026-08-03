# decimaljs-go

A Go port of [**MikeMcl/decimal.js**](https://github.com/MikeMcl/decimal.js) — an arbitrary-precision Decimal type for JavaScript.

Port Mortem 2026 (Code Resurrection Wave 2, Hackathon Raptors) · Track F: JavaScript → Go.

> **decimal.js's own test suite passes unmodified against this Go build: 22658 of 22658 assertions** — the same count a clean upstream checkout produces.
> Per-module table: [`results/parity.txt`](results/parity.txt). Test files are byte-for-byte upstream, hashed in [`tests/original/HASHES.txt`](tests/original/HASHES.txt).

| | |
|---|---|
| **Parity** | 22658 / 22658 · 32658 / 32658 including a module upstream cannot run |
| **Scope** | Complete — every method decimal.js exposes |
| **Differential fuzz** | 279188 cases, 90 s, **zero divergences** |
| **Upstream bug found** | [decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262), filed |
| **Dependencies** | None beyond the Go standard library |
| **Ported from** | decimal.js 10.6.0 at `cd73a7f` |

---

## What this is

A standalone, dependency-free Go package reproducing decimal.js's arbitrary-precision decimal arithmetic: the same results, the same rounding, the same edge cases — `NaN`, `±0`, `±Infinity`, configurable precision and nine rounding modes — digit for digit.

It is not a wrapper around a JavaScript engine and does not link the source-language runtime. All arithmetic is implemented in Go. The original JavaScript tests serve only as proof: they call into the Go build through a test-only adapter, and not one byte of them is modified.

Licensed MIT, as is the original; [`LICENSE`](LICENSE) carries both notices.

## Why port it

decimal.js is a widely used precision-arithmetic library with no direct Go equivalent. `shopspring/decimal`, the usual Go choice, is fixed-point with a different configuration model — decimal places rather than significant digits — which is a different design rather than the same behaviour expressed differently.

A faithful port brings decimal.js's significant-digit precision model and its rounding semantics to Go as a single dependency-free package, and makes the equivalence provable instead of asserted.

## Build

One command builds the library, the adapter, and runs the proof. Both commands below are verified; the numbers in this README come from a container run rather than a working tree.

```bash
docker build -t decimaljs-go .    # vet, unit tests, adapter build, marshalling smoke test
docker run --rm decimaljs-go      # decimal.js's suite against the Go build
```

Locally:

```bash
go test ./src/                                          # unit tests against generated expectations
go build -o adapter/bin/decimald ./adapter/cmd/decimald
node adapter/smoke.mjs                                  # marshalling boundary, 27 checks
node adapter/run-parity.mjs --all                       # the full original suite
node fuzz/harness.mjs --seconds 60                      # differential fuzz against decimal.js
```

The parity and fuzz runs need Node and a decimal.js checkout at `./decimal.js` (the reference, gitignored). The library itself needs neither.

## Usage

```go
import decimal "github.com/ishadowfighter/decimaljs-go/src"

x, err := decimal.Parse("0.1")   // exactly 0.1, not the float64 that literal denotes
y := decimal.NewFromFloat(0.2)   // via the shortest round-tripping form, as decimal.js does
fmt.Println(x.Add(y))            // 0.3

// A Context is decimal.js's Decimal.clone(): its own precision and rounding mode.
cfg := decimal.DefaultConfig()
cfg.Precision = 50
cfg.Rounding = decimal.RoundHalfEven
ctx := decimal.NewContext(cfg)

two, _ := ctx.Parse("2")
fmt.Println(ctx.ValueOf(ctx.Sqrt(two)))
// 1.4142135623730950488016887242096980785696718753769

r, err := ctx.Ln(two)            // errors where decimal.js throws
```

The API is Go-shaped rather than a transliteration: value receivers returning new values, `error` in place of `throw`, configuration as a value instead of global mutable state. Behaviour still matches decimal.js exactly; where idiom and behaviour conflicted, [`DECISIONS.md`](DECISIONS.md) records which won.

## Scope

Complete. Every method decimal.js exposes has a Go equivalent, and every module of its suite passes.

| Area | Methods |
|---|---|
| Construction & parsing | strings, numbers, integers, `NaN`, `±Infinity`, `-0`, scientific notation, underscore separators, `0x`/`0b`/`0o` literals with binary exponents |
| Arithmetic | `Add` `Sub` `Mul` `Div` `DivToInt` `Mod` `Abs` `Neg` `Pow` (integer and fractional) |
| Rounding | all nine modes; `Round` `Floor` `Ceil` `Trunc` `ToDecimalPlaces` `ToSignificantDigits` `ToNearest` |
| Comparison | `Cmp` `Eq` `Gt` `Gte` `Lt` `Lte`, and every predicate |
| Formatting | `String` `ValueOf` `MarshalJSON` `ToFixed` `ToExponential` `ToPrecision` `ToBinary` `ToOctal` `ToHexadecimal` `ToFraction` |
| Transcendental | `Sqrt` `Cbrt` `Exp` `Ln` `Log` `Log2` `Log10`, six trigonometric, six hyperbolic, `Atan2` `Hypot` |
| Other | `Min` `Max` `Sum` `Clamp` `Random` `DecimalPlaces` `SignificantDigits` |

### The 61st module

`tests/original/modules/powSqrt.js` cannot run as shipped. It loops on a free variable `total` that no version of `setup.js` defines, so it throws `ReferenceError` before its first assertion — on a clean upstream checkout as much as here — and upstream's own `test.js` omits it from the 60 modules it executes.

That is a defect in the original repository, filed as [decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262). The test runner here supplies the missing counter rather than editing the vendored file, and the module then passes **10000 / 10000**.

Those assertions are deliberately excluded from the 22658 headline: that figure exists to be compared against the upstream baseline, and upstream does not run this module. [`results/parity.txt`](results/parity.txt) reports both totals.

## Proving equivalence

decimal.js's tests live under [`tests/original/`](tests/original/) unmodified, with a SHA-256 per file in `HASHES.txt` and a single kickoff hash over that manifest in [`.port-mortem.toml`](.port-mortem.toml). Verify at any time:

```bash
cd tests/original && sha256sum -c HASHES.txt   # 69 files
```

They execute against the Go build through [`adapter/`](adapter/): a Go binary speaking one JSON request per line, and a JavaScript shim presenting decimal.js's surface that delegates each operation to it. `tests/original/setup.js` does `require('../decimal')`, which resolves to `tests/decimal.js` — outside the vendored tree — so the redirect needs no launch flag and no edit to any test file.

The wire form carries each value's full internal state (`.d`, `.e`, `.s`), because the suite asserts on those directly and because `toString` cannot express the sign of negative zero.

**Result: 22658 / 22658.** Baseline on a clean upstream checkout: 22658 / 22658. Including `powSqrt.js`, the container run totals **32658 / 32658**.

### Differential fuzzing

[`fuzz/harness.mjs`](fuzz/harness.mjs) drives both implementations with identical random operands and configurations and compares results. The recorded run in [`fuzz/log.txt`](fuzz/log.txt): **279188 cases in 90 seconds, zero divergences**, across 30 operations and seven precision/rounding combinations, seeded so any divergence is replayable.

decimal.js is the oracle rather than mpmath, deliberately. The fuzzer decimal.js ships (`test/hypothesis/error_hunt.py`, mpmath as ground truth) does not pass on a clean upstream checkout: at `precision: 14`, `sin(6504783935)` is `5.5360303649386E-8` from decimal.js and `5.5360303649385E-8` from mpmath. A port whose contract is equivalence cannot use an oracle that disagrees with the thing it must equal.

Five draws are bounded, with the reason recorded in the harness: decimal.js exhausts the V8 heap on `exp(-3e15)`, and its own source documents abandoning `cosh` at a large argument. Unbounded draws would measure patience rather than agreement.

## Performance

A tiebreaker, and nothing here was optimised for it. Methodology and caveats: [`bench/methodology.md`](bench/methodology.md). Raw numbers: [`bench/results.json`](bench/results.json).

At `precision: 34`, p99 latency, Windows/amd64, Go 1.26.3 against Node 22.16.0 with decimal.js 10.6.0:

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

`toString` is genuinely slower and stays that way. The port builds output through `strings.Builder` and intermediate slices where V8 optimises string concatenation heavily; tuning it would disturb code whose behaviour is pinned digit for digit by 500 assertions per formatting method. An earlier version of this benchmark reported a p99 of exactly 20 µs for `add` — that was the Windows clock's resolution divided by a fixed batch size, not a measurement. The fix is described in the methodology.

## Bonus criteria

| Criterion | Status |
|---|---|
| **Differential Fuzz Survivor** | 279188 cases, 90 s, zero divergences — [`fuzz/log.txt`](fuzz/log.txt) |
| **Zero Unsafe** | `src/` contains no `unsafe`, `reflect`, cgo or `any`; the 61 uses of `any` are in the test-only adapter's JSON boundary, disclosed |
| **Bug Catcher** | [decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262) filed — `powSqrt.js` has never run; made runnable here and passing 10000 / 10000 |
| **Decision Log** | 17 entries in [`DECISIONS.md`](DECISIONS.md), four documenting a first attempt that was wrong |

Evidence and caveats for each: the Bonus criteria section of [`DECISIONS.md`](DECISIONS.md).

## Layout

```
decimaljs-go/
├── src/                 the port — package decimal, no dependencies
├── tests/original/      decimal.js's tests, byte-for-byte, hashed
├── tests/port/          generators producing expectations from decimal.js
├── adapter/             test-only bridge: Go line-protocol server + JS shim
├── fuzz/                differential fuzzer and its recorded run
├── bench/               methodology and measurements
├── results/             parity table and the upstream issue as filed
├── DECISIONS.md         every non-trivial divergence and why
└── Dockerfile           one command to a built, proven artifact
```

The port's own tests are `src/*_test.go`, beside the code, because they exercise unexported internals — the limb array, the exponent, `finalise` itself. `tests/port/` holds the generators that produce their expected values by running decimal.js; the committed expectations are a stride sample of those sweeps. See [`tests/port/README.md`](tests/port/README.md).

## Write-up

The story of the port — what broke, how equivalence was proven, the benchmark
that measured the wrong thing, and the decision I would take back:

- Dev.to: _link once published_ · draft at [`writeup/devto.md`](writeup/devto.md)
- X: _link once published_ · draft at [`writeup/x-thread.md`](writeup/x-thread.md)

## Credits

Original library: [decimal.js](https://github.com/MikeMcl/decimal.js) by Michael Mclaughlin, MIT.
Built for [Port Mortem / Code Resurrection](https://coderesurrection.com), a Hackathon Raptors event.
