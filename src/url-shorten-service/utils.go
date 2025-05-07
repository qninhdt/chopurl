package main

import (
	"fmt"
	"math"
	"strings"
)

func IsValidURL(url string) bool {
	// Check if the URL starts with http:// or https://
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}
	// Check if the URL contains a domain name
	if strings.Count(url, ".") < 1 {
		return false
	}
	// Check if the URL contains a path
	if strings.Count(url, "/") < 2 {
		return false
	}
	return true
}

// convert int64 to 7 base62 characters
func Int64ToBase62(n int64) string {
	const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var result string
	for n > 0 {
		result = string(base62Chars[n%62]) + result
		n /= 62
	}
	// pad with leading zeros to make it 7 characters long
	for len(result) < 7 {
		result = "0" + result
	}
	return result
}

// convert 7 base62 characters to int64
func Base62ToInt64(s string) (int64, error) {
	const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	var result int64
	for i, c := range s {
		index := int64(strings.IndexByte(base62Chars, byte(c)))
		if index == -1 {
			return 0, fmt.Errorf("invalid character %c in base62 string", c)
		}
		result += index * int64(math.Pow(62, float64(len(s)-i-1)))
	}
	return result, nil
}
