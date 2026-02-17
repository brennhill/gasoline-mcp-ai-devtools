// Purpose: Owns security.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// security.go — Security Scanner (security_audit) MCP tool.
// Analyzes captured browser data to detect exposed credentials, PII leakage,
// missing security headers, insecure cookies, insecure transport, and
// missing authentication patterns.
// Design: SecurityScanner is a standalone struct with no external dependencies.
// All analysis operates on data already captured by Gasoline buffers.
package security

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Types
// ============================================

// SecurityFinding represents a single security issue detected by the scanner.
type SecurityFinding struct {
	Check       string `json:"check"`       // "credentials", "pii", "headers", "cookies", "transport", "auth"
	Severity    string `json:"severity"`    // "critical", "high", "medium", "low", "info"
	Title       string `json:"title"`
	Description string `json:"description"`
	Location    string `json:"location"`    // URL or header name where found
	Evidence    string `json:"evidence"`    // Redacted snippet showing the match
	Remediation string `json:"remediation"` // How to fix
}

// LogEntry represents a console log entry as a map of string to any
type LogEntry map[string]any

// SecurityScanInput contains the data to scan.
type SecurityScanInput struct {
	NetworkBodies    []capture.NetworkBody
	WaterfallEntries []capture.NetworkWaterfallEntry
	ConsoleEntries   []LogEntry
	PageURLs         []string
	URLFilter        string
	Checks           []string // Which checks to run; empty = all
	SeverityMin      string   // Minimum severity to report
}

// SecurityScanResult contains all findings from a scan.
type SecurityScanResult struct {
	Findings  []SecurityFinding `json:"findings"`
	Summary   ScanSummary       `json:"summary"`
	ScannedAt time.Time         `json:"scanned_at"`
}

// ScanSummary provides aggregate counts of findings.
type ScanSummary struct {
	TotalFindings int            `json:"total_findings"`
	BySeverity    map[string]int `json:"by_severity"`
	ByCheck       map[string]int `json:"by_check"`
	URLsScanned   int            `json:"urls_scanned"`
}

// SecurityScanner performs security analysis on captured browser data.
type SecurityScanner struct {
	mu sync.RWMutex
}

// capture.NetworkBody extension fields — these are added to the existing NetworkBody
// via a wrapper since we can't modify types.go without broader impact.
// The test uses the extended fields directly on capture.NetworkBody.

// ============================================
// Constructor
// ============================================

// NewSecurityScanner creates a new SecurityScanner instance.
func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{}
}

// ============================================
// Main Scan Entry Point
// ============================================

// defaultSecurityChecks is the full list of checks when none are specified.
var defaultSecurityChecks = []string{"credentials", "pii", "headers", "cookies", "transport", "auth", "network"}

// runSecurityChecks dispatches each enabled check and collects findings.
func (s *SecurityScanner) runSecurityChecks(checkSet map[string]bool, bodies []capture.NetworkBody, input SecurityScanInput) []SecurityFinding {
	type checkEntry struct {
		name string
		fn   func() []SecurityFinding
	}
	checks := []checkEntry{
		{"credentials", func() []SecurityFinding { return s.checkCredentials(bodies, input.ConsoleEntries) }},
		{"pii", func() []SecurityFinding { return s.checkPII(bodies, input.PageURLs) }},
		{"headers", func() []SecurityFinding { return s.checkSecurityHeaders(bodies) }},
		{"cookies", func() []SecurityFinding { return s.checkCookies(bodies) }},
		{"transport", func() []SecurityFinding { return s.checkTransport(bodies, input.PageURLs) }},
		{"auth", func() []SecurityFinding { return s.checkAuthPatterns(bodies) }},
		{"network", func() []SecurityFinding { return s.checkNetworkSecurity(input.WaterfallEntries, input.PageURLs) }},
	}

	var findings []SecurityFinding
	for _, c := range checks {
		if checkSet[c.name] {
			findings = append(findings, c.fn()...)
		}
	}
	return findings
}

// Scan analyzes the input data and returns security findings.
func (s *SecurityScanner) Scan(input SecurityScanInput) SecurityScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	checks := input.Checks
	if len(checks) == 0 {
		checks = defaultSecurityChecks
	}
	checkSet := make(map[string]bool)
	for _, c := range checks {
		checkSet[c] = true
	}

	bodies := filterBodiesByURL(input.NetworkBodies, input.URLFilter)
	findings := s.runSecurityChecks(checkSet, bodies, input)

	if input.SeverityMin != "" {
		findings = filterBySeverity(findings, input.SeverityMin)
	}

	return SecurityScanResult{
		Findings:  findings,
		Summary:   buildSummary(findings, bodies),
		ScannedAt: time.Now(),
	}
}

// ============================================
// MCP Tool Handler
// ============================================

// checkNetworkSecurity analyzes waterfall entries for suspicious origins, typosquatting,
// mixed content, non-standard ports, and IP address origins.
func (s *SecurityScanner) checkNetworkSecurity(entries []capture.NetworkWaterfallEntry, pageURLs []string) []SecurityFinding {
	var findings []SecurityFinding
	pageURL := ""
	if len(pageURLs) > 0 {
		pageURL = pageURLs[0]
	}

	for _, entry := range entries {
		flags := analyzeNetworkSecurity(entry, pageURL)
		for _, flag := range flags {
			findings = append(findings, SecurityFinding{
				Check:       "network",
				Severity:    flag.Severity,
				Title:       flag.Message,
				Description: networkFlagDescription(flag.Type),
				Location:    flag.Resource,
				Evidence:    flag.Origin,
				Remediation: networkFlagRemediation(flag.Type),
			})
		}
	}
	return findings
}

func networkFlagDescription(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Resource loaded from a TLD with elevated abuse rates. May indicate a supply chain attack or compromised dependency."
	case "non_standard_port":
		return "Resource loaded from a non-standard port, which may indicate compromised or temporary infrastructure."
	case "mixed_content":
		return "HTTP resource loaded on an HTTPS page. An attacker on the network can modify this resource."
	case "ip_address_origin":
		return "Resource loaded from an IP address instead of a domain name. May indicate compromised or ephemeral infrastructure."
	case "potential_typosquatting":
		return "Domain is suspiciously similar to a popular CDN or service. May be a typosquatting attack."
	default:
		return "Suspicious network origin detected."
	}
}

func networkFlagRemediation(flagType string) string {
	switch flagType {
	case "suspicious_tld":
		return "Verify the domain is legitimate. Consider using well-known CDNs for third-party resources."
	case "non_standard_port":
		return "Use standard ports (80/443) for production resources. Investigate why a non-standard port is in use."
	case "mixed_content":
		return "Upgrade all resource URLs to HTTPS. Use Content-Security-Policy: upgrade-insecure-requests."
	case "ip_address_origin":
		return "Use domain names with proper DNS. Investigate why a direct IP address is being used."
	case "potential_typosquatting":
		return "Verify the exact domain name. Check package.json / CDN references for typos."
	default:
		return "Investigate the flagged origin and verify it is legitimate."
	}
}

// HandleSecurityAudit processes MCP tool call parameters and runs the scan.
func (s *SecurityScanner) HandleSecurityAudit(params json.RawMessage, bodies []capture.NetworkBody, entries []LogEntry, pageURLs []string, waterfallEntries []capture.NetworkWaterfallEntry) (any, error) {
	var toolParams struct {
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		SeverityMin string   `json:"severity_min"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &toolParams)
	}

	input := SecurityScanInput{
		NetworkBodies:    bodies,
		WaterfallEntries: waterfallEntries,
		ConsoleEntries:   entries,
		PageURLs:         pageURLs,
		URLFilter:        toolParams.URLFilter,
		Checks:           toolParams.Checks,
		SeverityMin:      toolParams.SeverityMin,
	}

	result := s.Scan(input)
	return result, nil
}

// ============================================
// Credential Detection
// ============================================

var (
	// Credential patterns compiled once
	awsKeyPattern     = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubTokenPattern = regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`)
	stripeKeyPattern  = regexp.MustCompile(`sk_(test|live)_[A-Za-z0-9]{24,}`)
	jwtPattern        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)
	privateKeyPattern = regexp.MustCompile(`-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`)
	apiKeyURLPattern  = regexp.MustCompile(`(?i)[?&](api[_-]?key|apikey|key|token|secret|password|passwd|api_secret)=([^&]{8,})`)
	bearerPattern     = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{20,}`)
	apiKeyBodyPattern = regexp.MustCompile(`(?i)"(api[_-]?key|apiKey|api_secret|apiSecret)":\s*"([^"]{8,})"`)
	genericSecretURL  = regexp.MustCompile(`(?i)[?&]\w*(secret|password|passwd|token)\w*=([^&]{8,})`)

	// Test/dev key indicators — reduces severity
	testKeyIndicators = []string{"test", "dev", "example", "sample", "demo", "dummy", "fake", "mock"}
)

func (s *SecurityScanner) checkCredentials(bodies []capture.NetworkBody, entries []LogEntry) []SecurityFinding {
	var findings []SecurityFinding

	// Scan network bodies (URLs and body content)
	for _, body := range bodies {
		findings = append(findings, s.scanURLForCredentials(body)...)
		findings = append(findings, s.scanBodyForCredentials(body.RequestBody, body.URL, "request body")...)
		findings = append(findings, s.scanBodyForCredentials(body.ResponseBody, body.URL, "response body")...)
	}

	// Scan console entries
	for _, entry := range entries {
		findings = append(findings, s.scanConsoleForCredentials(entry)...)
	}

	return findings
}

// scanURLForAPIKeys checks for API keys in URL query parameters.
func (s *SecurityScanner) scanURLForAPIKeys(url string) []SecurityFinding {
	var findings []SecurityFinding
	matches := apiKeyURLPattern.FindAllStringSubmatch(url, 10)
	for _, m := range matches {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       fmt.Sprintf("API key exposed in URL query parameter '%s'", m[1]),
			Description: fmt.Sprintf("GET request includes secret in query parameter '%s'. Query parameters are logged in server access logs, browser history, and may be cached by proxies.", m[1]),
			Location:    url,
			Evidence:    redactSecret(m[2]),
			Remediation: "Move API key to Authorization header or request body. Never include secrets in URLs.",
		})
	}
	return findings
}

// scanURLForGenericSecrets checks for generic secret parameters in URL.
func (s *SecurityScanner) scanURLForGenericSecrets(url string) []SecurityFinding {
	if apiKeyURLPattern.MatchString(url) {
		return nil // avoid duplicating apiKey findings
	}
	var findings []SecurityFinding
	matches := genericSecretURL.FindAllStringSubmatch(url, 10)
	for _, m := range matches {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "Secret value exposed in URL query parameter",
			Description: "Request URL contains a secret-named parameter with a long value.",
			Location:    url,
			Evidence:    redactSecret(m[2]),
			Remediation: "Move secrets to Authorization header or request body.",
		})
	}
	return findings
}

func (s *SecurityScanner) scanURLForCredentials(body capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding
	findings = append(findings, s.scanURLForAPIKeys(body.URL)...)
	findings = append(findings, s.scanURLForGenericSecrets(body.URL)...)

	if jwtPattern.MatchString(body.URL) {
		findings = append(findings, SecurityFinding{
			Check: "credentials", Severity: "critical",
			Title:       "JWT token exposed in URL",
			Description: "A JWT token was found in the request URL. URLs are logged in browser history, server logs, and may leak via Referer headers.",
			Location:    body.URL, Evidence: redactSecret(jwtPattern.FindString(body.URL)),
			Remediation: "Pass JWT tokens in the Authorization header, not in URLs.",
		})
	}
	if awsKeyPattern.MatchString(body.URL) {
		findings = append(findings, SecurityFinding{
			Check: "credentials", Severity: "critical",
			Title:       "AWS access key exposed in URL",
			Description: "An AWS access key ID was found in the request URL.",
			Location:    body.URL, Evidence: redactSecret(awsKeyPattern.FindString(body.URL)),
			Remediation: "Use IAM roles or environment variables for AWS credentials. Never embed in URLs.",
		})
	}
	return findings
}

// credentialPatternCheck defines a single credential pattern to scan for in body content.
type credentialPatternCheck struct {
	pattern     *regexp.Regexp
	severity    string
	titleFmt    string
	descFmt     string
	remediation string
	evidence    string // if non-empty, use this instead of redacted match
	skipTestKey bool
}

// bodyCredentialChecks returns the ordered list of credential patterns to scan.
func bodyCredentialChecks() []credentialPatternCheck {
	return []credentialPatternCheck{
		{awsKeyPattern, "critical", "AWS access key in %s", "An AWS access key ID pattern was detected in the %s.", "Remove AWS credentials from API responses. Use short-lived STS tokens if needed.", "", true},
		{githubTokenPattern, "critical", "GitHub token in %s", "A GitHub personal access token was detected in the %s.", "Remove GitHub tokens from client-visible responses. Use short-lived tokens.", "", true},
		{stripeKeyPattern, "critical", "Stripe secret key in %s", "A Stripe secret key was detected in the %s. This key can be used to make charges.", "Never expose Stripe secret keys to the client. Use publishable keys (pk_*) for client-side operations.", "", true},
		{privateKeyPattern, "critical", "Private key material in %s", "Private key material was detected in the %s. This is a critical exposure.", "Never transmit private keys in API responses. Use key management services.", "-----BEGIN ... PRIVATE KEY-----", false},
		{jwtPattern, "medium", "JWT token in %s", "A JWT token was detected in the %s. Verify this is an intentional auth token delivery.", "Ensure JWT tokens are only delivered via secure, intended channels (e.g., httpOnly cookies).", "", false},
	}
}

func (s *SecurityScanner) scanBodyForCredentials(bodyContent, sourceURL, location string) []SecurityFinding {
	if bodyContent == "" {
		return nil
	}
	scanContent := bodyContent
	if len(scanContent) > 10240 {
		scanContent = scanContent[:10240]
	}

	var findings []SecurityFinding
	for _, check := range bodyCredentialChecks() {
		if !check.pattern.MatchString(scanContent) {
			continue
		}
		match := check.pattern.FindString(scanContent)
		if check.skipTestKey && isTestKey(match) {
			continue
		}
		evidence := check.evidence
		if evidence == "" {
			evidence = redactSecret(match)
		}
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    check.severity,
			Title:       fmt.Sprintf(check.titleFmt, location),
			Description: fmt.Sprintf(check.descFmt, location),
			Location:    sourceURL,
			Evidence:    evidence,
			Remediation: check.remediation,
		})
	}

	// API key in JSON body (multi-match pattern)
	for _, m := range apiKeyBodyPattern.FindAllStringSubmatch(scanContent, 5) {
		if len(m) < 3 || isTestKey(m[2]) {
			continue
		}
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "warning",
			Title:       fmt.Sprintf("API key '%s' in %s", m[1], location),
			Description: fmt.Sprintf("An API key field '%s' was found in the %s.", m[1], location),
			Location:    sourceURL,
			Evidence:    redactSecret(m[2]),
			Remediation: "Verify this key is meant to be client-visible. Use server-side proxy for secret keys.",
		})
	}

	return findings
}

func (s *SecurityScanner) scanConsoleForCredentials(entry LogEntry) []SecurityFinding {
	var findings []SecurityFinding

	msg := getEntryString(entry, "message")
	if msg == "" {
		return findings
	}

	// Limit scan depth
	if len(msg) > 10240 {
		msg = msg[:10240]
	}

	source := getEntryString(entry, "source")

	// Check for Bearer tokens
	if bearerPattern.MatchString(msg) {
		match := bearerPattern.FindString(msg)
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "Bearer token logged to console",
			Description: "A Bearer token was found in console output. Console logs may be captured by browser extensions or error tracking services.",
			Location:    source,
			Evidence:    redactSecret(match),
			Remediation: "Remove console.log statements that output tokens. Use structured logging with redaction.",
		})
	}

	// Check for JWT
	if jwtPattern.MatchString(msg) {
		match := jwtPattern.FindString(msg)
		// Don't double-count if already caught by bearer check
		if !bearerPattern.MatchString(msg) {
			findings = append(findings, SecurityFinding{
				Check:       "credentials",
				Severity:    "critical",
				Title:       "JWT token logged to console",
				Description: "A JWT token was found in console output.",
				Location:    source,
				Evidence:    redactSecret(match),
				Remediation: "Remove console.log statements that output tokens.",
			})
		}
	}

	// AWS key in console
	if awsKeyPattern.MatchString(msg) {
		match := awsKeyPattern.FindString(msg)
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "AWS access key logged to console",
			Description: "An AWS access key was found in console output.",
			Location:    source,
			Evidence:    redactSecret(match),
			Remediation: "Never log AWS credentials. Use environment variables and IAM roles.",
		})
	}

	return findings
}

// ============================================
// Helper Functions
// ============================================

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

func isLocalhostURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

func isHTMLResponse(body capture.NetworkBody) bool {
	ct := strings.ToLower(body.ContentType)
	return strings.Contains(ct, "text/html")
}

func isJavaScriptContent(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "javascript")
}

// extractOrigin extracts the origin (scheme://host[:port]) from a URL
// Returns empty string for data: URLs, blob: URLs (after extracting nested origin), and malformed URLs
func extractOrigin(rawURL string) string {
	// Handle data: URLs
	if strings.HasPrefix(rawURL, "data:") {
		return ""
	}

	// Handle blob: URLs - extract the nested origin
	// blob:https://example.com/uuid -> https://example.com
	rawURL = strings.TrimPrefix(rawURL, "blob:")

	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	// URL must have a scheme and host
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	// Reconstruct origin: scheme://host[:port]
	return parsed.Scheme + "://" + parsed.Host
}

func isThirdPartyURL(requestURL string, pageURLs []string) bool {
	if len(pageURLs) == 0 {
		return false
	}
	reqParsed, err := url.Parse(requestURL)
	if err != nil {
		return false
	}
	reqHost := reqParsed.Hostname()

	for _, pageURL := range pageURLs {
		pageParsed, err := url.Parse(pageURL)
		if err != nil {
			continue
		}
		pageHost := pageParsed.Hostname()
		// Same domain check (including subdomains)
		if reqHost == pageHost || strings.HasSuffix(reqHost, "."+pageHost) || strings.HasSuffix(pageHost, "."+reqHost) {
			return false
		}
	}
	return true
}

func filterBodiesByURL(bodies []capture.NetworkBody, filter string) []capture.NetworkBody {
	if filter == "" {
		return bodies
	}
	var filtered []capture.NetworkBody
	for _, b := range bodies {
		if strings.Contains(b.URL, filter) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

func filterBySeverity(findings []SecurityFinding, minSeverity string) []SecurityFinding {
	severityOrder := map[string]int{
		"info":     0,
		"low":      1,
		"medium":   2,
		"warning":  2,
		"high":     3,
		"critical": 4,
	}
	minLevel, ok := severityOrder[minSeverity]
	if !ok {
		return findings
	}

	var filtered []SecurityFinding
	for _, f := range findings {
		level := severityOrder[f.Severity]
		if level >= minLevel {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func buildSummary(findings []SecurityFinding, bodies []capture.NetworkBody) ScanSummary {
	bySeverity := make(map[string]int)
	byCheck := make(map[string]int)
	for _, f := range findings {
		bySeverity[f.Severity]++
		byCheck[f.Check]++
	}

	// Count unique URLs
	urlSet := make(map[string]bool)
	for _, b := range bodies {
		urlSet[b.URL] = true
	}

	return ScanSummary{
		TotalFindings: len(findings),
		BySeverity:    bySeverity,
		ByCheck:       byCheck,
		URLsScanned:   len(urlSet),
	}
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
