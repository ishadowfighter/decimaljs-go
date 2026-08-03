# X posts — all under 280 characters

The free tier caps a post at 280 characters. Everything below fits, counted with
a link as 23 characters (X's fixed cost for any URL, however long).

Link to use: https://dev.to/aaravsharma1/i-ported-decimaljs-to-go-and-ran-its-own-22658-tests-against-the-result-four-bugs-were-mine-one-3d24

Where a post is at the limit, `@raptors_hack` goes in a **reply** rather than the
post itself — a reply mention still notifies them and still associates the post.

---

## Standalone posts

Pick one. **A** is the strongest opener; **B** is the most likely to travel,
because "a test that has never run" is a hook on its own.

### A — the bug (279 chars)

```
Ported decimal.js → Go. Ran its own 22,658 tests against the Go build unmodified: 22,658/22,658.

Best bug: I left an intermediate unrounded — more precision can't hurt, right? It changed asin(1e-7)'s last digit.

In a port, "better" is a category error.

<LINK>
```

Reply with: `Write-up for @raptors_hack Port Mortem 2026.`

### B — the never-run test (278 chars)

```
Porting decimal.js to Go, I found a test of theirs that has never run.

It loops on an undefined global, so it throws before the first assertion — and it isn't in the module list, so nobody noticed.

Filed #262. Made it run: 10,000/10,000.

<LINK> @raptors_hack
```

### C — the benchmark (265 chars)

```
My first benchmark said p99 = exactly 20µs.

Suspiciously round. It was the Windows clock's 1ms resolution ÷ my batch of 50. I'd measured the clock and printed it as latency.

Write-up: porting decimal.js to Go, 22,658/22,658.

<LINK> @raptors_hack
```

---

## Thread — 7 posts, each under 280

Post 1 first, then reply to each in turn. Only the last carries the link, so the
earlier posts are not de-ranked for it.

**1/7** — 224 chars

```
Ported decimal.js (JS) → Go for a hackathon.

Generating the port is the easy half. Proving it behaves identically is the work.

So I ran decimal.js's own 22,658 assertions against the Go build, unmodified.

22,658/22,658. 🧵
```

**2/7** — 247 chars

```
The rule: the vendored test files are untouchable. If a test fails, the bug is mine.

Harder than it sounds — the suite asserts on decimal.js's internals: .d (base-1e7 limbs), .e, .s.

So the port had to reproduce representation, not just results.
```

**3/7** — 235 chars

```
Best bug:

decimal.js skips rounding on intermediates behind an internal flag. asin never clears it.

I kept mine unrounded, thinking more precision can't hurt.

asin(1e-7), precision 5, ROUND_UP
theirs: 0.00000010001
mine:   0.0000001
```

**4/7** — 231 chars

```
One operation caused it: sqrt(...) + 1

They round to 11 digits → 1.9999999999
I kept 1.99999999999

The 11th digit is exactly what ROUND_UP was inspecting.

More precision produced a wrong answer. You want agreement, not accuracy.
```

**5/7** — 245 chars

```
Then I found a bug that wasn't mine.

decimal.js's powSqrt.js loops on a free variable `total` that nothing defines. Throws before the first assertion.

test.js lists 60 modules. The folder has 61. It's not on the list.

That test has never run.
```

**6/7** — 264 chars

```
Filed decimal.js#262.

My runner supplies the missing counter (can't edit vendored files) and the module then passes 10,000/10,000 — which also proves the test itself is sound, only its loop bound was broken.

Those 10k stay OUT of my headline. Different baseline.
```

**7/7** — 272 chars

```
Honest numbers:

Go is 1.5–5.6x faster at p99 on 9 of 10 ops.
toString is 0.62x — slower. Left it slower; a rewrite risks the behaviour the suite pins.

My first benchmark reported p99 of exactly 20µs: the Windows clock ÷ batch size.

<LINK> @raptors_hack
```
