/**
 * Purpose: Owns ui-utils.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Popup UI Utilities
 * Helper functions for UI updates
 */

/**
 * Format bytes into human-readable file size
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / Math.pow(1024, i)
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`
}

/**
 * Check if a URL is an internal browser page that cannot be tracked.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 */
export function isInternalUrl(url: string | undefined): boolean {
  if (!url) return true
  const internalPrefixes = ['chrome://', 'chrome-extension://', 'about:', 'edge://', 'brave://', 'devtools://']
  return internalPrefixes.some((prefix) => url.startsWith(prefix))
}
