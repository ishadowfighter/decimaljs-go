// Generates expected results for integer exponentiation, straight from
// decimal.js.
//
// Usage:
//
//   node tests/port/gen_pow_cases.mjs ../decimal.js/decimal.mjs > src/testdata/pow.txt
//
// Each output line is: base<TAB>exponent<TAB>precision<TAB>rounding<TAB>expected
// where expected is the valueOf string of the result.

const modulePath = process.argv[2];
if (!modulePath) {
  console.error('usage: node gen_pow_cases.mjs <path to decimal.mjs>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

const bases = [
  '2', '-2', '10', '-10', '0.5', '-0.5', '1.5', '3', '7', '0.1', '1.0000001',
  '9999999', '123456.789', '-123456.789', '0.000001', '1', '-1',
  '0', '-0', 'NaN', 'Infinity', '-Infinity',
];
const exponents = [
  '0', '-0', '1', '-1', '2', '-2', '3', '-3', '10', '-10', '31', '53', '100', '-100',
  '1000', 'NaN', 'Infinity', '-Infinity',
];

const configs = [
  { precision: 20, rounding: 4 },
  { precision: 40, rounding: 4 },
  { precision: 5, rounding: 0 },
  { precision: 5, rounding: 3 },
  { precision: 5, rounding: 6 },
  { precision: 100, rounding: 4 },
];

const out = [];
for (const cfg of configs) {
  Decimal.set({ ...cfg, defaults: true, toExpNeg: -9e15, toExpPos: 9e15 });
  for (const b of bases) {
    for (const exp of exponents) {
      out.push([b, exp, cfg.precision, cfg.rounding, new Decimal(b).pow(exp).valueOf()].join('\t'));
    }
  }
}
console.log(out.join('\n'));
