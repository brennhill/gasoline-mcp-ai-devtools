// security_diff.go — Security Regression Detection (diff_security) MCP tool.
// Takes security posture snapshots and compares them to detect regressions
// in headers, cookies, auth patterns, and transport security.
// Design: Named snapshots stored in-memory with 4-hour TTL and max 5 slots.
// Comparison produces severity-rated regressions and improvements.
package security

import (
	"github.com/dev-console/dev-console/internal/capture"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ============================================
// Types
// ============================================

// SecurityDiffManager stores and compares security posture snapshots.
type SecurityDiffManager struct {
	mu        sync.RWMutex
	snapshots map[string]*SecuritySnapshot
	order     []string // insertion order for LRU eviction
	maxSnaps  int
	ttl       time.Duration
}

// SecuritySnapshot captures the security posture at a point in time.
type SecuritySnapshot struct {
	Name      string                       `json:"name"`
	TakenAt   time.Time                    `json:"taken_at"`
	Headers   map[string]map[string]string `json:"headers"`   // origin → headerName → value
	Cookies   map[string][]SecurityCookie  `json:"cookies"`   // origin → cookies
	Auth      map[string]bool              `json:"auth"`      // endpoint (method+url) → has_auth
	Transport map[string]string            `json:"transport"` // origin → "https" or "http"
}

// SecurityCookie records cookie attributes for comparison.
type SecurityCookie struct {
	Name     string `json:"name"`
	HttpOnly bool   `json:"httponly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"samesite"`
}

// SecurityDiffResult is the comparison response.
type SecurityDiffResult struct {
	Verdict      string              `json:"verdict"` // "regressed", "improved", "unchanged"
	Regressions  []SecurityChange    `json:"regressions"`
	Improvements []SecurityChange    `json:"improvements"`
	Summary      SecurityDiffSummary `json:"summary"`
}

// SecurityChange describes a single security posture change.
type SecurityChange struct {
	Category       string `json:"category"`                // "headers", "cookies", "auth", "transport"
	Severity       string `json:"severity"`                // "critical", "high", "warning", "info"
	Origin         string `json:"origin,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Change         string `json:"change"`                  // "header_removed", "header_added", "flag_removed", "flag_added", "auth_removed", "auth_added", "transport_downgrade", "transport_upgrade"
	Header         string `json:"header,omitempty"`
	CookieName     string `json:"cookie_name,omitempty"`
	Flag           string `json:"flag,omitempty"`
	Before         string `json:"before"`
	After          string `json:"after"`
	Recommendation string `json:"recommendation"`
}

// SecurityDiffSummary provides aggregate change counts.
type SecurityDiffSummary struct {
	TotalRegressions  int            `json:"total_regressions"`
	TotalImprovements int            `json:"total_improvements"`
	BySeverity        map[string]int `json:"by_severity"`
	ByCategory        map[string]int `json:"by_category"`
}

// SecuritySnapshotListEntry is a summary for the list response.
type SecuritySnapshotListEntry struct {
	Name    string `json:"name"`
	TakenAt string `json:"taken_at"`
	Age     string `json:"age"`
	Expired bool   `json:"expired"`
}

// ============================================
// Security headers tracked for diff comparison
// ============================================

var trackedSecurityHeaders = []string{
	"Strict-Transport-Security",
	"X-Content-Type-Options",
	"X-Frame-Options",
	"Content-Security-Policy",
	"Referrer-Policy",
	"Permissions-Policy",
}

// ============================================
// Constructor
// ============================================

// NewSecurityDiffManager creates a new SecurityDiffManager with defaults.
func NewSecurityDiffManager() *SecurityDiffManager {
	return &SecurityDiffManager{
		snapshots: make(map[string]*SecuritySnapshot),
		order:     make([]string, 0),
		maxSnaps:  5,
		ttl:       4 * time.Hour,
	}
}

// ============================================
// Snapshot Management
// ============================================

// TakeSnapshot captures the current security posture from network bodies.
func (m *SecurityDiffManager) TakeSnapshot(name string, bodies []capture.NetworkBody) (*SecuritySnapshot, error) {
	// Validate name
	if name == "" {
		return nil, fmt.Errorf("snapshot name cannot be empty")
	}
	if name == "current" {
		return nil, fmt.Errorf("snapshot name 'current' is reserved")
	}
	if len(name) > 50 {
		return nil, fmt.Errorf("snapshot name exceeds 50 characters")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// If name already exists, remove it from order (will be re-added)
	if _, exists := m.snapshots[name]; exists {
		m.removeFromOrder(name)
	}

	// Evict oldest if at capacity
	for len(m.order) >= m.maxSnaps {
		oldest := m.order[0]
		newOrder := make([]string, len(m.order)-1)
		copy(newOrder, m.order[1:])
		m.order = newOrder
		delete(m.snapshots, oldest)
	}

	// Build snapshot
	snap := &SecuritySnapshot{
		Name:      name,
		TakenAt:   time.Now(),
		Headers:   make(map[string]map[string]string),
		Cookies:   make(map[string][]SecurityCookie),
		Auth:      make(map[string]bool),
		Transport: make(map[string]string),
	}

	for _, body := range bodies {
		origin := extractSnapshotOrigin(body.URL)

		// Headers: only from HTML responses with ResponseHeaders
		if isHTMLResponse(body) && body.ResponseHeaders != nil {
			if snap.Headers[origin] == nil {
				snap.Headers[origin] = make(map[string]string)
			}
			for _, hdr := range trackedSecurityHeaders {
				if val, ok := body.ResponseHeaders[hdr]; ok && val != "" {
					snap.Headers[origin][hdr] = val
				}
			}
		}

		// Cookies: from Set-Cookie header
		if body.ResponseHeaders != nil {
			if setCookie, ok := body.ResponseHeaders["Set-Cookie"]; ok && setCookie != "" {
				cookies := parseSnapshotCookies(setCookie)
				if len(cookies) > 0 {
					snap.Cookies[origin] = append(snap.Cookies[origin], cookies...)
				}
			}
		}

		// Auth: method + url → has_auth
		endpoint := body.Method + " " + body.URL
		snap.Auth[endpoint] = body.HasAuthHeader

		// Transport: origin → scheme
		if scheme := extractScheme(body.URL); scheme != "" {
			snap.Transport[origin] = scheme
		}
	}

	m.snapshots[name] = snap
	m.order = append(m.order, name)

	return snap, nil
}

// Compare compares two snapshots and returns regressions and improvements.
func (m *SecurityDiffManager) Compare(fromName, toName string, currentBodies []capture.NetworkBody) (*SecurityDiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Load "from" snapshot
	fromSnap, ok := m.snapshots[fromName]
	if !ok {
		return nil, fmt.Errorf("snapshot %q not found", fromName)
	}
	if m.isExpired(fromSnap) {
		return nil, fmt.Errorf("snapshot %q has expired (TTL: %v)", fromName, m.ttl)
	}

	// Load "to" snapshot
	var toSnap *SecuritySnapshot
	if toName == "" || toName == "current" {
		// Build ephemeral snapshot from currentBodies
		var err error
		toSnap, err = m.buildEphemeralSnapshot(currentBodies)
		if err != nil {
			return nil, fmt.Errorf("failed to build current snapshot: %w", err)
		}
	} else {
		var exists bool
		toSnap, exists = m.snapshots[toName]
		if !exists {
			return nil, fmt.Errorf("snapshot %q not found", toName)
		}
		if m.isExpired(toSnap) {
			return nil, fmt.Errorf("snapshot %q has expired (TTL: %v)", toName, m.ttl)
		}
	}

	// Compare
	var regressions []SecurityChange
	var improvements []SecurityChange

	// Header comparison
	headerReg, headerImp := m.compareHeaders(fromSnap, toSnap)
	regressions = append(regressions, headerReg...)
	improvements = append(improvements, headerImp...)

	// Cookie comparison
	cookieReg, cookieImp := m.compareCookies(fromSnap, toSnap)
	regressions = append(regressions, cookieReg...)
	improvements = append(improvements, cookieImp...)

	// Auth comparison
	authReg, authImp := m.compareAuth(fromSnap, toSnap)
	regressions = append(regressions, authReg...)
	improvements = append(improvements, authImp...)

	// Transport comparison
	transReg, transImp := m.compareTransport(fromSnap, toSnap)
	regressions = append(regressions, transReg...)
	improvements = append(improvements, transImp...)

	// Determine verdict
	verdict := "unchanged"
	if len(regressions) > 0 {
		verdict = "regressed"
	} else if len(improvements) > 0 {
		verdict = "improved"
	}

	// Build summary
	summary := buildDiffSummary(regressions, improvements)

	return &SecurityDiffResult{
		Verdict:      verdict,
		Regressions:  regressions,
		Improvements: improvements,
		Summary:      summary,
	}, nil
}

// ListSnapshots returns a summary of all stored snapshots.
func (m *SecurityDiffManager) ListSnapshots() []SecuritySnapshotListEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]SecuritySnapshotListEntry, 0, len(m.order))
	for _, name := range m.order {
		snap, ok := m.snapshots[name]
		if !ok {
			continue
		}
		entries = append(entries, SecuritySnapshotListEntry{
			Name:    snap.Name,
			TakenAt: snap.TakenAt.Format(time.RFC3339),
			Age:     formatDuration(time.Since(snap.TakenAt)),
			Expired: m.isExpired(snap),
		})
	}
	return entries
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleDiffSecurity processes MCP tool call parameters and dispatches to the appropriate action.
func (m *SecurityDiffManager) HandleDiffSecurity(params json.RawMessage, bodies []capture.NetworkBody) (any, error) {
	var toolParams struct {
		Action      string `json:"action"`
		Name        string `json:"name"`
		CompareFrom string `json:"compare_from"`
		CompareTo   string `json:"compare_to"`
	}
	if len(params) > 0 {
		if err := json.Unmarshal(params, &toolParams); err != nil {
			return nil, fmt.Errorf("invalid parameters: %w", err)
		}
	}

	switch toolParams.Action {
	case "snapshot":
		return m.TakeSnapshot(toolParams.Name, bodies)
	case "compare":
		return m.Compare(toolParams.CompareFrom, toolParams.CompareTo, bodies)
	case "list":
		return m.ListSnapshots(), nil
	default:
		return nil, fmt.Errorf("unknown action %q; use 'snapshot', 'compare', or 'list'", toolParams.Action)
	}
}

// ============================================
// Comparison Helpers
// ============================================

func (m *SecurityDiffManager) compareHeaders(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	// Collect all origins from both snapshots
	origins := make(map[string]bool)
	for origin := range from.Headers {
		origins[origin] = true
	}
	for origin := range to.Headers {
		origins[origin] = true
	}

	for origin := range origins {
		fromHeaders := from.Headers[origin]
		toHeaders := to.Headers[origin]

		if fromHeaders == nil {
			fromHeaders = make(map[string]string)
		}
		if toHeaders == nil {
			toHeaders = make(map[string]string)
		}

		// Headers in "from" but not in "to" → regression
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
	}

	return regressions, improvements
}

func (m *SecurityDiffManager) compareCookies(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	// Collect all origins
	origins := make(map[string]bool)
	for origin := range from.Cookies {
		origins[origin] = true
	}
	for origin := range to.Cookies {
		origins[origin] = true
	}

	for origin := range origins {
		fromCookies := from.Cookies[origin]
		toCookies := to.Cookies[origin]

		// Build maps by cookie name
		fromMap := make(map[string]SecurityCookie)
		for _, c := range fromCookies {
			fromMap[c.Name] = c
		}
		toMap := make(map[string]SecurityCookie)
		for _, c := range toCookies {
			toMap[c.Name] = c
		}

		// Compare matching cookies
		for name, fromCookie := range fromMap {
			toCookie, exists := toMap[name]
			if !exists {
				continue // Cookie removed entirely — not tracked as flag change
			}

			// HttpOnly flag
			if fromCookie.HttpOnly && !toCookie.HttpOnly {
				regressions = append(regressions, SecurityChange{
					Category:       "cookies",
					Severity:       "warning",
					Origin:         origin,
					Change:         "flag_removed",
					CookieName:     name,
					Flag:           "HttpOnly",
					Before:         "present",
					After:          "absent",
					Recommendation: fmt.Sprintf("Cookie '%s' lost HttpOnly flag. Client-side JavaScript can now read it.", name),
				})
			} else if !fromCookie.HttpOnly && toCookie.HttpOnly {
				improvements = append(improvements, SecurityChange{
					Category:       "cookies",
					Severity:       "info",
					Origin:         origin,
					Change:         "flag_added",
					CookieName:     name,
					Flag:           "HttpOnly",
					Before:         "absent",
					After:          "present",
					Recommendation: fmt.Sprintf("Cookie '%s' gained HttpOnly flag.", name),
				})
			}

			// Secure flag
			if fromCookie.Secure && !toCookie.Secure {
				regressions = append(regressions, SecurityChange{
					Category:       "cookies",
					Severity:       "warning",
					Origin:         origin,
					Change:         "flag_removed",
					CookieName:     name,
					Flag:           "Secure",
					Before:         "present",
					After:          "absent",
					Recommendation: fmt.Sprintf("Cookie '%s' lost Secure flag. Cookie can now be sent over HTTP.", name),
				})
			} else if !fromCookie.Secure && toCookie.Secure {
				improvements = append(improvements, SecurityChange{
					Category:       "cookies",
					Severity:       "info",
					Origin:         origin,
					Change:         "flag_added",
					CookieName:     name,
					Flag:           "Secure",
					Before:         "absent",
					After:          "present",
					Recommendation: fmt.Sprintf("Cookie '%s' gained Secure flag.", name),
				})
			}

			// SameSite flag
			if fromCookie.SameSite != "" && toCookie.SameSite == "" {
				regressions = append(regressions, SecurityChange{
					Category:       "cookies",
					Severity:       "warning",
					Origin:         origin,
					Change:         "flag_removed",
					CookieName:     name,
					Flag:           "SameSite",
					Before:         fromCookie.SameSite,
					After:          "",
					Recommendation: fmt.Sprintf("Cookie '%s' lost SameSite flag. Cookie may now be sent in cross-site requests.", name),
				})
			} else if fromCookie.SameSite == "" && toCookie.SameSite != "" {
				improvements = append(improvements, SecurityChange{
					Category:       "cookies",
					Severity:       "info",
					Origin:         origin,
					Change:         "flag_added",
					CookieName:     name,
					Flag:           "SameSite",
					Before:         "",
					After:          toCookie.SameSite,
					Recommendation: fmt.Sprintf("Cookie '%s' gained SameSite flag.", name),
				})
			}
		}
	}

	return regressions, improvements
}

func (m *SecurityDiffManager) compareAuth(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	// Check all endpoints from both snapshots
	endpoints := make(map[string]bool)
	for ep := range from.Auth {
		endpoints[ep] = true
	}
	for ep := range to.Auth {
		endpoints[ep] = true
	}

	for endpoint := range endpoints {
		fromAuth := from.Auth[endpoint]
		toAuth := to.Auth[endpoint]

		if fromAuth && !toAuth {
			regressions = append(regressions, SecurityChange{
				Category:       "auth",
				Severity:       "critical",
				Endpoint:       endpoint,
				Change:         "auth_removed",
				Before:         "authenticated",
				After:          "unauthenticated",
				Recommendation: "This endpoint previously required authentication but no longer does. Verify this is intentional.",
			})
		} else if !fromAuth && toAuth {
			improvements = append(improvements, SecurityChange{
				Category:       "auth",
				Severity:       "info",
				Endpoint:       endpoint,
				Change:         "auth_added",
				Before:         "unauthenticated",
				After:          "authenticated",
				Recommendation: "This endpoint now requires authentication.",
			})
		}
	}

	return regressions, improvements
}

func (m *SecurityDiffManager) compareTransport(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	// Collect all origins
	origins := make(map[string]bool)
	for origin := range from.Transport {
		origins[origin] = true
	}
	for origin := range to.Transport {
		origins[origin] = true
	}

	// For transport comparison, we need to normalize origins to just host
	// since the scheme is the thing that changes
	fromByHost := make(map[string]string) // host → scheme
	toByHost := make(map[string]string)

	for origin, scheme := range from.Transport {
		host := extractHostFromOrigin(origin)
		if host != "" {
			fromByHost[host] = scheme
		}
	}
	for origin, scheme := range to.Transport {
		host := extractHostFromOrigin(origin)
		if host != "" {
			toByHost[host] = scheme
		}
	}

	hosts := make(map[string]bool)
	for h := range fromByHost {
		hosts[h] = true
	}
	for h := range toByHost {
		hosts[h] = true
	}

	for host := range hosts {
		fromScheme := fromByHost[host]
		toScheme := toByHost[host]

		if fromScheme == "https" && toScheme == "http" {
			regressions = append(regressions, SecurityChange{
				Category:       "transport",
				Severity:       "high",
				Origin:         host,
				Change:         "transport_downgrade",
				Before:         "https",
				After:          "http",
				Recommendation: "Origin downgraded from HTTPS to HTTP. Data in transit can be intercepted.",
			})
		} else if fromScheme == "http" && toScheme == "https" {
			improvements = append(improvements, SecurityChange{
				Category:       "transport",
				Severity:       "info",
				Origin:         host,
				Change:         "transport_upgrade",
				Before:         "http",
				After:          "https",
				Recommendation: "Origin upgraded from HTTP to HTTPS.",
			})
		}
	}

	return regressions, improvements
}

// ============================================
// Internal Helpers
// ============================================

func (m *SecurityDiffManager) isExpired(snap *SecuritySnapshot) bool {
	return time.Since(snap.TakenAt) > m.ttl
}

func (m *SecurityDiffManager) removeFromOrder(name string) {
	for i, n := range m.order {
		if n == name {
			newOrder := make([]string, len(m.order)-1)
			copy(newOrder, m.order[:i])
			copy(newOrder[i:], m.order[i+1:])
			m.order = newOrder
			return
		}
	}
}

func (m *SecurityDiffManager) buildEphemeralSnapshot(bodies []capture.NetworkBody) (*SecuritySnapshot, error) {
	snap := &SecuritySnapshot{
		Name:      "current",
		TakenAt:   time.Now(),
		Headers:   make(map[string]map[string]string),
		Cookies:   make(map[string][]SecurityCookie),
		Auth:      make(map[string]bool),
		Transport: make(map[string]string),
	}

	for _, body := range bodies {
		origin := extractSnapshotOrigin(body.URL)

		// Headers
		if isHTMLResponse(body) && body.ResponseHeaders != nil {
			if snap.Headers[origin] == nil {
				snap.Headers[origin] = make(map[string]string)
			}
			for _, hdr := range trackedSecurityHeaders {
				if val, ok := body.ResponseHeaders[hdr]; ok && val != "" {
					snap.Headers[origin][hdr] = val
				}
			}
		}

		// Cookies
		if body.ResponseHeaders != nil {
			if setCookie, ok := body.ResponseHeaders["Set-Cookie"]; ok && setCookie != "" {
				cookies := parseSnapshotCookies(setCookie)
				if len(cookies) > 0 {
					snap.Cookies[origin] = append(snap.Cookies[origin], cookies...)
				}
			}
		}

		// Auth
		endpoint := body.Method + " " + body.URL
		snap.Auth[endpoint] = body.HasAuthHeader

		// Transport
		if scheme := extractScheme(body.URL); scheme != "" {
			snap.Transport[origin] = scheme
		}
	}

	return snap, nil
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
	switch header {
	case "X-Frame-Options":
		return "X-Frame-Options was present before but is now missing. This exposes the app to clickjacking."
	case "Strict-Transport-Security":
		return "Strict-Transport-Security was present before but is now missing. This exposes the app to MITM downgrade."
	case "X-Content-Type-Options":
		return "X-Content-Type-Options was present before but is now missing. This exposes the app to MIME sniffing."
	case "Content-Security-Policy":
		return "Content-Security-Policy was present before but is now missing. This exposes the app to XSS."
	case "Referrer-Policy":
		return "Referrer-Policy was present before but is now missing. This exposes the app to referrer leakage."
	case "Permissions-Policy":
		return "Permissions-Policy was present before but is now missing. This exposes the app to feature abuse."
	default:
		return fmt.Sprintf("%s was present before but is now missing.", header)
	}
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

