---
title: "Network Body Capture"
description: "Capture HTTP request and response bodies for API debugging. See exactly what your app sends and receives without opening DevTools."
keywords: "network body capture, API request debugging, HTTP response body, network request payload, API debugging tool, MCP network"
permalink: /network-bodies/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "See exactly what your app sends and receives."
toc: true
toc_sticky: true
---

Gasoline captures HTTP request and response bodies, letting AI assistants see the actual data flowing between your app and APIs.

## <i class="fas fa-exclamation-circle"></i> The Problem

API debugging with AI assistants is painful. You see a 400 error, but without the response body your AI is guessing. You have to open DevTools, find the request, copy the response, paste it into chat — and repeat for every API issue.

With network body capture, your AI reads the payloads directly.

## <i class="fas fa-exchange-alt"></i> What Gets Captured

For every network request, Gasoline records:

| Field | Description |
|-------|-------------|
| <i class="fas fa-arrow-up"></i> Request body | POST/PUT/PATCH payloads (up to 8KB) |
| <i class="fas fa-arrow-down"></i> Response body | Server responses (up to 16KB) |
| <i class="fas fa-heading"></i> Headers | Request and response headers (auth stripped) |
| <i class="fas fa-hashtag"></i> Status code | 200, 401, 500, etc. |
| <i class="fas fa-file-alt"></i> Content type | JSON, HTML, text, etc. |
| <i class="fas fa-clock"></i> Duration | Request timing in milliseconds |

## <i class="fas fa-filter"></i> Filtering

Your AI can query exactly the requests it needs:

```json
// Find failed auth requests
{
  "url": "/api/auth",
  "status_min": 400,
  "status_max": 499
}

// Find slow POST requests
{
  "method": "POST",
  "limit": 5
}

// Find requests to a specific endpoint
{
  "url": "users/profile"
}
```

## <i class="fas fa-tools"></i> MCP Tool: `get_network_bodies`

| Parameter | Description |
|-----------|-------------|
| <i class="fas fa-link"></i> `url` | Filter by URL substring |
| <i class="fas fa-tag"></i> `method` | Filter by HTTP method (GET, POST, etc.) |
| <i class="fas fa-hashtag"></i> `status_min` | Minimum status code |
| <i class="fas fa-hashtag"></i> `status_max` | Maximum status code |
| <i class="fas fa-list-ol"></i> `limit` | Max entries to return (default: 20) |

## <i class="fas fa-file-code"></i> Example Response

```json
{
  "url": "https://api.example.com/users",
  "method": "POST",
  "status": 422,
  "contentType": "application/json",
  "duration": 145,
  "requestBody": "{\"email\":\"invalid\",\"name\":\"\"}",
  "responseBody": "{\"errors\":{\"email\":\"invalid format\",\"name\":\"required\"}}"
}
```

Your AI immediately sees the validation errors without you lifting a finger.

## <i class="fas fa-ruler"></i> Size Limits & Safety

| Limit | Value |
|-------|-------|
| Request body cap | 8KB (larger payloads truncated) |
| Response body cap | 16KB |
| Buffer size | 100 recent requests |
| Total memory budget | 8MB |
| Auth headers | Always stripped |

## <i class="fas fa-shield-alt"></i> Privacy

- **Auth headers stripped** — tokens never appear in logs
- **Localhost only** — captured data stays on your machine
- **Bounded buffers** — old requests evicted, never unbounded growth

## <i class="fas fa-fire-alt"></i> Use Cases

### API Validation Errors

> "My form submission is failing."

Your AI sees the exact validation errors in the response body and can fix the request payload.

### Authentication Issues

> "I'm getting 401s."

Your AI inspects the request to see what's missing and the response for error details.

### Data Format Mismatches

> "The API returns data but the UI is empty."

Your AI compares the response structure to what your code expects — spotting field name changes, missing properties, or type mismatches.

### Debugging Third-Party APIs

> "The Stripe webhook isn't working."

Your AI sees both what Stripe sent and how your server responded, identifying the disconnect.
