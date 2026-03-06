// Purpose: Package util — shared utility functions for binary detection, JSON responses, timestamps, URLs, and safe goroutines.
// Why: Centralizes cross-cutting helpers to avoid duplication across internal packages.

/*
Package util provides shared utility functions used across internal packages.

Key functions:
  - IsBinaryPayload: detects binary payload formats (MessagePack, CBOR, Protobuf, BSON) via heuristics.
  - WriteJSONResponse: writes a JSON HTTP response with proper content type and status code.
  - ParseTimestamp: parses RFC3339/RFC3339Nano timestamp strings.
  - ExtractURLPath: extracts the path component from a URL string.
  - SafeGo: launches a goroutine with panic recovery for background tasks.
*/
package util
