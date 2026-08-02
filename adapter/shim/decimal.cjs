'use strict';

// A stand-in for decimal.js that delegates to the Go port through the decimald
// subprocess.
//
// The vendored test suite reaches straight into decimal.js's internals — it
// asserts on .d, .e and .s, mutates the constructor's settings mid-file, and
// builds independently configured constructors with Decimal.clone() — so this
// shim has to present the same shape, not merely the same answers. Instances
// therefore carry the real limb array, exponent and sign, in decimal.js's
// spelling: .d is null for a non-finite value, .e is NaN for one, and .s is NaN
// for NaN.
//
// Operations the Go port has not implemented yet are present as stubs that
// throw an error saying so. That is deliberate. The suite's assertException
// counts any thrown Error whose message matches /DecimalError/ as a pass, so a
// missing operation must never be reported with that wording — an unimplemented
// operation has to fail the test, not accidentally satisfy it.

const { send, encodeNumber, decodeNumber } = require('./transport.cjs');

const TAG = '[object Decimal]';

const DEFAULTS = {
  precision: 20,
  rounding: 4,
  modulo: 1,
  toExpNeg: -7,
  toExpPos: 21,
  minE: -9e15,
  maxE: 9e15,
  crypto: false,
};

const SETTINGS = Object.keys(DEFAULTS);

// Error raised for anything the port has not implemented. The wording is
// checked by adapter/run-parity.mjs and, importantly, contains no "DecimalError"
// substring.
function notImplemented(name) {
  return new Error('not implemented in port: ' + name);
}

// A failed operation from decimald becomes either a DecimalError, which the
// suite may legitimately assert on, or a plainly labelled harness failure.
function raise(reply) {
  if (reply.kind === 'decimal') {
    const err = new Error('[DecimalError] ' + reply.error.replace(/^DecimalError:\s*/, ''));
    err.name = 'DecimalError';
    throw err;
  }
  if (reply.kind === 'unimplemented') {
    throw new Error(reply.error);
  }
  throw new Error('harness failure: ' + reply.error);
}

// setting renders one proposed setting for the wire. NaN and the infinities
// become their string spellings so that decimald can reject them; JSON would
// otherwise write all three as null, which is a value decimal.js also rejects
// but for a different reason. Negative zero is left as zero, since a setting
// carries no sign distinction.
function setting(v) {
  if (typeof v !== 'number') return v;
  if (Number.isNaN(v)) return 'NaN';
  if (v === Infinity) return 'Infinity';
  if (v === -Infinity) return '-Infinity';
  return v === 0 ? 0 : v;
}

function isDecimalInstance(v) {
  return (v && v.toStringTag === TAG) || false;
}

// clone builds a Decimal constructor with its own settings, which is what
// decimal.js's Decimal.clone does and what several test modules rely on to run
// two configurations side by side.
function clone(settings) {
  function Decimal(value) {
    if (!(this instanceof Decimal)) return new Decimal(value);
    // decimal.js stamps the constructor on the instance, not the prototype,
    // because every constructor it clones shares one prototype object.
    this.constructor = Decimal;
    const state = call(Decimal, 'new', [operand(value)]);
    this.d = state.d;
    this.e = decodeNumber(state.e);
    this.s = decodeNumber(state.s);
  }

  const P = SHARED_PROTOTYPE;
  Decimal.prototype = P;
  P.toStringTag = TAG;

  // -- Settings ------------------------------------------------------------

  SETTINGS.forEach(function (key) {
    Decimal[key] = settings && key in settings ? settings[key] : DEFAULTS[key];
  });

  Decimal.set = Decimal.config = function (obj) {
    if (!obj || typeof obj !== 'object') {
      const err = new Error('[DecimalError] Object expected: ' + obj);
      err.name = 'DecimalError';
      throw err;
    }

    const next = {};
    SETTINGS.forEach(function (key) {
      next[key] = obj.defaults === true ? DEFAULTS[key] : Decimal[key];
    });
    SETTINGS.forEach(function (key) {
      // An explicitly undefined setting is skipped rather than applied, as
      // decimal.js skips it: `Decimal.config({precision: undefined})` leaves
      // precision alone instead of resetting it.
      if (key in obj && obj[key] !== undefined) next[key] = setting(obj[key]);
    });

    // decimald owns the range checks, so there is one implementation of them.
    // A rejected setting must leave the constructor untouched, as it does in
    // decimal.js, which validates every value before assigning any.
    const reply = send({ op: 'config', args: [], config: next });
    if (!reply.ok) raise(reply);

    // The settings are read back from decimald rather than copied from the
    // request, so normalisation happens in one place: `crypto: 0` has to end up
    // as false, the way decimal.js stores it.
    SETTINGS.forEach(function (key) { Decimal[key] = reply.value[key]; });
    return Decimal;
  };

  Decimal.clone = function (obj) {
    const inherited = {};
    SETTINGS.forEach(function (key) {
      inherited[key] = obj && obj.defaults === true ? DEFAULTS[key] : Decimal[key];
    });
    const child = clone(inherited);
    // A null argument reaches set and is rejected there, as it is upstream.
    return child.set(obj === undefined ? {} : obj);
  };

  // -- Implemented instance methods ---------------------------------------

  P.comparedTo = P.cmp = function (y) {
    return decodeNumber(call(this.constructor, 'cmp', [wire(this), operand(y)]));
  };

  P.equals = P.eq = predicate('eq');
  P.greaterThan = P.gt = predicate('gt');
  P.greaterThanOrEqualTo = P.gte = predicate('gte');
  P.lessThan = P.lt = predicate('lt');
  P.lessThanOrEqualTo = P.lte = predicate('lte');

  P.isFinite = query('isFinite');
  P.isInteger = P.isInt = query('isInteger');
  P.isNaN = query('isNaN');
  P.isNegative = P.isNeg = query('isNegative');
  P.isPositive = P.isPos = query('isPositive');
  P.isZero = query('isZero');

  // toString is re-evaluated rather than cached from construction: it depends
  // on toExpNeg and toExpPos, which the suite changes between building a value
  // and printing it.
  P.toString = function () {
    return call(this.constructor, 'str', [wire(this)]);
  };
  // valueOf is not toString: decimal.js keeps the sign of negative zero here
  // and drops it there, and the suite checks both.
  P.valueOf = P.toJSON = function () {
    return call(this.constructor, 'valueOf', [wire(this)]);
  };
  if (typeof Symbol !== 'undefined' && Symbol.for) {
    P[Symbol.for('nodejs.util.inspect.custom')] = P.valueOf;
  }

  // Arithmetic. Each call sends both operands and the current settings, so a
  // mid-file `Decimal.rounding = 3` takes effect on the very next call without
  // any state living in the subprocess.

  P.plus = P.add = arithmetic('add');
  P.minus = P.sub = arithmetic('sub');
  P.times = P.mul = arithmetic('mul');
  P.dividedBy = P.div = arithmetic('div');
  P.dividedToIntegerBy = P.divToInt = arithmetic('divToInt');
  P.modulo = P.mod = arithmetic('mod');
  P.toPower = P.pow = arithmetic('pow');

  P.squareRoot = P.sqrt = transform('sqrt');
  P.cubeRoot = P.cbrt = transform('cbrt');
  P.absoluteValue = P.abs = transform('abs');
  P.negated = P.neg = transform('neg');
  P.round = transform('round');
  P.floor = transform('floor');
  P.ceil = transform('ceil');
  P.truncated = P.trunc = transform('trunc');

  P.toDecimalPlaces = P.toDP = function (dp, rm) {
    // An absent digit count returns a copy, which is what decimal.js does
    // before it validates anything.
    if (dp === undefined) return new this.constructor(this);
    return build(this.constructor, 'toDP', [wire(this), dp, roundingArg(this.constructor, rm)]);
  };

  P.toSignificantDigits = P.toSD = function (sd, rm) {
    const Ctor = this.constructor;
    if (sd === undefined) return build(Ctor, 'toSD', [wire(this), Ctor.precision, Ctor.rounding]);
    return build(Ctor, 'toSD', [wire(this), sd, roundingArg(Ctor, rm)]);
  };

  P.toNearest = function (y, rm) {
    const Ctor = this.constructor;
    // With no argument the nearest multiple of one is wanted.
    if (y === null || y === undefined) {
      return build(Ctor, 'toNearest', [wire(this), '1', Ctor.rounding]);
    }
    return build(Ctor, 'toNearest', [wire(this), operand(y), roundingArg(Ctor, rm)]);
  };

  P.clampedTo = P.clamp = function (min, max) {
    return build(this.constructor, 'clamp', [wire(this), operand(min), operand(max)]);
  };

  P.decimalPlaces = P.dp = function () {
    return decodeNumber(call(this.constructor, 'dp', [wire(this)]));
  };

  P.precision = P.sd = function (z) {
    // decimal.js accepts only a boolean, 1 or 0 here, and throws otherwise.
    if (z !== undefined && z !== !!z && z !== 1 && z !== 0) {
      throw decimalError('Invalid argument: ' + z);
    }
    return decodeNumber(call(this.constructor, 'sd', [wire(this), !!z]));
  };

  // Formatting. A digit count of -1 stands for decimal.js's absent argument,
  // which selects a different code path rather than a default value.

  P.toFixed = formatter('toFixed');
  P.toExponential = formatter('toExponential');
  P.toPrecision = formatter('toPrecision');

  P.toNumber = function () { return Number(this.valueOf()); };

  P.toBinary = baseFormatter('toBinary');
  P.toOctal = baseFormatter('toOctal');
  P.toHexadecimal = P.toHex = baseFormatter('toHex');

  // -- Implemented statics -------------------------------------------------

  Decimal.isDecimal = isDecimalInstance;
  Decimal.default = Decimal.Decimal = Decimal;

  Decimal.sign = function (x) {
    return decodeNumber(call(Decimal, 'sign', [operand(x)]));
  };

  // The statics are the instance methods applied to converted arguments, which
  // is how decimal.js defines them too.
  [
    ['add', 'plus'], ['sub', 'minus'], ['mul', 'times'], ['div', 'div'],
    ['mod', 'mod'], ['pow', 'pow'],
  ].forEach(function (pair) {
    Decimal[pair[0]] = function (x, y) { return new Decimal(x)[pair[1]](y); };
  });

  ['abs', 'floor', 'ceil', 'round', 'trunc', 'sqrt', 'cbrt'].forEach(function (name) {
    Decimal[name] = function (x) { return new Decimal(x)[name](); };
  });

  Decimal.clamp = function (x, min, max) { return new Decimal(x).clamp(min, max); };

  Decimal.random = function (sd) {
    return fromState(Decimal, call(Decimal, 'random', [sd === undefined ? 'absent' : sd]));
  };

  ['max', 'min', 'sum'].forEach(function (name) {
    Decimal[name] = function () {
      const args = Array.prototype.slice.call(arguments).map(operand);
      if (args.length === 0) throw decimalError('Invalid argument: no arguments');
      return fromState(Decimal, call(Decimal, name, args));
    };
  });

  ROUNDING_MODES.forEach(function (name, i) { Decimal[name] = i; });
  Decimal.EUCLID = 9;

  // -- Everything the port has not reached yet ----------------------------

  UNIMPLEMENTED_METHODS.forEach(function (name) {
    P[name] = function () { throw notImplemented('Decimal.prototype.' + name); };
  });
  UNIMPLEMENTED_STATICS.forEach(function (name) {
    Decimal[name] = function () { throw notImplemented('Decimal.' + name); };
  });

  return Decimal;

  // -- Helpers -------------------------------------------------------------

  function predicate(op) {
    return function (y) { return call(this.constructor, op, [wire(this), operand(y)]); };
  }

  function query(op) {
    return function () { return call(this.constructor, op, [wire(this)]); };
  }

  // arithmetic builds a two-operand method returning a new Decimal.
  function arithmetic(op) {
    return function (y) { return build(this.constructor, op, [wire(this), operand(y)]); };
  }

  // transform builds a one-operand method returning a new Decimal.
  function transform(op) {
    return function () { return build(this.constructor, op, [wire(this)]); };
  }

  // baseFormatter builds one of the three base-conversion methods, which take
  // the same optional digit count and rounding mode as the decimal ones.
  function baseFormatter(op) {
    return function (sd, rm) {
      const Ctor = this.constructor;
      if (sd === undefined) return call(Ctor, op, [wire(this), 'absent', Ctor.rounding]);
      return call(Ctor, op, [wire(this), sd, roundingArg(Ctor, rm)]);
    };
  }

  // formatter builds one of the three methods taking an optional digit count.
  function formatter(op) {
    return function (n, rm) {
      const Ctor = this.constructor;
      if (n === undefined) return call(Ctor, op, [wire(this), 'absent', Ctor.rounding]);
      return call(Ctor, op, [wire(this), n, roundingArg(Ctor, rm)]);
    };
  }
}

// build issues an operation that returns a Decimal and wraps the reply back up
// as an instance of Ctor.
function build(Ctor, op, args) {
  return fromState(Ctor, call(Ctor, op, args));
}

// fromState turns a wire decimal into an instance without going back through
// the constructor, which would re-parse it and lose the sign of -0.
function fromState(Ctor, state) {
  const x = Object.create(Ctor.prototype);
  x.constructor = Ctor;
  x.d = state.d;
  x.e = decodeNumber(state.e);
  x.s = decodeNumber(state.s);
  return x;
}

// roundingArg resolves decimal.js's optional rounding-mode argument. An absent
// one means the constructor's current setting; anything else is passed through
// so the Go side applies the same range check decimal.js does.
function roundingArg(Ctor, rm) {
  return rm === undefined ? Ctor.rounding : rm;
}

// decimalError builds the error decimal.js throws, which the suite recognises
// by the DecimalError substring alone.
function decimalError(message) {
  const err = new Error('[DecimalError] ' + message);
  err.name = 'DecimalError';
  return err;
}

// call issues one operation under Ctor's current settings and returns the value,
// converting a failure into a thrown error.
function call(Ctor, op, args) {
  const config = {};
  SETTINGS.forEach(function (key) { config[key] = Ctor[key]; });
  const reply = send({ op: op, args: args, config: config });
  if (!reply.ok) raise(reply);
  return reply.value;
}

// wire renders an existing instance as the protocol's decimal object, so that a
// value already built on the Go side round-trips without being re-parsed from
// its string form — which would lose the sign of -0.
function wire(x) {
  return { d: x.d, e: encodeNumber(x.e), s: encodeNumber(x.s), str: '' };
}

// operand renders a constructor argument. Instances go over as decimal objects;
// everything else goes over as a string, because JSON.stringify writes -0 as 0
// and cannot write NaN or the infinities at all.
function operand(v) {
  if (isDecimalInstance(v)) return wire(v);
  if (typeof v === 'number') {
    if (Number.isNaN(v)) return 'NaN';
    if (v === Infinity) return 'Infinity';
    if (v === -Infinity) return '-Infinity';
    if (Object.is(v, -0)) return '-0';
    return String(v);
  }
  if (typeof v === 'string') return v;
  if (typeof v === 'bigint') return v.toString();
  if (v === undefined) return 'undefined';
  if (v === null) return 'null';
  return String(v);
}

const ROUNDING_MODES = [
  'ROUND_UP',
  'ROUND_DOWN',
  'ROUND_CEIL',
  'ROUND_FLOOR',
  'ROUND_HALF_UP',
  'ROUND_HALF_DOWN',
  'ROUND_HALF_EVEN',
  'ROUND_HALF_CEIL',
  'ROUND_HALF_FLOOR',
];

// decimal.js's full prototype surface minus the part the port has reached. Every
// alias is listed separately because the suite calls both spellings.
const UNIMPLEMENTED_METHODS = [
  'cosine', 'cos',
  'hyperbolicCosine', 'cosh',
  'hyperbolicSine', 'sinh',
  'hyperbolicTangent', 'tanh',
  'inverseCosine', 'acos',
  'inverseHyperbolicCosine', 'acosh',
  'inverseHyperbolicSine', 'asinh',
  'inverseHyperbolicTangent', 'atanh',
  'inverseSine', 'asin',
  'inverseTangent', 'atan',
  'logarithm', 'log',
  'naturalExponential', 'exp',
  'naturalLogarithm', 'ln',
  'sine', 'sin',
  'tangent', 'tan',
  'toFraction',
];

// decimal.js's static surface minus config, set, clone, sign and isDecimal.
const UNIMPLEMENTED_STATICS = [
  'acos', 'acosh', 'asin', 'asinh', 'atan', 'atanh', 'atan2',
  'cos', 'cosh', 'exp', 'hypot',
  'ln', 'log', 'log2', 'log10',
  'sin', 'sinh', 'tan', 'tanh',
];

// One prototype object is shared by every cloned constructor, as in decimal.js,
// where `Decimal.prototype === Decimal.clone().prototype` is asserted directly.
const SHARED_PROTOTYPE = {};

module.exports = clone(DEFAULTS);
