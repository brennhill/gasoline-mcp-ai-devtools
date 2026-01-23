/**
 * @fileoverview Background service worker for Dev Console extension
 * Handles log batching, server communication, and badge updates
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
let errorGroupFlushTimer = null

// Rate limiting state
const screenshotTimestamps = new Map() // tabId -> [timestamps]

// Source map state
const sourceMapCache = new Map() // scriptUrl -> { mappings, sources, names, sourceRoot }
let sourceMapEnabled = false // Source map resolution (off by default)

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
  contextExcessiveTimestamps = contextExcessiveTimestamps.filter(
    (t) => now - t.ts < CONTEXT_WARNING_WINDOW_MS
  )

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
  return JSON.stringify({
    exportedAt: new Date().toISOString(),
    version: '3.0.0',
    debugMode,
    connectionStatus,
    settings: {
      logLevel: currentLogLevel,
      screenshotOnError,
      sourceMapEnabled,
    },
    entries: debugLogBuffer,
  }, null, 2)
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
export function createLogBatcher(flushFn, options = {}) {
  const debounceMs = options.debounceMs ?? DEFAULT_DEBOUNCE_MS
  const maxBatchSize = options.maxBatchSize ?? DEFAULT_MAX_BATCH_SIZE

  let pending = []
  let timeoutId = null

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

      if (pending.length >= maxBatchSize) {
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
 * Check server health
 */
export async function checkServerHealth() {
  try {
    const response = await fetch(`${serverUrl}/health`)

    if (!response.ok) {
      return { connected: false, error: `HTTP ${response.status}` }
    }

    const data = await response.json()
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
      return JSON.parse(serialized.slice(0, maxSize - 50) + '"} [truncated]')
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
        const jsonStr = atob(base64Match[1])
        const sourceMap = JSON.parse(jsonStr)
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

    const sourceMap = await mapResponse.json()
    const parsed = parseSourceMapData(sourceMap)
    sourceMapCache.set(scriptUrl, parsed)
    return parsed
  } catch {
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

        resolvedLines.push(`    at ${funcName} (${fileName}:${lineNum}:${colNum}) [resolved from ${resolved.fileName}:${resolved.lineNumber}:${resolved.columnNumber}]`)
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
      message.type === 'setWebSocketCaptureMode'
    ) {
      // Forward to all content scripts
      debugLog(DebugCategory.SETTINGS, `Setting ${message.type}: ${message.enabled}`)
      chrome.tabs.query({}, (tabs) => {
        for (const tab of tabs) {
          if (tab.id) {
            chrome.tabs.sendMessage(tab.id, message).catch(() => {
              // Tab may not have content script, ignore
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

  // Reconnect alarm
  chrome.alarms.create('reconnect', { periodInMinutes: RECONNECT_INTERVAL / 60000 })

  chrome.alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === 'reconnect') {
      checkConnectionAndUpdate()
    }
  })

  // Initial connection check
  checkConnectionAndUpdate()

  // Load saved settings
  chrome.storage.local.get(['serverUrl', 'logLevel', 'screenshotOnError', 'sourceMapEnabled', 'debugMode'], (result) => {
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
  })

  // Start error group flush timer
  errorGroupFlushTimer = setInterval(() => {
    const aggregatedEntries = flushErrorGroups()
    if (aggregatedEntries.length > 0) {
      aggregatedEntries.forEach((entry) => logBatcher.add(entry))
    }
  }, ERROR_GROUP_FLUSH_MS)

  // Clean up screenshot rate limits when tabs are closed
  chrome.tabs.onRemoved.addListener((tabId) => {
    screenshotTimestamps.delete(tabId)
  })
}

// Log batcher instance
const logBatcher = createLogBatcher(async (entries) => {
  // Monitor context annotation sizes
  checkContextAnnotations(entries)

  try {
    const result = await sendLogsToServer(entries)
    connectionStatus.entries = result.entries || connectionStatus.entries + entries.length
    connectionStatus.connected = true
    connectionStatus.errorCount += entries.filter((e) => e.level === 'error').length
    updateBadge(connectionStatus)
  } catch {
    connectionStatus.connected = false
    updateBadge(connectionStatus)
  }
})

// WebSocket event batcher instance
const wsBatcher = createLogBatcher(async (events) => {
  try {
    await sendWSEventsToServer(events)
    connectionStatus.connected = true
  } catch {
    connectionStatus.connected = false
    updateBadge(connectionStatus)
  }
}, { debounceMs: 200, maxBatchSize: 100 })

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

  // Notify popup if open
  if (typeof chrome !== 'undefined' && chrome.runtime) {
    chrome.runtime.sendMessage({ type: 'statusUpdate', status: connectionStatus }).catch(() => {
      // Popup not open, ignore
    })
  }
}
