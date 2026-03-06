/**
 * Purpose: Shared daemon HTTP request helpers (headers + JSON body init) used by extension modules.
 * Why: Keep daemon request contracts consistent across background, popup, and options surfaces.
 */

export interface DaemonHeaderOptions {
  clientName?: string
  extensionVersion?: string
  contentType?: string | null
  additionalHeaders?: Record<string, string>
}

export interface DaemonJSONRequestOptions extends DaemonHeaderOptions {
  method?: string
  signal?: AbortSignal
}

export interface PostDaemonJSONOptions extends DaemonJSONRequestOptions {
  timeoutMs?: number
}

const DEFAULT_CLIENT_NAME = 'gasoline-extension'

/**
 * Build standard daemon request headers with a shared extension client identifier.
 */
export function buildDaemonHeaders(options: DaemonHeaderOptions = {}): Record<string, string> {
  const {
    clientName = DEFAULT_CLIENT_NAME,
    extensionVersion,
    contentType = 'application/json',
    additionalHeaders = {}
  } = options

  const normalizedVersion = typeof extensionVersion === 'string' && extensionVersion.trim().length > 0
    ? extensionVersion.trim()
    : ''

  const headers: Record<string, string> = {
    'X-Gasoline-Client': normalizedVersion ? `${clientName}/${normalizedVersion}` : clientName
  }

  if (contentType !== null) {
    headers['Content-Type'] = contentType
  }
  if (normalizedVersion) {
    headers['X-Gasoline-Extension-Version'] = normalizedVersion
  }

  return {
    ...headers,
    ...additionalHeaders
  }
}

/**
 * Build a JSON request init object for daemon endpoints.
 */
export function buildDaemonJSONRequestInit(
  payload: unknown,
  options: DaemonJSONRequestOptions = {}
): RequestInit {
  const { method = 'POST', signal, ...headerOptions } = options
  return {
    method,
    headers: buildDaemonHeaders(headerOptions),
    body: JSON.stringify(payload),
    ...(signal ? { signal } : {})
  }
}

/**
 * POST JSON to a daemon endpoint with optional timeout handling.
 */
export async function postDaemonJSON(
  url: string,
  payload: unknown,
  options: PostDaemonJSONOptions = {}
): Promise<Response> {
  const { timeoutMs, signal, ...requestOptions } = options
  const effectiveSignal = signal ||
    (typeof timeoutMs === 'number' && timeoutMs > 0 && typeof AbortSignal.timeout === 'function'
      ? AbortSignal.timeout(timeoutMs)
      : undefined)

  return fetch(url, buildDaemonJSONRequestInit(payload, { ...requestOptions, signal: effectiveSignal }))
}
