# adapter — running the original decimal.js test suite against the Go port

`tests/original/` is a byte-for-byte copy of decimal.js's own test suite. It is
the port's specification, and it is never edited. This directory contains the
machinery that lets those files, unmodified, exercise Go code.

```
tests/original/test.js
  └─ requires tests/original/modules/*.js
       └─ each requires ../setup.js
            └─ line 24:  Decimal = require('../decimal')
                 └─ tests/decimal.js          ← the redirect
                      └─ adapter/shim/decimal.cjs   ← decimal.js-shaped JS object
                           └─ adapter/shim/transport.cjs
                                └─ adapter/bin/decimald    ← Go, one JSON line per call
                                     └─ src/  (the port)
```

## The redirect

`tests/original/setup.js` line 24 is

```js
Decimal = require('../decimal');
```

which resolves relative to `tests/original/`, i.e. to `tests/decimal`. It is the
suite's single hook point, and it sits in a file that must not change.

**Chosen mechanism: a file at `tests/decimal.js` that re-exports the shim.**
That path is outside `tests/original/`, so nothing sacred is touched, and Node's
ordinary resolution does the rest. The file is three lines; the implementation
lives in `adapter/`.

The alternative considered was a `--require` preload that patches
`Module._resolveFilename` or `Module._load` to intercept the specifier. It works,
but it costs more than it buys:

- every entry point has to remember the flag, and a forgotten flag fails in a
  confusing way (`Cannot find module '../decimal'`) rather than an obvious one;
- it hides the redirect from anyone reading the tree, where a file named
  `tests/decimal.js` is self-explanatory;
- it needs its own escape hatch for tooling that spawns Node without the flag.

Since a plain file achieves the same thing with zero edits to `tests/original/`
and zero launch ceremony, that is what is used. If the suite ever gains a real
`tests/decimal.js` of its own from upstream, the preload approach is the fallback.

## The transport

`adapter/shim/transport.cjs` sends one request per call with
`child_process.spawnSync`.

The suite is synchronous throughout — a module builds a value, asserts on it and
moves on, all in one call stack — so the shim cannot await anything. Node offers
two ways to get a synchronous answer from another process: `spawnSync`, or a
long-lived child plus a blocking read of its stdout. The second is faster per
call but not portable: on Windows a piped stdio stream is a named pipe with no
file descriptor `fs.readSync` will take, and the usual workaround — a worker
thread plus `Atomics.wait` — is a lot of machinery for a test harness to carry.

`spawnSync` costs a few milliseconds per call, which is slow but finishes, and a
differential harness exists to be correct rather than quick. Nothing above the
transport depends on the choice: `decimald` already speaks a streaming line
protocol and keeps no state between lines, so a persistent-process transport can
be dropped in later by editing that one file.

## Unimplemented operations fail; they never pass

`T.assertException` in `tests/original/setup.js` counts *any* thrown `Error`
whose message matches `/DecimalError/` as a pass. An operation the port has not
written yet must therefore never be reported with that wording, or a missing
feature would read as a green test.

So the two are kept apart at every layer:

- `decimald` tags a failure `decimal`, `unimplemented` or `protocol`;
- the shim throws `[DecimalError] …` only for the first, and
  `not implemented in port: …` for the second;
- `adapter/run-parity.mjs` marks a module that stopped early as `error` and
  reports the module as broken rather than as a score.

The shim declares stubs for decimal.js's whole public surface, so a call to an
unported method raises that clear error instead of `undefined is not a function`.

## Layout

| path | what it is |
| ---- | ---------- |
| `cmd/decimald/main.go` | The line protocol and the operation dispatch table. |
| `cmd/decimald/value.go` | Marshalling: internal state to and from JSON, and the settings. |
| `cmd/decimald/format.go` | A transliteration of decimal.js's `toString`. Temporary — see below. |
| `shim/decimal.cjs` | The decimal.js-shaped JavaScript object the suite talks to. |
| `shim/transport.cjs` | `spawnSync` request/response, and the number encoding. |
| `smoke.mjs` | Tests the marshalling boundary alone, against the Go binary. |
| `run-parity.mjs` | Runs named test modules and prints a passed/total table. |
| `parity-runner.cjs` | Runs one module in a process of its own; used by the above. |
| `PROTOCOL.md` | The wire protocol in full. |
| `../tests/decimal.js` | The redirect. Three lines, no logic. |

## Running it

Build the binary first. Everything else looks for it at `adapter/bin/decimald`
(`.exe` on Windows), or at `$DECIMALD_BIN`.

```sh
go build -o adapter/bin/decimald ./adapter/cmd/decimald
```

Check the boundary:

```sh
node adapter/smoke.mjs
```

This asserts that NaN, both infinities, negative zero, the exponent extremes and
a multi-limb coefficient all survive the round trip with their internal state
intact, and that an invalid input arrives as a `DecimalError` while an unported
operation does not. It prints one `pass`/`FAIL` line per case and exits non-zero
on any failure. It is the thing to run after touching anything in `adapter/`.

Run test modules:

```sh
node adapter/run-parity.mjs cmp config toString   # by name
node adapter/run-parity.mjs --verbose cmp         # with the suite's own output
node adapter/run-parity.mjs --all                 # everything; expect mass failure
```

Nothing is written to disk. The table is a snapshot of a port in progress, not an
artefact worth keeping.

`adapter/bin/` holds build output. On Windows the `*.exe` rule in `.gitignore`
already covers it; a `/adapter/bin/` line would cover it everywhere.

## Two things to know before extending this

**`format.go` is a stopgap.** The port exposes no `String` method yet, but every
response carries a string form, so decimal.js's `digitsToString` and
`finiteToString` are transliterated in that file. Two implementations of the same
formatting rules is exactly the drift a differential harness should be catching
rather than creating. Delete the file and call the port's own method as soon as
one exists.

**The shim routes number and string construction the same way.** decimal.js has
slightly different paths for `new Decimal(1)` and `new Decimal('1')`; the shim
stringifies the number and uses the string path for both. Nothing in the suite
has distinguished them so far, but it is a known simplification, not an
established equivalence.
