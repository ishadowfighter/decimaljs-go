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

## D4 — Module path is a placeholder

`go.mod` declares `github.com/USER/decimaljs-go`. Guessing an account name into
a committed file is worse than an obviously-unfilled placeholder, so the owning
account is left for the repository owner to substitute.
