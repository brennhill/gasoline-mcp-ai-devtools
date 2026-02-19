// heal.go — Test healing: selector analysis, repair, and batch processing.
package testgen

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ValidateTestFilePath ensures path is within project directory.
func ValidateTestFilePath(path string, projectDir string) error {
	if path == "" {
		return fmt.Errorf("test file path is required")
	}

	if strings.Contains(path, "..") {
		return fmt.Errorf(ErrPathNotAllowed + ": path contains '..'")
	}

	fullPath := ResolveTestPath(path, projectDir)

	cleanPath := filepath.Clean(fullPath)
	cleanProject := filepath.Clean(projectDir)

	if !strings.HasPrefix(cleanPath, cleanProject) {
		return fmt.Errorf(ErrPathNotAllowed + ": path escapes project directory")
	}

	return nil
}

// ErrPathNotAllowed is the error code for disallowed paths.
const ErrPathNotAllowed = "path_not_allowed"

// ResolveTestPath resolves a relative path against the project directory.
func ResolveTestPath(path, projectDir string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectDir, path)
}

// ContainsDangerousPattern checks if a selector contains dangerous patterns.
func ContainsDangerousPattern(selector string) (string, bool) {
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

// ValidateSelector validates a CSS selector for safety and correctness.
func ValidateSelector(selector string) error {
	if len(selector) > 1000 {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector exceeds 1000 characters")
	}
	if selector == "" {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector is empty")
	}
	if pattern, found := ContainsDangerousPattern(selector); found {
		return fmt.Errorf("%s: selector contains dangerous pattern: %s", ErrSelectorInjection, pattern)
	}
	if !isValidSelectorStart(selector[0]) {
		return fmt.Errorf(ErrInvalidSelectorSyntax + ": selector must start with valid CSS selector character")
	}
	return nil
}

// ExtractSelectorsFromTestFile extracts selectors from test file content.
func ExtractSelectorsFromTestFile(content string) []string {
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

// AnalyzeTestFile reads a test file and extracts selectors from it.
func AnalyzeTestFile(req TestHealRequest, projectDir string) ([]string, error) {
	if err := ValidateTestFilePath(req.TestFile, projectDir); err != nil {
		return nil, err
	}

	fullPath := ResolveTestPath(req.TestFile, projectDir)

	content, err := os.ReadFile(fullPath) // #nosec G304 -- path validated above // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads user test file for healing
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %s", ErrTestFileNotFound, req.TestFile)
		}
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	selectors := ExtractSelectorsFromTestFile(string(content))
	return selectors, nil
}

// RepairSelectors attempts to repair a list of broken selectors.
func RepairSelectors(req TestHealRequest) (*HealResult, error) {
	result := &HealResult{
		Healed:   make([]HealedSelector, 0),
		Unhealed: make([]string, 0),
		Summary: HealSummary{
			TotalBroken: len(req.BrokenSelectors),
		},
	}

	for _, selector := range req.BrokenSelectors {
		repairOneSelector(selector, req.AutoApply, result)
	}

	result.Summary.Unhealed = len(result.Unhealed)
	return result, nil
}

func repairOneSelector(selector string, autoApply bool, result *HealResult) {
	if err := ValidateSelector(selector); err != nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	healed, err := HealSelector(selector)
	if err != nil || healed == nil {
		result.Unhealed = append(result.Unhealed, selector)
		return
	}

	result.Healed = append(result.Healed, *healed)
	ClassifyHealedSelector(healed, autoApply, result)
}

// ClassifyHealedSelector categorizes a healed selector by confidence.
func ClassifyHealedSelector(healed *HealedSelector, autoApply bool, result *HealResult) {
	if autoApply && healed.Confidence >= 0.9 {
		result.Summary.HealedAuto++
	} else if healed.Confidence >= 0.5 {
		result.Summary.HealedManual++
	}
}

// HealSelector attempts to find a replacement for a broken selector.
func HealSelector(oldSelector string) (*HealedSelector, error) {
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

// HealTestBatch heals selectors across a batch of test files in a directory.
func HealTestBatch(req TestHealRequest, projectDir string) (*BatchHealResult, error) {
	fullPath, err := ValidateBatchDir(req.TestDir, projectDir)
	if err != nil {
		return nil, err
	}

	testFiles, err := FindTestFiles(fullPath)
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
		_, totalBatchSize = processTestFile(filePath, projectDir, totalBatchSize, result)
	}

	return result, nil
}

// ValidateBatchDir validates a directory path for batch healing.
func ValidateBatchDir(testDir, projectDir string) (string, error) {
	if err := ValidateTestFilePath(testDir, projectDir); err != nil {
		return "", err
	}

	fullPath := ResolveTestPath(testDir, projectDir)

	dirInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: directory not found: %s", ErrTestFileNotFound, testDir)
		}
		return "", fmt.Errorf("failed to access directory: %w", err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("invalid_param: test_dir must be a directory")
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

// CheckBatchFileSize validates individual and total batch file sizes.
func CheckBatchFileSize(fileSize, totalBatchSize int64) (string, bool) {
	if fileSize > MaxFileSizeBytes {
		return fmt.Sprintf("File size exceeds %dKB limit", MaxFileSizeBytes/1024), true
	}
	if totalBatchSize+fileSize > MaxTotalBatchSize {
		return fmt.Sprintf("Total batch size would exceed %dMB limit", MaxTotalBatchSize/(1024*1024)), true
	}
	return "", false
}

func processTestFile(filePath, projectDir string, totalBatchSize int64, result *BatchHealResult) (bool, int64) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		skipFileWithReason(filePath, "Failed to read file info", result)
		return true, totalBatchSize
	}

	if reason, skip := CheckBatchFileSize(fileInfo.Size(), totalBatchSize); skip {
		skipFileWithReason(filePath, reason, result)
		return true, totalBatchSize
	}
	totalBatchSize += fileInfo.Size()

	selectors, err := AnalyzeTestFile(TestHealRequest{Action: "analyze", TestFile: filePath}, projectDir)
	if err != nil {
		skipFileWithReason(filePath, "Failed to analyze file: "+err.Error(), result)
		return true, totalBatchSize
	}

	if len(selectors) > MaxSelectorsPerFile {
		result.Warnings = append(result.Warnings, fmt.Sprintf("File %s has %d selectors, limited to %d", filepath.Base(filePath), len(selectors), MaxSelectorsPerFile))
		selectors = selectors[:MaxSelectorsPerFile]
	}

	healResult, err := RepairSelectors(TestHealRequest{Action: "repair", BrokenSelectors: selectors})
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

// IsTestFile checks if a file path matches test file patterns.
func IsTestFile(path string) bool {
	testPatterns := []string{".spec.ts", ".test.ts", ".spec.js", ".test.js"}
	for _, pattern := range testPatterns {
		if strings.HasSuffix(path, pattern) {
			return true
		}
	}
	return false
}

// FindTestFiles recursively finds test files in a directory, skipping common ignored dirs.
func FindTestFiles(dir string) ([]string, error) {
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
		if IsTestFile(path) {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles, err
}

// FormatHealSummary formats a human-readable summary of healing results.
func FormatHealSummary(params TestHealRequest, result any) string {
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
