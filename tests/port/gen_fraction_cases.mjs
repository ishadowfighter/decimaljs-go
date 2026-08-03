// Generates expected results for toFraction, straight from decimal.js.
//
//   node tests/port/gen_fraction_cases.mjs ../decimal.js/decimal.mjs > src/testdata/fraction.txt
//
// Each line is: value<TAB>maxDenominator<TAB>precision<TAB>numerator/denominator
// with a max denominator of "none" where the method was called without one.

const modulePath = process.argv[2];
if (!modulePath) {
  console.error('usage: node gen_fraction_cases.mjs <path to decimal.mjs>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

const values = [
  '0', '-0', '1', '-1', '0.5', '-0.5', '0.1', '3.14159265358979', '-3.14159265358979',
  '2.5', '1e21', '1e-7', '123456.7891011', '0.333333333333', '1.0000001',
  '99999999999999999999', '7', '0.875', 'NaN', 'Infinity', '-Infinity',
];
const limits = ['none', '1', '2', '10', '100', '1000', '1e9', '3'];

const out = [];
for (const precision of [5, 20, 40]) {
  Decimal.set({ defaults: true, precision, rounding: 4, toExpNeg: -9e15, toExpPos: 9e15 });
  for (const v of values) {
    for (const lim of limits) {
      let r;
      try {
        const f = lim === 'none' ? new Decimal(v).toFraction() : new Decimal(v).toFraction(lim);
        // A non-finite value comes back as a single Decimal rather than a pair.
        r = Array.isArray(f)
          ? f[0].valueOf() + '/' + f[1].valueOf()
          : f.valueOf() + '/' + f.valueOf();
      } catch (e) {
        r = 'THROWS';
      }
      out.push([v, lim, precision, r].join('\t'));
    }
  }
}
console.log(out.join('\n'));
