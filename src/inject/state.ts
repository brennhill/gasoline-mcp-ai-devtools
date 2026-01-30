/**
 * @fileoverview State Management - Handles browser state capture/restore and
 * element highlighting for the AI Web Pilot.
 */

import type { BrowserStateSnapshot } from '../types/index';
import { sendPerformanceSnapshot } from '../lib/perf-snapshot';

let gasolineHighlighter: HTMLDivElement | null = null;

/**
 * Highlight result
 */
export interface HighlightResult {
  success: boolean;
  selector?: string;
  bounds?: { x: number; y: number; width: number; height: number };
  error?: string;
}

/**
 * Restored state counts
 */
export interface RestoredCounts {
  localStorage: number;
  sessionStorage: number;
  cookies: number;
  skipped: number;
}

/**
 * Restore state result
 */
export interface RestoreStateResult {
  success: boolean;
  restored?: RestoredCounts;
  error?: string;
}

/**
 * Capture browser state (localStorage, sessionStorage, cookies).
 * Returns a snapshot that can be restored later.
 */
export function captureState(): BrowserStateSnapshot {
  const state: BrowserStateSnapshot = {
    url: window.location.href,
    timestamp: Date.now(),
    localStorage: {},
    sessionStorage: {},
    cookies: document.cookie,
  };

  const localStorageData: Record<string, string> = {};
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key) {
      localStorageData[key] = localStorage.getItem(key) || '';
    }
  }
  (state as { localStorage: Record<string, string> }).localStorage = localStorageData;

  const sessionStorageData: Record<string, string> = {};
  for (let i = 0; i < sessionStorage.length; i++) {
    const key = sessionStorage.key(i);
    if (key) {
      sessionStorageData[key] = sessionStorage.getItem(key) || '';
    }
  }
  (state as { sessionStorage: Record<string, string> }).sessionStorage = sessionStorageData;

  return state;
}

/**
 * Validates a storage key to prevent prototype pollution and other attacks
 */
function isValidStorageKey(key: string): boolean {
  if (typeof key !== 'string') return false;
  if (key.length === 0 || key.length > 256) return false;

  // Reject prototype pollution vectors
  const dangerous = ['__proto__', 'constructor', 'prototype'];
  const lowerKey = key.toLowerCase();
  for (const pattern of dangerous) {
    if (lowerKey.includes(pattern)) return false;
  }

  return true;
}

/**
 * Restore browser state from a snapshot.
 * Clears existing state before restoring.
 */
export function restoreState(state: BrowserStateSnapshot, includeUrl: boolean = true): RestoreStateResult {
  // Validate state object
  if (!state || typeof state !== 'object') {
    return { success: false, error: 'Invalid state object' };
  }

  // Clear existing
  localStorage.clear();
  sessionStorage.clear();

  // Restore localStorage with validation
  let skipped = 0;
  for (const [key, value] of Object.entries(state.localStorage || {})) {
    if (!isValidStorageKey(key)) {
      skipped++;
      console.warn('[gasoline] Skipped localStorage key with invalid pattern:', key);
      continue;
    }
    // Limit value size (10MB max per item)
    if (typeof value === 'string' && value.length > 10 * 1024 * 1024) {
      skipped++;
      console.warn('[gasoline] Skipped localStorage value exceeding 10MB:', key);
      continue;
    }
    localStorage.setItem(key, value);
  }

  // Restore sessionStorage with validation
  for (const [key, value] of Object.entries(state.sessionStorage || {})) {
    if (!isValidStorageKey(key)) {
      skipped++;
      console.warn('[gasoline] Skipped sessionStorage key with invalid pattern:', key);
      continue;
    }
    if (typeof value === 'string' && value.length > 10 * 1024 * 1024) {
      skipped++;
      console.warn('[gasoline] Skipped sessionStorage value exceeding 10MB:', key);
      continue;
    }
    sessionStorage.setItem(key, value);
  }

  // Restore cookies (clear then set)
  document.cookie.split(';').forEach((c) => {
    const namePart = c.split('=')[0];
    if (namePart) {
      const name = namePart.trim();
      if (name) {
        document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
      }
    }
  });

  if (state.cookies) {
    state.cookies.split(';').forEach((c) => {
      document.cookie = c.trim();
    });
  }

  const restored: RestoredCounts = {
    localStorage: Object.keys(state.localStorage || {}).length - skipped,
    sessionStorage: Object.keys(state.sessionStorage || {}).length,
    cookies: (state.cookies || '').split(';').filter((c) => c.trim()).length,
    skipped,
  };

  // Navigate if requested (with basic URL validation)
  if (includeUrl && state.url && state.url !== window.location.href) {
    // Basic URL validation: must be http/https
    try {
      const url = new URL(state.url);
      if (url.protocol === 'http:' || url.protocol === 'https:') {
        window.location.href = state.url;
      } else {
        console.warn('[gasoline] Skipped navigation to non-HTTP(S) URL:', state.url);
      }
    } catch (e) {
      console.warn('[gasoline] Invalid URL for navigation:', state.url, e);
    }
  }

  if (skipped > 0) {
    console.warn(`[gasoline] restoreState completed with ${skipped} skipped item(s)`);
  }

  return { success: true, restored };
}

/**
 * Highlight a DOM element by injecting a red overlay div.
 */
export function highlightElement(selector: string, durationMs: number = 5000): HighlightResult | undefined {
  // Remove existing highlight
  if (gasolineHighlighter) {
    gasolineHighlighter.remove();
    gasolineHighlighter = null;
  }

  const element = document.querySelector(selector);
  if (!element) {
    return { success: false, error: 'element_not_found', selector };
  }

  const rect = element.getBoundingClientRect();

  gasolineHighlighter = document.createElement('div');
  gasolineHighlighter.id = 'gasoline-highlighter';
  gasolineHighlighter.dataset.selector = selector;
  Object.assign(gasolineHighlighter.style, {
    position: 'fixed',
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    border: '4px solid red',
    borderRadius: '4px',
    backgroundColor: 'rgba(255, 0, 0, 0.1)',
    zIndex: '2147483647',
    pointerEvents: 'none',
    boxSizing: 'border-box',
  });

  const targetElement = document.body || document.documentElement;
  if (targetElement) {
    targetElement.appendChild(gasolineHighlighter);
  } else {
    console.warn('[Gasoline] No document body available for highlighter injection');
    return;
  }

  setTimeout(() => {
    if (gasolineHighlighter) {
      gasolineHighlighter.remove();
      gasolineHighlighter = null;
    }
  }, durationMs);

  return {
    success: true,
    selector,
    bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height },
  };
}

/**
 * Clear any existing highlight
 */
export function clearHighlight(): void {
  if (gasolineHighlighter) {
    gasolineHighlighter.remove();
    gasolineHighlighter = null;
  }
}

/**
 * Handle scroll - update highlight position
 */
if (typeof window !== 'undefined') {
  window.addEventListener(
    'scroll',
    () => {
      if (gasolineHighlighter) {
        const selector = gasolineHighlighter.dataset.selector;
        if (selector) {
          const el = document.querySelector(selector);
          if (el) {
            const rect = el.getBoundingClientRect();
            gasolineHighlighter.style.top = `${rect.top}px`;
            gasolineHighlighter.style.left = `${rect.left}px`;
          }
        }
      }
    },
    { passive: true },
  );
}

/**
 * Handle GASOLINE_HIGHLIGHT_REQUEST messages from content script
 */
if (typeof window !== 'undefined') {
  window.addEventListener('message', (event: MessageEvent) => {
    if (event.source !== window) return;
    if (event.data?.type === 'GASOLINE_HIGHLIGHT_REQUEST') {
      const { requestId, params } = event.data;
      const { selector, duration_ms } = params || { selector: '' };
      const result = highlightElement(selector, duration_ms);
      window.postMessage(
        {
          type: 'GASOLINE_HIGHLIGHT_RESPONSE',
          requestId,
          result,
        },
        window.location.origin,
      );
    }
  });
}

/**
 * Wrapper for sending performance snapshot (exported for compatibility)
 */
export function sendPerformanceSnapshotWrapper(): void {
  sendPerformanceSnapshot();
}
