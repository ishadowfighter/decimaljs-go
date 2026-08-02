package decimal

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	mathrand "math/rand"
)

// Random returns a pseudo-random value in [0, 1) with sd significant digits,
// using the default context.
func Random(sd int) (Decimal, error) { return defaultContext.Random(sd) }

// Random returns a pseudo-random value in [0, 1) with sd significant digits. A
// sd of zero means the Context's precision.
//
// With Crypto set the digits come from crypto/rand, and values that would bias
// the modulo reduction are rejected and redrawn, as decimal.js does.
func (c *Context) Random(sd int) (Decimal, error) {
	cfg := c.config
	if sd == 0 {
		sd = cfg.Precision
	} else if sd < 1 || sd > maxDigits {
		return Decimal{}, wrapInvalidArgument("significant digits", sd)
	}

	k := ceilDiv(sd, logBase)
	rd := make([]int, k)
	for i := range rd {
		if cfg.Crypto {
			rd[i] = cryptoLimb()
		} else {
			rd[i] = mathrand.Intn(base)
		}
	}

	// Zero the digits of the last limb beyond sd.
	if last := k - 1; rd[last] != 0 && sd%logBase != 0 {
		n := pow10(logBase - sd%logBase)
		rd[last] = rd[last] / n * n
	}

	for len(rd) > 0 && rd[len(rd)-1] == 0 {
		rd = rd[:len(rd)-1]
	}

	if len(rd) == 0 {
		return Decimal{coefficient: []int{0}, exponent: 0, sign: 1}, nil
	}

	e := -1
	for rd[0] == 0 {
		rd = rd[1:]
		e -= logBase
	}
	if digits := digitCount(rd[0]); digits < logBase {
		e -= logBase - digits
	}

	return Decimal{coefficient: rd, exponent: e, sign: 1}, nil
}

// cryptoLimb draws one uniform limb, rejecting the tail of the 32-bit range
// that a modulo would over-represent.
func cryptoLimb() int {
	const limit = uint32(4294967296 - 4294967296%int64(base))
	var buf [4]byte
	for {
		if _, err := cryptorand.Read(buf[:]); err != nil {
			panic(err)
		}
		if n := binary.LittleEndian.Uint32(buf[:]); n < limit {
			return int(n % base)
		}
	}
}
