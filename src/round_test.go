package decimal

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestFinaliseAgainstOriginal drives the rounding engine on its own, before
// anything depends on it. Every case in src/testdata/rounding.txt is
// decimal.js's own toSignificantDigits output — the thinnest wrapper it has
// around finalise — across 50 values, 12 significant-digit counts and all nine
// rounding modes.
func TestFinaliseAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/rounding.txt")
	if err != nil {
		t.Fatalf("open test data: %v", err)
	}
	defer f.Close()

	cfg := DefaultConfig()
	cfg.ToExpNeg = -9e15
	cfg.ToExpPos = 9e15

	checked := 0
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for line := 1; scan.Scan(); line++ {
		text := scan.Text()
		if text == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 6 {
			t.Fatalf("testdata line %d: got %d fields, want 6", line, len(fields))
		}
		value, sd, rm := fields[0], atoiTest(t, fields[1]), RoundingMode(atoiTest(t, fields[2]))
		wantLimbs := parseLimbs(t, fields[3])
		wantExp := atoiTest(t, fields[4])
		wantSign := atoiTest(t, fields[5])

		x, err := Parse(value)
		if err != nil {
			t.Fatalf("testdata line %d: Parse(%q): %v", line, value, err)
		}

		got := finalise(x, sd, rm, false, cfg, true)
		if !equalLimbs(got.Coefficient(), wantLimbs) || got.Exponent() != wantExp || got.Sign() != wantSign {
			t.Errorf("finalise(%s, sd=%d, rm=%d) = d:%v e:%d s:%d, want d:%v e:%d s:%d",
				value, sd, rm, got.Coefficient(), got.Exponent(), got.Sign(), wantLimbs, wantExp, wantSign)
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked < 5000 {
		t.Fatalf("only %d cases checked; the test data looks truncated", checked)
	}
	t.Logf("checked %d rounding cases against decimal.js", checked)
}

// TestFinaliseLeavesArgumentUnchanged guards the port's immutability promise:
// decimal.js rounds in place on a fresh copy, and the Go version must not write
// through to the caller's coefficient. The original suite devotes 3311
// assertions to this property.
func TestFinaliseLeavesArgumentUnchanged(t *testing.T) {
	x := mustParse(t, "123456.7891011")
	before := x.Coefficient()
	beforeExp, beforeSign := x.Exponent(), x.Sign()

	for sd := 1; sd <= 12; sd++ {
		for rm := RoundUp; rm <= RoundHalfFloor; rm++ {
			finalise(x, sd, rm, false, DefaultConfig(), true)
		}
	}

	if !equalLimbs(x.Coefficient(), before) || x.Exponent() != beforeExp || x.Sign() != beforeSign {
		t.Errorf("finalise mutated its argument: d:%v e:%d s:%d, want d:%v e:%d s:%d",
			x.Coefficient(), x.Exponent(), x.Sign(), before, beforeExp, beforeSign)
	}
}

// TestFinaliseExponentLimits covers the overflow and underflow that finalise
// applies after rounding, and the flag that suppresses them for intermediate
// values inside a calculation.
func TestFinaliseExponentLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxE = 5
	cfg.MinE = -5

	over := finalise(mustParse(t, "999999"), 1, RoundHalfUp, false, cfg, true)
	if over.IsFinite() {
		t.Errorf("rounding 999999 to 1 digit with maxE=5: got %v, want Infinity", over.Coefficient())
	}

	under := finalise(mustParse(t, "-1e-6"), 3, RoundHalfUp, false, cfg, true)
	if !under.IsZero() || under.Sign() != -1 {
		t.Errorf("rounding -1e-6 with minE=-5: got d:%v s:%d, want zero with sign -1",
			under.Coefficient(), under.Sign())
	}

	internal := finalise(mustParse(t, "999999"), 1, RoundHalfUp, false, cfg, false)
	if !internal.IsFinite() || internal.Exponent() != 6 {
		t.Errorf("with limits suppressed: got finite:%t e:%d, want finite:true e:6",
			internal.IsFinite(), internal.Exponent())
	}
}

func parseLimbs(t *testing.T, s string) []int {
	t.Helper()
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	limbs := make([]int, len(parts))
	for i, p := range parts {
		limbs[i] = atoiTest(t, p)
	}
	return limbs
}

func atoiTest(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("bad integer %q in test data: %v", s, err)
	}
	return n
}
