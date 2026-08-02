// Runs vendored test modules against the Go port through the shim and prints a
// passed/total table.
//
// Usage:
//   node adapter/run-parity.mjs cmp isFiniteEtc
//   node adapter/run-parity.mjs tests/original/modules/cmp.js
//   node adapter/run-parity.mjs --all           # every module; expect mass failure
//   node adapter/run-parity.mjs --verbose cmp   # also show each module's output
//
// A module is named either by its bare name or by its path. Nothing is written
// to disk: the table is a snapshot of a port in progress, not an artefact worth
// keeping.
//
// Exit status is 0 when every requested module passed every assertion, 1
// otherwise, so this can be wired into a check once the port is far enough along
// for that to mean something.

import { spawnSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const adapterDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.join(adapterDir, '..');
const modulesDir = path.join(repoRoot, 'tests', 'original', 'modules');
const runner = path.join(adapterDir, 'parity-runner.cjs');

const argv = process.argv.slice(2);
const verbose = argv.includes('--verbose');
const all = argv.includes('--all');
const names = argv.filter((a) => !a.startsWith('--'));

function listAll() {
  return fs
    .readdirSync(modulesDir)
    .filter((f) => f.endsWith('.js'))
    .sort();
}

function resolveModule(name) {
  const candidates = name.includes('/') || name.includes('\\') || name.endsWith('.js')
    ? [path.resolve(name), path.join(modulesDir, path.basename(name))]
    : [path.join(modulesDir, name + '.js')];
  const found = candidates.find((c) => fs.existsSync(c));
  if (!found) {
    throw new Error(
      'no such test module: ' + name + '\nAvailable: ' + listAll().map((f) => f.replace(/\.js$/, '')).join(', ')
    );
  }
  return found;
}

let files;
if (all) {
  files = listAll().map((f) => path.join(modulesDir, f));
} else if (names.length > 0) {
  files = names.map(resolveModule);
} else {
  console.error('usage: node adapter/run-parity.mjs [--all] [--verbose] <module>...');
  console.error('available: ' + listAll().map((f) => f.replace(/\.js$/, '')).join(', '));
  process.exit(2);
}

function run(file) {
  const proc = spawnSync(process.execPath, [runner, file], {
    encoding: 'utf8',
    cwd: repoRoot,
    maxBuffer: 256 * 1024 * 1024,
  });
  const stdout = proc.stdout || '';
  if (verbose) {
    process.stdout.write(stdout.replace(/\n##PARITY##.*\n?/, ''));
    if (proc.stderr) process.stderr.write(proc.stderr);
  }

  const marker = stdout.match(/##PARITY## (.*)/);
  if (!marker) {
    return {
      file: path.basename(file),
      passed: 0,
      total: 0,
      status: 'crashed',
      message: (proc.stderr || '').trim().split('\n')[0] || 'no summary produced',
    };
  }
  return JSON.parse(marker[1]);
}

const rows = files.map(run);

const width = Math.max(6, ...rows.map((r) => r.file.length));
const pad = (s, n) => String(s) + ' '.repeat(Math.max(0, n - String(s).length));
const padLeft = (s, n) => ' '.repeat(Math.max(0, n - String(s).length)) + String(s);

console.log('');
console.log(pad('module', width) + '  ' + padLeft('passed', 8) + '  ' + padLeft('total', 7) + '  note');
console.log('-'.repeat(width) + '  ' + '-'.repeat(8) + '  ' + '-'.repeat(7) + '  ----');

let passed = 0;
let total = 0;
let broken = 0;

for (const r of rows) {
  passed += r.passed;
  total += r.total;
  let note = '';
  if (r.status !== 'ok') {
    broken++;
    // A module that throws stops at the first unimplemented operation, so its
    // total understates how much is left to do. Say so rather than let the
    // number read as progress.
    note = r.status + ': ' + truncate(r.message, 70);
  } else if (r.passed !== r.total) {
    note = r.total - r.passed + ' failed';
  }
  console.log(pad(r.file, width) + '  ' + padLeft(r.passed, 8) + '  ' + padLeft(r.total, 7) + '  ' + note);
}

console.log('-'.repeat(width) + '  ' + '-'.repeat(8) + '  ' + '-'.repeat(7) + '  ----');
console.log(
  pad('total', width) + '  ' + padLeft(passed, 8) + '  ' + padLeft(total, 7) + '  ' +
  (broken > 0 ? broken + ' module(s) stopped early' : '')
);
console.log('');

function truncate(s, n) {
  s = String(s).replace(/\s+/g, ' ').trim();
  return s.length > n ? s.slice(0, n - 1) + '…' : s;
}

process.exit(broken === 0 && passed === total && total > 0 ? 0 : 1);
