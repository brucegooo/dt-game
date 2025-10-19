package helper

import (
	"testing"
)

func TestLuhnGenerateAndCheck(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := Generate9PlusLuhn()
		if err != nil {
			t.Fatalf("generate error: %v", err)
		}
		if len(code) != 10 {
			t.Fatalf("len != 10: %s", code)
		}
		for j := 0; j < len(code); j++ {
			if code[j] < '0' || code[j] > '9' {
				t.Fatalf("non-digit: %s", code)
			}
		}
		if !LuhnCheck(code) {
			t.Fatalf("luhn check fail: %s", code)
		}
		// flip last digit to force fail (unless it's 0; add 1 mod 10)
		b := []byte(code)
		b[9] = byte('0' + (int(b[9]-'0')+1)%10)
		if LuhnCheck(string(b)) {
			t.Fatalf("luhn should fail after mutation: %s -> %s", code, string(b))
		}
	}
}

