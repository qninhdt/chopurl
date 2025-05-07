package main

import (
	"errors"
	"math"
)

const (
	base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// Base62ToInt64 converts a base62 string to an int64
func Base62ToInt64(s string) (int64, error) {
	var result int64
	for i := 0; i < len(s); i++ {
		// Find the position of the character in the base62Chars string
		pos := -1
		for j := 0; j < len(base62Chars); j++ {
			if base62Chars[j] == s[i] {
				pos = j
				break
			}
		}
		if pos == -1 {
			return 0, errors.New("invalid character in base62 string")
		}

		// Calculate the value for this position
		power := len(s) - i - 1
		value := int64(pos) * int64(math.Pow(62, float64(power)))

		// Check for overflow
		if result > math.MaxInt64-value {
			return 0, errors.New("base62 string too large for int64")
		}

		result += value
	}
	return result, nil
}

// Int64ToBase62 converts an int64 to a base62 string
func Int64ToBase62(n int64) string {
	if n == 0 {
		return "0"
	}

	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
}
