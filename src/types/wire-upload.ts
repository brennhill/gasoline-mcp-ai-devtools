// wire-upload.ts — Hand-maintained wire types for upload endpoints.
// Go source of truth: internal/upload/types.go
// Checked by: scripts/check-sync-wire-drift.js

/**
 * FileReadResponse is the response from POST /api/file/read.
 * Mirrors Go type: upload.FileReadResponse
 */
export interface FileReadResponse {
  success: boolean
  file_name?: string
  file_size?: number
  mime_type?: string
  data_base64?: string
  error?: string
}

/**
 * StageResponse is the generic response for upload stage operations.
 * Mirrors Go type: upload.StageResponse
 */
export interface StageResponse {
  success: boolean
  stage?: number
  status?: string
  error?: string
  file_name?: string
  file_size_bytes?: number
  duration_ms?: number
  escalation_reason?: string
  suggestions?: string[]
  bytes_sent?: number
  total_bytes?: number
  percent?: number
  eta_seconds?: number
  speed_mbps?: number
}

/**
 * OSAutomationResponse is the response from POST /api/os-automation/inject.
 * Mirrors a subset of StageResponse used by OS automation escalation.
 */
export interface OSAutomationResponse {
  success: boolean
  stage?: number
  error?: string
  file_name?: string
  suggestions?: string[]
}
