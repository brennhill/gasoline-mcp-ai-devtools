// @ts-nocheck
/**
 * @fileoverview inject.js — Page-level capture script for browser telemetry.
 * Runs in the page context (not extension sandbox) to intercept console methods,
 * fetch/XHR requests, WebSocket connections, errors, and user actions. Posts
 * captured events to the content script via window.postMessage.
 * Design: Monkey-patches native APIs (console, fetch, WebSocket) with safe wrappers.
 * Defers network/WS interception until after page load to avoid impacting performance.
 * Buffers are size-capped (actions: 20, waterfall: 50, perf entries: 50).
 * Exposes window.__gasoline for version detection and programmatic control.
 */

// Re-exports (barrel pattern — tests and consumers import from inject.js)
export { safeSerialize, getElementSelector, isSensitiveInput } from './lib/serialize.js'
export {
  getContextAnnotations,
  setContextAnnotation,
  removeContextAnnotation,
  clearContextAnnotations,
} from './lib/context.js'
export {
  getImplicitRole,
  isDynamicClass,
  computeCssPath,
  computeSelectors,
  recordEnhancedAction,
  getEnhancedActionBuffer,
  clearEnhancedActionBuffer,
  generatePlaywrightScript,
} from './lib/reproduction.js'
export {
  recordAction,
  getActionBuffer,
  clearActionBuffer,
  handleClick,
  handleInput,
  handleScroll,
  handleKeydown,
  handleChange,
  installActionCapture,
  uninstallActionCapture,
  setActionCaptureEnabled,
  installNavigationCapture,
  uninstallNavigationCapture,
} from './lib/actions.js'
export {
  parseResourceTiming,
  getNetworkWaterfall,
  trackPendingRequest,
  completePendingRequest,
  getPendingRequests,
  clearPendingRequests,
  getNetworkWaterfallForError,
  setNetworkWaterfallEnabled,
  isNetworkWaterfallEnabled,
  setNetworkBodyCaptureEnabled,
  isNetworkBodyCaptureEnabled,
  shouldCaptureUrl,
  setServerUrl,
  sanitizeHeaders,
  truncateRequestBody,
  truncateResponseBody,
  readResponseBody,
  readResponseBodyWithTimeout,
  wrapFetchWithBodies,
} from './lib/network.js'
export {
  getPerformanceMarks,
  getPerformanceMeasures,
  getCapturedMarks,
  getCapturedMeasures,
  installPerformanceCapture,
  uninstallPerformanceCapture,
  isPerformanceCaptureActive,
  getPerformanceSnapshotForError,
  setPerformanceMarksEnabled,
  isPerformanceMarksEnabled,
} from './lib/performance.js'
export { postLog } from './lib/bridge.js'
export { installConsoleCapture, uninstallConsoleCapture } from './lib/console.js'
export {
  parseStackFrames,
  parseSourceMap,
  extractSnippet,
  extractSourceSnippets,
  detectFramework,
  getReactComponentAncestry,
  captureStateSnapshot,
  generateAiSummary,
  enrichErrorWithAiContext,
  setAiContextEnabled,
  setAiContextStateSnapshot,
  setSourceMapCache,
  getSourceMapCache,
  getSourceMapCacheSize,
} from './lib/ai-context.js'
export { installExceptionCapture, uninstallExceptionCapture } from './lib/exceptions.js'
export {
  getSize,
  formatPayload,
  truncateWsMessage,
  createConnectionTracker,
  installWebSocketCapture,
  setWebSocketCaptureMode,
  setWebSocketCaptureEnabled,
  getWebSocketCaptureMode,
  uninstallWebSocketCapture,
} from './lib/websocket.js'
export {
  executeDOMQuery,
  getPageInfo,
  runAxeAudit,
  runAxeAuditWithTimeout,
  formatAxeResults,
} from './lib/dom-queries.js'
export {
  mapInitiatorType,
  aggregateResourceTiming,
  capturePerformanceSnapshot,
  installPerfObservers,
  uninstallPerfObservers,
  getLongTaskMetrics,
  getFCP,
  getLCP,
  getCLS,
  getINP,
  sendPerformanceSnapshot,
  isPerformanceSnapshotEnabled,
  setPerformanceSnapshotEnabled,
} from './lib/perf-snapshot.js'

// Imports used directly in this file's orchestration code
import {
  getContextAnnotations,
  setContextAnnotation,
  removeContextAnnotation,
  clearContextAnnotations,
} from './lib/context.js'
import {
  computeSelectors,
  recordEnhancedAction,
  getEnhancedActionBuffer,
  clearEnhancedActionBuffer,
  generatePlaywrightScript,
} from './lib/reproduction.js'
import {
  getActionBuffer,
  clearActionBuffer,
  installActionCapture,
  uninstallActionCapture,
  setActionCaptureEnabled,
  installNavigationCapture,
  uninstallNavigationCapture,
} from './lib/actions.js'
import {
  getNetworkWaterfall,
  setNetworkWaterfallEnabled,
  setNetworkBodyCaptureEnabled,
  setServerUrl,
  wrapFetchWithBodies,
} from './lib/network.js'
import {
  getPerformanceMarks,
  getPerformanceMeasures,
  installPerformanceCapture,
  uninstallPerformanceCapture,
  setPerformanceMarksEnabled,
} from './lib/performance.js'
import { postLog } from './lib/bridge.js'
import { installConsoleCapture, uninstallConsoleCapture } from './lib/console.js'
import { enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot } from './lib/ai-context.js'
import { installExceptionCapture, uninstallExceptionCapture } from './lib/exceptions.js'
import {
  installWebSocketCapture,
  setWebSocketCaptureMode,
  setWebSocketCaptureEnabled,
  uninstallWebSocketCapture,
} from './lib/websocket.js'
import {
  installPerfObservers,
  uninstallPerfObservers,
  sendPerformanceSnapshot,
  setPerformanceSnapshotEnabled,
} from './lib/perf-snapshot.js'

// Import constants used by wrapFetch and memory checks from the single source of truth
import { MAX_RESPONSE_LENGTH, SENSITIVE_HEADERS, MEMORY_SOFT_LIMIT_MB, MEMORY_HARD_LIMIT_MB } from './lib/constants.js'

// Re-export constants that tests import from inject.js
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from './lib/constants.js'

// Store original methods
let originalFetch = null

// Interception deferral state (Phase 1/Phase 2 split)
let deferralEnabled = true // Default: defer heavy interceptors
let phase2Installed = false // Whether Phase 2 (heavy interceptors) has fired
let injectionTimestamp = 0 // performance.now() at Phase 1
let phase2Timestamp = 0 // performance.now() at Phase 2

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

        // Filter sensitive headers (check both init.headers and Request object headers)
        const safeHeaders = {}
        const rawHeaders = init?.headers || (typeof input === 'object' && input?.headers) || null
        if (rawHeaders) {
          const headers = rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : rawHeaders
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
          ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}),
        })
      }

      return response
    } catch (error) {
      const duration = Date.now() - startTime

      // Filter sensitive headers for the error path
      const safeHeaders = {}
      const rawHeaders = init?.headers || (typeof input === 'object' && input?.headers) || null
      if (rawHeaders) {
        const headers = rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : rawHeaders
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
        error: error.message,
        duration,
        ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}),
      })

      throw error
    }
  }
}

/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 * This ensures both body capture (GASOLINE_NETWORK_BODY) and error logging work together.
 */
export function installFetchCapture() {
  originalFetch = window.fetch
  // Layer 1: wrapFetchWithBodies captures request/response bodies for ALL requests
  // Layer 2: wrapFetch captures detailed error logging for 4xx/5xx responses
  window.fetch = wrapFetch(wrapFetchWithBodies(originalFetch))
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
 * Install all capture hooks
 */
export function install() {
  installConsoleCapture()
  installFetchCapture()
  installExceptionCapture()
  installActionCapture()
  installNavigationCapture()
  installWebSocketCapture()
  installPerfObservers()
}

/**
 * Uninstall all capture hooks
 */
export function uninstall() {
  uninstallConsoleCapture()
  uninstallFetchCapture()
  uninstallExceptionCapture()
  uninstallActionCapture()
  uninstallNavigationCapture()
  uninstallWebSocketCapture()
  uninstallPerfObservers()
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

    // === AI Context ===

    /**
     * Enrich an error entry with AI context
     * @param {Object} error - Error entry to enrich
     * @returns {Promise<Object>} Enriched error entry
     */
    enrichError(error) {
      return enrichErrorWithAiContext(error)
    },

    /**
     * Enable or disable AI context enrichment
     * @param {boolean} enabled
     */
    setAiContext(enabled) {
      setAiContextEnabled(enabled)
    },

    /**
     * Enable or disable state snapshot in AI context
     * @param {boolean} enabled
     */
    setStateSnapshot(enabled) {
      setAiContextStateSnapshot(enabled)
    },

    // === Reproduction Scripts ===

    /**
     * Record an enhanced action (for testing)
     * @param {string} type - Action type
     * @param {Element} element - Target element
     * @param {Object} opts - Options
     */
    recordAction(type, element, opts) {
      return recordEnhancedAction(type, element, opts)
    },

    /**
     * Get the enhanced action buffer
     * @returns {Array}
     */
    getEnhancedActions() {
      return getEnhancedActionBuffer()
    },

    /**
     * Clear the enhanced action buffer
     */
    clearEnhancedActions() {
      clearEnhancedActionBuffer()
    },

    /**
     * Generate a Playwright reproduction script
     * @param {Array} actions - Actions to convert
     * @param {Object} opts - Generation options
     * @returns {string} Playwright test script
     */
    generateScript(actions, opts) {
      return generatePlaywrightScript(actions || getEnhancedActionBuffer(), opts)
    },

    /**
     * Compute multi-strategy selectors for an element
     * @param {Element} element
     * @returns {Object}
     */
    getSelectors(element) {
      return computeSelectors(element)
    },

    /**
     * Version of the Gasoline API
     */
    version: '5.2.0',
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
// AI WEB PILOT: EXECUTE JAVASCRIPT
// =============================================================================

/**
 * Safe serialization for complex objects returned from executeJavaScript.
 * Handles circular references, DOM nodes, functions, and large objects.
 * @param {any} value - Value to serialize
 * @param {number} depth - Current recursion depth
 * @param {WeakSet} seen - Set of already-seen objects for circular detection
 * @returns {any} Serialized value
 */
export function safeSerializeForExecute(value, depth = 0, seen = new WeakSet()) {
  if (depth > 10) return '[max depth exceeded]'
  if (value === null) return null
  if (value === undefined) return undefined

  const type = typeof value
  if (type === 'string' || type === 'number' || type === 'boolean') {
    return value
  }

  if (type === 'function') {
    return `[Function: ${value.name || 'anonymous'}]`
  }

  if (type === 'symbol') {
    return value.toString()
  }

  if (type === 'object') {
    if (seen.has(value)) return '[Circular]'
    seen.add(value)

    if (Array.isArray(value)) {
      return value.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen))
    }

    if (value instanceof Error) {
      return { error: value.message, stack: value.stack }
    }

    if (value instanceof Date) {
      return value.toISOString()
    }

    if (value instanceof RegExp) {
      return value.toString()
    }

    // DOM nodes
    if (typeof Node !== 'undefined' && value instanceof Node) {
      return `[${value.nodeName}${value.id ? '#' + value.id : ''}]`
    }

    // Plain objects
    const result = {}
    const keys = Object.keys(value).slice(0, 50)
    for (const key of keys) {
      try {
        result[key] = safeSerializeForExecute(value[key], depth + 1, seen)
      } catch {
        result[key] = '[unserializable]'
      }
    }
    if (Object.keys(value).length > 50) {
      result['...'] = `[${Object.keys(value).length - 50} more keys]`
    }
    return result
  }

  return String(value)
}

/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 * Used by the AI Web Pilot execute_javascript tool.
 * @param {string} script - JavaScript expression to evaluate
 * @param {number} timeoutMs - Timeout in milliseconds (default 5000)
 * @returns {Promise<Object>} Result with success/result or error details
 */
export function executeJavaScript(script, timeoutMs = 5000) {
  return new Promise((resolve) => {
    const timeoutId = setTimeout(() => {
      resolve({
        success: false,
        error: 'execution_timeout',
        message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`,
      })
    }, timeoutMs)

    try {
      // Use Function constructor to execute in global scope
      // This runs in page context (inject.js), not extension context
      const cleanScript = script.trim()

      // Detect if this is a multi-statement script (contains semicolons or already has return).
      // Known limitation: semicolons inside string literals (e.g., 'a;b') will cause the
      // script to be treated as multi-statement, requiring the user to add an explicit
      // return statement. A robust fix would require a full JS parser, which is not
      // warranted for this heuristic.
      const hasMultipleStatements = cleanScript.includes(';')
      const hasExplicitReturn = /\breturn\b/.test(cleanScript)

      let fnBody
      if (hasMultipleStatements || hasExplicitReturn) {
        // Multi-statement or explicit return: use script as-is
        // User must provide their own return statement
        fnBody = `"use strict"; ${cleanScript}`
      } else {
        // Single expression: wrap in return
        fnBody = `"use strict"; return (${cleanScript});`
      }

      // eslint-disable-next-line no-new-func -- Intentional: execute_js runs user-provided scripts in page context
      const fn = new Function(fnBody)

      const result = fn()

      // Handle promises - keep timeout active until promise settles
      if (result && typeof result.then === 'function') {
        result
          .then((value) => {
            clearTimeout(timeoutId)
            resolve({ success: true, result: safeSerializeForExecute(value) })
          })
          .catch((err) => {
            clearTimeout(timeoutId)
            resolve({
              success: false,
              error: 'promise_rejected',
              message: err.message,
              stack: err.stack,
            })
          })
      } else {
        // Synchronous result - clear timeout immediately
        clearTimeout(timeoutId)
        resolve({ success: true, result: safeSerializeForExecute(result) })
      }
    } catch (err) {
      clearTimeout(timeoutId)

      // Detect CSP blocking eval
      if (err.message && (err.message.includes('Content Security Policy') || err.message.includes('unsafe-eval'))) {
        resolve({
          success: false,
          error: 'csp_blocked',
          message:
            'This page has a Content Security Policy that blocks script execution. Try on a different page (e.g., localhost, about:blank, or a page without strict CSP).',
          original_error: err.message,
        })
      } else {
        resolve({
          success: false,
          error: 'execution_error',
          message: err.message,
          stack: err.stack,
        })
      }
    }
  })
}

// =============================================================================
// LIFECYCLE & MEMORY
// =============================================================================

/**
 * Check if heavy intercepts should be deferred until page load
 * @returns {boolean} True if page is still loading
 */
export function shouldDeferIntercepts() {
  if (typeof document === 'undefined') return false
  return document.readyState === 'loading'
}

/**
 * Check memory pressure and adjust buffer capacities
 * @param {Object} state - Current buffer state
 * @returns {Object} Adjusted state
 */
export function checkMemoryPressure(state) {
  const result = { ...state }

  if (state.memoryUsageMB >= MEMORY_HARD_LIMIT_MB) {
    // Hard limit: disable network bodies
    result.networkBodiesEnabled = false
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.25)
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.25)
  } else if (state.memoryUsageMB >= MEMORY_SOFT_LIMIT_MB) {
    // Soft limit: reduce buffers
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.5)
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.5)
  }

  return result
}

// Valid setting names from content script
const VALID_SETTINGS = new Set([
  'setNetworkWaterfallEnabled',
  'setPerformanceMarksEnabled',
  'setActionReplayEnabled',
  'setWebSocketCaptureEnabled',
  'setWebSocketCaptureMode',
  'setPerformanceSnapshotEnabled',
  'setDeferralEnabled',
  'setNetworkBodyCaptureEnabled',
  'setServerUrl',
])

const VALID_STATE_ACTIONS = new Set(['capture', 'restore'])

// Listen for settings changes from content script
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    // Only accept messages from this window
    if (event.source !== window) return

    // Handle settings messages from content script
    if (event.data?.type === 'GASOLINE_SETTING') {
      // Validate setting name
      if (!VALID_SETTINGS.has(event.data.setting)) {
        console.warn('[Gasoline] Invalid setting:', event.data.setting)
        return
      }

      // Validate parameter types based on setting
      if (event.data.setting === 'setWebSocketCaptureMode') {
        if (typeof event.data.mode !== 'string') {
          console.warn('[Gasoline] Invalid mode type for setWebSocketCaptureMode')
          return
        }
      } else if (event.data.setting === 'setServerUrl') {
        if (typeof event.data.url !== 'string') {
          console.warn('[Gasoline] Invalid url type for setServerUrl')
          return
        }
      } else {
        // Boolean settings
        if (typeof event.data.enabled !== 'boolean') {
          console.warn('[Gasoline] Invalid enabled value type')
          return
        }
      }

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
          setWebSocketCaptureEnabled(event.data.enabled)
          if (event.data.enabled) {
            installWebSocketCapture()
          } else {
            uninstallWebSocketCapture()
          }
          break
        case 'setWebSocketCaptureMode':
          setWebSocketCaptureMode(event.data.mode || 'lifecycle')
          break
        case 'setPerformanceSnapshotEnabled':
          setPerformanceSnapshotEnabled(event.data.enabled)
          break
        case 'setDeferralEnabled':
          setDeferralEnabled(event.data.enabled)
          break
        case 'setNetworkBodyCaptureEnabled':
          setNetworkBodyCaptureEnabled(event.data.enabled)
          break
        case 'setServerUrl':
          setServerUrl(event.data.url)
          break
      }
    }

    // Handle state management commands from content script
    if (event.data?.type === 'GASOLINE_STATE_COMMAND') {
      const { messageId, action, state } = event.data

      // Validate action
      if (!VALID_STATE_ACTIONS.has(action)) {
        console.warn('[Gasoline] Invalid state action:', action)
        window.postMessage(
          {
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: `Invalid action: ${action}` },
          },
          window.location.origin,
        )
        return
      }

      // Validate state object for restore action
      if (action === 'restore' && (!state || typeof state !== 'object')) {
        console.warn('[Gasoline] Invalid state object for restore')
        window.postMessage(
          {
            type: 'GASOLINE_STATE_RESPONSE',
            messageId,
            result: { error: 'Invalid state object' },
          },
          window.location.origin,
        )
        return
      }

      let result

      try {
        if (action === 'capture') {
          result = captureState()
        } else if (action === 'restore') {
          const includeUrl = event.data.include_url !== false
          result = restoreState(state, includeUrl)
        } else {
          result = { error: `Unknown action: ${action}` }
        }
      } catch (err) {
        result = { error: err.message }
      }

      // Send response back to content script
      window.postMessage(
        {
          type: 'GASOLINE_STATE_RESPONSE',
          messageId,
          result,
        },
        window.location.origin,
      )
    }

    // Handle GASOLINE_EXECUTE_JS from content script
    if (event.data?.type === 'GASOLINE_EXECUTE_JS') {
      const { requestId, script, timeoutMs } = event.data

      // Validate parameters
      if (typeof script !== 'string') {
        console.warn('[Gasoline] Script must be a string')
        window.postMessage(
          {
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result: { success: false, error: 'invalid_script', message: 'Script must be a string' },
          },
          window.location.origin,
        )
        return
      }

      if (typeof requestId !== 'number' && typeof requestId !== 'string') {
        console.warn('[Gasoline] Invalid requestId type')
        return
      }

      executeJavaScript(script, timeoutMs)
        .then((result) => {
          window.postMessage(
            {
              type: 'GASOLINE_EXECUTE_JS_RESULT',
              requestId,
              result,
            },
            window.location.origin,
          )
        })
        .catch((err) => {
          console.error('[Gasoline] Failed to execute JS:', err)
          // Attempt to notify requester of failure
          window.postMessage(
            {
              type: 'GASOLINE_EXECUTE_JS_RESULT',
              requestId,
              result: { success: false, error: 'execution_failed', message: err.message },
            },
            window.location.origin,
          )
        })
    }

    // Handle GASOLINE_A11Y_QUERY from content script (accessibility audit)
    if (event.data?.type === 'GASOLINE_A11Y_QUERY') {
      const { requestId, params } = event.data

      // Defensive check: Chrome extension caching can cause module-level
      // bindings to be undefined after updates. Return a clear error instead
      // of crashing with "runAxeAuditWithTimeout is not defined".
      if (typeof runAxeAuditWithTimeout !== 'function') {
        window.postMessage(
          {
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: {
              error: 'runAxeAuditWithTimeout not available — try reloading the extension',
            },
          },
          window.location.origin,
        )
        return
      }

      try {
        runAxeAuditWithTimeout(params || {})
          .then((result) => {
            // Send response back to content script
            window.postMessage(
              {
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result,
              },
              window.location.origin,
            )
          })
          .catch((err) => {
            console.error('[Gasoline] Accessibility audit error:', err)
            window.postMessage(
              {
                type: 'GASOLINE_A11Y_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'Accessibility audit failed' },
              },
              window.location.origin,
            )
          })
      } catch (err) {
        console.error('[Gasoline] Failed to run accessibility audit:', err)
        window.postMessage(
          {
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run accessibility audit' },
          },
          window.location.origin,
        )
      }
    }

    // Handle GASOLINE_DOM_QUERY from content script (CSS selector query)
    if (event.data?.type === 'GASOLINE_DOM_QUERY') {
      const { requestId, params } = event.data

      // Defensive check: Chrome extension caching can cause module-level
      // bindings to be undefined after updates. Return a clear error instead
      // of crashing with "executeDOMQuery is not defined".
      if (typeof executeDOMQuery !== 'function') {
        window.postMessage(
          {
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: {
              error: 'executeDOMQuery not available — try reloading the extension',
            },
          },
          window.location.origin,
        )
        return
      }

      try {
        executeDOMQuery(params || {})
          .then((result) => {
            // Send response back to content script
            window.postMessage(
              {
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result,
              },
              window.location.origin,
            )
          })
          .catch((err) => {
            console.error('[Gasoline] DOM query error:', err)
            window.postMessage(
              {
                type: 'GASOLINE_DOM_QUERY_RESPONSE',
                requestId,
                result: { error: err.message || 'DOM query failed' },
              },
              window.location.origin,
            )
          })
      } catch (err) {
        console.error('[Gasoline] Failed to run DOM query:', err)
        window.postMessage(
          {
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Failed to run DOM query' },
          },
          window.location.origin,
        )
      }
    }

    // Handle GASOLINE_GET_WATERFALL from content script (collect network timing data)
    if (event.data?.type === 'GASOLINE_GET_WATERFALL') {
      const { requestId } = event.data

      try {
        // Get all network waterfall entries (no time filtering - get everything)
        const entries = getNetworkWaterfall({})

        // Send response back to content script
        window.postMessage(
          {
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: entries || [],
          },
          window.location.origin,
        )
      } catch (err) {
        console.error('[Gasoline] Failed to get network waterfall:', err)
        // Send empty response on error
        window.postMessage(
          {
            type: 'GASOLINE_WATERFALL_RESPONSE',
            requestId,
            entries: [],
          },
          window.location.origin,
        )
      }
    }
  })
}

/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 * - Registers window.__gasoline API
 * - Sets up message listener (already done above)
 * - Starts PerformanceObservers for paint timing and CLS
 * - Records injection timestamp
 * - Triggers Phase 2 based on deferral settings
 */
export function installPhase1() {
  console.log('[Gasoline] Phase 1 installing (lightweight API + perf observers)')
  injectionTimestamp = performance.now()
  phase2Installed = false
  phase2Timestamp = 0

  // Install the __gasoline API (lightweight, no interception)
  installGasolineAPI()

  // Start PerformanceObservers (passive observers, no prototype modification)
  installPerfObservers()

  // Now handle Phase 2 scheduling
  if (!deferralEnabled) {
    // Deferral disabled: install Phase 2 immediately
    installPhase2()
  } else {
    const installDeferred = () => {
      if (!phase2Installed) setTimeout(installPhase2, 100)
    }
    if (document.readyState === 'complete') {
      // Page already loaded, defer by 100ms
      installDeferred()
    } else {
      // Wait for load event, then defer by 100ms
      window.addEventListener('load', installDeferred, { once: true })
      // 10-second timeout fallback
      setTimeout(() => {
        if (!phase2Installed) installPhase2()
      }, 10000)
    }
  }
}

/**
 * Phase 2 (Deferred): Heavy interceptors.
 * Installs console wrapping, fetch wrapping, WebSocket replacement,
 * error handlers, action capture, and navigation capture.
 */
export function installPhase2() {
  // Double-injection guard
  if (phase2Installed) return

  // Environment guard: ensure window/document still exist (protects against
  // test teardown or edge cases where the environment is destroyed)
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  console.log('[Gasoline] Phase 2 installing (heavy interceptors: console, fetch, WS, errors, actions)')
  phase2Timestamp = performance.now()
  phase2Installed = true

  // Install all heavy interceptors (console, fetch, WS, errors, actions, navigation)
  install()
}

/**
 * Get the current deferral state for diagnostics and testing.
 */
export function getDeferralState() {
  return {
    deferralEnabled,
    phase2Installed,
    injectionTimestamp,
    phase2Timestamp,
  }
}

/**
 * Set whether interception deferral is enabled.
 * When false, Phase 2 runs immediately (matching pre-deferral behavior).
 */
export function setDeferralEnabled(enabled) {
  deferralEnabled = enabled
}

// ============================================================================
// AI WEB PILOT: HIGHLIGHT
// ============================================================================

let gasolineHighlighter = null

/**
 * Highlight a DOM element by injecting a red overlay div.
 * @param {string} selector - CSS selector for the element to highlight
 * @param {number} durationMs - How long to show the highlight (default 5000ms)
 * @returns {Object} Result with success, bounds, or error
 */
export function highlightElement(selector, durationMs = 5000) {
  // Remove existing highlight
  if (gasolineHighlighter) {
    gasolineHighlighter.remove()
    gasolineHighlighter = null
  }

  const element = document.querySelector(selector)
  if (!element) {
    return { success: false, error: 'element_not_found', selector }
  }

  const rect = element.getBoundingClientRect()

  gasolineHighlighter = document.createElement('div')
  gasolineHighlighter.id = 'gasoline-highlighter'
  gasolineHighlighter.dataset.selector = selector
  Object.assign(gasolineHighlighter.style, {
    position: 'fixed',
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    border: '4px solid red',
    borderRadius: '4px',
    backgroundColor: 'rgba(255, 0, 0, 0.1)',
    zIndex: '2147483647',
    pointerEvents: 'none',
    boxSizing: 'border-box',
  })

  const targetElement = document.body || document.documentElement
  if (targetElement) {
    targetElement.appendChild(gasolineHighlighter)
  } else {
    console.warn('[Gasoline] No document body available for highlighter injection')
    return
  }

  setTimeout(() => {
    if (gasolineHighlighter) {
      gasolineHighlighter.remove()
      gasolineHighlighter = null
    }
  }, durationMs)

  return {
    success: true,
    selector,
    bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
  }
}

/**
 * Clear any existing highlight
 */
export function clearHighlight() {
  if (gasolineHighlighter) {
    gasolineHighlighter.remove()
    gasolineHighlighter = null
  }
}

// Handle scroll — update highlight position
if (typeof window !== 'undefined') {
  window.addEventListener(
    'scroll',
    () => {
      if (gasolineHighlighter) {
        const selector = gasolineHighlighter.dataset.selector
        if (selector) {
          const el = document.querySelector(selector)
          if (el) {
            const rect = el.getBoundingClientRect()
            gasolineHighlighter.style.top = `${rect.top}px`
            gasolineHighlighter.style.left = `${rect.left}px`
          }
        }
      }
    },
    { passive: true },
  )
}

// Handle GASOLINE_HIGHLIGHT_REQUEST messages from content script
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    if (event.source !== window) return
    if (event.data?.type === 'GASOLINE_HIGHLIGHT_REQUEST') {
      const { requestId, params } = event.data
      const { selector, duration_ms } = params || {}
      const result = highlightElement(selector, duration_ms)
      // Post result back to content script with requestId
      window.postMessage(
        {
          type: 'GASOLINE_HIGHLIGHT_RESPONSE',
          requestId,
          result,
        },
        window.location.origin,
      )
    }
  })
}

// ============================================================================
// AI WEB PILOT: STATE MANAGEMENT
// ============================================================================

/**
 * Capture browser state (localStorage, sessionStorage, cookies).
 * Returns a snapshot that can be restored later.
 * @returns {Object} State snapshot with url, timestamp, localStorage, sessionStorage, cookies
 */
export function captureState() {
  const state = {
    url: window.location.href,
    timestamp: Date.now(),
    localStorage: {},
    sessionStorage: {},
    cookies: document.cookie,
  }

  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i)
    state.localStorage[key] = localStorage.getItem(key)
  }

  for (let i = 0; i < sessionStorage.length; i++) {
    const key = sessionStorage.key(i)
    state.sessionStorage[key] = sessionStorage.getItem(key)
  }

  return state
}

/**
 * Restore browser state from a snapshot.
 * Clears existing state before restoring.
 * @param {Object} state - State snapshot from captureState()
 * @param {boolean} includeUrl - Whether to navigate to the saved URL (default true)
 * @returns {Object} Result with success and restored counts
 */
// Validates a storage key to prevent prototype pollution and other attacks
function isValidStorageKey(key) {
  if (typeof key !== 'string') return false
  if (key.length === 0 || key.length > 256) return false

  // Reject prototype pollution vectors
  const dangerous = ['__proto__', 'constructor', 'prototype']
  const lowerKey = key.toLowerCase()
  for (const pattern of dangerous) {
    if (lowerKey.includes(pattern)) return false
  }

  return true
}

export function restoreState(state, includeUrl = true) {
  // Validate state object
  if (!state || typeof state !== 'object') {
    return { success: false, error: 'Invalid state object' }
  }

  // Clear existing
  localStorage.clear()
  sessionStorage.clear()

  // Restore localStorage with validation
  let skipped = 0
  for (const [key, value] of Object.entries(state.localStorage || {})) {
    if (!isValidStorageKey(key)) {
      skipped++
      console.warn('[gasoline] Skipped localStorage key with invalid pattern:', key)
      continue
    }
    // Limit value size (10MB max per item)
    if (typeof value === 'string' && value.length > 10 * 1024 * 1024) {
      skipped++
      console.warn('[gasoline] Skipped localStorage value exceeding 10MB:', key)
      continue
    }
    localStorage.setItem(key, value)
  }

  // Restore sessionStorage with validation
  for (const [key, value] of Object.entries(state.sessionStorage || {})) {
    if (!isValidStorageKey(key)) {
      skipped++
      console.warn('[gasoline] Skipped sessionStorage key with invalid pattern:', key)
      continue
    }
    if (typeof value === 'string' && value.length > 10 * 1024 * 1024) {
      skipped++
      console.warn('[gasoline] Skipped sessionStorage value exceeding 10MB:', key)
      continue
    }
    sessionStorage.setItem(key, value)
  }

  // Restore cookies (clear then set)
  document.cookie.split(';').forEach((c) => {
    const name = c.split('=')[0].trim()
    if (name) {
      document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`
    }
  })

  if (state.cookies) {
    state.cookies.split(';').forEach((c) => {
      document.cookie = c.trim()
    })
  }

  const restored = {
    localStorage: Object.keys(state.localStorage || {}).length - skipped,
    sessionStorage: Object.keys(state.sessionStorage || {}).length,
    cookies: (state.cookies || '').split(';').filter((c) => c.trim()).length,
    skipped,
  }

  // Navigate if requested (with basic URL validation)
  if (includeUrl && state.url && state.url !== window.location.href) {
    // Basic URL validation: must be http/https
    try {
      const url = new URL(state.url)
      if (url.protocol === 'http:' || url.protocol === 'https:') {
        window.location.href = state.url
      } else {
        console.warn('[gasoline] Skipped navigation to non-HTTP(S) URL:', state.url)
      }
    } catch (e) {
      console.warn('[gasoline] Invalid URL for navigation:', state.url, e)
    }
  }

  if (skipped > 0) {
    console.warn(`[gasoline] restoreState completed with ${skipped} skipped item(s)`)
  }

  return { success: true, restored }
}

// Auto-install when loaded in browser
if (typeof window !== 'undefined' && typeof document !== 'undefined') {
  installPhase1()

  // Send performance snapshot after page load + 2s settling time
  window.addEventListener('load', () => {
    setTimeout(() => {
      sendPerformanceSnapshot()
    }, 2000)
  })
}
