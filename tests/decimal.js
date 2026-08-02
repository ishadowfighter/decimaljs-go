'use strict';

// Redirect for the vendored test suite.
//
// tests/original/setup.js line 24 does `Decimal = require('../decimal')`, which
// resolves relative to tests/original/ and therefore lands on this file. That
// line is inside the vendored suite, which is kept byte for byte identical to
// upstream, so the redirect has to happen at the path it already asks for
// rather than by editing it.
//
// The whole implementation lives in adapter/; this file is only the name the
// suite reaches for. See adapter/README.md.

module.exports = require('../adapter/shim/decimal.cjs');
