package decimal

import "fmt"

// DecimalPlaces returns the number of digits after the decimal point. ok is
// false for a non-finite value, where decimal.js returns NaN.
func (d Decimal) DecimalPlaces() (dp int, ok bool) {
	if d.coefficient == nil {
		return 0, false
	}

	last := len(d.coefficient) - 1
	n := (last - floorDiv(d.exponent, logBase)) * logBase

	// Discount the trailing zeros of the last limb, which are storage padding
	// rather than digits of the value.
	if w := d.coefficient[last]; w != 0 {
		for ; w%10 == 0; w /= 10 {
			n--
		}
	}
	if n < 0 {
		n = 0
	}
	return n, true
}

// SignificantDigits returns the number of significant digits. When
// includeZeros is set, the trailing zeros of an integer count too, so 1000 has
// four significant digits rather than one — decimal.js's `precision(true)`.
// ok is false for a non-finite value.
func (d Decimal) SignificantDigits(includeZeros bool) (sd int, ok bool) {
	if d.coefficient == nil {
		return 0, false
	}
	k := getPrecision(d.coefficient)
	if includeZeros && d.exponent+1 > k {
		k = d.exponent + 1
	}
	return k, true
}

// getPrecision counts the significant digits of a coefficient.
func getPrecision(digits []int) int {
	last := len(digits) - 1
	length := last*logBase + 1

	if w := digits[last]; w != 0 {
		for ; w%10 == 0; w /= 10 {
			length--
		}
		for w = digits[0]; w >= 10; w /= 10 {
			length++
		}
	}
	return length
}

// ToNearest returns d rounded to the nearest multiple of y, using the default
// context's rounding mode.
func (d Decimal) ToNearest(y Decimal) (Decimal, error) {
	return defaultContext.ToNearest(d, y, defaultContext.config.Rounding)
}

// ToNearest returns x rounded to the nearest multiple of y, with ties resolved
// by rm.
func (c *Context) ToNearest(x, y Decimal, rm RoundingMode) (Decimal, error) {
	if rm < RoundUp || rm > RoundHalfFloor {
		return Decimal{}, wrapInvalidArgument("rounding mode", int(rm))
	}
	cfg := c.config

	// A non-finite x is returned unchanged, unless y is NaN.
	if x.coefficient == nil {
		if y.sign == signNaN {
			return y, nil
		}
		return x, nil
	}

	// A non-finite y gives an infinity with the sign of x, or NaN.
	if y.coefficient == nil {
		if y.sign == signNaN {
			return y, nil
		}
		return Inf(x.sign), nil
	}

	// A zero y gives zero with the sign of x.
	if y.coefficient[0] == 0 {
		return Decimal{coefficient: []int{0}, exponent: 0, sign: x.sign}, nil
	}

	// The multiple is computed without rounding or exponent limits, which are
	// then applied once to the result.
	q := divide(x, y, 0, true, rm, true, 0, cfg, false)
	return finaliseNoRound(mul(q, y, cfg, false), cfg), nil
}

// Clamp returns d limited to the range [min, max].
func (d Decimal) Clamp(minimum, maximum Decimal) (Decimal, error) {
	return defaultContext.Clamp(d, minimum, maximum)
}

// Clamp returns x limited to the range [min, max]. It is NaN if either bound is
// NaN, and an error if the bounds are the wrong way round, which decimal.js
// throws for.
func (c *Context) Clamp(x, minimum, maximum Decimal) (Decimal, error) {
	if minimum.sign == signNaN || maximum.sign == signNaN {
		return NaN(), nil
	}
	if minimum.Gt(maximum) {
		return Decimal{}, fmt.Errorf("%w: clamp bounds out of order: %s > %s",
			ErrInvalidArgument, c.ValueOf(minimum), c.ValueOf(maximum))
	}
	if order, ok := x.Cmp(minimum); ok && order < 0 {
		return minimum, nil
	}
	if order, ok := x.Cmp(maximum); ok && order > 0 {
		return maximum, nil
	}
	return x, nil
}

// Max returns the largest of its arguments, or NaN if any of them is NaN.
func Max(values ...Decimal) Decimal { return maxOrMin(values, -1) }

// Min returns the smallest of its arguments, or NaN if any of them is NaN.
func Min(values ...Decimal) Decimal { return maxOrMin(values, 1) }

// maxOrMin selects the extreme value, with n of -1 for a maximum and 1 for a
// minimum. Ties are broken on the sign, so Max(0, -0) is 0 and Min(0, -0) is -0,
// as they are in decimal.js.
func maxOrMin(values []Decimal, n int) Decimal {
	if len(values) == 0 {
		return NaN()
	}
	x := values[0]
	for _, y := range values[1:] {
		if y.sign == signNaN {
			return y
		}
		k, ok := x.Cmp(y)
		if !ok {
			return x
		}
		if k == n || k == 0 && x.sign == n {
			x = y
		}
	}
	return x
}

// Sum returns the sum of its arguments, rounded once at the end rather than
// after each addition, using the default context.
func Sum(values ...Decimal) Decimal { return defaultContext.Sum(values...) }

// Sum returns the sum of its arguments under the Context's settings. Only the
// final result is rounded, which is what makes it more accurate than a chain of
// Add calls.
func (c *Context) Sum(values ...Decimal) Decimal {
	if len(values) == 0 {
		return NaN()
	}
	cfg := c.config
	x := values[0]
	for _, y := range values[1:] {
		if x.sign == signNaN {
			break
		}
		x = add(x, y, cfg, false)
	}
	return finalise(x, cfg.Precision, cfg.Rounding, false, cfg, true)
}
