/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */

/**
 * @fileoverview Request Tracking Module
 * Manages pending requests for AI Web Pilot features
 * Includes periodic cleanup timer to handle edge cases where pagehide/beforeunload don't fire.
 */

import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types/index.js'
import type { PendingRequestStats } from './types.js'

/**
 * Generic tracker for pending request/response pairs keyed by auto-incrementing numeric ID.
 */
class PendingRequestTracker<T> {
  private readonly pending = new Map<number, (result: T) => void>()
  private nextId = 0

  /** Register a resolver and return the assigned request ID. */
  register(resolve: (result: T) => void): number {
    const id = ++this.nextId
    this.pending.set(id, resolve)
    return id
  }

  /** Resolve a pending request by ID and remove it from the tracker. */
  resolve(id: number, result: T): void {
    const resolver = this.pending.get(id)
    if (resolver) {
      this.pending.delete(id)
      resolver(result)
    }
  }

  /** Check whether a request ID is still pending. */
  has(id: number): boolean {
    return this.pending.has(id)
  }

  /** Remove a pending request without resolving it. */
  delete(id: number): void {
    this.pending.delete(id)
  }

  /** Remove all pending requests. */
  clear(): void {
    this.pending.clear()
  }

  /** Number of pending requests. */
  get size(): number {
    return this.pending.size
  }
}

// Typed tracker instances for each request category
const highlightTracker = new PendingRequestTracker<HighlightResponse>()
const executeTracker = new PendingRequestTracker<ExecuteJsResult>()
const a11yTracker = new PendingRequestTracker<A11yAuditResult>()
const domTracker = new PendingRequestTracker<DomQueryResult>()

// Periodic cleanup timer (Issue #2 fix)
const CLEANUP_INTERVAL_MS = 30000 // 30 seconds
let cleanupTimer: ReturnType<typeof setInterval> | null = null

// Track request timestamps for stale detection
const requestTimestamps = new Map<number, number>()

/**
 * Get request timestamps for stale detection (Issue #2 fix).
 * Returns array of [requestId, timestamp] pairs for cleanup.
 */
function getRequestTimestamps(): [number, number][] {
  const timestamps: [number, number][] = []
  for (const [id, timestamp] of requestTimestamps) {
    timestamps.push([id, timestamp])
  }
  return timestamps
}

/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests(): void {
  highlightTracker.clear()
  executeTracker.clear()
  a11yTracker.clear()
  domTracker.clear()
  requestTimestamps.clear()
}

/**
 * Perform periodic cleanup of stale requests (Issue #2 fix).
 * Removes requests older than 60 seconds as a fallback when pagehide/beforeunload don't fire.
 */
function performPeriodicCleanup(): void {
  const now = Date.now()
  const staleThreshold = 60000 // 60 seconds

  for (const [id, timestamp] of getRequestTimestamps()) {
    if (now - timestamp > staleThreshold) {
      // Remove stale request from all trackers
      highlightTracker.delete(id)
      executeTracker.delete(id)
      a11yTracker.delete(id)
      domTracker.delete(id)
      requestTimestamps.delete(id)
    }
  }
}

/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export function getPendingRequestStats(): PendingRequestStats {
  return {
    highlight: highlightTracker.size,
    execute: executeTracker.size,
    a11y: a11yTracker.size,
    dom: domTracker.size
  }
}

// --- Highlight requests ---

export function registerHighlightRequest(resolve: (result: HighlightResponse) => void): number {
  return highlightTracker.register(resolve)
}

export function resolveHighlightRequest(requestId: number, result: HighlightResponse): void {
  highlightTracker.resolve(requestId, result)
}

export function hasHighlightRequest(requestId: number): boolean {
  return highlightTracker.has(requestId)
}

export function deleteHighlightRequest(requestId: number): void {
  highlightTracker.delete(requestId)
}

// --- Execute requests ---

export function registerExecuteRequest(resolve: (result: ExecuteJsResult) => void): number {
  return executeTracker.register(resolve)
}

export function resolveExecuteRequest(requestId: number, result: ExecuteJsResult): void {
  executeTracker.resolve(requestId, result)
}

export function hasExecuteRequest(requestId: number): boolean {
  return executeTracker.has(requestId)
}

export function deleteExecuteRequest(requestId: number): void {
  executeTracker.delete(requestId)
}

// --- A11y requests ---

export function registerA11yRequest(resolve: (result: A11yAuditResult) => void): number {
  return a11yTracker.register(resolve)
}

export function resolveA11yRequest(requestId: number, result: A11yAuditResult): void {
  a11yTracker.resolve(requestId, result)
}

export function hasA11yRequest(requestId: number): boolean {
  return a11yTracker.has(requestId)
}

export function deleteA11yRequest(requestId: number): void {
  a11yTracker.delete(requestId)
}

// --- DOM requests ---

export function registerDomRequest(resolve: (result: DomQueryResult) => void): number {
  return domTracker.register(resolve)
}

export function resolveDomRequest(requestId: number, result: DomQueryResult): void {
  domTracker.resolve(requestId, result)
}

export function hasDomRequest(requestId: number): boolean {
  return domTracker.has(requestId)
}

export function deleteDomRequest(requestId: number): void {
  domTracker.delete(requestId)
}

/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking(): void {
  if (cleanupTimer) {
    clearInterval(cleanupTimer)
    cleanupTimer = null
  }
  clearPendingRequests()
}

/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking(): void {
  // Register cleanup handlers for page unload/navigation (Issue 2 fix)
  // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
  window.addEventListener('pagehide', clearPendingRequests)
  window.addEventListener('beforeunload', clearPendingRequests)

  // Start periodic cleanup timer (Issue #2 fix)
  // Provides fallback when pagehide/beforeunload don't fire (e.g., page crash)
  cleanupTimer = setInterval(performPeriodicCleanup, CLEANUP_INTERVAL_MS)
}
