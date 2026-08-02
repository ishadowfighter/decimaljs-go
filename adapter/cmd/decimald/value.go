package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	decimal "github.com/ishadowfighter/decimaljs-go/src"
)

// JSON cannot spell the four JavaScript numbers the suite cares most about, so
// they travel as these strings wherever a number is expected. Both directions
// of the protocol use the same convention: a value that is one of these is the
// string, and anything else is a plain JSON number.
const (
	numNaN     = "NaN"
	numInf     = "Infinity"
	numNegInf  = "-Infinity"
	numNegZero = "-0"
)

// decimalJSON is the wire form of a Decimal. It carries the whole internal
// state, because the suite's assertEqualProps reads .d, .e and .s directly, and
// additionally the string form, so a caller can check a value without a second
// round trip.
//
// A non-finite value has d == null and e == "NaN", exactly as decimal.js
// represents it; the two infinities are then told apart by s, and NaN has
// s == "NaN" too.
type decimalJSON struct {
	D   []int  `json:"d"`
	E   any    `json:"e"`
	S   any    `json:"s"`
	Str string `json:"str"`
}

// encode converts a Decimal to its wire form under cfg, which is needed only
// for the string field, since decimal.js's toString consults toExpNeg and
// toExpPos.
func encode(d decimal.Decimal, cfg decimal.Config) decimalJSON {
	return decimalJSON{
		D:   d.Coefficient(),
		E:   exponentValue(d),
		S:   signValue(d),
		Str: decimal.NewContext(cfg).String(d),
	}
}

// exponentValue is decimal.js's .e: the base-10 exponent for a finite value and
// NaN for everything else.
func exponentValue(d decimal.Decimal) any {
	if !d.IsFinite() {
		return numNaN
	}
	return d.Exponent()
}

// signValue is decimal.js's .s: 1, -1, or NaN for NaN.
func signValue(d decimal.Decimal) any {
	if d.IsNaN() {
		return numNaN
	}
	return d.Sign()
}

// jsSign is decimal.js's Decimal.sign function, which differs from .s in that
// it reports a signed zero for zero. Decimal.Signum cannot express that, so it
// is reassembled here from the parts that can.
func jsSign(d decimal.Decimal) any {
	switch {
	case d.IsNaN():
		return numNaN
	case d.IsZero() && d.IsNegative():
		return numNegZero
	case d.IsZero():
		return 0
	case d.IsNegative():
		return -1
	default:
		return 1
	}
}

// construct turns one JSON argument into a Decimal. Three spellings are
// accepted:
//
//   - a string, parsed exactly as decimal.js parses a string operand;
//   - a number, parsed from its JSON literal (see the caveat below);
//   - an object, which is a decimalJSON produced by a previous response and is
//     reconstructed bit for bit.
//
// The shim never sends bare JSON numbers for operands, because JSON.stringify
// renders -0 as 0 and cannot render NaN or the infinities at all; it stringifies
// JavaScript numbers on its side instead. The number case exists only so that a
// request typed by hand, or one borrowed from the upstream fuzzer's line
// protocol, still works.
func construct(ctx *decimal.Context, raw json.RawMessage) (decimal.Decimal, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return decimal.Decimal{}, fmt.Errorf("%w: empty argument", errProtocol)
	}

	switch text[0] {
	case '{':
		var dj decimalJSON
		if err := json.Unmarshal(raw, &dj); err != nil {
			return decimal.Decimal{}, fmt.Errorf("%w: bad decimal object: %v", errProtocol, err)
		}
		return rebuild(dj)

	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return decimal.Decimal{}, fmt.Errorf("%w: bad string: %v", errProtocol, err)
		}
		return ctx.Parse(s)

	case 't', 'f', 'n', '[':
		return decimal.Decimal{}, fmt.Errorf("%w: %s is not a valid operand", errProtocol, text)

	default:
		// A JSON number literal is already a decimal string decimal.js
		// accepts, so it needs no conversion.
		return ctx.Parse(text)
	}
}

// rebuildContext is the configuration used to turn a wire decimal back into a
// Decimal. It is deliberately not the caller's configuration: reconstruction
// must be exact, so the exponent limits are opened all the way out and cannot
// clip the value to zero or Infinity on the way in.
var rebuildContext = decimal.NewContext(decimal.Config{
	Precision: 1e9,
	Rounding:  decimal.RoundHalfUp,
	Modulo:    decimal.ModTruncate,
	ToExpNeg:  -9e15,
	ToExpPos:  9e15,
	MinE:      -9e15,
	MaxE:      9e15,
})

// rebuild reconstructs a Decimal from the state a previous response reported.
// The port exposes no constructor that takes limbs directly, so the state is
// rendered as an exact exponential string and parsed back. That is lossless:
// parsing does not round to precision, and the wide exponent limits of
// rebuildContext keep the value away from the overflow and underflow paths.
func rebuild(dj decimalJSON) (decimal.Decimal, error) {
	sign, err := signOf(dj.S)
	if err != nil {
		return decimal.Decimal{}, err
	}

	if dj.D == nil {
		if sign == 0 {
			return decimal.NaN(), nil
		}
		return decimal.Inf(sign), nil
	}
	if sign == 0 {
		return decimal.Decimal{}, fmt.Errorf("%w: finite value with a NaN sign", errProtocol)
	}

	if _, ok := dj.E.(float64); !ok {
		return decimal.Decimal{}, fmt.Errorf("%w: finite value with a non-numeric exponent %v", errProtocol, dj.E)
	}

	exp, ok := dj.E.(float64)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("%w: finite value with a non-numeric exponent %v", errProtocol, dj.E)
	}

	// Rebuild from the limbs rather than from the string field, so that a
	// caller holding only the internal state - which is all a shim instance
	// carries - can send a value back. Writing the digits out with an explicit
	// exponent is lossless: parsing does not round to precision, and
	// rebuildContext's wide exponent limits keep the value away from the
	// overflow and underflow paths.
	digits := limbsToString(dj.D)
	var b strings.Builder
	if sign < 0 {
		b.WriteByte('-')
	}
	b.WriteString(digits[:1])
	if len(digits) > 1 {
		b.WriteByte('.')
		b.WriteString(digits[1:])
	}
	b.WriteByte('e')
	b.WriteString(strconv.Itoa(int(exp)))
	return rebuildContext.Parse(b.String())
}

// limbsToString concatenates base-1e7 limbs into their decimal digits. Every
// limb but the first is padded to seven digits, since a limb's leading zeros
// are significant digits of the value.
func limbsToString(limbs []int) string {
	var b strings.Builder
	for i, w := range limbs {
		ws := strconv.Itoa(w)
		if i > 0 {
			for n := len(ws); n < 7; n++ {
				b.WriteByte('0')
			}
		}
		b.WriteString(ws)
	}
	if b.Len() == 0 {
		return "0"
	}
	return b.String()
}

// signOf reads the .s field, which is 1, -1 or the string "NaN". It returns 0
// for NaN, matching the port's internal sentinel.
func signOf(v any) (int, error) {
	switch s := v.(type) {
	case float64:
		if s < 0 {
			return -1, nil
		}
		return 1, nil
	case string:
		if s == numNaN {
			return 0, nil
		}
		if strings.HasPrefix(s, "-") {
			return -1, nil
		}
		return 1, nil
	default:
		return 0, fmt.Errorf("%w: bad sign %v", errProtocol, v)
	}
}

// decodeFloat reads a JSON number, or one of the string spellings of the
// numbers JSON has no literal for.
func decodeFloat(raw json.RawMessage) (float64, error) {
	text := strings.TrimSpace(string(raw))
	if len(text) > 0 && text[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return 0, fmt.Errorf("%w: bad number: %v", errProtocol, err)
		}
		text = s
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: bad number %q", errProtocol, text)
	}
	return f, nil
}

// configJSON is the wire form of the settings, using decimal.js's names. It is
// kept as raw fields rather than typed ones so that a setting of the wrong type
// — which the config test module supplies deliberately, over and over — is
// reported as a DecimalError from the operation rather than as a failure to
// parse the request. decimal.js throws in exactly the same place.
//
// Every field is optional; an absent field keeps the default. Unknown fields are
// ignored, as decimal.js ignores properties outside its own list.
type configJSON map[string]json.RawMessage

// settingNames is the order settings are applied in. It has no effect on the
// result, since validation happens once at the end, but it keeps error messages
// deterministic.
var settingNames = []string{
	"precision", "rounding", "modulo", "toExpNeg", "toExpPos", "minE", "maxE", "crypto",
}

// resolve applies the request's settings on top of the defaults and validates
// the result the way decimal.js's Decimal.set does, so an out-of-range or
// wrongly typed setting is reported as a DecimalError rather than accepted.
//
// The `defaults` flag of the upstream fuzzer's protocol is accepted and ignored:
// every request already starts from the defaults, because the process keeps no
// configuration between lines.
func (c configJSON) resolve() (decimal.Config, error) {
	cfg := decimal.DefaultConfig()
	if c == nil {
		return cfg, nil
	}

	for _, name := range settingNames {
		raw, present := c[name]
		if !present {
			continue
		}
		if name == "crypto" {
			b, err := settingBool(name, raw)
			if err != nil {
				return cfg, err
			}
			cfg.Crypto = b
			continue
		}
		n, err := settingInt(name, raw)
		if err != nil {
			return cfg, err
		}
		switch name {
		case "precision":
			cfg.Precision = n
		case "rounding":
			cfg.Rounding = decimal.RoundingMode(n)
		case "modulo":
			cfg.Modulo = decimal.ModuloMode(n)
		case "toExpNeg":
			cfg.ToExpNeg = n
		case "toExpPos":
			cfg.ToExpPos = n
		case "minE":
			cfg.MinE = n
		case "maxE":
			cfg.MaxE = n
		}
	}

	// SetConfig carries the range checks; a throwaway context is the cheapest
	// way to reach them.
	if err := decimal.NewContext(decimal.DefaultConfig()).SetConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// settingInt reads a setting that must be a whole number. decimal.js rejects a
// fractional or non-numeric setting with the same DecimalError it uses for one
// that is merely out of range, so both arrive here as decimal-level failures.
func settingInt(name string, raw json.RawMessage) (int, error) {
	// json.Unmarshal accepts null into a float64 as a no-op, leaving 0 behind,
	// so null has to be rejected before it is mistaken for a valid setting.
	// The suite passes null, NaN and Infinity here on purpose.
	if isJSONNull(raw) {
		return 0, badSetting(name, raw)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, badSetting(name, raw)
	}
	n := int(f)
	if float64(n) != f {
		return 0, badSetting(name, raw)
	}
	return n, nil
}

// badSetting is decimal.js's `[DecimalError] Invalid argument: <name>: <value>`.
func badSetting(name string, raw json.RawMessage) error {
	return fmt.Errorf("DecimalError: invalid argument: %s: %s", name, strings.TrimSpace(string(raw)))
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

// settingBool reads a setting that must be a boolean. decimal.js also accepts
// the numbers 1 and 0 there.
func settingBool(name string, raw json.RawMessage) (bool, error) {
	if isJSONNull(raw) {
		return false, badSetting(name, raw)
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		if f == 0 {
			return false, nil
		}
		if f == 1 {
			return true, nil
		}
	}
	return false, badSetting(name, raw)
}

// encodeConfig reports the effective settings using decimal.js's names.
func encodeConfig(cfg decimal.Config) map[string]any {
	return map[string]any{
		"precision": cfg.Precision,
		"rounding":  int(cfg.Rounding),
		"modulo":    int(cfg.Modulo),
		"toExpNeg":  cfg.ToExpNeg,
		"toExpPos":  cfg.ToExpPos,
		"minE":      cfg.MinE,
		"maxE":      cfg.MaxE,
		"crypto":    cfg.Crypto,
	}
}
