package decimal

import "testing"

// The expected coefficients, exponents and signs below were taken from
// decimal.js itself, at its default configuration, since the vendored suite
// asserts against the internal representation and not only the printed value.
func TestParseInternals(t *testing.T) {
	cases := []struct {
		in    string
		limbs []int
		exp   int
		sign  int
	}{
		{"0", []int{0}, 0, 1},
		{"-0", []int{0}, 0, -1},
		{"1", []int{1}, 0, 1},
		{"-1", []int{1}, 0, -1},
		{"10", []int{10}, 1, 1},
		{"100", []int{100}, 2, 1},
		{"1e7", []int{1}, 7, 1},
		{"12345678", []int{1, 2345678}, 7, 1},
		{"0.1", []int{1000000}, -1, 1},
		{"-0.1", []int{1000000}, -1, -1},
		{"0.000001", []int{10}, -6, 1},
		{"1.5", []int{1, 5000000}, 0, 1},
		{"123456.7891011", []int{123456, 7891011}, 5, 1},
		{"9.622e-11", []int{9622}, -11, 1},
		{"1e-7", []int{1}, -7, 1},
		{"1e21", []int{1}, 21, 1},
		{"0.0000001234567890123", []int{1, 2345678, 9012300}, -7, 1},
		{"98765.43210987654321", []int{98765, 4321098, 7654321}, 4, 1},
		{"1e+15", []int{10}, 15, 1},
		{"-999.999", []int{999, 9990000}, 2, -1},
		{"+1.25", []int{1, 2500000}, 0, 1},
		{"000123.4500", []int{123, 4500000}, 2, 1},
		{".5", []int{5000000}, -1, 1},
		{"0.0", []int{0}, 0, 1},
		{"-0.000", []int{0}, 0, -1},
	}

	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error: %v", c.in, err)
			continue
		}
		if !equalLimbs(got.Coefficient(), c.limbs) || got.Exponent() != c.exp || got.Sign() != c.sign {
			t.Errorf("Parse(%q) = d:%v e:%d s:%d, want d:%v e:%d s:%d",
				c.in, got.Coefficient(), got.Exponent(), got.Sign(), c.limbs, c.exp, c.sign)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	// Strings decimal.js's decimal grammar refuses. Hexadecimal, binary and
	// octal literals are also refused for now: they are handled by a separate
	// path upstream that is not yet ported.
	for _, in := range []string{"", ".", "-", "+", "1.2.3", "1e", "1e+", "e5", "1 2", "1,2", "12e5.5", "0x10"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %v, want an error", in, got)
		}
	}
}

func TestParseExponentLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxE = 100
	cfg.MinE = -100
	c := NewContext(cfg)

	overflow, err := c.Parse("1e101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if overflow.Coefficient() != nil {
		t.Errorf("1e101 with maxE=100: got coefficient %v, want nil (Infinity)", overflow.Coefficient())
	}

	underflow, err := c.Parse("-1e-101")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !equalLimbs(underflow.Coefficient(), []int{0}) || underflow.Exponent() != 0 || underflow.Sign() != -1 {
		t.Errorf("-1e-101 with minE=-100: got d:%v e:%d s:%d, want d:[0] e:0 s:-1",
			underflow.Coefficient(), underflow.Exponent(), underflow.Sign())
	}
}

func TestNewFromInt(t *testing.T) {
	cases := []struct {
		in    int64
		limbs []int
		exp   int
		sign  int
	}{
		{0, []int{0}, 0, 1},
		{1, []int{1}, 0, 1},
		{-1, []int{1}, 0, -1},
		{12345678, []int{1, 2345678}, 7, 1},
		{-9007199254740991, []int{90, 719925, 4740991}, 15, -1},
	}
	for _, c := range cases {
		got := NewFromInt(c.in)
		if !equalLimbs(got.Coefficient(), c.limbs) || got.Exponent() != c.exp || got.Sign() != c.sign {
			t.Errorf("NewFromInt(%d) = d:%v e:%d s:%d, want d:%v e:%d s:%d",
				c.in, got.Coefficient(), got.Exponent(), got.Sign(), c.limbs, c.exp, c.sign)
		}
	}
}

func equalLimbs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
