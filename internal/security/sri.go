// sri.go — SRI Hash Generator (generate_sri) MCP tool.
// Generates Subresource Integrity hashes for third-party scripts and stylesheets
// observed in network traffic. Protects against CDN compromise by computing
// SHA-384 hashes that browsers verify before executing resources.
// Design: Stateless analyzer operating on capture.NetworkBody data. Filters for JS/CSS
// content types from third-party origins, computes SHA-384 hashes, and generates
// ready-to-use HTML tag templates.
package security

import (
	"github.com/dev-console/dev-console/internal/capture"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// ============================================
// Types
// ============================================

// SRIGenerator generates Subresource Integrity hashes for third-party resources.
type SRIGenerator struct{}

// SRIParams defines input parameters for the generate_sri tool.
type SRIParams struct {
	ResourceTypes []string `json:"resource_types"` // "scripts", "styles" - default: both
	Origins       []string `json:"origins"`        // Filter to specific origins
	OutputFormat  string   `json:"output_format"`  // "html", "json", "webpack", "vite" - default: html
}

// SRIResult is the full response from the generate_sri tool.
type SRIResult struct {
	Resources []SRIResource `json:"resources"`
	Summary   SRISummary    `json:"summary"`
	Warnings  []string      `json:"warnings,omitempty"`
}

// SRIResource represents a single resource with its computed SRI hash.
type SRIResource struct {
	URL          string `json:"url"`
	Type         string `json:"type"`          // "script" or "style"
	Hash         string `json:"hash"`          // "sha384-{base64hash}"
	Crossorigin  string `json:"crossorigin"`   // Always "anonymous"
	TagTemplate  string `json:"tag_template"`  // Ready-to-use HTML tag
	SizeBytes    int    `json:"size_bytes"`
	AlreadyHasSRI bool  `json:"already_has_sri"`
}

// SRISummary provides aggregate counts for the SRI generation.
type SRISummary struct {
	TotalThirdPartyResources int `json:"total_third_party_resources"`
	ScriptsWithoutSRI        int `json:"scripts_without_sri"`
	StylesWithoutSRI         int `json:"styles_without_sri"`
	AlreadyProtected         int `json:"already_protected"`
	HashesGenerated          int `json:"hashes_generated"`
}

// ============================================
// Constructor
// ============================================

// NewSRIGenerator creates a new SRIGenerator instance.
func NewSRIGenerator() *SRIGenerator {
	return &SRIGenerator{}
}

// ============================================
// Main Generation Logic
// ============================================

// sriFilterConfig holds pre-computed filter state for SRI generation.
type sriFilterConfig struct {
	firstPartyOrigins map[string]bool
	originFilter      map[string]bool
	includeScripts    bool
	includeStyles     bool
}

// newSRIFilterConfig builds the filter config from params and page URLs.
func newSRIFilterConfig(pageURLs []string, params SRIParams) sriFilterConfig {
	cfg := sriFilterConfig{
		firstPartyOrigins: make(map[string]bool),
		originFilter:      make(map[string]bool),
		includeScripts:    true,
		includeStyles:     true,
	}
	for _, pageURL := range pageURLs {
		if origin := extractOriginForSRI(pageURL); origin != "" {
			cfg.firstPartyOrigins[origin] = true
		}
	}
	for _, o := range params.Origins {
		cfg.originFilter[o] = true
	}
	if len(params.ResourceTypes) > 0 {
		cfg.includeScripts = false
		cfg.includeStyles = false
		for _, rt := range params.ResourceTypes {
			switch rt {
			case "scripts":
				cfg.includeScripts = true
			case "styles":
				cfg.includeStyles = true
			}
		}
	}
	return cfg
}

// shouldIncludeResourceType returns true if this resource type passes the filter.
func (cfg sriFilterConfig) shouldIncludeResourceType(resType string) bool {
	return (resType == "script" && cfg.includeScripts) || (resType == "style" && cfg.includeStyles)
}

// hasVaryUserAgent checks if response headers contain Vary: User-Agent.
func hasVaryUserAgent(headers map[string]string) bool {
	if headers == nil {
		return false
	}
	for key, val := range headers {
		if strings.EqualFold(key, "Vary") && strings.Contains(strings.ToLower(val), "user-agent") {
			return true
		}
	}
	return false
}

// sriBodyOutcome describes the result of evaluating a single network body for SRI.
type sriBodyOutcome struct {
	thirdParty bool
	resType    string // "script", "style", or "" if not applicable
	skip       bool   // true when filtered out or duplicate
	truncated  bool
	varyUA     bool
	resource   SRIResource
}

// evaluateBody checks a single network body against filters and, when eligible,
// computes its SRI hash. The seenURLs map is updated for deduplication.
func (g *SRIGenerator) evaluateBody(body capture.NetworkBody, cfg sriFilterConfig, seenURLs map[string]bool) sriBodyOutcome {
	if body.ResponseBody == "" {
		return sriBodyOutcome{skip: true}
	}
	origin := extractOriginForSRI(body.URL)
	if origin == "" || cfg.firstPartyOrigins[origin] {
		return sriBodyOutcome{skip: true}
	}

	resType := sriResourceType(body.ContentType)
	if resType == "" {
		return sriBodyOutcome{thirdParty: true, skip: true}
	}
	if !cfg.shouldIncludeResourceType(resType) || (len(cfg.originFilter) > 0 && !cfg.originFilter[origin]) || seenURLs[body.URL] {
		return sriBodyOutcome{thirdParty: true, resType: resType, skip: true}
	}
	seenURLs[body.URL] = true

	if body.ResponseTruncated {
		return sriBodyOutcome{thirdParty: true, resType: resType, truncated: true}
	}

	hash := computeSHA384(body.ResponseBody)
	return sriBodyOutcome{
		thirdParty: true,
		resType:    resType,
		varyUA:     hasVaryUserAgent(body.ResponseHeaders),
		resource: SRIResource{
			URL: body.URL, Type: resType, Hash: hash,
			Crossorigin: "anonymous", TagTemplate: generateTagTemplate(body.URL, hash, resType),
			SizeBytes: len(body.ResponseBody), AlreadyHasSRI: false,
		},
	}
}

// buildSRIWarnings converts collected URL lists into human-readable warning strings.
func buildSRIWarnings(truncated, varyUA []string) []string {
	warnings := make([]string, 0, len(truncated)+len(varyUA))
	for _, u := range truncated {
		warnings = append(warnings, fmt.Sprintf("%s — body was truncated, cannot compute SRI hash. Consider increasing capture limit.", u))
	}
	for _, u := range varyUA {
		warnings = append(warnings, fmt.Sprintf("%s — responds with Vary: User-Agent header. SRI hash may differ across browsers.", u))
	}
	return warnings
}

// Generate analyzes network bodies and produces SRI hashes for third-party scripts/styles.
func (g *SRIGenerator) Generate(bodies []capture.NetworkBody, pageURLs []string, params SRIParams) SRIResult {
	cfg := newSRIFilterConfig(pageURLs, params)
	result := SRIResult{Resources: []SRIResource{}, Warnings: []string{}}
	seenURLs := make(map[string]bool)

	var totalThirdParty, scriptsWithoutSRI, stylesWithoutSRI int
	var truncated, varyUA []string

	for _, body := range bodies {
		out := g.evaluateBody(body, cfg, seenURLs)
		if out.thirdParty {
			totalThirdParty++
		}
		switch out.resType {
		case "script":
			scriptsWithoutSRI++
		case "style":
			stylesWithoutSRI++
		}
		if out.truncated {
			truncated = append(truncated, body.URL)
			continue
		}
		if out.skip || out.resType == "" {
			continue
		}
		if out.varyUA {
			varyUA = append(varyUA, body.URL)
		}
		result.Resources = append(result.Resources, out.resource)
	}

	result.Warnings = buildSRIWarnings(truncated, varyUA)
	result.Summary = SRISummary{
		TotalThirdPartyResources: totalThirdParty,
		ScriptsWithoutSRI:        scriptsWithoutSRI,
		StylesWithoutSRI:         stylesWithoutSRI,
		HashesGenerated:          len(result.Resources),
	}
	return result
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleGenerateSRI processes MCP tool call parameters and generates SRI hashes.
func HandleGenerateSRI(params json.RawMessage, bodies []capture.NetworkBody, pageURLs []string) (any, error) {
	var toolParams SRIParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &toolParams); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}

	gen := NewSRIGenerator()
	result := gen.Generate(bodies, pageURLs, toolParams)
	return result, nil
}

// ============================================
// Helper Functions
// ============================================

// computeSHA384 computes the SHA-384 hash of content and returns it in SRI format.
func computeSHA384(content string) string {
	hasher := sha512.New384()
	hasher.Write([]byte(content))
	hash := hasher.Sum(nil)
	b64 := base64.StdEncoding.EncodeToString(hash)
	return "sha384-" + b64
}

// sriResourceType returns "script" or "style" based on content type, or empty string if not applicable.
func sriResourceType(contentType string) string {
	ct := strings.ToLower(contentType)

	// Strip parameters (e.g., "text/css; charset=utf-8" -> "text/css")
	if idx := strings.IndexByte(ct, ';'); idx >= 0 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)

	// JavaScript types
	if strings.Contains(ct, "javascript") {
		return "script"
	}

	// CSS
	if ct == "text/css" {
		return "style"
	}

	return ""
}

// extractOriginForSRI extracts scheme://host[:port] from a URL string.
func extractOriginForSRI(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// generateTagTemplate creates an HTML tag with SRI attributes.
func generateTagTemplate(resourceURL, hash, resType string) string {
	if resType == "script" {
		return fmt.Sprintf(`<script src="%s" integrity="%s" crossorigin="anonymous"></script>`, resourceURL, hash)
	}
	if resType == "style" {
		return fmt.Sprintf(`<link rel="stylesheet" href="%s" integrity="%s" crossorigin="anonymous">`, resourceURL, hash)
	}
	return ""
}
