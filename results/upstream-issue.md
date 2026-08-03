# Upstream issue — decimal.js

**Filed: https://github.com/MikeMcl/decimal.js/issues/262** (3 August 2026, open)

Found while porting decimal.js to Go and running its test suite unmodified
against the port. The text below is what was filed.

---

**Title:** `test/modules/powSqrt.js` has never run — `ReferenceError: total is not defined`

---

`test/modules/powSqrt.js` cannot execute. It throws before its first assertion,
on a clean checkout of `master`, on any Node version.

### Reproduction

```bash
git clone https://github.com/MikeMcl/decimal.js
cd decimal.js
node -e "require('./test/modules/powSqrt.js')"
```

```
 Testing pow against sqrt...
ReferenceError: total is not defined
    at .../test/modules/powSqrt.js:12
```

### Cause

Line 12 loops on a free variable:

```js
for (var e, n, p, r, s; total < 10000; ) {
```

Nothing defines `total`. `test/setup.js` keeps its counters as closure variables
inside `T` — `passed` and `testNumber` — and exposes them only afterwards, as
`T.result`. There is no global of that name, so the comparison throws on the
first evaluation of the loop condition.

`test/test.js` computes a local `total` while summing results, which looks like
where the name came from, but that variable is not in scope here and the module
is required in its own right.

### Why it went unnoticed

`test/test.js` lists 60 modules to require, and `powSqrt` is not among them —
`test/modules/` holds 61 files. So `npm test` never loads it, and the failure
never surfaces.

### Why it matters

The module is a genuinely valuable cross-check that is currently doing nothing.
It compares `r.pow(0.5)` against `r.sqrt()` for random values, at a random
rounding mode and a random precision in [1, 40], ten thousand times — which
exercises `naturalExponential` and `naturalLogarithm` against the independent
Newton-Raphson path in `squareRoot`. Nothing else in the suite pits those two
implementations against each other.

### Suggested fixes

Either would do:

1. **Use the harness's own counter.** `T.result` is only set once the module
   finishes, so this needs a live counter — e.g. exposing `testNumber` from
   `setup.js`, and looping on that.

2. **Loop a fixed number of times**, which is what the code appears to intend:

```js
for (var e, n, p, r, s, i = 0; i < 10000; i++) {
```

Then add `'powSqrt'` to the module list in `test/test.js` so it actually runs.
Be aware it is slow — ten thousand `pow(0.5)` calls at up to 40 significant
digits.

### Confirmation that it passes once it can run

Ported to Go (JavaScript → Go, decimal.js 10.6.0 at `cd73a7f`), this module was
run against the port by supplying the missing counter from the test runner
rather than editing the file. It completes and every assertion passes, so the
defect is only that the file cannot start — the test itself is sound.
