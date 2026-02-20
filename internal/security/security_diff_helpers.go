// security_diff_helpers.go — Diff comparison helpers for security posture snapshots.
// Contains per-origin header/cookie/transport diffing, cookie flag analysis,
// key collection utilities, URL parsing helpers, and summary building.
package security

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

func diffHeadersForOrigin(origin string, fromHeaders, toHeaders map[string]string) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	for _, hdr := range trackedSecurityHeaders {
		fromVal, fromHas := fromHeaders[hdr]
		toVal, toHas := toHeaders[hdr]

		if fromHas && !toHas {
			regressions = append(regressions, SecurityChange{
				Category:       "headers",
				Severity:       "warning",
				Origin:         origin,
				Change:         "header_removed",
				Header:         hdr,
				Before:         fromVal,
				After:          "",
				Recommendation: headerRemovedRecommendation(hdr),
			})
		} else if !fromHas && toHas {
			improvements = append(improvements, SecurityChange{
				Category:       "headers",
				Severity:       "info",
				Origin:         origin,
				Change:         "header_added",
				Header:         hdr,
				Before:         "",
				After:          toVal,
				Recommendation: fmt.Sprintf("%s header has been added, improving security posture.", hdr),
			})
		}
	}

	return regressions, improvements
}

type cookieFlagSpec struct {
	flagName   string
	fromActive bool
	toActive   bool
	fromVal    string
	toVal      string
	lostMsg    string
	gainedMsg  string
}

func diffCookieFlags(origin, name string, from, to SecurityCookie) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	flags := []cookieFlagSpec{
		{
			flagName: "HttpOnly", fromActive: from.HttpOnly, toActive: to.HttpOnly,
			fromVal: "present", toVal: "present",
			lostMsg:   fmt.Sprintf("Cookie '%s' lost HttpOnly flag. Client-side JavaScript can now read it.", name),
			gainedMsg: fmt.Sprintf("Cookie '%s' gained HttpOnly flag.", name),
		},
		{
			flagName: "Secure", fromActive: from.Secure, toActive: to.Secure,
			fromVal: "present", toVal: "present",
			lostMsg:   fmt.Sprintf("Cookie '%s' lost Secure flag. Cookie can now be sent over HTTP.", name),
			gainedMsg: fmt.Sprintf("Cookie '%s' gained Secure flag.", name),
		},
		{
			flagName: "SameSite", fromActive: from.SameSite != "", toActive: to.SameSite != "",
			fromVal: from.SameSite, toVal: to.SameSite,
			lostMsg:   fmt.Sprintf("Cookie '%s' lost SameSite flag. Cookie may now be sent in cross-site requests.", name),
			gainedMsg: fmt.Sprintf("Cookie '%s' gained SameSite flag.", name),
		},
	}

	for _, f := range flags {
		change := diffSingleCookieFlag(origin, name, f)
		if change == nil {
			continue
		}
		if change.Change == "flag_removed" {
			regressions = append(regressions, *change)
		} else {
			improvements = append(improvements, *change)
		}
	}

	return regressions, improvements
}

func diffSingleCookieFlag(origin, cookieName string, f cookieFlagSpec) *SecurityChange {
	if f.fromActive && !f.toActive {
		before := f.fromVal
		if f.flagName != "SameSite" {
			before = "present"
		}
		return &SecurityChange{
			Category: "cookies", Severity: "warning", Origin: origin,
			Change: "flag_removed", CookieName: cookieName, Flag: f.flagName,
			Before: before, After: flagAbsentValue(f.flagName, ""),
			Recommendation: f.lostMsg,
		}
	}
	if !f.fromActive && f.toActive {
		after := f.toVal
		if f.flagName != "SameSite" {
			after = "present"
		}
		return &SecurityChange{
			Category: "cookies", Severity: "info", Origin: origin,
			Change: "flag_added", CookieName: cookieName, Flag: f.flagName,
			Before: flagAbsentValue(f.flagName, ""), After: after,
			Recommendation: f.gainedMsg,
		}
	}
	return nil
}

func flagAbsentValue(flagName, fallback string) string {
	if flagName == "SameSite" {
		return fallback
	}
	return "absent"
}

func normalizeTransportByHost(transport map[string]string) map[string]string {
	byHost := make(map[string]string, len(transport))
	for origin, scheme := range transport {
		host := extractHostFromOrigin(origin)
		if host != "" {
			byHost[host] = scheme
		}
	}
	return byHost
}

func cookieSliceToMap(cookies []SecurityCookie) map[string]SecurityCookie {
	m := make(map[string]SecurityCookie, len(cookies))
	for _, c := range cookies {
		m[c.Name] = c
	}
	return m
}

func collectMapKeys[V any](a, b map[string]map[string]V) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectCookieMapKeys(a, b map[string][]SecurityCookie) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectBoolMapKeys(a, b map[string]bool) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func collectStringMapKeys(a, b map[string]string) map[string]bool {
	keys := make(map[string]bool, len(a)+len(b))
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	return keys
}

func extractSnapshotOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host
}

func extractScheme(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme
}

func extractHostFromOrigin(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return origin
	}
	return parsed.Host
}

func parseSnapshotCookies(setCookieHeader string) []SecurityCookie {
	var cookies []SecurityCookie

	lines := strings.Split(setCookieHeader, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parsed := parseSingleCookie(line)
		cookies = append(cookies, SecurityCookie(parsed))
	}

	return cookies
}

func headerRemovedRecommendation(header string) string {
	if rec, ok := headerRemovedRecommendations[header]; ok {
		return rec
	}
	return fmt.Sprintf("%s was present before but is now missing.", header)
}

func buildDiffSummary(regressions, improvements []SecurityChange) SecurityDiffSummary {
	bySeverity := make(map[string]int)
	byCategory := make(map[string]int)

	for _, r := range regressions {
		bySeverity[r.Severity]++
		byCategory[r.Category]++
	}

	return SecurityDiffSummary{
		TotalRegressions:  len(regressions),
		TotalImprovements: len(improvements),
		BySeverity:        bySeverity,
		ByCategory:        byCategory,
	}
}

// formatDuration converts a time.Duration to a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		if secs == 0 {
			return fmt.Sprintf("%dm", mins)
		}
		return fmt.Sprintf("%dm%02ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh%dm", hours, mins)
}
