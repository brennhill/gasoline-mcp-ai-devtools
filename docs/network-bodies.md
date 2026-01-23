---
title: "Network Body Capture"
description: "Capture HTTP request and response bodies for API debugging. See exactly what your app sends and receives without opening DevTools."
keywords: "network body capture, API request debugging, HTTP response body, network request payload, API debugging tool"
permalink: /network-bodies/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "See exactly what your app sends and receives."
toc: true
toc_sticky: true
---

Gasoline captures HTTP request and response bodies, letting AI assistants see the actual data flowing between your app and APIs.

## <i class="fas fa-exchange-alt"></i> What Gets Captured

- <i class="fas fa-arrow-up"></i> **Request bodies** — POST, PUT, PATCH payloads
- <i class="fas fa-arrow-down"></i> **Response bodies** — server responses (JSON, text, etc.)
- <i class="fas fa-heading"></i> **Headers** — request and response headers (auth stripped)
- <i class="fas fa-hashtag"></i> **Status codes** — HTTP status for correlation
- <i class="fas fa-clock"></i> **Timing** — request duration

## <i class="fas fa-tools"></i> MCP Tool: `get_network_bodies`

Query captured bodies with filters:

| Filter | Description |
|--------|-------------|
| <i class="fas fa-link"></i> URL | Match requests by URL pattern |
| <i class="fas fa-tag"></i> Method | Filter by HTTP method (GET, POST, etc.) |
| <i class="fas fa-hashtag"></i> Status | Filter by response status code |

## <i class="fas fa-shield-alt"></i> Privacy

- **Auth headers stripped** — tokens never appear in logs
- **Localhost only** — captured data stays on your machine
- **On-demand** — only matching requests are captured

## <i class="fas fa-fire-alt"></i> Use Cases

- _"What did the server actually return?"_ — debug API integration
- Verify request payloads match expected schemas
- Identify frontend/API response mismatches
- Trace auth and authorization failures
