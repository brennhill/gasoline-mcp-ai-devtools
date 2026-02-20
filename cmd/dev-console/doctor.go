// doctor.go — Setup check and diagnostic commands (--check, --doctor, configure(doctor)).
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
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

// ============================================
// HTTP Doctor (/doctor endpoint)
// ============================================

// handleDoctorHTTP serves the /doctor HTTP endpoint with JSON readiness checks.
func handleDoctorHTTP(w http.ResponseWriter, cap *capture.Capture) {
	checks := runDoctorChecks(cap)

	overallStatus := "healthy"
	readyForInteraction := true
	for _, c := range checks {
		if c.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		}
		if c.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":                overallStatus,
		"ready_for_interaction": readyForInteraction,
		"version":               version,
		"checks":                checks,
	})
}

// runDoctorChecks runs all live diagnostic checks against the capture instance.
func runDoctorChecks(cap *capture.Capture) []doctorCheck {
	checks := make([]doctorCheck, 0, 8)
	snap := cap.GetHealthSnapshot()

	// 1. Extension connectivity
	if cap.IsExtensionConnected() {
		lastSeen := "unknown"
		if !snap.LastPollTime.IsZero() {
			lastSeen = fmt.Sprintf("%.1fs ago", time.Since(snap.LastPollTime).Seconds())
		}
		checks = append(checks, doctorCheck{
			Name: "extension_connected", Status: "pass",
			Detail: "Extension connected (last seen: " + lastSeen + ")",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "extension_connected", Status: "fail",
			Detail: "Extension is not connected",
			Fix:    "Open the Gasoline extension popup and verify it shows 'Connected'. If not, click the extension icon or reload the page.",
		})
	}

	// 2. Pilot enabled
	if cap.IsPilotEnabled() {
		checks = append(checks, doctorCheck{
			Name: "pilot_enabled", Status: "pass",
			Detail: "AI Web Pilot is enabled",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "pilot_enabled", Status: "warn",
			Detail: "AI Web Pilot is disabled — interact actions will fail",
			Fix:    "Enable AI Web Pilot in the extension popup",
		})
	}

	// 3. Tracked tab
	tracking, tabID, tabURL := cap.GetTrackingStatus()
	if tracking && tabID != 0 {
		checks = append(checks, doctorCheck{
			Name: "tracked_tab", Status: "pass",
			Detail: fmt.Sprintf("Tracking tab %d: %s", tabID, tabURL),
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "tracked_tab", Status: "warn",
			Detail: "No tab is being tracked — observe and interact may return empty results",
			Fix:    "Navigate to a page in Chrome. The extension auto-tracks the active tab.",
		})
	}

	// 4. Circuit breaker
	if !snap.CircuitOpen {
		checks = append(checks, doctorCheck{
			Name: "circuit_breaker", Status: "pass",
			Detail: "Circuit breaker closed (healthy)",
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "circuit_breaker", Status: "fail",
			Detail: "Circuit breaker OPEN: " + snap.CircuitReason,
			Fix:    "Extension is sending too many errors. Check observe(errors) for root cause, then use configure(action:'clear',what:'circuit') to reset.",
		})
	}

	// 5. Command queue
	queueDepth := cap.QueueDepth()
	if queueDepth < 5 {
		detail := "Command queue empty"
		if queueDepth > 0 {
			detail = fmt.Sprintf("Command queue: %d pending", queueDepth)
		}
		checks = append(checks, doctorCheck{
			Name: "command_queue", Status: "pass", Detail: detail,
		})
	} else {
		checks = append(checks, doctorCheck{
			Name: "command_queue", Status: "warn",
			Detail: fmt.Sprintf("Command queue has %d pending commands — extension may be falling behind", queueDepth),
			Fix:    "Wait for commands to complete, or check extension connectivity.",
		})
	}

	return checks
}

// ============================================
// MCP Doctor (configure action:"doctor")
// ============================================

// doctorCheck represents a single diagnostic check result.
type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "pass", "warn", "fail"
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// toolDoctor runs all live diagnostic checks and returns structured results.
// This is the MCP-facing doctor — the daemon is already running.
func (h *ToolHandler) toolDoctor(req JSONRPCRequest) JSONRPCResponse {
	checks := runDoctorChecks(h.capture)

	// Add server uptime (only available via ToolHandler)
	if h.healthMetrics != nil {
		uptime := h.healthMetrics.GetUptime()
		checks = append(checks, doctorCheck{
			Name:   "server_uptime",
			Status: "pass",
			Detail: fmt.Sprintf("Server running for %s (version %s)", uptime.Round(time.Second), version),
		})
	}

	// Aggregate status
	overallStatus := "healthy"
	readyForInteraction := true
	for _, c := range checks {
		if c.Status == "fail" {
			overallStatus = "unhealthy"
			readyForInteraction = false
		}
		if c.Status == "warn" && overallStatus != "unhealthy" {
			overallStatus = "degraded"
			readyForInteraction = false
		}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Doctor: "+overallStatus, map[string]any{
		"status":                overallStatus,
		"ready_for_interaction": readyForInteraction,
		"checks":                checks,
		"hint":                  h.DiagnosticHintString(),
	})}
}
