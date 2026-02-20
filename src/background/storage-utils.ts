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

import type { StorageAreaName, ChromeStorageWithSession } from '../types'

// =============================================================================
// FEATURE DETECTION
// =============================================================================

/**
 * Type-safe access to chrome.storage with session storage support
 * Chrome.storage.session is only available in Chrome 102+
 */
function getStorageWithSession(): ChromeStorageWithSession | null {
  if (typeof chrome === 'undefined' || !chrome.storage) return null
  return chrome.storage as unknown as ChromeStorageWithSession
}

/**
 * Check if chrome.storage.session is available (Chrome 102+)
 */
function isSessionStorageAvailable(): boolean {
  const storage = getStorageWithSession()
  return storage !== null && storage.session !== undefined
}

// =============================================================================
// SESSION STORAGE UTILITIES (ephemeral, resets on service worker restart)
// =============================================================================

/**
 * Set an ephemeral value in session storage (callback-based)
 * Falls back to memory for older Chrome versions
 */
export function setSessionValue(key: string, value: unknown, callback?: () => void): void {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    // Graceful degradation: store in memory (will be lost on service worker restart anyway)
    if (callback) callback()
    return
  }
  storage.session.set({ [key]: value }, () => {
    if (callback) callback()
  })
}

/**
 * Get an ephemeral value from session storage (callback-based)
 * Falls back to undefined for older Chrome versions
 */
export function getSessionValue(key: string, callback: (value: unknown) => void): void {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    callback(undefined)
    return
  }
  storage.session.get([key], (result: Record<string, unknown>) => {
    callback(result[key])
  })
}

/**
 * Remove an ephemeral value from session storage (callback-based)
 */
export function removeSessionValue(key: string, callback?: () => void): void {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    if (callback) callback()
    return
  }
  storage.session.remove([key], () => {
    if (callback) callback()
  })
}

/**
 * Clear all ephemeral values from session storage (callback-based)
 */
export function clearSessionStorage(callback?: () => void): void {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    if (callback) callback()
    return
  }
  storage.session.clear(() => {
    if (callback) callback()
  })
}

// =============================================================================
// LOCAL STORAGE UTILITIES (persistent, survives browser restart)
// =============================================================================

/**
 * Set a persistent value in local storage (callback-based)
 */
export function setLocalValue(key: string, value: unknown, callback?: () => void): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    if (callback) callback()
    return
  }
  chrome.storage.local.set({ [key]: value }, () => {
    if (chrome.runtime.lastError) {
      console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal storage key, not user-controlled
    }
    if (callback) callback()
  })
}

/**
 * Get a persistent value from local storage (callback-based)
 */
export function getLocalValue(key: string, callback: (value: unknown) => void): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    callback(undefined)
    return
  }
  chrome.storage.local.get([key], (result: Record<string, unknown>) => {
    if (chrome.runtime.lastError) {
      console.warn(`[Gasoline] Storage error for key ${key}:`, chrome.runtime.lastError.message) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.warn with internal storage key, not user-controlled
      callback(undefined)
      return
    }
    callback(result[key])
  })
}

/**
 * Remove a persistent value from local storage (callback-based)
 */
export function removeLocalValue(key: string, callback?: () => void): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    if (callback) callback()
    return
  }
  chrome.storage.local.remove([key], () => {
    if (callback) callback()
  })
}

// =============================================================================
// FACADE FUNCTIONS - Choose storage area automatically
// =============================================================================

/**
 * Set a value in the appropriate storage area (callback-based)
 * For ephemeral data, prefers session storage (Chrome 102+), falls back to memory
 * For persistent data, uses local storage
 */
export function setValue(key: string, value: unknown, areaName?: StorageAreaName, callback?: () => void): void {
  const area = areaName || 'session'
  if (area === 'session') {
    setSessionValue(key, value, callback)
  } else if (area === 'local') {
    setLocalValue(key, value, callback)
  } else {
    if (callback) callback()
  }
}

/**
 * Get a value from the appropriate storage area (callback-based)
 */
export function getValue(key: string, areaName: StorageAreaName | undefined, callback: (value: unknown) => void): void {
  const area = areaName || 'session'
  if (area === 'session') {
    getSessionValue(key, callback)
  } else if (area === 'local') {
    getLocalValue(key, callback)
  } else {
    callback(undefined)
  }
}

/**
 * Remove a value from the appropriate storage area (callback-based)
 */
export function removeValue(key: string, areaName?: StorageAreaName, callback?: () => void): void {
  const area = areaName || 'session'
  if (area === 'session') {
    removeSessionValue(key, callback)
  } else if (area === 'local') {
    removeLocalValue(key, callback)
  } else {
    if (callback) callback()
  }
}

// =============================================================================
// STATE RECOVERY & DIAGNOSTICS
// =============================================================================

/**
 * Get diagnostic info about storage availability
 */
export function getStorageDiagnostics(): {
  sessionStorageAvailable: boolean
  localStorageAvailable: boolean
  browserVersion: string
} {
  return {
    sessionStorageAvailable: isSessionStorageAvailable(),
    localStorageAvailable: typeof chrome !== 'undefined' && !!chrome.storage?.local,
    browserVersion: navigator.userAgent
  }
}

/**
 * State version key for recovery detection
 */
const STATE_VERSION_KEY = 'gasoline_state_version'
const CURRENT_STATE_VERSION = '1.0.0'

/**
 * Check if service worker was restarted (state version mismatch)
 * Returns true if state was lost/cleared
 */
export async function wasServiceWorkerRestarted(): Promise<boolean> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    // Can't detect restart without session storage
    return false
  }
  const result = await storage.session.get([STATE_VERSION_KEY])
  return result[STATE_VERSION_KEY] !== CURRENT_STATE_VERSION
}

/**
 * Mark the current state version (call on init)
 */
export async function markStateVersion(): Promise<void> {
  const storage = getStorageWithSession()
  if (!storage || !storage.session) {
    return
  }
  await storage.session.set({ [STATE_VERSION_KEY]: CURRENT_STATE_VERSION })
}
