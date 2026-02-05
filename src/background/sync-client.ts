/**
 * @fileoverview Unified Sync Client - Replaces multiple polling loops with single /sync endpoint.
 * Features: Simple exponential backoff, binary connection state, self-healing for MV3.
 */

import type { PendingQuery } from '../types'

// =============================================================================
// TYPES
// =============================================================================

/** Settings to send to server */
export interface SyncSettings {
  pilot_enabled: boolean
  tracking_enabled: boolean
  tracked_tab_id: number
  tracked_tab_url: string
  tracked_tab_title: string
  capture_logs: boolean
  capture_network: boolean
  capture_websocket: boolean
  capture_actions: boolean
}

/** Extension log entry */
export interface SyncExtensionLog {
  timestamp: string
  level: string
  message: string
  source: string
  category: string
  data?: unknown
}

/** Command result to send to server */
export interface SyncCommandResult {
  id: string
  correlation_id?: string
  status: 'complete' | 'error' | 'timeout'
  result?: unknown
  error?: string
}

/** Request sent to /sync */
interface SyncRequest {
  session_id: string
  settings?: SyncSettings
  extension_logs?: SyncExtensionLog[]
  last_command_ack?: string
  command_results?: SyncCommandResult[]
}

/** Command from server */
export interface SyncCommand {
  id: string
  type: string
  params: unknown
  correlation_id?: string
}

/** Response from /sync */
interface SyncResponse {
  ack: boolean
  commands: SyncCommand[]
  next_poll_ms: number
  server_time: string
  server_version?: string
  capture_overrides?: Record<string, string>
}

/** Sync state */
export interface SyncState {
  connected: boolean
  lastSyncAt: number
  consecutiveFailures: number
  lastCommandAck: string | null
}

/** Callbacks for sync client */
export interface SyncClientCallbacks {
  onCommand: (command: SyncCommand) => Promise<void>
  onConnectionChange: (connected: boolean) => void
  onCaptureOverrides?: (overrides: Record<string, string>) => void
  getSettings: () => Promise<SyncSettings>
  getExtensionLogs: () => SyncExtensionLog[]
  clearExtensionLogs: () => void
  debugLog?: (category: string, message: string, data?: unknown) => void
}

// =============================================================================
// CONSTANTS
// =============================================================================

const BASE_POLL_MS = 1000

// =============================================================================
// SYNC CLIENT CLASS
// =============================================================================

export class SyncClient {
  private serverUrl: string
  private sessionId: string
  private callbacks: SyncClientCallbacks
  private state: SyncState
  private intervalId: ReturnType<typeof setInterval> | null = null
  private running = false
  private pendingResults: SyncCommandResult[] = []
  private extensionVersion: string

  constructor(serverUrl: string, sessionId: string, callbacks: SyncClientCallbacks, extensionVersion = '') {
    this.serverUrl = serverUrl
    this.sessionId = sessionId
    this.callbacks = callbacks
    this.extensionVersion = extensionVersion
    this.state = {
      connected: false,
      lastSyncAt: 0,
      consecutiveFailures: 0,
      lastCommandAck: null,
    }
  }

  /** Get current sync state */
  getState(): SyncState {
    return { ...this.state }
  }

  /** Check if connected */
  isConnected(): boolean {
    return this.state.connected
  }

  /** Start the sync loop */
  start(): void {
    if (this.running) return
    this.running = true
    this.log('Starting sync client')
    this.scheduleNextSync(0) // Sync immediately
  }

  /** Stop the sync loop */
  stop(): void {
    this.running = false
    if (this.intervalId) {
      clearTimeout(this.intervalId)
      this.intervalId = null
    }
    this.log('Stopped sync client')
  }

  /** Queue a command result to send on next sync */
  queueCommandResult(result: SyncCommandResult): void {
    this.pendingResults.push(result)
    // Cap queue size to prevent memory leak if server is unreachable
    const MAX_PENDING_RESULTS = 200
    if (this.pendingResults.length > MAX_PENDING_RESULTS) {
      this.pendingResults.splice(0, this.pendingResults.length - MAX_PENDING_RESULTS)
    }
  }

  /** Reset connection state (e.g., when user toggles pilot/tracking) */
  resetConnection(): void {
    this.state.consecutiveFailures = 0
    this.log('Connection state reset')
    // Trigger immediate sync if running
    if (this.running && this.intervalId) {
      clearTimeout(this.intervalId)
      this.scheduleNextSync(0)
    }
  }

  /** Update server URL */
  setServerUrl(url: string): void {
    this.serverUrl = url
  }

  // =============================================================================
  // PRIVATE METHODS
  // =============================================================================

  private scheduleNextSync(delayMs: number): void {
    if (!this.running) return
    this.intervalId = setTimeout(() => this.doSync(), delayMs)
  }

  private async doSync(): Promise<void> {
    if (!this.running) return

    try {
      // Build request
      const settings = await this.callbacks.getSettings()
      const logs = this.callbacks.getExtensionLogs()

      const request: SyncRequest = {
        session_id: this.sessionId,
        settings,
      }

      // Include logs if any
      if (logs.length > 0) {
        request.extension_logs = logs
      }

      // Include pending command results
      if (this.pendingResults.length > 0) {
        request.command_results = [...this.pendingResults]
      }

      // Include last command ack
      if (this.state.lastCommandAck) {
        request.last_command_ack = this.state.lastCommandAck
      }

      // Make request with timeout to prevent hanging forever
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 3000) // 3s timeout

      const response = await fetch(`${this.serverUrl}/sync`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Gasoline-Extension-Version': this.extensionVersion,
        },
        body: JSON.stringify(request),
        signal: controller.signal,
      })

      clearTimeout(timeoutId)

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }

      const data: SyncResponse = await response.json()

      // Success - update state
      this.onSuccess()

      // Clear sent logs and results
      if (logs.length > 0) {
        this.callbacks.clearExtensionLogs()
      }
      this.pendingResults = []

      // Process commands
      if (data.commands && data.commands.length > 0) {
        this.log('Received commands', { count: data.commands.length })
        for (const command of data.commands) {
          // Track last command for ack
          this.state.lastCommandAck = command.id
          try {
            await this.callbacks.onCommand(command)
          } catch (err) {
            this.log('Error processing command', { id: command.id, error: (err as Error).message })
          }
        }
      }

      // Handle capture overrides
      if (data.capture_overrides && this.callbacks.onCaptureOverrides) {
        this.callbacks.onCaptureOverrides(data.capture_overrides)
      }

      // Schedule next sync (use server-provided interval or default)
      const nextPollMs = data.next_poll_ms || BASE_POLL_MS
      this.scheduleNextSync(nextPollMs)
    } catch (err) {
      // Failure - just retry after 1 second (no exponential backoff needed)
      this.onFailure()
      this.log('Sync failed, retrying', { error: (err as Error).message })
      this.scheduleNextSync(BASE_POLL_MS)
    }
  }

  private onSuccess(): void {
    const wasDisconnected = !this.state.connected
    this.state.connected = true
    this.state.lastSyncAt = Date.now()
    this.state.consecutiveFailures = 0

    if (wasDisconnected) {
      this.log('Connected')
      this.callbacks.onConnectionChange(true)
    }
  }

  private onFailure(): void {
    const wasConnected = this.state.connected
    this.state.connected = false
    this.state.consecutiveFailures++

    if (wasConnected) {
      this.log('Disconnected')
      this.callbacks.onConnectionChange(false)
    }
  }

  private log(message: string, data?: unknown): void {
    if (this.callbacks.debugLog) {
      this.callbacks.debugLog('sync', message, data)
    } else {
      console.log(`[SyncClient] ${message}`, data || '')
    }
  }
}

// =============================================================================
// FACTORY FUNCTION
// =============================================================================

/**
 * Create a sync client instance
 */
export function createSyncClient(
  serverUrl: string,
  sessionId: string,
  callbacks: SyncClientCallbacks,
  extensionVersion = '',
): SyncClient {
  return new SyncClient(serverUrl, sessionId, callbacks, extensionVersion)
}
