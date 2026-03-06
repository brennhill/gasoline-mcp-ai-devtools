/**
 * Purpose: Shared wrapper functions for chrome.storage supporting persistent (local) and ephemeral (session) storage with graceful degradation.
 * Why: Abstracts Chrome storage API differences and provides a single facade usable from both background and popup contexts.
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
            console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal storage key, not user-controlled
        }
        if (callback)
            callback();
    });
}
/**
 * Set multiple persistent values in local storage (callback-based)
 */
export function setLocalValues(items, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        if (callback)
            callback();
        return;
    }
    chrome.storage.local.set(items, () => {
        if (chrome.runtime.lastError) {
            console.warn('[Gasoline] Storage error setting multiple keys:', chrome.runtime.lastError.message);
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
            console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message); // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal storage key, not user-controlled
            callback(undefined);
            return;
        }
        callback(result[key]);
    });
}
/**
 * Get multiple persistent values from local storage (callback-based)
 */
export function getLocalValues(keys, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback({});
        return;
    }
    chrome.storage.local.get(keys, (result) => {
        if (chrome.runtime.lastError) {
            console.warn('[Gasoline] Storage error getting multiple keys:', chrome.runtime.lastError.message);
            callback({});
            return;
        }
        callback(result);
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
/**
 * Remove multiple persistent values from local storage (callback-based)
 */
export function removeLocalValues(keys, callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        if (callback)
            callback();
        return;
    }
    chrome.storage.local.remove(keys, () => {
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
// ASYNC LOCAL STORAGE (Promise-based — preferred API for new code)
// =============================================================================
/**
 * Get a persistent value from local storage (async)
 */
export async function getLocal(key) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return undefined;
    const result = await chrome.storage.local.get([key]);
    return result[key];
}
/**
 * Get multiple persistent values from local storage (async)
 */
export async function getLocals(keys) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return {};
    return await chrome.storage.local.get(keys);
}
/**
 * Set a persistent value in local storage (async)
 */
export async function setLocal(key, value) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await chrome.storage.local.set({ [key]: value });
}
/**
 * Set multiple persistent values in local storage (async)
 */
export async function setLocals(items) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await chrome.storage.local.set(items);
}
/**
 * Remove a persistent value from local storage (async)
 */
export async function removeLocal(key) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await chrome.storage.local.remove([key]);
}
/**
 * Remove multiple persistent values from local storage (async)
 */
export async function removeLocals(keys) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    await chrome.storage.local.remove(keys);
}
// =============================================================================
// ASYNC SESSION STORAGE (Promise-based — preferred API for new code)
// =============================================================================
/**
 * Get an ephemeral value from session storage (async)
 */
export async function getSession(key) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session)
        return undefined;
    const result = await storage.session.get([key]);
    return result[key];
}
/**
 * Set an ephemeral value in session storage (async)
 */
export async function setSession(key, value) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session)
        return;
    await storage.session.set({ [key]: value });
}
/**
 * Remove an ephemeral value from session storage (async)
 */
export async function removeSession(key) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session)
        return;
    await storage.session.remove([key]);
}
/**
 * Remove multiple ephemeral values from session storage (async)
 */
export async function removeSessions(keys) {
    const storage = getStorageWithSession();
    if (!storage || !storage.session)
        return;
    await storage.session.remove(keys);
}
/**
 * Register a storage change listener. Returns an unsubscribe function.
 */
export function onStorageChanged(listener) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return () => { };
    chrome.storage.onChanged.addListener(listener);
    return () => chrome.storage.onChanged.removeListener(listener);
}
// =============================================================================
// SESSION ACCESS LEVEL
// =============================================================================
/**
 * Set session storage access level (e.g., to allow content scripts access).
 * Required for terminal state persistence in content scripts.
 */
export async function setSessionAccessLevel(accessLevel) {
    const storage = getStorageWithSession();
    if (!storage?.session?.setAccessLevel)
        return;
    await storage.session.setAccessLevel({ accessLevel });
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
        browserVersion: navigator.userAgent
    };
}
/**
 * State version key for recovery detection
 */
const STATE_VERSION_KEY = 'gasoline_state_version';
const CURRENT_STATE_VERSION = '1.0.0';
/**
 * Check if service worker was restarted (state version mismatch)
 * Returns true if state was lost/cleared
 */
export async function wasServiceWorkerRestarted() {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        // Can't detect restart without session storage
        return false;
    }
    const result = await storage.session.get([STATE_VERSION_KEY]);
    return result[STATE_VERSION_KEY] !== CURRENT_STATE_VERSION;
}
/**
 * Mark the current state version (call on init)
 */
export async function markStateVersion() {
    const storage = getStorageWithSession();
    if (!storage || !storage.session) {
        return;
    }
    await storage.session.set({ [STATE_VERSION_KEY]: CURRENT_STATE_VERSION });
}
//# sourceMappingURL=storage-utils.js.map