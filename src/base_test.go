package decimal

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

// TestBaseOutputAgainstOriginal checks toBinary, toOctal and toHexadecimal
// against decimal.js, both in their positional form and in the binary-exponent
// form a digit count selects.
func TestBaseOutputAgainstOriginal(t *testing.T) {
	f, err := os.Open("testdata/base.txt")
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
		if len(fields) != 6 {
			t.Fatalf("base.txt line %d: got %d fields, want 6", line, len(fields))
		}

		op, value := fields[0], fields[1]
		cfg := DefaultConfig()
		cfg.Precision = atoiTest(t, fields[2])
		cfg.Rounding = RoundingMode(atoiTest(t, fields[3]))
		sd := atoiTest(t, fields[4])
		want := fields[5]

		ctx := NewContext(cfg)
		x, err := ctx.Parse(value)
		if err != nil {
			t.Fatalf("base.txt line %d: Parse(%q): %v", line, value, err)
		}

		var got string
		render := map[string]func(Decimal, int, bool, RoundingMode) (string, error){
			"toBinary":      ctx.ToBinary,
			"toOctal":       ctx.ToOctal,
			"toHexadecimal": ctx.ToHexadecimal,
		}[op]
		if render == nil {
			t.Fatalf("base.txt line %d: unknown op %q", line, op)
		}
		got, err = render(x, sd, sd >= 0, cfg.Rounding)
		if err != nil {
			t.Errorf("%s(%s, sd=%d): unexpected error: %v", op, value, sd, err)
			continue
		}
		if got != want {
			t.Errorf("%s(%s, sd=%d) at precision %d rounding %d = %s, want %s",
				op, value, sd, cfg.Precision, cfg.Rounding, got, want)
		}
		checked++
	}
	if err := scan.Err(); err != nil {
		t.Fatalf("read test data: %v", err)
	}
	if checked == 0 {
		t.Fatal("base.txt contained no cases")
	}
	t.Logf("checked %d base-conversion cases against decimal.js", checked)
}
