/**
 * @fileoverview Storage Utilities - Wrapper functions for chrome.storage with support for both
 * persistent (local) and ephemeral (session) storage.
 *
 * Usage:
 * - Ephemeral state (resets on service worker restart): use session storage
 *   * trackedTabId, trackedTabUrl
 *   * debugMode (user preference is persistent, but cache resets on restart)
 *   * aiWebPilotEnabled cache
 *
 * - Persistent state (survives browser restart): use local storage
 *   * serverUrl (user setting)
 *   * logLevel (user preference)
 *   * screenshotOnError (user preference)
 *   * sourceMapEnabled (user preference)
 *   * state snapshots
 *
 * Note: chrome.storage.session only available in Chrome 102+
 * This module handles graceful degradation for older versions
 */
// =============================================================================
// FEATURE DETECTION
// =============================================================================
/**
 * Type-safe access to chrome.storage with session storage support
 * Chrome.storage.session is only available in Chrome 102+
 */
function getStorageWithSession() {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return null;
    return chrome.storage;
}
/**
 * Check if chrome.storage.session is available (Chrome 102+)
 */
function isSessionStorageAvailable() {
    const storage = getStorageWithSession();
    return storage !== null && storage.session !== undefined;
}
// =============================================================================
// SESSION STORAGE UTILITIES (ephemeral, resets on service worker restart)
// =============================================================================
/**
 * Set an ephemeral value in session storage (callback-based)
 * Falls back to memory for older Chrome versions
 */
export function setSessionValue(key, value, callback) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        // Graceful degradation: store in memory (will be lost on service worker restart anyway)
        if (callback)
            callback();
        return;
    }
    storage.session.set({ [key]: value }, () => {
        if (callback)
            callback();
    });
}
/**
 * Get an ephemeral value from session storage (callback-based)
 * Falls back to undefined for older Chrome versions
 */
export function getSessionValue(key, callback) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        callback(undefined);
        return;
    }
    storage.session.get([key], (result) => {
        // eslint-disable-next-line security/detect-object-injection -- Safe: key is string parameter from caller
        callback(result[key]);
    });
}
/**
 * Remove an ephemeral value from session storage (callback-based)
 */
export function removeSessionValue(key, callback) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        if (callback)
            callback();
        return;
    }
    storage.session.remove([key], () => {
        if (callback)
            callback();
    });
}
/**
 * Clear all ephemeral values from session storage (callback-based)
 */
export function clearSessionStorage(callback) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        if (callback)
            callback();
        return;
    }
    storage.session.clear(() => {
        if (callback)
            callback();
    });
}
// =============================================================================
// LOCAL STORAGE UTILITIES (persistent, survives browser restart)
// =============================================================================
/**
 * Set a persistent value in local storage (callback-based)
 */
export function setLocalValue(key, value, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        if (callback)
            callback();
        return;
    }
    chrome.storage.local.set({ [key]: value }, () => {
        if (chrome.runtime.lastError) {
            console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message);
        }
        if (callback)
            callback();
    });
}
/**
 * Get a persistent value from local storage (callback-based)
 */
export function getLocalValue(key, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback(undefined);
        return;
    }
    chrome.storage.local.get([key], (result) => {
        if (chrome.runtime.lastError) {
            console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message);
            callback(undefined);
            return;
        }
        // eslint-disable-next-line security/detect-object-injection -- Safe: key is string parameter from caller
        callback(result[key]);
    });
}
/**
 * Remove a persistent value from local storage (callback-based)
 */
export function removeLocalValue(key, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        if (callback)
            callback();
        return;
    }
    chrome.storage.local.remove([key], () => {
        if (callback)
            callback();
    });
}
// =============================================================================
// FACADE FUNCTIONS - Choose storage area automatically
// =============================================================================
/**
 * Set a value in the appropriate storage area (callback-based)
 * For ephemeral data, prefers session storage (Chrome 102+), falls back to memory
 * For persistent data, uses local storage
 */
export function setValue(key, value, areaName, callback) {
    const area = areaName || 'session';
    if (area === 'session') {
        setSessionValue(key, value, callback);
    }
    else if (area === 'local') {
        setLocalValue(key, value, callback);
    }
    else {
        if (callback)
            callback();
    }
}
/**
 * Get a value from the appropriate storage area (callback-based)
 */
export function getValue(key, areaName, callback) {
    const area = areaName || 'session';
    if (area === 'session') {
        getSessionValue(key, callback);
    }
    else if (area === 'local') {
        getLocalValue(key, callback);
    }
    else {
        callback(undefined);
    }
}
/**
 * Remove a value from the appropriate storage area (callback-based)
 */
export function removeValue(key, areaName, callback) {
    const area = areaName || 'session';
    if (area === 'session') {
        removeSessionValue(key, callback);
    }
    else if (area === 'local') {
        removeLocalValue(key, callback);
    }
    else {
        if (callback)
            callback();
    }
}
// =============================================================================
// STATE RECOVERY & DIAGNOSTICS
// =============================================================================
/**
 * Get diagnostic info about storage availability
 */
export function getStorageDiagnostics() {
    return {
        sessionStorageAvailable: isSessionStorageAvailable(),
        localStorageAvailable: typeof chrome !== 'undefined' && !!chrome.storage?.local,
        browserVersion: navigator.userAgent,
    };
}
/**
 * State version key for recovery detection
 */
const STATE_VERSION_KEY = 'gasoline_state_version';
const CURRENT_STATE_VERSION = '1.0.0';
/**
 * Check if service worker was restarted (state version mismatch)
 * Returns true if state was lost/cleared (callback-based)
 */
export function wasServiceWorkerRestarted(callback) {
    if (!isSessionStorageAvailable()) {
        // Can't detect restart without session storage
        callback(false);
        return;
    }
    getSessionValue(STATE_VERSION_KEY, (storedVersion) => {
        callback(storedVersion !== CURRENT_STATE_VERSION);
    });
}
/**
 * Mark the current state version (call on init) - callback-based
 */
export function markStateVersion(callback) {
    setSessionValue(STATE_VERSION_KEY, CURRENT_STATE_VERSION, callback);
}
//# sourceMappingURL=storage-utils.js.map