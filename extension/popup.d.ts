/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */
import { updateConnectionStatus } from './popup/status-display'
import { handleClearLogs, resetClearConfirm } from './popup/settings'
export { resetClearConfirm, handleClearLogs }
export { updateConnectionStatus }
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles'
export { handleFeatureToggle } from './popup/feature-toggles'
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot'
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking'
export { handleLogLevelChange, handleWebSocketModeChange } from './popup/settings'
export { initLogLevelSelector } from './popup/settings'
export { initWebSocketModeSelector } from './popup/settings'
export { isInternalUrl } from './popup/ui-utils'
/**
 * Initialize the popup
 */
export declare function initPopup(): Promise<void>
//# sourceMappingURL=popup.d.ts.map
