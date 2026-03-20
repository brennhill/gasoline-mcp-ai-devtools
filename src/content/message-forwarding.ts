/**
 * Purpose: Forwards window.postMessage events from the inject context to the background script via chrome.runtime.sendMessage.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Message Forwarding Module
 * Forwards messages between page context and background script
 */

import type { BackgroundMessageFromContent } from './types.js'

// Dispatch table: page postMessage type -> background message type
export const MESSAGE_MAP: Record<string, string> = {
  gasoline_log: 'log',
  gasoline_ws: 'ws_event',
  gasoline_network_body: 'network_body',
  gasoline_enhanced_action: 'enhanced_action',
  gasoline_performance_snapshot: 'performance_snapshot'
} as const

// Track whether the extension context is still valid
let contextValid = true

/**
 * Safely send a message to the background script
 * Handles extension context invalidation gracefully
 */
export function safeSendMessage(msg: BackgroundMessageFromContent): void {
  if (!contextValid) return
  try {
    chrome.runtime.sendMessage(msg)
  } catch (e) {
    if (e instanceof Error && e.message?.includes('Extension context invalidated')) {
      contextValid = false
      console.warn(
        '[Gasoline] Please refresh this page. The Gasoline extension was reloaded ' +
          'and this page still has the old content script. A page refresh will ' +
          'reconnect capture automatically.'
      )
    }
  }
}

/**
 * Check if the extension context is still valid
 */
function isContextValid(): boolean {
  return contextValid
}
