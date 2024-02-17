package infra

import "strings"

// Maske secret or sensitive data.
// "ABCDEF", 2 -> "****EF"
func SecretStrMask(original string, tailSize int) string {
	if tailSize == 0 {
		return original
	}
	tmp := ([]rune)(strings.TrimSpace(original))

	n := len(tmp)
	if n == 0 {
		return ""
	}

	mask := "*"
	if n < tailSize {
		return strings.TrimSpace(strings.Repeat(mask, n))
	}

	return strings.TrimSpace(strings.Repeat(mask, n-tailSize) + string(tmp[n-tailSize:]))
}
