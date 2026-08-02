package decimal

import "testing"

// The full 14x14 comparison matrix below is decimal.js's own cmp output,
// generated from the original library, so the port's handling of NaN, the
// infinities and signed zero is pinned against it rather than against
// assumption.
var cmpCases = []struct {
	a, b  string
	order int
	ok    bool
}{
	{"0", "0", 0, true},
	{"0", "-0", 0, true},
	{"0", "1", -1, true},
	{"0", "-1", 1, true},
	{"0", "0.1", -1, true},
	{"0", "0.10", -1, true},
	{"0", "1e21", -1, true},
	{"0", "1e-7", -1, true},
	{"0", "NaN", 0, false},
	{"0", "Infinity", -1, true},
	{"0", "-Infinity", 1, true},
	{"0", "9.99", -1, true},
	{"0", "-9.99", 1, true},
	{"0", "123456.7891011", -1, true},
	{"-0", "0", 0, true},
	{"-0", "-0", 0, true},
	{"-0", "1", -1, true},
	{"-0", "-1", 1, true},
	{"-0", "0.1", -1, true},
	{"-0", "0.10", -1, true},
	{"-0", "1e21", -1, true},
	{"-0", "1e-7", -1, true},
	{"-0", "NaN", 0, false},
	{"-0", "Infinity", -1, true},
	{"-0", "-Infinity", 1, true},
	{"-0", "9.99", -1, true},
	{"-0", "-9.99", 1, true},
	{"-0", "123456.7891011", -1, true},
	{"1", "0", 1, true},
	{"1", "-0", 1, true},
	{"1", "1", 0, true},
	{"1", "-1", 1, true},
	{"1", "0.1", 1, true},
	{"1", "0.10", 1, true},
	{"1", "1e21", -1, true},
	{"1", "1e-7", 1, true},
	{"1", "NaN", 0, false},
	{"1", "Infinity", -1, true},
	{"1", "-Infinity", 1, true},
	{"1", "9.99", -1, true},
	{"1", "-9.99", 1, true},
	{"1", "123456.7891011", -1, true},
	{"-1", "0", -1, true},
	{"-1", "-0", -1, true},
	{"-1", "1", -1, true},
	{"-1", "-1", 0, true},
	{"-1", "0.1", -1, true},
	{"-1", "0.10", -1, true},
	{"-1", "1e21", -1, true},
	{"-1", "1e-7", -1, true},
	{"-1", "NaN", 0, false},
	{"-1", "Infinity", -1, true},
	{"-1", "-Infinity", 1, true},
	{"-1", "9.99", -1, true},
	{"-1", "-9.99", 1, true},
	{"-1", "123456.7891011", -1, true},
	{"0.1", "0", 1, true},
	{"0.1", "-0", 1, true},
	{"0.1", "1", -1, true},
	{"0.1", "-1", 1, true},
	{"0.1", "0.1", 0, true},
	{"0.1", "0.10", 0, true},
	{"0.1", "1e21", -1, true},
	{"0.1", "1e-7", 1, true},
	{"0.1", "NaN", 0, false},
	{"0.1", "Infinity", -1, true},
	{"0.1", "-Infinity", 1, true},
	{"0.1", "9.99", -1, true},
	{"0.1", "-9.99", 1, true},
	{"0.1", "123456.7891011", -1, true},
	{"0.10", "0", 1, true},
	{"0.10", "-0", 1, true},
	{"0.10", "1", -1, true},
	{"0.10", "-1", 1, true},
	{"0.10", "0.1", 0, true},
	{"0.10", "0.10", 0, true},
	{"0.10", "1e21", -1, true},
	{"0.10", "1e-7", 1, true},
	{"0.10", "NaN", 0, false},
	{"0.10", "Infinity", -1, true},
	{"0.10", "-Infinity", 1, true},
	{"0.10", "9.99", -1, true},
	{"0.10", "-9.99", 1, true},
	{"0.10", "123456.7891011", -1, true},
	{"1e21", "0", 1, true},
	{"1e21", "-0", 1, true},
	{"1e21", "1", 1, true},
	{"1e21", "-1", 1, true},
	{"1e21", "0.1", 1, true},
	{"1e21", "0.10", 1, true},
	{"1e21", "1e21", 0, true},
	{"1e21", "1e-7", 1, true},
	{"1e21", "NaN", 0, false},
	{"1e21", "Infinity", -1, true},
	{"1e21", "-Infinity", 1, true},
	{"1e21", "9.99", 1, true},
	{"1e21", "-9.99", 1, true},
	{"1e21", "123456.7891011", 1, true},
	{"1e-7", "0", 1, true},
	{"1e-7", "-0", 1, true},
	{"1e-7", "1", -1, true},
	{"1e-7", "-1", 1, true},
	{"1e-7", "0.1", -1, true},
	{"1e-7", "0.10", -1, true},
	{"1e-7", "1e21", -1, true},
	{"1e-7", "1e-7", 0, true},
	{"1e-7", "NaN", 0, false},
	{"1e-7", "Infinity", -1, true},
	{"1e-7", "-Infinity", 1, true},
	{"1e-7", "9.99", -1, true},
	{"1e-7", "-9.99", 1, true},
	{"1e-7", "123456.7891011", -1, true},
	{"NaN", "0", 0, false},
	{"NaN", "-0", 0, false},
	{"NaN", "1", 0, false},
	{"NaN", "-1", 0, false},
	{"NaN", "0.1", 0, false},
	{"NaN", "0.10", 0, false},
	{"NaN", "1e21", 0, false},
	{"NaN", "1e-7", 0, false},
	{"NaN", "NaN", 0, false},
	{"NaN", "Infinity", 0, false},
	{"NaN", "-Infinity", 0, false},
	{"NaN", "9.99", 0, false},
	{"NaN", "-9.99", 0, false},
	{"NaN", "123456.7891011", 0, false},
	{"Infinity", "0", 1, true},
	{"Infinity", "-0", 1, true},
	{"Infinity", "1", 1, true},
	{"Infinity", "-1", 1, true},
	{"Infinity", "0.1", 1, true},
	{"Infinity", "0.10", 1, true},
	{"Infinity", "1e21", 1, true},
	{"Infinity", "1e-7", 1, true},
	{"Infinity", "NaN", 0, false},
	{"Infinity", "Infinity", 0, true},
	{"Infinity", "-Infinity", 1, true},
	{"Infinity", "9.99", 1, true},
	{"Infinity", "-9.99", 1, true},
	{"Infinity", "123456.7891011", 1, true},
	{"-Infinity", "0", -1, true},
	{"-Infinity", "-0", -1, true},
	{"-Infinity", "1", -1, true},
	{"-Infinity", "-1", -1, true},
	{"-Infinity", "0.1", -1, true},
	{"-Infinity", "0.10", -1, true},
	{"-Infinity", "1e21", -1, true},
	{"-Infinity", "1e-7", -1, true},
	{"-Infinity", "NaN", 0, false},
	{"-Infinity", "Infinity", -1, true},
	{"-Infinity", "-Infinity", 0, true},
	{"-Infinity", "9.99", -1, true},
	{"-Infinity", "-9.99", -1, true},
	{"-Infinity", "123456.7891011", -1, true},
	{"9.99", "0", 1, true},
	{"9.99", "-0", 1, true},
	{"9.99", "1", 1, true},
	{"9.99", "-1", 1, true},
	{"9.99", "0.1", 1, true},
	{"9.99", "0.10", 1, true},
	{"9.99", "1e21", -1, true},
	{"9.99", "1e-7", 1, true},
	{"9.99", "NaN", 0, false},
	{"9.99", "Infinity", -1, true},
	{"9.99", "-Infinity", 1, true},
	{"9.99", "9.99", 0, true},
	{"9.99", "-9.99", 1, true},
	{"9.99", "123456.7891011", -1, true},
	{"-9.99", "0", -1, true},
	{"-9.99", "-0", -1, true},
	{"-9.99", "1", -1, true},
	{"-9.99", "-1", -1, true},
	{"-9.99", "0.1", -1, true},
	{"-9.99", "0.10", -1, true},
	{"-9.99", "1e21", -1, true},
	{"-9.99", "1e-7", -1, true},
	{"-9.99", "NaN", 0, false},
	{"-9.99", "Infinity", -1, true},
	{"-9.99", "-Infinity", 1, true},
	{"-9.99", "9.99", -1, true},
	{"-9.99", "-9.99", 0, true},
	{"-9.99", "123456.7891011", -1, true},
	{"123456.7891011", "0", 1, true},
	{"123456.7891011", "-0", 1, true},
	{"123456.7891011", "1", 1, true},
	{"123456.7891011", "-1", 1, true},
	{"123456.7891011", "0.1", 1, true},
	{"123456.7891011", "0.10", 1, true},
	{"123456.7891011", "1e21", -1, true},
	{"123456.7891011", "1e-7", 1, true},
	{"123456.7891011", "NaN", 0, false},
	{"123456.7891011", "Infinity", -1, true},
	{"123456.7891011", "-Infinity", 1, true},
	{"123456.7891011", "9.99", 1, true},
	{"123456.7891011", "-9.99", 1, true},
	{"123456.7891011", "123456.7891011", 0, true},
}

func TestCmp(t *testing.T) {
	for _, c := range cmpCases {
		x := mustParse(t, c.a)
		y := mustParse(t, c.b)
		order, ok := x.Cmp(y)
		if ok != c.ok || (ok && order != c.order) {
			t.Errorf("Parse(%q).Cmp(%q) = %d, %t; want %d, %t", c.a, c.b, order, ok, c.order, c.ok)
		}
	}
}

func TestComparisonPredicates(t *testing.T) {
	for _, c := range cmpCases {
		x := mustParse(t, c.a)
		y := mustParse(t, c.b)

		// Every comparison involving NaN is false, so the expectations fall
		// out of the ordering whenever ok is true and are false otherwise.
		want := map[string]bool{
			"Eq":  c.ok && c.order == 0,
			"Gt":  c.ok && c.order > 0,
			"Gte": c.ok && c.order >= 0,
			"Lt":  c.ok && c.order < 0,
			"Lte": c.ok && c.order <= 0,
		}
		got := map[string]bool{
			"Eq":  x.Eq(y),
			"Gt":  x.Gt(y),
			"Gte": x.Gte(y),
			"Lt":  x.Lt(y),
			"Lte": x.Lte(y),
		}
		for name := range want {
			if got[name] != want[name] {
				t.Errorf("Parse(%q).%s(%q) = %t, want %t", c.a, name, c.b, got[name], want[name])
			}
		}
	}
}

func TestPredicates(t *testing.T) {
	cases := []struct {
		in                                           string
		isInt, isZero, isNeg, isPos, isNaN, isFinite bool
		signum                                       int
	}{
		{"0", true, true, false, true, false, true, 0},
		{"-0", true, true, true, false, false, true, 0},
		{"1", true, false, false, true, false, true, 1},
		{"-1", true, false, true, false, false, true, -1},
		{"9.99", false, false, false, true, false, true, 1},
		{"-9.99", false, false, true, false, false, true, -1},
		{"1e21", true, false, false, true, false, true, 1},
		{"1e-7", false, false, false, true, false, true, 1},
		{"12345678", true, false, false, true, false, true, 1},
		{"123456.7891011", false, false, false, true, false, true, 1},
		{"NaN", false, false, false, false, true, false, 0},
		{"Infinity", false, false, false, true, false, false, 1},
		{"-Infinity", false, false, true, false, false, false, -1},
	}

	for _, c := range cases {
		x := mustParse(t, c.in)
		if x.IsInteger() != c.isInt {
			t.Errorf("Parse(%q).IsInteger() = %t, want %t", c.in, x.IsInteger(), c.isInt)
		}
		if x.IsZero() != c.isZero {
			t.Errorf("Parse(%q).IsZero() = %t, want %t", c.in, x.IsZero(), c.isZero)
		}
		if x.IsNegative() != c.isNeg {
			t.Errorf("Parse(%q).IsNegative() = %t, want %t", c.in, x.IsNegative(), c.isNeg)
		}
		if x.IsPositive() != c.isPos {
			t.Errorf("Parse(%q).IsPositive() = %t, want %t", c.in, x.IsPositive(), c.isPos)
		}
		if x.IsNaN() != c.isNaN {
			t.Errorf("Parse(%q).IsNaN() = %t, want %t", c.in, x.IsNaN(), c.isNaN)
		}
		if x.IsFinite() != c.isFinite {
			t.Errorf("Parse(%q).IsFinite() = %t, want %t", c.in, x.IsFinite(), c.isFinite)
		}
		if x.Signum() != c.signum {
			t.Errorf("Parse(%q).Signum() = %d, want %d", c.in, x.Signum(), c.signum)
		}
	}
}

func mustParse(t *testing.T, s string) Decimal {
	t.Helper()
	d, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return d
}
