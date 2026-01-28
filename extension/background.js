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

// Session ID for detecting extension reloads - generated fresh each time service worker starts
const EXTENSION_SESSION_ID = `ext_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`

// Debug mode flag - initialized here, later fully loaded from storage
let debugMode = false // Controls all diagnostic logging ([DIAGNOSTIC] logs and extDebugLog)

/**
 * Log a diagnostic message only when debug mode is enabled
 * Gated behind debugMode flag to avoid console spam in production
 * @param {string} message - Diagnostic message to log
 */
function diagnosticLog(message) {
  if (debugMode) {
    console.log(message)
  }
}

// DIAGNOSTIC: Measure initialization timing
const _moduleLoadTime = performance.now()
diagnosticLog(`[DIAGNOSTIC] Module load start at ${_moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`)

// Startup verification (always logs, regardless of debug mode)
console.log(`[Gasoline] Background service worker loaded - session ${EXTENSION_SESSION_ID}`)

// Load debug mode setting from storage (early load for startup diagnostics)
chrome.storage.local.get(['debugMode'], (result) => {
  debugMode = result.debugMode === true
  if (debugMode) {
    console.log('[Gasoline] Debug mode enabled on startup')
  }
})

// ============================================================================
// TAB TRACKING: Clear on Browser Restart
// ============================================================================
// Tab IDs are invalidated after a browser restart. Clear tracking state
// to enter "no tracking" mode until the user explicitly re-enables it.
if (chrome.runtime && chrome.runtime.onStartup) {
  chrome.runtime.onStartup.addListener(() => {
    console.log('[Gasoline] Browser restarted - clearing tracking state')
    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
  })
}

/**
 * ============================================================================
 * AI WEB PILOT STATE HANDLING — SINGLE SOURCE OF TRUTH
 * ============================================================================
 * ARCHITECTURE PRINCIPLE: background.js _aiWebPilotEnabledCache is the
 * authoritative source for whether AI Web Pilot is enabled. All other
 * components (popup, content scripts, server) query this cache or the
 * underlying storage.
 *
 * STATE FLOW:
 * 1. User toggles in popup.html checkbox
 * 2. Popup sends message: { type: 'setAiWebPilotEnabled', enabled: boolean }
 * 3. Background receives message, updates _aiWebPilotEnabledCache
 * 4. Background persists to chrome.storage (sync/local/session)
 * 5. Storage change listener keeps cache in sync if storage changes
 * 6. Pilot commands (execute_js, highlight, etc.) check cache
 *
 * CRITICAL: DO NOT let popup or content scripts write to chrome.storage
 * directly. Only background.js updates storage to prevent race conditions
 * where cache gets out of sync.
 *
 * WHY THIS MATTERS:
 * - Chrome storage APIs are async with unpredictable ordering
 * - Multiple writers create race conditions where cache != storage
 * - This caused a persistent UAT bug where UI showed "on" but cache was "off"
 * - Solution: single writer (background) + listener for sync guarantees
 *
 * ============================================================================
 */

// AI Web Pilot toggle - stored in chrome.storage.local only (no sync/session complexity)
// SIMPLIFIED: One storage area, one cache, defaults to false if not found
let _aiWebPilotEnabledCache = false
let _aiWebPilotCacheInitialized = false // Track when async init completes
let _pilotInitCallback = null // Callback to invoke when init is complete
const _pilotLoadStartTime = performance.now()

// Load AI Web Pilot state from chrome.storage.local on startup
// CRITICAL: This is async, so polling won't start until this callback fires
if (typeof chrome !== 'undefined' && chrome.storage) {
  chrome.storage.local.get(['aiWebPilotEnabled'], (result) => {
    // If not set in storage, cache is already false. If set, use that value.
    const wasLoaded = result.aiWebPilotEnabled === true
    _aiWebPilotEnabledCache = wasLoaded
    _aiWebPilotCacheInitialized = true // Mark as initialized
    const loadTime = performance.now() - _pilotLoadStartTime
    console.log(`[Gasoline] AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`)
    console.log('[Gasoline] Storage value:', result.aiWebPilotEnabled, '| Cache value:', _aiWebPilotEnabledCache)

    // Invoke callback if one is waiting
    if (_pilotInitCallback) {
      _pilotInitCallback()
      _pilotInitCallback = null
    }

    // POST settings to server immediately after cache loads (protocol: POST on init)
    // See docs/plugin-server-communications.md
    chrome.storage.local.get(['serverUrl'], (serverResult) => {
      if (serverResult.serverUrl) {
        postSettings(serverResult.serverUrl)
      }
    })
  })
}

// Reset pilot cache for testing (module-level state persists across Node.js cached imports)
export function _resetPilotCacheForTesting(value) {
  _aiWebPilotEnabledCache = value !== undefined ? value : null
}

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

// Error group max age (1 hour) - entries older than this are cleaned up regardless of flush cycle
// Issue 6 fix: prevents unbounded memory growth with many unique errors
export const ERROR_GROUP_MAX_AGE_MS = 3600000 // 1 hour

/**
 * Get current state of error groups (for testing)
 * @returns {Map} Current error groups map
 */
export function getErrorGroupsState() {
  return errorGroups
}

/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS.
 * Called periodically to prevent unbounded memory growth (Issue 6 fix).
 */
export function cleanupStaleErrorGroups() {
  const now = Date.now()
  for (const [signature, group] of errorGroups) {
    if (now - group.lastSeen > ERROR_GROUP_MAX_AGE_MS) {
      errorGroups.delete(signature)
      debugLog(DebugCategory.ERROR, 'Cleaned up stale error group', {
        signature: signature.slice(0, 50) + '...',
        age: Math.round((now - group.lastSeen) / 60000) + ' min',
      })
    }
  }
}

/**
 * Log a diagnostic message only if debug mode is enabled
 * Usage: extDebugLog('message', obj)
 */
function extDebugLog(...args) {
  if (debugMode) {
    console.log('[Gasoline DEBUG]', ...args)
  }
}

// Rate limiting state
const screenshotTimestamps = new Map() // tabId -> [timestamps]

// Source map state
const sourceMapCache = new Map() // scriptUrl -> { mappings, sources, names, sourceRoot }
let sourceMapEnabled = false // Source map resolution (off by default)

// Export the cache size constant for testing
export { SOURCE_MAP_CACHE_SIZE }

/**
 * Set an entry in the source map cache with LRU eviction (Issue 4 fix)
 * Similar to aiSourceMapCache pattern in ai-context.js
 * @param {string} url - The script URL
 * @param {Object|null} map - The parsed source map or null
 */
export function setSourceMapCacheEntry(url, map) {
  // Evict oldest if adding new entry and at capacity
  if (!sourceMapCache.has(url) && sourceMapCache.size >= SOURCE_MAP_CACHE_SIZE) {
    const firstKey = sourceMapCache.keys().next().value
    sourceMapCache.delete(firstKey)
  }
  // Move to end (LRU): delete first if exists, then add
  // This ensures recently accessed/updated entries are kept longest
  sourceMapCache.delete(url)
  sourceMapCache.set(url, map)
}

/**
 * Get an entry from the source map cache
 * @param {string} url - The script URL
 * @returns {Object|null} The cached source map or null
 */
export function getSourceMapCacheEntry(url) {
  return sourceMapCache.get(url) || null
}

/**
 * Get the current size of the source map cache
 * @returns {number} Number of entries in cache
 */
export function getSourceMapCacheSize() {
  return sourceMapCache.size
}

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
// debugMode is now declared at the top of the file
const debugLogBuffer = [] // Circular buffer of debug entries
const extensionLogQueue = [] // Queue for sending to server

/**
 * Log categories for debug output
 */
export const DebugCategory = {
  CONNECTION: 'connection',
  CAPTURE: 'capture',
  ERROR: 'error',
  LIFECYCLE: 'lifecycle',
  SETTINGS: 'settings',
  SOURCEMAP: 'sourcemap',
  QUERY: 'query',
}

/**
 * Log a debug message (only when debug mode is enabled)
 * @param {string} category - Log category (connection, capture, error, lifecycle, settings)
 * @param {string} message - Debug message
 * @param {Object} data - Optional additional data
 */
export function debugLog(category, message, data = null) {
  const timestamp = new Date().toISOString()
  const entry = {
    ts: timestamp,
    category,
    message,
    ...(data ? { data } : {}),
  }

  // Always add to buffer (for export even if debug mode was off)
  debugLogBuffer.push(entry)
  if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
    debugLogBuffer.shift()
  }

  // Queue for server (if connected)
  if (connectionStatus.connected) {
    extensionLogQueue.push({
      timestamp,
      level: 'debug',
      message,
      source: 'background',
      category,
      ...(data ? { data } : {}),
    })
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

  function recordFailure() {
    consecutiveFailures++
    totalFailures++
    lastFailureTime = Date.now()
    if (consecutiveFailures >= maxFailures) {
      state = 'open'
    }
    currentBackoff =
      consecutiveFailures >= 2 ? Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff) : 0
  }

  return { execute, getState, getStats, reset, recordFailure }
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

// Maximum pending buffer size to prevent unbounded growth when circuit breaker is open
export const MAX_PENDING_BUFFER = 1000

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
    // eslint-disable-next-line security/detect-object-injection -- idx is computed from Math.min with bounded array index
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
      // Record failure on shared circuit breaker without re-sending
      cb.recordFailure()
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

    // If circuit is open, put entries back into pending buffer (capped)
    if (currentState === 'open') {
      pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER)
      return
    }

    // Each flush attempt records one success/failure on the circuit breaker.
    // The retry budget controls how many times we attempt this batch.
    try {
      await attemptSend(entries)
      localConnectionStatus.connected = true
    } catch {
      localConnectionStatus.connected = false

      // If circuit opened, buffer the entries for later draining (capped)
      if (cb.getState() === 'open') {
        pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER)
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

          // If circuit opened during retry, buffer entries (capped)
          if (cb.getState() === 'open') {
            pending = entries.concat(pending).slice(0, MAX_PENDING_BUFFER)
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
      if (pending.length >= MAX_PENDING_BUFFER) return // Cap pending buffer
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
      if (pending.length >= MAX_PENDING_BUFFER) return // Cap pending buffer
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
 * Send network waterfall data to server (PerformanceResourceTiming entries)
 * @param {Object} payload - Waterfall payload with entries and pageURL
 */
export async function sendNetworkWaterfallToServer(payload) {
  debugLog(DebugCategory.CONNECTION, `Sending ${payload.entries.length} waterfall entries to server`)

  const response = await fetch(`${serverUrl}/network-waterfall`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    const error = `Server error (network waterfall): ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  debugLog(DebugCategory.CONNECTION, `Server accepted ${payload.entries.length} waterfall entries`)
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
 * Send performance snapshots to server (batch endpoint)
 * @param {Array} snapshots - Array of performance snapshot objects
 */
export async function sendPerformanceSnapshotsToServer(snapshots) {
  debugLog(DebugCategory.CONNECTION, `Sending ${snapshots.length} performance snapshots to server`)

  const response = await fetch(`${serverUrl}/performance-snapshots`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ snapshots }),
  })

  if (!response.ok) {
    const error = `Server error (performance snapshots): ${response.status} ${response.statusText}`
    debugLog(DebugCategory.ERROR, error)
    throw new Error(error)
  }

  debugLog(DebugCategory.CONNECTION, `Server accepted ${snapshots.length} performance snapshots`)
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

  // eslint-disable-next-line security/detect-object-injection -- lineIndex validated against array bounds above
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

    // eslint-disable-next-line security/detect-object-injection -- li bounded by validated lineIndex
    const segments = sourceMap.mappings[li]
    for (const segment of segments) {
      if (segment.length >= 1) genCol += segment[0]
      if (segment.length >= 2) sourceIndex += segment[1]
      if (segment.length >= 3) origLine += segment[2]
      if (segment.length >= 4) origCol += segment[3]
      if (segment.length >= 5) nameIndex += segment[4]

      if (li === lineIndex && genCol <= column) {
        bestMatch = {
          // eslint-disable-next-line security/detect-object-injection -- sourceIndex accumulated from validated source map data
          source: sourceMap.sources[sourceIndex],
          line: origLine + 1, // Convert to 1-based
          column: origCol,
          // eslint-disable-next-line security/detect-object-injection -- nameIndex accumulated from validated source map data
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
    // Content scripts can't access chrome.tabs — provide tab ID on request
    if (message.type === 'GET_TAB_ID') {
      sendResponse({ tabId: sender.tab?.id })
      return true
    }

    if (message.type === 'ws_event') {
      wsBatcher.add(message.payload)
    } else if (message.type === 'enhanced_action') {
      enhancedActionBatcher.add(message.payload)
    } else if (message.type === 'network_body') {
      if (networkBodyCaptureDisabled) {
        debugLog(DebugCategory.CAPTURE, 'Network body dropped: capture disabled')
        return true
      }
      networkBodyBatcher.add(message.payload)
    } else if (message.type === 'performance_snapshot') {
      perfBatcher.add(message.payload)
    } else if (message.type === 'log') {
      extDebugLog('Received error message, adding to logBatcher', message.payload?.level)
      handleLogMessage(message.payload, sender, message.tabId)
        .then(() => {
          // Async complete, but no response needed
        })
        .catch((err) => {
          console.error('[Gasoline] Failed to handle log message:', err)
        })
      return true // Async handler
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
      handleClearLogs()
        .then(sendResponse)
        .catch((err) => {
          console.error('[Gasoline] Failed to clear logs:', err)
          sendResponse({ error: err.message })
        })
      return true // async response
    } else if (message.type === 'setLogLevel') {
      currentLogLevel = message.level
      chrome.storage.local.set({ logLevel: message.level })
    } else if (message.type === 'setScreenshotOnError') {
      screenshotOnError = message.enabled
      chrome.storage.local.set({ screenshotOnError: message.enabled })
      sendResponse({ success: true })
    } else if (message.type === 'setAiWebPilotEnabled') {
      // Update cache and persist to chrome.storage.local (single source of truth)
      // CRITICAL: Cache is updated AFTER storage write succeeds to prevent race condition
      // on service worker restart. DO NOT move cache update before storage.local.set callback.
      const newValue = message.enabled === true
      const oldValue = _aiWebPilotEnabledCache
      console.log(`[Gasoline] AI Web Pilot toggle: ${oldValue} -> ${newValue}`)

      // Persist to storage, then update cache and POST to server on success
      chrome.storage.local.set({ aiWebPilotEnabled: newValue }, () => {
        _aiWebPilotEnabledCache = newValue
        console.log(`[Gasoline] AI Web Pilot persisted to storage: ${newValue}`)

        // Immediately POST to server (protocol: POST on toggle change)
        // See docs/plugin-server-communications.md
        chrome.storage.local.get(['serverUrl'], (result) => {
          if (result.serverUrl) {
            postSettings(result.serverUrl)
          }
        })
      })

      sendResponse({ success: true })
    } else if (message.type === 'getAiWebPilotEnabled') {
      // Return cached value immediately (cache is always initialized)
      sendResponse({ enabled: _aiWebPilotEnabledCache === true })
    } else if (message.type === 'getDiagnosticState') {
      // Return diagnostic info about current cache and storage state
      chrome.storage.local.get(['aiWebPilotEnabled'], (result) => {
        sendResponse({
          cache: _aiWebPilotEnabledCache,
          storage: result.aiWebPilotEnabled,
          timestamp: new Date().toISOString(),
        })
      })
      return true // Indicate async response
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
      // Broadcast to all content scripts so inject.js can filter captures
      chrome.tabs.query({}, (tabs) => {
        for (const tab of tabs) {
          if (tab.id) {
            chrome.tabs.sendMessage(tab.id, { type: 'setServerUrl', url: serverUrl }).catch(() => {})
          }
        }
      })
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
    // Issue 6 fix: cleanup stale error groups every 10 minutes
    chrome.alarms.create('errorGroupCleanup', { periodInMinutes: 10 })

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
      } else if (alarm.name === 'errorGroupCleanup') {
        // Issue 6 fix: cleanup error groups older than 1 hour
        cleanupStaleErrorGroups()
      }
    })
  }

  // Initial connection check - WAIT for AI Web Pilot cache to initialize first
  // This prevents a race condition where polling would start before storage is loaded
  if (_aiWebPilotCacheInitialized) {
    // Already initialized (shouldn't happen, but handle it)
    checkConnectionAndUpdate()
  } else {
    // Wait for initialization before checking connection
    _pilotInitCallback = checkConnectionAndUpdate
  }

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
  // Also clear tracking if the tracked tab is closed
  if (chrome.tabs && chrome.tabs.onRemoved) {
    chrome.tabs.onRemoved.addListener((tabId) => {
      screenshotTimestamps.delete(tabId)

      // If the tracked tab was closed, clear tracking state
      chrome.storage.local.get(['trackedTabId'], (result) => {
        if (result.trackedTabId === tabId) {
          console.log('[Gasoline] Tracked tab closed (id:', tabId, '), clearing tracking')
          chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
        }
      })
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

// Performance snapshot batcher instance with circuit breaker
const perfBatcherWithCB = createBatcherWithCircuitBreaker(withConnectionStatus(sendPerformanceSnapshotsToServer), {
  debounceMs: 500,
  maxBatchSize: 10,
  sharedCircuitBreaker: sharedServerCircuitBreaker,
})
const perfBatcher = perfBatcherWithCB.batcher

async function handleLogMessage(payload, sender, tabId) {
  if (!shouldCaptureLog(payload.level, currentLogLevel, payload.type)) {
    extDebugLog('Error filtered out:', { level: payload.level, type: payload.type, currentLogLevel })
    debugLog(DebugCategory.CAPTURE, `Log filtered out: level=${payload.level}, type=${payload.type}`)
    return
  }

  let entry = formatLogEntry(payload)

  // Attach tabId so the server can surface which tab produced each log entry.
  // Prefer the explicit tabId from content.js; fall back to sender.tab.id.
  const resolvedTabId = tabId ?? sender?.tab?.id
  if (resolvedTabId !== null && resolvedTabId !== undefined) {
    entry.tabId = resolvedTabId
  }
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
    extDebugLog('Adding to logBatcher:', { level: processedEntry.level, message: processedEntry.message })
    logBatcher.add(processedEntry)
    debugLog(DebugCategory.CAPTURE, `Log queued for server: type=${processedEntry.type}`, {
      aggregatedCount: processedEntry._aggregatedCount,
    })

    // Try to auto-screenshot on error (if enabled)
    maybeAutoScreenshot(processedEntry, sender)
  } else {
    extDebugLog('Log deduplicated (error grouping):', { level: payload.level, message: payload.message })
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

// Issue 5 fix: Mutex flag to prevent multiple simultaneous checkConnectionAndUpdate executions
let _connectionCheckRunning = false

/**
 * Check if a connection check is currently running (for testing)
 * @returns {boolean} True if a connection check is in progress
 */
export function isConnectionCheckRunning() {
  return _connectionCheckRunning
}

async function checkConnectionAndUpdate() {
  // Issue 5 fix: Prevent multiple simultaneous executions (mutex pattern)
  if (_connectionCheckRunning) {
    debugLog(DebugCategory.CONNECTION, 'Skipping connection check - already running')
    return
  }
  _connectionCheckRunning = true

  try {
    const health = await checkServerHealth()
    const wasConnected = connectionStatus.connected
    connectionStatus = {
      ...connectionStatus,
      ...health,
      connected: health.connected,
    }

    // Promote nested log info to top-level for popup display
    if (health.logs) {
      connectionStatus.logFile = health.logs.logFile || connectionStatus.logFile
      connectionStatus.logFileSize = health.logs.logFileSize
      connectionStatus.entries = health.logs.entries ?? connectionStatus.entries
      connectionStatus.maxEntries = health.logs.maxEntries ?? connectionStatus.maxEntries
    }

    // Check version compatibility between extension and server
    if (health.connected && health.version) {
      const extVersion = chrome.runtime.getManifest().version
      const serverMajor = health.version.split('.')[0]
      const extMajor = extVersion.split('.')[0]
      connectionStatus.serverVersion = health.version
      connectionStatus.extensionVersion = extVersion
      connectionStatus.versionMismatch = serverMajor !== extMajor
    }

    updateBadge(connectionStatus)

    // Log connection status changes
    if (wasConnected !== health.connected) {
      debugLog(DebugCategory.CONNECTION, health.connected ? 'Connected to server' : 'Disconnected from server', {
        entries: connectionStatus.entries,
        error: health.error || null,
        serverVersion: health.version || null,
      })
    }

    // Poll capture settings when connected
    if (health.connected) {
      const overrides = await pollCaptureSettings(serverUrl)
      if (overrides !== null) {
        applyCaptureOverrides(overrides)
      }

      // ⚠️ CRITICAL BUG FIX: Service Worker Restart Race Condition
      //
      // When service worker restarts (Chrome suspends/resumes background script),
      // the cache initialization from chrome.storage.local.get() is async via callback.
      // If polling starts before the callback fires, first polls report the wrong state!
      //
      // SYMPTOM: Toggle shows "on" in popup, but server sees "pilot_enabled: false"
      // Start polling for pending queries (execute_javascript, highlight, etc.)
      startQueryPolling(serverUrl)
      // Start settings heartbeat (POST /settings every 2s)
      // See docs/plugin-server-communications.md
      startSettingsHeartbeat(serverUrl)
      // Start network waterfall posting (POST /network-waterfall every 10s)
      startWaterfallPosting(serverUrl)
      // Start extension logs posting (POST /extension-logs every 5s)
      startExtensionLogsPosting(serverUrl)
      // Start status ping (POST /api/extension-status every 30s)
      startStatusPing(serverUrl)
    } else {
      // Stop polling when disconnected
      stopQueryPolling()
      stopSettingsHeartbeat()
      stopWaterfallPosting()
      stopExtensionLogsPosting()
      stopStatusPing()
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
  } finally {
    // Issue 5 fix: Release mutex
    // eslint-disable-next-line require-atomic-updates -- mutex pattern: intentional single-point release
    _connectionCheckRunning = false
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
// Track queries currently being processed to prevent duplicate processing
// Changed from Set to Map to store timestamps for TTL-based cleanup (Issue 1 fix)
const _processingQueries = new Map() // queryId -> timestamp

// TTL for processing queries (60 seconds) - entries older than this are considered stale
const PROCESSING_QUERY_TTL_MS = 60000

/**
 * Get current state of processing queries (for testing)
 * @returns {Map} Current processing queries map
 */
export function getProcessingQueriesState() {
  return _processingQueries
}

/**
 * Add a query to the processing set with timestamp
 * @param {string} queryId - The query ID
 * @param {number} [timestamp] - Optional timestamp (defaults to now)
 */
export function addProcessingQuery(queryId, timestamp = Date.now()) {
  _processingQueries.set(queryId, timestamp)
}

/**
 * Remove a query from the processing set
 * @param {string} queryId - The query ID
 */
export function removeProcessingQuery(queryId) {
  _processingQueries.delete(queryId)
}

/**
 * Check if a query is currently being processed
 * @param {string} queryId - The query ID
 * @returns {boolean} True if query is being processed
 */
export function isQueryProcessing(queryId) {
  return _processingQueries.has(queryId)
}

/**
 * Clean up stale processing queries that have exceeded the TTL.
 * Called on each poll cycle to prevent unbounded growth.
 */
export function cleanupStaleProcessingQueries() {
  const now = Date.now()
  for (const [queryId, timestamp] of _processingQueries) {
    if (now - timestamp > PROCESSING_QUERY_TTL_MS) {
      _processingQueries.delete(queryId)
      debugLog(DebugCategory.CONNECTION, 'Cleaned up stale processing query', {
        queryId,
        age: Math.round((now - timestamp) / 1000) + 's',
      })
    }
  }
}

/**
 * Poll the server for pending queries (DOM queries, a11y audits)
 * @param {string} serverUrl - The server base URL
 */
export async function pollPendingQueries(serverUrl) {
  try {
    // Clean up stale processing queries at the start of each poll cycle (Issue 1 fix)
    cleanupStaleProcessingQueries()

    // Determine pilot state for header (cache is always initialized)
    const pilotState = _aiWebPilotEnabledCache === true ? '1' : '0'

    // DIAGNOSTIC: Log every poll request with cache state
    diagnosticLog(`[Diagnostic] Poll request: cache=${_aiWebPilotEnabledCache}, header=${pilotState}`)

    const response = await fetch(`${serverUrl}/pending-queries`, {
      headers: {
        'X-Gasoline-Session': EXTENSION_SESSION_ID,
        // DEPRECATED: X-Gasoline-Pilot header kept for backward compatibility only
        // Server now uses POST /settings for pilot state (see docs/plugin-server-communications.md)
        'X-Gasoline-Pilot': pilotState,
      },
    })
    if (!response.ok) {
      debugLog(DebugCategory.CONNECTION, 'Poll pending-queries failed', { status: response.status })
      return
    }

    const data = await response.json()
    if (!data.queries || data.queries.length === 0) return

    debugLog(DebugCategory.CONNECTION, 'Got pending queries', { count: data.queries.length })
    for (const query of data.queries) {
      // Skip queries already being processed (using Map for TTL tracking)
      if (_processingQueries.has(query.id)) {
        debugLog(DebugCategory.CONNECTION, 'Skipping already processing query', { id: query.id })
        continue
      }
      // Mark as processing with timestamp (Issue 1 fix: track time for TTL cleanup)
      _processingQueries.set(query.id, Date.now())
      try {
        await handlePendingQuery(query, serverUrl)
      } finally {
        // Remove from processing map when done
        _processingQueries.delete(query.id)
      }
    }
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Poll pending-queries error', { error: err.message })
  }
}

/**
 * Ping content script to check if it's loaded
 * @param {number} tabId - The tab ID to ping
 * @param {number} timeoutMs - Timeout in milliseconds
 * @returns {Promise<boolean>} True if content script responds
 */
async function pingContentScript(tabId, timeoutMs = 500) {
  try {
    const response = await Promise.race([
      chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_PING' }),
      new Promise((_, reject) => {
        setTimeout(() => reject(new Error('timeout')), timeoutMs)
      }),
    ])
    return response?.status === 'alive'
  } catch {
    return false
  }
}

/**
 * Wait for tab to finish loading
 * @param {number} tabId - The tab ID to wait for
 * @param {number} timeoutMs - Maximum wait time
 * @returns {Promise<boolean>} True if tab loaded successfully
 */
async function waitForTabLoad(tabId, timeoutMs = 5000) {
  const startTime = Date.now()
  while (Date.now() - startTime < timeoutMs) {
    try {
      const tab = await chrome.tabs.get(tabId)
      if (tab.status === 'complete') return true
    } catch {
      return false
    }
    await new Promise((r) => {
      setTimeout(r, 100)
    })
  }
  return false
}

/**
 * Handle browser action commands (refresh, navigate, go back/forward).
 * These run in the background script and don't require content scripts.
 * For navigate: auto-detects if content script is loaded and refreshes if needed.
 * @param {number} tabId - The tab ID to act on
 * @param {Object} params - { action, url? }
 * @returns {Promise<Object>} Result with success/error
 */
async function handleBrowserAction(tabId, params) {
  const { action, url } = params || {}

  // Check AI Web Pilot toggle for safety
  const enabled = await isAiWebPilotEnabled()
  if (!enabled) {
    return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' }
  }

  try {
    switch (action) {
      case 'refresh':
        await chrome.tabs.reload(tabId)
        await waitForTabLoad(tabId)
        return { success: true, action: 'refresh' }

      case 'navigate': {
        if (!url) {
          return { success: false, error: 'missing_url', message: 'URL required for navigate action' }
        }

        // Check for restricted URLs
        if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
          return {
            success: false,
            error: 'restricted_url',
            message: 'Cannot navigate to Chrome internal pages',
          }
        }

        // Navigate to the URL
        await chrome.tabs.update(tabId, { url })

        // Wait for page to load
        await waitForTabLoad(tabId)

        // Brief delay for content script initialization
        await new Promise((r) => {
          setTimeout(r, 500)
        })

        // Check if content script is loaded
        const contentScriptLoaded = await pingContentScript(tabId)

        if (contentScriptLoaded) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'loaded',
            message: 'Content script ready',
          }
        }

        // Content script not loaded - check if it's a file:// URL
        const tab = await chrome.tabs.get(tabId)
        if (tab.url?.startsWith('file://')) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'unavailable',
            message:
              'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings (chrome://extensions).',
          }
        }

        // Auto-refresh to inject content script
        debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url })
        await chrome.tabs.reload(tabId)
        await waitForTabLoad(tabId)

        // Wait for content script after refresh
        await new Promise((r) => {
          setTimeout(r, 1000)
        })

        // Check again
        const loadedAfterRefresh = await pingContentScript(tabId)

        if (loadedAfterRefresh) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'refreshed',
            message: 'Page refreshed to load content script',
          }
        }

        // Still not loaded - return with warning
        return {
          success: true,
          action: 'navigate',
          url,
          content_script_status: 'failed',
          message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.',
        }
      }

      case 'back':
        await chrome.tabs.goBack(tabId)
        return { success: true, action: 'back' }

      case 'forward':
        await chrome.tabs.goForward(tabId)
        return { success: true, action: 'forward' }

      default:
        return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` }
    }
  } catch (err) {
    return { success: false, error: 'browser_action_failed', message: err.message }
  }
}

/**
 * Handle a single pending query by dispatching to the active tab
 * @param {Object} query - { id, type, params }
 * @param {string} serverUrl - The server base URL
 */
export async function handlePendingQuery(query, serverUrl) {
  try {
    // Handle state management queries locally (in background script)
    if (query.type.startsWith('state_')) {
      await handleStateQuery(query, serverUrl)
      return
    }

    // Check if we're tracking a specific tab
    const storage = await chrome.storage.local.get(['trackedTabId'])
    let tabs
    let tabId

    if (storage.trackedTabId) {
      // Use the tracked tab
      diagnosticLog(`[Diagnostic] Using tracked tab ${storage.trackedTabId} for query ${query.type}`)
      try {
        const trackedTab = await chrome.tabs.get(storage.trackedTabId)
        tabs = [trackedTab]
        tabId = storage.trackedTabId
      } catch {
        // Tracked tab no longer exists - clear tracking and fall back to active tab
        diagnosticLog(`[Diagnostic] Tracked tab ${storage.trackedTabId} no longer exists, clearing tracking`)
        await chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
        tabs = await new Promise((resolve) => {
          chrome.tabs.query({ active: true, currentWindow: true }, resolve)
        })
        if (!tabs || tabs.length === 0) return
        tabId = tabs[0].id
      }
    } else {
      // No tracking - use active tab
      tabs = await new Promise((resolve) => {
        chrome.tabs.query({ active: true, currentWindow: true }, resolve)
      })
      if (!tabs || tabs.length === 0) return
      tabId = tabs[0].id
    }

    // Handle browser action queries (refresh, navigate, etc.)
    if (query.type === 'browser_action') {
      const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params

      // ASYNC COMMAND EXECUTION: If query has correlation_id, use async pattern.
      // MUST await — without it, _processingQueries.delete() fires immediately
      // while the action is still running (10s+), causing duplicate processing
      // and timeouts after 5-6 operations (Issue #5).
      if (query.correlation_id) {
        await handleAsyncBrowserAction(query, tabId, params, serverUrl)
        return
      }

      // LEGACY SYNC PATH: No correlation_id, use old synchronous behavior
      const result = await handleBrowserAction(tabId, params)
      await postQueryResult(serverUrl, query.id, 'browser_action', result)
      return
    }

    // Handle highlight queries via the pilot command system
    if (query.type === 'highlight') {
      const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params)
      // Post result back to server
      await postQueryResult(serverUrl, query.id, 'highlight', result)
      return
    }

    // Handle page_info queries - get page info from tab
    if (query.type === 'page_info') {
      const tab = tabs[0]
      const result = {
        url: tab.url,
        title: tab.title,
        favicon: tab.favIconUrl,
        status: tab.status,
        viewport: {
          width: tab.width,
          height: tab.height,
        },
      }
      await postQueryResult(serverUrl, query.id, 'page_info', result)
      return
    }

    // ============================================================================
    // BUGFIX 2026-01-26: Handle tabs queries - get all browser tabs
    // ============================================================================
    // PROBLEM: Server sent 'tabs' queries but extension had no handler, causing
    // MCP tools to timeout with "extension_timeout" error. Extension was connected
    // and polling but never responded to observe({what: "tabs"}) requests.
    //
    // SOLUTION: Added explicit handler that queries all tabs and posts result back
    // to server via postQueryResult(). Critical: MUST call postQueryResult() or
    // server will timeout waiting for response.
    //
    // DO NOT CHANGE: This handler is required for observe({what: "tabs"}) to work.
    // ============================================================================
    if (query.type === 'tabs') {
      const allTabs = await new Promise((resolve) => {
        chrome.tabs.query({}, resolve)
      })
      const tabsList = allTabs.map((tab) => ({
        id: tab.id,
        url: tab.url,
        title: tab.title,
        active: tab.active,
        windowId: tab.windowId,
        index: tab.index,
      }))
      await postQueryResult(serverUrl, query.id, 'dom', { tabs: tabsList })
      return
    }

    // Handle dom queries - forward to content script for DOM query execution
    if (query.type === 'dom') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'DOM_QUERY',
          params: query.params,
        })
        await postQueryResult(serverUrl, query.id, 'dom', result)
      } catch (err) {
        await postQueryResult(serverUrl, query.id, 'dom', {
          error: 'dom_query_failed',
          message: err.message || 'Failed to execute DOM query',
        })
      }
      return
    }

    // Handle a11y queries - forward to content script for axe-core audit
    if (query.type === 'a11y') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'A11Y_QUERY',
          params: query.params,
        })
        await postQueryResult(serverUrl, query.id, 'a11y', result)
      } catch (err) {
        await postQueryResult(serverUrl, query.id, 'a11y', {
          error: 'a11y_audit_failed',
          message: err.message || 'Failed to execute accessibility audit',
        })
      }
      return
    }

    // Handle execute queries - for AI Web Pilot commands
    if (query.type === 'execute') {
      const enabled = await isAiWebPilotEnabled()
      if (!enabled) {
        // ASYNC: If query has correlation_id, use async result posting
        if (query.correlation_id) {
          await postAsyncCommandResult(serverUrl, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled')
        } else {
          await postQueryResult(serverUrl, query.id, 'execute', {
            success: false,
            error: 'ai_web_pilot_disabled',
            message: 'AI Web Pilot is not enabled in the extension popup',
          })
        }
        return
      }

      // ASYNC COMMAND EXECUTION (v6.0.0)
      // If query has correlation_id, use async pattern with 2s/10s timeouts
      // MUST await — without it, _processingQueries.delete() fires immediately
      // while the action is still running (10s+), causing duplicate processing
      // and timeouts after 5-6 operations (Issue #5).
      if (query.correlation_id) {
        await handleAsyncExecuteCommand(query, tabId, serverUrl)
        return
      }

      // LEGACY SYNC PATH: No correlation_id, use old synchronous behavior
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'GASOLINE_EXECUTE_QUERY',
          queryId: query.id,
          params: query.params,
        })
        await postQueryResult(serverUrl, query.id, 'execute', result)
      } catch (err) {
        let message = err.message || 'Tab communication failed'
        if (message.includes('Receiving end does not exist')) {
          message = 'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({action: "refresh"})\n\nThen retry your command.'
        }
        await postQueryResult(serverUrl, query.id, 'execute', {
          success: false,
          error: 'content_script_not_loaded',
          message,
        })
      }
      return
    }
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
      type: query.type,
      id: query.id,
      error: err.message,
    })
  }
}

/**
 * Handle state management queries (save, load, list, delete).
 * These queries manage browser state snapshots.
 * @param {Object} query - { id, type, params }
 * @param {string} serverUrl - The server base URL
 */
async function handleStateQuery(query, serverUrl) {
  // Check if AI Web Pilot is enabled
  const enabled = await isAiWebPilotEnabled()
  if (!enabled) {
    await postQueryResult(serverUrl, query.id, 'state', { error: 'ai_web_pilot_disabled' })
    return
  }

  const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
  const action = params.action

  try {
    let result

    switch (action) {
      case 'capture': {
        // Capture current state from the active tab without saving
        const tabs = await new Promise((resolve) => {
          chrome.tabs.query({ active: true, currentWindow: true }, resolve)
        })

        if (!tabs || tabs.length === 0) {
          await postQueryResult(serverUrl, query.id, 'state', { error: 'no_active_tab' })
          return
        }

        // Forward to content script to capture state
        const captureResult = await chrome.tabs.sendMessage(tabs[0].id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' },
        })

        result = captureResult
        break
      }

      case 'save': {
        // First capture state from the active tab
        const tabs = await new Promise((resolve) => {
          chrome.tabs.query({ active: true, currentWindow: true }, resolve)
        })

        if (!tabs || tabs.length === 0) {
          await postQueryResult(serverUrl, query.id, 'state', { error: 'no_active_tab' })
          return
        }

        // Forward to content script to capture state
        const captureResult = await chrome.tabs.sendMessage(tabs[0].id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' },
        })

        if (captureResult.error) {
          await postQueryResult(serverUrl, query.id, 'state', { error: captureResult.error })
          return
        }

        // Save the captured state
        result = await saveStateSnapshot(params.name, captureResult)
        break
      }

      case 'load': {
        const snapshot = await loadStateSnapshot(params.name)
        if (!snapshot) {
          await postQueryResult(serverUrl, query.id, 'state', { error: `Snapshot '${params.name}' not found` })
          return
        }

        // Forward to content script to restore state
        const tabs = await new Promise((resolve) => {
          chrome.tabs.query({ active: true, currentWindow: true }, resolve)
        })

        if (!tabs || tabs.length === 0) {
          await postQueryResult(serverUrl, query.id, 'state', { error: 'no_active_tab' })
          return
        }

        const restoreResult = await chrome.tabs.sendMessage(tabs[0].id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: {
            action: 'restore',
            state: snapshot,
            include_url: params.include_url !== false,
          },
        })

        result = restoreResult
        break
      }

      case 'list':
        result = { snapshots: await listStateSnapshots() }
        break

      case 'delete':
        result = await deleteStateSnapshot(params.name)
        break

      default:
        result = { error: `Unknown action: ${action}` }
    }

    await postQueryResult(serverUrl, query.id, 'state', result)
  } catch (err) {
    await postQueryResult(serverUrl, query.id, 'state', { error: err.message })
  }
}

/**
 * Post query results back to the server
 * @param {string} serverUrl - The server base URL
 * @param {string} queryId - The query ID
 * @param {string} type - Query type ('dom', 'a11y', 'highlight', 'state', or 'execute')
 * @param {Object} result - The query result
 */
export async function postQueryResult(serverUrl, queryId, type, result) {
  let endpoint
  if (type === 'a11y') {
    endpoint = '/a11y-result'
  } else if (type === 'state') {
    endpoint = '/state-result'
  } else if (type === 'highlight') {
    endpoint = '/highlight-result'
  } else if (type === 'execute' || type === 'browser_action') {
    endpoint = '/execute-result'
  } else {
    endpoint = '/dom-result'
  }

  await fetch(`${serverUrl}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: queryId, result }),
  })
}

/**
 * POST async command result to server using correlation_id (v6.0.0)
 * @param {string} serverUrl - The server base URL
 * @param {string} correlationId - Correlation ID for async command tracking
 * @param {string} status - "pending", "complete", or "timeout"
 * @param {any} result - Command result (if complete)
 * @param {string} error - Error message (if failed)
 */
async function postAsyncCommandResult(serverUrl, correlationId, status, result = null, error = null) {
  const payload = {
    correlation_id: correlationId,
    status: status,
  }
  if (result !== null) {
    payload.result = result
  }
  if (error !== null) {
    payload.error = error
  }

  try {
    await fetch(`${serverUrl}/execute-result`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Failed to post async command result', {
      correlationId,
      status,
      error: err.message,
    })
  }
}

/**
 * Handle async execute command with 2s/10s timeout pattern (v6.0.0)
 * @param {Object} query - Query object with correlation_id
 * @param {number} tabId - Target tab ID
 * @param {string} serverUrl - Server base URL
 */
async function handleAsyncExecuteCommand(query, tabId, serverUrl) {
  const startTime = Date.now()
  let completed = false
  let pendingPosted = false

  // Start command execution
  const executionPromise = chrome.tabs
    .sendMessage(tabId, {
      type: 'GASOLINE_EXECUTE_QUERY',
      queryId: query.id,
      params: query.params,
    })
    .then((result) => {
      completed = true
      return { success: true, result }
    })
    .catch((err) => {
      completed = true
      let message = err.message || 'Tab communication failed'
      if (message.includes('Receiving end does not exist')) {
        message = 'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({action: "refresh"})\n\nThen retry your command.'
      }
      return {
        success: false,
        error: 'content_script_not_loaded',
        message,
      }
    })

  // 2s decision point: return result or post "pending" status
  const twoSecondTimer = setTimeout(async () => {
    if (!completed && !pendingPosted) {
      pendingPosted = true
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'pending')
      debugLog(DebugCategory.CONNECTION, 'Posted pending status for async command', {
        correlationId: query.correlation_id,
        elapsed: Date.now() - startTime,
      })
    }
  }, 2000)

  // Wait for execution (up to 10s total timeout)
  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), 10000)
      }),
    ])

    clearTimeout(twoSecondTimer)

    // Post final result
    if (execResult.success) {
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'complete', execResult.result)
    } else {
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'complete', null, execResult.error || execResult.message)
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async command', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success,
    })
  } catch {
    clearTimeout(twoSecondTimer)

    // Post timeout error with actionable guidance
    const timeoutMessage = `JavaScript execution exceeded 10s timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < 2s for best results
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope

Example: Instead of processing 1000 elements at once, process 100 at a time.`

    await postAsyncCommandResult(serverUrl, query.correlation_id, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    })
  }
}

/**
 * Handle browser action with async pattern (2s/10s timeouts)
 * @param {Object} query - The query object with correlation_id
 * @param {number} tabId - The tab ID to execute in
 * @param {Object} params - The browser action parameters
 * @param {string} serverUrl - The server base URL
 */
async function handleAsyncBrowserAction(query, tabId, params, serverUrl) {
  const startTime = Date.now()
  let completed = false
  let pendingPosted = false

  // Start command execution
  const executionPromise = handleBrowserAction(tabId, params)
    .then((result) => {
      completed = true
      return result
    })
    .catch((err) => {
      completed = true
      return {
        success: false,
        error: err.message || 'Browser action failed',
      }
    })

  // 2s decision point: return result or post "pending" status
  const twoSecondTimer = setTimeout(async () => {
    if (!completed && !pendingPosted) {
      pendingPosted = true
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'pending')
      debugLog(DebugCategory.CONNECTION, 'Posted pending status for async browser action', {
        correlationId: query.correlation_id,
        elapsed: Date.now() - startTime,
      })
    }
  }, 2000)

  // Wait for execution (up to 10s total timeout)
  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), 10000)
      }),
    ])

    clearTimeout(twoSecondTimer)

    // Post final result
    if (execResult.success !== false) {
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'complete', execResult)
    } else {
      await postAsyncCommandResult(serverUrl, query.correlation_id, 'complete', null, execResult.error)
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success !== false,
    })
  } catch {
    clearTimeout(twoSecondTimer)

    // Post timeout error with diagnostic guidance
    const timeoutMessage = `Browser action exceeded 10s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network', status_min: 400})

Possible causes: slow page load, navigation blocked by popup, network issues, or page JavaScript errors.`

    await postAsyncCommandResult(serverUrl, query.correlation_id, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    })
  }
}

/**
 * POST current settings to the server
 * Implements the protocol documented in docs/plugin-server-communications.md
 * @param {string} serverUrl - The server base URL
 */
export async function postSettings(serverUrl) {
  // Only send if cache initialized (don't send null/undefined values)
  if (!_aiWebPilotCacheInitialized) {
    debugLog(DebugCategory.CONNECTION, 'Skipping settings POST: cache not initialized')
    return
  }

  try {
    // Read all config toggles from storage for diagnostics
    const result = await chrome.storage.local.get([
      'aiWebPilotEnabled',
      'webSocketCaptureEnabled',
      'networkWaterfallEnabled',
      'performanceMarksEnabled',
      'actionReplayEnabled',
      'screenshotOnError',
      'sourceMapEnabled',
      'networkBodyCaptureEnabled',
    ])

    // Build settings object with all available config values
    const settings = {}

    // Include all settings that are defined (even if false)
    if (result.aiWebPilotEnabled !== null && result.aiWebPilotEnabled !== undefined) {
      settings.aiWebPilotEnabled = result.aiWebPilotEnabled
    } else if (_aiWebPilotEnabledCache !== null && _aiWebPilotEnabledCache !== undefined) {
      // Fallback to cache for aiWebPilot (already tracked separately)
      settings.aiWebPilotEnabled = _aiWebPilotEnabledCache
    }

    if (result.webSocketCaptureEnabled !== null && result.webSocketCaptureEnabled !== undefined) {
      settings.webSocketCaptureEnabled = result.webSocketCaptureEnabled
    }

    if (result.networkWaterfallEnabled !== null && result.networkWaterfallEnabled !== undefined) {
      settings.networkWaterfallEnabled = result.networkWaterfallEnabled
    }

    if (result.performanceMarksEnabled !== null && result.performanceMarksEnabled !== undefined) {
      settings.performanceMarksEnabled = result.performanceMarksEnabled
    }

    if (result.actionReplayEnabled !== null && result.actionReplayEnabled !== undefined) {
      settings.actionReplayEnabled = result.actionReplayEnabled
    }

    if (result.screenshotOnError !== null && result.screenshotOnError !== undefined) {
      settings.screenshotOnError = result.screenshotOnError
    }

    if (result.sourceMapEnabled !== null && result.sourceMapEnabled !== undefined) {
      settings.sourceMapEnabled = result.sourceMapEnabled
    }

    if (result.networkBodyCaptureEnabled !== null && result.networkBodyCaptureEnabled !== undefined) {
      settings.networkBodyCaptureEnabled = result.networkBodyCaptureEnabled
    }

    await fetch(`${serverUrl}/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        session_id: EXTENSION_SESSION_ID,
        settings: settings,
      }),
    })
    debugLog(DebugCategory.CONNECTION, 'Posted settings to server', settings)
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Failed to post settings', { error: err.message })
  }
}

// Settings heartbeat interval (POST /settings every 2 seconds)
let settingsHeartbeatInterval = null

/**
 * Start settings heartbeat: POST /settings every 2 seconds
 * @param {string} serverUrl - The server base URL
 */
export function startSettingsHeartbeat(serverUrl) {
  stopSettingsHeartbeat()
  debugLog(DebugCategory.CONNECTION, 'Starting settings heartbeat', { serverUrl })
  // Post immediately, then every 2 seconds
  postSettings(serverUrl)
  settingsHeartbeatInterval = setInterval(() => postSettings(serverUrl), 2000)
}

/**
 * Stop settings heartbeat
 */
export function stopSettingsHeartbeat() {
  if (settingsHeartbeatInterval) {
    clearInterval(settingsHeartbeatInterval)
    settingsHeartbeatInterval = null
    debugLog(DebugCategory.CONNECTION, 'Stopped settings heartbeat')
  }
}

// ============================================================================
// EXTENSION STATUS PING (Tracking State)
// ============================================================================

const STATUS_PING_INTERVAL = 30000 // 30 seconds

/**
 * Send a status ping to the server with current tracking state.
 * This tells the server (and LLM) whether tab tracking is enabled,
 * which tab is being tracked, and the extension connection status.
 */
async function sendStatusPing() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'])

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      message: storage.trackedTabId ? 'tracking enabled' : 'no tab tracking enabled',
      extension_connected: true,
      timestamp: new Date().toISOString(),
    }

    await fetch(`${serverUrl}/api/extension-status`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(statusMessage),
    })
  } catch (err) {
    // Silent failure - status ping is non-critical
    diagnosticLog('[Gasoline] Status ping error: ' + err.message)
  }
}

// Status ping interval
let statusPingInterval = null

/**
 * Start status ping: POST /api/extension-status every 30 seconds
 * @param {string} serverUrl - The server base URL
 */
export function startStatusPing(_serverUrl) {
  stopStatusPing()
  sendStatusPing() // Send immediately on start
  statusPingInterval = setInterval(() => sendStatusPing(), STATUS_PING_INTERVAL)
}

/**
 * Stop status ping
 */
export function stopStatusPing() {
  if (statusPingInterval) {
    clearInterval(statusPingInterval)
    statusPingInterval = null
  }
}

// Also send immediate status ping when tracking changes
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    sendStatusPing()
  }
})

/**
 * Post queued extension logs to server
 * @param {string} serverUrl - The server base URL
 */
async function postExtensionLogs(serverUrl) {
  if (extensionLogQueue.length === 0) return

  // Drain queue (take all logs)
  const logsToSend = extensionLogQueue.splice(0)

  try {
    await fetch(`${serverUrl}/extension-logs`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ logs: logsToSend }),
    })
  } catch (err) {
    // Silent failure - don't create infinite loop
    console.error('[Gasoline] Failed to post extension logs', err)
  }
}

// Extension logs posting interval (POST /extension-logs every 5 seconds)
let extensionLogsInterval = null

/**
 * Start extension logs posting: POST /extension-logs every 5 seconds
 * @param {string} serverUrl - The server base URL
 */
export function startExtensionLogsPosting(serverUrl) {
  stopExtensionLogsPosting()
  // Post every 5 seconds (batch logs to reduce overhead)
  extensionLogsInterval = setInterval(() => postExtensionLogs(serverUrl), 5000)
}

/**
 * Stop extension logs posting
 */
export function stopExtensionLogsPosting() {
  if (extensionLogsInterval) {
    clearInterval(extensionLogsInterval)
    extensionLogsInterval = null
  }
}

/**
 * Start polling for pending queries at 1-second intervals
 * @param {string} serverUrl - The server base URL
 */
export function startQueryPolling(serverUrl) {
  stopQueryPolling()
  debugLog(DebugCategory.CONNECTION, 'Starting query polling', { serverUrl })
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

/**
 * Network waterfall posting interval (every 10 seconds)
 */
let waterfallPostingInterval = null

/**
 * Post network waterfall data to server (collects PerformanceResourceTiming data)
 * @param {string} serverUrl - The server base URL
 */
async function postNetworkWaterfall(_serverUrl) {
  try {
    // Query active tab for waterfall data
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    if (!tabs || tabs.length === 0) return

    const tabId = tabs[0].id
    const pageURL = tabs[0].url

    // Request waterfall data from content script
    const result = await chrome.tabs.sendMessage(tabId, {
      type: 'GET_NETWORK_WATERFALL',
    })

    if (!result || !result.entries || result.entries.length === 0) {
      debugLog(DebugCategory.CAPTURE, 'No waterfall entries to send')
      return
    }

    // Send to server
    await sendNetworkWaterfallToServer({
      entries: result.entries,
      pageURL: pageURL,
    })
  } catch (err) {
    // Silently fail - tab may have closed or content script not ready
    debugLog(DebugCategory.CAPTURE, 'Failed to post waterfall', { error: err.message })
  }
}

/**
 * Start network waterfall posting: POST /network-waterfall every 10 seconds
 * @param {string} serverUrl - The server base URL
 */
export function startWaterfallPosting(serverUrl) {
  stopWaterfallPosting()
  debugLog(DebugCategory.CONNECTION, 'Starting waterfall posting', { serverUrl })
  // Post immediately, then every 10 seconds
  postNetworkWaterfall(serverUrl)
  waterfallPostingInterval = setInterval(() => postNetworkWaterfall(serverUrl), 10000)
}

/**
 * Stop network waterfall posting
 */
export function stopWaterfallPosting() {
  if (waterfallPostingInterval) {
    clearInterval(waterfallPostingInterval)
    waterfallPostingInterval = null
    debugLog(DebugCategory.CONNECTION, 'Stopped waterfall posting')
  }
}

// =============================================================================
// AI WEB PILOT SAFETY GATE
// =============================================================================

/**
 * STORAGE CONSISTENCY LISTENER — Critical for maintaining cache accuracy
 * This listener is the failsafe that keeps _aiWebPilotEnabledCache in sync
 * with the underlying storage. If storage ever changes (e.g., external update
 * or content script indirect write), this listener ensures the cache reflects it.
 *
 * WHY THIS EXISTS: Although background.js is the only writer (via
 * setAiWebPilotEnabled), external events (content scripts, tests, other
 * extensions) might update storage. This listener guarantees cache consistency.
 */
if (typeof chrome !== 'undefined' && chrome.storage) {
  chrome.storage.onChanged.addListener((changes, areaName) => {
    if (areaName === 'local' && changes.aiWebPilotEnabled) {
      _aiWebPilotEnabledCache = changes.aiWebPilotEnabled.newValue === true
      console.log('[Gasoline] AI Web Pilot cache updated from storage:', _aiWebPilotEnabledCache)
    }
  })
}

/**
 * Check if AI Web Pilot is enabled.
 * Returns the cached value (defaults to false if not set in storage).
 * @returns {boolean} True if AI Web Pilot is enabled
 */
export function isAiWebPilotEnabled() {
  return _aiWebPilotEnabledCache === true
}

/**
 * Handle pilot commands (GASOLINE_HIGHLIGHT, GASOLINE_MANAGE_STATE, GASOLINE_EXECUTE_JS).
 *
 * CRITICAL: This function checks _aiWebPilotEnabledCache (via isAiWebPilotEnabled()).
 * The cache is the source of truth. If cache != storage, this is caught by the
 * fail-safe below and corrected.
 *
 * State checking flow:
 * 1. Check cache via isAiWebPilotEnabled()
 * 2. If cache says false, verify with storage as fail-safe
 * 3. If storage says true but cache says false: fix cache and proceed
 *    (This shouldn't happen with proper message handling, but it's a safety net)
 * 4. If both agree it's false: return error
 *
 * @param {string} command - The pilot command type
 * @param {Object} params - Command parameters
 * @returns {Promise<Object>} Result or error response
 */
export async function handlePilotCommand(command, params) {
  // Check if AI Web Pilot is enabled (reads from cache, not storage)
  let enabled = await isAiWebPilotEnabled()

  // Fail-safe: if cache says false, double-check storage in case of sync issues
  // This catches the case where cache got out of sync from storage
  if (!enabled) {
    const localResult = await new Promise((resolve) => {
      chrome.storage.local.get(['aiWebPilotEnabled'], resolve)
    })
    if (localResult.aiWebPilotEnabled === true) {
      // Storage says true but cache said false - cache was out of sync
      // Fix it and proceed (this should be rare if message handling is correct)
      _aiWebPilotEnabledCache = true
      enabled = true
      extDebugLog('[Gasoline] Cache/storage mismatch: corrected cache from storage')
    }
  }

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

// =============================================================================
// AI WEB PILOT: STATE SNAPSHOT STORAGE
// =============================================================================

const SNAPSHOT_KEY = 'gasoline_state_snapshots'

/**
 * Save a state snapshot to chrome.storage.local.
 * @param {string} name - Snapshot name
 * @param {Object} state - State object from captureState()
 * @returns {Promise<Object>} Result with success, snapshot_name, size_bytes
 */
export async function saveStateSnapshot(name, state) {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
      // eslint-disable-next-line security/detect-object-injection -- SNAPSHOT_KEY is a constant defined in this module
      const snapshots = result[SNAPSHOT_KEY] || {}
      // eslint-disable-next-line security/detect-object-injection -- name is validated snapshot identifier
      snapshots[name] = {
        ...state,
        name,
        size_bytes: JSON.stringify(state).length,
      }
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({
          success: true,
          snapshot_name: name,
          // eslint-disable-next-line security/detect-object-injection -- name is the key we just set above
          size_bytes: snapshots[name].size_bytes,
        })
      })
    })
  })
}

/**
 * Load a state snapshot from chrome.storage.local.
 * @param {string} name - Snapshot name
 * @returns {Promise<Object|null>} Snapshot or null if not found
 */
export async function loadStateSnapshot(name) {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
      // eslint-disable-next-line security/detect-object-injection -- SNAPSHOT_KEY is a constant defined in this module
      const snapshots = result[SNAPSHOT_KEY] || {}
      // eslint-disable-next-line security/detect-object-injection -- name is validated snapshot identifier
      resolve(snapshots[name] || null)
    })
  })
}

/**
 * List all state snapshots with metadata.
 * @returns {Promise<Array>} Array of snapshot metadata objects
 */
export async function listStateSnapshots() {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
      // eslint-disable-next-line security/detect-object-injection -- SNAPSHOT_KEY is a constant defined in this module
      const snapshots = result[SNAPSHOT_KEY] || {}
      const list = Object.values(snapshots).map((s) => ({
        name: s.name,
        url: s.url,
        timestamp: s.timestamp,
        size_bytes: s.size_bytes,
      }))
      resolve(list)
    })
  })
}

/**
 * Delete a state snapshot from chrome.storage.local.
 * @param {string} name - Snapshot name
 * @returns {Promise<Object>} Result with success and deleted name
 */
export async function deleteStateSnapshot(name) {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result) => {
      // eslint-disable-next-line security/detect-object-injection -- SNAPSHOT_KEY is a constant defined in this module
      const snapshots = result[SNAPSHOT_KEY] || {}
      // eslint-disable-next-line security/detect-object-injection -- name is validated snapshot identifier
      delete snapshots[name]
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({ success: true, deleted: name })
      })
    })
  })
}
