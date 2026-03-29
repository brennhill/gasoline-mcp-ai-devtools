// terminal_server.go — Dedicated HTTP server for the in-browser terminal.
// Why: Isolates terminal WebSocket, HTML page, static assets, and session lifecycle
// onto a separate port so shared timeouts and middleware on the main server can't interfere
// with long-lived terminal connections.
// Docs: docs/features/feature/terminal/index.md

package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/pty"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// terminalPortOffset is the offset from the main daemon port for the terminal server.
const terminalPortOffset = 1

// setupTerminalMux creates a new ServeMux with only terminal routes.
// No AuthMiddleware — terminal uses its own session token validation.
func setupTerminalMux(server *Server, mgr *pty.Manager, cap *capture.Store) *http.ServeMux {
	mux := http.NewServeMux()
	registerTerminalRoutes(mux, server, mgr, cap)
	return mux
}

// startTerminalServer launches the terminal HTTP server on the given port.
// WriteTimeout and IdleTimeout are zero because WebSocket connections are long-lived.
// Returns the server, a done channel (closes if listener dies), and any bind error.
func startTerminalServer(server *Server, port int, mux *http.ServeMux) (*http.Server, <-chan struct{}, error) {
	ready := make(chan error, 1)
	done := make(chan struct{})
	srv := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 0, // WebSocket connections are long-lived
		IdleTimeout:  0, // No idle timeout for terminal connections
		Handler:      mux,
	}
	util.SafeGo(func() {
		defer close(done)
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			ready <- err
			return
		}
		ready <- nil
		// #nosec G114 -- localhost-only terminal server
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			stderrf("[Kaboom] terminal server error on port %d: %v\n", port, err)
		}
	})

	if err := <-ready; err != nil {
		return nil, nil, fmt.Errorf("terminal server: cannot bind port %d: %w", port, err)
	}
	return srv, done, nil
}
