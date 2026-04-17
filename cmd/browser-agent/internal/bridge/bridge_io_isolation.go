// bridge_io_isolation.go -- Configures stdout/stderr isolation in bridge mode so MCP JSON-RPC framing cannot be corrupted by diagnostic output.
// Why: Duplicates the original stdout for MCP transport and redirects os.Stdout/Stderr to a wrapper log file.
// Docs: docs/features/feature/bridge-restart/index.md

package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// BridgeWrapperLogFileName is the name of the bridge wrapper log file.
const BridgeWrapperLogFileName = "bridge-wrapper.log"

const bridgeWrapperLogFileName = BridgeWrapperLogFileName

var (
	bridgeIOSetupMu     sync.Mutex
	bridgeIOConfigured  bool
	bridgeMCPTransport  atomic.Pointer[os.File] // set once during setup, read on every MCP write
	bridgeWrapperLogOut *os.File
)

// ActiveMCPTransportWriter returns the file used for MCP JSON-RPC transport.
// In normal mode this is os.Stdout; in bridge isolation mode it's a dedicated
// duplicate of the original stdout pipe.
// Lock-free on the read path: uses atomic.Pointer since the value is set once at setup.
func ActiveMCPTransportWriter() *os.File {
	if f := bridgeMCPTransport.Load(); f != nil {
		return f
	}
	return os.Stdout
}

// EnsureIOIsolation configures bridge mode so stdout/stderr noise cannot
// corrupt MCP JSON-RPC framing on stdout.
func EnsureIOIsolation(logFileHint string) error {
	bridgeIOSetupMu.Lock()
	defer bridgeIOSetupMu.Unlock()
	if bridgeIOConfigured {
		return nil
	}

	transport, err := duplicateStdoutForTransport(os.Stdout)
	if err != nil {
		return fmt.Errorf("duplicate transport stdout: %w", err)
	}

	wrapperLogPath := resolveBridgeWrapperLogPath(logFileHint)
	// #nosec G301 -- runtime state directory: owner rwx, group rx for diagnostics
	if mkErr := os.MkdirAll(filepath.Dir(wrapperLogPath), 0o750); mkErr != nil {
		_ = transport.Close()
		return fmt.Errorf("create bridge log directory: %w", mkErr)
	}
	// #nosec G304 -- path resolved from runtime state directory or temp fallback
	logOut, openErr := os.OpenFile(wrapperLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) // nosemgrep: go_filesystem_rule-fileread -- runtime log sink
	if openErr != nil {
		_ = transport.Close()
		return fmt.Errorf("open bridge log file: %w", openErr)
	}

	if redirErr := redirectProcessStdStreams(logOut); redirErr != nil {
		_ = transport.Close()
		_ = logOut.Close()
		return fmt.Errorf("redirect std streams: %w", redirErr)
	}

	bridgeMCPTransport.Store(transport)
	bridgeWrapperLogOut = logOut
	bridgeIOConfigured = true
	deps.SetStderrSink(logOut)
	deps.Stderrf("[kaboom-bridge] stdio isolation enabled; wrapper logs -> %s\n", wrapperLogPath)

	return nil
}

func resolveBridgeWrapperLogPath(logFileHint string) string {
	if path, err := state.InRoot("logs", bridgeWrapperLogFileName); err == nil {
		return path
	}
	if strings.TrimSpace(logFileHint) != "" {
		baseDir := filepath.Dir(logFileHint)
		return filepath.Join(baseDir, bridgeWrapperLogFileName)
	}
	return filepath.Join(os.TempDir(), "kaboom", "logs", bridgeWrapperLogFileName)
}
