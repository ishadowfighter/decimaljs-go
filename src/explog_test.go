package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// runTranscendentalCases checks a one-operand function against decimal.js
// across five precisions and all nine rounding modes.
func runTranscendentalCases(t *testing.T, file string, apply func(*Context, Decimal) (string, error)) {
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
		if len(fields) != 4 {
			t.Fatalf("%s line %d: got %d fields, want 4", file, line, len(fields))
		}

		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[1])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[2]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		x, err := ctx.Parse(fields[0])
		if err != nil {
			t.Fatalf("%s line %d: Parse(%q): %v", file, line, fields[0], err)
		}

		got, err := apply(ctx, x)
		if err != nil {
			if fields[3] == "THROWS" {
				checked++
				continue
			}
			t.Errorf("%s(%s) at precision %d: unexpected error: %v", file, fields[0], cfg.Precision, err)
			continue
		}
		if got != fields[3] {
			t.Errorf("%s(%s) at precision %d rounding %d = %s, want %s",
				file, fields[0], cfg.Precision, cfg.Rounding, got, fields[3])
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	t.Logf("checked %d cases against decimal.js", checked)
}

func TestExpAgainstOriginal(t *testing.T) {
	runTranscendentalCases(t, "exp.txt", func(c *Context, x Decimal) (string, error) {
		return c.ValueOf(c.Exp(x)), nil
	})
}

func TestLnAgainstOriginal(t *testing.T) {
	runTranscendentalCases(t, "ln.txt", func(c *Context, x Decimal) (string, error) {
		r, err := c.Ln(x)
		if err != nil {
			return "", err
		}
		return c.ValueOf(r), nil
	})
}

func TestLogAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/log.txt")
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
		if len(fields) != 5 {
			t.Fatalf("log.txt line %d: got %d fields, want 5", line, len(fields))
		}

		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[3]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		x := mustParseCtx(t, ctx, fields[0])
		base := mustParseCtx(t, ctx, fields[1])

		got, err := ctx.Log(x, base)
		if err != nil {
			if fields[4] == "THROWS" {
				checked++
				continue
			}
			t.Errorf("Log(%s, %s) at precision %d: unexpected error: %v", fields[0], fields[1], cfg.Precision, err)
			continue
		}
		if s := ctx.ValueOf(got); s != fields[4] {
			t.Errorf("Log(%s, base %s) at precision %d rounding %d = %s, want %s",
				fields[0], fields[1], cfg.Precision, cfg.Rounding, s, fields[4])
		}
		checked++
	}
	t.Logf("checked %d logarithms against decimal.js", checked)
}
