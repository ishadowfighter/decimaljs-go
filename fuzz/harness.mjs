// Differential fuzzer: the Go port against decimal.js, operation by operation.
//
//   node fuzz/harness.mjs --seconds 60
//   node fuzz/harness.mjs --seconds 60 --seed 12345 > fuzz/log.txt
//
// Both sides are given the same operands and the same configuration, and their
// results are compared as strings. decimal.js runs in process; the Go build
// answers over the adapter's line protocol, so what is compared is the same
// binary the test harness drives.
//
// mpmath is deliberately not the oracle here. The upstream fuzzer
// (test/hypothesis/error_hunt.py) compares decimal.js against mpmath and does
// not pass on a clean upstream checkout: at precision 14, sin(6504783935) is
// 5.5360303649386E-8 from decimal.js and 5.5360303649385E-8 from mpmath. This
// port's contract is equivalence with decimal.js, so decimal.js is the oracle
// and any difference is a bug in the port.

import { spawn } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';
import { pathToFileURL } from 'url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.join(here, '..');

const argv = process.argv.slice(2);
const readFlag = (name, fallback) => {
  const i = argv.indexOf('--' + name);
  return i === -1 ? fallback : argv[i + 1];
};

const seconds = Number(readFlag('seconds', 60));
let seed = Number(readFlag('seed', Date.now() % 2147483647));
const referencePath = readFlag('reference', path.join(repoRoot, 'decimal.js', 'decimal.mjs'));

const { Decimal } = await import(pathToFileURL(referencePath).href);

// A small deterministic generator, so a divergence can be replayed from its
// seed alone.
function nextRandom() {
  seed = (seed * 16807) % 2147483647;
  return (seed - 1) / 2147483646;
}

const pick = (xs) => xs[Math.floor(nextRandom() * xs.length) % xs.length];
const intBetween = (lo, hi) => lo + Math.floor(nextRandom() * (hi - lo + 1));

// Operand shapes worth hitting: the special values, small integers, values
// near one, and values spread across the exponent range.
function randomOperand() {
  const shape = intBetween(0, 11);
  switch (shape) {
    case 0: return pick(['0', '-0', '1', '-1', 'NaN', 'Infinity', '-Infinity']);
    case 1: return String(intBetween(-1000, 1000));
    case 2: return (nextRandom() < 0.5 ? '' : '-') + '0.' + randomDigits(intBetween(1, 25));
    case 3: return '1.' + randomDigits(intBetween(1, 30));
    case 4: return '0.999999999999999999' + randomDigits(intBetween(0, 10));
    case 5: return '1.000000000000000001' + randomDigits(intBetween(0, 10));
    default: {
      const sign = nextRandom() < 0.5 ? '' : '-';
      const digits = randomDigits(intBetween(1, 40));
      const exp = intBetween(-60, 60);
      return `${sign}${digits[0]}.${digits.slice(1) || '0'}e${exp}`;
    }
  }
}

// moderate draws a value small enough for the hyperbolic functions to finish.
function moderate() {
  if (nextRandom() < 0.2) return pick(['0', '-0', '1', '-1', 'NaN', 'Infinity', '-Infinity']);
  const sign = nextRandom() < 0.5 ? '' : '-';
  return `${sign}${intBetween(0, 200)}.${randomDigits(intBetween(1, 25))}`;
}

// smallExponent draws an exponent in a range where both implementations
// finish quickly.
function smallExponent() {
  if (nextRandom() < 0.3) return String(intBetween(-40, 40));
  return (nextRandom() < 0.5 ? '' : '-') + intBetween(0, 40) + '.' + randomDigits(intBetween(1, 12));
}

function randomDigits(n) {
  let s = '';
  for (let i = 0; i < n; i++) s += String(intBetween(0, 9));
  return s;
}

// Operations, with the arity and any domain restriction the fuzzer respects so
// that most draws land on a defined result rather than NaN.
const OPERATIONS = [
  { name: 'add', op: 'add', arity: 2, js: (x, y) => x.plus(y) },
  { name: 'sub', op: 'sub', arity: 2, js: (x, y) => x.minus(y) },
  { name: 'mul', op: 'mul', arity: 2, js: (x, y) => x.times(y) },
  { name: 'div', op: 'div', arity: 2, js: (x, y) => x.div(y) },
  { name: 'mod', op: 'mod', arity: 2, js: (x, y) => x.mod(y) },
  { name: 'divToInt', op: 'divToInt', arity: 2, js: (x, y) => x.divToInt(y) },
  // The exponent is kept moderate: decimal.js itself takes minutes on
  // something like 1.000000000000000001^9.9e58, which would stall the run
  // without saying anything about equivalence.
  { name: 'pow', op: 'pow', arity: 2, operand: (i) => (i === 1 ? smallExponent() : randomOperand()), js: (x, y) => x.pow(y) },
  { name: 'sqrt', op: 'sqrt', arity: 1, js: (x) => x.sqrt() },
  { name: 'cbrt', op: 'cbrt', arity: 1, js: (x) => x.cbrt() },
  // exp only shortcuts above 1e17, so an argument like -3e15 runs decimal.js
  // out of memory before it produces anything to compare.
  { name: 'exp', op: 'exp', arity: 1, operand: moderate, js: (x) => x.exp() },
  { name: 'ln', op: 'ln', arity: 1, js: (x) => x.ln() },
  { name: 'sin', op: 'sin', arity: 1, js: (x) => x.sin() },
  { name: 'cos', op: 'cos', arity: 1, js: (x) => x.cos() },
  { name: 'tan', op: 'tan', arity: 1, js: (x) => x.tan() },
  { name: 'atan', op: 'atan', arity: 1, js: (x) => x.atan() },
  { name: 'asin', op: 'asin', arity: 1, js: (x) => x.asin() },
  { name: 'acos', op: 'acos', arity: 1, js: (x) => x.acos() },
  // The hyperbolic functions have no shortcut for a large argument, and
  // decimal.js itself is documented as abandoning cosh(1e7). Their operands are
  // kept moderate so the run measures agreement rather than patience.
  { name: 'sinh', op: 'sinh', arity: 1, operand: moderate, js: (x) => x.sinh() },
  { name: 'cosh', op: 'cosh', arity: 1, operand: moderate, js: (x) => x.cosh() },
  { name: 'tanh', op: 'tanh', arity: 1, operand: moderate, js: (x) => x.tanh() },
  { name: 'atanh', op: 'atanh', arity: 1, js: (x) => x.atanh() },
  { name: 'asinh', op: 'asinh', arity: 1, js: (x) => x.asinh() },
  { name: 'acosh', op: 'acosh', arity: 1, js: (x) => x.acosh() },
  { name: 'round', op: 'round', arity: 1, js: (x) => x.round() },
  { name: 'floor', op: 'floor', arity: 1, js: (x) => x.floor() },
  { name: 'trunc', op: 'trunc', arity: 1, js: (x) => x.trunc() },
  { name: 'toBinary', op: 'toBinary', arity: 1, string: true, extra: ['absent', 4], js: (x) => x.toBinary() },
  { name: 'toHex', op: 'toHex', arity: 1, string: true, extra: ['absent', 4], js: (x) => x.toHexadecimal() },
  { name: 'toString', op: 'toString', arity: 1, string: true, js: (x) => x.toString() },
  { name: 'valueOf', op: 'valueOf', arity: 1, string: true, js: (x) => x.valueOf() },
];

// A persistent decimald process, driven one line at a time.
const binary = path.join(repoRoot, 'adapter', 'bin', process.platform === 'win32' ? 'decimald.exe' : 'decimald');
const child = spawn(binary, [], { stdio: ['pipe', 'pipe', 'inherit'] });
child.stdout.setEncoding('utf8');

let pending = null;
let buffered = '';
child.stdout.on('data', (chunk) => {
  buffered += chunk;
  let i;
  while ((i = buffered.indexOf('\n')) !== -1) {
    const line = buffered.slice(0, i);
    buffered = buffered.slice(i + 1);
    const resolve = pending;
    pending = null;
    if (resolve) resolve(JSON.parse(line));
  }
});

let nextId = 1;
function ask(op, args, config) {
  return new Promise((resolve) => {
    pending = resolve;
    child.stdin.write(JSON.stringify({ id: nextId++, op, args, config }) + '\n');
  });
}

const CONFIGS = [
  { precision: 20, rounding: 4 },
  { precision: 1, rounding: 4 },
  { precision: 7, rounding: 0 },
  { precision: 7, rounding: 3 },
  { precision: 7, rounding: 6 },
  { precision: 34, rounding: 4 },
  { precision: 60, rounding: 1 },
];

const started = Date.now();
const counts = new Map();
const divergences = [];
let cases = 0;

console.log(`# differential fuzz: Go port vs decimal.js`);
console.log(`# seed ${seed}, budget ${seconds}s, reference ${path.relative(repoRoot, referencePath)}`);

while ((Date.now() - started) / 1000 < seconds) {
  const spec = pick(OPERATIONS);
  const cfg = { ...pick(CONFIGS), modulo: 1, toExpNeg: -9e15, toExpPos: 9e15, minE: -9e15, maxE: 9e15, crypto: false };
  const args = [];
  for (let i = 0; i < spec.arity; i++) args.push(spec.operand ? spec.operand(i) : randomOperand());

  Decimal.set({ ...cfg, defaults: true });
  if (process.env.FUZZ_TRACE) process.stderr.write(`${spec.name}(${args.join(', ')}) pr=${cfg.precision}
`);

  let expected;
  try {
    const operands = args.map((a) => new Decimal(a));
    const r = spec.js(...operands);
    expected = spec.string ? r : r.valueOf();
  } catch (e) {
    // decimal.js threw; the port is expected to fail too, which the parity
    // suite already covers, so these draws are skipped.
    continue;
  }

  // Some operations take a digit count and rounding mode the fuzzer does not
  // vary; "absent" is the protocol's spelling for an omitted digit count.
  const reply = await ask(spec.op, spec.extra ? [...args, ...spec.extra] : args, cfg);
  let actual;
  if (!reply.ok) {
    actual = 'ERROR: ' + reply.error;
  } else {
    actual = spec.string ? reply.value : reply.value.str === undefined ? String(reply.value) : signedValueOf(reply.value);
  }

  cases++;
  counts.set(spec.name, (counts.get(spec.name) || 0) + 1);

  if (actual !== expected) {
    divergences.push({ op: spec.name, args, config: cfg, expected, actual });
    console.log(`DIVERGENCE ${spec.name}(${args.join(', ')}) at precision ${cfg.precision} rounding ${cfg.rounding}`);
    console.log(`  decimal.js: ${expected}`);
    console.log(`  go port:    ${actual}`);
    if (divergences.length >= 20) break;
  }
}

// valueOf keeps the sign of negative zero, which the wire form carries in .s
// rather than in the string.
function signedValueOf(v) {
  if (v.d && v.d.length === 1 && v.d[0] === 0 && v.s === -1) return '-0';
  return v.str;
}

const elapsed = ((Date.now() - started) / 1000).toFixed(1);
console.log('');
console.log(`# ${cases} cases in ${elapsed}s, ${divergences.length} divergence(s)`);
for (const [name, n] of [...counts].sort()) console.log(`#   ${name.padEnd(10)} ${n}`);

child.stdin.end();
process.exit(divergences.length === 0 ? 0 : 1);
