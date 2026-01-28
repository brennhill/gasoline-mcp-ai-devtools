package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ============================================
// C-1: HAR Export Path Traversal via Symlink
// ============================================

func TestIsPathSafe_SymlinkTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping symlink test in short mode")
	}

	// Create a sensitive directory in the current working directory (NOT in temp)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sensitiveDir := filepath.Join(cwd, ".gasoline-test-sensitive")
	if err := os.Mkdir(sensitiveDir, 0700); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	defer os.RemoveAll(sensitiveDir)

	sensitiveFile := filepath.Join(sensitiveDir, "secret.txt")
	if err := os.WriteFile(sensitiveFile, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink in /tmp pointing to the sensitive directory outside /tmp
	symlinkDir := filepath.Join(os.TempDir(), "gasoline-test-symlink")
	// Clean up any existing symlink
	os.Remove(symlinkDir)
	if err := os.Symlink(sensitiveDir, symlinkDir); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(symlinkDir)

	// Attempt to write through the symlink
	// This should be blocked by a secure isPathSafe implementation
	attackPath := filepath.Join(symlinkDir, "secret.txt")

	// The vulnerable isPathSafe would allow this because symlinkDir starts with /tmp
	// But after resolving the symlink, it actually points to cwd (outside /tmp)
	if isPathSafe(attackPath) {
		// Resolve the symlink to see where it really points
		resolved, err := filepath.EvalSymlinks(attackPath)
		if err == nil {
			// Resolve both tmpDir and /tmp to handle macOS /tmp -> /private/tmp symlink
			resolvedTmpDir, _ := filepath.EvalSymlinks(os.TempDir())
			if !strings.HasPrefix(resolved, resolvedTmpDir) && !strings.HasPrefix(resolved, "/tmp") && !strings.HasPrefix(resolved, "/private/tmp") {
				t.Errorf("isPathSafe allowed symlink traversal: %s resolves to %s (outside temp)", attackPath, resolved)
			}
		}
	}
}

func TestIsPathSafe_LegitimateTemporaryFile(t *testing.T) {
	// Legitimate temporary file should still work
	tmpFile := filepath.Join(os.TempDir(), "gasoline-test-har.json")
	if !isPathSafe(tmpFile) {
		t.Errorf("isPathSafe rejected legitimate temporary file: %s", tmpFile)
	}
}

// ============================================
// C-2: ReDoS via User-Supplied Regex
// ============================================

func TestNoiseConfig_AddRules_RejectsComplexRegex(t *testing.T) {
	nc := NewNoiseConfig()

	tests := []struct {
		name        string
		pattern     string
		shouldError bool
		reason      string
	}{
		{
			name:        "Nested quantifiers (ReDoS risk)",
			pattern:     "(a+)+$",
			shouldError: true,
			reason:      "nested quantifiers",
		},
		{
			name:        "Multiple nested groups",
			pattern:     "((a+)+)+$",
			shouldError: true,
			reason:      "multiple nested quantifiers",
		},
		{
			name:        "Excessive length",
			pattern:     strings.Repeat("(a|b)", 200) + "$", // 200 * 5 + 1 = 1001 chars, exceeds 512 limit
			shouldError: true,
			reason:      "pattern exceeds length limit",
		},
		{
			name:        "Simple valid pattern",
			pattern:     "^error: .*$",
			shouldError: false,
			reason:      "",
		},
		{
			name:        "Valid alternation",
			pattern:     "^(warning|error|info):.*",
			shouldError: false,
			reason:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NoiseRule{
				ID:         "test-rule",
				Category:   "console",
				Classification: "framework",
				MatchSpec: NoiseMatchSpec{
					MessageRegex: tt.pattern,
				},
			}

			err := nc.AddRules([]NoiseRule{rule})

			if tt.shouldError && err == nil {
				t.Errorf("Expected AddRules to reject %s pattern, but it was accepted", tt.reason)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected AddRules to accept pattern, but got error: %v", err)
			}
		})
	}
}

// ============================================
// C-3: Unescaped Test Name in Codegen
// ============================================

func TestGenerateTestScript_EscapesTestName(t *testing.T) {
	tests := []struct {
		name          string
		inputTestName string
		mustContain   string // The properly escaped version we MUST see
		mustNotMatch  string // A regex that would match if improperly escaped
		description   string
	}{
		{
			name:          "Single quote injection",
			inputTestName: "Test'); process.exit(1); //",
			mustContain:   "Test\\'); process.exit(1); //", // The ' must be escaped as \'
			mustNotMatch:  `test\('Test'\);`,               // Would match if quote breaks out of string
			description:   "single quote must be escaped",
		},
		{
			name:          "Newline injection",
			inputTestName: "Test\nmalicious code here",
			mustContain:   "Test\\nmalicious code here", // Newline must be escaped as \n
			mustNotMatch:  "Test\nmalicious",             // Would match if newline is literal
			description:   "newline must be escaped",
		},
		{
			name:          "Backslash escape",
			inputTestName: "Test\\' injection",
			mustContain:   "Test\\\\\\' injection", // Backslash must be escaped, then quote escaped
			mustNotMatch:  `Test\\' injection`,     // Would match if backslash not properly escaped
			description:   "backslash must be escaped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal timeline for test script generation
			timeline := []TimelineEntry{
				{
					Kind:      "action",
					Type:      "click",
					Timestamp: 1000,
					Selectors: map[string]interface{}{
						"css": ".button",
					},
				},
			}

			opts := TestGenerationOptions{
				TestName:       tt.inputTestName,
				AssertNetwork:  false,
				AssertNoErrors: false,
			}

			script := generateTestScript(timeline, opts)

			// Verify the escaped version is present
			if !strings.Contains(script, tt.mustContain) {
				t.Errorf("Generated script missing properly escaped test name: %s\nExpected substring: %s\nGot:\n%s",
					tt.description, tt.mustContain, script)
			}

			// Check that dangerous patterns don't match (would indicate improper escaping)
			if matched, _ := regexp.MatchString(tt.mustNotMatch, script); matched {
				t.Errorf("Generated script has improper escaping: %s\nDangerous pattern matched: %s\nScript:\n%s",
					tt.description, tt.mustNotMatch, script)
			}
		})
	}
}
