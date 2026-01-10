// export_sarif.go — SARIF 2.1.0 accessibility report generation.
// Converts axe-core accessibility audit results into the Static Analysis
// Results Interchange Format, compatible with GitHub Code Scanning and
// other SARIF-consuming tools.
// Design: Each axe-core violation becomes a SARIF result with rule metadata,
// affected element locations, and remediation guidance.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	InformationURI string      `json:"informationUri"` // camelCase: SARIF 2.1.0 spec standard
	Rules          []SARIFRule `json:"rules"`
}

// SARIFRule describes a single analysis rule.
type SARIFRule struct {
	ID               string               `json:"id"`
	ShortDescription SARIFMessage         `json:"shortDescription"` // camelCase: SARIF 2.1.0 spec standard
	FullDescription  SARIFMessage         `json:"fullDescription"`  // camelCase: SARIF 2.1.0 spec standard
	HelpURI          string               `json:"helpUri"`          // camelCase: SARIF 2.1.0 spec standard
	Properties       *SARIFRuleProperties `json:"properties,omitempty"`
}

// SARIFRuleProperties holds additional rule metadata.
type SARIFRuleProperties struct {
	Tags []string `json:"tags,omitempty"`
}

// SARIFResult represents a single analysis finding.
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`    // camelCase: SARIF 2.1.0 spec standard
	RuleIndex int             `json:"ruleIndex"` // camelCase: SARIF 2.1.0 spec standard
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
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"` // camelCase: SARIF 2.1.0 spec standard
}

// SARIFPhysicalLocation describes the physical location of a finding.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"` // camelCase: SARIF 2.1.0 spec standard
	Region           SARIFRegion           `json:"region"`
}

// SARIFArtifactLocation identifies the artifact (file, DOM element, etc.).
type SARIFArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"` // camelCase: SARIF 2.1.0 spec standard
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
	HelpURL     string    `json:"helpUrl"`
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

// ExportSARIF converts an axe-core accessibility result to SARIF 2.1.0 format.
// If opts.SaveTo is set, it also writes the SARIF JSON to the specified file.
func ExportSARIF(a11yResultJSON json.RawMessage, opts SARIFExportOptions) (*SARIFLog, error) {
	var axe axeResult
	if err := json.Unmarshal(a11yResultJSON, &axe); err != nil {
		return nil, fmt.Errorf("failed to parse a11y result: %w", err)
	}

	log := &SARIFLog{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json",
		Version: "2.1.0",
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

	// Track rule indices for deduplication
	ruleIndices := make(map[string]int)

	// Convert violations
	for i := range axe.Violations {
		v := axe.Violations[i]
		ruleIdx := ensureRule(run, ruleIndices, v)
		for _, node := range v.Nodes {
			result := nodeToResult(v, node, ruleIdx, axeImpactToLevel(node.Impact))
			run.Results = append(run.Results, result)
		}
	}

	// Convert passes if requested
	if opts.IncludePasses {
		for i := range axe.Passes {
			p := axe.Passes[i]
			ruleIdx := ensureRule(run, ruleIndices, p)
			for _, node := range p.Nodes {
				result := nodeToResult(p, node, ruleIdx, "none")
				run.Results = append(run.Results, result)
			}
		}
	}

	// Save to file if requested
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

// saveSARIFToFile writes the SARIF log to the specified path with security checks.
func saveSARIFToFile(log *SARIFLog, path string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Security: resolve symlinks to prevent symlink-based path traversal.
	// We resolve the target path and the allowed base directories, then compare
	// the resolved paths to ensure the target stays within allowed boundaries.
	resolvedPath := resolveExistingPath(absPath)

	allowed := false

	// Check temp directory (resolved to handle symlinks in temp path itself)
	tmpDir := os.TempDir()
	if resolvedTmp, err := filepath.EvalSymlinks(tmpDir); err == nil {
		if strings.HasPrefix(resolvedPath, resolvedTmp+string(os.PathSeparator)) {
			allowed = true
		}
	}

	// Check current working directory
	if !allowed {
		cwd, err := os.Getwd()
		if err == nil {
			if resolvedCwd, err := filepath.EvalSymlinks(cwd); err == nil {
				if strings.HasPrefix(resolvedPath, resolvedCwd+string(os.PathSeparator)) {
					allowed = true
				}
			}
		}
	}

	if !allowed {
		return fmt.Errorf("save_to path must be under the current working directory or temp directory: %s", absPath)
	}

	// Ensure parent directory exists
	dir := filepath.Dir(absPath)
	// #nosec G301 -- 0755 for export directory is appropriate
	if err := os.MkdirAll(dir, 0o755); err != nil {
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
