// Purpose: Orchestrates security checks by dispatching to credential, header, cookie, and transport scanners.
// Why: Separates scan orchestration from individual check implementations.
package security

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

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
