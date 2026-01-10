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
  WaterfallEntry,
} from '../types';

/**
 * Server health response
 */
export interface ServerHealthResponse {
  connected: boolean;
  error?: string;
  version?: string;
  logs?: {
    logFile?: string;
    logFileSize?: number;
    entries?: number;
    maxEntries?: number;
  };
}

/**
 * Send log entries to the server
 */
export async function sendLogsToServer(
  serverUrl: string,
  entries: LogEntry[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<{ entries: number }> {
  if (debugLogFn) debugLogFn('connection', `Sending ${entries.length} entries to server`);

  const response = await fetch(`${serverUrl}/logs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ entries }),
  });

  if (!response.ok) {
    const error = `Server error: ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  const result = (await response.json()) as { entries: number };
  if (debugLogFn) debugLogFn('connection', `Server accepted entries, total: ${result.entries}`);
  return result;
}

/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(
  serverUrl: string,
  events: WebSocketEvent[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${events.length} WS events to server`);

  const response = await fetch(`${serverUrl}/websocket-events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ events }),
  });

  if (!response.ok) {
    const error = `Server error (WS): ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${events.length} WS events`);
}

/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(
  serverUrl: string,
  bodies: NetworkBodyPayload[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${bodies.length} network bodies to server`);

  const response = await fetch(`${serverUrl}/network-bodies`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ bodies }),
  });

  if (!response.ok) {
    const error = `Server error (network bodies): ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${bodies.length} network bodies`);
}

/**
 * Send network waterfall data to server
 */
export async function sendNetworkWaterfallToServer(
  serverUrl: string,
  payload: { entries: WaterfallEntry[]; pageURL: string },
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${payload.entries.length} waterfall entries to server`);

  const response = await fetch(`${serverUrl}/network-waterfall`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    const error = `Server error (network waterfall): ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${payload.entries.length} waterfall entries`);
}

/**
 * Send enhanced actions to server
 */
export async function sendEnhancedActionsToServer(
  serverUrl: string,
  actions: EnhancedAction[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${actions.length} enhanced actions to server`);

  const response = await fetch(`${serverUrl}/enhanced-actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ actions }),
  });

  if (!response.ok) {
    const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${actions.length} enhanced actions`);
}

/**
 * Send performance snapshots to server
 */
export async function sendPerformanceSnapshotsToServer(
  serverUrl: string,
  snapshots: PerformanceSnapshot[],
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  if (debugLogFn) debugLogFn('connection', `Sending ${snapshots.length} performance snapshots to server`);

  const response = await fetch(`${serverUrl}/performance-snapshots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ snapshots }),
  });

  if (!response.ok) {
    const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`;
    if (debugLogFn) debugLogFn('error', error);
    throw new Error(error);
  }

  if (debugLogFn) debugLogFn('connection', `Server accepted ${snapshots.length} performance snapshots`);
}

/**
 * Check server health
 */
export async function checkServerHealth(serverUrl: string): Promise<ServerHealthResponse> {
  try {
    const response = await fetch(`${serverUrl}/health`);

    if (!response.ok) {
      return { connected: false, error: `HTTP ${response.status}` };
    }

    let data: ServerHealthResponse;
    try {
      data = (await response.json()) as ServerHealthResponse;
    } catch {
      return {
        connected: false,
        error: 'Server returned invalid response - check Server URL in options',
      };
    }
    return {
      ...data,
      connected: true,
    };
  } catch (error) {
    return {
      connected: false,
      error: (error as Error).message,
    };
  }
}

/**
 * Update extension badge
 */
export function updateBadge(status: ConnectionStatus): void {
  if (typeof chrome === 'undefined' || !chrome.action) return;

  if (status.connected) {
    const errorCount = status.errorCount || 0;

    chrome.action.setBadgeText({
      text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount),
    });

    chrome.action.setBadgeBackgroundColor({
      color: '#3fb950',
    });
  } else {
    chrome.action.setBadgeText({ text: '!' });
    chrome.action.setBadgeBackgroundColor({
      color: '#f85149',
    });
  }
}

/**
 * Post query results back to the server
 */
export async function postQueryResult(
  serverUrl: string,
  queryId: string,
  type: string,
  result: unknown
): Promise<void> {
  let endpoint: string;
  if (type === 'a11y') {
    endpoint = '/a11y-result';
  } else if (type === 'state') {
    endpoint = '/state-result';
  } else if (type === 'highlight') {
    endpoint = '/highlight-result';
  } else if (type === 'execute' || type === 'browser_action') {
    endpoint = '/execute-result';
  } else {
    endpoint = '/dom-result';
  }

  await fetch(`${serverUrl}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: queryId, result }),
  });
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
    correlation_id: string;
    status: string;
    result?: unknown;
    error?: string;
  } = {
    correlation_id: correlationId,
    status: status,
  };
  if (result !== null) {
    payload.result = result;
  }
  if (error !== null) {
    payload.error = error;
  }

  try {
    await fetch(`${serverUrl}/execute-result`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
  } catch (err) {
    if (debugLogFn) {
      debugLogFn('connection', 'Failed to post async command result', {
        correlationId,
        status,
        error: (err as Error).message,
      });
    }
  }
}

/**
 * Post extension settings to server
 */
export async function postSettings(
  serverUrl: string,
  sessionId: string,
  settings: Record<string, boolean | string>,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<void> {
  try {
    await fetch(`${serverUrl}/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: sessionId,
        settings: settings,
      }),
    });
    if (debugLogFn) debugLogFn('connection', 'Posted settings to server', settings);
  } catch (err) {
    if (debugLogFn) debugLogFn('connection', 'Failed to post settings', { error: (err as Error).message });
  }
}

/**
 * Poll the server's /settings endpoint for AI capture overrides
 */
export async function pollCaptureSettings(
  serverUrl: string
): Promise<Record<string, string> | null> {
  try {
    const response = await fetch(`${serverUrl}/settings`);
    if (!response.ok) return null;
    const data = (await response.json()) as { capture_overrides?: Record<string, string> };
    return data.capture_overrides || {};
  } catch {
    return null;
  }
}

/**
 * Post extension logs to server
 */
export async function postExtensionLogs(
  serverUrl: string,
  logs: Array<{
    timestamp: string;
    level: string;
    message: string;
    source: string;
    category: string;
    data?: unknown;
  }>
): Promise<void> {
  if (logs.length === 0) return;

  try {
    await fetch(`${serverUrl}/extension-logs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ logs }),
    });
  } catch (err) {
    console.error('[Gasoline] Failed to post extension logs', err);
  }
}

/**
 * Send status ping to server
 */
export async function sendStatusPing(
  serverUrl: string,
  statusMessage: {
    type: string;
    tracking_enabled: boolean;
    tracked_tab_id: number | null;
    tracked_tab_url: string | null;
    message: string;
    extension_connected: boolean;
    timestamp: string;
  },
  diagnosticLogFn?: (message: string) => void
): Promise<void> {
  try {
    await fetch(`${serverUrl}/api/extension-status`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(statusMessage),
    });
  } catch (err) {
    if (diagnosticLogFn) {
      diagnosticLogFn('[Gasoline] Status ping error: ' + (err as Error).message);
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
): Promise<Array<{
  id: string;
  type: string;
  params: string | Record<string, unknown>;
  correlation_id?: string;
}>> {
  try {
    if (diagnosticLogFn) {
      diagnosticLogFn(`[Diagnostic] Poll request: header=${pilotState}`);
    }

    const response = await fetch(`${serverUrl}/pending-queries`, {
      headers: {
        'X-Gasoline-Session': sessionId,
        'X-Gasoline-Pilot': pilotState,
      },
    });

    if (!response.ok) {
      if (debugLogFn) debugLogFn('connection', 'Poll pending-queries failed', { status: response.status });
      return [];
    }

    const data = (await response.json()) as {
      queries?: Array<{
        id: string;
        type: string;
        params: string | Record<string, unknown>;
        correlation_id?: string;
      }>;
    };

    if (!data.queries || data.queries.length === 0) return [];

    if (debugLogFn) debugLogFn('connection', 'Got pending queries', { count: data.queries.length });
    return data.queries;
  } catch (err) {
    if (debugLogFn) debugLogFn('connection', 'Poll pending-queries error', { error: (err as Error).message });
    return [];
  }
}
