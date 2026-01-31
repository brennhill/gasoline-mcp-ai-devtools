---
status: draft
priority: tier-1
phase: v5.4-foundation
effort: 2 weeks
relates-to: [layer1-be-observability.md, target-architecture.md]
blocks: [layer1-debugging, checkpoint-validation]
last-updated: 2026-01-31
---

# Backend Log Ingestion — Product Specification

**Goal:** Capture backend logs (local dev and production) and merge with FE telemetry for unified request tracing in Layer 1.

---

## Problem Statement

v5.3 provides FE observability:
- ✅ Console logs, network calls, exceptions
- ✅ User actions with DOM context
- ✅ WebSocket messages

Missing for Layer 1 debugging:
- ❌ Backend service logs (no visibility into what the server saw)
- ❌ Request correlation (no way to link FE event to BE event)
- ❌ Full request flow (UI → API → database → response)

**Impact:** AI can see "checkout failed" in UI but not "why" (DB timeout, validation error, auth failure, etc.)

---

## User Stories

### Story 1: Local Dev Debugging
**As:** Developer debugging locally
**I want:** To see backend logs merged with FE events automatically
**So that:** I understand what happened when a user action fails

```
Workflow:
1. Run: gasoline-run npm run dev
2. Browser opens: localhost:3000
3. User clicks: "Add to Cart"
4. Extension captures: Click event, network call
5. Stdout capture: "[2024-01-01T12:00:00Z] INFO [req:abc123] Cart updated"
6. Gasoline merges both
7. Query result shows:
   - FE: User clicked button (12:00:00.000)
   - FE: POST /api/cart sent (12:00:00.002)
   - BE: Received request (12:00:00.003) ← from stdout
   - BE: Database query (12:00:00.015) ← from stdout
   - FE: Response received 200 (12:00:00.025)
8. AI analyzes: "Cart updated in 25ms, no errors detected"
```

---

### Story 2: Production Debugging (SSH)
**As:** Developer debugging production
**I want:** To see production logs correlated with FE events without SSH-ing manually
**So that:** I can understand production issues faster

```
Workflow:
1. Configure: ~/.gasoline/config.yaml
   ```yaml
   backend:
     logs:
       - type: ssh
         host: prod.example.com
         user: ubuntu
         path: /var/log/app.log
   ```
2. Start: gasoline (daemon connects to prod via SSH)
3. Open: production website in browser
4. User sees: Error on checkout page
5. Extension captures: FE error, network 500
6. Gasoline tails: /var/log/app.log on prod server
7. Correlates: FE error (timestamp 12:00:00) ← → BE error (timestamp 12:00:00.050)
8. Query shows:
   - FE: User clicked "Checkout" (12:00:00.000)
   - FE: POST /api/checkout sent (12:00:00.005)
   - BE: Received request (12:00:00.006) ← from SSH
   - BE: Database connection timeout (12:00:00.050) ← from SSH
   - BE: 500 error response (12:00:00.055) ← from SSH
   - FE: Error displayed to user (12:00:00.060)
9. AI analyzes: "Database is down, checkout is failing"
```

---

### Story 3: Multi-Process Local Dev
**As:** Developer with microservices locally (compose)
**I want:** To see logs from all services merged in one timeline
**So that:** I can trace requests across service boundaries

```
Workflow:
1. docker-compose up
   - Service A (FE): localhost:3000
   - Service B (Auth): localhost:3001
   - Service C (API): localhost:3002
2. gasoline-run docker-compose logs --follow
   OR configure multiple log sources:
   ```yaml
   backend:
     logs:
       - type: file
         path: service-a.log
       - type: file
         path: service-b.log
       - type: file
         path: service-c.log
   ```
3. User clicks: Login button
4. Timeline shows:
   - FE: Click login (Service A stdout)
   - BE: POST /auth (Service B stdout)
   - BE: POST /api/validate (Service C stdout)
   - BE: Response 200 (Service B stdout)
   - FE: Redirect home (Service A stdout)
5. AI traces: Full request path through services
```

---

## Features to Implement

### Feature 1: gasoline-run Wrapper (Local Dev)
**What:** Wrapper command that intercepts stdout/stderr and streams to daemon

**How:**
```bash
gasoline-run npm run dev           # Node
gasoline-run python -m flask run   # Python
gasoline-run go run main.go        # Go
gasoline-run docker-compose up     # Docker
```

**Data captured:**
- Every line of stdout/stderr
- Real-time streaming to daemon
- Pass-through to user's terminal (logs still visible)
- Auto-correlate by request ID in logs

**Success Criteria:**
- [ ] Works with any child command
- [ ] Logs appear in daemon within 10ms of writing
- [ ] Zero performance impact on dev server
- [ ] Pass-through doesn't buffer output
- [ ] Works with piped output (e.g., `gasoline-run npm run dev | tee output.log`)

---

### Feature 2: Local File Log Tailer (Production Single-Server)
**What:** Tail a log file and stream new lines to daemon

**How:**
```yaml
backend:
  logs:
    - type: file
      path: /var/log/app.log
      poll_interval_ms: 100
```

**Data captured:**
- New lines added to file
- Polls every 100ms (configurable)
- Handles file rotation (if log file disappears, reopen)
- Extracts: timestamp, level, message, correlation_id

**Success Criteria:**
- [ ] Detects new lines within 100ms of being written
- [ ] Handles file rotation gracefully
- [ ] Memory efficient (doesn't buffer entire file)
- [ ] Continues from last position on restart
- [ ] Works with large log files (1GB+)

---

### Feature 3: SSH Remote Log Tailer (Production Distributed)
**What:** SSH into remote server and tail log file

**How:**
```yaml
backend:
  logs:
    - type: ssh
      host: prod.example.com
      user: ubuntu
      path: /var/log/app.log
      auth: ~/.ssh/id_rsa
```

**Data captured:**
- SSH connection to remote server
- Runs `tail -f` on remote server
- Streams back to local daemon
- Same log parsing as local file

**Success Criteria:**
- [ ] SSH connection established within 5 seconds
- [ ] Log lines stream within 500ms (network dependent)
- [ ] Reconnects on SSH timeout
- [ ] Continues tailing after brief network interruptions
- [ ] Supports key-based auth (no password)

---

### Feature 4: Multi-Log Source Configuration
**What:** Support multiple log sources simultaneously

**How:**
```yaml
backend:
  logs:
    # Service A
    - type: file
      path: /var/log/service-a.log
      tags: ["service:a"]

    # Service B (remote)
    - type: ssh
      host: service-b.prod.com
      path: /var/log/app.log
      tags: ["service:b"]

    # Service C (remote)
    - type: ssh
      host: service-c.prod.com
      path: /var/log/app.log
      tags: ["service:c"]
```

**Data captured:**
- All sources stream to same daemon
- Events tagged with source service
- Query can filter by service or aggregate across all

**Success Criteria:**
- [ ] All sources stream simultaneously
- [ ] No interference between sources
- [ ] Query can show "all logs" or "logs from service X"
- [ ] Supports 5+ concurrent log sources
- [ ] Graceful degradation if one source fails

---

### Feature 5: Log Parser (Multi-Format)
**What:** Parse logs in any format and extract key fields

**Supported formats:**
- JSON (auto-detect)
- Structured plaintext (regex patterns)
- Simple plaintext (fallback)

**How it works:**
1. **JSON:** Auto-extract standard fields
   ```json
   {
     "timestamp": "2024-01-01T12:00:00Z",
     "level": "info",
     "message": "Request processed",
     "trace_id": "abc123"
   }
   ```

2. **Structured:** User provides regex pattern
   ```
   Pattern: ^\[(.*?)\]\s+(\w+)\s+\[(.*?)\]\s+(.*)$
   Example: [2024-01-01T12:00:00Z] INFO [trace:abc123] Request processed
   Extract: timestamp, level, trace_id, message
   ```

3. **Simple:** No structure, just text
   ```
   "User logged in"
   → timestamp: now, level: info (guessed), message: "User logged in"
   ```

**Data captured:**
- Timestamp (auto-detected format)
- Log level (normalized: debug, info, warn, error, critical)
- Message (primary text)
- Correlation ID (extracted from: req, trace_id, request_id, correlation_id, etc.)
- Metadata (all other fields preserved)

**Success Criteria:**
- [ ] JSON logs parsed 100% correctly
- [ ] Structured logs with regex provided parsed correctly
- [ ] Simple logs never crash (graceful fallback)
- [ ] Correlation ID detected in 95% of logs
- [ ] Timestamp auto-detected in common formats
- [ ] Performance: Parse 10K log lines in <100ms

---

### Feature 6: Correlation & Merging
**What:** Link FE and BE events by correlation_id (trace ID)

**How:**
1. **FE generates trace ID:** UUID (e.g., `550e8400-e29b-41d4`)
2. **FE sends in request:** `X-Trace-ID: 550e8400-e29b-41d4`
3. **BE logs include trace ID:** `[trace:550e8400-e29b-41d4] Processing`
4. **Parser extracts:** `correlation_id: "550e8400-e29b-41d4"`
5. **Query merges:** All events with same `correlation_id`

**Result:**
```
Timeline (sorted by timestamp):
1. [FE] Click "Add to Cart" (12:00:00.000)
2. [FE] POST /api/cart (12:00:00.002)
3. [BE] Received request (12:00:00.003)
4. [BE] Inserted into database (12:00:00.015)
5. [FE] Response 200 (12:00:00.025)
```

**Fallback (no trace ID):**
- Match by timestamp + endpoint (less reliable)
- Flag as "inferred" (for AI context)

**Success Criteria:**
- [ ] All FE requests include X-Trace-ID
- [ ] All BE log entries include trace ID
- [ ] Query by correlation_id returns complete flow
- [ ] Fallback matching works for untagged logs

---

## Architecture: Daemon with Multiple Responsibilities

### Current v5.3 Daemon
```
HTTP Server (localhost:7890)
├─ POST /event ← Extension sends FE events
├─ GET /buffers/timeline ← LLM queries
└─ Ring Buffers ← Stores all events

MCP Server (stdio)
├─ observe() ← LLM queries
├─ generate() ← LLM requests tests
└─ → Reads from Ring Buffers
```

### Extended v5.4+ Daemon
```
HTTP Server (localhost:7890)
├─ POST /event ← Extension sends FE events
├─ POST /event ← gasoline-run sends BE events
├─ GET /buffers/timeline ← LLM queries
└─ Ring Buffers ← Stores all events

Log Ingestion (Concurrent goroutines)
├─ gasoline-run listener
│  ├─ Receives POST /event from wrapper
│  └─ Normalizes to NormalizedEvent
├─ File tailer
│  ├─ Polls local files
│  └─ Sends to HTTP POST /event
└─ SSH tailer
   ├─ SSH connection to remote
   └─ Sends to HTTP POST /event

MCP Server (stdio)
├─ observe() ← LLM queries
├─ generate() ← LLM requests tests
└─ → Reads from Ring Buffers (merged FE + BE)
```

### Key Design Decisions

1. **Single daemon process** (not multiple services)
   - Simpler deployment
   - Shared ring buffers (automatic merging)
   - Single binary stays zero-deps

2. **All ingest via HTTP POST /event**
   - gasoline-run sends HTTP
   - File tailer sends HTTP
   - SSH tailer sends HTTP
   - Consistent interface

3. **Concurrent ingest goroutines**
   - Each log source runs independently
   - Non-blocking to each other
   - Graceful degradation if one fails

4. **Ring buffers unchanged**
   - Events from all sources merge naturally by timestamp
   - Correlation by trace ID works across sources

5. **Backward compatible**
   - v5.3 users don't need to use BE log ingestion
   - v5.4 adds it as optional feature
   - Old configs still work

---

## Success Criteria

### Functional
- [ ] FE and BE events queryable together
- [ ] Correlation by trace ID works reliably
- [ ] gasoline-run wrapper works with any command
- [ ] SSH tailer works without password auth
- [ ] File tailer handles log rotation
- [ ] Multi-source logging works simultaneously

### Performance
- [ ] Log line ingestion: <10ms (local) or <500ms (SSH)
- [ ] Parser throughput: 10K lines/sec
- [ ] Memory footprint: <50MB overhead for log ingestion
- [ ] No impact on FE event latency
- [ ] No impact on MCP query latency

### Developer Experience
- [ ] Local dev: `gasoline-run npm run dev` just works
- [ ] Production: YAML config, no code changes
- [ ] Help text: `gasoline-run --help` explains usage
- [ ] Errors: Clear messages when config wrong
- [ ] Debugging: Logs show what's being ingested

### Backward Compatibility
- [ ] v5.3 config still works
- [ ] Extension still works
- [ ] MCP queries still work
- [ ] No required migrations

---

## Out of Scope (v5.4)

- [ ] Log aggregation platform APIs (BigQuery, Datadog, Splunk) → v7.0
- [ ] Log filtering/sampling (capture all, filter later)
- [ ] Custom log format DSL (regex patterns only)
- [ ] Structured log enrichment (store as-is)
- [ ] Multi-user/multi-tenant support
- [ ] Long-term retention (24h TTL)
- [ ] Log alerting/webhooks

---

## Related Documents

- **Architecture:** [layer1-be-observability.md](../../core/layer1-be-observability.md)
- **Target Architecture:** [target-architecture.md](../../core/target-architecture.md)

---

**Status:** Ready for spec review and tech spec development
**Owner:** (Assign 1-2 engineers)
**Duration:** 2 weeks (gasoline-run + file tailer + parser)
**Next:** Tech specs per process (gasoline-run, local tailer, SSH tailer)
