// security_credentials.go — Credential detection, URL/body/console scanning, and shared helpers.
package security

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/util"
)

var (
	// Credential patterns compiled once
	awsKeyPattern      = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubTokenPattern = regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`)
	stripeKeyPattern   = regexp.MustCompile(`sk_(test|live)_[A-Za-z0-9]{24,}`)
	jwtPattern         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)
	privateKeyPattern  = regexp.MustCompile(`-----BEGIN (RSA|DSA|EC|OPENSSH) PRIVATE KEY-----`)
	apiKeyURLPattern   = regexp.MustCompile(`(?i)[?&](api[_-]?key|apikey|key|token|secret|password|passwd|api_secret)=([^&]{8,})`)
	bearerPattern      = regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9._\-]{20,}`)
	apiKeyBodyPattern  = regexp.MustCompile(`(?i)"(api[_-]?key|apiKey|api_secret|apiSecret)":\s*"([^"]{8,})"`)
	genericSecretURL   = regexp.MustCompile(`(?i)[?&]\w*(secret|password|passwd|token)\w*=([^&]{8,})`)

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

// extractOrigin delegates to util.ExtractOrigin for origin extraction.
func extractOrigin(rawURL string) string {
	return util.ExtractOrigin(rawURL)
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
