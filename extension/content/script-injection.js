/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
import { SettingName } from '../lib/constants.js';
/** Whether inject.bundled.js has been injected into the page (MAIN world) */
let injected = false;
/** Per-page-load nonce for authenticating postMessages to inject.js */
const pageNonce = crypto
    .getRandomValues(new Uint8Array(16))
    .reduce((s, b) => s + b.toString(16).padStart(2, '0'), '');
/** Get the page nonce for authenticating postMessages to inject.js */
export function getPageNonce() {
    return pageNonce;
}
/** Check if inject script has been loaded into the page context */
export function isInjectScriptLoaded() {
    return injected;
}
/** Settings that need to be synced to inject script on page load */
const SYNC_SETTINGS = [
    { storageKey: 'webSocketCaptureEnabled', messageType: SettingName.WEBSOCKET_CAPTURE },
    { storageKey: 'webSocketCaptureMode', messageType: SettingName.WEBSOCKET_CAPTURE_MODE, isMode: true },
    { storageKey: 'networkWaterfallEnabled', messageType: SettingName.NETWORK_WATERFALL },
    { storageKey: 'performanceMarksEnabled', messageType: SettingName.PERFORMANCE_MARKS },
    { storageKey: 'actionReplayEnabled', messageType: SettingName.ACTION_REPLAY },
    { storageKey: 'networkBodyCaptureEnabled', messageType: SettingName.NETWORK_BODY_CAPTURE }
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
                window.postMessage({
                    type: 'GASOLINE_SETTING',
                    setting: setting.messageType,
                    mode: value,
                    _nonce: pageNonce
                }, window.location.origin);
            }
            else {
                window.postMessage({ type: 'GASOLINE_SETTING', setting: setting.messageType, enabled: value, _nonce: pageNonce }, window.location.origin);
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
    script.dataset.gasolineNonce = pageNonce;
    script.onload = () => {
        script.remove();
        injected = true;
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