// compress_output.go — Output compression for Claude Code PostToolUse hooks.
// Detects test runner and build output patterns, compresses to summary + errors.

package hook

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	minLinesForCompression = 50
	minLinesForGenericTrunc = 100
	genericHeadLines       = 30
	genericTailLines       = 20
)

// CompressResult holds the outcome of output compression.
type CompressResult struct {
	Category     string
	Compressed   string
	TokensBefore int
	TokensAfter  int
	OrigLines    int
	CompLines    int
}

// CompressOutput detects patterns in Bash tool output and compresses verbose results.
// Returns nil if no compression was applied (output too short or no pattern matched).
func CompressOutput(input Input) *CompressResult {
	if input.ToolName != "Bash" {
		return nil
	}

	output := input.ResponseText()
	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	totalLines := len(lines)

	if totalLines < minLinesForCompression {
		return nil
	}

	fields := input.ParseToolInput()
	command := fields.Command

	category, compressed := detectAndCompress(lines, command)

	if compressed == "" && totalLines <= minLinesForGenericTrunc {
		return nil
	}

	if compressed == "" && totalLines > minLinesForGenericTrunc {
		category = "generic_truncation"
		head := strings.Join(lines[:genericHeadLines], "\n")
		tail := strings.Join(lines[totalLines-genericTailLines:], "\n")
		compressed = fmt.Sprintf("%s\n\n...truncated (%d total lines)\n\n%s", head, totalLines, tail)
	}

	if compressed == "" {
		return nil
	}

	compLines := len(strings.Split(compressed, "\n"))
	tokensBefore := len(output) / 4
	tokensAfter := len(compressed) / 4

	return &CompressResult{
		Category:     category,
		Compressed:   compressed,
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		OrigLines:    totalLines,
		CompLines:    compLines,
	}
}

// FormatContext returns the additionalContext string for the hook output.
func (r *CompressResult) FormatContext() string {
	return fmt.Sprintf("[Output compressed: %d lines -> %d lines, ~%d -> ~%d tokens]\n\n%s",
		r.OrigLines, r.CompLines, r.TokensBefore, r.TokensAfter, r.Compressed)
}

// PostStats sends token savings to the daemon (best-effort, non-blocking).
func (r *CompressResult) PostStats(port string) {
	body := fmt.Sprintf(`{"category":%q,"tokens_before":%d,"tokens_after":%d}`,
		r.Category, r.TokensBefore, r.TokensAfter)
	client := &http.Client{Timeout: 1e9} // 1 second
	u := fmt.Sprintf("http://127.0.0.1:%s/api/token-savings", url.PathEscape(port))
	resp, err := client.Post(u, "application/json", strings.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}

// detectAndCompress tries each compression pattern and returns (category, compressed).
func detectAndCompress(lines []string, command string) (string, string) {
	cmdLower := strings.ToLower(command)

	// Test runners.
	if strings.Contains(cmdLower, "go test") || hasGoTestMarkers(lines) {
		if result := compressGoTest(lines); result != "" {
			return "test_output", result
		}
	}
	if containsAny(cmdLower, "jest", "vitest") || hasJestMarkers(lines) {
		if result := compressJestVitest(lines); result != "" {
			return "test_output", result
		}
	}
	if strings.Contains(cmdLower, "cargo test") || hasCargoTestMarkers(lines) {
		if result := compressCargoTest(lines); result != "" {
			return "test_output", result
		}
	}
	if strings.Contains(cmdLower, "pytest") || hasPytestMarkers(lines) {
		if result := compressPytest(lines); result != "" {
			return "test_output", result
		}
	}

	// Build tools.
	if containsAny(cmdLower, "go build", "go vet") {
		if result := compressGoBuild(lines); result != "" {
			return "build_output", result
		}
	}
	if makeCmdPattern.MatchString(cmdLower) {
		if result := compressMake(lines); result != "" {
			return "build_output", result
		}
	}
	if strings.Contains(cmdLower, "tsc") {
		if result := compressTsc(lines); result != "" {
			return "build_output", result
		}
	}
	if containsAny(cmdLower, "npm run build", "webpack") {
		if result := compressNpmBuild(lines); result != "" {
			return "build_output", result
		}
	}
	if strings.Contains(cmdLower, "cargo build") {
		if result := compressCargoBuild(lines); result != "" {
			return "build_output", result
		}
	}

	return "", ""
}

var (
	makeCmdPattern  = regexp.MustCompile(`^make\b`)
	goErrorPattern  = regexp.MustCompile(`.*\.go:\d+:\d+:.*`)
	durationPattern = regexp.MustCompile(`(\d+\.\d+s)`)
	pytestPattern   = regexp.MustCompile(`\d+ passed|\d+ failed|\d+ error`)
)

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func hasGoTestMarkers(lines []string) bool {
	for _, l := range lines {
		s := strings.TrimSpace(l)
		if strings.HasPrefix(s, "--- PASS:") || strings.HasPrefix(s, "--- FAIL:") || strings.HasPrefix(s, "ok \t") {
			return true
		}
	}
	return false
}

func hasJestMarkers(lines []string) bool {
	for _, l := range lines {
		if strings.Contains(l, "Test Suites:") || strings.Contains(l, "Tests:") {
			return true
		}
	}
	return false
}

func hasPytestMarkers(lines []string) bool {
	end := len(lines)
	start := end - 20
	if start < 0 {
		start = 0
	}
	for _, l := range lines[start:end] {
		if pytestPattern.MatchString(l) {
			return true
		}
	}
	return false
}

func hasCargoTestMarkers(lines []string) bool {
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "test result:") {
			return true
		}
	}
	return false
}

// --- Compression functions ---

func compressGoTest(lines []string) string {
	var passed, failed, summaryLines []string
	failDetails := map[string]string{} // failed test -> first error line
	var durations []string
	var currentRun string
	var pendingErrors []string

	for _, line := range lines {
		s := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(s, "=== RUN"):
			parts := strings.Fields(s)
			if len(parts) > 2 {
				currentRun = parts[len(parts)-1]
			}
			pendingErrors = nil
		case strings.HasPrefix(s, "--- PASS:"):
			passed = append(passed, s)
			currentRun = ""
			pendingErrors = nil
		case strings.HasPrefix(s, "--- FAIL:"):
			failed = append(failed, s)
			if len(pendingErrors) > 0 {
				failDetails[s] = pendingErrors[0]
			}
			currentRun = ""
			pendingErrors = nil
		case strings.HasPrefix(s, "FAIL") && !strings.HasPrefix(s, "--- FAIL:"):
			summaryLines = append(summaryLines, s)
		case strings.HasPrefix(s, "ok "):
			summaryLines = append(summaryLines, s)
			if m := durationPattern.FindString(s); m != "" {
				durations = append(durations, m)
			}
		default:
			if currentRun != "" && s != "" && !strings.HasPrefix(s, "===") {
				pendingErrors = append(pendingErrors, s)
			}
		}
	}

	if len(passed) == 0 && len(failed) == 0 && len(summaryLines) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "go test summary: %d passed, %d failed", len(passed), len(failed))
	if len(durations) > 0 {
		fmt.Fprintf(&b, "\nduration: %s", durations[len(durations)-1])
	}
	if len(failed) > 0 {
		b.WriteString("\n\nFAILED TESTS:")
		for _, f := range failed {
			fmt.Fprintf(&b, "\n  %s", f)
			if detail, ok := failDetails[f]; ok {
				fmt.Fprintf(&b, "\n    %s", detail)
			}
		}
	}
	if len(summaryLines) > 0 {
		b.WriteString("\n")
		for _, s := range summaryLines {
			fmt.Fprintf(&b, "\n%s", s)
		}
	}

	return b.String()
}

func compressJestVitest(lines []string) string {
	var summary, failFiles []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if containsAny(s, "Test Suites:", "Tests:", "Snapshots:", "Time:") {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "FAIL ") && strings.Contains(s, "/") {
			failFiles = append(failFiles, s)
		}
	}
	if len(summary) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("jest/vitest summary:")
	if len(failFiles) > 0 {
		b.WriteString("\nFAILURES:")
		for _, f := range failFiles {
			fmt.Fprintf(&b, "\n  %s", f)
		}
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressPytest(lines []string) string {
	var summary, failures []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if pytestPattern.MatchString(s) {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "FAILED ") || strings.HasPrefix(s, "ERROR ") {
			failures = append(failures, s)
		}
	}
	if len(summary) == 0 && len(failures) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("pytest summary:")
	for _, f := range failures {
		fmt.Fprintf(&b, "\n%s", f)
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressCargoTest(lines []string) string {
	var summary, failures []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "test result:") {
			summary = append(summary, s)
		} else if strings.HasPrefix(s, "test ") && strings.Contains(s, "FAILED") {
			failures = append(failures, s)
		}
	}
	if len(summary) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("cargo test summary:")
	if len(failures) > 0 {
		b.WriteString("\nFAILURES:")
		for _, f := range failures {
			fmt.Fprintf(&b, "\n  %s", f)
		}
	}
	for _, s := range summary {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressGoBuild(lines []string) string {
	var errors []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if goErrorPattern.MatchString(s) || strings.HasPrefix(s, "#") {
			errors = append(errors, s)
		}
	}
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "go build/vet: %d error(s):", len(errors))
	for _, e := range errors {
		fmt.Fprintf(&b, "\n%s", e)
	}
	return b.String()
}

func compressMake(lines []string) string {
	var important []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.Contains(s, "Error") || strings.Contains(s, "make: ***") ||
			strings.Contains(strings.ToLower(s), "warning:") {
			important = append(important, s)
		}
	}
	if len(important) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "make: %d issue(s):", len(important))
	for _, s := range important {
		fmt.Fprintf(&b, "\n%s", s)
	}
	return b.String()
}

func compressTsc(lines []string) string {
	var errors []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.Contains(s, "error TS") {
			errors = append(errors, s)
		}
	}
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "tsc: %d error(s):", len(errors))
	for _, e := range errors {
		fmt.Fprintf(&b, "\n%s", e)
	}
	return b.String()
}

func compressNpmBuild(lines []string) string {
	var errors []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.Contains(s, "ERROR") || strings.Contains(s, "Module not found") {
			errors = append(errors, s)
		}
	}
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "build: %d error(s):", len(errors))
	for _, e := range errors {
		fmt.Fprintf(&b, "\n%s", e)
	}
	return b.String()
}

func compressCargoBuild(lines []string) string {
	var errors []string
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if strings.Contains(s, "error[E") {
			errors = append(errors, s)
		}
	}
	if len(errors) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "cargo build: %d error(s):", len(errors))
	for _, e := range errors {
		fmt.Fprintf(&b, "\n%s", e)
	}
	return b.String()
}
