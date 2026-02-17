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
export type WebSocketCaptureMode = 'low' | 'medium' | 'high' | 'all'

/**
 * WebSocket event types
 */
export type WebSocketEventType = 'open' | 'close' | 'error' | 'message'

/**
 * WebSocket event payload
 */
export interface WebSocketEvent {
  readonly type: WebSocketEventType
  readonly url: string
  readonly ts: string
  readonly connectionId?: string
  readonly data?: string
  readonly size?: number
  readonly direction?: 'sent' | 'received'
  readonly code?: number
  readonly reason?: string
}
