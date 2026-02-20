/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
// query-execution.ts — JavaScript execution with world-aware routing and CSP fallback.
// Handles execute_js queries via content script (MAIN world) or chrome.scripting API (ISOLATED).
import * as index from './index.js';
import { DebugCategory } from './debug.js';
import { scaleTimeout } from '../lib/timeouts.js';
const { debugLog } = index;
/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
export async function executeViaScriptingAPI(tabId, script, timeoutMs) {
    const timeoutPromise = new Promise((_, reject) => {
        setTimeout(() => reject(new Error(`Script exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000);
    });
    const executionPromise = chrome.scripting.executeScript({
        target: { tabId },
        world: 'MAIN',
        func: (code) => {
            try {
                const cleaned = code.trim();
                // Try expression form first (captures return values from IIFEs, expressions).
                // If SyntaxError (statements like try/catch, if/else), fall back to statement form.
                let fn;
                try {
                    // eslint-disable-next-line no-new-func
                    fn = new Function(`"use strict"; return (${cleaned});`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
                }
                catch {
                    // eslint-disable-next-line no-new-func
                    fn = new Function(`"use strict"; ${cleaned}`); // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- chrome.scripting.executeScript API, not eval()
                }
                const result = fn();
                if (result !== null && result !== undefined && typeof result.then === 'function') {
                    return result
                        .then((v) => {
                        return { success: true, result: serialize(v) };
                    })
                        .catch((err) => {
                        const e = err;
                        return { success: false, error: 'promise_rejected', message: e.message };
                    });
                }
                return { success: true, result: serialize(result) };
            }
            catch (err) {
                const e = err;
                const msg = e.message || '';
                if (msg.includes('Content Security Policy') || msg.includes('Trusted Type') || msg.includes('unsafe-eval')) {
                    return {
                        success: false,
                        error: 'csp_blocked_all_worlds',
                        message: 'Page CSP blocks dynamic script execution. ' +
                            'Use query_dom for DOM operations or navigate away from this CSP-restricted page.'
                    };
                }
                return { success: false, error: 'execution_error', message: msg, stack: e.stack };
            }
            function serialize(value, depth = 0, seen = new WeakSet()) {
                if (depth > 10)
                    return '[max depth]';
                if (value === null || value === undefined)
                    return value;
                const t = typeof value;
                if (t === 'string' || t === 'number' || t === 'boolean')
                    return value;
                if (t === 'function')
                    return '[Function]';
                if (t === 'symbol')
                    return String(value);
                if (t === 'object') {
                    const obj = value;
                    if (seen.has(obj))
                        return '[Circular]';
                    seen.add(obj);
                    if (Array.isArray(obj))
                        return obj.slice(0, 100).map((v) => serialize(v, depth + 1, seen));
                    if (obj instanceof Error)
                        return { error: obj.message };
                    if (obj instanceof Date)
                        return obj.toISOString();
                    if (obj instanceof RegExp)
                        return String(obj);
                    // DOM node duck-type check (works across worlds)
                    if ('nodeType' in obj && 'nodeName' in obj) {
                        const node = obj;
                        return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`;
                    }
                    const result = {};
                    for (const key of Object.keys(obj).slice(0, 50)) {
                        try {
                            result[key] = serialize(obj[key], depth + 1, seen);
                        }
                        catch {
                            result[key] = '[unserializable]';
                        }
                    }
                    return result;
                }
                return String(value);
            }
        },
        args: [script]
    });
    try {
        const results = await Promise.race([executionPromise, timeoutPromise]);
        const firstResult = results?.[0]?.result;
        if (firstResult && typeof firstResult === 'object') {
            return firstResult;
        }
        return { success: false, error: 'no_result', message: 'chrome.scripting.executeScript produced no result' };
    }
    catch (err) {
        const msg = err.message || '';
        if (msg.includes('timeout')) {
            return { success: false, error: 'execution_timeout', message: msg };
        }
        return { success: false, error: 'scripting_api_error', message: msg };
    }
}
/**
 * Execute JS with world-aware routing.
 * - isolated: execute directly via chrome.scripting API
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script, fallback to scripting API on CSP/inject errors
 */
// #lizard forgives
export async function executeWithWorldRouting(tabId, queryParams, world) {
    let parsedParams;
    try {
        parsedParams = typeof queryParams === 'string' ? JSON.parse(queryParams) : queryParams;
    }
    catch {
        parsedParams = {};
    }
    const script = parsedParams.script || '';
    const timeoutMs = parsedParams.timeout_ms || scaleTimeout(5000);
    if (world === 'isolated') {
        return executeViaScriptingAPI(tabId, script, timeoutMs);
    }
    // MAIN or AUTO: try content script (MAIN world) first
    try {
        const result = (await chrome.tabs.sendMessage(tabId, {
            type: 'GASOLINE_EXECUTE_QUERY',
            params: queryParams
        }));
        // Auto-fallback: retry via scripting API on CSP or inject issues
        if (world === 'auto' &&
            result &&
            !result.success &&
            (result.error === 'csp_blocked' || result.error === 'inject_not_loaded')) {
            debugLog(DebugCategory.CONNECTION, 'Auto-fallback to chrome.scripting API', {
                error: result.error,
                tabId
            });
            return executeViaScriptingAPI(tabId, script, timeoutMs);
        }
        return result;
    }
    catch (err) {
        let message = err.message || 'Tab communication failed';
        // Auto-fallback: content script not reachable
        if (world === 'auto' && message.includes('Receiving end does not exist')) {
            debugLog(DebugCategory.CONNECTION, 'Auto-fallback (content script unreachable)', { tabId });
            return executeViaScriptingAPI(tabId, script, timeoutMs);
        }
        if (message.includes('Receiving end does not exist')) {
            message =
                'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({what: "refresh"})\n\nThen retry your command.';
        }
        return { success: false, error: 'content_script_not_loaded', message };
    }
}
//# sourceMappingURL=query-execution.js.map