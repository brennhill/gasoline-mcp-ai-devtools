// Purpose: Owns testgen_heal.go runtime behavior and integration logic.
// Docs: docs/features/feature/test-generation/index.md

// testgen_heal.go — Test healing: selector analysis, repair, and batch processing.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// validateTestFilePath ensures path is within project directory
// Uses the same pattern as validatePathInDir from ai_persistence.go
func validateTestFilePath(path string, projectDir string) error {
	if path == "" {
		return fmt.Errorf("test file path is required")
	}

	// Reject path traversal sequences
	if strings.Contains(path, "..") {
		return fmt.Errorf(ErrPathNotAllowed + ": path contains '..'")
	}

	// Construct full path
	fullPath := resolveTestPath(path, projectDir)

	// Clean and validate
	cleanPath := filepath.Clean(fullPath)
	cleanProject := filepath.Clean(projectDir)

	// Ensure resolved path is within project directory
	if !strings.HasPrefix(cleanPath, cleanProject) {
		return fmt.Errorf(ErrPathNotAllowed + ": path escapes project directory")
	}

	return nil
}

func resolveTestPath(path, projectDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectDir, path)
}

func containsDangerousPattern(selector string) (string, bool) {
	dangerous := []string{"javascript:", "<script", "onerror=", "onload="}
	lowerSelector := strings.ToLower(selector)
	for _, pattern := range dangerous {
		if strings.Contains(lowerSelector, pattern) {
			return pattern, true
		}
	}
	return "", false
}

// validSelectorStartChars contains characters that may begin a CSS selector.
const validSelectorStartChars = "#.[*"

func isValidSelectorStart(ch byte) bool {
	c := rune(ch)
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	return strings.ContainsRune(validSelectorStartChars, c)
}

func validateSelector(selector string) error {
	if len(selector) > 1000 {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector exceeds 1000 characters")
	}
	if selector == "" {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector is empty")
	}
	if pattern, found := containsDangerousPattern(selector); found {
		return fmt.Errorf("%s: selector contains dangerous pattern: %s", ErrSelectorInjection, pattern)
	}
	if !isValidSelectorStart(selector[0]) {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector must start with valid CSS selector character")
	}
	return nil
}

func (h *ToolHandler) handleGenerateTestHeal(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params TestHealRequest

	warnings, err := unmarshalWithWarnings(args, &params)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			),
		}
	}

	if errResp, ok := validateHealParams(req, params); ok {
		return errResp
	}

	projectDir, _ := os.Getwd()

	result, errResp, ok := h.dispatchHealAction(req, params, projectDir)
	if ok {
		return errResp
	}

	summary := formatHealSummary(params, result)
	data := map[string]any{"result": result}
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, data),
	}
	return appendWarningsToResponse(resp, warnings)
}

var validHealActions = map[string]bool{"analyze": true, "repair": true, "batch": true}

func validateHealParams(req JSONRPCRequest, params TestHealRequest) (JSONRPCResponse, bool) {
	if params.Action == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'action' is missing",
				"Add the 'action' parameter and call again",
				withParam("action"),
				withHint("Valid values: analyze, repair"),
			),
		}, true
	}
	if !validHealActions[params.Action] {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Invalid action value: "+params.Action,
				"Use a valid action value",
				withParam("action"),
				withHint("Valid values: analyze, repair, batch"),
			),
		}, true
	}
	return JSONRPCResponse{}, false
}

// dispatchHealAction runs the appropriate action handler and returns the result.
// If an error response is needed, the bool return is true and the JSONRPCResponse is set.
func (h *ToolHandler) dispatchHealAction(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	switch params.Action {
	case "analyze":
		return h.handleHealAnalyze(req, params, projectDir)
	case "repair":
		return h.handleHealRepair(req, params, projectDir)
	case "batch":
		return h.handleHealBatch(req, params, projectDir)
	}
	return nil, JSONRPCResponse{}, false
}

func (h *ToolHandler) handleHealAnalyze(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if params.TestFile == "" {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'test_file' is missing for analyze action",
				"Add the 'test_file' parameter and call again",
				withParam("test_file"),
			),
		}, true
	}
	selectors, err := h.analyzeTestFile(params, projectDir)
	if err != nil {
		return nil, mapAnalyzeError(req, params, err), true
	}
	result := map[string]any{
		"broken_selectors": selectors,
		"count":            len(selectors),
	}
	return result, JSONRPCResponse{}, false
}

func mapAnalyzeError(req JSONRPCRequest, params TestHealRequest, err error) JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, ErrTestFileNotFound) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrTestFileNotFound,
				"Test file not found: "+params.TestFile,
				"Check the file path and try again",
			),
		}
	}
	if strings.Contains(errMsg, ErrPathNotAllowed) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrPathNotAllowed,
				errMsg,
				"Use a path within the project directory",
			),
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpStructuredError(ErrInternal, "Failed to analyze test file: "+errMsg, "Check that the test file path is valid and readable"),
	}
}

func (h *ToolHandler) handleHealRepair(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if len(params.BrokenSelectors) == 0 {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'broken_selectors' is missing for repair action",
				"Add the 'broken_selectors' parameter and call again",
				withParam("broken_selectors"),
			),
		}, true
	}
	healResult, err := h.repairSelectors(params, projectDir)
	if err != nil {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInternal, "Failed to repair selectors: "+err.Error(), "Check the broken_selectors input and retry"),
		}, true
	}
	return healResult, JSONRPCResponse{}, false
}

func (h *ToolHandler) handleHealBatch(req JSONRPCRequest, params TestHealRequest, projectDir string) (any, JSONRPCResponse, bool) {
	if params.TestDir == "" {
		return nil, JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'test_dir' is missing for batch action",
				"Add the 'test_dir' parameter and call again",
				withParam("test_dir"),
			),
		}, true
	}
	batchResult, err := h.healTestBatch(params, projectDir)
	if err != nil {
		return nil, mapBatchError(req, err), true
	}
	return batchResult, JSONRPCResponse{}, false
}

func mapBatchError(req JSONRPCRequest, err error) JSONRPCResponse {
	errMsg := err.Error()
	if strings.Contains(errMsg, ErrPathNotAllowed) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrPathNotAllowed,
				errMsg,
				"Use a path within the project directory",
			),
		}
	}
	if strings.Contains(errMsg, ErrBatchTooLarge) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrBatchTooLarge,
				errMsg,
				"Reduce the number or size of test files",
			),
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpStructuredError(ErrInternal, "Failed to heal test batch: "+errMsg, "Check the test_dir path and file permissions, then retry"),
	}
}

func formatHealSummary(params TestHealRequest, result any) string {
	switch params.Action {
	case "analyze":
		var count int
		if m, ok := result.(map[string]any); ok {
			count, _ = m["count"].(int)
		}
		return fmt.Sprintf("Found %d selectors in %s", count, params.TestFile)
	case "repair":
		hr := result.(*HealResult)
		return fmt.Sprintf("Healed %d/%d selectors (%d unhealed, %d auto-applied)",
			len(hr.Healed), hr.Summary.TotalBroken, hr.Summary.Unhealed, hr.Summary.HealedAuto)
	case "batch":
		br := result.(*BatchHealResult)
		return fmt.Sprintf("Healed %d/%d selectors across %d files (%d files skipped, %d selectors unhealed)",
			br.TotalHealed, br.TotalSelectors, br.FilesProcessed, br.FilesSkipped, br.TotalUnhealed)
	}
	return ""
}

func (h *ToolHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	if err := validateTestFilePath(req.TestFile, projectDir); err != nil {
		return nil, err
	}

	fullPath := resolveTestPath(req.TestFile, projectDir)

	content, err := os.ReadFile(fullPath) // #nosec G304 -- path validated above // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads user test file for healing
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %s", ErrTestFileNotFound, req.TestFile)
		}
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	selectors := extractSelectorsFromTestFile(string(content))
	return selectors, nil
}

func extractSelectorsFromTestFile(content string) []string {
	var selectors []string
	seen := make(map[string]bool)

	patterns := []string{
		`getByTestId\(['"]([^'"]+)['"]\)`,
		`locator\(['"]([^'"]+)['"]\)`,
		`getByRole\(['"]([^'"]+)['"]\)`,
		`getByLabel\(['"]([^'"]+)['"]\)`,
		`getByText\(['"]([^'"]+)['"]\)`,
		`querySelector\(['"]([^'"]+)['"]\)`,
		`querySelectorAll\(['"]([^'"]+)['"]\)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 && !seen[match[1]] {
				selectors = append(selectors, match[1])
				seen[match[1]] = true
			}
		}
	}

	return selectors
}

func (h *ToolHandler) repairSelectors(req TestHealRequest, projectDir string) (*HealResult, error) {
	result := &HealResult{
		Healed:   make([]HealedSelector, 0),
		Unhealed: make([]string, 0),
		Summary: HealSummary{
			TotalBroken: len(req.BrokenSelectors),
		},
	}

	for _, selector := range req.BrokenSelectors {
		h.repairOneSelector(selector, req.AutoApply, result)
	}

	result.Summary.Unhealed = len(result.Unhealed)
	return result, nil
}

func (h *ToolHandler) repairOneSelector(selector string, autoApply bool, result *HealResult) {
	if err := validateSelector(selector); err != nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	healed, err := h.healSelector(selector)
	if err != nil || healed == nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	result.Healed = append(result.Healed, *healed)
	classifyHealedSelector(healed, autoApply, result)
}

func classifyHealedSelector(healed *HealedSelector, autoApply bool, result *HealResult) {
	if autoApply && healed.Confidence >= 0.9 {
		result.Summary.HealedAuto++
	} else if healed.Confidence >= 0.5 {
		result.Summary.HealedManual++
	}
}

// healSelector attempts to find a replacement for a broken selector
// For Phase 2, this is a simplified implementation that uses basic heuristics
// In a full implementation, this would query the DOM via the extension
func (h *ToolHandler) healSelector(oldSelector string) (*HealedSelector, error) {
	var newSelector string
	var confidence float64
	var strategy string

	if strings.HasPrefix(oldSelector, "#") {
		idValue := strings.TrimPrefix(oldSelector, "#")
		newSelector = fmt.Sprintf("[data-testid='%s']", idValue)
		confidence = 0.6
		strategy = "testid_match"
	} else if strings.HasPrefix(oldSelector, ".") {
		newSelector = oldSelector
		confidence = 0.3
		strategy = "structural_match"
	} else {
		return nil, fmt.Errorf("cannot heal selector: %s", oldSelector)
	}

	return &HealedSelector{
		OldSelector: oldSelector,
		NewSelector: newSelector,
		Confidence:  confidence,
		Strategy:    strategy,
		LineNumber:  0,
	}, nil
}

func (h *ToolHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	fullPath, err := validateBatchDir(req.TestDir, projectDir)
	if err != nil {
		return nil, err
	}

	testFiles, err := findTestFiles(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	result := &BatchHealResult{
		FileResults: make([]FileHealResult, 0),
		Warnings:    make([]string, 0),
	}

	if len(testFiles) > MaxFilesPerBatch {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Batch limited to %d files (found %d)", MaxFilesPerBatch, len(testFiles)))
		testFiles = testFiles[:MaxFilesPerBatch]
	}

	var totalBatchSize int64
	for _, filePath := range testFiles {
		_, totalBatchSize = h.processTestFile(filePath, projectDir, totalBatchSize, result)
	}

	return result, nil
}

func validateBatchDir(testDir, projectDir string) (string, error) {
	if err := validateTestFilePath(testDir, projectDir); err != nil {
		return "", err
	}

	fullPath := resolveTestPath(testDir, projectDir)

	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: directory not found: %s", ErrTestFileNotFound, testDir)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf(ErrInvalidParam + ": test_dir must be a directory")
	}

	return fullPath, nil
}

func skipFileWithReason(filePath, reason string, result *BatchHealResult) {
	result.FileResults = append(result.FileResults, FileHealResult{
		FilePath: filePath,
		Skipped:  true,
		Reason:   reason,
	})
	result.FilesSkipped++
}

func checkBatchFileSize(fileSize, totalBatchSize int64) (string, bool) {
	if fileSize > MaxFileSizeBytes {
		return fmt.Sprintf("File size exceeds %dKB limit", MaxFileSizeBytes/1024), true
	}
	if totalBatchSize+fileSize > MaxTotalBatchSize {
		return fmt.Sprintf("Total batch size would exceed %dMB limit", MaxTotalBatchSize/(1024*1024)), true
	}
	return "", false
}

// processTestFile handles a single file within a batch heal operation.
// Returns true if the file was skipped, plus the updated totalBatchSize.
func (h *ToolHandler) processTestFile(filePath, projectDir string, totalBatchSize int64, result *BatchHealResult) (bool, int64) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		skipFileWithReason(filePath, "Failed to read file info", result)
		return true, totalBatchSize
	}

	if reason, skip := checkBatchFileSize(fileInfo.Size(), totalBatchSize); skip {
		skipFileWithReason(filePath, reason, result)
		return true, totalBatchSize
	}
	totalBatchSize += fileInfo.Size()

	selectors, err := h.analyzeTestFile(TestHealRequest{Action: "analyze", TestFile: filePath}, projectDir)
	if err != nil {
		skipFileWithReason(filePath, "Failed to analyze file: "+err.Error(), result)
		return true, totalBatchSize
	}

	if len(selectors) > MaxSelectorsPerFile {
		result.Warnings = append(result.Warnings, fmt.Sprintf("File %s has %d selectors, limited to %d", filepath.Base(filePath), len(selectors), MaxSelectorsPerFile))
		selectors = selectors[:MaxSelectorsPerFile]
	}

	healResult, err := h.repairSelectors(TestHealRequest{Action: "repair", BrokenSelectors: selectors}, projectDir)
	if err != nil {
		skipFileWithReason(filePath, "Failed to repair selectors: "+err.Error(), result)
		return true, totalBatchSize
	}

	result.FileResults = append(result.FileResults, FileHealResult{
		FilePath: filePath,
		Healed:   len(healResult.Healed),
		Unhealed: len(healResult.Unhealed),
	})
	result.FilesProcessed++
	result.TotalSelectors += len(selectors)
	result.TotalHealed += len(healResult.Healed)
	result.TotalUnhealed += len(healResult.Unhealed)
	return false, totalBatchSize
}

var skipDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"dist":         true,
	"build":        true,
}

func isTestFile(path string) bool {
	testPatterns := []string{".spec.ts", ".test.ts", ".spec.js", ".test.js"}
	for _, pattern := range testPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}
	return false
}

func findTestFiles(dir string) ([]string, error) {
	var testFiles []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if isTestFile(path) {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles, err
}
