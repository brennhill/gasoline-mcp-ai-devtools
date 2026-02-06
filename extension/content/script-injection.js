/**
 * @fileoverview Script Injection Module
 * Injects capture script into the page context and syncs stored settings
 */
/** Settings that need to be synced to inject script on page load */
const SYNC_SETTINGS = [
    { storageKey: 'webSocketCaptureEnabled', messageType: 'setWebSocketCaptureEnabled' },
    { storageKey: 'webSocketCaptureMode', messageType: 'setWebSocketCaptureMode', isMode: true },
    { storageKey: 'networkWaterfallEnabled', messageType: 'setNetworkWaterfallEnabled' },
    { storageKey: 'performanceMarksEnabled', messageType: 'setPerformanceMarksEnabled' },
    { storageKey: 'actionReplayEnabled', messageType: 'setActionReplayEnabled' },
    { storageKey: 'networkBodyCaptureEnabled', messageType: 'setNetworkBodyCaptureEnabled' },
];
/**
 * Sync stored settings to the inject script after it loads.
 * This ensures new pages receive the current settings state.
 */
function syncStoredSettings() {
    const storageKeys = SYNC_SETTINGS.map((s) => s.storageKey);
    chrome.storage.local.get(storageKeys, (result) => {
        for (const setting of SYNC_SETTINGS) {
            const value = result[setting.storageKey];
            if (value === undefined)
                continue; // Use default if not set
            if (setting.isMode) {
                window.postMessage({ type: 'GASOLINE_SETTING', setting: setting.messageType, mode: value }, window.location.origin);
            }
            else {
                window.postMessage({ type: 'GASOLINE_SETTING', setting: setting.messageType, enabled: value }, window.location.origin);
            }
        }
    });
}
/**
 * Inject axe-core library into the page
 * Must be called from content script context (has chrome.runtime API access)
 */
export function injectAxeCore() {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('lib/axe.min.js');
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
}
/**
 * Inject the capture script into the page
 */
export function injectScript() {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('inject.bundled.js');
    script.type = 'module';
    script.onload = () => {
        script.remove();
        // Sync stored settings after inject script loads
        // Small delay to ensure inject script has initialized its message listeners
        setTimeout(syncStoredSettings, 50);
    };
    (document.head || document.documentElement).appendChild(script);
}
/**
 * Initialize script injection (call when DOM is ready)
 */
export function initScriptInjection() {
    // Inject when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            injectAxeCore(); // Inject axe-core first (needed by inject script)
            injectScript();
        }, { once: true });
    }
    else {
        injectAxeCore(); // Inject axe-core first (needed by inject script)
        injectScript();
    }
}
//# sourceMappingURL=script-injection.js.map