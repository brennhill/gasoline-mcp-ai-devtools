/**
 * @fileoverview Script Injection Module
 * Injects capture script into the page context
 */
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
        document.addEventListener('DOMContentLoaded', injectScript, { once: true });
    }
    else {
        injectScript();
    }
}
//# sourceMappingURL=script-injection.js.map