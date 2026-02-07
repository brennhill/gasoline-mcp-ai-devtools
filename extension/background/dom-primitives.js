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
    function resolveElement(sel) {
        if (!sel)
            return null;
        // text=Submit → find visible element whose textContent contains the text
        if (sel.startsWith('text=')) {
            const searchText = sel.slice(5);
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
        // role=button → [role="button"], prefer visible
        if (sel.startsWith('role=')) {
            const role = sel.slice(5);
            return firstVisible(document.querySelectorAll(`[role="${role}"]`));
        }
        // placeholder=Email → [placeholder="Email"], prefer visible
        if (sel.startsWith('placeholder=')) {
            const ph = sel.slice(12);
            return firstVisible(document.querySelectorAll(`[placeholder="${ph}"]`));
        }
        // label=Email → find label, follow `for` attribute
        if (sel.startsWith('label=')) {
            const labelText = sel.slice(6);
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
        // aria-label=Close → starts-with match, prefer visible
        // Handles cases like Gmail's "Send ‪(⌘Enter)‬" matching "aria-label=Send"
        if (sel.startsWith('aria-label=')) {
            const al = sel.slice(11);
            // Try exact match first
            const exact = document.querySelectorAll(`[aria-label="${al}"]`);
            if (exact.length > 0)
                return firstVisible(exact);
            // Starts-with match: find all [aria-label] and check prefix
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
        // Default: CSS selector
        return document.querySelector(sel);
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
            '[tabindex]',
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
                // Build a unique selector for this element
                let uniqueSelector = '';
                if (el.id) {
                    uniqueSelector = `#${el.id}`;
                }
                else if (el instanceof HTMLInputElement && el.name) {
                    uniqueSelector = `input[name="${el.name}"]`;
                }
                else {
                    // Use aria-label, placeholder, or text content as selector hint
                    const ariaLabel = el.getAttribute('aria-label');
                    const placeholder = el.getAttribute('placeholder');
                    if (ariaLabel) {
                        uniqueSelector = `aria-label=${ariaLabel}`;
                    }
                    else if (placeholder) {
                        uniqueSelector = `placeholder=${placeholder}`;
                    }
                    else {
                        const text = (htmlEl.textContent || '').trim().slice(0, 40);
                        if (text) {
                            uniqueSelector = `text=${text}`;
                        }
                        else {
                            uniqueSelector = cssSelector;
                        }
                    }
                }
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
                    visible,
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
            message: `No element matches selector: ${selector}`,
        };
    }
    // ---------------------------------------------------------------
    // Action dispatch
    // ---------------------------------------------------------------
    switch (action) {
        case 'click': {
            if (!(el instanceof HTMLElement)) {
                return { success: false, action, selector, error: 'not_interactive', message: `Element is not an HTMLElement: ${el.tagName}` };
            }
            el.click();
            return { success: true, action, selector };
        }
        case 'type': {
            const text = options.text || '';
            // Contenteditable elements (Gmail compose body, rich text editors)
            // Use execCommand('insertText') — fires beforeinput/input events properly,
            // integrates with undo/redo, and works with Gmail's editor framework.
            if (el instanceof HTMLElement && el.isContentEditable) {
                el.focus();
                if (options.clear) {
                    // Select all then replace
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
                return { success: false, action, selector, error: 'not_typeable', message: `Element is not an input, textarea, or contenteditable: ${el.tagName}` };
            }
            // Use native prototype setter to trigger React/Vue/Angular state updates
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
        }
        case 'select': {
            if (!(el instanceof HTMLSelectElement)) {
                return { success: false, action, selector, error: 'not_select', message: `Element is not a <select>: ${el.tagName}` };
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
        }
        case 'check': {
            if (!(el instanceof HTMLInputElement) || (el.type !== 'checkbox' && el.type !== 'radio')) {
                return {
                    success: false,
                    action,
                    selector,
                    error: 'not_checkable',
                    message: `Element is not a checkbox or radio: ${el.tagName} type=${el.type || 'N/A'}`,
                };
            }
            const desired = options.checked !== undefined ? options.checked : true;
            // Use .click() for React compatibility — toggles checked and fires all events
            if (el.checked !== desired) {
                el.click();
            }
            return { success: true, action, selector, value: el.checked };
        }
        case 'get_text': {
            return { success: true, action, selector, value: el.textContent };
        }
        case 'get_value': {
            if (!('value' in el)) {
                return { success: false, action, selector, error: 'no_value_property', message: `Element has no value property: ${el.tagName}` };
            }
            return { success: true, action, selector, value: el.value };
        }
        case 'get_attribute': {
            return { success: true, action, selector, value: el.getAttribute(options.name || '') };
        }
        case 'set_attribute': {
            el.setAttribute(options.name || '', options.value || '');
            return { success: true, action, selector, value: el.getAttribute(options.name || '') };
        }
        case 'focus': {
            if (!(el instanceof HTMLElement)) {
                return { success: false, action, selector, error: 'not_focusable', message: `Element is not an HTMLElement: ${el.tagName}` };
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
            if (!(el instanceof HTMLElement)) {
                return { success: false, action, selector, error: 'not_interactive', message: `Element is not an HTMLElement: ${el.tagName}` };
            }
            const key = options.text || 'Enter';
            const keyMap = {
                Enter: { key: 'Enter', code: 'Enter', keyCode: 13 },
                Tab: { key: 'Tab', code: 'Tab', keyCode: 9 },
                Escape: { key: 'Escape', code: 'Escape', keyCode: 27 },
                Backspace: { key: 'Backspace', code: 'Backspace', keyCode: 8 },
                ArrowDown: { key: 'ArrowDown', code: 'ArrowDown', keyCode: 40 },
                ArrowUp: { key: 'ArrowUp', code: 'ArrowUp', keyCode: 38 },
                Space: { key: ' ', code: 'Space', keyCode: 32 },
            };
            const mapped = keyMap[key] || { key, code: key, keyCode: 0 };
            el.dispatchEvent(new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
            el.dispatchEvent(new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
            el.dispatchEvent(new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
            return { success: true, action, selector, value: key };
        }
        default:
            return { success: false, action, selector, error: 'unknown_action', message: `Unknown DOM action: ${action}` };
    }
}
/**
 * wait_for variant that polls with MutationObserver (used when element not found initially).
 * Separate function because it returns a Promise.
 */
export function domWaitFor(selector, timeoutMs) {
    // ---------------------------------------------------------------
    // Inline selector resolver (must be self-contained for chrome.scripting)
    // ---------------------------------------------------------------
    function resolveElement(sel) {
        if (!sel)
            return null;
        if (sel.startsWith('text=')) {
            const searchText = sel.slice(5);
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
        if (sel.startsWith('role='))
            return document.querySelector(`[role="${sel.slice(5)}"]`);
        if (sel.startsWith('placeholder='))
            return document.querySelector(`[placeholder="${sel.slice(12)}"]`);
        if (sel.startsWith('aria-label='))
            return document.querySelector(`[aria-label="${sel.slice(11)}"]`);
        if (sel.startsWith('label=')) {
            const labelText = sel.slice(6);
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
                    message: `Element not found within ${timeoutMs}ms: ${selector}`,
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
// =============================================================================
// Dispatcher: routes dom_action queries to pre-compiled functions
// =============================================================================
export async function executeDOMAction(query, tabId, syncClient, sendAsyncResult, actionToast) {
    let params;
    try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
    }
    catch {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'invalid_params');
        return;
    }
    const { action, selector, reason } = params;
    if (!action) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_action');
        return;
    }
    // Toast detail: show reason if provided, otherwise selector
    const toastDetail = reason || selector || 'page';
    try {
        let results;
        if (action === 'wait_for' && !selector) {
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'missing_selector');
            return;
        }
        // Show "trying" toast (skipped for list_interactive and read-only gets)
        const readOnly = action === 'list_interactive' || action.startsWith('get_');
        const tryingShownAt = Date.now();
        if (!readOnly) {
            actionToast(tabId, action, toastDetail, 'trying', 10000);
        }
        // For wait_for: check if element exists first with domPrimitive.
        // If not found, use the async domWaitFor with MutationObserver.
        if (action === 'wait_for') {
            const quickCheck = await chrome.scripting.executeScript({
                target: { tabId },
                world: 'MAIN',
                func: domPrimitive,
                args: [action, selector || '', { timeout_ms: params.timeout_ms }],
            });
            const quickResult = quickCheck?.[0]?.result;
            if (quickResult && quickResult.success) {
                actionToast(tabId, 'wait_for', toastDetail, 'success');
                sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', quickResult);
                return;
            }
            // Element not found — use async MutationObserver version
            results = await chrome.scripting.executeScript({
                target: { tabId },
                world: 'MAIN',
                func: domWaitFor,
                args: [selector || '', params.timeout_ms || 5000],
            });
        }
        else {
            results = await chrome.scripting.executeScript({
                target: { tabId },
                world: 'MAIN',
                func: domPrimitive,
                args: [action, selector || '', {
                        text: params.text,
                        value: params.value,
                        clear: params.clear,
                        checked: params.checked,
                        name: params.name,
                        timeout_ms: params.timeout_ms,
                    }],
            });
        }
        // Ensure the "trying" toast is visible for at least 500ms before replacing
        const MIN_TOAST_MS = 500;
        const elapsed = Date.now() - tryingShownAt;
        if (!readOnly && elapsed < MIN_TOAST_MS) {
            await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed));
        }
        const firstResult = results?.[0]?.result;
        if (firstResult && typeof firstResult === 'object') {
            const result = firstResult;
            if (!readOnly) {
                if (result.success) {
                    actionToast(tabId, action, toastDetail, 'success');
                }
                else {
                    actionToast(tabId, action, result.error || 'failed', 'error');
                }
            }
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', firstResult);
        }
        else {
            if (!readOnly) {
                actionToast(tabId, action, 'no result', 'error');
            }
            sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, 'no_result');
        }
    }
    catch (err) {
        actionToast(tabId, action, err.message, 'error');
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, err.message);
    }
}
//# sourceMappingURL=dom-primitives.js.map