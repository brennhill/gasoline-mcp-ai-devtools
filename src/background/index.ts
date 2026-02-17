/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Main Background Service Worker
 * Manages server communication, batchers, log handling, and pending query processing.
 * Receives captured events from content scripts, batches them with debouncing,
 * and posts to the Go server. Handles error deduplication, connection status,
 * badge updates, and on-demand query polling.
 */

import type {
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot,
  ConnectionStatus,
  ChromeMessageSender,
  PendingQuery,
  WaterfallEntry
} from '../types'

import * as stateManager from './state-manager'
import * as communication from './communication'
import * as eventListeners from './event-listeners'
import { DebugCategory } from './debug'
import { getRequestHeaders } from './server'
import {
  installMessageListener,
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot,
  type MessageHandlerDependencies
} from './message-handlers'
import {
  handlePendingQuery as handlePendingQueryImpl,
  handlePilotCommand as handlePilotCommandImpl
} from './pending-queries'
import { createSyncClient, type SyncClient, type SyncCommand, type SyncSettings } from './sync-client'
import { updateVersionFromHealth } from './version-check'

// =============================================================================
// CONSTANTS
// =============================================================================

export const DEFAULT_SERVER_URL = 'http://localhost:7890'

// =============================================================================
// MODULE STATE
// =============================================================================

/** Session ID for detecting extension reloads */
export const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`

// All communication now uses unified /sync endpoint

/** Sync client instance (initialized lazily) */
let syncClient: SyncClient | null = null

/** Server URL */
export let serverUrl = DEFAULT_SERVER_URL

/** Debug mode flag */
export let debugMode = false

/** Connection status (mutable internal state) */
interface MutableConnectionStatus {
  connected: boolean
  entries: number
  maxEntries: number
  errorCount: number
  logFile: string
  logFileSize?: number
  serverVersion?: string
  extensionVersion?: string
  versionMismatch?: boolean
}

export let connectionStatus: MutableConnectionStatus = {
  connected: false,
  entries: 0,
  maxEntries: 1000,
  errorCount: 0,
  logFile: ''
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
export let __aiWebPilotEnabledCache = false
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
// STATE SETTERS (for init.ts)
// =============================================================================
// Note: setDebugMode is defined later in the file

export function setServerUrl(url: string): void {
  serverUrl = url
}

export function setCurrentLogLevel(level: string): void {
  currentLogLevel = level
}

export function setScreenshotOnError(enabled: boolean): void {
  screenshotOnError = enabled
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

// =============================================================================
// DEBUG LOGGING
// =============================================================================

// Re-export DebugCategory from debug module (to avoid circular dependencies)
export { DebugCategory } from './debug'

/**
 * Log a diagnostic message only when debug mode is enabled
 */
export function diagnosticLog(message: string): void {
  if (debugMode) {
    console.log(message)
  }
}

/**
 * Log a debug message (only when debug mode is enabled)
 */
export function debugLog(category: string, message: string, data: unknown = null): void {
  const timestamp = new Date().toISOString()
  // Cast category to DebugCategory - callers use DebugCategory constants
  const entry: import('../types').DebugLogEntry = {
    ts: timestamp,
    category: category as import('../types').DebugCategory,
    message,
    ...(data !== null ? { data } : {})
  }

  stateManager.addDebugLogEntry(entry)

  if (connectionStatus.connected) {
    extensionLogQueue.push({
      timestamp,
      level: 'debug',
      message,
      source: 'background',
      category,
      ...(data !== null ? { data } : {})
    })
    // Cap queue size to prevent memory leak if server is unreachable
    const MAX_EXTENSION_LOGS = 2000
    if (extensionLogQueue.length > MAX_EXTENSION_LOGS) {
      extensionLogQueue.splice(0, extensionLogQueue.length - MAX_EXTENSION_LOGS)
    }
  }

  if (debugMode) {
    const prefix = `[Gasoline:${category}]`
    if (data !== null) {
      console.log(prefix, message, data) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
    } else {
      console.log(prefix, message) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal error message, not user-controlled
    }
  }
}

/**
 * Get all debug log entries
 */
export function getDebugLog() {
  return stateManager.getDebugLog()
}

/**
 * Clear debug log buffer
 */
export function clearDebugLog(): void {
  stateManager.clearDebugLog()
}

/**
 * Export debug log as JSON string
 */
export function exportDebugLog(): string {
  return JSON.stringify(
    {
      exportedAt: new Date().toISOString(),
      version: typeof chrome !== 'undefined' ? chrome.runtime.getManifest().version : 'test',
      debugMode,
      connectionStatus,
      settings: {
        logLevel: currentLogLevel,
        screenshotOnError,
        sourceMapEnabled: stateManager.isSourceMapEnabled()
      },
      entries: stateManager.getDebugLog()
    },
    null,
    2
  )
}

/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled: boolean): void {
  debugMode = enabled
  debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`)
}

// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================

export const sharedServerCircuitBreaker = communication.createCircuitBreaker(
  () => Promise.reject(new Error('shared circuit breaker')),
  {
    maxFailures: communication.RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: communication.RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
  }
)

// =============================================================================
// BATCHERS
// =============================================================================

function withConnectionStatus<T>(
  sendFn: (entries: T[]) => Promise<unknown>,
  onSuccess?: (entries: T[], result: unknown) => void
): (entries: T[]) => Promise<unknown> {
  return async (entries: T[]) => {
    try {
      const result = await sendFn(entries)
      connectionStatus.connected = true
      if (onSuccess) onSuccess(entries, result)
      communication.updateBadge(connectionStatus)
      return result
    } catch (err) {
      connectionStatus.connected = false
      communication.updateBadge(connectionStatus)
      throw err
    }
  }
}

export const logBatcherWithCB = communication.createBatcherWithCircuitBreaker<LogEntry>(
  withConnectionStatus(
    (entries) => {
      stateManager.checkContextAnnotations(entries)
      return communication.sendLogsToServer(serverUrl, entries, debugLog)
    },
    (entries, result) => {
      const typedResult = result as { entries?: number }
      connectionStatus.entries = typedResult.entries || connectionStatus.entries + entries.length
      connectionStatus.errorCount += entries.filter((e) => e.level === 'error').length
    }
  ),
  { sharedCircuitBreaker: sharedServerCircuitBreaker }
)
export const logBatcher = logBatcherWithCB.batcher

export const wsBatcherWithCB = communication.createBatcherWithCircuitBreaker<WebSocketEvent>(
  withConnectionStatus((events) => communication.sendWSEventsToServer(serverUrl, events, debugLog)),
  { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker: sharedServerCircuitBreaker }
)
export const wsBatcher = wsBatcherWithCB.batcher

export const enhancedActionBatcherWithCB = communication.createBatcherWithCircuitBreaker<EnhancedAction>(
  withConnectionStatus((actions) => communication.sendEnhancedActionsToServer(serverUrl, actions, debugLog)),
  { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker }
)
export const enhancedActionBatcher = enhancedActionBatcherWithCB.batcher

export const networkBodyBatcherWithCB = communication.createBatcherWithCircuitBreaker<NetworkBodyPayload>(
  withConnectionStatus((bodies) => communication.sendNetworkBodiesToServer(serverUrl, bodies, debugLog)),
  { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker }
)
export const networkBodyBatcher = networkBodyBatcherWithCB.batcher

export const perfBatcherWithCB = communication.createBatcherWithCircuitBreaker<PerformanceSnapshot>(
  withConnectionStatus((snapshots) => communication.sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLog)),
  { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker: sharedServerCircuitBreaker }
)
export const perfBatcher = perfBatcherWithCB.batcher

// =============================================================================
// LOG HANDLING
// =============================================================================

async function tryResolveSourceMap(entry: LogEntry): Promise<LogEntry> {
  if (!stateManager.isSourceMapEnabled()) return entry
  if (!(entry as { stack?: string }).stack) return entry

  try {
    const resolvedStack = await stateManager.resolveStackTrace((entry as { stack: string }).stack, debugLog)
    const existingEnrichments = (entry as { _enrichments?: readonly string[] })._enrichments
    const enrichments: string[] = existingEnrichments ? [...existingEnrichments] : []
    if (!enrichments.includes('sourceMap')) {
      enrichments.push('sourceMap')
    }
    debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map')
    return {
      ...entry,
      stack: resolvedStack,
      _sourceMapResolved: true,
      _enrichments: enrichments
    } as LogEntry
  } catch (err) {
    debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: (err as Error).message })
    return entry
  }
}

export async function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void> {
  if (!communication.shouldCaptureLog(payload.level, currentLogLevel, (payload as { type?: string }).type)) {
    debugLog(
      DebugCategory.CAPTURE,
      `Log filtered out: level=${payload.level}, type=${(payload as { type?: string }).type}` // nosemgrep: missing-template-string-indicator
    )
    return
  }

  let entry = communication.formatLogEntry(payload)

  const resolvedTabId = tabId ?? sender?.tab?.id
  if (resolvedTabId !== null && resolvedTabId !== undefined) {
    entry = { ...entry, tabId: resolvedTabId } as LogEntry
  }

  // nosemgrep: missing-template-string-indicator
  debugLog(DebugCategory.CAPTURE, `Log received: type=${(entry as { type?: string }).type}, level=${entry.level}`, {
    url: (entry as { url?: string }).url,
    enrichments: (entry as { _enrichments?: string[] })._enrichments
  })

  entry = await tryResolveSourceMap(entry)

  const { shouldSend, entry: processedEntry } = stateManager.processErrorGroup(entry)

  if (shouldSend && processedEntry) {
    logBatcher.add(processedEntry)
    // nosemgrep: missing-template-string-indicator
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${(processedEntry as { type?: string }).type}`, {
      aggregatedCount: (processedEntry as { _aggregatedCount?: number })._aggregatedCount
    })

    maybeAutoScreenshot(processedEntry, sender)
  } else {
    debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)')
  }
}

async function maybeAutoScreenshot(errorEntry: LogEntry, sender: ChromeMessageSender): Promise<void> {
  if (!screenshotOnError) return
  if (!sender?.tab?.id) return
  if (errorEntry.level !== 'error') return

  const entryType = (errorEntry as { type?: string }).type
  if (entryType !== 'exception' && entryType !== 'network') return

  const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  ;(errorEntry as { _errorId?: string })._errorId = errorId

  const result = await communication.captureScreenshot(
    sender.tab.id,
    serverUrl,
    errorId,
    entryType || null,
    stateManager.canTakeScreenshot,
    stateManager.recordScreenshot,
    debugLog
  )

  if (result.success && result.entry) {
    logBatcher.add(result.entry)
  }
}

export async function handleClearLogs(): Promise<{ success: boolean; error?: string }> {
  try {
    await fetch(`${serverUrl}/logs`, { method: 'DELETE', headers: getRequestHeaders() })
    connectionStatus.entries = 0
    connectionStatus.errorCount = 0
    communication.updateBadge(connectionStatus)
    return { success: true }
  } catch (error) {
    return { success: false, error: (error as Error).message }
  }
}

// =============================================================================
// CONNECTION MANAGEMENT
// =============================================================================

/**
 * Check if a connection check is currently running (for testing)
 */
export function isConnectionCheckRunning(): boolean {
  return _connectionCheckRunning
}

// #lizard forgives
function updateVersionFromHealthSafe(health: { version?: string; availableVersion?: string }): void {
  try {
    updateVersionFromHealth({ version: health.version, availableVersion: health.availableVersion }, debugLog)
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: (err as Error).message })
  }
}

function applyHealthLogs(health: {
  logs?: { logFile?: string; logFileSize?: number; entries?: number; maxEntries?: number }
}): void {
  if (!health.logs) return
  connectionStatus.logFile = health.logs.logFile || connectionStatus.logFile
  connectionStatus.logFileSize = health.logs.logFileSize
  connectionStatus.entries = health.logs.entries ?? connectionStatus.entries
  connectionStatus.maxEntries = health.logs.maxEntries ?? connectionStatus.maxEntries
}

function applyVersionMismatchCheck(health: { connected: boolean; version?: string }): void {
  if (!health.connected || !health.version || typeof chrome === 'undefined') return
  const extVersion = chrome.runtime.getManifest().version
  connectionStatus.serverVersion = health.version
  connectionStatus.extensionVersion = extVersion
  connectionStatus.versionMismatch = health.version.split('.')[0] !== extVersion.split('.')[0]
}

function logConnectionChange(
  wasConnected: boolean,
  health: { connected: boolean; error?: string; version?: string }
): void {
  if (wasConnected === health.connected) return
  debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
    entries: connectionStatus.entries,
    error: health.error || null,
    serverVersion: health.version || null
  })
}

function broadcastStatusUpdate(): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) return
  chrome.runtime
    .sendMessage({ type: 'statusUpdate', status: { ...connectionStatus, aiControlled } })
    .catch((err) => console.error('[Gasoline] Error sending status update:', err))
}

// eslint-disable-next-line security-node/detect-unhandled-async-errors
export async function checkConnectionAndUpdate(): Promise<void> {
  if (_connectionCheckRunning) {
    debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running')
    return
  }
  _connectionCheckRunning = true

  try {
    const health = await communication.checkServerHealth(serverUrl)
    const wasConnected = connectionStatus.connected

    if (health.connected) {
      updateVersionFromHealthSafe(health)
    }

    connectionStatus = { ...connectionStatus, ...health, connected: health.connected }
    applyHealthLogs(health)
    applyVersionMismatchCheck(health)

    communication.updateBadge(connectionStatus)
    logConnectionChange(wasConnected, health)

    // Always start sync client - it handles failures gracefully with 1s retry
    startSyncClient()
    broadcastStatusUpdate()
  } finally {
    _connectionCheckRunning = false
  }
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
}

// =============================================================================
// STATUS PING (still used for tracked tab change notifications)
// =============================================================================

export async function sendStatusPingWrapper(): Promise<void> {
  const trackingInfo = await eventListeners.getTrackedTabInfo()

  const statusMessage = {
    type: 'status',
    tracking_enabled: !!trackingInfo.trackedTabId,
    tracked_tab_id: trackingInfo.trackedTabId,
    tracked_tab_url: trackingInfo.trackedTabUrl,
    message: trackingInfo.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
    extension_connected: true,
    timestamp: new Date().toISOString()
  }

  await communication.sendStatusPing(serverUrl, statusMessage, diagnosticLog)
}

// =============================================================================
// SYNC CLIENT
// =============================================================================

/**
 * Get extension version safely
 */
function getExtensionVersion(): string {
  if (typeof chrome !== 'undefined' && chrome.runtime?.getManifest) {
    return chrome.runtime.getManifest().version
  }
  return ''
}

/**
 * Start the sync client (unified /sync endpoint)
 */
function startSyncClient(): void {
  if (syncClient) {
    // Already running, nothing to do
    return
  }

  syncClient = createSyncClient(
    serverUrl,
    EXTENSION_SESSION_ID,
    {
      // Handle commands from server
      // #lizard forgives
      onCommand: async (command: SyncCommand) => {
        debugLog(DebugCategory.CONNECTION, 'Processing sync command', { type: command.type, id: command.id })
        if (stateManager.isQueryProcessing(command.id)) {
          debugLog(DebugCategory.CONNECTION, 'Skipping already processing command', { id: command.id })
          return
        }
        stateManager.addProcessingQuery(command.id)
        try {
          await handlePendingQueryImpl(command as unknown as PendingQuery, syncClient!)
        } catch (err) {
          debugLog(DebugCategory.CONNECTION, 'Error processing sync command', {
            type: command.type,
            error: (err as Error).message
          })
        } finally {
          stateManager.removeProcessingQuery(command.id)
        }
      },

      // Handle connection state changes
      onConnectionChange: (connected: boolean) => {
        connectionStatus.connected = connected
        communication.updateBadge(connectionStatus)
        debugLog(DebugCategory.CONNECTION, connected ? 'Sync connected' : 'Sync disconnected')

        // Notify popup
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'statusUpdate',
              status: { ...connectionStatus, aiControlled }
            })
            .catch(() => {
              /* popup may not be open */
            })
        }
      },

      // Handle capture overrides from server
      onCaptureOverrides: (overrides: Record<string, string>) => {
        applyCaptureOverrides(overrides)
      },

      // Handle version mismatch between extension and server
      onVersionMismatch: (extensionVersion: string, serverVersion: string) => {
        debugLog(DebugCategory.CONNECTION, 'Version mismatch detected', { extensionVersion, serverVersion })
        // Update connection status with version info
        connectionStatus.serverVersion = serverVersion
        connectionStatus.extensionVersion = extensionVersion
        connectionStatus.versionMismatch = extensionVersion !== serverVersion
        // Notify popup about version mismatch
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'versionMismatch',
              extensionVersion,
              serverVersion
            })
            .catch(() => {
              /* popup may not be open */
            })
        }
      },

      // Get current settings to send to server
      getSettings: async (): Promise<SyncSettings> => {
        const trackingInfo = await eventListeners.getTrackedTabInfo()
        return {
          pilot_enabled: __aiWebPilotEnabledCache,
          tracking_enabled: !!trackingInfo.trackedTabId,
          tracked_tab_id: trackingInfo.trackedTabId || 0,
          tracked_tab_url: trackingInfo.trackedTabUrl || '',
          tracked_tab_title: trackingInfo.trackedTabTitle || '',
          capture_logs: true,
          capture_network: true,
          capture_websocket: true,
          capture_actions: true
        }
      },

      // Get pending extension logs
      getExtensionLogs: () => {
        return extensionLogQueue.map((log) => ({
          timestamp: log.timestamp,
          level: log.level,
          message: log.message,
          source: log.source,
          category: log.category,
          data: log.data
        }))
      },

      // Clear extension logs after sending
      clearExtensionLogs: () => {
        extensionLogQueue.length = 0
      },

      // Debug logging
      debugLog: (category: string, message: string, data?: unknown) => {
        debugLog(DebugCategory.CONNECTION, `[Sync] ${message}`, data)
      }
    },
    getExtensionVersion()
  )

  syncClient.start()
  debugLog(DebugCategory.CONNECTION, 'Sync client started')
}

/**
 * Stop the sync client
 */
function stopSyncClient(): void {
  if (syncClient) {
    syncClient.stop()
    debugLog(DebugCategory.CONNECTION, 'Sync client stopped')
  }
}

/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(): void {
  if (syncClient) {
    syncClient.resetConnection()
    debugLog(DebugCategory.CONNECTION, 'Sync client connection reset')
  }
}

// =============================================================================
// AI WEB PILOT UTILITIES
// =============================================================================

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

// Re-export statically imported functions (Service Workers don't support dynamic import())
export const handlePendingQuery = handlePendingQueryImpl
export const handlePilotCommand = handlePilotCommandImpl

// Export snapshot/state management for backward compatibility
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot }
