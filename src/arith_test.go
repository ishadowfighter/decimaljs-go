package decimal

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"testing"
)

// binaryCase is one line of a testdata file generated from decimal.js.
type binaryCase struct {
	op        string
	a, b      string
	precision int
	rounding  RoundingMode
	modulo    ModuloMode
	wantLimbs []int
	wantExp   string
	wantSign  string
}

// runBinaryCases checks apply against every expectation in the named testdata
// file. The expectations are decimal.js's own results, taken across twelve
// precision and rounding-mode combinations so the arithmetic is exercised
// together with all nine rounding modes rather than only the default.
func runBinaryCases(t *testing.T, file string, apply func(c *Context, x, y Decimal) Decimal) {
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
		if len(fields) != 9 {
			t.Fatalf("%s line %d: got %d fields, want 9", file, line, len(fields))
		}
		c := binaryCase{
			op: fields[0], a: fields[1], b: fields[2],
			precision: atoiTest(t, fields[3]),
			rounding:  RoundingMode(atoiTest(t, fields[4])),
			modulo:    ModuloMode(atoiTest(t, fields[5])),
			wantLimbs: parseLimbs(t, fields[6]),
			wantExp:   fields[7],
			wantSign:  fields[8],
		}

		cfg := DefaultConfig()
		cfg.Precision = c.precision
		cfg.Rounding = c.rounding
		cfg.Modulo = c.modulo
		cfg.ToExpNeg = -9e15
		cfg.ToExpPos = 9e15
		ctx := NewContext(cfg)

		x, err := ctx.Parse(c.a)
		if err != nil {
			t.Fatalf("%s line %d: Parse(%q): %v", file, line, c.a, err)
		}
		y, err := ctx.Parse(c.b)
		if err != nil {
			t.Fatalf("%s line %d: Parse(%q): %v", file, line, c.b, err)
		}

		got := apply(ctx, x, y)
		if !matchesExpectation(got, c.wantLimbs, c.wantExp, c.wantSign) {
			t.Errorf("%s(%s, %s) at precision %d rounding %d modulo %d = d:%v e:%d s:%d, want d:%v e:%s s:%s",
				c.op, c.a, c.b, c.precision, c.rounding, c.modulo,
				got.Coefficient(), got.Exponent(), got.Sign(),
				c.wantLimbs, c.wantExp, c.wantSign)
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

// matchesExpectation compares a result against one testdata row. decimal.js
// stores NaN in the exponent of every non-finite value and in the sign of NaN
// itself, so those fields arrive as the string "NaN".
func matchesExpectation(got Decimal, wantLimbs []int, wantExp, wantSign string) bool {
	if wantLimbs == nil {
		if got.IsFinite() {
			return false
		}
		if wantSign == "NaN" {
			return got.IsNaN()
		}
		return got.IsInf() && strconv.Itoa(got.Sign()) == wantSign
	}
	return equalLimbs(got.Coefficient(), wantLimbs) &&
		strconv.Itoa(got.Exponent()) == wantExp &&
		strconv.Itoa(got.Sign()) == wantSign
}

func TestAddAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "plus.txt", func(c *Context, x, y Decimal) Decimal { return c.Add(x, y) })
}

func TestSubAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "minus.txt", func(c *Context, x, y Decimal) Decimal { return c.Sub(x, y) })
}

// TestAddSubDoNotMutateOperands guards the immutability the original suite
// checks at length: both operands' coefficients are shared with the caller and
// must survive an operation untouched.
func TestAddSubDoNotMutateOperands(t *testing.T) {
	x := mustParse(t, "99999999999999999999.123456789")
	y := mustParse(t, "0.000000000000000000000001")

	xBefore, yBefore := x.Coefficient(), y.Coefficient()
	x.Add(y)
	x.Sub(y)
	y.Sub(x)

	if !equalLimbs(x.Coefficient(), xBefore) {
		t.Errorf("left operand mutated: %v, want %v", x.Coefficient(), xBefore)
	}
	if !equalLimbs(y.Coefficient(), yBefore) {
		t.Errorf("right operand mutated: %v, want %v", y.Coefficient(), yBefore)
	}
}

func TestMulAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "times.txt", func(c *Context, x, y Decimal) Decimal { return c.Mul(x, y) })
}

func TestDivAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "div.txt", func(c *Context, x, y Decimal) Decimal { return c.Div(x, y) })
}

func TestDivToIntAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "divToInt.txt", func(c *Context, x, y Decimal) Decimal { return c.DivToInt(x, y) })
}

func TestModAgainstOriginal(t *testing.T) {
	runBinaryCases(t, "mod.txt", func(c *Context, x, y Decimal) Decimal { return c.Mod(x, y) })
}
