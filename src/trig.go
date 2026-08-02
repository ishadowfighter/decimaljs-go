package decimal

import "math"

// getPi returns pi rounded to sd digits with mode rm.
func getPi(sd int, rm RoundingMode, cfg Config) (Decimal, error) {
	if sd > piPrecision {
		return Decimal{}, ErrPrecisionLimitExceeded
	}
	x, _ := NewContext(cfg).Parse(piDigits)
	return finalise(x, sd, rm, true, cfg, false), nil
}

// tinyPow is b^e for the small integer exponents the argument reductions use,
// computed by repeated multiplication as decimal.js does so the float64 result
// is bit for bit the same.
func tinyPow(b float64, e int) float64 {
	n := b
	for ; e > 1; e-- {
		n *= b
	}
	return n
}

// taylorSeries sums the alternating (or, for isHyperbolic, non-alternating)
// series decimal.js uses for sine, cosine and their hyperbolic counterparts.
// Terms are taken two at a time, which is why n advances four times per pass.
func taylorSeries(n int, x, y Decimal, isHyperbolic bool, cfg Config) Decimal {
	pr := cfg.Precision
	k := ceilDiv(pr, logBase)

	x2 := mul(x, x, cfg, false)
	u := y
	var t Decimal

	for {
		a := n
		n++
		b := n
		n++
		t = divide(mul(u, x2, cfg, false), intDecimal(a*b), pr, true, RoundDown, false, 0, cfg, false)
		if isHyperbolic {
			u = add(y, t, cfg, false)
		} else {
			u = sub(y, t, cfg, false)
		}

		a = n
		n++
		b = n
		n++
		y = divide(mul(t, x2, cfg, false), intDecimal(a*b), pr, true, RoundDown, false, 0, cfg, false)
		t = add(u, y, cfg, false)

		if len(t.coefficient) > k {
			j := k
			for j >= 0 && t.coefficient[j] == limbAt(u.coefficient, j) {
				j--
			}
			if j == -1 {
				break
			}
		}

		u, y, t = y, t, u
	}

	if len(t.coefficient) > k+1 {
		t.coefficient = t.coefficient[:k+1]
	}
	return t
}

// isOddLimb reports whether the value's last limb is odd, which for an integer
// decides the parity.
func isOddLimb(x Decimal) bool {
	return x.coefficient[len(x.coefficient)-1]&1 == 1
}

// toLessThanHalfPi reduces x into [0, pi/2] and reports which quadrant the
// original angle fell in, as decimal.js's module-level `quadrant` records.
func toLessThanHalfPi(x Decimal, cfg Config) (Decimal, int, error) {
	isNeg := x.sign < 0
	pi, err := getPi(cfg.Precision, RoundDown, cfg)
	if err != nil {
		return Decimal{}, 0, err
	}
	halfPi := mul(pi, half(), cfg, true)

	x = abs(x)

	if x.Lte(halfPi) {
		if isNeg {
			return x, 4, nil
		}
		return x, 1, nil
	}

	t := NewContext(cfg).DivToInt(x, pi)

	if t.IsZero() {
		if isNeg {
			return absSub(x, pi, cfg), 3, nil
		}
		return absSub(x, pi, cfg), 2, nil
	}

	x = sub(x, mul(t, pi, cfg, true), cfg, true)

	// 0 <= x < pi
	if x.Lte(halfPi) {
		switch {
		case isOddLimb(t) && isNeg:
			return x, 2, nil
		case isOddLimb(t):
			return x, 3, nil
		case isNeg:
			return x, 4, nil
		default:
			return x, 1, nil
		}
	}

	var quadrant int
	switch {
	case isOddLimb(t) && isNeg:
		quadrant = 1
	case isOddLimb(t):
		quadrant = 4
	case isNeg:
		quadrant = 3
	default:
		quadrant = 2
	}
	return absSub(x, pi, cfg), quadrant, nil
}

// absSub returns |x - y|.
func absSub(x, y Decimal, cfg Config) Decimal { return abs(sub(x, y, cfg, false)) }

// sine is the series for sin with decimal.js's fifth-angle argument reduction.
func sine(x Decimal, cfg Config) Decimal {
	length := len(x.coefficient)

	if length < 3 {
		if x.IsZero() {
			return x
		}
		return taylorSeries(2, x, x, false, cfg)
	}

	k := 1.4 * math.Sqrt(float64(length))
	ki := 16
	if k <= 16 {
		ki = int(k)
	}

	x = mul(x, NewContext(cfg).NewFromFloat(1/tinyPow(5, ki)), cfg, true)
	x = taylorSeries(2, x, x, false, cfg)

	// sin(x) = sin(x/5)(5 + sin^2(x/5)(16 sin^2(x/5) - 20))
	d5, d16, d20 := intDecimal(5), intDecimal(16), intDecimal(20)
	for ; ki > 0; ki-- {
		sin2 := mul(x, x, cfg, true)
		inner := sub(mul(d16, sin2, cfg, true), d20, cfg, true)
		x = mul(x, add(d5, mul(sin2, inner, cfg, true), cfg, true), cfg, true)
	}
	return x
}

// cosine is the series for cos with decimal.js's quarter-angle reduction.
func cosine(x Decimal, cfg Config) Decimal {
	if x.IsZero() {
		return x
	}

	length := len(x.coefficient)
	var k int
	var y Decimal
	ctx := NewContext(cfg)
	if length < 32 {
		k = ceilDiv(length, 3)
		y = ctx.NewFromFloat(1 / tinyPow(4, k))
	} else {
		k = 16
		y, _ = ctx.Parse("2.3283064365386962890625e-10")
	}

	wcfg := cfg
	wcfg.Precision += k

	x = taylorSeries(1, mul(x, y, wcfg, true), one(), false, wcfg)

	// cos(x) = 8*(cos^4(x/4) - cos^2(x/4)) + 1
	for i := k; i > 0; i-- {
		cos2 := mul(x, x, wcfg, true)
		x = add(mul(sub(mul(cos2, cos2, wcfg, true), cos2, wcfg, true), intDecimal(8), wcfg, true), one(), wcfg, true)
	}

	return x
}

// workingConfig widens the precision the way the trigonometric methods do,
// and switches to rounding down while the series runs.
func workingConfig(cfg Config, extra int) Config {
	out := cfg
	out.Precision += extra
	out.Rounding = RoundDown
	return out
}

// Sin returns the sine of d, taken in radians.
func (d Decimal) Sin() (Decimal, error) { return defaultContext.Sin(d) }

// Sin returns the sine of x.
func (c *Context) Sin(x Decimal) (Decimal, error) {
	r, _, err := sinWithQuadrant(x, c.config)
	return r, err
}

// sinWithQuadrant returns sin(x) and the quadrant of the reduced argument,
// which tan needs and decimal.js keeps in a module-level variable.
func sinWithQuadrant(x Decimal, cfg Config) (Decimal, int, error) {
	if !x.IsFinite() {
		return NaN(), 0, nil
	}
	if x.IsZero() {
		return x, 0, nil
	}

	pr, rm := cfg.Precision, cfg.Rounding
	wcfg := workingConfig(cfg, maxInt(x.exponent, sdOf(x))+logBase)

	reduced, quadrant, err := toLessThanHalfPi(x, wcfg)
	if err != nil {
		return Decimal{}, 0, err
	}
	r := sine(reduced, wcfg)

	if quadrant > 2 {
		r = negated(r)
	}
	return finalise(r, pr, rm, true, cfg, true), quadrant, nil
}

// Cos returns the cosine of d, taken in radians.
func (d Decimal) Cos() (Decimal, error) { return defaultContext.Cos(d) }

// Cos returns the cosine of x.
func (c *Context) Cos(x Decimal) (Decimal, error) {
	cfg := c.config
	if !x.IsFinite() {
		return NaN(), nil
	}
	if x.IsZero() {
		return one(), nil
	}

	pr, rm := cfg.Precision, cfg.Rounding
	wcfg := workingConfig(cfg, maxInt(x.exponent, sdOf(x))+logBase)

	reduced, quadrant, err := toLessThanHalfPi(x, wcfg)
	if err != nil {
		return Decimal{}, err
	}
	r := cosine(reduced, wcfg)

	if quadrant == 2 || quadrant == 3 {
		r = negated(r)
	}
	return finalise(r, pr, rm, true, cfg, true), nil
}

// Tan returns the tangent of d, taken in radians.
func (d Decimal) Tan() (Decimal, error) { return defaultContext.Tan(d) }

// Tan returns the tangent of x, computed as sin/sqrt(1-sin^2) so that only one
// series has to run.
func (c *Context) Tan(x Decimal) (Decimal, error) {
	cfg := c.config
	if !x.IsFinite() {
		return NaN(), nil
	}
	if x.IsZero() {
		return x, nil
	}

	pr, rm := cfg.Precision, cfg.Rounding
	wcfg := workingConfig(cfg, 10)

	s, quadrant, err := sinWithQuadrant(x, wcfg)
	if err != nil {
		return Decimal{}, err
	}
	s.sign = 1

	denom := NewContext(wcfg).Sqrt(sub(one(), mul(s, s, wcfg, true), wcfg, true))
	r := divide(s, denom, pr+10, true, RoundUp, false, 0, wcfg, true)

	if quadrant == 2 || quadrant == 4 {
		r = negated(r)
	}
	return finalise(r, pr, rm, true, cfg, true), nil
}

// Atan returns the arctangent of d.
func (d Decimal) Atan() (Decimal, error) { return defaultContext.Atan(d) }

// Atan returns the arctangent of x, by the series for atan after halving the
// argument until it is small enough to converge quickly.
func (c *Context) Atan(x Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding

	switch {
	case !x.IsFinite():
		if x.sign == signNaN {
			return NaN(), nil
		}
		if pr+4 <= piPrecision {
			r, err := getPi(pr+4, rm, cfg)
			if err != nil {
				return Decimal{}, err
			}
			r = mul(r, half(), cfg, true)
			r.sign = x.sign
			return r, nil
		}
	case x.IsZero():
		return x, nil
	case abs(x).Eq(one()) && pr+4 <= piPrecision:
		r, err := getPi(pr+4, rm, cfg)
		if err != nil {
			return Decimal{}, err
		}
		quarter, _ := NewContext(cfg).Parse("0.25")
		r = mul(r, quarter, cfg, true)
		r.sign = x.sign
		return r, nil
	}

	wpr := pr + 10
	wcfg := cfg
	wcfg.Precision = wpr
	wcfg.Rounding = RoundDown

	// atan(x) = 2*atan(x / (1 + sqrt(1 + x^2))), applied k times.
	k := wpr/logBase + 2
	if k > 28 {
		k = 28
	}
	ctx := NewContext(wcfg)
	for i := k; i > 0; i-- {
		root := ctx.Sqrt(add(mul(x, x, wcfg, true), one(), wcfg, true))
		x = divide(x, add(root, one(), wcfg, true), wpr, true, RoundDown, false, 0, wcfg, true)
	}

	j := ceilDiv(wpr, logBase)
	n := 1
	x2 := mul(x, x, wcfg, false)
	r := x
	px := x
	i := 0

	// atan(x) = x - x^3/3 + x^5/5 - ...
	for i != -1 {
		px = mul(px, x2, wcfg, false)
		n += 2
		t := sub(r, divide(px, intDecimal(n), wpr, true, RoundDown, false, 0, wcfg, false), wcfg, false)

		px = mul(px, x2, wcfg, false)
		n += 2
		r = add(t, divide(px, intDecimal(n), wpr, true, RoundDown, false, 0, wcfg, false), wcfg, false)

		if len(r.coefficient) > j {
			for i = j; i >= 0 && r.coefficient[i] == limbAt(t.coefficient, i); i-- {
			}
		}
	}

	if k > 0 {
		r = mul(r, intDecimal(2<<(k-1)), wcfg, false)
	}

	return finalise(r, pr, rm, true, cfg, true), nil
}

// Asin returns the arcsine of d.
func (d Decimal) Asin() (Decimal, error) { return defaultContext.Asin(d) }

// Asin returns the arcsine of x, via atan of a half-angle expression that
// avoids cancellation near the ends of the domain.
func (c *Context) Asin(x Decimal) (Decimal, error) {
	cfg := c.config
	if x.IsZero() {
		return x, nil
	}

	pr, rm := cfg.Precision, cfg.Rounding
	k, ok := abs(x).Cmp(one())
	if !ok || k != -1 {
		if ok && k == 0 {
			halfPi, err := getPi(pr+4, rm, cfg)
			if err != nil {
				return Decimal{}, err
			}
			halfPi = mul(halfPi, half(), cfg, true)
			halfPi.sign = x.sign
			return halfPi, nil
		}
		return NaN(), nil
	}

	wcfg := workingConfig(cfg, 6)
	wctx := NewContext(wcfg)

	root := wctx.Sqrt(mul(sub(one(), x, wcfg, true), add(one(), x, wcfg, true), wcfg, true))
	q := divide(x, add(root, one(), wcfg, true), wcfg.Precision, false, RoundDown, false, 0, wcfg, true)
	a, err := wctx.Atan(q)
	if err != nil {
		return Decimal{}, err
	}
	return mul(a, intDecimal(2), cfg, true), nil
}

// Acos returns the arccosine of d.
func (d Decimal) Acos() (Decimal, error) { return defaultContext.Acos(d) }

// Acos returns the arccosine of x.
func (c *Context) Acos(x Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding

	k, ok := abs(x).Cmp(one())
	if !ok || k != -1 {
		if ok && k == 0 {
			if x.IsNegative() {
				return getPi(pr, rm, cfg)
			}
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}, nil
		}
		return NaN(), nil
	}

	if x.IsZero() {
		pi, err := getPi(pr+4, rm, cfg)
		if err != nil {
			return Decimal{}, err
		}
		return mul(pi, half(), cfg, true), nil
	}

	wcfg := workingConfig(cfg, 6)
	wctx := NewContext(wcfg)

	q := divide(sub(one(), x, wcfg, true), add(x, one(), wcfg, true), wcfg.Precision, false, RoundDown, false, 0, wcfg, true)
	a, err := wctx.Atan(wctx.Sqrt(q))
	if err != nil {
		return Decimal{}, err
	}
	return mul(a, intDecimal(2), cfg, true), nil
}

// Atan2 returns the angle of the point (x, y) from the positive x axis.
func Atan2(y, x Decimal) (Decimal, error) { return defaultContext.Atan2(y, x) }

// Atan2 returns the angle of the point (x, y), in the correct quadrant.
func (c *Context) Atan2(y, x Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding
	wpr := pr + 4

	switch {
	case y.sign == signNaN || x.sign == signNaN:
		return NaN(), nil

	case !y.IsFinite() && !x.IsFinite():
		pi, err := getPi(wpr, RoundDown, cfg)
		if err != nil {
			return Decimal{}, err
		}
		frac, _ := NewContext(cfg).Parse("0.25")
		if x.sign <= 0 {
			frac, _ = NewContext(cfg).Parse("0.75")
		}
		r := mul(pi, frac, cfg, true)
		r.sign = y.sign
		return r, nil

	case !x.IsFinite() || y.IsZero():
		var r Decimal
		if x.sign < 0 {
			pi, err := getPi(pr, rm, cfg)
			if err != nil {
				return Decimal{}, err
			}
			r = pi
		} else {
			r = Decimal{coefficient: []int{0}, exponent: 0, sign: 1}
		}
		r.sign = y.sign
		return r, nil

	case !y.IsFinite() || x.IsZero():
		pi, err := getPi(wpr, RoundDown, cfg)
		if err != nil {
			return Decimal{}, err
		}
		r := mul(pi, half(), cfg, true)
		r.sign = y.sign
		return r, nil

	case x.sign < 0:
		wcfg := cfg
		wcfg.Precision = wpr
		wcfg.Rounding = RoundDown
		wctx := NewContext(wcfg)
		r, err := wctx.Atan(divide(y, x, wpr, true, RoundDown, false, 0, wcfg, true))
		if err != nil {
			return Decimal{}, err
		}
		pi, err := getPi(wpr, RoundDown, wcfg)
		if err != nil {
			return Decimal{}, err
		}
		if y.sign < 0 {
			return sub(r, pi, cfg, true), nil
		}
		return add(r, pi, cfg, true), nil

	default:
		return c.Atan(divide(y, x, wpr, true, RoundDown, false, 0, cfg, true))
	}
}

// Sinh returns the hyperbolic sine of d.
func (d Decimal) Sinh() Decimal { return defaultContext.Sinh(d) }

// Sinh returns the hyperbolic sine of x.
func (c *Context) Sinh(x Decimal) Decimal {
	cfg := c.config
	if !x.IsFinite() || x.IsZero() {
		return x
	}

	pr, rm := cfg.Precision, cfg.Rounding
	wcfg := workingConfig(cfg, maxInt(x.exponent, sdOf(x))+4)
	length := len(x.coefficient)

	if length < 3 {
		x = taylorSeries(2, x, x, true, wcfg)
	} else {
		k := 1.4 * math.Sqrt(float64(length))
		ki := 16
		if k <= 16 {
			ki = int(k)
		}

		x = mul(x, NewContext(wcfg).NewFromFloat(1/tinyPow(5, ki)), wcfg, true)
		x = taylorSeries(2, x, x, true, wcfg)

		d5, d16, d20 := intDecimal(5), intDecimal(16), intDecimal(20)
		for ; ki > 0; ki-- {
			sinh2 := mul(x, x, wcfg, true)
			inner := add(mul(d16, sinh2, wcfg, true), d20, wcfg, true)
			x = mul(x, add(d5, mul(sinh2, inner, wcfg, true), wcfg, true), wcfg, true)
		}
	}

	return finalise(x, pr, rm, true, cfg, true)
}

// Cosh returns the hyperbolic cosine of d.
func (d Decimal) Cosh() Decimal { return defaultContext.Cosh(d) }

// Cosh returns the hyperbolic cosine of x.
func (c *Context) Cosh(x Decimal) Decimal {
	cfg := c.config
	if !x.IsFinite() {
		if x.sign == signNaN {
			return NaN()
		}
		return Inf(1)
	}
	if x.IsZero() {
		return one()
	}

	pr, rm := cfg.Precision, cfg.Rounding
	wcfg := workingConfig(cfg, maxInt(x.exponent, sdOf(x))+4)
	length := len(x.coefficient)

	var k int
	var n Decimal
	ctx := NewContext(wcfg)
	if length < 32 {
		k = ceilDiv(length, 3)
		n = ctx.NewFromFloat(1 / tinyPow(4, k))
	} else {
		k = 16
		n, _ = ctx.Parse("2.3283064365386962890625e-10")
	}

	x = taylorSeries(1, mul(x, n, wcfg, true), one(), true, wcfg)

	d8 := intDecimal(8)
	for i := k; i > 0; i-- {
		cosh2 := mul(x, x, wcfg, true)
		x = sub(one(), mul(cosh2, sub(d8, mul(cosh2, d8, wcfg, true), wcfg, true), wcfg, true), wcfg, true)
	}

	return finalise(x, pr, rm, true, cfg, true)
}

// Tanh returns the hyperbolic tangent of d.
func (d Decimal) Tanh() Decimal { return defaultContext.Tanh(d) }

// Tanh returns the hyperbolic tangent of x.
func (c *Context) Tanh(x Decimal) Decimal {
	cfg := c.config
	if !x.IsFinite() {
		if x.sign == signNaN {
			return NaN()
		}
		return Decimal{coefficient: []int{1}, exponent: 0, sign: x.sign}
	}
	if x.IsZero() {
		return x
	}

	wcfg := workingConfig(cfg, 7)
	wctx := NewContext(wcfg)
	return divide(wctx.Sinh(x), wctx.Cosh(x), cfg.Precision, false, cfg.Rounding, false, 0, cfg, true)
}

// Asinh returns the inverse hyperbolic sine of d.
func (d Decimal) Asinh() (Decimal, error) { return defaultContext.Asinh(d) }

// Asinh returns the inverse hyperbolic sine of x, as ln(x + sqrt(x^2+1)).
func (c *Context) Asinh(x Decimal) (Decimal, error) {
	cfg := c.config
	if !x.IsFinite() || x.IsZero() {
		return x, nil
	}

	wcfg := workingConfig(cfg, 2*maxInt(absInt(x.exponent), sdOf(x))+6)
	root := NewContext(wcfg).Sqrt(add(mul(x, x, wcfg, true), one(), wcfg, true))
	return NewContext(cfg).Ln(add(root, x, wcfg, true))
}

// Acosh returns the inverse hyperbolic cosine of d.
func (d Decimal) Acosh() (Decimal, error) { return defaultContext.Acosh(d) }

// Acosh returns the inverse hyperbolic cosine of x, as ln(x + sqrt(x^2-1)).
func (c *Context) Acosh(x Decimal) (Decimal, error) {
	cfg := c.config
	if x.Lte(one()) {
		if x.Eq(one()) {
			return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}, nil
		}
		return NaN(), nil
	}
	if !x.IsFinite() {
		return x, nil
	}

	wcfg := workingConfig(cfg, maxInt(absInt(x.exponent), sdOf(x))+4)
	root := NewContext(wcfg).Sqrt(sub(mul(x, x, wcfg, true), one(), wcfg, true))
	return NewContext(cfg).Ln(add(root, x, wcfg, true))
}

// Atanh returns the inverse hyperbolic tangent of d.
func (d Decimal) Atanh() (Decimal, error) { return defaultContext.Atanh(d) }

// Atanh returns the inverse hyperbolic tangent of x, as ln((1+x)/(1-x))/2.
func (c *Context) Atanh(x Decimal) (Decimal, error) {
	cfg := c.config
	pr, rm := cfg.Precision, cfg.Rounding

	if !x.IsFinite() {
		return NaN(), nil
	}
	if x.exponent >= 0 {
		switch {
		case abs(x).Eq(one()):
			return Inf(x.sign), nil
		case x.IsZero():
			return x, nil
		default:
			return NaN(), nil
		}
	}

	xsd := sdOf(x)
	if maxInt(xsd, pr) < 2*-x.exponent-1 {
		// x is so small that the result equals x to the requested precision.
		return finalise(x, pr, rm, true, cfg, true), nil
	}

	wpr := xsd - x.exponent
	wcfg := cfg
	wcfg.Precision = wpr
	q := divide(add(x, one(), wcfg, true), sub(one(), x, wcfg, true), wpr+pr, true, RoundDown, false, 0, wcfg, true)

	lcfg := workingConfig(cfg, 4)
	l, err := NewContext(lcfg).Ln(q)
	if err != nil {
		return Decimal{}, err
	}
	return mul(l, half(), cfg, true), nil
}

// Hypot returns the square root of the sum of the squares of its arguments,
// computed without an intermediate overflow.
func Hypot(values ...Decimal) Decimal { return defaultContext.Hypot(values...) }

// Hypot returns sqrt(sum of squares) under the Context's settings.
func (c *Context) Hypot(values ...Decimal) Decimal {
	cfg := c.config
	t := Decimal{coefficient: []int{0}, exponent: 0, sign: 1}

	for _, n := range values {
		if !n.IsFinite() {
			if n.sign != signNaN {
				return Inf(1)
			}
			t = n
		} else if t.IsFinite() {
			t = add(t, mul(n, n, cfg, false), cfg, false)
		}
	}

	return c.Sqrt(t)
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// sdOf is the significant-digit count of a finite value, and zero otherwise.
func sdOf(x Decimal) int {
	if x.coefficient == nil {
		return 0
	}
	return getPrecision(x.coefficient)
}

func maxInt(values ...int) int {
	m := values[0]
	for _, v := range values[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
