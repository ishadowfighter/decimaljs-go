package decimal

import (
	"fmt"
	"math"
	"strconv"
)

// ErrNotImplemented reports an operation this port does not yet cover. It is
// returned rather than guessed at, so a gap can never be mistaken for a result.
var ErrNotImplemented = fmt.Errorf("%w: not implemented in this port", Err)

// one returns the Decimal 1.
func one() Decimal {
	return Decimal{coefficient: []int{1}, exponent: 0, sign: 1}
}

// Float64 returns d as a float64, by way of the string decimal.js's number
// conversion would produce. Values beyond float64's range become infinities and
// values below it become zero, as they do in JavaScript.
func (d Decimal) Float64() float64 {
	if d.IsNaN() {
		return math.NaN()
	}
	if d.IsInf() {
		return math.Inf(d.sign)
	}
	f, err := strconv.ParseFloat(defaultContext.ValueOf(d), 64)
	if err != nil {
		// ParseFloat only reports a range error here, and it still returns the
		// saturated value, which is what JavaScript's conversion gives.
		return f
	}
	return f
}

// jsPow is JavaScript's Math.pow, which differs from Go's math.Pow in two
// cases that the original test suite checks directly. ECMAScript returns NaN
// whenever the exponent is NaN, and whenever the base has magnitude 1 and the
// exponent is infinite; IEEE 754, which Go follows, returns 1 for both.
func jsPow(x, y float64) float64 {
	if math.IsNaN(y) {
		return math.NaN()
	}
	if math.IsInf(y, 0) && math.Abs(x) == 1 {
		return math.NaN()
	}
	return math.Pow(x, y)
}

// truncateLimbs shortens limbs to at most n, reporting whether anything was
// dropped.
func truncateLimbs(limbs []int, n int) ([]int, bool) {
	if len(limbs) > n {
		return limbs[:n], true
	}
	return limbs, false
}

// intPow raises x to a non-negative integer power by exponentiation by
// squaring, keeping intermediate values to a working length with guard digits.
//
// It is decimal.js's intPow, including the detail that makes truncation safe:
// if anything was dropped and the last remaining limb is zero, it is
// incremented, so the value stays strictly greater than the truncated digits
// and the final rounding cannot fall the wrong way.
func intPow(x Decimal, n int64, pr int, cfg Config) Decimal {
	r := one()

	// The working length leaves between 28 and 34 guard digits.
	k := int(math.Ceil(float64(pr)/logBase + 4))

	truncated := false
	for {
		if n%2 != 0 {
			r = mul(r, x, cfg, false)
			var dropped bool
			r.coefficient, dropped = truncateLimbs(r.coefficient, k)
			if dropped {
				truncated = true
			}
		}

		n /= 2
		if n == 0 {
			last := len(r.coefficient) - 1
			if truncated && r.coefficient[last] == 0 {
				r.coefficient[last]++
			}
			break
		}

		x = mul(x, x, cfg, false)
		x.coefficient, _ = truncateLimbs(x.coefficient, k)
	}

	return r
}

// Pow returns d raised to the power y, for an integer y.
func (d Decimal) Pow(y Decimal) (Decimal, error) { return defaultContext.Pow(d, y) }

// Pow returns x raised to the power y.
//
// Only integer exponents are supported: they take decimal.js's exact
// exponentiation-by-squaring path. A fractional exponent needs exp and ln,
// which are not ported yet, and returns ErrNotImplemented rather than an
// approximation.
func (c *Context) Pow(x, y Decimal) (Decimal, error) {
	cfg := c.config

	// Non-finite or zero operands follow JavaScript's Math.pow on the values
	// converted to numbers, which is what decimal.js does here.
	if x.coefficient == nil || y.coefficient == nil || x.coefficient[0] == 0 || y.coefficient[0] == 0 {
		return c.NewFromFloat(jsPow(x.Float64(), y.Float64())), nil
	}

	if x.Eq(one()) {
		return x, nil
	}
	if y.Eq(one()) {
		return finalise(x, cfg.Precision, cfg.Rounding, false, cfg, true), nil
	}

	// y is an integer when its limbs run out at or before the units limb.
	e := floorDiv(y.exponent, logBase)
	if e < len(y.coefficient)-1 {
		return Decimal{}, fmt.Errorf("%w: fractional exponent %s", ErrNotImplemented, c.ValueOf(y))
	}

	yn := y.Float64()
	k := yn
	if k < 0 {
		k = -k
	}
	if k > maxSafeInteger {
		return Decimal{}, fmt.Errorf("%w: exponent %s exceeds the exact integer range", ErrNotImplemented, c.ValueOf(y))
	}

	r := intPow(x, int64(k), cfg.Precision, cfg)
	if y.sign < 0 {
		return divide(one(), r, 0, false, cfg.Rounding, false, 0, cfg, true), nil
	}
	return finalise(r, cfg.Precision, cfg.Rounding, false, cfg, true), nil
}
