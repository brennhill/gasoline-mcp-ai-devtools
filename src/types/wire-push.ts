// wire-push.ts — Hand-maintained wire types for push endpoints.
// Go source of truth: cmd/browser-agent/push_handlers.go
// Checked by: scripts/check-sync-wire-drift.js

/**
 * PushScreenshotRequest is the request body for POST /push/screenshot.
 * Mirrors the inline struct in handlePushScreenshot.
 */
export interface PushScreenshotRequest {
  screenshot_data_url: string
  note: string
  page_url: string
  tab_id: number
}

/**
 * PushMessageRequest is the request body for POST /push/message.
 * Mirrors the inline struct in handlePushMessage.
 */
export interface PushMessageRequest {
  message: string
  page_url: string
  tab_id: number
}

/**
 * PushCapabilities is the response from GET /push/capabilities.
 * Mirrors the response shape in handlePushCapabilities.
 */
export interface PushCapabilities {
  push_enabled: boolean
  supports_sampling: boolean
  supports_notifications: boolean
  client_name: string
  inbox_count: number
}

/**
 * PushResponse is the response from POST /push/screenshot and POST /push/message.
 * Mirrors the jsonResponse shape in handlePushScreenshot/handlePushMessage.
 */
export interface PushResponse {
  status: string
  event_id: string
  delivery_method: string
}
