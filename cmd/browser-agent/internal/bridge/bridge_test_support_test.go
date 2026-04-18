// bridge_test_support_test.go -- Test support helpers for bridge package tests.
package bridge

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)


// Note: resetFastPathResourceReadCounters, resetFastPathCounters,
// captureBridgeIO, and parseJSONLines are defined in the test files that were moved from main.

// fastPathTelemetrySummary is a test-local summary type for telemetry log parsing.
type fastPathTelemetrySummary struct {
	total      int
	success    int
	failure    int
	errorCodes map[int]int
	methods    map[string]int
}

// summarizeFastPathTelemetryLog parses fast-path telemetry from a log file (test-only copy).
func summarizeFastPathTelemetryLog(path string, maxLines int) fastPathTelemetrySummary {
	summary := fastPathTelemetrySummary{
		errorCodes: map[int]int{},
		methods:    map[string]int{},
	}
	if maxLines <= 0 {
		return summary
	}

	f, err := os.Open(path)
	if err != nil {
		return summary
	}
	defer func() { _ = f.Close() }()

	lines := make([]string, 0, maxLines)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if len(lines) > maxLines {
			lines = lines[1:]
		}
	}

	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		event, _ := entry["event"].(string)
		if event != "bridge_fastpath_method" {
			continue
		}
		summary.total++
		if ok, _ := entry["success"].(bool); ok {
			summary.success++
		} else {
			summary.failure++
		}
		if method, _ := entry["method"].(string); method != "" {
			summary.methods[method]++
		}
		if code, ok := entry["error_code"].(float64); ok {
			codeInt := int(code)
			if codeInt != 0 {
				summary.errorCodes[codeInt]++
			}
		}
	}
	return summary
}

// setStderrSink is a test helper that delegates to deps.SetStderrSink.
func setStderrSink(w io.Writer) {
	if deps.SetStderrSink != nil {
		deps.SetStderrSink(w)
	}
}
