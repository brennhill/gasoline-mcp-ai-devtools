/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Gasoline MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Gasoline server.
 */

import type {
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot,
  ConnectionStatus
} from '../types/index.js'
import { getExtensionVersion } from './version-check.js'

/**
 * Server health response
 */
export interface ServerHealthResponse {
  connected: boolean
  error?: string
  version?: string
  availableVersion?: string
  logs?: {
    logFile?: string
    logFileSize?: number
    entries?: number
    maxEntries?: number
  }
}

/**
 * Get standard headers for API requests including version header
 */
export function getRequestHeaders(additionalHeaders: Record<string, string> = {}): Record<string, string> {
  return {
    'Content-Type': 'application/json',
    'X-Gasoline-Client': `gasoline-extension/${getExtensionVersion()}`,
    'X-Gasoline-Extension-Version': getExtensionVersion(),
    ...additionalHeaders
  }
}

/**
 * Send log entries to the server
 */
export async function sendLogsToServer(
  serverUrl: string,
  entries: LogEntry[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<{ entries: number }> {
  if (debugLogFn) debugLogFn('connection', `Sending ${entries.length} entries to server`)

  const response = await fetch(`${serverUrl}/logs`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ entries })
  })

  if (!response.ok) {
    const error = `Server error: ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  const result = (await response.json()) as { entries: number }
  if (debugLogFn) debugLogFn('connection', `Server accepted entries, total: ${result.entries}`)
  return result
}

/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(
  serverUrl: string,
  events: WebSocketEvent[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${events.length} WS events to server`)

  const response = await fetch(`${serverUrl}/websocket-events`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ events })
  })

  if (!response.ok) {
    const error = `Server error (WS): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${events.length} WS events`)
}

/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(
  serverUrl: string,
  bodies: NetworkBodyPayload[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${bodies.length} network bodies to server`)

  const response = await fetch(`${serverUrl}/network-bodies`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ bodies })
  })

  if (!response.ok) {
    const error = `Server error (network bodies): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${bodies.length} network bodies`)
}

/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(
  serverUrl: string,
  actions: EnhancedAction[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${actions.length} enhanced actions to server`)

  const response = await fetch(`${serverUrl}/enhanced-actions`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ actions })
  })

  if (!response.ok) {
    const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${actions.length} enhanced actions`)
}

/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(
  serverUrl: string,
  snapshots: PerformanceSnapshot[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${snapshots.length} performance snapshots to server`)

  const response = await fetch(`${serverUrl}/performance-snapshots`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ snapshots })
  })

  if (!response.ok) {
    const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${snapshots.length} performance snapshots`)
}

/**
 * Check server health
 */
export async function checkServerHealth(serverUrl: string): Promise<ServerHealthResponse> {
  try {
    const response = await fetch(`${serverUrl}/health`)

    if (!response.ok) {
      return { connected: false, error: `HTTP ${response.status}` }
    }

    let data: ServerHealthResponse
    try {
      data = (await response.json()) as ServerHealthResponse
    } catch {
      return {
        connected: false,
        error: 'Server returned invalid response - check Server URL in options'
      }
    }
    return {
      ...data,
      connected: true
    }
  } catch (error) {
    return {
      connected: false,
      error: (error as Error).message
    }
  }
}

/**
 * Update extension badge.
 * Uses Promise.all to ensure both text and color are applied atomically
 * before the MV3 service worker can be suspended.
 */
export function updateBadge(status: ConnectionStatus): void {
  if (typeof chrome === 'undefined' || !chrome.action) return

  if (status.connected) {
    const errorCount = status.errorCount || 0

    Promise.all([
      chrome.action.setBadgeText({
        text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount)
      }),
      chrome.action.setBadgeBackgroundColor({
        color: '#3fb950'
      })
    ]).catch(() => {
      /* badge update failed — SW may be shutting down */
    })
  } else {
    Promise.all([
      chrome.action.setBadgeText({ text: '!' }),
      chrome.action.setBadgeBackgroundColor({
        color: '#f85149'
      })
    ]).catch(() => {
      /* badge update failed — SW may be shutting down */
    })
  }
}

/**
 * Send status ping to server
 */
export async function sendStatusPing(
  serverUrl: string,
  statusMessage: {
    type: string
    tracking_enabled: boolean
    tracked_tab_id: number | null
    tracked_tab_url: string | null
    message: string
    extension_connected: boolean
    timestamp: string
  },
  diagnosticLogFn?: (message: string) => void
): Promise<void> {
  try {
    const response = await fetch(`${serverUrl}/api/extension-status`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify(statusMessage)
    })

    if (!response.ok) {
      console.error(`[Gasoline] Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type }) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
    }
  } catch (err) {
    console.error('[Gasoline] Error sending status ping:', { type: statusMessage.type, error: (err as Error).message })
    if (diagnosticLogFn) {
      diagnosticLogFn('[Gasoline] Status ping error: ' + (err as Error).message)
    }
  }
}
