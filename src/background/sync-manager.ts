// sync-manager.ts — Sync client lifecycle management.
// Owns the sync client instance and provides start/stop/reset operations.
// Dependencies are injected to avoid circular imports with index.ts.

import type { PendingQuery } from '../types'
import { createSyncClient, type SyncClient, type SyncCommand, type SyncSettings } from './sync-client'
import { DebugCategory } from './debug'
import * as communication from './communication'
import * as stateManager from './state-manager'
import * as eventListeners from './event-listeners'
import { handlePendingQuery as handlePendingQueryImpl } from './pending-queries'

// =============================================================================
// TYPES
// =============================================================================

type DebugLogFn = (category: string, message: string, data?: unknown) => void

/** Mutable connection status (same shape as index.ts) */
export interface SyncConnectionStatusRef {
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

/** Extension log queue entry */
export interface ExtensionLogEntry {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

/** Dependencies injected by index.ts to avoid circular imports */
export interface SyncManagerDeps {
  getServerUrl: () => string
  getExtSessionId: () => string
  getConnectionStatus: () => SyncConnectionStatusRef
  setConnectionStatus: (patch: Partial<SyncConnectionStatusRef>) => void
  getAiControlled: () => boolean
  getAiWebPilotEnabledCache: () => boolean
  getExtensionLogQueue: () => ExtensionLogEntry[]
  clearExtensionLogQueue: () => void
  applyCaptureOverrides: (overrides: Record<string, string>) => void
  debugLog: DebugLogFn
}

// =============================================================================
// MODULE STATE
// =============================================================================

/** Sync client instance (initialized lazily) */
let syncClient: SyncClient | null = null

// =============================================================================
// HELPERS
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

// =============================================================================
// SYNC CLIENT LIFECYCLE
// =============================================================================

/**
 * Start the sync client (unified /sync endpoint).
 * Safe to call multiple times — will no-op if already running.
 */
// #lizard forgives
export function startSyncClient(deps: SyncManagerDeps): void {
  if (syncClient) {
    // Already running, nothing to do
    return
  }

  syncClient = createSyncClient(
    deps.getServerUrl(),
    deps.getExtSessionId(),
    {
      // Handle commands from server
      // #lizard forgives
      onCommand: async (command: SyncCommand) => {
        deps.debugLog(DebugCategory.CONNECTION, 'Processing sync command', { type: command.type, id: command.id })
        if (stateManager.isQueryProcessing(command.id)) {
          deps.debugLog(DebugCategory.CONNECTION, 'Skipping already processing command', { id: command.id })
          return
        }
        stateManager.addProcessingQuery(command.id)
        try {
          await handlePendingQueryImpl(command as unknown as PendingQuery, syncClient!)
        } catch (err) {
          deps.debugLog(DebugCategory.CONNECTION, 'Error processing sync command', {
            type: command.type,
            error: (err as Error).message
          })
        } finally {
          stateManager.removeProcessingQuery(command.id)
        }
      },

      // Handle connection state changes
      onConnectionChange: (connected: boolean) => {
        deps.setConnectionStatus({ connected })
        communication.updateBadge(deps.getConnectionStatus())
        deps.debugLog(DebugCategory.CONNECTION, connected ? 'Sync connected' : 'Sync disconnected')

        // Notify popup
        if (typeof chrome !== 'undefined' && chrome.runtime) {
          chrome.runtime
            .sendMessage({
              type: 'statusUpdate',
              status: { ...deps.getConnectionStatus(), aiControlled: deps.getAiControlled() }
            })
            .catch(() => {
              /* popup may not be open */
            })
        }
      },

      // Handle capture overrides from server
      onCaptureOverrides: (overrides: Record<string, string>) => {
        deps.applyCaptureOverrides(overrides)
      },

      // Handle version mismatch between extension and server
      onVersionMismatch: (extensionVersion: string, serverVersion: string) => {
        deps.debugLog(DebugCategory.CONNECTION, 'Version mismatch detected', { extensionVersion, serverVersion })
        // Update connection status with version info
        deps.setConnectionStatus({
          serverVersion,
          extensionVersion,
          versionMismatch: extensionVersion !== serverVersion
        })
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
          pilot_enabled: deps.getAiWebPilotEnabledCache(),
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
        return deps.getExtensionLogQueue().map((log) => ({
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
        deps.clearExtensionLogQueue()
      },

      // Debug logging
      debugLog: (category: string, message: string, data?: unknown) => {
        deps.debugLog(DebugCategory.CONNECTION, `[Sync] ${message}`, data)
      }
    },
    getExtensionVersion()
  )

  syncClient.start()
  deps.debugLog(DebugCategory.CONNECTION, 'Sync client started')
}

/**
 * Stop the sync client
 */
export function stopSyncClient(debugLog: DebugLogFn): void {
  if (syncClient) {
    syncClient.stop()
    debugLog(DebugCategory.CONNECTION, 'Sync client stopped')
  }
}

/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export function resetSyncClientConnection(debugLog: DebugLogFn): void {
  if (syncClient) {
    syncClient.resetConnection()
    debugLog(DebugCategory.CONNECTION, 'Sync client connection reset')
  }
}
