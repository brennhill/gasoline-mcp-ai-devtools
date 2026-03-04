# Flow Map: Terminal Server Isolation

## Overview

The terminal runs on a dedicated HTTP server (port+1) isolated from the main MCP daemon. This prevents shared timeouts, middleware, and connection handling from interfering with long-lived terminal WebSocket connections.

## Startup Flow

```
runMCPMode()
  │
  ├─ startHTTPServer(port)          → Main daemon (MCP, capture, health)
  │    └─ AuthMiddleware(apiKey)
  │    └─ WriteTimeout: 65s
  │
  ├─ setupTerminalMux()             → New ServeMux with terminal routes only
  │    └─ registerTerminalRoutes()
  │    └─ No AuthMiddleware
  │
  ├─ startTerminalServer(port+1)    → Dedicated terminal server
  │    └─ WriteTimeout: 0 (WebSocket)
  │    └─ IdleTimeout: 0
  │    └─ On bind failure → log WARNING, continue without terminal
  │
  └─ awaitShutdownSignal()
       └─ termSrv.Shutdown() first (2s timeout)
       └─ srv.Shutdown() second (3s timeout)
```

## Port Assignment

```
Main:     127.0.0.1:{PORT}      (default 7890)
Terminal: 127.0.0.1:{PORT+1}    (default 7891)
```

## Extension URL Computation

```
Base URL:     http://localhost:7890
Terminal URL: http://localhost:7891  (port + TERMINAL_PORT_OFFSET)
```

Applied to: iframe src, fetch() calls, postMessage origin checks.

## Failure Modes

| Scenario | Behavior |
|----------|----------|
| Port+1 busy at startup | WARNING logged, daemon starts without terminal |
| Terminal server dies at runtime | Logged, `terminal_port` set to 0, main daemon unaffected |
| Main daemon dies | Terminal server also shuts down (graceful shutdown sequence) |

## Code Anchors

| File | Purpose |
|------|---------|
| `cmd/dev-console/terminal_server.go` | `setupTerminalMux()`, `startTerminalServer()` |
| `cmd/dev-console/terminal_handlers.go` | All terminal route handlers |
| `cmd/dev-console/main_connection_mcp.go` | Terminal server startup wiring |
| `cmd/dev-console/main_connection_mcp_shutdown.go` | Terminal server graceful shutdown |
| `cmd/dev-console/server_routes_health_diagnostics.go` | `terminal_port` in health response |
| `src/content/ui/terminal-widget.ts` | `getTerminalServerUrl()`, all fetch/iframe URLs |
| `src/lib/constants.ts` | `TERMINAL_PORT_OFFSET = 1` |

## Feature Docs

- [Terminal feature index](../../features/feature/terminal/index.md)
