// sri.go — SRI Hash Generator (generate_sri) MCP tool.
// Generates Subresource Integrity hashes for third-party scripts and stylesheets
// observed in network traffic. Protects against CDN compromise by computing
// SHA-384 hashes that browsers verify before executing resources.
// Design: Stateless analyzer operating on NetworkBody data. Filters for JS/CSS
// content types from third-party origins, computes SHA-384 hashes, and generates
// ready-to-use HTML tag templates.
package main

import (
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

// Generate analyzes network bodies and produces SRI hashes for third-party scripts/styles.
func (g *SRIGenerator) Generate(bodies []NetworkBody, pageURLs []string, params SRIParams) SRIResult {
	result := SRIResult{
		Resources: []SRIResource{},
		Warnings:  []string{},
	}

	// Build set of first-party origins from page URLs
	firstPartyOrigins := make(map[string]bool)
	for _, pageURL := range pageURLs {
		origin := extractOriginForSRI(pageURL)
		if origin != "" {
			firstPartyOrigins[origin] = true
		}
	}

	// Build origin filter set if specified
	originFilter := make(map[string]bool)
	for _, o := range params.Origins {
		originFilter[o] = true
	}

	// Determine which resource types to include
	includeScripts := true
	includeStyles := true
	if len(params.ResourceTypes) > 0 {
		includeScripts = false
		includeStyles = false
		for _, rt := range params.ResourceTypes {
			if rt == "scripts" {
				includeScripts = true
			}
			if rt == "styles" {
				includeStyles = true
			}
		}
	}

	// Track seen URLs for deduplication
	seenURLs := make(map[string]bool)

	// Track stats for summary
	totalThirdParty := 0
	scriptsWithoutSRI := 0
	stylesWithoutSRI := 0
	truncatedResources := []string{}
	varyUserAgentResources := []string{}

	for _, body := range bodies {
		// Skip empty bodies
		if body.ResponseBody == "" {
			continue
		}

		// Extract origin and check if third-party
		origin := extractOriginForSRI(body.URL)
		if origin == "" {
			continue
		}

		// Skip first-party origins
		if firstPartyOrigins[origin] {
			continue
		}

		// Count as third-party resource (all types)
		totalThirdParty++

		// Determine resource type from content-type
		resType := sriResourceType(body.ContentType)
		if resType == "" {
			continue // Not a script or style - skip hash generation but counted above
		}

		// Track scripts/styles for summary
		switch resType {
		case "script":
			scriptsWithoutSRI++
		case "style":
			stylesWithoutSRI++
		}

		// Apply resource type filter
		if resType == "script" && !includeScripts {
			continue
		}
		if resType == "style" && !includeStyles {
			continue
		}

		// Apply origin filter if specified
		if len(originFilter) > 0 && !originFilter[origin] {
			continue
		}

		// Skip duplicates
		if seenURLs[body.URL] {
			continue
		}
		seenURLs[body.URL] = true

		// Check for truncated body
		if body.ResponseTruncated {
			truncatedResources = append(truncatedResources, body.URL)
			continue
		}

		// Check for Vary: User-Agent header
		if body.ResponseHeaders != nil {
			for key, val := range body.ResponseHeaders {
				if strings.EqualFold(key, "Vary") && strings.Contains(strings.ToLower(val), "user-agent") {
					varyUserAgentResources = append(varyUserAgentResources, body.URL)
				}
			}
		}

		// Compute SHA-384 hash
		hash := computeSHA384(body.ResponseBody)

		// Generate tag template
		tagTemplate := generateTagTemplate(body.URL, hash, resType)

		resource := SRIResource{
			URL:          body.URL,
			Type:         resType,
			Hash:         hash,
			Crossorigin:  "anonymous",
			TagTemplate:  tagTemplate,
			SizeBytes:    len(body.ResponseBody),
			AlreadyHasSRI: false,
		}

		result.Resources = append(result.Resources, resource)
	}

	// Build warnings
	for _, url := range truncatedResources {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s — body was truncated, cannot compute SRI hash. Consider increasing capture limit.", url))
	}
	for _, url := range varyUserAgentResources {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%s — responds with Vary: User-Agent header. SRI hash may differ across browsers.", url))
	}

	// Build summary
	result.Summary = SRISummary{
		TotalThirdPartyResources: totalThirdParty,
		ScriptsWithoutSRI:        scriptsWithoutSRI,
		StylesWithoutSRI:         stylesWithoutSRI,
		AlreadyProtected:         0,
		HashesGenerated:          len(result.Resources),
	}

	return result
}

// ============================================
// MCP Tool Handler
// ============================================

// HandleGenerateSRI processes MCP tool call parameters and generates SRI hashes.
func HandleGenerateSRI(params json.RawMessage, bodies []NetworkBody, pageURLs []string) (interface{}, error) {
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
