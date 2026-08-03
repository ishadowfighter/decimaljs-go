// Thins the generated expectation files down to a representative sample.
//
//   node tests/port/sample_testdata.mjs [--max 500]
//
// The generators produce exhaustive sweeps — every value against every
// precision and rounding mode — which runs to 215000 lines. That is the right
// thing to check against while porting and the wrong thing to carry in a
// repository: it is this port's own output, not an upstream fixture.
//
// Sampling takes every Nth line rather than the first N. Each file is ordered
// by configuration block, so a prefix would keep one precision and drop the
// rest, while a stride keeps every configuration, every rounding mode and the
// whole spread of values.
//
// The full sweep is one command away whenever it is wanted:
//
//   node tests/port/gen_binary_cases.mjs ./decimal.js/decimal.mjs mod > src/testdata/mod.txt
//
// and the tests read whatever is in the file, so they simply get stronger.

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const here = path.dirname(fileURLToPath(import.meta.url));
const dir = path.join(here, '..', '..', 'src', 'testdata');

const argv = process.argv.slice(2);
const maxIndex = argv.indexOf('--max');
const MAX = maxIndex === -1 ? 500 : Number(argv[maxIndex + 1]);

let before = 0;
let after = 0;

for (const name of fs.readdirSync(dir).filter((f) => f.endsWith('.txt')).sort()) {
  const file = path.join(dir, name);
  const lines = fs.readFileSync(file, 'utf8').split('\n').filter((l) => l !== '');
  before += lines.length;

  if (lines.length <= MAX) {
    after += lines.length;
    console.log(`${name.padEnd(16)} ${String(lines.length).padStart(6)} kept whole`);
    continue;
  }

  const stride = Math.ceil(lines.length / MAX);
  const kept = lines.filter((_, i) => i % stride === 0);

  // The last line of the sweep is the last configuration's last case; keeping
  // it means the sample always spans the whole file.
  if (kept[kept.length - 1] !== lines[lines.length - 1]) kept.push(lines[lines.length - 1]);

  fs.writeFileSync(file, kept.join('\n') + '\n');
  after += kept.length;
  console.log(`${name.padEnd(16)} ${String(lines.length).padStart(6)} -> ${String(kept.length).padStart(5)} (every ${stride})`);
}

console.log(`\ntotal ${before} -> ${after} lines`);
