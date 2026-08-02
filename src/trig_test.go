package decimal

import "testing"

// The trigonometric and hyperbolic functions are checked against decimal.js
// over six precision and rounding-mode combinations. The parity run against the
// original suite is the wider check; these keep the failure local when
// something breaks.
func TestTrigAgainstOriginal(t *testing.T) {
	cases := []struct {
		file  string
		apply func(*Context, Decimal) (Decimal, error)
	}{
		{"sin.txt", (*Context).Sin},
		{"cos.txt", (*Context).Cos},
		{"tan.txt", (*Context).Tan},
		{"asin.txt", (*Context).Asin},
		{"acos.txt", (*Context).Acos},
		{"atan.txt", (*Context).Atan},
		{"asinh.txt", (*Context).Asinh},
		{"acosh.txt", (*Context).Acosh},
		{"atanh.txt", (*Context).Atanh},
		{"sinh.txt", noError((*Context).Sinh)},
		{"cosh.txt", noError((*Context).Cosh)},
		{"tanh.txt", noError((*Context).Tanh)},
	}

	for _, c := range cases {
		file, apply := c.file, c.apply
		t.Run(file, func(t *testing.T) {
			runTranscendentalCases(t, file, func(ctx *Context, x Decimal) (string, error) {
				r, err := apply(ctx, x)
				if err != nil {
					return "", err
				}
				return ctx.ValueOf(r), nil
			})
		})
	}
}

func noError(f func(*Context, Decimal) Decimal) func(*Context, Decimal) (Decimal, error) {
	return func(c *Context, x Decimal) (Decimal, error) { return f(c, x), nil }
}
