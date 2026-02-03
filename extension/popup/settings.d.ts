/**
 * @fileoverview Settings Module
 * Handles log level, WebSocket mode, and clear logs functionality
 */
import type { WebSocketCaptureMode } from '../types'
/**
 * Initialize the log level selector
 */
export declare function initLogLevelSelector(): Promise<void>
/**
 * Handle log level change
 */
export declare function handleLogLevelChange(level: string): Promise<void>
/**
 * Handle WebSocket mode change
 */
export declare function handleWebSocketModeChange(mode: WebSocketCaptureMode): void
/**
 * Initialize the WebSocket mode selector
 */
export declare function initWebSocketModeSelector(): Promise<void>
/**
 * Reset clear confirmation state (exported for testing)
 */
export declare function resetClearConfirm(): void
/**
 * Handle clear logs button click (with confirmation)
 */
export declare function handleClearLogs(): Promise<{
  success?: boolean
  error?: string
} | null>
//# sourceMappingURL=settings.d.ts.map
