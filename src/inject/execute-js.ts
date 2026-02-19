// execute-js.ts — JavaScript execution sandbox for in-page script evaluation.

import type { ExecuteJsResult } from '../types/index'
import { createDeferredPromise } from '../lib/timeout-utils'

/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
// #lizard forgives
function serializeObject(obj: object, depth: number, seen: WeakSet<object>): unknown {
  if (seen.has(obj)) return '[Circular]'
  seen.add(obj)

  if (Array.isArray(obj)) return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen))
  if (obj instanceof Error) return { error: obj.message, stack: obj.stack }
  if (obj instanceof Date) return obj.toISOString()
  if (obj instanceof RegExp) return obj.toString()
  if (typeof Node !== 'undefined' && obj instanceof Node) {
    const node = obj as Node & { id?: string }
    return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`
  }

  const result: Record<string, unknown> = {}
  const keys = Object.keys(obj).slice(0, 50)
  for (const key of keys) {
    try {
      result[key] = safeSerializeForExecute((obj as Record<string, unknown>)[key], depth + 1, seen)
    } catch {
      result[key] = '[unserializable]'
    }
  }
  if (Object.keys(obj).length > 50) {
    result['...'] = `[${Object.keys(obj).length - 50} more keys]`
  }
  return result
}

export function safeSerializeForExecute(
  value: unknown,
  depth: number = 0,
  seen: WeakSet<object> = new WeakSet()
): unknown {
  if (depth > 10) return '[max depth exceeded]'
  if (value === null || value === undefined) return value

  const type = typeof value
  if (type === 'string' || type === 'number' || type === 'boolean') return value
  if (type === 'function') return `[Function: ${(value as (...args: unknown[]) => unknown).name || 'anonymous'}]`
  if (type === 'symbol') return (value as symbol).toString()
  if (type === 'object') return serializeObject(value as object, depth, seen)

  return String(value)
}

/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export function executeJavaScript(script: string, timeoutMs: number = 5000): Promise<ExecuteJsResult> {
  const deferred = createDeferredPromise<ExecuteJsResult>()

  // #lizard forgives
  const executeWithTimeoutProtection = async (): Promise<void> => {
    const timeoutHandle = setTimeout(() => {
      deferred.resolve({
        success: false,
        error: 'execution_timeout',
        message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
      })
    }, timeoutMs)

    try {
      const cleanScript = script.trim()

      // Try expression form first (captures return values from IIFEs, expressions).
      // If it throws SyntaxError (statements like try/catch, if/else), fall back to statement form.
      let fn: () => unknown
      try {
        // eslint-disable-next-line no-new-func
        fn = new Function(`"use strict"; return (${cleanScript});`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
      } catch {
        // eslint-disable-next-line no-new-func
        fn = new Function(`"use strict"; ${cleanScript}`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
      }

      const result = fn()

      // Handle promises
      if (result && typeof (result as Promise<unknown>).then === 'function') {
        ;(result as Promise<unknown>)
          .then((value) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({ success: true, result: safeSerializeForExecute(value) })
          })
          .catch((err: Error) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({
              success: false,
              error: 'promise_rejected',
              message: err.message,
              stack: err.stack
            })
          })
      } else {
        clearTimeout(timeoutHandle)
        deferred.resolve({ success: true, result: safeSerializeForExecute(result) })
      }
    } catch (err) {
      clearTimeout(timeoutHandle)

      const error = err as Error
      if (
        error.message &&
        (error.message.includes('Content Security Policy') ||
          error.message.includes('unsafe-eval') ||
          error.message.includes('Trusted Type'))
      ) {
        deferred.resolve({
          success: false,
          error: 'csp_blocked',
          message:
            'This page has a Content Security Policy that blocks script execution in the MAIN world. ' +
            'Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). ' +
            'With world: "auto" (default), this fallback happens automatically.'
        })
      } else {
        deferred.resolve({
          success: false,
          error: 'execution_error',
          message: error.message,
          stack: error.stack
        })
      }
    }
  }

  executeWithTimeoutProtection().catch((err) => {
    console.error('[Gasoline] Unexpected error in executeJavaScript:', err)
    deferred.resolve({
      success: false,
      error: 'execution_error',
      message: 'Unexpected error during script execution'
    })
  })

  return deferred.promise
}
