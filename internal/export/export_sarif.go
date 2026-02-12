// export_sarif.go — SARIF 2.1.0 accessibility report generation.
// Converts axe-core accessibility audit results into the Static Analysis
// Results Interchange Format, compatible with GitHub Code Scanning and
// other SARIF-consuming tools.
// Design: Each axe-core violation becomes a SARIF result with rule metadata,
// affected element locations, and remediation guidance.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
// SPEC:SARIF — SARIF 2.1.0 fields use camelCase per OASIS specification.
// SPEC:axe-core — axeViolation.HelpURL uses camelCase per axe-core library output.
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// version is set at build time via -ldflags "-X ...internal/export.version=..."
// Fallback used for `go run` (no ldflags).
var version = "dev"

// SARIF 2.1.0 specification constants
const (
	sarifSpecVersion = "2.1.0"
	sarifSchemaURL   = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
)

// ============================================
// SARIF 2.1.0 Types
// ============================================

// SARIFLog is the top-level SARIF 2.1.0 object.
type SARIFLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a single analysis run.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool describes the analysis tool.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver describes the tool driver (primary component).
type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"` // SPEC:SARIF
	Rules          []SARIFRule `json:"rules"`
}

// SARIFRule describes a single analysis rule.
type SARIFRule struct {
	ID               string               `json:"id"`
	ShortDescription SARIFMessage         `json:"shortDescription"` // SPEC:SARIF
	FullDescription  SARIFMessage         `json:"fullDescription"`  // SPEC:SARIF
	HelpURI          string               `json:"helpUri"`          // SPEC:SARIF
	Properties       *SARIFRuleProperties `json:"properties,omitempty"`
}

// SARIFRuleProperties holds additional rule metadata.
type SARIFRuleProperties struct {
	Tags []string `json:"tags,omitempty"`
}

// SARIFResult represents a single analysis finding.
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`    // SPEC:SARIF
	RuleIndex int             `json:"ruleIndex"` // SPEC:SARIF
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations"`
}

// SARIFMessage is a simple text message.
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFLocation represents a finding location.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"` // SPEC:SARIF
}

// SARIFPhysicalLocation describes the physical location of a finding.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"` // SPEC:SARIF
	Region           SARIFRegion           `json:"region"`
}

// SARIFArtifactLocation identifies the artifact (file, DOM element, etc.).
type SARIFArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"` // SPEC:SARIF
}

// SARIFRegion describes a region within an artifact.
type SARIFRegion struct {
	Snippet SARIFSnippet `json:"snippet"`
}

// SARIFSnippet contains a text snippet of the region.
type SARIFSnippet struct {
	Text string `json:"text"`
}

// ============================================
// Export Options
// ============================================

// SARIFExportOptions controls the export behavior.
type SARIFExportOptions struct {
	Scope         string `json:"scope"`
	IncludePasses bool   `json:"include_passes"`
	SaveTo        string `json:"save_to"`
}

// ============================================
// Axe-Core Result Types (for parsing)
// ============================================

type axeResult struct {
	Violations   []axeViolation `json:"violations"`
	Passes       []axeViolation `json:"passes"`
	Incomplete   []axeViolation `json:"incomplete"`
	Inapplicable []axeViolation `json:"inapplicable"`
}

type axeViolation struct {
	ID          string    `json:"id"`
	Impact      string    `json:"impact"`
	Description string    `json:"description"`
	Help        string    `json:"help"`
	HelpURL     string    `json:"helpUrl"` // SPEC:axe-core
	Tags        []string  `json:"tags"`
	Nodes       []axeNode `json:"nodes"`
}

type axeNode struct {
	HTML   string   `json:"html"`
	Target []string `json:"target"`
	Impact string   `json:"impact"`
}

// ============================================
// Conversion Functions
// ============================================

// convertViolationsToResults converts axe violations to SARIF results.
func convertViolationsToResults(run *SARIFRun, ruleIndices map[string]int, violations []axeViolation) {
	for i := range violations {
		v := violations[i]
		ruleIdx := ensureRule(run, ruleIndices, v)
		for _, node := range v.Nodes {
			run.Results = append(run.Results, nodeToResult(v, node, ruleIdx, axeImpactToLevel(node.Impact)))
		}
	}
}

// convertPassesToResults converts axe passes to SARIF results with "none" level.
func convertPassesToResults(run *SARIFRun, ruleIndices map[string]int, passes []axeViolation) {
	for i := range passes {
		p := passes[i]
		ruleIdx := ensureRule(run, ruleIndices, p)
		for _, node := range p.Nodes {
			run.Results = append(run.Results, nodeToResult(p, node, ruleIdx, "none"))
		}
	}
}

// ExportSARIF converts an axe-core accessibility result to SARIF 2.1.0 format.
// If opts.SaveTo is set, it also writes the SARIF JSON to the specified file.
func ExportSARIF(a11yResultJSON json.RawMessage, opts SARIFExportOptions) (*SARIFLog, error) {
	var axe axeResult
	if err := json.Unmarshal(a11yResultJSON, &axe); err != nil {
		return nil, fmt.Errorf("failed to parse a11y result: %w", err)
	}

	log := &SARIFLog{
		Schema:  sarifSchemaURL,
		Version: sarifSpecVersion,
		Runs: []SARIFRun{{
			Tool: SARIFTool{
				Driver: SARIFDriver{
					Name:           "Gasoline",
					Version:        version,
					InformationURI: "https://github.com/anthropics/gasoline",
					Rules:          []SARIFRule{},
				},
			},
			Results: []SARIFResult{},
		}},
	}

	run := &log.Runs[0]
	ruleIndices := make(map[string]int)

	convertViolationsToResults(run, ruleIndices, axe.Violations)
	if opts.IncludePasses {
		convertPassesToResults(run, ruleIndices, axe.Passes)
	}

	if opts.SaveTo != "" {
		if err := saveSARIFToFile(log, opts.SaveTo); err != nil {
			return nil, err
		}
	}
	return log, nil
}

// ensureRule adds a rule to the driver rules if not already present, returns the index.
func ensureRule(run *SARIFRun, indices map[string]int, v axeViolation) int {
	if idx, exists := indices[v.ID]; exists {
		return idx
	}

	rule := SARIFRule{
		ID:               v.ID,
		ShortDescription: SARIFMessage{Text: v.Description},
		FullDescription:  SARIFMessage{Text: v.Help},
		HelpURI:          v.HelpURL,
	}

	wcagTags := extractWCAGTags(v.Tags)
	if len(wcagTags) > 0 {
		rule.Properties = &SARIFRuleProperties{Tags: wcagTags}
	}

	idx := len(run.Tool.Driver.Rules)
	run.Tool.Driver.Rules = append(run.Tool.Driver.Rules, rule)
	indices[v.ID] = idx
	return idx
}

// nodeToResult converts a single axe node to a SARIF result.
func nodeToResult(v axeViolation, node axeNode, ruleIndex int, level string) SARIFResult {
	selector := ""
	if len(node.Target) > 0 {
		selector = node.Target[0]
	}

	return SARIFResult{
		RuleID:    v.ID,
		RuleIndex: ruleIndex,
		Level:     level,
		Message:   SARIFMessage{Text: v.Help},
		Locations: []SARIFLocation{{
			PhysicalLocation: SARIFPhysicalLocation{
				ArtifactLocation: SARIFArtifactLocation{
					URI: selector,
				},
				Region: SARIFRegion{
					Snippet: SARIFSnippet{Text: node.HTML},
				},
			},
		}},
	}
}

// axeImpactToLevel maps axe-core impact levels to SARIF levels.
func axeImpactToLevel(impact string) string {
	switch impact {
	case "critical", "serious":
		return "error"
	case "moderate":
		return "warning"
	case "minor":
		return "note"
	default:
		return "warning"
	}
}

// extractWCAGTags filters a slice of axe-core tags to only those starting with "wcag".
func extractWCAGTags(tags []string) []string {
	result := make([]string, 0)
	for _, tag := range tags {
		if strings.HasPrefix(tag, "wcag") {
			result = append(result, tag)
		}
	}
	return result
}

// resolveExistingPath resolves symlinks on the longest existing prefix of the path.
// For paths where the file doesn't exist yet, it resolves the nearest existing
// ancestor and appends the remaining path components.
func resolveExistingPath(path string) string {
	path = filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved
	}
	// Path doesn't exist; resolve parent and append this component
	parent := filepath.Dir(path)
	if parent == path {
		return path // reached root
	}
	return filepath.Join(resolveExistingPath(parent), filepath.Base(path))
}

// isPathUnderResolvedDir checks if resolvedPath is under an allowed directory.
func isPathUnderResolvedDir(resolvedPath, dir string) bool {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false
	}
	return strings.HasPrefix(resolvedPath, resolved+string(os.PathSeparator))
}

// validateSARIFSavePath checks that the path is under an allowed directory.
func validateSARIFSavePath(absPath, resolvedPath string) error {
	if isPathUnderResolvedDir(resolvedPath, os.TempDir()) {
		return nil
	}
	if cwd, err := os.Getwd(); err == nil && isPathUnderResolvedDir(resolvedPath, cwd) {
		return nil
	}
	return fmt.Errorf("save_to path must be under the current working directory or temp directory: %s", absPath)
}

// saveSARIFToFile writes the SARIF log to the specified path with security checks.
func saveSARIFToFile(log *SARIFLog, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	resolvedPath := resolveExistingPath(absPath)
	if err := validateSARIFSavePath(absPath, resolvedPath); err != nil {
		return err
	}

	// #nosec G301 -- 0755 for export directory is appropriate
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SARIF: %w", err)
	}

	// #nosec G306 -- export files are intentionally world-readable
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write SARIF file: %w", err)
	}
	return nil
}
