// config.go — CLI flag definitions, parsing, validation, and server configuration.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dev-console/dev-console/internal/session"
	"github.com/dev-console/dev-console/internal/state"
	"github.com/dev-console/dev-console/internal/upload"
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
	port       int
	logFile    string
	maxEntries int
	apiKey     string
	stateDir   string
	clientID   string
	bridgeMode bool
	daemonMode bool
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
	forceCleanup                                                         *bool
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
	f.stateDir = flag.String("state-dir", "", "Directory for runtime state (default: OS app state directory)")
	f.enableOsUploadAutomation = flag.Bool("enable-os-upload-automation", false, "Enable OS-level file upload automation (Stage 4: AppleScript/xdotool)")
	f.uploadDir = flag.String("upload-dir", "", "Directory from which file uploads are allowed (required for Stages 2-4)")
	f.forceCleanup = flag.Bool("force", false, "Force kill all running gasoline daemons (used during install to ensure clean upgrade)")
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

// handleEarlyExitModes handles --version, --help, --force, --check/--doctor, --stop, --connect.
// Calls os.Exit for any matched mode; returns normally if none matched.
func handleEarlyExitModes(f *parsedFlags) {
	if *f.showVersion {
		fmt.Printf("gasoline v%s\n", version)
		os.Exit(0)
	}
	if *f.showHelp {
		printHelp()
		os.Exit(0)
	}
	if *f.forceCleanup {
		runForceCleanup()
		os.Exit(0)
	}
	if *f.checkSetup || *f.doctorMode {
		ok := runSetupCheckWithOptions(*f.port, setupCheckOptions{
			minSamples:      *f.fastPathMinSamples,
			maxFailureRatio: *f.fastPathMaxFailureRatio,
		})
		if !ok {
			os.Exit(1)
		}
		os.Exit(0)
	}
	if *f.stopMode {
		runStopMode(*f.port)
		os.Exit(0)
	}
	if *f.connectMode {
		cwd, _ := os.Getwd()
		id := *f.clientID
		if id == "" {
			id = session.DeriveClientID(cwd)
		}
		runConnectMode(*f.port, id, cwd)
		os.Exit(0)
	}
}

// parseAndValidateFlags parses CLI flags, validates them, and handles early-exit modes.
func parseAndValidateFlags() *serverConfig {
	f := registerFlags()

	osUploadAutomationFlag = *f.enableOsUploadAutomation
	upload.SSRFAllowedHostsList = f.ssrfAllowedHosts
	initUploadSecurity(*f.enableOsUploadAutomation, *f.uploadDir, f.uploadDenyPatterns)
	validatePort(*f.port)
	normalizeStateDir(f.stateDir)
	handleEarlyExitModes(f)
	resolveDefaultLogFile(f.logFile)

	return &serverConfig{
		port:       *f.port,
		logFile:    *f.logFile,
		maxEntries: *f.maxEntries,
		apiKey:     *f.apiKey,
		stateDir:   *f.stateDir,
		clientID:   *f.clientID,
		bridgeMode: *f.bridgeMode,
		daemonMode: *f.daemonMode,
	}
}

// initUploadSecurity validates upload security configuration from CLI flags.
// Always defaults to ~/gasoline-upload-dir when no --upload-dir is specified.
// When --enable-os-upload-automation is NOT set and the dir can't be created, falls back gracefully.
func initUploadSecurity(enabled bool, dir string, denyPatterns multiFlag) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			if enabled {
				fmt.Fprintf(os.Stderr, "[gasoline] Cannot determine home directory for default upload dir: %v\n", err)
				os.Exit(1)
			}
			uploadSecurityConfig = &UploadSecurity{}
			return
		}
		dir = filepath.Join(home, "gasoline-upload-dir")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			if enabled {
				fmt.Fprintf(os.Stderr, "[gasoline] Cannot create default upload dir %s: %v\n", dir, err)
				os.Exit(1)
			}
			uploadSecurityConfig = &UploadSecurity{}
			return
		}
	}
	sec, err := ValidateUploadDir(dir, denyPatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Upload security validation failed: %v\n", err)
		os.Exit(1)
	}
	uploadSecurityConfig = sec
}

// validatePort ensures the port is within the valid TCP range.
func validatePort(port int) {
	if port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid port: %d (must be 1-65535)\n", port)
		os.Exit(1)
	}
}

// normalizeStateDir resolves the --state-dir flag to an absolute path and exports it.
func normalizeStateDir(stateDir *string) {
	if *stateDir == "" {
		return
	}
	absStateDir, err := filepath.Abs(*stateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Invalid --state-dir: %v\n", err)
		os.Exit(1)
	}
	*stateDir = filepath.Clean(absStateDir)
	if err := os.Setenv(state.StateDirEnv, *stateDir); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] Failed to set %s: %v\n", state.StateDirEnv, err)
		os.Exit(1)
	}
}

// resolveDefaultLogFile sets the log file to the runtime state directory default if empty.
func resolveDefaultLogFile(logFile *string) {
	if *logFile != "" {
		return
	}
	defaultLogFile, err := state.DefaultLogFile()
	if err != nil {
		fallback := filepath.Join(os.TempDir(), "gasoline", "logs", "gasoline.jsonl")
		startupWarnings = append(startupWarnings, fmt.Sprintf("state_dir_unwritable: %v; falling back to %s", err, fallback))
		*logFile = fallback
		return
	}
	*logFile = defaultLogFile
}
