package domain

import (
	"fmt"
	"strings"
)

// NormalizePhone converts a phone number to a simplified E.164 form. Ten-digit
// numbers are assumed to be North American (+1). This is intentionally
// lightweight; production code would use libphonenumber.
func NormalizePhone(raw string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		default:
			return "", fmt.Errorf("%w: phone contains invalid character", ErrInvalidInput)
		}
	}
	digits := b.String()
	switch {
	case len(digits) == 10:
		return "+1" + digits, nil
	case len(digits) >= 11 && len(digits) <= 15:
		return "+" + digits, nil
	default:
		return "", fmt.Errorf("%w: phone must have 10-15 digits", ErrInvalidInput)
	}
}
