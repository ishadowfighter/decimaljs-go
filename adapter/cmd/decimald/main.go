// Command decimald is the Go side of the test harness. It reads one JSON
// request per line from stdin and writes one JSON response per line to stdout,
// so that the vendored decimal.js test suite can drive the Go port through a
// thin JavaScript shim without either side being modified.
//
// The protocol is specified in adapter/PROTOCOL.md. In short:
//
//	{"id":1,"op":"new","args":["1.5"],"config":{"precision":20}}
//	{"id":1,"ok":true,"value":{"d":[1,5000000],"e":0,"s":1,"str":"1.5"}}
//
// Every request is evaluated from a fresh configuration: the process holds no
// state between lines, so a caller that spawns one process per call and a
// caller that streams a thousand calls through one process observe identical
// results. That keeps Decimal.clone(), which the suite uses to run several
// independently configured constructors at once, from needing server-side
// bookkeeping.
//
// Only the operations the port has implemented are registered. Anything else
// is reported as an "unimplemented" failure that the shim turns into a plainly
// labelled JavaScript error, never into a DecimalError — the suite treats a
// DecimalError as a passing outcome in assertException, so a missing operation
// must not be able to masquerade as one.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"

	decimal "github.com/USER/decimaljs-go/src"
)

// maxLine bounds a single request. decimal.js accepts operands with a million
// digits, so the default scanner buffer is far too small.
const maxLine = 64 << 20

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64<<10), maxLine)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	enc := json.NewEncoder(out)
	// Struck through by encoding/json by default; the suite compares strings
	// containing '<' and '&' in no place, but escaped output is harder to read
	// when debugging a failing line by hand.
	enc.SetEscapeHTML(false)

	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		if err := enc.Encode(handle([]byte(line))); err != nil {
			fmt.Fprintln(os.Stderr, "decimald: write:", err)
			os.Exit(1)
		}
		// Flush per line: a caller doing synchronous request/response over a
		// pipe deadlocks if the reply sits in a buffer.
		if err := out.Flush(); err != nil {
			fmt.Fprintln(os.Stderr, "decimald: flush:", err)
			os.Exit(1)
		}
	}
	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "decimald: read:", err)
		os.Exit(1)
	}
}

// request is one line of input.
type request struct {
	ID     int               `json:"id"`
	Op     string            `json:"op"`
	Args   []json.RawMessage `json:"args"`
	Config configJSON        `json:"config"`
}

// response is one line of output. Value is always present, and is null on
// failure; Error and Kind are present only on failure.
type response struct {
	ID    int    `json:"id"`
	OK    bool   `json:"ok"`
	Value any    `json:"value"`
	Error string `json:"error,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// Failure kinds. "decimal" is a condition decimal.js signals by throwing a
// DecimalError and the suite may legitimately assert on; "unimplemented" and
// "protocol" are harness faults that must always surface as test failures.
const (
	kindDecimal       = "decimal"
	kindUnimplemented = "unimplemented"
	kindProtocol      = "protocol"
)

// errUnimplemented marks an operation the port does not provide yet.
var errUnimplemented = errors.New("not implemented in port")

// errProtocol marks a malformed request.
var errProtocol = errors.New("protocol error")

func handle(line []byte) response {
	var req request
	if err := json.Unmarshal(line, &req); err != nil {
		return fail(0, fmt.Errorf("%w: %v", errProtocol, err))
	}

	cfg, err := req.Config.resolve()
	if err != nil {
		return fail(req.ID, err)
	}

	op, ok := ops[req.Op]
	if !ok {
		return fail(req.ID, fmt.Errorf("%w: op %q", errUnimplemented, req.Op))
	}
	if op.arity >= 0 && len(req.Args) != op.arity {
		return fail(req.ID, fmt.Errorf("%w: op %q takes %d args, got %d",
			errProtocol, req.Op, op.arity, len(req.Args)))
	}

	value, err := op.fn(decimal.NewContext(cfg), cfg, req.Args)
	if err != nil {
		return fail(req.ID, err)
	}
	return response{ID: req.ID, OK: true, Value: value}
}

func fail(id int, err error) response {
	kind := kindDecimal
	switch {
	case errors.Is(err, errUnimplemented):
		kind = kindUnimplemented
	case errors.Is(err, errProtocol):
		kind = kindProtocol
	}

	msg := err.Error()
	if kind == kindDecimal && !strings.Contains(msg, "DecimalError") {
		// The port's own errors already read "DecimalError: ...", because
		// decimal.Err carries that text. Anything else that reaches here is
		// still a decimal-level failure and gets the label the suite's
		// assertException matches on.
		msg = "DecimalError: " + msg
	}
	return response{ID: id, OK: false, Value: nil, Error: msg, Kind: kind}
}

// operation is one entry of the dispatch table. Adding an arithmetic or
// formatting operation once the port grows one is a single line below.
type operation struct {
	// arity is the exact number of arguments, or -1 for variadic.
	arity int
	fn    func(ctx *decimal.Context, cfg decimal.Config, args []json.RawMessage) (any, error)
}

// ops is the dispatch table. Only operations backed by the port are listed;
// every other name falls through to an "unimplemented" failure.
var ops = map[string]operation{
	// Construction.
	"new":       {1, unary(func(d decimal.Decimal, g decimal.Config) (any, error) { return encode(d, g), nil })},
	"fromInt":   {1, opFromInt},
	"fromFloat": {1, opFromFloat},
	"nan": {0, func(_ *decimal.Context, g decimal.Config, _ []json.RawMessage) (any, error) {
		return encode(decimal.NaN(), g), nil
	}},
	"inf": {1, opInf},

	// Internal state, exposed because the suite asserts on it directly.
	"d":    {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return d.Coefficient(), nil })},
	"e":    {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return exponentValue(d), nil })},
	"s":    {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return signValue(d), nil })},
	"str":  {1, unary(func(d decimal.Decimal, g decimal.Config) (any, error) { return decimal.NewContext(g).String(d), nil })},
	"echo": {1, unary(func(d decimal.Decimal, g decimal.Config) (any, error) { return encode(d, g), nil })},

	// Predicates.
	"isNaN":      {1, predicate(decimal.Decimal.IsNaN)},
	"isFinite":   {1, predicate(decimal.Decimal.IsFinite)},
	"isInf":      {1, predicate(decimal.Decimal.IsInf)},
	"isZero":     {1, predicate(decimal.Decimal.IsZero)},
	"isInteger":  {1, predicate(decimal.Decimal.IsInteger)},
	"isNegative": {1, predicate(decimal.Decimal.IsNegative)},
	"isPositive": {1, predicate(decimal.Decimal.IsPositive)},

	// decimal.js's Decimal.sign: the sign as a JavaScript number, which is
	// finer grained than Signum because it keeps -0 and NaN apart.
	"sign":   {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return jsSign(d), nil })},
	"signum": {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return d.Signum(), nil })},

	// Comparison.
	"cmp": {2, binary(func(x, y decimal.Decimal, _ decimal.Config) (any, error) {
		order, ok := x.Cmp(y)
		if !ok {
			return numNaN, nil
		}
		return order, nil
	})},
	"eq":  {2, comparison(decimal.Decimal.Eq)},
	"gt":  {2, comparison(decimal.Decimal.Gt)},
	"gte": {2, comparison(decimal.Decimal.Gte)},
	"lt":  {2, comparison(decimal.Decimal.Lt)},
	"lte": {2, comparison(decimal.Decimal.Lte)},

	// Arithmetic.
	"add": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal { return c.Add(x, y) })},
	"sub": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal { return c.Sub(x, y) })},
	"mul": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal { return c.Mul(x, y) })},
	"div": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal { return c.Div(x, y) })},
	"divToInt": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal {
		return c.DivToInt(x, y)
	})},
	"mod": {2, decimalBinary(func(c *decimal.Context, x, y decimal.Decimal) decimal.Decimal { return c.Mod(x, y) })},
	"pow": {2, func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, y, err := twoOperands(c, a)
		if err != nil {
			return nil, err
		}
		r, err := c.Pow(x, y)
		if err != nil {
			return nil, unimplementedIfUnported(err)
		}
		return encode(r, g), nil
	}},

	// Sign and whole-number rounding.
	"abs":   {1, decimalUnary((*decimal.Context).Abs)},
	"neg":   {1, decimalUnary((*decimal.Context).Neg)},
	"round": {1, decimalUnary((*decimal.Context).Round)},
	"floor": {1, decimalUnary((*decimal.Context).Floor)},
	"ceil":  {1, decimalUnary((*decimal.Context).Ceil)},
	"trunc": {1, decimalUnary((*decimal.Context).Trunc)},

	// Rounding to a digit count. The rounding mode is always sent explicitly:
	// the shim resolves decimal.js's optional argument against the current
	// configuration before the call, so the server never has to guess.
	"toDP": {3, withDigitsAndMode(func(c *decimal.Context, x decimal.Decimal, n int, rm decimal.RoundingMode) (decimal.Decimal, error) {
		return c.ToDecimalPlaces(x, n, rm)
	})},
	"toSD": {3, withDigitsAndMode(func(c *decimal.Context, x decimal.Decimal, n int, rm decimal.RoundingMode) (decimal.Decimal, error) {
		return c.ToSignificantDigits(x, n, rm)
	})},
	"toNearest": {3, func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, y, err := twoOperands(c, a)
		if err != nil {
			return nil, err
		}
		rm, err := roundingMode(a[2])
		if err != nil {
			return nil, err
		}
		r, err := c.ToNearest(x, y, rm)
		if err != nil {
			return nil, err
		}
		return encode(r, g), nil
	}},
	"clamp": {3, func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		lo, err := construct(c, a[1])
		if err != nil {
			return nil, err
		}
		hi, err := construct(c, a[2])
		if err != nil {
			return nil, err
		}
		r, err := c.Clamp(x, lo, hi)
		if err != nil {
			return nil, err
		}
		return encode(r, g), nil
	}},

	// Formatting. Each takes the digit count and rounding mode explicitly, with
	// a digit count of -1 standing for decimal.js's absent argument, which is a
	// separate code path rather than a default value.
	"toString":      {1, stringOp((*decimal.Context).String)},
	"valueOf":       {1, stringOp((*decimal.Context).ValueOf)},
	"toFixed":       {3, formatOp((*decimal.Context).StringNoExponent, (*decimal.Context).ToFixed)},
	"toExponential": {3, formatOp((*decimal.Context).StringExponent, (*decimal.Context).ToExponential)},
	"toPrecision":   {3, formatOp((*decimal.Context).String, (*decimal.Context).ToPrecision)},

	// Base conversion. A digit count of "absent" selects the positional form;
	// any number selects the binary-exponent form.
	"toBinary": {3, baseOp((*decimal.Context).ToBinary)},
	"toOctal":  {3, baseOp((*decimal.Context).ToOctal)},
	"toHex":    {3, baseOp((*decimal.Context).ToHexadecimal)},

	// Digit counts. Both report NaN for a non-finite value, as decimal.js does.
	"dp": {1, unary(func(d decimal.Decimal, _ decimal.Config) (any, error) {
		if n, ok := d.DecimalPlaces(); ok {
			return n, nil
		}
		return numNaN, nil
	})},
	"sd": {2, func(c *decimal.Context, _ decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		var includeZeros bool
		if err := json.Unmarshal(a[1], &includeZeros); err != nil {
			return nil, fmt.Errorf("%w: sd wants a boolean: %v", errProtocol, err)
		}
		if n, ok := x.SignificantDigits(includeZeros); ok {
			return n, nil
		}
		return numNaN, nil
	}},

	// Variadic reductions.
	"max": {-1, reduction(func(_ *decimal.Context, xs []decimal.Decimal) decimal.Decimal { return decimal.Max(xs...) })},
	"min": {-1, reduction(func(_ *decimal.Context, xs []decimal.Decimal) decimal.Decimal { return decimal.Min(xs...) })},
	"sum": {-1, reduction(func(c *decimal.Context, xs []decimal.Decimal) decimal.Decimal { return c.Sum(xs...) })},

	// Configuration. Echoes back the effective settings, so the shim can
	// implement Decimal.config's read-back and its range checking without
	// duplicating the rules.
	"config": {0, func(_ *decimal.Context, g decimal.Config, _ []json.RawMessage) (any, error) {
		return encodeConfig(g), nil
	}},
}

// unimplementedIfUnported relabels the port's "not implemented" error so the
// shim reports it as a harness gap rather than as a DecimalError, which the
// suite's assertException would count as a pass.
func unimplementedIfUnported(err error) error {
	if errors.Is(err, decimal.ErrNotImplemented) {
		return fmt.Errorf("%w: %v", errUnimplemented, err)
	}
	return err
}

// decimalUnary adapts a one-operand Context method returning a Decimal.
func decimalUnary(f func(*decimal.Context, decimal.Decimal) decimal.Decimal) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		return encode(f(c, x), g), nil
	}
}

// decimalBinary adapts a two-operand Context method returning a Decimal.
func decimalBinary(f func(*decimal.Context, decimal.Decimal, decimal.Decimal) decimal.Decimal) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, y, err := twoOperands(c, a)
		if err != nil {
			return nil, err
		}
		return encode(f(c, x, y), g), nil
	}
}

// stringOp adapts a Context method rendering a Decimal with no arguments.
func stringOp(f func(*decimal.Context, decimal.Decimal) string) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, _ decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		return f(c, x), nil
	}
}

// formatOp adapts the three formatting methods that take an optional digit
// count: a count below zero selects the no-argument form.
func formatOp(
	noArg func(*decimal.Context, decimal.Decimal) string,
	withArg func(*decimal.Context, decimal.Decimal, int, decimal.RoundingMode) (string, error),
) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, _ decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		n, err := digitCountArg(a[1])
		if err != nil {
			return nil, err
		}
		if n == absentArg {
			return noArg(c, x), nil
		}
		rm, err := roundingMode(a[2])
		if err != nil {
			return nil, err
		}
		return withArg(c, x, n, rm)
	}
}

// withDigitsAndMode adapts the two rounding methods that take a digit count and
// a rounding mode.
func withDigitsAndMode(f func(*decimal.Context, decimal.Decimal, int, decimal.RoundingMode) (decimal.Decimal, error)) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		n, err := digitCountArg(a[1])
		if err != nil {
			return nil, err
		}
		rm, err := roundingMode(a[2])
		if err != nil {
			return nil, err
		}
		r, err := f(c, x, n, rm)
		if err != nil {
			return nil, err
		}
		return encode(r, g), nil
	}
}

// baseOp adapts one of the three base-conversion methods.
func baseOp(f func(*decimal.Context, decimal.Decimal, int, bool, decimal.RoundingMode) (string, error)) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, _ decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		n, err := digitCountArg(a[1])
		if err != nil {
			return nil, err
		}
		if n == absentArg {
			return f(c, x, 0, false, c.Config().Rounding)
		}
		rm, err := roundingMode(a[2])
		if err != nil {
			return nil, err
		}
		return f(c, x, n, true, rm)
	}
}

// reduction adapts the variadic functions.
func reduction(f func(*decimal.Context, []decimal.Decimal) decimal.Decimal) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		if len(a) == 0 {
			return nil, fmt.Errorf("%w: needs at least one operand", errProtocol)
		}
		xs := make([]decimal.Decimal, len(a))
		for i, raw := range a {
			x, err := construct(c, raw)
			if err != nil {
				return nil, err
			}
			xs[i] = x
		}
		return encode(f(c, xs), g), nil
	}
}

// twoOperands constructs the first two arguments.
func twoOperands(c *decimal.Context, a []json.RawMessage) (decimal.Decimal, decimal.Decimal, error) {
	x, err := construct(c, a[0])
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, err
	}
	y, err := construct(c, a[1])
	if err != nil {
		return decimal.Decimal{}, decimal.Decimal{}, err
	}
	return x, y, nil
}

// absentArg marks a digit count the caller did not supply. decimal.js branches
// on `dp === undefined` before it validates anything, and that path is distinct
// from any number: toFixed(-1) is an error, toFixed() is not.
const absentArg = math.MinInt32 - 1

// digitCountArg reads a digit-count argument.
func digitCountArg(raw json.RawMessage) (int, error) {
	if strings.TrimSpace(string(raw)) == `"absent"` {
		return absentArg, nil
	}
	return integerArg("digit count", raw)
}

// roundingMode reads a rounding-mode argument. Out-of-range values are left to
// the port, which reports them the way decimal.js does.
func roundingMode(raw json.RawMessage) (decimal.RoundingMode, error) {
	n, err := integerArg("rounding mode", raw)
	if err != nil {
		return 0, err
	}
	return decimal.RoundingMode(n), nil
}

// integerArg decodes a numeric argument that decimal.js requires to be a
// 32-bit integer. It is read as a float first so that a fractional or wildly
// out-of-range value - both of which decimal.js rejects with a DecimalError -
// is reported as one, rather than as a protocol fault that would look like a
// harness bug.
func integerArg(name string, raw json.RawMessage) (int, error) {
	text := strings.TrimSpace(string(raw))

	// JSON has no NaN or Infinity, so JavaScript sends all three of NaN,
	// Infinity and null as null. decimal.js rejects every one of them, and so
	// does this - silently reading null as zero would turn a test that expects
	// an error into a passing call.
	if text == "null" {
		return 0, fmt.Errorf("DecimalError: invalid argument: %s: null", name)
	}

	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		// A string, array or object argument is not a protocol fault: the suite
		// passes those deliberately and expects a DecimalError back.
		return 0, fmt.Errorf("DecimalError: invalid argument: %s: %s", name, text)
	}
	if f != math.Trunc(f) || f < math.MinInt32 || f > math.MaxInt32 {
		return 0, fmt.Errorf("DecimalError: invalid argument: %s: %v", name, f)
	}
	return int(f), nil
}

// The introspection operation lists the table it lives in, so it is registered
// after the table is built rather than inside its literal, which would be an
// initialisation cycle. A caller uses it to tell "not ported yet" from
// "misspelt".
func init() {
	ops["ops"] = operation{0, func(_ *decimal.Context, _ decimal.Config, _ []json.RawMessage) (any, error) {
		names := make([]string, 0, len(ops))
		for name := range ops {
			names = append(names, name)
		}
		sortStrings(names)
		return names, nil
	}}
}

// unary adapts a one-operand function to the table's signature.
func unary(f func(decimal.Decimal, decimal.Config) (any, error)) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		return f(x, g)
	}
}

// binary adapts a two-operand function to the table's signature.
func binary(f func(x, y decimal.Decimal, cfg decimal.Config) (any, error)) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return func(c *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
		x, err := construct(c, a[0])
		if err != nil {
			return nil, err
		}
		y, err := construct(c, a[1])
		if err != nil {
			return nil, err
		}
		return f(x, y, g)
	}
}

func predicate(f func(decimal.Decimal) bool) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return unary(func(d decimal.Decimal, _ decimal.Config) (any, error) { return f(d), nil })
}

func comparison(f func(decimal.Decimal, decimal.Decimal) bool) func(*decimal.Context, decimal.Config, []json.RawMessage) (any, error) {
	return binary(func(x, y decimal.Decimal, _ decimal.Config) (any, error) { return f(x, y), nil })
}

func opFromInt(_ *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
	var n int64
	if err := json.Unmarshal(a[0], &n); err != nil {
		return nil, fmt.Errorf("%w: fromInt wants an integer: %v", errProtocol, err)
	}
	return encode(decimal.NewContext(g).NewFromInt(n), g), nil
}

func opFromFloat(_ *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
	f, err := decodeFloat(a[0])
	if err != nil {
		return nil, err
	}
	return encode(decimal.NewContext(g).NewFromFloat(f), g), nil
}

func opInf(_ *decimal.Context, g decimal.Config, a []json.RawMessage) (any, error) {
	var sign int
	if err := json.Unmarshal(a[0], &sign); err != nil {
		return nil, fmt.Errorf("%w: inf wants a sign: %v", errProtocol, err)
	}
	return encode(decimal.Inf(sign), g), nil
}

// sortStrings is an insertion sort, used once on a short slice to keep the
// import list minimal.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
