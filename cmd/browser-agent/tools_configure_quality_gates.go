// tools_configure_quality_gates.go — Handler for configure(what="setup_quality_gates").
// Scaffolds .gasoline.json and gasoline-code-standards.md for code quality gate enforcement.
// Docs: docs/features/feature/quality-gates/index.md

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const (
	gasolineConfigFile         = ".gasoline.json"
	defaultCodeStandardsFile   = "gasoline-code-standards.md"
	defaultFileSizeLimit       = 800
	defaultDuplicateThreshold  = 8
)

// gasolineConfig is the structure of .gasoline.json.
type gasolineConfig struct {
	CodeStandards      string `json:"code_standards"`
	FileSizeLimit      int    `json:"file_size_limit"`
	DuplicateThreshold int    `json:"duplicate_threshold"`
}

// toolConfigureSetupQualityGates handles configure(what="setup_quality_gates").
// Creates .gasoline.json and gasoline-code-standards.md in the target directory.
func (h *ToolHandler) toolConfigureSetupQualityGates(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		TargetDir string `json:"target_dir"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &params); stop {
			return resp
		}
	}

	// Resolve the project root directory.
	projectDir := h.server.GetActiveCodebase()
	if projectDir == "" {
		return fail(req, ErrNotInitialized,
			"No active codebase set. Cannot determine project directory.",
			"Set active_codebase via configure(what='store', key='active_codebase', data='<path>') first")
	}

	targetDir := projectDir
	if params.TargetDir != "" {
		targetDir = params.TargetDir
	}

	// Security: target_dir must be within the project directory.
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return fail(req, ErrInvalidParam, "Invalid target_dir: "+err.Error(), "Provide a valid directory path", withParam("target_dir"))
	}
	absProject, _ := filepath.Abs(projectDir)
	if !strings.HasPrefix(absTarget+string(filepath.Separator), absProject+string(filepath.Separator)) && absTarget != absProject {
		return fail(req, ErrPathNotAllowed,
			"target_dir is outside the project directory",
			"Use a path within "+absProject, withParam("target_dir"))
	}

	// Ensure target directory exists.
	if stat, err := os.Stat(absTarget); err != nil || !stat.IsDir() {
		return fail(req, ErrInvalidParam,
			"target_dir does not exist or is not a directory: "+absTarget,
			"Provide an existing directory path", withParam("target_dir"))
	}

	configPath := filepath.Join(absTarget, gasolineConfigFile)
	configExisted := false
	standardsCreated := false
	standardsPath := ""

	// Write .gasoline.json if it doesn't exist.
	if _, err := os.Stat(configPath); err == nil {
		configExisted = true
	} else {
		cfg := gasolineConfig{
			CodeStandards:      defaultCodeStandardsFile,
			FileSizeLimit:      defaultFileSizeLimit,
			DuplicateThreshold: defaultDuplicateThreshold,
		}
		cfgJSON, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fail(req, ErrInternal, "Failed to marshal config: "+err.Error(), "Internal error — do not retry")
		}
		cfgJSON = append(cfgJSON, '\n')
		if err := os.WriteFile(configPath, cfgJSON, 0644); err != nil {
			return fail(req, ErrInternal, "Failed to write "+gasolineConfigFile+": "+err.Error(), "Check file system permissions")
		}
	}

	// Determine code_standards path from config.
	codeStandardsRef := defaultCodeStandardsFile
	if configExisted {
		existingCfg, err := os.ReadFile(configPath)
		if err == nil {
			var parsed gasolineConfig
			if json.Unmarshal(existingCfg, &parsed) == nil && parsed.CodeStandards != "" {
				codeStandardsRef = parsed.CodeStandards
			}
		}
	}

	// Only create the standards file if it's the default name (not a custom path).
	if codeStandardsRef == defaultCodeStandardsFile {
		standardsPath = filepath.Join(absTarget, defaultCodeStandardsFile)
		if _, err := os.Stat(standardsPath); err != nil {
			if err := os.WriteFile(standardsPath, []byte(defaultCodeStandardsContent), 0644); err != nil {
				return fail(req, ErrInternal, "Failed to write "+defaultCodeStandardsFile+": "+err.Error(), "Check file system permissions")
			}
			standardsCreated = true
		}
	}

	// Build response.
	defaults := map[string]any{
		"code_standards":      codeStandardsRef,
		"file_size_limit":     defaultFileSizeLimit,
		"duplicate_threshold": defaultDuplicateThreshold,
	}

	suggestions := buildQualityGateSuggestions(configExisted, standardsCreated, codeStandardsRef)

	responseData := map[string]any{
		"config_path":    configPath,
		"config_existed": configExisted,
		"defaults":       defaults,
		"suggestions":    suggestions,
	}
	if standardsPath != "" {
		responseData["standards_path"] = standardsPath
		responseData["standards_created"] = standardsCreated
	}

	summary := "Quality gates configured"
	if configExisted {
		summary = "Quality gates already configured — existing config preserved"
	}

	return succeed(req, summary, responseData)
}

// buildQualityGateSuggestions returns contextual suggestions based on setup state.
func buildQualityGateSuggestions(configExisted, standardsCreated bool, codeStandardsRef string) []string {
	suggestions := []string{}

	if !configExisted {
		suggestions = append(suggestions,
			"Edit .gasoline.json to customize quality gate thresholds",
			"Set code_standards to your existing conventions doc if one exists",
		)
	}
	if standardsCreated {
		suggestions = append(suggestions,
			"Edit gasoline-code-standards.md to add your project's coding patterns and conventions",
		)
	}
	if codeStandardsRef != defaultCodeStandardsFile {
		suggestions = append(suggestions,
			"Your config points to "+codeStandardsRef+" — ensure this file exists",
		)
	}

	suggestions = append(suggestions,
		"Configure a Claude Code prompt hook to use Haiku for automatic code review on every edit",
	)

	return suggestions
}

// defaultCodeStandardsContent is the starter content for gasoline-code-standards.md.
const defaultCodeStandardsContent = `# Code Standards

> This file defines your project's coding standards. Gasoline's quality gates use this
> file to check edits for pattern violations. Write rules the way you would explain them
> to a new team member — plain markdown, no special format required.

## General Rules

- Keep files under 800 lines. Refactor if larger.
- Extract repeated logic into helpers — do not inline.
- All JSON fields use snake_case.
- Prefer named functions over anonymous closures for readability.

## Patterns

### Error Handling
- Always handle errors explicitly. Do not ignore return values.
- Use structured error messages: "{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}"

### Code Organization
- One concept per file. If a file has multiple unrelated concerns, split it.
- Group related functions together. Export only what is needed.
- Keep public interfaces minimal and explicit.

### Naming
- Use descriptive names. Avoid abbreviations except well-known ones (e.g., URL, ID, HTTP).
- Function names should describe what they do, not how they do it.

### Testing
- Write tests first (TDD) when adding new functions.
- Use deterministic tests — avoid sleep-based timing.
- Each bug fix should include a regression test.

## Add Your Patterns Below

<!-- Add project-specific patterns here. Examples:

### Command Pattern
Functions with 3+ sequential phases (setup, execute, cleanup) should implement
the Command interface. See internal/cmd/base.go for the canonical implementation.

### Validation Guards
Request validation should use validateAndRespond() from internal/util/guards.go.
Do not write inline if/else validation chains.

### Response Builder
All API responses must use buildResponse() helper from internal/util/response.go.
Never construct response JSON inline.

-->
`
