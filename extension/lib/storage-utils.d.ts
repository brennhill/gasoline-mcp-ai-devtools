/**
 * Purpose: Shared wrapper functions for chrome.storage supporting persistent (local) and ephemeral (session) storage with graceful degradation.
 * Why: Abstracts Chrome storage API differences and provides a single facade usable from both background and popup contexts.
 */
import type { StorageAreaName } from '../types/index.js';
/**
 * Set an ephemeral value in session storage (callback-based)
 * Falls back to memory for older Chrome versions
 */
export declare function setSessionValue(key: string, value: unknown, callback?: () => void): void;
/**
 * Get an ephemeral value from session storage (callback-based)
 * Falls back to undefined for older Chrome versions
 */
export declare function getSessionValue(key: string, callback: (value: unknown) => void): void;
/**
 * Remove an ephemeral value from session storage (callback-based)
 */
export declare function removeSessionValue(key: string, callback?: () => void): void;
/**
 * Clear all ephemeral values from session storage (callback-based)
 */
export declare function clearSessionStorage(callback?: () => void): void;
/**
 * Set a persistent value in local storage (callback-based)
 */
export declare function setLocalValue(key: string, value: unknown, callback?: () => void): void;
/**
 * Set multiple persistent values in local storage (callback-based)
 */
export declare function setLocalValues(items: Record<string, unknown>, callback?: () => void): void;
/**
 * Get a persistent value from local storage (callback-based)
 */
export declare function getLocalValue(key: string, callback: (value: unknown) => void): void;
/**
 * Get multiple persistent values from local storage (callback-based)
 */
export declare function getLocalValues(keys: string[], callback: (result: Record<string, unknown>) => void): void;
/**
 * Remove a persistent value from local storage (callback-based)
 */
export declare function removeLocalValue(key: string, callback?: () => void): void;
/**
 * Remove multiple persistent values from local storage (callback-based)
 */
export declare function removeLocalValues(keys: string[], callback?: () => void): void;
/**
 * Set a value in the appropriate storage area (callback-based)
 * For ephemeral data, prefers session storage (Chrome 102+), falls back to memory
 * For persistent data, uses local storage
 */
export declare function setValue(key: string, value: unknown, areaName?: StorageAreaName, callback?: () => void): void;
/**
 * Get a value from the appropriate storage area (callback-based)
 */
export declare function getValue(key: string, areaName: StorageAreaName | undefined, callback: (value: unknown) => void): void;
/**
 * Remove a value from the appropriate storage area (callback-based)
 */
export declare function removeValue(key: string, areaName?: StorageAreaName, callback?: () => void): void;
/**
 * Get a persistent value from local storage (async)
 */
export declare function getLocal(key: string): Promise<unknown>;
/**
 * Get multiple persistent values from local storage (async)
 */
export declare function getLocals(keys: string[]): Promise<Record<string, unknown>>;
/**
 * Set a persistent value in local storage (async)
 */
export declare function setLocal(key: string, value: unknown): Promise<void>;
/**
 * Set multiple persistent values in local storage (async)
 */
export declare function setLocals(items: Record<string, unknown>): Promise<void>;
/**
 * Remove a persistent value from local storage (async)
 */
export declare function removeLocal(key: string): Promise<void>;
/**
 * Remove multiple persistent values from local storage (async)
 */
export declare function removeLocals(keys: string[]): Promise<void>;
/**
 * Get an ephemeral value from session storage (async)
 */
export declare function getSession(key: string): Promise<unknown>;
/**
 * Set an ephemeral value in session storage (async)
 */
export declare function setSession(key: string, value: unknown): Promise<void>;
/**
 * Remove an ephemeral value from session storage (async)
 */
export declare function removeSession(key: string): Promise<void>;
/**
 * Remove multiple ephemeral values from session storage (async)
 */
export declare function removeSessions(keys: string[]): Promise<void>;
type StorageChange = {
    oldValue?: unknown;
    newValue?: unknown;
};
type StorageChangeListener = (changes: {
    [key: string]: StorageChange;
}, areaName: string) => void;
/**
 * Register a storage change listener. Returns an unsubscribe function.
 */
export declare function onStorageChanged(listener: StorageChangeListener): () => void;
/**
 * Set session storage access level (e.g., to allow content scripts access).
 * Required for terminal state persistence in content scripts.
 */
export declare function setSessionAccessLevel(accessLevel: 'TRUSTED_CONTEXTS' | 'TRUSTED_AND_UNTRUSTED_CONTEXTS'): Promise<void>;
/**
 * Get diagnostic info about storage availability
 */
export declare function getStorageDiagnostics(): {
    sessionStorageAvailable: boolean;
    localStorageAvailable: boolean;
    browserVersion: string;
};
/**
 * Check if service worker was restarted (state version mismatch)
 * Returns true if state was lost/cleared
 */
export declare function wasServiceWorkerRestarted(): Promise<boolean>;
/**
 * Mark the current state version (call on init)
 */
export declare function markStateVersion(): Promise<void>;
export {};
//# sourceMappingURL=storage-utils.d.ts.map