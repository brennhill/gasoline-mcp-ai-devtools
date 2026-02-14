/**
 * @fileoverview Timeout Utilities for Content Script
 * Inlined utilities for promise timeout handling
 * Content scripts cannot use ES module imports, so these utilities are duplicated
 */

/** Custom error for timeout operations */
export class TimeoutError extends Error {
  constructor(
    message: string,
    public fallback?: unknown
  ) {
    super(message)
    this.name = 'TimeoutError'
  }
}

/** Deferred Promise interface */
export interface DeferredPromise<T> {
  promise: Promise<T>
  resolve: (value: T | PromiseLike<T>) => void
  reject: (reason?: unknown) => void
}

/** Create a deferred promise for external resolution */
export function createDeferredPromise<T>(): DeferredPromise<T> {
  let resolve!: (value: T | PromiseLike<T>) => void
  let reject!: (reason?: unknown) => void

  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })

  return { promise, resolve, reject }
}

/** Race a promise against a timeout with cleanup on timeout */
export async function promiseRaceWithCleanup<T>(
  promise: Promise<T>,
  timeoutMs: number,
  timeoutFallback: T | undefined,
  cleanup?: () => void
): Promise<T> {
  try {
    return await Promise.race([
      promise,
      new Promise<T>((_, reject) =>
        setTimeout(() => {
          cleanup?.()
          if (timeoutFallback !== undefined) {
            reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`, timeoutFallback))
          } else {
            reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`))
          }
        }, timeoutMs)
      )
    ])
  } catch (err) {
    if (err instanceof TimeoutError && err.fallback !== undefined) {
      return err.fallback as T
    }
    throw err
  }
}
