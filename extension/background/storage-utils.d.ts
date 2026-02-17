/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
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
import type { StorageAreaName } from '../types';
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
 * Get a persistent value from local storage (callback-based)
 */
export declare function getLocalValue(key: string, callback: (value: unknown) => void): void;
/**
 * Remove a persistent value from local storage (callback-based)
 */
export declare function removeLocalValue(key: string, callback?: () => void): void;
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
 * Returns true if state was lost/cleared (callback-based)
 */
export declare function wasServiceWorkerRestarted(callback: (wasRestarted: boolean) => void): void;
/**
 * Mark the current state version (call on init) - callback-based
 */
export declare function markStateVersion(callback?: () => void): void;
//# sourceMappingURL=storage-utils.d.ts.map