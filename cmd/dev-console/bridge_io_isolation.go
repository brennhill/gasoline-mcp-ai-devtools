// bridge_io_isolation.go — bridge-mode stdio isolation for MCP transport integrity.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dev-console/dev-console/internal/state"
)

const bridgeWrapperLogFileName = "bridge-wrapper.log"

var (
	bridgeIOSetupMu     sync.Mutex
	bridgeIOConfigured  bool
	bridgeMCPTransport  *os.File
	bridgeWrapperLogOut *os.File
)

// activeMCPTransportWriter returns the file used for MCP JSON-RPC transport.
// In normal mode this is os.Stdout; in bridge isolation mode it's a dedicated
// duplicate of the original stdout pipe.
func activeMCPTransportWriter() *os.File {
	bridgeIOSetupMu.Lock()
	defer bridgeIOSetupMu.Unlock()
	if bridgeMCPTransport != nil {
		return bridgeMCPTransport
	}
	return os.Stdout
}

// ensureBridgeIOIsolation configures bridge mode so stdout/stderr noise cannot
// corrupt MCP JSON-RPC framing on stdout.
func ensureBridgeIOIsolation(logFileHint string) error {
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

	bridgeMCPTransport = transport
	bridgeWrapperLogOut = logOut
	bridgeIOConfigured = true
	setStderrSink(logOut)
	stderrf("[gasoline-bridge] stdio isolation enabled; wrapper logs -> %s\n", wrapperLogPath)

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
	return filepath.Join(os.TempDir(), "gasoline", "logs", bridgeWrapperLogFileName)
}
