/**
 * Purpose: Declares generated canonical TypeScript wire contracts for websocket event payloads.
 * Why: Guarantees extension/server parity for websocket telemetry serialization across releases.
 * Docs: docs/features/feature/normalized-event-schema/index.md
 */

// THIS FILE IS GENERATED — do not edit by hand.
// Source: internal/types/wire_websocket_event.go
// Generator: scripts/generate-wire-types.js

/**
 * @fileoverview Wire type for WebSocket events — matches internal/types/wire_websocket_event.go
 *
 * Canonical TypeScript definition for the WebSocketEvent HTTP payload.
 * Changes here MUST be mirrored in the Go counterpart. Run `make check-wire-drift`.
 */

/**
 * WireWebSocketEvent is the JSON shape sent over HTTP for captured WebSocket events.
 */
export interface WireWebSocketEvent {
  readonly ts?: string
  readonly type?: string
  readonly event: string
  readonly id: string
  readonly url?: string
  readonly direction?: 'incoming' | 'outgoing'
  readonly data?: string
  readonly size?: number
  readonly code?: number
  readonly reason?: string
  // server-only: sampled, binary_format, format_confidence, tab_id, test_ids
}
