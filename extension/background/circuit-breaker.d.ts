/**
 * @fileoverview Circuit Breaker - Implements circuit breaker pattern with
 * exponential backoff for protecting server communication.
 */
import type { CircuitBreakerState, CircuitBreakerStats } from '../types';
export type { CircuitBreakerState, CircuitBreakerStats };
/** Circuit breaker options */
export interface CircuitBreakerOptions {
    maxFailures?: number;
    resetTimeout?: number;
    initialBackoff?: number;
    maxBackoff?: number;
}
/** Circuit breaker instance */
export interface CircuitBreaker {
    execute: <T>(args: unknown) => Promise<T>;
    getState: () => CircuitBreakerState;
    getStats: () => CircuitBreakerStats;
    reset: () => void;
    recordFailure: () => void;
}
/**
 * Circuit breaker with exponential backoff for server communication.
 * Prevents the extension from hammering a down/slow server.
 */
export declare function createCircuitBreaker(sendFn: (args: unknown) => Promise<unknown>, options?: CircuitBreakerOptions): CircuitBreaker;
//# sourceMappingURL=circuit-breaker.d.ts.map