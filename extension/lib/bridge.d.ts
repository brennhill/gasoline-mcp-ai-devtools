/**
 * @fileoverview Message bridge for posting log events to the content script.
 * Enriches error-level messages with context annotations and user action replay.
 */
export interface BridgePayload {
  level?: string
  message?: string
  error?: string
  args?: unknown[]
  filename?: string
  lineno?: number
  [key: string]: unknown
}
/**
 * Post a log message to the content script
 */
export declare function postLog(payload: BridgePayload): void
//# sourceMappingURL=bridge.d.ts.map
