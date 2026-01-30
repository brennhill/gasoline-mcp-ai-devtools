/**
 * @fileoverview Exception and unhandled rejection capture.
 * Monkey-patches window.onerror and listens for unhandledrejection events,
 * enriching errors with AI context before posting via bridge.
 */

import { postLog, type BridgePayload } from './bridge';
import { enrichErrorWithAiContext } from './ai-context';

interface ExceptionEntry extends Record<string, unknown> {
  level: 'error';
  type: 'exception';
  message: string;
  source?: string;
  filename?: string;
  lineno?: number;
  colno?: number;
  stack?: string;
}

// Exception capture state
let originalOnerror: OnErrorEventHandler | null = null;
let unhandledrejectionHandler: ((event: PromiseRejectionEvent) => void) | null = null;

/**
 * Install exception capture
 */
export function installExceptionCapture(): void {
  originalOnerror = window.onerror;

  window.onerror = function (
    message: string | Event,
    filename?: string,
    lineno?: number,
    colno?: number,
    error?: Error,
  ): boolean | void {
    const messageStr = typeof message === 'string' ? message : (message as Event).type || 'Error';
    const entry: ExceptionEntry = {
      level: 'error',
      type: 'exception',
      message: messageStr,
      source: filename ? `${filename}:${lineno || 0}` : '',
      filename: filename || '',
      lineno: lineno || 0,
      colno: colno || 0,
      stack: error?.stack || '',
    };

    // Enrich with AI context then post (async, fire-and-forget)
    void (async (): Promise<void> => {
      try {
        const enriched = await enrichErrorWithAiContext(entry);
        postLog(enriched as unknown as BridgePayload);
      } catch {
        postLog(entry as unknown as BridgePayload);
      }
    })().catch((err: Error) => {
      console.error('[Gasoline] Exception enrichment error:', err);
      // Fallback: ensure entry is logged even if something fails
      try {
        postLog(entry as unknown as BridgePayload);
      } catch (postErr) {
        console.error('[Gasoline] Failed to log entry:', postErr);
      }
    });

    // Call original if exists
    if (originalOnerror) {
      return originalOnerror(message, filename, lineno, colno, error);
    }
    return false;
  };

  // Unhandled promise rejections
  unhandledrejectionHandler = function (event: PromiseRejectionEvent): void {
    const error = event.reason;
    let message = '';
    let stack = '';

    if (error instanceof Error) {
      message = error.message;
      stack = error.stack || '';
    } else if (typeof error === 'string') {
      message = error;
    } else {
      message = String(error);
    }

    const entry: ExceptionEntry = {
      level: 'error',
      type: 'exception',
      message: `Unhandled Promise Rejection: ${message}`,
      stack,
    };

    // Enrich with AI context then post (async, fire-and-forget)
    void (async (): Promise<void> => {
      try {
        const enriched = await enrichErrorWithAiContext(entry);
        postLog(enriched as unknown as BridgePayload);
      } catch {
        postLog(entry as unknown as BridgePayload);
      }
    })().catch((err: Error) => {
      console.error('[Gasoline] Exception enrichment error:', err);
      // Fallback: ensure entry is logged even if something fails
      try {
        postLog(entry as unknown as BridgePayload);
      } catch (postErr) {
        console.error('[Gasoline] Failed to log entry:', postErr);
      }
    });
  };

  window.addEventListener('unhandledrejection', unhandledrejectionHandler);
}

/**
 * Uninstall exception capture
 */
export function uninstallExceptionCapture(): void {
  if (originalOnerror !== null) {
    window.onerror = originalOnerror;
    originalOnerror = null;
  }

  if (unhandledrejectionHandler) {
    window.removeEventListener('unhandledrejection', unhandledrejectionHandler);
    unhandledrejectionHandler = null;
  }
}
