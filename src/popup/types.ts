/**
 * @fileoverview Popup Types
 * Type definitions for popup UI
 */

import type { ConnectionStatus, MemoryPressureState, ContextWarning, WebSocketCaptureMode } from '../types'

/**
 * Extended connection status for popup
 */
export interface PopupConnectionStatus extends ConnectionStatus {
  serverUrl?: string
  circuitBreakerState?: 'closed' | 'open' | 'half-open'
  memoryPressure?: MemoryPressureState
  contextWarning?: ContextWarning
  error?: string
}

/**
 * Feature toggle configuration type
 */
export interface FeatureToggleConfig {
  id: string
  storageKey: string
  messageType: string
  default: boolean
}

/**
 * Toggle warning configuration
 */
export interface ToggleWarningConfig {
  toggleId: string
  warningId: string
}
