package decimal

import (
	"errors"
	"fmt"
)

// RoundingMode selects how a value is rounded to a digit limit. The numeric
// values are decimal.js's, and the original test suite passes them as bare
// integers, so they must not be renumbered.
type RoundingMode int

const (
	RoundUp        RoundingMode = 0 // away from zero
	RoundDown      RoundingMode = 1 // towards zero
	RoundCeil      RoundingMode = 2 // towards +Infinity
	RoundFloor     RoundingMode = 3 // towards -Infinity
	RoundHalfUp    RoundingMode = 4 // nearest; ties away from zero
	RoundHalfDown  RoundingMode = 5 // nearest; ties towards zero
	RoundHalfEven  RoundingMode = 6 // nearest; ties to even
	RoundHalfCeil  RoundingMode = 7 // nearest; ties towards +Infinity
	RoundHalfFloor RoundingMode = 8 // nearest; ties towards -Infinity
)

// ModuloMode selects how the quotient is rounded when computing a modulus; the
// remainder is then a - n*q. Values match decimal.js.
type ModuloMode int

const (
	ModRoundUp   ModuloMode = 0 // remainder takes the opposite sign to the dividend
	ModTruncate  ModuloMode = 1 // remainder takes the sign of the dividend (JavaScript %)
	ModFloor     ModuloMode = 3 // remainder takes the sign of the divisor (Python %)
	ModHalfEven  ModuloMode = 6 // the IEEE 754 remainder
	ModEuclidean ModuloMode = 9 // always non-negative
)

// Config is the set of settings that govern arithmetic and formatting. It
// corresponds to decimal.js's constructor properties, settable there with
// Decimal.set.
type Config struct {
	// Precision is the maximum number of significant digits in the result of
	// a calculation, in 1..maxDigits.
	Precision int

	// Rounding is the mode used when rounding to Precision, in 0..8.
	Rounding RoundingMode

	// Modulo is the mode used by Mod, in 0..9.
	Modulo ModuloMode

	// ToExpNeg is the exponent at and below which String uses exponential
	// notation, in -expLimit..0.
	ToExpNeg int

	// ToExpPos is the exponent at and above which String uses exponential
	// notation, in 0..expLimit.
	ToExpPos int

	// MinE is the exponent below which a value underflows to zero, in
	// -expLimit..-1.
	MinE int

	// MaxE is the exponent above which a value overflows to Infinity, in
	// 1..expLimit.
	MaxE int

	// Crypto requests cryptographically secure randomness from Random.
	Crypto bool
}

// DefaultConfig returns decimal.js's initial configuration.
func DefaultConfig() Config {
	return Config{
		Precision: 20,
		Rounding:  RoundHalfUp,
		Modulo:    ModTruncate,
		ToExpNeg:  -7,
		ToExpPos:  21,
		MinE:      -expLimit,
		MaxE:      expLimit,
		Crypto:    false,
	}
}

// A Context carries a Config and constructs and operates on Decimals under it.
// It is the port's equivalent of a decimal.js constructor: Decimal.clone()
// produces an independently configurable constructor, and a Context is that
// same thing without the global mutable state.
type Context struct {
	config Config
}

// NewContext returns a Context using cfg.
func NewContext(cfg Config) *Context { return &Context{config: cfg} }

// Config returns the Context's settings.
func (c *Context) Config() Config { return c.config }

// SetConfig replaces the Context's settings, reporting an error if any value is
// out of range. decimal.js throws in that case; here it is an error return.
func (c *Context) SetConfig(cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	c.config = cfg
	return nil
}

// Err is the error kind returned where decimal.js throws a DecimalError. The
// original suite only asserts that a DecimalError was raised, not its type, so
// callers can match with errors.Is.
var Err = errors.New("DecimalError")

// ErrInvalidArgument reports an argument decimal.js rejects: out of range, not
// an integer where one is required, or an unparseable value.
var ErrInvalidArgument = fmt.Errorf("%w: invalid argument", Err)

// ErrPrecisionLimitExceeded reports a request for more digits than the
// precomputed constants can supply.
var ErrPrecisionLimitExceeded = fmt.Errorf("%w: precision limit exceeded", Err)

// ErrCryptoUnavailable reports Crypto being set without a usable source of
// secure randomness.
var ErrCryptoUnavailable = fmt.Errorf("%w: crypto unavailable", Err)

// validateConfig checks each setting against the range decimal.js enforces in
// Decimal.set. Ranges are inclusive and mirror the original exactly, including
// its asymmetric exponent bounds.
func validateConfig(cfg Config) error {
	if cfg.Precision < 1 || cfg.Precision > maxDigits {
		return fmt.Errorf("%w: precision: %d", ErrInvalidArgument, cfg.Precision)
	}
	if cfg.Rounding < RoundUp || cfg.Rounding > RoundHalfFloor {
		return fmt.Errorf("%w: rounding: %d", ErrInvalidArgument, cfg.Rounding)
	}
	if cfg.Modulo < 0 || cfg.Modulo > 9 {
		return fmt.Errorf("%w: modulo: %d", ErrInvalidArgument, cfg.Modulo)
	}
	if cfg.ToExpNeg < -expLimit || cfg.ToExpNeg > 0 {
		return fmt.Errorf("%w: toExpNeg: %d", ErrInvalidArgument, cfg.ToExpNeg)
	}
	if cfg.ToExpPos < 0 || cfg.ToExpPos > expLimit {
		return fmt.Errorf("%w: toExpPos: %d", ErrInvalidArgument, cfg.ToExpPos)
	}
	if cfg.MinE < -expLimit || cfg.MinE > 0 {
		return fmt.Errorf("%w: minE: %d", ErrInvalidArgument, cfg.MinE)
	}
	if cfg.MaxE < 0 || cfg.MaxE > expLimit {
		return fmt.Errorf("%w: maxE: %d", ErrInvalidArgument, cfg.MaxE)
	}
	return nil
}

// wrapInvalidArgument builds the error the port returns where decimal.js
// throws `[DecimalError] Invalid argument`.
func wrapInvalidArgument(what string, value int) error {
	return fmt.Errorf("%w: %s: %d", ErrInvalidArgument, what, value)
}
