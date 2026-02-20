/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Main Background Service Worker — Business logic and export hub.
 * Mutable state lives in state.ts; this module owns debug logging, log handling,
 * connection management, and batcher wiring. Delegates batcher instance creation
 * to batcher-instances.ts and sync client lifecycle to sync-manager.ts.
 */

import type {
  LogEntry,
  ChromeMessageSender
} from '../types'

import {
  debugMode,
  connectionStatus,
  extensionLogQueue,
  currentLogLevel,
  screenshotOnError,
  _setDebugModeRaw,
  serverUrl,
  setConnectionStatus,
  _connectionCheckRunning,
  setConnectionCheckRunning,
  EXTENSION_SESSION_ID,
  aiControlled,
  __aiWebPilotEnabledCache,
  applyCaptureOverrides,
  type MutableConnectionStatus
} from './state'
import {
  addDebugLogEntry,
  getDebugLog as getDebugLogEntries,
  clearDebugLog as clearDebugLogEntries,
  isSourceMapEnabled,
  resolveStackTrace,
  processErrorGroup,
  canTakeScreenshot,
  recordScreenshot
} from './state-manager'
import {
  createCircuitBreaker,
  RATE_LIMIT_CONFIG,
  shouldCaptureLog,
  formatLogEntry,
  captureScreenshot,
  updateBadge,
  checkServerHealth,
  sendStatusPing
} from './communication'
import { getTrackedTabInfo } from './event-listeners'
import { DebugCategory } from './debug'
import { getRequestHeaders } from './server'
import {
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot
} from './message-handlers'
import {
  handlePendingQuery as handlePendingQueryImpl,
  handlePilotCommand as handlePilotCommandImpl
} from './pending-queries'
import { updateVersionFromHealth } from './version-check'
import { createBatcherInstances } from './batcher-instances'
import {
  startSyncClient as startSyncClientImpl,
  resetSyncClientConnection as resetSyncClientConnectionImpl
} from './sync-manager'

// Re-export for consumers that already import from here
export { DEFAULT_SERVER_URL } from '../lib/constants'

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

  addDebugLogEntry(entry)

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
  return getDebugLogEntries()
}

/**
 * Clear debug log buffer
 */
export function clearDebugLog(): void {
  clearDebugLogEntries()
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
        sourceMapEnabled: isSourceMapEnabled()
      },
      entries: getDebugLogEntries()
    },
    null,
    2
  )
}

/**
 * Set debug mode enabled/disabled
 */
export function setDebugMode(enabled: boolean): void {
  _setDebugModeRaw(enabled)
  debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`)
}

// =============================================================================
// SHARED CIRCUIT BREAKER
// =============================================================================

export const sharedServerCircuitBreaker = createCircuitBreaker(
  () => Promise.reject(new Error('shared circuit breaker')),
  {
    maxFailures: RATE_LIMIT_CONFIG.maxFailures,
    resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
    initialBackoff: 0,
    maxBackoff: 0
  }
)

// =============================================================================
// BATCHERS (delegated to batcher-instances.ts)
// =============================================================================

const _batchers = createBatcherInstances(
  {
    getServerUrl: () => serverUrl,
    getConnectionStatus: () => connectionStatus,
    setConnectionStatus: (patch) => {
      setConnectionStatus(patch)
    },
    debugLog
  },
  sharedServerCircuitBreaker
)

export const logBatcherWithCB = _batchers.logBatcherWithCB
export const logBatcher = _batchers.logBatcher
export const wsBatcherWithCB = _batchers.wsBatcherWithCB
export const wsBatcher = _batchers.wsBatcher
export const enhancedActionBatcherWithCB = _batchers.enhancedActionBatcherWithCB
export const enhancedActionBatcher = _batchers.enhancedActionBatcher
export const networkBodyBatcherWithCB = _batchers.networkBodyBatcherWithCB
export const networkBodyBatcher = _batchers.networkBodyBatcher
export const perfBatcherWithCB = _batchers.perfBatcherWithCB
export const perfBatcher = _batchers.perfBatcher

// =============================================================================
// LOG HANDLING
// =============================================================================

async function tryResolveSourceMap(entry: LogEntry): Promise<LogEntry> {
  if (!isSourceMapEnabled()) return entry
  if (!(entry as { stack?: string }).stack) return entry

  try {
    const resolvedStack = await resolveStackTrace((entry as { stack: string }).stack, debugLog)
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
  if (!shouldCaptureLog(payload.level, currentLogLevel, (payload as { type?: string }).type)) {
    debugLog(
      DebugCategory.CAPTURE,
      `Log filtered out: level=${payload.level}, type=${(payload as { type?: string }).type}` // nosemgrep: missing-template-string-indicator
    )
    return
  }

  let entry = formatLogEntry(payload)

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

  const { shouldSend, entry: processedEntry } = processErrorGroup(entry)

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

  const result = await captureScreenshot(
    sender.tab.id,
    serverUrl,
    errorId,
    entryType || null,
    canTakeScreenshot,
    recordScreenshot,
    debugLog
  )

  if (result.success && result.entry) {
    logBatcher.add(result.entry)
  }
}

export async function handleClearLogs(): Promise<{ success: boolean; error?: string }> {
  try {
    await fetch(`${serverUrl}/logs`, { method: 'DELETE', headers: getRequestHeaders() })
    setConnectionStatus({ entries: 0, errorCount: 0 })
    updateBadge(connectionStatus)
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
  setConnectionStatus({
    logFile: health.logs.logFile || connectionStatus.logFile,
    logFileSize: health.logs.logFileSize,
    entries: health.logs.entries ?? connectionStatus.entries,
    maxEntries: health.logs.maxEntries ?? connectionStatus.maxEntries
  })
}

function applyVersionMismatchCheck(health: { connected: boolean; version?: string }): void {
  if (!health.connected || !health.version || typeof chrome === 'undefined') return
  const extVersion = chrome.runtime.getManifest().version
  setConnectionStatus({
    serverVersion: health.version,
    extensionVersion: extVersion,
    versionMismatch: health.version.split('.')[0] !== extVersion.split('.')[0]
  })
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
  setConnectionCheckRunning(true)

  try {
    const health = await checkServerHealth(serverUrl)
    const wasConnected = connectionStatus.connected

    if (health.connected) {
      updateVersionFromHealthSafe(health)
    }

    setConnectionStatus({ ...health, connected: health.connected })
    applyHealthLogs(health)
    applyVersionMismatchCheck(health)

    updateBadge(connectionStatus)
    logConnectionChange(wasConnected, health)

    // Always start sync client - it handles failures gracefully with 1s retry
    startSyncClientImpl(syncManagerDeps)
    broadcastStatusUpdate()
  } finally {
    setConnectionCheckRunning(false)
  }
}

// =============================================================================
// STATUS PING (still used for tracked tab change notifications)
// =============================================================================

export async function sendStatusPingWrapper(): Promise<void> {
  const trackingInfo = await getTrackedTabInfo()

  const statusMessage = {
    type: 'status',
    tracking_enabled: !!trackingInfo.trackedTabId,
    tracked_tab_id: trackingInfo.trackedTabId,
    tracked_tab_url: trackingInfo.trackedTabUrl,
    message: trackingInfo.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
    extension_connected: true,
    timestamp: new Date().toISOString()
  }

  await sendStatusPing(serverUrl, statusMessage, diagnosticLog)
}

// =============================================================================
// SYNC CLIENT (delegated to sync-manager.ts)
// =============================================================================

/** Shared deps object for sync-manager — created once, closures read live state */
const syncManagerDeps = {
  getServerUrl: () => serverUrl,
  getExtSessionId: () => EXTENSION_SESSION_ID,
  getConnectionStatus: () => connectionStatus,
  setConnectionStatus: (patch: Partial<MutableConnectionStatus>) => {
    setConnectionStatus(patch)
  },
  getAiControlled: () => aiControlled,
  getAiWebPilotEnabledCache: () => __aiWebPilotEnabledCache,
  getExtensionLogQueue: () => extensionLogQueue,
  clearExtensionLogQueue: () => {
    extensionLogQueue.length = 0
  },
  applyCaptureOverrides,
  debugLog
}

/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(): void {
  resetSyncClientConnectionImpl(debugLog)
}

// Re-export statically imported functions (Service Workers don't support dynamic import())
export const handlePendingQuery = handlePendingQueryImpl
export const handlePilotCommand = handlePilotCommandImpl

// Export snapshot/state management for backward compatibility
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot }
