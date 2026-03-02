// Purpose: Implements fast-path telemetry diagnostics for setup doctor checks.
// Why: Keeps telemetry parsing and threshold policy separate from user-facing setup orchestration.

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

type fastPathTelemetrySummary struct {
	total      int
	success    int
	failure    int
	errorCodes map[int]int
	methods    map[string]int
}

func summarizeFastPathTelemetryLog(path string, maxLines int) fastPathTelemetrySummary {
	summary := fastPathTelemetrySummary{
		errorCodes: map[int]int{},
		methods:    map[string]int{},
	}
	if maxLines <= 0 {
		return summary
	}

	// #nosec G304 -- path is deterministic under runtime state dir.
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

func evaluateFastPathFailureThreshold(summary fastPathTelemetrySummary, minSamples int, maxFailureRatio float64) error {
	if maxFailureRatio < 0 {
		return nil
	}
	if maxFailureRatio > 1 {
		return fmt.Errorf("max failure ratio must be <= 1.0")
	}
	if minSamples < 1 {
		return fmt.Errorf("min samples must be >= 1")
	}
	if summary.total < minSamples {
		return fmt.Errorf("insufficient samples: got %d, need %d", summary.total, minSamples)
	}
	ratio := float64(summary.failure) / float64(summary.total)
	if ratio > maxFailureRatio {
		return fmt.Errorf("failure ratio %.4f exceeds threshold %.4f (%d/%d failures)", ratio, maxFailureRatio, summary.failure, summary.total)
	}
	return nil
}

func printFastPathTelemetryDiagnostics(maxLines int) (fastPathTelemetrySummary, bool) {
	fmt.Print("Checking bridge fast-path telemetry... ")
	path, err := fastPathTelemetryLogPath()
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Cannot resolve telemetry log path: %v\n", err)
		fmt.Println()
		return fastPathTelemetrySummary{errorCodes: map[int]int{}, methods: map[string]int{}}, false
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			fmt.Println("OK")
			fmt.Printf("  Telemetry log: %s\n", path)
			fmt.Println("  Status: no fast-path telemetry recorded yet")
			fmt.Println()
			return fastPathTelemetrySummary{errorCodes: map[int]int{}, methods: map[string]int{}}, false
		}
		fmt.Println("FAILED")
		fmt.Printf("  Telemetry log read error: %v\n", statErr)
		fmt.Println()
		return fastPathTelemetrySummary{errorCodes: map[int]int{}, methods: map[string]int{}}, false
	}

	summary := summarizeFastPathTelemetryLog(path, maxLines)
	fmt.Println("OK")
	fmt.Printf("  Telemetry log: %s\n", path)
	fmt.Printf("  Last %d events: total=%d success=%d failure=%d\n", maxLines, summary.total, summary.success, summary.failure)

	if len(summary.methods) > 0 {
		methods := make([]string, 0, len(summary.methods))
		for method := range summary.methods {
			methods = append(methods, method)
		}
		sort.Strings(methods)
		parts := make([]string, 0, len(methods))
		for _, method := range methods {
			parts = append(parts, fmt.Sprintf("%s=%d", method, summary.methods[method]))
		}
		fmt.Printf("  Methods: %s\n", strings.Join(parts, ", "))
	}

	if len(summary.errorCodes) > 0 {
		codes := make([]int, 0, len(summary.errorCodes))
		for code := range summary.errorCodes {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		parts := make([]string, 0, len(codes))
		for _, code := range codes {
			parts = append(parts, fmt.Sprintf("%d=%d", code, summary.errorCodes[code]))
		}
		fmt.Printf("  Error codes: %s\n", strings.Join(parts, ", "))
	} else {
		fmt.Println("  Error codes: none")
	}
	fmt.Println()
	return summary, true
}
