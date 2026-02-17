// Purpose: Owns security_checks.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// security_checks.go — Security check implementations for PII, headers, cookies, transport, and auth patterns.
package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
)

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

// scanForSSN checks for Social Security Number patterns.
func scanForSSN(content, sourceURL, location string, isThirdParty bool) *SecurityFinding {
	if !ssnPattern.MatchString(content) {
		return nil
	}
	match := ssnPattern.FindString(content)
	severity := "high"
	desc := fmt.Sprintf("A Social Security Number pattern was detected in %s.", location)
	if isThirdParty {
		severity = "critical"
		desc = fmt.Sprintf("A Social Security Number pattern is being sent to a third-party endpoint in %s.", location)
	}
	return &SecurityFinding{
		Check: "pii", Severity: severity,
		Title: "SSN pattern detected in " + location, Description: desc,
		Location: sourceURL, Evidence: redactSecret(match),
		Remediation: "Never transmit SSNs in plain text. Use tokenization or encryption.",
	}
}

// scanForCreditCard checks for credit card number patterns.
func scanForCreditCard(content, sourceURL, location string) *SecurityFinding {
	if !ccPattern.MatchString(content) {
		return nil
	}
	match := ccPattern.FindString(content)
	cleaned := strings.ReplaceAll(strings.ReplaceAll(match, " ", ""), "-", "")
	if len(cleaned) < 13 || len(cleaned) > 19 || !looksLikeCreditCard(cleaned) {
		return nil
	}
	return &SecurityFinding{
		Check: "pii", Severity: "critical",
		Title: "Credit card number detected in " + location,
		Description: fmt.Sprintf("A credit card number pattern was detected in %s.", location),
		Location: sourceURL, Evidence: redactSecret(match),
		Remediation: "Never transmit full card numbers. Use tokenization (e.g., Stripe tokens).",
	}
}

// thirdPartySeverity returns "warning" for third-party, "info" otherwise.
func thirdPartySeverity(isThirdParty bool) string {
	if isThirdParty {
		return "warning"
	}
	return "info"
}

// scanForEmailPII checks for email address patterns.
func scanForEmailPII(content, sourceURL, location string, isThirdParty bool) *SecurityFinding {
	if !emailPattern.MatchString(content) {
		return nil
	}
	return &SecurityFinding{
		Check: "pii", Severity: thirdPartySeverity(isThirdParty),
		Title: "Email address in " + location,
		Description: fmt.Sprintf("An email address was detected in %s.", location),
		Location: sourceURL, Evidence: redactSecret(emailPattern.FindString(content)),
		Remediation: "Review whether PII needs to be sent to this endpoint.",
	}
}

// scanForPhonePII checks for phone number patterns.
func scanForPhonePII(content, sourceURL, location string, isThirdParty bool) *SecurityFinding {
	if !phonePattern.MatchString(content) {
		return nil
	}
	match := phonePattern.FindString(content)
	cleaned := strings.NewReplacer("-", "", " ", "", "(", "").Replace(match)
	if len(cleaned) < 10 {
		return nil
	}
	return &SecurityFinding{
		Check: "pii", Severity: thirdPartySeverity(isThirdParty),
		Title: "Phone number in " + location,
		Description: fmt.Sprintf("A phone number pattern was detected in %s.", location),
		Location: sourceURL, Evidence: redactSecret(match),
		Remediation: "Review whether PII needs to be sent to this endpoint.",
	}
}

func (s *SecurityScanner) scanForPII(content, sourceURL, location string, isThirdParty bool) []SecurityFinding {
	if len(content) > 10240 {
		content = content[:10240]
	}

	var findings []SecurityFinding
	piiChecks := []*SecurityFinding{
		scanForSSN(content, sourceURL, location, isThirdParty),
		scanForCreditCard(content, sourceURL, location),
		scanForEmailPII(content, sourceURL, location, isThirdParty),
		scanForPhonePII(content, sourceURL, location, isThirdParty),
	}
	for _, f := range piiChecks {
		if f != nil {
			findings = append(findings, *f)
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

// shouldSkipHSTS returns true if an HSTS check should be skipped for this body.
func shouldSkipHSTS(headerName string, body capture.NetworkBody) bool {
	if headerName != "Strict-Transport-Security" {
		return false
	}
	return isLocalhostURL(body.URL) || !strings.HasPrefix(body.URL, "https://")
}

// checkHeadersForOrigin checks a single HTML response for missing security headers.
func checkHeadersForOrigin(body capture.NetworkBody, origin string) []SecurityFinding {
	var findings []SecurityFinding
	for _, header := range requiredSecurityHeaders {
		if shouldSkipHSTS(header.Name, body) {
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
	return findings
}

func (s *SecurityScanner) checkSecurityHeaders(bodies []capture.NetworkBody) []SecurityFinding {
	var findings []SecurityFinding
	checkedOrigins := make(map[string]bool)

	for _, body := range bodies {
		if !isHTMLResponse(body) {
			continue
		}
		origin := extractOrigin(body.URL)
		if checkedOrigins[origin] {
			continue
		}
		checkedOrigins[origin] = true
		findings = append(findings, checkHeadersForOrigin(body, origin)...)
	}
	return findings
}

// ============================================
// Cookie Security Check
// ============================================

var sessionCookiePattern = regexp.MustCompile(`(?i)(session|token|auth|jwt|sid)`)

// checkSingleCookie checks a single cookie for missing security attributes.
func checkSingleCookie(cookie cookieAttrs, bodyURL string, isHTTPS bool) []SecurityFinding {
	var findings []SecurityFinding
	isSensitive := sessionCookiePattern.MatchString(cookie.Name)

	if isSensitive && !cookie.HttpOnly {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Session cookie '%s' missing HttpOnly flag", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' appears to be a session cookie but lacks the HttpOnly flag, making it accessible to JavaScript (XSS risk).", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no HttpOnly)", cookie.Name),
			Remediation: "Add the HttpOnly flag to prevent JavaScript access to this cookie.",
		})
	}
	if isHTTPS && !cookie.Secure {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing Secure flag on HTTPS", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' is set on an HTTPS page but lacks the Secure flag, meaning it could be sent over HTTP.", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no Secure)", cookie.Name),
			Remediation: "Add the Secure flag so the cookie is only sent over HTTPS.",
		})
	}
	if isSensitive && cookie.SameSite == "" {
		findings = append(findings, SecurityFinding{
			Check: "cookies", Severity: "warning",
			Title:       fmt.Sprintf("Cookie '%s' missing SameSite attribute", cookie.Name),
			Description: fmt.Sprintf("The cookie '%s' lacks a SameSite attribute, which may allow cross-site request forgery.", cookie.Name),
			Location:    bodyURL,
			Evidence:    fmt.Sprintf("Set-Cookie: %s (no SameSite)", cookie.Name),
			Remediation: "Add SameSite=Lax or SameSite=Strict to prevent CSRF attacks.",
		})
	}
	return findings
}

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
		isHTTPS := strings.HasPrefix(body.URL, "https://")
		for _, cookie := range parseCookies(setCookie) {
			findings = append(findings, checkSingleCookie(cookie, body.URL, isHTTPS)...)
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

// detectPIIFields checks a body string for PII patterns and returns field names found.
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
