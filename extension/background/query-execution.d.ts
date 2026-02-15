/** Result shape from JS execution */
export interface ExecutionResult {
  success: boolean
  error?: string
  message?: string
  result?: unknown
  stack?: string
}
/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
export declare function executeViaScriptingAPI(
  tabId: number,
  script: string,
  timeoutMs: number
): Promise<ExecutionResult>
/**
 * Execute JS with world-aware routing.
 * - isolated: execute directly via chrome.scripting API
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script, fallback to scripting API on CSP/inject errors
 */
export declare function executeWithWorldRouting(
  tabId: number,
  queryParams: string | Record<string, unknown>,
  world: string
): Promise<ExecutionResult>
//# sourceMappingURL=query-execution.d.ts.map
