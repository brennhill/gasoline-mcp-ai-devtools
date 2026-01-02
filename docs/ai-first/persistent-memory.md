# Persistent Cross-Session Memory (`session_store`)

## Status: Specification

---

## Justification

### The Problem

Gasoline's server is ephemeral. When it restarts (developer closes laptop, system update, new terminal session), all captured data is lost:

- Network response patterns learned over hours → gone
- Noise rules configured for this project → gone
- Behavioral baselines saved → gone
- API schemas inferred from traffic → gone
- Error patterns and their resolutions → gone

An AI agent starting a fresh session has zero context about the application's historical behavior. It re-discovers the API, re-classifies noise, and re-learns what "correct" means — wasting the first 5-10 minutes of every session on bootstrap.

### The Cost of Amnesia

| What's forgotten | Re-discovery cost | Sessions to rebuild |
|------------------|-------------------|---------------------|
| API schema (endpoints, shapes) | Agent must hit each endpoint again | 1-3 |
| Noise rules | Agent investigates same noise patterns | 1-2 |
| Behavioral baselines | Agent rebuilds "correct" reference | 1 per feature |
| Error history | Same bugs re-investigated from scratch | Ongoing |
| Performance baselines | No regression detection until re-established | 1-2 |

### Why This is AI-Critical

Human developers carry project knowledge between sessions in their heads. They know "this app has 14 API endpoints," "the dashboard always throws that React warning," "the checkout flow takes 2s to load." AI agents have no equivalent — their context resets completely between sessions (or even within long sessions due to context window limits).

Persistent memory transforms the agent from a "new hire every session" to an "experienced team member" that accumulates project understanding over time.

---

## MCP Tool Interface

### Tool: `session_store`

Manages persistent cross-session storage.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `action` | string | Yes | — | `"save"`, `"load"`, `"list"`, `"delete"`, `"stats"` |
| `key` | string | For save/load/delete | — | Storage key |
| `data` | object | For save | — | Data to persist |
| `namespace` | string | No | `"default"` | Logical grouping (e.g., `"baselines"`, `"noise"`, `"api_schema"`) |

### Tool: `load_session_context`

Convenience tool: loads all relevant context for the current project in one call.

No parameters.

### Response (`load_session_context`)

```json
{
  "project_id": "gasoline_myapp",
  "last_session": "2026-01-22T18:30:00Z",
  "sessions_count": 14,

  "baselines": {
    "count": 5,
    "names": ["login", "dashboard", "checkout", "settings", "user-profile"],
    "last_validated": "2026-01-22T17:45:00Z"
  },

  "api_schema": {
    "endpoints_known": 14,
    "last_updated": "2026-01-22T18:20:00Z",
    "coverage": "12/14 endpoints observed in last 3 sessions"
  },

  "noise_config": {
    "rules_count": 12,
    "auto_detected": 8,
    "manual": 4,
    "entries_filtered_lifetime": 3847
  },

  "error_history": {
    "unresolved": [
      {"fingerprint": "TypeError_app.js:42", "first_seen": "2026-01-20", "occurrences": 7}
    ],
    "resolved_recently": [
      {"fingerprint": "NetworkError_/api/auth", "resolved": "2026-01-22", "sessions_to_fix": 2}
    ]
  },

  "performance_baselines": {
    "endpoints_tracked": 8,
    "degraded_since_last": ["GET /api/projects (was 150ms, now 380ms)"]
  }
}
```

---

## Implementation

### Storage Architecture

```
~/.gasoline/
├── store/
│   ├── {project_hash}/              # Per-project isolation
│   │   ├── baselines/               # Behavioral baselines
│   │   │   ├── login.json
│   │   │   ├── dashboard.json
│   │   │   └── _index.json
│   │   ├── noise/                   # Noise configuration
│   │   │   └── rules.json
│   │   ├── api_schema/              # Inferred API schemas
│   │   │   └── schema.json
│   │   ├── error_history/           # Error occurrence tracking
│   │   │   └── errors.json
│   │   ├── performance/             # Performance baselines
│   │   │   └── endpoints.json
│   │   └── meta.json                # Project metadata
│   └── _global/                     # Cross-project settings
│       └── preferences.json
└── gasoline.db                      # Optional: SQLite for complex queries
```

### Project Identification

Projects are identified by the working directory hash (same as how Git identifies repositories):

```go
func projectID() string {
    // Use CWD of the process that started Gasoline
    // (typically the IDE or terminal where the dev server runs)
    cwd, _ := os.Getwd()
    hash := sha256.Sum256([]byte(cwd))
    return hex.EncodeToString(hash[:8]) // 16-char hex ID
}
```

### Data Types Persisted

| Namespace | What | Size per project | Update frequency |
|-----------|------|-----------------|-----------------|
| `baselines` | Behavioral snapshots | 50-500KB | Per feature completion |
| `noise` | Noise classification rules | 5-20KB | Once per session (auto-detect) |
| `api_schema` | Inferred endpoint schemas | 20-100KB | Per new endpoint observed |
| `error_history` | Error fingerprints + resolutions | 10-50KB | Per error occurrence |
| `performance` | Endpoint latency baselines | 5-20KB | Per session (updated averages) |
| `meta` | Session count, timestamps, project info | < 1KB | Per session |

### Persistence Strategy

```go
type SessionStore struct {
    projectDir string
    mu         sync.RWMutex
    dirty      map[string]bool // Namespaces with unsaved changes
    flushInterval time.Duration // Default: 30 seconds
}

// Write-through for critical data (baselines)
func (s *SessionStore) SaveBaseline(b Baseline) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.writeFile("baselines", b.Name+".json", b)
}

// Buffered for high-frequency data (error history, performance)
func (s *SessionStore) RecordError(e ErrorFingerprint) {
    s.mu.Lock()
    s.errorBuffer = append(s.errorBuffer, e)
    s.dirty["error_history"] = true
    s.mu.Unlock()
    // Flushed every 30s by background goroutine
}

// Flush on shutdown
func (s *SessionStore) Shutdown() {
    s.mu.Lock()
    defer s.mu.Unlock()
    for ns := range s.dirty {
        s.flushNamespace(ns)
    }
}
```

### Automatic Persistence Points

Data is persisted automatically at:
1. **Baseline save/update** — immediate write
2. **Session end** — flush all dirty namespaces (server shutdown, SIGTERM)
3. **Every 30 seconds** — background flush of buffered data
4. **Noise rule changes** — immediate write
5. **API schema updates** — batched with 30s flush

### Automatic Session Bootstrap

When the Gasoline server starts, it automatically:

1. Identifies the project (CWD hash)
2. Loads persisted state into memory:
   - Noise rules → applied to filtering immediately
   - API schema → available to `get_api_schema` immediately
   - Baselines → available to `compare_baseline` immediately
   - Error history → available to `diagnose_error` for pattern matching
3. Logs: `"Loaded session context: 5 baselines, 12 noise rules, 14 endpoints"`

---

## Size Management

### Limits

| Limit | Value | Rationale |
|-------|-------|-----------|
| Max per-project storage | 10MB | Reasonable for development metadata |
| Max baselines per project | 50 | Prevent unbounded growth |
| Max error history entries | 500 | Keep recent patterns, evict old |
| Max API schema endpoints | 200 | Most apps have < 100 endpoints |
| Global storage cap | 100MB (all projects) | Laptop-friendly |

### Eviction Strategy

When limits are reached:
1. **Baselines:** LRU (least recently compared)
2. **Error history:** FIFO (oldest entries removed)
3. **API schema:** Merge/consolidate infrequently-seen endpoints
4. **Performance data:** Keep last 30 days only

### Cleanup Command

```
session_store(action: "stats")

{
  "project": "myapp",
  "total_size_bytes": 245760,
  "namespaces": {
    "baselines": {"size": 180000, "entries": 5},
    "noise": {"size": 8200, "entries": 12},
    "api_schema": {"size": 45000, "endpoints": 14},
    "error_history": {"size": 12000, "entries": 47},
    "performance": {"size": 560, "endpoints": 8}
  },
  "oldest_data": "2026-01-10T09:00:00Z",
  "sessions_recorded": 14
}
```

---

## Proving Improvements

### Metrics

| Metric | Without persistence | With persistence | Measurement |
|--------|-------------------|-----------------|-------------|
| Session bootstrap time | 5-10 min (re-discover everything) | < 5s (load from disk) | Time from server start to agent's first productive action |
| Noise re-investigation | 3-5 noise events investigated per session | 0 (pre-filtered from session 2 onward) | Count noise-triggered investigations |
| Baseline availability | None (lost on restart) | Immediate | Count baselines available at session start |
| Regression detection in session 1 | Impossible (no reference) | Available (loaded baselines) | Can `compare_baseline` detect known regressions? |
| API understanding bootstrap | Agent must observe all endpoints | Immediate (loaded schema) | Endpoints known at session start |
| Cumulative value | Zero (resets each session) | Grows each session | Total stored context size over time |

### Benchmark: Multi-Session Productivity

1. Run 5 sessions with persistence OFF:
   - Each session: agent builds a feature, discovers API, configures noise
   - Measure: tokens spent on bootstrap per session

2. Run 5 sessions with persistence ON:
   - Same features
   - Measure: tokens spent on bootstrap per session

**Target:** Session 1 is identical. Sessions 2-5 show 80% reduction in bootstrap overhead with persistence.

### Benchmark: Cross-Session Regression Detection

1. Session 1: Agent builds features, saves baselines
2. (Server restarts)
3. Session 2: Introduce regressions to features built in Session 1
4. Agent calls `compare_baseline` in Session 2

**Target:** All regressions caught immediately in Session 2 (not possible without persistence).

---

## Privacy & Security

| Concern | Mitigation |
|---------|-----------|
| Sensitive data in baselines | Response shapes only (not values); same sanitization as network bodies (auth headers stripped) |
| Disk access | `~/.gasoline/` only; no network access; no cloud sync |
| Multi-user machines | Per-user home directory; standard Unix permissions |
| Project isolation | CWD hash ensures no cross-project leakage |
| Clearing data | `session_store(action: "delete", namespace: "*")` removes all project data |

---

## Edge Cases

| Case | Handling |
|------|---------|
| Project directory moved | New CWD hash → fresh context. Old data remains until manual cleanup. |
| Multiple projects same machine | Each gets independent storage under its CWD hash |
| Disk full | Graceful degradation: write failures logged, in-memory operation continues |
| Corrupted storage file | Skip corrupted file, log warning, continue with available data |
| Very large API schema | Truncate to top 200 endpoints by observation frequency |
| Server crash (no graceful shutdown) | Last flush was ≤ 30s ago; at most 30s of data lost |
| Concurrent Gasoline instances | File-level locking via `flock()`; second instance reads but doesn't write |
