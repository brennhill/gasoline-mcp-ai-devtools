/**
 * @fileoverview Link Health Checker
 * Extracts all links from the current page and checks their health.
 * Categorizes issues as: ok (2xx), redirect (3xx), requires_auth (401/403),
 * broken (4xx/5xx), or timeout.
 */
export interface LinkHealthParams {
    readonly timeout_ms?: number;
    readonly max_workers?: number;
}
export interface LinkCheckResult {
    readonly url: string;
    readonly status: number | null;
    readonly code: 'ok' | 'redirect' | 'requires_auth' | 'broken' | 'timeout' | 'cors_blocked';
    readonly timeMs: number;
    readonly isExternal: boolean;
    readonly redirectTo?: string;
    readonly error?: string;
    readonly needsServerVerification?: boolean;
}
export interface LinkHealthCheckResult {
    readonly summary: {
        readonly totalLinks: number;
        readonly ok: number;
        readonly redirect: number;
        readonly requiresAuth: number;
        readonly broken: number;
        readonly timeout: number;
        readonly corsBlocked: number;
        readonly needsServerVerification: number;
    };
    readonly results: LinkCheckResult[];
}
/**
 * Check all links on the current page for health issues.
 * Extracts links, deduplicates, and checks max 20 concurrently.
 */
export declare function checkLinkHealth(params: LinkHealthParams): Promise<LinkHealthCheckResult>;
//# sourceMappingURL=link-health.d.ts.map