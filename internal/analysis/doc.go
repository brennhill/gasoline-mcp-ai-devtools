// doc.go — Package documentation for browser telemetry analysis.

// Package analysis provides intelligent analysis of captured browser telemetry.
//
// Features:
//   - API schema inference from network traffic (endpoint detection, parameter analysis)
//   - Error clustering using normalized patterns (removes IDs, timestamps, UUIDs)
//   - Third-party domain classification (first-party vs third-party detection)
//   - API contract validation and violation detection
//
// The SchemaStore tracks observed API endpoints and infers request/response schemas
// by analyzing network bodies. The clustering algorithm groups similar errors to
// reduce noise in error logs.
//
// All analysis is performed in-memory with configurable thresholds for minimum
// observations before considering an endpoint "stable".
package analysis
