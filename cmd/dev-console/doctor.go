// Purpose: Runs CLI setup checks for port/state/telemetry readiness before startup.
// Why: Keeps preflight setup diagnostics separate from live doctor check handlers.

package main

import (
	"fmt"
	"net"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
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
