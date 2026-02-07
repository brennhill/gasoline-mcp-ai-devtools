/**
 * @fileoverview Observers - Observer registration and management for DOM, network,
 * performance, and WebSocket events.
 */

import { installPerformanceCapture, uninstallPerformanceCapture, setPerformanceMarksEnabled } from '../lib/performance'
import { installPerfObservers } from '../lib/perf-snapshot'
import {
  installWebSocketCapture,
  setWebSocketCaptureMode,
  setWebSocketCaptureEnabled,
  uninstallWebSocketCapture,
} from '../lib/websocket'
import {
  setNetworkWaterfallEnabled,
  setNetworkBodyCaptureEnabled,
  setServerUrl,
  wrapFetchWithBodies,
} from '../lib/network'
import { installConsoleCapture, uninstallConsoleCapture } from '../lib/console'
import { installExceptionCapture, uninstallExceptionCapture } from '../lib/exceptions'
import {
  installActionCapture,
  uninstallActionCapture,
  installNavigationCapture,
  uninstallNavigationCapture,
} from '../lib/actions'
import { postLog } from '../lib/bridge'
import { MAX_RESPONSE_LENGTH, SENSITIVE_HEADERS, MEMORY_SOFT_LIMIT_MB, MEMORY_HARD_LIMIT_MB } from '../lib/constants'
import { createDeferredPromise } from '../lib/timeout-utils'

// Store original fetch for restoration
let originalFetch: typeof fetch | null = null

// Interception deferral state (Phase 1/Phase 2 split)
let deferralEnabled = true
let phase2Installed = false
let injectionTimestamp = 0
let phase2Timestamp = 0

/**
 * Network error log payload
 */
interface NetworkErrorLog {
  level: 'error'
  type: 'network'
  method: string
  url: string
  status?: number
  statusText?: string
  duration: number
  response?: string
  error?: string
  headers?: Record<string, string>
  [key: string]: unknown
}

/**
 * Wrap fetch to capture network errors
 */
export function wrapFetch(originalFetchFn: typeof fetch): typeof fetch {
  return async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const startTime = Date.now()
    const url = typeof input === 'string' ? input : (input as Request).url
    const method =
      init?.method || (typeof input === 'object' && 'method' in input ? (input as Request).method : 'GET') || 'GET'

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
        const safeHeaders: Record<string, string> = {}
        const rawHeaders =
          init?.headers || (typeof input === 'object' && 'headers' in input ? (input as Request).headers : null)
        if (rawHeaders) {
          const headers: Record<string, string> =
            rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : (rawHeaders as Record<string, string>)
          Object.keys(headers).forEach((key) => {
            const value = headers[key]
            if (value && !SENSITIVE_HEADERS.includes(key.toLowerCase())) {
              safeHeaders[key] = value
            }
          })
        }

        const logPayload: NetworkErrorLog = {
          level: 'error',
          type: 'network',
          method: method.toUpperCase(),
          url,
          status: response.status,
          statusText: response.statusText,
          duration,
          response: responseBody,
          ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}),
        }

        postLog(logPayload)
      }

      return response
    } catch (error) {
      const duration = Date.now() - startTime

      // Filter sensitive headers for the error path
      const safeHeaders: Record<string, string> = {}
      const rawHeaders =
        init?.headers || (typeof input === 'object' && 'headers' in input ? (input as Request).headers : null)
      if (rawHeaders) {
        const headers: Record<string, string> =
          rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : (rawHeaders as Record<string, string>)
        Object.keys(headers).forEach((key) => {
          const value = headers[key]
          if (value && !SENSITIVE_HEADERS.includes(key.toLowerCase())) {
            safeHeaders[key] = value
          }
        })
      }

      const logPayload: NetworkErrorLog = {
        level: 'error',
        type: 'network',
        method: method.toUpperCase(),
        url,
        error: (error as Error).message,
        duration,
        ...(Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}),
      }

      postLog(logPayload)

      throw error
    }
  }
}

/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 */
export function installFetchCapture(): void {
  originalFetch = window.fetch
  // Layer 1: wrapFetchWithBodies captures request/response bodies for ALL requests
  // Layer 2: wrapFetch captures detailed error logging for 4xx/5xx responses
  // Use unknown intermediate cast to handle TypeScript's strict fetch overload types
  // This is necessary because the DOM lib defines fetch with multiple overloads
  // that TypeScript cannot reconcile with our simpler function signature
  const wrappedWithBodies = wrapFetchWithBodies(originalFetch as unknown as Parameters<typeof wrapFetchWithBodies>[0])
  window.fetch = wrapFetch(wrappedWithBodies as unknown as typeof window.fetch)
}

/**
 * Uninstall fetch capture
 */
export function uninstallFetchCapture(): void {
  if (originalFetch) {
    window.fetch = originalFetch
    originalFetch = null
  }
}

/**
 * Install all capture hooks
 */
export function install(): void {
  installConsoleCapture()
  installFetchCapture()
  installExceptionCapture()
  installActionCapture()
  installNavigationCapture()
  installWebSocketCapture()
  installPerformanceCapture()
}

/**
 * Uninstall all capture hooks
 */
export function uninstall(): void {
  uninstallConsoleCapture()
  uninstallFetchCapture()
  uninstallExceptionCapture()
  uninstallActionCapture()
  uninstallNavigationCapture()
  uninstallWebSocketCapture()
  uninstallPerformanceCapture()
}

/**
 * Check if heavy intercepts should be deferred until page load
 */
export function shouldDeferIntercepts(): boolean {
  if (typeof document === 'undefined') return false
  return document.readyState === 'loading'
}

/**
 * Memory pressure check state
 */
interface MemoryPressureState {
  memoryUsageMB: number
  networkBodiesEnabled: boolean
  wsBufferCapacity: number
  networkBufferCapacity: number
}

/**
 * Check memory pressure and adjust buffer capacities
 */
export function checkMemoryPressure(state: MemoryPressureState): MemoryPressureState {
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

/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 */
export function installPhase1(): void {
  console.log('[Gasoline] Phase 1 installing (lightweight API + perf observers)')
  injectionTimestamp = performance.now()
  phase2Installed = false
  phase2Timestamp = 0

  // Start PerformanceObservers (passive observers, no prototype modification)
  installPerformanceCapture()

  // Now handle Phase 2 scheduling
  if (!deferralEnabled) {
    // Deferral disabled: install Phase 2 immediately
    installPhase2()
  } else {
    const installDeferred = (): void => {
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
 */
export function installPhase2(): void {
  // Double-injection guard
  if (phase2Installed) return

  // Environment guard
  if (typeof window === 'undefined' || typeof document === 'undefined') return

  console.log('[Gasoline] Phase 2 installing (heavy interceptors: console, fetch, WS, errors, actions)')
  phase2Timestamp = performance.now()
  phase2Installed = true

  // Install all heavy interceptors
  install()

  // FCP/LCP/CLS/INP/long-task observers (buffered: true replays pre-Phase-2 entries)
  installPerfObservers()
}

/**
 * Get the current deferral state for diagnostics and testing.
 */
export interface DeferralState {
  deferralEnabled: boolean
  phase2Installed: boolean
  injectionTimestamp: number
  phase2Timestamp: number
}

export function getDeferralState(): DeferralState {
  return {
    deferralEnabled,
    phase2Installed,
    injectionTimestamp,
    phase2Timestamp,
  }
}

/**
 * Set whether interception deferral is enabled.
 */
export function setDeferralEnabled(enabled: boolean): void {
  deferralEnabled = enabled
}
