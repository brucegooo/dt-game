package helper

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// LuhnCheck verifies a numeric string with Luhn checksum
func LuhnCheck(code string) bool {
	if len(code) == 0 {
		return false
	}
	// must be digits only
	for i := 0; i < len(code); i++ {
		c := code[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	sum := 0
	double := false
	for i := len(code) - 1; i >= 0; i-- {
		d := int(code[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// Generate9PlusLuhn returns 9 random digits + 1 luhn checksum = 10 digits
// Leading zeros are allowed.
func Generate9PlusLuhn() (string, error) {
	// build 9-digit body
	var b strings.Builder
	b.Grow(10)
	for i := 0; i < 9; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + n.Int64()))
	}
	body := b.String()
	chk := luhnDigit(body)
	return body + string('0'+chk), nil
}

// luhnDigit computes check digit for given numeric body
func luhnDigit(body string) byte {
	sum := 0
	double := true // start doubling from right of body (since check digit would be at the end)
	for i := len(body) - 1; i >= 0; i-- {
		d := int(body[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return byte((10 - (sum % 10)) % 10)
}

