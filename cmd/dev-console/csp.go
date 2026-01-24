// csp.go — CSP Generator: produces Content-Security-Policy headers from observed traffic.
// Maintains an append-only origin accumulator that records every unique
// origin+resourceType+pageURL combination. Independent of the ring buffer,
// so origins are never lost to eviction.
// Design: Confidence scoring prevents observation poisoning (single injected
// requests are excluded). Development pollution filtering removes extensions,
// HMR, and dev-only origins automatically.
package main

import (
	"encoding/json"
	"fmt"
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
	Mode            string   `json:"mode"`             // strict, moderate, report_only
	IncludeReportURI bool    `json:"include_report_uri"`
	ExcludeOrigins  []string `json:"exclude_origins"`
}

// CSPResponse is the full response from GenerateCSP.
type CSPResponse struct {
	CSPHeader         string                       `json:"csp_header"`
	HeaderName        string                       `json:"header_name"`
	MetaTag           string                       `json:"meta_tag"`
	Directives        map[string][]string          `json:"directives"`
	OriginDetails     []OriginDetail               `json:"origin_details"`
	FilteredOrigins   []FilteredOrigin             `json:"filtered_origins"`
	Observations      CSPObservations              `json:"observations"`
	Warnings          []string                     `json:"warnings"`
	RecommendedNextStep string                     `json:"recommended_next_step"`
}

// OriginDetail provides per-origin confidence and inclusion info.
type OriginDetail struct {
	Origin          string   `json:"origin"`
	Directive       string   `json:"directive"`
	Confidence      string   `json:"confidence"`
	ObservationCount int     `json:"observation_count"`
	FirstSeen       string   `json:"first_seen"`
	LastSeen        string   `json:"last_seen"`
	PagesSeenOn     []string `json:"pages_seen_on"`
	Included        bool     `json:"included"`
	ExclusionReason string   `json:"exclusion_reason,omitempty"`
}

// FilteredOrigin describes an origin that was automatically filtered.
type FilteredOrigin struct {
	Origin string `json:"origin"`
	Reason string `json:"reason"`
}

// CSPObservations summarizes the observation session.
type CSPObservations struct {
	TotalResources int `json:"total_resources"`
	UniqueOrigins  int `json:"unique_origins"`
	OriginsIncluded int `json:"origins_included"`
	OriginsFiltered int `json:"origins_filtered"`
	PagesVisited   int `json:"pages_visited"`
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
	entry.Pages[pageURL] = true

	g.pages[pageURL] = true
}

// Reset clears the origin accumulator (called on session reset).
func (g *CSPGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.origins = make(map[string]*OriginEntry)
	g.pages = make(map[string]bool)
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

// GenerateCSP produces a CSP policy from accumulated origin observations.
func (g *CSPGenerator) GenerateCSP(params CSPParams) CSPResponse {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if params.Mode == "" {
		params.Mode = "moderate"
	}

	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, origin := range params.ExcludeOrigins {
		excludeSet[origin] = true
	}

	// Determine page origins (for first-party detection)
	pageOrigins := g.extractPageOrigins()

	// Process all entries
	directives := map[string]map[string]bool{
		"default-src": {"'self'": true},
	}
	var originDetails []OriginDetail
	var filteredOrigins []FilteredOrigin
	totalResources := 0
	uniqueOriginSet := make(map[string]bool)
	originsIncluded := 0
	originsFiltered := 0

	for _, entry := range g.origins {
		totalResources += entry.Count
		uniqueOriginSet[entry.Origin] = true

		directive := resourceTypeToDirective[entry.ResourceType]
		if directive == "" {
			directive = "default-src"
		}

		// Check if filtered (dev pollution)
		if reason := g.isDevPollution(entry.Origin, pageOrigins); reason != "" {
			filteredOrigins = append(filteredOrigins, FilteredOrigin{
				Origin: entry.Origin,
				Reason: reason,
			})
			originsFiltered++
			continue
		}

		// Check manual exclusion
		if excludeSet[entry.Origin] {
			originsFiltered++
			continue
		}

		// Compute confidence
		confidence := g.computeConfidence(entry)
		included := confidence != "low"

		pages := g.entryPages(entry)

		detail := OriginDetail{
			Origin:           entry.Origin,
			Directive:        directive,
			Confidence:       confidence,
			ObservationCount: entry.Count,
			FirstSeen:        entry.FirstSeen.Format(time.RFC3339),
			LastSeen:         entry.LastSeen.Format(time.RFC3339),
			PagesSeenOn:      pages,
			Included:         included,
		}

		if !included {
			detail.ExclusionReason = "Low confidence: observed only once. May be injected by extension or ad network. Add to exclude_origins or manually include after verification."
		}

		originDetails = append(originDetails, detail)

		if included {
			if directives[directive] == nil {
				directives[directive] = make(map[string]bool)
			}
			directives[directive][entry.Origin] = true
			originsIncluded++
		}
	}

	// Build sorted directives map
	sortedDirectives := make(map[string][]string)
	for dir, sources := range directives {
		var sorted []string
		for src := range sources {
			sorted = append(sorted, src)
		}
		sort.Strings(sorted)
		sortedDirectives[dir] = sorted
	}

	// Build CSP header string
	cspHeader := g.buildCSPHeader(sortedDirectives)

	// Determine header name
	headerName := "Content-Security-Policy"
	if params.Mode == "report_only" {
		headerName = "Content-Security-Policy-Report-Only"
	}

	// Build meta tag
	metaTag := fmt.Sprintf(`<meta http-equiv="Content-Security-Policy" content="%s">`, cspHeader)

	// Build warnings
	warnings := g.buildWarnings(len(g.pages), originsFiltered, originDetails)

	// Build observations
	observations := CSPObservations{
		TotalResources:  totalResources,
		UniqueOrigins:   len(uniqueOriginSet),
		OriginsIncluded: originsIncluded,
		OriginsFiltered: originsFiltered,
		PagesVisited:    len(g.pages),
	}

	// Recommended next step
	recommendedNextStep := "Deploy as Content-Security-Policy-Report-Only first. Browse all pages again and check for violations via Gasoline's console error capture. Once no violations occur, switch to enforcing mode."

	return CSPResponse{
		CSPHeader:           cspHeader,
		HeaderName:          headerName,
		MetaTag:             metaTag,
		Directives:          sortedDirectives,
		OriginDetails:       originDetails,
		FilteredOrigins:     filteredOrigins,
		Observations:        observations,
		Warnings:            warnings,
		RecommendedNextStep: recommendedNextStep,
	}
}

// ============================================
// Confidence Scoring
// ============================================

// computeConfidence determines the confidence level for an origin entry.
// High: 3+ observations AND 2+ pages (spec says 5+ times across 2+ pages for high,
//       but also says 3+ times AND 2+ pages in the task description; using the
//       task description criteria: 3+ AND 2+ pages = high)
// Medium: 2+ observations OR 2+ pages
// Low: exactly 1 observation on 1 page
// Exception: connect-src has relaxed threshold (1 observation = medium)
func (g *CSPGenerator) computeConfidence(entry *OriginEntry) string {
	pageCount := len(entry.Pages)
	obsCount := entry.Count

	// connect-src relaxation: single observation still gets medium
	directive := resourceTypeToDirective[entry.ResourceType]
	if directive == "connect-src" && obsCount >= 1 {
		if obsCount >= 3 && pageCount >= 2 {
			return "high"
		}
		return "medium"
	}

	// High: seen 3+ times AND on 2+ pages
	if obsCount >= 3 && pageCount >= 2 {
		return "high"
	}

	// Medium: seen 2+ times OR on 2+ pages
	if obsCount >= 2 || pageCount >= 2 {
		return "medium"
	}

	// Low: exactly 1 observation on 1 page
	return "low"
}

// ============================================
// Development Pollution Filtering
// ============================================

// isDevPollution checks if an origin is a known development-only pattern.
// Returns the reason string if filtered, empty string if not filtered.
func (g *CSPGenerator) isDevPollution(origin string, pageOrigins map[string]bool) string {
	// chrome-extension://
	if strings.HasPrefix(origin, "chrome-extension://") {
		return "Browser extension origin (auto-filtered)"
	}

	// moz-extension://
	if strings.HasPrefix(origin, "moz-extension://") {
		return "Firefox extension origin (auto-filtered)"
	}

	// Parse the origin to check localhost patterns
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}

	host := parsed.Hostname()

	// Check if it's localhost or 127.0.0.1
	if host == "localhost" || host == "127.0.0.1" {
		// If this origin matches a page origin, it's first-party (not filtered)
		if pageOrigins[origin] {
			return ""
		}
		// Different port from page = dev server
		return "Development server (auto-filtered, different port from app)"
	}

	return ""
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

// buildCSPHeader builds the CSP header string from sorted directives.
func (g *CSPGenerator) buildCSPHeader(directives map[string][]string) string {
	var parts []string

	for _, dir := range directiveOrder {
		if sources, ok := directives[dir]; ok && len(sources) > 0 {
			parts = append(parts, dir+" "+strings.Join(sources, " "))
		}
	}

	// Include any directives not in the canonical order
	for dir, sources := range directives {
		found := false
		for _, ordered := range directiveOrder {
			if dir == ordered {
				found = true
				break
			}
		}
		if !found && len(sources) > 0 {
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
	var warnings []string

	if len(g.origins) == 0 {
		warnings = append(warnings, "No origins observed yet. Browse your app to capture resource loading patterns before generating a CSP.")
		return warnings
	}

	// Low page count warning
	if pagesVisited < 5 {
		warnings = append(warnings, fmt.Sprintf("Only %d pages visited — ensure all app routes are exercised for complete coverage.", pagesVisited))
	}

	// Count low-confidence exclusions
	lowCount := 0
	for _, d := range details {
		if d.Confidence == "low" && !d.Included {
			lowCount++
		}
	}
	if lowCount > 0 {
		warnings = append(warnings, fmt.Sprintf("%d origin(s) excluded due to low confidence (seen once) — review origin_details for details.", lowCount))
	}

	return warnings
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
func (g *CSPGenerator) RecordOriginFromBody(body NetworkBody, pageURL string) {
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

// contentTypeToResourceType maps HTTP Content-Type to CSP resource category.
func contentTypeToResourceType(ct string) string {
	ct = strings.ToLower(ct)

	// Strip parameters (e.g., "text/html; charset=utf-8" → "text/html")
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	switch {
	case strings.Contains(ct, "javascript"):
		return "script"
	case ct == "text/css":
		return "style"
	case strings.HasPrefix(ct, "font/") || strings.Contains(ct, "application/font") || ct == "application/x-font-ttf" || ct == "application/x-font-woff":
		return "font"
	case strings.HasPrefix(ct, "image/"):
		return "img"
	case strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/"):
		return "media"
	default:
		// API calls, JSON, etc. → connect-src
		return "connect"
	}
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleGenerateCSP is the MCP tool handler for generate_csp.
func (g *CSPGenerator) HandleGenerateCSP(params json.RawMessage) (interface{}, error) {
	var cspParams CSPParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &cspParams); err != nil {
			return nil, fmt.Errorf("invalid CSP parameters: %w", err)
		}
	}

	resp := g.GenerateCSP(cspParams)
	return &resp, nil
}
