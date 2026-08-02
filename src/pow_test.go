package decimal

import (
	"bufio"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestPowAgainstOriginal checks integer exponentiation against decimal.js.
// Every exponent in the test data is an integer, since fractional exponents are
// not ported; the fractional case is checked separately to make sure it reports
// a gap instead of guessing.
func TestPowAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/pow.txt")
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
			t.Fatalf("pow.txt line %d: got %d fields, want 5", line, len(fields))
		}

		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[3]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		base, err := ctx.Parse(fields[0])
		if err != nil {
			t.Fatalf("pow.txt line %d: Parse(%q): %v", line, fields[0], err)
		}
		exp, err := ctx.Parse(fields[1])
		if err != nil {
			t.Fatalf("pow.txt line %d: Parse(%q): %v", line, fields[1], err)
		}

		got, err := ctx.Pow(base, exp)
		if err != nil {
			t.Errorf("Pow(%s, %s) at precision %d: unexpected error: %v",
				fields[0], fields[1], cfg.Precision, err)
			continue
		}
		if s := ctx.ValueOf(got); s != fields[4] {
			t.Errorf("Pow(%s, %s) at precision %d rounding %d = %s, want %s",
				fields[0], fields[1], cfg.Precision, cfg.Rounding, s, fields[4])
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked == 0 {
		t.Fatal("pow.txt contained no cases")
	}
	t.Logf("checked %d pow cases against decimal.js", checked)
}

// TestPowFractionalExponentReportsGap makes sure the unported case is reported
// rather than approximated.
func TestPowFractionalExponentReportsGap(t *testing.T) {
	c := NewContext(DefaultConfig())
	x := mustParse(t, "2.32456")
	y := mustParse(t, "2.29")

	if _, err := c.Pow(x, y); !errors.Is(err, ErrNotImplemented) {
		t.Errorf("Pow with a fractional exponent returned error %v, want ErrNotImplemented", err)
	}
}
