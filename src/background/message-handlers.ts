/**
 * @fileoverview Message Handlers - Handles all chrome.runtime.onMessage routing
 * with type-safe message discrimination.
 */

import type {
  LogEntry,
  BackgroundMessage,
  ChromeMessageSender,
  BrowserStateSnapshot,
  ConnectionStatus,
  ContextWarning,
  CircuitBreakerState,
  MemoryPressureState,
  WebSocketEvent,
  EnhancedAction,
  NetworkBodyPayload,
  PerformanceSnapshot
} from '../types'

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/** Message handler dependencies */
export interface MessageHandlerDependencies {
  // State getters
  getServerUrl: () => string
  getConnectionStatus: () => ConnectionStatus
  getDebugMode: () => boolean
  getScreenshotOnError: () => boolean
  getSourceMapEnabled: () => boolean
  getCurrentLogLevel: () => string
  getContextWarning: () => ContextWarning | null
  getCircuitBreakerState: () => CircuitBreakerState
  getMemoryPressureState: () => MemoryPressureState
  getAiWebPilotEnabled: () => boolean
  isNetworkBodyCaptureDisabled: () => boolean

  // State setters
  setServerUrl: (url: string) => void
  setCurrentLogLevel: (level: string) => void
  setScreenshotOnError: (enabled: boolean) => void
  setSourceMapEnabled: (enabled: boolean) => void
  setDebugMode: (enabled: boolean) => void
  setAiWebPilotEnabled: (enabled: boolean, callback?: () => void) => void

  // Batchers
  addToLogBatcher: (entry: LogEntry) => void
  addToWsBatcher: (event: WebSocketEvent) => void
  addToEnhancedActionBatcher: (action: EnhancedAction) => void
  addToNetworkBodyBatcher: (body: NetworkBodyPayload) => void
  addToPerfBatcher: (snapshot: PerformanceSnapshot) => void

  // Actions
  handleLogMessage: (payload: LogEntry, sender: ChromeMessageSender, tabId?: number) => Promise<void>
  handleClearLogs: () => Promise<{ success: boolean; error?: string }>
  captureScreenshot: (
    tabId: number,
    relatedErrorId: string | null
  ) => Promise<{
    success: boolean
    entry?: LogEntry
    error?: string
  }>
  checkConnectionAndUpdate: () => Promise<void>
  clearSourceMapCache: () => void

  // Debug logging
  debugLog: (category: string, message: string, data?: unknown) => void
  exportDebugLog: () => string
  clearDebugLog: () => void

  // Storage
  saveSetting: (key: string, value: unknown) => void
  forwardToAllContentScripts: (message: { type: string; [key: string]: unknown }) => void
}

/** Message response type */
type MessageResponse = unknown

/** Send response callback type */
type SendResponse = (response?: MessageResponse) => void

// =============================================================================
// MESSAGE HANDLER
// =============================================================================

/**
 * Security: Validate that sender is from extension or content script
 * Prevents messages from untrusted sources
 */
function isValidMessageSender(sender: ChromeMessageSender & { id?: string }): boolean {
  // Content scripts have sender.tab with tabId and url
  // Background/popup scripts have sender.id === chrome.runtime.id
  // Extension pages (popup, options) have sender.tab?.url starting with 'chrome-extension://'
  if (sender.tab?.id !== undefined && sender.tab?.url) {
    // Content script: has tab context
    return true
  }
  if (typeof chrome !== 'undefined' && chrome.runtime && sender.id === chrome.runtime.id) {
    // Internal extension message
    return true
  }
  // Reject messages from web pages
  return false
}

/**
 * Install the main message listener
 * All messages are validated for sender origin to ensure they come from trusted extension contexts
 */
// #lizard forgives
export function installMessageListener(deps: MessageHandlerDependencies): void {
  if (typeof chrome === 'undefined' || !chrome.runtime) return

  chrome.runtime.onMessage.addListener(
    (message: BackgroundMessage, sender: chrome.runtime.MessageSender, sendResponse: SendResponse): boolean => {
      // SECURITY: Validate sender before processing any message
      if (!isValidMessageSender(sender as ChromeMessageSender & { id?: string })) {
        deps.debugLog('error', 'Rejected message from untrusted sender', { senderId: sender.id, senderUrl: sender.url })
        return false
      }
      return handleMessage(message, sender as ChromeMessageSender, sendResponse, deps)
    }
  )
}

/**
 * Type guard to validate message structure before processing
 * Returns true if message passes validation, logs rejection otherwise
 */
function validateMessageType(message: unknown, expectedType: string, deps: MessageHandlerDependencies): boolean {
  if (typeof message !== 'object' || message === null) {
    deps.debugLog('error', `Invalid message: not an object`, { messageType: typeof message })
    return false
  }
  const msg = message as Record<string, unknown>
  if (msg.type !== expectedType) {
    deps.debugLog('error', `Message type mismatch`, { expected: expectedType, received: msg.type })
    return false
  }
  return true
}

/**
 * Handle incoming message
 * Returns true if response will be sent asynchronously
 * Security: All messages are type-validated using discriminated unions
 */
function handleMessage(
  message: BackgroundMessage,
  sender: ChromeMessageSender,
  sendResponse: SendResponse,
  deps: MessageHandlerDependencies
): boolean {
  const messageType = message.type

  // Type validation: ensure message conforms to expected discriminated union
  // TypeScript's type system ensures exhaustiveness, but add logging for debugging
  switch (messageType) {
    case 'GET_TAB_ID':
      sendResponse({ tabId: sender.tab?.id })
      return true

    case 'ws_event':
      deps.addToWsBatcher(message.payload)
      return false

    case 'enhanced_action':
      deps.addToEnhancedActionBatcher(message.payload)
      return false

    case 'network_body':
      if (deps.isNetworkBodyCaptureDisabled()) {
        deps.debugLog('capture', 'Network body dropped: capture disabled')
        return true
      }
      // Attach tabId to payload before batching (v5.3+)
      deps.addToNetworkBodyBatcher({ ...message.payload, tabId: message.tabId })
      return false

    case 'performance_snapshot':
      deps.addToPerfBatcher(message.payload)
      return false

    case 'log':
      handleLogMessageAsync(message, sender, deps)
      return true

    case 'getStatus':
      sendResponse({
        ...deps.getConnectionStatus(),
        serverUrl: deps.getServerUrl(),
        screenshotOnError: deps.getScreenshotOnError(),
        sourceMapEnabled: deps.getSourceMapEnabled(),
        debugMode: deps.getDebugMode(),
        contextWarning: deps.getContextWarning(),
        circuitBreakerState: deps.getCircuitBreakerState(),
        memoryPressure: deps.getMemoryPressureState()
      })
      return false

    case 'clearLogs':
      handleClearLogsAsync(sendResponse, deps)
      return true

    case 'setLogLevel':
      deps.setCurrentLogLevel(message.level)
      deps.saveSetting('logLevel', message.level)
      return false

    case 'setScreenshotOnError':
      deps.setScreenshotOnError(message.enabled)
      deps.saveSetting('screenshotOnError', message.enabled)
      sendResponse({ success: true })
      return false

    case 'setAiWebPilotEnabled':
      handleSetAiWebPilotEnabled(message.enabled, sendResponse, deps)
      return false

    case 'getAiWebPilotEnabled':
      sendResponse({ enabled: deps.getAiWebPilotEnabled() })
      return false

    case 'getTrackingState':
      handleGetTrackingState(sendResponse, deps, sender.tab?.id)
      return true

    case 'getDiagnosticState':
      handleGetDiagnosticState(sendResponse, deps)
      return true

    case 'captureScreenshot':
      handleCaptureScreenshot(sendResponse, deps)
      return true

    case 'setSourceMapEnabled':
      deps.setSourceMapEnabled(message.enabled)
      deps.saveSetting('sourceMapEnabled', message.enabled)
      if (!message.enabled) {
        deps.clearSourceMapCache()
      }
      sendResponse({ success: true })
      return false

    case 'setNetworkWaterfallEnabled':
    case 'setPerformanceMarksEnabled':
    case 'setActionReplayEnabled':
    case 'setWebSocketCaptureEnabled':
    case 'setWebSocketCaptureMode':
    case 'setPerformanceSnapshotEnabled':
    case 'setDeferralEnabled':
    case 'setNetworkBodyCaptureEnabled':
    case 'setActionToastsEnabled':
    case 'setSubtitlesEnabled':
      handleForwardedSetting(message, sendResponse, deps)
      return false

    case 'setDebugMode':
      deps.setDebugMode(message.enabled)
      deps.saveSetting('debugMode', message.enabled)
      sendResponse({ success: true })
      return false

    case 'getDebugLog':
      sendResponse({ log: deps.exportDebugLog() })
      return false

    case 'clearDebugLog':
      deps.clearDebugLog()
      deps.debugLog('lifecycle', 'Debug log cleared')
      sendResponse({ success: true })
      return false

    case 'setServerUrl':
      handleSetServerUrl(message.url, sendResponse, deps)
      return false

    case 'GASOLINE_CAPTURE_SCREENSHOT':
      // Content script requests screenshot capture (while draw mode overlay is still visible)
      handleDrawModeCaptureScreenshot(sender, sendResponse)
      return true

    case 'DRAW_MODE_COMPLETED':
      // Fire-and-forget: content script sends draw mode results
      handleDrawModeCompletedAsync(message as unknown as Record<string, unknown>, sender, deps)
      return false

    default:
      // Unknown message type
      return false
  }
}

// =============================================================================
// ASYNC HANDLERS
// =============================================================================

async function handleLogMessageAsync(
  message: { type: 'log'; payload: LogEntry; tabId?: number },
  sender: ChromeMessageSender,
  deps: MessageHandlerDependencies
): Promise<void> {
  try {
    await deps.handleLogMessage(message.payload, sender, message.tabId)
  } catch (err) {
    console.error('[Gasoline] Failed to handle log message:', err)
  }
}

// #lizard forgives
async function handleClearLogsAsync(sendResponse: SendResponse, deps: MessageHandlerDependencies): Promise<void> {
  try {
    const result = await deps.handleClearLogs()
    sendResponse(result)
  } catch (err) {
    console.error('[Gasoline] Failed to clear logs:', err)
    sendResponse({ error: (err as Error).message })
  }
}

function handleSetAiWebPilotEnabled(
  enabled: boolean,
  sendResponse: SendResponse,
  deps: MessageHandlerDependencies
): void {
  const newValue = enabled === true
  console.log(`[Gasoline] AI Web Pilot toggle: -> ${newValue}`)

  deps.setAiWebPilotEnabled(newValue, () => {
    console.log(`[Gasoline] AI Web Pilot persisted to storage: ${newValue}`)
    // Settings now sent automatically via /sync
    // Broadcast tracking state change to tracked tab (for favicon flicker)
    broadcastTrackingState()
  })

  sendResponse({ success: true })
}

/**
 * Handle getTrackingState request from content script.
 * Returns current tracking and AI Pilot state for favicon replacer.
 * Uses sender's tab ID (not active tab query) to correctly identify the requesting tab.
 */
async function handleGetTrackingState(
  sendResponse: SendResponse,
  deps: MessageHandlerDependencies,
  senderTabId?: number
): Promise<void> {
  try {
    const result = await chrome.storage.local.get(['trackedTabId'])
    const trackedTabId = result.trackedTabId as number | undefined
    const aiPilotEnabled = deps.getAiWebPilotEnabled()

    sendResponse({
      state: {
        isTracked: senderTabId !== undefined && senderTabId === trackedTabId,
        aiPilotEnabled: aiPilotEnabled
      }
    })
  } catch (err) {
    console.error('[Gasoline] Failed to get tracking state:', err)
    sendResponse({ state: { isTracked: false, aiPilotEnabled: false } })
  }
}

/**
 * Broadcast tracking state to the tracked tab.
 * Used by favicon replacer to show/hide flicker animation.
 * Exported for use in init.ts storage change handlers.
 * @param untrackedTabId - Optional tab ID that was just untracked (to notify it to stop flicker)
 */
export async function broadcastTrackingState(untrackedTabId?: number | null): Promise<void> {
  try {
    const result = await chrome.storage.local.get(['trackedTabId', 'aiWebPilotEnabled'])
    const trackedTabId = result.trackedTabId as number | undefined
    const aiPilotEnabled = result.aiWebPilotEnabled === true

    // Notify the currently tracked tab it's being tracked
    if (trackedTabId) {
      chrome.tabs
        .sendMessage(trackedTabId, {
          type: 'trackingStateChanged',
          state: {
            isTracked: true,
            aiPilotEnabled: aiPilotEnabled
          }
        })
        .catch(() => {
          // Tab might not have content script loaded yet, ignore
        })
    }

    // Notify the previously tracked tab it's no longer tracked (to stop favicon flicker)
    if (untrackedTabId && untrackedTabId !== trackedTabId) {
      chrome.tabs
        .sendMessage(untrackedTabId, {
          type: 'trackingStateChanged',
          state: {
            isTracked: false,
            aiPilotEnabled: false
          }
        })
        .catch(() => {
          // Tab might not have content script loaded, ignore
        })
    }
  } catch (err) {
    console.error('[Gasoline] Failed to broadcast tracking state:', err)
  }
}

function handleGetDiagnosticState(sendResponse: SendResponse, deps: MessageHandlerDependencies): void {
  if (typeof chrome === 'undefined' || !chrome.storage) {
    sendResponse({
      cache: deps.getAiWebPilotEnabled(),
      storage: undefined,
      timestamp: new Date().toISOString()
    })
    return
  }

  chrome.storage.local.get(['aiWebPilotEnabled'], (result: { aiWebPilotEnabled?: boolean }) => {
    sendResponse({
      cache: deps.getAiWebPilotEnabled(),
      storage: result.aiWebPilotEnabled,
      timestamp: new Date().toISOString()
    })
  })
}

function handleCaptureScreenshot(sendResponse: SendResponse, deps: MessageHandlerDependencies): void {
  if (typeof chrome === 'undefined' || !chrome.tabs) {
    sendResponse({ success: false, error: 'Chrome tabs API not available' })
    return
  }

  chrome.tabs.query({ active: true, currentWindow: true }, async (tabs) => {
    if (tabs[0]?.id) {
      const result = await deps.captureScreenshot(tabs[0].id, null)
      if (result.success && result.entry) {
        deps.addToLogBatcher(result.entry)
      }
      sendResponse(result)
    } else {
      sendResponse({ success: false, error: 'No active tab' })
    }
  })
}

function handleForwardedSetting(
  message: { type: string; enabled?: boolean; mode?: string },
  sendResponse: SendResponse,
  deps: MessageHandlerDependencies
): void {
  deps.debugLog('settings', `Setting ${message.type}: ${message.enabled ?? message.mode}`)
  deps.forwardToAllContentScripts(message as { type: string; [key: string]: unknown })
  sendResponse({ success: true })
}

/**
 * Handle GASOLINE_CAPTURE_SCREENSHOT from content script.
 * Captures visible tab while draw mode overlay is still visible (annotations in screenshot).
 */
async function handleDrawModeCaptureScreenshot(sender: ChromeMessageSender, sendResponse: SendResponse): Promise<void> {
  const tabId = sender.tab?.id
  if (!tabId) {
    sendResponse({ dataUrl: '' })
    return
  }
  try {
    const tab = await chrome.tabs.get(tabId)
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, { format: 'png' })
    sendResponse({ dataUrl })
  } catch (err) {
    console.error('[Gasoline] Draw mode screenshot capture failed:', (err as Error).message)
    sendResponse({ dataUrl: '' })
  }
}

/**
 * Handle draw mode completion from content script.
 * Uses screenshot already captured by content script (before overlay removal).
 */
async function handleDrawModeCompletedAsync(
  message: Record<string, unknown>,
  sender: ChromeMessageSender,
  deps: MessageHandlerDependencies
): Promise<void> {
  const tabId = sender.tab?.id
  if (!tabId) return
  try {
    const serverUrl = deps.getServerUrl()
    const body: Record<string, unknown> = {
      screenshot_data_url: (message.screenshot_data_url as string) || '',
      annotations: (message.annotations as unknown[]) || [],
      element_details: (message.elementDetails as Record<string, unknown>) || {},
      page_url: (message.page_url as string) || '',
      tab_id: tabId,
      correlation_id: (message.correlation_id as string) || ''
    }
    if (message.session_name) {
      body.session_name = message.session_name
    }
    const response = await fetch(`${serverUrl}/draw-mode/complete`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
      body: JSON.stringify(body)
    })
    if (!response.ok) {
      const respBody = await response.text().catch(() => '')
      deps.debugLog('error', `Draw mode POST failed: ${response.status} ${respBody}`)
    } else {
      deps.debugLog('draw', `Draw mode results delivered (${(message.annotations as unknown[])?.length || 0} annotations)`)
    }
  } catch (err) {
    deps.debugLog('error', `Draw mode completion error: ${(err as Error).message}. Server may be unreachable.`)
  }
}

function handleSetServerUrl(url: string, sendResponse: SendResponse, deps: MessageHandlerDependencies): void {
  deps.setServerUrl(url || 'http://localhost:7890')
  deps.saveSetting('serverUrl', deps.getServerUrl())
  deps.debugLog('settings', `Server URL changed to: ${deps.getServerUrl()}`)

  // Broadcast to all content scripts
  deps.forwardToAllContentScripts({ type: 'setServerUrl', url: deps.getServerUrl() })

  // Re-check connection with new URL
  deps.checkConnectionAndUpdate()

  sendResponse({ success: true })
}

// =============================================================================
// STATE SNAPSHOT STORAGE
// =============================================================================

const SNAPSHOT_KEY = 'gasoline_state_snapshots'

interface StoredStateSnapshot extends BrowserStateSnapshot {
  name: string
  size_bytes: number
}

interface StateSnapshotStorage {
  [name: string]: StoredStateSnapshot
}

/**
 * Save a state snapshot to chrome.storage.local
 */
export async function saveStateSnapshot(
  name: string,
  state: BrowserStateSnapshot
): Promise<{ success: boolean; snapshot_name: string; size_bytes: number }> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      const sizeBytes = JSON.stringify(state).length // nosemgrep: no-stringify-keys
      snapshots[name] = {
        ...state,
        name,
        size_bytes: sizeBytes
      }
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({
          success: true,
          snapshot_name: name,
          size_bytes: sizeBytes
        })
      })
    })
  })
}

/**
 * Load a state snapshot from chrome.storage.local
 */
export async function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      resolve(snapshots[name] || null)
    })
  })
}

/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots(): Promise<
  Array<{ name: string; url: string; timestamp: number; size_bytes: number }>
> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      const list = Object.values(snapshots).map((s) => ({
        name: s.name,
        url: s.url,
        timestamp: s.timestamp,
        size_bytes: s.size_bytes
      }))
      resolve(list)
    })
  })
}

/**
 * Delete a state snapshot from chrome.storage.local
 */
export async function deleteStateSnapshot(name: string): Promise<{ success: boolean; deleted: string }> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      delete snapshots[name]
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({ success: true, deleted: name })
      })
    })
  })
}
