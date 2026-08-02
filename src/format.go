package decimal

import (
	"strconv"
	"strings"
)

// zeros returns a run of n zero characters, or the empty string for n <= 0.
func zeros(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("0", n)
}

// digitsToString writes a coefficient out as significant decimal digits, with
// no point and no exponent. Every limb but the first is zero-padded to seven
// digits, and trailing zeros of the last limb are dropped, which is what makes
// the digit count reflect the value's precision rather than its storage.
func digitsToString(d []int) string {
	if len(d) == 0 {
		return "0"
	}

	last := len(d) - 1
	w := d[0]
	var b strings.Builder

	if last > 0 {
		b.WriteString(strconv.Itoa(w))
		for i := 1; i < last; i++ {
			ws := strconv.Itoa(d[i])
			b.WriteString(zeros(logBase - len(ws)))
			b.WriteString(ws)
		}
		w = d[last]
		b.WriteString(zeros(logBase - len(strconv.Itoa(w))))
	} else if w == 0 {
		return "0"
	}

	for w%10 == 0 {
		w /= 10
	}
	b.WriteString(strconv.Itoa(w))
	return b.String()
}

// nonFiniteToString names a non-finite value without its sign, as decimal.js
// does before the caller prepends a minus.
func nonFiniteToString(x Decimal) string {
	if x.sign == signNaN {
		return "NaN"
	}
	return "Infinity"
}

// finiteToString renders x without its sign, in exponential notation if isExp
// is set. A non-zero sd pads the result out to that many digits, which is how
// toFixed, toExponential and toPrecision keep trailing zeros that the
// coefficient does not store.
func finiteToString(x Decimal, isExp bool, sd int) string {
	if !x.IsFinite() {
		return nonFiniteToString(x)
	}

	e := x.exponent
	str := digitsToString(x.coefficient)
	length := len(str)

	switch {
	case isExp:
		if k := sd - length; sd != 0 && k > 0 {
			str = str[:1] + "." + str[1:] + zeros(k)
		} else if length > 1 {
			str = str[:1] + "." + str[1:]
		}
		if e < 0 {
			str += "e" + strconv.Itoa(e)
		} else {
			str += "e+" + strconv.Itoa(e)
		}
	case e < 0:
		str = "0." + zeros(-e-1) + str
		if k := sd - length; sd != 0 && k > 0 {
			str += zeros(k)
		}
	case e >= length:
		str += zeros(e + 1 - length)
		if k := sd - e - 1; sd != 0 && k > 0 {
			str += "." + zeros(k)
		}
	default:
		if k := e + 1; k < length {
			str = str[:k] + "." + str[k:]
		}
		if k := sd - length; sd != 0 && k > 0 {
			if e+1 == length {
				str += "."
			}
			str += zeros(k)
		}
	}

	return str
}

// useExponential reports whether a value with exponent e is printed in
// exponential notation under cfg.
func useExponential(e int, cfg Config) bool {
	return e <= cfg.ToExpNeg || e >= cfg.ToExpPos
}

// String returns d in the notation decimal.js's toString produces under the
// default context: exponential outside the configured exponent thresholds,
// positional within them.
//
// Negative zero prints as "0" here, as it does in decimal.js's toString; only
// ValueOf keeps its sign.
func (d Decimal) String() string { return defaultContext.String(d) }

// String returns x as decimal.js's toString would under the Context's settings.
func (c *Context) String(x Decimal) string {
	str := finiteToString(x, useExponential(x.exponent, c.config), 0)
	if x.IsNegative() && !x.IsZero() {
		return "-" + str
	}
	return str
}

// ValueOf returns d as decimal.js's valueOf and toJSON produce it. It differs
// from String in one case: negative zero keeps its sign, printing as "-0".
func (d Decimal) ValueOf() string { return defaultContext.ValueOf(d) }

// ValueOf returns x as decimal.js's valueOf would under the Context's settings.
func (c *Context) ValueOf(x Decimal) string {
	str := finiteToString(x, useExponential(x.exponent, c.config), 0)
	if x.IsNegative() {
		return "-" + str
	}
	return str
}

// MarshalJSON encodes d as a JSON string, matching decimal.js's toJSON.
func (d Decimal) MarshalJSON() ([]byte, error) {
	return strconv.AppendQuote(nil, d.ValueOf()), nil
}

// ToFixed returns d in positional notation with dp digits after the point,
// rounded with the default context's rounding mode.
func (d Decimal) ToFixed(dp int) (string, error) {
	return defaultContext.ToFixed(d, dp, defaultContext.config.Rounding)
}

// ToFixed returns x in positional notation with dp digits after the point.
// Unlike String it never uses exponential notation.
func (c *Context) ToFixed(x Decimal, dp int, rm RoundingMode) (string, error) {
	if dp < 0 || dp > maxDigits {
		return "", wrapInvalidArgument("decimal places", dp)
	}
	if rm < RoundUp || rm > RoundHalfFloor {
		return "", wrapInvalidArgument("rounding mode", int(rm))
	}

	y := finalise(x, dp+x.exponent+1, rm, false, c.config, true)
	str := finiteToString(y, false, dp+y.exponent+1)

	// The sign is decided by the value before rounding, so that a small
	// negative value rounding to zero still prints as negative.
	if x.IsNegative() && !x.IsZero() {
		return "-" + str, nil
	}
	return str, nil
}

// ToExponential returns d in exponential notation with dp digits after the
// point, rounded with the default context's rounding mode.
func (d Decimal) ToExponential(dp int) (string, error) {
	return defaultContext.ToExponential(d, dp, defaultContext.config.Rounding)
}

// ToExponential returns x in exponential notation with dp digits after the
// point.
func (c *Context) ToExponential(x Decimal, dp int, rm RoundingMode) (string, error) {
	if dp < 0 || dp > maxDigits {
		return "", wrapInvalidArgument("decimal places", dp)
	}
	if rm < RoundUp || rm > RoundHalfFloor {
		return "", wrapInvalidArgument("rounding mode", int(rm))
	}

	y := finalise(x, dp+1, rm, false, c.config, true)
	str := finiteToString(y, true, dp+1)

	if y.IsNegative() && !y.IsZero() {
		return "-" + str, nil
	}
	return str, nil
}

// ToPrecision returns d to sd significant digits, rounded with the default
// context's rounding mode.
func (d Decimal) ToPrecision(sd int) (string, error) {
	return defaultContext.ToPrecision(d, sd, defaultContext.config.Rounding)
}

// ToPrecision returns x to sd significant digits, in exponential notation if
// the value needs more integer digits than that or falls below the configured
// negative-exponent threshold.
func (c *Context) ToPrecision(x Decimal, sd int, rm RoundingMode) (string, error) {
	if sd < 1 || sd > maxDigits {
		return "", wrapInvalidArgument("significant digits", sd)
	}
	if rm < RoundUp || rm > RoundHalfFloor {
		return "", wrapInvalidArgument("rounding mode", int(rm))
	}

	y := finalise(x, sd, rm, false, c.config, true)
	str := finiteToString(y, sd <= y.exponent || y.exponent <= c.config.ToExpNeg, sd)

	if y.IsNegative() && !y.IsZero() {
		return "-" + str, nil
	}
	return str, nil
}

// StringNoExponent returns d in positional notation regardless of the exponent
// thresholds, which is decimal.js's toFixed with no argument.
func (d Decimal) StringNoExponent() string { return defaultContext.StringNoExponent(d) }

// StringNoExponent returns x in positional notation regardless of the exponent
// thresholds.
func (c *Context) StringNoExponent(x Decimal) string {
	str := finiteToString(x, false, 0)
	if x.IsNegative() && !x.IsZero() {
		return "-" + str
	}
	return str
}

// StringExponent returns d in exponential notation regardless of the exponent
// thresholds, which is decimal.js's toExponential with no argument.
func (d Decimal) StringExponent() string { return defaultContext.StringExponent(d) }

// StringExponent returns x in exponential notation regardless of the exponent
// thresholds.
func (c *Context) StringExponent(x Decimal) string {
	str := finiteToString(x, true, 0)
	if x.IsNegative() && !x.IsZero() {
		return "-" + str
	}
	return str
}
