/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Timeout and Promise utilities - Reusable patterns for handling timeouts,
 * promise races, and message-based async operations with cleanup.
 *
 * These utilities extract common patterns found throughout the Gasoline extension:
 * - Promise.race with timeout fallback
 * - Message-based request/response with timeout and cleanup
 * - Deferred promises for storing resolvers/rejecters
 * - Safe timeout management with resource cleanup
 */
/**
 * Deferred Promise - Holds resolve/reject callbacks for external resolution
 * Useful for resolving a promise from outside async/await context
 * @template T The type of the resolved value
 */
export interface DeferredPromise<T> {
    promise: Promise<T>;
    resolve: (value: T | PromiseLike<T>) => void;
    reject: (reason?: unknown) => void;
}
/**
 * Create a deferred promise
 * @template T The type of the resolved value
 * @returns Deferred promise with resolve/reject methods
 *
 * @example
 * const deferred = createDeferredPromise<number>();
 * setTimeout(() => deferred.resolve(42), 100);
 * const result = await deferred.promise; // 42
 */
export declare function createDeferredPromise<T>(): DeferredPromise<T>;
/**
 * Wrap a promise with a timeout fallback
 * Returns the result of the promise if it resolves before timeout,
 * otherwise returns the fallback value (or rejects if no fallback)
 *
 * @template T The type of the promise value
 * @param promise The promise to wrap
 * @param timeoutMs Timeout in milliseconds
 * @param fallback Optional fallback value to return on timeout
 * @returns Promise that resolves to the result or fallback (or rejects on timeout if no fallback)
 *
 * @example
 * // With fallback value
 * const result = await withTimeout(fetch('/api'), 5000, { ok: false });
 *
 * @example
 * // Without fallback (rejects on timeout)
 * try {
 *   const result = await withTimeout(slowOperation(), 3000);
 * } catch (err) {
 *   // Handle timeout
 * }
 */
export declare function withTimeout<T>(promise: Promise<T>, timeoutMs: number, fallback?: T): Promise<T>;
/**
 * Custom error for timeout operations that optionally carries a fallback value
 */
export declare class TimeoutError extends Error {
    fallback?: unknown | undefined;
    constructor(message: string, fallback?: unknown | undefined);
}
/**
 * Wrap a promise with a timeout that rejects on timeout
 * This is a stricter version of withTimeout - no fallback allowed
 *
 * @template T The type of the promise value
 * @param promise The promise to wrap
 * @param timeoutMs Timeout in milliseconds
 * @returns Promise that resolves to the result or rejects on timeout
 *
 * @example
 * try {
 *   const data = await promiseWithTimeout(fetchData(), 5000);
 * } catch (err) {
 *   if (err instanceof TimeoutError) {
 *     console.error('Request timed out');
 *   }
 * }
 */
export declare function promiseWithTimeout<T>(promise: Promise<T>, timeoutMs: number): Promise<T>;
/**
 * Message-based async operation with timeout and cleanup
 * Manages request/response correlation using a Map and IDs, with automatic cleanup
 *
 * This pattern is used extensively in content.ts for:
 * - Highlight requests (30s timeout)
 * - Execute JS requests (30s timeout)
 * - A11y audit requests (30s timeout)
 * - DOM query requests (30s timeout)
 * - Network waterfall requests (5s timeout)
 *
 * @template T The type of the response value
 * @param sender Function that sends the message/request
 * @param timeoutMs Timeout in milliseconds
 * @param cleanup Optional cleanup function called on timeout (e.g., to remove event listeners)
 * @returns Promise that resolves to the response or rejects on timeout
 *
 * @example
 * // Simple message send with timeout
 * const response = await messageWithTimeout(
 *   async () => chrome.runtime.sendMessage({ type: 'PING' }),
 *   5000
 * );
 *
 * @example
 * // With event listener cleanup
 * const response = await messageWithTimeout(
 *   async () => {
 *     const requestId = ++requestIdCounter;
 *     pendingRequests.set(requestId, (result) => deferred.resolve(result));
 *     window.postMessage({ type: 'REQUEST', requestId }, origin);
 *     return deferred.promise;
 *   },
 *   30000,
 *   () => {
 *     pendingRequests.delete(requestId);
 *     window.removeEventListener('message', handler);
 *   }
 * );
 */
export declare function messageWithTimeout<T>(sender: () => Promise<T>, timeoutMs: number, cleanup?: () => void): Promise<T>;
/**
 * Race a promise against a timeout, calling a cleanup function if timeout wins
 * Used for operations that set up listeners or other resources that need cleanup
 *
 * @template T The type of the promise value
 * @param promise The promise to race against timeout
 * @param timeoutMs Timeout in milliseconds
 * @param timeoutFallback Value to return on timeout (if provided, doesn't throw)
 * @param cleanup Function to call if timeout occurs
 * @returns Promise that resolves to result, fallback (if provided), or rejects
 *
 * @example
 * const result = await promiseRaceWithCleanup(
 *   waitForResponse(),
 *   5000,
 *   { entries: [] }, // fallback for timeout
 *   () => removeEventListener('message', handler) // cleanup
 * );
 */
export declare function promiseRaceWithCleanup<T>(promise: Promise<T>, timeoutMs: number, timeoutFallback: T | undefined, cleanup?: () => void): Promise<T>;
/**
 * Execute a callback with automatic timeout and fallback
 * The callback should return a promise that resolves with the result
 *
 * @template T The type of the result
 * @param callback Function that returns a promise
 * @param timeoutMs Timeout in milliseconds
 * @param fallback Optional fallback value to return on timeout
 * @returns Promise that resolves to the result, fallback, or rejects
 *
 * @example
 * const result = await executeWithTimeout(
 *   () => fetch('/api/data'),
 *   5000,
 *   { ok: false, status: 408 } // fallback
 * );
 */
export declare function executeWithTimeout<T>(callback: () => Promise<T>, timeoutMs: number, fallback?: T): Promise<T>;
/**
 * Create a promise that resolves after a delay
 * Useful for retry logic or deferring operations
 *
 * @param delayMs Delay in milliseconds
 * @returns Promise that resolves after the delay
 *
 * @example
 * await delay(1000); // Wait 1 second
 */
export declare function delay(delayMs: number): Promise<void>;
/**
 * Retry a promise-returning function with exponential backoff
 * Useful for flaky operations like network requests
 *
 * @template T The type of the result
 * @param fn Function that returns a promise
 * @param maxAttempts Maximum number of attempts
 * @param initialDelayMs Initial delay before first retry (doubles each attempt)
 * @returns Promise that resolves if any attempt succeeds, or rejects if all fail
 *
 * @example
 * const result = await retryWithBackoff(
 *   () => fetch('/api/data'),
 *   3,
 *   100
 * );
 */
export declare function retryWithBackoff<T>(fn: () => Promise<T>, maxAttempts?: number, initialDelayMs?: number): Promise<T>;
/**
 * Create a cancellable promise that can be aborted
 * @template T The type of the result
 * @param promise The promise to wrap
 * @param operationName Optional human-readable name of the operation for error messages
 * @returns Object with the promise and a cancel function
 *
 * @example
 * const { promise, cancel } = makeCancellable(fetch('/api/data'), 'fetch user data');
 * setTimeout(() => cancel(), 5000);
 * try {
 *   const result = await promise;
 * } catch (err) {
 *   if (err.message.includes('cancelled')) {
 *     console.log('Operation was cancelled:', err.message);
 *   }
 * }
 */
export declare function makeCancellable<T>(promise: Promise<T>, operationName?: string): {
    promise: Promise<T>;
    cancel: () => void;
};
/**
 * Wait for a condition to become true or timeout
 * Polls at regular intervals until condition is true or timeout occurs
 *
 * @param condition Function that returns true when condition is met
 * @param timeoutMs Maximum time to wait in milliseconds
 * @param pollIntervalMs How often to check the condition (default 100ms)
 * @returns Promise that resolves if condition becomes true, rejects on timeout
 *
 * @example
 * await waitFor(() => element.classList.contains('visible'), 5000);
 */
export declare function waitFor(condition: () => boolean, timeoutMs: number, pollIntervalMs?: number): Promise<void>;
/**
 * Race multiple promises and return the result of the first one that settles
 * (resolves or rejects). This differs from Promise.race in that it includes
 * rejection reasons.
 *
 * @template T The type of the result
 * @param promises Promises to race
 * @returns Promise that settles with the result of the first settling promise
 *
 * @example
 * const result = await racePromises([
 *   fetch('/api/data'),
 *   delay(5000).then(() => { throw new TimeoutError('Too slow'); })
 * ]);
 */
export declare function racePromises<T>(promises: Promise<T>[]): Promise<T>;
/**
 * Combine multiple timeout utilities: execute a callback with timeout,
 * automatic cleanup on timeout, and optional fallback
 *
 * @template T The type of the result
 * @param callback Callback that returns a promise
 * @param timeoutMs Timeout in milliseconds
 * @param fallback Optional fallback value on timeout
 * @param cleanup Optional cleanup function called on timeout
 * @returns Promise that resolves to result, fallback, or rejects
 *
 * @example
 * const result = await executeWithTimeoutAndCleanup(
 *   async () => {
 *     const requestId = generateId();
 *     window.addEventListener('message', handler);
 *     window.postMessage({ type: 'REQUEST', requestId });
 *     return deferred.promise;
 *   },
 *   5000,
 *   { success: false },
 *   () => window.removeEventListener('message', handler)
 * );
 */
export declare function executeWithTimeoutAndCleanup<T>(callback: () => Promise<T>, timeoutMs: number, fallback?: T, cleanup?: () => void): Promise<T>;
//# sourceMappingURL=timeout-utils.d.ts.map