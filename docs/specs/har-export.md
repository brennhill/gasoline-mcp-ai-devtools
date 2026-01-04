# HAR Export

## Status: Implementation Ready

---

## Overview

Export captured network traffic as a [HAR (HTTP Archive) 1.2](http://www.softwareishard.com/blog/har-12-spec/) file. HAR is the universal standard for HTTP traffic analysis, supported by Chrome DevTools, Firefox, Postman, Charles Proxy, Fiddler, and dozens of other tools.

**Philosophy check:** Pure format conversion of already-captured data. No interpretation, no filtering, no enrichment. The HAR file contains exactly what the extension captured — just in a standard format.

---

## MCP Tool Definition

### `export_har`

**Description:** Export captured network traffic as a HAR (HTTP Archive) file. The HAR format is supported by Chrome DevTools, Postman, Charles Proxy, and other HTTP analysis tools.

**Input Schema:**

```json
{
  "type": "object",
  "properties": {
    "url": {
      "type": "string",
      "description": "Filter to requests matching this URL substring"
    },
    "method": {
      "type": "string",
      "description": "Filter to this HTTP method (GET, POST, etc.)"
    },
    "status_min": {
      "type": "number",
      "description": "Minimum status code to include"
    },
    "status_max": {
      "type": "number",
      "description": "Maximum status code to include"
    },
    "save_to": {
      "type": "string",
      "description": "File path to save the HAR file. If omitted, returns JSON in response."
    }
  }
}
```

**Response (when `save_to` is omitted):**

```json
{
  "log": {
    "version": "1.2",
    "creator": {
      "name": "Gasoline",
      "version": "4.0.0"
    },
    "entries": [
      {
        "startedDateTime": "2026-01-23T10:30:00.000Z",
        "time": 142,
        "request": {
          "method": "POST",
          "url": "https://example.com/api/users",
          "httpVersion": "HTTP/2.0",
          "headers": [],
          "queryString": [],
          "postData": {
            "mimeType": "application/json",
            "text": "{\"name\": \"Alice\"}"
          },
          "headersSize": -1,
          "bodySize": 17
        },
        "response": {
          "status": 201,
          "statusText": "Created",
          "httpVersion": "HTTP/2.0",
          "headers": [],
          "content": {
            "size": 45,
            "mimeType": "application/json",
            "text": "{\"id\": 42, \"name\": \"Alice\"}"
          },
          "redirectURL": "",
          "headersSize": -1,
          "bodySize": 45
        },
        "cache": {},
        "timings": {
          "send": -1,
          "wait": 142,
          "receive": -1
        }
      }
    ]
  }
}
```

**Response (when `save_to` is provided):**

```json
{
  "saved_to": "/tmp/session-2026-01-23.har",
  "entries_count": 87,
  "file_size_bytes": 24500
}
```

---

## HAR 1.2 Mapping

### What We Have → HAR Fields

| NetworkBody Field | HAR Field | Notes |
|-------------------|-----------|-------|
| `ts` | `entry.startedDateTime` | ISO 8601 format |
| `duration` | `entry.time`, `timings.wait` | milliseconds |
| `method` | `request.method` | |
| `url` | `request.url` | Full URL |
| `requestBody` | `request.postData.text` | Only if present |
| `contentType` | `request.postData.mimeType` | Only for request |
| `status` | `response.status` | |
| — | `response.statusText` | Derived from status code |
| `responseBody` | `response.content.text` | |
| `contentType` | `response.content.mimeType` | Content-Type header |
| `requestTruncated` | `request.comment` | "Body truncated at 8KB" |
| `responseTruncated` | `response.comment` | "Body truncated at 16KB" |

### What We Don't Have (HAR fields set to -1 or omitted)

| HAR Field | Why Missing | Value |
|-----------|-------------|-------|
| `request.headers` | Extension captures bodies, not all headers | `[]` |
| `response.headers` | Same | `[]` |
| `request.cookies` | Privacy — never captured | `[]` |
| `response.cookies` | Privacy — never captured | `[]` |
| `timings.dns` | Not available from fetch intercept | `-1` |
| `timings.connect` | Not available from fetch intercept | `-1` |
| `timings.ssl` | Not available from fetch intercept | `-1` |
| `timings.send` | Not available from fetch intercept | `-1` |
| `timings.receive` | Not available from fetch intercept | `-1` |

### Status Text Mapping

```go
var statusTexts = map[int]string{
    200: "OK",
    201: "Created",
    204: "No Content",
    301: "Moved Permanently",
    302: "Found",
    304: "Not Modified",
    400: "Bad Request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not Found",
    405: "Method Not Allowed",
    409: "Conflict",
    422: "Unprocessable Entity",
    429: "Too Many Requests",
    500: "Internal Server Error",
    502: "Bad Gateway",
    503: "Service Unavailable",
}
```

Use Go's `http.StatusText()` for the full mapping.

---

## Server Implementation

### Function

```go
func (v *V4Server) ExportHAR(filter NetworkBodyFilter) HARLog
```

### Types

```go
type HARLog struct {
    Log HARLogContent `json:"log"`
}

type HARLogContent struct {
    Version string     `json:"version"`
    Creator HARCreator `json:"creator"`
    Entries []HAREntry `json:"entries"`
}

type HARCreator struct {
    Name    string `json:"name"`
    Version string `json:"version"`
}

type HAREntry struct {
    StartedDateTime string      `json:"startedDateTime"`
    Time            int         `json:"time"`
    Request         HARRequest  `json:"request"`
    Response        HARResponse `json:"response"`
    Cache           struct{}    `json:"cache"`
    Timings         HARTimings  `json:"timings"`
    Comment         string      `json:"comment,omitempty"`
}

type HARRequest struct {
    Method      string         `json:"method"`
    URL         string         `json:"url"`
    HTTPVersion string         `json:"httpVersion"`
    Headers     []HARHeader    `json:"headers"`
    QueryString []HARQuery     `json:"queryString"`
    PostData    *HARPostData   `json:"postData,omitempty"`
    HeadersSize int            `json:"headersSize"`
    BodySize    int            `json:"bodySize"`
    Comment     string         `json:"comment,omitempty"`
}

type HARResponse struct {
    Status      int         `json:"status"`
    StatusText  string      `json:"statusText"`
    HTTPVersion string      `json:"httpVersion"`
    Headers     []HARHeader `json:"headers"`
    Content     HARContent  `json:"content"`
    RedirectURL string      `json:"redirectURL"`
    HeadersSize int         `json:"headersSize"`
    BodySize    int         `json:"bodySize"`
    Comment     string      `json:"comment,omitempty"`
}

type HARContent struct {
    Size     int    `json:"size"`
    MimeType string `json:"mimeType"`
    Text     string `json:"text,omitempty"`
}

type HARPostData struct {
    MimeType string `json:"mimeType"`
    Text     string `json:"text"`
}

type HARTimings struct {
    Send    int `json:"send"`
    Wait    int `json:"wait"`
    Receive int `json:"receive"`
}

type HARHeader struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

type HARQuery struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}
```

### Conversion Logic

```go
func networkBodyToHAREntry(body NetworkBody) HAREntry {
    entry := HAREntry{
        StartedDateTime: body.Timestamp,
        Time:            body.Duration,
        Request: HARRequest{
            Method:      body.Method,
            URL:         body.URL,
            HTTPVersion: "HTTP/2.0",
            Headers:     []HARHeader{},
            QueryString: parseQueryString(body.URL),
            HeadersSize: -1,
            BodySize:    len(body.RequestBody),
        },
        Response: HARResponse{
            Status:      body.Status,
            StatusText:  http.StatusText(body.Status),
            HTTPVersion: "HTTP/2.0",
            Headers:     []HARHeader{},
            Content: HARContent{
                Size:     len(body.ResponseBody),
                MimeType: body.ContentType,
                Text:     body.ResponseBody,
            },
            RedirectURL: "",
            HeadersSize: -1,
            BodySize:    len(body.ResponseBody),
        },
        Timings: HARTimings{
            Send:    -1,
            Wait:    body.Duration,
            Receive: -1,
        },
    }

    if body.RequestBody != "" {
        entry.Request.PostData = &HARPostData{
            MimeType: body.ContentType,
            Text:     body.RequestBody,
        }
    }

    if body.RequestTruncated {
        entry.Request.Comment = "Body truncated at 8KB by Gasoline"
    }
    if body.ResponseTruncated {
        entry.Response.Comment = "Body truncated at 16KB by Gasoline"
    }

    return entry
}
```

### File Save

When `save_to` is provided:
1. Generate the HAR JSON
2. Write to the specified path using `os.WriteFile`
3. Return the file path, entry count, and file size
4. If the path is not writable, return an MCP error

Security: Only write to paths under the current working directory or `/tmp`. Reject absolute paths outside these locations.

---

## MCP Handler

```go
func (h *MCPHandlerV4) toolExportHAR(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse
```

Added to `v4ToolsList()` and dispatched in `handleV4ToolCall()`.

---

## Test Cases

### Conversion (`TestNetworkBodyToHAREntry`)

| Test | Input | Expected |
|------|-------|----------|
| Basic GET | `{method: "GET", url: "...", status: 200, duration: 100}` | Valid HAR entry with correct fields |
| POST with body | `{method: "POST", requestBody: "{...}", ...}` | `postData` present with correct mimeType |
| No body | `{method: "GET", requestBody: ""}` | `postData` omitted (nil) |
| Truncated request | `{requestTruncated: true}` | Comment on request |
| Truncated response | `{responseTruncated: true}` | Comment on response |
| Query string parsing | URL: `https://x.com/api?foo=bar&baz=1` | `queryString` has 2 entries |
| Unknown status | `{status: 999}` | statusText is empty string |

### Full Export (`TestExportHAR`)

| Test | Setup | Expected |
|------|-------|----------|
| Empty | No network bodies | `{"log": {"entries": [], ...}}` |
| Multiple entries | 3 bodies | 3 HAR entries, correct order (chronological) |
| With filter | Filter by method=POST | Only POST entries in output |
| HAR validity | Generated output | Passes HAR 1.2 schema validation |
| Creator field | Any export | `name: "Gasoline"`, `version: "4.0.0"` |

### File Save (`TestExportHARToFile`)

| Test | Setup | Expected |
|------|-------|----------|
| Save to /tmp | `save_to: "/tmp/test.har"` | File written, response has path + size |
| Invalid path | `save_to: "/root/nope.har"` | MCP error response |
| Path traversal | `save_to: "../../etc/passwd"` | Rejected |

### MCP Integration (`TestExportHARTool`)

| Test | Setup | Expected |
|------|-------|----------|
| No save_to | Call tool without save_to | Full HAR JSON in response |
| With save_to | Call tool with save_to | File saved, summary in response |

---

## SDK Replacement Angle

### What This Replaces

| Traditional Tool | What It Does | Gasoline Equivalent |
|-----------------|--------------|---------------------|
| Chrome DevTools "Save as HAR" | Manual export from Network tab | `export_har` — automated, filterable, AI-callable |
| Charles Proxy / Fiddler recording | Proxy-based traffic capture + HAR export | Same capture, same export, no proxy setup |
| `har-recorder` npm package | Programmatic HAR generation in tests | Export from real browser session, not synthetic |
| Playwright `page.on('response')` + HAR | Test framework network recording | Captures real dev session, not just test runs |
| Postman history export | Export request history as collection | HAR is more standard and widely supported |

### Key Differentiators

1. **No proxy setup.** Charles/Fiddler require proxy configuration and certificate installation. Gasoline captures from within the browser.
2. **AI-triggered.** The AI agent can export traffic at any point during debugging — no manual "start recording" step.
3. **Filterable.** Export only 500-status requests, or only POST to `/api/checkout`. SDKs export everything or nothing.
4. **Session-aware.** HAR includes only the development session's traffic, not background noise from other tabs.
5. **Composable.** Feed the HAR into any tool: import into Postman, replay with `har-replay`, analyze with `har-analyzer`, diff with `har-diff`.

### Ecosystem Integrations

| Tool | How It Uses Gasoline HAR |
|------|-------------------------|
| Postman | Import HAR → auto-generate collection with examples |
| Playwright | `routeFromHAR()` → replay captured responses in tests |
| `har-to-openapi` | Generate OpenAPI spec draft from observed traffic |
| `har-diff` | Compare traffic before/after code changes |
| CI/CD | Archive HAR files as build artifacts for regression analysis |
| `mitmproxy` | Import HAR for traffic replay and modification |

---

## Extension Changes

**None.** HAR export converts existing `NetworkBody` data server-side.

---

## Implementation Notes

- All changes in `cmd/dev-console/v4.go` (types + conversion) and MCP handler
- Approximately 150 lines Go (types are verbose but trivial)
- Query string parsing uses `net/url` from stdlib
- File writing uses `os.WriteFile` (stdlib)
- No new dependencies
