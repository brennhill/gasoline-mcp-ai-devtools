/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
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
  ConnectionStatus,
  WaterfallEntry
} from '../types'
import { getExtensionVersion } from './version-check'

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

  // Convert camelCase payload keys to snake_case for the Go server API
  const snakeBodies = bodies.map((b) => ({
    url: b.url,
    method: b.method,
    status: b.status,
    content_type: b.contentType,
    request_body: b.requestBody,
    response_body: b.responseBody,
    ...(b.responseTruncated ? { response_truncated: true } : {}),
    duration: b.duration,
    ...(b.tabId != null ? { tab_id: b.tabId } : {})
  }))

  const response = await fetch(`${serverUrl}/network-bodies`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify({ bodies: snakeBodies })
  })

  if (!response.ok) {
    const error = `Server error (network bodies): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${bodies.length} network bodies`)
}

/**
 * Send network waterfall data to server
 */
export async function sendNetworkWaterfallToServer(
  serverUrl: string,
  payload: { entries: WaterfallEntry[]; page_url: string },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${payload.entries.length} waterfall entries to server`)

  const response = await fetch(`${serverUrl}/network-waterfall`, {
    method: 'POST',
    headers: getRequestHeaders(),
    body: JSON.stringify(payload)
  })

  if (!response.ok) {
    const error = `Server error (network waterfall): ${response.status} ${response.statusText}`
    if (debugLogFn) debugLogFn('error', error)
    throw new Error(error)
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${payload.entries.length} waterfall entries`)
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
 * Post query results back to the server
 */
export async function postQueryResult(
  serverUrl: string,
  queryId: string,
  type: string,
  result: unknown,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  const endpoint = '/query-result'

  const logData = { queryId, type, endpoint, resultSize: JSON.stringify(result).length }
  if (debugLogFn) debugLogFn('api', `POST ${endpoint}`, logData)
  console.log(`[Gasoline API] POST ${endpoint}`, logData) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string

  try {
    const response = await fetch(`${serverUrl}${endpoint}`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify({ id: queryId, result })
    })

    if (!response.ok) {
      const errMsg = `Failed to post query result: HTTP ${response.status}`
      if (debugLogFn) debugLogFn('api', errMsg, { queryId, type, endpoint })
      console.error(`[Gasoline API] ${errMsg}`, { queryId, type, endpoint }) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
    } else {
      if (debugLogFn) debugLogFn('api', `POST ${endpoint} success`, { queryId })
      console.log(`[Gasoline API] POST ${endpoint} success`, { queryId }) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
    }
  } catch (err) {
    const errMsg = (err as Error).message
    if (debugLogFn) debugLogFn('api', `POST ${endpoint} error: ${errMsg}`, { queryId, type })
    console.error('[Gasoline API] Error posting query result:', { queryId, type, endpoint, error: errMsg })
  }
}

/**
 * POST async command result to server using correlation_id
 */
export async function postAsyncCommandResult(
  serverUrl: string,
  correlationId: string,
  status: 'pending' | 'complete' | 'timeout',
  result: unknown = null,
  error: string | null = null,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  const payload: {
    correlation_id: string
    status: string
    result?: unknown
    error?: string
  } = {
    correlation_id: correlationId,
    status: status
  }
  if (result !== null) {
    payload.result = result
  }
  if (error !== null) {
    payload.error = error
  }

  try {
    const response = await fetch(`${serverUrl}/query-result`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify(payload)
    })

    if (!response.ok) {
      console.error(`[Gasoline] Failed to post async command result: HTTP ${response.status}`, {
        // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
        correlationId,
        status
      })
    }
  } catch (err) {
    console.error('[Gasoline] Error posting async command result:', {
      correlationId,
      status,
      error: (err as Error).message
    })
    if (debugLogFn) {
      debugLogFn('connection', 'Failed to post async command result', {
        correlationId,
        status,
        error: (err as Error).message
      })
    }
  }
}

// NOTE: postSettings and pollCaptureSettings removed - use /sync for all communication

/**
 * Post extension logs to server
 */
export async function postExtensionLogs(
  serverUrl: string,
  logs: Array<{
    timestamp: string
    level: string
    message: string
    source: string
    category: string
    data?: unknown
  }>
): Promise<void> {
  if (logs.length === 0) return

  try {
    const response = await fetch(`${serverUrl}/extension-logs`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify({ logs })
    })

    if (!response.ok) {
      console.error(`[Gasoline] Failed to post extension logs: HTTP ${response.status}`, { count: logs.length }) // nosemgrep: javascript.lang.security.audit.unsafe-formatstring.unsafe-formatstring -- console.log with internal server state, not user-controlled format string
    }
  } catch (err) {
    console.error('[Gasoline] Error posting extension logs:', { count: logs.length, error: (err as Error).message })
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

/**
 * Poll server for pending queries
 */
export async function pollPendingQueries(
  serverUrl: string,
  sessionId: string,
  pilotState: '0' | '1',
  diagnosticLogFn?: (message: string) => void,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<
  Array<{
    id: string
    type: string
    params: string | Record<string, unknown>
    correlation_id?: string
  }>
> {
  try {
    if (diagnosticLogFn) {
      diagnosticLogFn(`[Diagnostic] Poll request: header=${pilotState}`)
    }

    const response = await fetch(`${serverUrl}/pending-queries`, {
      headers: {
        ...getRequestHeaders({ 'X-Gasoline-Session': sessionId, 'X-Gasoline-Pilot': pilotState })
      }
    })

    if (!response.ok) {
      if (debugLogFn) debugLogFn('connection', 'Poll pending-queries failed', { status: response.status })
      return []
    }

    const data = (await response.json()) as {
      queries?: Array<{
        id: string
        type: string
        params: string | Record<string, unknown>
        correlation_id?: string
      }>
    }

    if (!data.queries || data.queries.length === 0) return []

    if (debugLogFn) debugLogFn('connection', 'Got pending queries', { count: data.queries.length })
    return data.queries
  } catch (err) {
    if (debugLogFn) debugLogFn('connection', 'Poll pending-queries error', { error: (err as Error).message })
    return []
  }
}
