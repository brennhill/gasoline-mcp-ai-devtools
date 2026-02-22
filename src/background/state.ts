/**
 * @fileoverview Mutable module-level state for the background service worker.
 * Owns all `let` variables and their setter functions so that state ownership
 * is explicit and separated from business logic in index.ts.
 */

import { DEFAULT_SERVER_URL } from '../lib/constants'

// =============================================================================
// MODULE STATE
// =============================================================================

/** Session ID for detecting extension reloads */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`

/** Server URL */
export let serverUrl = DEFAULT_SERVER_URL

/** Debug mode flag */
export let debugMode = false

/** Connection status (mutable internal state) */
export interface MutableConnectionStatus {
  connected: boolean
  entries: number
  maxEntries: number
  errorCount: number
  logFile: string
  logFileSize?: number
  serverVersion?: string
  extensionVersion?: string
  versionMismatch?: boolean
  securityMode?: 'normal' | 'insecure_proxy'
  productionParity?: boolean
  insecureRewritesApplied?: string[]
}

export let connectionStatus: MutableConnectionStatus = {
  connected: false,
  entries: 0,
  maxEntries: 1000,
  errorCount: 0,
  logFile: '',
  securityMode: 'normal',
  productionParity: true,
  insecureRewritesApplied: []
}

/** Log level filter */
export let currentLogLevel = 'all'

/** Screenshot settings */
export let screenshotOnError = false

/** AI capture control state */
export let _captureOverrides: Record<string, string> = {}
export let aiControlled = false

/** Connection check mutex */
export let _connectionCheckRunning = false

/** AI Web Pilot state */
export let __aiWebPilotEnabledCache = true
export let __aiWebPilotCacheInitialized = false
export let __pilotInitCallback: (() => void) | null = null

/** Init-ready gate: resolves when initialization completes so early commands wait for cache */
let _initResolve: (() => void) | null = null
export const initReady: Promise<void> = new Promise((resolve) => {
  _initResolve = resolve
})
export function markInitComplete(): void {
  if (_initResolve) {
    _initResolve()
    _initResolve = null
  }
}

/** Extension log queue for server posting */
export const extensionLogQueue: Array<{
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}> = []

// =============================================================================
// STATE SETTERS
// =============================================================================

export function setServerUrl(url: string): void {
  serverUrl = url
}

/** Low-level flag setter. Use index.setDebugMode for the version that also logs. */
export function _setDebugModeRaw(enabled: boolean): void {
  debugMode = enabled
}

export function setCurrentLogLevel(level: string): void {
  currentLogLevel = level
}

export function setScreenshotOnError(enabled: boolean): void {
  screenshotOnError = enabled
}

export function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void {
  connectionStatus = { ...connectionStatus, ...patch }
}

export function setConnectionCheckRunning(running: boolean): void {
  _connectionCheckRunning = running
}

export function setAiWebPilotEnabledCache(enabled: boolean): void {
  __aiWebPilotEnabledCache = enabled
}

export function setAiWebPilotCacheInitialized(initialized: boolean): void {
  __aiWebPilotCacheInitialized = initialized
}

export function setPilotInitCallback(callback: (() => void) | null): void {
  __pilotInitCallback = callback
}

export function applyCaptureOverrides(overrides: Record<string, string>): void {
  _captureOverrides = overrides
  aiControlled = Object.keys(overrides).length > 0

  if (overrides.log_level !== undefined) {
    currentLogLevel = overrides.log_level
  }
  if (overrides.screenshot_on_error !== undefined) {
    screenshotOnError = overrides.screenshot_on_error === 'true'
  }

  const securityMode = overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal'
  const productionParity = overrides.production_parity !== 'false'
  const rewritesRaw = overrides.insecure_rewrites_applied || ''
  const rewrites = rewritesRaw
    .split(',')
    .map((v) => v.trim())
    .filter((v) => v.length > 0)

  connectionStatus = {
    ...connectionStatus,
    securityMode,
    productionParity,
    insecureRewritesApplied: rewrites
  }
}

/**
 * Reset pilot cache for testing
 */
export function _resetPilotCacheForTesting(value?: boolean): void {
  __aiWebPilotEnabledCache = value !== undefined ? value : false
}

/**
 * Check if AI Web Pilot is enabled
 */
export function isAiWebPilotEnabled(): boolean {
  return __aiWebPilotEnabledCache === true
}
