# API Schema Inference (`get_api_schema`)

## Status: Specification

---

## Justification

### The Problem

When an AI agent is asked to modify a feature, it needs to understand the API contract:
- What endpoints exist?
- What HTTP methods do they accept?
- What request bodies do they expect?
- What do successful responses look like?
- What do error responses look like?
- How fast should they respond?

Today, the agent either:
1. **Reads documentation** — Often outdated, incomplete, or nonexistent (especially in vibe-coded apps)
2. **Reads server source code** — Requires understanding the backend framework, routing, middleware, ORM
3. **Asks the human** — Breaks the autonomous loop

### The Insight

Gasoline already captures every network request/response body. This traffic IS the API specification — derived from actual behavior, always up-to-date, and requiring no additional effort.

By aggregating observations across a session (or across sessions via persistent memory), Gasoline can infer a typed API schema: endpoints, methods, request/response shapes, status codes, latency profiles, and error patterns.

### Why This is AI-Critical

- **Vibe-coded apps have no docs.** The AI generated the backend, and neither the AI nor the human documented it. The traffic is the only source of truth.
- **Docs drift from reality.** Even well-documented APIs diverge from their specs over time. Observed traffic reflects what the API *actually* does.
- **Agent needs machine-readable contracts.** A human can read Swagger docs. An AI agent needs structured endpoint/shape data to generate correct API calls, validate responses, and write accurate tests.
- **Eliminates round-trips.** Without schema knowledge, the agent must guess-and-check API calls. With a schema, it generates correct calls on the first try.

---

## MCP Tool Interface

### Tool: `get_api_schema`

Returns the inferred API schema from observed network traffic.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `url_filter` | string | No | All | Filter endpoints by URL pattern |
| `include_examples` | bool | No | `false` | Include sanitized example values |
| `min_observations` | int | No | `1` | Minimum times an endpoint must be observed to include |
| `format` | string | No | `"gasoline"` | Output format: `"gasoline"`, `"openapi_stub"` |

### Response (Gasoline format)

```json
{
  "base_url": "http://localhost:3000",
  "observed_since": "2026-01-23T09:00:00Z",
  "total_observations": 247,

  "endpoints": [
    {
      "method": "POST",
      "path": "/api/users",
      "path_pattern": "/api/users",
      "observations": 12,
      "last_seen": "2026-01-23T10:30:00Z",

      "request": {
        "content_type": "application/json",
        "shape": {
          "name": {"type": "string", "required": true, "observed": 12},
          "email": {"type": "string", "required": true, "observed": 12},
          "role": {"type": "string", "required": false, "observed": 3}
        },
        "example": {"name": "...", "email": "...", "role": "admin"}
      },

      "responses": {
        "201": {
          "count": 10,
          "content_type": "application/json",
          "shape": {
            "id": {"type": "string", "format": "uuid"},
            "name": {"type": "string"},
            "email": {"type": "string"},
            "createdAt": {"type": "string", "format": "datetime"}
          }
        },
        "422": {
          "count": 2,
          "content_type": "application/json",
          "shape": {
            "error": {"type": "string"},
            "fields": {"type": "object"}
          }
        }
      },

      "timing": {
        "avg_ms": 145,
        "p50_ms": 120,
        "p95_ms": 310,
        "max_ms": 520
      }
    },
    {
      "method": "GET",
      "path": "/api/users/{id}",
      "path_pattern": "/api/users/:id",
      "observations": 34,
      "path_params": [
        {"name": "id", "type": "string", "format": "uuid", "position": 2}
      ],
      "query_params": [
        {"name": "include", "type": "string", "values_observed": ["profile", "settings"], "required": false}
      ],
      "responses": {
        "200": {
          "count": 30,
          "shape": {
            "id": {"type": "string", "format": "uuid"},
            "name": {"type": "string"},
            "email": {"type": "string"},
            "profile": {"type": "object", "conditional": "include=profile"}
          }
        },
        "404": {
          "count": 4,
          "shape": {"error": {"type": "string"}}
        }
      },
      "timing": {"avg_ms": 45, "p50_ms": 38, "p95_ms": 110}
    }
  ],

  "websockets": [
    {
      "url": "ws://localhost:3000/ws/events",
      "observations": 3,
      "message_schemas": {
        "incoming": [
          {"type_field": "type", "types_observed": ["user_joined", "message", "typing"]},
          {"shape": {"type": "string", "payload": "object", "timestamp": "number"}}
        ],
        "outgoing": [
          {"shape": {"action": "string", "data": "object"}}
        ]
      }
    }
  ],

  "auth_pattern": {
    "type": "bearer",
    "header": "Authorization",
    "observed_in": "85% of requests",
    "excluded_paths": ["/api/auth/login", "/api/health"]
  },

  "coverage": {
    "endpoints_observed": 14,
    "methods_breakdown": {"GET": 8, "POST": 4, "PUT": 1, "DELETE": 1},
    "error_rate": "8.5%",
    "avg_response_time_ms": 95
  }
}
```

### Response (OpenAPI stub format)

When `format: "openapi_stub"`, returns a minimal OpenAPI 3.0 document:

```yaml
openapi: "3.0.0"
info:
  title: "Inferred API Schema"
  description: "Auto-generated from observed traffic by Gasoline"
  version: "observed-2026-01-23"
paths:
  /api/users:
    post:
      summary: "Create user (12 observations)"
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name, email]
              properties:
                name: {type: string}
                email: {type: string}
                role: {type: string}
      responses:
        "201":
          description: "Success (10 observations)"
          content:
            application/json:
              schema:
                type: object
                properties:
                  id: {type: string, format: uuid}
                  name: {type: string}
                  email: {type: string}
                  createdAt: {type: string, format: date-time}
        "422":
          description: "Validation error (2 observations)"
```

---

## Implementation

### Schema Inference Engine

```go
type SchemaInferrer struct {
    endpoints map[string]*EndpointAccumulator // "METHOD /path/pattern" → accumulator
    mu        sync.RWMutex
}

type EndpointAccumulator struct {
    Method       string
    PathPattern  string
    Observations int
    LastSeen     time.Time

    RequestShapes  []map[string]FieldInfo  // One per observation
    ResponseShapes map[int][]map[string]FieldInfo // status → shapes

    Timings []int // latency in ms
}

type FieldInfo struct {
    Type     string // "string", "number", "boolean", "object", "array", "null"
    Format   string // "uuid", "datetime", "email", "url", ""
    Required bool
    Values   []string // Sample values (sanitized) for enum detection
}
```

### Type Inference Rules

```go
func inferType(value interface{}) FieldInfo {
    switch v := value.(type) {
    case string:
        fi := FieldInfo{Type: "string"}
        if isUUID(v)     { fi.Format = "uuid" }
        if isDatetime(v) { fi.Format = "datetime" }
        if isEmail(v)    { fi.Format = "email" }
        if isURL(v)      { fi.Format = "url" }
        return fi
    case float64:
        if v == math.Trunc(v) { return FieldInfo{Type: "integer"} }
        return FieldInfo{Type: "number"}
    case bool:
        return FieldInfo{Type: "boolean"}
    case []interface{}:
        return FieldInfo{Type: "array"}
    case map[string]interface{}:
        return FieldInfo{Type: "object"}
    case nil:
        return FieldInfo{Type: "null"}
    }
    return FieldInfo{Type: "unknown"}
}
```

### Path Parameterization

Detect dynamic path segments:

```go
// Observations:
//   GET /api/users/abc-123
//   GET /api/users/def-456
//   GET /api/users/ghi-789
//
// Inferred pattern: GET /api/users/{id}

func detectPathPattern(paths []string) string {
    // Split paths into segments
    // For each position, if all values are different AND match a pattern (UUID, numeric):
    //   → replace with {param}
    // If all values are the same:
    //   → keep literal
}
```

**Parameterization heuristics:**
- Position has > 3 unique values → likely a parameter
- Values match UUID pattern → `{uuid}`
- Values are all numeric → `{id}`
- Values are all different but no pattern → `{slug}`

### Schema Merging (across observations)

When multiple observations exist for the same endpoint:

```go
func mergeSchemas(observations []map[string]FieldInfo) map[string]MergedField {
    result := map[string]MergedField{}
    totalObs := len(observations)

    // Count how often each field appears
    for _, obs := range observations {
        for field, info := range obs {
            if existing, ok := result[field]; ok {
                existing.ObservedCount++
                existing.Types = append(existing.Types, info.Type)
            } else {
                result[field] = MergedField{
                    Info: info,
                    ObservedCount: 1,
                }
            }
        }
    }

    // Field is "required" if present in > 90% of observations
    for field, merged := range result {
        merged.Required = float64(merged.ObservedCount)/float64(totalObs) > 0.9
        result[field] = merged
    }

    return result
}
```

### Auth Pattern Detection

```go
func detectAuthPattern(bodies []NetworkBody) *AuthPattern {
    authHeaders := 0
    totalRequests := len(bodies)
    excludedPaths := []string{}

    for _, b := range bodies {
        if hasAuthHeader(b) {
            authHeaders++
        } else {
            excludedPaths = append(excludedPaths, b.URL)
        }
    }

    if float64(authHeaders)/float64(totalRequests) > 0.5 {
        return &AuthPattern{
            Type:          "bearer",
            Header:        "Authorization",
            ObservedRatio: float64(authHeaders) / float64(totalRequests),
            ExcludedPaths: deduplicate(excludedPaths),
        }
    }
    return nil
}
```

### Sensitive Data Handling

All values in the schema are sanitized:
- Auth tokens → `"[bearer_token]"`
- Email addresses → `"user@example.com"` (generic)
- UUIDs → kept (not sensitive, useful for format detection)
- Passwords → never captured (already redacted by extension)
- API keys → `"[api_key]"`

Only structural information (types, shapes, field names) is retained. Values are only used for format inference, then discarded.

---

## Proving Improvements

### Metrics

| Metric | Without schema inference | With schema inference | Measurement |
|--------|------------------------|---------------------|-------------|
| Agent's API understanding time | 5-10 min (read source/docs) | < 1s (load inferred schema) | Time from "modify this endpoint" to agent's first correct API call |
| Incorrect API calls | 2-5 per feature (guess-and-check) | 0-1 (schema-informed) | Count 4xx responses from agent-generated code |
| Test accuracy | Low (wrong assertions on response shape) | High (assertions match observed shape) | Percentage of generated test assertions that pass on first run |
| Documentation sync | Often outdated | Always current (traffic-derived) | Compare inferred schema against actual API behavior |

### Benchmark: Agent API Accuracy

1. Give agent a task: "Add a new feature that calls the /api/projects endpoint"
2. **Without schema:** Agent reads source code or guesses the API shape
3. **With schema:** Agent calls `get_api_schema(url_filter: "/api/projects")`
4. Measure: number of incorrect API calls, time to first correct implementation

**Target:** 80% reduction in incorrect API calls; 50% faster implementation.

### Benchmark: Schema Accuracy

1. Compare inferred schema against actual OpenAPI spec (if one exists)
2. Measure:
   - **Precision:** % of inferred fields that actually exist
   - **Recall:** % of actual fields that were inferred
   - **Type accuracy:** % of inferred types that match actual types

**Target:** > 95% precision, > 80% recall (limited by which endpoints were actually called), > 90% type accuracy.

### Benchmark: Compound Learning

1. Session 1: Agent observes 5 endpoints → schema has 5 entries
2. Session 2: Agent observes 3 new + 4 existing → schema has 8 entries (existing enriched)
3. Session 5: Schema covers 14 endpoints with high-confidence types

**Target:** Schema coverage grows monotonically; types converge to correct values within 3-5 observations.

---

## Integration with Other Features

| Feature | Relationship |
|---------|-------------|
| `generate_test` | Uses schema for response shape assertions |
| `generate_mocks` | Uses schema to generate type-accurate mock responses |
| `validate_api` (v6) | Compares new responses against inferred schema for contract violations |
| Persistent Memory | Schema persists across sessions, enriched each session |
| `get_test_context` | Schema is the core data source for test context |
| `diagnose_error` | Schema provides "expected" behavior for error diagnosis |

---

## Edge Cases

| Case | Handling |
|------|---------|
| Endpoint never observed | Not in schema. Agent must discover it. |
| Endpoint returns different shapes based on query params | Tracked as conditional fields with `conditional` annotation |
| Binary responses (images, files) | Recorded as `content_type: "image/png"`, no shape inference |
| GraphQL (single endpoint, variable shapes) | Detect `POST /graphql`; parse query field for operation-level schemas |
| Paginated responses | Detect pagination patterns (page/limit, cursor, offset); annotate in schema |
| Rate-limited endpoints | Record 429 responses as a known status; include rate limit info if headers captured |
| Endpoint removed (no longer observed) | Keep in schema with `stale: true` after N sessions without observation |
| Same path, different auth levels (different responses) | Track as separate response variants by status code |

---

## Performance Budget

| Operation | Budget | Rationale |
|-----------|--------|-----------|
| Schema inference (per request) | < 1ms | Just updating accumulators |
| Schema query (`get_api_schema`) | < 50ms | Merge and serialize accumulated data |
| Path parameterization | < 10ms | String comparison across stored paths |
| OpenAPI export | < 100ms | Template rendering |
| Memory for schema data | < 2MB | Reasonable for 200 endpoints × 20 fields each |
