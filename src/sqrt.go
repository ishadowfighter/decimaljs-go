package decimal

import (
	"math"
	"strconv"
	"strings"
)

// half is the constant the Newton-Raphson step multiplies by.
func half() Decimal { return Decimal{coefficient: []int{5000000}, exponent: -1, sign: 1} }

// Sqrt returns the square root of d, rounded to the default context's
// precision.
func (d Decimal) Sqrt() Decimal { return defaultContext.Sqrt(d) }

// Sqrt returns the square root of x.
//
// A float64 provides the initial estimate, refined by Newton-Raphson at three
// guard digits. The iteration stops when two successive estimates agree to the
// working precision, except near a rounding boundary — four rounding digits of
// 9999, or 4999 on the first pass — where the last digit may be off by one and
// the working precision is extended instead.
func (c *Context) Sqrt(x Decimal) Decimal {
	cfg := c.config

	if x.sign != 1 || x.coefficient == nil || x.coefficient[0] == 0 {
		switch {
		case x.sign == signNaN, x.sign < 0 && (x.coefficient == nil || x.coefficient[0] != 0):
			return NaN()
		case x.coefficient != nil:
			// Zero, of either sign.
			return x
		default:
			return Inf(1)
		}
	}

	r := sqrtEstimate(x, cfg)

	pr := cfg.Precision
	sd := pr + 3
	inexact := false
	repeated := false

	for {
		t := r
		q := divide(x, t, sd+2, true, RoundDown, false, 0, cfg, false)
		r = mul(add(t, q, cfg, false), half(), cfg, false)

		ts := digitsToString(t.coefficient)
		rs := digitsToString(r.coefficient)
		if truncateTo(ts, sd) != truncateTo(rs, sd) {
			continue
		}

		n := sliceDigits(rs, sd-3, sd+1)

		if n == "9999" || !repeated && n == "4999" {
			// Approaching a rounding boundary. On the first pass the nines may
			// be an exact result in disguise, so check that before widening.
			if !repeated {
				t = finalise(t, pr+1, RoundUp, false, cfg, false)
				if mul(t, t, cfg, false).Eq(x) {
					r = t
					break
				}
			}
			sd += 4
			repeated = true
			continue
		}

		// Rounding digits of nothing, 0{0,4} or 50{0,3} may still be exact.
		if digitsAllZero(n) || digitsAllZero(n[1:]) && strings.HasPrefix(n, "5") {
			r = finalise(r, pr+1, RoundDown, false, cfg, false)
			inexact = !mul(r, r, cfg, false).Eq(x)
		}
		break
	}

	return finalise(r, pr, cfg.Rounding, inexact, cfg, true)
}

// sqrtEstimate produces the starting value for the iteration: a float64 square
// root where the value is in range, and otherwise the root of the digits with
// the exponent halved by hand.
func sqrtEstimate(x Decimal, cfg Config) Decimal {
	ctx := NewContext(cfg)

	if s := math.Sqrt(x.Float64()); s != 0 && !math.IsInf(s, 0) {
		r, _ := ctx.Parse(strconv.FormatFloat(s, 'g', -1, 64))
		return r
	}

	n := digitsToString(x.coefficient)
	e := x.exponent
	if (len(n)+e)%2 == 0 {
		n += "0"
	}

	s, _ := strconv.ParseFloat(n, 64)
	s = math.Sqrt(s)

	e = floorDiv(e+1, 2)
	if e2 := x.exponent; e2 < 0 || e2%2 != 0 {
		e--
	}

	var text string
	if math.IsInf(s, 0) {
		text = "5e" + itoa(e)
	} else {
		mantissa := strconv.FormatFloat(s, 'e', -1, 64)
		text = mantissa[:strings.IndexByte(mantissa, 'e')+1] + itoa(e)
	}
	r, _ := ctx.Parse(text)
	return r
}

// truncateTo returns the first n characters of s, or all of it.
func truncateTo(s string, n int) string {
	if n >= len(s) {
		return s
	}
	return s[:n]
}

// sliceDigits returns s[from:to] clamped to the string, as JavaScript's slice
// does rather than panicking.
func sliceDigits(s string, from, to int) string {
	if from < 0 {
		from = 0
	}
	if from > len(s) {
		return ""
	}
	if to > len(s) {
		to = len(s)
	}
	return s[from:to]
}

// digitsAllZero reports whether s is empty or all zeros, standing in for
// JavaScript's `!+n` on a run of digits.
func digitsAllZero(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}
