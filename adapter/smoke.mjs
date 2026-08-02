// Smoke test for the marshalling boundary between Node and the Go port.
//
// This checks the wire protocol, not the arithmetic: that a value's internal
// state survives the trip to decimald and back unchanged, that the values JSON
// has no literal for (NaN, the infinities, negative zero) are not quietly
// flattened, and that a rejected input arrives as a DecimalError rather than as
// a silently wrong answer.
//
// The expected internals below were taken from the reference decimal.js and are
// its representation, not this port's — a case that fails here is a real
// disagreement, not a stale expectation to be updated.
//
// Run:  node adapter/smoke.mjs
// Exits non-zero if any case fails.

import { createRequire } from 'module';

const require = createRequire(import.meta.url);
const { send, encodeNumber, decodeNumber, binaryPath } = require('./shim/transport.cjs');

let passed = 0;
let failed = 0;

function report(name, ok, detail) {
  if (ok) {
    passed++;
    console.log('pass  ' + name);
  } else {
    failed++;
    console.log('FAIL  ' + name + (detail ? '  -- ' + detail : ''));
  }
}

function show(v) {
  return JSON.stringify(v, (_, x) => (typeof x === 'number' && !Number.isFinite(x) ? String(x) : x));
}

function same(a, b) {
  if (Array.isArray(a) || Array.isArray(b)) {
    if (!Array.isArray(a) || !Array.isArray(b) || a.length !== b.length) return false;
    return a.every((v, i) => same(v, b[i]));
  }
  // Object.is separates -0 from 0 and treats NaN as equal to itself, which is
  // exactly the discrimination this test exists to check.
  return Object.is(a, b);
}

// state issues one `new` and returns the decoded internals.
function state(input) {
  const reply = send({ op: 'new', args: [input] });
  if (!reply.ok) throw new Error(reply.error);
  const v = reply.value;
  return { d: v.d, e: decodeNumber(v.e), s: decodeNumber(v.s), str: v.str, wire: v };
}

function checkValue(name, input, expected) {
  let got;
  try {
    got = state(input);
  } catch (e) {
    report(name, false, 'threw: ' + e.message);
    return;
  }
  const problems = [];
  if (!same(got.d, expected.d)) problems.push('d ' + show(got.d) + ' != ' + show(expected.d));
  if (!same(got.e, expected.e)) problems.push('e ' + show(got.e) + ' != ' + show(expected.e));
  if (!same(got.s, expected.s)) problems.push('s ' + show(got.s) + ' != ' + show(expected.s));
  if (got.str !== expected.str) problems.push('str ' + show(got.str) + ' != ' + show(expected.str));
  report(name, problems.length === 0, problems.join('; '));
}

// checkRoundTrip feeds a value's own wire form back in and requires the state to
// come out identical. This is the property the shim depends on when it passes an
// already-built operand to a second operation.
function checkRoundTrip(name, input) {
  let first;
  try {
    first = state(input);
  } catch (e) {
    report('round-trip ' + name, false, 'threw: ' + e.message);
    return;
  }
  const reply = send({ op: 'echo', args: [first.wire] });
  if (!reply.ok) {
    report('round-trip ' + name, false, 'echo failed: ' + reply.error);
    return;
  }
  const back = reply.value;
  const ok =
    same(back.d, first.d) &&
    same(decodeNumber(back.e), first.e) &&
    same(decodeNumber(back.s), first.s) &&
    back.str === first.str;
  report('round-trip ' + name, ok, ok ? '' : show(back) + ' != ' + show(first.wire));
}

console.log('decimald: ' + binaryPath() + '\n');

// -- Non-finite values --------------------------------------------------------

checkValue('NaN', 'NaN', { d: null, e: NaN, s: NaN, str: 'NaN' });
checkValue('+Infinity', 'Infinity', { d: null, e: NaN, s: 1, str: 'Infinity' });
checkValue('-Infinity', '-Infinity', { d: null, e: NaN, s: -1, str: '-Infinity' });

// -- Signed zero --------------------------------------------------------------

checkValue('zero', '0', { d: [0], e: 0, s: 1, str: '0' });
// decimal.js keeps the sign of -0 in .s while printing it without one, so the
// string form alone cannot carry the value: this is the case a naive
// string-based protocol loses.
checkValue('negative zero', '-0', { d: [0], e: 0, s: -1, str: '0' });

// -- Exponent extremes --------------------------------------------------------

checkValue('1e21 (toExpPos boundary)', '1e21', { d: [1], e: 21, s: 1, str: '1e+21' });
checkValue('1e-7 (toExpNeg boundary)', '1e-7', { d: [1], e: -7, s: 1, str: '1e-7' });
checkValue('9e15 (maxE)', '9e15', { d: [90], e: 15, s: 1, str: '9000000000000000' });
checkValue('-9e15', '-9e15', { d: [90], e: 15, s: -1, str: '-9000000000000000' });
checkValue('1e-30', '1e-30', { d: [100000], e: -30, s: 1, str: '1e-30' });

// -- Limb packing -------------------------------------------------------------

checkValue('multi-limb', '123456.7891011', {
  d: [123456, 7891011],
  e: 5,
  s: 1,
  str: '123456.7891011',
});
checkValue('multi-limb with interior zeros', '9.999e15', {
  d: [99, 9900000],
  e: 15,
  s: 1,
  str: '9999000000000000',
});

// -- Round trips --------------------------------------------------------------

['NaN', 'Infinity', '-Infinity', '-0', '0', '1e21', '1e-7', '9e15', '123456.7891011'].forEach(
  (v) => checkRoundTrip(v, v)
);

// -- Failure signalling -------------------------------------------------------

{
  const reply = send({ op: 'new', args: ['hello'] });
  const ok = reply.ok === false && reply.kind === 'decimal' && /DecimalError/.test(reply.error);
  report('invalid input is a DecimalError', ok, show(reply));
}

{
  const reply = send({ op: 'plus', args: ['1', '2'] });
  const ok =
    reply.ok === false && reply.kind === 'unimplemented' && !/DecimalError/.test(reply.error);
  report('unimplemented op is not a DecimalError', ok, show(reply));
}

{
  // An out-of-range setting is decimal.js's other DecimalError.
  const reply = send({ op: 'new', args: ['1'], config: { precision: 0 } });
  const ok = reply.ok === false && reply.kind === 'decimal' && /DecimalError/.test(reply.error);
  report('bad config is a DecimalError', ok, show(reply));
}

// -- Number encoding ----------------------------------------------------------

{
  const cases = [
    [NaN, 'NaN'],
    [Infinity, 'Infinity'],
    [-Infinity, '-Infinity'],
    [-0, '-0'],
    [0, 0],
    [21, 21],
  ];
  const ok = cases.every(([n, wireForm]) =>
    same(encodeNumber(n), wireForm) && same(decodeNumber(wireForm), n)
  );
  report('number encoding survives both directions', ok);
}

{
  const reply = send({ op: 'sign', args: ['-0'] });
  const ok = reply.ok && Object.is(decodeNumber(reply.value), -0);
  report('Decimal.sign(-0) is -0', ok, show(reply));
}

{
  const reply = send({ op: 'cmp', args: ['1', 'NaN'] });
  const ok = reply.ok && Number.isNaN(decodeNumber(reply.value));
  report('comparing with NaN yields NaN', ok, show(reply));
}

console.log('\n' + passed + ' passed, ' + failed + ' failed');
process.exit(failed === 0 ? 0 : 1);
