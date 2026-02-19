// settings.ts — Settings dispatch and state command handling for inject context.
import { setNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, setServerUrl } from '../lib/network.js';
import { setPerformanceMarksEnabled, installPerformanceCapture, uninstallPerformanceCapture } from '../lib/performance.js';
import { setActionCaptureEnabled } from '../lib/actions.js';
import { setWebSocketCaptureEnabled, setWebSocketCaptureMode, installWebSocketCapture, uninstallWebSocketCapture } from '../lib/websocket.js';
import { setPerformanceSnapshotEnabled } from '../lib/perf-snapshot.js';
import { setDeferralEnabled } from './observers.js';
/**
 * Valid setting names from content script
 */
export const VALID_SETTINGS = new Set([
    'setNetworkWaterfallEnabled',
    'setPerformanceMarksEnabled',
    'setActionReplayEnabled',
    'setWebSocketCaptureEnabled',
    'setWebSocketCaptureMode',
    'setPerformanceSnapshotEnabled',
    'setDeferralEnabled',
    'setNetworkBodyCaptureEnabled',
    'setServerUrl'
]);
export const VALID_STATE_ACTIONS = new Set(['capture', 'restore']);
export function isValidSettingPayload(data) {
    if (!VALID_SETTINGS.has(data.setting)) {
        console.warn('[Gasoline] Invalid setting:', data.setting);
        return false;
    }
    if (data.setting === 'setWebSocketCaptureMode')
        return typeof data.mode === 'string';
    if (data.setting === 'setServerUrl')
        return typeof data.url === 'string';
    // Boolean settings
    if (typeof data.enabled !== 'boolean') {
        console.warn('[Gasoline] Invalid enabled value type');
        return false;
    }
    return true;
}
const SETTING_HANDLERS = {
    setNetworkWaterfallEnabled: (data) => setNetworkWaterfallEnabled(data.enabled),
    setPerformanceMarksEnabled: (data) => {
        setPerformanceMarksEnabled(data.enabled);
        if (data.enabled)
            installPerformanceCapture();
        else
            uninstallPerformanceCapture();
    },
    setActionReplayEnabled: (data) => setActionCaptureEnabled(data.enabled),
    setWebSocketCaptureEnabled: (data) => {
        setWebSocketCaptureEnabled(data.enabled);
        if (data.enabled)
            installWebSocketCapture();
        else
            uninstallWebSocketCapture();
    },
    setWebSocketCaptureMode: (data) => setWebSocketCaptureMode((data.mode || 'medium')),
    setPerformanceSnapshotEnabled: (data) => setPerformanceSnapshotEnabled(data.enabled),
    setDeferralEnabled: (data) => setDeferralEnabled(data.enabled),
    setNetworkBodyCaptureEnabled: (data) => setNetworkBodyCaptureEnabled(data.enabled),
    setServerUrl: (data) => setServerUrl(data.url)
};
export function handleSetting(data) {
    const handler = SETTING_HANDLERS[data.setting];
    if (handler)
        handler(data);
}
export function handleStateCommand(data, captureStateFn, restoreStateFn) {
    const { messageId, action, state } = data;
    // Validate action
    if (!VALID_STATE_ACTIONS.has(action)) {
        console.warn('[Gasoline] Invalid state action:', action);
        window.postMessage({
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: `Invalid action: ${action}` }
        }, window.location.origin);
        return;
    }
    // Validate state object for restore action
    if (action === 'restore' && (!state || typeof state !== 'object')) {
        console.warn('[Gasoline] Invalid state object for restore');
        window.postMessage({
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: 'Invalid state object' }
        }, window.location.origin);
        return;
    }
    let result;
    try {
        if (action === 'capture') {
            result = captureStateFn();
        }
        else if (action === 'restore') {
            const includeUrl = data.include_url !== false;
            result = restoreStateFn(state, includeUrl);
        }
        else {
            result = { error: `Unknown action: ${action}` };
        }
    }
    catch (err) {
        result = { error: err.message };
    }
    // Send response back to content script
    window.postMessage({
        type: 'GASOLINE_STATE_RESPONSE',
        messageId,
        result
    }, window.location.origin);
}
//# sourceMappingURL=settings.js.map