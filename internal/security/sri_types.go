// Purpose: Defines types for SRI generator, parameters, and output entries.
// Why: Centralizes SRI type definitions so generation, helpers, and tooling share one schema.
package security

// SRIGenerator generates Subresource Integrity hashes for third-party resources.
type SRIGenerator struct{}

// SRIParams defines filter/output options for generate_sri tool.
//
// Invariants:
// - ResourceTypes accepts scripts/styles; unknown values are ignored.
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
	URL           string `json:"url"`
	Type          string `json:"type"` // "script" or "style"
	Hash          string `json:"hash"` // "sha384-{base64hash}"
	Crossorigin   string `json:"crossorigin"`
	TagTemplate   string `json:"tag_template"` // Ready-to-use HTML tag
	SizeBytes     int    `json:"size_bytes"`
	AlreadyHasSRI bool   `json:"already_has_sri"`
}

// SRISummary provides aggregate counts for the SRI generation.
type SRISummary struct {
	TotalThirdPartyResources int `json:"total_third_party_resources"`
	ScriptsWithoutSRI        int `json:"scripts_without_sri"`
	StylesWithoutSRI         int `json:"styles_without_sri"`
	AlreadyProtected         int `json:"already_protected"`
	HashesGenerated          int `json:"hashes_generated"`
}

// sriFilterConfig holds pre-computed include/exclude decisions for one run.
//
// Invariants:
// - firstPartyOrigins is derived from pageURLs and treated as trust boundary for third-party detection.
type sriFilterConfig struct {
	firstPartyOrigins map[string]bool
	originFilter      map[string]bool
	includeScripts    bool
	includeStyles     bool
}

// sriBodyOutcome describes the result of evaluating a single network body for SRI.
type sriBodyOutcome struct {
	thirdParty  bool
	resType     string // "script", "style", or "" if not applicable
	skip        bool   // true when filtered out or duplicate
	truncated   bool
	placeholder bool // true when body is a capture placeholder, not real content
	varyUA      bool
	resource    SRIResource
}

// NewSRIGenerator creates a new SRIGenerator instance.
func NewSRIGenerator() *SRIGenerator {
	return &SRIGenerator{}
}
