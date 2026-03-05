/**
 * Purpose: Apply persisted light-theme class early for extension UI pages.
 * Must remain an external script to satisfy MV3 CSP (no inline script tags).
 */

chrome.storage.local.get('theme', (result) => {
  if (result?.theme === 'light') {
    document.body?.classList.add('light-theme')
  }
})
