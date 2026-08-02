// Generates expected results for the binary arithmetic unit tests, straight
// from decimal.js.
//
// Usage:
//
//   node tests/port/gen_binary_cases.mjs ../decimal.js/decimal.mjs plus > src/testdata/plus.txt
//
// Each output line is:
//   op<TAB>a<TAB>b<TAB>precision<TAB>rounding<TAB>digits,...<TAB>exponent<TAB>sign
// with an empty digits field for a non-finite result, and NaN in the exponent
// and sign fields where decimal.js stores NaN there.

const [modulePath, op] = process.argv.slice(2);
if (!modulePath || !op) {
  console.error('usage: node gen_binary_cases.mjs <path to decimal.mjs> <op>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

// Values chosen to exercise the paths the algorithms actually branch on:
// signs, zeros, non-finite values, equal and wildly different exponents,
// operands that cancel exactly, and carries and borrows across limb
// boundaries.
const values = [
  '0', '-0', '1', '-1', '2', '10', '-10', '0.1', '-0.1', '0.5', '1.5', '-1.5',
  '9999999', '10000000', '9999999.9999999', '0.0000001', '1e-7', '1e7', '1e21', '1e-21',
  '123456.7891011', '-123456.7891011', '987654321.123456789', '0.000000000123456789',
  '99999999999999999999', '100000000000000000000', '1e100', '1e-100', '3', '7',
  '0.3333333333333333333333', '2.5', '-2.5', 'NaN', 'Infinity', '-Infinity',
];

const configs = [
  { precision: 20, rounding: 4 },
  { precision: 1, rounding: 4 },
  { precision: 5, rounding: 0 },
  { precision: 5, rounding: 1 },
  { precision: 5, rounding: 2 },
  { precision: 5, rounding: 3 },
  { precision: 5, rounding: 5 },
  { precision: 5, rounding: 6 },
  { precision: 5, rounding: 7 },
  { precision: 5, rounding: 8 },
  { precision: 40, rounding: 4 },
  { precision: 200, rounding: 4 },
];

const out = [];
for (const cfg of configs) {
  Decimal.set({ ...cfg, defaults: true, toExpNeg: -9e15, toExpPos: 9e15 });
  for (const a of values) {
    for (const b of values) {
      let x;
      try {
        x = new Decimal(a)[op](b);
      } catch (e) {
        continue;
      }
      const d = x.d === null ? '' : x.d.join(',');
      const e = Number.isNaN(x.e) ? 'NaN' : String(x.e);
      const s = Number.isNaN(x.s) ? 'NaN' : String(x.s);
      out.push([op, a, b, cfg.precision, cfg.rounding, d, e, s].join('\t'));
    }
  }
}
console.log(out.join('\n'));
