// Purpose: Package streaming — push-style alert delivery with deduplication, throttling, and CI materialization.
// Why: Enables real-time notification of browser events without flooding consumers with duplicate noise.
// Docs: docs/features/feature/push-alerts/index.md

/*
Package streaming implements push-style alert delivery for real-time browser event notification.

Key types:
  - StreamState: configuration and runtime state for streaming (enabled, events, throttle).
  - StreamConfig: user-configurable streaming parameters (events, throttle, severity).
  - AlertBuffer: bounded buffer with deduplication, correlation, and CI alert materialization.

Key functions:
  - NewStreamState: creates a stream state with default configuration.
  - AddAlert: appends an alert to the buffer with capacity-based eviction.
  - EmitNotification: sends an MCP notification if streaming is enabled and not throttled.
*/
package streaming
