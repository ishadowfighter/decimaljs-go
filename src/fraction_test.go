package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestToFractionAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/fraction.txt")
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
			t.Fatalf("fraction.txt line %d: got %d fields, want 4", line, len(fields))
		}

		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		x := mustParseCtx(t, ctx, fields[0])
		var limit Decimal
		limitSet := fields[1] != "none"
		if limitSet {
			limit = mustParseCtx(t, ctx, fields[1])
		}

		num, den, err := ctx.ToFraction(x, limit, limitSet)
		if err != nil {
			if fields[3] == "THROWS" {
				checked++
				continue
			}
			t.Errorf("ToFraction(%s, %s): unexpected error: %v", fields[0], fields[1], err)
			continue
		}
		got := ctx.ValueOf(num) + "/" + ctx.ValueOf(den)
		if got != fields[3] {
			t.Errorf("ToFraction(%s, limit %s) at precision %d = %s, want %s",
				fields[0], fields[1], cfg.Precision, got, fields[3])
		}
		checked++
	}
	t.Logf("checked %d fractions against decimal.js", checked)
}
