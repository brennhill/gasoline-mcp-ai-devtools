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
  WaterfallEntry,
} from '../types'

import * as stateManager from './state-manager'
import * as communication from './communication'
import * as polling from './polling'
import * as eventListeners from './event-listeners'
import { DebugCategory } from './debug'
import {
  installMessageListener,
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot,
  type MessageHandlerDependencies,
} from './message-handlers'
import {
  handlePendingQuery as handlePendingQueryImpl,
  handlePilotCommand as handlePilotCommandImpl,
} from './pending-queries'

// =============================================================================
// CONSTANTS
// =============================================================================

export const DEFAULT_SERVER_URL = 'http://localhost:7890'

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
  logFile: '',
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
    ...(data !== null ? { data } : {}),
  }

  stateManager.addDebugLogEntry(entry)

  if (connectionStatus.connected) {
    extensionLogQueue.push({
      timestamp,
      level: 'debug',
      message,
      source: 'background',
      category,
      ...(data !== null ? { data } : {}),
    })
  }

  if (debugMode) {
    const prefix = `[Gasoline:${category}]`
    if (data !== null) {
      console.log(prefix, message, data)
    } else {
      console.log(prefix, message)
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
        sourceMapEnabled: stateManager.isSourceMapEnabled(),
      },
      entries: stateManager.getDebugLog(),
    },
    null,
    2,
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
    maxBackoff: 0,
  },
)

// =============================================================================
// BATCHERS
// =============================================================================

function withConnectionStatus<T>(
  sendFn: (entries: T[]) => Promise<unknown>,
  onSuccess?: (entries: T[], result: unknown) => void,
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
    },
  ),
  { sharedCircuitBreaker: sharedServerCircuitBreaker },
)
export const logBatcher = logBatcherWithCB.batcher

export const wsBatcherWithCB = communication.createBatcherWithCircuitBreaker<WebSocketEvent>(
  withConnectionStatus((events) => communication.sendWSEventsToServer(serverUrl, events, debugLog)),
  { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker: sharedServerCircuitBreaker },
)
export const wsBatcher = wsBatcherWithCB.batcher

export const enhancedActionBatcherWithCB = communication.createBatcherWithCircuitBreaker<EnhancedAction>(
  withConnectionStatus((actions) => communication.sendEnhancedActionsToServer(serverUrl, actions, debugLog)),
  { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker },
)
export const enhancedActionBatcher = enhancedActionBatcherWithCB.batcher

export const networkBodyBatcherWithCB = communication.createBatcherWithCircuitBreaker<NetworkBodyPayload>(
  withConnectionStatus((bodies) => communication.sendNetworkBodiesToServer(serverUrl, bodies, debugLog)),
  { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker: sharedServerCircuitBreaker },
)
export const networkBodyBatcher = networkBodyBatcherWithCB.batcher

export const perfBatcherWithCB = communication.createBatcherWithCircuitBreaker<PerformanceSnapshot>(
  withConnectionStatus((snapshots) => communication.sendPerformanceSnapshotsToServer(serverUrl, snapshots, debugLog)),
  { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker: sharedServerCircuitBreaker },
)
export const perfBatcher = perfBatcherWithCB.batcher

// =============================================================================
// LOG HANDLING
// =============================================================================

export async function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void> {
  if (!communication.shouldCaptureLog(payload.level, currentLogLevel, (payload as { type?: string }).type)) {
    debugLog(
      DebugCategory.CAPTURE,
      `Log filtered out: level=${payload.level}, type=${(payload as { type?: string }).type}`,
    )
    return
  }

  let entry = communication.formatLogEntry(payload)

  const resolvedTabId = tabId ?? sender?.tab?.id
  if (resolvedTabId !== null && resolvedTabId !== undefined) {
    entry = { ...entry, tabId: resolvedTabId } as LogEntry
  }

  debugLog(DebugCategory.CAPTURE, `Log received: type=${(entry as { type?: string }).type}, level=${entry.level}`, {
    url: (entry as { url?: string }).url,
    enrichments: (entry as { _enrichments?: string[] })._enrichments,
  })

  if (stateManager.isSourceMapEnabled() && (entry as { stack?: string }).stack) {
    try {
      const resolvedStack = await stateManager.resolveStackTrace((entry as { stack: string }).stack, debugLog)
      const existingEnrichments = (entry as { _enrichments?: readonly string[] })._enrichments
      const enrichments: string[] = existingEnrichments ? [...existingEnrichments] : []
      if (!enrichments.includes('sourceMap')) {
        enrichments.push('sourceMap')
      }
      entry = {
        ...entry,
        stack: resolvedStack,
        _sourceMapResolved: true,
        _enrichments: enrichments,
      } as LogEntry
      debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map')
    } catch (err) {
      debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: (err as Error).message })
    }
  }

  const { shouldSend, entry: processedEntry } = stateManager.processErrorGroup(entry)

  if (shouldSend && processedEntry) {
    logBatcher.add(processedEntry)
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${(processedEntry as { type?: string }).type}`, {
      aggregatedCount: (processedEntry as { _aggregatedCount?: number })._aggregatedCount,
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
    debugLog,
  )

  if (result.success && result.entry) {
    logBatcher.add(result.entry)
  }
}

export async function handleClearLogs(): Promise<{ success: boolean; error?: string }> {
  try {
    await fetch(`${serverUrl}/logs`, { method: 'DELETE' })
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

export async function checkConnectionAndUpdate(): Promise<void> {
  if (_connectionCheckRunning) {
    debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running')
    return
  }
  _connectionCheckRunning = true

  try {
    const health = await communication.checkServerHealth(serverUrl)

    // Update version information from health response
    if (health.connected) {
      import('./version-check')
        .then((vc) => {
          vc.updateVersionFromHealth(
            {
              version: health.version,
              availableVersion: health.availableVersion,
            },
            debugLog,
          )
        })
        .catch((err) => {
          debugLog(DebugCategory.CONNECTION, 'Failed to update version info', { error: (err as Error).message })
        })
    }

    const wasConnected = connectionStatus.connected
    connectionStatus = {
      ...connectionStatus,
      ...health,
      connected: health.connected,
    }

    if (health.logs) {
      connectionStatus.logFile = health.logs.logFile || connectionStatus.logFile
      connectionStatus.logFileSize = health.logs.logFileSize
      connectionStatus.entries = health.logs.entries ?? connectionStatus.entries
      connectionStatus.maxEntries = health.logs.maxEntries ?? connectionStatus.maxEntries
    }

    if (health.connected && health.version && typeof chrome !== 'undefined') {
      const extVersion = chrome.runtime.getManifest().version
      const serverMajor = health.version.split('.')[0]
      const extMajor = extVersion.split('.')[0]
      connectionStatus.serverVersion = health.version
      connectionStatus.extensionVersion = extVersion
      connectionStatus.versionMismatch = serverMajor !== extMajor
    }

    communication.updateBadge(connectionStatus)

    if (wasConnected !== health.connected) {
      debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
        entries: connectionStatus.entries,
        error: health.error || null,
        serverVersion: health.version || null,
      })
    }

    if (health.connected) {
      const overrides = await communication.pollCaptureSettings(serverUrl)
      if (overrides !== null) {
        applyCaptureOverrides(overrides)
      }

      polling.startQueryPolling(() => pollPendingQueriesWrapper(), debugLog)
      polling.startSettingsHeartbeat(() => postSettingsWrapper(), debugLog)
      polling.startWaterfallPosting(() => postNetworkWaterfall(), debugLog)
      polling.startExtensionLogsPosting(() => postExtensionLogsWrapper())
      polling.startStatusPing(() => sendStatusPingWrapper())
    } else {
      polling.stopAllPolling()
    }

    if (typeof chrome !== 'undefined' && chrome.runtime) {
      chrome.runtime
        .sendMessage({
          type: 'statusUpdate',
          status: { ...connectionStatus, aiControlled },
        })
        .catch((err) => console.error('[Gasoline] Error sending status update:', err))
    }
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
// POLLING WRAPPERS
// =============================================================================

export async function pollPendingQueriesWrapper(): Promise<void> {
  stateManager.cleanupStaleProcessingQueries(debugLog)

  const pilotState = (__aiWebPilotEnabledCache ? '1' : '0') as '0' | '1'
  const queries = await communication.pollPendingQueries(
    serverUrl,
    EXTENSION_SESSION_ID,
    pilotState,
    diagnosticLog,
    debugLog,
  )

  debugLog(DebugCategory.CONNECTION, 'Poll result', {
    count: queries.length,
    queries: queries.map((q) => ({ id: q.id, type: q.type })),
  })

  for (const query of queries) {
    debugLog(DebugCategory.CONNECTION, 'Processing query', { type: query.type, id: query.id })
    if (stateManager.isQueryProcessing(query.id)) {
      debugLog(DebugCategory.CONNECTION, 'Skipping already processing query', { id: query.id })
      continue
    }
    stateManager.addProcessingQuery(query.id)
    try {
      debugLog(DebugCategory.CONNECTION, 'Calling handlePendingQuery', { type: query.type })
      await handlePendingQuery(query as unknown as PendingQuery)
      debugLog(DebugCategory.CONNECTION, 'handlePendingQuery completed', { type: query.type })
    } catch (err) {
      debugLog(DebugCategory.CONNECTION, 'Error in handlePendingQuery', {
        type: query.type,
        error: (err as Error).message,
      })
      console.error('[Gasoline] Error in handlePendingQuery:', query.type, err)
    } finally {
      stateManager.removeProcessingQuery(query.id)
    }
  }
}

export async function postSettingsWrapper(): Promise<void> {
  if (!__aiWebPilotCacheInitialized) {
    debugLog(DebugCategory.CONNECTION, 'Skipping settings POST: cache not initialized')
    return
  }

  const configSettings = await eventListeners.getAllConfigSettings()
  const settings: Record<string, boolean | string> = {}

  if (configSettings.aiWebPilotEnabled !== undefined) {
    settings.aiWebPilotEnabled = configSettings.aiWebPilotEnabled as boolean
  } else if (__aiWebPilotEnabledCache !== undefined) {
    settings.aiWebPilotEnabled = __aiWebPilotEnabledCache
  }

  for (const key of [
    'webSocketCaptureEnabled',
    'networkWaterfallEnabled',
    'performanceMarksEnabled',
    'actionReplayEnabled',
    'screenshotOnError',
    'sourceMapEnabled',
    'networkBodyCaptureEnabled',
  ]) {
    if (configSettings[key] !== undefined) {
      settings[key] = configSettings[key] as boolean
    }
  }

  await communication.postSettings(serverUrl, EXTENSION_SESSION_ID, settings, debugLog)
}

export async function postNetworkWaterfall(): Promise<void> {
  if (typeof chrome === 'undefined' || !chrome.tabs) return

  try {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    const firstTab = tabs[0]
    if (!firstTab?.id) return

    const tabId = firstTab.id
    const pageURL = firstTab.url

    const result = (await chrome.tabs.sendMessage(tabId, {
      type: 'GET_NETWORK_WATERFALL',
    })) as { entries?: unknown[] }

    if (!result || !result.entries || result.entries.length === 0) {
      debugLog(DebugCategory.CAPTURE, 'No waterfall entries to send')
      return
    }

    await communication.sendNetworkWaterfallToServer(
      serverUrl,
      { entries: result.entries as WaterfallEntry[], pageURL: pageURL || '' },
      debugLog,
    )
  } catch (err) {
    debugLog(DebugCategory.CAPTURE, 'Failed to post waterfall', { error: (err as Error).message })
  }
}

export async function postExtensionLogsWrapper(): Promise<void> {
  if (extensionLogQueue.length === 0) return
  const logsToSend = extensionLogQueue.splice(0)
  await communication.postExtensionLogs(serverUrl, logsToSend)
}

export async function sendStatusPingWrapper(): Promise<void> {
  const trackingInfo = await eventListeners.getTrackedTabInfo()

  const statusMessage = {
    type: 'status',
    tracking_enabled: !!trackingInfo.trackedTabId,
    tracked_tab_id: trackingInfo.trackedTabId,
    tracked_tab_url: trackingInfo.trackedTabUrl,
    message: trackingInfo.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
    extension_connected: true,
    timestamp: new Date().toISOString(),
  }

  await communication.sendStatusPing(serverUrl, statusMessage, diagnosticLog)
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
