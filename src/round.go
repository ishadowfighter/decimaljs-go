package decimal

// pow10 returns 10^n for 0 <= n <= 18, and 0 for larger n where the caller only
// ever divides by the result. decimal.js computes these with Math.pow on
// float64s; the values needed here all fit an int exactly.
func pow10(n int) int {
	if n < 0 || n > 18 {
		return 0
	}
	return pow10Table[n]
}

var pow10Table = [19]int{
	1, 10, 100, 1000, 10000, 100000, 1000000, 10000000,
	1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18,
}

// digitCount returns the number of decimal digits in a non-negative limb.
func digitCount(n int) int {
	digits := 1
	for ; n >= 10; n /= 10 {
		digits++
	}
	return digits
}

// divPow10 returns floor(n / 10^p), where a p large enough to shift every digit
// out gives zero. decimal.js relies on float division here and truncates with
// `| 0`, which comes to the same thing for the non-negative values involved.
func divPow10(n, p int) int {
	if p < 0 {
		// Multiplying up can only ever push the digits of interest out of
		// range; every use of a negative p here is followed by a `% 10`, and
		// n * 10^|p| is always a multiple of 10.
		return 0
	}
	d := pow10(p)
	if d == 0 {
		return 0
	}
	return n / d
}

// modPow10 returns n mod 10^p: the digits of n to the right of position p.
func modPow10(n, p int) int {
	if p <= 0 {
		return 0
	}
	d := pow10(p)
	if d == 0 {
		return n
	}
	return n % d
}

// ceilDiv divides rounding away from zero for positive operands, as
// Math.ceil(a / b) does.
func ceilDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a > 0) == (b > 0) {
		q++
	}
	return q
}

// finalise rounds x to sd significant digits using rounding mode rm, and then
// applies the exponent limits.
//
// It is a transliteration of decimal.js's finalise, which is the single most
// load-bearing function in the library: every arithmetic result and every
// formatting method passes through it, so its digit-position bookkeeping is
// reproduced step for step rather than reimplemented. The variables keep their
// original roles:
//
//	rd     the rounding digit, the first digit being discarded
//	w      the limb containing rd
//	xdi    the index of w within the coefficient
//	digits the number of digits in w
//	i      the index of rd within w, as if limbs were written with leading zeros
//	j      the actual index of rd within w; negative when rd is a leading zero
//
// truncated says that digits beyond the coefficient were already discarded, so
// the value is known to be larger in magnitude than its limbs alone show. The
// series-based functions need it; it also lets finalise extend the coefficient
// rather than give up when sd reaches past the end.
//
// applyLimits corresponds to decimal.js's `external` flag: overflow to Infinity
// and underflow to zero happen for results handed back to the caller, not for
// intermediates inside a calculation.
func finalise(x Decimal, sd int, rm RoundingMode, truncated bool, cfg Config, applyLimits bool) Decimal {
	if x.coefficient == nil {
		return x
	}

	xd := make([]int, len(x.coefficient), len(x.coefficient)+2)
	copy(xd, x.coefficient)
	e := x.exponent
	rounded := true

	digits := digitCount(xd[0])
	i := sd - digits

	var j, w, xdi, rd int

	if i < 0 {
		// The rounding digit is in the first limb.
		i += logBase
		j = sd
		xdi = 0
		w = xd[0]
		rd = divPow10(w, digits-j-1) % 10
	} else {
		xdi = ceilDiv(i+1, logBase)
		if xdi >= len(xd) {
			if truncated {
				// Needed by the series-based functions: extend with zero limbs
				// and round against the known-truncated tail.
				for len(xd) <= xdi {
					xd = append(xd, 0)
				}
				w, rd = 0, 0
				digits = 1
				i %= logBase
				j = i - logBase + 1
			} else {
				// Fewer digits than requested: nothing to round.
				rounded = false
			}
		} else {
			w = xd[xdi]
			digits = digitCount(w)
			i %= logBase
			// The number of leading zeros of w is logBase - digits.
			j = i - logBase + digits
			if j < 0 {
				rd = 0
			} else {
				rd = divPow10(w, digits-j-1) % 10
			}
		}
	}

	if rounded {
		// Any non-zero digit after the rounding digit makes the discarded tail
		// non-zero, which several modes need to know about.
		tailNonZero := false
		if j < 0 {
			tailNonZero = w != 0
		} else {
			tailNonZero = modPow10(w, digits-j-1) != 0
		}
		truncated = truncated || sd < 0 || xdi+1 < len(xd) || tailNonZero

		roundUp := shouldRoundUp(rm, rd, truncated, x.sign, i, j, w, digits, xd, xdi)

		if sd < 1 || xd[0] == 0 {
			// Everything is discarded: the result is zero, or a one in the
			// last retained place if rounding up.
			xd = xd[:0]
			if roundUp {
				// Convert sd to decimal places, then build 1, 0.1, 0.01, ...
				sd -= e + 1
				xd = append(xd, pow10((logBase-sd%logBase)%logBase))
				e = -sd
			} else {
				xd = append(xd, 0)
				e = 0
			}
			return applyExponentLimits(Decimal{coefficient: xd, exponent: e, sign: x.sign}, cfg, applyLimits)
		}

		// Discard the digits beyond the rounding position. k is the value of
		// one unit in the last retained place, i.e. the carry to add when
		// rounding up.
		var k int
		if i == 0 {
			xd = xd[:xdi]
			k = 1
			xdi--
		} else {
			xd = xd[:xdi+1]
			k = pow10(logBase - i)
			// E.g. 56700 becomes 56000 if 7 is the rounding digit. j > 0 means
			// i is past the leading zeros of w.
			if j > 0 {
				xd[xdi] = modPow10(divPow10(w, digits-j), j) * k
			} else {
				xd[xdi] = 0
			}
		}

		if roundUp {
			for {
				if xdi == 0 {
					// A carry out of the first limb lengthens the value.
					before := digitCount(xd[0])
					xd[0] += k
					if before != digitCount(xd[0]) {
						e++
						if xd[0] == base {
							xd[0] = 1
						}
					}
					break
				}
				xd[xdi] += k
				if xd[xdi] != base {
					break
				}
				xd[xdi] = 0
				xdi--
				k = 1
			}
		}

		for len(xd) > 0 && xd[len(xd)-1] == 0 {
			xd = xd[:len(xd)-1]
		}
	}

	return applyExponentLimits(Decimal{coefficient: xd, exponent: e, sign: x.sign}, cfg, applyLimits)
}

// shouldRoundUp decides whether the retained digits are incremented, given the
// rounding digit rd and whether anything non-zero follows it. The mode numbers
// are decimal.js's, and the two direction-sensitive pairs — ceil/floor and
// half-ceil/half-floor — depend on the sign of the value.
func shouldRoundUp(rm RoundingMode, rd int, truncated bool, sign, i, j, w, digits int, xd []int, xdi int) bool {
	if rm < RoundHalfUp {
		// Away from zero, towards zero, or towards one of the infinities.
		awayFromZero := RoundCeil
		if sign < 0 {
			awayFromZero = RoundFloor
		}
		return (rd != 0 || truncated) && (rm == RoundUp || rm == awayFromZero)
	}

	if rd > 5 {
		return true
	}
	if rd < 5 {
		return false
	}

	// Exactly half, so far as the rounding digit shows.
	if rm == RoundHalfUp || truncated {
		return true
	}
	if rm == RoundHalfEven && isOddDigitLeftOf(i, j, w, digits, xd, xdi) {
		return true
	}
	halfAwayFromZero := RoundHalfCeil
	if sign < 0 {
		halfAwayFromZero = RoundHalfFloor
	}
	return rm == halfAwayFromZero
}

// isOddDigitLeftOf reports whether the digit immediately to the left of the
// rounding digit is odd, which is what round-half-even turns on. When the
// rounding digit is the first digit of its limb, the digit to its left is the
// last digit of the previous limb.
func isOddDigitLeftOf(i, j, w, digits int, xd []int, xdi int) bool {
	var left int
	switch {
	case i > 0:
		if j > 0 {
			left = divPow10(w, digits-j)
		}
	case xdi > 0:
		left = xd[xdi-1]
	}
	return left%10&1 == 1
}

// applyExponentLimits overflows to Infinity above maxE and underflows to zero
// below minE, keeping the sign in both cases.
func applyExponentLimits(x Decimal, cfg Config, apply bool) Decimal {
	if !apply {
		return x
	}
	switch {
	case x.exponent > cfg.MaxE:
		return Decimal{coefficient: nil, exponent: 0, sign: x.sign}
	case x.exponent < cfg.MinE:
		return Decimal{coefficient: []int{0}, exponent: 0, sign: x.sign}
	}
	return x
}

// finaliseNoRound applies the exponent limits without rounding, which is what
// decimal.js's finalise does when called with no significant-digit count.
func finaliseNoRound(x Decimal, cfg Config) Decimal {
	if x.coefficient == nil {
		return x
	}
	return applyExponentLimits(x, cfg, true)
}

// Abs returns the magnitude of d. NaN stays NaN, and -0 becomes 0.
func (d Decimal) Abs() Decimal { return defaultContext.Abs(d) }

// Abs returns the magnitude of x.
func (c *Context) Abs(x Decimal) Decimal { return finaliseNoRound(abs(x), c.config) }

// Neg returns d with its sign flipped. NaN stays NaN, and the sign of zero
// flips as decimal.js's does.
func (d Decimal) Neg() Decimal { return defaultContext.Neg(d) }

// Neg returns x with its sign flipped.
func (c *Context) Neg(x Decimal) Decimal { return finaliseNoRound(negated(x), c.config) }

// Round returns d rounded to a whole number using the default context's
// rounding mode.
func (d Decimal) Round() Decimal { return defaultContext.Round(d) }

// Round returns x rounded to a whole number using the Context's rounding mode.
func (c *Context) Round(x Decimal) Decimal {
	return finalise(x, x.exponent+1, c.config.Rounding, false, c.config, true)
}

// Floor returns d rounded to a whole number towards -Infinity.
func (d Decimal) Floor() Decimal { return defaultContext.Floor(d) }

// Floor returns x rounded to a whole number towards -Infinity.
func (c *Context) Floor(x Decimal) Decimal {
	return finalise(x, x.exponent+1, RoundFloor, false, c.config, true)
}

// Ceil returns d rounded to a whole number towards +Infinity.
func (d Decimal) Ceil() Decimal { return defaultContext.Ceil(d) }

// Ceil returns x rounded to a whole number towards +Infinity.
func (c *Context) Ceil(x Decimal) Decimal {
	return finalise(x, x.exponent+1, RoundCeil, false, c.config, true)
}

// Trunc returns d with its fraction part discarded.
func (d Decimal) Trunc() Decimal { return defaultContext.Trunc(d) }

// Trunc returns x with its fraction part discarded.
func (c *Context) Trunc(x Decimal) Decimal {
	return finalise(x, x.exponent+1, RoundDown, false, c.config, true)
}

// ToDecimalPlaces returns d rounded to dp decimal places using the default
// context's rounding mode.
func (d Decimal) ToDecimalPlaces(dp int) (Decimal, error) {
	return defaultContext.ToDecimalPlaces(d, dp, defaultContext.config.Rounding)
}

// ToDecimalPlaces returns x rounded to dp decimal places using rounding mode
// rm. decimal.js throws for a dp outside 0..1e9 or an unknown rounding mode;
// here those are errors.
func (c *Context) ToDecimalPlaces(x Decimal, dp int, rm RoundingMode) (Decimal, error) {
	if dp < 0 || dp > maxDigits {
		return Decimal{}, wrapInvalidArgument("decimal places", dp)
	}
	if rm < RoundUp || rm > RoundHalfFloor {
		return Decimal{}, wrapInvalidArgument("rounding mode", int(rm))
	}
	return finalise(x, dp+x.exponent+1, rm, false, c.config, true), nil
}

// ToSignificantDigits returns d rounded to sd significant digits using the
// default context's rounding mode.
func (d Decimal) ToSignificantDigits(sd int) (Decimal, error) {
	return defaultContext.ToSignificantDigits(d, sd, defaultContext.config.Rounding)
}

// ToSignificantDigits returns x rounded to sd significant digits using rounding
// mode rm.
func (c *Context) ToSignificantDigits(x Decimal, sd int, rm RoundingMode) (Decimal, error) {
	if sd < 1 || sd > maxDigits {
		return Decimal{}, wrapInvalidArgument("significant digits", sd)
	}
	if rm < RoundUp || rm > RoundHalfFloor {
		return Decimal{}, wrapInvalidArgument("rounding mode", int(rm))
	}
	return finalise(x, sd, rm, false, c.config, true), nil
}
