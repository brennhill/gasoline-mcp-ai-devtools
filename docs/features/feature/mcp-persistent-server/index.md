# MCP Persistent Server

Enables the Gasoline daemon to run as a long-lived persistent process rather than a transient stdio-based MCP server.

## Overview

The persistent server mode allows the daemon to survive beyond a single MCP client session. This includes process management utilities for detecting and managing the daemon lifecycle across platforms.

## Key Capabilities

- Platform-specific process detection (Unix signals, Windows process queries)
- Daemon lifecycle management across MCP client reconnections
- Process identity verification to prevent stale PID conflicts

## Code References

- `internal/util/proc_unix.go` — Unix process management
- `internal/util/proc_windows.go` — Windows process management
- Flow map: [flow-map.md](./flow-map.md)
- Related flow map: [Daemon Stop and Force Cleanup](../../../architecture/flow-maps/daemon-stop-and-force-cleanup.md)

## Status

**Shipped** — Active in production with platform-specific process utilities.
