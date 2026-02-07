/**
 * @fileoverview On-demand DOM queries.
 * Provides structured DOM querying, page info extraction, and
 * accessibility auditing via axe-core.
 */
import { DOM_QUERY_MAX_ELEMENTS, DOM_QUERY_MAX_TEXT, DOM_QUERY_MAX_DEPTH, DOM_QUERY_MAX_HTML, A11Y_MAX_NODES_PER_VIOLATION, A11Y_AUDIT_TIMEOUT_MS, } from './constants.js';
/**
 * Execute a DOM query and return structured results
 */
export async function executeDOMQuery(params) {
    const { selector, include_styles, properties, include_children, max_depth } = params;
    const elements = document.querySelectorAll(selector);
    const matchCount = elements.length;
    const cappedDepth = Math.min(max_depth || 3, DOM_QUERY_MAX_DEPTH);
    const matches = [];
    for (let i = 0; i < Math.min(elements.length, DOM_QUERY_MAX_ELEMENTS); i++) {
        const el = elements[i];
        if (!el)
            continue;
        const entry = serializeDOMElement(el, include_styles, properties, include_children, cappedDepth, 0);
        matches.push(entry);
    }
    return {
        url: window.location.href,
        title: document.title,
        matchCount,
        returnedCount: matches.length,
        matches,
    };
}
/**
 * Serialize a DOM element to a plain object
 */
function serializeDOMElement(el, includeStyles, styleProps, includeChildren, maxDepth, currentDepth) {
    const entry = {
        tag: el.tagName ? el.tagName.toLowerCase() : '',
        text: (el.textContent || '').slice(0, DOM_QUERY_MAX_TEXT),
        visible: el.offsetParent !== null || (el.getBoundingClientRect && el.getBoundingClientRect().width > 0),
    };
    // Attributes
    if (el.attributes && el.attributes.length > 0) {
        entry.attributes = {};
        for (const attr of el.attributes) {
            entry.attributes[attr.name] = attr.value;
        }
    }
    // Bounding box
    if (el.getBoundingClientRect) {
        const rect = el.getBoundingClientRect();
        entry.boundingBox = { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
    }
    // Computed styles
    if (includeStyles && typeof window.getComputedStyle === 'function') {
        const computed = window.getComputedStyle(el);
        entry.styles = {};
        if (styleProps && styleProps.length > 0) {
            for (const prop of styleProps) {
                entry.styles[prop] = computed.getPropertyValue(prop);
            }
        }
        else {
            entry.styles = { display: computed.display, color: computed.color, position: computed.position };
        }
    }
    // Children (capped to avoid unbounded serialization)
    if (includeChildren && currentDepth < maxDepth && el.children && el.children.length > 0) {
        entry.children = [];
        const maxChildren = Math.min(el.children.length, DOM_QUERY_MAX_ELEMENTS);
        for (let i = 0; i < maxChildren; i++) {
            const child = el.children[i];
            if (child) {
                entry.children.push(serializeDOMElement(child, false, undefined, true, maxDepth, currentDepth + 1));
            }
        }
    }
    return entry;
}
/**
 * Get comprehensive page info
 */
export async function getPageInfo() {
    const headings = [];
    const headingEls = document.querySelectorAll('h1,h2,h3,h4,h5,h6');
    for (const h of headingEls) {
        headings.push((h.textContent || '').slice(0, DOM_QUERY_MAX_TEXT));
    }
    const forms = [];
    const formEls = document.querySelectorAll('form');
    for (const form of formEls) {
        const fields = [];
        const inputs = form.querySelectorAll('input,select,textarea');
        for (const input of inputs) {
            const inputEl = input;
            if (inputEl.name)
                fields.push(inputEl.name);
        }
        forms.push({
            id: form.id || undefined,
            action: form.action || undefined,
            fields,
        });
    }
    return {
        url: window.location.href,
        title: document.title,
        viewport: { width: window.innerWidth, height: window.innerHeight },
        scroll: { x: window.scrollX, y: window.scrollY },
        documentHeight: document.documentElement.scrollHeight,
        headings,
        links: document.querySelectorAll('a').length,
        images: document.querySelectorAll('img').length,
        interactiveElements: document.querySelectorAll('button,input,select,textarea,a[href]').length,
        forms,
    };
}
/**
 * Load axe-core dynamically if not already present.
 *
 * IMPORTANT: axe-core MUST be loaded from the bundled local copy (lib/axe.min.js).
 * Chrome Web Store policy prohibits loading remotely hosted code. All third-party
 * libraries must be bundled with the extension package.
 */
function loadAxeCore() {
    return new Promise((resolve, reject) => {
        if (window.axe) {
            resolve();
            return;
        }
        // Wait for axe-core to be injected by content script (which has chrome.runtime API access)
        // Note: This function runs in page context (inject script), so we can't call chrome.runtime.getURL()
        const checkInterval = setInterval(() => {
            if (window.axe) {
                clearInterval(checkInterval);
                resolve();
            }
        }, 100);
        // Timeout after 5 seconds
        setTimeout(() => {
            clearInterval(checkInterval);
            reject(new Error('Accessibility audit failed: axe-core library not loaded (5s timeout). The extension content script may not have been injected on this page. Try reloading the tab and re-running the audit.'));
        }, 5000);
    });
}
/**
 * Run an accessibility audit using axe-core
 */
export async function runAxeAudit(params) {
    await loadAxeCore();
    const context = params.scope ? { include: [params.scope] } : document;
    const config = {};
    if (params.tags && params.tags.length > 0) {
        config.runOnly = params.tags;
    }
    if (params.include_passes) {
        config.resultTypes = ['violations', 'passes', 'incomplete', 'inapplicable'];
    }
    else {
        config.resultTypes = ['violations', 'incomplete'];
    }
    const results = await window.axe.run(context, config);
    return formatAxeResults(results);
}
/**
 * Run axe audit with a timeout
 */
export async function runAxeAuditWithTimeout(params, timeoutMs = A11Y_AUDIT_TIMEOUT_MS) {
    return Promise.race([
        runAxeAudit(params),
        new Promise((resolve) => {
            setTimeout(() => resolve({
                violations: [],
                summary: { violations: 0, passes: 0, incomplete: 0, inapplicable: 0 },
                error: 'Accessibility audit timeout',
            }), timeoutMs);
        }),
    ]);
}
/**
 * Format axe-core results into a compact representation
 */
export function formatAxeResults(axeResult) {
    const formatViolation = (v) => {
        const formatted = {
            id: v.id,
            impact: v.impact,
            description: v.description,
            helpUrl: v.helpUrl,
            nodes: [],
        };
        // Extract WCAG tags
        if (v.tags) {
            formatted.wcag = v.tags.filter((t) => t.startsWith('wcag'));
        }
        // Format nodes (cap at 10)
        formatted.nodes = (v.nodes || []).slice(0, A11Y_MAX_NODES_PER_VIOLATION).map((node) => {
            const selector = Array.isArray(node.target) ? node.target[0] : node.target;
            return {
                selector: selector || '',
                html: (node.html || '').slice(0, DOM_QUERY_MAX_HTML),
                ...(node.failureSummary ? { failureSummary: node.failureSummary } : {}),
            };
        });
        if (v.nodes && v.nodes.length > A11Y_MAX_NODES_PER_VIOLATION) {
            formatted.nodeCount = v.nodes.length;
        }
        return formatted;
    };
    return {
        violations: (axeResult.violations || []).map(formatViolation),
        summary: {
            violations: (axeResult.violations || []).length,
            passes: (axeResult.passes || []).length,
            incomplete: (axeResult.incomplete || []).length,
            inapplicable: (axeResult.inapplicable || []).length,
        },
    };
}
//# sourceMappingURL=dom-queries.js.map