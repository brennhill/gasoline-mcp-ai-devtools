/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/backend-log-streaming/index.md
 */

/**
 * @fileoverview Network waterfall and body capture.
 * Provides PerformanceResourceTiming parsing, pending request tracking,
 * fetch body capture with size limits, and sensitive header sanitization.
 */

import type { WaterfallEntry, WaterfallPhases, PendingRequest } from '../types/index'

import {
  MAX_WATERFALL_ENTRIES,
  WATERFALL_TIME_WINDOW_MS,
  REQUEST_BODY_MAX,
  RESPONSE_BODY_MAX,
  BODY_READ_TIMEOUT_MS,
  SENSITIVE_HEADER_PATTERNS,
  BINARY_CONTENT_TYPES
} from './constants.js'

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/**
 * Options for filtering network waterfall entries
 */
interface WaterfallFilterOptions {
  since?: number
  initiatorTypes?: string[]
}

/**
 * Truncation result for request/response bodies
 */
interface TruncationResult {
  body: string | null
  truncated: boolean
}

/**
 * Internal pending request tracking with mutable id
 */
interface InternalPendingRequest {
  id: string
  url: string
  method: string
  startTime: number
}

/**
 * Request info for tracking
 */
interface RequestInfo {
  url: string
  method: string
  startTime: number
}

/**
 * Network body payload posted to content script
 */
interface NetworkBodyPostMessage {
  type: 'GASOLINE_NETWORK_BODY'
  payload: {
    url: string
    method: string
    status: number
    contentType: string
    requestBody?: string
    responseBody?: string
    responseTruncated?: boolean
    duration: number
  }
}

// =============================================================================
// MODULE STATE
// =============================================================================

// Configured server URL for filtering (updated via setServerUrl)
let configuredServerUrl = ''

// Network Waterfall state
let networkWaterfallEnabled = false
const pendingRequests = new Map<string, InternalPendingRequest>() // requestId -> { url, method, startTime }
let requestIdCounter = 0

// Network body capture state
let networkBodyCaptureEnabled = true // Default: capture request/response bodies

/** URL patterns for auth endpoints whose response bodies should be redacted */
const SENSITIVE_URL_PATTERNS = /\/(auth|login|signin|signup|token|oauth|session|api[_-]?key|password|register)\b/i

// =============================================================================
// NETWORK WATERFALL
// =============================================================================

/**
 * Parse a PerformanceResourceTiming entry into waterfall phases
 * @param timing - The timing entry
 * @returns Parsed waterfall entry
 */
export function parseResourceTiming(timing: PerformanceResourceTiming): WaterfallEntry {
  const phases: WaterfallPhases = {
    dns: Math.max(0, timing.domainLookupEnd - timing.domainLookupStart),
    connect: Math.max(0, timing.connectEnd - timing.connectStart),
    tls: timing.secureConnectionStart > 0 ? Math.max(0, timing.connectEnd - timing.secureConnectionStart) : 0,
    ttfb: Math.max(0, timing.responseStart - timing.requestStart),
    download: Math.max(0, timing.responseEnd - timing.responseStart)
  }

  const result: WaterfallEntry = {
    url: timing.name,
    initiatorType: timing.initiatorType,
    startTime: timing.startTime,
    duration: timing.duration,
    phases,
    transferSize: timing.transferSize || 0,
    encodedBodySize: timing.encodedBodySize || 0,
    decodedBodySize: timing.decodedBodySize || 0
  }

  // Detect cache hit
  if (timing.transferSize === 0 && timing.encodedBodySize > 0) {
    ;(result as { cached?: boolean }).cached = true
  }

  return result
}

/**
 * Get network waterfall entries
 * @param options - Options for filtering
 * @returns Array of waterfall entries
 */
export function getNetworkWaterfall(options: WaterfallFilterOptions = {}): WaterfallEntry[] {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let entries = (performance.getEntriesByType('resource') as PerformanceResourceTiming[]) || []

    // Filter by time range
    if (options.since) {
      entries = entries.filter((e) => e.startTime >= options.since!)
    }

    // Filter by initiator type
    if (options.initiatorTypes) {
      entries = entries.filter((e) => options.initiatorTypes!.includes(e.initiatorType))
    }

    // Exclude data URLs
    entries = entries.filter((e) => !e.name.startsWith('data:'))

    // Sort by start time
    entries.sort((a, b) => a.startTime - b.startTime)

    // Limit entries
    if (entries.length > MAX_WATERFALL_ENTRIES) {
      entries = entries.slice(-MAX_WATERFALL_ENTRIES)
    }

    return entries.map(parseResourceTiming)
  } catch {
    return []
  }
}

/**
 * Track a pending request
 * @param request - Request info { url, method, startTime }
 * @returns Request ID
 */
export function trackPendingRequest(request: RequestInfo): string {
  const id = `req_${++requestIdCounter}`
  pendingRequests.set(id, {
    ...request,
    id
  })
  return id
}

/**
 * Complete a pending request
 * @param requestId - The request ID to complete
 */
export function completePendingRequest(requestId: string): void {
  pendingRequests.delete(requestId)
}

/**
 * Get all pending requests
 * @returns Array of pending requests
 */
export function getPendingRequests(): PendingRequest[] {
  return Array.from(pendingRequests.values())
}

/**
 * Clear all pending requests
 */
export function clearPendingRequests(): void {
  pendingRequests.clear()
}

/**
 * Network waterfall snapshot for an error
 */
interface NetworkWaterfallSnapshot {
  type: 'network_waterfall'
  ts: string
  _errorTs: string
  entries: WaterfallEntry[]
  pending: PendingRequest[]
}

/**
 * Error entry with timestamp
 */
interface ErrorEntry {
  ts: string
}

/**
 * Get network waterfall snapshot for an error
 * @param errorEntry - The error entry
 * @returns The waterfall snapshot
 */
export async function getNetworkWaterfallForError(errorEntry: ErrorEntry): Promise<NetworkWaterfallSnapshot | null> {
  if (!networkWaterfallEnabled) return null

  const now = typeof performance !== 'undefined' && performance?.now ? performance.now() : 0
  const since = Math.max(0, now - WATERFALL_TIME_WINDOW_MS)

  const entries = getNetworkWaterfall({ since })
  const pending = getPendingRequests()

  return {
    type: 'network_waterfall',
    ts: new Date().toISOString(),
    _errorTs: errorEntry.ts,
    entries,
    pending
  }
}

/**
 * Set whether network waterfall is enabled
 * @param enabled - Whether to enable network waterfall
 */
export function setNetworkWaterfallEnabled(enabled: boolean): void {
  networkWaterfallEnabled = enabled
}

/**
 * Check if network waterfall is enabled
 * @returns Whether network waterfall is enabled
 */
export function isNetworkWaterfallEnabled(): boolean {
  return networkWaterfallEnabled
}

// =============================================================================
// NETWORK BODY CAPTURE
// =============================================================================

/**
 * Set whether network body capture is enabled
 * @param enabled - Whether to enable body capture
 */
export function setNetworkBodyCaptureEnabled(enabled: boolean): void {
  networkBodyCaptureEnabled = enabled
}

/**
 * Check if network body capture is enabled
 * @returns Whether body capture is enabled
 */
export function isNetworkBodyCaptureEnabled(): boolean {
  return networkBodyCaptureEnabled
}

/**
 * Set the configured server URL for capture filtering.
 * Called when the server URL is loaded from settings.
 * @param url - The server URL (e.g., 'http://localhost:7890')
 */
export function setServerUrl(url: string): void {
  configuredServerUrl = url || ''
}

/**
 * Check if a URL should be captured (not gasoline server or extension)
 * @param url - The URL to check
 * @returns True if the URL should be captured
 */
export function shouldCaptureUrl(url: string): boolean {
  if (!url) return true
  // Filter against the configured server URL if set
  if (configuredServerUrl) {
    try {
      const serverParsed = new URL(configuredServerUrl)
      const hostPort = serverParsed.host // e.g., 'localhost:7890'
      if (url.includes(hostPort)) return false
    } catch {
      // Fall through to hardcoded defaults
    }
  }
  // Hardcoded fallback for default server URL
  if (url.includes('localhost:7890') || url.includes('127.0.0.1:7890')) return false
  if (url.startsWith('chrome-extension://')) return false
  return true
}

/**
 * Sanitize headers by removing sensitive ones
 * @param headers - Headers to sanitize
 * @returns Sanitized headers object
 */
// #lizard forgives
export function sanitizeHeaders(
  headers: HeadersInit | Headers | Record<string, string> | null
): Record<string, string> {
  if (!headers) return {}

  const result: Record<string, string> = {}

  if (headers instanceof Headers || typeof (headers as unknown as Headers).forEach === 'function') {
    // Headers object or Map
    ;(headers as unknown as Headers).forEach((value: string, key: string) => {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    })
  } else if (typeof (headers as { entries?: () => Iterable<[string, string]> }).entries === 'function') {
    for (const [key, value] of (headers as { entries: () => Iterable<[string, string]> }).entries()) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    }
  } else if (typeof headers === 'object') {
    for (const [key, value] of Object.entries(headers as Record<string, string>)) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    }
  }

  return result
}

/**
 * Truncate request body at 8KB limit
 * @param body - The request body
 * @returns Truncation result
 */
export function truncateRequestBody(body: string | null | undefined): TruncationResult {
  if (body === null || body === undefined) return { body: null, truncated: false }
  if (body.length <= REQUEST_BODY_MAX) return { body, truncated: false }
  return { body: body.slice(0, REQUEST_BODY_MAX), truncated: true }
}

/**
 * Truncate response body at 16KB limit
 * @param body - The response body
 * @returns Truncation result
 */
export function truncateResponseBody(body: string | null | undefined): TruncationResult {
  if (body === null || body === undefined) return { body: null, truncated: false }
  if (body.length <= RESPONSE_BODY_MAX) return { body, truncated: false }
  return { body: body.slice(0, RESPONSE_BODY_MAX), truncated: true }
}

/**
 * Read a response body, returning text for text types and size info for binary
 * @param response - The cloned response object
 * @returns The body content or binary size placeholder
 */
export async function readResponseBody(response: Response): Promise<string> {
  const contentType = response.headers?.get?.('content-type') || ''

  if (BINARY_CONTENT_TYPES.test(contentType)) {
    const blob = await response.blob()
    return `[Binary: ${blob.size} bytes, ${contentType}]`
  }

  // Text-like or unknown content type: try reading as text
  return await response.text()
}

/**
 * Read response body with a timeout
 * @param response - The cloned response object
 * @param timeoutMs - Timeout in milliseconds
 * @returns The body or timeout message
 */
export async function readResponseBodyWithTimeout(
  response: Response,
  timeoutMs: number = BODY_READ_TIMEOUT_MS
): Promise<string> {
  return Promise.race([
    readResponseBody(response),
    new Promise<string>((resolve) => {
      setTimeout(() => resolve('[Skipped: body read timeout]'), timeoutMs)
    })
  ])
}

/**
 * Reset all module state for testing purposes
 * Clears pending requests, resets counters, and restores default settings.
 * Call this in beforeEach/afterEach test hooks to prevent test pollution.
 */
export function resetForTesting(): void {
  configuredServerUrl = ''
  networkWaterfallEnabled = false
  pendingRequests.clear()
  requestIdCounter = 0
  networkBodyCaptureEnabled = true
}

/**
 * Type alias for fetch-like functions (avoids overload complexity)
 */
type FetchLike = (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>

/**
 * Wrap a fetch function to capture request/response bodies
 * @param fetchFn - The original fetch function
 * @returns Wrapped fetch that captures bodies
 */
function extractFetchInfo(
  input: RequestInfo | URL,
  init?: RequestInit
): { url: string; method: string; requestBody: BodyInit | null | undefined } {
  let url = ''
  let method = 'GET'
  if (typeof input === 'string') {
    url = input
  } else if (input && (input as unknown as Request).url) {
    url = (input as unknown as Request).url
    method = (input as unknown as Request).method || 'GET'
  }
  if (init) {
    method = init.method || method
  }
  return { url, method, requestBody: init?.body || null }
}

async function readCapturedBody(url: string, cloned: Response | null, contentType: string): Promise<string> {
  if (SENSITIVE_URL_PATTERNS.test(url)) return '[REDACTED: auth endpoint]'
  if (!cloned) return ''
  if (BINARY_CONTENT_TYPES.test(contentType)) {
    const blob = await cloned.blob()
    return `[Binary: ${blob.size} bytes, ${contentType}]`
  }
  return readResponseBodyWithTimeout(cloned)
}

function postNetworkBody(
  win: Window,
  url: string,
  method: string,
  response: Response,
  contentType: string,
  requestBody: BodyInit | null | undefined,
  duration: number,
  truncResp: string,
  truncReq: string | null,
  responseTruncated: boolean
): void {
  const message: NetworkBodyPostMessage = {
    type: 'GASOLINE_NETWORK_BODY',
    payload: {
      url,
      method,
      status: response.status,
      contentType,
      requestBody: truncReq || (typeof requestBody === 'string' ? requestBody : undefined),
      responseBody: truncResp,
      ...(responseTruncated ? { responseTruncated: true } : {}),
      duration
    }
  }
  win.postMessage(message, window.location.origin)
}

export function wrapFetchWithBodies(fetchFn: FetchLike): FetchLike {
  return async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const { url, method, requestBody } = extractFetchInfo(input, init)
    if (!shouldCaptureUrl(url)) return fetchFn(input, init)

    const startTime = Date.now()
    const response = await fetchFn(input, init)
    const duration = Date.now() - startTime
    const contentType = response.headers?.get?.('content-type') || ''
    const cloned = response.clone ? response.clone() : null
    const win = typeof window !== 'undefined' ? window : null

    Promise.resolve()
      .then(async () => {
        try {
          const responseBody = await readCapturedBody(url, cloned, contentType)
          const { body: truncResp, truncated: respTruncated } = truncateResponseBody(responseBody)
          const rawReq = SENSITIVE_URL_PATTERNS.test(url)
            ? '[REDACTED: auth endpoint]'
            : typeof requestBody === 'string'
              ? requestBody
              : null
          const { body: truncReq } = truncateRequestBody(rawReq)
          if (win && networkBodyCaptureEnabled) {
            postNetworkBody(
              win,
              url,
              method,
              response,
              contentType,
              requestBody,
              duration,
              truncResp || responseBody,
              truncReq,
              respTruncated
            )
          }
        } catch {
          /* Body capture failure should not affect user code */
        }
      })
      .catch((err: Error) => {
        console.debug('[Gasoline] Network body capture error:', err)
      })

    return response
  }
}
