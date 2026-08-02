package decimal

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestMiscAgainstOriginal covers decimalPlaces, precision, toNearest, clamp,
// max, min and sum against decimal.js's own results.
func TestMiscAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/misc.txt")
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
		if len(fields) != 7 {
			t.Fatalf("misc.txt line %d: got %d fields, want 7", line, len(fields))
		}

		op := fields[0]
		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[1])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[2]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)
		a1, a2, a3, want := fields[3], fields[4], fields[5], fields[6]

		got, err := applyMisc(t, ctx, op, a1, a2, a3)
		if err != nil {
			t.Errorf("%s(%q, %q, %q): unexpected error: %v", op, a1, a2, a3, err)
			continue
		}
		if got != want {
			t.Errorf("%s(%q, %q, %q) at precision %d rounding %d = %s, want %s",
				op, a1, a2, a3, cfg.Precision, cfg.Rounding, got, want)
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked == 0 {
		t.Fatal("misc.txt contained no cases")
	}
	t.Logf("checked %d cases against decimal.js", checked)
}

// applyMisc dispatches one testdata row, rendering the result the way the
// generator rendered decimal.js's: a valueOf string for the methods returning a
// Decimal, and a plain number or "NaN" for the two that return counts.
func applyMisc(t *testing.T, c *Context, op, a1, a2, a3 string) (string, error) {
	t.Helper()

	parseList := func(s string) []Decimal {
		parts := strings.Fields(s)
		values := make([]Decimal, len(parts))
		for i, p := range parts {
			values[i] = mustParseCtx(t, c, p)
		}
		return values
	}

	switch op {
	case "dp":
		dp, ok := mustParseCtx(t, c, a1).DecimalPlaces()
		if !ok {
			return "NaN", nil
		}
		return strconv.Itoa(dp), nil
	case "sd":
		sd, ok := mustParseCtx(t, c, a1).SignificantDigits(a2 == "1")
		if !ok {
			return "NaN", nil
		}
		return strconv.Itoa(sd), nil
	case "toNearest":
		got, err := c.ToNearest(mustParseCtx(t, c, a1), mustParseCtx(t, c, a2), RoundingMode(atoiTest(t, a3)))
		if err != nil {
			return "", err
		}
		return c.ValueOf(got), nil
	case "clamp":
		got, err := c.Clamp(mustParseCtx(t, c, a1), mustParseCtx(t, c, a2), mustParseCtx(t, c, a3))
		if err != nil {
			return "", err
		}
		return c.ValueOf(got), nil
	case "max":
		return c.ValueOf(Max(parseList(a1)...)), nil
	case "min":
		return c.ValueOf(Min(parseList(a1)...)), nil
	case "sum":
		return c.ValueOf(c.Sum(parseList(a1)...)), nil
	}
	return "", wrapInvalidArgument("unknown op in test data", 0)
}

func mustParseCtx(t *testing.T, c *Context, s string) Decimal {
	t.Helper()
	d, err := c.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return d
}

// TestClampRejectsReversedBounds covers the one case decimal.js throws for.
func TestClampRejectsReversedBounds(t *testing.T) {
	c := NewContext(DefaultConfig())
	if _, err := c.Clamp(mustParse(t, "5"), mustParse(t, "10"), mustParse(t, "1")); err == nil {
		t.Error("Clamp with min greater than max succeeded, want an error")
	}
}
