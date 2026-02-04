/**
 * @fileoverview Popup UI Utilities
 * Helper functions for UI updates
 */
/**
 * Format bytes into human-readable file size
 */
export declare function formatFileSize(bytes: number): string;
/**
 * Check if a URL is an internal browser page that cannot be tracked.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 */
export declare function isInternalUrl(url: string | undefined): boolean;
//# sourceMappingURL=ui-utils.d.ts.map