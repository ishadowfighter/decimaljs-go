// Generates the expected results for the rounding-engine unit tests, straight
// from decimal.js, so the port is checked against the original's behaviour
// rather than against a reading of its source.
//
// Usage, with a decimal.js checkout available:
//
//   node tests/port/gen_rounding_cases.mjs ../decimal.js/decimal.mjs > src/testdata/rounding.txt
//
// Each output line is: value<TAB>sd<TAB>rm<TAB>digits,digits,...<TAB>exponent<TAB>sign
// toSignificantDigits is used because it is the thinnest wrapper around
// decimal.js's finalise, so the comparison isolates the rounding engine.

const modulePath = process.argv[2];
if (!modulePath) {
  console.error('usage: node gen_rounding_cases.mjs <path to decimal.mjs>');
  process.exit(2);
}

// Resolve against the working directory, not this file's directory, so the
// path on the command line means what it looks like it means.
const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

Decimal.set({ defaults: true, precision: 20, rounding: 4, toExpNeg: -9e15, toExpPos: 9e15 });

const values = [
  '0', '-0', '1', '-1', '5', '-5', '9', '4.5', '-4.5', '5.5', '-5.5', '2.5', '3.5',
  '0.5', '-0.5', '0.05', '9.5', '99.5', '999.5', '0.15', '0.25', '0.35',
  '1.005', '1.0049999', '9.9999999', '99999999.5', '123456.7891011',
  '9999999.9999999', '10000000', '9999999999999999', '0.000012345678901234567',
  '1e-7', '1e21', '5e-324', '1.7976931348623157e308',
  '0.1', '0.12', '0.123', '0.1234', '0.12345', '0.123456', '0.1234567', '0.12345678',
  '-0.1', '-0.12345678', '87654321.123456789', '1000000.0000001', '999999999999999999999.5',
  '0.000000000000000000005', '19.999999999999999999999', '-19.999999999999999999999',
];

const out = [];
for (const v of values) {
  for (let sd = 1; sd <= 12; sd++) {
    for (let rm = 0; rm <= 8; rm++) {
      const x = new Decimal(v).toSignificantDigits(sd, rm);
      const d = x.d === null ? '' : x.d.join(',');
      const e = Number.isNaN(x.e) ? 'NaN' : String(x.e);
      const s = Number.isNaN(x.s) ? 'NaN' : String(x.s);
      out.push([v, sd, rm, d, e, s].join('\t'));
    }
  }
}
console.log(out.join('\n'));
