// csp.go — CSP Generator: produces Content-Security-Policy headers from observed traffic.
// Maintains an append-only origin accumulator that records every unique
// origin+resourceType+pageURL combination. Independent of the ring buffer,
// so origins are never lost to eviction.
// Design: Confidence scoring prevents observation poisoning (single injected
// requests are excluded). Development pollution filtering removes extensions,
// HMR, and dev-only origins automatically.
package security

import (
	"encoding/json"
	"fmt"
	"github.com/dev-console/dev-console/internal/capture"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============================================
// CSP Generator Types
// ============================================

// OriginEntry records observations of a specific origin+resourceType pair.
type OriginEntry struct {
	Origin       string          `json:"origin"`
	ResourceType string          `json:"resource_type"`
	Pages        map[string]bool `json:"-"`
	Count        int             `json:"observation_count"`
	FirstSeen    time.Time       `json:"first_seen"`
	LastSeen     time.Time       `json:"last_seen"`
}

// CSPGenerator maintains the origin accumulator and generates CSP policies.
type CSPGenerator struct {
	mu      sync.RWMutex
	origins map[string]*OriginEntry // key: "origin|resourceType"
	pages   map[string]bool         // all unique page URLs observed
}

// CSPParams defines the input parameters for CSP generation.
type CSPParams struct {
	Mode              string   `json:"mode"` // strict, moderate, report_only
	IncludeReportURI  bool     `json:"include_report_uri"`
	ExcludeOrigins    []string `json:"exclude_origins"`
	WhitelistOverride []string `json:"whitelist_override,omitempty"` // SESSION-ONLY temporary whitelist (not persisted)
	SuppressFlags     []string `json:"suppress_flags,omitempty"`     // SESSION-ONLY flag suppression (not persisted)
}

// CSPResponse is the full response from GenerateCSP.
type CSPResponse struct {
	CSPHeader           string              `json:"csp_header"`
	HeaderName          string              `json:"header_name"`
	MetaTag             string              `json:"meta_tag"`
	Directives          map[string][]string `json:"directives"`
	OriginDetails       []OriginDetail      `json:"origin_details"`
	FilteredOrigins     []FilteredOrigin    `json:"filtered_origins"`
	Observations        CSPObservations     `json:"observations"`
	Warnings            []string            `json:"warnings"`
	RecommendedNextStep string              `json:"recommended_next_step"`
	Audit               *CSPAudit           `json:"audit,omitempty"` // Security boundary audit info
}

// OriginDetail provides per-origin confidence and inclusion info.
type OriginDetail struct {
	Origin           string   `json:"origin"`
	Directive        string   `json:"directive"`
	Confidence       string   `json:"confidence"`
	ObservationCount int      `json:"observation_count"`
	FirstSeen        string   `json:"first_seen"`
	LastSeen         string   `json:"last_seen"`
	PagesSeenOn      []string `json:"pages_seen_on"`
	Included         bool     `json:"included"`
	ExclusionReason  string   `json:"exclusion_reason,omitempty"`
}

// FilteredOrigin describes an origin that was automatically filtered.
type FilteredOrigin struct {
	Origin string `json:"origin"`
	Reason string `json:"reason"`
}

// CSPObservations summarizes the observation session.
type CSPObservations struct {
	TotalResources  int `json:"total_resources"`
	UniqueOrigins   int `json:"unique_origins"`
	OriginsIncluded int `json:"origins_included"`
	OriginsFiltered int `json:"origins_filtered"`
	PagesVisited    int `json:"pages_visited"`
}

// ============================================
// CSP Generator Constructor
// ============================================

// NewCSPGenerator creates a new CSP generator with an empty origin accumulator.
func NewCSPGenerator() *CSPGenerator {
	return &CSPGenerator{
		origins: make(map[string]*OriginEntry),
		pages:   make(map[string]bool),
	}
}

// ============================================
// Origin Recording
// ============================================

// RecordOrigin adds an observation of an origin+resourceType from a specific page.
func (g *CSPGenerator) RecordOrigin(origin, resourceType, pageURL string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := origin + "|" + resourceType
	now := time.Now()

	entry, exists := g.origins[key]
	if !exists {
		entry = &OriginEntry{
			Origin:       origin,
			ResourceType: resourceType,
			Pages:        make(map[string]bool),
			Count:        0,
			FirstSeen:    now,
		}
		g.origins[key] = entry
	}

	entry.Count++
	entry.LastSeen = now
	if len(entry.Pages) < 1000 {
		entry.Pages[pageURL] = true
	}

	if len(g.pages) < 1000 {
		g.pages[pageURL] = true
	}

	if len(g.origins) > 10000 {
		g.evictOldestOrigin()
	}
}

// evictOldestOrigin removes the origin entry with the earliest FirstSeen timestamp.
func (g *CSPGenerator) evictOldestOrigin() {
	var oldestKey string
	var oldestTime time.Time
	for k, v := range g.origins {
		if oldestKey == "" || v.FirstSeen.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.FirstSeen
		}
	}
	if oldestKey != "" {
		delete(g.origins, oldestKey)
	}
}

// Reset clears the origin accumulator (called on session reset).
func (g *CSPGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.origins = make(map[string]*OriginEntry)
	g.pages = make(map[string]bool)
}

// GetPages returns a copy of all observed page URLs.
func (g *CSPGenerator) GetPages() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pages := make([]string, 0, len(g.pages))
	for p := range g.pages {
		pages = append(pages, p)
	}
	return pages
}

// ============================================
// CSP Generation
// ============================================

// resourceTypeToDirective maps resource types to CSP directive names.
var resourceTypeToDirective = map[string]string{
	"script":  "script-src",
	"style":   "style-src",
	"font":    "font-src",
	"img":     "img-src",
	"connect": "connect-src",
	"frame":   "frame-src",
	"media":   "media-src",
	"worker":  "worker-src",
}

// originProcessingResult holds intermediate state from processing origin entries.
type originProcessingResult struct {
	directives      map[string]map[string]bool
	originDetails   []OriginDetail
	filteredOrigins []FilteredOrigin
	totalResources  int
	uniqueOrigins   int
	originsIncluded int
	originsFiltered int
}

// GenerateCSP produces a CSP policy from accumulated origin observations.
func (g *CSPGenerator) GenerateCSP(params CSPParams) CSPResponse {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if params.Mode == "" {
		params.Mode = "moderate"
	}

	excludeSet := make(map[string]bool)
	for _, origin := range params.ExcludeOrigins {
		excludeSet[origin] = true
	}

	result := g.processOriginEntries(excludeSet)
	sortedDirectives := sortDirectiveSources(result.directives)
	cspHeader := g.buildCSPHeader(sortedDirectives)

	headerName := "Content-Security-Policy"
	if params.Mode == "report_only" {
		headerName = "Content-Security-Policy-Report-Only"
	}

	response := CSPResponse{
		CSPHeader:  cspHeader,
		HeaderName: headerName,
		MetaTag:    formatMetaTag(cspHeader),
		Directives: sortedDirectives,
		OriginDetails:   result.originDetails,
		FilteredOrigins: result.filteredOrigins,
		Observations: CSPObservations{
			TotalResources:  result.totalResources,
			UniqueOrigins:   result.uniqueOrigins,
			OriginsIncluded: result.originsIncluded,
			OriginsFiltered: result.originsFiltered,
			PagesVisited:    len(g.pages),
		},
		Warnings:            g.buildWarnings(len(g.pages), result.originsFiltered, result.originDetails),
		RecommendedNextStep: "Deploy as Content-Security-Policy-Report-Only first. Browse all pages again and check for violations via Gasoline's console error capture. Once no violations occur, switch to enforcing mode.",
	}

	if len(params.WhitelistOverride) > 0 {
		g.applyWhitelistOverrides(&response, params.WhitelistOverride)
	}

	return response
}

// processOriginEntries iterates all accumulated origins and classifies each as
// included, filtered (dev pollution), or excluded (manual/low confidence).
func (g *CSPGenerator) processOriginEntries(excludeSet map[string]bool) originProcessingResult {
	pageOrigins := g.extractPageOrigins()
	directives := map[string]map[string]bool{
		"default-src": {"'self'": true},
	}
	uniqueOriginSet := make(map[string]bool)
	var r originProcessingResult

	for _, entry := range g.origins {
		r.totalResources += entry.Count
		uniqueOriginSet[entry.Origin] = true

		directive := directiveForResourceType(entry.ResourceType)

		if reason := g.isDevPollution(entry.Origin, pageOrigins); reason != "" {
			r.filteredOrigins = append(r.filteredOrigins, FilteredOrigin{Origin: entry.Origin, Reason: reason})
			r.originsFiltered++
			continue
		}
		if excludeSet[entry.Origin] {
			r.originsFiltered++
			continue
		}

		detail := g.buildOriginDetail(entry, directive)
		r.originDetails = append(r.originDetails, detail)

		if detail.Included {
			addDirectiveSource(directives, directive, entry.Origin)
			r.originsIncluded++
		}
	}

	r.directives = directives
	r.uniqueOrigins = len(uniqueOriginSet)
	return r
}

// directiveForResourceType returns the CSP directive name for a resource type,
// falling back to "default-src" for unknown types.
func directiveForResourceType(resourceType string) string {
	if d := resourceTypeToDirective[resourceType]; d != "" {
		return d
	}
	return "default-src"
}

// addDirectiveSource adds an origin to the given directive's source set.
func addDirectiveSource(directives map[string]map[string]bool, directive, origin string) {
	if directives[directive] == nil {
		directives[directive] = make(map[string]bool)
	}
	directives[directive][origin] = true
}

// buildOriginDetail creates an OriginDetail for a single entry with confidence scoring.
func (g *CSPGenerator) buildOriginDetail(entry *OriginEntry, directive string) OriginDetail {
	confidence := g.computeConfidence(entry)
	included := confidence != "low"

	detail := OriginDetail{
		Origin:           entry.Origin,
		Directive:        directive,
		Confidence:       confidence,
		ObservationCount: entry.Count,
		FirstSeen:        entry.FirstSeen.Format(time.RFC3339),
		LastSeen:         entry.LastSeen.Format(time.RFC3339),
		PagesSeenOn:      g.entryPages(entry),
		Included:         included,
	}

	if !included {
		detail.ExclusionReason = "Low confidence: observed only once. May be injected by extension or ad network. Add to exclude_origins or manually include after verification."
	}
	return detail
}

// sortDirectiveSources converts directive sets to sorted string slices.
func sortDirectiveSources(directives map[string]map[string]bool) map[string][]string {
	sorted := make(map[string][]string, len(directives))
	for dir, sources := range directives {
		list := make([]string, 0, len(sources))
		for src := range sources {
			list = append(list, src)
		}
		sort.Strings(list)
		sorted[dir] = list
	}
	return sorted
}

// formatMetaTag wraps a CSP header string in an HTML meta tag.
func formatMetaTag(cspHeader string) string {
	return fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="%s">`, cspHeader)
}

// applyWhitelistOverrides applies session-only whitelist origins and logs security events.
func (g *CSPGenerator) applyWhitelistOverrides(response *CSPResponse, overrides []string) {
	if response.Directives["default-src"] == nil {
		response.Directives["default-src"] = []string{"'self'"}
	}
	response.Directives["default-src"] = append(response.Directives["default-src"], overrides...)

	response.CSPHeader = g.buildCSPHeader(response.Directives)
	response.MetaTag = formatMetaTag(response.CSPHeader)

	for _, origin := range overrides {
		response.Warnings = append(response.Warnings, fmt.Sprintf(
			"⚠️  SECURITY: Temporary whitelist override applied (SESSION-ONLY)\n"+
				"   Origin: %s\n"+
				"   Source: MCP tool parameter\n"+
				"   Action: Review origin legitimacy before permanent whitelist\n"+
				"\n"+
				"💡 To permanently whitelist (after human review):\n"+
				"   1. Verify origin is legitimate and trusted\n"+
				"   2. %s\n"+
				"   3. Add to 'whitelisted_origins' array",
			origin,
			securityConfigEditInstruction(),
		))
	}

	response.Audit = &CSPAudit{
		SessionOverrides:    overrides,
		PersistentWhitelist: []string{},
		OverrideSource:      "mcp_tool_parameter",
	}

	for _, origin := range overrides {
		LogSecurityEvent(SecurityAuditEvent{
			Action:     "whitelist_override",
			Origin:     origin,
			Reason:     "CSP generation with session-only override",
			Persistent: false,
			Source:     "mcp",
		})
	}
}

// ============================================
// Confidence Scoring
// ============================================

// computeConfidence determines the confidence level for an origin entry.
// High: 3+ observations AND 2+ pages
// Medium: 2+ observations OR 2+ pages
// Low: exactly 1 observation on 1 page
// Exception: connect-src has relaxed threshold (1 observation = medium)
func (g *CSPGenerator) computeConfidence(entry *OriginEntry) string {
	pageCount := len(entry.Pages)
	obsCount := entry.Count

	if isHighConfidence(obsCount, pageCount) {
		return "high"
	}
	if resourceTypeToDirective[entry.ResourceType] == "connect-src" {
		return "medium"
	}
	if obsCount >= 2 || pageCount >= 2 {
		return "medium"
	}
	return "low"
}

func isHighConfidence(obsCount, pageCount int) bool {
	return obsCount >= 3 && pageCount >= 2
}

// ============================================
// Development Pollution Filtering
// ============================================

// extensionPrefixes maps browser extension URL prefixes to their filter reasons.
var extensionPrefixes = []struct {
	prefix string
	reason string
}{
	{"chrome-extension://", "Browser extension origin (auto-filtered)"},
	{"moz-extension://", "Firefox extension origin (auto-filtered)"},
}

// isDevPollution checks if an origin is a known development-only pattern.
// Returns the reason string if filtered, empty string if not filtered.
func (g *CSPGenerator) isDevPollution(origin string, pageOrigins map[string]bool) string {
	if reason := matchExtensionPrefix(origin); reason != "" {
		return reason
	}
	return g.checkLocalhostDevServer(origin, pageOrigins)
}

// matchExtensionPrefix returns a filter reason if the origin matches a known browser extension prefix.
func matchExtensionPrefix(origin string) string {
	for _, ext := range extensionPrefixes {
		if strings.HasPrefix(origin, ext.prefix) {
			return ext.reason
		}
	}
	return ""
}

// checkLocalhostDevServer filters localhost origins that differ from observed page origins.
func (g *CSPGenerator) checkLocalhostDevServer(origin string, pageOrigins map[string]bool) string {
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	if host != "localhost" && host != "127.0.0.1" {
		return ""
	}
	if pageOrigins[origin] {
		return ""
	}
	return "Development server (auto-filtered, different port from app)"
}

// extractPageOrigins returns the set of origins from observed page URLs.
func (g *CSPGenerator) extractPageOrigins() map[string]bool {
	origins := make(map[string]bool)
	for pageURL := range g.pages {
		parsed, err := url.Parse(pageURL)
		if err != nil {
			continue
		}
		// Reconstruct origin: scheme://host[:port]
		origin := parsed.Scheme + "://" + parsed.Host
		origins[origin] = true
	}
	return origins
}

// ============================================
// CSP Header Building
// ============================================

// directiveOrder defines the canonical order for CSP directives.
var directiveOrder = []string{
	"default-src",
	"script-src",
	"style-src",
	"img-src",
	"font-src",
	"connect-src",
	"frame-src",
	"media-src",
	"worker-src",
	"base-uri",
	"form-action",
	"frame-ancestors",
}

// directiveOrderSet is built from directiveOrder for O(1) membership checks.
var directiveOrderSet = buildDirectiveOrderSet()

func buildDirectiveOrderSet() map[string]bool {
	s := make(map[string]bool, len(directiveOrder))
	for _, d := range directiveOrder {
		s[d] = true
	}
	return s
}

// buildCSPHeader builds the CSP header string from sorted directives.
func (g *CSPGenerator) buildCSPHeader(directives map[string][]string) string {
	parts := make([]string, 0, len(directives))

	for _, dir := range directiveOrder {
		if sources, ok := directives[dir]; ok && len(sources) > 0 {
			parts = append(parts, dir+" "+strings.Join(sources, " "))
		}
	}

	for dir, sources := range directives {
		if !directiveOrderSet[dir] && len(sources) > 0 {
			parts = append(parts, dir+" "+strings.Join(sources, " "))
		}
	}

	return strings.Join(parts, "; ")
}

// ============================================
// Warnings
// ============================================

// buildWarnings generates advisory warnings based on the observation state.
func (g *CSPGenerator) buildWarnings(pagesVisited, originsFiltered int, details []OriginDetail) []string {
	if len(g.origins) == 0 {
		return []string{"No origins observed yet. Browse your app to capture resource loading patterns before generating a CSP."}
	}

	var warnings []string
	if pagesVisited < 5 {
		warnings = append(warnings, fmt.Sprintf("Only %d pages visited — ensure all app routes are exercised for complete coverage.", pagesVisited))
	}
	if lowCount := countLowConfidenceExclusions(details); lowCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d origin(s) excluded due to low confidence (seen once) — review origin_details for details.", lowCount))
	}
	return warnings
}

// countLowConfidenceExclusions returns the number of origin details excluded due to low confidence.
func countLowConfidenceExclusions(details []OriginDetail) int {
	count := 0
	for _, d := range details {
		if d.Confidence == "low" && !d.Included {
			count++
		}
	}
	return count
}

// ============================================
// Helper Functions
// ============================================

// entryPages returns a sorted list of page URLs for an origin entry (up to 10).
func (g *CSPGenerator) entryPages(entry *OriginEntry) []string {
	var pages []string
	for p := range entry.Pages {
		pages = append(pages, p)
		if len(pages) >= 10 {
			break
		}
	}
	sort.Strings(pages)
	return pages
}

// ============================================
// Network Body Integration
// ============================================

// RecordOriginFromBody extracts origin and resource type from a NetworkBody
// and records it in the origin accumulator. Called from the network ingestion path.
func (g *CSPGenerator) RecordOriginFromBody(body capture.NetworkBody, pageURL string) {
	origin := extractOriginFromURL(body.URL)
	if origin == "" {
		return
	}
	resourceType := contentTypeToResourceType(body.ContentType)
	g.RecordOrigin(origin, resourceType, pageURL)
}

// extractOriginFromURL extracts scheme://host[:port] from a URL string.
func extractOriginFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// exactContentTypeMap maps exact Content-Type values to CSP resource categories.
var exactContentTypeMap = map[string]string{
	"text/css":                "style",
	"application/x-font-ttf":  "font",
	"application/x-font-woff": "font",
}

// contentTypeToResourceType maps HTTP Content-Type to CSP resource category.
func contentTypeToResourceType(ct string) string {
	ct = strings.ToLower(ct)
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	if res, ok := exactContentTypeMap[ct]; ok {
		return res
	}
	return contentTypePrefixMatch(ct)
}

// contentTypePrefixMatch classifies content types by prefix or substring patterns.
func contentTypePrefixMatch(ct string) string {
	if strings.Contains(ct, "javascript") {
		return "script"
	}
	if strings.HasPrefix(ct, "font/") || strings.Contains(ct, "application/font") {
		return "font"
	}
	if strings.HasPrefix(ct, "image/") {
		return "img"
	}
	if strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		return "media"
	}
	return "connect"
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleGenerateCSP is the MCP tool handler for generate_csp.
func (g *CSPGenerator) HandleGenerateCSP(params json.RawMessage) (any, error) {
	var cspParams CSPParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &cspParams); err != nil {
			return nil, fmt.Errorf("invalid CSP parameters: %w", err)
		}
	}

	resp := g.GenerateCSP(cspParams)
	return &resp, nil
}
