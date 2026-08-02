# decimaljs-go

A Go port of [**MikeMcl/decimal.js**](https://github.com/MikeMcl/decimal.js) — an arbitrary-precision Decimal type for JavaScript.

Built for **Port Mortem 2026** (Code Resurrection Wave 2, Hackathon Raptors) · Track F: JavaScript → Go.

> **Status: in progress.** This port is being built during the hackathon window. Nothing here is finished yet — the sections below describe what the project *is* and how it *will be* built and verified. Pass-rate numbers, scope, and the final API are filled in as the work lands, not before.

---

## What this is

`decimaljs-go` aims to be a standalone, idiomatic Go package that reproduces decimal.js's arbitrary-precision decimal arithmetic: the same results, the same rounding behavior, and the same edge cases (`NaN`, `±0`, `±Infinity`, configurable precision and rounding modes).

The goal is not "it compiles." The goal is a port whose behavior is *provably* the same as the original — verified by running decimal.js's own test suite, unmodified, against this Go build.

It is **not** a wrapper around a JavaScript engine and does **not** link the source-language runtime. All arithmetic is implemented in Go. The original JavaScript tests are used only as a proof harness — they call into the Go build through a thin, test-only adapter.

- **Ported from:** decimal.js — source commit recorded in `.port-mortem.toml` at kickoff
- **Original license:** MIT · **This port's license:** MIT (see `LICENSE`)
- **Source language:** JavaScript · **Target:** Go

## Why port it

decimal.js is a widely used precision-arithmetic library with no direct Go equivalent. `shopspring/decimal`, the common Go choice, is fixed-point with a different configuration model (decimal places vs. significant-digit precision) — a different design, not the same behavior. A faithful Go port brings decimal.js's significant-digit precision model and rounding semantics to Go as a single dependency-free package, and lets equivalence be *proven* rather than asserted.

---

## Project structure

Following the Port Mortem recommended layout.

```
decimaljs-go/
├── README.md            migration rationale, build instructions (this file)
├── DECISIONS.md         every non-trivial divergence from the original + why
├── Dockerfile           one command to a runnable artifact
├── .port-mortem.toml    track letter, source repo URL, kickoff hash
│
├── src/                 idiomatic Go port (the product)
│   ├── decimal.go         Decimal value type: sign, digit array, exponent
│   ├── config.go          precision / rounding / exponent-threshold config
│   ├── parse.go           construction & parsing (string, number, Decimal)
│   ├── arith.go           plus, minus, times, div, mod, abs, neg
│   ├── round.go           the rounding engine (all rounding modes)
│   ├── compare.go         cmp, eq, gt/gte/lt/lte, predicates
│   ├── format.go          toString, toFixed, toExponential, toPrecision, ...
│   ├── pow.go             integer & (scoped) fractional powers
│   └── transcendental.go  sqrt/cbrt, ln/exp/log, trig, hyperbolic (scoped)
│
├── tests/
│   ├── original/        decimal.js's tests — hashed at kickoff, kept unmodified
│   └── port/            new Go-side tests added during the port (optional)
│
├── adapter/             test-only glue: bridges the original JS tests → Go build
│                        (not part of the product; excluded from code-quality scope)
│
├── fuzz/
│   ├── harness.*        differential fuzz harness (decimal.js vs port vs mpmath)
│   └── log.txt          captured run + any divergences found
│
└── bench/
    ├── methodology.md   how performance was measured
    └── results.json     benchmark output (performance is a tiebreaker only)
```

---

## Build

> One command produces a runnable artifact.

```bash
docker build -t decimaljs-go .
# or, locally:
go build ./...
```

Prerequisites for the proof harness (not for using the library): Node and Python with `hypothesis` and `mpmath`. Exact versions are pinned in the `Dockerfile` once the harness lands.

## Usage

The public API will be Go-shaped (idiomatic Go, not a transliteration of the JS API) while matching decimal.js's behavior. A usage example will be added here once the core is implemented and passing its tests — the intent is that every example in this section actually runs.

---

## Proving equivalence

This is the core of the project. The original decimal.js tests live under `tests/original/`, unmodified, with their kickoff hashes recorded. They are executed against this Go build through the adapter — not one byte of the original test files is changed.

Planned commands (added as the harness is built):

```bash
make test-parity     # runs the original JS suite against the Go build
make fuzz            # differential fuzz: port vs decimal.js vs mpmath ground truth
```

The parity run will emit a per-file pass/fail table and an overall count. The fuzz run captures its output — a divergence between the port and *both* decimal.js and mpmath is a bug in this port; agreement with mpmath where decimal.js disagrees is a candidate upstream bug, which would be documented in `DECISIONS.md`.

---

## Scope & honesty

This port follows the organizers' guidance that a well-scoped, fully-proven subset beats an unstable full port.

- **In scope:** to be listed here once the core lands (construction/parsing, arithmetic, rounding modes, comparison, formatting, integer powers, and whichever transcendental functions reach parity).
- **Scoped out:** any functions that do not reach parity within the window will be listed here explicitly, with the reason in `DECISIONS.md`.

**Pass rate:** to be reported here as `X / Y` original assertions passing, measured through the adapter, once measured. Where this port does not match decimal.js, it will be listed; where decimal.js's own suite does not pass fully on a clean upstream clone, that baseline will be noted too. No number here is inflated — a disclosed gap is reported as a gap.

---

## Credits

- Original library: [decimal.js](https://github.com/MikeMcl/decimal.js) by Michael Mclaughlin (MIT).
- Ground-truth reference for fuzzing: [mpmath](https://mpmath.org/).
- Built for [Port Mortem / Code Resurrection](https://coderesurrection.com), a Hackathon Raptors event.

See `DECISIONS.md` for the engineering rationale behind each divergence.