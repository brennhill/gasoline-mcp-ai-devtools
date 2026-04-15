/**
 * Purpose: HTTP functions for sending telemetry data (logs, WebSocket events, network bodies, actions, performance) to the Kaboom MCP server.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Server Communication - HTTP functions for sending data to
 * the Kaboom server.
 */

import type {
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot,
  ConnectionStatus
} from '../types/index.js'
import { KABOOM_LOG_PREFIX } from '../lib/brand.js'
import { getExtensionVersion } from './version-check.js'
import { errorMessage } from '../lib/error-utils.js'
import { buildDaemonHeaders } from '../lib/daemon-http.js'

/**
 * Server health response
 */
export interface ServerHealthResponse {
  connected: boolean
  error?: string
  version?: string
  availableVersion?: string
  capture?: {
    available?: boolean
    pilot_enabled?: boolean
    pilot_state?: string
    extension_connected?: boolean
    extension_last_seen?: string
    extension_client_id?: string
    security_mode?: string
    production_parity?: boolean
    insecure_rewrites?: number
  }
  logs?: {
    logFile?: string
    logFileSize?: number
    entries?: number
    maxEntries?: number
  }
}

function buildHeartbeatStatusError(capture: ServerHealthResponse['capture']): string {
  if (!capture || typeof capture.extension_connected !== 'boolean') {
    return 'Server reachable, but extension heartbeat status is unavailable. Update the server and extension, then reopen the popup.'
  }
  if (capture.extension_last_seen && capture.extension_last_seen.trim().length > 0) {
    return `Server reachable, but extension heartbeat is stale (last seen ${capture.extension_last_seen}). Reopen the Kaboom popup and click "Track This Tab".`
  }
  return 'Server reachable, but extension heartbeat is missing. Open the Kaboom popup and click "Track This Tab".'
}

/**
 * Get standard headers for API requests including version header
 */
export function getRequestHeaders(additionalHeaders: Record<string, string> = {}): Record<string, string> {
  return buildDaemonHeaders({
    extensionVersion: getExtensionVersion(),
    additionalHeaders
  })
}

/**
 * Generic telemetry batch sender. All telemetry POST endpoints follow the same
 * pattern: log count, POST JSON, check response.ok, log acceptance.
 * Includes AbortSignal.timeout(10000) to prevent hanging requests.
 */
async function sendTelemetryBatch<T>(
  serverUrl: string,
  endpoint: string,
  payloadKey: string,
  items: T[],
  label: string,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<Response> {
  if (debugLogFn) debugLogFn('connection', `Sending ${items.length} ${label} to server`)

  const response = await fetch(`${serverUrl}${endpoint}`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ [payloadKey]: items }),
    signal: AbortSignal.timeout(10000)
  })

  if (!response.ok) {
    const error = `Server error (${label}): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${items.length} ${label}`)
  return response
}

/**
 * Send log entries to the server
 */
export async function sendLogsToServer(
  serverUrl: string,
  entries: LogEntry[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<{ entries: number }> {
  const response = await sendTelemetryBatch(serverUrl, '/logs', 'entries', entries, 'entries', debugLogFn)
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
  await sendTelemetryBatch(serverUrl, '/websocket-events', 'events', events, 'WS events', debugLogFn)
}

/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(
  serverUrl: string,
  bodies: NetworkBodyPayload[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  await sendTelemetryBatch(serverUrl, '/network-bodies', 'bodies', bodies, 'network bodies', debugLogFn)
}

/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(
  serverUrl: string,
  actions: EnhancedAction[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  await sendTelemetryBatch(serverUrl, '/enhanced-actions', 'actions', actions, 'enhanced actions', debugLogFn)
}

/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(
  serverUrl: string,
  snapshots: PerformanceSnapshot[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  await sendTelemetryBatch(serverUrl, '/performance-snapshots', 'snapshots', snapshots, 'performance snapshots', debugLogFn)
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
    if (data.capture?.extension_connected !== true) {
      return {
        ...data,
        connected: false,
        error: buildHeartbeatStatusError(data.capture)
      }
    }
    return {
      ...data,
      connected: true
    }
  } catch (error) {
    return {
      connected: false,
      error: errorMessage(error)
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
      console.error(`${KABOOM_LOG_PREFIX} Failed to send status ping: HTTP ${response.status}`, { type: statusMessage.type }) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
    }
  } catch (err) {
    console.error(`${KABOOM_LOG_PREFIX} Error sending status ping:`, { type: statusMessage.type, error: errorMessage(err) })
    if (diagnosticLogFn) {
      diagnosticLogFn(`${KABOOM_LOG_PREFIX} Status ping error: ${errorMessage(err)}`)
    }
  }
}
