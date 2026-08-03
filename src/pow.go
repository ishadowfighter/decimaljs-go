package decimal

import (
	"math"
	"strconv"
)

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

// Pow returns d raised to the power y.
func (d Decimal) Pow(y Decimal) (Decimal, error) { return defaultContext.Pow(d, y) }

// Pow returns x raised to the power y.
//
// An integer exponent takes decimal.js's exact path, exponentiation by
// squaring. Everything else is exp(y*ln(x)) with guard digits, re-run at higher
// precision when the result lands on a rounding boundary.
func (c *Context) Pow(x, y Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding

	if x.coefficient == nil || y.coefficient == nil || x.coefficient[0] == 0 || y.coefficient[0] == 0 {
		return c.NewFromFloat(jsPow(x.Float64(), y.Float64())), nil
	}

	if x.Eq(one()) {
		return x, nil
	}
	if y.Eq(one()) {
		return finalise(x, pr, rm, false, cfg, true), nil
	}

	// y is an integer when its limbs run out at or before the units limb.
	e := floorDiv(y.exponent, logBase)
	yn := y.Float64()
	if e >= len(y.coefficient)-1 && math.Abs(yn) <= maxSafeInteger {
		k := math.Abs(yn)
		r := intPow(x, int64(k), pr, cfg)
		if y.sign < 0 {
			return divide(one(), r, 0, false, rm, false, 0, cfg, true), nil
		}
		return finalise(r, pr, rm, false, cfg, true), nil
	}

	sign := x.sign
	if sign < 0 {
		// A negative base needs an integer exponent; the result is positive
		// when that integer is even.
		if e < len(y.coefficient)-1 {
			return NaN(), nil
		}
		// Reading past the end gives undefined in the original, and
		// `undefined & 1` is zero, so an absent limb counts as even.
		if limbAt(y.coefficient, e)&1 == 0 {
			sign = 1
		}
		if x.exponent == 0 && x.coefficient[0] == 1 && len(x.coefficient) == 1 {
			// x is -1.
			x.sign = sign
			return x, nil
		}
	}

	// Estimate the result exponent from x^y = 10^(y*log10(x)). The estimate is
	// kept in a float64 until it has been range-checked: for a large exponent
	// it runs far past what an int can hold, and converting first would wrap.
	var eFloat float64
	k := jsPow(x.Float64(), yn)
	if k == 0 || math.IsInf(k, 0) {
		mantissa, _ := strconv.ParseFloat("0."+digitsToString(x.coefficient), 64)
		eFloat = math.Floor(yn * (math.Log(mantissa)/math.Ln10 + float64(x.exponent) + 1))
	} else {
		eFloat = float64(c.NewFromFloat(k).exponent)
	}

	if eFloat > float64(cfg.MaxE)+1 || eFloat < float64(cfg.MinE)-1 {
		if eFloat > 0 {
			return Inf(sign), nil
		}
		return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}, nil
	}
	e = int(eFloat)

	// The working configuration rounds down: the guard digits, not the mode,
	// decide the last digit until the very end.
	wcfg := cfg
	wcfg.Rounding = RoundHalfUp
	base := abs(x)

	// Extra guard digits for the logarithm, from the size of the exponent.
	guard := len(itoa(e))
	if guard > 12 {
		guard = 12
	}

	compute := func(workPr, lnPr int) (Decimal, error) {
		wcfg.Precision = workPr
		ln, err := naturalLogarithm(base, lnPr, true, wcfg)
		if err != nil {
			return Decimal{}, err
		}
		return naturalExponential(mul(y, ln, wcfg, false), workPr, true, wcfg), nil
	}

	r, err := compute(pr, pr+guard)
	if err != nil {
		return Decimal{}, err
	}

	if r.coefficient != nil {
		r = finalise(r, pr+5, RoundDown, false, wcfg, false)

		if checkRoundingDigits(r.coefficient, pr, rm, -1) {
			workPr := pr + 10
			r, err = compute(workPr, workPr+guard)
			if err != nil {
				return Decimal{}, err
			}
			r = finalise(r, workPr+5, RoundDown, false, wcfg, false)

			// Fourteen nines from the second rounding digit mean the result is
			// exact and rounds up.
			if digitsAllNine(sliceDigits(digitsToString(r.coefficient), pr+1, pr+15)) {
				r = finalise(r, pr+1, RoundUp, false, wcfg, false)
			}
		}
	}

	r.sign = sign
	return finalise(r, pr, rm, false, cfg, true), nil
}
