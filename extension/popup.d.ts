/**
 * Purpose: Owns popup.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */
import { updateConnectionStatus } from './popup/status-display';
import { handleClearLogs, resetClearConfirm } from './popup/settings';
export { resetClearConfirm, handleClearLogs };
export { updateConnectionStatus };
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles';
export { handleFeatureToggle } from './popup/feature-toggles';
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot';
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking';
export { handleWebSocketModeChange } from './popup/settings';
export { initWebSocketModeSelector } from './popup/settings';
export { isInternalUrl } from './popup/ui-utils';
/**
 * Initialize the popup
 */
export declare function initPopup(): Promise<void>;
//# sourceMappingURL=popup.d.ts.map