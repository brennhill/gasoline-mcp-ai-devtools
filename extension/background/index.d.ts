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
  ChromeMessageSender
} from '../types'
import * as communication from './communication'
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './message-handlers'
import {
  handlePendingQuery as handlePendingQueryImpl,
  handlePilotCommand as handlePilotCommandImpl
} from './pending-queries'
export declare const DEFAULT_SERVER_URL = 'http://localhost:7890'
/** Session ID for detecting extension reloads */
export declare const EXTENSION_SESSION_ID: string
/** Server URL */
export declare let serverUrl: string
/** Debug mode flag */
export declare let debugMode: boolean
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
export declare let connectionStatus: MutableConnectionStatus
/** Log level filter */
export declare let currentLogLevel: string
/** Screenshot settings */
export declare let screenshotOnError: boolean
/** AI capture control state */
export declare let _captureOverrides: Record<string, string>
export declare let aiControlled: boolean
/** Connection check mutex */
export declare let _connectionCheckRunning: boolean
/** AI Web Pilot state */
export declare let __aiWebPilotEnabledCache: boolean
export declare let __aiWebPilotCacheInitialized: boolean
export declare let __pilotInitCallback: (() => void) | null
export declare const initReady: Promise<void>
export declare function markInitComplete(): void
/** Extension log queue for server posting */
export declare const extensionLogQueue: Array<{
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}>
export declare function setServerUrl(url: string): void
export declare function setCurrentLogLevel(level: string): void
export declare function setScreenshotOnError(enabled: boolean): void
export declare function setAiWebPilotEnabledCache(enabled: boolean): void
export declare function setAiWebPilotCacheInitialized(initialized: boolean): void
export declare function setPilotInitCallback(callback: (() => void) | null): void
export { DebugCategory } from './debug'
/**
 * Log a diagnostic message only when debug mode is enabled
 */
export declare function diagnosticLog(message: string): void
/**
 * Log a debug message (only when debug mode is enabled)
 */
export declare function debugLog(category: string, message: string, data?: unknown): void
/**
 * Get all debug log entries
 */
export declare function getDebugLog(): import('../types').DebugLogEntry[]
/**
 * Clear debug log buffer
 */
export declare function clearDebugLog(): void
/**
 * Export debug log as JSON string
 */
export declare function exportDebugLog(): string
/**
 * Set debug mode enabled/disabled
 */
export declare function setDebugMode(enabled: boolean): void
export declare const sharedServerCircuitBreaker: communication.CircuitBreaker
export declare const logBatcherWithCB: communication.BatcherWithCircuitBreaker<LogEntry>
export declare const logBatcher: communication.Batcher<LogEntry>
export declare const wsBatcherWithCB: communication.BatcherWithCircuitBreaker<WebSocketEvent>
export declare const wsBatcher: communication.Batcher<WebSocketEvent>
export declare const enhancedActionBatcherWithCB: communication.BatcherWithCircuitBreaker<EnhancedAction>
export declare const enhancedActionBatcher: communication.Batcher<EnhancedAction>
export declare const networkBodyBatcherWithCB: communication.BatcherWithCircuitBreaker<NetworkBodyPayload>
export declare const networkBodyBatcher: communication.Batcher<NetworkBodyPayload>
export declare const perfBatcherWithCB: communication.BatcherWithCircuitBreaker<PerformanceSnapshot>
export declare const perfBatcher: communication.Batcher<PerformanceSnapshot>
export declare function handleLogMessage(payload: LogEntry, sender: ChromeMessageSender, tabId?: number): Promise<void>
export declare function handleClearLogs(): Promise<{
  success: boolean
  error?: string
}>
/**
 * Check if a connection check is currently running (for testing)
 */
export declare function isConnectionCheckRunning(): boolean
export declare function checkConnectionAndUpdate(): Promise<void>
export declare function applyCaptureOverrides(overrides: Record<string, string>): void
export declare function sendStatusPingWrapper(): Promise<void>
/**
 * Reset sync client connection (call when user enables pilot/tracking)
 */
export declare function resetSyncClientConnection(): void
/**
 * Reset pilot cache for testing
 */
export declare function _resetPilotCacheForTesting(value?: boolean): void
/**
 * Check if AI Web Pilot is enabled
 */
export declare function isAiWebPilotEnabled(): boolean
export declare const handlePendingQuery: typeof handlePendingQueryImpl
export declare const handlePilotCommand: typeof handlePilotCommandImpl
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot }
//# sourceMappingURL=index.d.ts.map
