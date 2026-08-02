package decimal

import "math"

// getLn10 returns ln(10) rounded down to sd digits, and fails if sd exceeds the
// digits the constant holds.
func getLn10(sd int, cfg Config) (Decimal, error) {
	if sd > ln10Precision {
		return Decimal{}, ErrPrecisionLimitExceeded
	}
	x, _ := NewContext(cfg).Parse(ln10Digits)
	return finalise(x, sd, RoundDown, true, cfg, false), nil
}

// checkRoundingDigits reports whether the digits around position i sit on a
// rounding boundary, where the sum so far cannot be rounded safely and the
// series has to be recomputed at a higher precision.
//
// repeating is -1 for decimal.js's absent argument, which selects a different
// pattern of digits to look for.
func checkRoundingDigits(d []int, i int, rm RoundingMode, repeating int) bool {
	for k := d[0]; k >= 10; k /= 10 {
		i--
	}

	var di int
	if i--; i < 0 {
		i += logBase
		di = 0
	} else {
		di = ceilDiv(i+1, logBase)
		i %= logBase
	}

	k := pow10(logBase - i)
	rd := limbAt(d, di) % k

	// d[di+1] past the end is undefined, and the original truncates it with
	// `| 0`, which turns NaN into zero rather than failing the comparison.
	next := limbAt(d, di+1)

	if repeating < 0 {
		if i < 3 {
			switch i {
			case 0:
				rd /= 100
			case 1:
				rd /= 10
			}
			return rm < 4 && rd == 99999 || rm > 3 && rd == 49999 || rd == 50000 || rd == 0
		}
		return (rm < 4 && rd+1 == k || rm > 3 && rd+1 == k/2) &&
			next/k/100 == pow10(i-2)-1 ||
			(rd == k/2 || rd == 0) && next/k/100 == 0
	}

	if i < 4 {
		switch i {
		case 0:
			rd /= 1000
		case 1:
			rd /= 100
		case 2:
			rd /= 10
		}
		return (repeating > 0 || rm < 4) && rd == 9999 || repeating == 0 && rm > 3 && rd == 4999
	}
	return ((repeating > 0 || rm < 4) && rd+1 == k || repeating == 0 && rm > 3 && rd+1 == k/2) &&
		next/k/1000 == pow10(i-3)-1
}

// firstDigits returns the first n significant digits of x, for the convergence
// test the series use.
func firstDigits(x Decimal, n int) string {
	return truncateTo(digitsToString(x.coefficient), n)
}

// intDecimal builds a small integer Decimal, used for series denominators.
func intDecimal(n int) Decimal { return NewFromInt(int64(n)) }

// naturalExponential is decimal.js's naturalExponential: e^x by Taylor series
// after halving the argument enough times that it converges quickly, then
// squaring the result back up.
//
// sdSet corresponds to the original's `sd != null`, which means the caller
// wants a raw result at a stated precision rather than a rounded one.
func naturalExponential(x Decimal, sd int, sdSet bool, cfg Config) Decimal {
	pr, rm := cfg.Precision, cfg.Rounding

	if x.coefficient == nil || x.coefficient[0] == 0 || x.exponent > 17 {
		switch {
		case x.coefficient != nil && x.coefficient[0] == 0:
			return one()
		case x.coefficient != nil && x.sign < 0:
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}
		case x.coefficient != nil:
			return Inf(1)
		case x.sign == signNaN:
			return NaN()
		case x.sign < 0:
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}
		default:
			return x
		}
	}

	wpr := pr
	if sdSet {
		wpr = sd
	}

	// Halve the argument by fives until |x| < 0.1, so the series converges in
	// few terms; the result is squared back up afterwards.
	k := 0
	wcfg := cfg
	oneOver32, _ := NewContext(wcfg).Parse("0.03125")
	for x.exponent > -2 {
		x = mul(x, oneOver32, wcfg, false)
		k += 5
	}

	// Empirically derived guard digits: 2*log10(2^k) + 5.
	guard := int(math.Log(math.Pow(2, float64(k)))/math.Ln10*2 + 5)
	wpr += guard
	wcfg.Precision = wpr

	denominator, pow, sum := one(), one(), one()
	i := 0
	rep := 0

	for {
		pow = finalise(mul(pow, x, wcfg, false), wpr, RoundDown, false, wcfg, false)
		i++
		denominator = mul(denominator, intDecimal(i), wcfg, false)
		t := add(sum, divide(pow, denominator, wpr, true, RoundDown, false, 0, wcfg, false), wcfg, false)

		if firstDigits(t, wpr) == firstDigits(sum, wpr) {
			for j := k; j > 0; j-- {
				sum = finalise(mul(sum, sum, wcfg, false), wpr, RoundDown, false, wcfg, false)
			}

			if !sdSet {
				if rep < 3 && checkRoundingDigits(sum.coefficient, wpr-guard, rm, rep) {
					wpr += 10
					wcfg.Precision = wpr
					denominator, pow, t = one(), one(), one()
					i = 0
					rep++
				} else {
					// The original passes its `external = true` assignment as
					// the isTruncated argument here, so the series result is
					// always rounded as if digits were discarded.
					return finalise(sum, pr, rm, true, cfg, true)
				}
			} else {
				return sum
			}
		}

		sum = t
	}
}

// naturalLogarithm is decimal.js's naturalLogarithm: ln(x) by the series for
// ln((1+z)/(1-z)) after reducing the argument towards 1 and splitting off the
// power of ten.
func naturalLogarithm(y Decimal, sd int, sdSet bool, cfg Config) (Decimal, error) {
	const guard = 10
	pr, rm := cfg.Precision, cfg.Rounding
	x := y
	xd := x.coefficient
	n := 1

	if x.sign < 0 || xd == nil || xd[0] == 0 || x.exponent == 0 && xd[0] == 1 && len(xd) == 1 {
		switch {
		case xd != nil && xd[0] == 0:
			return Inf(-1), nil
		case x.sign != 1:
			return NaN(), nil
		case xd != nil:
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}, nil
		default:
			return x, nil
		}
	}

	wpr := pr
	if sdSet {
		wpr = sd
	}
	wpr += guard
	wcfg := cfg
	wcfg.Precision = wpr

	c := digitsToString(xd)
	c0 := c[0]
	e := x.exponent

	if math.Abs(float64(e)) >= 1.5e15 {
		// The reduction below would overflow for an exponent this large, so
		// ln(x*10^e) = ln(x) + e*ln(10) is applied directly instead.
		ln10, err := getLn10(wpr+2, wcfg)
		if err != nil {
			return Decimal{}, err
		}
		t := mul(ln10, NewFromInt(int64(e)), wcfg, false)
		mantissa, _ := NewContext(wcfg).Parse(string(c0) + "." + c[1:])
		lnm, err := naturalLogarithm(mantissa, wpr-guard, true, wcfg)
		if err != nil {
			return Decimal{}, err
		}
		result := add(lnm, t, wcfg, false)
		if !sdSet {
			return finalise(result, pr, rm, true, cfg, true), nil
		}
		return result, nil
	}

	// Argument reduction: multiply x by itself until its leading digits are in
	// [0.7, 1.3], recording the count so the series result can be divided by
	// it, then separate the power of ten.
	for c0 < '7' && c0 != '1' || c0 == '1' && len(c) > 1 && c[1] > '3' {
		x = mul(x, y, wcfg, false)
		c = digitsToString(x.coefficient)
		c0 = c[0]
		n++
	}

	e = x.exponent
	ctx := NewContext(wcfg)
	if c0 > '1' {
		x, _ = ctx.Parse("0." + c)
		e++
	} else {
		x, _ = ctx.Parse(string(c0) + "." + c[1:])
	}

	x1 := x

	// ln(y) = 2(z + z^3/3 + z^5/5 + ...) with z = (y-1)/(y+1).
	z := divide(sub(x, one(), wcfg, false), add(x, one(), wcfg, false), wpr, true, RoundDown, false, 0, wcfg, false)
	sum, numerator := z, z
	z2 := finalise(mul(z, z, wcfg, false), wpr, RoundDown, false, wcfg, false)
	denominator := 3
	rep := -1

	for {
		numerator = finalise(mul(numerator, z2, wcfg, false), wpr, RoundDown, false, wcfg, false)
		t := add(sum, divide(numerator, intDecimal(denominator), wpr, true, RoundDown, false, 0, wcfg, false), wcfg, false)

		if firstDigits(t, wpr) == firstDigits(sum, wpr) {
			sum = mul(sum, intDecimal(2), wcfg, false)

			// Reverse the argument reduction. The exponent is checked against
			// zero so that a negative zero result keeps its sign.
			if e != 0 {
				ln10, err := getLn10(wpr+2, wcfg)
				if err != nil {
					return Decimal{}, err
				}
				sum = add(sum, mul(ln10, NewFromInt(int64(e)), wcfg, false), wcfg, false)
			}
			sum = divide(sum, intDecimal(n), wpr, true, RoundDown, false, 0, wcfg, false)

			if !sdSet {
				if checkRoundingDigits(sum.coefficient, wpr-guard, rm, rep) {
					wpr += guard
					wcfg.Precision = wpr
					z = divide(sub(x1, one(), wcfg, false), add(x1, one(), wcfg, false), wpr, true, RoundDown, false, 0, wcfg, false)
					numerator, t = z, z
					sum = z
					z2 = finalise(mul(z, z, wcfg, false), wpr, RoundDown, false, wcfg, false)
					denominator = 1
					rep = 1
				} else {
					return finalise(sum, pr, rm, true, cfg, true), nil
				}
			} else {
				return sum, nil
			}
		}

		sum = t
		denominator += 2
	}
}

// Exp returns e raised to the power d, rounded to the default context's
// precision.
func (d Decimal) Exp() Decimal { return defaultContext.Exp(d) }

// Exp returns e raised to the power x.
func (c *Context) Exp(x Decimal) Decimal { return naturalExponential(x, 0, false, c.config) }

// Ln returns the natural logarithm of d.
func (d Decimal) Ln() (Decimal, error) { return defaultContext.Ln(d) }

// Ln returns the natural logarithm of x. It fails only where decimal.js throws:
// a request for more digits than the built-in ln(10) constant holds.
func (c *Context) Ln(x Decimal) (Decimal, error) {
	return naturalLogarithm(x, 0, false, c.config)
}

// Log returns the base-y logarithm of d.
func (d Decimal) Log(base Decimal) (Decimal, error) { return defaultContext.Log(d, base) }

// Log returns the logarithm of x to the given base.
//
// It is ln(x)/ln(base) computed with five guard digits, and then decimal.js's
// boundary check: if the quotient's rounding digits sit on a boundary the
// calculation is repeated with ten more digits. When the result cannot have a
// terminating expansion that repeats until the boundary is cleared; otherwise
// fourteen nines after the rounding digit are taken as an exact result and
// rounded up.
func (c *Context) Log(x, base Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding
	const guard = 5

	bd := base.coefficient
	if base.sign < 0 || bd == nil || bd[0] == 0 || base.Eq(one()) {
		return NaN(), nil
	}
	isBase10 := base.Eq(intDecimal(10))

	xd := x.coefficient
	if x.sign < 0 || xd == nil || xd[0] == 0 || x.Eq(one()) {
		switch {
		case xd != nil && xd[0] == 0:
			return Inf(-1), nil
		case x.sign != 1:
			return NaN(), nil
		case xd != nil:
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}, nil
		default:
			return Inf(1), nil
		}
	}

	// Base ten gives a terminating expansion only for an integer power of ten.
	infinite := false
	if isBase10 {
		if len(xd) > 1 {
			infinite = true
		} else {
			k := xd[0]
			for k%10 == 0 {
				k /= 10
			}
			infinite = k != 1
		}
	}

	wcfg := cfg
	sd := pr + guard

	compute := func(sd int) (Decimal, error) {
		wcfg.Precision = sd
		num, err := naturalLogarithm(x, sd, true, wcfg)
		if err != nil {
			return Decimal{}, err
		}
		var den Decimal
		if isBase10 {
			den, err = getLn10(sd+10, wcfg)
		} else {
			den, err = naturalLogarithm(base, sd, true, wcfg)
		}
		if err != nil {
			return Decimal{}, err
		}
		return divide(num, den, sd, true, RoundDown, false, 0, wcfg, false), nil
	}

	r, err := compute(sd)
	if err != nil {
		return Decimal{}, err
	}

	k := pr
	if checkRoundingDigits(r.coefficient, k, rm, -1) {
		for {
			sd += 10
			r, err = compute(sd)
			if err != nil {
				return Decimal{}, err
			}

			if !infinite {
				// Fourteen nines from the second rounding digit - the first may
				// be a four - mean the expansion is exact.
				if digitsAllNine(sliceDigits(digitsToString(r.coefficient), k+1, k+15)) {
					r = finalise(r, pr+1, RoundUp, false, wcfg, false)
				}
				break
			}

			k += 10
			if !checkRoundingDigits(r.coefficient, k, rm, -1) {
				break
			}
		}
	}

	return finalise(r, pr, rm, false, cfg, true), nil
}

// digitsAllNine reports whether s is exactly fourteen nines, standing in for
// the original's `+slice + 1 == 1e14`.
func digitsAllNine(s string) bool {
	if len(s) != 14 {
		return false
	}
	for i := 0; i < 14; i++ {
		if s[i] != '9' {
			return false
		}
	}
	return true
}

// Log10 returns the base-10 logarithm of d.
func (d Decimal) Log10() (Decimal, error) { return defaultContext.Log10(d) }

// Log10 returns the base-10 logarithm of x.
func (c *Context) Log10(x Decimal) (Decimal, error) { return c.Log(x, intDecimal(10)) }

// Log2 returns the base-2 logarithm of d.
func (d Decimal) Log2() (Decimal, error) { return defaultContext.Log2(d) }

// Log2 returns the base-2 logarithm of x.
func (c *Context) Log2(x Decimal) (Decimal, error) { return c.Log(x, intDecimal(2)) }
