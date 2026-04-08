/**
 * Purpose: Provides shared popup UI helper utilities for formatting and browser-page eligibility checks.
 * Why: Avoids duplicated UI utility logic across popup modules and keeps display behavior consistent.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */
/**
 * @fileoverview Popup UI Utilities
 * Helper functions for UI updates
 */
/**
 * Format bytes into human-readable file size
 */
export function formatFileSize(bytes) {
    if (bytes === 0)
        return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`;
}
/**
 * Check if a URL is an internal browser page that cannot be tracked.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 */
export function isInternalUrl(url) {
    if (!url)
        return true;
    const internalPrefixes = ['chrome://', 'chrome-extension://', 'about:', 'edge://', 'brave://', 'devtools://'];
    return internalPrefixes.some((prefix) => url.startsWith(prefix));
}
/**
 * Start a recurring timer display that shows elapsed time as "M:SS" in the given element.
 * Returns a cleanup function that stops the interval.
 */
export function startTimerDisplay(statusEl, startTime) {
    const update = () => {
        const elapsed = Math.round((Date.now() - startTime) / 1000);
        const mins = Math.floor(elapsed / 60);
        const secs = elapsed % 60;
        statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`;
    };
    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
}
//# sourceMappingURL=ui-utils.js.map