package decimal

import (
	"math"
	"strconv"
	"strings"
)

// isDecimalString reports whether s matches decimal.js's `isDecimal` grammar:
// digits with an optional fraction, or a bare fraction, with an optional
// exponent part. The sign has already been stripped by the caller.
//
//	/^(\d+(\.\d*)?|\.\d+)(e[+-]?\d+)?$/i
func isDecimalString(s string) bool {
	i := 0
	intDigits := 0
	for i < len(s) && isDigit(s[i]) {
		i++
		intDigits++
	}
	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
			fracDigits++
		}
		// A decimal point with no integer digits needs fraction digits.
		if intDigits == 0 && fracDigits == 0 {
			return false
		}
	} else if intDigits == 0 {
		return false
	}
	if i == len(s) {
		return true
	}
	if s[i] != 'e' && s[i] != 'E' {
		return false
	}
	i++
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	if i == len(s) {
		return false
	}
	for ; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// parseDecimalString builds the coefficient and exponent of a value written in
// decimal.js's decimal grammar. sign is applied by the caller; s carries no
// sign. It is a transliteration of decimal.js's parseDecimal, including its
// habit of stripping the point and then treating the whole thing as an integer
// string with a computed exponent.
//
// applyLimits corresponds to decimal.js's `external` flag: overflow to Infinity
// and underflow to zero are applied for values built for the caller, and
// skipped for values built inside a calculation.
func parseDecimalString(sign int, s string, cfg Config, applyLimits bool) Decimal {
	// e ends up as the base-10 exponent of the first digit of s, counted as
	// "digits before the point"; it is decremented to decimal.js's `.e`
	// convention at the end.
	e := strings.IndexByte(s, '.')
	if e > -1 {
		s = s[:e] + s[e+1:]
	}

	if i := indexAnyByte(s, 'e', 'E'); i > 0 {
		if e < 0 {
			e = i
		}
		// The exponent part is guaranteed well-formed by the grammar check,
		// but it can be far outside the range of an int; saturate rather than
		// wrap, since anything that large overflows or underflows anyway.
		e += parseExponentPart(s[i+1:])
		s = s[:i]
	} else if e < 0 {
		e = len(s)
	}

	// Leading zeros.
	lead := 0
	for lead < len(s) && s[lead] == '0' {
		lead++
	}

	// Trailing zeros.
	end := len(s)
	for end > 0 && s[end-1] == '0' {
		end--
	}

	if lead >= end {
		// The value is zero; decimal.js keeps the sign but normalises the
		// exponent and coefficient.
		return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}
	}

	digits := s[lead:end]
	e = e - lead - 1

	if applyLimits {
		if e > cfg.MaxE {
			return Decimal{coefficient: nil, exponent: 0, sign: sign}
		}
		if e < cfg.MinE {
			return Decimal{coefficient: []int{0}, exponent: 0, sign: sign}
		}
	}

	return Decimal{coefficient: packDigits(digits, e), exponent: e, sign: sign}
}

// packDigits splits a run of significant decimal digits into base-1e7 limbs,
// aligned the way decimal.js aligns them: the last limb is left-aligned within
// its seven digits, and the first limb is short so that limb boundaries fall on
// multiples of seven counting from the decimal point rather than from the start
// of the digit string. That alignment is observable, since the vendored tests
// assert on the limb array directly.
func packDigits(digits string, e int) []int {
	length := len(digits)

	// i is the width of the first limb.
	i := (e + 1) % logBase
	if e < 0 {
		i += logBase
	}

	limbs := make([]int, 0, length/logBase+2)
	if i < length {
		if i > 0 {
			limbs = append(limbs, atoi(digits[:i]))
		}
		for stop := length - logBase; i < stop; i += logBase {
			limbs = append(limbs, atoi(digits[i:i+logBase]))
		}
		digits = digits[i:]
		i = logBase - len(digits)
	} else {
		i -= length
	}

	// Right-pad the final limb with zeros so it occupies the full width.
	return append(limbs, atoi(digits+strings.Repeat("0", i)))
}

// atoi converts a run of at most seven decimal digits. The grammar has already
// been validated, so a failure here is a bug in the port rather than bad input.
func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// parseExponentPart reads the digits after `e`, saturating instead of
// overflowing. JavaScript would produce ±Infinity here and the value would then
// overflow or underflow; saturating at a magnitude far beyond expLimit reaches
// the same outcome without wrapping.
func parseExponentPart(s string) int {
	const saturate = 1 << 60
	neg := false
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
		if n > saturate {
			n = saturate
			break
		}
	}
	if neg {
		return -n
	}
	return n
}

func indexAnyByte(s string, a, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == a || s[i] == b {
			return i
		}
	}
	return -1
}

// NaN returns the NaN value. decimal.js stores it as a null coefficient with a
// NaN sign; here the sign is signNaN.
func NaN() Decimal { return Decimal{sign: signNaN} }

// Inf returns +Infinity for a non-negative sign and -Infinity otherwise.
func Inf(sign int) Decimal {
	if sign < 0 {
		return Decimal{sign: -1}
	}
	return Decimal{sign: 1}
}

// IsNaN reports whether d is NaN.
func (d Decimal) IsNaN() bool { return d.coefficient == nil && d.sign == signNaN }

// IsInf reports whether d is +Infinity or -Infinity.
func (d Decimal) IsInf() bool { return d.coefficient == nil && d.sign != signNaN }

// IsFinite reports whether d is a finite number, i.e. neither NaN nor an
// infinity.
func (d Decimal) IsFinite() bool { return d.coefficient != nil }

// stripUnderscores removes the separators decimal.js allows between digits,
// matching its `/(\d)_(?=\d)/g` replacement: an underscore only disappears when
// it sits between two digits, so "1_000" is 1000 while "_1", "1_" and "1__0"
// stay invalid.
func stripUnderscores(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '_' && i > 0 && isDigit(s[i-1]) && i+1 < len(s) && isDigit(s[i+1]) {
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Parse returns the Decimal represented by s, using the default context.
func Parse(s string) (Decimal, error) { return defaultContext.Parse(s) }

// Parse returns the Decimal represented by s. It accepts the same strings
// decimal.js accepts. Where decimal.js throws `[DecimalError] Invalid argument`,
// Parse returns an error wrapping ErrInvalidArgument.
func (c *Context) Parse(s string) (Decimal, error) {
	sign := 1
	if len(s) > 0 {
		switch s[0] {
		case '-':
			sign = -1
			s = s[1:]
		case '+':
			s = s[1:]
		}
	}
	if isDecimalString(s) {
		return parseDecimalString(sign, s, c.config, true), nil
	}

	// decimal.js's parseOther: separators first, then the two spelled-out
	// non-finite values. Both are matched exactly and case-sensitively, and the
	// sign has already been consumed, so "-Infinity" arrives here as
	// "Infinity" while "infinity" and "nan" stay invalid.
	if strings.ContainsRune(s, '_') {
		if stripped := stripUnderscores(s); isDecimalString(stripped) {
			return parseDecimalString(sign, stripped, c.config, true), nil
		}
	} else {
		switch s {
		case "Infinity":
			return Inf(sign), nil
		case "NaN":
			return NaN(), nil
		}
	}

	return parseRadixLiteral(sign, s, c.config)
}

// NewFromFloat returns the Decimal equal to the shortest decimal representation
// of f, using the default context.
func NewFromFloat(f float64) Decimal { return defaultContext.NewFromFloat(f) }

// NewFromFloat returns the Decimal equal to f. Like decimal.js's number
// constructor it converts through the shortest string that round-trips to f, so
// New(0.1) is exactly 0.1 rather than the binary value 0.1 denotes. Negative
// zero keeps its sign, and the non-finite floats map to the non-finite
// Decimals.
func (c *Context) NewFromFloat(f float64) Decimal {
	switch {
	case math.IsNaN(f):
		return NaN()
	case math.IsInf(f, 0):
		return Inf(sgn(f))
	case f == 0:
		return Decimal{coefficient: []int{0}, exponent: 0, sign: sgn(math.Copysign(1, f))}
	}

	sign := 1
	if f < 0 {
		sign = -1
		f = -f
	}
	// 'g' with a precision of -1 is the shortest representation that parses
	// back to f, which is exactly what JavaScript's Number-to-string
	// conversion produces. The two disagree on when to switch to exponential
	// notation, but not on the digits, and parseDecimalString accepts both
	// spellings.
	return parseDecimalString(sign, strconv.FormatFloat(f, 'g', -1, 64), c.config, true)
}

func sgn(f float64) int {
	if math.Signbit(f) {
		return -1
	}
	return 1
}

// NewFromInt returns the Decimal equal to n.
func NewFromInt(n int64) Decimal { return defaultContext.NewFromInt(n) }

// NewFromInt returns the Decimal equal to n. decimal.js only accepts numbers up
// to Number.MAX_SAFE_INTEGER exactly, but an int64 argument carries no such
// ambiguity, so the full range is converted exactly.
func (c *Context) NewFromInt(n int64) Decimal {
	sign := 1
	if n < 0 {
		sign = -1
	}
	// strconv handles math.MinInt64, which negating in place would not.
	s := strconv.FormatInt(n, 10)
	if sign < 0 {
		s = s[1:]
	}
	return parseDecimalString(sign, s, c.config, true)
}

// defaultContext backs the package-level constructors and carries the settings
// that decimal.js keeps on its constructor object.
var defaultContext = NewContext(DefaultConfig())
