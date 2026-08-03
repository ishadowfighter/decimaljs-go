# X / Twitter

Two options: a thread (better for the write-up prize — it carries the technical
substance) and a single standalone post if you want something lighter.

Tag `@hackathonraptors`. Post the thread, then reply to your own last post with
the repo link if you'd rather keep link-suppression off the first post.

---

## Option A — thread (10 posts)

**1/**
I ported decimal.js to Go for a hackathon.

Generating the port was the easy half. Proving it behaves identically is where
the work was.

I ran decimal.js's own 22,658 assertions against the Go build, unmodified.

22,658 / 22,658.

Four of the bugs I hit were interesting. 🧵

**2/**
Rule I set: the vendored test files are untouchable. If a test fails, the bug is
in my port or my harness — never the test.

That's harder than it sounds. The suite asserts on decimal.js's *internals*:
`.d` (base-1e7 limbs), `.e`, `.s`.

So the port had to reproduce representation, not just results.

**3/**
First surprise: `new Decimal('0.1').d` is `[1000000]`, not `[1]`.

Limb boundaries align from the decimal point, not the start of the digit string.
The leading limb is the leftover.

You can't tidy this up. The tests compare the array element by element.

**4/**
The bug I'd put on a slide:

decimal.js has an `external` flag. When false, arithmetic skips its final
rounding. `asin` never clears it — so every intermediate rounds.

I kept mine unrounded, reasoning more precision can't hurt.

```
asin(1e-7), precision 5, ROUND_UP
decimal.js: 0.00000010001
mine:       0.0000001
```

**5/**
One operation caused it: `sqrt(...) + 1`.

decimal.js rounds that to 11 digits → 1.9999999999
I kept 1.99999999999

The 11th digit is exactly what ROUND_UP was looking at.

**More precision produced a wrong answer.** In a port, "better" is a category
error. The target is agreement, not accuracy.

**6/**
JavaScript detail that's load-bearing in two opposite directions:

`undefined | 0` → 0, so an absent array element participates as zero.
`undefined` compared directly → false, so it terminates a loop.

Same file. Both relied on. Go has no `undefined` to lean on, so each site has to
match *which* idiom it uses.

**7/**
Best line in the original:

```js
finalise(sum, Ctor.precision = pr, rm, external = true);
```

That 4th arg is `isTruncated`. `external = true` is an assignment expression — it
evaluates to `true`.

Reads like a typo. Isn't. Without it, `ln` at precision 85 comes out two digits
short.

**8/**
Then I found one that wasn't mine.

`test/modules/powSqrt.js` loops on a free variable `total` that nothing defines.
It throws before its first assertion.

Nobody noticed because `test.js` lists 60 modules and the folder has 61. It's not
on the list.

That test has never run.

**9/**
Filed upstream: github.com/MikeMcl/decimal.js/issues/262

My runner supplies the missing counter (can't edit vendored files), and the
module then passes 10,000/10,000 — which also proves the test itself is sound
and only its loop bound was broken.

Those 10k stay OUT of my headline number. Different baseline.

**10/**
Honest numbers, because that's the point:

Go is 1.5–5.6× faster at p99 on 9 of 10 ops.
`toString` is **0.62× — slower.** Left it slower; rewriting it risks the
behaviour the whole suite pins.

My first benchmark reported p99 = exactly 20µs. That was the Windows clock's
resolution ÷ my batch size, not a measurement.

Repo + full write-up: github.com/ishadowfighter/decimaljs-go
@hackathonraptors #PortMortem

---

## Option B — single post

Ported decimal.js (JS) → Go, then ran its own 22,658 assertions against the Go
build unmodified. 22,658/22,658.

Best bug: I kept an intermediate *unrounded*, thinking more precision couldn't
hurt. It changed asin(1e-7) in the last digit. In a port, "better" is a category
error — you want agreement, not accuracy.

Also found a decimal.js test that has never run: it loops on an undefined global
and isn't in the module list, so nobody noticed. Filed #262. Made it runnable —
passes 10,000/10,000.

And my first benchmark measured the Windows clock's resolution instead of my
code (p99 of exactly 20µs = 1ms ÷ batch of 50).

Write-up + repo: github.com/ishadowfighter/decimaljs-go
@hackathonraptors

---

## LinkedIn variant

Same substance, one block, less staccato:

---

I spent a hackathon porting decimal.js — an arbitrary-precision decimal library
in ~4,500 lines of dense JavaScript — to Go.

Generating a port is the easy half. Proving it behaves identically is the part
almost nobody does, so that's where I put the effort: decimal.js ships 22,658
assertions across 60 test modules, and I ran those against the Go build
unmodified, with the test files vendored byte-for-byte and hashed.

Final result: 22,658 / 22,658 — the same count a clean upstream checkout
produces.

The interesting part is what was wrong before that.

The bug I'd put on a slide: decimal.js carries a flag that makes arithmetic skip
its final rounding for intermediate values. I kept certain intermediates
unrounded, reasoning that more precision can't hurt. It can — asin(1e-7) at
precision 5 came out as 0.0000001 instead of 0.00000010001, because the digit
the rounding mode was inspecting had moved. More precision produced a wrong
answer. In a port, "better" is a category error: the target is agreement, not
accuracy.

I also found a defect that wasn't mine. One of decimal.js's test modules loops on
an undefined global and throws before its first assertion — and it isn't in the
list of 60 modules the test runner executes, so it has never run at all. Filed
upstream as issue #262. My harness supplies the missing counter without touching
the vendored file, and the module then passes 10,000/10,000, which is also the
evidence that the test itself is sound.

On numbers: the Go port is 1.5–5.6× faster at p99 on nine of ten operations, and
0.62× — slower — on toString. I left it slower and documented why. My first
benchmark run reported a p99 of exactly 20 µs, which turned out to be the Windows
clock's resolution divided by my batch size rather than a measurement.

Full write-up, parity table, fuzz log and a DECISIONS.md with 17 entries —
including four where I was wrong first:
github.com/ishadowfighter/decimaljs-go

Built for Port Mortem / Code Resurrection by Hackathon Raptors.
