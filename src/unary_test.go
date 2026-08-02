package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// runUnaryCases checks apply against decimal.js's results for a single-operand
// method, across four precisions and all nine rounding modes.
func runUnaryCases(t *testing.T, file string, apply func(c *Context, x Decimal, arg int) (Decimal, error)) {
	t.Helper()

	f, err := os.Open("testdata/" + file)
	if err != nil {
		t.Fatalf("open test data: %v", err)
	}
	defer f.Close()

	checked := 0
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for line := 1; scan.Scan(); line++ {
		text := scan.Text()
		if text == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 8 {
			t.Fatalf("%s line %d: got %d fields, want 8", file, line, len(fields))
		}
		op, value := fields[0], fields[1]
		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[3]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		arg := atoiTest(t, fields[4])
		wantLimbs := parseLimbs(t, fields[5])
		wantExp, wantSign := fields[6], fields[7]

		ctx := NewContext(cfg)
		x, err := ctx.Parse(value)
		if err != nil {
			t.Fatalf("%s line %d: Parse(%q): %v", file, line, value, err)
		}

		got, err := apply(ctx, x, arg)
		if err != nil {
			t.Errorf("%s(%s, %d): unexpected error: %v", op, value, arg, err)
			continue
		}
		if !matchesExpectation(got, wantLimbs, wantExp, wantSign) {
			t.Errorf("%s(%s, arg=%d) at precision %d rounding %d = d:%v e:%d s:%d, want d:%v e:%s s:%s",
				op, value, arg, cfg.Precision, cfg.Rounding,
				got.Coefficient(), got.Exponent(), got.Sign(), wantLimbs, wantExp, wantSign)
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked == 0 {
		t.Fatalf("%s contained no cases", file)
	}
	t.Logf("checked %d cases against decimal.js", checked)
}

func noArg(f func(c *Context, x Decimal) Decimal) func(*Context, Decimal, int) (Decimal, error) {
	return func(c *Context, x Decimal, _ int) (Decimal, error) { return f(c, x), nil }
}

func TestAbsAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "abs.txt", noArg((*Context).Abs))
}

func TestNegAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "neg.txt", noArg((*Context).Neg))
}

func TestRoundAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "round.txt", noArg((*Context).Round))
}

func TestFloorAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "floor.txt", noArg((*Context).Floor))
}

func TestCeilAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "ceil.txt", noArg((*Context).Ceil))
}

func TestTruncAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "trunc.txt", noArg((*Context).Trunc))
}

func TestToDecimalPlacesAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "toDP.txt", func(c *Context, x Decimal, arg int) (Decimal, error) {
		return c.ToDecimalPlaces(x, arg, c.Config().Rounding)
	})
}

func TestToSignificantDigitsAgainstOriginal(t *testing.T) {
	runUnaryCases(t, "toSD.txt", func(c *Context, x Decimal, arg int) (Decimal, error) {
		return c.ToSignificantDigits(x, arg, c.Config().Rounding)
	})
}

// TestRoundingArgumentValidation covers the places decimal.js throws a
// DecimalError for an out-of-range argument.
func TestRoundingArgumentValidation(t *testing.T) {
	c := NewContext(DefaultConfig())
	x := mustParse(t, "1.5")

	if _, err := c.ToDecimalPlaces(x, -1, RoundHalfUp); err == nil {
		t.Error("ToDecimalPlaces with dp of -1 succeeded, want an error")
	}
	if _, err := c.ToSignificantDigits(x, 0, RoundHalfUp); err == nil {
		t.Error("ToSignificantDigits with sd of 0 succeeded, want an error")
	}
	if _, err := c.ToSignificantDigits(x, 5, RoundingMode(9)); err == nil {
		t.Error("ToSignificantDigits with rounding mode 9 succeeded, want an error")
	}
}
