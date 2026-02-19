// settings.ts — Settings dispatch and state command handling for inject context.
import { setNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, setServerUrl } from '../lib/network.js';
import { setPerformanceMarksEnabled, installPerformanceCapture, uninstallPerformanceCapture } from '../lib/performance.js';
import { setActionCaptureEnabled } from '../lib/actions.js';
import { setWebSocketCaptureEnabled, setWebSocketCaptureMode, installWebSocketCapture, uninstallWebSocketCapture } from '../lib/websocket.js';
import { setPerformanceSnapshotEnabled } from '../lib/perf-snapshot.js';
import { setDeferralEnabled } from './observers.js';
import { INJECT_FORWARDED_SETTINGS, SettingName } from '../lib/constants.js';
/**
 * Valid setting names from content script — imported from canonical constants.
 */
export const VALID_SETTINGS = INJECT_FORWARDED_SETTINGS;
export const VALID_STATE_ACTIONS = new Set(['capture', 'restore']);
export function isValidSettingPayload(data) {
    if (!VALID_SETTINGS.has(data.setting)) {
        console.warn('[Gasoline] Invalid setting:', data.setting);
        return false;
    }
    if (data.setting === SettingName.WEBSOCKET_CAPTURE_MODE)
        return typeof data.mode === 'string';
    if (data.setting === SettingName.SERVER_URL)
        return typeof data.url === 'string';
    // Boolean settings
    if (typeof data.enabled !== 'boolean') {
        console.warn('[Gasoline] Invalid enabled value type');
        return false;
    }
    return true;
}
const SETTING_HANDLERS = {
    [SettingName.NETWORK_WATERFALL]: (data) => setNetworkWaterfallEnabled(data.enabled),
    [SettingName.PERFORMANCE_MARKS]: (data) => {
        setPerformanceMarksEnabled(data.enabled);
        if (data.enabled)
            installPerformanceCapture();
        else
            uninstallPerformanceCapture();
    },
    [SettingName.ACTION_REPLAY]: (data) => setActionCaptureEnabled(data.enabled),
    [SettingName.WEBSOCKET_CAPTURE]: (data) => {
        setWebSocketCaptureEnabled(data.enabled);
        if (data.enabled)
            installWebSocketCapture();
        else
            uninstallWebSocketCapture();
    },
    [SettingName.WEBSOCKET_CAPTURE_MODE]: (data) => setWebSocketCaptureMode((data.mode || 'medium')),
    [SettingName.PERFORMANCE_SNAPSHOT]: (data) => setPerformanceSnapshotEnabled(data.enabled),
    [SettingName.DEFERRAL]: (data) => setDeferralEnabled(data.enabled),
    [SettingName.NETWORK_BODY_CAPTURE]: (data) => setNetworkBodyCaptureEnabled(data.enabled),
    [SettingName.SERVER_URL]: (data) => setServerUrl(data.url)
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