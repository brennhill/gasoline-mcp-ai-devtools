// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source template: scripts/templates/dom-primitives.ts.tpl
// Generator: scripts/generate-dom-primitives.js
// Re-export list_interactive primitive for backward compatibility
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export function domPrimitive(action, selector, options) {
    // — Shadow DOM: deep traversal utilities —
    function getShadowRoot(el) {
        return el.shadowRoot ?? null;
        // Closed root support: see feat/closed-shadow-capture branch
    }
    function querySelectorDeep(selector, root = document) {
        const fast = root.querySelector(selector);
        if (fast)
            return fast;
        return querySelectorDeepWalk(selector, root);
    }
    function querySelectorDeepWalk(selector, root, depth = 0) {
        if (depth > 10)
            return null;
        // Navigate to children: handle Document (has body/documentElement) and Element/ShadowRoot (has children)
        const children = 'children' in root
            ? root.children
            : root.body?.children || root.documentElement?.children;
        if (!children)
            return null;
        for (let i = 0; i < children.length; i++) {
            const child = children[i];
            const shadow = getShadowRoot(child);
            if (shadow) {
                const match = shadow.querySelector(selector);
                if (match)
                    return match;
                const deep = querySelectorDeepWalk(selector, shadow, depth + 1);
                if (deep)
                    return deep;
            }
            if (child.children.length > 0) {
                const deep = querySelectorDeepWalk(selector, child, depth + 1);
                if (deep)
                    return deep;
            }
        }
        return null;
    }
    function querySelectorAllDeep(selector, root = document, results = [], depth = 0) {
        if (depth > 10)
            return results;
        results.push(...Array.from(root.querySelectorAll(selector)));
        const children = 'children' in root
            ? root.children
            : root.body?.children || root.documentElement?.children;
        if (!children)
            return results;
        for (let i = 0; i < children.length; i++) {
            const child = children[i];
            const shadow = getShadowRoot(child);
            if (shadow) {
                querySelectorAllDeep(selector, shadow, results, depth + 1);
            }
        }
        return results;
    }
    function resolveDeepCombinator(selector, root = document) {
        const parts = selector.split(' >>> ');
        if (parts.length <= 1)
            return null;
        let current = root;
        for (let i = 0; i < parts.length; i++) {
            const part = parts[i].trim();
            if (i < parts.length - 1) {
                const host = querySelectorDeep(part, current);
                if (!host)
                    return null;
                const shadow = getShadowRoot(host);
                if (!shadow)
                    return null;
                current = shadow;
            }
            else {
                return querySelectorDeep(part, current);
            }
        }
        return null;
    }
    // — Selector resolver: CSS or semantic (text=, role=, placeholder=, label=, aria-label=) —
    // Visibility check: skip display:none, visibility:hidden, zero-size elements
    function isVisible(el) {
        if (!(el instanceof HTMLElement))
            return true;
        const style = getComputedStyle(el);
        if (style.visibility === 'hidden' || style.display === 'none')
            return false;
        if (el.offsetParent === null && style.position !== 'fixed' && style.position !== 'sticky') {
            const rect = el.getBoundingClientRect();
            if (rect.width === 0 && rect.height === 0)
                return false;
        }
        return true;
    }
    // Return first visible match from a list, falling back to first match
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
    function resolveScopeRoot(rawScope) {
        const scope = (rawScope || '').trim();
        if (!scope)
            return document;
        try {
            const matches = querySelectorAllDeep(scope);
            if (matches.length === 0)
                return null;
            return firstVisible(matches) || matches[0] || null;
        }
        catch {
            return null;
        }
    }
    const scopeRoot = resolveScopeRoot(options.scope_selector);
    function getElementHandleStore() {
        const root = globalThis;
        if (root.__gasolineElementHandles) {
            return root.__gasolineElementHandles;
        }
        const created = {
            byElement: new WeakMap(),
            byID: new Map(),
            nextID: 1
        };
        root.__gasolineElementHandles = created;
        return created;
    }
    function getOrCreateElementID(el) {
        const store = getElementHandleStore();
        const existing = store.byElement.get(el);
        if (existing) {
            store.byID.set(existing, el);
            return existing;
        }
        const elementID = `el_${(store.nextID++).toString(36)}`;
        store.byElement.set(el, elementID);
        store.byID.set(elementID, el);
        return elementID;
    }
    function resolveElementByID(rawElementID) {
        const elementID = (rawElementID || '').trim();
        if (!elementID)
            return null;
        const store = getElementHandleStore();
        const node = store.byID.get(elementID);
        if (!node)
            return null;
        if (node.isConnected === false) {
            store.byID.delete(elementID);
            return null;
        }
        return node;
    }
    function resolveByTextAll(searchText, scope = document) {
        const results = [];
        const seen = new Set();
        function walkScope(root) {
            const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
            while (walker.nextNode()) {
                const node = walker.currentNode;
                if (node.textContent && node.textContent.trim().includes(searchText)) {
                    const parent = node.parentElement;
                    if (!parent)
                        continue;
                    const interactive = parent.closest('a, button, [role="button"], [role="link"], label, summary');
                    const target = interactive || parent;
                    if (!seen.has(target)) {
                        seen.add(target);
                        results.push(target);
                    }
                }
            }
            const children = 'children' in root
                ? root.children
                : root.body?.children || root.documentElement?.children;
            if (children) {
                for (let i = 0; i < children.length; i++) {
                    const child = children[i];
                    const shadow = getShadowRoot(child);
                    if (shadow)
                        walkScope(shadow);
                }
            }
        }
        walkScope(scope);
        return results;
    }
    function resolveByLabelAll(labelText, scope = document) {
        const labels = querySelectorAllDeep('label', scope);
        const results = [];
        const seen = new Set();
        const allowGlobalIdLookup = scope === document || scope === document.body || scope === document.documentElement;
        for (const label of labels) {
            if (label.textContent && label.textContent.trim().includes(labelText)) {
                const forAttr = label.getAttribute('for');
                if (forAttr) {
                    const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0];
                    const target = local || (allowGlobalIdLookup ? document.getElementById(forAttr) : null);
                    if (target && !seen.has(target)) {
                        seen.add(target);
                        results.push(target);
                    }
                }
                const nested = label.querySelector('input, select, textarea');
                if (nested && !seen.has(nested)) {
                    seen.add(nested);
                    results.push(nested);
                }
                if (!seen.has(label)) {
                    seen.add(label);
                    results.push(label);
                }
            }
        }
        return results;
    }
    function resolveByAriaLabelAll(al, scope = document) {
        const results = [];
        const seen = new Set();
        const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope);
        for (const el of exact) {
            if (!seen.has(el)) {
                seen.add(el);
                results.push(el);
            }
        }
        const all = querySelectorAllDeep('[aria-label]', scope);
        for (const el of all) {
            const label = el.getAttribute('aria-label') || '';
            if (label.startsWith(al) && !seen.has(el)) {
                seen.add(el);
                results.push(el);
            }
        }
        return results;
    }
    function resolveByText(searchText, scope = document) {
        let fallback = null;
        function walkScope(root) {
            const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
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
            const children = 'children' in root
                ? root.children
                : root.body?.children || root.documentElement?.children;
            if (children) {
                for (let i = 0; i < children.length; i++) {
                    const child = children[i];
                    const shadow = getShadowRoot(child);
                    if (shadow) {
                        const result = walkScope(shadow);
                        if (result)
                            return result;
                    }
                }
            }
            return null;
        }
        return walkScope(scope) || fallback;
    }
    function resolveByLabel(labelText, scope = document) {
        const labels = querySelectorAllDeep('label', scope);
        const allowGlobalIdLookup = scope === document || scope === document.body || scope === document.documentElement;
        for (const label of labels) {
            if (label.textContent && label.textContent.trim().includes(labelText)) {
                const forAttr = label.getAttribute('for');
                if (forAttr) {
                    const local = querySelectorAllDeep(`#${CSS.escape(forAttr)}`, scope)[0];
                    if (local)
                        return local;
                    const target = allowGlobalIdLookup ? document.getElementById(forAttr) : null;
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
    function resolveByAriaLabel(al, scope = document) {
        const exact = querySelectorAllDeep(`[aria-label="${CSS.escape(al)}"]`, scope);
        if (exact.length > 0)
            return firstVisible(exact);
        const all = querySelectorAllDeep('[aria-label]', scope);
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
    function resolveElements(sel, scope = document) {
        if (!sel)
            return [];
        if (sel.startsWith('text='))
            return resolveByTextAll(sel.slice('text='.length), scope);
        if (sel.startsWith('role='))
            return querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope);
        if (sel.startsWith('placeholder='))
            return querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope);
        if (sel.startsWith('label='))
            return resolveByLabelAll(sel.slice('label='.length), scope);
        if (sel.startsWith('aria-label='))
            return resolveByAriaLabelAll(sel.slice('aria-label='.length), scope);
        try {
            return querySelectorAllDeep(sel, scope);
        }
        catch {
            return [];
        }
    }
    function resolveElement(sel, scope = document) {
        if (!sel)
            return null;
        if (sel.includes(' >>> '))
            return resolveDeepCombinator(sel, scope);
        const nthMatch = sel.match(/^(.*):nth-match\((\d+)\)$/);
        if (nthMatch) {
            const base = nthMatch[1] || '';
            const n = Number.parseInt(nthMatch[2] || '0', 10);
            if (!base || Number.isNaN(n) || n < 1)
                return null;
            const matches = resolveElements(base, scope);
            return matches[n - 1] || null;
        }
        if (sel.startsWith('text='))
            return resolveByText(sel.slice('text='.length), scope);
        if (sel.startsWith('role='))
            return firstVisible(querySelectorAllDeep(`[role="${CSS.escape(sel.slice('role='.length))}"]`, scope));
        if (sel.startsWith('placeholder='))
            return firstVisible(querySelectorAllDeep(`[placeholder="${CSS.escape(sel.slice('placeholder='.length))}"]`, scope));
        if (sel.startsWith('label='))
            return resolveByLabel(sel.slice('label='.length), scope);
        if (sel.startsWith('aria-label='))
            return resolveByAriaLabel(sel.slice('aria-label='.length), scope);
        return querySelectorDeep(sel, scope);
    }
    // list_interactive is handled by domPrimitiveListInteractive in production dispatch,
    // but remains available here for backward compatibility and direct tests.
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
    function buildShadowSelector(el) {
        const rootNode = el.getRootNode();
        if (!(rootNode instanceof ShadowRoot))
            return null;
        const parts = [];
        let node = el;
        let root = rootNode;
        while (root instanceof ShadowRoot) {
            const inner = buildUniqueSelector(node, node, node.tagName.toLowerCase());
            parts.unshift(inner);
            node = root.host;
            root = node.getRootNode();
        }
        const hostSelector = buildUniqueSelector(node, node, node.tagName.toLowerCase());
        parts.unshift(hostSelector);
        return parts.join(' >>> ');
    }
    function classifyElement(el) {
        const tag = el.tagName.toLowerCase();
        if (tag === 'a')
            return 'link';
        if (tag === 'button' || el.getAttribute('role') === 'button')
            return 'button';
        if (tag === 'input') {
            const inputType = el.type || 'text';
            if (inputType === 'submit' || inputType === 'button' || inputType === 'reset')
                return 'button';
            if (inputType === 'checkbox' || inputType === 'radio')
                return 'checkbox';
            return 'input';
        }
        if (tag === 'select')
            return 'select';
        if (tag === 'textarea')
            return 'textarea';
        if (el.getAttribute('role') === 'link')
            return 'link';
        if (el.getAttribute('role') === 'tab')
            return 'tab';
        if (el.getAttribute('role') === 'menuitem')
            return 'menuitem';
        if (el.getAttribute('contenteditable') === 'true')
            return 'textarea';
        return 'interactive';
    }
    function isVisibleElement(el) {
        const htmlEl = el;
        if (!htmlEl || typeof htmlEl.getBoundingClientRect !== 'function')
            return true;
        const rect = htmlEl.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
    }
    function extractElementLabel(el) {
        const htmlEl = el;
        return (el.getAttribute('aria-label') ||
            el.getAttribute('title') ||
            el.getAttribute('placeholder') ||
            (htmlEl?.textContent || '').trim().slice(0, 80) ||
            el.tagName.toLowerCase());
    }
    function chooseBestScopeMatch(matches) {
        if (matches.length === 1)
            return matches[0];
        const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply)/i;
        let best = matches[0];
        let bestScore = -1;
        for (const candidate of matches) {
            const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', candidate);
            const visibleTextboxes = textboxes.filter(isVisibleElement).length;
            const buttonCandidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', candidate);
            let visibleButtons = 0;
            let submitLikeButtons = 0;
            for (const btn of buttonCandidates) {
                if (!isVisibleElement(btn))
                    continue;
                visibleButtons++;
                if (submitVerb.test(extractElementLabel(btn))) {
                    submitLikeButtons++;
                }
            }
            const interactiveCandidates = querySelectorAllDeep('a[href], button, input, select, textarea, [role="button"], [role="link"], [role="tab"], [role="menuitem"], [contenteditable="true"]', candidate);
            const visibleInteractive = interactiveCandidates.filter(isVisibleElement).length;
            const hiddenInteractive = Math.max(0, interactiveCandidates.length - visibleInteractive);
            const rect = candidate.getBoundingClientRect?.();
            const areaScore = rect && rect.width > 0 && rect.height > 0
                ? Math.min(20, Math.round((rect.width * rect.height) / 50000))
                : 0;
            const score = visibleTextboxes * 1000 +
                submitLikeButtons * 250 +
                visibleButtons * 10 +
                visibleInteractive -
                hiddenInteractive +
                areaScore;
            if (score > bestScore) {
                bestScore = score;
                best = candidate;
            }
        }
        return best;
    }
    function listInteractiveCompatibility() {
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
        const rawEntries = [];
        const scope = (selector || '').trim();
        const scopeRoot = (() => {
            if (!scope)
                return document;
            try {
                const matches = querySelectorAllDeep(scope);
                if (matches.length === 0)
                    return null;
                return chooseBestScopeMatch(matches);
            }
            catch {
                return null;
            }
        })();
        if (!scopeRoot) {
            return {
                success: false,
                elements: [],
                error: 'scope_not_found',
                message: `No scope element matches selector: ${scope}`
            };
        }
        for (const cssSelector of interactiveSelectors) {
            const matches = querySelectorAllDeep(cssSelector, scopeRoot);
            for (const el of matches) {
                if (seen.has(el))
                    continue;
                seen.add(el);
                const htmlEl = el;
                const rect = typeof htmlEl.getBoundingClientRect === 'function'
                    ? htmlEl.getBoundingClientRect()
                    : { width: 0, height: 0 };
                const visible = rect.width > 0 && rect.height > 0 && htmlEl.offsetParent !== null;
                const shadowSelector = buildShadowSelector(el);
                const baseSelector = shadowSelector || buildUniqueSelector(el, htmlEl, cssSelector);
                const label = el.getAttribute('aria-label') ||
                    el.getAttribute('title') ||
                    el.getAttribute('placeholder') ||
                    (htmlEl.textContent || '').trim().slice(0, 60) ||
                    el.tagName.toLowerCase();
                rawEntries.push({
                    element: el,
                    baseSelector,
                    tag: el.tagName.toLowerCase(),
                    inputType: el instanceof HTMLInputElement ? el.type : undefined,
                    elementType: classifyElement(el),
                    label,
                    role: el.getAttribute('role') || undefined,
                    placeholder: el.getAttribute('placeholder') || undefined,
                    visible
                });
                if (rawEntries.length >= 100)
                    break;
            }
            if (rawEntries.length >= 100)
                break;
        }
        const selectorCount = new Map();
        for (const entry of rawEntries) {
            selectorCount.set(entry.baseSelector, (selectorCount.get(entry.baseSelector) || 0) + 1);
        }
        const selectorIndex = new Map();
        const elements = rawEntries.map((entry, index) => {
            let selector = entry.baseSelector;
            const count = selectorCount.get(entry.baseSelector) || 1;
            if (count > 1) {
                const nth = (selectorIndex.get(entry.baseSelector) || 0) + 1;
                selectorIndex.set(entry.baseSelector, nth);
                selector = `${entry.baseSelector}:nth-match(${nth})`;
            }
            return {
                index,
                tag: entry.tag,
                type: entry.inputType,
                element_type: entry.elementType,
                selector,
                element_id: getOrCreateElementID(entry.element),
                label: entry.label,
                role: entry.role,
                placeholder: entry.placeholder,
                visible: entry.visible
            };
        });
        return { success: true, elements };
    }
    if (action === 'list_interactive') {
        return listInteractiveCompatibility();
    }
    // — Resolve element for all other actions —
    function domError(error, message) {
        return { success: false, action, selector, error, message };
    }
    function matchedTarget(node) {
        const htmlEl = node;
        const textPreview = (htmlEl.textContent || '').trim().slice(0, 80);
        return {
            tag: node.tagName.toLowerCase(),
            role: node.getAttribute('role') || undefined,
            aria_label: node.getAttribute('aria-label') || undefined,
            text_preview: textPreview || undefined,
            selector,
            element_id: getOrCreateElementID(node),
            scope_selector_used: resolvedScopeSelector
        };
    }
    function isActionableVisible(el) {
        if (!(el instanceof HTMLElement))
            return true;
        const rect = typeof el.getBoundingClientRect === 'function'
            ? el.getBoundingClientRect()
            : { width: 0, height: 0 };
        return rect.width > 0 && rect.height > 0 && el.offsetParent !== null;
    }
    function summarizeCandidates(matches) {
        return matches.slice(0, 8).map((candidate) => {
            const htmlEl = candidate;
            const fallback = candidate.tagName.toLowerCase();
            return {
                tag: fallback,
                role: candidate.getAttribute('role') || undefined,
                aria_label: candidate.getAttribute('aria-label') || undefined,
                text_preview: (htmlEl.textContent || '').trim().slice(0, 80) || undefined,
                selector: buildUniqueSelector(candidate, htmlEl, fallback),
                element_id: getOrCreateElementID(candidate),
                visible: isActionableVisible(candidate)
            };
        });
    }
    const intentActions = new Set([
        'open_composer',
        'submit_active_composer',
        'confirm_top_dialog',
        'dismiss_top_overlay'
    ]);
    function uniqueElements(elements) {
        const out = [];
        const seen = new Set();
        for (const element of elements) {
            if (seen.has(element))
                continue;
            seen.add(element);
            out.push(element);
        }
        return out;
    }
    function elementZIndexScore(el) {
        if (!(el instanceof HTMLElement))
            return 0;
        const style = getComputedStyle(el);
        const raw = style.zIndex || '';
        const parsed = Number.parseInt(raw, 10);
        if (Number.isNaN(parsed))
            return 0;
        return parsed;
    }
    function areaScore(el, max) {
        if (!(el instanceof HTMLElement) || typeof el.getBoundingClientRect !== 'function')
            return 0;
        const rect = el.getBoundingClientRect();
        if (rect.width <= 0 || rect.height <= 0)
            return 0;
        return Math.min(max, Math.round((rect.width * rect.height) / 10000));
    }
    function pickBestIntentTarget(ranked, matchStrategy, notFoundError, notFoundMessage) {
        const viable = ranked
            .filter((entry) => entry.score > 0 && isActionableVisible(entry.element))
            .sort((a, b) => b.score - a.score);
        if (viable.length === 0) {
            return { error: domError(notFoundError, notFoundMessage) };
        }
        const topScore = viable[0].score;
        const tiedTop = viable.filter((entry) => entry.score === topScore);
        if (tiedTop.length > 1) {
            return {
                error: {
                    success: false,
                    action,
                    selector,
                    error: 'ambiguous_target',
                    message: `Multiple candidates tie for ${action}. Use scope_selector or list_interactive element_id.`,
                    match_count: tiedTop.length,
                    match_strategy: matchStrategy,
                    candidates: summarizeCandidates(tiedTop.map((entry) => entry.element))
                }
            };
        }
        return {
            element: viable[0].element,
            match_count: 1,
            match_strategy: matchStrategy
        };
    }
    function collectDialogs() {
        const selectors = ['[role="dialog"]', '[aria-modal="true"]', 'dialog[open]'];
        const dialogs = [];
        for (const dialogSelector of selectors) {
            dialogs.push(...querySelectorAllDeep(dialogSelector));
        }
        return uniqueElements(dialogs).filter(isActionableVisible);
    }
    function pickTopDialog(dialogs) {
        if (dialogs.length === 0)
            return null;
        const ranked = dialogs
            .map((dialog, index) => ({
            element: dialog,
            score: elementZIndexScore(dialog) * 1000 + areaScore(dialog, 200) + index
        }))
            .sort((a, b) => b.score - a.score);
        return ranked[0]?.element || null;
    }
    function resolveIntentTarget(requestedScope, activeScope) {
        const submitVerb = /(post|share|publish|send|submit|save|done|continue|next|create|apply|confirm|yes|allow|accept)/i;
        const dismissVerb = /(close|dismiss|cancel|not now|no thanks|skip|x|×|hide|back)/i;
        const composerVerb = /(start( a)? post|create post|write (a )?post|what'?s on your mind|share( an)? update|compose|new post)/i;
        if (action === 'open_composer') {
            const selectors = [
                'button',
                '[role="button"]',
                'a[href]',
                '[role="link"]',
                '[contenteditable="true"]',
                '[role="textbox"]',
                'textarea',
                'input[type="text"]',
                'input:not([type])'
            ];
            const candidates = [];
            for (const candidateSelector of selectors) {
                candidates.push(...querySelectorAllDeep(candidateSelector, activeScope));
            }
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate).toLowerCase();
                const tag = candidate.tagName.toLowerCase();
                const role = candidate.getAttribute('role') || '';
                const contentEditable = candidate.getAttribute('contenteditable') === 'true';
                let score = 0;
                if (composerVerb.test(label))
                    score += 700;
                if (/\b(post|share|publish|compose|write|update)\b/i.test(label))
                    score += 280;
                if (contentEditable || role === 'textbox' || tag === 'textarea' || tag === 'input')
                    score += 220;
                if (tag === 'button' || role === 'button')
                    score += 80;
                score += areaScore(candidate, 50);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_open_composer', 'composer_not_found', 'No composer trigger was found. Try a tighter scope_selector.');
            return { ...best, scope_selector_used: requestedScope || undefined };
        }
        if (action === 'submit_active_composer') {
            let scopeRoot = activeScope;
            let scopeUsed = requestedScope || undefined;
            if (!requestedScope) {
                const dialogs = collectDialogs();
                const rankedDialogs = dialogs
                    .map((dialog) => {
                    const textboxes = querySelectorAllDeep('[role="textbox"], textarea, [contenteditable="true"]', dialog).filter(isActionableVisible).length;
                    const buttons = querySelectorAllDeep('button, [role="button"], input[type="submit"]', dialog);
                    const submitLikeButtons = buttons.filter((button) => isActionableVisible(button) && submitVerb.test(extractElementLabel(button))).length;
                    return {
                        element: dialog,
                        score: textboxes * 1200 + submitLikeButtons * 300 + elementZIndexScore(dialog) * 2 + areaScore(dialog, 80)
                    };
                })
                    .sort((a, b) => b.score - a.score);
                if ((rankedDialogs[0]?.score || 0) > 0) {
                    scopeRoot = rankedDialogs[0].element;
                    scopeUsed = 'intent:auto_composer_scope';
                }
            }
            const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', scopeRoot);
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate);
                let score = 0;
                if (submitVerb.test(label))
                    score += 700;
                if (dismissVerb.test(label))
                    score -= 500;
                score += areaScore(candidate, 30);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_submit_active_composer', 'composer_submit_not_found', 'No submit control found in active composer scope.');
            return { ...best, scope_selector_used: scopeUsed };
        }
        if (action === 'confirm_top_dialog') {
            const scopeRoot = requestedScope ? activeScope : pickTopDialog(collectDialogs());
            if (!scopeRoot) {
                return {
                    error: domError('dialog_not_found', 'No visible dialog/overlay found to confirm.')
                };
            }
            const candidates = querySelectorAllDeep('button, [role="button"], input[type="submit"]', scopeRoot);
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate);
                let score = 0;
                if (submitVerb.test(label))
                    score += 700;
                if (dismissVerb.test(label))
                    score -= 500;
                score += areaScore(candidate, 30);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_confirm_top_dialog', 'confirm_action_not_found', 'No confirm control found in the top dialog.');
            return {
                ...best,
                scope_selector_used: requestedScope || 'intent:auto_top_dialog'
            };
        }
        if (action === 'dismiss_top_overlay') {
            const scopeRoot = requestedScope ? activeScope : pickTopDialog(collectDialogs());
            if (!scopeRoot) {
                return {
                    error: domError('overlay_not_found', 'No visible dialog/overlay found to dismiss.')
                };
            }
            const candidates = querySelectorAllDeep('button, [role="button"], [aria-label], [data-testid], [title]', scopeRoot);
            const ranked = uniqueElements(candidates).map((candidate) => {
                const label = extractElementLabel(candidate);
                let score = 0;
                if (dismissVerb.test(label))
                    score += 800;
                if (submitVerb.test(label))
                    score -= 550;
                score += areaScore(candidate, 30);
                score += elementZIndexScore(candidate);
                return { element: candidate, score };
            });
            const best = pickBestIntentTarget(ranked, 'intent_dismiss_top_overlay', 'dismiss_action_not_found', 'No dismiss control found in the top overlay.');
            return {
                ...best,
                scope_selector_used: requestedScope || 'intent:auto_top_dialog'
            };
        }
        return { error: domError('unknown_action', `Unknown DOM action: ${action}`) };
    }
    function resolveActionTarget() {
        const requestedScope = (options.scope_selector || '').trim();
        if (requestedScope && !scopeRoot) {
            return {
                error: domError('scope_not_found', `No scope element matches selector: ${requestedScope}`)
            };
        }
        const activeScope = scopeRoot || document;
        const scopeSelectorUsed = requestedScope || undefined;
        if (intentActions.has(action)) {
            return resolveIntentTarget(requestedScope, activeScope);
        }
        const requestedElementID = (options.element_id || '').trim();
        if (requestedElementID) {
            const resolvedByID = resolveElementByID(requestedElementID);
            if (!resolvedByID) {
                return {
                    error: domError('stale_element_id', `Element handle is stale or unknown: ${requestedElementID}. Call list_interactive again.`)
                };
            }
            if (activeScope !== document && typeof activeScope.contains === 'function') {
                const contains = activeScope.contains(resolvedByID);
                if (!contains) {
                    return {
                        error: domError('element_id_scope_mismatch', `Element handle does not belong to scope: ${requestedScope || '<none>'}`)
                    };
                }
            }
            return {
                element: resolvedByID,
                match_count: 1,
                match_strategy: 'element_id',
                scope_selector_used: scopeSelectorUsed
            };
        }
        const ambiguitySensitiveActions = new Set([
            'click', 'type', 'select', 'check', 'set_attribute',
            'paste', 'key_press', 'focus', 'scroll_to'
        ]);
        if (!ambiguitySensitiveActions.has(action)) {
            const found = resolveElement(selector, activeScope);
            if (!found)
                return { error: domError('element_not_found', `No element matches selector: ${selector}`) };
            return {
                element: found,
                match_count: 1,
                match_strategy: requestedScope ? 'scoped_selector' : 'selector',
                scope_selector_used: scopeSelectorUsed
            };
        }
        const rawMatches = resolveElements(selector, activeScope);
        const uniqueMatches = [];
        const seen = new Set();
        for (const match of rawMatches) {
            if (seen.has(match))
                continue;
            seen.add(match);
            uniqueMatches.push(match);
        }
        const viableMatches = (() => {
            if (uniqueMatches.length === 0)
                return uniqueMatches;
            const visible = uniqueMatches.filter(isActionableVisible);
            return visible.length > 0 ? visible : uniqueMatches;
        })();
        if (viableMatches.length > 1) {
            return {
                error: {
                    success: false,
                    action,
                    selector,
                    error: 'ambiguous_target',
                    message: `Selector matches multiple viable elements: ${selector}. Add scope, or use list_interactive element_id/index.`,
                    match_count: viableMatches.length,
                    match_strategy: 'ambiguous_selector',
                    candidates: summarizeCandidates(viableMatches)
                }
            };
        }
        const found = viableMatches[0] || resolveElement(selector, activeScope);
        if (!found)
            return { error: domError('element_not_found', `No element matches selector: ${selector}`) };
        const strategy = (() => {
            if (selector.includes(':nth-match('))
                return 'nth_match_selector';
            if (requestedScope)
                return 'scoped_selector';
            return 'selector';
        })();
        return {
            element: found,
            match_count: 1,
            match_strategy: strategy,
            scope_selector_used: scopeSelectorUsed
        };
    }
    const resolved = resolveActionTarget();
    if (resolved.error)
        return resolved.error;
    const el = resolved.element;
    const resolvedMatchCount = resolved.match_count || 1;
    const resolvedMatchStrategy = resolved.match_strategy || 'selector';
    const resolvedScopeSelector = resolved.scope_selector_used;
    function mutatingSuccess(node, extra) {
        return {
            success: true,
            action,
            selector,
            ...(extra || {}),
            matched: matchedTarget(node),
            match_count: resolvedMatchCount,
            match_strategy: resolvedMatchStrategy
        };
    }
    // — Mutation tracking: MutationObserver wrapper for DOM change capture —
    function withMutationTracking(fn) {
        const t0 = performance.now();
        const mutations = [];
        const observer = new MutationObserver((records) => {
            mutations.push(...records);
        });
        observer.observe(document.body || document.documentElement, {
            childList: true,
            subtree: true,
            attributes: true,
            attributeOldValue: !!options.observe_mutations
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
                if (options.observe_mutations) {
                    const maxEntries = 50;
                    const entries = [];
                    for (const m of mutations) {
                        if (entries.length >= maxEntries)
                            break;
                        if (m.type === 'childList') {
                            for (let i = 0; i < m.addedNodes.length && entries.length < maxEntries; i++) {
                                const n = m.addedNodes[i];
                                if (n && n.nodeType === 1) {
                                    const el = n;
                                    entries.push({ type: 'added', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined });
                                }
                            }
                            for (let i = 0; i < m.removedNodes.length && entries.length < maxEntries; i++) {
                                const n = m.removedNodes[i];
                                if (n && n.nodeType === 1) {
                                    const el = n;
                                    entries.push({ type: 'removed', tag: el.tagName?.toLowerCase(), id: el.id || undefined, class: el.className?.toString()?.slice(0, 80) || undefined, text_preview: el.textContent?.slice(0, 100) || undefined });
                                }
                            }
                        }
                        else if (m.type === 'attributes' && m.target.nodeType === 1) {
                            const el = m.target;
                            entries.push({ type: 'attribute', tag: el.tagName?.toLowerCase(), id: el.id || undefined, attribute: m.attributeName || undefined, old_value: m.oldValue?.slice(0, 100) || undefined, new_value: el.getAttribute(m.attributeName || '')?.slice(0, 100) || undefined });
                        }
                    }
                    enriched.dom_mutations = entries;
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
    function buildActionHandlers(node) {
        return {
            click: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                node.click();
                return mutatingSuccess(node);
            }),
            type: () => withMutationTracking(() => {
                const text = options.text || '';
                // Contenteditable elements (Gmail compose body, rich text editors)
                if (node instanceof HTMLElement && node.isContentEditable) {
                    node.focus();
                    if (options.clear) {
                        const selection = document.getSelection();
                        if (selection) {
                            selection.selectAllChildren(node);
                            selection.deleteFromDocument();
                        }
                    }
                    // Split on newlines — each \n becomes an insertParagraph command
                    const lines = text.split('\n');
                    for (let i = 0; i < lines.length; i++) {
                        const line = lines[i];
                        if (i > 0) {
                            document.execCommand('insertParagraph', false);
                        }
                        if (line.length > 0) {
                            document.execCommand('insertText', false, line);
                        }
                    }
                    return mutatingSuccess(node, { value: node.innerText });
                }
                if (!(node instanceof HTMLInputElement) && !(node instanceof HTMLTextAreaElement)) {
                    return domError('not_typeable', `Element is not an input, textarea, or contenteditable: ${node.tagName}`);
                }
                const proto = node instanceof HTMLTextAreaElement ? HTMLTextAreaElement : HTMLInputElement;
                const nativeSetter = Object.getOwnPropertyDescriptor(proto.prototype, 'value')?.set;
                if (nativeSetter) {
                    const newValue = options.clear ? text : node.value + text;
                    nativeSetter.call(node, newValue);
                }
                else {
                    node.value = options.clear ? text : node.value + text;
                }
                node.dispatchEvent(new InputEvent('input', { bubbles: true, data: text, inputType: 'insertText' }));
                node.dispatchEvent(new Event('change', { bubbles: true }));
                return mutatingSuccess(node, { value: node.value });
            }),
            select: () => withMutationTracking(() => {
                if (!(node instanceof HTMLSelectElement))
                    return domError('not_select', `Element is not a <select>: ${node.tagName}`); // nosemgrep: html-in-template-string
                const nativeSelectSetter = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value')?.set;
                if (nativeSelectSetter) {
                    nativeSelectSetter.call(node, options.value || '');
                }
                else {
                    node.value = options.value || '';
                }
                node.dispatchEvent(new Event('change', { bubbles: true }));
                return mutatingSuccess(node, { value: node.value });
            }),
            check: () => withMutationTracking(() => {
                if (!(node instanceof HTMLInputElement) || (node.type !== 'checkbox' && node.type !== 'radio')) {
                    return domError('not_checkable', `Element is not a checkbox or radio: ${node.tagName} type=${node.type || 'N/A'}`);
                }
                const desired = options.checked !== undefined ? options.checked : true;
                if (node.checked !== desired) {
                    node.click();
                }
                return mutatingSuccess(node, { value: node.checked });
            }),
            get_text: () => {
                const text = node instanceof HTMLElement ? node.innerText : node.textContent;
                if (text === null || text === undefined) {
                    return {
                        success: true,
                        action,
                        selector,
                        value: text,
                        reason: 'no_text_content',
                        message: 'Resolved text content is null'
                    };
                }
                return { success: true, action, selector, value: text };
            },
            get_value: () => {
                if (!('value' in node))
                    return domError('no_value_property', `Element has no value property: ${node.tagName}`);
                const value = node.value;
                if (value === null || value === undefined) {
                    return {
                        success: true,
                        action,
                        selector,
                        value,
                        reason: 'no_value',
                        message: 'Element value is null'
                    };
                }
                return { success: true, action, selector, value };
            },
            get_attribute: () => {
                const attrName = options.name || '';
                const value = node.getAttribute(attrName);
                if (value === null) {
                    return {
                        success: true,
                        action,
                        selector,
                        value,
                        reason: 'attribute_not_found',
                        message: `Attribute "${attrName}" not found`
                    };
                }
                return { success: true, action, selector, value };
            },
            set_attribute: () => withMutationTracking(() => {
                node.setAttribute(options.name || '', options.value || '');
                return mutatingSuccess(node, { value: node.getAttribute(options.name || '') });
            }),
            focus: () => {
                if (!(node instanceof HTMLElement))
                    return domError('not_focusable', `Element is not an HTMLElement: ${node.tagName}`);
                node.focus();
                return mutatingSuccess(node);
            },
            scroll_to: () => {
                node.scrollIntoView({ behavior: 'smooth', block: 'center' });
                return mutatingSuccess(node);
            },
            wait_for: () => ({ success: true, action, selector, value: node.tagName.toLowerCase() }),
            paste: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                node.focus();
                if (options.clear) {
                    const selection = document.getSelection();
                    if (selection) {
                        selection.selectAllChildren(node);
                        selection.deleteFromDocument();
                    }
                }
                const pasteText = options.text || '';
                const dt = new DataTransfer();
                dt.setData('text/plain', pasteText);
                const event = new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true });
                node.dispatchEvent(event);
                return mutatingSuccess(node, { value: node.innerText });
            }),
            key_press: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                const key = options.text || 'Enter';
                // Tab/Shift+Tab: manually move focus (dispatchEvent can't trigger native tab traversal)
                if (key === 'Tab' || key === 'Shift+Tab') {
                    const focusable = Array.from(node.ownerDocument.querySelectorAll('a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')).filter((e) => e.offsetParent !== null);
                    const idx = focusable.indexOf(node);
                    const next = key === 'Shift+Tab' ? focusable[idx - 1] : focusable[idx + 1];
                    if (next) {
                        next.focus();
                        return mutatingSuccess(node, { value: key });
                    }
                    return mutatingSuccess(node, { value: key, message: 'No next focusable element' });
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
                node.dispatchEvent(new KeyboardEvent('keydown', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                node.dispatchEvent(new KeyboardEvent('keypress', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                node.dispatchEvent(new KeyboardEvent('keyup', { key: mapped.key, code: mapped.code, keyCode: mapped.keyCode, bubbles: true }));
                return mutatingSuccess(node, { value: key });
            }),
            open_composer: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                const tag = node.tagName.toLowerCase();
                const isInputLike = node.isContentEditable ||
                    node.getAttribute('role') === 'textbox' ||
                    tag === 'textarea' ||
                    tag === 'input';
                if (isInputLike) {
                    node.focus();
                    return mutatingSuccess(node, { reason: 'composer_ready' });
                }
                node.click();
                return mutatingSuccess(node);
            }),
            submit_active_composer: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                node.click();
                return mutatingSuccess(node);
            }),
            confirm_top_dialog: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                node.click();
                return mutatingSuccess(node);
            }),
            dismiss_top_overlay: () => withMutationTracking(() => {
                if (!(node instanceof HTMLElement))
                    return domError('not_interactive', `Element is not an HTMLElement: ${node.tagName}`);
                node.click();
                return mutatingSuccess(node);
            })
        };
    }
    const handlers = buildActionHandlers(el);
    const handler = handlers[action];
    if (!handler) {
        return domError('unknown_action', `Unknown DOM action: ${action}`);
    }
    return handler();
}
export function domWaitFor(selector, timeoutMs = 5000) {
    const timeout = Math.max(1, timeoutMs);
    return new Promise((resolve) => {
        let resolved = false;
        let pollTimer = null;
        let timeoutTimer = null;
        let observer = null;
        const timeoutResult = {
            success: false,
            action: 'wait_for',
            selector,
            error: 'timeout',
            message: `Element not found within ${timeout}ms: ${selector}`
        };
        function cleanup() {
            if (pollTimer)
                clearInterval(pollTimer);
            if (timeoutTimer)
                clearTimeout(timeoutTimer);
            if (observer)
                observer.disconnect();
        }
        function finish(result) {
            if (resolved)
                return;
            resolved = true;
            cleanup();
            resolve(result);
        }
        function checkNow() {
            const result = domPrimitive('wait_for', selector, { timeout_ms: timeout });
            if (result && typeof result.then === 'function') {
                void result
                    .then((resolvedResult) => {
                    if (resolvedResult.success)
                        finish(resolvedResult);
                })
                    .catch(() => { });
                return;
            }
            if (result.success) {
                finish(result);
            }
        }
        checkNow();
        if (resolved)
            return;
        if (typeof MutationObserver === 'function') {
            observer = new MutationObserver(() => {
                checkNow();
            });
            observer.observe(document.body || document.documentElement, {
                childList: true,
                subtree: true,
                attributes: true
            });
        }
        pollTimer = setInterval(checkNow, Math.min(80, timeout));
        timeoutTimer = setTimeout(() => finish(timeoutResult), timeout);
    });
}
// Dispatcher utilities (parseDOMParams, executeDOMAction, etc.) moved to ./dom-dispatch.ts
//# sourceMappingURL=dom-primitives.js.map