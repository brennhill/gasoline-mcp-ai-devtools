package redaction

import "strings"

// luhnValid checks if a numeric string passes the Luhn algorithm.
func luhnValid(number string) bool {
	// Strip non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, number)

	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if alt {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alt = !alt
	}
	return sum%10 == 0
}

// luhnValidateMatch is the validation function used by the credit-card pattern.
func luhnValidateMatch(match string) bool {
	return luhnValid(match)
}
