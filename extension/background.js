// @ts-nocheck
/**
 * @fileoverview background.js — Service worker managing server communication and state.
 * Receives captured events from content scripts, batches them with debouncing,
 * and posts to the Go server. Handles error deduplication/grouping, connection
 * status, badge updates, screenshot capture, source map resolution, and
 * on-demand query polling (DOM, a11y, performance snapshots).
 * Design: Debounced batching (100ms default, max 50 per batch) prevents flooding.
 * Circuit breaker with exponential backoff protects against server unavailability.
 * Rate-limited screenshots (5s cooldown, 10/session max). LRU source map cache (50 entries).
 */

const DEFAULT_SERVER_URL = 'http://localhost:7890'
let serverUrl = DEFAULT_SERVER_URL
const RECONNECT_INTERVAL = 5000 // 5 seconds
const DEFAULT_DEBOUNCE_MS = 100
const DEFAULT_MAX_BATCH_SIZE = 50

// Error grouping settings
const ERROR_DEDUP_WINDOW_MS = 5000 // 5 seconds - suppress duplicates within this window
const ERROR_GROUP_FLUSH_MS = 10000 // 10 seconds - flush error counts periodically
const MAX_TRACKED_ERRORS = 100 // Max unique errors to track

// Rate limiting settings
const SCREENSHOT_RATE_LIMIT_MS = 5000 // Min 5 seconds between screenshots
const SCREENSHOT_MAX_PER_SESSION = 10 // Max screenshots per page session

// Source map settings
const SOURCE_MAP_CACHE_SIZE = 50 // Max cached source maps
const SOURCE_MAP_FETCH_TIMEOUT = 5000 // 5 seconds timeout for fetching
const STACK_FRAME_REGEX = /^\s*at\s+(?:(.+?)\s+\()?(?:(.+?):(\d+):(\d+)|(.+?):(\d+))\)?$/
const ANONYMOUS_FRAME_REGEX = /^\s*at\s+(.+?):(\d+):(\d+)$/

// State
let connectionStatus = {
  connected: false,
  entries: 0,
  maxEntries: 1000,
  errorCount: 0,
  logFile: '',
}

let currentLogLevel = 'all'
let screenshotOnError = false // Auto-capture screenshot on error (off by default)

// Error grouping state
const errorGroups = new Map() // signature -> { entry, count, firstSeen, lastSeen }

// Rate limiting state
const screenshotTimestamps = new Map() // tabId -> [timestamps]

// Source map state
const sourceMapCache = new Map() // scriptUrl -> { mappings, sources, names, sourceRoot }
let sourceMapEnabled = false // Source map resolution (off by default)

// Memory enforcement constants
export const MEMORY_SOFT_LIMIT = 20 * 1024 * 1024 // 20MB
export const MEMORY_HARD_LIMIT = 50 * 1024 * 1024 // 50MB
export const MEMORY_CHECK_INTERVAL_MS = 30000 // 30 seconds
export const MEMORY_AVG_LOG_ENTRY_SIZE = 500 // ~500 bytes per log entry
export const MEMORY_AVG_WS_EVENT_SIZE = 300 // ~300 bytes per WS event + data length
export const MEMORY_AVG_NETWORK_BODY_SIZE = 1000 // ~1000 bytes per network body + body lengths
export const MEMORY_AVG_ACTION_SIZE = 400 // ~400 bytes per enhanced action

// Memory enforcement state
let memoryPressureLevel = 'normal' // 'normal' | 'soft' | 'hard'
let lastMemoryCheck = 0
let networkBodyCaptureDisabled = false
let reducedCapacities = false

// AI capture control state
let _captureOverrides = {} // Current AI-set overrides from /settings
let aiControlled = false // Whether AI has active overrides

// Context annotation monitoring state
const CONTEXT_SIZE_THRESHOLD = 20 * 1024 // 20KB threshold
const CONTEXT_WARNING_WINDOW_MS = 60000 // 60-second window
const CONTEXT_WARNING_COUNT = 3 // Entries needed to trigger warning
let contextExcessiveTimestamps = [] // Timestamps of excessive entries
let contextWarningState = null // { sizeKB, count, triggeredAt }

/**
 * Measure the serialized byte size of _context in a log entry
 * @param {Object} entry - The log entry
 * @returns {number} Approximate byte size of _context
 */
export function measureContextSize(entry) {
  if (!entry._context || typeof entry._context !== 'object') return 0
  const keys = Object.keys(entry._context)
  if (keys.length === 0) return 0
  return JSON.stringify(entry._context).length
}

/**
 * Check a batch of entries for excessive context annotation usage.
 * Triggers a warning if 3+ entries exceed 20KB within 60 seconds.
 * @param {Array} entries - Batch of log entries to check
 */
export function checkContextAnnotations(entries) {
  const now = Date.now()

  for (const entry of entries) {
    const size = measureContextSize(entry)
    if (size > CONTEXT_SIZE_THRESHOLD) {
      contextExcessiveTimestamps.push({ ts: now, size })
    }
  }

  // Prune timestamps outside the 60s window
  contextExcessiveTimestamps = contextExcessiveTimestamps.filter((t) => now - t.ts < CONTEXT_WARNING_WINDOW_MS)

  // Check if we've hit the threshold
  if (contextExcessiveTimestamps.length >= CONTEXT_WARNING_COUNT) {
    const avgSize = contextExcessiveTimestamps.reduce((sum, t) => sum + t.size, 0) / contextExcessiveTimestamps.length
    contextWarningState = {
      sizeKB: Math.round(avgSize / 1024),
      count: contextExcessiveTimestamps.length,
      triggeredAt: now,
    }
  } else if (contextWarningState && contextExcessiveTimestamps.length === 0) {
    // Clear warning if no excessive entries in the window
    contextWarningState = null
  }
}

/**
 * Get the current context annotation warning state
 * @returns {Object|null} Warning info or null if no warning
 */
export function getContextWarning() {
  return contextWarningState
}

/**
 * Reset the context annotation warning (for testing)
 */
export function resetContextWarning() {
  contextExcessiveTimestamps = []
  contextWarningState = null
}

// =============================================================================
// DEBUG LOGGING
// =============================================================================

const DEBUG_LOG_MAX_ENTRIES = 200 // Max debug log entries to keep
let debugMode = false // Debug logging (off by default)
const debugLogBuffer = [] // Circular buffer of debug entries

/**
 * Log categories for debug output
 */
export const DebugCategory = {
  CONNECTION: 'connection',
  CAPTURE: 'capture',
  ERROR: 'error',
  LIFECYCLE: 'lifecycle',
  SETTINGS: 'settings',
}

/**
 * Log a debug message (only when debug mode is enabled)
 * @param {string} category - Log category (connection, capture, error, lifecycle, settings)
 * @param {string} message - Debug message
 * @param {Object} data - Optional additional data
 */
export function debugLog(category, message, data = null) {
  const entry = {
    ts: new Date().toISOString(),
    category,
    message,
    ...(data ? { data } : {}),
  }

  // Always add to buffer (for export even if debug mode was off)
  debugLogBuffer.push(entry)
  if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
    debugLogBuffer.shift()
  }

  // Only log to console if debug mode is on
  if (debugMode) {
    const prefix = `[Gasoline:${category}]`
    if (data) {
      console.log(prefix, message, data)
    } else {
      console.log(prefix, message)
    }
  }
}

/**
 * Get all debug log entries
 * @returns {Array} Debug log entries
 */
export function getDebugLog() {
  return [...debugLogBuffer]
}

/**
 * Clear debug log buffer
 */
export function clearDebugLog() {
  debugLogBuffer.length = 0
}

/**
 * Export debug log as JSON string
 * @returns {string} JSON formatted debug log with metadata
 */
export function exportDebugLog() {
  return JSON.stringify(
    {
      exportedAt: new Date().toISOString(),
      version: chrome.runtime.getManifest().version,
      debugMode,
      connectionStatus,
      settings: {
        logLevel: currentLogLevel,
        screenshotOnError,
        sourceMapEnabled,
      },
      entries: debugLogBuffer,
    },
    null,
    2,
  )
}

/**
 * Set debug mode enabled/disabled
 * @param {boolean} enabled - Whether to enable debug mode
 */
export function setDebugMode(enabled) {
  debugMode = enabled
  debugLog(DebugCategory.SETTINGS, `Debug mode ${enabled ? 'enabled' : 'disabled'}`)
}

/**
 * Create a log batcher that debounces and batches log entries
 */
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 *
 * States:
 * - closed: Normal operation, requests pass through
 * - open: Circuit tripped, requests rejected immediately
 * - half-open: Testing with a single probe request
 */
export function createCircuitBreaker(sendFn, options = {}) {
  const maxFailures = options.maxFailures ?? 5
  const resetTimeout = options.resetTimeout ?? 30000
  const initialBackoff = options.initialBackoff ?? 1000
  const maxBackoff = options.maxBackoff ?? 30000

  let state = 'closed'
  let consecutiveFailures = 0
  let totalFailures = 0
  let totalSuccesses = 0
  let currentBackoff = 0
  let lastFailureTime = 0
  let probeInFlight = false

  function getState() {
    if (state === 'open' && Date.now() - lastFailureTime >= resetTimeout) {
      state = 'half-open'
    }
    return state
  }

  function getStats() {
    return {
      state: getState(),
      consecutiveFailures,
      totalFailures,
      totalSuccesses,
      currentBackoff,
    }
  }

  function reset() {
    state = 'closed'
    consecutiveFailures = 0
    currentBackoff = 0
    probeInFlight = false
  }

  function onSuccess() {
    consecutiveFailures = 0
    currentBackoff = 0
    totalSuccesses++
    state = 'closed'
    probeInFlight = false
  }

  function onFailure() {
    consecutiveFailures++
    totalFailures++
    lastFailureTime = Date.now()
    probeInFlight = false

    if (consecutiveFailures >= maxFailures) {
      state = 'open'
    }

    // Calculate next backoff (exponential: initialBackoff * 2^(failures-1))
    if (consecutiveFailures > 1) {
      currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff)
    } else {
      currentBackoff = 0
    }
  }

  async function execute(args) {
    const currentState = getState()

    if (currentState === 'open') {
      throw new Error('Circuit breaker is open')
    }

    if (currentState === 'half-open') {
      if (probeInFlight) {
        throw new Error('Circuit breaker is open')
      }
      probeInFlight = true
    }

    // Apply backoff delay
    if (currentBackoff > 0) {
      await new Promise((r) => {
        setTimeout(r, currentBackoff)
      })
    }

    try {
      const result = await sendFn(args)
      onSuccess()
      return result
    } catch (err) {
      onFailure()
      throw err
    }
  }

  return { execute, getState, getStats, reset }
}

/**
 * Rate limit configuration matching the spec:
 * - Backoff schedule: 100ms, 500ms, 2000ms
 * - Circuit opens after 5 consecutive failures
 * - 30-second pause when circuit opens
 * - Retry budget of 3 per batch
 */
export const RATE_LIMIT_CONFIG = {
  maxFailures: 5,
  resetTimeout: 30000,
  backoffSchedule: [100, 500, 2000],
  retryBudget: 3,
}

/**
 * Creates a batcher wired with circuit breaker logic for rate limiting.
 *
 * The circuit breaker manages state transitions (closed/open/half-open)
 * while the wrapper applies schedule-based backoff delays matching the spec.
 *
 * When the circuit is open, data continues to buffer locally (add() still works)
 * but no POSTs are sent until the circuit transitions to half-open for a probe.
 *
 * @param {Function} sendFn - The async function to send a batch of entries
 * @param {Object} options - Configuration options
 * @param {number} options.debounceMs - Debounce interval (default 100)
 * @param {number} options.maxBatchSize - Max batch size (default 50)
 * @param {number} options.retryBudget - Max retries per batch (default 3)
 * @param {number} options.maxFailures - Failures to open circuit (default 5)
 * @param {number} options.resetTimeout - Time before half-open probe (default 30000)
 * @param {Object} options.sharedCircuitBreaker - Optional shared circuit breaker instance
 * @returns {{ batcher, circuitBreaker, getConnectionStatus }}
 */
export function createBatcherWithCircuitBreaker(sendFn, options = {}) {
  const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
  const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE
  const retryBudget = options.retryBudget ?? RATE_LIMIT_CONFIG.retryBudget
  const maxFailures = options.maxFailures ?? RATE_LIMIT_CONFIG.maxFailures
  const resetTimeout = options.resetTimeout ?? RATE_LIMIT_CONFIG.resetTimeout
  const backoffSchedule = RATE_LIMIT_CONFIG.backoffSchedule

  // Track connection status locally
  const localConnectionStatus = { connected: true }

  const isSharedCB = !!options.sharedCircuitBreaker

  // Create a circuit breaker that wraps the sendFn.
  // initialBackoff and maxBackoff are set to 0 because we apply our own
  // schedule-based backoff externally (the CB still tracks state/failures).
  const cb =
    options.sharedCircuitBreaker ||
    createCircuitBreaker(sendFn, { maxFailures, resetTimeout, initialBackoff: 0, maxBackoff: 0 })

  // Schedule-based backoff: returns delay based on consecutive failures
  function getScheduledBackoff(failures) {
    if (failures <= 0) return 0
    const idx = Math.min(failures - 1, backoffSchedule.length - 1)
    return backoffSchedule[idx]
  }

  // Wrapped circuit breaker facade that exposes schedule-based backoff
  const wrappedCircuitBreaker = {
    getState: () => cb.getState(),
    getStats: () => {
      const stats = cb.getStats()
      return {
        ...stats,
        currentBackoff: getScheduledBackoff(stats.consecutiveFailures),
      }
    },
    reset: () => cb.reset(),
  }

  // Attempt to send entries through the circuit breaker.
  // For a dedicated (non-shared) CB, we use cb.execute() directly since
  // it wraps our sendFn. For a shared CB, we call sendFn directly and
  // record the outcome on the shared CB separately.
  async function attemptSend(entries) {
    if (!isSharedCB) {
      // Dedicated CB wraps sendFn - execute handles everything
      return await cb.execute(entries)
    }

    // Shared CB: check state first, then call sendFn directly
    const state = cb.getState()
    if (state === 'open') {
      throw new Error('Circuit breaker is open')
    }

    try {
      const result = await sendFn(entries)
      // Record success on shared CB
      cb.reset()
      return result
    } catch (err) {
      // Trigger shared circuit breaker's failure counter
      try {
        await cb.execute(entries)
      } catch {
        /* expected */
      }
      throw err
    }
  }

  let pending = []
  let timeoutId = null

  async function flushWithCircuitBreaker() {
    if (pending.length === 0) return

    const entries = pending
    pending = []

    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }

    const currentState = cb.getState()

    // If circuit is open, put entries back into pending buffer
    if (currentState === 'open') {
      pending = entries.concat(pending)
      return
    }

    // Each flush attempt records one success/failure on the circuit breaker.
    // The retry budget controls how many times we attempt this batch.
    try {
      await attemptSend(entries)
      localConnectionStatus.connected = true
    } catch {
      localConnectionStatus.connected = false

      // If circuit opened, buffer the entries for later draining
      if (cb.getState() === 'open') {
        pending = entries.concat(pending)
        return
      }

      // Retry with budget: attempt remaining retries for this batch
      let retriesLeft = retryBudget - 1 // Already used one attempt above
      while (retriesLeft > 0) {
        retriesLeft--

        // Apply schedule-based backoff delay before retry
        const stats = cb.getStats()
        const backoff = getScheduledBackoff(stats.consecutiveFailures)
        if (backoff > 0) {
          await new Promise((r) => {
            setTimeout(r, backoff)
          })
        }

        try {
          await attemptSend(entries)
          localConnectionStatus.connected = true
          return
        } catch {
          localConnectionStatus.connected = false

          // If circuit opened during retry, buffer entries
          if (cb.getState() === 'open') {
            pending = entries.concat(pending)
            return
          }
        }
      }

      // Retry budget exhausted - abandon batch (don't put back in pending)
    }
  }

  const scheduleFlush = () => {
    if (timeoutId) return
    timeoutId = setTimeout(() => {
      timeoutId = null
      flushWithCircuitBreaker()
    }, debounceMs)
  }

  const batcher = {
    add(entry) {
      pending.push(entry)
      if (pending.length >= maxBatchSize) {
        flushWithCircuitBreaker()
      } else {
        scheduleFlush()
      }
    },

    async flush() {
      await flushWithCircuitBreaker()
    },

    clear() {
      pending = []
      if (timeoutId) {
        clearTimeout(timeoutId)
        timeoutId = null
      }
    },

    getPending() {
      return [...pending]
    },
  }

  return {
    batcher,
    circuitBreaker: wrappedCircuitBreaker,
    getConnectionStatus: () => ({ ...localConnectionStatus }),
  }
}

export function createLogBatcher(flushFn, options = {}) {
  const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
  const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE
  const memoryPressureGetter = options.memoryPressureGetter ?? null

  let pending = []
  let timeoutId = null

  const getEffectiveMaxBatchSize = () => {
    if (memoryPressureGetter) {
      const state = memoryPressureGetter()
      if (state.reducedCapacities) {
        return Math.floor(maxBatchSize / 2)
      }
    }
    return maxBatchSize
  }

  const flush = () => {
    if (pending.length === 0) return

    const entries = pending
    pending = []

    if (timeoutId) {
      clearTimeout(timeoutId)
      timeoutId = null
    }

    flushFn(entries)
  }

  const scheduleFlush = () => {
    if (timeoutId) return

    timeoutId = setTimeout(() => {
      timeoutId = null
      flush()
    }, debounceMs)
  }

  return {
    add(entry) {
      pending.push(entry)

      const effectiveMax = getEffectiveMaxBatchSize()
      if (pending.length >= effectiveMax) {
        flush()
      } else {
        scheduleFlush()
      }
    },

    flush() {
      flush()
    },

    clear() {
      pending = []
      if (timeoutId) {
        clearTimeout(timeoutId)
        timeoutId = null
      }
    },
  }
}

/**
 * Send log entries to the server
 */
export async function sendLogsToServer(entries) {
  debugLog(DebugCategory.CONNECTION, `Sending ${entries.length} entries to server`)

  const response = await fetch(`${serverUrl}/logs`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ entries }),
  })

  if (!response.ok) {
    const error = `Server error: ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  const result = await response.json()
  debugLog(DebugCategory.CONNECTION, `Server accepted entries, total: ${result.entries}`)
  return result
}

/**
 * Send WebSocket events to the server
 */
export async function sendWSEventsToServer(events) {
  debugLog(DebugCategory.CONNECTION, `Sending ${events.length} WS events to server`)

  const response = await fetch(`${serverUrl}/websocket-events`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ events }),
  })

  if (!response.ok) {
    const error = `Server error (WS): ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  debugLog(DebugCategory.CONNECTION, `Server accepted ${events.length} WS events`)
}

/**
 * Send network bodies to the server
 */
export async function sendNetworkBodiesToServer(bodies) {
  debugLog(DebugCategory.CONNECTION, `Sending ${bodies.length} network bodies to server`)

  const response = await fetch(`${serverUrl}/network-bodies`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ bodies }),
  })

  if (!response.ok) {
    const error = `Server error (network bodies): ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  debugLog(DebugCategory.CONNECTION, `Server accepted ${bodies.length} network bodies`)
}

/**
 * Send enhanced actions to server
 * @param {Array} actions - Array of enhanced action objects
 */
export async function sendEnhancedActionsToServer(actions) {
  debugLog(DebugCategory.CONNECTION, `Sending ${actions.length} enhanced actions to server`)

  const response = await fetch(`${serverUrl}/enhanced-actions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ actions }),
  })

  if (!response.ok) {
    const error = `Server error (enhanced actions): ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  debugLog(DebugCategory.CONNECTION, `Server accepted ${actions.length} enhanced actions`)
}

/**
 * Send performance snapshot to server
 * @param {Object} snapshot - Performance snapshot object
 */
export async function sendPerformanceSnapshotToServer(snapshot) {
  try {
    const response = await fetch(`${serverUrl}/performance-snapshot`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(snapshot),
    })
    if (!response.ok) {
      debugLog(DebugCategory.ERROR, `Server error (performance snapshot): ${response.status}`)
    }
  } catch (err) {
    debugLog(DebugCategory.ERROR, `Failed to send performance snapshot: ${err.message}`)
  }
}

/**
 * Check server health
 */
export async function checkServerHealth() {
  try {
    const response = await fetch(`${serverUrl}/health`)

    if (!response.ok) {
      return { connected: false, error: `HTTP ${response.status}` }
    }

    let data
    try {
      data = await response.json()
    } catch {
      // Server returned non-JSON response (possibly wrong endpoint or proxy)
      return {
        connected: false,
        error: 'Server returned invalid response - check Server URL in options',
      }
    }
    return {
      ...data,
      connected: true,
    }
  } catch (error) {
    return {
      connected: false,
      error: error.message,
    }
  }
}

/**
 * Update extension badge
 */
export function updateBadge(status) {
  if (typeof chrome === 'undefined' || !chrome.action) return

  if (status.connected) {
    const errorCount = status.errorCount || 0

    chrome.action.setBadgeText({
      text: errorCount === 0 ? '' : errorCount > 99 ? '99+' : String(errorCount),
    })

    chrome.action.setBadgeBackgroundColor({
      color: '#3fb950', // green
    })
  } else {
    chrome.action.setBadgeText({ text: '!' })
    chrome.action.setBadgeBackgroundColor({
      color: '#f85149', // red
    })
  }
}

/**
 * Format a log entry with timestamp and truncation
 */
export function formatLogEntry(entry) {
  const formatted = { ...entry }

  // Add timestamp if not present
  if (!formatted.ts) {
    formatted.ts = new Date().toISOString()
  }

  // Truncate large args
  if (formatted.args && Array.isArray(formatted.args)) {
    formatted.args = formatted.args.map((arg) => truncateArg(arg))
  }

  return formatted
}

/**
 * Truncate a single argument if too large
 */
function truncateArg(arg, maxSize = 10240) {
  if (arg === null || arg === undefined) return arg

  // Handle circular references
  try {
    const serialized = JSON.stringify(arg)
    if (serialized.length > maxSize) {
      if (typeof arg === 'string') {
        return arg.slice(0, maxSize) + '... [truncated]'
      }
      return serialized.slice(0, maxSize) + '...[truncated]'
    }
    return arg
  } catch {
    // Circular reference or other serialization issue
    if (typeof arg === 'object') {
      return '[Circular or unserializable object]'
    }
    return String(arg)
  }
}

/**
 * Determine if a log should be captured based on level filter
 */
export function shouldCaptureLog(logLevel, filterLevel, logType) {
  // Always capture network errors and exceptions
  if (logType === 'network' || logType === 'exception') {
    return true
  }

  const levels = ['debug', 'log', 'info', 'warn', 'error']
  const logIndex = levels.indexOf(logLevel)
  const filterIndex = levels.indexOf(filterLevel === 'all' ? 'debug' : filterLevel)

  return logIndex >= filterIndex
}

/**
 * Create a signature for an error to identify duplicates
 */
export function createErrorSignature(entry) {
  const parts = []

  // Type (console, network, exception)
  parts.push(entry.type || 'unknown')

  // Level
  parts.push(entry.level || 'error')

  if (entry.type === 'exception') {
    // For exceptions, use message and first stack frame
    parts.push(entry.message || '')
    if (entry.stack) {
      const firstFrame = entry.stack.split('\n')[1] || ''
      parts.push(firstFrame.trim())
    }
  } else if (entry.type === 'network') {
    // For network errors, use method, URL path (without query), and status
    parts.push(entry.method || 'GET')
    try {
      const url = new URL(entry.url, window?.location?.origin || 'http://localhost')
      parts.push(url.pathname)
    } catch {
      parts.push(entry.url || '')
    }
    parts.push(String(entry.status || 0))
  } else if (entry.type === 'console') {
    // For console logs, use first arg stringified
    if (entry.args && entry.args.length > 0) {
      const firstArg = entry.args[0]
      parts.push(typeof firstArg === 'string' ? firstArg.slice(0, 200) : JSON.stringify(firstArg).slice(0, 200))
    }
  }

  return parts.join('|')
}

/**
 * Process an error through the grouping system
 * Returns true if the error should be sent (first occurrence or flush)
 * Returns false if it's a duplicate that was counted
 */
export function processErrorGroup(entry) {
  // Only group errors, not info/debug logs
  if (entry.level !== 'error' && entry.level !== 'warn') {
    return { shouldSend: true, entry }
  }

  const signature = createErrorSignature(entry)
  const now = Date.now()

  if (errorGroups.has(signature)) {
    const group = errorGroups.get(signature)

    // Check if within dedup window
    if (now - group.lastSeen < ERROR_DEDUP_WINDOW_MS) {
      // Duplicate - just increment count
      group.count++
      group.lastSeen = now
      return { shouldSend: false }
    }

    // Outside dedup window - send as new occurrence with previous count
    const countToReport = group.count
    group.count = 1
    group.lastSeen = now
    group.firstSeen = now

    if (countToReport > 1) {
      // Add count to entry
      return {
        shouldSend: true,
        entry: { ...entry, _previousOccurrences: countToReport - 1 },
      }
    }
    return { shouldSend: true, entry }
  }

  // New error - track it
  if (errorGroups.size >= MAX_TRACKED_ERRORS) {
    // Evict oldest error group
    let oldestSig = null
    let oldestTime = Infinity
    for (const [sig, group] of errorGroups) {
      if (group.lastSeen < oldestTime) {
        oldestTime = group.lastSeen
        oldestSig = sig
      }
    }
    if (oldestSig) {
      errorGroups.delete(oldestSig)
    }
  }

  errorGroups.set(signature, {
    entry,
    count: 1,
    firstSeen: now,
    lastSeen: now,
  })

  return { shouldSend: true, entry }
}

/**
 * Flush error groups - send any accumulated counts
 */
export function flushErrorGroups() {
  const now = Date.now()
  const entriesToSend = []

  for (const [signature, group] of errorGroups) {
    // If there are unreported duplicates
    if (group.count > 1) {
      entriesToSend.push({
        ...group.entry,
        ts: new Date().toISOString(),
        _aggregatedCount: group.count,
        _firstSeen: new Date(group.firstSeen).toISOString(),
        _lastSeen: new Date(group.lastSeen).toISOString(),
      })
      group.count = 0
    }

    // Clean up old entries
    if (now - group.lastSeen > ERROR_GROUP_FLUSH_MS * 2) {
      errorGroups.delete(signature)
    }
  }

  return entriesToSend
}

/**
 * Check if a screenshot is allowed based on rate limiting
 */
export function canTakeScreenshot(tabId) {
  const now = Date.now()

  if (!screenshotTimestamps.has(tabId)) {
    screenshotTimestamps.set(tabId, [])
  }

  const timestamps = screenshotTimestamps.get(tabId)

  // Clean old timestamps
  const recentTimestamps = timestamps.filter((t) => now - t < 60000) // Keep last minute

  // Check session limit
  if (recentTimestamps.length >= SCREENSHOT_MAX_PER_SESSION) {
    return { allowed: false, reason: 'session_limit', nextAllowedIn: null }
  }

  // Check rate limit
  const lastTimestamp = recentTimestamps[recentTimestamps.length - 1]
  if (lastTimestamp && now - lastTimestamp < SCREENSHOT_RATE_LIMIT_MS) {
    return {
      allowed: false,
      reason: 'rate_limit',
      nextAllowedIn: SCREENSHOT_RATE_LIMIT_MS - (now - lastTimestamp),
    }
  }

  return { allowed: true }
}

/**
 * Record a screenshot timestamp
 */
export function recordScreenshot(tabId) {
  if (!screenshotTimestamps.has(tabId)) {
    screenshotTimestamps.set(tabId, [])
  }
  screenshotTimestamps.get(tabId).push(Date.now())
}

/**
 * Capture a screenshot of the visible tab area
 * @param {number} tabId - The tab ID to capture
 * @param {string} relatedErrorId - Optional error ID to link screenshot to
 * @returns {Promise<{success: boolean, dataUrl?: string, error?: string}>}
 */
export async function captureScreenshot(tabId, relatedErrorId, errorType) {
  // Check rate limiting
  const rateCheck = canTakeScreenshot(tabId)
  if (!rateCheck.allowed) {
    debugLog(DebugCategory.CAPTURE, `Screenshot rate limited: ${rateCheck.reason}`, {
      tabId,
      nextAllowedIn: rateCheck.nextAllowedIn,
    })
    return {
      success: false,
      error: `Rate limited: ${rateCheck.reason}`,
      nextAllowedIn: rateCheck.nextAllowedIn,
    }
  }

  try {
    // Get the tab's window ID
    const tab = await chrome.tabs.get(tabId)

    // Capture as JPEG for smaller file size
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: 'jpeg',
      quality: 80,
    })

    // Record the screenshot for rate limiting
    recordScreenshot(tabId)

    // POST screenshot to server, which saves to disk and returns filename
    const response = await fetch(`${serverUrl}/screenshots`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        dataUrl,
        url: tab.url,
        errorId: relatedErrorId || '',
        errorType: errorType || '',
      }),
    })

    if (!response.ok) {
      throw new Error(`Server returned ${response.status}`)
    }

    const result = await response.json()

    // Create log entry with filename reference only (no base64 data)
    const screenshotEntry = {
      ts: new Date().toISOString(),
      type: 'screenshot',
      level: 'info',
      url: tab.url,
      _enrichments: ['screenshot'],
      screenshotFile: result.filename,
      trigger: relatedErrorId ? 'error' : 'manual',
    }

    if (relatedErrorId) {
      screenshotEntry.relatedErrorId = relatedErrorId
    }

    debugLog(DebugCategory.CAPTURE, `Screenshot saved: ${result.filename}`, {
      trigger: screenshotEntry.trigger,
      relatedErrorId,
    })

    return { success: true, entry: screenshotEntry }
  } catch (error) {
    debugLog(DebugCategory.ERROR, 'Screenshot capture failed', { error: error.message })
    // Log screenshot failure to server so it's visible in the JSONL log
    const failureEntry = {
      ts: new Date().toISOString(),
      type: 'screenshot',
      level: 'warning',
      _screenshotFailed: true,
      error: error.message,
      trigger: relatedErrorId ? 'error' : 'manual',
      relatedErrorId: relatedErrorId || undefined,
    }
    logBatcher.add(failureEntry)
    return { success: false, error: error.message }
  }
}

/**
 * VLQ character to integer mapping
 */
const VLQ_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'
const VLQ_CHAR_MAP = new Map(VLQ_CHARS.split('').map((c, i) => [c, i]))

/**
 * Decode a VLQ-encoded string into an array of integers
 * @param {string} str - VLQ encoded string
 * @returns {number[]} Decoded integers
 */
export function decodeVLQ(str) {
  const result = []
  let shift = 0
  let value = 0

  for (const char of str) {
    const digit = VLQ_CHAR_MAP.get(char)
    if (digit === undefined) {
      throw new Error(`Invalid VLQ character: ${char}`)
    }

    const continued = digit & 32 // Check continuation bit
    value += (digit & 31) << shift

    if (continued) {
      shift += 5
    } else {
      // Check sign bit and convert
      const negate = value & 1
      value = value >> 1
      result.push(negate ? -value : value)
      value = 0
      shift = 0
    }
  }

  return result
}

/**
 * Parse a source map's mappings string into a structured format
 * @param {string} mappingsStr - The mappings field from source map
 * @returns {Array<Array<Array<number>>>} Parsed mappings [line][segment][field]
 */
export function parseMappings(mappingsStr) {
  const lines = mappingsStr.split(';')
  const parsed = []

  for (const line of lines) {
    const segments = []
    if (line.length > 0) {
      const segmentStrs = line.split(',')
      for (const segmentStr of segmentStrs) {
        if (segmentStr.length > 0) {
          segments.push(decodeVLQ(segmentStr))
        }
      }
    }
    parsed.push(segments)
  }

  return parsed
}

/**
 * Parse a stack trace line into components
 * @param {string} line - A single stack trace line
 * @returns {Object|null} Parsed frame or null
 */
export function parseStackFrame(line) {
  // Try standard format: "at functionName (file:line:col)" or "at file:line:col"
  const match = line.match(STACK_FRAME_REGEX)
  if (match) {
    const [, functionName, file1, line1, col1, file2, line2] = match
    return {
      functionName: functionName || '<anonymous>',
      fileName: file1 || file2,
      lineNumber: parseInt(line1 || line2, 10),
      columnNumber: col1 ? parseInt(col1, 10) : 0,
      raw: line,
    }
  }

  // Try anonymous format: "at file:line:col"
  const anonMatch = line.match(ANONYMOUS_FRAME_REGEX)
  if (anonMatch) {
    return {
      functionName: '<anonymous>',
      fileName: anonMatch[1],
      lineNumber: parseInt(anonMatch[2], 10),
      columnNumber: parseInt(anonMatch[3], 10),
      raw: line,
    }
  }

  return null
}

/**
 * Extract sourceMappingURL from script content
 * @param {string} content - Script content
 * @returns {string|null} Source map URL or null
 */
export function extractSourceMapUrl(content) {
  // Look for //# sourceMappingURL= or //@ sourceMappingURL= (deprecated)
  const regex = /\/\/[#@]\s*sourceMappingURL=(.+?)(?:\s|$)/
  const match = content.match(regex)
  return match ? match[1].trim() : null
}

/**
 * Fetch a source map for a script URL
 * @param {string} scriptUrl - The URL of the JavaScript file
 * @returns {Promise<Object|null>} Parsed source map or null
 */
export async function fetchSourceMap(scriptUrl) {
  // Check cache first
  if (sourceMapCache.has(scriptUrl)) {
    return sourceMapCache.get(scriptUrl)
  }

  try {
    // Enforce cache size limit
    if (sourceMapCache.size >= SOURCE_MAP_CACHE_SIZE) {
      const firstKey = sourceMapCache.keys().next().value
      sourceMapCache.delete(firstKey)
    }

    // Fetch the script to find the sourceMappingURL
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), SOURCE_MAP_FETCH_TIMEOUT)

    const scriptResponse = await fetch(scriptUrl, { signal: controller.signal })
    clearTimeout(timeoutId)

    if (!scriptResponse.ok) {
      sourceMapCache.set(scriptUrl, null)
      return null
    }

    const scriptContent = await scriptResponse.text()
    let sourceMapUrl = extractSourceMapUrl(scriptContent)

    if (!sourceMapUrl) {
      sourceMapCache.set(scriptUrl, null)
      return null
    }

    // Handle inline source maps (data URLs)
    if (sourceMapUrl.startsWith('data:')) {
      const base64Match = sourceMapUrl.match(/^data:application\/json;base64,(.+)$/)
      if (base64Match) {
        let jsonStr
        try {
          jsonStr = atob(base64Match[1])
        } catch {
          debugLog(DebugCategory.SOURCEMAP, 'Invalid base64 in inline source map', { scriptUrl })
          sourceMapCache.set(scriptUrl, null)
          return null
        }
        let sourceMap
        try {
          sourceMap = JSON.parse(jsonStr)
        } catch {
          debugLog(DebugCategory.SOURCEMAP, 'Invalid JSON in inline source map', { scriptUrl })
          sourceMapCache.set(scriptUrl, null)
          return null
        }
        const parsed = parseSourceMapData(sourceMap)
        sourceMapCache.set(scriptUrl, parsed)
        return parsed
      }
      sourceMapCache.set(scriptUrl, null)
      return null
    }

    // Resolve relative URLs
    if (!sourceMapUrl.startsWith('http')) {
      const base = scriptUrl.substring(0, scriptUrl.lastIndexOf('/') + 1)
      sourceMapUrl = new URL(sourceMapUrl, base).href
    }

    // Fetch the source map
    const mapController = new AbortController()
    const mapTimeoutId = setTimeout(() => mapController.abort(), SOURCE_MAP_FETCH_TIMEOUT)

    const mapResponse = await fetch(sourceMapUrl, { signal: mapController.signal })
    clearTimeout(mapTimeoutId)

    if (!mapResponse.ok) {
      sourceMapCache.set(scriptUrl, null)
      return null
    }

    let sourceMap
    try {
      sourceMap = await mapResponse.json()
    } catch {
      debugLog(DebugCategory.SOURCEMAP, 'Invalid JSON in external source map', {
        scriptUrl,
        sourceMapUrl,
      })
      sourceMapCache.set(scriptUrl, null)
      return null
    }
    const parsed = parseSourceMapData(sourceMap)
    sourceMapCache.set(scriptUrl, parsed)
    return parsed
  } catch (err) {
    // Network errors, timeouts, or other fetch failures
    debugLog(DebugCategory.SOURCEMAP, 'Source map fetch failed', {
      scriptUrl,
      error: err.message,
    })
    sourceMapCache.set(scriptUrl, null)
    return null
  }
}

/**
 * Parse source map data into a usable format
 * @param {Object} sourceMap - Raw source map JSON
 * @returns {Object} Parsed source map with decoded mappings
 */
export function parseSourceMapData(sourceMap) {
  const mappings = parseMappings(sourceMap.mappings || '')
  return {
    sources: sourceMap.sources || [],
    names: sourceMap.names || [],
    sourceRoot: sourceMap.sourceRoot || '',
    mappings,
    sourcesContent: sourceMap.sourcesContent || [],
  }
}

/**
 * Find original location from source map
 * @param {Object} sourceMap - Parsed source map
 * @param {number} line - Generated line (1-based)
 * @param {number} column - Generated column (0-based)
 * @returns {Object|null} Original location or null
 */
export function findOriginalLocation(sourceMap, line, column) {
  if (!sourceMap || !sourceMap.mappings) return null

  // Convert to 0-based line
  const lineIndex = line - 1
  if (lineIndex < 0 || lineIndex >= sourceMap.mappings.length) return null

  const lineSegments = sourceMap.mappings[lineIndex]
  if (!lineSegments || lineSegments.length === 0) return null

  // Track accumulated values (source map uses relative values)
  let genCol = 0
  let sourceIndex = 0
  let origLine = 0
  let origCol = 0
  let nameIndex = 0

  // Find the segment for this column
  let bestMatch = null

  // We need to accumulate from the beginning
  for (let li = 0; li <= lineIndex; li++) {
    genCol = 0 // Reset column for each line

    const segments = sourceMap.mappings[li]
    for (const segment of segments) {
      if (segment.length >= 1) genCol += segment[0]
      if (segment.length >= 2) sourceIndex += segment[1]
      if (segment.length >= 3) origLine += segment[2]
      if (segment.length >= 4) origCol += segment[3]
      if (segment.length >= 5) nameIndex += segment[4]

      if (li === lineIndex && genCol <= column) {
        bestMatch = {
          source: sourceMap.sources[sourceIndex],
          line: origLine + 1, // Convert to 1-based
          column: origCol,
          name: segment.length >= 5 ? sourceMap.names[nameIndex] : null,
        }
      }
    }
  }

  return bestMatch
}

/**
 * Resolve a single stack frame to original location
 * @param {Object} frame - Parsed stack frame
 * @returns {Promise<Object>} Resolved frame
 */
export async function resolveStackFrame(frame) {
  if (!frame.fileName || !frame.fileName.startsWith('http')) {
    return frame
  }

  const sourceMap = await fetchSourceMap(frame.fileName)
  if (!sourceMap) {
    return frame
  }

  const original = findOriginalLocation(sourceMap, frame.lineNumber, frame.columnNumber)
  if (!original) {
    return frame
  }

  return {
    ...frame,
    originalFileName: original.source,
    originalLineNumber: original.line,
    originalColumnNumber: original.column,
    originalFunctionName: original.name || frame.functionName,
    resolved: true,
  }
}

/**
 * Resolve an entire stack trace
 * @param {string} stack - Stack trace string
 * @returns {Promise<string>} Resolved stack trace
 */
export async function resolveStackTrace(stack) {
  if (!stack || !sourceMapEnabled) return stack

  const lines = stack.split('\n')
  const resolvedLines = []

  for (const line of lines) {
    const frame = parseStackFrame(line)
    if (!frame) {
      resolvedLines.push(line)
      continue
    }

    try {
      const resolved = await resolveStackFrame(frame)
      if (resolved.resolved) {
        // Format resolved frame
        const funcName = resolved.originalFunctionName || resolved.functionName
        const fileName = resolved.originalFileName
        const lineNum = resolved.originalLineNumber
        const colNum = resolved.originalColumnNumber

        resolvedLines.push(
          `    at ${funcName} (${fileName}:${lineNum}:${colNum}) [resolved from ${resolved.fileName}:${resolved.lineNumber}:${resolved.columnNumber}]`,
        )
      } else {
        resolvedLines.push(line)
      }
    } catch {
      resolvedLines.push(line)
    }
  }

  return resolvedLines.join('\n')
}

/**
 * Clear the source map cache
 */
export function clearSourceMapCache() {
  sourceMapCache.clear()
}

// =============================================================================
// MEMORY ENFORCEMENT
// =============================================================================

/**
 * Estimate total buffer memory usage from buffer contents.
 * Uses count * average entry size + variable data lengths.
 *
 * @param {Object} buffers - Object containing buffer arrays
 * @param {Array} buffers.logEntries - Log entry buffer
 * @param {Array} buffers.wsEvents - WebSocket event buffer
 * @param {Array} buffers.networkBodies - Network body buffer
 * @param {Array} buffers.enhancedActions - Enhanced action buffer
 * @returns {number} Estimated memory usage in bytes
 */
export function estimateBufferMemory(buffers) {
  let total = 0

  // Log entries: count * avg size
  total += buffers.logEntries.length * MEMORY_AVG_LOG_ENTRY_SIZE

  // WebSocket events: count * avg size + data lengths
  for (const event of buffers.wsEvents) {
    total += MEMORY_AVG_WS_EVENT_SIZE
    if (event.data && typeof event.data === 'string') {
      total += event.data.length
    }
  }

  // Network bodies: count * avg size + request/response body lengths
  for (const body of buffers.networkBodies) {
    total += MEMORY_AVG_NETWORK_BODY_SIZE
    if (body.requestBody && typeof body.requestBody === 'string') {
      total += body.requestBody.length
    }
    if (body.responseBody && typeof body.responseBody === 'string') {
      total += body.responseBody.length
    }
  }

  // Enhanced actions: count * avg size
  total += buffers.enhancedActions.length * MEMORY_AVG_ACTION_SIZE

  return total
}

/**
 * Check memory pressure and take appropriate action.
 * Updates the memory pressure state based on estimated memory usage.
 *
 * @param {Object} buffers - Buffer contents to estimate memory from
 * @returns {Object} Result with level, action, estimatedMemory, and alreadyApplied
 */
export function checkMemoryPressure(buffers) {
  const estimatedMemory = estimateBufferMemory(buffers)
  lastMemoryCheck = Date.now()

  if (estimatedMemory >= MEMORY_HARD_LIMIT) {
    const alreadyApplied = memoryPressureLevel === 'hard'
    memoryPressureLevel = 'hard'
    networkBodyCaptureDisabled = true
    reducedCapacities = true
    return {
      level: 'hard',
      action: 'disable_network_capture',
      estimatedMemory,
      alreadyApplied,
    }
  }

  if (estimatedMemory >= MEMORY_SOFT_LIMIT) {
    const alreadyApplied = memoryPressureLevel === 'soft' || memoryPressureLevel === 'hard'
    memoryPressureLevel = 'soft'
    reducedCapacities = true
    // If we were at hard and dropped to soft, re-enable network capture
    if (networkBodyCaptureDisabled && estimatedMemory < MEMORY_HARD_LIMIT) {
      networkBodyCaptureDisabled = false
    }
    return {
      level: 'soft',
      action: 'reduce_capacities',
      estimatedMemory,
      alreadyApplied,
    }
  }

  // Below soft limit - recover to normal
  memoryPressureLevel = 'normal'
  reducedCapacities = false
  networkBodyCaptureDisabled = false
  return {
    level: 'normal',
    action: 'none',
    estimatedMemory,
    alreadyApplied: false,
  }
}

/**
 * Get the current memory pressure state.
 * @returns {Object} Current memory pressure state
 */
export function getMemoryPressureState() {
  return {
    memoryPressureLevel,
    lastMemoryCheck,
    networkBodyCaptureDisabled,
    reducedCapacities,
  }
}

/**
 * Reset memory pressure state to initial values (for testing).
 */
export function resetMemoryPressureState() {
  memoryPressureLevel = 'normal'
  lastMemoryCheck = 0
  networkBodyCaptureDisabled = false
  reducedCapacities = false
}

/**
 * Handle auto-screenshot on error if enabled
 */
async function maybeAutoScreenshot(errorEntry, sender) {
  if (!screenshotOnError) return
  if (!sender?.tab?.id) return

  // Only screenshot for actual errors
  if (errorEntry.level !== 'error') return

  // Only for exceptions and certain types
  if (errorEntry.type !== 'exception' && errorEntry.type !== 'network') return

  // Generate a simple error ID
  const errorId = `err_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  errorEntry._errorId = errorId

  const result = await captureScreenshot(sender.tab.id, errorId, errorEntry.type)

  if (result.success && result.entry) {
    logBatcher.add(result.entry)
  }
}

// Initialize if running in extension context
if (typeof chrome !== 'undefined' && chrome.runtime) {
  // Message handler
  chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.type === 'ws_event') {
      wsBatcher.add(message.payload)
    } else if (message.type === 'enhanced_action') {
      enhancedActionBatcher.add(message.payload)
    } else if (message.type === 'network_body') {
      networkBodyBatcher.add(message.payload)
    } else if (message.type === 'performance_snapshot') {
      sendPerformanceSnapshotToServer(message.payload)
    } else if (message.type === 'log') {
      handleLogMessage(message.payload, sender)
    } else if (message.type === 'getStatus') {
      sendResponse({
        ...connectionStatus,
        serverUrl,
        screenshotOnError,
        sourceMapEnabled,
        debugMode,
        contextWarning: getContextWarning(),
        circuitBreakerState: sharedServerCircuitBreaker.getState(),
        memoryPressure: getMemoryPressureState(),
      })
    } else if (message.type === 'clearLogs') {
      handleClearLogs().then(sendResponse)
      return true // async response
    } else if (message.type === 'setLogLevel') {
      currentLogLevel = message.level
      chrome.storage.local.set({ logLevel: message.level })
    } else if (message.type === 'setScreenshotOnError') {
      screenshotOnError = message.enabled
      chrome.storage.local.set({ screenshotOnError: message.enabled })
      sendResponse({ success: true })
    } else if (message.type === 'captureScreenshot') {
      // Manual screenshot capture
      chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
        if (tabs[0]?.id) {
          const result = await captureScreenshot(tabs[0].id, null)
          if (result.success && result.entry) {
            logBatcher.add(result.entry)
          }
          sendResponse(result)
        } else {
          sendResponse({ success: false, error: 'No active tab' })
        }
      })
      return true // async response
    } else if (message.type === 'setSourceMapEnabled') {
      sourceMapEnabled = message.enabled
      chrome.storage.local.set({ sourceMapEnabled: message.enabled })
      if (!message.enabled) {
        clearSourceMapCache()
      }
      sendResponse({ success: true })
    } else if (
      message.type === 'setNetworkWaterfallEnabled' ||
      message.type === 'setPerformanceMarksEnabled' ||
      message.type === 'setActionReplayEnabled' ||
      message.type === 'setWebSocketCaptureEnabled' ||
      message.type === 'setWebSocketCaptureMode' ||
      message.type === 'setPerformanceSnapshotEnabled' ||
      message.type === 'setDeferralEnabled' ||
      message.type === 'setNetworkBodyCaptureEnabled'
    ) {
      // Forward to all content scripts
      debugLog(DebugCategory.SETTINGS, `Setting ${message.type}: ${message.enabled}`)
      chrome.tabs.query({}, (tabs) => {
        for (const tab of tabs) {
          if (tab.id) {
            chrome.tabs.sendMessage(tab.id, message).catch((err) => {
              // Expected: tabs without content scripts (chrome://, edge://, file://, etc.)
              // Log unexpected errors for debugging
              if (
                !err.message?.includes('Receiving end does not exist') &&
                !err.message?.includes('Could not establish connection')
              ) {
                debugLog(DebugCategory.ERROR, 'Unexpected error forwarding setting to tab', {
                  tabId: tab.id,
                  error: err.message,
                })
              }
            })
          }
        }
      })
      sendResponse({ success: true })
    } else if (message.type === 'setDebugMode') {
      setDebugMode(message.enabled)
      chrome.storage.local.set({ debugMode: message.enabled })
      sendResponse({ success: true })
    } else if (message.type === 'getDebugLog') {
      sendResponse({ log: exportDebugLog() })
    } else if (message.type === 'clearDebugLog') {
      clearDebugLog()
      debugLog(DebugCategory.LIFECYCLE, 'Debug log cleared')
      sendResponse({ success: true })
    } else if (message.type === 'setServerUrl') {
      serverUrl = message.url || DEFAULT_SERVER_URL
      chrome.storage.local.set({ serverUrl })
      debugLog(DebugCategory.SETTINGS, `Server URL changed to: ${serverUrl}`)
      // Re-check connection with new URL
      checkConnectionAndUpdate()
      sendResponse({ success: true })
    }
  })

  // Set up chrome.alarms for periodic tasks (reliable across service worker restarts)
  if (chrome.alarms) {
    chrome.alarms.create('reconnect', { periodInMinutes: RECONNECT_INTERVAL / 60000 })
    chrome.alarms.create('errorGroupFlush', { periodInMinutes: 0.5 })
    chrome.alarms.create('memoryCheck', { periodInMinutes: MEMORY_CHECK_INTERVAL_MS / 60000 })

    chrome.alarms.onAlarm.addListener((alarm) => {
      if (alarm.name === 'reconnect') {
        checkConnectionAndUpdate()
      } else if (alarm.name === 'errorGroupFlush') {
        const aggregatedEntries = flushErrorGroups()
        if (aggregatedEntries.length > 0) {
          aggregatedEntries.forEach((entry) => logBatcher.add(entry))
        }
      } else if (alarm.name === 'memoryCheck') {
        debugLog(DebugCategory.LIFECYCLE, 'Memory check alarm fired')
      }
    })
  }

  // Initial connection check
  checkConnectionAndUpdate()

  // Load saved settings
  chrome.storage.local.get(
    ['serverUrl', 'logLevel', 'screenshotOnError', 'sourceMapEnabled', 'debugMode'],
    (result) => {
      if (chrome.runtime.lastError) {
        console.warn('[Gasoline] Could not load saved settings:', chrome.runtime.lastError.message, '- using defaults')
        // Continue with defaults already set at module level
        return
      }
      serverUrl = result.serverUrl || DEFAULT_SERVER_URL
      currentLogLevel = result.logLevel || 'error'
      screenshotOnError = result.screenshotOnError || false
      sourceMapEnabled = result.sourceMapEnabled || false
      debugMode = result.debugMode || false
      debugLog(DebugCategory.LIFECYCLE, 'Extension initialized', {
        serverUrl,
        logLevel: currentLogLevel,
        screenshotOnError,
        sourceMapEnabled,
        debugMode,
      })
    },
  )

  // Clean up screenshot rate limits when tabs are closed
  if (chrome.tabs && chrome.tabs.onRemoved) {
    chrome.tabs.onRemoved.addListener((tabId) => {
      screenshotTimestamps.delete(tabId)
    })
  }
}

// Shared circuit breaker instance for all batchers (global rate limit per spec).
// All ingest batchers share this single circuit breaker so that failures from
// any endpoint (logs, WS, actions, network bodies) contribute to the same
// consecutive failure count and circuit state.
const sharedServerCircuitBreaker = createCircuitBreaker(() => Promise.reject(new Error('shared circuit breaker')), {
  maxFailures: RATE_LIMIT_CONFIG.maxFailures,
  resetTimeout: RATE_LIMIT_CONFIG.resetTimeout,
  initialBackoff: 0,
  maxBackoff: 0,
})

// Helper to create a send function that updates global connectionStatus on failure
function withConnectionStatus(sendFn, onSuccess) {
  return async (entries) => {
    try {
      const result = await sendFn(entries)
      connectionStatus.connected = true
      if (onSuccess) onSuccess(entries, result)
      updateBadge(connectionStatus)
      return result
    } catch (err) {
      connectionStatus.connected = false
      updateBadge(connectionStatus)
      throw err
    }
  }
}

// Log batcher instance with circuit breaker
const logBatcherWithCB = createBatcherWithCircuitBreaker(
  withConnectionStatus(
    (entries) => {
      checkContextAnnotations(entries)
      return sendLogsToServer(entries)
    },
    (entries, result) => {
      connectionStatus.entries = result.entries || connectionStatus.entries + entries.length
      connectionStatus.errorCount += entries.filter((e) => e.level === 'error').length
    },
  ),
  { sharedCircuitBreaker: sharedServerCircuitBreaker },
)
const logBatcher = logBatcherWithCB.batcher

// WebSocket event batcher instance with circuit breaker
const wsBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(sendWSEventsToServer), {
  debounceMs: 200,
  maxBatchSize: 100,
  sharedCircuitBreaker: sharedServerCircuitBreaker,
})
const wsBatcher = wsBatcherWithCB.batcher

// Enhanced action batcher instance with circuit breaker
const enhancedActionBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(sendEnhancedActionsToServer), {
  debounceMs: 200,
  maxBatchSize: 50,
  sharedCircuitBreaker: sharedServerCircuitBreaker,
})
const enhancedActionBatcher = enhancedActionBatcherWithCB.batcher

// Network body batcher instance with circuit breaker
const networkBodyBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(sendNetworkBodiesToServer), {
  debounceMs: 200,
  maxBatchSize: 50,
  sharedCircuitBreaker: sharedServerCircuitBreaker,
})
const networkBodyBatcher = networkBodyBatcherWithCB.batcher

async function handleLogMessage(payload, sender) {
  if (!shouldCaptureLog(payload.level, currentLogLevel, payload.type)) {
    debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}`)
    return
  }

  let entry = formatLogEntry(payload)
  debugLog(DebugCategory.CAPTURE, `Log received: type=${entry.type}, level=${entry.level}`, {
    url: entry.url,
    enrichments: entry._enrichments,
  })

  // Resolve stack traces if source maps are enabled
  if (sourceMapEnabled && entry.stack) {
    try {
      const resolvedStack = await resolveStackTrace(entry.stack)
      // Add sourceMap to enrichments list
      const enrichments = entry._enrichments ? [...entry._enrichments] : []
      if (!enrichments.includes('sourceMap')) {
        enrichments.push('sourceMap')
      }
      entry = {
        ...entry,
        stack: resolvedStack,
        _sourceMapResolved: true,
        _enrichments: enrichments,
      }
      debugLog(DebugCategory.CAPTURE, 'Stack trace resolved via source map')
    } catch (err) {
      debugLog(DebugCategory.ERROR, 'Source map resolution failed', { error: err.message })
    }
  }

  // Process through error grouping
  const { shouldSend, entry: processedEntry } = processErrorGroup(entry)

  if (shouldSend && processedEntry) {
    logBatcher.add(processedEntry)
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
      aggregatedCount: processedEntry._aggregatedCount,
    })

    // Try to auto-screenshot on error (if enabled)
    maybeAutoScreenshot(processedEntry, sender)
  } else {
    debugLog(DebugCategory.CAPTURE, 'Log deduplicated (error grouping)')
  }
}

async function handleClearLogs() {
  try {
    await fetch(`${serverUrl}/logs`, { method: 'DELETE' })
    connectionStatus.entries = 0
    connectionStatus.errorCount = 0
    updateBadge(connectionStatus)
    return { success: true }
  } catch (error) {
    return { success: false, error: error.message }
  }
}

async function checkConnectionAndUpdate() {
  const health = await checkServerHealth()
  const wasConnected = connectionStatus.connected
  connectionStatus = {
    ...connectionStatus,
    ...health,
    connected: health.connected,
  }
  updateBadge(connectionStatus)

  // Log connection status changes
  if (wasConnected !== health.connected) {
    debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
      entries: connectionStatus.entries,
      error: health.error || null,
    })
  }

  // Poll capture settings when connected
  if (health.connected) {
    const overrides = await pollCaptureSettings(serverUrl)
    if (overrides !== null) {
      applyCaptureOverrides(overrides)
    }
  }

  // Notify popup if open
  if (typeof chrome !== 'undefined' && chrome.runtime) {
    chrome.runtime
      .sendMessage({
        type: 'statusUpdate',
        status: { ...connectionStatus, aiControlled },
      })
      .catch(() => {
        // Popup not open, ignore
      })
  }
}

/**
 * Poll the server's /settings endpoint for AI capture overrides.
 * @param {string} url - Server base URL
 * @returns {Promise<Object|null>} Overrides map or null on error
 */
export async function pollCaptureSettings(url) {
  try {
    const response = await fetch(`${url}/settings`)
    if (!response.ok) return null
    const data = await response.json()
    return data.capture_overrides || {}
  } catch {
    return null
  }
}

/**
 * Apply AI capture overrides to the extension's settings.
 * @param {Object} overrides - Map of setting name to value
 */
export function applyCaptureOverrides(overrides) {
  _captureOverrides = overrides
  aiControlled = Object.keys(overrides).length > 0

  if (overrides.log_level !== undefined) {
    currentLogLevel = overrides.log_level
  }
  if (overrides.network_bodies !== undefined) {
    networkBodyCaptureDisabled = overrides.network_bodies === 'false'
  }
  if (overrides.screenshot_on_error !== undefined) {
    screenshotOnError = overrides.screenshot_on_error === 'true'
  }
}

// =============================================================================
// ON-DEMAND QUERY POLLING
// =============================================================================

let queryPollingInterval = null

/**
 * Poll the server for pending queries (DOM queries, a11y audits)
 * @param {string} serverUrl - The server base URL
 */
export async function pollPendingQueries(serverUrl) {
  try {
    const response = await fetch(`${serverUrl}/pending-queries`)
    if (!response.ok) return

    const data = await response.json()
    if (!data.queries || data.queries.length === 0) return

    for (const query of data.queries) {
      await handlePendingQuery(query, serverUrl)
    }
  } catch {
    // Server unavailable, silently ignore
  }
}

/**
 * Handle a single pending query by dispatching to the active tab
 * @param {Object} query - { id, type, params }
 * @param {string} serverUrl - The server base URL
 */
export async function handlePendingQuery(query, _serverUrl) {
  try {
    const tabs = await new Promise((resolve) => {
      chrome.tabs.query({ active: true, currentWindow: true }, resolve)
    })

    if (!tabs || tabs.length === 0) return

    const tabId = tabs[0].id
    const messageType = query.type === 'a11y' ? 'GASOLINE_A11Y_AUDIT' : 'GASOLINE_DOM_QUERY'

    await chrome.tabs.sendMessage(tabId, {
      type: messageType,
      queryId: query.id,
      params: query.params,
    })
  } catch {
    // Tab communication failed
  }
}

/**
 * Post query results back to the server
 * @param {string} serverUrl - The server base URL
 * @param {string} queryId - The query ID
 * @param {string} type - Query type ('dom' or 'a11y')
 * @param {Object} result - The query result
 */
export async function postQueryResult(serverUrl, queryId, type, result) {
  const endpoint = type === 'a11y' ? '/a11y-result' : '/dom-result'

  await fetch(`${serverUrl}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: queryId, result }),
  })
}

/**
 * Start polling for pending queries at 1-second intervals
 * @param {string} serverUrl - The server base URL
 */
export function startQueryPolling(serverUrl) {
  stopQueryPolling()
  queryPollingInterval = setInterval(() => pollPendingQueries(serverUrl), 1000)
}

/**
 * Stop polling for pending queries
 */
export function stopQueryPolling() {
  if (queryPollingInterval) {
    clearInterval(queryPollingInterval)
    queryPollingInterval = null
  }
}

// =============================================================================
// AI WEB PILOT SAFETY GATE
// =============================================================================

/**
 * Check if AI Web Pilot is enabled in the extension popup.
 * Uses chrome.storage.sync for cross-device persistence.
 * @returns {Promise<boolean>} True if AI Web Pilot is enabled
 */
export async function isAiWebPilotEnabled() {
  return new Promise((resolve) => {
    chrome.storage.sync.get(['aiWebPilotEnabled'], (result) => {
      // Default to false (disabled) for safety
      resolve(result.aiWebPilotEnabled === true)
    })
  })
}

/**
 * Handle pilot commands (GASOLINE_HIGHLIGHT, GASOLINE_MANAGE_STATE, GASOLINE_EXECUTE_JS).
 * Checks the AI Web Pilot toggle before forwarding to content scripts.
 * @param {string} command - The pilot command type
 * @param {Object} params - Command parameters
 * @returns {Promise<Object>} Result or error response
 */
export async function handlePilotCommand(command, params) {
  // Check if AI Web Pilot is enabled
  const enabled = await isAiWebPilotEnabled()

  if (!enabled) {
    return { error: 'ai_web_pilot_disabled' }
  }

  // Phase 1: Stub - just acknowledge the command is accepted
  // Phase 2 will implement actual forwarding to content scripts
  try {
    const tabs = await new Promise((resolve) => {
      chrome.tabs.query({ active: true, currentWindow: true }, resolve)
    })

    if (!tabs || tabs.length === 0) {
      return { error: 'no_active_tab' }
    }

    const tabId = tabs[0].id

    // Forward command to content script
    const result = await chrome.tabs.sendMessage(tabId, {
      type: command,
      params,
    })

    return result || { success: true }
  } catch (err) {
    return { error: err.message || 'command_failed' }
  }
}
