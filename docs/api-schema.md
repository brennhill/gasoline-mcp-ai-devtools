---
title: "API Schema Inference"
description: "Automatically discover your API schema from live network traffic. Gasoline observes HTTP requests and infers endpoints, methods, status codes, and response shapes — no OpenAPI file required."
keywords: "API schema inference, OpenAPI generation, API discovery, network traffic analysis, endpoint detection, API documentation, response shape"
permalink: /api-schema/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "No OpenAPI file? No problem. Gasoline infers it from traffic."
toc: true
toc_sticky: true
---

Gasoline watches your app's network traffic and automatically builds an API schema — endpoints, methods, status codes, and response shapes — without any manual configuration.

## <i class="fas fa-exclamation-circle"></i> The Problem

Your AI assistant is debugging a failed API call, but it doesn't know what endpoints your app uses, what they expect, or what they return. Without an OpenAPI spec (which most projects don't maintain), the AI is flying blind.

It could read your backend source code, but that's slow and may not reflect the actual runtime behavior. What if the frontend calls third-party APIs too? What if the backend has middleware that transforms responses?

Gasoline solves this by observing what actually happens on the wire.

## <i class="fas fa-project-diagram"></i> How It Works

1. <i class="fas fa-globe"></i> The extension captures every HTTP request/response as you use your app
2. <i class="fas fa-brain"></i> The server groups requests by endpoint pattern (normalizing path parameters)
3. <i class="fas fa-shapes"></i> Response shapes are inferred from JSON bodies
4. <i class="fas fa-file-code"></i> Your AI queries the schema via `analyze` with `target: "api"`

The schema improves with every request — the more you use your app, the more complete it becomes.

## <i class="fas fa-terminal"></i> Usage

```json
// Get inferred API schema
{ "tool": "analyze", "arguments": { "target": "api" } }

// Filter to specific endpoints
{ "tool": "analyze", "arguments": { "target": "api", "url_filter": "/api/users" } }

// Only show endpoints observed 3+ times
{ "tool": "analyze", "arguments": { "target": "api", "min_observations": 3 } }

// Get OpenAPI stub format
{ "tool": "analyze", "arguments": { "target": "api", "format": "openapi_stub" } }
```

## <i class="fas fa-sitemap"></i> What Gets Inferred

For each endpoint, Gasoline tracks:

| Field | Description |
|-------|-------------|
| **Path pattern** | URL with path parameters normalized (e.g., `/api/users/:id`) |
| **Methods** | HTTP methods observed (GET, POST, PUT, DELETE) |
| **Status codes** | Response codes seen (200, 201, 404, 500) |
| **Response shape** | JSON structure with types (string, number, array, object) |
| **Observation count** | How many times this endpoint was called |
| **Content types** | Response content types (application/json, text/html) |

## <i class="fas fa-code"></i> Output Formats

### Gasoline Format (default)

Compact, AI-friendly format optimized for context windows:

```json
{
  "endpoints": [
    {
      "path": "/api/users/:id",
      "methods": ["GET", "PUT"],
      "statuses": [200, 404],
      "observations": 8,
      "responseShape": {
        "id": "number",
        "name": "string",
        "email": "string",
        "roles": ["string"]
      }
    }
  ]
}
```

### OpenAPI Stub

Standard OpenAPI 3.0 structure for integration with other tools:

```json
{
  "openapi": "3.0.0",
  "paths": {
    "/api/users/{id}": {
      "get": {
        "responses": {
          "200": { "description": "OK" },
          "404": { "description": "Not Found" }
        }
      }
    }
  }
}
```

## <i class="fas fa-search"></i> What Your AI Can Do With This

- **Understand the API without docs** — "Your app uses 12 endpoints. The `/api/orders` endpoint returns paginated results with `items[]` and `totalCount`."
- **Debug mismatches** — "The frontend expects `user.firstName` but the API returns `user.first_name`. This is likely the cause of the undefined error."
- **Suggest missing error handling** — "The `/api/payments` endpoint has returned 500 errors 3 times. Your frontend doesn't handle this status code."
- **Generate types** — "Based on observed responses, here's a TypeScript interface for the User endpoint."

## <i class="fas fa-filter"></i> Path Normalization

Gasoline automatically normalizes dynamic path segments:

| Observed URLs | Inferred Pattern |
|--------------|-----------------|
| `/api/users/123`, `/api/users/456` | `/api/users/:id` |
| `/api/orders/abc-def/items` | `/api/orders/:id/items` |

This prevents endpoint explosion from dynamic routes.

## <i class="fas fa-link"></i> Related

- [Network Bodies](/network-bodies/) — Full request/response payload capture
- [HAR Export](/har-export/) — Export raw traffic as HTTP Archive
- [Test Generation](/generate-test/) — Generate tests with API assertions
