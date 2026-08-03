# Benchmark methodology

Performance is a tiebreaker for this port, not its purpose: equivalence was
locked first and nothing here was optimised for. These numbers exist so the
cost of the port is stated rather than guessed at, including where it is worse.

## What is measured

Ten operations, on the same operands, at `precision: 34` — IEEE 754 decimal128,
a realistic working precision rather than a flattering one:

    parse  add  sub  mul  div  sqrt  ln  exp  sin  toString

`add`, `sub`, `mul` and `div` use two 32-digit operands; `ln` and `sqrt` use the
larger of them; `exp` and `sin` use a value near √3, since both are far slower
for a large argument and would otherwise measure the argument reduction alone.

## How

- **Latency distribution, not a mean.** Each operation reports p50, p99 and the
  maximum over 400 samples. Arbitrary-precision arithmetic reallocates, so the
  mean hides exactly the spikes a caller cares about.
- **Adaptive batching.** The Windows wall clock advances in steps of roughly a
  millisecond. Timing a one-microsecond operation against it reports the clock's
  resolution, not the operation: an earlier run of this benchmark produced a
  p99 of exactly 20 µs for `add`, which is one clock tick divided by a fixed
  batch of 50. Each operation now grows its batch until a single sample spans at
  least 20 ms, and the batch size used is recorded in `results.json` next to
  every measurement. Both sides use the same rule, so the two distributions are
  shaped by the same method rather than by two different clocks.
- **2000 warm-up iterations** before measurement, so the JavaScript side is
  running optimised code and the Go side has a warm allocator.
- **Results are kept reachable** through a package-level sink on the Go side and
  a module-level one in JavaScript, so neither compiler can delete the work.
- **Allocation** is Go-only: `runtime.MemStats.TotalAlloc` divided by the
  operation count. There is no comparable per-operation figure from V8.

## Startup

Wall time for a process that performs one addition and exits, median of 15 runs:
the Go binary answering one line of the adapter protocol, against `node -e`
loading decimal.js and doing the same addition. This is the number that matters
for a short-lived process, and it is also the cost the test harness pays on
every call, since it spawns the binary per operation.

## Memory

Each runtime reports its own figure after the benchmark workload: Go's
`runtime.MemStats.Sys`, Node's `process.memoryUsage().rss`. These are **not**
directly comparable — one is what the Go runtime obtained from the OS, the other
is the resident set of a whole V8 process — and they are labelled as such in
`results.json`. A peak-working-set measurement of both processes from outside
would be comparable, and is not portable enough to include here.

## Reproducing

```bash
go build -o adapter/bin/decimald ./adapter/cmd/decimald
BENCH=1 go test ./bench/ -run TestMeasure -count 1   # writes bench/results.go.json
node bench/run.mjs                                    # writes bench/results.json
```

Both scripts print their numbers as they go. `results.go.json` is the raw Go
half; `results.json` is the merged report.

## What the numbers say

At the time of measurement, on Windows 10 / amd64 with Go 1.26.3 and Node
22.16.0, against decimal.js 10.6.0:

- The Go port is faster on nine of the ten operations, by roughly 1.5× to 5.6×
  at p99. Arithmetic and `parse` gain the most.
- **`toString` is slower — about 0.6× at p99.** The port builds its output
  through `strings.Builder` and several intermediate slices where decimal.js
  concatenates JavaScript strings, which V8 optimises heavily. This is a real
  regression and is left in rather than tuned away, because tuning it would mean
  touching code whose behaviour is currently pinned digit for digit.
- Startup is where the port wins outright: about 17 ms against about 82 ms.
  That gap is also the harness's per-call tax, and the reason the full parity
  run takes minutes rather than seconds.
- Nothing here has been profiled or optimised. The port is a faithful
  transliteration, including algorithmic choices that a Go-native implementation
  would make differently — the base-1e7 limb layout exists to match decimal.js's
  rounding boundaries, not because it is the fastest representation available.
