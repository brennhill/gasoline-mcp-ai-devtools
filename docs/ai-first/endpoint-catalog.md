# Endpoint Catalog

## Status: Implementation Ready

---

## Overview

A server-side aggregation of observed API endpoints into a discoverable catalog. No type inference, no schema learning — just "here are the APIs this app talks to" with basic metadata.

**Philosophy check:** AI agents can't discover endpoints they haven't observed. This provides discovery (map of the API surface) without interpretation (no types, no schemas, no "this endpoint is slow").

---

## MCP Tool Definition

### `get_endpoint_catalog`

**Description:** List every API endpoint the app has called, with call counts, status codes, and latency. Useful for understanding the API surface before debugging.

**Input Schema:**

```json
{
  "type": "object",
  "properties": {
    "method": {
      "type": "string",
      "description": "Filter by HTTP method (GET, POST, etc.)"
    },
    "status": {
      "type": "number",
      "description": "Filter to endpoints that have returned this status code"
    },
    "min_calls": {
      "type": "number",
      "description": "Only show endpoints with at least this many calls"
    }
  }
}
```

**Response:**

```json
{
  "endpoints": [
    {
      "method": "GET",
      "path": "/api/users",
      "status_codes": {"200": 45, "401": 2},
      "call_count": 47,
      "avg_latency_ms": 142,
      "last_seen": "2026-01-23T10:30:00Z"
    },
    {
      "method": "POST",
      "path": "/api/users",
      "status_codes": {"201": 2, "422": 1},
      "call_count": 3,
      "avg_latency_ms": 289,
      "last_seen": "2026-01-23T10:28:00Z"
    },
    {
      "method": "GET",
      "path": "/api/projects/:id",
      "status_codes": {"200": 10, "404": 2},
      "call_count": 12,
      "avg_latency_ms": 95,
      "last_seen": "2026-01-23T10:31:00Z"
    }
  ],
  "total_endpoints": 14,
  "observation_window": "45m02s"
}
```

---

## Server Implementation

### Types

```go
// EndpointStats tracks aggregate stats for a single logical endpoint
type EndpointStats struct {
    Method       string         `json:"method"`
    PathTemplate string         `json:"path"`
    StatusCodes  map[int]int    `json:"status_codes"`
    CallCount    int            `json:"call_count"`
    TotalLatency int64          `json:"-"` // internal: sum of durations in ms
    AvgLatencyMs int            `json:"avg_latency_ms"`
    LastSeen     time.Time      `json:"last_seen"`
    FirstSeen    time.Time      `json:"-"` // internal: for observation window
}

// EndpointCatalogFilter defines filtering criteria
type EndpointCatalogFilter struct {
    Method   string
    Status   int
    MinCalls int
}
```

### Catalog on V4Server

The endpoint catalog lives on `V4Server` alongside existing buffers:

```go
type V4Server struct {
    // ... existing fields ...

    // Endpoint catalog
    endpointCatalog map[string]*EndpointStats // key: "METHOD /path/template"
    catalogFirstSeen time.Time
}
```

### Aggregation Logic

The catalog aggregates **on ingest** — when `AddNetworkBodies` is called, each body is also fed to the catalog:

```go
func (v *V4Server) updateEndpointCatalog(body NetworkBody) {
    pathTemplate := parameterizePath(body.URL)
    key := body.Method + " " + pathTemplate

    stats, exists := v.endpointCatalog[key]
    if !exists {
        // Enforce 200 endpoint cap with LRU eviction
        if len(v.endpointCatalog) >= maxEndpoints {
            v.evictOldestEndpoint()
        }
        stats = &EndpointStats{
            Method:       body.Method,
            PathTemplate: pathTemplate,
            StatusCodes:  make(map[int]int),
            FirstSeen:    time.Now(),
        }
        v.endpointCatalog[key] = stats
    }

    stats.CallCount++
    stats.StatusCodes[body.Status]++
    stats.TotalLatency += int64(body.Duration)
    stats.AvgLatencyMs = int(stats.TotalLatency / int64(stats.CallCount))
    stats.LastSeen = time.Now()
}
```

### Path Parameterization

```go
func parameterizePath(rawURL string) string
```

Rules (applied per path segment):

| Pattern | Example | Result |
|---------|---------|--------|
| UUID v4 | `550e8400-e29b-41d4-a716-446655440000` | `:id` |
| Numeric (1+ digits) | `42`, `12345` | `:id` |
| MongoDB ObjectId (24 hex chars) | `507f1f77bcf86cd799439011` | `:id` |
| Short hex (8+ chars, all hex) | `a1b2c3d4e5` | `:id` |
| Everything else | `users`, `v2`, `api` | kept as-is |

```
Input:  https://example.com/api/users/550e8400-e29b-41d4-a716-446655440000
Output: /api/users/:id

Input:  https://example.com/api/projects/42/tasks/17
Output: /api/projects/:id/tasks/:id

Input:  https://example.com/api/v2/items?page=3
Output: /api/v2/items

Input:  https://example.com/graphql
Output: /graphql
```

Query strings are stripped. Only the path is kept.

### GraphQL Handling

For requests to known GraphQL paths (`/graphql`, `/api/graphql`, `*/graphql`):

```go
func extractGraphQLOperation(method, url, requestBody string) (string, string)
```

- Parse `operationName` from JSON request body
- Return method as `QUERY` or `MUTATION` (from body's `query` field prefix detection)
- Path template becomes: `QUERY GetUsers` or `MUTATION CreateUser`
- Fallback: if no operationName and body parse fails, use raw path `/graphql`

### Memory & Limits

| Constraint | Value |
|-----------|-------|
| Max endpoints | 200 |
| Memory per entry | ~200 bytes |
| Total memory budget | ~40KB |
| Eviction strategy | Oldest `LastSeen` when at cap |

### Constants

```go
const (
    maxEndpoints = 200
)
```

---

## MCP Handler

```go
func (h *MCPHandlerV4) toolGetEndpointCatalog(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
```

Tool is added to `v4ToolsList()` and dispatched in `handleV4ToolCall()`.

Response is sorted by `call_count` descending (most-called endpoints first).

---

## HTTP Endpoint

No new HTTP endpoint needed. The catalog aggregates data already received via `POST /network-bodies`.

---

## Test Cases

### Path Parameterization (`TestParameterizePath`)

| Input URL | Expected Output |
|-----------|-----------------|
| `https://example.com/api/users` | `/api/users` |
| `https://example.com/api/users/123` | `/api/users/:id` |
| `https://example.com/api/users/550e8400-e29b-41d4-a716-446655440000` | `/api/users/:id` |
| `https://example.com/api/projects/42/tasks/17` | `/api/projects/:id/tasks/:id` |
| `https://example.com/api/v2/items` | `/api/v2/items` |
| `https://example.com/api/users?page=2&limit=10` | `/api/users` |
| `https://example.com/api/users/507f1f77bcf86cd799439011` | `/api/users/:id` |
| `https://example.com/api/items/abc` | `/api/items/abc` (3 chars, not hex-like) |
| `https://example.com/` | `/` |
| `https://example.com` | `/` |

### Catalog Aggregation (`TestEndpointCatalogAggregation`)

- Add 3 bodies with same method+path, different status codes → single entry, call_count=3, all statuses in map
- Add bodies with same path but different methods → separate entries
- Verify avg_latency_ms calculated correctly
- Verify last_seen updates on each addition

### Catalog Cap (`TestEndpointCatalogCap`)

- Add 201 unique endpoints → catalog has exactly 200
- The evicted entry is the one with oldest `LastSeen`
- Add a body to an existing endpoint after cap → updates existing, doesn't create new

### Catalog Filtering (`TestEndpointCatalogFilter`)

- Filter by method: only GET endpoints returned
- Filter by status: only endpoints that have seen status 500
- Filter by min_calls: only endpoints with call_count >= 10
- Combined filters: method=POST AND status=422

### GraphQL (`TestGraphQLOperationExtraction`)

- Body with `operationName: "GetUsers"` → path = `QUERY GetUsers`
- Body with mutation keyword → path = `MUTATION CreateUser`
- Body with no operationName → path = `/graphql`
- Malformed/empty body → path = `/graphql`
- Non-GraphQL URL with JSON body → normal path parameterization

### MCP Tool Response (`TestGetEndpointCatalogTool`)

- Empty catalog → returns `{"endpoints": [], "total_endpoints": 0, ...}`
- Populated catalog → sorted by call_count desc
- observation_window calculated from first ingest to now

### Concurrency (`TestEndpointCatalogConcurrency`)

- 10 goroutines adding bodies simultaneously → no race, no panic
- Concurrent reads and writes → consistent results

---

## What We Don't Do

| Feature | Why Not |
|---------|---------|
| Request/response type inference | AI reads JSON natively |
| Body schemas | Already in `get_network_bodies` |
| OpenAPI generation | Out of scope |
| Authentication detection | Too heuristic; show status codes |
| Rate/trend analysis | Interpretation; AI does this |

---

## SDK Replacement Angle

### What This Replaces

| Traditional Tool | What It Does | Gasoline Equivalent |
|-----------------|--------------|---------------------|
| Sentry APM / Transaction traces | Track which endpoints are called, latency, error rates | `get_endpoint_catalog` — same data, zero SDK |
| DataDog APM Service Map | Map of services and their dependencies | Endpoint catalog + WebSocket status = full communication map |
| Postman Collections (auto-generated) | List of endpoints with examples | Catalog provides the list; `get_network_bodies` provides examples |
| New Relic Browser Agent | Client-side performance per endpoint | Catalog provides latency; perf capture provides Web Vitals |
| LogRocket / FullStory API tracking | Which network calls happen during user sessions | Catalog + enhanced actions = complete session picture |

### Key Differentiators

1. **Zero code changes.** No SDK installation, no initialization, no build step. The browser extension captures everything.
2. **No data leaves localhost.** Unlike Sentry/DataDog/LogRocket which send data to their cloud, Gasoline keeps everything local.
3. **AI-native output.** SDKs produce dashboards for humans to interpret. Gasoline produces structured JSON for AI to reason about.
4. **No sampling.** APM tools sample 1-10% of transactions. Gasoline captures every request in the development session.
5. **No production overhead.** APM SDKs add latency and bundle size to production apps. Gasoline only runs during development.

### Positioning

> "The endpoint catalog you'd get from a week of Sentry APM setup — in zero lines of code, with zero data leaving your machine, optimized for your AI coding assistant instead of a dashboard."

### Ecosystem Value

The endpoint catalog is the foundation for standard format exports:
- **HAR Export:** Feed the catalog + bodies into any HTTP analysis tool
- **OpenAPI Skeleton:** Generate a draft OpenAPI spec from observed endpoints (external tool, not Gasoline)
- **Playwright API mocking:** Generate MSW/Playwright route handlers from observed traffic

---

## Extension Changes

**None.** The catalog aggregates data already sent via `POST /network-bodies`.
