'use strict';

// Runs one vendored test module in a process of its own and prints a machine
// readable summary line.
//
// A separate process per module is not just tidiness: the modules mutate global
// constructor settings and leave them mutated, and several set the globals `T`
// and `Decimal` on first require. Sharing one process would make a module's
// result depend on which modules ran before it.
//
// Usage:  node adapter/parity-runner.cjs <path to tests/original/modules/x.js>

const path = require('path');

const file = process.argv[2];
if (!file) {
  console.error('usage: node adapter/parity-runner.cjs <module.js>');
  process.exit(2);
}

let status = 'ok';
let message = '';

// powSqrt.js loops on a free variable named `total`, which no version of the
// vendored setup.js defines: `for (...; total < 10000; )` throws a
// ReferenceError before a single assertion runs, on this port and on a clean
// upstream checkout alike. Upstream's own test.js does not list the module,
// so nothing there notices.
//
// The missing piece is a harness global, not a defect in the test, and the
// vendored file cannot be edited. So the runner supplies it: `total` reads the
// live assertion count, which the module uses purely as a loop bound. Every
// assertion it makes still runs for real.
let assertions = 0;
Object.defineProperty(globalThis, 'total', {
  configurable: true,
  get: () => assertions,
});

require(path.resolve(__dirname, '..', 'tests', 'original', 'setup.js'));

// Count assertions as they happen. The wrappers only tally; the originals do
// the deciding.
for (const name of ['assert', 'assertEqual', 'assertEqualDecimal', 'assertEqualProps', 'assertException']) {
  const original = T[name];
  T[name] = function () {
    assertions++;
    return original.apply(this, arguments);
  };
}

try {
  require(path.resolve(file));
} catch (e) {
  status = 'error';
  message = (e && e.message) || String(e);
}

// T.result is [passed, total, ms], set by tests/original/setup.js when the
// module's T(...) call returns. It is absent if the module threw before then.
const result = (typeof T !== 'undefined' && T.result) || [0, 0, 0];

process.stdout.write(
  '\n##PARITY## ' +
    JSON.stringify({
      file: path.basename(file),
      passed: result[0],
      total: result[1],
      ms: result[2],
      status: status,
      message: message,
    }) +
    '\n'
);
