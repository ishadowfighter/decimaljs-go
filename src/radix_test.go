package decimal

import "testing"

// Expectations from decimal.js for the non-decimal literal forms it accepts.
func TestParseRadixLiterals(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0x1", "1"},
		{"0xff", "255"},
		{"0x1.8", "1.5"},
		{"0b1101", "13"},
		{"0o777", "511"},
		{"0x1p8", "256"},
		{"0b1.1p-2", "0.375"},
		{"0xa.bp4", "171"},
		{"0X1F", "31"},
		{"0o0.4", "0.5"},
		{"0x1.fp-3", "0.2421875"},
		{"-0xff", "-255"},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if s := got.ValueOf(); s != c.want {
			t.Errorf("Parse(%q) = %s, want %s", c.in, s, c.want)
		}
	}

	for _, in := range []string{"0x", "0b", "0o", "0x2g", "0b2", "0o8", "0xp4", "0x1p", "0x1p+", "0x1.2.3"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", in)
		}
	}
}
