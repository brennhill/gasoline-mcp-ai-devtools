/**
 * @fileoverview Message Handlers Module
 * Handles messages from background script
 */
import type {
  ContentMessage,
  ContentPingResponse,
  WebSocketCaptureMode,
  HighlightResponse,
  WaterfallEntry,
  StateAction,
  BrowserStateSnapshot,
  A11yAuditResult,
} from '../types'
export declare const TOGGLE_MESSAGES: Set<string>
/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export declare function isValidBackgroundSender(sender: chrome.runtime.MessageSender): boolean
/**
 * Forward a highlight message from background to inject.js
 */
export declare function forwardHighlightMessage(message: {
  params: {
    selector: string
    duration_ms?: number
  }
}): Promise<HighlightResponse>
/**
 * Handle state capture/restore commands
 */
export declare function handleStateCommand(
  params:
    | {
        action?: StateAction
        name?: string
        state?: BrowserStateSnapshot
        include_url?: boolean
      }
    | undefined,
): Promise<{
  error?: string
  [key: string]: unknown
}>
/**
 * Handle GASOLINE_PING message
 */
export declare function handlePing(sendResponse: (response: ContentPingResponse) => void): boolean
/**
 * Handle toggle messages
 */
export declare function handleToggleMessage(
  message: ContentMessage & {
    enabled?: boolean
    mode?: WebSocketCaptureMode
    url?: string
  },
): void
/**
 * Handle GASOLINE_EXECUTE_JS message
 */
export declare function handleExecuteJs(
  params: {
    script?: string
    timeout_ms?: number
  },
  sendResponse: (result: { success: boolean; error?: string; message?: string; result?: unknown }) => void,
): boolean
/**
 * Handle GASOLINE_EXECUTE_QUERY message
 */
export declare function handleExecuteQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: { success: boolean; error?: string; message?: string; result?: unknown }) => void,
): boolean
/**
 * Handle A11Y_QUERY message
 */
export declare function handleA11yQuery(
  params: string | Record<string, unknown>,
  sendResponse: (
    result:
      | A11yAuditResult
      | {
          error: string
        },
  ) => void,
): boolean
/**
 * Handle DOM_QUERY message
 */
export declare function handleDomQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: { error?: string; matches?: unknown[] }) => void,
): boolean
/**
 * Handle GET_NETWORK_WATERFALL message
 */
export declare function handleGetNetworkWaterfall(
  sendResponse: (result: { entries: WaterfallEntry[] }) => void,
): boolean
//# sourceMappingURL=message-handlers.d.ts.map
