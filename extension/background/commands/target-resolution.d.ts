/**
 * Purpose: Resolves the target tab for command dispatch -- explicit, tracked, active, or auto-track fallback.
 */
import type { PendingQuery } from '../../types/index.js';
export type QueryParamsObject = Record<string, unknown>;
type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab' | 'active_tab_fallback' | 'auto_tracked_active_tab' | 'auto_tracked_random_tab' | 'auto_tracked_new_tab';
export interface TargetResolution {
    tabId: number;
    url: string;
    source: TargetResolutionSource;
    requestedTabId?: number;
    trackedTabId?: number | null;
    useActiveTab: boolean;
}
interface TargetResolutionError {
    payload: Record<string, unknown>;
    message: string;
}
export declare function withTargetContext(result: unknown, target: TargetResolution): Record<string, unknown>;
export declare function requiresTargetTab(queryType: string): boolean;
export declare function isBrowserEscapeAction(queryType: string, paramsObj: QueryParamsObject): boolean;
export declare function persistTrackedTab(tab: chrome.tabs.Tab): Promise<void>;
export declare function resolveTargetTab(query: PendingQuery, paramsObj: QueryParamsObject): Promise<{
    target?: TargetResolution;
    error?: TargetResolutionError;
}>;
/**
 * Check if a URL is restricted -- content scripts cannot run on these pages.
 * Covers internal browser pages and known CSP-restricted origins.
 */
export declare function isRestrictedUrl(url: string | undefined): boolean;
export {};
//# sourceMappingURL=target-resolution.d.ts.map