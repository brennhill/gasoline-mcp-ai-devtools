/**
 * @fileoverview Circuit Breaker - Implements circuit breaker pattern with
 * exponential backoff for protecting server communication.
 */

import type { CircuitBreakerState, CircuitBreakerStats } from '../types'

// Re-export types for external use
export type { CircuitBreakerState, CircuitBreakerStats }

/** Circuit breaker options */
export interface CircuitBreakerOptions {
  maxFailures?: number
  resetTimeout?: number
  initialBackoff?: number
  maxBackoff?: number
}

/** Circuit breaker instance */
export interface CircuitBreaker {
  execute: <T>(args: unknown) => Promise<T>
  getState: () => CircuitBreakerState
  getStats: () => CircuitBreakerStats
  reset: () => void
  recordFailure: () => void
}

/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export function createCircuitBreaker(
  sendFn: (args: unknown) => Promise<unknown>,
  options: CircuitBreakerOptions = {},
): CircuitBreaker {
  const maxFailures = options.maxFailures ?? 5
  const resetTimeout = options.resetTimeout ?? 30000
  const initialBackoff = options.initialBackoff ?? 1000
  const maxBackoff = options.maxBackoff ?? 30000

  let state: CircuitBreakerState = 'closed'
  let consecutiveFailures = 0
  let totalFailures = 0
  let totalSuccesses = 0
  let currentBackoff = 0
  let lastFailureTime = 0
  let probeInFlight = false

  function getState(): CircuitBreakerState {
    if (state === 'open' && Date.now() - lastFailureTime >= resetTimeout) {
      state = 'half-open'
    }
    return state
  }

  function getStats(): CircuitBreakerStats {
    return {
      state: getState(),
      consecutiveFailures,
      totalFailures,
      totalSuccesses,
      currentBackoff,
    }
  }

  function reset(): void {
    state = 'closed'
    consecutiveFailures = 0
    currentBackoff = 0
    probeInFlight = false
  }

  function onSuccess(): void {
    consecutiveFailures = 0
    currentBackoff = 0
    totalSuccesses++
    state = 'closed'
    probeInFlight = false
  }

  function onFailure(): void {
    consecutiveFailures++
    totalFailures++
    lastFailureTime = Date.now()
    probeInFlight = false

    if (consecutiveFailures >= maxFailures) {
      state = 'open'
    }

    if (consecutiveFailures > 1) {
      currentBackoff = Math.min(initialBackoff * Math.pow(2, consecutiveFailures - 2), maxBackoff)
    } else {
      currentBackoff = 0
    }
  }

  async function execute<T>(args: unknown): Promise<T> {
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

    if (currentBackoff > 0) {
      await new Promise<void>((r) => {
        setTimeout(r, currentBackoff)
      })
    }

    try {
      const result = (await sendFn(args)) as T
      onSuccess()
      return result
    } catch (err) {
      onFailure()
      throw err
    }
  }

  function recordFailure(): void {
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
