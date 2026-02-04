/**
 * @fileoverview Script Injection Module
 * Injects capture script into the page context
 */
/**
 * Inject axe-core library into the page
 * Must be called from content script context (has chrome.runtime API access)
 */
export function injectAxeCore() {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('lib/axe.min.js');
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
}
/**
 * Inject the capture script into the page
 */
export function injectScript() {
    const script = document.createElement('script');
    script.src = chrome.runtime.getURL('inject.bundled.js');
    script.type = 'module';
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
}
/**
 * Initialize script injection (call when DOM is ready)
 */
export function initScriptInjection() {
    // Inject when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', () => {
            injectAxeCore(); // Inject axe-core first (needed by inject script)
            injectScript();
        }, { once: true });
    }
    else {
        injectAxeCore(); // Inject axe-core first (needed by inject script)
        injectScript();
    }
}
//# sourceMappingURL=script-injection.js.map