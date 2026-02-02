---
status: draft
scope: v6-v7-architecture
last-updated: 2026-01-31
---

# Target Architecture: Gasoline v6-v7

**Scope:** End-state system architecture for Layer 1 (Monolith) and Layer 2 (Distributed) debugging.

---

## Components

### Browser Extension (MV3)
- Captures FE logs, network requests, user actions, DOM changes
- Runs in user's browser
- Sends events via HTTPS to local Gasoline daemon
- Location: User's machine

### Local Gasoline Daemon
- Single Go binary (zero deps)
- Runs on developer's machine (localhost:7890)
- Ingests events from: Extension, backend logs, SDK/log adapters
- Stores in ring buffers (bounded memory, ~200MB)
- Exposes query API (HTTP)
- Exposes MCP interface (stdio)
- Location: Developer's machine

### Backend Components
**Option A: Gasoline SDKs** (per-language)
- Node.js, Python, Go, etc.
- Installed in each service
- Auto-captures logs, network calls, trace headers
- Sends events to local daemon via HTTPS

**Option B: Log Adapters** (platform integration)
- BigQuery, Datadog, Splunk, CloudWatch, etc.
- Developer configures credentials + query filters
- Local daemon polls for logs matching trace ID + timestamp
- Fetches into local ring buffers

**Option C: Both**
- Services with SDKs: Real-time event streaming
- Services without SDKs: Polling from log platform
- Hybrid approach, per-service configuration

### Data Pipeline
```
Event Source → Normalize → Correlate (v7) → Ring Buffer → Query API → LLM
```

---

## Communication Protocols

| Link | Protocol | Direction | Purpose |
|------|----------|-----------|---------|
| Extension → Daemon | HTTPS | Push | Extension sends FE events |
| SDK → Daemon | HTTPS | Push | Backend service sends events |
| Daemon ← Log Adapter | HTTPS (API) | Pull | Daemon queries log platform |
| Inter-service calls | HTTP/HTTPS + W3C Trace Context | Traced | Services propagate trace IDs |
| Daemon → LLM | stdio (MCP) | Query/Response | AI queries captured events |

---

## Hosting Strategy

### Local Development
```
Developer's Machine (single)
├─ Browser (localhost:3000 app)
├─ Dev server (Node/Python/Go)
├─ Gasoline daemon (localhost:7890)
└─ All communication: localhost only
```

**Deployment:** Dev installs extension + runs `go run ./cmd/dev-console`

---

### Layer 1: Live Monolith
```
Developer's Machine
├─ Browser (production.com)
├─ Gasoline daemon (localhost:7890)
└─ Ingest: Backend logs from production

Production
└─ Single service (FE + BE + DB)
   └─ Logs available via: SSH/tail, log aggregation, or log adapter
```

**Communication:**
- Extension captures FE events, sends to localhost:7890 via HTTPS
- Daemon pulls backend logs from production log source (SSH, API, etc.)
- Logs matched by timestamp/request ID

---

### Layer 2: Live Distributed
```
Developer's Machine
├─ Browser (production.com)
├─ Gasoline daemon (localhost:7890)
└─ Ingest from:
   - Option A: SDKs in each service
   - Option B: Log adapters (BigQuery, Datadog, etc.)
   - Option C: Mix of both

Production
├─ Service A (FE)
├─ Service B (Auth/API)
├─ Service C (API/DB)
└─ Service D (Database)

Each service either:
- Runs Gasoline SDK (streams events directly)
- Logs to centralized platform (daemon polls)
```

**Communication:**
- Extension captures FE click with trace ID
- Daemon correlates events from all services via trace ID
- Services send W3C Trace Context headers in all calls
- Daemon reconstructs request flow: FE → Service B → Service C → DB → response

---

## Diagrams

### Local Dev (v6.0)
```
┌─────────────────────────────────────────────────┐
│          Developer's Machine                    │
│                                                  │
│  ┌──────────────┐                               │
│  │  Browser     │──┐                            │
│  │ localhost:30 │  │                            │
│  │      00      │  │  postMessage               │
│  └──────────────┘  │                            │
│                    │                            │
│  ┌─────────────────┘                            │
│  │                                              │
│  ▼                                              │
│  ┌──────────────────────────────────────────┐  │
│  │  Gasoline Daemon (localhost:7890)       │  │
│  │  ┌────────────────────────────────────┐ │  │
│  │  │ Ring Buffers (200MB in memory)    │ │  │
│  │  │ - Browser logs                    │ │  │
│  │  │ - Network calls                   │ │  │
│  │  │ - Backend logs (from tail)        │ │  │
│  │  └────────────────────────────────────┘ │  │
│  │                                          │  │
│  │  HTTP API: /buffers/timeline, /query   │  │
│  │  MCP (stdio): observe(), generate()    │  │
│  └──────────────────────────────────────────┘  │
│     ▲                              ▲           │
│     │                              │           │
│     │ tail -f                      │ LLM       │
│     │                              │ queries   │
│  ┌──┴──────┐                   ┌───┴─────┐    │
│  │  Dev    │                   │ Claude  │    │
│  │ Server  │                   │  Code   │    │
│  │Stdout   │                   │ (stdio) │    │
│  └─────────┘                   └─────────┘    │
│                                                  │
└─────────────────────────────────────────────────┘
```

---

### Layer 1: Live Monolith (v6.0+)
```
┌─────────────────────────────────────────────────┐
│          Developer's Machine                    │
│                                                  │
│  ┌──────────────┐                               │
│  │  Browser     │──┐                            │
│  │ prod.com     │  │  HTTPS                     │
│  └──────────────┘  │                            │
│                    │                            │
│  ┌─────────────────┘                            │
│  │                                              │
│  ▼                                              │
│  ┌──────────────────────────────────────────┐  │
│  │  Gasoline Daemon (localhost:7890)       │  │
│  │  ┌────────────────────────────────────┐ │  │
│  │  │ Ring Buffers (200MB)              │ │  │
│  │  │ - FE events (extension)           │ │  │
│  │  │ - BE logs (polling)               │ │  │
│  │  └────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────┘  │
│     ▲                         │                │
│     │                         │ queries        │
│     │                         ▼                │
│     │                    ┌──────────┐          │
│     │                    │ Claude   │          │
│     │                    │  Code    │          │
│     │                    └──────────┘          │
│     │                                          │
│  ┌──┴──────────────────────────────┐           │
│  │ SSH tunnel / Log API            │           │
│  │ polls production logs            │           │
│  │ (BigQuery, Splunk, S3, etc.)    │           │
│  └────────────────────────────────┘            │
│                                                 │
└─────────────────────────────────────────────────┘
                      │
                      │ HTTPS
                      ▼
             ┌─────────────────┐
             │  Production     │
             │  Service        │
             │ (FE + BE + DB)  │
             │                 │
             │ Logs → Platform │
             │ (BigQuery, etc.)│
             └─────────────────┘
```

---

### Layer 2: Live Distributed (v7.0)
```
┌─────────────────────────────────────────────────┐
│          Developer's Machine                    │
│                                                  │
│  ┌──────────────┐                               │
│  │  Browser     │──┐                            │
│  │ prod.com     │  │  HTTPS + trace ID          │
│  └──────────────┘  │                            │
│                    │                            │
│  ┌─────────────────┘                            │
│  │                                              │
│  ▼                                              │
│  ┌──────────────────────────────────────────┐  │
│  │  Gasoline Daemon (localhost:7890)       │  │
│  │  ┌────────────────────────────────────┐ │  │
│  │  │ Ring Buffers (200MB)              │ │  │
│  │  │ - FE events                       │ │  │
│  │  │ - Service A logs (SDK or poll)   │ │  │
│  │  │ - Service B logs (SDK or poll)   │ │  │
│  │  │ - Service C logs (SDK or poll)   │ │  │
│  │  │ - Inter-service calls (traced)   │ │  │
│  │  └────────────────────────────────────┘ │  │
│  │                                          │  │
│  │  Correlation Engine (v7):               │  │
│  │  Links events by trace ID               │  │
│  └──────────────────────────────────────────┘  │
│     ▲  ▲  ▲  ▲                  │              │
│     │  │  │  │                  │ queries      │
│     │  │  │  │                  ▼              │
│     │  │  │  │             ┌──────────┐        │
│     │  │  │  │             │ Claude   │        │
│     │  │  │  │             │  Code    │        │
│     │  │  │  │             └──────────┘        │
│     │  │  │  │                                 │
│     │  │  │  └─ HTTPS (Pull logs)              │
│     │  │  │                                    │
└─────┼──┼──┼────────────────────────────────────┘
      │  │  │
      │  │  │              Production
      │  │  │              ──────────────────
      │  │  │
      │  │  └─► ┌────────────────────┐
      │  │      │ Service C (API)    │
      │  │      │ - SDK (streams)    │
      │  │      │   OR               │
      │  │      │ - Logs in platform │
      │  │      └────────────────────┘
      │  │            │ HTTP + trace ID
      │  │            ▼
      │  └─► ┌────────────────────┐
      │      │ Service D (DB)     │
      │      │ - SDK (streams)    │
      │      │   OR               │
      │      │ - Logs in platform │
      │      └────────────────────┘
      │
      └─► ┌────────────────────┐
          │ Service B (Auth)   │
          │ - SDK (streams)    │
          │   OR               │
          │ - Logs in platform │
          └────────────────────┘
               │ HTTP + trace ID
               ▼
          ┌────────────────────┐
          │ Service A (FE)     │
          │ - SDK (streams)    │
          │   OR               │
          │ - Logs in platform │
          └────────────────────┘
               │
               ▼ HTTPS
          Browser
          (prod.com)
```

---

## Key Technologies

| Component | Technology | Reasoning |
|-----------|-----------|-----------|
| Extension | Chrome MV3 | Standard browser extension |
| Local Daemon | Go 1.21+ | Single binary, zero deps, cross-platform |
| Storage | Ring buffers (in-memory) | O(1) ops, bounded memory, fast queries |
| Correlation | Trace ID matching (v7) | W3C Trace Context standard |
| Query API | HTTP REST | Simple, language-agnostic |
| LLM Interface | MCP (stdio) | Standard, works with any AI tool |
| Log Polling | Platform-specific APIs | BigQuery, Datadog, Splunk, CloudWatch, etc. |
| SDK Transport | HTTPS | Secure, over internet or VPN |
| Inter-service | HTTP/HTTPS + headers | Existing service infrastructure |

---

## Architecture Phases

### Phase 1: Local Dev (v6.0)
- Extension + local daemon
- Scope: Single machine, single service
- Sufficient for: Layer 1 monolith debugging

### Phase 2: Live Layer 1 (v6.0+)
- Extension + local daemon + log polling
- Scope: Production monolith, logs accessible via API/SSH
- Sufficient for: Layer 1 production debugging

### Phase 3: Live Layer 2 with SDKs (v7.0)
- Extension + local daemon + SDKs in services
- Scope: Distributed system, real-time event streaming
- Sufficient for: Layer 2 production debugging

### Phase 4: Live Layer 2 with Adapters (v7.0)
- Extension + local daemon + log adapters
- Scope: Distributed system, logs in centralized platform
- Sufficient for: Layer 2 production debugging (no SDK install)

### Phase 5: Hybrid (v7.0+)
- Mix of SDKs and log adapters
- Flexible per-service configuration
- Recommended for: Large teams with mixed infrastructure

---

## Out of Scope

- Service mesh topology discovery (Layer 3)
- Automated service discovery
- Credential rotation / secret management
- Multi-user shared observability
- Long-term retention / archival
- Real-time alerting

---

**Status:** Target architecture for v6-v7
**Next:** Detailed deployment guides per layer
