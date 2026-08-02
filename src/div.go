package decimal

// limbAt returns limbs[i], or zero past the end. decimal.js reads off the end
// of its arrays freely and relies on `x[i] || 0`; the places that do so
// deliberately are transliterated with this helper.
func limbAt(limbs []int, i int) int {
	if i < 0 || i >= len(limbs) {
		return 0
	}
	return limbs[i]
}

// setLimb assigns limbs[i], extending the slice with zeros if i is past the
// end, as an out-of-range assignment does in JavaScript.
func setLimb(limbs []int, i, v int) []int {
	for len(limbs) <= i {
		limbs = append(limbs, 0)
	}
	limbs[i] = v
	return limbs
}

// multiplyLimbs returns limbs * k, for a non-zero k below radix.
func multiplyLimbs(limbs []int, k, radix int) []int {
	out := make([]int, len(limbs))
	carry := 0
	for i := len(limbs) - 1; i >= 0; i-- {
		t := limbs[i]*k + carry
		out[i] = t % radix
		carry = t / radix
	}
	if carry != 0 {
		out = append([]int{carry}, out...)
	}
	return out
}

// compareLimbs orders two limb slices of the stated lengths, most significant
// limb first.
func compareLimbs(a, b []int, aL, bL int) int {
	if aL != bL {
		if aL > bL {
			return 1
		}
		return -1
	}
	for i := 0; i < aL; i++ {
		if a[i] != b[i] {
			if a[i] > b[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// subtractLimbs subtracts b from the first aL limbs of a in place, then drops
// leading zeros.
func subtractLimbs(a, b []int, aL, radix int) []int {
	borrow := 0
	for i := aL - 1; i >= 0; i-- {
		a[i] -= borrow
		bi := limbAt(b, i)
		if a[i] < bi {
			borrow = 1
		} else {
			borrow = 0
		}
		a[i] = borrow*radix + a[i] - bi
	}
	for len(a) > 1 && a[0] == 0 {
		a = a[1:]
	}
	return a
}

// divide is decimal.js's `divide`: long division producing q = x / y, rounded to
// sd significant digits, or — when dp is set — to pr decimal places.
//
// prSet distinguishes decimal.js's `pr == null`, which means "use the context's
// precision and rounding mode". radix is zero for ordinary division and a real
// radix when the routine is reused for base conversion, where it works one
// digit at a time instead of one limb at a time.
//
// The trial-digit loop is the original's, normalisation and correction steps
// included: it produces the same sequence of remainders, which is what makes
// the final rounding land in the same place.
func divide(x, y Decimal, pr int, prSet bool, rm RoundingMode, dp bool, radix int, cfg Config, applyLimits bool) Decimal {
	xd, yd := x.coefficient, y.coefficient

	sign := 1
	if x.sign != y.sign {
		sign = -1
	}

	if xd == nil || len(xd) > 0 && xd[0] == 0 || yd == nil || len(yd) > 0 && yd[0] == 0 {
		switch {
		case x.sign == signNaN || y.sign == signNaN:
			return NaN()
		case xd != nil && yd != nil && xd[0] == yd[0]:
			// Both zero.
			return NaN()
		case xd == nil && yd == nil:
			// Both infinite.
			return NaN()
		case xd != nil && xd[0] == 0 || yd == nil:
			// Zero divided by something, or anything over an infinity.
			return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}
		default:
			// Division by zero.
			return Inf(sign)
		}
	}

	var lb, e int
	if radix != 0 {
		lb = 1
		e = x.exponent - y.exponent
	} else {
		radix = base
		lb = logBase
		e = floorDiv(x.exponent, lb) - floorDiv(y.exponent, lb)
	}

	yL, xL := len(yd), len(xd)
	var qd []int

	// The result exponent may be one less than e, depending on whether the
	// divisor's leading digits exceed the dividend's.
	i := 0
	for i < yL && yd[i] == limbAt(xd, i) {
		i++
	}
	if i < yL && yd[i] > limbAt(xd, i) {
		e--
	}

	var sd int
	if !prSet {
		sd = cfg.Precision
		pr = sd
		rm = cfg.Rounding
	} else if dp {
		sd = pr + (x.exponent - y.exponent) + 1
	} else {
		sd = pr
	}

	more := false

	if sd < 0 {
		qd = append(qd, 1)
		more = true
	} else {
		// Convert a count of base-10 digits into a count of limbs, with two
		// limbs of headroom for the rounding decision.
		sd = sd/lb + 2

		if yL == 1 {
			// Short division.
			k := 0
			divisor := yd[0]
			sd++
			for i = 0; (i < xL || k != 0) && sd != 0; i++ {
				sd--
				t := k*radix + limbAt(xd, i)
				qd = setLimb(qd, i, t/divisor)
				k = t % divisor
			}
			more = k != 0 || i < xL
		} else {
			// Normalise so that the divisor's leading limb is at least half the
			// radix, which bounds the trial-digit error.
			k := radix / (yd[0] + 1)
			if k > 1 {
				yd = multiplyLimbs(yd, k, radix)
				xd = multiplyLimbs(xd, k, radix)
				yL, xL = len(yd), len(xd)
			}

			xi := yL
			rem := make([]int, yL)
			copy(rem, xd[:min(yL, len(xd))])
			remL := len(rem)

			// yz is the divisor shifted one limb left, used when the remainder
			// has grown by a limb.
			yz := append([]int{0}, yd...)

			yd0 := yd[0]
			if len(yd) > 1 && yd[1] >= radix/2 {
				yd0++
			}

			i = 0
			// remDefined stands in for JavaScript's `rem[0] !== undefined`: the
			// remainder runs out when the dividend does.
			remDefined := true

			for {
				k = 0

				cmp := compareLimbs(yd, rem, yL, remL)

				if cmp < 0 {
					// Estimate how many times the divisor goes into the
					// remainder, then correct the estimate.
					rem0 := rem[0]
					if yL != remL {
						rem0 = rem0*radix + limbAt(rem, 1)
					}
					k = rem0 / yd0

					var prod []int
					if k > 1 {
						if k >= radix {
							k = radix - 1
						}
						prod = multiplyLimbs(yd, k, radix)
						prodL := len(prod)
						remL = len(rem)
						cmp = compareLimbs(prod, rem, prodL, remL)
						if cmp == 1 {
							k--
							sub := yd
							if yL < prodL {
								sub = yz
							}
							prod = subtractLimbs(prod, sub, prodL, radix)
						}
					} else {
						// k is 0 or 1. With k of 0 there is nothing to compare
						// afterwards, so cmp is forced to 1 to skip that step.
						if k == 0 {
							cmp, k = 1, 1
						}
						prod = append([]int(nil), yd...)
					}

					if len(prod) < remL {
						prod = append([]int{0}, prod...)
					}
					rem = subtractLimbs(rem, prod, remL, radix)

					if cmp == -1 {
						// The product was smaller than the remainder, so the
						// divisor may still fit once more.
						remL = len(rem)
						cmp = compareLimbs(yd, rem, yL, remL)
						if cmp < 1 {
							k++
							sub := yd
							if yL < remL {
								sub = yz
							}
							rem = subtractLimbs(rem, sub, remL, radix)
						}
					}

					remL = len(rem)
				} else if cmp == 0 {
					k++
					rem = []int{0}
					remL = 1
				}
				// cmp == 1 leaves k at zero.

				qd = setLimb(qd, i, k)
				i++

				if cmp != 0 && rem[0] != 0 {
					// Bring down the next limb of the dividend.
					rem = setLimb(rem, remL, limbAt(xd, xi))
					remL++
				} else {
					if xi < xL {
						rem = []int{xd[xi]}
						remDefined = true
					} else {
						rem = []int{0}
						remDefined = false
					}
					remL = 1
				}

				hasDigit := xi < xL
				xi++
				if !hasDigit && !remDefined {
					break
				}
				if sd == 0 {
					break
				}
				sd--
			}

			more = remDefined
		}

		if len(qd) > 0 && qd[0] == 0 {
			qd = qd[1:]
		}
	}

	q := Decimal{coefficient: qd, sign: sign}

	if lb == 1 {
		// Base conversion works in single digits and does its own rounding.
		q.exponent = e
		return q
	}

	q.exponent = digitCount(limbAt(qd, 0)) + e*lb - 1
	if dp {
		return finalise(q, pr+q.exponent+1, rm, more, cfg, applyLimits)
	}
	return finalise(q, pr, rm, more, cfg, applyLimits)
}

// Div returns d / y, rounded to the default context's precision.
func (d Decimal) Div(y Decimal) Decimal { return defaultContext.Div(d, y) }

// Div returns x / y, rounded to the Context's precision using its rounding
// mode.
func (c *Context) Div(x, y Decimal) Decimal {
	return divide(x, y, 0, false, c.config.Rounding, false, 0, c.config, true)
}

// DivToInt returns the integer part of d / y, rounded to the default context's
// precision.
func (d Decimal) DivToInt(y Decimal) Decimal { return defaultContext.DivToInt(d, y) }

// DivToInt returns the integer part of x / y: the quotient truncated towards
// zero, then rounded to the Context's precision.
func (c *Context) DivToInt(x, y Decimal) Decimal {
	q := divide(x, y, 0, true, RoundDown, true, 0, c.config, true)
	return finalise(q, c.config.Precision, c.config.Rounding, false, c.config, true)
}

// Mod returns d modulo y under the default context's modulo mode.
func (d Decimal) Mod(y Decimal) Decimal { return defaultContext.Mod(d, y) }

// Mod returns x modulo y. The quotient is rounded according to the Context's
// modulo mode and the remainder is then x - q*y, which is how decimal.js
// expresses every one of its modulo modes, including Euclidean division.
func (c *Context) Mod(x, y Decimal) Decimal {
	cfg := c.config

	// NaN if x is non-finite, or y is NaN or zero.
	if x.coefficient == nil || y.sign == signNaN || y.coefficient != nil && y.coefficient[0] == 0 {
		return NaN()
	}

	// x itself if y is infinite or x is zero.
	if y.coefficient == nil || x.coefficient[0] == 0 {
		return finalise(x, cfg.Precision, cfg.Rounding, false, cfg, true)
	}

	// Intermediate results are computed without rounding or exponent limits,
	// as the original disables its `external` flag here.
	var q Decimal
	if cfg.Modulo == ModEuclidean {
		// q = sign(y) * floor(x / abs(y)), so the remainder is non-negative.
		q = divide(x, abs(y), 0, true, RoundFloor, true, 0, cfg, false)
		q.sign *= y.sign
	} else {
		q = divide(x, y, 0, true, RoundingMode(cfg.Modulo), true, 0, cfg, false)
	}

	q = mul(q, y, cfg, false)

	return sub(x, q, cfg, true)
}

// abs returns the magnitude of d, leaving NaN alone.
func abs(d Decimal) Decimal {
	if d.sign == signNaN {
		return d
	}
	d.sign = 1
	return d
}
