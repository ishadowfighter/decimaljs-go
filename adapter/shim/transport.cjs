'use strict';

// Synchronous request/response transport to the decimald subprocess.
//
// The vendored test suite is synchronous from top to bottom — a module builds a
// value, asserts on it, and moves on, all inside one call stack — so the shim
// cannot await anything. Node offers exactly two ways to get a synchronous
// answer out of another process:
//
//   1. child_process.spawnSync: start the process, hand it stdin, get stdout
//      back when it exits.
//   2. A long-lived child plus a blocking read of its stdout.
//
// This transport uses (1). Option (2) is faster per call, but a blocking read
// of a child's pipe is not portable: on Windows a piped stdio stream is a named
// pipe with no file descriptor Node will hand to fs.readSync, and the usual
// workaround — a worker thread plus Atomics.wait — is a large amount of
// machinery to maintain in a harness. A process launch is a few milliseconds
// and the whole suite is a few tens of thousands of calls, which is slow but
// finishes, and correctness is what a differential harness is for.
//
// Nothing above the transport depends on the choice: decimald already speaks a
// streaming line protocol and holds no state between lines, so swapping in a
// persistent-process transport later is a change to this file alone.

const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

// Where the built binary lives. Override with DECIMALD_BIN when building
// somewhere else.
function binaryPath() {
  if (process.env.DECIMALD_BIN) return process.env.DECIMALD_BIN;
  const base = path.join(__dirname, '..', 'bin', 'decimald');
  return process.platform === 'win32' ? base + '.exe' : base;
}

let resolvedBinary = null;

function binary() {
  if (resolvedBinary) return resolvedBinary;
  const bin = binaryPath();
  if (!fs.existsSync(bin)) {
    throw new Error(
      'decimald binary not found at ' + bin + '\n' +
      'Build it first:  go build -o adapter/bin/decimald ./adapter/cmd/decimald\n' +
      '(or point DECIMALD_BIN at an existing build)'
    );
  }
  resolvedBinary = bin;
  return bin;
}

let nextId = 1;

// send writes one request and returns the parsed response object. It throws
// only for transport failures; a failed operation comes back as a response with
// ok: false, which the caller turns into whatever error its layer wants.
function send(request) {
  const id = nextId++;
  const line = JSON.stringify(Object.assign({ id: id }, request)) + '\n';

  const result = spawnSync(binary(), [], {
    input: line,
    encoding: 'utf8',
    maxBuffer: 256 * 1024 * 1024,
  });

  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(
      'decimald exited with status ' + result.status + ': ' + (result.stderr || '').trim()
    );
  }

  const reply = (result.stdout || '').split('\n').find(function (s) { return s.trim() !== ''; });
  if (!reply) {
    throw new Error('decimald produced no response for: ' + line.trim());
  }

  let parsed;
  try {
    parsed = JSON.parse(reply);
  } catch (e) {
    throw new Error('decimald produced unparseable output: ' + reply);
  }
  if (parsed.id !== id) {
    throw new Error('decimald replied to id ' + parsed.id + ', expected ' + id);
  }
  return parsed;
}

// JSON has no literal for these four numbers, so the protocol spells them as
// strings in both directions. See adapter/PROTOCOL.md.
function encodeNumber(n) {
  if (typeof n !== 'number') return n;
  if (Number.isNaN(n)) return 'NaN';
  if (n === Infinity) return 'Infinity';
  if (n === -Infinity) return '-Infinity';
  if (Object.is(n, -0)) return '-0';
  return n;
}

function decodeNumber(v) {
  if (typeof v !== 'string') return v;
  switch (v) {
    case 'NaN': return NaN;
    case 'Infinity': return Infinity;
    case '-Infinity': return -Infinity;
    case '-0': return -0;
    default: return v;
  }
}

module.exports = { send, encodeNumber, decodeNumber, binaryPath };
