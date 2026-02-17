/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/** Get the page nonce for authenticating postMessages to inject.js */
export declare function getPageNonce(): string;
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