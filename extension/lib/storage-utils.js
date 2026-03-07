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
// LOCAL STORAGE (Promise-based)
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
// SESSION STORAGE (Promise-based)
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