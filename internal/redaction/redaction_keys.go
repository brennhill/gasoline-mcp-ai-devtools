package redaction

import "strings"

// sensitiveKeyNames matches key names that indicate sensitive data.
// Values for these keys are always redacted regardless of content.
var sensitiveKeyNames = map[string]bool{
	"password":   true,
	"passwd":     true,
	"secret":     true,
	"token":      true,
	"ssn":        true,
	"creditcard": true,
	"cvv":        true,
	"cvc":        true,
	"auth":       true,
	"credential": true,
	"apikey":     true,
	"passcode":   true,
	"session":    true,
	"cookie":     true,
	"bearer":     true,
	"otp":        true,
}

// sensitiveKeyFragments catches common key-name variants (snake_case, kebab-case, camelCase).
var sensitiveKeyFragments = []string{
	"password",
	"passwd",
	"passcode",
	"token",
	"secret",
	"apikey",
	"auth",
	"credential",
	"session",
	"cookie",
	"bearer",
	"otp",
	"ssn",
	"creditcard",
	"cvv",
	"cvc",
}

func normalizeSensitiveKeyName(key string) string {
	key = strings.ToLower(key)
	var b strings.Builder
	b.Grow(len(key))
	for _, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isSensitiveKeyName(key string) bool {
	normalized := normalizeSensitiveKeyName(key)
	if normalized == "" {
		return false
	}
	if sensitiveKeyNames[normalized] {
		return true
	}
	for _, fragment := range sensitiveKeyFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
