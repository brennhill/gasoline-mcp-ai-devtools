/**
 * Purpose: Captures visible tab screenshots with rate limiting and uploads to the daemon server.
 * Why: Isolates screenshot capture (rate-check, tab capture, server upload) from unrelated log/badge logic.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
import type { LogEntry } from '../types/index.js';
/**
 * Capture a screenshot of the visible tab area
 */
export declare function captureScreenshot(tabId: number, serverUrl: string, relatedErrorId: string | null, errorType: string | null, canTakeScreenshotFn: (tabId: number) => {
    allowed: boolean;
    reason?: string;
    nextAllowedIn?: number | null;
}, recordScreenshotFn: (tabId: number) => void, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<{
    success: boolean;
    entry?: LogEntry;
    error?: string;
    nextAllowedIn?: number | null;
}>;
//# sourceMappingURL=screenshot-capture.d.ts.map