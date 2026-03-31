// doctor.go — Runs CLI setup checks for port/state/telemetry readiness before startup.
// Why: Keeps preflight setup diagnostics separate from live doctor check handlers.

package health

import (
	"fmt"
	"net"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// IsLocalPortAvailable checks whether a local TCP port is available.
func IsLocalPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close() //nolint:errcheck // pre-flight check; port availability probe only
	return true
}

// SuggestAvailablePort finds an available port starting from startPort.
func SuggestAvailablePort(startPort, maxOffset int) (int, bool) {
	for offset := 0; offset <= maxOffset; offset++ {
		candidate := startPort + offset
		if candidate <= 0 {
			continue
		}
		if IsLocalPortAvailable(candidate) {
			return candidate, true
		}
	}
	return 0, false
}

// CheckPortAvailability prints port availability status.
func CheckPortAvailability(port int, portKillHint func(int) string) {
	fmt.Print("Checking port availability... ")
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Println("FAILED")
		fmt.Printf("  Port %d is already in use.\n", port)
		fmt.Printf("  Fix: %s\n", portKillHint(port))
		fmt.Printf("  Quick stop (Kaboom): kaboom --stop --port %d\n", port)
		if suggested, ok := SuggestAvailablePort(port+1, 25); ok {
			fmt.Printf("  Suggested free port: --port %d\n", suggested)
		} else {
			fmt.Printf("  Or use a different port: --port %d\n", port+1)
		}
	} else {
		_ = ln.Close() //nolint:errcheck // pre-flight check; port availability test only
		fmt.Println("OK")
		fmt.Printf("  Port %d is available.\n", port)
	}
	fmt.Println()
}

// CheckStateDirectory prints runtime state directory status.
func CheckStateDirectory() {
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

// RunSetupCheckWithOptions runs the full setup check and returns whether all thresholds pass.
func RunSetupCheckWithOptions(port int, options SetupCheckOptions, deps SetupDeps) bool {
	if options.MinSamples == 0 && options.MaxFailureRatio == 0 {
		options.MaxFailureRatio = -1
	}
	if options.MinSamples == 0 {
		options.MinSamples = 50
	}

	fmt.Println()
	fmt.Println("KABOOM SETUP CHECK")
	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("Version: %s\n", deps.Version)
	fmt.Printf("Port:    %d\n", port)
	fmt.Println()

	CheckPortAvailability(port, deps.PortKillHint)
	CheckStateDirectory()
	summary, _ := PrintFastPathTelemetryDiagnostics(200, deps.FastPathTelemetryLogPath)

	thresholdOK := true
	if options.MaxFailureRatio >= 0 {
		fmt.Print("Checking fast-path failure threshold... ")
		if err := EvaluateFastPathFailureThreshold(summary, options.MinSamples, options.MaxFailureRatio); err != nil {
			fmt.Println("FAILED")
			fmt.Printf("  %v\n", err)
			fmt.Println()
			thresholdOK = false
		} else {
			ratio := 0.0
			if summary.Total > 0 {
				ratio = float64(summary.Failure) / float64(summary.Total)
			}
			fmt.Println("OK")
			fmt.Printf("  Ratio %.4f within threshold %.4f (samples=%d)\n", ratio, options.MaxFailureRatio, summary.Total)
			fmt.Println()
		}
	}

	fmt.Println("────────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Start server:    npx kaboom-agentic-browser")
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
