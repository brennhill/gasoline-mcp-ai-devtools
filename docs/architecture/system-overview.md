---
doc_type: architecture_overview
status: active
last_reviewed: 2026-02-24
owners: []
---

# System Overview

## Scope

Gasoline MCP is a local-first system that allows AI clients to inspect and automate browser behavior through a Go MCP server and Chrome extension.

## Major Components

| Component | Responsibility | Primary Code |
| --- | --- | --- |
| AI client (Codex/Claude/Gemini) | Sends MCP JSON-RPC tool requests | External client + `cmd/dev-console/handler.go` |
| Bridge/daemon binary (`gasoline-mcp`) | MCP transport, tool execution, persistence, and extension integration | `cmd/dev-console/` |
| Capture + tool internals | Buffer state, async queue, tool-specific logic | `internal/capture/`, `internal/tools/` |
| Chrome extension | Captures telemetry and executes browser actions | `src/background/`, `src/content/`, `src/popup/` |
| Runtime state | Local logs, pid, artifacts | `internal/state/` |

## Top-Level Data Flows

1. AI client -> bridge/daemon -> `ToolHandler` -> result (`observe`, `analyze`, `generate`, `configure`, `interact`).
2. Extension telemetry -> HTTP ingest (`/sync`, `/network-*`, `/ws-events`) -> capture buffers -> tool reads.
3. Async browser commands -> queue in server -> extension polls `/sync` -> extension posts result -> tool returns final status/result.

## Trust and Boundary Model

- Data is local by default; exports are explicit user actions.
- Bridge and daemon communicate over localhost.
- Sensitive content should be redacted before response or storage where required.

## Canonical References

- [Server Architecture](../core/server-architecture.md)
- [Extension Architecture](../core/extension-architecture.md)
- [Async Queue ADR](ADR-001-async-queue-pattern.md)
- [Async Queue Immutability ADR](ADR-002-async-queue-immutability.md)
