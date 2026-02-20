// security.go — Security Scanner (security_audit) MCP tool.
// Analyzes captured browser data to detect exposed credentials, PII leakage,
// missing security headers, insecure cookies, insecure transport, and
// missing authentication patterns.
// Design: SecurityScanner is a standalone struct with no external dependencies.
// All analysis operates on data already captured by Gasoline buffers.
package security

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

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

// NewSecurityScanner creates a new SecurityScanner instance.
func NewSecurityScanner() *SecurityScanner {
	return &SecurityScanner{}
}

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

// buildSummary creates aggregate counts of findings.
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

// filterBodiesByURL filters network bodies to only those matching the URL filter.
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

// filterBySeverity filters findings to only those at or above the minimum severity.
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
