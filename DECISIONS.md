# Decisions

Every non-trivial fork in the road taken while porting decimal.js to Go, and why
it was taken. Written as the work happened rather than reconstructed afterwards,
which is why four entries record a first attempt that was wrong.

The governing rule: **behaviour beats idiom.** Where an idiomatic Go choice would
change an observable result, decimal.js's behaviour wins and the compromise is
recorded here.

| | Decision |
|---|---|
| D1 | [The baseline is decimal.js, not correctness](#d1--the-baseline-is-decimaljs-not-correctness) |
| D2 | [The vendored tests are stored with upstream line endings](#d2--the-vendored-tests-are-stored-with-upstream-line-endings) |
| D3 | [Licence carries both notices](#d3--licence-carries-both-notices) |
| D4 | [Module path](#d4--module-path) |
| D5 | [Configuration is a value, not global mutable state](#d5--configuration-is-a-value-not-global-mutable-state) |
| D6 | [Every JavaScript `throw` becomes an error return](#d6--every-javascript-throw-becomes-an-error-return) |
| D7 | [Values are immutable, and the coefficient is copied on the way out](#d7--values-are-immutable-and-the-coefficient-is-copied-on-the-way-out) |
| D8 | [Math.pow's two ECMAScript-only cases are reimplemented](#d8--mathpows-two-ecmascript-only-cases-are-reimplemented) |
| D9 | [`pow` keeps both of decimal.js's paths](#d9--pow-keeps-both-of-decimaljss-paths) |
| D10 | [Where decimal.js stops rounding, and why it matters](#d10--where-decimaljs-stops-rounding-and-why-it-matters) |
| D11 | [Two JavaScript idioms that carry meaning](#d11--two-javascript-idioms-that-carry-meaning) |
| D12 | [Test data is generated from decimal.js, not written by hand](#d12--test-data-is-generated-from-decimaljs-not-written-by-hand) |
| D13 | [The benchmark measured the clock, not the code](#d13--the-benchmark-measured-the-clock-not-the-code) |
| D14 | [`toString` is slower, and stays slower](#d14--tostring-is-slower-and-stays-slower) |
| D15 | [Bounded operands in the fuzzer, and why that is not cheating](#d15--bounded-operands-in-the-fuzzer-and-why-that-is-not-cheating) |
| D16 | [A test in the original repo that has never run](#d16--a-test-in-the-original-repo-that-has-never-run) |
| D17 | [The fix goes in the runner, because the test file is untouchable](#d17--the-fix-goes-in-the-runner-because-the-test-file-is-untouchable) |

The [Bonus criteria](#bonus-criteria) section at the end states where the port
stands against each of the four, with the evidence and the asterisks together.

---

## D1 — The baseline is decimal.js, not correctness

The upstream suite was run on a clean clone of `cd73a7f` before any port code
was written: `22658 of 22658` assertions pass. That number is the denominator.
Where decimal.js produces a result that is arguably wrong, this port reproduces
it, because the goal is equivalence rather than an improved decimal library.
Any such quirk is recorded here as a candidate upstream bug rather than
silently "fixed".

The differential fuzzer shipped upstream (`test/hypothesis/error_hunt.py`)
compares decimal.js against mpmath, and it does **not** pass clean on upstream
either — for example `sin(6504783935)` at `precision: 14` yields
`5.5360303649386E-8` from decimal.js versus `5.5360303649385E-8` from mpmath, a
last-digit disagreement from cancellation in the argument reduction. So mpmath
is not a usable pass/fail oracle for this port. Fuzzing here compares the Go
build against **decimal.js directly**, with mpmath kept only as a third opinion
to tell "this port is wrong" apart from "decimal.js and the port agree, and
mpmath doesn't".

## D2 — The vendored tests are stored with upstream line endings

`tests/original/` reproduces the upstream `test/` tree byte-for-byte. The local
clone was checked out with `core.autocrlf=true`, so its working files carry
CRLF; the bytes in the upstream repository are LF. The LF form is what is
vendored, since that is what upstream actually publishes, and
`.gitattributes` marks `tests/original/**` as `-text` so no checkout on any
platform can rewrite it back. `HASHES.txt` records a SHA-256 per file, making
any later modification — accidental or otherwise — detectable with a single
`sha256sum -c`.

## D3 — Licence carries both notices

The port is MIT, and the original decimal.js MIT notice is reproduced in
`LICENSE` beneath it. Both the ported algorithms and the vendored test suite are
derivative of upstream, so dropping the original notice would not be
permissible; adding it once at the top level is cleaner than a per-file header.

## D4 — Module path

`go.mod` declares `github.com/ishadowfighter/decimaljs-go`. It was carried as a
placeholder until the owning account was confirmed, rather than guessed at, so
that no wrong identifier ever landed in a commit.

## D5 — Configuration is a value, not global mutable state

decimal.js keeps `precision`, `rounding` and the rest on the constructor
object, and `Decimal.clone()` exists to get an independently configurable one.
The port makes that explicit: `Config` is a plain struct and a `Context` carries
one, so `NewContext` is what `clone` was. Package-level functions and the
methods on `Decimal` use a default context, which keeps the common case short
and matches the original's global-feeling API.

The original suite mutates the global configuration mid-file
(`Decimal.rounding = 3`), so the adapter needs a mutable default context to
present that surface. Keeping the mutability at the adapter boundary rather than
in the library is the compromise: the port itself has no global that a
concurrent caller can trip over.

## D6 — Every JavaScript `throw` becomes an error return

decimal.js throws `[DecimalError] Invalid argument` for an out-of-range digit
count, an unknown rounding mode, or an unparseable string. Those all return an
error wrapping the exported `Err`, so callers can match with `errors.Is`, and
the adapter turns them back into thrown errors whose message contains
`DecimalError` — which is all the original's `assertException` checks.

Operations that cannot fail keep single-value signatures, so `Add`, `Mul` and
the rest read normally; only the ones with a genuinely invalid input return an
error.

## D7 — Values are immutable, and the coefficient is copied on the way out

decimal.js mutates its digit arrays freely inside an operation but always on a
fresh copy of the operands, which is what its 3311-assertion immutability
module checks. A Go `Decimal` holds a slice, so the same discipline needs care:
every operation allocates rather than writing through, and `Coefficient()`
returns a copy so a caller cannot reach in and corrupt a value it was handed.

## D8 — Math.pow's two ECMAScript-only cases are reimplemented

For a non-finite or zero operand, decimal.js falls back to JavaScript's
`Math.pow` on the values converted to numbers. Go's `math.Pow` follows IEEE 754
and disagrees with ECMAScript in exactly two places: `pow(1, NaN)` and
`pow(±1, ±Infinity)` are 1 under IEEE 754 and NaN under ECMAScript. The
original suite asserts the NaN results, so the port carries a small `jsPow`
wrapper rather than calling `math.Pow` directly. This is a real behavioural
difference between the two languages' standard libraries, not a quirk of
decimal.js.

## D9 — `pow` keeps both of decimal.js's paths

An integer exponent takes the exact path: exponentiation by squaring, with the
guard-digit truncation and the detail that makes it safe — when digits are
dropped and the last retained limb is zero it is incremented, so the working
value stays strictly above the true one and the final rounding cannot fall the
wrong way.

Everything else is `exp(y * ln(x))` with guard digits, recomputed at higher
precision when the result lands on a rounding boundary. Its exponent estimate is
kept in a `float64` until it has been range-checked: for an exponent such as
1e21 the estimate runs far past what an `int` can hold, and converting first
wrapped it to the wrong sign, which turned an overflow to Infinity into a
result of zero.

## D10 — Where decimal.js stops rounding, and why it matters

decimal.js carries a module-level `external` flag. When it is false, `plus`,
`times` and the rest skip their final rounding, so intermediate values keep full
width; the flag is cleared inside `sqrt`, `cbrt`, `intPow`, `mod`, `hypot`,
`toFraction`, the Taylor series helper and the `atan` series loop, and nowhere
else. The port threads that through as an explicit `applyLimits` argument rather
than a package-level variable, because a global would make the library unsafe
for concurrent use.

Getting this wrong is silent. Computing `1 + sqrt((1-x)(1+x))` without the
rounding decimal.js applies changed `asin(1e-7)` at precision 5 from
`0.00000010001` to `0.0000001` — a correct-looking answer that disagrees in the
last place.

## D11 — Two JavaScript idioms that carry meaning

Reading past the end of an array gives `undefined`, and the original relies on
what that does next in two opposite ways. In `checkRoundingDigits` the value is
truncated with `| 0`, which turns it into zero, so an absent limb participates
in the comparison; in `divide` and `finalise` it is compared directly, where
`undefined` fails every comparison and so ends a loop. Treating both as "read
zero" or both as "not present" changes results, so each site is transliterated
to match the idiom it uses.

The series functions also pass their `external = true` assignment as the
`isTruncated` argument of the final rounding, so the result is always rounded as
if digits had been discarded. It reads like a typo and is not: without it, `ln`
at a high precision with ROUND_UP comes out two digits short.

## D12 — Test data is generated from decimal.js, not written by hand

Every unit test under `src/` compares against expectations produced by running
decimal.js itself, by the scripts in `tests/port/`. Hand-written expectations
would encode a reading of the source rather than its behaviour, and the point of
the exercise is equivalence. Where a hand-written expectation did slip in, it
was wrong and the port was right — `NewFromInt(-9007199254740991)` was expected
to have four limbs and has three.

## D13 — The benchmark measured the clock, not the code

The first benchmark run reported a p99 of exactly 20 µs for `add` — suspiciously
round, and wrong. The Windows wall clock advances in steps of roughly a
millisecond, and the harness was timing a fixed batch of 50 operations: 1 ms
divided by 50 is 20 µs, so the number was the clock's resolution rather than the
port's latency. Earlier still, with no batching at all, every fast operation
reported 0 ns.

Each operation now grows its batch until a single sample spans at least 20 ms,
and the batch size it settled on is recorded next to every measurement in
`bench/results.json`. Both implementations use the same rule, so the comparison
is not between two different clocks.

## D14 — `toString` is slower, and stays slower

The port is faster than decimal.js on nine of the ten benchmarked operations and
slower on `toString`, at about 0.62× at p99. The cause is known: the port builds
its output through `strings.Builder` and intermediate slices where decimal.js
concatenates JavaScript strings, which V8 optimises heavily.

It is left alone. Formatting is the code whose behaviour is pinned most tightly
by the vendored suite — 500 assertions per formatting method, plus the exponent
thresholds — and a rewrite for speed would put that at risk for a benchmark that
is explicitly a tiebreaker. A disclosed regression is worth more than an
undisclosed risk to the thing that is actually being scored.

## D15 — Bounded operands in the fuzzer, and why that is not cheating

Five draws are restricted: `pow`'s exponent, and the arguments to `exp`, `sinh`,
`cosh` and `tanh`. The reason is upstream, not here.
decimal.js has no shortcut for `exp` below an exponent of 1e17, so `exp(-3e15)`
exhausts the V8 heap before producing anything to compare against; `cosh` is
documented in decimal.js's own source as having been abandoned after a
two-minute wait at 1e7. Left unbounded, the fuzzer would spend its budget
waiting on the reference implementation rather than comparing results.

The restriction is in the harness with the reasoning next to it, so it can be
lifted and re-run by anyone who wants to.

## D16 — A test in the original repo that has never run

Running decimal.js's suite module by module turned up one that cannot start.
`test/modules/powSqrt.js` line 12 is:

```js
for (var e, n, p, r, s; total < 10000; ) {
```

`total` is a free variable. `test/setup.js` keeps its counters as closure
variables inside `T` and publishes them only afterwards as `T.result`, so no
global of that name exists and the loop condition throws
`ReferenceError: total is not defined` before the first assertion. It
reproduces on a clean upstream checkout:

```bash
cd decimal.js && node -e "require('./test/modules/powSqrt.js')"
```

It survives unnoticed because `test/test.js` requires 60 modules by name and
`powSqrt` is not one of them, while `test/modules/` holds 61 files. `npm test`
never loads it.

The loss is real rather than cosmetic. The module compares `pow(0.5)` against
`sqrt` ten thousand times at random precisions and rounding modes, which is the
only place in the suite where the series-based `exp`/`ln` path is checked
against the independent Newton-Raphson path in `squareRoot`. Nothing else pits
those two implementations against each other.

Filed upstream as
[MikeMcl/decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262) on
3 August 2026. The text is kept at `results/upstream-issue.md`.

## D17 — The fix goes in the runner, because the test file is untouchable

`tests/original/` is reproduced byte for byte and hashed; editing the file to
repair it would trade a stronger claim for a weaker one. The defect is a missing
harness global, so the harness supplies it. `adapter/parity-runner.cjs` defines
`total` as a getter over a live assertion count, and wraps each `T.assert*` to
tally as it delegates:

```js
let assertions = 0;
Object.defineProperty(globalThis, 'total', { get: () => assertions });
```

The module uses `total` purely as a loop bound, so nothing about what it checks
changes — every assertion still runs against the port, and the originals still
decide pass or fail. The wrappers only count.

With the counter supplied, the module runs to completion and passes **10000 of
10000** — ten thousand comparisons of `pow(0.5)` against `sqrt` at random
precisions and rounding modes. That is worth two things: it is the strongest
single cross-check this port has, and it is the evidence that the upstream test
is sound and only its loop bound was broken.

Its assertions stay outside the 22658 headline. Upstream's own runner does not
execute this module, so folding them in would break the comparison against the
baseline that number exists to make. `results/parity.txt` reports both figures
and the difference between them: 22658 against the baseline, 32658 including
powSqrt.

---

# Bonus criteria

All four are claimed. Evidence and caveats for each are below; where something
is short of the full claim, it says so.

## Differential Fuzz Survivor

`fuzz/harness.mjs`, log committed at `fuzz/log.txt`:

    # seed 99, budget 90s, reference decimal.js/decimal.mjs
    # 279188 cases in 90.0s, 0 divergence(s)

Ninety continuous seconds against the required sixty, 279188 cases, zero
divergences. Thirty operations across the shared public API — the arithmetic,
`pow`, `sqrt`, `cbrt`, `exp`, `ln`, all the trigonometric and hyperbolic
functions, the rounding methods and the formatters — each drawn against seven
precision and rounding-mode combinations. The run is seeded, so any divergence
is replayable from its seed alone.

Two caveats that belong with the claim:

- Five operations draw from a restricted operand range: `pow`'s exponent, and
  the arguments to `exp`, `sinh`, `cosh` and `tanh`. The reason is upstream, not
  here — decimal.js exhausts the V8 heap on `exp(-3e15)` and its own source
  documents abandoning `cosh` at 1e7 after a two-minute wait. Unbounded draws
  measured patience rather than agreement. The bounds are in the harness with
  the reasoning beside them.
- The oracle is decimal.js, not mpmath. See D1: the mpmath fuzzer decimal.js
  ships does not pass on a clean upstream checkout, so it cannot settle what
  "correct" means for a port whose contract is equivalence.

## Zero Unsafe

The ported library, `src/`, contains:

| Escape hatch | Count |
|---|---|
| `unsafe` | 0 |
| `reflect` | 0 |
| cgo (`import "C"`) | 0 |
| `any` / `interface{}` | 0 |

Its entire import set is `errors`, `fmt`, `math`, `strconv`, `strings`,
`crypto/rand` and `encoding/binary` — the last two only for `Random` when
`Crypto` is set. There are no dependencies beyond the standard library.

The honest asterisk: `adapter/` uses `any` 61 times. That is the JSON
marshalling boundary of the test harness — a wire protocol whose values are
genuinely heterogeneous, and code that is explicitly not part of the product.
The library a caller imports has none of it. Verify with:

```bash
grep -rn "unsafe\.\|reflect\.\|interface{}\|[^a-zA-Z]any[^a-zA-Z]" src/*.go | grep -v _test.go
```

## Bug Catcher

`test/modules/powSqrt.js` in the original repository throws
`ReferenceError: total is not defined` before its first assertion and has never
run. Full analysis in D16; the fix on this side, and why it belongs in the
runner rather than the test file, in D17. Filed upstream as
[MikeMcl/decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262) on
3 August 2026, open at the time of writing; the text is kept at
`results/upstream-issue.md`.

Two things make this more than a typo report. The module is the only place in
the suite that pits the series-based `exp`/`ln` path against the independent
Newton-Raphson `sqrt`, so its silence has cost real coverage for as long as it
has been there. And because the port can now run it, the report comes with
evidence that the test itself is sound: with the missing counter supplied, it
passes 10000 of 10000.

## Write-up

Published account of the port, covering the four entries above where the first
attempt was wrong, the oracle problem, and the adapter decision I would take
back:

**[I ported decimal.js to Go and ran its own 22,658 tests against the result.
Four bugs were mine. One was theirs.](https://dev.to/aaravsharma1/i-ported-decimaljs-to-go-and-ran-its-own-22658-tests-against-the-result-four-bugs-were-mine-one-3d24)** — Dev.to, also on [X](https://x.com/ishadowfighter/status/2084295981173264632)

It covers D2 and D10 (the unrounded intermediate that changed `asin` in the last
digit), D11 (the two opposite meanings JavaScript gives an absent array element),
D13 (the benchmark that reported the Windows clock's resolution as latency), D16
and D17 (the upstream test that had never run), and the adapter decision I would
take back. Source at `writeup/devto.md`; short-form versions at
`writeup/x-posts.md`.

## Decision Log

Seventeen entries above this section, each a real fork in the road with the
reasoning that decided it, including four where the first answer was wrong and
the record says so: the benchmark that measured the Windows clock (D13), the
hand-written test expectation that the port correctly contradicted (D12), the
`toString` regression left in place rather than tuned away (D14), and the
argument reduction that rounded where decimal.js does not (D10).
