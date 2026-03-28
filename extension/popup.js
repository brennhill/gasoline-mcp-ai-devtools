/**
 * Purpose: Orchestrates popup initialization and binds UI modules for tracking, recording, draw mode, and pilot controls.
 * Why: Keeps popup behavior consistent by coordinating status/state hydration in one lifecycle entrypoint.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */
import { RuntimeMessageName, StorageKey } from './lib/constants.js';
import { getLocal, setSession, getSession, onStorageChanged } from './lib/storage-utils.js';
import { updateConnectionStatus } from './popup/status-display.js';
import { setupRecordingUI } from './popup/recording.js';
import { setupDrawModeButton } from './popup/draw-mode.js';
import { setupActionRecordingUI } from './popup/action-recording.js';
import { initFeatureToggles } from './popup/feature-toggles.js';
import { initTrackPageButton } from './popup/tab-tracking.js';
import { initAiWebPilotToggle } from './popup/ai-web-pilot.js';
import { initPopupLogoMotion } from './popup/logo-motion.js';
import { initWebSocketModeSelector, handleWebSocketModeChange, handleClearLogs, resetClearConfirm } from './popup/settings.js';
// Re-export for testing
export { resetClearConfirm, handleClearLogs };
export { updateConnectionStatus };
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles.js';
export { handleFeatureToggle } from './popup/feature-toggles.js';
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot.js';
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking.js';
export { handleWebSocketModeChange } from './popup/settings.js';
export { initWebSocketModeSelector } from './popup/settings.js';
export { isInternalUrl } from './popup/ui-utils.js';
// Apply theme early to prevent flash of unstyled content (moved from inline script for CSP compliance).
void getLocal('theme').then((value) => {
    if (value === 'light')
        document.body.classList.add('light-theme');
});
const DEFAULT_MAX_ENTRIES = 1000;
const RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE = {
    type: RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
};
/**
 * Bind a toggle element to show/hide a target element based on a condition.
 * Sets initial display state and adds a change listener.
 */
function bindToggleVisibility(toggle, target, isVisible) {
    target.style.display = isVisible() ? 'block' : 'none';
    toggle.addEventListener('change', () => {
        target.style.display = isVisible() ? 'block' : 'none';
    });
}
// #lizard forgives
function setupWebSocketUI() {
    const wsToggle = document.getElementById('toggle-websocket');
    const wsModeContainer = document.getElementById('ws-mode-container');
    if (wsToggle && wsModeContainer) {
        bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked);
    }
    const wsModeSelect = document.getElementById('ws-mode');
    if (wsModeSelect) {
        wsModeSelect.addEventListener('change', (e) => {
            handleWebSocketModeChange(e.target.value);
        });
    }
    const wsMessagesWarning = document.getElementById('ws-messages-warning');
    if (wsModeSelect && wsMessagesWarning) {
        bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === 'all');
    }
}
function setupToggleWarnings() {
    const toggleWarnings = [
        { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
        { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
        { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' }
    ];
    for (const { toggleId, warningId } of toggleWarnings) {
        const toggle = document.getElementById(toggleId);
        const warning = document.getElementById(warningId);
        if (toggle && warning) {
            warning.style.display = toggle.checked ? 'block' : 'none';
            toggle.addEventListener('change', () => {
                warning.style.display = toggle.checked ? 'block' : 'none';
            });
        }
    }
}
function requestTrackedHoverLauncherReshow() {
    if (!chrome.tabs?.query || !chrome.tabs?.sendMessage)
        return;
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const tabId = tabs[0]?.id;
        if (!tabId)
            return;
        chrome.tabs.sendMessage(tabId, RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE, () => {
            void chrome.runtime.lastError;
        });
    });
}
/** Cache status to session storage so the popup renders instantly on next open. */
function cacheStatus(status) {
    void setSession(StorageKey.POPUP_LAST_STATUS, status);
}
/**
 * Initialize the popup
 */
export function initPopup() {
    // Re-show tracked-tab quick launcher if user hid it from the page UI.
    requestTrackedHoverLauncherReshow();
    // 1) Hydrate immediately from cached status (local, no network, no IPC wait).
    void getSession(StorageKey.POPUP_LAST_STATUS).then((value) => {
        const cached = value;
        if (cached)
            updateConnectionStatus(cached);
    });
    // 2) Request fresh status from background worker (async — updates UI when ready).
    try {
        chrome.runtime.sendMessage({ type: 'get_status' }, (status) => {
            if (chrome.runtime.lastError) {
                updateConnectionStatus({
                    connected: false,
                    entries: 0,
                    maxEntries: DEFAULT_MAX_ENTRIES,
                    errorCount: 0,
                    logFile: '',
                    error: 'Extension restarting — please wait a moment and reopen popup'
                });
                return;
            }
            if (status) {
                updateConnectionStatus(status);
                cacheStatus(status);
            }
        });
    }
    catch {
        updateConnectionStatus({
            connected: false,
            entries: 0,
            maxEntries: DEFAULT_MAX_ENTRIES,
            errorCount: 0,
            logFile: '',
            error: 'Extension error — try reloading the extension'
        });
    }
    // Initialize all UI synchronously — no awaits, no blocking.
    // Each init reads chrome.storage via callback and updates DOM when ready.
    // None depend on each other, so they all fire in parallel.
    initPopupLogoMotion();
    setupRecordingUI();
    setupActionRecordingUI();
    initFeatureToggles();
    initWebSocketModeSelector();
    initAiWebPilotToggle();
    initTrackPageButton();
    setupWebSocketUI();
    setupToggleWarnings();
    setupDrawModeButton();
    const clearBtn = document.getElementById('clear-btn');
    if (clearBtn)
        clearBtn.addEventListener('click', handleClearLogs);
    // Listen for status updates
    chrome.runtime.onMessage.addListener((message) => {
        if (message.type === 'status_update' && message.status) {
            updateConnectionStatus(message.status);
            cacheStatus(message.status);
        }
    });
    // Listen for storage changes (e.g., tracked tab URL updates)
    onStorageChanged((changes, areaName) => {
        if (areaName === 'local' && changes[StorageKey.TRACKED_TAB_URL]) {
            const urlEl = document.getElementById('tracking-bar-url');
            if (urlEl && changes[StorageKey.TRACKED_TAB_URL].newValue) {
                urlEl.textContent = changes[StorageKey.TRACKED_TAB_URL].newValue;
                console.log('[Gasoline] Tracked tab URL updated in popup:', changes[StorageKey.TRACKED_TAB_URL].newValue);
            }
        }
    });
}
// Initialize when DOM is ready
if (typeof document !== 'undefined' && typeof globalThis.process === 'undefined') {
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initPopup);
    }
    else {
        initPopup();
    }
}
//# sourceMappingURL=popup.js.map