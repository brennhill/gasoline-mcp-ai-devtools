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

export interface ExtensionLogQueueEntry {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

interface BackgroundStateStore {
  serverUrl: string
  debugMode: boolean
  connectionStatus: MutableConnectionStatus
  currentLogLevel: string
  screenshotOnError: boolean
  captureOverrides: Record<string, string>
  aiControlled: boolean
  connectionCheckRunning: boolean
  aiWebPilotEnabledCache: boolean
  aiWebPilotCacheInitialized: boolean
  pilotInitCallback: (() => void) | null
  extensionLogQueue: ExtensionLogQueueEntry[]
}

const state: BackgroundStateStore = {
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
}

/**
 * Compatibility mirrors for legacy imports.
 * New code should prefer getters/setters below.
 */
export let serverUrl = state.serverUrl
export let debugMode = state.debugMode
export let connectionStatus: MutableConnectionStatus = state.connectionStatus
export let currentLogLevel = state.currentLogLevel
export let screenshotOnError = state.screenshotOnError
export let _captureOverrides: Record<string, string> = state.captureOverrides
export let aiControlled = state.aiControlled
export let _connectionCheckRunning = state.connectionCheckRunning
export let __aiWebPilotEnabledCache = state.aiWebPilotEnabledCache
export let __aiWebPilotCacheInitialized = state.aiWebPilotCacheInitialized
export let __pilotInitCallback: (() => void) | null = state.pilotInitCallback
export const extensionLogQueue = state.extensionLogQueue

function syncLegacyExports(): void {
  serverUrl = state.serverUrl
  debugMode = state.debugMode
  connectionStatus = state.connectionStatus
  currentLogLevel = state.currentLogLevel
  screenshotOnError = state.screenshotOnError
  _captureOverrides = state.captureOverrides
  aiControlled = state.aiControlled
  _connectionCheckRunning = state.connectionCheckRunning
  __aiWebPilotEnabledCache = state.aiWebPilotEnabledCache
  __aiWebPilotCacheInitialized = state.aiWebPilotCacheInitialized
  __pilotInitCallback = state.pilotInitCallback
}

export function getServerUrl(): string {
  return state.serverUrl
}

export function isDebugMode(): boolean {
  return state.debugMode
}

export function getConnectionStatus(): MutableConnectionStatus {
  return state.connectionStatus
}

export function getCurrentLogLevel(): string {
  return state.currentLogLevel
}

export function isScreenshotOnError(): boolean {
  return state.screenshotOnError
}

export function getCaptureOverrides(): Record<string, string> {
  return state.captureOverrides
}

export function isAiControlled(): boolean {
  return state.aiControlled
}

export function isConnectionCheckRunning(): boolean {
  return state.connectionCheckRunning
}

export function isAiWebPilotCacheInitialized(): boolean {
  return state.aiWebPilotCacheInitialized
}

export function getPilotInitCallback(): (() => void) | null {
  return state.pilotInitCallback
}

export function getExtensionLogQueue(): ExtensionLogQueueEntry[] {
  return state.extensionLogQueue
}

export function clearExtensionLogQueue(): void {
  state.extensionLogQueue.length = 0
}

export function pushExtensionLog(entry: ExtensionLogQueueEntry): void {
  state.extensionLogQueue.push(entry)
}

function capExtensionLogQueue(maxEntries: number): void {
  if (state.extensionLogQueue.length <= maxEntries) return
  state.extensionLogQueue.splice(0, state.extensionLogQueue.length - maxEntries)
}

export function capExtensionLogs(maxEntries: number): void {
  capExtensionLogQueue(maxEntries)
}

const defaultConnectionStatus: MutableConnectionStatus = {
  connected: false,
  entries: 0,
  maxEntries: 1000,
  errorCount: 0,
  logFile: '',
  securityMode: 'normal',
  productionParity: true,
  insecureRewritesApplied: []
}

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

// =============================================================================
// STATE SETTERS
// =============================================================================

export function setServerUrl(url: string): void {
  state.serverUrl = url
  syncLegacyExports()
}

/** Low-level flag setter. Use index.setDebugMode for the version that also logs. */
export function _setDebugModeRaw(enabled: boolean): void {
  state.debugMode = enabled
  syncLegacyExports()
}

export function setCurrentLogLevel(level: string): void {
  state.currentLogLevel = level
  syncLegacyExports()
}

export function setScreenshotOnError(enabled: boolean): void {
  state.screenshotOnError = enabled
  syncLegacyExports()
}

export function setConnectionStatus(patch: Partial<MutableConnectionStatus>): void {
  state.connectionStatus = { ...state.connectionStatus, ...patch }
  syncLegacyExports()
}

export function setConnectionCheckRunning(running: boolean): void {
  state.connectionCheckRunning = running
  syncLegacyExports()
}

export function setAiWebPilotEnabledCache(enabled: boolean): void {
  state.aiWebPilotEnabledCache = enabled
  syncLegacyExports()
}

export function setAiWebPilotCacheInitialized(initialized: boolean): void {
  state.aiWebPilotCacheInitialized = initialized
  syncLegacyExports()
}

export function setPilotInitCallback(callback: (() => void) | null): void {
  state.pilotInitCallback = callback
  syncLegacyExports()
}

export function applyCaptureOverrides(overrides: Record<string, string>): void {
  state.captureOverrides = overrides
  state.aiControlled = Object.keys(overrides).length > 0

  if (overrides.log_level !== undefined) {
    state.currentLogLevel = overrides.log_level
  }
  if (overrides.screenshot_on_error !== undefined) {
    state.screenshotOnError = overrides.screenshot_on_error === 'true'
  }

  const securityMode = overrides.security_mode === 'insecure_proxy' ? 'insecure_proxy' : 'normal'
  const productionParity = overrides.production_parity !== 'false'
  const rewritesRaw = overrides.insecure_rewrites_applied || ''
  const rewrites = rewritesRaw
    .split(',')
    .map((v) => v.trim())
    .filter((v) => v.length > 0)

  state.connectionStatus = {
    ...state.connectionStatus,
    securityMode,
    productionParity,
    insecureRewritesApplied: rewrites
  }
  syncLegacyExports()
}

/**
 * Reset pilot cache for testing
 */
export function _resetPilotCacheForTesting(value?: boolean): void {
  state.aiWebPilotEnabledCache = value !== undefined ? value : false
  syncLegacyExports()
}

/**
 * Check if AI Web Pilot is enabled
 */
export function isAiWebPilotEnabled(): boolean {
  return state.aiWebPilotEnabledCache === true
}

export function resetStateForTesting(): void {
  state.serverUrl = DEFAULT_SERVER_URL
  state.debugMode = false
  state.connectionStatus = { ...defaultConnectionStatus }
  state.currentLogLevel = 'all'
  state.screenshotOnError = false
  state.captureOverrides = {}
  state.aiControlled = false
  state.connectionCheckRunning = false
  state.aiWebPilotEnabledCache = false
  state.aiWebPilotCacheInitialized = false
  state.pilotInitCallback = null
  state.extensionLogQueue.length = 0
  syncLegacyExports()
}
