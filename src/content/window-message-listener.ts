/**
 * @fileoverview Window Message Listener Module
 * Handles window.postMessage events from inject.js
 */

import type {
  HighlightResponse,
  ExecuteJsResult,
  A11yAuditResult,
  DomQueryResult,
} from '../types';
import type { PageMessageEventData, BackgroundMessageFromContent } from './types';
import {
  resolveHighlightRequest,
  resolveExecuteRequest,
  resolveA11yRequest,
  resolveDomRequest,
} from './request-tracking';
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding';
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking';

/**
 * Initialize consolidated window message listener
 * Handles all messages from inject.js
 */
export function initWindowMessageListener(): void {
  window.addEventListener('message', (event: MessageEvent<PageMessageEventData>) => {
    // Only accept messages from this window
    if (event.source !== window) return;

    const { type: messageType, requestId, result, payload } = event.data || {};

    // Handle highlight responses
    if (messageType === 'GASOLINE_HIGHLIGHT_RESPONSE') {
      if (requestId !== undefined) {
        resolveHighlightRequest(requestId, result as HighlightResponse);
      }
      return;
    }

    // Handle execute JS results
    if (messageType === 'GASOLINE_EXECUTE_JS_RESULT') {
      if (requestId !== undefined) {
        resolveExecuteRequest(requestId, result as ExecuteJsResult);
      }
      return;
    }

    // Handle a11y audit results from inject.js
    if (messageType === 'GASOLINE_A11Y_QUERY_RESPONSE') {
      if (requestId !== undefined) {
        resolveA11yRequest(requestId, result as A11yAuditResult);
      }
      return;
    }

    // Handle DOM query results from inject.js
    if (messageType === 'GASOLINE_DOM_QUERY_RESPONSE') {
      if (requestId !== undefined) {
        resolveDomRequest(requestId, result as DomQueryResult);
      }
      return;
    }

    // Tab isolation filter: only forward captured data from the tracked tab.
    // Response messages (highlight, execute JS, a11y) are NOT filtered because
    // they are responses to explicit commands from the background script.
    if (!getIsTrackedTab()) {
      return; // Drop captured data from untracked tabs
    }

    // Handle MESSAGE_MAP forwarding - attach tabId to every message
    if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
      const mappedType = MESSAGE_MAP[messageType];
      if (mappedType) {
        safeSendMessage({
          type: mappedType,
          payload,
          tabId: getCurrentTabId(),
        } as BackgroundMessageFromContent);
      }
    }
  });
}
