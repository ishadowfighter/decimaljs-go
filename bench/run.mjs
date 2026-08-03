// Measures decimal.js on the same operations as bench/bench_test.go, adds
// startup and resident-memory numbers for both sides, and merges everything
// into bench/results.json.
//
//   BENCH=1 go test ./bench/ -run TestMeasure -count 1
//   node bench/run.mjs
//
// Latency is reported as a distribution rather than a mean, because the mean
// hides the reallocation spikes that arbitrary-precision arithmetic produces.

import fs from 'fs';
import path from 'path';
import { execFileSync, spawnSync } from 'child_process';
import { fileURLToPath, pathToFileURL } from 'url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.join(here, '..');
const SAMPLES = 400;
const WARMUP = 2000;
const MIN_SAMPLE_NS = 20e6;
const PRECISION = 34;

const { Decimal } = await import(pathToFileURL(path.join(repoRoot, 'decimal.js', 'decimal.mjs')).href);
Decimal.set({ defaults: true, precision: PRECISION });

const a = new Decimal('12345.6789012345678901234567890123');
const b = new Decimal('9876.54321098765432109876543210987');
const small = new Decimal('1.7320508075688772935274463415059');

const ops = [
  ['parse', () => new Decimal('12345.6789012345678901234567890123')],
  ['add', () => a.plus(b)],
  ['sub', () => a.minus(b)],
  ['mul', () => a.times(b)],
  ['div', () => a.div(b)],
  ['sqrt', () => a.sqrt()],
  ['ln', () => a.ln()],
  ['exp', () => small.exp()],
  ['sin', () => small.sin()],
  ['toString', () => a.toString()],
];

let sink;

// Batched to match the Go side exactly, and to keep a single call from being
// lost in the clock's resolution.
function measure(fn) {
  for (let i = 0; i < WARMUP; i++) sink = fn();

  // Same batching rule as the Go side, so the two distributions are shaped by
  // the same measurement, not by two different clocks.
  let batch = 1;
  for (;;) {
    const t0 = process.hrtime.bigint();
    for (let j = 0; j < batch; j++) sink = fn();
    if (Number(process.hrtime.bigint() - t0) >= MIN_SAMPLE_NS || batch >= 1 << 20) break;
    batch *= 2;
  }

  const times = new Float64Array(SAMPLES);
  for (let i = 0; i < SAMPLES; i++) {
    const t0 = process.hrtime.bigint();
    for (let j = 0; j < batch; j++) sink = fn();
    times[i] = Number(process.hrtime.bigint() - t0) / batch;
  }
  times.sort();
  const mean = times.reduce((s, v) => s + v, 0) / SAMPLES;
  return {
    samples: SAMPLES,
    batch_size: batch,
    mean_ns: mean,
    p50_ns: times[Math.floor(SAMPLES / 2)],
    p99_ns: times[Math.floor((SAMPLES * 99) / 100)],
    max_ns: times[SAMPLES - 1],
    ops_per_sec: 1e9 / mean,
  };
}

const js = ops.map(([op, fn]) => ({ op, ...measure(fn) }));

// Startup: wall time for a process that performs one operation and exits. This
// is where a subprocess adapter pays, and where a Go binary wins.
function timeProcess(cmd, args, input) {
  const runs = 15;
  const times = [];
  for (let i = 0; i < runs; i++) {
    const t0 = process.hrtime.bigint();
    spawnSync(cmd, args, { input, encoding: 'utf8' });
    times.push(Number(process.hrtime.bigint() - t0) / 1e6);
  }
  times.sort((x, y) => x - y);
  return { runs, median_ms: times[Math.floor(runs / 2)], min_ms: times[0], max_ms: times[runs - 1] };
}

const decimald = path.join(repoRoot, 'adapter', 'bin', process.platform === 'win32' ? 'decimald.exe' : 'decimald');
const request = JSON.stringify({
  id: 1,
  op: 'add',
  args: ['12345.6789012345678901234567890123', '9876.54321098765432109876543210987'],
  config: { precision: PRECISION, rounding: 4, modulo: 1, toExpNeg: -7, toExpPos: 21, minE: -9e15, maxE: 9e15, crypto: false },
}) + '\n';

const startup = {
  go_binary: timeProcess(decimald, [], request),
  node_decimaljs: timeProcess(process.execPath, ['-e',
    `const {Decimal}=require(${JSON.stringify(path.join(repoRoot, 'decimal.js', 'decimal.js'))});` +
    `Decimal.set({precision:${PRECISION}});` +
    `new Decimal('12345.6789012345678901234567890123').plus('9876.54321098765432109876543210987').toString();`], ''),
};

// Resident memory. Each runtime reports its own, which is the only measurement
// available on every platform: Go's runtime.MemStats.Sys and Node's
// process.memoryUsage().rss, both taken after the same workload above. They are
// not directly comparable figures, and are labelled as such.
const memory = {
  note: 'self-reported after the benchmark workload; Go reports runtime.MemStats.Sys, Node reports rss',
  go_sys_kb: null,
  node_rss_kb: Math.round(process.memoryUsage().rss / 1024),
};

const goPath = path.join(here, 'results.go.json');
if (!fs.existsSync(goPath)) {
  console.error('run "BENCH=1 go test ./bench/ -run TestMeasure -count 1" first');
  process.exit(2);
}
const go = JSON.parse(fs.readFileSync(goPath, 'utf8'));
memory.go_sys_kb = go.heap_sys_kb;

const byOp = new Map(go.measurements.map((m) => [m.op, m]));
const comparison = js.map((m) => {
  const g = byOp.get(m.op);
  return {
    op: m.op,
    go: { batch_size: g.batch_size, mean_ns: round(g.mean_ns), p50_ns: round(g.p50_ns), p99_ns: round(g.p99_ns), max_ns: round(g.max_ns), ops_per_sec: round(g.ops_per_sec), alloc_bytes_per_op: g.alloc_bytes_per_op },
    decimaljs: { batch_size: m.batch_size, mean_ns: round(m.mean_ns), p50_ns: round(m.p50_ns), p99_ns: round(m.p99_ns), max_ns: round(m.max_ns), ops_per_sec: round(m.ops_per_sec) },
    go_speedup_mean: round(m.mean_ns / g.mean_ns, 2),
    go_speedup_p99: round(m.p99_ns / g.p99_ns, 2),
  };
});

function round(v, places = 0) {
  const f = 10 ** places;
  return Math.round(v * f) / f;
}

const results = {
  measured_at: new Date().toISOString().slice(0, 10),
  methodology: 'bench/methodology.md',
  precision: PRECISION,
  samples_per_op: SAMPLES,
  environment: {
    go: go.runtime,
    goos: go.goos,
    goarch: go.goarch,
    node: process.version,
    decimaljs: '10.6.0',
  },
  operations: comparison,
  startup,
  memory,
};

fs.writeFileSync(path.join(here, 'results.json'), JSON.stringify(results, null, 2) + '\n');
console.log('wrote bench/results.json');
for (const c of comparison) {
  console.log(`${c.op.padEnd(9)} go p99 ${String(c.go.p99_ns).padStart(9)} ns   decimal.js p99 ${String(c.decimaljs.p99_ns).padStart(9)} ns   x${c.go_speedup_p99}`);
}
