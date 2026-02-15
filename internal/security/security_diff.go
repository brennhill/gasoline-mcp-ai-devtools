// security_diff.go — Security Regression Detection (diff_security) MCP tool.
// Takes security posture snapshots and compares them to detect regressions
// in headers, cookies, auth patterns, and transport security.
// Design: Named snapshots stored in-memory with 4-hour TTL and max 5 slots.
// Comparison produces severity-rated regressions and improvements.
package security

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
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
	Headers   map[string]map[string]string `json:"headers"`   // origin -> headerName -> value
	Cookies   map[string][]SecurityCookie  `json:"cookies"`   // origin -> cookies
	Auth      map[string]bool              `json:"auth"`      // endpoint (method+url) -> has_auth
	Transport map[string]string            `json:"transport"` // origin -> "https" or "http"
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
	Category       string `json:"category"`            // "headers", "cookies", "auth", "transport"
	Severity       string `json:"severity"`            // "critical", "high", "warning", "info"
	Origin         string `json:"origin,omitempty"`
	Endpoint       string `json:"endpoint,omitempty"`
	Change         string `json:"change"`              // "header_removed", "header_added", etc.
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

var headerRemovedRecommendations = map[string]string{
	"X-Frame-Options":           "X-Frame-Options was present before but is now missing. This exposes the app to clickjacking.",
	"Strict-Transport-Security": "Strict-Transport-Security was present before but is now missing. This exposes the app to MITM downgrade.",
	"X-Content-Type-Options":    "X-Content-Type-Options was present before but is now missing. This exposes the app to MIME sniffing.",
	"Content-Security-Policy":   "Content-Security-Policy was present before but is now missing. This exposes the app to XSS.",
	"Referrer-Policy":           "Referrer-Policy was present before but is now missing. This exposes the app to referrer leakage.",
	"Permissions-Policy":        "Permissions-Policy was present before but is now missing. This exposes the app to feature abuse.",
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
	if err := validateSnapshotName(name); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.snapshots[name]; exists {
		m.removeFromOrder(name)
	}
	m.evictOldest()

	snap := newEmptySnapshot(name)
	populateSnapshotFromBodies(snap, bodies)

	m.snapshots[name] = snap
	m.order = append(m.order, name)

	return snap, nil
}

// Compare compares two snapshots and returns regressions and improvements.
func (m *SecurityDiffManager) Compare(fromName, toName string, currentBodies []capture.NetworkBody) (*SecurityDiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fromSnap, err := m.resolveSnapshot(fromName)
	if err != nil {
		return nil, err
	}

	toSnap, err := m.resolveToSnapshot(toName, currentBodies)
	if err != nil {
		return nil, err
	}

	regressions, improvements := m.collectAllChanges(fromSnap, toSnap)
	verdict := determineVerdict(regressions, improvements)
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
	origins := collectMapKeys(from.Headers, to.Headers)

	for origin := range origins {
		fromHeaders := from.Headers[origin]
		toHeaders := to.Headers[origin]
		if fromHeaders == nil {
			fromHeaders = make(map[string]string)
		}
		if toHeaders == nil {
			toHeaders = make(map[string]string)
		}

		reg, imp := diffHeadersForOrigin(origin, fromHeaders, toHeaders)
		regressions = append(regressions, reg...)
		improvements = append(improvements, imp...)
	}

	return regressions, improvements
}

func (m *SecurityDiffManager) compareCookies(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange
	origins := collectCookieMapKeys(from.Cookies, to.Cookies)

	for origin := range origins {
		fromMap := cookieSliceToMap(from.Cookies[origin])
		toMap := cookieSliceToMap(to.Cookies[origin])

		for name, fromCookie := range fromMap {
			toCookie, exists := toMap[name]
			if !exists {
				continue
			}
			reg, imp := diffCookieFlags(origin, name, fromCookie, toCookie)
			regressions = append(regressions, reg...)
			improvements = append(improvements, imp...)
		}
	}

	return regressions, improvements
}

func (m *SecurityDiffManager) compareAuth(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange
	endpoints := collectBoolMapKeys(from.Auth, to.Auth)

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

	fromByHost := normalizeTransportByHost(from.Transport)
	toByHost := normalizeTransportByHost(to.Transport)
	hosts := collectStringMapKeys(fromByHost, toByHost)

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

func validateSnapshotName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if name == "current" {
		return fmt.Errorf("snapshot name 'current' is reserved")
	}
	if len(name) > 50 {
		return fmt.Errorf("snapshot name exceeds 50 characters")
	}
	return nil
}

func newEmptySnapshot(name string) *SecuritySnapshot {
	return &SecuritySnapshot{
		Name:      name,
		TakenAt:   time.Now(),
		Headers:   make(map[string]map[string]string),
		Cookies:   make(map[string][]SecurityCookie),
		Auth:      make(map[string]bool),
		Transport: make(map[string]string),
	}
}

func populateSnapshotFromBodies(snap *SecuritySnapshot, bodies []capture.NetworkBody) {
	for _, body := range bodies {
		origin := extractSnapshotOrigin(body.URL)
		populateHeaders(snap, origin, body)
		populateCookies(snap, origin, body)
		snap.Auth[body.Method+" "+body.URL] = body.HasAuthHeader
		if scheme := extractScheme(body.URL); scheme != "" {
			snap.Transport[origin] = scheme
		}
	}
}

func populateHeaders(snap *SecuritySnapshot, origin string, body capture.NetworkBody) {
	if !isHTMLResponse(body) || body.ResponseHeaders == nil {
		return
	}
	if snap.Headers[origin] == nil {
		snap.Headers[origin] = make(map[string]string)
	}
	for _, hdr := range trackedSecurityHeaders {
		if val, ok := body.ResponseHeaders[hdr]; ok && val != "" {
			snap.Headers[origin][hdr] = val
		}
	}
}

func populateCookies(snap *SecuritySnapshot, origin string, body capture.NetworkBody) {
	if body.ResponseHeaders == nil {
		return
	}
	setCookie, ok := body.ResponseHeaders["Set-Cookie"]
	if !ok || setCookie == "" {
		return
	}
	cookies := parseSnapshotCookies(setCookie)
	if len(cookies) > 0 {
		snap.Cookies[origin] = append(snap.Cookies[origin], cookies...)
	}
}

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

func (m *SecurityDiffManager) evictOldest() {
	for len(m.order) >= m.maxSnaps {
		oldest := m.order[0]
		newOrder := make([]string, len(m.order)-1)
		copy(newOrder, m.order[1:])
		m.order = newOrder
		delete(m.snapshots, oldest)
	}
}

func (m *SecurityDiffManager) resolveSnapshot(name string) (*SecuritySnapshot, error) {
	snap, ok := m.snapshots[name]
	if !ok {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}
	if m.isExpired(snap) {
		return nil, fmt.Errorf("snapshot %q has expired (TTL: %v)", name, m.ttl)
	}
	return snap, nil
}

func (m *SecurityDiffManager) resolveToSnapshot(toName string, currentBodies []capture.NetworkBody) (*SecuritySnapshot, error) {
	if toName == "" || toName == "current" {
		snap := newEmptySnapshot("current")
		populateSnapshotFromBodies(snap, currentBodies)
		return snap, nil
	}
	return m.resolveSnapshot(toName)
}

func (m *SecurityDiffManager) collectAllChanges(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	compareFns := []func(*SecuritySnapshot, *SecuritySnapshot) ([]SecurityChange, []SecurityChange){
		m.compareHeaders,
		m.compareCookies,
		m.compareAuth,
		m.compareTransport,
	}
	for _, fn := range compareFns {
		reg, imp := fn(from, to)
		regressions = append(regressions, reg...)
		improvements = append(improvements, imp...)
	}

	return regressions, improvements
}

func determineVerdict(regressions, improvements []SecurityChange) string {
	if len(regressions) > 0 {
		return "regressed"
	}
	if len(improvements) > 0 {
		return "improved"
	}
	return "unchanged"
}

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

