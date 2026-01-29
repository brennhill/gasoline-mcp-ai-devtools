/**
 * @fileoverview content.ts - Message bridge between page and extension contexts.
 * Injects inject.js into the page as a module script, then listens for
 * window.postMessage events (GASOLINE_LOG, GASOLINE_WS, GASOLINE_NETWORK_BODY,
 * GASOLINE_ENHANCED_ACTION, GASOLINE_PERF_SNAPSHOT) and forwards them to the
 * background service worker via chrome.runtime.sendMessage.
 * Also handles chrome.runtime messages for on-demand queries (DOM, a11y, perf).
 * Design: Tab-scoped filtering - only forwards messages from the explicitly
 * tracked tab. Validates message origin (event.source === window) to prevent
 * cross-frame injection. Attaches tabId to all forwarded messages.
 */

import type {
  PageMessageType,
  ContentToPageMessageType,
  ContentMessage,
  ContentPingResponse,
  HighlightResponse,
  ExecuteJsResult,
  DomQueryResult,
  A11yAuditResult,
  WaterfallEntry,
  StateAction,
  BrowserStateSnapshot,
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot,
  WebSocketCaptureMode,
  StorageChange,
} from './types';
import { createDeferredPromise, promiseRaceWithCleanup } from './lib/timeout-utils';

// ============================================================================
// TYPE DEFINITIONS FOR INTERNAL USE
// ============================================================================

/**
 * Pending request statistics
 */
interface PendingRequestStats {
  readonly highlight: number;
  readonly execute: number;
  readonly a11y: number;
  readonly dom: number;
}

/**
 * Page message event data from inject.js
 */
interface PageMessageEventData {
  type?: PageMessageType;
  requestId?: number;
  result?: unknown;
  payload?: unknown;
}

/**
 * Setting message to be posted to page context
 */
interface SettingMessage {
  type: 'GASOLINE_SETTING';
  setting: string;
  enabled?: boolean;
  mode?: WebSocketCaptureMode;
  url?: string;
}

/**
 * Highlight request message to page context
 */
interface HighlightRequestMessage {
  type: 'GASOLINE_HIGHLIGHT_REQUEST';
  requestId: number;
  params: {
    selector: string;
    duration_ms?: number;
  };
}

/**
 * Execute JS request message to page context
 */
interface ExecuteJsRequestMessage {
  type: 'GASOLINE_EXECUTE_JS';
  requestId: number;
  script: string;
  timeoutMs: number;
}

/**
 * A11y query request message to page context
 */
interface A11yQueryRequestMessage {
  type: 'GASOLINE_A11Y_QUERY';
  requestId: number;
  params: Record<string, unknown>;
}

/**
 * DOM query request message to page context
 */
interface DomQueryRequestMessage {
  type: 'GASOLINE_DOM_QUERY';
  requestId: number;
  params: Record<string, unknown>;
}

/**
 * Get waterfall request message to page context
 */
interface GetWaterfallRequestMessage {
  type: 'GASOLINE_GET_WATERFALL';
  requestId: number;
}

/**
 * State command message to page context
 */
interface StateCommandMessage {
  type: 'GASOLINE_STATE_COMMAND';
  messageId: string;
  action: StateAction;
  name?: string;
  state?: BrowserStateSnapshot;
  include_url?: boolean;
}

/**
 * Union of all messages posted to page context
 */
type PagePostMessage =
  | SettingMessage
  | HighlightRequestMessage
  | ExecuteJsRequestMessage
  | A11yQueryRequestMessage
  | DomQueryRequestMessage
  | GetWaterfallRequestMessage
  | StateCommandMessage;

/**
 * Background message types sent from content script
 */
interface LogMessageToBackground {
  type: 'log';
  payload: LogEntry;
  tabId: number | null;
}

interface WsEventMessageToBackground {
  type: 'ws_event';
  payload: WebSocketEvent;
  tabId: number | null;
}

interface NetworkBodyMessageToBackground {
  type: 'network_body';
  payload: NetworkBodyPayload;
  tabId: number | null;
}

interface EnhancedActionMessageToBackground {
  type: 'enhanced_action';
  payload: EnhancedAction;
  tabId: number | null;
}

interface PerformanceSnapshotMessageToBackground {
  type: 'performance_snapshot';
  payload: PerformanceSnapshot;
  tabId: number | null;
}

type BackgroundMessageFromContent =
  | LogMessageToBackground
  | WsEventMessageToBackground
  | NetworkBodyMessageToBackground
  | EnhancedActionMessageToBackground
  | PerformanceSnapshotMessageToBackground;

// ============================================================================
// TAB TRACKING STATE
// ============================================================================

// Whether this content script's tab is the currently tracked tab
let isTrackedTab = false;
// The tab ID of this content script's tab
let currentTabId: number | null = null;

/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
async function updateTrackingStatus(): Promise<void> {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId']);

    // Request tab ID from background script (content scripts can't access chrome.tabs)
    const response = await chrome.runtime.sendMessage({ type: 'GET_TAB_ID' }) as { tabId?: number } | undefined;
    currentTabId = response?.tabId ?? null;

    isTrackedTab =
      currentTabId !== null &&
      currentTabId !== undefined &&
      currentTabId === storage.trackedTabId;
  } catch {
    // Graceful degradation: if we can't check, assume not tracked
    isTrackedTab = false;
  }
}

// Initialize tracking status on script load
updateTrackingStatus();

// Listen for tracking changes in storage
chrome.storage.onChanged.addListener((changes: { [key: string]: StorageChange }) => {
  if (changes.trackedTabId) {
    updateTrackingStatus();
  }
});

// Note: chrome.tabs is NOT available in content scripts.
// Tab activation re-checks happen via storage change events:
// when the popup tracks a new tab, it writes trackedTabId to storage,
// which triggers the storage.onChanged listener above.

// ============================================================================
// SCRIPT INJECTION
// ============================================================================

// Inject the capture script into the page
function injectScript(): void {
  const script = document.createElement('script');
  script.src = chrome.runtime.getURL('inject.js');
  script.type = 'module';
  script.onload = () => script.remove();
  (document.head || document.documentElement).appendChild(script);
}

// Dispatch table: page postMessage type -> background message type
const MESSAGE_MAP: Record<string, string> = {
  GASOLINE_LOG: 'log',
  GASOLINE_WS: 'ws_event',
  GASOLINE_NETWORK_BODY: 'network_body',
  GASOLINE_ENHANCED_ACTION: 'enhanced_action',
  GASOLINE_PERFORMANCE_SNAPSHOT: 'performance_snapshot',
} as const;

// Track whether the extension context is still valid
let contextValid = true;

function safeSendMessage(msg: BackgroundMessageFromContent): void {
  if (!contextValid) return;
  try {
    chrome.runtime.sendMessage(msg);
  } catch (e) {
    if (e instanceof Error && e.message?.includes('Extension context invalidated')) {
      contextValid = false;
      console.warn(
        '[Gasoline] Please refresh this page. The Gasoline extension was reloaded ' +
          'and this page still has the old content script. A page refresh will ' +
          'reconnect capture automatically.'
      );
    }
  }
}

// ============================================================================
// AI WEB PILOT: PENDING REQUEST TRACKING
// ============================================================================

// Pending highlight response resolvers (keyed by request ID)
const pendingHighlightRequests = new Map<number, (result: HighlightResponse) => void>();
let highlightRequestId = 0;

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map<number, (result: ExecuteJsResult) => void>();
let executeRequestId = 0;

// Pending a11y audit requests waiting for responses from inject.js
const pendingA11yRequests = new Map<number, (result: A11yAuditResult) => void>();
let a11yRequestId = 0;

// Pending DOM query requests waiting for responses from inject.js
const pendingDomRequests = new Map<number, (result: DomQueryResult) => void>();
let domRequestId = 0;

// ============================================================================
// ISSUE 2 FIX: PENDING REQUEST CLEANUP ON PAGE UNLOAD
// ============================================================================

/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests(): void {
  pendingHighlightRequests.clear();
  pendingExecuteRequests.clear();
  pendingA11yRequests.clear();
  pendingDomRequests.clear();
}

/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export function getPendingRequestStats(): PendingRequestStats {
  return {
    highlight: pendingHighlightRequests.size,
    execute: pendingExecuteRequests.size,
    a11y: pendingA11yRequests.size,
    dom: pendingDomRequests.size,
  };
}

// Register cleanup handlers for page unload/navigation (Issue 2 fix)
// Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
window.addEventListener('pagehide', clearPendingRequests);
window.addEventListener('beforeunload', clearPendingRequests);

// ============================================================================
// CONSOLIDATED WINDOW MESSAGE LISTENER
// ============================================================================

// Consolidated message listener for all injected script messages
window.addEventListener('message', (event: MessageEvent<PageMessageEventData>) => {
  // Only accept messages from this window
  if (event.source !== window) return;

  const { type: messageType, requestId, result, payload } = event.data || {};

  // Handle highlight responses
  if (messageType === 'GASOLINE_HIGHLIGHT_RESPONSE') {
    if (requestId !== undefined) {
      const resolve = pendingHighlightRequests.get(requestId);
      if (resolve) {
        pendingHighlightRequests.delete(requestId);
        resolve(result as HighlightResponse);
      }
    }
    return;
  }

  // Handle execute JS results
  if (messageType === 'GASOLINE_EXECUTE_JS_RESULT') {
    if (requestId !== undefined) {
      const sendResponse = pendingExecuteRequests.get(requestId);
      if (sendResponse) {
        pendingExecuteRequests.delete(requestId);
        sendResponse(result as ExecuteJsResult);
      }
    }
    return;
  }

  // Handle a11y audit results from inject.js
  if (messageType === 'GASOLINE_A11Y_QUERY_RESPONSE') {
    if (requestId !== undefined) {
      const sendResponse = pendingA11yRequests.get(requestId);
      if (sendResponse) {
        pendingA11yRequests.delete(requestId);
        sendResponse(result as A11yAuditResult);
      }
    }
    return;
  }

  // Handle DOM query results from inject.js
  if (messageType === 'GASOLINE_DOM_QUERY_RESPONSE') {
    if (requestId !== undefined) {
      const sendResponse = pendingDomRequests.get(requestId);
      if (sendResponse) {
        pendingDomRequests.delete(requestId);
        sendResponse(result as DomQueryResult);
      }
    }
    return;
  }

  // Tab isolation filter: only forward captured data from the tracked tab.
  // Response messages (highlight, execute JS, a11y) are NOT filtered because
  // they are responses to explicit commands from the background script.
  if (!isTrackedTab) {
    return; // Drop captured data from untracked tabs
  }

  // Handle MESSAGE_MAP forwarding - attach tabId to every message
  if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
    const mappedType = MESSAGE_MAP[messageType];
    if (mappedType) {
      safeSendMessage({
        type: mappedType,
        payload,
        tabId: currentTabId,
      } as BackgroundMessageFromContent);
    }
  }
});

// Feature toggle message types forwarded from background to inject.js
const TOGGLE_MESSAGES = new Set([
  'setNetworkWaterfallEnabled',
  'setPerformanceMarksEnabled',
  'setActionReplayEnabled',
  'setWebSocketCaptureEnabled',
  'setWebSocketCaptureMode',
  'setPerformanceSnapshotEnabled',
  'setDeferralEnabled',
  'setNetworkBodyCaptureEnabled',
  'setServerUrl',
]);

// ============================================================================
// HELPER: REQUEST TIMEOUT CLEANUP
// ============================================================================

/**
 * Create a timeout handler that cleans up a pending request from a Map
 * @param requestId - The request ID to clean up
 * @param pendingMap - Map of pending requests with callbacks
 * @param errorResponse - Error response to send
 * @returns Callback function to be used as timeout handler
 */
function createRequestTimeoutCleanup<T extends { error: string }>(
  requestId: number,
  pendingMap: Map<number, (result: T) => void>,
  errorResponse: T
): () => void {
  return () => {
    if (pendingMap.has(requestId)) {
      const cb = pendingMap.get(requestId);
      pendingMap.delete(requestId);
      if (cb) {
        cb(errorResponse);
      }
    }
  };
}

// ============================================================================
// AI WEB PILOT: HIGHLIGHT MESSAGE FORWARDING
// ============================================================================

/**
 * Forward a highlight message from background to inject.js
 * @param message - The GASOLINE_HIGHLIGHT message
 * @returns Result from inject.js
 */
function forwardHighlightMessage(message: { params: { selector: string; duration_ms?: number } }): Promise<HighlightResponse> {
  const requestId = ++highlightRequestId;
  const deferred = createDeferredPromise<HighlightResponse>();
  pendingHighlightRequests.set(requestId, deferred.resolve);

  // Post message to page context (inject.js)
  window.postMessage(
    {
      type: 'GASOLINE_HIGHLIGHT_REQUEST',
      requestId,
      params: message.params,
    } satisfies HighlightRequestMessage,
    window.location.origin
  );

  // Timeout fallback + cleanup stale entries after 30 seconds
  // Guarded against double-resolution: Both this timeout and the response handler check
  // has() before get(). JavaScript is single-threaded, so only the first to run will
  // delete the entry; the second's get() returns undefined, preventing double-callback.
  return promiseRaceWithCleanup(
    deferred.promise,
    30000,
    { success: false, error: 'timeout' },
    () => {
      if (pendingHighlightRequests.has(requestId)) {
        pendingHighlightRequests.delete(requestId);
      }
    }
  );
}

// ============================================================================
// STATE MANAGEMENT COMMAND HANDLER
// ============================================================================

interface StateCommandParams {
  action?: StateAction;
  name?: string;
  state?: BrowserStateSnapshot;
  include_url?: boolean;
}

interface StateCommandResult {
  error?: string;
  [key: string]: unknown;
}

/**
 * Handle state capture/restore commands
 */
async function handleStateCommand(params: StateCommandParams | undefined): Promise<StateCommandResult> {
  const { action, name, state, include_url } = params || {};

  // Create a promise to receive response from inject.js
  const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`;
  const deferred = createDeferredPromise<StateCommandResult>();

  // Set up listener for response from inject.js
  const responseHandler = (event: MessageEvent<{ type?: string; messageId?: string; result?: StateCommandResult }>) => {
    if (event.source !== window) return;
    if (event.data?.type === 'GASOLINE_STATE_RESPONSE' && event.data?.messageId === messageId) {
      window.removeEventListener('message', responseHandler);
      deferred.resolve(event.data.result || { error: 'No result from state command' });
    }
  };
  window.addEventListener('message', responseHandler);

  // Send command to inject.js (include state for restore action)
  window.postMessage(
    {
      type: 'GASOLINE_STATE_COMMAND',
      messageId,
      action,
      name,
      state,
      include_url,
    } as StateCommandMessage,
    window.location.origin
  );

  // Timeout after 5 seconds with cleanup
  return promiseRaceWithCleanup(
    deferred.promise,
    5000,
    { error: 'State command timeout' },
    () => window.removeEventListener('message', responseHandler)
  );
}

// ============================================================================
// MESSAGE HANDLERS FROM BACKGROUND
// ============================================================================

/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
function isValidBackgroundSender(sender: any): boolean {
  // Messages from background should NOT have a tab (or have tab with chrome-extension:// url)
  // Messages from content scripts have tab.id
  // We only want messages from the background service worker
  // Cast to any to access runtime.id which isn't always typed
  return typeof sender.id === 'string' && sender.id === (chrome.runtime as any).id;
}

// Listen for messages from background (feature toggles and pilot commands)
chrome.runtime.onMessage.addListener(
  (
    message: ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string; params?: unknown },
    sender: chrome.runtime.MessageSender,
    sendResponse: (response?: unknown) => void
  ): boolean | undefined => {
    // SECURITY: Validate sender is from the extension background, not from page context
    if (!isValidBackgroundSender(sender)) {
      console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id);
      return false;
    }

    // Handle ping to check if content script is loaded
    if (message.type === 'GASOLINE_PING') {
      sendResponse({ status: 'alive', timestamp: Date.now() } satisfies ContentPingResponse);
      return true;
    }

    if (TOGGLE_MESSAGES.has(message.type)) {
      const payload: SettingMessage = { type: 'GASOLINE_SETTING', setting: message.type };
      if (message.type === 'setWebSocketCaptureMode') {
        payload.mode = message.mode;
      } else if (message.type === 'setServerUrl') {
        payload.url = message.url;
      } else {
        payload.enabled = message.enabled;
      }
      // SECURITY: Use explicit targetOrigin (window.location.origin) not "*"
      // Prevents message interception by other extensions/cross-origin iframes
      window.postMessage(payload, window.location.origin);
    }

    // Handle GASOLINE_HIGHLIGHT from background
    if (message.type === 'GASOLINE_HIGHLIGHT') {
      forwardHighlightMessage(message)
        .then((result) => {
          sendResponse(result);
        })
        .catch((err: Error) => {
          sendResponse({ success: false, error: err.message });
        });
      return true; // Will respond asynchronously
    }

    // Handle state management commands from background
    if (message.type === 'GASOLINE_MANAGE_STATE') {
      handleStateCommand(message.params as StateCommandParams)
        .then((result) => sendResponse(result))
        .catch((err: Error) => sendResponse({ error: err.message }));
      return true; // Keep channel open for async response
    }

    // Handle GASOLINE_EXECUTE_JS from background (direct pilot command)
    if (message.type === 'GASOLINE_EXECUTE_JS') {
      const requestId = ++executeRequestId;
      const params = (message.params as { script?: string; timeout_ms?: number }) || {};

      // Store the sendResponse callback for when we get the result
      pendingExecuteRequests.set(requestId, sendResponse as (result: ExecuteJsResult) => void);

      // Timeout fallback: respond with error and cleanup after 30 seconds
      setTimeout(
        createRequestTimeoutCleanup(
          requestId,
          pendingExecuteRequests,
          { success: false, error: 'timeout', message: 'Execute request timed out after 30s' }
        ),
        30000
      );

      // Forward to inject.js via postMessage
      window.postMessage(
        {
          type: 'GASOLINE_EXECUTE_JS',
          requestId,
          script: params.script || '',
          timeoutMs: params.timeout_ms || 5000,
        } satisfies ExecuteJsRequestMessage,
        window.location.origin
      );

      // Return true to indicate we'll respond asynchronously
      return true;
    }

    // Handle GASOLINE_EXECUTE_QUERY from background (polling system)
    if (message.type === 'GASOLINE_EXECUTE_QUERY') {
      const requestId = ++executeRequestId;
      const params = message.params || {};

      // Parse params if it's a string (from JSON)
      let parsedParams: { script?: string; timeout_ms?: number } = {};
      if (typeof params === 'string') {
        try {
          parsedParams = JSON.parse(params) as { script?: string; timeout_ms?: number };
        } catch {
          parsedParams = {};
        }
      } else if (typeof params === 'object') {
        parsedParams = params as { script?: string; timeout_ms?: number };
      }

      // Store the sendResponse callback for when we get the result
      pendingExecuteRequests.set(requestId, sendResponse as (result: ExecuteJsResult) => void);

      // Timeout fallback: respond with error and cleanup after 30 seconds
      setTimeout(
        createRequestTimeoutCleanup(
          requestId,
          pendingExecuteRequests,
          { success: false, error: 'timeout', message: 'Execute query timed out after 30s' }
        ),
        30000
      );

      // Forward to inject.js via postMessage
      window.postMessage(
        {
          type: 'GASOLINE_EXECUTE_JS',
          requestId,
          script: parsedParams.script || '',
          timeoutMs: parsedParams.timeout_ms || 5000,
        } satisfies ExecuteJsRequestMessage,
        window.location.origin
      );

      // Return true to indicate we'll respond asynchronously
      return true;
    }

    // Handle A11Y_QUERY from background (run accessibility audit in page context)
    if (message.type === 'A11Y_QUERY') {
      const requestId = ++a11yRequestId;
      const params = message.params || {};

      // Parse params if it's a string (from JSON)
      let parsedParams: Record<string, unknown> = {};
      if (typeof params === 'string') {
        try {
          parsedParams = JSON.parse(params) as Record<string, unknown>;
        } catch {
          parsedParams = {};
        }
      } else if (typeof params === 'object') {
        parsedParams = params as Record<string, unknown>;
      }

      // Store the sendResponse callback for when we get the result
      pendingA11yRequests.set(requestId, sendResponse as (result: A11yAuditResult) => void);

      // Timeout fallback: respond with error and cleanup after 30 seconds (a11y audits take longer)
      setTimeout(
        createRequestTimeoutCleanup(
          requestId,
          pendingA11yRequests,
          { error: 'Accessibility audit timeout' } as A11yAuditResult
        ),
        30000
      );

      // Forward to inject.js via postMessage
      window.postMessage(
        {
          type: 'GASOLINE_A11Y_QUERY',
          requestId,
          params: parsedParams,
        } satisfies A11yQueryRequestMessage,
        window.location.origin
      );

      return true; // Will respond asynchronously
    }

    // Handle DOM_QUERY from background (execute CSS selector query in page context)
    if (message.type === 'DOM_QUERY') {
      const requestId = ++domRequestId;
      const params = message.params || {};

      // Parse params if it's a string (from JSON)
      let parsedParams: Record<string, unknown> = {};
      if (typeof params === 'string') {
        try {
          parsedParams = JSON.parse(params) as Record<string, unknown>;
        } catch {
          parsedParams = {};
        }
      } else if (typeof params === 'object') {
        parsedParams = params as Record<string, unknown>;
      }

      // Store the sendResponse callback for when we get the result
      pendingDomRequests.set(requestId, sendResponse as (result: DomQueryResult) => void);

      // Timeout fallback: respond with error and cleanup after 30 seconds
      setTimeout(
        createRequestTimeoutCleanup(
          requestId,
          pendingDomRequests,
          { error: 'DOM query timeout' } as DomQueryResult
        ),
        30000
      );

      // Forward to inject.js via postMessage
      window.postMessage(
        {
          type: 'GASOLINE_DOM_QUERY',
          requestId,
          params: parsedParams,
        } satisfies DomQueryRequestMessage,
        window.location.origin
      );

      return true; // Will respond asynchronously
    }

    // Handle GET_NETWORK_WATERFALL from background (collect PerformanceResourceTiming data)
    if (message.type === 'GET_NETWORK_WATERFALL') {
      // Query the injected gasoline API for waterfall data
      const requestId = Date.now();
      const deferred = createDeferredPromise<{ entries: WaterfallEntry[] }>();

      // Set up a one-time listener for the response
      const responseHandler = (event: MessageEvent<{ type?: string; entries?: WaterfallEntry[] }>) => {
        if (event.source !== window) return;
        if (event.data?.type === 'GASOLINE_WATERFALL_RESPONSE') {
          window.removeEventListener('message', responseHandler);
          deferred.resolve({ entries: event.data.entries || [] });
        }
      };

      window.addEventListener('message', responseHandler);

      // Post message to page context
      window.postMessage(
        {
          type: 'GASOLINE_GET_WATERFALL',
          requestId,
        } satisfies GetWaterfallRequestMessage,
        window.location.origin
      );

      // Timeout fallback: respond with empty array after 5 seconds
      promiseRaceWithCleanup(deferred.promise, 5000, { entries: [] }, () => {
        window.removeEventListener('message', responseHandler);
      }).then((result) => {
        sendResponse(result);
      });

      return true; // Will respond asynchronously
    }

    return undefined;
  }
);

// ============================================================================
// SCRIPT INJECTION ON DOM READY
// ============================================================================

// Inject when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectScript, { once: true });
} else {
  injectScript();
}
