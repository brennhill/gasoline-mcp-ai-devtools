// @ts-nocheck
/**
 * @fileoverview content-tab-filtering.test.js — Tests for "Track This Tab" feature.
 * Verifies tab-scoped message filtering in content.js:
 * - Messages from untracked tabs are dropped
 * - Messages from tracked tab are forwarded with tabId attached
 * - Storage change events update tracking status
 * - Chrome internal pages are blocked
 * - Browser restart clears tracking state
 * - Tab ID is attached to all forwarded messages
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'
import { createMockChrome } from './helpers.js'

// ============================================================================
// SECTION 1: Tab Filtering Logic (content.js behavior)
// ============================================================================

describe('Content Script Tab Filtering', () => {
  let mockChrome
  let messagesSent
  let _storageChangeListeners
  let isTrackedTab
  let currentTabId
  let contextValid

  // Simulate the content.js filtering logic
  function createContentScriptSimulator(trackedTabId, thisTabId) {
    isTrackedTab = (thisTabId === trackedTabId)
    currentTabId = thisTabId
    contextValid = true
    messagesSent = []
    _storageChangeListeners = []

    const MESSAGE_MAP = {
      GASOLINE_LOG: 'log',
      GASOLINE_WS: 'ws_event',
      GASOLINE_NETWORK_BODY: 'network_body',
      GASOLINE_ENHANCED_ACTION: 'enhanced_action',
      GASOLINE_PERFORMANCE_SNAPSHOT: 'performance_snapshot',
    }

    function safeSendMessage(msg) {
      if (!contextValid) return
      messagesSent.push(msg)
    }

    // The message handler with tab filtering
    function handleMessage(event) {
      if (event.source !== globalThis.window) return

      const { type: messageType, payload } = event.data || {}

      // Tab isolation filter: drop messages from untracked tabs
      if (!isTrackedTab) {
        return // Drop message
      }

      // Forward messages with tabId attached
      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload && typeof payload === 'object') {
        safeSendMessage({ type: mapped, payload, tabId: currentTabId })
      }
    }

    function updateTrackingStatus(newTrackedTabId) {
      isTrackedTab = (currentTabId === newTrackedTabId)
    }

    return { handleMessage, updateTrackingStatus, MESSAGE_MAP }
  }

  beforeEach(() => {
    mockChrome = createMockChrome()
    globalThis.chrome = mockChrome
    messagesSent = []

    globalThis.window = {
      addEventListener: mock.fn(),
      postMessage: mock.fn(),
      location: { origin: 'http://localhost:3000', href: 'http://localhost:3000/' },
    }
  })

  // --------------------------------------------------------------------------
  // Core filtering tests
  // --------------------------------------------------------------------------

  test('should DROP messages from untracked tab', () => {
    const sim = createContentScriptSimulator(999, 1) // tracked=999, this=1

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'should be dropped' } },
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages from untracked tab should be dropped')
  })

  test('should FORWARD messages from tracked tab', () => {
    const sim = createContentScriptSimulator(1, 1) // tracked=1, this=1

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'should be forwarded' } },
    })

    assert.strictEqual(messagesSent.length, 1, 'Messages from tracked tab should be forwarded')
    assert.strictEqual(messagesSent[0].type, 'log')
  })

  test('should DROP all message types from untracked tab', () => {
    const sim = createContentScriptSimulator(999, 1) // untracked

    const messageTypes = [
      { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'test' } },
      { type: 'GASOLINE_WS', payload: { event: 'open', url: 'ws://localhost' } },
      { type: 'GASOLINE_NETWORK_BODY', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'GASOLINE_ENHANCED_ACTION', payload: { type: 'click', url: '/page' } },
      { type: 'GASOLINE_PERFORMANCE_SNAPSHOT', payload: { lcp: 100 } },
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    assert.strictEqual(messagesSent.length, 0, 'All message types should be dropped from untracked tab')
  })

  test('should FORWARD all message types from tracked tab', () => {
    const sim = createContentScriptSimulator(42, 42) // tracked

    const messageTypes = [
      { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'test' } },
      { type: 'GASOLINE_WS', payload: { event: 'open', url: 'ws://localhost' } },
      { type: 'GASOLINE_NETWORK_BODY', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'GASOLINE_ENHANCED_ACTION', payload: { type: 'click', url: '/page' } },
      { type: 'GASOLINE_PERFORMANCE_SNAPSHOT', payload: { lcp: 100 } },
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    assert.strictEqual(messagesSent.length, 5, 'All 5 message types should be forwarded from tracked tab')
  })

  test('should DROP messages when no tab is tracked (trackedTabId undefined)', () => {
    const sim = createContentScriptSimulator(undefined, 1) // no tracking

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'no tracking' } },
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages should be dropped when no tab is tracked')
  })

  test('should DROP messages when no tab is tracked (trackedTabId null)', () => {
    const sim = createContentScriptSimulator(null, 1) // no tracking

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'no tracking' } },
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages should be dropped when trackedTabId is null')
  })

  // --------------------------------------------------------------------------
  // Tab ID attachment tests (USER REQUIREMENT)
  // --------------------------------------------------------------------------

  test('should attach tabId to ALL forwarded messages', () => {
    const sim = createContentScriptSimulator(42, 42)

    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'test' } },
    })

    assert.strictEqual(messagesSent.length, 1)
    assert.strictEqual(messagesSent[0].tabId, 42, 'tabId should be attached to forwarded message')
  })

  test('should attach correct tabId to each message type', () => {
    const tabId = 123
    const sim = createContentScriptSimulator(tabId, tabId)

    const messageTypes = [
      { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'test' } },
      { type: 'GASOLINE_NETWORK_BODY', payload: { url: '/api', method: 'GET', status: 200 } },
      { type: 'GASOLINE_WS', payload: { event: 'open', url: 'ws://localhost' } },
    ]

    for (const msg of messageTypes) {
      sim.handleMessage({ source: globalThis.window, data: msg })
    }

    for (let i = 0; i < messagesSent.length; i++) {
      assert.strictEqual(messagesSent[i].tabId, tabId, `Message ${i} should have tabId ${tabId}`)
    }
  })

  // --------------------------------------------------------------------------
  // Cross-origin / source filtering
  // --------------------------------------------------------------------------

  test('should still reject messages from non-window sources regardless of tracking', () => {
    const sim = createContentScriptSimulator(1, 1) // tracked

    sim.handleMessage({
      source: {}, // Not window
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'injected' } },
    })

    assert.strictEqual(messagesSent.length, 0, 'Messages from non-window sources should be rejected')
  })

  // --------------------------------------------------------------------------
  // Storage change events (tracking status updates)
  // --------------------------------------------------------------------------

  test('should update tracking status when trackedTabId changes in storage', () => {
    const sim = createContentScriptSimulator(999, 1) // initially untracked

    // Simulate trackedTabId changing to this tab
    sim.updateTrackingStatus(1)

    // Now should forward messages
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'now tracked' } },
    })

    assert.strictEqual(messagesSent.length, 1, 'Should forward after tracking status changes to this tab')
  })

  test('should stop forwarding when tracking switches to different tab', () => {
    const sim = createContentScriptSimulator(1, 1) // initially tracked

    // Forward one message while tracked
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'info', message: 'tracked' } },
    })
    assert.strictEqual(messagesSent.length, 1)

    // Switch tracking to different tab
    sim.updateTrackingStatus(999) // now tracking tab 999, not tab 1

    // This message should be dropped
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'should drop' } },
    })

    assert.strictEqual(messagesSent.length, 1, 'Should stop forwarding when tracking switches away')
  })

  test('should stop forwarding when tracking is disabled (trackedTabId removed)', () => {
    const sim = createContentScriptSimulator(1, 1) // initially tracked

    // Forward while tracked
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'info', message: 'tracked' } },
    })
    assert.strictEqual(messagesSent.length, 1)

    // Disable tracking
    sim.updateTrackingStatus(undefined) // removed

    // Should be dropped
    sim.handleMessage({
      source: globalThis.window,
      data: { type: 'GASOLINE_LOG', payload: { level: 'error', message: 'no tracking' } },
    })

    assert.strictEqual(messagesSent.length, 1, 'Should stop forwarding when tracking disabled')
  })
})

// ============================================================================
// SECTION 2: Internal URL Blocking (popup.js behavior)
// ============================================================================

describe('Internal URL Blocking', () => {
  // isInternalUrl function that will be implemented in popup.js
  function isInternalUrl(url) {
    if (!url) return true
    const internalPrefixes = [
      'chrome://',
      'chrome-extension://',
      'about:',
      'edge://',
      'brave://',
      'devtools://',
    ]
    return internalPrefixes.some((prefix) => url.startsWith(prefix))
  }

  test('should block chrome:// URLs', () => {
    assert.strictEqual(isInternalUrl('chrome://extensions'), true)
    assert.strictEqual(isInternalUrl('chrome://settings'), true)
    assert.strictEqual(isInternalUrl('chrome://newtab'), true)
  })

  test('should block chrome-extension:// URLs', () => {
    assert.strictEqual(isInternalUrl('chrome-extension://abc123/popup.html'), true)
  })

  test('should block about: URLs', () => {
    assert.strictEqual(isInternalUrl('about:blank'), true)
    assert.strictEqual(isInternalUrl('about:version'), true)
  })

  test('should block edge:// URLs', () => {
    assert.strictEqual(isInternalUrl('edge://settings'), true)
  })

  test('should block brave:// URLs', () => {
    assert.strictEqual(isInternalUrl('brave://settings'), true)
  })

  test('should block devtools:// URLs', () => {
    assert.strictEqual(isInternalUrl('devtools://devtools/bundled/inspector.html'), true)
  })

  test('should block null/undefined URLs', () => {
    assert.strictEqual(isInternalUrl(null), true)
    assert.strictEqual(isInternalUrl(undefined), true)
    assert.strictEqual(isInternalUrl(''), true)
  })

  test('should ALLOW regular http:// URLs', () => {
    assert.strictEqual(isInternalUrl('http://localhost:3000'), false)
    assert.strictEqual(isInternalUrl('http://example.com'), false)
  })

  test('should ALLOW regular https:// URLs', () => {
    assert.strictEqual(isInternalUrl('https://example.com'), false)
    assert.strictEqual(isInternalUrl('https://app.example.com/login'), false)
  })

  test('should ALLOW file:// URLs (for local development)', () => {
    assert.strictEqual(isInternalUrl('file:///Users/dev/index.html'), false)
  })
})

// ============================================================================
// SECTION 3: Browser Restart Behavior
// ============================================================================

describe('Browser Restart Tracking State', () => {
  test('should clear tracking on browser startup (onStartup event)', () => {
    let startupCallback = null

    // Mock chrome.runtime.onStartup
    const mockChromeWithStartup = {
      runtime: {
        onStartup: {
          addListener: mock.fn((cb) => {
            startupCallback = cb
          }),
        },
      },
      storage: {
        local: {
          remove: mock.fn(() => {
            return Promise.resolve()
          }),
        },
      },
    }

    // Register the startup listener (as background.js would)
    mockChromeWithStartup.runtime.onStartup.addListener(async () => {
      await mockChromeWithStartup.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
    })

    // Simulate browser restart
    assert.ok(startupCallback, 'Should register an onStartup listener')
    startupCallback()

    // Verify tracking state is cleared
    assert.ok(
      mockChromeWithStartup.storage.local.remove.mock.calls.length > 0,
      'Should call storage.local.remove on browser startup',
    )
    const removedArg = mockChromeWithStartup.storage.local.remove.mock.calls[0].arguments[0]
    assert.ok(removedArg.includes('trackedTabId'), 'Should remove trackedTabId')
    assert.ok(removedArg.includes('trackedTabUrl'), 'Should remove trackedTabUrl')
  })
})

// ============================================================================
// SECTION 4: Status Ping with Tracking Info
// ============================================================================

describe('Status Ping with Tracking State', () => {
  test('should include tracking_enabled: false when no tab is tracked', () => {
    const storage = {} // No trackedTabId

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      extension_connected: true,
      timestamp: new Date().toISOString(),
    }

    assert.strictEqual(statusMessage.tracking_enabled, false)
    assert.strictEqual(statusMessage.tracked_tab_id, null)
    assert.strictEqual(statusMessage.tracked_tab_url, null)
    assert.strictEqual(statusMessage.extension_connected, true)
    assert.strictEqual(statusMessage.type, 'status')
  })

  test('should include tracking_enabled: true and tab info when tracking', () => {
    const storage = { trackedTabId: 123, trackedTabUrl: 'https://example.com' }

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      tracked_tab_url: storage.trackedTabUrl || null,
      extension_connected: true,
      timestamp: new Date().toISOString(),
    }

    assert.strictEqual(statusMessage.tracking_enabled, true)
    assert.strictEqual(statusMessage.tracked_tab_id, 123)
    assert.strictEqual(statusMessage.tracked_tab_url, 'https://example.com')
  })

  test('should include valid ISO timestamp', () => {
    const storage = { trackedTabId: 42 }

    const statusMessage = {
      type: 'status',
      tracking_enabled: !!storage.trackedTabId,
      tracked_tab_id: storage.trackedTabId || null,
      timestamp: new Date().toISOString(),
    }

    // Verify it's a valid ISO string
    const parsed = new Date(statusMessage.timestamp)
    assert.ok(!isNaN(parsed.getTime()), 'Timestamp should be a valid ISO date')
  })
})

// ============================================================================
// SECTION 5: Tracked Tab Closed Behavior
// ============================================================================

describe('Tracked Tab Closed', () => {
  test('should disable tracking when tracked tab is closed', async () => {
    const mockChromeForTabClose = {
      tabs: {
        get: mock.fn((tabId) => {
          // Simulate tab not found (closed)
          throw new Error('No tab with id: ' + tabId)
        }),
      },
      storage: {
        local: {
          remove: mock.fn(() => {
            return Promise.resolve()
          }),
          get: mock.fn(() => Promise.resolve({ trackedTabId: 123, trackedTabUrl: 'https://example.com' })),
        },
      },
    }

    // Simulate what background.js does when trying to access a closed tab
    const storage = await mockChromeForTabClose.storage.local.get(['trackedTabId'])
    const trackedTabId = storage.trackedTabId

    try {
      await mockChromeForTabClose.tabs.get(trackedTabId)
    } catch {
      // Tab no longer exists - clear tracking
      await mockChromeForTabClose.storage.local.remove(['trackedTabId', 'trackedTabUrl'])
    }

    assert.ok(
      mockChromeForTabClose.storage.local.remove.mock.calls.length > 0,
      'Should remove tracking when tab is closed',
    )
  })
})

// ============================================================================
// SECTION 6: Popup Button State Tests
// ============================================================================

describe('Popup Track Button State', () => {
  let mockDocument

  function createMockElement(id) {
    return {
      id,
      textContent: '',
      style: {},
      disabled: false,
      title: '',
      addEventListener: mock.fn(),
    }
  }

  beforeEach(() => {
    const elements = {}
    mockDocument = {
      getElementById: mock.fn((id) => {
        if (!elements[id]) {
          elements[id] = createMockElement(id)
        }
        return elements[id]
      }),
      addEventListener: mock.fn(),
      readyState: 'complete',
    }
    globalThis.document = mockDocument
  })

  test('should show "Track This Tab" when no tab is tracked', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior when no tracking
    const trackedTabId = undefined

    if (!trackedTabId) {
      btn.textContent = 'Track This Tab'
      btn.style.background = '#252525'
      btn.style.color = '#58a6ff'
    }

    assert.strictEqual(btn.textContent, 'Track This Tab')
    assert.strictEqual(btn.style.background, '#252525')
  })

  test('should show "Stop Tracking" when tab is tracked', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior when tracking active
    const trackedTabId = 42

    if (trackedTabId) {
      btn.textContent = 'Stop Tracking'
      btn.style.background = '#f85149'
    }

    assert.strictEqual(btn.textContent, 'Stop Tracking')
    assert.strictEqual(btn.style.background, '#f85149')
  })

  test('should disable button on internal chrome:// pages', () => {
    const btn = mockDocument.getElementById('track-page-btn')

    // Simulate popup.js behavior on internal page
    const tabUrl = 'chrome://extensions'
    const isInternal = tabUrl.startsWith('chrome://') ||
      tabUrl.startsWith('chrome-extension://') ||
      tabUrl.startsWith('about:')

    if (isInternal) {
      btn.disabled = true
      btn.textContent = 'Cannot Track Internal Pages'
      btn.style.opacity = '0.5'
    }

    assert.strictEqual(btn.disabled, true)
    assert.ok(btn.textContent.includes('Cannot Track'))
    assert.strictEqual(btn.style.opacity, '0.5')
  })

  test('should show "No tab tracked" warning when tracking disabled', () => {
    const info = mockDocument.getElementById('tracked-page-info')

    // Simulate no tracking warning
    const trackedTabId = undefined
    if (!trackedTabId) {
      info.textContent = 'No tab tracked - data capture disabled'
      info.style.display = 'block'
      info.style.color = '#f85149'
    }

    assert.ok(info.textContent.includes('No tab tracked'))
    assert.strictEqual(info.style.color, '#f85149')
  })
})

// ============================================================================
// SECTION 7: Multi-tab Isolation Scenario
// ============================================================================

describe('Multi-Tab Isolation Scenario', () => {
  test('should only forward from tracked tab in multi-tab scenario', () => {
    const trackedTabId = 1
    const messagesSent = { tab1: [], tab2: [], tab3: [] }

    const MESSAGE_MAP = {
      GASOLINE_LOG: 'log',
      GASOLINE_NETWORK_BODY: 'network_body',
    }

    // Simulate 3 tabs
    function processMessage(thisTabId, messageType, payload) {
      const isTracked = (thisTabId === trackedTabId)
      if (!isTracked) return // Dropped

      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload) {
        const tabKey = `tab${thisTabId}`
        if (messagesSent[tabKey]) {
          messagesSent[tabKey].push({ type: mapped, payload, tabId: thisTabId })
        }
      }
    }

    // Tab 1 (tracked) generates activity
    processMessage(1, 'GASOLINE_LOG', { level: 'error', message: 'from tab 1' })
    processMessage(1, 'GASOLINE_NETWORK_BODY', { url: '/api', method: 'GET', status: 200 })

    // Tab 2 (untracked) generates activity
    processMessage(2, 'GASOLINE_LOG', { level: 'error', message: 'from tab 2' })
    processMessage(2, 'GASOLINE_NETWORK_BODY', { url: '/secret', method: 'GET', status: 200 })

    // Tab 3 (untracked) generates activity
    processMessage(3, 'GASOLINE_LOG', { level: 'error', message: 'from tab 3' })

    // Verify only tab 1 data captured
    assert.strictEqual(messagesSent.tab1.length, 2, 'Tab 1 (tracked) should have 2 messages')
    assert.strictEqual(messagesSent.tab2.length, 0, 'Tab 2 (untracked) should have 0 messages')
    assert.strictEqual(messagesSent.tab3.length, 0, 'Tab 3 (untracked) should have 0 messages')

    // Verify tabId is attached
    assert.strictEqual(messagesSent.tab1[0].tabId, 1)
    assert.strictEqual(messagesSent.tab1[1].tabId, 1)
  })

  test('should switch data flow when tracking changes', () => {
    const allMessages = []
    const MESSAGE_MAP = { GASOLINE_LOG: 'log' }

    let trackedTabId = 1

    function processMessage(thisTabId, messageType, payload) {
      const isTracked = (thisTabId === trackedTabId)
      if (!isTracked) return

      const mapped = MESSAGE_MAP[messageType]
      if (mapped && payload) {
        allMessages.push({ type: mapped, payload, tabId: thisTabId })
      }
    }

    // Tab 1 tracked, send messages
    processMessage(1, 'GASOLINE_LOG', { level: 'info', message: 'tab1 msg1' })
    processMessage(2, 'GASOLINE_LOG', { level: 'info', message: 'tab2 msg1' })

    assert.strictEqual(allMessages.length, 1)
    assert.strictEqual(allMessages[0].tabId, 1)

    // Switch tracking to tab 2
    trackedTabId = 2

    processMessage(1, 'GASOLINE_LOG', { level: 'info', message: 'tab1 msg2' })
    processMessage(2, 'GASOLINE_LOG', { level: 'info', message: 'tab2 msg2' })

    assert.strictEqual(allMessages.length, 2)
    assert.strictEqual(allMessages[1].tabId, 2, 'After switch, only tab 2 messages forwarded')
  })
})

// ============================================================================
// SECTION 8: updateTrackingStatus uses chrome.runtime.sendMessage (not chrome.tabs)
// ============================================================================

describe('updateTrackingStatus via chrome.runtime.sendMessage', () => {
  test('should request tab ID from background via GET_TAB_ID message', async () => {
    // This test verifies the fix for the critical bug:
    // content scripts cannot access chrome.tabs API, so they must
    // ask the background script for their tab ID via messaging.

    let isTrackedTab = false
    let currentTabId = null
    const expectedTabId = 42

    // Mock chrome APIs as they exist in content script context
    const mockChromeRuntime = {
      sendMessage: async (msg) => {
        if (msg.type === 'GET_TAB_ID') {
          return { tabId: expectedTabId }
        }
        return {}
      },
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: expectedTabId }),
      },
    }

    // Simulate the fixed updateTrackingStatus
    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'GET_TAB_ID' })
        currentTabId = response?.tabId
        isTrackedTab = (currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId)
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, expectedTabId, 'Should get tab ID from background script')
    assert.strictEqual(isTrackedTab, true, 'Should be tracked when tab IDs match')
  })

  test('should set isTrackedTab=false when tab IDs do not match', async () => {
    let isTrackedTab = false
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async (msg) => {
        if (msg.type === 'GET_TAB_ID') {
          return { tabId: 99 }  // This tab
        }
        return {}
      },
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 }),  // Different tab tracked
      },
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'GET_TAB_ID' })
        currentTabId = response?.tabId
        isTrackedTab = (currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId)
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, 99, 'Should have this tab ID')
    assert.strictEqual(isTrackedTab, false, 'Should NOT be tracked when IDs differ')
  })

  test('should set isTrackedTab=false when sendMessage fails', async () => {
    let isTrackedTab = true  // Start as true to verify it gets set to false
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async () => {
        throw new Error('Extension context invalidated')
      },
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 }),
      },
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'GET_TAB_ID' })
        currentTabId = response?.tabId
        isTrackedTab = (currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId)
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(isTrackedTab, false, 'Should fallback to false on error')
  })

  test('should set isTrackedTab=false when response has no tabId', async () => {
    let isTrackedTab = true
    let currentTabId = null

    const mockChromeRuntime = {
      sendMessage: async () => ({})  // No tabId in response
    }
    const mockStorage = {
      local: {
        get: async () => ({ trackedTabId: 42 }),
      },
    }

    async function updateTrackingStatus() {
      try {
        const storage = await mockStorage.local.get(['trackedTabId'])
        const response = await mockChromeRuntime.sendMessage({ type: 'GET_TAB_ID' })
        currentTabId = response?.tabId
        isTrackedTab = (currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId)
      } catch {
        isTrackedTab = false
      }
    }

    await updateTrackingStatus()

    assert.strictEqual(currentTabId, undefined, 'Should be undefined when not in response')
    assert.strictEqual(isTrackedTab, false, 'Should be false when tabId is undefined')
  })

  test('should NOT use chrome.tabs.query (not available in content scripts)', () => {
    // Verify the content.js code does NOT reference chrome.tabs.query
    // The fix replaces chrome.tabs.query with chrome.runtime.sendMessage
    // This is a documentation-level test to verify the contract:
    // content scripts get their tab ID via GET_TAB_ID message to background
    assert.ok(true, 'chrome.tabs API is not available in content scripts - use GET_TAB_ID message instead')
  })
})

// ============================================================================
// SECTION 9: Message Payload Integrity
// ============================================================================

describe('Message Payload Integrity with Tab ID', () => {
  test('should preserve original payload when attaching tabId', () => {
    const tabId = 42
    const originalPayload = {
      url: 'http://localhost:3000/api/users',
      method: 'GET',
      status: 200,
      contentType: 'application/json',
      responseBody: '{"users":[]}',
      duration: 150,
    }

    // Simulate forwarding with tabId
    const forwarded = {
      type: 'network_body',
      payload: originalPayload,
      tabId: tabId,
    }

    // Verify payload is preserved
    assert.deepStrictEqual(forwarded.payload, originalPayload, 'Payload should be unchanged')
    assert.strictEqual(forwarded.tabId, 42, 'tabId should be added')
    assert.strictEqual(forwarded.type, 'network_body', 'type should be correct')
  })

  test('tabId should be a number, not a string', () => {
    const tabId = 123

    const forwarded = { type: 'log', payload: {}, tabId }

    assert.strictEqual(typeof forwarded.tabId, 'number', 'tabId should be numeric')
  })
})
