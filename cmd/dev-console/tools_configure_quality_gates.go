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

	// Build the recommended hook config for the user's .claude/settings.json.
	hookConfig := buildQualityGateHookConfig(absTarget, codeStandardsRef)

	responseData := map[string]any{
		"config_path":    configPath,
		"config_existed": configExisted,
		"defaults":       defaults,
		"suggestions":    suggestions,
		"hook_config":    hookConfig,
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

// buildQualityGateHookConfig returns the recommended Claude Code hook configuration.
// Hook config uses Claude Code's naming conventions (PascalCase event names), so it's
// returned as a pre-formatted JSON string to avoid snake_case field validation.
func buildQualityGateHookConfig(projectDir, codeStandardsRef string) map[string]any {
	// The quality-gate-hook.sh script reads .gasoline.json, loads the standards doc,
	// runs file size checks and jscpd duplicate detection, then injects all findings
	// as additionalContext. It finds the project root by walking up from the changed file.
	commandHook := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": "` + filepath.Join(projectDir, "scripts", "quality-gate-hook.sh") + `",
            "timeout": 30
          }
        ]
      }
    ]
  }
}`

	promptHook := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "prompt",
            "prompt": "Review this code change against these project standards. Only flag clear violations, not style preferences. If the change looks good, respond {\"ok\": true}. If there are issues, respond {\"ok\": false, \"reason\": \"specific findings\"}.\n\nProject standards from ` + codeStandardsRef + ` apply.",
            "model": "haiku",
            "timeout": 30
          }
        ]
      }
    ]
  }
}`

	return map[string]any{
		"description":       "Add one of these hook configs to .claude/settings.json for automatic code review on every Edit/Write",
		"settings_path":     filepath.Join(projectDir, ".claude", "settings.json"),
		"command_hook_json": commandHook,
		"prompt_hook_json":  promptHook,
	}
}

// defaultCodeStandardsContent is the starter content for gasoline-code-standards.md.
// Rules are written to be actionable by an LLM reviewer (Haiku): each rule has a
// specific trigger condition and a concrete action. Vague rules generate false positives.
const defaultCodeStandardsContent = `# Code Standards

> Quality gate rules for automated code review. Each rule has a trigger (when to flag)
> and an action (what to do instead). Only flag clear violations — not style preferences.

## File Structure

- **Max 800 lines per file.** If a file exceeds this, it must be split.
- **One concept per file.** If a file has two unrelated concerns, split them.
- **No orphan code.** Dead code, commented-out blocks, and unused imports must be removed.

## Naming Conventions

Functions: verb-phrase describing the action — ` + "`" + `buildResponse` + "`" + `, ` + "`" + `parseArgs` + "`" + `, ` + "`" + `validateToken` + "`" + `.
Types/structs/classes: noun-phrase — ` + "`" + `ToolHandler` + "`" + `, ` + "`" + `QueryResult` + "`" + `, ` + "`" + `SessionStore` + "`" + `.
Booleans: predicate-phrase — ` + "`" + `isReady` + "`" + `, ` + "`" + `hasExpired` + "`" + `, ` + "`" + `canRetry` + "`" + `.
Constants: describe the value's purpose, not its content — ` + "`" + `maxRetries` + "`" + ` not ` + "`" + `three` + "`" + `.
Avoid abbreviations except well-known ones (URL, ID, HTTP, JSON, API).

## Error Handling

- Always handle errors explicitly. Never silently ignore error return values.
- Use structured error messages: "{OPERATION}: {ROOT_CAUSE}. {RECOVERY_ACTION}"
- Errors should be actionable — tell the caller what went wrong and how to fix it.

## Duplication & Reuse

- **3+ similar lines = extract a helper.** If you see the same logic repeated, it should be a function.
- **Before writing a new utility, check if one exists.** Search the codebase for similar function signatures.
- **Prefer composition over inheritance.** Small, focused functions composed together beat deep class hierarchies.

## Structural Patterns

- **3+ switch/case branches dispatching to similar logic** → extract to a handler map or strategy pattern.
- **3+ sequential phases (setup, execute, cleanup)** → use a builder or command pattern if one exists in the codebase.
- **Nested callbacks or deeply indented logic (4+ levels)** → extract inner blocks into named functions.
- **God functions (50+ lines doing multiple things)** → split into focused sub-functions.

## Testing

- New functions should have tests. Bug fixes must include a regression test.
- Use deterministic tests — no sleep-based timing, no flaky network calls.
- Test the contract (inputs/outputs), not the implementation details.

## Security

- Never log secrets, tokens, API keys, or credentials.
- Validate all external input at system boundaries (user input, API responses, file reads).
- Do not trust internal data structures to be valid — defensive checks at module boundaries.

## Add Your Project Patterns Below

<!-- Add project-specific patterns here. Be specific — vague rules cause false positives.

Good: "Request validation must use validateAndRespond() from internal/util/guards.go.
       Do not write inline if/else validation chains."

Bad:  "Use good patterns." (too vague, will flag everything)

-->
`
