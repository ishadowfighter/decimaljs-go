package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestCbrtAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/cbrt.txt")
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
			t.Fatalf("cbrt.txt line %d: got %d fields, want 4", line, len(fields))
		}

		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[1])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[2]))
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		x, err := ctx.Parse(fields[0])
		if err != nil {
			t.Fatalf("cbrt.txt line %d: Parse(%q): %v", line, fields[0], err)
		}
		if got := ctx.ValueOf(ctx.Cbrt(x)); got != fields[3] {
			t.Errorf("Cbrt(%s) at precision %d rounding %d = %s, want %s",
				fields[0], cfg.Precision, cfg.Rounding, got, fields[3])
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	t.Logf("checked %d cube roots against decimal.js", checked)
}
