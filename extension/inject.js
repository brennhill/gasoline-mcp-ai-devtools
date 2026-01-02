/**
 * @fileoverview Injected script for capturing browser events
 * This script runs in the page context to intercept console, fetch, and errors
 */

const MAX_STRING_LENGTH = 10240 // 10KB
const MAX_RESPONSE_LENGTH = 5120 // 5KB
const MAX_DEPTH = 10
const MAX_CONTEXT_SIZE = 50 // Max number of context keys
const MAX_CONTEXT_VALUE_SIZE = 4096 // Max size of serialized context value
const SENSITIVE_HEADERS = ['authorization', 'cookie', 'set-cookie', 'x-auth-token']

// User action replay settings
const MAX_ACTION_BUFFER_SIZE = 20 // Max number of recent actions to keep
const SCROLL_THROTTLE_MS = 250 // Throttle scroll events
const SENSITIVE_INPUT_TYPES = ['password', 'credit-card', 'cc-number', 'cc-exp', 'cc-csc']

// Network Waterfall settings
export const MAX_WATERFALL_ENTRIES = 50 // Max network entries to capture
const WATERFALL_TIME_WINDOW_MS = 30000 // Only capture last 30 seconds

// Performance Marks settings
export const MAX_PERFORMANCE_ENTRIES = 50 // Max performance entries to capture
const PERFORMANCE_TIME_WINDOW_MS = 60000 // Only capture last 60 seconds

// Store original methods
let originalConsole = {}
let originalFetch = null
let originalOnerror = null
let unhandledrejectionHandler = null

// Context annotations storage
const contextAnnotations = new Map()

// User action replay buffer
let actionBuffer = []
let lastScrollTime = 0
let actionCaptureEnabled = true
let clickHandler = null
let inputHandler = null
let scrollHandler = null

// Network Waterfall state
let networkWaterfallEnabled = false
let pendingRequests = new Map() // requestId -> { url, method, startTime }
let requestIdCounter = 0

// Performance Marks state
let performanceMarksEnabled = false
let capturedMarks = []
let capturedMeasures = []
let originalPerformanceMark = null
let originalPerformanceMeasure = null
let performanceObserver = null
let performanceCaptureActive = false

/**
 * Safely serialize a value, handling circular references and special types
 */
export function safeSerialize(value, depth = 0, seen = new WeakSet()) {
  // Handle null/undefined
  if (value === null) return null
  if (value === undefined) return undefined

  // Handle primitives
  const type = typeof value
  if (type === 'string') {
    if (value.length > MAX_STRING_LENGTH) {
      return value.slice(0, MAX_STRING_LENGTH) + '... [truncated]'
    }
    return value
  }
  if (type === 'number' || type === 'boolean') {
    return value
  }

  // Handle functions
  if (type === 'function') {
    return `[Function: ${value.name || 'anonymous'}]`
  }

  // Handle Error objects specially
  if (value instanceof Error) {
    return {
      name: value.name,
      message: value.message,
      stack: value.stack,
    }
  }

  // Depth limit
  if (depth >= MAX_DEPTH) {
    return '[Max depth exceeded]'
  }

  // Handle objects
  if (type === 'object') {
    // Circular reference check
    if (seen.has(value)) {
      return '[Circular]'
    }
    seen.add(value)

    // Handle DOM elements
    if (value.nodeType) {
      const tag = value.tagName ? value.tagName.toLowerCase() : 'node'
      const id = value.id ? `#${value.id}` : ''
      const className = value.className ? `.${value.className.split(' ').join('.')}` : ''
      return `[${tag}${id}${className}]`
    }

    // Handle arrays
    if (Array.isArray(value)) {
      return value.map((item) => safeSerialize(item, depth + 1, seen))
    }

    // Handle plain objects
    const result = {}
    for (const key of Object.keys(value)) {
      try {
        result[key] = safeSerialize(value[key], depth + 1, seen)
      } catch {
        result[key] = '[Unserializable]'
      }
    }
    return result
  }

  return String(value)
}

/**
 * Get current context annotations as an object
 */
export function getContextAnnotations() {
  if (contextAnnotations.size === 0) return null

  const result = {}
  for (const [key, value] of contextAnnotations) {
    result[key] = value
  }
  return result
}

/**
 * Set a context annotation
 * @param {string} key - The annotation key (e.g., 'checkout-flow', 'user', 'feature')
 * @param {any} value - The annotation value (will be serialized)
 */
export function setContextAnnotation(key, value) {
  if (typeof key !== 'string' || key.length === 0) {
    console.warn('[Gasoline] annotate() requires a non-empty string key')
    return false
  }

  if (key.length > 100) {
    console.warn('[Gasoline] annotate() key must be 100 characters or less')
    return false
  }

  // Enforce max context keys
  if (!contextAnnotations.has(key) && contextAnnotations.size >= MAX_CONTEXT_SIZE) {
    console.warn(`[Gasoline] Maximum context annotations (${MAX_CONTEXT_SIZE}) reached`)
    return false
  }

  // Serialize and check size
  const serialized = safeSerialize(value)
  const serializedStr = JSON.stringify(serialized)

  if (serializedStr.length > MAX_CONTEXT_VALUE_SIZE) {
    console.warn(`[Gasoline] Context value for "${key}" exceeds max size, truncating`)
    contextAnnotations.set(key, '[Value too large]')
    return false
  }

  contextAnnotations.set(key, serialized)
  return true
}

/**
 * Remove a context annotation
 * @param {string} key - The annotation key to remove
 */
export function removeContextAnnotation(key) {
  return contextAnnotations.delete(key)
}

/**
 * Clear all context annotations
 */
export function clearContextAnnotations() {
  contextAnnotations.clear()
}

/**
 * Get element selector for identification
 * @param {Element} element - The DOM element
 * @returns {string} A selector string for the element
 */
export function getElementSelector(element) {
  if (!element || !element.tagName) return ''

  const tag = element.tagName.toLowerCase()
  const id = element.id ? `#${element.id}` : ''
  const classes = element.className && typeof element.className === 'string'
    ? '.' + element.className.trim().split(/\s+/).slice(0, 2).join('.')
    : ''

  // Add data-testid if present
  const testId = element.getAttribute('data-testid')
  const testIdStr = testId ? `[data-testid="${testId}"]` : ''

  return `${tag}${id}${classes}${testIdStr}`.slice(0, 100)
}

/**
 * Check if an input contains sensitive data
 * @param {Element} element - The input element
 * @returns {boolean} True if the input is sensitive
 */
export function isSensitiveInput(element) {
  if (!element) return false

  const type = (element.type || '').toLowerCase()
  const autocomplete = (element.autocomplete || '').toLowerCase()
  const name = (element.name || '').toLowerCase()

  // Check type attribute
  if (SENSITIVE_INPUT_TYPES.includes(type)) return true

  // Check autocomplete attribute
  if (autocomplete.includes('password') ||
      autocomplete.includes('cc-') ||
      autocomplete.includes('credit-card')) return true

  // Check name attribute for common patterns
  if (name.includes('password') ||
      name.includes('passwd') ||
      name.includes('secret') ||
      name.includes('token') ||
      name.includes('credit') ||
      name.includes('card') ||
      name.includes('cvv') ||
      name.includes('cvc') ||
      name.includes('ssn')) return true

  return false
}

/**
 * Record a user action to the buffer
 * @param {Object} action - The action to record
 */
export function recordAction(action) {
  if (!actionCaptureEnabled) return

  actionBuffer.push({
    ts: new Date().toISOString(),
    ...action,
  })

  // Keep buffer size limited
  if (actionBuffer.length > MAX_ACTION_BUFFER_SIZE) {
    actionBuffer.shift()
  }
}

/**
 * Get the current action buffer
 * @returns {Array} The action buffer
 */
export function getActionBuffer() {
  return [...actionBuffer]
}

/**
 * Clear the action buffer
 */
export function clearActionBuffer() {
  actionBuffer = []
}

/**
 * Handle click events
 * @param {MouseEvent} event - The click event
 */
export function handleClick(event) {
  const target = event.target
  if (!target) return

  const action = {
    type: 'click',
    target: getElementSelector(target),
    x: event.clientX,
    y: event.clientY,
  }

  // Include button text if available (truncated)
  const text = target.textContent || target.innerText || ''
  if (text && text.length > 0) {
    action.text = text.trim().slice(0, 50)
  }

  recordAction(action)
}

/**
 * Handle input events
 * @param {Event} event - The input event
 */
export function handleInput(event) {
  const target = event.target
  if (!target) return

  const action = {
    type: 'input',
    target: getElementSelector(target),
    inputType: target.type || 'text',
  }

  // Only include value for non-sensitive fields
  if (!isSensitiveInput(target)) {
    const value = target.value || ''
    action.value = value.slice(0, 100)
    action.length = value.length
  } else {
    action.value = '[redacted]'
    action.length = (target.value || '').length
  }

  recordAction(action)
}

/**
 * Handle scroll events (throttled)
 * @param {Event} event - The scroll event
 */
export function handleScroll(event) {
  const now = Date.now()
  if (now - lastScrollTime < SCROLL_THROTTLE_MS) return
  lastScrollTime = now

  recordAction({
    type: 'scroll',
    scrollX: Math.round(window.scrollX),
    scrollY: Math.round(window.scrollY),
    target: event.target === document ? 'document' : getElementSelector(event.target),
  })
}

/**
 * Install user action capture
 */
export function installActionCapture() {
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  clickHandler = handleClick
  inputHandler = handleInput
  scrollHandler = handleScroll

  document.addEventListener('click', clickHandler, { capture: true, passive: true })
  document.addEventListener('input', inputHandler, { capture: true, passive: true })
  window.addEventListener('scroll', scrollHandler, { capture: true, passive: true })
}

/**
 * Uninstall user action capture
 */
export function uninstallActionCapture() {
  if (clickHandler) {
    document.removeEventListener('click', clickHandler, { capture: true })
    clickHandler = null
  }
  if (inputHandler) {
    document.removeEventListener('input', inputHandler, { capture: true })
    inputHandler = null
  }
  if (scrollHandler) {
    window.removeEventListener('scroll', scrollHandler, { capture: true })
    scrollHandler = null
  }
  clearActionBuffer()
}

/**
 * Set whether action capture is enabled
 * @param {boolean} enabled - Whether to enable action capture
 */
export function setActionCaptureEnabled(enabled) {
  actionCaptureEnabled = enabled
  if (!enabled) {
    clearActionBuffer()
  }
}

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
// PERFORMANCE MARKS
// =============================================================================

/**
 * Get performance marks
 * @param {Object} options - Options for filtering
 * @returns {Array} Array of mark entries
 */
export function getPerformanceMarks(options = {}) {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let marks = performance.getEntriesByType('mark') || []

    // Filter by time range
    if (options.since) {
      marks = marks.filter((m) => m.startTime >= options.since)
    }

    // Sort by start time
    marks.sort((a, b) => a.startTime - b.startTime)

    // Limit entries
    if (marks.length > MAX_PERFORMANCE_ENTRIES) {
      marks = marks.slice(-MAX_PERFORMANCE_ENTRIES)
    }

    return marks.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      detail: m.detail || null,
    }))
  } catch {
    return []
  }
}

/**
 * Get performance measures
 * @param {Object} options - Options for filtering
 * @returns {Array} Array of measure entries
 */
export function getPerformanceMeasures(options = {}) {
  if (typeof performance === 'undefined' || !performance) return []

  try {
    let measures = performance.getEntriesByType('measure') || []

    // Filter by time range
    if (options.since) {
      measures = measures.filter((m) => m.startTime >= options.since)
    }

    // Sort by start time
    measures.sort((a, b) => a.startTime - b.startTime)

    // Limit entries
    if (measures.length > MAX_PERFORMANCE_ENTRIES) {
      measures = measures.slice(-MAX_PERFORMANCE_ENTRIES)
    }

    return measures.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      duration: m.duration,
      detail: m.detail || null,
    }))
  } catch {
    return []
  }
}

/**
 * Get captured marks from wrapper
 * @returns {Array} Array of captured marks
 */
export function getCapturedMarks() {
  return [...capturedMarks]
}

/**
 * Get captured measures from wrapper
 * @returns {Array} Array of captured measures
 */
export function getCapturedMeasures() {
  return [...capturedMeasures]
}

/**
 * Install performance capture wrapper
 */
export function installPerformanceCapture() {
  if (typeof performance === 'undefined' || !performance) return

  // Clear previous captured data
  capturedMarks = []
  capturedMeasures = []

  // Store originals
  originalPerformanceMark = performance.mark
  originalPerformanceMeasure = performance.measure

  // Wrap performance.mark
  performance.mark = function (name, options) {
    const result = originalPerformanceMark.call(performance, name, options)

    capturedMarks.push({
      name,
      startTime: result?.startTime || performance.now(),
      detail: options?.detail || null,
      capturedAt: new Date().toISOString(),
    })

    // Limit captured marks
    if (capturedMarks.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMarks.shift()
    }

    return result
  }

  // Wrap performance.measure
  performance.measure = function (name, startMark, endMark) {
    const result = originalPerformanceMeasure.call(performance, name, startMark, endMark)

    capturedMeasures.push({
      name,
      startTime: result?.startTime || 0,
      duration: result?.duration || 0,
      capturedAt: new Date().toISOString(),
    })

    // Limit captured measures
    if (capturedMeasures.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMeasures.shift()
    }

    return result
  }

  performanceCaptureActive = true

  // Try to use PerformanceObserver for additional entries
  if (typeof window !== 'undefined' && window.PerformanceObserver) {
    try {
      performanceObserver = new window.PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.entryType === 'mark') {
            // Avoid duplicates from our wrapper
            if (!capturedMarks.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMarks.push({
                name: entry.name,
                startTime: entry.startTime,
                detail: entry.detail || null,
                capturedAt: new Date().toISOString(),
              })
            }
          } else if (entry.entryType === 'measure') {
            if (!capturedMeasures.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMeasures.push({
                name: entry.name,
                startTime: entry.startTime,
                duration: entry.duration,
                capturedAt: new Date().toISOString(),
              })
            }
          }
        }
      })
      performanceObserver.observe({ entryTypes: ['mark', 'measure'] })
    } catch {
      // PerformanceObserver not supported, continue without it
    }
  }
}

/**
 * Uninstall performance capture wrapper
 */
export function uninstallPerformanceCapture() {
  if (typeof performance === 'undefined' || !performance) return

  if (originalPerformanceMark) {
    performance.mark = originalPerformanceMark
    originalPerformanceMark = null
  }

  if (originalPerformanceMeasure) {
    performance.measure = originalPerformanceMeasure
    originalPerformanceMeasure = null
  }

  if (performanceObserver) {
    performanceObserver.disconnect()
    performanceObserver = null
  }

  capturedMarks = []
  capturedMeasures = []
  performanceCaptureActive = false
}

/**
 * Check if performance capture is active
 * @returns {boolean} Whether performance capture is active
 */
export function isPerformanceCaptureActive() {
  return performanceCaptureActive
}

/**
 * Get performance snapshot for an error
 * @param {Object} errorEntry - The error entry
 * @returns {Promise<Object|null>} The performance snapshot
 */
export async function getPerformanceSnapshotForError(errorEntry) {
  if (!performanceMarksEnabled) return null

  const now = typeof performance !== 'undefined' && performance?.now ? performance.now() : 0
  const since = Math.max(0, now - PERFORMANCE_TIME_WINDOW_MS)

  const marks = getPerformanceMarks({ since })
  const measures = getPerformanceMeasures({ since })

  // Include navigation timing if available
  let navigation = null
  if (typeof performance !== 'undefined' && performance) {
    try {
      const navEntries = performance.getEntriesByType('navigation')
      if (navEntries && navEntries.length > 0) {
        const nav = navEntries[0]
        navigation = {
          type: nav.type,
          startTime: nav.startTime,
          domContentLoadedEventEnd: nav.domContentLoadedEventEnd,
          loadEventEnd: nav.loadEventEnd,
        }
      }
    } catch {
      // Navigation timing not available
    }
  }

  return {
    type: 'performance',
    ts: new Date().toISOString(),
    _enrichments: ['performanceMarks'],
    _errorTs: errorEntry.ts,
    marks,
    measures,
    navigation,
  }
}

/**
 * Set whether performance marks are enabled
 * @param {boolean} enabled - Whether to enable performance marks
 */
export function setPerformanceMarksEnabled(enabled) {
  performanceMarksEnabled = enabled
}

/**
 * Check if performance marks are enabled
 * @returns {boolean} Whether performance marks are enabled
 */
export function isPerformanceMarksEnabled() {
  return performanceMarksEnabled
}

/**
 * Post a log message to the content script
 */
function postLog(payload) {
  // Include context annotations and action replay for errors
  const context = getContextAnnotations()
  const actions = payload.level === 'error' ? getActionBuffer() : null

  // Build enrichments list to help AI understand what data is attached
  const enrichments = []
  if (context && payload.level === 'error') enrichments.push('context')
  if (actions && actions.length > 0) enrichments.push('userActions')

  window.postMessage(
    {
      type: 'DEV_CONSOLE_LOG',
      payload: {
        ts: new Date().toISOString(),
        url: window.location.href,
        ...(enrichments.length > 0 ? { _enrichments: enrichments } : {}),
        ...(context && payload.level === 'error' ? { _context: context } : {}),
        ...(actions && actions.length > 0 ? { _actions: actions } : {}),
        ...payload, // Allow payload to override defaults like url
      },
    },
    '*'
  )
}

/**
 * Install console capture hooks
 */
export function installConsoleCapture() {
  const methods = ['log', 'warn', 'error', 'info', 'debug']

  methods.forEach((method) => {
    originalConsole[method] = console[method]

    console[method] = function (...args) {
      // Post to extension
      postLog({
        level: method,
        type: 'console',
        args: args.map((arg) => safeSerialize(arg)),
      })

      // Call original
      originalConsole[method].apply(console, args)
    }
  })
}

/**
 * Uninstall console capture hooks
 */
export function uninstallConsoleCapture() {
  Object.keys(originalConsole).forEach((method) => {
    console[method] = originalConsole[method]
  })
  originalConsole = {}
}

/**
 * Wrap fetch to capture network errors
 */
export function wrapFetch(originalFetchFn) {
  return async function (input, init) {
    const startTime = Date.now()
    const url = typeof input === 'string' ? input : input.url
    const method = init?.method || (typeof input === 'object' ? input.method : 'GET') || 'GET'

    try {
      const response = await originalFetchFn(input, init)
      const duration = Date.now() - startTime

      // Capture errors (4xx, 5xx)
      if (!response.ok) {
        let responseBody = ''
        try {
          const cloned = response.clone()
          responseBody = await cloned.text()
          if (responseBody.length > MAX_RESPONSE_LENGTH) {
            responseBody = responseBody.slice(0, MAX_RESPONSE_LENGTH) + '... [truncated]'
          }
        } catch {
          responseBody = '[Could not read response]'
        }

        // Filter sensitive headers
        const safeHeaders = {}
        if (init?.headers) {
          const headers =
            init.headers instanceof Headers ? Object.fromEntries(init.headers) : init.headers
          Object.keys(headers).forEach((key) => {
            if (!SENSITIVE_HEADERS.includes(key.toLowerCase())) {
              safeHeaders[key] = headers[key]
            }
          })
        }

        postLog({
          level: 'error',
          type: 'network',
          method: method.toUpperCase(),
          url,
          status: response.status,
          statusText: response.statusText,
          duration,
          response: responseBody,
        })
      }

      return response
    } catch (error) {
      const duration = Date.now() - startTime

      postLog({
        level: 'error',
        type: 'network',
        method: method.toUpperCase(),
        url,
        error: error.message,
        duration,
      })

      throw error
    }
  }
}

/**
 * Install fetch capture
 */
export function installFetchCapture() {
  originalFetch = window.fetch
  window.fetch = wrapFetch(originalFetch)
}

/**
 * Uninstall fetch capture
 */
export function uninstallFetchCapture() {
  if (originalFetch) {
    window.fetch = originalFetch
    originalFetch = null
  }
}

/**
 * Install exception capture
 */
export function installExceptionCapture() {
  originalOnerror = window.onerror

  window.onerror = function (message, filename, lineno, colno, error) {
    postLog({
      level: 'error',
      type: 'exception',
      message: String(message),
      filename: filename || '',
      lineno: lineno || 0,
      colno: colno || 0,
      stack: error?.stack || '',
    })

    // Call original if exists
    if (originalOnerror) {
      return originalOnerror(message, filename, lineno, colno, error)
    }
    return false
  }

  // Unhandled promise rejections
  unhandledrejectionHandler = function (event) {
    const error = event.reason
    let message = ''
    let stack = ''

    if (error instanceof Error) {
      message = error.message
      stack = error.stack || ''
    } else if (typeof error === 'string') {
      message = error
    } else {
      message = String(error)
    }

    postLog({
      level: 'error',
      type: 'exception',
      message: `Unhandled Promise Rejection: ${message}`,
      stack,
    })
  }

  window.addEventListener('unhandledrejection', unhandledrejectionHandler)
}

/**
 * Uninstall exception capture
 */
export function uninstallExceptionCapture() {
  if (originalOnerror !== null) {
    window.onerror = originalOnerror
    originalOnerror = null
  }

  if (unhandledrejectionHandler) {
    window.removeEventListener('unhandledrejection', unhandledrejectionHandler)
    unhandledrejectionHandler = null
  }
}

/**
 * Install all capture hooks
 */
export function install() {
  installConsoleCapture()
  installFetchCapture()
  installExceptionCapture()
  installActionCapture()
}

/**
 * Uninstall all capture hooks
 */
export function uninstall() {
  uninstallConsoleCapture()
  uninstallFetchCapture()
  uninstallExceptionCapture()
  uninstallActionCapture()
}

/**
 * Install the window.__gasoline API for developers to interact with Gasoline
 */
export function installGasolineAPI() {
  if (typeof window === 'undefined') return

  window.__gasoline = {
    /**
     * Add a context annotation that will be included with errors
     * @param {string} key - Annotation key (e.g., 'checkout-flow', 'user')
     * @param {any} value - Annotation value
     * @example
     * window.__gasoline.annotate('checkout-flow', { step: 'payment', items: 3 })
     */
    annotate(key, value) {
      return setContextAnnotation(key, value)
    },

    /**
     * Remove a context annotation
     * @param {string} key - Annotation key to remove
     */
    removeAnnotation(key) {
      return removeContextAnnotation(key)
    },

    /**
     * Clear all context annotations
     */
    clearAnnotations() {
      clearContextAnnotations()
    },

    /**
     * Get current context annotations
     * @returns {Object|null} Current annotations or null if none
     */
    getContext() {
      return getContextAnnotations()
    },

    /**
     * Get the user action replay buffer
     * @returns {Array} Recent user actions
     */
    getActions() {
      return getActionBuffer()
    },

    /**
     * Clear the user action replay buffer
     */
    clearActions() {
      clearActionBuffer()
    },

    /**
     * Enable or disable action capture
     * @param {boolean} enabled - Whether to capture user actions
     */
    setActionCapture(enabled) {
      setActionCaptureEnabled(enabled)
    },

    /**
     * Enable or disable network waterfall capture
     * @param {boolean} enabled - Whether to capture network waterfall
     */
    setNetworkWaterfall(enabled) {
      setNetworkWaterfallEnabled(enabled)
    },

    /**
     * Get current network waterfall
     * @param {Object} options - Filter options
     * @returns {Array} Network waterfall entries
     */
    getNetworkWaterfall(options) {
      return getNetworkWaterfall(options)
    },

    /**
     * Enable or disable performance marks capture
     * @param {boolean} enabled - Whether to capture performance marks
     */
    setPerformanceMarks(enabled) {
      setPerformanceMarksEnabled(enabled)
    },

    /**
     * Get performance marks
     * @param {Object} options - Filter options
     * @returns {Array} Performance mark entries
     */
    getMarks(options) {
      return getPerformanceMarks(options)
    },

    /**
     * Get performance measures
     * @param {Object} options - Filter options
     * @returns {Array} Performance measure entries
     */
    getMeasures(options) {
      return getPerformanceMeasures(options)
    },

    /**
     * Version of the Gasoline API
     */
    version: '3.0.0',
  }
}

/**
 * Uninstall the window.__gasoline API
 */
export function uninstallGasolineAPI() {
  if (typeof window !== 'undefined' && window.__gasoline) {
    delete window.__gasoline
  }
}

// =============================================================================
// WEBSOCKET CAPTURE (v4)
// =============================================================================

const WS_MAX_BODY_SIZE = 4096 // 4KB truncation limit
const WS_PREVIEW_LIMIT = 200 // Preview character limit

// WebSocket capture state
let originalWebSocket = null
let webSocketCaptureEnabled = false
let webSocketCaptureMode = 'lifecycle' // 'lifecycle' or 'messages'

/**
 * Get the byte size of a WebSocket message
 * @param {string|ArrayBuffer|Blob|Object} data - The message data
 * @returns {number} Size in bytes
 */
export function getSize(data) {
  if (typeof data === 'string') return data.length
  if (data instanceof ArrayBuffer) return data.byteLength
  if (data && typeof data === 'object' && 'size' in data) return data.size
  return 0
}

/**
 * Format a WebSocket payload for logging
 * @param {string|ArrayBuffer|Blob|Object} data - The message data
 * @returns {string} Formatted payload string
 */
export function formatPayload(data) {
  if (typeof data === 'string') return data

  if (data instanceof ArrayBuffer) {
    const bytes = new Uint8Array(data)
    if (data.byteLength < 256) {
      // Small binary: hex preview
      let hex = ''
      for (let i = 0; i < bytes.length; i++) {
        hex += bytes[i].toString(16).padStart(2, '0')
      }
      return `[Binary: ${data.byteLength}B] ${hex}`
    } else {
      // Large binary: size + magic bytes (first 4 bytes)
      let magic = ''
      for (let i = 0; i < Math.min(4, bytes.length); i++) {
        magic += bytes[i].toString(16).padStart(2, '0')
      }
      return `[Binary: ${data.byteLength}B, magic:${magic}]`
    }
  }

  // Blob or Blob-like
  if (data && typeof data === 'object' && 'size' in data) {
    return `[Binary: ${data.size}B]`
  }

  return String(data)
}

/**
 * Truncate a WebSocket message to the size limit
 * @param {string} message - The message to truncate
 * @returns {{data: string, truncated: boolean}} Truncation result
 */
export function truncateWsMessage(message) {
  if (typeof message === 'string' && message.length > WS_MAX_BODY_SIZE) {
    return { data: message.slice(0, WS_MAX_BODY_SIZE), truncated: true }
  }
  return { data: message, truncated: false }
}

/**
 * Create a connection tracker for adaptive sampling and schema detection
 * @param {string} id - Connection ID
 * @param {string} url - WebSocket URL
 * @returns {Object} Connection tracker instance
 */
export function createConnectionTracker(id, url) {
  const tracker = {
    id,
    url,
    messageCount: 0,
    _sampleCounter: 0,
    _messageRate: 0,
    _messageTimestamps: [],
    _schemaKeys: [],
    _schemaVariants: new Map(),
    _schemaConsistent: true,
    _schemaDetected: false,

    stats: {
      incoming: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
      outgoing: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
    },

    /**
     * Record a message for stats and schema detection
     */
    recordMessage(direction, data) {
      this.messageCount++
      const size = data ? (typeof data === 'string' ? data.length : getSize(data)) : 0
      const now = Date.now()

      this.stats[direction].count++
      this.stats[direction].bytes += size
      this.stats[direction].lastAt = now

      if (data && typeof data === 'string') {
        this.stats[direction].lastPreview = data.length > WS_PREVIEW_LIMIT
          ? data.slice(0, WS_PREVIEW_LIMIT)
          : data
      }

      // Track timestamps for rate calculation
      this._messageTimestamps.push(now)
      // Keep only last 5 seconds
      const cutoff = now - 5000
      this._messageTimestamps = this._messageTimestamps.filter(t => t >= cutoff)

      // Schema detection from first 5 incoming JSON messages
      if (direction === 'incoming' && data && typeof data === 'string' && this._schemaKeys.length < 5) {
        try {
          const parsed = JSON.parse(data)
          if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed).sort()
            const keyStr = keys.join(',')
            this._schemaKeys.push(keyStr)

            // Track variants
            this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1)

            // Check consistency after 2+ messages
            if (this._schemaKeys.length >= 2) {
              const first = this._schemaKeys[0]
              this._schemaConsistent = this._schemaKeys.every(k => k === first)
            }

            if (this._schemaKeys.length >= 5) {
              this._schemaDetected = true
            }
          }
        } catch {
          // Not JSON, no schema
        }
      }

      // Track variants for messages beyond the first 5
      if (direction === 'incoming' && data && typeof data === 'string' && this._schemaDetected) {
        try {
          const parsed = JSON.parse(data)
          if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed).sort()
            const keyStr = keys.join(',')
            this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1)
          }
        } catch {
          // Not JSON
        }
      }
    },

    /**
     * Determine if a message should be sampled (logged)
     */
    shouldSample(direction) {
      this._sampleCounter++

      // Always log first 5 messages on a connection
      if (this.messageCount > 0 && this.messageCount <= 5) return true

      const rate = this._messageRate || this.getMessageRate()

      if (rate < 10) return true
      if (rate < 50) {
        // Target ~10 msg/s
        const n = Math.max(1, Math.round(rate / 10))
        return (this._sampleCounter % n) === 0
      }
      if (rate < 200) {
        // Target ~5 msg/s
        const n = Math.max(1, Math.round(rate / 5))
        return (this._sampleCounter % n) === 0
      }
      // > 200: target ~2 msg/s
      const n = Math.max(1, Math.round(rate / 2))
      return (this._sampleCounter % n) === 0
    },

    /**
     * Lifecycle events should always be logged
     */
    shouldLogLifecycle() {
      return true
    },

    /**
     * Get sampling info
     */
    getSamplingInfo() {
      const rate = this._messageRate || this.getMessageRate()
      let targetRate = rate
      if (rate >= 10 && rate < 50) targetRate = 10
      else if (rate >= 50 && rate < 200) targetRate = 5
      else if (rate >= 200) targetRate = 2

      return {
        rate: `${rate}/s`,
        logged: `${targetRate}/${Math.round(rate)}`,
        window: '5s',
      }
    },

    /**
     * Get the current message rate (messages per second)
     */
    getMessageRate() {
      if (this._messageTimestamps.length < 2) return this._messageTimestamps.length
      const window = (this._messageTimestamps[this._messageTimestamps.length - 1] - this._messageTimestamps[0]) / 1000
      return window > 0 ? this._messageTimestamps.length / window : this._messageTimestamps.length
    },

    /**
     * Set the message rate manually (for testing)
     */
    setMessageRate(rate) {
      this._messageRate = rate
    },

    /**
     * Get the detected schema info
     */
    getSchema() {
      if (this._schemaKeys.length === 0) {
        return { detectedKeys: null, consistent: true }
      }

      // Check if all recorded schemas are non-JSON
      const hasKeys = this._schemaKeys.length > 0
      if (!hasKeys) {
        return { detectedKeys: null, consistent: true }
      }

      // Get union of all detected keys
      const allKeys = new Set()
      for (const keyStr of this._schemaKeys) {
        for (const k of keyStr.split(',')) {
          if (k) allKeys.add(k)
        }
      }

      // Build variants list
      const variants = []
      for (const [keyStr, count] of this._schemaVariants) {
        if (count > 0) variants.push(keyStr)
      }

      return {
        detectedKeys: allKeys.size > 0 ? Array.from(allKeys).sort() : null,
        consistent: this._schemaConsistent,
        variants: variants.length > 1 ? variants : undefined,
      }
    },

    /**
     * Check if a message represents a schema change
     */
    isSchemaChange(data) {
      if (!this._schemaDetected || !data || typeof data !== 'string') return false
      try {
        const parsed = JSON.parse(data)
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return false
        const keys = Object.keys(parsed).sort().join(',')
        // It's a change if none of the first 5 schemas match
        return !this._schemaKeys.includes(keys)
      } catch {
        return false
      }
    },
  }

  return tracker
}

/**
 * Install WebSocket capture by wrapping the WebSocket constructor
 */
export function installWebSocketCapture() {
  if (typeof window === 'undefined') return
  if (originalWebSocket) return // Already installed

  originalWebSocket = window.WebSocket

  const OriginalWS = window.WebSocket

  function GasolineWebSocket(url, protocols) {
    const ws = new OriginalWS(url, protocols)
    const connectionId = crypto.randomUUID()

    ws.addEventListener('open', () => {
      window.postMessage({
        type: 'GASOLINE_WS',
        payload: { event: 'open', id: connectionId, url, ts: new Date().toISOString() },
      }, '*')
    })

    ws.addEventListener('close', (event) => {
      window.postMessage({
        type: 'GASOLINE_WS',
        payload: {
          event: 'close', id: connectionId, url,
          code: event.code, reason: event.reason,
          ts: new Date().toISOString(),
        },
      }, '*')
    })

    ws.addEventListener('error', () => {
      window.postMessage({
        type: 'GASOLINE_WS',
        payload: { event: 'error', id: connectionId, url, ts: new Date().toISOString() },
      }, '*')
    })

    ws.addEventListener('message', (event) => {
      if (webSocketCaptureMode !== 'messages') return

      const data = event.data
      const size = getSize(data)
      const formatted = formatPayload(data)
      const { data: truncatedData, truncated } = truncateWsMessage(formatted)

      window.postMessage({
        type: 'GASOLINE_WS',
        payload: {
          event: 'message', id: connectionId, url,
          direction: 'incoming', data: truncatedData,
          size, truncated: truncated || undefined,
          ts: new Date().toISOString(),
        },
      }, '*')
    })

    // Wrap send() to capture outgoing messages
    const originalSend = ws.send.bind(ws)
    ws.send = function (data) {
      if (webSocketCaptureMode === 'messages') {
        const size = getSize(data)
        const formatted = formatPayload(data)
        const { data: truncatedData, truncated } = truncateWsMessage(formatted)

        window.postMessage({
          type: 'GASOLINE_WS',
          payload: {
            event: 'message', id: connectionId, url,
            direction: 'outgoing', data: truncatedData,
            size, truncated: truncated || undefined,
            ts: new Date().toISOString(),
          },
        }, '*')
      }

      return originalSend(data)
    }

    return ws
  }

  GasolineWebSocket.prototype = OriginalWS.prototype

  window.WebSocket = GasolineWebSocket
}

/**
 * Set the WebSocket capture mode
 * @param {string} mode - 'lifecycle' or 'messages'
 */
export function setWebSocketCaptureMode(mode) {
  webSocketCaptureMode = mode
}

/**
 * Get the current WebSocket capture mode
 * @returns {string} 'lifecycle' or 'messages'
 */
export function getWebSocketCaptureMode() {
  return webSocketCaptureMode
}

/**
 * Uninstall WebSocket capture, restoring the original constructor
 */
export function uninstallWebSocketCapture() {
  if (typeof window === 'undefined') return
  if (originalWebSocket) {
    window.WebSocket = originalWebSocket
    originalWebSocket = null
  }
}

// Listen for settings changes from content script
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    // Only accept messages from this window
    if (event.source !== window) return

    // Handle settings messages from content script
    if (event.data?.type === 'DEV_CONSOLE_SETTING') {
      switch (event.data.setting) {
        case 'setNetworkWaterfallEnabled':
          setNetworkWaterfallEnabled(event.data.enabled)
          break
        case 'setPerformanceMarksEnabled':
          setPerformanceMarksEnabled(event.data.enabled)
          if (event.data.enabled) {
            installPerformanceCapture()
          } else {
            uninstallPerformanceCapture()
          }
          break
        case 'setActionReplayEnabled':
          setActionCaptureEnabled(event.data.enabled)
          break
        case 'setWebSocketCaptureEnabled':
          webSocketCaptureEnabled = event.data.enabled
          if (event.data.enabled) {
            installWebSocketCapture()
          } else {
            uninstallWebSocketCapture()
          }
          break
        case 'setWebSocketCaptureMode':
          webSocketCaptureMode = event.data.mode || 'lifecycle'
          break
      }
    }
  })
}

// Auto-install when loaded in browser
if (typeof window !== 'undefined' && typeof document !== 'undefined') {
  install()
  installGasolineAPI()
}
