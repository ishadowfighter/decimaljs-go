---
title: "I ported decimal.js to Go and ran its own 22,658 tests against the result. Four bugs were mine. One was theirs."
published: false
description: "Porting is the easy half. Proving the port behaves identically is where the work is — and where the interesting failures live."
tags: go, javascript, testing, showdev
canonical_url:
---

I spent a hackathon porting [decimal.js](https://github.com/MikeMcl/decimal.js) — an arbitrary-precision decimal library, ~4,500 lines of dense JavaScript — to Go.

Generating a port is the easy half. Any model will hand you plausible Go. The question that actually matters is: **how do you know it behaves the same?**

My answer was to refuse to write my own tests as the primary proof. decimal.js ships 22,658 assertions across 60 test modules. I vendored them byte-for-byte, hashed them, and ran *those* against the Go build, unmodified.

Final state: **22,658 / 22,658.** Same count a clean upstream checkout produces.

That number is boring. The interesting part is everything that was wrong before it.

---

## The setup: don't touch the tests

The rule I gave myself: `tests/original/` is untouchable. Not to fix a failure, not to tidy. If a test fails, the bug is in my port or my harness.

This is harder than it sounds, because the suite reaches straight into decimal.js's internals:

```js
T.assertEqualProps = function (digits, exponent, sign, n) {
  while (i < len && digits[i] === n.d[i]) ++i;
  if (i === len && i === n.d.length && exponent === n.e && sign === n.s) ...
```

It asserts on `.d` (the base-1e7 limb array), `.e` (exponent), and `.s` (sign). So my Go values had to reproduce not just *results* but *internal representation*.

The plumbing: a Go binary speaking one JSON request per line, and a JavaScript shim presenting decimal.js's API that delegates every operation to it. The redirect point turned out to be free — `tests/original/setup.js` does:

```js
Decimal = require('../decimal');
```

which resolves to `tests/decimal.js`, *outside* the vendored tree. Drop the shim there and the unmodified suite loads it. No launch flags, no module patching, no edited test files.

---

## Bug 1: the limb layout is observable

First thing that surprised me. In decimal.js, `0.1` is not `d: [1]`. It's:

```js
new Decimal('0.1').d   // [1000000]
new Decimal(9e15).d    // [90, 719925, 4740991]  — leading limb is short
```

Limbs are base-1e7, but boundaries are aligned from the **decimal point**, not the start of the digit string. The leading limb is whatever's left over.

You cannot "improve" this. `assertEqualProps` compares the array element by element. Any tidier representation fails thousands of assertions. The whole port is written against this constraint: reproduce the layout, not just the value.

---

## Bug 2: the one that changed a digit and nothing else

This is the one I'd put on a slide.

decimal.js has a module-level flag called `external`. When it's `false`, `plus`, `times` and friends **skip their final rounding**, so intermediates keep full width:

```js
P.plus = function (y) {
  ...
  return external ? finalise(y, pr, rm) : y;
};
```

I threaded this through my Go port as an explicit `applyLimits bool` rather than a package global — a global would make the library unsafe for concurrent use.

Then I ported `asin`:

```js
x = x.div(new Ctor(1).minus(x).times(new Ctor(1).plus(x)).sqrt().plus(1)).atan();
```

I wrote the intermediates unrounded, reasoning that more precision can't hurt.

It can. `asin` never clears `external`. Every one of those operations rounds to the working precision. My result:

```
asin(1e-7) at precision 5, ROUND_UP
  decimal.js:  0.00000010001
  my port:     0.0000001
```

Trace it back and the divergence is one operation: `sqrt(...) + 1`. decimal.js rounds that sum to 11 digits, giving `1.9999999999`. I kept `1.99999999999`. Divide `1e-7` by each and the eleventh digit differs — which is exactly the digit `ROUND_UP` was looking at.

**More precision produced a wrong answer.** That's the lesson: in a port, "better" is a category error. The target isn't accuracy, it's *agreement*.

I audited every function against the original's `external` handling. It's cleared in exactly seven places: `sqrt`, `cbrt`, `intPow`, `mod`, `hypot`, `toFraction`, the Taylor series helper, and the `atan` series loop. Nowhere else.

---

## Bug 3: `undefined | 0` is 0, and that's load-bearing

JavaScript reading past an array end gives `undefined`. decimal.js relies on what happens *next* — in two opposite ways, in the same file.

In `checkRoundingDigits`:

```js
rd = d[di] % k | 0;
... (d[di + 1] / k / 100 | 0) == 0
```

`undefined / k / 100` is `NaN`, and `NaN | 0` is **0**. So an absent limb participates in the comparison as zero.

In `divide`:

```js
} while ((xi++ < xL || rem[0] !== void 0) && sd--);
```

Here `undefined` is compared directly, fails every comparison, and *terminates the loop*.

I first modelled absent limbs as "not present" everywhere — a `hasNext` flag, comparisons returning false. Clean Go. Wrong: `ln` at high precision with certain rounding modes drifted in the last digit, because a boundary check that should have fired didn't.

Two idioms, opposite meanings, and Go has no `undefined` to lean on. Each site has to be transliterated to match *which* idiom it uses.

The best one in this family:

```js
return finalise(sum, Ctor.precision = pr, rm, external = true);
```

That fourth argument is `isTruncated`. It's an **assignment expression** — `external = true` evaluates to `true`. So the series result is always rounded as if digits were discarded. It reads like a typo. It isn't. Without it, `ln` at precision 85 with `ROUND_UP` comes out two digits short.

---

## Bug 4: an int that wrapped

`pow` estimates its result exponent before computing anything, to bail out early on overflow:

```js
e = mathfloor(yn * (Math.log('0.' + digitsToString(x.d)) / Math.LN10 + x.e + 1));
if (e > Ctor.maxE + 1 || e < Ctor.minE - 1) return new Ctor(e > 0 ? s / 0 : 0);
```

In JavaScript that arithmetic is float64 throughout. I converted to `int` first — reasonable-looking, and fine until the exponent is large:

```
1.5 ** 1e21
  expected:  Infinity
  got:       0
```

The estimate is ~1.76e20. `int64` maxes at ~9.2e18. It wrapped negative, the sign check took the underflow branch, and overflow-to-infinity became zero. The fix is to range-check in `float64` and convert only afterwards.

A fuzzer found this in seconds. I would not have found it by reading.

---

## The bug that wasn't mine

Running the suite module by module, one refused to start:

```
$ node -e "require('./test/modules/powSqrt.js')"
 Testing pow against sqrt...
ReferenceError: total is not defined
```

Line 12:

```js
for (var e, n, p, r, s; total < 10000; ) {
```

`total` is a free variable. `setup.js` keeps its counters as closure variables inside `T` and publishes them only afterwards as `T.result`. There's no global by that name, so the loop condition throws before the first assertion.

Why has nobody noticed? `test/test.js` requires 60 modules by name. `test/modules/` contains 61. `powSqrt` isn't on the list, so `npm test` never loads it.

That test has never run. And it's a good one — it compares `pow(0.5)` against `sqrt()` ten thousand times at random precisions and rounding modes, which is the only place in the suite where the series-based `exp`/`ln` path is checked against the independent Newton–Raphson `sqrt`.

Filed as [decimal.js#262](https://github.com/MikeMcl/decimal.js/issues/262).

Since I couldn't edit the vendored file, my test runner supplies the missing counter as a getter over a live assertion count. The module then runs — and passes **10,000 / 10,000**, which is also the evidence that the test itself is sound and only its loop bound was broken.

Those 10,000 assertions stay *out* of my headline number. 22,658 exists to be compared against upstream's baseline, and upstream doesn't run this module. Folding them in would inflate a figure whose only purpose is comparison.

---

## The oracle problem

decimal.js ships a differential fuzzer (`test/hypothesis/error_hunt.py`) that checks it against mpmath. Obvious move: point it at my Go build.

I ran it against **unmodified decimal.js** first, to establish a baseline. It fails:

```
assert Decimal('5.5360303649385E-8') == Decimal('5.5360303649386E-8')
  fn='sin', x=6504783935.0000, precision=14
```

Last-digit disagreement between decimal.js and mpmath from cancellation in the argument reduction. Not a bug in either, exactly — a consequence of decimal.js's approach at that precision.

But it settles the oracle question. My contract is *equivalence with decimal.js*, not mathematical truth. An oracle that disagrees with the thing I'm meant to equal can't adjudicate.

So I wrote my own: same random operands, same config, both implementations, compare strings. **279,188 cases in 90 seconds, zero divergences**, seeded so any failure is replayable.

Five operations draw bounded operands, and I'd rather say so than have it found: decimal.js exhausts the V8 heap on `exp(-3e15)`, and its own source comments record abandoning `cosh` at 1e7 after a two-minute wait. Unbounded draws measure patience, not agreement.

---

## The benchmark that measured my clock

First run, p99 for `add`: **exactly 20 µs.** Suspiciously round.

The Windows wall clock advances in ~1 ms steps. I was timing fixed batches of 50 operations. 1 ms ÷ 50 = 20 µs. I had measured the clock's resolution and printed it as latency. The run *before* that reported `0 ns` for every fast operation, which at least had the decency to look wrong.

Fixed by growing each batch until a sample spans ≥20 ms, recording the batch size next to every measurement. Real numbers, p99 at precision 34:

| Operation | Go | decimal.js | Ratio |
|---|---:|---:|---:|
| parse | 0.68 µs | 3.80 µs | **5.6×** |
| add | 1.11 µs | 3.44 µs | **3.1×** |
| div | 6.02 µs | 32.2 µs | **5.4×** |
| sqrt | 36.6 µs | 182 µs | **5.0×** |
| ln | 360 µs | 550 µs | **1.5×** |
| **toString** | **2.13 µs** | **1.32 µs** | **0.62× — slower** |

`toString` is slower and I left it slower. The port builds output through `strings.Builder` and intermediate slices where V8 optimises string concatenation heavily. I could fix it — but formatting is the code most tightly pinned by the suite (500 assertions per method, plus exponent thresholds), and rewriting it for a tiebreaker metric risks the thing actually being scored. A disclosed regression beats an undisclosed risk.

Startup: **17 ms** for the Go binary against **82 ms** for Node loading decimal.js.

---

## The decision I'd take back

The adapter spawns a fresh process per call. `spawnSync`, one JSON line in, one out.

It's simple, stateless, and portable — a persistent child with synchronous reads isn't portable on Windows, where a piped stdio stream has no fd `fs.readSync` will accept, and the worker-thread + `Atomics.wait` workaround is heavy.

But 22,658 assertions × a process spawn each is *minutes* per full run. During development that's a brutal feedback loop, and it made one module — 10,000 iterations, each spawning twice — take over half an hour.

I should have paid the portability cost up front for a persistent process, or run the parity suite in a Linux container from day one where the simple version works. The protocol is already streaming and stateless; it's a one-file swap. I just never had a quiet moment to make it.

---

## What I'd tell someone starting one of these

**Build the rounding engine first, alone, and test it in isolation.** `finalise` is 120 lines of digit-position bookkeeping that every arithmetic result and every formatting method passes through. I built it before anything depended on it and checked it against 5,508 generated cases across 50 values, 12 digit counts and all nine rounding modes. It was correct on the first run, and I've never had to revisit it. One bug there fails thousands of assertions in bulk; getting it right early makes every later failure *local*.

**Generate your expectations, don't write them.** Every unit test compares against output produced by running decimal.js itself. A hand-written expectation encodes your *reading* of the source; a generated one encodes its behaviour. The one hand-written expectation that slipped in was wrong — I claimed `NewFromInt(-9007199254740991)` had four limbs. It has three. The port was right and I was wrong, which is precisely the failure mode generated data prevents.

**Decide what "correct" means before you start.** Every hard call collapsed into one rule: *behaviour beats idiom*. Signed zero, NaN's sign field, `Math.pow(1, NaN)` being `NaN` in ECMAScript but `1` in IEEE 754 — each one is a place where the Go-shaped answer and the correct answer differ. Write the rule down first and the decisions make themselves.

---

**Repo:** [github.com/ishadowfighter/decimaljs-go](https://github.com/ishadowfighter/decimaljs-go) — 22,658/22,658, full parity table, fuzz log, benchmark methodology, and a `DECISIONS.md` with 17 entries including the four above where I was wrong first.

Built for [Port Mortem / Code Resurrection](https://coderesurrection.com) by Hackathon Raptors.
