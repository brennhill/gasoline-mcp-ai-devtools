// Purpose: Computes regressions and improvements between security snapshots.
// Why: Isolates comparison logic so security findings remain deterministic and maintainable.
// Docs: docs/features/feature/security-hardening/index.md

package security

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"

// Compare computes regressions/improvements between two snapshots.
//
// Failure semantics:
// - Missing/expired snapshot references return errors rather than partial comparisons.
func (m *SecurityDiffManager) Compare(fromName, toName string, currentBodies []capture.NetworkBody) (*SecurityDiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fromSnapshot, err := m.resolveSnapshot(fromName)
	if err != nil {
		return nil, err
	}

	toSnapshot, err := m.resolveToSnapshot(toName, currentBodies)
	if err != nil {
		return nil, err
	}

	regressions, improvements := m.collectAllChanges(fromSnapshot, toSnapshot)
	verdict := determineVerdict(regressions, improvements)
	summary := buildDiffSummary(regressions, improvements)

	return &SecurityDiffResult{
		Verdict:      verdict,
		Regressions:  regressions,
		Improvements: improvements,
		Summary:      summary,
	}, nil
}

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

func (m *SecurityDiffManager) collectAllChanges(from, to *SecuritySnapshot) ([]SecurityChange, []SecurityChange) {
	var regressions, improvements []SecurityChange

	compareFns := []func(*SecuritySnapshot, *SecuritySnapshot) ([]SecurityChange, []SecurityChange){
		m.compareHeaders,
		m.compareCookies,
		m.compareAuth,
		m.compareTransport,
	}
	for _, compareFn := range compareFns {
		reg, imp := compareFn(from, to)
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
