package security

import "strings"

// redactSecret masks a secret value, showing only the first 6 and last 3 characters.
func redactSecret(s string) string {
	if len(s) <= 6 {
		if len(s) <= 3 {
			return s + "***"
		}
		return s[:3] + "***"
	}
	if len(s) <= 10 {
		return s[:6] + "***"
	}
	return s[:6] + "***" + s[len(s)-3:]
}

func isTestKey(value string) bool {
	lower := strings.ToLower(value)
	for _, indicator := range testKeyIndicators {
		if strings.Contains(lower, indicator) {
			return true
		}
	}
	return false
}

func getEntryString(entry LogEntry, key string) string {
	val, ok := entry[key]
	if !ok || val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}
