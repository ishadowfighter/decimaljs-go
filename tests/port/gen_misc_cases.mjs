// Generates expected results for the remaining single- and multi-operand
// methods, straight from decimal.js.
//
// Usage:
//
//   node tests/port/gen_misc_cases.mjs ../decimal.js/decimal.mjs > src/testdata/misc.txt
//
// Each output line is:
//   op<TAB>precision<TAB>rounding<TAB>arg1<TAB>arg2<TAB>arg3<TAB>expected
// Unused argument columns are empty. `expected` is a valueOf string for the
// methods returning a Decimal, a number for dp and sd, and NaN where the
// original returns NaN.

const modulePath = process.argv[2];
if (!modulePath) {
  console.error('usage: node gen_misc_cases.mjs <path to decimal.mjs>');
  process.exit(2);
}

const { pathToFileURL } = await import('node:url');
const { resolve } = await import('node:path');
const { Decimal } = await import(pathToFileURL(resolve(modulePath)).href);

const values = [
  '0', '-0', '1', '-1', '0.5', '-0.5', '1.5', '-2.5', '10', '100', '1000',
  '0.1', '9.99', '1e21', '1e-21', '123456.7891011', '-123456.7891011',
  '9999999.9999999', '0.000000000123456789', '1e100', '0.00001',
  'NaN', 'Infinity', '-Infinity',
];

const configs = [
  { precision: 20, rounding: 4 },
  { precision: 5, rounding: 0 },
  { precision: 5, rounding: 3 },
  { precision: 40, rounding: 6 },
];

const out = [];
const row = (op, cfg, a1, a2, a3, expected) =>
  out.push([op, cfg.precision, cfg.rounding, a1 ?? '', a2 ?? '', a3 ?? '', String(expected)].join('\t'));

for (const cfg of configs) {
  Decimal.set({ ...cfg, defaults: true, toExpNeg: -9e15, toExpPos: 9e15 });

  for (const v of values) {
    const x = new Decimal(v);
    row('dp', cfg, v, null, null, x.dp());
    row('sd', cfg, v, '0', null, x.sd());
    row('sd', cfg, v, '1', null, x.sd(true));

    for (const y of ['1', '0.5', '0', '-0', '10', '0.001', 'NaN', 'Infinity', '-Infinity']) {
      for (const rm of [0, 4, 6]) {
        row('toNearest', cfg, v, y, String(rm), x.toNearest(y, rm).valueOf());
      }
    }

    for (const [lo, hi] of [['0', '1'], ['-1', '1'], ['1', '1'], ['-Infinity', 'Infinity'], ['NaN', '1'], ['1', 'NaN']]) {
      row('clamp', cfg, v, lo, hi, x.clamp(lo, hi).valueOf());
    }
  }

  const groups = [
    ['1', '2', '3'], ['-1', '-2', '-3'], ['0', '-0'], ['-0', '0'],
    ['NaN', '1'], ['1', 'NaN'], ['Infinity', '1'], ['-Infinity', '1'],
    ['1e21', '1e-21', '1'], ['0.1', '0.2', '0.3'], ['9999999.9999999', '0.0000001'],
  ];
  for (const g of groups) {
    row('max', cfg, g.join(' '), null, null, Decimal.max(...g).valueOf());
    row('min', cfg, g.join(' '), null, null, Decimal.min(...g).valueOf());
    row('sum', cfg, g.join(' '), null, null, Decimal.sum(...g).valueOf());
  }
}
console.log(out.join('\n'));
