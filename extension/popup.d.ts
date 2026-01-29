/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */
import type { ConnectionStatus, MemoryPressureState, ContextWarning, WebSocketCaptureMode } from './types/index';
interface PopupConnectionStatus extends ConnectionStatus {
    serverUrl?: string;
    circuitBreakerState?: 'closed' | 'open' | 'half-open';
    memoryPressure?: MemoryPressureState;
    contextWarning?: ContextWarning;
    error?: string;
}
interface FeatureToggleConfig {
    id: string;
    storageKey: string;
    messageType: string;
    default: boolean;
}
/**
 * Update the connection status display
 */
export declare function updateConnectionStatus(status: PopupConnectionStatus): void;
/**
 * Feature toggle configuration
 */
export declare const FEATURE_TOGGLES: readonly FeatureToggleConfig[];
/**
 * Initialize all feature toggles
 */
export declare function initFeatureToggles(): Promise<void>;
/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export declare function handleFeatureToggle(storageKey: string, messageType: string, enabled: boolean): void;
/**
 * Initialize the AI Web Pilot toggle.
 * Read the current state from chrome.storage.local.
 */
export declare function initAiWebPilotToggle(): Promise<void>;
/**
 * Check if a URL is an internal browser page that cannot be tracked.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 */
export declare function isInternalUrl(url: string | undefined): boolean;
/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
export declare function initTrackPageButton(): Promise<void>;
/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export declare function handleTrackPageClick(): Promise<void>;
/**
 * Handle AI Web Pilot toggle change.
 *
 * CRITICAL: ONLY background.js updates the state via setAiWebPilotEnabled message.
 * Popup NEVER writes to chrome.storage directly.
 *
 * This ensures single source of truth. If popup wrote to storage directly:
 * 1. Popup updates storage
 * 2. Background cache doesn't update (no listener yet)
 * 3. Pilot command checks cache and gets wrong value
 * 4. User sees toggle "on" but commands fail saying "off"
 *
 * By routing through background, we guarantee:
 * 1. Popup sends message to background
 * 2. Background updates cache immediately
 * 3. Background writes to storage
 * 4. Pilot commands see correct cache state
 * 5. Everything is consistent
 */
export declare function handleAiWebPilotToggle(enabled: boolean): Promise<void>;
/**
 * Handle WebSocket mode change
 */
export declare function handleWebSocketModeChange(mode: WebSocketCaptureMode): void;
/**
 * Initialize the WebSocket mode selector
 */
export declare function initWebSocketModeSelector(): Promise<void>;
/**
 * Initialize the log level selector
 */
export declare function initLogLevelSelector(): Promise<void>;
/**
 * Handle log level change
 */
export declare function handleLogLevelChange(level: string): Promise<void>;
/**
 * Reset clear confirmation state (exported for testing)
 */
export declare function resetClearConfirm(): void;
/**
 * Handle clear logs button click (with confirmation)
 */
export declare function handleClearLogs(): Promise<{
    success?: boolean;
    error?: string;
} | null>;
/**
 * Initialize the popup
 */
export declare function initPopup(): Promise<void>;
export {};
//# sourceMappingURL=popup.d.ts.map