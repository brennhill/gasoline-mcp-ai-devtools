// dom-primitives.ts — Pre-compiled DOM interaction functions for chrome.scripting.executeScript.
// These bypass CSP restrictions because they use the `func` parameter (no eval/new Function).
// Each function MUST be self-contained — no closures over external variables.
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitive(action, selector, options) {
    // ---------------------------------------------------------------
    // Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=)
    // All semantic selectors prefer visible elements over hidden ones.
    // ---------------------------------------------------------------
    // Visibility check: skip display:none, visibility:hidden, zero-size elements
    function isVisible(el) {
        if (!(el instanceof HTMLElement))
            return true;
        if (el.offsetParent === null && el.style.position !== 'fixed' && el.style.position !== 'sticky')
            return false;
        const style = getComputedStyle(el);
        if (style.visibility === 'hidden' || style.display === 'none')
            return false;
        return true;
    }
    // Return first visible match from a NodeList, falling back to first match
    function firstVisible(els) {
        let fallback = null;
        for (const el of els) {
            if (!fallback)
                fallback = el;
            if (isVisible(el))
                return el;
        }
        return fallback;
    }
    function resolveByText(searchText) {
        const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_TEXT);
        let fallback = null;
        while (walker.nextNode()) {
            const node = walker.currentNode;
            if (node.textContent && node.textContent.trim().includes(searchText)) {
                const parent = node.parentElement;
                if (!parent)
                    continue;
                const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary');
                const target = interactive || parent;
                if (!fallback)
                    fallback = target;
                if (isVisible(target))
                    return target;
            }
        }
        return fallback;
    }
    function resolveByLabel(labelText) {
        const labels = document.querySelectorAll('label');
        for (const label of labels) {
            if (label.textContent && label.textContent.trim().includes(labelText)) {
                const forAttr = label.getAttribute('for');
                if (forAttr) {
                    const target = document.getElementById(forAttr);
                    if (target)
                        return target;
                }
                const nested = label.querySelector('input, select, textarea');
                if (nested)
                    return nested;
                return label;
            }
        }
        return null;
    }
    function resolveByAriaLabel(al) {
        const exact = document.querySelectorAll(`[aria-label="${CSS.escape(al)}"]`);
        if (exact.length > 0)
            return firstVisible(exact);
        const all = document.querySelectorAll('[aria-label]');
        let fallback = null;
        for (const el of all) {
            const label = el.getAttribute('aria-label') || '';
            if (label.startsWith(al)) {
                if (!fallback)
                    fallback = el;
                if (isVisible(el))
                    return el;
            }
        }
        return fallback;
    }
    // Semantic selector prefix resolvers
    const selectorResolvers = [
        ['text=', (v) => resolveByText(v)],
        ['role=', (v) => firstVisible(document.querySelectorAll(`[role="${CSS.escape(v)}"]`))],
        ['placeholder=', (v) => firstVisible(document.querySelectorAll(`[placeholder="${CSS.escape(v)}"]`))],
        ['label=', (v) => resolveByLabel(v)],
        ['aria-label=', (v) => resolveByAriaLabel(v)]
    ];
    function resolveElement(sel) {
        if (!sel)
            return null;
        for (const [prefix, resolver] of selectorResolvers) {
            if (sel.startsWith(prefix))
                return resolver(sel.slice(prefix.length));
        }
        return document.querySelector(sel);
    }
    function buildUniqueSelector(el, htmlEl, fallbackSelector) {
        if (el.id)
            return `#${el.id}`;
        if (el instanceof HTMLInputElement && el.name)
            return `input[name="${el.name}"]`;
        const ariaLabel = el.getAttribute('aria-label');
        if (ariaLabel)
            return `aria-label=${ariaLabel}`;
        const placeholder = el.getAttribute('placeholder');
        if (placeholder)
            return `placeholder=${placeholder}`;
        const text = (htmlEl.textContent || '').trim().slice(0, 40);
        if (text)
            return `text=${text}`;
        return fallbackSelector;
    }
    // ---------------------------------------------------------------
    // list_interactive: scan the page for interactive elements
    // ---------------------------------------------------------------
    if (action === 'list_interactive') {
        const interactiveSelectors = [
            'a[href]',
            'button',
            'input',
            'select',
            'textarea',
            '[role="button"]',
            '[role="link"]',
            '[role="tab"]',
            '[role="menuitem"]',
            '[contenteditable="true"]',
            '[onclick]',
            '[tabindex]'
        ];
        const seen = new Set();
        const elements = [];
        for (const cssSelector of interactiveSelectors) {
            const matches = document.querySelectorAll(cssSelector);
            for (const el of matches) {
                if (seen.has(el))
                    continue;
                seen.add(el);
                const htmlEl = el;
                const rect = htmlEl.getBoundingClientRect();
                const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
                const uniqueSelector = buildUniqueSelector(el, htmlEl, cssSelector);
                // Build human-readable label
                const label = el.getAttribute('aria-label') ||
                    el.getAttribute('title') ||
                    el.getAttribute('placeholder') ||
                    (htmlEl.textContent || '').trim().slice(0, 60) ||
                    el.tagName.toLowerCase();
                elements.push({
                    tag: el.tagName.toLowerCase(),
                    type: el instanceof HTMLInputElement ? el.type : undefined,
                    selector: uniqueSelector,
                    label,
                    role: el.getAttribute('role') || undefined,
                    placeholder: el.getAttribute('placeholder') || undefined,
                    visible
                });
                if (elements.length >= 100)
                    break; // Cap at 100 elements
            }
            if (elements.length >= 100)
                break;
        }
        return { success: true, elements };
    }
    // ---------------------------------------------------------------
    // Resolve element for all other actions
    // ---------------------------------------------------------------
    const el = resolveElement(selector);
    if (!el) {
        return {
            success: false,
            action,
            selector,
            error: 'element_not_found',
            message: `No element matches selector: ${selector}`
        };
    }
    // ---------------------------------------------------------------
    // Mutation tracking: wraps an action with MutationObserver to capture DOM changes.
    // Returns a compact dom_summary (always) and detailed dom_changes (when analyze:true).
    // ---------------------------------------------------------------
    function withMutationTracking(fn) {
        const t0 = performance.now();
        const mutations = [];
        const observer = new MutationObserver((records) => {
            mutations.push(...records);
        });
        observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true
        });
        const result = fn();
        if (!result.success) {
            observer.disconnect();
            return Promise.resolve(result);
        }
        return new Promise((resolve) => {
            let resolved = false;
            function finish() {
                if (resolved)
                    return;
                resolved = true;
                observer.disconnect();
                const totalMs = Math.round(performance.now() - t0);
                const added = mutations.reduce((s, m) => s + m.addedNodes.length, 0);
                const removed = mutations.reduce((s, m) => s + m.removedNodes.length, 0);
                const modified = mutations.filter((m) => m.type === 'attributes').length;
                const parts = [];
                if (added > 0)
                    parts.push(`${added} added`);
                if (removed > 0)
                    parts.push(`${removed} removed`);
                if (modified > 0)
                    parts.push(`${modified} modified`);
                const summary = parts.length > 0 ? parts.join(', ') : 'no DOM changes';
                const enriched = { ...result, dom_summary: summary };
                if (options.analyze) {
                    enriched.timing = { total_ms: totalMs };
                    enriched.dom_changes = { added, removed, modified, summary };
                    enriched.analysis = `${result.action} completed in ${totalMs}ms. ${summary}.`;
                }
                resolve(enriched);
            }
            // setTimeout fallback — always fires, even in backgrounded/headless tabs
            // where requestAnimationFrame is suppressed
            setTimeout(finish, 80);
            // Try rAF for better timing when tab is visible, but don't depend on it
            if (typeof requestAnimationFrame === 'function') {
                requestAnimationFrame(() => setTimeout(finish, 50));
            }
        });
    }
    // ---------------------------------------------------------------
    // Action dispatch
    // ---------------------------------------------------------------
    switch (action) {
        case 'click': {
            return withMutationTracking(() => {
                if (!(el instanceof HTMLElement)) {
                    return {
                        success: false,
                        action,
                        selector,
                        error: 'not_interactive',
                        message: `Element is not an HTMLElement: ${el.tagName}`
                    };
                }
                el.click();
                return { success: true, action, selector };
            });
        }
        case 'type': {
            return withMutationTracking(() => {
                const text = options.text || '';
                // Contenteditable elements (Gmail compose body, rich text editors)
                if (el instanceof HTMLElement && el.isContentEditable) {
                    el.focus();
                    if (options.clear) {
                        const selection = document.getSelection();
                        if (selection) {
                            selection.selectAllChildren(el);
                            selection.deleteFromDocument();
                        }
                    }
                    document.execCommand('insertText', false, text);
                    return { success: true, action, selector, value: el.textContent };
                }
                if (!(el instanceof HTMLInputElement) && !(el instanceof HTMLTextAreaElement)) {
                    return {
                        success: false,
                        action,
                        selector,
                        error: 'not_typeable',
                        message: `Element is not an input, textarea, or contenteditable: ${el.tagName}`
                    };
                }
                const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement;
                const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set;
                if (nativeSetter) {
                    const newValue = options.clear ? text : el.value + text;
                    nativeSetter.call(el, newValue);
                }
                else {
                    el.value = options.clear ? text : el.value + text;
                }
                el.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }));
                el.dispatchEvent(new Event('change', { bubbles: true }));
                return { success: true, action, selector, value: el.value };
            });
        }
        case 'select': {
            return withMutationTracking(() => {
                if (!(el instanceof HTMLSelectElement)) {
                    return {
                        success: false,
                        action,
                        selector,
                        error: 'not_select',
                        message: `Element is not a <select>: ${el.tagName}` // nosemgrep: html-in-template-string
                    };
                }
                const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set;
                if (nativeSelectSetter) {
                    nativeSelectSetter.call(el, options.value || '');
                }
                else {
                    el.value = options.value || '';
                }
                el.dispatchEvent(new Event('change', { bubbles: true }));
                return { success: true, action, selector, value: el.value };
            });
        }
        case 'check': {
            return withMutationTracking(() => {
                if (!(el instanceof HTMLInputElement) || (el.type !== 'checkbox' && el.type !== 'radio')) {
                    return {
                        success: false,
                        action,
                        selector,
                        error: 'not_checkable',
                        message: `Element is not a checkbox or radio: ${el.tagName} type=${el.type || 'N/A'}`
                    };
                }
                const desired = options.checked !== undefined ? options.checked : true;
                if (el.checked !== desired) {
                    el.click();
                }
                return { success: true, action, selector, value: el.checked };
            });
        }
        case 'get_text': {
            return { success: true, action, selector, value: el.textContent };
        }
        case 'get_value': {
            if (!('value' in el)) {
                return {
                    success: false,
                    action,
                    selector,
                    error: 'no_value_property',
                    message: `Element has no value property: ${el.tagName}`
                };
            }
            return { success: true, action, selector, value: el.value };
        }
        case 'get_attribute': {
            return { success: true, action, selector, value: el.getAttribute(options.name || '') };
        }
        case 'set_attribute': {
            return withMutationTracking(() => {
                el.setAttribute(options.name || '', options.value || '');
                return { success: true, action, selector, value: el.getAttribute(options.name || '') };
            });
        }
        case 'focus': {
            if (!(el instanceof HTMLElement)) {
                return {
                    success: false,
                    action,
                    selector,
                    error: 'not_focusable',
                    message: `Element is not an HTMLElement: ${el.tagName}`
                };
            }
            el.focus();
            return { success: true, action, selector };
        }
        case 'scroll_to': {
            el.scrollIntoView({ behavior: 'smooth', block: 'center' });
            return { success: true, action, selector };
        }
        case 'wait_for': {
            // Already found — return immediately
            return { success: true, action, selector, value: el.tagName.toLowerCase() };
        }
        case 'key_press': {
            return withMutationTracking(() => {
                if (!(el instanceof HTMLElement)) {
                    return {
                        success: false,
                        action,
                        selector,
                        error: 'not_interactive',
                        message: `Element is not an HTMLElement: ${el.tagName}`
                    };
                }
                const key = options.text || 'Enter';
                // Tab/Shift+Tab: manually move focus (dispatchEvent can't trigger native tab traversal)
                if (key === 'Tab' || key === 'Shift+Tab') {
                    const focusable = Array.from(el.ownerDocument.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((e) => e.offsetParent !== null);
                    const idx = focusable.indexOf(el);
                    const next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1];
                    if (next) {
                        next.focus();
                        return { success: true, action, selector, value: key };
                    }
                    return { success: true, action, selector, value: key, message: 'No next focusable element' };
                }
                const keyMap = {
                    Enter: { key: 'Enter', code: 'Enter', keyCode: 13 },
                    Tab: { key: 'Tab', code: 'Tab', keyCode: 9 },
                    Escape: { key: 'Escape', code: 'Escape', keyCode: 27 },
                    Backspace: { key: 'Backspace', code: 'Backspace', keyCode: 8 },
                    ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
                    ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
                    Space: { key: ' ', code: 'Space', keyCode: 32 }
                };
                const mapped = keyMap[key] || { key, code: key, keyCode: 0 };
                el.dispatchEvent(new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                el.dispatchEvent(new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                el.dispatchEvent(new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                return { success: true, action, selector, value: key };
            });
        }
        default:
            return { success: false, action, selector, error: 'unknown_action', message: `Unknown DOM action: ${action}` };
    }
}
/**
 * wait_for variant that polls with MutationObserver (used when element not found initially).
 * Separate function because it returns a Promise.
 */
// #lizard forgives
export function domWaitFor(selector, timeoutMs) {
    // ---------------------------------------------------------------
    // Inline selector resolver (must be self-contained for chrome.scripting)
    // ---------------------------------------------------------------
    // #lizard forgives
    function resolveByTextSimple(searchText) {
        const walker = document.createTreeWalker(document.body || document.documentElement, NodeFilter.SHOW_TEXT);
        while (walker.nextNode()) {
            const node = walker.currentNode;
            if (node.textContent && node.textContent.trim().includes(searchText)) {
                const parent = node.parentElement;
                if (!parent)
                    continue;
                return parent.closest('a, button, [role="button"], [role="link"], label, summary') || parent;
            }
        }
        return null;
    }
    function resolveByLabelSimple(labelText) {
        for (const label of document.querySelectorAll('label')) {
            if (label.textContent && label.textContent.trim().includes(labelText)) {
                const forAttr = label.getAttribute('for');
                if (forAttr) {
                    const t = document.getElementById(forAttr);
                    if (t)
                        return t;
                }
                return label.querySelector('input, select, textarea') || label;
            }
        }
        return null;
    }
    const waitResolvers = [
        ['text=', (v) => resolveByTextSimple(v)],
        ['role=', (v) => document.querySelector(`[role="${CSS.escape(v)}"]`)],
        ['placeholder=', (v) => document.querySelector(`[placeholder="${CSS.escape(v)}"]`)],
        ['aria-label=', (v) => document.querySelector(`[aria-label="${CSS.escape(v)}"]`)],
        ['label=', (v) => resolveByLabelSimple(v)]
    ];
    function resolveElement(sel) {
        if (!sel)
            return null;
        for (const [prefix, resolver] of waitResolvers) {
            if (sel.startsWith(prefix))
                return resolver(sel.slice(prefix.length));
        }
        return document.querySelector(sel);
    }
    return new Promise((resolve) => {
        // Check immediately
        const existing = resolveElement(selector);
        if (existing) {
            resolve({ success: true, action: 'wait_for', selector, value: existing.tagName.toLowerCase() });
            return;
        }
        let resolved = false;
        const timer = setTimeout(() => {
            if (!resolved) {
                resolved = true;
                observer.disconnect();
                resolve({
                    success: false,
                    action: 'wait_for',
                    selector,
                    error: 'timeout',
                    message: `Element not found within ${timeoutMs}ms: ${selector}`
                });
            }
        }, timeoutMs);
        const observer = new MutationObserver(() => {
            const el = resolveElement(selector);
            if (el && !resolved) {
                resolved = true;
                clearTimeout(timer);
                observer.disconnect();
                resolve({ success: true, action: 'wait_for', selector, value: el.tagName.toLowerCase() });
            }
        });
        observer.observe(document.documentElement, { childList: true, subtree: true });
    });
}
function parseDOMParams(query) {
    try {
        return typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        return null;
    }
}
function isReadOnlyAction(action) {
    return action === 'list_interactive' || action.startsWith('get_');
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
/**
 * Frame-matching probe executed in page context.
 * Must stay self-contained for chrome.scripting.executeScript({ func }).
 */
export function domFrameProbe(frameTarget) {
    const isTop = window === window.top;
    const getParentFrameIndex = () => {
        if (isTop)
            return -1;
        try {
            const parentFrames = window.parent?.frames;
            if (!parentFrames)
                return -1;
            for (let i = 0; i < parentFrames.length; i++) {
                if (parentFrames[i] === window)
                    return i;
            }
        }
        catch {
            return -1;
        }
        return -1;
    };
    if (typeof frameTarget === 'number') {
        return { matches: getParentFrameIndex() === frameTarget };
    }
    if (frameTarget === 'all') {
        return { matches: true };
    }
    if (isTop) {
        return { matches: false };
    }
    try {
        const frameEl = window.frameElement;
        if (!frameEl || typeof frameEl.matches !== 'function') {
            return { matches: false };
        }
        return { matches: frameEl.matches(frameTarget) };
    }
    catch {
        return { matches: false };
    }
}
async function resolveExecutionTarget(tabId, frame) {
    const normalized = normalizeFrameTarget(frame);
    if (normalized === null) {
        throw new Error('invalid_frame');
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
        throw new Error('frame_not_found');
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
    for (const r of results) {
        const res = r.result;
        if (res?.elements)
            elements.push(...res.elements);
        if (elements.length >= 100)
            break;
    }
    return { success: true, elements: elements.slice(0, 100) };
}
async function executeWaitFor(target, params) {
    const selector = params.selector || '';
    const quickCheck = await chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domPrimitive,
        args: [params.action, selector, { timeout_ms: params.timeout_ms }]
    });
    const quickPicked = pickFrameResult(quickCheck);
    const quickResult = quickPicked?.result;
    if (quickResult?.success)
        return quickResult;
    return chrome.scripting.executeScript({
        target,
        world: 'MAIN',
        func: domWaitFor,
        args: [selector, params.timeout_ms || 5000]
    });
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
                value: params.value,
                clear: params.clear,
                checked: params.checked,
                name: params.name,
                timeout_ms: params.timeout_ms,
                analyze: params.analyze
            }
        ]
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
    if (action === 'wait_for' && !selector) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_selector');
        return;
    }
    const toastLabel = reason || action;
    const toastDetail = reason ? undefined : selector || 'page';
    const readOnly = isReadOnlyAction(action);
    // Enrich successful results with effective tab context (post-execution URL).
    // Agents compare resolved_url (dispatch time) vs effective_url (execution time) to detect drift.
    const enrichWithEffectiveContext = async (result) => {
        try {
            const tab = await chrome.tabs.get(tabId);
            if (result && typeof result === 'object' && !Array.isArray(result)) {
                return { ...result, effective_tab_id: tabId, effective_url: tab.url };
            }
            return result;
        }
        catch {
            return result;
        }
    };
    try {
        const executionTarget = await resolveExecutionTarget(tabId, params.frame);
        const tryingShownAt = Date.now();
        if (!readOnly)
            actionToast(tabId, toastLabel, toastDetail, 'trying', 10000);
        const rawResult = action === 'wait_for'
            ? await executeWaitFor(executionTarget, params)
            : await executeStandardAction(executionTarget, params);
        // wait_for quick-check can return a DOMResult directly
        if (!Array.isArray(rawResult)) {
            if (!readOnly)
                actionToast(tabId, toastLabel, toastDetail, 'success');
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(rawResult));
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
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(merged));
            return;
        }
        const picked = pickFrameResult(rawResult);
        const firstResult = picked?.result;
        if (firstResult && typeof firstResult === 'object') {
            const resultPayload = params.frame !== undefined && params.frame !== null && picked
                ? { ...firstResult, frame_id: picked.frameId }
                : firstResult;
            sendToastForResult(tabId, readOnly, resultPayload, actionToast, toastLabel, toastDetail);
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', await enrichWithEffectiveContext(resultPayload));
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
//# sourceMappingURL=dom-primitives.js.map