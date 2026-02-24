/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Why: Keeps content-script bridging predictable between extension and page contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/** Get the page nonce for authenticating postMessages to inject.js */
export declare function getPageNonce(): string;
/** Check if inject script has been loaded into the page context */
export declare function isInjectScriptLoaded(): boolean;
/** Check if inject bridge has acknowledged a readiness ping */
export declare function isInjectBridgeReady(): boolean;
/**
 * Inject axe-core library into the page
 * Must be called from content script context (has chrome.runtime API access)
 */
export declare function injectAxeCore(): void;
/**
 * Inject the capture script into the page
 */
export declare function injectScript(): Promise<boolean>;
/**
 * Ensure inject script is present, deduplicating concurrent inject attempts.
 * Optionally force a fresh reinjection attempt.
 */
export declare function ensureInjectScriptReady(timeoutMs?: number, force?: boolean): Promise<boolean>;
/**
 * Ensure inject bridge responds to a ping, proving MAIN-world messaging is live.
 */
export declare function ensureInjectBridgeReady(timeoutMs?: number): Promise<boolean>;
/**
 * Initialize script injection (call when DOM is ready)
 */
export declare function initScriptInjection(force?: boolean): void;
//# sourceMappingURL=script-injection.d.ts.map