/**
 * @fileoverview Script Injection Module
 * Injects capture script into the page context and syncs stored settings
 */
/** Check if inject script has been loaded into the page context */
export declare function isInjectScriptLoaded(): boolean;
/**
 * Inject axe-core library into the page
 * Must be called from content script context (has chrome.runtime API access)
 */
export declare function injectAxeCore(): void;
/**
 * Inject the capture script into the page
 */
export declare function injectScript(): void;
/**
 * Initialize script injection (call when DOM is ready)
 */
export declare function initScriptInjection(): void;
//# sourceMappingURL=script-injection.d.ts.map