// @ts-nocheck
/**
 * @fileoverview Network waterfall and body capture.
 * Provides PerformanceResourceTiming parsing, pending request tracking,
 * fetch body capture with size limits, and sensitive header sanitization.
 */

import {
  MAX_WATERFALL_ENTRIES,
  WATERFALL_TIME_WINDOW_MS,
  REQUEST_BODY_MAX,
  RESPONSE_BODY_MAX,
  BODY_READ_TIMEOUT_MS,
  SENSITIVE_HEADER_PATTERNS,
  BINARY_CONTENT_TYPES,
} from './constants.js'

// Network Waterfall state
let networkWaterfallEnabled = false
const pendingRequests = new Map() // requestId -> { url, method, startTime }
let requestIdCounter = 0

// Network body capture state
let networkBodyCaptureEnabled = true // Default: capture request/response bodies

// =============================================================================
// NETWORK WATERFALL
// =============================================================================

/**
 * Parse a PerformanceResourceTiming entry into waterfall phases
 * @param {PerformanceResourceTiming} timing - The timing entry
 * @returns {Object} Parsed waterfall entry
 */
export function parseResourceTiming(timing) {
  const result = {
    url: timing.name,
    initiatorType: timing.initiatorType,
    startTime: timing.startTime,
    duration: timing.duration,
    phases: {
      dns: Math.max(0, timing.domainLookupEnd - timing.domainLookupStart),
      connect: Math.max(0, timing.connectEnd - timing.connectStart),
      tls: timing.secureConnectionStart > 0 ? Math.max(0, timing.connectEnd - timing.secureConnectionStart) : 0,
      ttfb: Math.max(0, timing.responseStart - timing.requestStart),
      download: Math.max(0, timing.responseEnd - timing.responseStart),
    },
    transferSize: timing.transferSize || 0,
    encodedBodySize: timing.encodedBodySize || 0,
    decodedBodySize: timing.decodedBodySize || 0,
  }

  // Detect cache hit
  if (timing.transferSize === 0 && timing.encodedBodySize > 0) {
    result.cached = true
  }

  return result
}

/**
 * Get network waterfall entries
 * @param {Object} options - Options for filtering
 * @returns {Array} Array of waterfall entries
 */
export function getNetworkWaterfall(options = {}) {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let entries = performance.getEntriesByType('resource') || []

    // Filter by time range
    if (options.since) {
      entries = entries.filter((e) => e.startTime >= options.since)
    }

    // Filter by initiator type
    if (options.initiatorTypes) {
      entries = entries.filter((e) => options.initiatorTypes.includes(e.initiatorType))
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
 * @param {Object} request - Request info { url, method, startTime }
 * @returns {string} Request ID
 */
export function trackPendingRequest(request) {
  const id = `req_${++requestIdCounter}`
  pendingRequests.set(id, {
    ...request,
    id,
  })
  return id
}

/**
 * Complete a pending request
 * @param {string} requestId - The request ID to complete
 */
export function completePendingRequest(requestId) {
  pendingRequests.delete(requestId)
}

/**
 * Get all pending requests
 * @returns {Array} Array of pending requests
 */
export function getPendingRequests() {
  return Array.from(pendingRequests.values())
}

/**
 * Clear all pending requests
 */
export function clearPendingRequests() {
  pendingRequests.clear()
}

/**
 * Get network waterfall snapshot for an error
 * @param {Object} errorEntry - The error entry
 * @returns {Promise<Object|null>} The waterfall snapshot
 */
export async function getNetworkWaterfallForError(errorEntry) {
  if (!networkWaterfallEnabled) return null

  const now = typeof performance !== 'undefined' && performance?.now ? performance.now() : 0
  const since = Math.max(0, now - WATERFALL_TIME_WINDOW_MS)

  const entries = getNetworkWaterfall({ since })
  const pending = getPendingRequests()

  return {
    type: 'network_waterfall',
    ts: new Date().toISOString(),
    _enrichments: ['networkWaterfall'],
    _errorTs: errorEntry.ts,
    entries,
    pending,
  }
}

/**
 * Set whether network waterfall is enabled
 * @param {boolean} enabled - Whether to enable network waterfall
 */
export function setNetworkWaterfallEnabled(enabled) {
  networkWaterfallEnabled = enabled
}

/**
 * Check if network waterfall is enabled
 * @returns {boolean} Whether network waterfall is enabled
 */
export function isNetworkWaterfallEnabled() {
  return networkWaterfallEnabled
}

// =============================================================================
// NETWORK BODY CAPTURE
// =============================================================================

/**
 * Set whether network body capture is enabled
 * @param {boolean} enabled - Whether to enable body capture
 */
export function setNetworkBodyCaptureEnabled(enabled) {
  networkBodyCaptureEnabled = enabled
}

/**
 * Check if network body capture is enabled
 * @returns {boolean} Whether body capture is enabled
 */
export function isNetworkBodyCaptureEnabled() {
  return networkBodyCaptureEnabled
}

/**
 * Check if a URL should be captured (not gasoline server or extension)
 * @param {string} url - The URL to check
 * @returns {boolean} True if the URL should be captured
 */
export function shouldCaptureUrl(url) {
  if (!url) return true
  if (url.includes('localhost:7890') || url.includes('127.0.0.1:7890')) return false
  if (url.startsWith('chrome-extension://')) return false
  return true
}

/**
 * Sanitize headers by removing sensitive ones
 * @param {Object|Map|Headers|null} headers - Headers to sanitize
 * @returns {Object} Sanitized headers object
 */
export function sanitizeHeaders(headers) {
  if (!headers) return {}

  const result = {}

  if (typeof headers.forEach === 'function') {
    // Headers object or Map
    headers.forEach((value, key) => {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    })
  } else if (typeof headers.entries === 'function') {
    for (const [key, value] of headers.entries()) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    }
  } else if (typeof headers === 'object') {
    for (const [key, value] of Object.entries(headers)) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value
      }
    }
  }

  return result
}

/**
 * Truncate request body at 8KB limit
 * @param {string|null} body - The request body
 * @returns {{ body: string|null, truncated: boolean }}
 */
export function truncateRequestBody(body) {
  if (body === null || body === undefined) return { body: null, truncated: false }
  if (body.length <= REQUEST_BODY_MAX) return { body, truncated: false }
  return { body: body.slice(0, REQUEST_BODY_MAX), truncated: true }
}

/**
 * Truncate response body at 16KB limit
 * @param {string|null} body - The response body
 * @returns {{ body: string|null, truncated: boolean }}
 */
export function truncateResponseBody(body) {
  if (body === null || body === undefined) return { body: null, truncated: false }
  if (body.length <= RESPONSE_BODY_MAX) return { body, truncated: false }
  return { body: body.slice(0, RESPONSE_BODY_MAX), truncated: true }
}

/**
 * Read a response body, returning text for text types and size info for binary
 * @param {Object} response - The cloned response object
 * @returns {Promise<string>} The body content or binary size placeholder
 */
export async function readResponseBody(response) {
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
 * @param {Object} response - The cloned response object
 * @param {number} timeoutMs - Timeout in milliseconds
 * @returns {Promise<string>} The body or timeout message
 */
export async function readResponseBodyWithTimeout(response, timeoutMs = BODY_READ_TIMEOUT_MS) {
  return Promise.race([
    readResponseBody(response),
    new Promise((resolve) => {
      setTimeout(() => resolve('[Skipped: body read timeout]'), timeoutMs)
    }),
  ])
}

/**
 * Wrap a fetch function to capture request/response bodies
 * @param {Function} fetchFn - The original fetch function
 * @returns {Function} Wrapped fetch that captures bodies
 */
export function wrapFetchWithBodies(fetchFn) {
  return async function (input, init) {
    const startTime = Date.now()

    // Extract URL and method
    let url = ''
    let method = 'GET'
    let requestBody = null

    if (typeof input === 'string') {
      url = input
    } else if (input && input.url) {
      url = input.url
      method = input.method || 'GET'
    }

    if (init) {
      method = init.method || method
      requestBody = init.body || null
    }

    // Skip gasoline server requests
    if (!shouldCaptureUrl(url)) {
      return fetchFn(input, init)
    }

    // Call original fetch
    const response = await fetchFn(input, init)
    const duration = Date.now() - startTime

    // Capture body asynchronously (don't block return)
    const contentType = response.headers?.get?.('content-type') || ''
    const cloned = response.clone ? response.clone() : null
    // Capture window reference now so deferred callback posts to correct target
    const win = typeof window !== 'undefined' ? window : null

    Promise.resolve().then(async () => {
      try {
        let responseBody = ''
        if (cloned) {
          if (BINARY_CONTENT_TYPES.test(contentType)) {
            const blob = await cloned.blob()
            responseBody = `[Binary: ${blob.size} bytes, ${contentType}]`
          } else {
            responseBody = await cloned.text()
          }
        }

        const { body: truncResp } = truncateResponseBody(responseBody)
        const { body: truncReq } = truncateRequestBody(typeof requestBody === 'string' ? requestBody : null)

        if (win && networkBodyCaptureEnabled) {
          win.postMessage(
            {
              type: 'GASOLINE_NETWORK_BODY',
              payload: {
                url,
                method,
                status: response.status,
                contentType,
                requestBody: truncReq || (typeof requestBody === 'string' ? requestBody : undefined),
                responseBody: truncResp || responseBody,
                duration,
              },
            },
            '*',
          )
        }
      } catch {
        // Body capture failure should not affect user code
      }
    })

    return response
  }
}
