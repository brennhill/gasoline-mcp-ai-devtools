// Purpose: Defines serverConfig struct, CLI flag parsing, runtime mode constants, and startup orchestration.
// Why: Centralizes all command-line configuration and mode selection logic for the daemon entry point.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"
)

// multiFlag implements flag.Value for repeatable string flags (e.g., --upload-deny-pattern).
type multiFlag []string

func (f *multiFlag) String() string { return strings.Join(*f, ", ") }
func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// serverConfig holds the parsed command-line flags for the server.
type serverConfig struct {
	port         int
	logFile      string
	maxEntries   int
	apiKey       string
	stateDir     string
	clientID     string
	bridgeMode   bool
	daemonMode   bool
	parallelMode bool
}

type runtimeMode string

const (
	modeBridge runtimeMode = "bridge"
	modeDaemon runtimeMode = "daemon"
)

// parsedFlags holds the raw parsed flag values before validation.
type parsedFlags struct {
	port, maxEntries                                                     *int
	fastPathMinSamples                                                   *int
	logFile, apiKey, clientID, stateDir, uploadDir                       *string
	fastPathMaxFailureRatio                                              *float64
	showVersion, showHelp, checkSetup, doctorMode, stopMode, connectMode *bool
	bridgeMode, daemonMode, enableOsUploadAutomation                     *bool
	parallelMode                                                         *bool
	forceCleanup                                                         *bool
	installMode                                                          *bool
	uploadDenyPatterns                                                   multiFlag
	ssrfAllowedHosts                                                     multiFlag
}

// registerFlags defines all CLI flags and returns the parsed values.
func registerFlags() *parsedFlags {
	f := &parsedFlags{}
	f.port = flag.Int("port", defaultPort, "Port to listen on")
	f.logFile = flag.String("log-file", "", "Path to log file (default: in runtime state dir)")
	f.maxEntries = flag.Int("max-entries", defaultMaxEntries, "Max log entries before rotation")
	f.fastPathMinSamples = flag.Int("fastpath-min-samples", 50, "Minimum fast-path telemetry samples required when threshold check is enabled")
	f.fastPathMaxFailureRatio = flag.Float64("fastpath-max-failure-ratio", -1, "Maximum allowed fast-path failure ratio in --check (set >=0 to enforce)")
	f.showVersion = flag.Bool("version", false, "Show version")
	f.showHelp = flag.Bool("help", false, "Show help")
	f.apiKey = flag.String("api-key", os.Getenv("GASOLINE_API_KEY"), "API key for HTTP authentication (optional, or GASOLINE_API_KEY env)")
	f.checkSetup = flag.Bool("check", false, "Verify setup: check if port is available and print status")
	f.doctorMode = flag.Bool("doctor", false, "Run full diagnostics (alias of --check)")
	f.stopMode = flag.Bool("stop", false, "Stop the running server on the specified port")
	f.connectMode = flag.Bool("connect", false, "Connect to existing server (multi-client mode)")
	f.clientID = flag.String("client-id", "", "Override client ID (default: derived from CWD)")
	f.bridgeMode = flag.Bool("bridge", false, "Run as stdio-to-HTTP bridge (spawns daemon if needed)")
	f.daemonMode = flag.Bool("daemon", false, "Run as background server daemon (internal use)")
	f.parallelMode = flag.Bool("parallel", false, "Enable isolated parallel daemon mode (skip takeover; requires unique port/state-dir)")
	f.stateDir = flag.String("state-dir", "", "Directory for runtime state (default: OS app state directory)")
	f.enableOsUploadAutomation = flag.Bool("enable-os-upload-automation", false, "Enable OS-level file upload automation (Stage 4: AppleScript/xdotool)")
	f.uploadDir = flag.String("upload-dir", "", "Directory from which file uploads are allowed (required for Stages 2-4)")
	f.forceCleanup = flag.Bool("force", false, "Force kill all running gasoline daemons (used during install to ensure clean upgrade)")
	f.installMode = flag.Bool("install", false, "Auto-install Gasoline to all detected MCP clients")
	flag.Bool("mcp", false, "Run in MCP mode (default, kept for backwards compatibility)")
	flag.Bool("persist", true, "Deprecated no-op (server persistence is default, kept for backwards compatibility)")
	flag.Var(&f.uploadDenyPatterns, "upload-deny-pattern", "Additional sensitive path patterns to block (repeatable)")
	flag.Var(&f.ssrfAllowedHosts, "ssrf-allow-host", "Host:port to allow for form submit SSRF (repeatable, test use)")
	flag.Parse()
	return f
}

type setupCheckOptions struct {
	minSamples      int
	maxFailureRatio float64
}

// parseAndValidateFlags parses CLI flags, validates them, and handles early-exit modes.
func parseAndValidateFlags() *serverConfig {
	f := registerFlags()

	osUploadAutomationFlag = *f.enableOsUploadAutomation
	upload.SSRFAllowedHostsList = f.ssrfAllowedHosts
	initUploadSecurity(*f.enableOsUploadAutomation, *f.uploadDir, f.uploadDenyPatterns)
	validatePort(*f.port)
	normalizeStateDir(f.stateDir)
	if err := applyParallelModeStateDir(*f.parallelMode, f.stateDir); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid --parallel setup: %v\n", err)
		os.Exit(1)
	}
	handleEarlyExitModes(f)
	resolveDefaultLogFile(f.logFile)

	return &serverConfig{
		port:         *f.port,
		logFile:      *f.logFile,
		maxEntries:   *f.maxEntries,
		apiKey:       *f.apiKey,
		stateDir:     *f.stateDir,
		clientID:     *f.clientID,
		bridgeMode:   *f.bridgeMode,
		daemonMode:   *f.daemonMode,
		parallelMode: *f.parallelMode,
	}
}
