package decimal

import "fmt"

// ToFraction returns the simplest fraction n/d equal to d within maxDenominator,
// using the default context.
func (d Decimal) ToFraction(maxDenominator Decimal, limitSet bool) (num, den Decimal, err error) {
	return defaultContext.ToFraction(d, maxDenominator, limitSet)
}

// ToFraction returns a fraction n/d whose value is x, with d no larger than
// maxDenominator. Without a limit the fraction is exact.
//
// It is decimal.js's toFraction: the continued-fraction expansion of x, stopped
// as soon as the denominator would exceed the limit, followed by a comparison
// of the two neighbouring candidates so the closer one is returned.
func (c *Context) ToFraction(x Decimal, maxDenominator Decimal, limitSet bool) (Decimal, Decimal, error) {
	cfg := c.config

	if !x.IsFinite() {
		return x, x, nil
	}

	n1, d0 := one(), one()
	d1 := Decimal{coefficient: []int{0}, exponent: 0, sign: 1}
	n0 := d1

	// d is 10^(number of fraction digits): the exact denominator of x.
	e := getPrecision(x.coefficient) - x.exponent - 1
	k := e % logBase
	if k < 0 {
		k += logBase
	}
	d := Decimal{coefficient: []int{pow10(k)}, exponent: e, sign: 1}

	limit := d
	if e <= 0 {
		limit = n1
	}
	if limitSet {
		if !maxDenominator.IsInteger() || maxDenominator.Lt(n1) {
			return Decimal{}, Decimal{}, fmt.Errorf("%w: max denominator: %s",
				ErrInvalidArgument, c.ValueOf(maxDenominator))
		}
		if !maxDenominator.Gt(d) {
			limit = maxDenominator
		}
	}

	// The expansion runs without rounding, at a precision wide enough to hold
	// every intermediate exactly.
	wcfg := cfg
	wcfg.Precision = len(x.coefficient) * logBase * 2
	exact := wcfg.Precision

	n, _ := NewContext(wcfg).Parse(digitsToString(x.coefficient))

	for {
		q := divide(n, d, 0, true, RoundDown, true, 0, wcfg, false)
		d2 := add(d0, mul(q, d1, wcfg, false), wcfg, false)
		if order, ok := d2.Cmp(limit); ok && order == 1 {
			break
		}
		d0, d1 = d1, d2
		d2 = n1
		n1 = add(n0, mul(q, d2, wcfg, false), wcfg, false)
		n0 = d2
		d2 = d
		d = sub(n, mul(q, d2, wcfg, false), wcfg, false)
		n = d2
	}

	d2 := divide(sub(limit, d0, wcfg, false), d1, 0, true, RoundDown, true, 0, wcfg, false)
	n0 = add(n0, mul(d2, n1, wcfg, false), wcfg, false)
	d0 = add(d0, mul(d2, d1, wcfg, false), wcfg, false)
	n0.sign = x.sign
	n1.sign = x.sign

	// Keep whichever candidate is closer to x.
	distA := abs(sub(divide(n1, d1, exact, true, RoundDown, false, 0, wcfg, false), x, wcfg, false))
	distB := abs(sub(divide(n0, d0, exact, true, RoundDown, false, 0, wcfg, false), x, wcfg, false))
	if order, ok := distA.Cmp(distB); ok && order < 1 {
		return n1, d1, nil
	}
	return n0, d0, nil
}
