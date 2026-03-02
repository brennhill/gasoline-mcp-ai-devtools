// Purpose: Parses Set-Cookie header strings into normalized attribute structs.
// Why: Shares cookie parsing across checks and diff computation without duplication.
// Docs: docs/features/feature/security-hardening/index.md

package security

import "strings"

// cookieAttrs represents parsed Set-Cookie attributes.
type cookieAttrs struct {
	Name     string
	HttpOnly bool
	Secure   bool
	SameSite string
}

func parseCookies(setCookieHeader string) []cookieAttrs {
	var cookies []cookieAttrs

	lines := strings.Split(setCookieHeader, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cookie := parseSingleCookie(line)
		cookies = append(cookies, cookie)
	}

	return cookies
}

func parseSingleCookie(raw string) cookieAttrs {
	parts := strings.Split(raw, ";")
	cookie := cookieAttrs{}

	if len(parts) > 0 {
		nameValue := strings.TrimSpace(parts[0])
		eqIdx := strings.Index(nameValue, "=")
		if eqIdx > 0 {
			cookie.Name = nameValue[:eqIdx]
		}
	}

	for _, part := range parts[1:] {
		attr := strings.TrimSpace(strings.ToLower(part))
		if attr == "httponly" {
			cookie.HttpOnly = true
		} else if attr == "secure" {
			cookie.Secure = true
		} else if strings.HasPrefix(attr, "samesite=") {
			cookie.SameSite = strings.TrimPrefix(attr, "samesite=")
		} else if attr == "samesite" {
			cookie.SameSite = "unspecified"
		}
	}

	return cookie
}
