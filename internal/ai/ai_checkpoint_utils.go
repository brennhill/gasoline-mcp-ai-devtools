package ai

import (
	"regexp"
	"unicode/utf8"
)

var (
	uuidRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	largeNumberRe  = regexp.MustCompile(`\b\d{4,}\b`)
	isoTimestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`)
)

func FingerprintMessage(msg string) string {
	result := uuidRegex.ReplaceAllString(msg, "{uuid}")
	result = isoTimestampRe.ReplaceAllString(result, "{ts}")
	result = largeNumberRe.ReplaceAllString(result, "{n}")
	return result
}

func truncateMessage(msg string) string {
	if len(msg) <= maxMessageLen {
		return msg
	}
	truncated := msg[:maxMessageLen]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func recentSlice(available, newCount int) int {
	if newCount <= 0 {
		return -1
	}
	if newCount > available {
		return available
	}
	return newCount
}
