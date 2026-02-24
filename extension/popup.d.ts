/**
 * Purpose: Orchestrates popup initialization and binds UI modules for tracking, recording, draw mode, and pilot controls.
 * Why: Keeps popup behavior consistent by coordinating status/state hydration in one lifecycle entrypoint.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
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