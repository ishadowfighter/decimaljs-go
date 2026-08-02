package decimal

import (
	"fmt"
	"math"
	"strings"
)

// numerals are the digit characters decimal.js accepts and emits for bases up
// to 16.
const numerals = "0123456789abcdef"

// numeralValue returns the value of a numeral character, or -1.
func numeralValue(c byte) int {
	return strings.IndexByte(numerals, c)
}

// convertBase reinterprets the digits of str, written in baseIn, as digits in
// baseOut, most significant first. It is decimal.js's convertBase: repeated
// multiply-and-carry over a little-endian accumulator, reversed at the end.
func convertBase(str string, baseIn, baseOut int) []int {
	arr := []int{0}

	for i := 0; i < len(str); i++ {
		for j := range arr {
			arr[j] *= baseIn
		}
		arr[0] += numeralValue(str[i])

		for j := 0; j < len(arr); j++ {
			if arr[j] > baseOut-1 {
				if j+1 == len(arr) {
					arr = append(arr, 0)
				}
				arr[j+1] += arr[j] / baseOut
				arr[j] %= baseOut
			}
		}
	}

	// Reverse into most-significant-first order.
	for i, j := 0, len(arr)-1; i < j; i, j = i+1, j-1 {
		arr[i], arr[j] = arr[j], arr[i]
	}
	return arr
}

// radixLiteral matches decimal.js's isHex, isBinary and isOctal patterns:
// a base prefix, digits with an optional fraction or a bare fraction, and an
// optional binary exponent introduced by p.
//
// It returns the radix, the digits with the prefix and exponent removed, and
// the binary exponent.
func radixLiteral(s string) (radix int, digits string, binExp int, ok bool) {
	if len(s) < 3 || s[0] != '0' {
		return 0, "", 0, false
	}

	var maxDigit byte
	switch s[1] {
	case 'x', 'X':
		radix, maxDigit = 16, 'f'
	case 'b', 'B':
		radix, maxDigit = 2, '1'
	case 'o', 'O':
		radix, maxDigit = 8, '7'
	default:
		return 0, "", 0, false
	}

	body := strings.ToLower(s[2:])

	// Split off the binary exponent part.
	if i := strings.IndexByte(body, 'p'); i >= 0 {
		expPart := body[i+1:]
		body = body[:i]
		if expPart == "" {
			return 0, "", 0, false
		}
		rest := expPart
		if rest[0] == '+' || rest[0] == '-' {
			rest = rest[1:]
		}
		if rest == "" {
			return 0, "", 0, false
		}
		for i := 0; i < len(rest); i++ {
			if !isDigit(rest[i]) {
				return 0, "", 0, false
			}
		}
		binExp = parseExponentPart(expPart)
	}

	valid := func(c byte) bool {
		if radix == 16 {
			return isDigit(c) || c >= 'a' && c <= 'f'
		}
		return c >= '0' && c <= maxDigit
	}

	intDigits, fracDigits := 0, 0
	i := 0
	for i < len(body) && valid(body[i]) {
		i++
		intDigits++
	}
	if i < len(body) && body[i] == '.' {
		i++
		for i < len(body) && valid(body[i]) {
			i++
			fracDigits++
		}
		if intDigits == 0 && fracDigits == 0 {
			return 0, "", 0, false
		}
	} else if intDigits == 0 {
		return 0, "", 0, false
	}
	if i != len(body) {
		return 0, "", 0, false
	}

	return radix, body, binExp, true
}

// parseRadixLiteral builds a Decimal from a hexadecimal, binary or octal
// literal, as decimal.js's parseOther does.
//
// The digits are read as an integer in their own base and then divided by that
// base raised to the number of fraction digits, which restores the fraction
// exactly. A binary exponent, if present, is applied last.
func parseRadixLiteral(sign int, s string, cfg Config) (Decimal, error) {
	radix, body, binExp, ok := radixLiteral(s)
	if !ok {
		return Decimal{}, fmt.Errorf("%w: %s", ErrInvalidArgument, s)
	}

	var divisor Decimal
	isFloat := false
	if i := strings.IndexByte(body, '.'); i >= 0 {
		body = body[:i] + body[i+1:]
		fracDigits := len(body) - i
		isFloat = true

		// log10(16) is 1.21, log10(8) is 0.91: twice the fraction-digit count
		// is always enough working precision for the divisor.
		divisor = intPow(Decimal{coefficient: []int{radix}, exponent: digitCount(radix) - 1, sign: 1},
			int64(fracDigits), fracDigits*2, cfg)
	}

	xd := convertBase(body, radix, base)
	xe := len(xd) - 1

	for i := xe; i >= 0 && xd[i] == 0; i-- {
		xd = xd[:i]
	}
	if len(xd) == 0 {
		return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}, nil
	}

	x := Decimal{coefficient: xd, exponent: getBase10Exponent(xd, xe), sign: sign}

	// The conversion and the binary-exponent scaling are exact, so neither
	// rounds to precision nor applies the exponent limits.
	if isFloat {
		// Four decimal digits per input digit is always enough to divide
		// exactly, whichever of the three bases this is.
		x = divide(x, divisor, len(body)*4, true, RoundHalfDown, false, 0, cfg, false)
	}
	if binExp != 0 {
		x = mul(x, powerOfTwo(binExp, cfg), cfg, false)
	}

	return applyExponentLimits(x, cfg, true), nil
}

// powerOfTwo returns 2^p. Small exponents come straight from a float64, which
// holds them exactly, as decimal.js's Math.pow shortcut does; larger ones go
// through exponentiation by squaring.
func powerOfTwo(p int, cfg Config) Decimal {
	if p < 0 {
		if -p < 54 {
			return NewContext(cfg).NewFromFloat(math.Pow(2, float64(p)))
		}
		r := intPow(Decimal{coefficient: []int{2}, exponent: 0, sign: 1}, int64(-p), cfg.Precision, cfg)
		return divide(one(), r, 0, false, cfg.Rounding, false, 0, cfg, true)
	}
	if p < 54 {
		return NewContext(cfg).NewFromFloat(math.Pow(2, float64(p)))
	}
	r := intPow(Decimal{coefficient: []int{2}, exponent: 0, sign: 1}, int64(p), cfg.Precision, cfg)
	return finalise(r, cfg.Precision, cfg.Rounding, false, cfg, true)
}

// toStringBinary renders x in base 2, 8 or 16, as decimal.js's toStringBinary
// does. With sdSet the output is in the binary-exponent form ending in p+n,
// rounded to sd significant digits of that base; without it the value is
// written positionally at the context's precision.
//
// The value is converted through its decimal digits: the integer part is
// converted directly, and a fraction is restored by dividing by the
// corresponding power of the output base, which is where the inexact flag from
// division feeds the rounding decision.
func toStringBinary(x Decimal, baseOut, sd int, sdSet bool, rm RoundingMode, cfg Config) (string, error) {
	if sdSet {
		if sd < 1 || sd > maxDigits {
			return "", wrapInvalidArgument("significant digits", sd)
		}
		if rm < RoundUp || rm > RoundHalfFloor {
			return "", wrapInvalidArgument("rounding mode", int(rm))
		}
	} else {
		sd = cfg.Precision
		rm = cfg.Rounding
	}

	if !x.IsFinite() {
		str := nonFiniteToString(x)
		if x.sign < 0 {
			return "-" + str, nil
		}
		return str, nil
	}

	str := finiteToString(x, false, 0)
	pointAt := strings.IndexByte(str, '.')

	radix := baseOut
	if sdSet {
		// The exponent form is built in binary and regrouped afterwards, so the
		// requested digit count is converted into a count of bits.
		radix = 2
		switch baseOut {
		case 16:
			sd = sd*4 - 3
		case 8:
			sd = sd*3 - 2
		}
	}

	// A fraction is restored by dividing by radix^(fraction digits).
	var divisor Decimal
	if pointAt >= 0 {
		str = strings.Replace(str, ".", "", 1)
		y := Decimal{coefficient: []int{1}, exponent: len(str) - pointAt, sign: 1}
		y.coefficient = convertBase(finiteToString(y, false, 0), 10, radix)
		y.exponent = len(y.coefficient)
		divisor = y
	}

	xd := convertBase(str, 10, radix)
	e := len(xd)
	for length := len(xd); length > 0 && xd[length-1] == 0; length-- {
		xd = xd[:length-1]
	}

	if len(xd) == 0 || xd[0] == 0 {
		if sdSet {
			str = "0p+0"
		} else {
			str = "0"
		}
		return withBasePrefix(str, baseOut, x.sign), nil
	}

	roundUp := false
	if pointAt < 0 {
		e--
	} else {
		q, inexact := divideCore(Decimal{coefficient: xd, exponent: e, sign: x.sign},
			divisor, sd, true, rm, false, radix, cfg, false)
		xd = q.coefficient
		e = q.exponent
		roundUp = inexact
	}

	// The rounding digit is the first one beyond the requested count.
	rd, rdSet := 0, false
	if sd < len(xd) {
		rd, rdSet = xd[sd], true
	}
	half := radix / 2
	roundUp = roundUp || sd+1 < len(xd)

	if rm < RoundHalfUp {
		awayFromZero := RoundCeil
		if x.sign < 0 {
			awayFromZero = RoundFloor
		}
		roundUp = (rdSet || roundUp) && (rm == RoundUp || rm == awayFromZero)
	} else {
		halfAwayFromZero := RoundHalfCeil
		if x.sign < 0 {
			halfAwayFromZero = RoundHalfFloor
		}
		roundUp = rd > half || rd == half && rdSet &&
			(rm == RoundHalfUp || roundUp ||
				rm == RoundHalfEven && sd > 0 && xd[sd-1]&1 == 1 ||
				rm == halfAwayFromZero)
	}

	if sd < len(xd) {
		xd = xd[:sd]
	}
	for len(xd) < sd {
		xd = append(xd, 0)
	}

	if roundUp {
		// A carry can run all the way off the front, lengthening the value.
		for i := sd - 1; ; i-- {
			if i < 0 {
				e++
				xd = append([]int{1}, xd...)
				break
			}
			xd[i]++
			if xd[i] <= radix-1 {
				break
			}
			xd[i] = 0
		}
	}

	length := len(xd)
	for length > 0 && xd[length-1] == 0 {
		length--
	}

	var b strings.Builder
	for i := 0; i < length; i++ {
		b.WriteByte(numerals[xd[i]])
	}
	str = b.String()

	switch {
	case sdSet:
		if length > 1 {
			if baseOut == 16 || baseOut == 8 {
				// Regroup the bits into hexadecimal or octal digits, padding the
				// last group out so the regrouping is exact.
				group := 3
				if baseOut == 16 {
					group = 4
				}
				for length--; length%group != 0; length++ {
					str += "0"
				}
				xd = convertBase(str, radix, baseOut)
				length = len(xd)
				for length > 0 && xd[length-1] == 0 {
					length--
				}
				// The leading digit is always 1 here.
				var g strings.Builder
				g.WriteString("1.")
				for i := 1; i < length; i++ {
					g.WriteByte(numerals[xd[i]])
				}
				str = g.String()
			} else {
				str = str[:1] + "." + str[1:]
			}
		}
		if e < 0 {
			str += "p" + itoa(e)
		} else {
			str += "p+" + itoa(e)
		}
	case e < 0:
		str = "0." + zeros(-e-1) + str
	default:
		if e++; e > length {
			str += zeros(e - length)
		} else if e < length {
			str = str[:e] + "." + str[e:]
		}
	}

	return withBasePrefix(str, baseOut, x.sign), nil
}

// withBasePrefix adds the literal prefix for the base and the sign.
func withBasePrefix(str string, baseOut, sign int) string {
	switch baseOut {
	case 16:
		str = "0x" + str
	case 2:
		str = "0b" + str
	case 8:
		str = "0o" + str
	}
	if sign < 0 {
		return "-" + str
	}
	return str
}

// ToBinary returns d as a binary literal at the default context's precision.
func (d Decimal) ToBinary() string { return defaultContext.mustBase(d, 2) }

// ToOctal returns d as an octal literal at the default context's precision.
func (d Decimal) ToOctal() string { return defaultContext.mustBase(d, 8) }

// ToHexadecimal returns d as a hexadecimal literal at the default context's
// precision.
func (d Decimal) ToHexadecimal() string { return defaultContext.mustBase(d, 16) }

// mustBase renders without a digit count, where no argument can be invalid.
func (c *Context) mustBase(x Decimal, baseOut int) string {
	s, err := toStringBinary(x, baseOut, 0, false, c.config.Rounding, c.config)
	if err != nil {
		// Unreachable: the argument checks only run when a digit count is given.
		panic(err)
	}
	return s
}

// ToBinary returns x as a binary literal. With sdSet the binary-exponent form
// is used, rounded to sd significant binary digits.
func (c *Context) ToBinary(x Decimal, sd int, sdSet bool, rm RoundingMode) (string, error) {
	return toStringBinary(x, 2, sd, sdSet, rm, c.config)
}

// ToOctal returns x as an octal literal.
func (c *Context) ToOctal(x Decimal, sd int, sdSet bool, rm RoundingMode) (string, error) {
	return toStringBinary(x, 8, sd, sdSet, rm, c.config)
}

// ToHexadecimal returns x as a hexadecimal literal.
func (c *Context) ToHexadecimal(x Decimal, sd int, sdSet bool, rm RoundingMode) (string, error) {
	return toStringBinary(x, 16, sd, sdSet, rm, c.config)
}
