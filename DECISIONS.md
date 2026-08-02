# Decisions

Every non-trivial fork in the road taken while porting decimal.js to Go, and
why it was taken. Written as the work happens, not reconstructed afterwards.

Behaviour beats idiom: where an idiomatic Go choice would change an observable
result, decimal.js's behaviour wins and the compromise is recorded here.

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

## D9 — Integer exponents only, for now

`Pow` implements decimal.js's exact path: exponentiation by squaring for an
integer exponent, including the guard-digit truncation and the trick of
incrementing the last retained limb when digits were dropped, which keeps the
truncated value strictly above the true one so the final rounding cannot fall
the wrong way.

A fractional exponent needs `exp` and `ln`, which are Tier 2 and not ported yet.
Rather than approximate it, `Pow` returns `ErrNotImplemented`. An exponent
beyond the exact-integer range does the same, since that path also routes
through the logarithm upstream. A reported gap is worth more than a wrong
answer that looks like a result.
