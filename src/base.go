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
