// Security flagging: Detects suspicious origins in network data
// Identifies supply chain attacks, poisoned dependencies, and non-standard network behavior.
// Used by security_audit and third_party_audit modes.
package main

import (
	"net"
	"net/url"
	"strings"
	"time"
)

// ============================================
// Security Flagging
// ============================================
// Detects suspicious origins in network waterfall data
// to help identify supply chain attacks and poisoned dependencies.

// SecurityFlag represents a detected security issue
type SecurityFlag struct {
	Type      string    `json:"type"`      // "suspicious_tld", "non_standard_port", etc.
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Origin    string    `json:"origin"`    // The flagged origin
	Message   string    `json:"message"`   // Human-readable explanation
	Resource  string    `json:"resource"`  // Specific resource URL (optional)
	PageURL   string    `json:"page_url"`  // Page that loaded this resource
	Timestamp time.Time `json:"timestamp"` // When flagged
}

// ============================================
// TLD Reputation Database
// ============================================

// TLDReputation defines risk level for a TLD
type TLDReputation struct {
	Severity string // "low", "medium", "high"
	Reason   string
}

// SuspiciousTLDs maps TLDs to their risk profiles
var SuspiciousTLDs = map[string]TLDReputation{
	".xyz":     {"medium", "TLD .xyz has elevated abuse rates"},
	".top":     {"medium", "TLD .top frequently used for malicious domains"},
	".loan":    {"high", "TLD .loan commonly associated with phishing"},
	".click":   {"medium", "TLD .click has elevated spam rates"},
	".stream":  {"medium", "TLD .stream frequently used for piracy sites"},
	".download": {"high", "TLD .download associated with malware distribution"},
	".review":  {"medium", "TLD .review has elevated fraud rates"},
	".country": {"medium", "TLD .country frequently used for scams"},
}

// KnownLegitimateOrigins whitelist for false positive avoidance
var KnownLegitimateOrigins = map[string]string{
	"https://pages.dev":              "Cloudflare Pages (legitimate)",
	"https://vercel.app":              "Vercel hosting (legitimate)",
	"https://netlify.app":             "Netlify hosting (legitimate)",
	"https://railway.app":             "Railway hosting (legitimate)",
	"https://fly.dev":                 "Fly.io hosting (legitimate)",
	"https://workers.dev":             "Cloudflare Workers (legitimate)",
	"https://web.app":                 "Firebase Hosting (legitimate)",
	"https://firebaseapp.com":         "Firebase Hosting (legitimate)",
}

// ============================================
// Detection Algorithms
// ============================================

// checkSuspiciousTLD returns warning if TLD is known for malicious activity
func checkSuspiciousTLD(origin string) *SecurityFlag {
	// Check whitelist first (avoid false positives)
	if _, ok := KnownLegitimateOrigins[origin]; ok {
		return nil
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()

	// Check each TLD in our database
	for tld, rep := range SuspiciousTLDs {
		if strings.HasSuffix(hostname, tld) {
			// Skip low-severity TLDs (too many false positives)
			if rep.Severity == "low" {
				return nil
			}

			return &SecurityFlag{
				Type:      "suspicious_tld",
				Severity:  rep.Severity,
				Origin:    origin,
				Message:   rep.Reason,
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

// checkNonStandardPort flags origins using unusual ports for web traffic
func checkNonStandardPort(origin string) *SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	port := parsed.Port()
	if port == "" {
		return nil // Default ports (80, 443) are fine
	}

	// Standard web ports
	standardPorts := map[string]bool{
		"80":   true,
		"443":  true,
	}

	if standardPorts[port] {
		return nil
	}

	// Development ports (localhost only)
	hostname := parsed.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		devPorts := map[string]bool{
			"3000":  true,
			"3001":  true,
			"4200":  true,
			"5000":  true,
			"5173":  true,
			"8000":  true,
			"8080":  true,
			"9000":  true,
		}
		if devPorts[port] {
			return nil
		}
	}

	return &SecurityFlag{
		Type:      "non_standard_port",
		Severity:  "medium",
		Origin:    origin,
		Message:   "Origin uses non-standard port " + port + " which may indicate compromised infrastructure",
		Timestamp: time.Now(),
	}
}

// checkMixedContent detects HTTP resources loaded on HTTPS pages
func checkMixedContent(entry NetworkWaterfallEntry, pageURL string) *SecurityFlag {
	pageParsed, err := url.Parse(pageURL)
	if err != nil || pageParsed.Scheme != "https" {
		return nil // Not an HTTPS page
	}

	entryParsed, err := url.Parse(entry.URL)
	if err != nil {
		return nil
	}

	if entryParsed.Scheme == "http" {
		// Determine severity based on resource type
		severity := "medium"
		if entry.InitiatorType == "script" || entry.InitiatorType == "stylesheet" {
			severity = "high" // Scripts and styles can inject malicious content
		}

		return &SecurityFlag{
			Type:      "mixed_content",
			Severity:  severity,
			Origin:    entryParsed.Scheme + "://" + entryParsed.Host,
			Message:   "HTTP resource loaded on HTTPS page (mixed content vulnerability)",
			Resource:  entry.URL,
			PageURL:   pageURL,
			Timestamp: time.Now(),
		}
	}

	return nil
}

// checkIPAddressOrigin flags direct IP access (often indicates compromised infrastructure)
func checkIPAddressOrigin(origin string) *SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()

	// Allow localhost
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return nil
	}

	// Check if hostname is an IP address
	if net.ParseIP(hostname) != nil {
		return &SecurityFlag{
			Type:      "ip_address_origin",
			Severity:  "medium",
			Origin:    origin,
			Message:   "Origin uses IP address instead of domain name, which may indicate compromised or temporary infrastructure",
			Timestamp: time.Now(),
		}
	}

	return nil
}

// checkTyposquatting detects domains similar to popular CDNs/services
func checkTyposquatting(origin string) *SecurityFlag {
	parsed, err := url.Parse(origin)
	if err != nil {
		return nil
	}

	hostname := parsed.Hostname()

	// Popular CDN/service domains to check against
	popularDomains := []string{
		"unpkg.com",
		"jsdelivr.net",
		"cdnjs.cloudflare.com",
		"cloudflare.com",
		"googleapis.com",
		"gstatic.com",
		"jquery.com",
		"bootstrap.com",
	}

	for _, popular := range popularDomains {
		// Calculate Levenshtein distance
		distance := levenshteinDistance(hostname, popular)

		// Flag if similar but not exact match (1-2 character difference)
		if distance > 0 && distance <= 2 {
			return &SecurityFlag{
				Type:      "potential_typosquatting",
				Severity:  "high",
				Origin:    origin,
				Message:   "Domain is similar to " + popular + " (possible typosquatting)",
				Timestamp: time.Now(),
			}
		}
	}

	return nil
}

// ============================================
// Analysis Orchestration
// ============================================

// analyzeNetworkSecurity runs all security checks on a network entry
func analyzeNetworkSecurity(entry NetworkWaterfallEntry, pageURL string) []SecurityFlag {
	origin := extractOrigin(entry.URL)
	if origin == "" {
		return nil
	}

	var flags []SecurityFlag

	// Run all detection algorithms
	checks := []func(string) *SecurityFlag{
		checkSuspiciousTLD,
		checkNonStandardPort,
		checkIPAddressOrigin,
		checkTyposquatting,
	}

	for _, check := range checks {
		if flag := check(origin); flag != nil {
			flag.Resource = entry.URL
			flag.PageURL = pageURL
			flags = append(flags, *flag)
		}
	}

	// Mixed content check requires both entry and page URL
	if flag := checkMixedContent(entry, pageURL); flag != nil {
		flags = append(flags, *flag)
	}

	return flags
}

// ============================================
// Helper Functions
// ============================================

// levenshteinDistance calculates edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
