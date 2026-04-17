// bridge_transport_helpers.go -- Bridge transport support helpers (stdio framing, error envelopes, restart fast path, and shutdown stats).
// Why: Keeps forwarding core focused while helper concerns remain reusable and independently testable.

package bridge

import (
	"bufio"
	"errors"
	"io"
	"sync"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

const (
	// bridgeShutdownResponseDrain is the maximum time to wait for the last response
	// to be sent before closing the bridge stdio connection.
	bridgeShutdownResponseDrain = 5 * time.Second

	// bridgeShutdownFlushDelay is the pause after flushing stdout to let the
	// parent process read the final bytes before the bridge process exits.
	bridgeShutdownFlushDelay = 100 * time.Millisecond
)

type bridgeSessionStats struct {
	requests             int
	parseErrors          int
	invalidIDs           int
	fastPath             int
	forwarded            int
	methodNotFound       int
	starting             int
	lineFraming          int
	contentLengthFraming int
	lastMethod           string
}

// readMCPStdioMessage delegates to internal/bridge for stdio message parsing.
func readMCPStdioMessage(reader *bufio.Reader) ([]byte, internbridge.StdioFraming, error) {
	return internbridge.ReadStdioMessageWithMode(reader, int(deps.MaxPostBodySize))
}

// bridgeShutdown waits for in-flight requests and performs clean shutdown.
func bridgeShutdown(wg *sync.WaitGroup, readErr error, responseSent chan bool, stats *bridgeSessionStats) {
	wg.Wait()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		deps.Stderrf("[kaboom-bridge] ERROR: stdin read error: %v\n", readErr)
	}

	select {
	case <-responseSent:
	case <-time.After(bridgeShutdownResponseDrain):
	}
	close(responseSent)

	FlushStdout()
	time.Sleep(bridgeShutdownFlushDelay)

	if stats != nil {
		// PRIVACY: beacon props must NOT include readErr or any error messages.
		// The extra map below (for appendExitDiagnostic) intentionally includes
		// more detail because it writes to a local file, not to telemetry.
		if stats.parseErrors > 0 || stats.methodNotFound > 0 || (readErr != nil && !errors.Is(readErr, io.EOF)) {
			telemetry.AppError("bridge_exit_error", nil)
		}
		reason := "stdin_eof"
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			reason = "stdin_read_error"
		}
		extra := map[string]any{
			"reason":                 reason,
			"requests":               stats.requests,
			"parse_errors":           stats.parseErrors,
			"invalid_ids":            stats.invalidIDs,
			"fast_path":              stats.fastPath,
			"forwarded":              stats.forwarded,
			"method_not_found":       stats.methodNotFound,
			"starting_retries":       stats.starting,
			"line_framing":           stats.lineFraming,
			"content_length_framing": stats.contentLengthFraming,
			"last_method":            stats.lastMethod,
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			extra["read_error"] = readErr.Error()
		}
		_ = deps.AppendExitDiagnostic("bridge_exit", extra)
	}
}
