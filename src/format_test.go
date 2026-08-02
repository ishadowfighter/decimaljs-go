package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestFormattingAgainstOriginal checks every formatting method against
// decimal.js's own output, across six configurations that move the exponent
// thresholds as well as the precision and rounding mode.
func TestFormattingAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/strings.txt")
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
			t.Fatalf("strings.txt line %d: got %d fields, want 8", line, len(fields))
		}

		op, value := fields[0], fields[1]
		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[3]))
		cfg.ToExpNeg = atoiTest(t, fields[4])
		cfg.ToExpPos = atoiTest(t, fields[5])
		arg := atoiTest(t, fields[6])
		want := fields[7]

		ctx := NewContext(cfg)
		x, err := ctx.Parse(value)
		if err != nil {
			t.Fatalf("strings.txt line %d: Parse(%q): %v", line, value, err)
		}

		got, err := formatWith(ctx, x, op, arg)
		if err != nil {
			t.Errorf("%s(%s, arg=%d): unexpected error: %v", op, value, arg, err)
			continue
		}
		if got != want {
			t.Errorf("%s(%s, arg=%d) with toExpNeg=%d toExpPos=%d precision=%d rounding=%d = %q, want %q",
				op, value, arg, cfg.ToExpNeg, cfg.ToExpPos, cfg.Precision, cfg.Rounding, got, want)
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked == 0 {
		t.Fatal("strings.txt contained no cases")
	}
	t.Logf("checked %d formatting cases against decimal.js", checked)
}

// formatWith dispatches one testdata row. An arg below zero means the method
// was called with no argument, which for three of them is a distinct code path
// in the original rather than a default value.
func formatWith(c *Context, x Decimal, op string, arg int) (string, error) {
	switch op {
	case "toString":
		return c.String(x), nil
	case "valueOf":
		return c.ValueOf(x), nil
	case "toFixed":
		if arg < 0 {
			return c.StringNoExponent(x), nil
		}
		return c.ToFixed(x, arg, c.Config().Rounding)
	case "toExponential":
		if arg < 0 {
			return c.StringExponent(x), nil
		}
		return c.ToExponential(x, arg, c.Config().Rounding)
	case "toPrecision":
		if arg < 0 {
			return c.String(x), nil
		}
		return c.ToPrecision(x, arg, c.Config().Rounding)
	}
	return "", wrapInvalidArgument("unknown formatting op in test data", 0)
}

// TestMarshalJSON checks the JSON encoding, which decimal.js defines as its
// valueOf output and which therefore keeps the sign of negative zero.
func TestMarshalJSON(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"1.5", `"1.5"`},
		{"-0", `"-0"`},
		{"NaN", `"NaN"`},
		{"-Infinity", `"-Infinity"`},
	} {
		got, err := mustParse(t, c.in).MarshalJSON()
		if err != nil {
			t.Errorf("MarshalJSON(%q): %v", c.in, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("MarshalJSON(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestFormattingArgumentValidation covers the arguments decimal.js rejects.
func TestFormattingArgumentValidation(t *testing.T) {
	c := NewContext(DefaultConfig())
	x := mustParse(t, "1.5")

	if _, err := c.ToFixed(x, -1, RoundHalfUp); err == nil {
		t.Error("ToFixed with dp of -1 succeeded, want an error")
	}
	if _, err := c.ToExponential(x, -1, RoundHalfUp); err == nil {
		t.Error("ToExponential with dp of -1 succeeded, want an error")
	}
	if _, err := c.ToPrecision(x, 0, RoundHalfUp); err == nil {
		t.Error("ToPrecision with sd of 0 succeeded, want an error")
	}
}
