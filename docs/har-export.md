---
title: "HAR Export"
description: "Export captured network traffic as HTTP Archive (HAR) files. Standard format compatible with Chrome DevTools, Charles Proxy, and any HAR viewer for sharing and analysis."
keywords: "HAR export, HTTP Archive, network traffic export, HAR file, traffic recording, network debugging, request export, API traffic"
permalink: /har-export/
header:
  overlay_image: /assets/images/hero-banner.png
  overlay_filter: 0.85
  excerpt: "Export your network traffic. Standard format, any viewer."
toc: true
toc_sticky: true
---

Gasoline exports captured network traffic as HAR (HTTP Archive) files — the standard format supported by Chrome DevTools, Firefox, Charles Proxy, and dozens of analysis tools.

## <i class="fas fa-exclamation-circle"></i> The Problem

You captured a complex bug involving multiple API calls. Now you need to:
- Share the network trace with a backend developer
- Attach it to a bug report
- Analyze it in a specialized tool
- Compare it against a known-good recording

Browser DevTools can export HAR, but only if you had the Network tab open at the time. If you missed it, the data is gone. Gasoline captures everything automatically, and you can export at any time.

## <i class="fas fa-terminal"></i> Usage

```json
// Export all captured network traffic as HAR
{ "tool": "generate", "arguments": { "format": "har" } }

// Export only requests to a specific API
{ "tool": "generate", "arguments": {
  "format": "har",
  "url": "/api/orders"
} }

// Export only failed requests
{ "tool": "generate", "arguments": {
  "format": "har",
  "status_min": 400
} }

// Filter by method
{ "tool": "generate", "arguments": {
  "format": "har",
  "method": "POST"
} }

// Save directly to a file
{ "tool": "generate", "arguments": {
  "format": "har",
  "save_to": "./debug/session-traffic.har"
} }
```

## <i class="fas fa-file-code"></i> HAR Format

The exported file follows the [HAR 1.2 specification](http://www.softwareishard.com/blog/har-12-spec/):

```json
{
  "log": {
    "version": "1.2",
    "creator": { "name": "Gasoline", "version": "4.8.0" },
    "entries": [
      {
        "startedDateTime": "2024-01-15T10:30:00.000Z",
        "request": {
          "method": "POST",
          "url": "https://api.example.com/orders",
          "headers": [...],
          "postData": { "mimeType": "application/json", "text": "{...}" }
        },
        "response": {
          "status": 201,
          "headers": [...],
          "content": { "mimeType": "application/json", "text": "{...}" }
        },
        "time": 245
      }
    ]
  }
}
```

## <i class="fas fa-tools"></i> Compatible Tools

HAR files can be opened in:

| Tool | Use Case |
|------|----------|
| Chrome DevTools (Network tab) | Import and replay |
| Firefox DevTools | Import and analyze |
| Charles Proxy | Deep request inspection |
| Fiddler | Windows traffic analysis |
| [HAR Viewer](http://www.softwareishard.com/har/viewer/) | Browser-based visualization |
| Postman | Import as collection |

## <i class="fas fa-filter"></i> Filtering Options

Combine filters to export exactly what you need:

| Filter | Description |
|--------|-------------|
| `url` | Substring match on request URL |
| `method` | HTTP method (GET, POST, PUT, DELETE) |
| `status_min` | Minimum response status code |
| `status_max` | Maximum response status code |

Example: Export only failed POST requests to the payments API:

```json
{ "tool": "generate", "arguments": {
  "format": "har",
  "url": "/api/payments",
  "method": "POST",
  "status_min": 400
} }
```

## <i class="fas fa-shield-alt"></i> Privacy

- Auth headers (`Authorization`, `Cookie`) are automatically stripped from exports
- Sensitive request/response bodies can be filtered using URL patterns
- HAR files are written locally — never transmitted over the network

## <i class="fas fa-link"></i> Related

- [Network Bodies](/network-bodies/) — Full request/response payload capture
- [API Schema Inference](/api-schema/) — Auto-discover API structure from traffic
- [Reproduction Scripts](/reproduction-scripts/) — Generate runnable scripts instead of passive recordings
