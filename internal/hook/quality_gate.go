// quality_gate.go — Quality gate enforcement for Claude Code PostToolUse hooks.
// Reads .gasoline.json, loads standards doc, checks file size, and injects findings.

package hook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	gasolineConfigFile       = ".gasoline.json"
	defaultCodeStandardsFile = "gasoline-code-standards.md"
	defaultFileSizeLimit     = 800
	maxStandardsLines        = 150
)

// gasolineConfig is the structure of .gasoline.json.
type gasolineConfig struct {
	CodeStandards      string `json:"code_standards"`
	FileSizeLimit      int    `json:"file_size_limit"`
	DuplicateThreshold int    `json:"duplicate_threshold"`
}

// QualityGateResult holds the findings from the quality gate check.
type QualityGateResult struct {
	Context string
}

// RunQualityGate checks the edited/written file against project standards.
// Returns nil if no findings or if the file/config doesn't exist.
func RunQualityGate(input Input) *QualityGateResult {
	if input.ToolName != "Edit" && input.ToolName != "Write" {
		return nil
	}

	fields := input.ParseToolInput()
	filePath := fields.FilePath
	if filePath == "" {
		return nil
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil
	}

	projectRoot := findProjectRoot(filePath)
	if projectRoot == "" {
		return nil
	}

	cfg := loadGasolineConfig(filepath.Join(projectRoot, gasolineConfigFile))

	var parts []string

	// 1. Standards doc.
	standardsPath := filepath.Join(projectRoot, cfg.CodeStandards)
	if content, err := readHeadLines(standardsPath, maxStandardsLines); err == nil && content != "" {
		parts = append(parts,
			"=== PROJECT CODE STANDARDS ===",
			content,
			"=== END STANDARDS ===",
		)
	}

	// 2. File size check.
	if lineCount, err := countLines(filePath); err == nil {
		if lineCount > cfg.FileSizeLimit {
			parts = append(parts, fmt.Sprintf(
				"WARNING: %s is %d lines (limit: %d). This file must be split.",
				filePath, lineCount, cfg.FileSizeLimit))
		} else if lineCount > cfg.FileSizeLimit*90/100 {
			parts = append(parts, fmt.Sprintf(
				"NOTE: %s is %d lines (limit: %d). Approaching the limit — consider splitting.",
				filePath, lineCount, cfg.FileSizeLimit))
		}
	}

	// 3. Review instruction.
	if len(parts) > 0 {
		parts = append(parts,
			"QUALITY GATE: Review your change against the standards above. Fix any violations before proceeding.")
	}

	if len(parts) == 0 {
		return nil
	}

	return &QualityGateResult{
		Context: strings.Join(parts, "\n"),
	}
}

// findProjectRoot walks up from filePath looking for .gasoline.json.
func findProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)
	for dir != "/" && dir != "." {
		if _, err := os.Stat(filepath.Join(dir, gasolineConfigFile)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Check root too.
	if _, err := os.Stat(filepath.Join(dir, gasolineConfigFile)); err == nil {
		return dir
	}
	return ""
}

// loadGasolineConfig reads and parses .gasoline.json with defaults.
func loadGasolineConfig(path string) gasolineConfig {
	cfg := gasolineConfig{
		CodeStandards: defaultCodeStandardsFile,
		FileSizeLimit: defaultFileSizeLimit,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	if cfg.CodeStandards == "" {
		cfg.CodeStandards = defaultCodeStandardsFile
	}
	if cfg.FileSizeLimit <= 0 {
		cfg.FileSizeLimit = defaultFileSizeLimit
	}
	return cfg
}

// readHeadLines reads up to maxLines from a file.
func readHeadLines(path string, maxLines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n"), nil
}

// countLines counts newlines in a file.
func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	count := strings.Count(string(data), "\n")
	// If the file doesn't end with a newline, add 1.
	if data[len(data)-1] != '\n' {
		count++
	}
	return count, nil
}
