// Generates expected results for the single-operand rounding methods, straight
// from decimal.js.
//
// Usage:
//
//   node tests/port/gen_unary_cases.mjs ../decimal.js/decimal.mjs abs > src/testdata/abs.txt
//
// Each output line is:
//   op<TAB>value<TAB>precision<TAB>rounding<TAB>arg<TAB>digits,...<TAB>exponent<TAB>sign
// The arg column is the decimal-place or significant-digit count for toDP and
// toSD, and -1 for the methods that take no argument.

const [modulePath, op] = process.argv.slice(2);
if (!modulePath || !op) {
  console.error('usage: node gen_unary_cases.mjs <path to decimal.mjs> <op>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

const values = [
  '0', '-0', '1', '-1', '0.5', '-0.5', '1.5', '-1.5', '2.5', '-2.5', '0.1', '-0.1',
  '9.99', '-9.99', '99.5', '-99.5', '9999999.9999999', '10000000', '0.0000001',
  '123456.7891011', '-123456.7891011', '1e21', '1e-21', '5e-324', '1e100',
  '0.000000000123456789', '99999999999999999999.5', '-99999999999999999999.5',
  '0.99999999999999999999', 'NaN', 'Infinity', '-Infinity',
];

const configs = [];
for (const precision of [1, 5, 20, 40]) {
  for (const rounding of [0, 1, 2, 3, 4, 5, 6, 7, 8]) configs.push({ precision, rounding });
}

// Methods taking no argument, and those taking a digit count.
const takesArg = op === 'toDP' || op === 'toSD';
const args = takesArg ? (op === 'toSD' ? [1, 2, 5, 10, 20] : [0, 1, 2, 5, 10]) : [-1];

const out = [];
for (const cfg of configs) {
  Decimal.set({ ...cfg, defaults: true, toExpNeg: -9e15, toExpPos: 9e15 });
  for (const v of values) {
    for (const arg of args) {
      const x = takesArg ? new Decimal(v)[op](arg) : new Decimal(v)[op]();
      const d = x.d === null ? '' : x.d.join(',');
      const e = Number.isNaN(x.e) ? 'NaN' : String(x.e);
      const s = Number.isNaN(x.s) ? 'NaN' : String(x.s);
      out.push([op, v, cfg.precision, cfg.rounding, arg, d, e, s].join('\t'));
    }
  }
}
console.log(out.join('\n'));
