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
import { getNetworkWaterfall, setNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled } from './lib/network.js'
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

// Constants used by wrapFetch (still in this file)
const MAX_RESPONSE_LENGTH = 5120 // 5KB
const SENSITIVE_HEADERS = ['authorization', 'cookie', 'set-cookie', 'x-auth-token']

// Re-export constants that tests import from inject.js
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES } from './lib/constants.js'

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

        // Filter sensitive headers
        const safeHeaders = {}
        if (init?.headers) {
          const headers = init.headers instanceof Headers ? Object.fromEntries(init.headers) : init.headers
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
    version: '5.0.0',
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
        message: `Script exceeded ${timeoutMs}ms timeout`,
      })
    }, timeoutMs)

    try {
      // Use Function constructor to execute in global scope
      // This runs in page context (inject.js), not extension context
      const fn = new Function(`
        "use strict";
        return (${script});
      `)

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
      resolve({
        success: false,
        error: 'execution_error',
        message: err.message,
        stack: err.stack,
      })
    }
  })
}

// =============================================================================
// LIFECYCLE & MEMORY
// =============================================================================

const MEMORY_SOFT_LIMIT_MB = 20
const MEMORY_HARD_LIMIT_MB = 50

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

// Listen for settings changes from content script
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    // Only accept messages from this window
    if (event.source !== window) return

    // Handle settings messages from content script
    if (event.data?.type === 'GASOLINE_SETTING') {
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
      }
    }

    // Handle state management commands from content script
    if (event.data?.type === 'GASOLINE_STATE_COMMAND') {
      const { messageId, action } = event.data
      let result

      try {
        if (action === 'capture') {
          result = captureState()
        } else if (action === 'restore') {
          const state = event.data.state
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
        '*',
      )
    }

    // Handle GASOLINE_EXECUTE_JS from content script
    if (event.data?.type === 'GASOLINE_EXECUTE_JS') {
      const { requestId, script, timeoutMs } = event.data
      executeJavaScript(script, timeoutMs).then((result) => {
        window.postMessage(
          {
            type: 'GASOLINE_EXECUTE_JS_RESULT',
            requestId,
            result,
          },
          '*',
        )
      })
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

  document.body.appendChild(gasolineHighlighter)

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
      const { selector, duration_ms } = event.data.params || {}
      const result = highlightElement(selector, duration_ms)
      // Post result back to content script
      window.postMessage(
        {
          type: 'GASOLINE_HIGHLIGHT_RESPONSE',
          result,
        },
        '*',
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
export function restoreState(state, includeUrl = true) {
  // Clear existing
  localStorage.clear()
  sessionStorage.clear()

  // Restore localStorage
  for (const [key, value] of Object.entries(state.localStorage || {})) {
    localStorage.setItem(key, value)
  }

  // Restore sessionStorage
  for (const [key, value] of Object.entries(state.sessionStorage || {})) {
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
    localStorage: Object.keys(state.localStorage || {}).length,
    sessionStorage: Object.keys(state.sessionStorage || {}).length,
    cookies: (state.cookies || '').split(';').filter((c) => c.trim()).length,
  }

  // Navigate if requested
  if (includeUrl && state.url && state.url !== window.location.href) {
    window.location.href = state.url
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
