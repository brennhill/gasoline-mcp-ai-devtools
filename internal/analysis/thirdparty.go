// thirdparty.go — Third-Party Risk Audit (audit_third_parties) MCP tool.
// Analyzes captured network traffic to map all third-party origins,
// classify risk levels, detect outbound PII, and apply domain reputation heuristics.
// Design: Stateless analyzer operating on capture.NetworkBody data. Risk classification
// uses resource type + data flow direction. Reputation uses bundled heuristics
// (known CDNs, abuse TLDs, DGA detection) with no network calls.
package analysis

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
)

// ThirdPartyAuditor performs third-party domain analysis.
type ThirdPartyAuditor struct{}

// ThirdPartyParams defines input for the audit tool.
type ThirdPartyParams struct {
	FirstPartyOrigins []string     `json:"first_party_origins"`
	IncludeStatic     *bool        `json:"include_static"`
	CustomLists       *CustomLists `json:"custom_lists"`
	CustomListsFile   string       `json:"custom_lists_file"`
}

// CustomLists defines enterprise custom domain lists.
type CustomLists struct {
	Allowed  []string `json:"allowed"`
	Blocked  []string `json:"blocked"`
	Internal []string `json:"internal"`
}

// ThirdPartyResult is the full audit response.
type ThirdPartyResult struct {
	FirstPartyOrigin string            `json:"first_party_origin"`
	ThirdParties     []ThirdPartyEntry `json:"third_parties"`
	Summary          ThirdPartySummary `json:"summary"`
	Recommendations  []string          `json:"recommendations"`
}

// ThirdPartyEntry describes a single third-party origin.
type ThirdPartyEntry struct {
	Origin          string           `json:"origin"`
	RiskLevel       string           `json:"risk_level"`
	RiskReason      string           `json:"risk_reason"`
	Resources       ResourceCounts   `json:"resources"`
	DataOutbound    bool             `json:"data_outbound"`
	OutboundDetails *OutboundDetails `json:"outbound_details,omitempty"`
	SetsCookies     bool             `json:"sets_cookies"`
	RequestCount    int              `json:"request_count"`
	TotalBytes      int64            `json:"total_transfer_bytes"`
	URLs            []string         `json:"urls"`
	Reputation      DomainReputation `json:"reputation"`
}

// ResourceCounts tracks resource types loaded from an origin.
type ResourceCounts struct {
	Scripts int `json:"scripts"`
	Styles  int `json:"styles"`
	Fonts   int `json:"fonts"`
	Images  int `json:"images"`
	Other   int `json:"other"`
}

// OutboundDetails describes data sent to a third party.
type OutboundDetails struct {
	Methods      []string `json:"methods"`
	ContentTypes []string `json:"content_types"`
	PIIFields    []string `json:"contains_pii_fields,omitempty"`
}

// DomainReputation is the reputation assessment for a domain.
type DomainReputation struct {
	Classification string   `json:"classification"` // known_cdn, suspicious, unknown, enterprise_allowed, enterprise_blocked
	Source         string   `json:"source,omitempty"`
	SuspicionFlags []string `json:"suspicion_flags,omitempty"`
	Notes          string   `json:"notes,omitempty"`
}

// ThirdPartySummary provides aggregate counts.
type ThirdPartySummary struct {
	TotalThirdParties     int `json:"total_third_parties"`
	CriticalRisk          int `json:"critical_risk"`
	HighRisk              int `json:"high_risk"`
	MediumRisk            int `json:"medium_risk"`
	LowRisk               int `json:"low_risk"`
	ScriptsFromThirdParty int `json:"scripts_from_third_parties"`
	OriginsReceivingData  int `json:"origins_receiving_data"`
	OriginsSettingCookies int `json:"origins_setting_cookies"`
	SuspiciousOrigins     int `json:"suspicious_origins"`
}

// knownCDNs is a hardcoded set of known CDN hostnames.
var knownCDNs = map[string]bool{
	"cdn.jsdelivr.net":           true,
	"cdnjs.cloudflare.com":      true,
	"unpkg.com":                  true,
	"fonts.googleapis.com":       true,
	"fonts.gstatic.com":          true,
	"ajax.googleapis.com":        true,
	"code.jquery.com":            true,
	"maxcdn.bootstrapcdn.com":    true,
	"stackpath.bootstrapcdn.com": true,
	"cdn.cloudflare.com":         true,
	"raw.githubusercontent.com":  true,
	"github.githubassets.com":    true,
	"cdn.tailwindcss.com":        true,
	"esm.sh":                     true,
	"deno.land":                  true,
	"cdn.skypack.dev":            true,
}

// abuseTLDs is a hardcoded set of TLDs commonly associated with abuse.
var abuseTLDs = map[string]bool{
	".xyz":     true,
	".top":     true,
	".click":   true,
	".loan":    true,
	".work":    true,
	".tk":      true,
	".ml":      true,
	".ga":      true,
	".cf":      true,
	".gq":      true,
	".buzz":    true,
	".monster": true,
}

// NewThirdPartyAuditor creates a new ThirdPartyAuditor.
func NewThirdPartyAuditor() *ThirdPartyAuditor {
	return &ThirdPartyAuditor{}
}

// Audit analyzes network bodies to identify and classify third-party origins.
func (a *ThirdPartyAuditor) Audit(bodies []capture.NetworkBody, pageURLs []string, params ThirdPartyParams) ThirdPartyResult {
	// Load custom lists from file if specified
	customLists := params.CustomLists
	if params.CustomListsFile != "" && customLists == nil {
		if loaded := loadCustomListsFile(params.CustomListsFile); loaded != nil {
			customLists = loaded
		}
	}

	// Determine first-party origins
	firstPartyOrigins := make(map[string]bool)
	if len(params.FirstPartyOrigins) > 0 {
		for _, o := range params.FirstPartyOrigins {
			firstPartyOrigins[o] = true
		}
	} else {
		for _, pageURL := range pageURLs {
			origin := extractOrigin(pageURL)
			if origin != "" {
				firstPartyOrigins[origin] = true
			}
		}
	}

	// Treat internal domains as first-party
	if customLists != nil {
		for _, internal := range customLists.Internal {
			origin := internal
			// If it looks like a bare hostname, add scheme
			if !strings.Contains(origin, "://") {
				origin = "https://" + origin
			}
			parsed := extractOrigin(origin)
			if parsed != "" {
				firstPartyOrigins[parsed] = true
			}
		}
	}

	// Record the primary first-party origin for result
	primaryFirstParty := ""
	if len(pageURLs) > 0 {
		primaryFirstParty = extractOrigin(pageURLs[0])
	}

	// Group bodies by origin
	type originData struct {
		bodies []capture.NetworkBody
		urls   []string
	}
	originMap := make(map[string]*originData)

	for _, body := range bodies {
		origin := extractOrigin(body.URL)
		if origin == "" {
			continue
		}
		if firstPartyOrigins[origin] {
			continue
		}
		if _, ok := originMap[origin]; !ok {
			originMap[origin] = &originData{}
		}
		originMap[origin].bodies = append(originMap[origin].bodies, body)
		originMap[origin].urls = append(originMap[origin].urls, body.URL)
	}

	// Build entries for each third-party origin
	entries := make([]ThirdPartyEntry, 0)
	for origin, data := range originMap {
		entry := buildThirdPartyEntry(origin, data.bodies, data.urls, customLists)
		entries = append(entries, entry)
	}

	// Filter by include_static
	if params.IncludeStatic != nil && !*params.IncludeStatic {
		var filtered []ThirdPartyEntry
		for _, entry := range entries {
			if entry.RiskLevel != "low" {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	// Sort entries by risk level (critical first)
	sort.Slice(entries, func(i, j int) bool {
		return riskOrder(entries[i].RiskLevel) < riskOrder(entries[j].RiskLevel)
	})

	// Build summary
	summary := buildThirdPartySummary(entries)

	// Build recommendations
	recommendations := buildRecommendations(entries)

	return ThirdPartyResult{
		FirstPartyOrigin: primaryFirstParty,
		ThirdParties:     entries,
		Summary:          summary,
		Recommendations:  recommendations,
	}
}

// buildThirdPartyEntry creates a ThirdPartyEntry for a single origin.
func buildThirdPartyEntry(origin string, bodies []capture.NetworkBody, urls []string, customLists *CustomLists) ThirdPartyEntry {
	entry := ThirdPartyEntry{
		Origin:       origin,
		RequestCount: len(bodies),
	}

	// Count resources by type
	for _, body := range bodies {
		resType := contentTypeToResourceType(body.ContentType)
		switch resType {
		case "script":
			entry.Resources.Scripts++
		case "style":
			entry.Resources.Styles++
		case "font":
			entry.Resources.Fonts++
		case "image":
			entry.Resources.Images++
		default:
			entry.Resources.Other++
		}

		// Check for Set-Cookie header
		if body.ResponseHeaders != nil {
			for key := range body.ResponseHeaders {
				if strings.EqualFold(key, "set-cookie") {
					entry.SetsCookies = true
					break
				}
			}
		}
	}

	// Check for outbound data (POST/PUT/PATCH)
	var outboundMethods []string
	var outboundContentTypes []string
	var allPIIFields []string
	methodSet := make(map[string]bool)
	ctSet := make(map[string]bool)

	for _, body := range bodies {
		method := strings.ToUpper(body.Method)
		if method == "POST" || method == "PUT" || method == "PATCH" {
			entry.DataOutbound = true
			if !methodSet[method] {
				outboundMethods = append(outboundMethods, method)
				methodSet[method] = true
			}
			if body.ContentType != "" && !ctSet[body.ContentType] {
				outboundContentTypes = append(outboundContentTypes, body.ContentType)
				ctSet[body.ContentType] = true
			}
			// Scan for PII in request body
			if body.RequestBody != "" {
				piiFields := detectPIIFields(body.RequestBody)
				for _, f := range piiFields {
					allPIIFields = appendUnique(allPIIFields, f)
				}
			}
		}
	}

	if entry.DataOutbound {
		entry.OutboundDetails = &OutboundDetails{
			Methods:      outboundMethods,
			ContentTypes: outboundContentTypes,
			PIIFields:    allPIIFields,
		}
	}

	// Determine risk level
	hasScripts := entry.Resources.Scripts > 0
	hasOutbound := entry.DataOutbound
	hasCookies := entry.SetsCookies

	switch {
	case hasScripts && hasOutbound:
		entry.RiskLevel = "critical"
		entry.RiskReason = "loads executable scripts AND sends data outbound"
	case hasScripts:
		entry.RiskLevel = "high"
		entry.RiskReason = "loads executable scripts (can execute code in page context)"
	case hasOutbound || hasCookies:
		entry.RiskLevel = "medium"
		if hasOutbound && hasCookies {
			entry.RiskReason = "receives outbound data and sets cookies"
		} else if hasOutbound {
			entry.RiskReason = "receives outbound data"
		} else {
			entry.RiskReason = "sets cookies for tracking"
		}
	default:
		entry.RiskLevel = "low"
		entry.RiskReason = "static assets only (images, fonts, styles)"
	}

	// Apply reputation heuristics
	hostname := extractHostname(origin)
	entry.Reputation = classifyReputation(hostname, customLists)

	// Override risk for blocked domains
	if entry.Reputation.Classification == "enterprise_blocked" {
		entry.RiskLevel = "critical"
		entry.RiskReason = "domain is on enterprise blocked list"
	}

	// Collect up to 10 URLs
	urlLimit := 10
	if len(urls) < urlLimit {
		urlLimit = len(urls)
	}
	entry.URLs = urls[:urlLimit]

	return entry
}

// classifyReputation determines the reputation classification for a hostname.
func classifyReputation(hostname string, customLists *CustomLists) DomainReputation {
	rep := DomainReputation{
		Classification: "unknown",
	}

	// Check custom lists first
	if customLists != nil {
		for _, allowed := range customLists.Allowed {
			if hostname == allowed || strings.HasSuffix(hostname, "."+allowed) {
				rep.Classification = "enterprise_allowed"
				rep.Source = "custom_list"
				return rep
			}
		}
		for _, blocked := range customLists.Blocked {
			if hostname == blocked || strings.HasSuffix(hostname, "."+blocked) {
				rep.Classification = "enterprise_blocked"
				rep.Source = "custom_list"
				return rep
			}
		}
	}

	// Check known CDNs
	if knownCDNs[hostname] {
		rep.Classification = "known_cdn"
		rep.Source = "bundled_list"
		return rep
	}

	// Check abuse TLD
	var flags []string
	tld := extractTLD(hostname)
	if abuseTLDs[tld] {
		flags = append(flags, "abuse_tld")
	}

	// Check DGA
	subdomain := extractSubdomain(hostname)
	if subdomain != "" && len(subdomain) > 8 {
		entropy := shannonEntropy(subdomain)
		if entropy > 3.5 {
			flags = append(flags, "possible_dga")
		}
	}

	if len(flags) >= 1 {
		rep.Classification = "suspicious"
		rep.SuspicionFlags = flags
		rep.Source = "heuristic"
	}

	return rep
}

// shannonEntropy computes Shannon entropy in bits per character.
func shannonEntropy(s string) float64 {
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	length := float64(len([]rune(s)))
	entropy := 0.0
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

// extractHostname extracts the hostname from a URL or origin string.
func extractHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// extractTLD extracts the TLD with dot prefix from a hostname.
func extractTLD(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return ""
	}
	return "." + parts[len(parts)-1]
}

// extractSubdomain extracts the first subdomain label from a hostname.
func extractSubdomain(hostname string) string {
	parts := strings.Split(hostname, ".")
	if len(parts) <= 2 {
		return ""
	}
	return parts[0]
}

// buildThirdPartySummary computes aggregate counts from entries.
func buildThirdPartySummary(entries []ThirdPartyEntry) ThirdPartySummary {
	s := ThirdPartySummary{
		TotalThirdParties: len(entries),
	}
	for _, e := range entries {
		switch e.RiskLevel {
		case "critical":
			s.CriticalRisk++
		case "high":
			s.HighRisk++
		case "medium":
			s.MediumRisk++
		case "low":
			s.LowRisk++
		}
		if e.Resources.Scripts > 0 {
			s.ScriptsFromThirdParty++
		}
		if e.DataOutbound {
			s.OriginsReceivingData++
		}
		if e.SetsCookies {
			s.OriginsSettingCookies++
		}
		if e.Reputation.Classification == "suspicious" {
			s.SuspiciousOrigins++
		}
	}
	return s
}

// buildRecommendations generates actionable recommendation strings.
func buildRecommendations(entries []ThirdPartyEntry) []string {
	recs := make([]string, 0)

	// Check for suspicious origins with scripts
	for _, e := range entries {
		if e.Reputation.Classification == "suspicious" && e.Resources.Scripts > 0 {
			recs = append(recs, fmt.Sprintf(
				"CRITICAL: %s loads scripts AND is flagged suspicious — investigate immediately",
				e.Origin,
			))
		}
	}

	// Count origins receiving data
	dataReceivers := 0
	for _, e := range entries {
		if e.DataOutbound {
			dataReceivers++
		}
	}
	if dataReceivers > 0 {
		recs = append(recs, fmt.Sprintf(
			"%d origin(s) receive user data — verify these are intentional",
			dataReceivers,
		))
	}

	// Check for PII in outbound data
	for _, e := range entries {
		if e.OutboundDetails != nil && len(e.OutboundDetails.PIIFields) > 0 {
			recs = append(recs, fmt.Sprintf(
				"%s receives PII fields (%s) — ensure privacy policy covers this",
				e.Origin,
				strings.Join(e.OutboundDetails.PIIFields, ", "),
			))
		}
	}

	// Check for cookie-setting third parties
	cookieSetters := 0
	for _, e := range entries {
		if e.SetsCookies {
			cookieSetters++
		}
	}
	if cookieSetters > 0 {
		recs = append(recs, fmt.Sprintf(
			"%d third-party origin(s) set cookies — review for GDPR/CCPA compliance",
			cookieSetters,
		))
	}

	return recs
}

// riskOrder returns a sort order for risk levels (lower = more severe).
func riskOrder(level string) int {
	switch level {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

// appendUnique appends a value to a slice if not already present.
func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// loadCustomListsFile reads and parses a CustomLists JSON file.
func loadCustomListsFile(path string) *CustomLists {
	data, err := os.ReadFile(path) // #nosec G304 -- path is from trusted config location
	if err != nil {
		return nil
	}
	var lists CustomLists
	if err := json.Unmarshal(data, &lists); err != nil {
		return nil
	}
	return &lists
}

// HandleAuditThirdParties is the MCP handler for the audit_third_parties tool.
func HandleAuditThirdParties(params json.RawMessage, bodies []capture.NetworkBody, pageURLs []string) (any, error) {
	var p ThirdPartyParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	auditor := NewThirdPartyAuditor()
	result := auditor.Audit(bodies, pageURLs, p)
	return result, nil
}

// ============================================
// Helper Functions (duplicated from security package to avoid circular imports)
// ============================================

// extractOrigin extracts the origin (scheme://host[:port]) from a URL.
// Returns empty string for data: URLs, blob: URLs (after extracting nested origin), and malformed URLs.
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

// contentTypeToResourceType maps content-type to a resource category.
func contentTypeToResourceType(ct string) string {
	ct = strings.ToLower(ct)
	ct = strings.Split(ct, ";")[0] // strip charset etc
	ct = strings.TrimSpace(ct)

	switch {
	case strings.HasPrefix(ct, "application/javascript"),
		strings.HasPrefix(ct, "text/javascript"):
		return "script"
	case strings.HasPrefix(ct, "text/css"):
		return "style"
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "font/"),
		strings.Contains(ct, "woff"),
		strings.Contains(ct, "opentype"):
		return "font"
	case strings.HasPrefix(ct, "text/html"):
		return "document"
	case strings.HasPrefix(ct, "application/json"):
		return "fetch"
	case strings.Contains(ct, "form"):
		return "fetch"
	default:
		return "other"
	}
}

// detectPIIFields scans a request/response body for common PII patterns.
// Returns a list of detected PII field names (email, phone, ssn).
func detectPIIFields(body string) []string {
	var fields []string
	if piiEmailPattern.MatchString(body) {
		fields = append(fields, "email")
	}
	if piiPhonePattern.MatchString(body) {
		fields = append(fields, "phone")
	}
	if piiSSNPattern.MatchString(body) {
		fields = append(fields, "ssn")
	}
	return fields
}

// PII detection patterns for detectPIIFields
var (
	piiEmailPattern = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	piiPhonePattern = regexp.MustCompile(`\b(\+?1[-.]?)?\(?[0-9]{3}\)?[-. ]?[0-9]{3}[-. ]?[0-9]{4}\b`)
	piiSSNPattern   = regexp.MustCompile(`\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b`)
)
