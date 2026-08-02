package decimal

// Cmp compares d and y, returning -1, 0 or 1 as d is less than, equal to, or
// greater than y. ok is false when either value is NaN, where decimal.js
// returns NaN rather than an ordering.
//
// Zeros compare equal regardless of sign, as in decimal.js: -0 is neither less
// than nor greater than 0.
func (d Decimal) Cmp(y Decimal) (order int, ok bool) {
	xd, yd := d.coefficient, y.coefficient
	xs, ys := d.sign, y.sign

	// Either value non-finite.
	if xd == nil || yd == nil {
		switch {
		case xs == signNaN || ys == signNaN:
			return 0, false
		case xs != ys:
			return xs, true
		case xd == nil && yd == nil:
			// Both are infinities of the same sign.
			return 0, true
		case (xd == nil) != (xs < 0):
			// d is +Infinity, or d is finite and negative while y is
			// -Infinity.
			return 1, true
		default:
			return -1, true
		}
	}

	// Either value zero.
	if xd[0] == 0 || yd[0] == 0 {
		switch {
		case xd[0] != 0:
			return xs, true
		case yd[0] != 0:
			return -ys, true
		default:
			return 0, true
		}
	}

	if xs != ys {
		return xs, true
	}

	if d.exponent != y.exponent {
		return orderBy(d.exponent > y.exponent, xs), true
	}

	for i, j := 0, min(len(xd), len(yd)); i < j; i++ {
		if xd[i] != yd[i] {
			return orderBy(xd[i] > yd[i], xs), true
		}
	}

	if len(xd) == len(yd) {
		return 0, true
	}
	return orderBy(len(xd) > len(yd), xs), true
}

// orderBy turns "the magnitude of d is the greater" into an ordering, flipping
// it for negative values. It stands in for decimal.js's `a > b ^ xs < 0`.
func orderBy(greaterMagnitude bool, sign int) int {
	if greaterMagnitude != (sign < 0) {
		return 1
	}
	return -1
}

// Eq reports whether d and y are numerically equal. NaN is equal to nothing,
// including itself, and -0 equals 0.
func (d Decimal) Eq(y Decimal) bool {
	order, ok := d.Cmp(y)
	return ok && order == 0
}

// Gt reports whether d is greater than y; false if either is NaN.
func (d Decimal) Gt(y Decimal) bool {
	order, ok := d.Cmp(y)
	return ok && order > 0
}

// Gte reports whether d is greater than or equal to y; false if either is NaN.
func (d Decimal) Gte(y Decimal) bool {
	order, ok := d.Cmp(y)
	return ok && order >= 0
}

// Lt reports whether d is less than y; false if either is NaN.
func (d Decimal) Lt(y Decimal) bool {
	order, ok := d.Cmp(y)
	return ok && order < 0
}

// Lte reports whether d is less than or equal to y; false if either is NaN.
func (d Decimal) Lte(y Decimal) bool {
	order, ok := d.Cmp(y)
	return ok && order <= 0
}

// IsInteger reports whether d is a finite whole number.
//
// The test is decimal.js's: with limbs aligned to seven-digit boundaries
// counted from the decimal point, a value is an integer exactly when the limb
// holding the units digit is the last one, which is what comparing
// floor(e/logBase) against len(d)-2 amounts to.
func (d Decimal) IsInteger() bool {
	return d.coefficient != nil && floorDiv(d.exponent, logBase) > len(d.coefficient)-2
}

// IsZero reports whether d is 0 or -0.
func (d Decimal) IsZero() bool {
	return d.coefficient != nil && d.coefficient[0] == 0
}

// IsNegative reports whether d has a negative sign, which includes -0 and
// -Infinity and excludes NaN.
func (d Decimal) IsNegative() bool { return d.sign < 0 }

// IsPositive reports whether d has a positive sign, which includes 0 and
// +Infinity and excludes NaN.
func (d Decimal) IsPositive() bool { return d.sign > 0 }

// Signum reports -1, 0 or 1 as d is negative, zero or positive.
//
// decimal.js's Decimal.sign is finer grained: it distinguishes -0 from 0 and
// returns NaN for NaN, neither of which an int can express. Callers that need
// those cases can reconstruct them from Sign, IsZero and IsNaN, which is what
// the test adapter does.
func (d Decimal) Signum() int {
	switch {
	case d.IsNaN(), d.IsZero():
		return 0
	case d.sign < 0:
		return -1
	default:
		return 1
	}
}

// floorDiv divides rounding towards negative infinity, as JavaScript's
// Math.floor(a / b) does. Go's integer division truncates towards zero, which
// differs for negative numerators and would misplace limb boundaries.
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
