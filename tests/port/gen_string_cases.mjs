// Generates expected output for the formatting methods, straight from
// decimal.js.
//
// Usage:
//
//   node tests/port/gen_string_cases.mjs ../decimal.js/decimal.mjs > src/testdata/strings.txt
//
// Each output line is:
//   op<TAB>value<TAB>precision<TAB>rounding<TAB>toExpNeg<TAB>toExpPos<TAB>arg<TAB>expected
// with an arg of -1 where the method was called without one.

const modulePath = process.argv[2];
if (!modulePath) {
  console.error('usage: node gen_string_cases.mjs <path to decimal.mjs>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

const values = [
  '0', '-0', '1', '-1', '0.5', '-0.5', '1.5', '-2.5', '0.1', '-0.1', '9.99',
  '1e-7', '-1e-7', '1e21', '-1e21', '1e-21', '123456.7891011', '-123456.7891011',
  '10000000', '9999999.9999999', '0.000000000123456789', '5e-324', '1e100',
  '99999999999999999999', '0.00001', '100', '1000000000000000000000',
  '0.0000001234567890123', '-0.999999999999999999995', 'NaN', 'Infinity', '-Infinity',
];

const configs = [
  { precision: 20, rounding: 4, toExpNeg: -7, toExpPos: 21 },
  { precision: 20, rounding: 4, toExpNeg: -9e15, toExpPos: 9e15 },
  { precision: 20, rounding: 4, toExpNeg: 0, toExpPos: 0 },
  { precision: 5, rounding: 0, toExpNeg: -7, toExpPos: 21 },
  { precision: 5, rounding: 3, toExpNeg: -7, toExpPos: 21 },
  { precision: 5, rounding: 6, toExpNeg: -2, toExpPos: 4 },
];

// The no-argument forms, then the forms taking a digit count.
const ops = [
  { name: 'toString', args: [-1] },
  { name: 'valueOf', args: [-1] },
  { name: 'toFixed', args: [-1, 0, 1, 2, 5, 10, 25] },
  { name: 'toExponential', args: [-1, 0, 1, 2, 5, 10, 25] },
  { name: 'toPrecision', args: [-1, 1, 2, 5, 10, 25] },
];

const out = [];
for (const cfg of configs) {
  Decimal.set({ ...cfg, defaults: true });
  for (const { name, args } of ops) {
    for (const v of values) {
      for (const arg of args) {
        const x = new Decimal(v);
        const result = arg < 0 ? x[name]() : x[name](arg);
        out.push([name, v, cfg.precision, cfg.rounding, cfg.toExpNeg, cfg.toExpPos, arg, result].join('\t'));
      }
    }
  }
}
console.log(out.join('\n'));
