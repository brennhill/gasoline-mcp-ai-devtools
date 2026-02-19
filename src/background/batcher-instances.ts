// batcher-instances.ts — Concrete batcher instances for each data type.
// Creates log, WebSocket, enhanced-action, network-body, and performance batchers,
// each wired to the shared circuit breaker and connection-status tracking.

import type {
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot
} from '../types'

import * as communication from './communication'
import * as stateManager from './state-manager'

// =============================================================================
// TYPES
// =============================================================================

/** Mutable connection status passed in from the state owner */
export interface ConnectionStatusRef {
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

type DebugLogFn = (category: string, message: string, data?: unknown) => void

/** Dependencies injected by index.ts to avoid circular imports */
export interface BatcherDeps {
  getServerUrl: () => string
  getConnectionStatus: () => ConnectionStatusRef
  setConnectionStatus: (patch: Partial<ConnectionStatusRef>) => void
  debugLog: DebugLogFn
}

// =============================================================================
// CONNECTION STATUS WRAPPER
// =============================================================================

function withConnectionStatus<T>(
  deps: BatcherDeps,
  sendFn: (entries: T[]) => Promise<unknown>,
  onSuccess?: (entries: T[], result: unknown) => void
): (entries: T[]) => Promise<unknown> {
  return async (entries: T[]) => {
    try {
      const result = await sendFn(entries)
      deps.setConnectionStatus({ connected: true })
      if (onSuccess) onSuccess(entries, result)
      communication.updateBadge(deps.getConnectionStatus())
      return result
    } catch (err) {
      deps.setConnectionStatus({ connected: false })
      communication.updateBadge(deps.getConnectionStatus())
      throw err
    }
  }
}

// =============================================================================
// FACTORY
// =============================================================================

export interface BatcherInstances {
  logBatcherWithCB: communication.BatcherWithCircuitBreaker<LogEntry>
  logBatcher: communication.Batcher<LogEntry>
  wsBatcherWithCB: communication.BatcherWithCircuitBreaker<WebSocketEvent>
  wsBatcher: communication.Batcher<WebSocketEvent>
  enhancedActionBatcherWithCB: communication.BatcherWithCircuitBreaker<EnhancedAction>
  enhancedActionBatcher: communication.Batcher<EnhancedAction>
  networkBodyBatcherWithCB: communication.BatcherWithCircuitBreaker<NetworkBodyPayload>
  networkBodyBatcher: communication.Batcher<NetworkBodyPayload>
  perfBatcherWithCB: communication.BatcherWithCircuitBreaker<PerformanceSnapshot>
  perfBatcher: communication.Batcher<PerformanceSnapshot>
}

/**
 * Create all batcher instances wired to the shared circuit breaker.
 * Called once from index.ts during module initialization.
 */
export function createBatcherInstances(
  deps: BatcherDeps,
  sharedCircuitBreaker: communication.CircuitBreaker
): BatcherInstances {
  const logBatcherWithCB = communication.createBatcherWithCircuitBreaker<LogEntry>(
    withConnectionStatus(
      deps,
      (entries) => {
        stateManager.checkContextAnnotations(entries)
        return communication.sendLogsToServer(deps.getServerUrl(), entries, deps.debugLog)
      },
      (entries, result) => {
        const typedResult = result as { entries?: number }
        const status = deps.getConnectionStatus()
        deps.setConnectionStatus({
          entries: typedResult.entries || status.entries + entries.length,
          errorCount: status.errorCount + entries.filter((e) => e.level === 'error').length
        })
      }
    ),
    { sharedCircuitBreaker }
  )

  const wsBatcherWithCB = communication.createBatcherWithCircuitBreaker<WebSocketEvent>(
    withConnectionStatus(deps, (events) =>
      communication.sendWSEventsToServer(deps.getServerUrl(), events, deps.debugLog)
    ),
    { debounceMs: 200, maxBatchSize: 100, sharedCircuitBreaker }
  )

  const enhancedActionBatcherWithCB = communication.createBatcherWithCircuitBreaker<EnhancedAction>(
    withConnectionStatus(deps, (actions) =>
      communication.sendEnhancedActionsToServer(deps.getServerUrl(), actions, deps.debugLog)
    ),
    { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker }
  )

  const networkBodyBatcherWithCB = communication.createBatcherWithCircuitBreaker<NetworkBodyPayload>(
    withConnectionStatus(deps, (bodies) =>
      communication.sendNetworkBodiesToServer(deps.getServerUrl(), bodies, deps.debugLog)
    ),
    { debounceMs: 200, maxBatchSize: 50, sharedCircuitBreaker }
  )

  const perfBatcherWithCB = communication.createBatcherWithCircuitBreaker<PerformanceSnapshot>(
    withConnectionStatus(deps, (snapshots) =>
      communication.sendPerformanceSnapshotsToServer(deps.getServerUrl(), snapshots, deps.debugLog)
    ),
    { debounceMs: 500, maxBatchSize: 10, sharedCircuitBreaker }
  )

  return {
    logBatcherWithCB,
    logBatcher: logBatcherWithCB.batcher,
    wsBatcherWithCB,
    wsBatcher: wsBatcherWithCB.batcher,
    enhancedActionBatcherWithCB,
    enhancedActionBatcher: enhancedActionBatcherWithCB.batcher,
    networkBodyBatcherWithCB,
    networkBodyBatcher: networkBodyBatcherWithCB.batcher,
    perfBatcherWithCB,
    perfBatcher: perfBatcherWithCB.batcher
  }
}
