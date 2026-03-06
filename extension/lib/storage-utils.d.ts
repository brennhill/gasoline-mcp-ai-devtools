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
//# sourceMappingURL=storage-utils.d.ts.map