// doctor.go — Setup check and diagnostic commands (--check, --doctor).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/state"
)

// checkPortAvailability prints port availability status.
func checkPortAvailability(port int) {
	fmt.Print("Checking port availability... ")
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Port %d is already in use.\n", port)
		fmt.Printf("  Fix: %s\n", portKillHint(port))
		fmt.Printf("  Or use a different port: --port %d\n", port+1)
	} else {
		_ = ln.Close() //nolint:errcheck // pre-flight check; port availability test only
		fmt.Println("OK")
		fmt.Printf("  Port %d is available.\n", port)
	}
	fmt.Println()
}

// checkStateDirectory prints runtime state directory status.
func checkStateDirectory() {
	fmt.Print("Checking runtime state directory... ")
	rootDir, err := state.RootDir()
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Cannot determine runtime state directory: %v\n", err)
	} else {
		logFile, _ := state.DefaultLogFile()
		fmt.Println("OK")
		fmt.Printf("  State dir: %s\n", rootDir)
		fmt.Printf("  Log file: %s\n", logFile)
	}
	fmt.Println()
}

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

// runSetupCheck verifies the setup and prints diagnostic information
func runSetupCheck(port int) {
	_ = runSetupCheckWithOptions(port, setupCheckOptions{
		minSamples:      50,
		maxFailureRatio: -1,
	})
}

func runSetupCheckWithOptions(port int, options setupCheckOptions) bool {
	fmt.Println()
	fmt.Println("GASOLINE SETUP CHECK")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Port:    %d\n", port)
	fmt.Println()

	checkPortAvailability(port)
	checkStateDirectory()
	summary, _ := printFastPathTelemetryDiagnostics(200)

	thresholdOK := true
	if options.maxFailureRatio >= 0 {
		fmt.Print("Checking fast-path failure threshold... ")
		if err := evaluateFastPathFailureThreshold(summary, options.minSamples, options.maxFailureRatio); err != nil {
			fmt.Println("FAILED")
			fmt.Printf("  %v\n", err)
			fmt.Println()
			thresholdOK = false
		} else {
			ratio := 0.0
			if summary.total > 0 {
				ratio = float64(summary.failure) / float64(summary.total)
			}
			fmt.Println("OK")
			fmt.Printf("  Ratio %.4f within threshold %.4f (samples=%d)\n", ratio, options.maxFailureRatio, summary.total)
			fmt.Println()
		}
	}

	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start server:    npx gasoline-mcp")
	fmt.Println("  2. Install extension:")
	fmt.Println("     - Open chrome://extensions")
	fmt.Println("     - Enable Developer mode")
	fmt.Println("     - Click 'Load unpacked' → select extension/ folder")
	fmt.Println("  3. Open any website")
	fmt.Println("  4. Extension popup should show 'Connected'")
	fmt.Println()
	fmt.Printf("Verify:  curl http://localhost:%d/health\n", port)
	fmt.Println()
	return thresholdOK
}
