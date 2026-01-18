// security.go — Security Scanner (security_audit) MCP tool.
// Analyzes captured browser data to detect exposed credentials, PII leakage,
// missing security headers, insecure cookies, insecure transport, and
// missing authentication patterns.
// Design: SecurityScanner is a standalone struct with no external dependencies.
// All analysis operates on data already captured by Gasoline buffers.
package security

import (
	"github.com/dev-console/dev-console/internal/capture"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
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
	NetworkBodies []capture.NetworkBody
	ConsoleEntries []LogEntry
	PageURLs       []string
	URLFilter      string
	Checks         []string // Which checks to run; empty = all
	SeverityMin    string   // Minimum severity to report
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

// Scan analyzes the input data and returns security findings.
func (s *SecurityScanner) Scan(input SecurityScanInput) SecurityScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var findings []SecurityFinding
	checks := input.Checks
	if len(checks) == 0 {
		checks = []string{"credentials", "pii", "headers", "cookies", "transport", "auth"}
	}

	checkSet := make(map[string]bool)
	for _, c := range checks {
		checkSet[c] = true
	}

	// Filter network bodies by URL if specified
	bodies := filterBodiesByURL(input.NetworkBodies, input.URLFilter)

	if checkSet["credentials"] {
		findings = append(findings, s.checkCredentials(bodies, input.ConsoleEntries)...)
	}
	if checkSet["pii"] {
		findings = append(findings, s.checkPII(bodies, input.PageURLs)...)
	}
	if checkSet["headers"] {
		findings = append(findings, s.checkSecurityHeaders(bodies)...)
	}
	if checkSet["cookies"] {
		findings = append(findings, s.checkCookies(bodies)...)
	}
	if checkSet["transport"] {
		findings = append(findings, s.checkTransport(bodies, input.PageURLs)...)
	}
	if checkSet["auth"] {
		findings = append(findings, s.checkAuthPatterns(bodies)...)
	}

	// Apply severity filter
	if input.SeverityMin != "" {
		findings = filterBySeverity(findings, input.SeverityMin)
	}

	// Build summary
	summary := buildSummary(findings, bodies)

	return SecurityScanResult{
		Findings:  findings,
		Summary:   summary,
		ScannedAt: time.Now(),
	}
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleSecurityAudit processes MCP tool call parameters and runs the scan.
func (s *SecurityScanner) HandleSecurityAudit(params json.RawMessage, bodies []capture.NetworkBody, entries []LogEntry, pageURLs []string) (any, error) {
	var toolParams struct {
		Checks      []string `json:"checks"`
		URLFilter   string   `json:"url"`
		SeverityMin string   `json:"severity_min"`
	}
	if len(params) > 0 {
		_ = json.Unmarshal(params, &toolParams)
	}

	input := SecurityScanInput{
		NetworkBodies:  bodies,
		ConsoleEntries: entries,
		PageURLs:       pageURLs,
		URLFilter:      toolParams.URLFilter,
		Checks:         toolParams.Checks,
		SeverityMin:    toolParams.SeverityMin,
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

func (s *SecurityScanner) scanURLForCredentials(body capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding

	// Check for API keys / secrets in URL query params
	if matches := apiKeyURLPattern.FindAllStringSubmatch(body.URL, 10); len(matches) > 0 {
		for _, m := range matches {
			if len(m) >= 3 {
				paramName := m[1]
				paramValue := m[2]
				if isTestKey(paramValue) {
					continue
				}
				findings = append(findings, SecurityFinding{
					Check:       "credentials",
					Severity:    "critical",
					Title:       fmt.Sprintf("API key exposed in URL query parameter '%s'", paramName),
					Description: fmt.Sprintf("GET request includes secret in query parameter '%s'. Query parameters are logged in server access logs, browser history, and may be cached by proxies.", paramName),
					Location:    body.URL,
					Evidence:    redactSecret(paramValue),
					Remediation: "Move API key to Authorization header or request body. Never include secrets in URLs.",
				})
			}
		}
	}

	// Check for generic secrets in URL
	if matches := genericSecretURL.FindAllStringSubmatch(body.URL, 10); len(matches) > 0 {
		for _, m := range matches {
			if len(m) >= 3 {
				paramValue := m[2]
				if isTestKey(paramValue) {
					continue
				}
				// Avoid duplicate with apiKeyURLPattern
				if apiKeyURLPattern.MatchString(body.URL) {
					continue
				}
				findings = append(findings, SecurityFinding{
					Check:       "credentials",
					Severity:    "critical",
					Title:       "Secret value exposed in URL query parameter",
					Description: "Request URL contains a secret-named parameter with a long value.",
					Location:    body.URL,
					Evidence:    redactSecret(paramValue),
					Remediation: "Move secrets to Authorization header or request body.",
				})
			}
		}
	}

	// Check for JWT in URL
	if jwtPattern.MatchString(body.URL) {
		match := jwtPattern.FindString(body.URL)
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "JWT token exposed in URL",
			Description: "A JWT token was found in the request URL. URLs are logged in browser history, server logs, and may leak via Referer headers.",
			Location:    body.URL,
			Evidence:    redactSecret(match),
			Remediation: "Pass JWT tokens in the Authorization header, not in URLs.",
		})
	}

	// Check for AWS keys in URL
	if awsKeyPattern.MatchString(body.URL) {
		match := awsKeyPattern.FindString(body.URL)
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "AWS access key exposed in URL",
			Description: "An AWS access key ID was found in the request URL.",
			Location:    body.URL,
			Evidence:    redactSecret(match),
			Remediation: "Use IAM roles or environment variables for AWS credentials. Never embed in URLs.",
		})
	}

	return findings
}

func (s *SecurityScanner) scanBodyForCredentials(bodyContent, sourceURL, location string) []SecurityFinding {
	var findings []SecurityFinding
	if bodyContent == "" {
		return findings
	}

	// Limit scan depth
	scanContent := bodyContent
	if len(scanContent) > 10240 {
		scanContent = scanContent[:10240]
	}

	// AWS access key
	if awsKeyPattern.MatchString(scanContent) {
		match := awsKeyPattern.FindString(scanContent)
		if !isTestKey(match) {
			findings = append(findings, SecurityFinding{
				Check:       "credentials",
				Severity:    "critical",
				Title:       "AWS access key in " + location,
				Description: "An AWS access key ID pattern was detected in the " + location + ".",
				Location:    sourceURL,
				Evidence:    redactSecret(match),
				Remediation: "Remove AWS credentials from API responses. Use short-lived STS tokens if needed.",
			})
		}
	}

	// GitHub token
	if githubTokenPattern.MatchString(scanContent) {
		match := githubTokenPattern.FindString(scanContent)
		if !isTestKey(match) {
			findings = append(findings, SecurityFinding{
				Check:       "credentials",
				Severity:    "critical",
				Title:       "GitHub token in " + location,
				Description: "A GitHub personal access token was detected in the " + location + ".",
				Location:    sourceURL,
				Evidence:    redactSecret(match),
				Remediation: "Remove GitHub tokens from client-visible responses. Use short-lived tokens.",
			})
		}
	}

	// Stripe secret key
	if stripeKeyPattern.MatchString(scanContent) {
		match := stripeKeyPattern.FindString(scanContent)
		if !isTestKey(match) {
			findings = append(findings, SecurityFinding{
				Check:       "credentials",
				Severity:    "critical",
				Title:       "Stripe secret key in " + location,
				Description: "A Stripe secret key was detected in the " + location + ". This key can be used to make charges.",
				Location:    sourceURL,
				Evidence:    redactSecret(match),
				Remediation: "Never expose Stripe secret keys to the client. Use publishable keys (pk_*) for client-side operations.",
			})
		}
	}

	// Private key material
	if privateKeyPattern.MatchString(scanContent) {
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "critical",
			Title:       "Private key material in " + location,
			Description: "Private key material was detected in the " + location + ". This is a critical exposure.",
			Location:    sourceURL,
			Evidence:    "-----BEGIN ... PRIVATE KEY-----",
			Remediation: "Never transmit private keys in API responses. Use key management services.",
		})
	}

	// JWT in body (warning level — may be intentional for auth flows)
	if jwtPattern.MatchString(scanContent) {
		match := jwtPattern.FindString(scanContent)
		findings = append(findings, SecurityFinding{
			Check:       "credentials",
			Severity:    "medium",
			Title:       "JWT token in " + location,
			Description: "A JWT token was detected in the " + location + ". Verify this is an intentional auth token delivery.",
			Location:    sourceURL,
			Evidence:    redactSecret(match),
			Remediation: "Ensure JWT tokens are only delivered via secure, intended channels (e.g., httpOnly cookies).",
		})
	}

	// API key in JSON body
	if matches := apiKeyBodyPattern.FindAllStringSubmatch(scanContent, 5); len(matches) > 0 {
		for _, m := range matches {
			if len(m) >= 3 {
				keyName := m[1]
				keyValue := m[2]
				if isTestKey(keyValue) {
					continue
				}
				findings = append(findings, SecurityFinding{
					Check:       "credentials",
					Severity:    "warning",
					Title:       fmt.Sprintf("API key '%s' in %s", keyName, location),
					Description: fmt.Sprintf("An API key field '%s' was found in the %s.", keyName, location),
					Location:    sourceURL,
					Evidence:    redactSecret(keyValue),
					Remediation: "Verify this key is meant to be client-visible. Use server-side proxy for secret keys.",
				})
			}
		}
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
// PII Detection
// ============================================

var (
	emailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	phonePattern = regexp.MustCompile(`(\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`)
	ssnPattern   = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ccPattern    = regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`)
)

func (s *SecurityScanner) checkPII(bodies []capture.NetworkBody, pageURLs []string) []SecurityFinding {
	var findings []SecurityFinding

	for _, body := range bodies {
		// Check request body for PII sent to third parties
		if body.RequestBody != "" {
			isThirdParty := isThirdPartyURL(body.URL, pageURLs)
			findings = append(findings, s.scanForPII(body.RequestBody, body.URL, "request body", isThirdParty)...)
		}

		// Check response body for PII
		if body.ResponseBody != "" {
			findings = append(findings, s.scanForPII(body.ResponseBody, body.URL, "response body", false)...)
		}
	}

	return findings
}

func (s *SecurityScanner) scanForPII(content, sourceURL, location string, isThirdParty bool) []SecurityFinding {
	var findings []SecurityFinding

	if len(content) > 10240 {
		content = content[:10240]
	}

	// SSN detection (always high severity)
	if ssnPattern.MatchString(content) {
		match := ssnPattern.FindString(content)
		severity := "high"
		desc := fmt.Sprintf("A Social Security Number pattern was detected in %s.", location)
		if isThirdParty {
			severity = "critical"
			desc = fmt.Sprintf("A Social Security Number pattern is being sent to a third-party endpoint in %s.", location)
		}
		findings = append(findings, SecurityFinding{
			Check:       "pii",
			Severity:    severity,
			Title:       "SSN pattern detected in " + location,
			Description: desc,
			Location:    sourceURL,
			Evidence:    redactSecret(match),
			Remediation: "Never transmit SSNs in plain text. Use tokenization or encryption.",
		})
	}

	// Credit card detection
	if ccPattern.MatchString(content) {
		match := ccPattern.FindString(content)
		// Simple Luhn-like check: just verify it looks like a card number
		cleaned := strings.ReplaceAll(strings.ReplaceAll(match, " ", ""), "-", "")
		if len(cleaned) >= 13 && len(cleaned) <= 19 && looksLikeCreditCard(cleaned) {
			findings = append(findings, SecurityFinding{
				Check:       "pii",
				Severity:    "critical",
				Title:       "Credit card number detected in " + location,
				Description: fmt.Sprintf("A credit card number pattern was detected in %s.", location),
				Location:    sourceURL,
				Evidence:    redactSecret(match),
				Remediation: "Never transmit full card numbers. Use tokenization (e.g., Stripe tokens).",
			})
		}
	}

	// Email detection
	if emailPattern.MatchString(content) {
		match := emailPattern.FindString(content)
		severity := "info"
		if isThirdParty {
			severity = "warning"
		}
		findings = append(findings, SecurityFinding{
			Check:       "pii",
			Severity:    severity,
			Title:       "Email address in " + location,
			Description: fmt.Sprintf("An email address was detected in %s.", location),
			Location:    sourceURL,
			Evidence:    redactSecret(match),
			Remediation: "Review whether PII needs to be sent to this endpoint.",
		})
	}

	// Phone detection
	if phonePattern.MatchString(content) {
		match := phonePattern.FindString(content)
		// Only flag if it looks like a real phone (not just any digit sequence)
		if len(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(match, "-", ""), " ", ""), "(", "")) >= 10 {
			severity := "info"
			if isThirdParty {
				severity = "warning"
			}
			findings = append(findings, SecurityFinding{
				Check:       "pii",
				Severity:    severity,
				Title:       "Phone number in " + location,
				Description: fmt.Sprintf("A phone number pattern was detected in %s.", location),
				Location:    sourceURL,
				Evidence:    redactSecret(match),
				Remediation: "Review whether PII needs to be sent to this endpoint.",
			})
		}
	}

	return findings
}

// ============================================
// Security Headers Check
// ============================================

var requiredSecurityHeaders = []struct {
	Name     string
	Severity string
}{
	{"Strict-Transport-Security", "high"},
	{"X-Content-Type-Options", "medium"},
	{"X-Frame-Options", "medium"},
	{"Content-Security-Policy", "medium"},
	{"Referrer-Policy", "low"},
	{"Permissions-Policy", "low"},
}

func (s *SecurityScanner) checkSecurityHeaders(bodies []capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding

	// Only check HTML responses
	checkedOrigins := make(map[string]bool)
	for _, body := range bodies {
		if !isHTMLResponse(body) {
			continue
		}
		// Skip localhost for HSTS
		origin := extractOrigin(body.URL)
		if checkedOrigins[origin] {
			continue
		}
		checkedOrigins[origin] = true

		isLocalhost := isLocalhostURL(body.URL)

		for _, header := range requiredSecurityHeaders {
			// Skip HSTS for localhost
			if header.Name == "Strict-Transport-Security" && isLocalhost {
				continue
			}
			// Skip HSTS for non-HTTPS
			if header.Name == "Strict-Transport-Security" && !strings.HasPrefix(body.URL, "https://") {
				continue
			}

			if body.ResponseHeaders == nil || body.ResponseHeaders[header.Name] == "" {
				findings = append(findings, SecurityFinding{
					Check:       "headers",
					Severity:    header.Severity,
					Title:       fmt.Sprintf("Missing %s header", header.Name),
					Description: fmt.Sprintf("The response from %s does not include the %s header.", origin, header.Name),
					Location:    body.URL,
					Evidence:    "Header not present in response",
					Remediation: fmt.Sprintf("Add the %s header to your server responses.", header.Name),
				})
			}
		}
	}

	return findings
}

// ============================================
// Cookie Security Check
// ============================================

var sessionCookiePattern = regexp.MustCompile(`(?i)(session|token|auth|jwt|sid)`)

func (s *SecurityScanner) checkCookies(bodies []capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding

	for _, body := range bodies {
		if body.ResponseHeaders == nil {
			continue
		}
		setCookie := body.ResponseHeaders["Set-Cookie"]
		if setCookie == "" {
			continue
		}

		cookies := parseCookies(setCookie)
		isHTTPS := strings.HasPrefix(body.URL, "https://")

		for _, cookie := range cookies {
			isSensitive := sessionCookiePattern.MatchString(cookie.Name)

			// HttpOnly check for session cookies
			if isSensitive && !cookie.HttpOnly {
				findings = append(findings, SecurityFinding{
					Check:       "cookies",
					Severity:    "warning",
					Title:       fmt.Sprintf("Session cookie '%s' missing HttpOnly flag", cookie.Name),
					Description: fmt.Sprintf("The cookie '%s' appears to be a session cookie but lacks the HttpOnly flag, making it accessible to JavaScript (XSS risk).", cookie.Name),
					Location:    body.URL,
					Evidence:    fmt.Sprintf("Set-Cookie: %s (no HttpOnly)", cookie.Name),
					Remediation: "Add the HttpOnly flag to prevent JavaScript access to this cookie.",
				})
			}

			// Secure flag check for HTTPS
			if isHTTPS && !cookie.Secure {
				findings = append(findings, SecurityFinding{
					Check:       "cookies",
					Severity:    "warning",
					Title:       fmt.Sprintf("Cookie '%s' missing Secure flag on HTTPS", cookie.Name),
					Description: fmt.Sprintf("The cookie '%s' is set on an HTTPS page but lacks the Secure flag, meaning it could be sent over HTTP.", cookie.Name),
					Location:    body.URL,
					Evidence:    fmt.Sprintf("Set-Cookie: %s (no Secure)", cookie.Name),
					Remediation: "Add the Secure flag so the cookie is only sent over HTTPS.",
				})
			}

			// SameSite check
			if isSensitive && cookie.SameSite == "" {
				findings = append(findings, SecurityFinding{
					Check:       "cookies",
					Severity:    "warning",
					Title:       fmt.Sprintf("Cookie '%s' missing SameSite attribute", cookie.Name),
					Description: fmt.Sprintf("The cookie '%s' lacks a SameSite attribute, which may allow cross-site request forgery.", cookie.Name),
					Location:    body.URL,
					Evidence:    fmt.Sprintf("Set-Cookie: %s (no SameSite)", cookie.Name),
					Remediation: "Add SameSite=Lax or SameSite=Strict to prevent CSRF attacks.",
				})
			}
		}
	}

	return findings
}

// ============================================
// Transport Security Check
// ============================================

func (s *SecurityScanner) checkTransport(bodies []capture.NetworkBody, pageURLs []string) []SecurityFinding {
	var findings []SecurityFinding

	pageIsHTTPS := false
	for _, u := range pageURLs {
		if strings.HasPrefix(u, "https://") {
			pageIsHTTPS = true
			break
		}
	}

	for _, body := range bodies {
		if !strings.HasPrefix(body.URL, "http://") {
			continue
		}
		if isLocalhostURL(body.URL) {
			continue
		}

		// Plain HTTP request to non-localhost
		findings = append(findings, SecurityFinding{
			Check:       "transport",
			Severity:    "warning",
			Title:       "API call over unencrypted HTTP",
			Description: fmt.Sprintf("%s %s uses unencrypted HTTP. Data in transit can be intercepted.", body.Method, body.URL),
			Location:    body.URL,
			Evidence:    fmt.Sprintf("%s %s", body.Method, body.URL),
			Remediation: "Use HTTPS for all API calls. Configure your server with TLS.",
		})

		// Mixed content check
		if pageIsHTTPS {
			severity := "warning"
			if isJavaScriptContent(body.ContentType) {
				severity = "critical"
			}
			findings = append(findings, SecurityFinding{
				Check:       "transport",
				Severity:    severity,
				Title:       "Mixed content: HTTPS page loading HTTP resource",
				Description: fmt.Sprintf("An HTTPS page is loading a resource from %s over unencrypted HTTP.", body.URL),
				Location:    body.URL,
				Evidence:    fmt.Sprintf("HTTP resource on HTTPS page: %s", body.URL),
				Remediation: "Use HTTPS for all resources. Mixed content can be intercepted by network attackers.",
			})
		}
	}

	return findings
}

// ============================================
// Auth Pattern Check
// ============================================

func (s *SecurityScanner) checkAuthPatterns(bodies []capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding

	for _, body := range bodies {
		if body.HasAuthHeader {
			continue
		}
		if body.ResponseBody == "" {
			continue
		}

		// Check if response contains PII
		piiFields := detectPIIFields(body.ResponseBody)
		if len(piiFields) > 0 {
			findings = append(findings, SecurityFinding{
				Check:       "auth",
				Severity:    "warning",
				Title:       "Endpoint returns sensitive data without authentication",
				Description: fmt.Sprintf("GET %s returned PII fields (%s) but no Authorization header was present.", body.URL, strings.Join(piiFields, ", ")),
				Location:    body.URL,
				Evidence:    fmt.Sprintf("PII fields in response: %s, auth: none", strings.Join(piiFields, ", ")),
				Remediation: "Ensure this endpoint requires authentication. If public by design, verify no sensitive data is exposed.",
			})
		}
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

func detectPIIFields(body string) []string {
	var fields []string
	if emailPattern.MatchString(body) {
		fields = append(fields, "email")
	}
	if phonePattern.MatchString(body) {
		fields = append(fields, "phone")
	}
	if ssnPattern.MatchString(body) {
		fields = append(fields, "ssn")
	}
	return fields
}

// looksLikeCreditCard performs a basic Luhn check on a digit string.
func looksLikeCreditCard(digits string) bool {
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// cookieAttrs represents parsed Set-Cookie attributes
type cookieAttrs struct {
	Name     string
	HttpOnly bool
	Secure   bool
	SameSite string
}

func parseCookies(setCookieHeader string) []cookieAttrs {
	var cookies []cookieAttrs

	// Handle multiple cookies (could be separated by newlines in practice)
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

	// First part is name=value
	if len(parts) > 0 {
		nameValue := strings.TrimSpace(parts[0])
		eqIdx := strings.Index(nameValue, "=")
		if eqIdx > 0 {
			cookie.Name = nameValue[:eqIdx]
		}
	}

	// Parse attributes
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
