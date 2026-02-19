/**
 * Purpose: Owns websocket.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview WebSocket Types
 * WebSocket capture modes, events, and connection tracking
 */
/**
 * WebSocket capture modes
 */
export type WebSocketCaptureMode = 'low' | 'medium' | 'high' | 'all';
/**
 * WebSocket event types
 */
export type WebSocketEventType = 'open' | 'close' | 'error' | 'message';
/**
 * WebSocket event — re-exported from wire type (canonical HTTP payload shape).
 * The stale interface previously used camelCase fields (connectionId, direction: 'sent'|'received')
 * that didn't match the actual runtime data or Go server expectations.
 */
export type { WireWebSocketEvent as WebSocketEvent } from './wire-websocket-event';
//# sourceMappingURL=websocket.d.ts.map