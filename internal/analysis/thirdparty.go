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

// NewThirdPartyAuditor creates a new ThirdPartyAuditor.
func NewThirdPartyAuditor() *ThirdPartyAuditor {
	return &ThirdPartyAuditor{}
}

// originData groups network bodies and URLs for a single origin.
type originData struct {
	bodies []capture.NetworkBody
	urls   []string
}

// Audit analyzes network bodies to identify and classify third-party origins.
func (a *ThirdPartyAuditor) Audit(bodies []capture.NetworkBody, pageURLs []string, params ThirdPartyParams) ThirdPartyResult {
	customLists := resolveCustomLists(params)
	firstPartyOrigins := buildFirstPartySet(params, pageURLs, customLists)

	originMap := groupByThirdPartyOrigin(bodies, firstPartyOrigins)
	entries := buildAllEntries(originMap, customLists)
	entries = filterAndSort(entries, params.IncludeStatic)

	primaryFirstParty := ""
	if len(pageURLs) > 0 {
		primaryFirstParty = extractOrigin(pageURLs[0])
	}

	return ThirdPartyResult{
		FirstPartyOrigin: primaryFirstParty,
		ThirdParties:     entries,
		Summary:          buildThirdPartySummary(entries),
		Recommendations:  buildRecommendations(entries),
	}
}

// resolveCustomLists loads custom lists from file if needed.
func resolveCustomLists(params ThirdPartyParams) *CustomLists {
	if params.CustomLists != nil {
		return params.CustomLists
	}
	if params.CustomListsFile != "" {
		return loadCustomListsFile(params.CustomListsFile)
	}
	return nil
}

// buildFirstPartySet determines first-party origins from params and page URLs.
func buildFirstPartySet(params ThirdPartyParams, pageURLs []string, customLists *CustomLists) map[string]bool {
	firstParty := make(map[string]bool)
	if len(params.FirstPartyOrigins) > 0 {
		for _, o := range params.FirstPartyOrigins {
			firstParty[o] = true
		}
	} else {
		for _, pageURL := range pageURLs {
			if origin := extractOrigin(pageURL); origin != "" {
				firstParty[origin] = true
			}
		}
	}
	addInternalDomains(firstParty, customLists)
	return firstParty
}

// addInternalDomains treats custom internal domains as first-party.
func addInternalDomains(firstParty map[string]bool, customLists *CustomLists) {
	if customLists == nil {
		return
	}
	for _, internal := range customLists.Internal {
		origin := internal
		if !strings.Contains(origin, "://") {
			origin = "https://" + origin
		}
		if parsed := extractOrigin(origin); parsed != "" {
			firstParty[parsed] = true
		}
	}
}

// groupByThirdPartyOrigin groups bodies by third-party origin (excluding first-party).
func groupByThirdPartyOrigin(bodies []capture.NetworkBody, firstParty map[string]bool) map[string]*originData {
	originMap := make(map[string]*originData)
	for _, body := range bodies {
		origin := extractOrigin(body.URL)
		if origin == "" || firstParty[origin] {
			continue
		}
		if _, ok := originMap[origin]; !ok {
			originMap[origin] = &originData{}
		}
		originMap[origin].bodies = append(originMap[origin].bodies, body)
		originMap[origin].urls = append(originMap[origin].urls, body.URL)
	}
	return originMap
}

// buildAllEntries creates ThirdPartyEntry structs for each origin.
func buildAllEntries(originMap map[string]*originData, customLists *CustomLists) []ThirdPartyEntry {
	entries := make([]ThirdPartyEntry, 0, len(originMap))
	for origin, data := range originMap {
		entries = append(entries, buildThirdPartyEntry(origin, data.bodies, data.urls, customLists))
	}
	return entries
}

// filterAndSort optionally removes static-only entries and sorts by risk level.
func filterAndSort(entries []ThirdPartyEntry, includeStatic *bool) []ThirdPartyEntry {
	if includeStatic != nil && !*includeStatic {
		var filtered []ThirdPartyEntry
		for _, entry := range entries {
			if entry.RiskLevel != "low" {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}
	sort.Slice(entries, func(i, j int) bool {
		return riskOrder(entries[i].RiskLevel) < riskOrder(entries[j].RiskLevel)
	})
	return entries
}

// buildThirdPartyEntry creates a ThirdPartyEntry for a single origin.
func buildThirdPartyEntry(origin string, bodies []capture.NetworkBody, urls []string, customLists *CustomLists) ThirdPartyEntry {
	entry := ThirdPartyEntry{
		Origin:       origin,
		RequestCount: len(bodies),
	}

	countResources(&entry, bodies)
	collectOutboundData(&entry, bodies)
	classifyRiskLevel(&entry)

	hostname := extractHostname(origin)
	entry.Reputation = classifyReputation(hostname, customLists)
	if entry.Reputation.Classification == "enterprise_blocked" {
		entry.RiskLevel = "critical"
		entry.RiskReason = "domain is on enterprise blocked list"
	}

	urlLimit := 10
	if len(urls) < urlLimit {
		urlLimit = len(urls)
	}
	entry.URLs = urls[:urlLimit]
	return entry
}

// countResources counts resource types and detects cookie-setting from response headers.
func countResources(entry *ThirdPartyEntry, bodies []capture.NetworkBody) {
	for _, body := range bodies {
		switch contentTypeToResourceType(body.ContentType) {
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
		if !entry.SetsCookies && hasSetCookieHeader(body.ResponseHeaders) {
			entry.SetsCookies = true
		}
	}
}

// hasSetCookieHeader checks if response headers contain a Set-Cookie header.
func hasSetCookieHeader(headers map[string]string) bool {
	if headers == nil {
		return false
	}
	for key := range headers {
		if strings.EqualFold(key, "set-cookie") {
			return true
		}
	}
	return false
}

// collectOutboundData detects outbound data methods, content types, and PII.
func collectOutboundData(entry *ThirdPartyEntry, bodies []capture.NetworkBody) {
	methodSet := make(map[string]bool)
	ctSet := make(map[string]bool)
	var outboundMethods, outboundContentTypes, allPIIFields []string

	for _, body := range bodies {
		method := strings.ToUpper(body.Method)
		if method != "POST" && method != "PUT" && method != "PATCH" {
			continue
		}
		entry.DataOutbound = true
		if !methodSet[method] {
			outboundMethods = append(outboundMethods, method)
			methodSet[method] = true
		}
		if body.ContentType != "" && !ctSet[body.ContentType] {
			outboundContentTypes = append(outboundContentTypes, body.ContentType)
			ctSet[body.ContentType] = true
		}
		if body.RequestBody != "" {
			for _, f := range detectPIIFields(body.RequestBody) {
				allPIIFields = appendUnique(allPIIFields, f)
			}
		}
	}

	if entry.DataOutbound {
		entry.OutboundDetails = &OutboundDetails{
			Methods: outboundMethods, ContentTypes: outboundContentTypes, PIIFields: allPIIFields,
		}
	}
}

// classifyRiskLevel determines the risk level and reason based on resources and data flow.
func classifyRiskLevel(entry *ThirdPartyEntry) {
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
	case hasOutbound && hasCookies:
		entry.RiskLevel = "medium"
		entry.RiskReason = "receives outbound data and sets cookies"
	case hasOutbound:
		entry.RiskLevel = "medium"
		entry.RiskReason = "receives outbound data"
	case hasCookies:
		entry.RiskLevel = "medium"
		entry.RiskReason = "sets cookies for tracking"
	default:
		entry.RiskLevel = "low"
		entry.RiskReason = "static assets only (images, fonts, styles)"
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
