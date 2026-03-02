package observe

import (
	"strings"
	"unicode"
)

// fingerprintMessage normalizes a log message into a stable fingerprint.
func fingerprintMessage(msg string) string {
	if msg == "" {
		return ""
	}
	s := msg
	s = reANSI.ReplaceAllString(s, "")
	s = reUUID.ReplaceAllString(s, "{uuid}")
	s = reTimestamp.ReplaceAllString(s, "{timestamp}")
	s = reURL.ReplaceAllString(s, "{url}")
	s = reHexHash.ReplaceAllString(s, "{hash}")
	s = reNumbers.ReplaceAllString(s, "{n}")
	s = reLongQuoted.ReplaceAllString(s, `"{string}"`)
	s = rePath.ReplaceAllString(s, "{path}")
	s = reWhitespace.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	return slugify(s)
}

// slugify converts a normalized message into a URL-safe slug.
func slugify(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return '_'
	}, s)
	s = reSlugDup.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if len(s) > maxFingerprintLen {
		s = s[:maxFingerprintLen]
	}
	return s
}
