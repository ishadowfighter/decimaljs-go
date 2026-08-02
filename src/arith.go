package decimal

// getBase10Exponent converts a base-1e7 limb exponent into the base-10
// exponent of the value's first digit.
func getBase10Exponent(limbs []int, e int) int {
	return e*logBase + digitCount(limbs[0]) - 1
}

// negated returns d with its sign flipped, leaving NaN alone. It is the
// internal form of decimal.js's `y.s = -y.s`, used where addition and
// subtraction hand work to each other.
func negated(d Decimal) Decimal {
	if d.sign == signNaN {
		return d
	}
	d.sign = -d.sign
	return d
}

// prependZeroLimbs returns limbs with n zero limbs in front, which is how both
// operations line up coefficients whose limb exponents differ.
func prependZeroLimbs(limbs []int, n int) []int {
	out := make([]int, n+len(limbs))
	copy(out[n:], limbs)
	return out
}

// Add returns d + y, rounded to the default context's precision.
func (d Decimal) Add(y Decimal) Decimal { return defaultContext.Add(d, y) }

// Add returns x + y, rounded to the Context's precision using its rounding
// mode.
func (c *Context) Add(x, y Decimal) Decimal { return add(x, y, c.config, true) }

// Sub returns d - y, rounded to the default context's precision.
func (d Decimal) Sub(y Decimal) Decimal { return defaultContext.Sub(d, y) }

// Sub returns x - y, rounded to the Context's precision using its rounding
// mode.
func (c *Context) Sub(x, y Decimal) Decimal { return sub(x, y, c.config, true) }

// add is decimal.js's P.plus. Operands of unlike sign are handed to sub, so
// this function only ever adds magnitudes.
func add(x, y Decimal, cfg Config, applyLimits bool) Decimal {
	if x.coefficient == nil || y.coefficient == nil {
		switch {
		case x.sign == signNaN || y.sign == signNaN:
			return NaN()
		case x.coefficient != nil:
			// x is finite, so the infinity wins.
			return y
		case y.coefficient != nil || x.sign == y.sign:
			// y is finite, or both are infinities pointing the same way.
			return x
		default:
			// Infinity - Infinity.
			return NaN()
		}
	}

	if x.sign != y.sign {
		return sub(x, negated(y), cfg, applyLimits)
	}

	pr, rm := cfg.Precision, cfg.Rounding
	xd, yd := x.coefficient, y.coefficient

	if xd[0] == 0 || yd[0] == 0 {
		result := y
		if yd[0] == 0 {
			result = x
		}
		if applyLimits {
			return finalise(result, pr, rm, false, cfg, true)
		}
		return result
	}

	// Both operands are finite, non-zero and of the same sign.
	xd = append([]int(nil), xd...)
	yd = append([]int(nil), yd...)

	k := floorDiv(x.exponent, logBase)
	e := floorDiv(y.exponent, logBase)
	i := k - e

	if i != 0 {
		// Line up the limbs by padding the operand with the smaller limb
		// exponent. Wildly different exponents would need an unbounded number
		// of zero limbs, so the padding is capped: beyond that point the
		// smaller operand can only affect the result through rounding, and one
		// surviving limb carries enough information for that.
		var d *[]int
		var length int
		if i < 0 {
			d = &xd
			i = -i
			length = len(yd)
		} else {
			d = &yd
			e = k
			length = len(xd)
		}

		if limit := ceilDiv(pr, logBase); limit > length {
			length = limit + 1
		} else {
			length++
		}
		if i > length {
			i = length
			*d = (*d)[:1]
		}
		*d = prependZeroLimbs(*d, i)
	}

	// Add from the least significant limb of the shorter operand; anything
	// further left in the longer one is already in place.
	if len(xd) < len(yd) {
		xd, yd = yd, xd
	}
	carry := 0
	for i = len(yd); i > 0; {
		i--
		sum := xd[i] + yd[i] + carry
		carry = sum / base
		xd[i] = sum % base
	}
	if carry != 0 {
		xd = prependZeroLimbs(xd, 1)
		xd[0] = carry
		e++
	}

	// No zero check is needed: two non-zero values of the same sign cannot
	// cancel.
	for len(xd) > 0 && xd[len(xd)-1] == 0 {
		xd = xd[:len(xd)-1]
	}

	result := Decimal{coefficient: xd, exponent: getBase10Exponent(xd, e), sign: y.sign}
	if applyLimits {
		return finalise(result, pr, rm, false, cfg, true)
	}
	return result
}

// sub is decimal.js's P.minus. Operands of unlike sign are handed to add, so
// this function only ever subtracts magnitudes.
func sub(x, y Decimal, cfg Config, applyLimits bool) Decimal {
	if x.coefficient == nil || y.coefficient == nil {
		switch {
		case x.sign == signNaN || y.sign == signNaN:
			return NaN()
		case x.coefficient != nil:
			// x is finite, so the result is the negated infinity.
			return negated(y)
		case y.coefficient != nil || x.sign != y.sign:
			// y is finite, or the two infinities point opposite ways.
			return x
		default:
			// Infinity - Infinity.
			return NaN()
		}
	}

	if x.sign != y.sign {
		return add(x, negated(y), cfg, applyLimits)
	}

	pr, rm := cfg.Precision, cfg.Rounding
	xd, yd := x.coefficient, y.coefficient
	sign := y.sign

	if xd[0] == 0 || yd[0] == 0 {
		var result Decimal
		switch {
		case yd[0] != 0:
			result = negated(y)
		case xd[0] != 0:
			result = x
		default:
			// IEEE 754 (2008) 6.3: a sum of two zeros that cancel is +0 in
			// every rounding mode except towards -Infinity.
			zeroSign := 1
			if rm == RoundFloor {
				zeroSign = -1
			}
			return Decimal{coefficient: []int{0}, exponent: 0, sign: zeroSign}
		}
		if applyLimits {
			return finalise(result, pr, rm, false, cfg, true)
		}
		return result
	}

	xd = append([]int(nil), xd...)
	yd = append([]int(nil), yd...)

	e := floorDiv(y.exponent, logBase)
	xe := floorDiv(x.exponent, logBase)
	k := xe - e

	// xLTy records whether the magnitude of x is the smaller, in which case the
	// operands are swapped and the sign of the result flips.
	var xLTy bool

	if k != 0 {
		xLTy = k < 0

		var d *[]int
		var length int
		if xLTy {
			d = &xd
			k = -k
			length = len(yd)
		} else {
			d = &yd
			e = xe
			length = len(xd)
		}

		// As in add, the zero-limb padding is capped; the cap is one limb
		// larger here because subtraction can cancel leading digits and expose
		// digits further right.
		if limit := ceilDiv(pr, logBase); limit > length {
			length = limit
		}
		length += 2
		if k > length {
			k = length
			*d = (*d)[:1]
		}
		*d = prependZeroLimbs(*d, k)
	} else {
		// Equal limb exponents: compare digits to find the larger magnitude.
		length := len(yd)
		xLTy = len(xd) < length
		if xLTy {
			length = len(xd)
		}
		for i := 0; i < length; i++ {
			if xd[i] != yd[i] {
				xLTy = xd[i] < yd[i]
				break
			}
		}
		k = 0
	}

	if xLTy {
		xd, yd = yd, xd
		sign = -sign
	}

	// Pad xd to the length of yd; yd is left short, since the subtraction only
	// needs to start where yd's digits do.
	if len(yd) > len(xd) {
		xd = append(xd, make([]int, len(yd)-len(xd))...)
	}

	for i := len(yd); i > k; {
		i--
		if xd[i] < yd[i] {
			// Borrow from the nearest non-zero limb to the left.
			j := i
			for j > 0 {
				j--
				if xd[j] != 0 {
					break
				}
				xd[j] = base - 1
			}
			xd[j]--
			xd[i] += base
		}
		xd[i] -= yd[i]
	}

	for len(xd) > 0 && xd[len(xd)-1] == 0 {
		xd = xd[:len(xd)-1]
	}
	for len(xd) > 0 && xd[0] == 0 {
		xd = xd[1:]
		e--
	}

	if len(xd) == 0 {
		zeroSign := 1
		if rm == RoundFloor {
			zeroSign = -1
		}
		return Decimal{coefficient: []int{0}, exponent: 0, sign: zeroSign}
	}

	result := Decimal{coefficient: xd, exponent: getBase10Exponent(xd, e), sign: sign}
	if applyLimits {
		return finalise(result, pr, rm, false, cfg, true)
	}
	return result
}

// Mul returns d * y, rounded to the default context's precision.
func (d Decimal) Mul(y Decimal) Decimal { return defaultContext.Mul(d, y) }

// Mul returns x * y, rounded to the Context's precision using its rounding
// mode.
func (c *Context) Mul(x, y Decimal) Decimal { return mul(x, y, c.config, true) }

// mul is decimal.js's P.times: long multiplication over base-1e7 limbs.
//
// The sign is the product of the two signs, which also carries NaN through for
// free: NaN's sign is zero here, and zero times anything is zero.
func mul(x, y Decimal, cfg Config, applyLimits bool) Decimal {
	xd, yd := x.coefficient, y.coefficient
	sign := x.sign * y.sign

	if xd == nil || len(xd) > 0 && xd[0] == 0 || yd == nil || len(yd) > 0 && yd[0] == 0 {
		switch {
		case sign == signNaN,
			// Zero times infinity, either way round.
			xd != nil && xd[0] == 0 && yd == nil,
			yd != nil && yd[0] == 0 && xd == nil:
			return NaN()
		case xd == nil || yd == nil:
			return Inf(sign)
		default:
			return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}
		}
	}

	e := floorDiv(x.exponent, logBase) + floorDiv(y.exponent, logBase)

	// Let xd be the longer operand, so the inner loop is the long one.
	if len(xd) < len(yd) {
		xd, yd = yd, xd
	}

	// Each partial product is below (1e7-1)^2 + 2e7, comfortably inside the
	// exact-integer range of the float64s decimal.js uses and of Go's int.
	r := make([]int, len(xd)+len(yd))
	carry := 0
	for i := len(yd) - 1; i >= 0; i-- {
		carry = 0
		k := len(xd) + i
		for ; k > i; k-- {
			t := r[k] + yd[i]*xd[k-i-1] + carry
			r[k] = t % base
			carry = t / base
		}
		r[k] = (r[k] + carry) % base
	}

	for len(r) > 0 && r[len(r)-1] == 0 {
		r = r[:len(r)-1]
	}

	// The final carry either occupies the leading limb or leaves it empty.
	if carry != 0 {
		e++
	} else {
		r = r[1:]
	}

	result := Decimal{coefficient: r, exponent: getBase10Exponent(r, e), sign: sign}
	if applyLimits {
		return finalise(result, cfg.Precision, cfg.Rounding, false, cfg, true)
	}
	return result
}
