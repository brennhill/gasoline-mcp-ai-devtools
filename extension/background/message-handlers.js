/**
 * @fileoverview Message Handlers - Handles all chrome.runtime.onMessage routing
 * with type-safe message discrimination.
 */
// =============================================================================
// MESSAGE HANDLER
// =============================================================================
/**
 * Install the main message listener
 */
export function installMessageListener(deps) {
    if (typeof chrome === 'undefined' || !chrome.runtime)
        return;
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
        return handleMessage(message, sender, sendResponse, deps);
    });
}
/**
 * Handle incoming message
 * Returns true if response will be sent asynchronously
 */
function handleMessage(message, sender, sendResponse, deps) {
    const messageType = message.type;
    switch (messageType) {
        case 'GET_TAB_ID':
            sendResponse({ tabId: sender.tab?.id });
            return true;
        case 'ws_event':
            deps.addToWsBatcher(message.payload);
            return false;
        case 'enhanced_action':
            deps.addToEnhancedActionBatcher(message.payload);
            return false;
        case 'network_body':
            if (deps.isNetworkBodyCaptureDisabled()) {
                deps.debugLog('capture', 'Network body dropped: capture disabled');
                return true;
            }
            deps.addToNetworkBodyBatcher(message.payload);
            return false;
        case 'performance_snapshot':
            deps.addToPerfBatcher(message.payload);
            return false;
        case 'log':
            handleLogMessageAsync(message, sender, deps);
            return true;
        case 'getStatus':
            sendResponse({
                ...deps.getConnectionStatus(),
                serverUrl: deps.getServerUrl(),
                screenshotOnError: deps.getScreenshotOnError(),
                sourceMapEnabled: deps.getSourceMapEnabled(),
                debugMode: deps.getDebugMode(),
                contextWarning: deps.getContextWarning(),
                circuitBreakerState: deps.getCircuitBreakerState(),
                memoryPressure: deps.getMemoryPressureState(),
            });
            return false;
        case 'clearLogs':
            handleClearLogsAsync(sendResponse, deps);
            return true;
        case 'setLogLevel':
            deps.setCurrentLogLevel(message.level);
            deps.saveSetting('logLevel', message.level);
            return false;
        case 'setScreenshotOnError':
            deps.setScreenshotOnError(message.enabled);
            deps.saveSetting('screenshotOnError', message.enabled);
            sendResponse({ success: true });
            return false;
        case 'setAiWebPilotEnabled':
            handleSetAiWebPilotEnabled(message.enabled, sendResponse, deps);
            return false;
        case 'getAiWebPilotEnabled':
            sendResponse({ enabled: deps.getAiWebPilotEnabled() });
            return false;
        case 'getDiagnosticState':
            handleGetDiagnosticState(sendResponse, deps);
            return true;
        case 'captureScreenshot':
            handleCaptureScreenshot(sendResponse, deps);
            return true;
        case 'setSourceMapEnabled':
            deps.setSourceMapEnabled(message.enabled);
            deps.saveSetting('sourceMapEnabled', message.enabled);
            if (!message.enabled) {
                deps.clearSourceMapCache();
            }
            sendResponse({ success: true });
            return false;
        case 'setNetworkWaterfallEnabled':
        case 'setPerformanceMarksEnabled':
        case 'setActionReplayEnabled':
        case 'setWebSocketCaptureEnabled':
        case 'setWebSocketCaptureMode':
        case 'setPerformanceSnapshotEnabled':
        case 'setDeferralEnabled':
        case 'setNetworkBodyCaptureEnabled':
            handleForwardedSetting(message, sendResponse, deps);
            return false;
        case 'setDebugMode':
            deps.setDebugMode(message.enabled);
            deps.saveSetting('debugMode', message.enabled);
            sendResponse({ success: true });
            return false;
        case 'getDebugLog':
            sendResponse({ log: deps.exportDebugLog() });
            return false;
        case 'clearDebugLog':
            deps.clearDebugLog();
            deps.debugLog('lifecycle', 'Debug log cleared');
            sendResponse({ success: true });
            return false;
        case 'setServerUrl':
            handleSetServerUrl(message.url, sendResponse, deps);
            return false;
        default:
            // Unknown message type
            return false;
    }
}
// =============================================================================
// ASYNC HANDLERS
// =============================================================================
async function handleLogMessageAsync(message, sender, deps) {
    try {
        await deps.handleLogMessage(message.payload, sender, message.tabId);
    }
    catch (err) {
        console.error('[Gasoline] Failed to handle log message:', err);
    }
}
async function handleClearLogsAsync(sendResponse, deps) {
    try {
        const result = await deps.handleClearLogs();
        sendResponse(result);
    }
    catch (err) {
        console.error('[Gasoline] Failed to clear logs:', err);
        sendResponse({ error: err.message });
    }
}
function handleSetAiWebPilotEnabled(enabled, sendResponse, deps) {
    const newValue = enabled === true;
    console.log(`[Gasoline] AI Web Pilot toggle: -> ${newValue}`);
    deps.setAiWebPilotEnabled(newValue, () => {
        console.log(`[Gasoline] AI Web Pilot persisted to storage: ${newValue}`);
        deps.postSettings(deps.getServerUrl());
    });
    sendResponse({ success: true });
}
function handleGetDiagnosticState(sendResponse, deps) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        sendResponse({
            cache: deps.getAiWebPilotEnabled(),
            storage: undefined,
            timestamp: new Date().toISOString(),
        });
        return;
    }
    chrome.storage.local.get(['aiWebPilotEnabled'], (result) => {
        sendResponse({
            cache: deps.getAiWebPilotEnabled(),
            storage: result.aiWebPilotEnabled,
            timestamp: new Date().toISOString(),
        });
    });
}
function handleCaptureScreenshot(sendResponse, deps) {
    if (typeof chrome === 'undefined' || !chrome.tabs) {
        sendResponse({ success: false, error: 'Chrome tabs API not available' });
        return;
    }
    chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
        if (tabs[0]?.id) {
            const result = await deps.captureScreenshot(tabs[0].id, null);
            if (result.success && result.entry) {
                deps.addToLogBatcher(result.entry);
            }
            sendResponse(result);
        }
        else {
            sendResponse({ success: false, error: 'No active tab' });
        }
    });
}
function handleForwardedSetting(message, sendResponse, deps) {
    deps.debugLog('settings', `Setting ${message.type}: ${message.enabled ?? message.mode}`);
    deps.forwardToAllContentScripts(message);
    sendResponse({ success: true });
}
function handleSetServerUrl(url, sendResponse, deps) {
    deps.setServerUrl(url || 'http://localhost:7890');
    deps.saveSetting('serverUrl', deps.getServerUrl());
    deps.debugLog('settings', `Server URL changed to: ${deps.getServerUrl()}`);
    // Broadcast to all content scripts
    deps.forwardToAllContentScripts({ type: 'setServerUrl', url: deps.getServerUrl() });
    // Re-check connection with new URL
    deps.checkConnectionAndUpdate();
    sendResponse({ success: true });
}
// =============================================================================
// STATE SNAPSHOT STORAGE
// =============================================================================
const SNAPSHOT_KEY = 'gasoline_state_snapshots';
/**
 * Save a state snapshot to chrome.storage.local
 */
export async function saveStateSnapshot(name, state) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            const sizeBytes = JSON.stringify(state).length;
            snapshots[name] = {
                ...state,
                name,
                size_bytes: sizeBytes,
            };
            chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
                resolve({
                    success: true,
                    snapshot_name: name,
                    size_bytes: sizeBytes,
                });
            });
        });
    });
}
/**
 * Load a state snapshot from chrome.storage.local
 */
export async function loadStateSnapshot(name) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            resolve(snapshots[name] || null);
        });
    });
}
/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots() {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            const list = Object.values(snapshots).map((s) => ({
                name: s.name,
                url: s.url,
                timestamp: s.timestamp,
                size_bytes: s.size_bytes,
            }));
            resolve(list);
        });
    });
}
/**
 * Delete a state snapshot from chrome.storage.local
 */
export async function deleteStateSnapshot(name) {
    return new Promise((resolve) => {
        chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
            const snapshots = result[SNAPSHOT_KEY] || {};
            delete snapshots[name];
            chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
                resolve({ success: true, deleted: name });
            });
        });
    });
}
//# sourceMappingURL=message-handlers.js.map