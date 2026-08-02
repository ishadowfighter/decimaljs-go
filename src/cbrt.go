package decimal

import (
	"math"
	"strconv"
	"strings"
)

// Cbrt returns the cube root of d, rounded to the default context's precision.
func (d Decimal) Cbrt() Decimal { return defaultContext.Cbrt(d) }

// Cbrt returns the cube root of x, refined by Halley's method from a float64
// estimate. The stopping rule is the one Sqrt uses, for the same reason.
func (c *Context) Cbrt(x Decimal) Decimal {
	cfg := c.config
	if !x.IsFinite() || x.IsZero() {
		return x
	}

	r := cbrtEstimate(x, cfg)

	pr := cfg.Precision
	sd := pr + 3
	inexact := false
	repeated := false

	for {
		t := r
		t3 := mul(mul(t, t, cfg, false), t, cfg, false)
		t3plusx := add(t3, x, cfg, false)
		r = divide(
			mul(add(t3plusx, x, cfg, false), t, cfg, false),
			add(t3plusx, t3, cfg, false),
			sd+2, true, RoundDown, false, 0, cfg, false)

		ts := digitsToString(t.coefficient)
		rs := digitsToString(r.coefficient)
		if truncateTo(ts, sd) != truncateTo(rs, sd) {
			continue
		}

		n := sliceDigits(rs, sd-3, sd+1)

		if n == "9999" || !repeated && n == "4999" {
			if !repeated {
				t = finalise(t, pr+1, RoundUp, false, cfg, false)
				if cube(t, cfg).Eq(x) {
					r = t
					break
				}
			}
			sd += 4
			repeated = true
			continue
		}

		if digitsAllZero(n) || digitsAllZero(n[1:]) && strings.HasPrefix(n, "5") {
			r = finalise(r, pr+1, RoundDown, false, cfg, false)
			inexact = !cube(r, cfg).Eq(x)
		}
		break
	}

	return finalise(r, pr, cfg.Rounding, inexact, cfg, true)
}

func cube(x Decimal, cfg Config) Decimal {
	return mul(mul(x, x, cfg, false), x, cfg, false)
}

// cbrtEstimate mirrors decimal.js's starting value, including its exponent
// bookkeeping for values a float64 cannot hold.
func cbrtEstimate(x Decimal, cfg Config) Decimal {
	ctx := NewContext(cfg)

	s := float64(x.sign) * math.Cbrt(math.Abs(x.Float64()))
	if s != 0 && !math.IsInf(s, 0) {
		r, _ := ctx.Parse(strconv.FormatFloat(s, 'g', -1, 64))
		return r
	}

	n := digitsToString(x.coefficient)
	e := x.exponent

	// Pad so the exponent is a multiple of three away from the digits.
	if k := (e - len(n) + 1) % 3; k != 0 {
		if k == 1 || k == -2 {
			n += "0"
		} else {
			n += "00"
		}
	}

	f, _ := strconv.ParseFloat(n, 64)
	f = math.Cbrt(f)

	want := 2
	if e < 0 {
		want = -1
	}
	e = floorDiv(e+1, 3)
	if x.exponent%3 == want {
		e--
	}

	var text string
	if math.IsInf(f, 0) {
		text = "5e" + itoa(e)
	} else {
		mantissa := strconv.FormatFloat(f, 'e', -1, 64)
		text = mantissa[:strings.IndexByte(mantissa, 'e')+1] + itoa(e)
	}
	r, _ := ctx.Parse(text)
	r.sign = x.sign
	return r
}
