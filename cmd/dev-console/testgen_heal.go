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
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(projectDir, path)
	}

	// Clean and validate
	cleanPath := filepath.Clean(fullPath)
	cleanProject := filepath.Clean(projectDir)

	// Ensure resolved path is within project directory
	if !strings.HasPrefix(cleanPath, cleanProject) {
		return fmt.Errorf(ErrPathNotAllowed + ": path escapes project directory")
	}

	return nil
}

// validateSelector ensures selector is safe for DOM query
func validateSelector(selector string) error {
	// Max length
	if len(selector) > 1000 {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector exceeds 1000 characters")
	}

	// Empty selector
	if selector == "" {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector is empty")
	}

	// Disallow script injection patterns
	dangerous := []string{"javascript:", "<script", "onerror=", "onload="}
	lowerSelector := strings.ToLower(selector)
	for _, pattern := range dangerous {
		if strings.Contains(lowerSelector, pattern) {
			return fmt.Errorf(ErrSelectorInjection + ": selector contains dangerous pattern: " + pattern)
		}
	}

	// Basic CSS selector syntax check (must start with valid chars)
	// Valid selectors start with: letter, #, ., [, or *
	if len(selector) > 0 {
		firstChar := rune(selector[0])
		isValid := (firstChar >= 'a' && firstChar <= 'z') ||
			(firstChar >= 'A' && firstChar <= 'Z') ||
			firstChar == '#' ||
			firstChar == '.' ||
			firstChar == '[' ||
			firstChar == '*'
		if !isValid {
			return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector must start with valid CSS selector character")
		}
	}

	return nil
}

// handleGenerateTestHeal handles the test_heal mode of the generate tool
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

	// Validate required parameters
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
		}
	}

	// Validate action value
	validActions := map[string]bool{"analyze": true, "repair": true, "batch": true}
	if !validActions[params.Action] {
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
		}
	}

	// Get project directory
	projectDir, _ := os.Getwd()

	// Dispatch based on action
	var result any
	switch params.Action {
	case "analyze":
		if params.TestFile == "" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'test_file' is missing for analyze action",
					"Add the 'test_file' parameter and call again",
					withParam("test_file"),
				),
			}
		}
		selectors, err := h.analyzeTestFile(params, projectDir)
		if err != nil {
			if strings.Contains(err.Error(), ErrTestFileNotFound) {
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
			if strings.Contains(err.Error(), ErrPathNotAllowed) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrPathNotAllowed,
						err.Error(),
						"Use a path within the project directory",
					),
				}
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to analyze test file: " + err.Error()),
			}
		}
		result = map[string]any{
			"broken_selectors": selectors,
			"count":            len(selectors),
		}

	case "repair":
		if len(params.BrokenSelectors) == 0 {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'broken_selectors' is missing for repair action",
					"Add the 'broken_selectors' parameter and call again",
					withParam("broken_selectors"),
				),
			}
		}

		healResult, err := h.repairSelectors(params, projectDir)
		if err != nil {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to repair selectors: " + err.Error()),
			}
		}
		result = healResult

	case "batch":
		if params.TestDir == "" {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrMissingParam,
					"Required parameter 'test_dir' is missing for batch action",
					"Add the 'test_dir' parameter and call again",
					withParam("test_dir"),
				),
			}
		}

		batchResult, err := h.healTestBatch(params, projectDir)
		if err != nil {
			if strings.Contains(err.Error(), ErrPathNotAllowed) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrPathNotAllowed,
						err.Error(),
						"Use a path within the project directory",
					),
				}
			}
			if strings.Contains(err.Error(), ErrBatchTooLarge) {
				return JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: mcpStructuredError(
						ErrBatchTooLarge,
						err.Error(),
						"Reduce the number or size of test files",
					),
				}
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpErrorResponse("Failed to heal test batch: " + err.Error()),
			}
		}
		result = batchResult
	}

	// Format response
	var summary string
	if params.Action == "analyze" {
		var count int
		if m, ok := result.(map[string]any); ok {
			count, _ = m["count"].(int)
		}
		summary = fmt.Sprintf("Found %d selectors in %s", count, params.TestFile)
	} else if params.Action == "repair" {
		healResult := result.(*HealResult)
		summary = fmt.Sprintf("Healed %d/%d selectors (%d unhealed, %d auto-applied)",
			len(healResult.Healed),
			healResult.Summary.TotalBroken,
			healResult.Summary.Unhealed,
			healResult.Summary.HealedAuto)
	} else if params.Action == "batch" {
		batchResult := result.(*BatchHealResult)
		summary = fmt.Sprintf("Healed %d/%d selectors across %d files (%d files skipped, %d selectors unhealed)",
			batchResult.TotalHealed,
			batchResult.TotalSelectors,
			batchResult.FilesProcessed,
			batchResult.FilesSkipped,
			batchResult.TotalUnhealed)
	}

	data := map[string]any{
		"result": result,
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  mcpJSONResponse(summary, data),
	}

	return appendWarningsToResponse(resp, warnings)
}

// analyzeTestFile reads a test file and extracts all selectors used in it
func (h *ToolHandler) analyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	// Validate path
	if err := validateTestFilePath(req.TestFile, projectDir); err != nil {
		return nil, err
	}

	// Construct full path
	var fullPath string
	if filepath.IsAbs(req.TestFile) {
		fullPath = req.TestFile
	} else {
		fullPath = filepath.Join(projectDir, req.TestFile)
	}

	// Read file
	content, err := os.ReadFile(fullPath) // #nosec G304 -- path validated above
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrTestFileNotFound + ": " + req.TestFile)
		}
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	// Extract selectors using regex patterns
	selectors := extractSelectorsFromTestFile(string(content))

	return selectors, nil
}

// extractSelectorsFromTestFile uses regex to find selector patterns in test code
func extractSelectorsFromTestFile(content string) []string {
	var selectors []string
	seen := make(map[string]bool)

	// Patterns to match common Playwright selector methods
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
			if len(match) > 1 {
				selector := match[1]
				if !seen[selector] {
					selectors = append(selectors, selector)
					seen[selector] = true
				}
			}
		}
	}

	return selectors
}

// repairSelectors attempts to heal broken selectors
func (h *ToolHandler) repairSelectors(req TestHealRequest, projectDir string) (*HealResult, error) {
	result := &HealResult{
		Healed:   make([]HealedSelector, 0),
		Unhealed: make([]string, 0),
		Summary: HealSummary{
			TotalBroken: len(req.BrokenSelectors),
		},
	}

	// Validate all selectors first
	for _, selector := range req.BrokenSelectors {
		if err := validateSelector(selector); err != nil {
			// If selector is invalid, mark as unhealed
			result.Unhealed = append(result.Unhealed, selector)
			continue
		}

		// Try to heal the selector
		healed, err := h.healSelector(selector)
		if err != nil || healed == nil {
			result.Unhealed = append(result.Unhealed, selector)
			continue
		}

		result.Healed = append(result.Healed, *healed)

		// Track auto-apply if confidence is high enough
		if req.AutoApply && healed.Confidence >= 0.9 {
			result.Summary.HealedAuto++
		} else if healed.Confidence >= 0.5 {
			result.Summary.HealedManual++
		}
	}

	result.Summary.Unhealed = len(result.Unhealed)

	return result, nil
}

// healSelector attempts to find a replacement for a broken selector
// For Phase 2, this is a simplified implementation that uses basic heuristics
// In a full implementation, this would query the DOM via the extension
func (h *ToolHandler) healSelector(oldSelector string) (*HealedSelector, error) {
	// For now, return a mock implementation
	// In production, this would:
	// 1. Query the DOM for similar elements
	// 2. Score candidates using healing strategies
	// 3. Return the best match

	// Simplified heuristic: try to infer a better selector
	var newSelector string
	var confidence float64
	var strategy string

	// If selector looks like an ID, try to suggest data-testid
	if strings.HasPrefix(oldSelector, "#") {
		idValue := strings.TrimPrefix(oldSelector, "#")
		newSelector = fmt.Sprintf("[data-testid='%s']", idValue)
		confidence = 0.6
		strategy = "testid_match"
	} else if strings.HasPrefix(oldSelector, ".") {
		// Class selector - low confidence
		newSelector = oldSelector
		confidence = 0.3
		strategy = "structural_match"
	} else {
		// Unknown selector type
		return nil, fmt.Errorf("cannot heal selector: %s", oldSelector)
	}

	return &HealedSelector{
		OldSelector: oldSelector,
		NewSelector: newSelector,
		Confidence:  confidence,
		Strategy:    strategy,
		LineNumber:  0, // Would need file parsing to determine
	}, nil
}

// healTestBatch processes multiple test files in a directory
func (h *ToolHandler) healTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	// 1. Validate test_dir parameter
	if err := validateTestFilePath(req.TestDir, projectDir); err != nil {
		return nil, err
	}

	// Construct full path
	var fullPath string
	if filepath.IsAbs(req.TestDir) {
		fullPath = req.TestDir
	} else {
		fullPath = filepath.Join(projectDir, req.TestDir)
	}

	// Check if directory exists
	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(ErrTestFileNotFound + ": directory not found: " + req.TestDir)
		}
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}

	if !dirInfo.IsDir() {
		return nil, fmt.Errorf(ErrInvalidParam + ": test_dir must be a directory")
	}

	// 2. Scan directory for test files
	testFiles, err := findTestFiles(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Initialize result
	result := &BatchHealResult{
		FilesProcessed: 0,
		FilesSkipped:   0,
		TotalSelectors: 0,
		TotalHealed:    0,
		TotalUnhealed:  0,
		FileResults:    make([]FileHealResult, 0),
		Warnings:       make([]string, 0),
	}

	// 3. Enforce batch limits
	if len(testFiles) > MaxFilesPerBatch {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Batch limited to %d files (found %d)", MaxFilesPerBatch, len(testFiles)))
		testFiles = testFiles[:MaxFilesPerBatch]
	}

	var totalBatchSize int64

	// 4. For each file: extract selectors, attempt healing, track results
	for _, filePath := range testFiles {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   "Failed to read file info",
			})
			result.FilesSkipped++
			continue
		}

		// Check file size limit
		if fileInfo.Size() > MaxFileSizeBytes {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   fmt.Sprintf("File size exceeds %dKB limit", MaxFileSizeBytes/1024),
			})
			result.FilesSkipped++
			continue
		}

		// Check total batch size limit
		if totalBatchSize+fileInfo.Size() > MaxTotalBatchSize {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   fmt.Sprintf("Total batch size would exceed %dMB limit", MaxTotalBatchSize/(1024*1024)),
			})
			result.FilesSkipped++
			continue
		}

		totalBatchSize += fileInfo.Size()

		// Extract selectors from file
		fileReq := TestHealRequest{
			Action:   "analyze",
			TestFile: filePath,
		}
		selectors, err := h.analyzeTestFile(fileReq, projectDir)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: 0,
				Skipped:  true,
				Reason:   "Failed to analyze file: " + err.Error(),
			})
			result.FilesSkipped++
			continue
		}

		// Limit selectors per file
		if len(selectors) > MaxSelectorsPerFile {
			result.Warnings = append(result.Warnings, fmt.Sprintf("File %s has %d selectors, limited to %d", filepath.Base(filePath), len(selectors), MaxSelectorsPerFile))
			selectors = selectors[:MaxSelectorsPerFile]
		}

		// Attempt to heal selectors
		repairReq := TestHealRequest{
			Action:          "repair",
			BrokenSelectors: selectors,
			AutoApply:       false, // Never auto-apply in batch mode
		}

		healResult, err := h.repairSelectors(repairReq, projectDir)
		if err != nil {
			result.FileResults = append(result.FileResults, FileHealResult{
				FilePath: filePath,
				Healed:   0,
				Unhealed: len(selectors),
				Skipped:  true,
				Reason:   "Failed to repair selectors: " + err.Error(),
			})
			result.FilesSkipped++
			continue
		}

		// Track results for this file
		fileResult := FileHealResult{
			FilePath: filePath,
			Healed:   len(healResult.Healed),
			Unhealed: len(healResult.Unhealed),
			Skipped:  false,
		}
		result.FileResults = append(result.FileResults, fileResult)

		// Update totals
		result.FilesProcessed++
		result.TotalSelectors += len(selectors)
		result.TotalHealed += len(healResult.Healed)
		result.TotalUnhealed += len(healResult.Unhealed)
	}

	return result, nil
}

// findTestFiles recursively finds test files in a directory
func findTestFiles(dir string) ([]string, error) {
	var testFiles []string
	testPatterns := []string{".spec.ts", ".test.ts", ".spec.js", ".test.js"}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip node_modules and other common non-test directories
			if info.Name() == "node_modules" || info.Name() == ".git" || info.Name() == "dist" || info.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches test patterns
		for _, pattern := range testPatterns {
			if strings.HasSuffix(path, pattern) {
				testFiles = append(testFiles, path)
				break
			}
		}

		return nil
	})

	return testFiles, err
}
