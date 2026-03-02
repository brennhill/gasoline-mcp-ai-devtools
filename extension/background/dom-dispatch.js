/**
 * Purpose: Dispatches DOM actions (click, type, wait_for, list_interactive, query) to injected page scripts with frame targeting and CDP escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import { domFrameProbe } from './dom-frame-probe.js';
import { domPrimitive } from './dom-primitives.js';
import { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
import { domPrimitiveQuery } from './dom-primitives-query.js';
import { isCDPEscalatable, tryCDPEscalation } from './cdp-dispatch.js';
function parseDOMParams(query) {
    try {
        return typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        return null;
    }
}
function isReadOnlyAction(action) {
    return action === 'list_interactive' || action === 'query' || action.startsWith('get_');
}
function isMutatingAction(action) {
    return (action === 'click' ||
        action === 'type' ||
        action === 'select' ||
        action === 'check' ||
        action === 'set_attribute' ||
        action === 'paste' ||
        action === 'key_press' ||
        action === 'focus' ||
        action === 'scroll_to' ||
        action === 'hover');
}
function hasMatchedTargetEvidence(result) {
    const matched = result.matched;
    if (!matched || typeof matched !== 'object' || Array.isArray(matched))
        return false;
    return (typeof matched.selector === 'string' ||
        typeof matched.tag === 'string' ||
        typeof matched.element_id === 'string' ||
        typeof matched.aria_label === 'string' ||
        typeof matched.role === 'string' ||
        typeof matched.text_preview === 'string');
}
function normalizeFrameTarget(frame) {
    if (frame === undefined || frame === null)
        return undefined;
    if (typeof frame === 'number') {
        if (!Number.isInteger(frame) || frame < 0)
            return null;
        return frame;
    }
    if (typeof frame === 'string') {
        const trimmed = frame.trim();
        if (trimmed.length === 0)
            return null;
        return trimmed;
    }
    return null;
}
async function resolveExecutionTarget(tabId, frame) {
    const normalized = normalizeFrameTarget(frame);
    if (normalized === null) {
        throw new Error('invalid_frame: frame parameter must be a CSS selector, 0-based index, or "all". Got unsupported type or value');
    }
    if (normalized === undefined || normalized === 'all') {
        return { tabId, allFrames: true };
    }
    const probeResults = await chrome.scripting.executeScript({
        target: { tabId, allFrames: true },
        world: 'MAIN',
        func: domFrameProbe,
        args: [normalized]
    });
    const frameIds = Array.from(new Set(probeResults
        .filter((r) => !!r.result?.matches)
        .map((r) => r.frameId)
        .filter((id) => typeof id === 'number')));
    if (frameIds.length === 0) {
        throw new Error('frame_not_found: no iframe matched the given selector or index. Verify the iframe exists and is loaded on the page');
    }
    return { tabId, frameIds };
}
/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
function pickFrameResult(results) {
    const mainFrame = results.find((r) => r.frameId === 0);
    if (mainFrame?.result && mainFrame.result.success) {
        return { result: mainFrame.result, frameId: 0 };
    }
    for (const r of results) {
        if (r.result && r.result.success) {
            return { result: r.result, frameId: r.frameId };
        }
    }
    if (mainFrame?.result)
        return { result: mainFrame.result, frameId: 0 };
    return results[0] ? { result: results[0].result, frameId: results[0].frameId } : null;
}
/** Merge list_interactive results from all frames (up to 100 elements). */
function mergeListInteractive(results) {
    const elements = [];
    let firstError = null;
    let firstScopeRectUsed;
    for (const r of results) {
        const res = r.result;
        if (res?.success === false) {
            if (!firstError)
                firstError = { error: res.error, message: res.message };
            continue;
        }
        if (firstScopeRectUsed === undefined && res?.scope_rect_used !== undefined) {
            firstScopeRectUsed = res.scope_rect_used;
        }
        if (res?.elements)
            elements.push(...res.elements);
        if (elements.length >= 100)
            break;
    }
    if (elements.length === 0 && firstError?.error) {
        return { success: false, elements: [], error: firstError.error, message: firstError.message };
    }
    const cappedElements = elements.slice(0, 100);
    const merged = {
        success: true,
        elements: cappedElements,
        candidate_count: cappedElements.length
    };
    if (firstScopeRectUsed !== undefined) {
        merged.scope_rect_used = firstScopeRectUsed;
    }
    return merged;
}
const WAIT_FOR_POLL_INTERVAL_MS = 80;
function toDOMResult(value) {
    if (!value || typeof value !== 'object')
        return null;
    const candidate = value;
    if (typeof candidate.success !== 'boolean')
        return null;
    if (typeof candidate.action !== 'string' || typeof candidate.selector !== 'string')
        return null;
    return candidate;
}
function wait(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
}
/** Resolve which DOM action name to dispatch for wait_for based on params.
 *  Callers must validate mutual exclusivity before calling this. */
function resolveWaitForAction(params) {
    if (params.absent)
        return 'wait_for_absent';
    if (params.text)
        return 'wait_for_text';
    return 'wait_for';
}
async function executeWaitForURL(tabId, params) {
    const urlSubstring = params.url_contains;
    const timeoutMs = Math.max(1, params.timeout_ms ?? 5000);
    const startedAt = Date.now();
    while (true) {
        const tab = await chrome.tabs.get(tabId);
        if (tab.url && tab.url.includes(urlSubstring)) {
            return {
                success: true,
                action: 'wait_for',
                selector: '',
                value: tab.url
            };
        }
        if (Date.now() - startedAt >= timeoutMs) {
            return {
                success: false,
                action: 'wait_for',
                selector: '',
                error: 'timeout',
                message: `URL did not contain "${urlSubstring}" within ${timeoutMs}ms`
            };
        }
        const remaining = timeoutMs - (Date.now() - startedAt);
        await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)));
    }
}
async function executeWaitFor(target, params) {
    const selector = params.selector || '';
    const timeoutMs = Math.max(1, params.timeout_ms ?? 5000);
    const domAction = resolveWaitForAction(params);
    const domOpts = { timeout_ms: timeoutMs, text: params.text };
    const startedAt = Date.now();
    const quickCheck = await chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitive,
        args: [domAction, selector, domOpts]
    });
    const quickPicked = pickFrameResult(quickCheck);
    const quickResult = toDOMResult(quickPicked?.result);
    if (quickResult?.success) {
        return quickResult;
    }
    let lastResult = toDOMResult(quickPicked?.result) ?? null;
    while (Date.now() - startedAt < timeoutMs) {
        const remaining = timeoutMs - (Date.now() - startedAt);
        await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)));
        const probeResults = await chrome.scripting.executeScript({
            target,
            world: 'MAIN',
            func: domPrimitive,
            args: [domAction, selector, domOpts]
        });
        const picked = pickFrameResult(probeResults);
        const result = toDOMResult(picked?.result);
        if (result)
            lastResult = result;
        if (result?.success) {
            return result;
        }
    }
    const label = domAction === 'wait_for_text'
        ? `Text "${params.text}" not found within ${timeoutMs}ms`
        : domAction === 'wait_for_absent'
            ? `Element still present within ${timeoutMs}ms: ${selector}`
            : undefined;
    return lastResult ?? {
        success: false,
        action: 'wait_for',
        selector,
        error: 'timeout',
        message: label || `Element not found within ${timeoutMs}ms: ${selector}`
    };
}
async function executeStandardAction(target, params) {
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitive,
        args: [
            params.action,
            params.selector || '',
            {
                text: params.text,
                key: params.key,
                value: params.value,
                direction: params.direction,
                clear: params.clear,
                checked: params.checked,
                name: params.name,
                timeout_ms: params.timeout_ms,
                stability_ms: params.stability_ms,
                analyze: params.analyze,
                observe_mutations: params.observe_mutations,
                element_id: params.element_id,
                scope_selector: params.scope_selector,
                scope_rect: params.scope_rect,
                nth: params.nth,
                new_tab: params.new_tab,
                structured: params.structured
            }
        ]
    });
}
async function executeListInteractive(target, params) {
    // Build options object with scope_rect and filter params (#369)
    const opts = {};
    if (params.scope_rect)
        opts.scope_rect = params.scope_rect;
    if (params.text_contains)
        opts.text_contains = params.text_contains;
    if (params.role)
        opts.role = params.role;
    if (params.visible_only)
        opts.visible_only = params.visible_only;
    if (params.exclude_nav)
        opts.exclude_nav = params.exclude_nav;
    const hasOpts = Object.keys(opts).length > 0;
    const args = hasOpts
        ? [params.selector || '', opts]
        : [params.selector || ''];
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveListInteractive,
        args
    });
}
// #370: Execute DOM query (exists, count, text, text_all, attributes)
async function executeQuery(target, params) {
    const opts = {};
    if (params.query_type)
        opts.query_type = params.query_type;
    if (params.attribute_names)
        opts.attribute_names = params.attribute_names;
    if (params.scope_selector)
        opts.scope_selector = params.scope_selector;
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitiveQuery,
        args: [params.selector || '', Object.keys(opts).length > 0 ? opts : undefined]
    });
}
function sendToastForResult(tabId, readOnly, result, actionToast, toastLabel, toastDetail) {
    if (readOnly)
        return;
    if (result.success) {
        actionToast(tabId, toastLabel, toastDetail, 'success');
    }
    else {
        actionToast(tabId, toastLabel, result.error || 'failed', 'error');
    }
}
function reconcileDOMLifecycle(action, selector, result) {
    const domResult = toDOMResult(result);
    if (!domResult) {
        if (!isMutatingAction(action))
            return { result, status: 'complete' };
        const coerced = {
            success: false,
            action,
            selector,
            error: 'status_mismatch',
            message: `Mutating action returned non-DOM payload: ${action}`
        };
        return { result: coerced, status: 'error', error: 'status_mismatch' };
    }
    if (!domResult.success) {
        return {
            result: domResult,
            status: 'error',
            error: domResult.error || domResult.message || 'dom_action_failed'
        };
    }
    if (domResult.error) {
        const coerced = {
            ...domResult,
            success: false,
            error: 'status_mismatch',
            message: `Payload marked success but includes error: ${domResult.error}`
        };
        return { result: coerced, status: 'error', error: 'status_mismatch' };
    }
    if (isMutatingAction(action) && !hasMatchedTargetEvidence(domResult)) {
        const coerced = {
            ...domResult,
            success: false,
            error: 'missing_match_evidence',
            message: `Mutating action completed without matched target evidence: ${action}`
        };
        return { result: coerced, status: 'error', error: 'missing_match_evidence' };
    }
    return { result: domResult, status: 'complete' };
}
function deriveAsyncStatusFromDOMResult(action, selector, result) {
    const reconciled = reconcileDOMLifecycle(action, selector, result);
    if (reconciled.status === 'complete') {
        return reconciled;
    }
    return {
        status: 'error',
        error: reconciled.error || 'dom_action_failed',
        result: reconciled.result
    };
}
// Enrich results with effective tab context (post-execution URL).
// Agents compare resolved_url (dispatch time) vs effective_url (execution time) to detect drift.
async function enrichWithEffectiveContext(tabId, result) {
    try {
        const tab = await chrome.tabs.get(tabId);
        if (result && typeof result === 'object' && !Array.isArray(result)) {
            return { ...result, effective_tab_id: tabId, effective_url: tab.url, effective_title: tab.title };
        }
        return result;
    }
    catch {
        return result;
    }
}
// #lizard forgives
export async function executeDOMAction(query, tabId, syncClient, sendAsyncResult, actionToast) {
    const params = parseDOMParams(query);
    if (!params) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'invalid_params');
        return;
    }
    const { action, selector, reason } = params;
    if (!action) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_action');
        return;
    }
    if (action === 'wait_for') {
        const hasSelector = !!(selector || params.element_id);
        const hasText = !!params.text;
        const hasURL = !!params.url_contains;
        const condCount = ((hasSelector || params.absent) ? 1 : 0) + (hasText ? 1 : 0) + (hasURL ? 1 : 0);
        if (condCount === 0) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'wait_for requires selector, text, or url_contains');
            return;
        }
        if (condCount > 1) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'wait_for conditions are mutually exclusive');
            return;
        }
        if (params.absent && !hasSelector) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'wait_for with absent requires a selector');
            return;
        }
    }
    const toastLabel = reason || action;
    const toastDetail = reason ? undefined : selector || 'page';
    const readOnly = isReadOnlyAction(action);
    // URL-based wait_for: polls chrome.tabs.get from background — no page injection needed.
    if (action === 'wait_for' && params.url_contains) {
        try {
            const urlResult = await executeWaitForURL(tabId, params);
            const status = urlResult.success ? 'complete' : 'error';
            sendAsyncResult(syncClient, query.id, query.correlation_id, status, await enrichWithEffectiveContext(tabId, urlResult), urlResult.success ? undefined : urlResult.error);
        }
        catch (err) {
            actionToast(tabId, action, err.message, 'error');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, err.message);
        }
        return;
    }
    try {
        const executionTarget = await resolveExecutionTarget(tabId, params.frame);
        const tryingShownAt = Date.now();
        if (!readOnly)
            actionToast(tabId, toastLabel, toastDetail, 'trying', 10000);
        // CDP auto-escalation: try hardware events first for click/type/key_press (main frame only).
        // Falls back to DOM primitives silently if CDP is unavailable or fails.
        if (isCDPEscalatable(action) && !params.frame && params.nth === undefined) {
            try {
                const cdpResult = await tryCDPEscalation(tabId, action, params);
                if (cdpResult) {
                    const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector || '', cdpResult);
                    const domResult = toDOMResult(reconciledResult);
                    if (domResult) {
                        sendToastForResult(tabId, false, domResult, actionToast, toastLabel, toastDetail);
                    }
                    else {
                        actionToast(tabId, toastLabel, toastDetail, 'success');
                    }
                    sendAsyncResult(syncClient, query.id, query.correlation_id, status, await enrichWithEffectiveContext(tabId, reconciledResult), error);
                    return;
                }
            }
            catch {
                // CDP failed — fall through to DOM primitives
            }
        }
        const rawResult = action === 'list_interactive'
            ? await executeListInteractive(executionTarget, params)
            : action === 'query'
                ? await executeQuery(executionTarget, params)
                : action === 'wait_for'
                    ? await executeWaitFor(executionTarget, params)
                    : await executeStandardAction(executionTarget, params);
        // wait_for quick-check can return a DOMResult directly
        if (!Array.isArray(rawResult)) {
            if (rawResult === null || rawResult === undefined) {
                if (!readOnly)
                    actionToast(tabId, toastLabel, 'no result', 'error');
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'no_result');
                return;
            }
            const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector || '', rawResult);
            const domResult = toDOMResult(reconciledResult);
            if (domResult) {
                sendToastForResult(tabId, readOnly, domResult, actionToast, toastLabel, toastDetail);
            }
            else if (!readOnly && status === 'complete') {
                actionToast(tabId, toastLabel, toastDetail, 'success');
            }
            else if (!readOnly && status === 'error') {
                actionToast(tabId, toastLabel, error || 'failed', 'error');
            }
            sendAsyncResult(syncClient, query.id, query.correlation_id, status, await enrichWithEffectiveContext(tabId, reconciledResult), error);
            return;
        }
        // Ensure "trying" toast is visible for at least 500ms
        const MIN_TOAST_MS = 500;
        const elapsed = Date.now() - tryingShownAt;
        if (!readOnly && elapsed < MIN_TOAST_MS)
            await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed));
        // list_interactive: merge elements from all frames
        if (action === 'list_interactive') {
            const merged = mergeListInteractive(rawResult);
            const status = merged.success ? 'complete' : 'error';
            sendAsyncResult(syncClient, query.id, query.correlation_id, status, await enrichWithEffectiveContext(tabId, merged), merged.success ? undefined : merged.error || 'list_interactive_failed');
            return;
        }
        const picked = pickFrameResult(rawResult);
        const firstResult = picked?.result;
        if (firstResult && typeof firstResult === 'object') {
            let resultPayload;
            if (picked) {
                const base = { ...firstResult, frame_id: picked.frameId };
                const matched = base["matched"];
                if (matched && typeof matched === 'object' && !Array.isArray(matched)) {
                    base["matched"] = { ...matched, frame_id: picked.frameId };
                }
                resultPayload = base;
            }
            else {
                resultPayload = firstResult;
            }
            const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(action, selector || '', resultPayload);
            const domResult = toDOMResult(reconciledResult);
            if (domResult) {
                sendToastForResult(tabId, readOnly, domResult, actionToast, toastLabel, toastDetail);
            }
            else if (!readOnly && status === 'error') {
                actionToast(tabId, toastLabel, error || 'failed', 'error');
            }
            sendAsyncResult(syncClient, query.id, query.correlation_id, status, await enrichWithEffectiveContext(tabId, reconciledResult), error);
        }
        else {
            if (!readOnly)
                actionToast(tabId, toastLabel, 'no result', 'error');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'no_result');
        }
    }
    catch (err) {
        actionToast(tabId, action, err.message, 'error');
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, err.message);
    }
}
//# sourceMappingURL=dom-dispatch.js.map