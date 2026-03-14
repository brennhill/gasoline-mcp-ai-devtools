/**
 * Purpose: Accessibility auditing via axe-core -- loads axe-core dynamically, runs audits with timeout, and formats results.
 * Docs: docs/features/feature/query-dom/index.md
 */
import { DOM_QUERY_MAX_HTML, A11Y_MAX_NODES_PER_VIOLATION, A11Y_AUDIT_TIMEOUT_MS } from './constants.js';
import { scaleTimeout } from './timeouts.js';
/**
 * Load axe-core dynamically if not already present.
 *
 * IMPORTANT: axe-core MUST be loaded from the bundled local copy (lib/axe.min.js).
 * Chrome Web Store policy prohibits loading remotely hosted code. All third-party
 * libraries must be bundled with the extension package.
 */
function loadAxeCore() {
    return new Promise((resolve, reject) => {
        const hasAxe = () => typeof window !== 'undefined' && !!window.axe;
        if (hasAxe()) {
            resolve();
            return;
        }
        let settled = false;
        const finish = (fn) => {
            if (settled)
                return;
            settled = true;
            fn();
        };
        // Wait for axe-core to be injected by content script (which has chrome.runtime API access)
        // Note: This function runs in page context (inject script), so we can't call chrome.runtime.getURL()
        const checkInterval = setInterval(() => {
            if (hasAxe()) {
                finish(() => {
                    clearInterval(checkInterval);
                    clearTimeout(loadTimeout);
                    resolve();
                });
            }
        }, scaleTimeout(100));
        // Timeout after 5 seconds
        const loadTimeout = setTimeout(() => {
            finish(() => {
                clearInterval(checkInterval);
                reject(new Error('Accessibility audit failed: axe-core library not loaded (5s timeout). The extension content script may not have been injected on this page. Try reloading the tab and re-running the audit.'));
            });
        }, scaleTimeout(5000));
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
 * Build an empty partial result with an error message.
 * Used by timeout and catch paths to avoid duplicated object literals.
 */
function emptyPartialResult(errorMessage) {
    return {
        violations: [],
        passes: [],
        incomplete: [],
        inapplicable: [],
        summary: { violations: 0, passes: 0, incomplete: 0, inapplicable: 0 },
        partial: true,
        error: errorMessage
    };
}
/**
 * Run axe audit with a timeout.
 * Issue #276: Returns partial results on timeout or conflict instead of throwing.
 */
export async function runAxeAuditWithTimeout(params, timeoutMs = A11Y_AUDIT_TIMEOUT_MS) {
    try {
        return await Promise.race([
            runAxeAudit(params),
            new Promise((resolve) => {
                setTimeout(() => resolve(emptyPartialResult('Accessibility audit timeout')), timeoutMs);
            })
        ]);
    }
    catch (err) {
        // Issue #276: Return partial results with error instead of throwing.
        // Handles "Axe is already running" and other runtime errors gracefully.
        return emptyPartialResult(err instanceof Error ? err.message : String(err));
    }
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
            nodes: []
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
                ...(node.failureSummary ? { failureSummary: node.failureSummary } : {})
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
            inapplicable: (axeResult.inapplicable || []).length
        }
    };
}
//# sourceMappingURL=a11y-audit.js.map