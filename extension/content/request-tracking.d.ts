/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * @fileoverview Request Tracking Module
 * Manages pending requests for AI Web Pilot features
 * Includes periodic cleanup timer to handle edge cases where pagehide/beforeunload don't fire.
 */
import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types/index.js';
import type { PendingRequestStats } from './types.js';
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export declare function clearPendingRequests(): void;
/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export declare function getPendingRequestStats(): PendingRequestStats;
export declare function registerHighlightRequest(resolve: (result: HighlightResponse) => void): number;
export declare function resolveHighlightRequest(requestId: number, result: HighlightResponse): void;
export declare function hasHighlightRequest(requestId: number): boolean;
export declare function deleteHighlightRequest(requestId: number): void;
export declare function registerExecuteRequest(resolve: (result: ExecuteJsResult) => void): number;
export declare function resolveExecuteRequest(requestId: number, result: ExecuteJsResult): void;
export declare function hasExecuteRequest(requestId: number): boolean;
export declare function deleteExecuteRequest(requestId: number): void;
export declare function registerA11yRequest(resolve: (result: A11yAuditResult) => void): number;
export declare function resolveA11yRequest(requestId: number, result: A11yAuditResult): void;
export declare function hasA11yRequest(requestId: number): boolean;
export declare function deleteA11yRequest(requestId: number): void;
export declare function registerDomRequest(resolve: (result: DomQueryResult) => void): number;
export declare function resolveDomRequest(requestId: number, result: DomQueryResult): void;
export declare function hasDomRequest(requestId: number): boolean;
export declare function deleteDomRequest(requestId: number): void;
/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export declare function cleanupRequestTracking(): void;
/**
 * Initialize request tracking (register cleanup handlers)
 */
export declare function initRequestTracking(): void;
//# sourceMappingURL=request-tracking.d.ts.map