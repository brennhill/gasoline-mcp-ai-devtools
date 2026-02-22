/**
 * @fileoverview Mutable module-level state for the background service worker.
 * Owns all `let` variables and their setter functions so that state ownership
 * is explicit and separated from business logic in index.ts.
 */
import { DEFAULT_SERVER_URL } from '../lib/constants.js';
// =============================================================================
// MODULE STATE
// =============================================================================
/** Session ID for detecting extension reloads */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
const state = {
    serverUrl: DEFAULT_SERVER_URL,
    debugMode: false,
    connectionStatus: {
        connected: false,
        entries: 0,
        maxEntries: 1000,
        errorCount: 0,
        logFile: '',
        securityMode: 'normal',
        productionParity: true,
        insecureRewritesApplied: []
    },
    currentLogLevel: 'all',
    screenshotOnError: false,
    captureOverrides: {},
    aiControlled: false,
    connectionCheckRunning: false,
    aiWebPilotEnabledCache: true,
    aiWebPilotCacheInitialized: false,
    pilotInitCallback: null,
    extensionLogQueue: []
};
/**
 * Compatibility mirrors for legacy imports.
 * New code should prefer getters/setters below.
 */
export let serverUrl = state.serverUrl;
export let debugMode = state.debugMode;
export let connectionStatus = state.connectionStatus;
export let currentLogLevel = state.currentLogLevel;
export let screenshotOnError = state.screenshotOnError;
export let _captureOverrides = state.captureOverrides;
export let aiControlled = state.aiControlled;
export let _connectionCheckRunning = state.connectionCheckRunning;
export let __aiWebPilotEnabledCache = state.aiWebPilotEnabledCache;
export let __aiWebPilotCacheInitialized = state.aiWebPilotCacheInitialized;
export let __pilotInitCallback = state.pilotInitCallback;
export const extensionLogQueue = state.extensionLogQueue;
function syncLegacyExports() {
    serverUrl = state.serverUrl;
    debugMode = state.debugMode;
    connectionStatus = state.connectionStatus;
    currentLogLevel = state.currentLogLevel;
    screenshotOnError = state.screenshotOnError;
    _captureOverrides = state.captureOverrides;
    aiControlled = state.aiControlled;
    _connectionCheckRunning = state.connectionCheckRunning;
    __aiWebPilotEnabledCache = state.aiWebPilotEnabledCache;
    __aiWebPilotCacheInitialized = state.aiWebPilotCacheInitialized;
    __pilotInitCallback = state.pilotInitCallback;
}
export function getServerUrl() {
    return state.serverUrl;
}
export function isDebugMode() {
    return state.debugMode;
}
export function getConnectionStatus() {
    return state.connectionStatus;
}
export function getCurrentLogLevel() {
    return state.currentLogLevel;
}
export function isScreenshotOnError() {
    return state.screenshotOnError;
}
export function getCaptureOverrides() {
    return state.captureOverrides;
}
export function isAiControlled() {
    return state.aiControlled;
}
export function isConnectionCheckRunning() {
    return state.connectionCheckRunning;
}
export function isAiWebPilotCacheInitialized() {
    return state.aiWebPilotCacheInitialized;
}
export function getPilotInitCallback() {
    return state.pilotInitCallback;
}
export function getExtensionLogQueue() {
    return state.extensionLogQueue;
}
export function clearExtensionLogQueue() {
    state.extensionLogQueue.length = 0;
}
export function pushExtensionLog(entry) {
    state.extensionLogQueue.push(entry);
}
function capExtensionLogQueue(maxEntries) {
    if (state.extensionLogQueue.length <= maxEntries)
        return;
    state.extensionLogQueue.splice(0, state.extensionLogQueue.length - maxEntries);
}
export function capExtensionLogs(maxEntries) {
    capExtensionLogQueue(maxEntries);
}
const defaultConnectionStatus = {
    connected: false,
    entries: 0,
    maxEntries: 1000,
    errorCount: 0,
    logFile: '',
    securityMode: 'normal',
    productionParity: true,
    insecureRewritesApplied: []
};
/** Init-ready gate: resolves when initialization completes so early commands wait for cache */
let _initResolve = null;
export const initReady = new Promise((resolve) => {
    _initResolve = resolve;
});
export function markInitComplete() {
    if (_initResolve) {
        _initResolve();
        _initResolve = null;
    }
}
// =============================================================================
// STATE SETTERS
// =============================================================================
export function setServerUrl(url) {
    state.serverUrl = url;
    syncLegacyExports();
}
/** Low-level flag setter. Use index.setDebugMode for the version that also logs. */
export function _setDebugModeRaw(enabled) {
    state.debugMode = enabled;
    syncLegacyExports();
}
export function setCurrentLogLevel(level) {
    state.currentLogLevel = level;
    syncLegacyExports();
}
export function setScreenshotOnError(enabled) {
    state.screenshotOnError = enabled;
    syncLegacyExports();
}
export function setConnectionStatus(patch) {
    state.connectionStatus = { ...state.connectionStatus, ...patch };
    syncLegacyExports();
}
export function setConnectionCheckRunning(running) {
    state.connectionCheckRunning = running;
    syncLegacyExports();
}
export function setAiWebPilotEnabledCache(enabled) {
    state.aiWebPilotEnabledCache = enabled;
    syncLegacyExports();
}
export function setAiWebPilotCacheInitialized(initialized) {
    state.aiWebPilotCacheInitialized = initialized;
    syncLegacyExports();
}
export function setPilotInitCallback(callback) {
    state.pilotInitCallback = callback;
    syncLegacyExports();
}
export function applyCaptureOverrides(overrides) {
    state.captureOverrides = overrides;
    state.aiControlled = Object.keys(overrides).length > 0;
    if (overrides.log_level !== undefined) {
        state.currentLogLevel = overrides.log_level;
    }
    if (overrides.screenshot_on_error !== undefined) {
        state.screenshotOnError = overrides.screenshot_on_error === 'true';
    }
    const securityMode = overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal';
    const productionParity = overrides.production_parity !== 'false';
    const rewritesRaw = overrides.insecure_rewrites_applied || '';
    const rewrites = rewritesRaw
        .split(',')
        .map((v) => v.trim())
        .filter((v) => v.length > 0);
    state.connectionStatus = {
        ...state.connectionStatus,
        securityMode,
        productionParity,
        insecureRewritesApplied: rewrites
    };
    syncLegacyExports();
}
/**
 * Reset pilot cache for testing
 */
export function _resetPilotCacheForTesting(value) {
    state.aiWebPilotEnabledCache = value !== undefined ? value : false;
    syncLegacyExports();
}
/**
 * Check if AI Web Pilot is enabled
 */
export function isAiWebPilotEnabled() {
    return state.aiWebPilotEnabledCache === true;
}
export function resetStateForTesting() {
    state.serverUrl = DEFAULT_SERVER_URL;
    state.debugMode = false;
    state.connectionStatus = { ...defaultConnectionStatus };
    state.currentLogLevel = 'all';
    state.screenshotOnError = false;
    state.captureOverrides = {};
    state.aiControlled = false;
    state.connectionCheckRunning = false;
    state.aiWebPilotEnabledCache = false;
    state.aiWebPilotCacheInitialized = false;
    state.pilotInitCallback = null;
    state.extensionLogQueue.length = 0;
    syncLegacyExports();
}
//# sourceMappingURL=state.js.map