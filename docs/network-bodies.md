---
title: "Network Body Capture"
description: "Capture HTTP request and response bodies for API debugging. See exactly what your app sends and receives without opening DevTools."
keywords: "network body capture, API request debugging, HTTP response body, network request payload, API debugging tool"
permalink: /network-bodies/
toc: true
toc_sticky: true
---

Gasoline captures HTTP request and response bodies, letting AI assistants see the actual data flowing between your app and APIs.

## What Gets Captured

- **Request bodies** — POST, PUT, PATCH payloads
- **Response bodies** — server responses (JSON, text, etc.)
- **Headers** — request and response headers (auth headers stripped)
- **Status codes** — HTTP status for correlation
- **Timing** — request duration

## MCP Tool: `get_network_bodies`

Query captured network bodies with filters:

| Filter | Description |
|--------|-------------|
| URL | Match requests by URL pattern |
| Method | Filter by HTTP method (GET, POST, etc.) |
| Status | Filter by response status code |

## Privacy

- **Authorization headers are stripped** — tokens never appear in logs
- **Localhost only** — captured data stays on your machine
- **On-demand** — not all requests are captured, only those matching your configuration

## Use Cases

- Debug API integration issues ("What did the server actually return?")
- Verify request payloads match expected schemas
- Identify mismatches between frontend expectations and API responses
- Trace authentication and authorization failures
