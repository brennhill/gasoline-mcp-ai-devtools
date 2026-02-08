// recording.ts — Tab video recording pipeline (offscreen architecture).
// Orchestrates recording via an offscreen document that handles MediaRecorder
// and uploads. The service worker manages tab capture stream IDs, watermarks,
// and popup sync.

import * as index from './index'
import { pingContentScript, waitForTabLoad } from './event-listeners'
import type {
  OffscreenRecordingStartedMessage,
  OffscreenRecordingStoppedMessage,
} from '../types/runtime-messages'

interface RecordingState {
  active: boolean
  name: string
  startTime: number
  fps: number
  audioMode: string
  tabId: number
  url: string
  queryId: string
}

const defaultState: RecordingState = {
  active: false,
  name: '',
  startTime: 0,
  fps: 15,
  audioMode: '',
  tabId: 0,
  url: '',
  queryId: '',
}

let recordingState: RecordingState = { ...defaultState }

const LOG = '[Gasoline REC]'

/** Listener to re-send watermark when recording tab navigates or content script re-injects. */
let tabUpdateListener: ((tabId: number, changeInfo: chrome.tabs.TabChangeInfo) => void) | null = null

// Clear stale recording state from previous session (e.g., browser crash during recording)
console.log(LOG, 'Module loaded, clearing stale gasoline_recording from storage')
chrome.storage.local.remove('gasoline_recording').catch(() => {})

/** Returns whether a recording is currently active. */
export function isRecording(): boolean {
  return recordingState.active
}

/** Returns current recording info for popup sync. */
export function getRecordingInfo(): { active: boolean; name: string; startTime: number } {
  return {
    active: recordingState.active,
    name: recordingState.name,
    startTime: recordingState.startTime,
  }
}

/** Ensure the offscreen document exists for recording. */
async function ensureOffscreenDocument(): Promise<void> {
  // Check if an offscreen document already exists
  const contexts = await chrome.runtime.getContexts({
    contextTypes: [chrome.runtime.ContextType.OFFSCREEN_DOCUMENT],
  })
  if (contexts.length > 0) return

  await chrome.offscreen.createDocument({
    url: 'offscreen.html',
    reasons: [chrome.offscreen.Reason.USER_MEDIA],
    justification: 'Tab video recording via MediaRecorder',
  })
}

/**
 * Get a media stream ID, recovering from "active stream" errors by closing the
 * stale offscreen document (which releases leaked streams) and retrying once.
 */
async function getStreamIdWithRecovery(tabId: number): Promise<string> {
  try {
    return await getStreamId(tabId)
  } catch (err) {
    if ((err as Error).message?.includes('active stream')) {
      console.warn(LOG, 'Active stream detected — closing offscreen document to release leaked streams')
      try {
        await chrome.offscreen.closeDocument()
      } catch { /* might not exist */ }
      // Brief pause to let Chrome release the capture
      await new Promise((r) => setTimeout(r, 200))
      console.log(LOG, 'Retrying getMediaStreamId after cleanup')
      return await getStreamId(tabId)
    }
    throw err
  }
}

/** Wrapper around chrome.tabCapture.getMediaStreamId with logging. */
function getStreamId(tabId: number): Promise<string> {
  return new Promise<string>((resolve, reject) => {
    chrome.tabCapture.getMediaStreamId({ targetTabId: tabId }, (id) => {
      if (chrome.runtime.lastError) {
        console.error(LOG, 'getMediaStreamId FAILED:', chrome.runtime.lastError.message)
        reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'))
      } else {
        console.log(LOG, 'Got stream ID:', id?.substring(0, 20) + '...')
        resolve(id)
      }
    })
  })
}

/**
 * Start recording the active tab.
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param queryId — PendingQuery ID for result resolution
 * @param audio — Audio mode: 'tab', 'mic', 'both', or '' (no audio)
 * @param fromPopup — true when initiated from popup (activeTab already granted, skip reload)
 */
export async function startRecording(
  name: string,
  fps: number = 15,
  queryId: string = '',
  audio: string = '',
  fromPopup: boolean = false,
): Promise<{ status: string; name: string; startTime?: number; error?: string }> {
  console.log(LOG, 'startRecording called', { name, fps, queryId, audio, fromPopup, currentlyActive: recordingState.active })

  if (recordingState.active) {
    console.warn(LOG, 'START BLOCKED: already recording', { currentState: { ...recordingState } })
    return { status: 'error', name: '', error: 'RECORD_START: Already recording. Stop current recording first.' }
  }

  // Mark active immediately to prevent TOCTOU race across awaits
  recordingState.active = true // eslint-disable-line require-atomic-updates
  console.log(LOG, 'Marked active=true (TOCTOU guard)')

  // Clamp fps
  fps = Math.max(5, Math.min(60, fps))

  try {
    // Get active tab
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    const tab = tabs[0]
    console.log(LOG, 'Active tab:', { id: tab?.id, url: tab?.url?.substring(0, 80), title: tab?.title?.substring(0, 40) })
    if (!tab?.id) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      console.error(LOG, 'START FAILED: No active tab found')
      return { status: 'error', name: '', error: 'RECORD_START: No active tab found.' }
    }

    // Auto-enable tab tracking if not already tracked
    const storage = await chrome.storage.local.get('trackedTabId')
    console.log(LOG, 'Tracked tab:', { trackedTabId: storage.trackedTabId, willAutoTrack: !storage.trackedTabId })
    if (!storage.trackedTabId) {
      await chrome.storage.local.set({ trackedTabId: tab.id, trackedTabUrl: tab.url ?? '', trackedTabTitle: tab.title ?? '' })
    }

    // Ensure content script is responsive (needed for toasts + watermark).
    // Skip when from popup — tab reload would close the popup.
    if (!fromPopup) {
      console.log(LOG, 'Pinging content script on tab', tab.id)
      const alive = await pingContentScript(tab.id)
      console.log(LOG, 'Content script alive:', alive)
      if (!alive) {
        console.log(LOG, 'Reloading tab for content script injection')
        chrome.tabs.reload(tab.id)
        await waitForTabLoad(tab.id, 5000)
      }
      // Add extra delay to ensure extension is fully initialized for tabCapture
      console.log(LOG, 'Waiting for extension to fully initialize...')
      await new Promise((r) => setTimeout(r, 1000))
    } else {
      console.log(LOG, 'Skipping content script ping (fromPopup=true)')
    }

    let streamId: string

    if (fromPopup) {
      // Popup click grants activeTab — get stream directly with targetTabId
      console.log(LOG, 'Getting stream ID via fromPopup path (targetTabId:', tab.id, ')')
      streamId = await getStreamIdWithRecovery(tab.id)
    } else if (audio) {
      // MCP-initiated audio: requires activeTab via user gesture.
      // Auto-activate the tracked tab to ensure content script is loaded and user sees the toast
      chrome.tabs.update(tab.id, { active: true })

      // Prompt the user to click the Gasoline extension icon with orange highlight.
      chrome.tabs.sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Audio Recording - Click Gasoline Icon',
        detail: 'Click the Gasoline icon (↗ upper right) to grant audio recording permission',
        state: 'audio' as const,
        duration_ms: 30000,
      }).catch(() => {})

      await chrome.storage.local.set({
        gasoline_pending_recording: { name, fps, audio, tabId: tab.id, url: tab.url },
      })

      const gestureGranted = await waitForRecordingGesture(30000)
      await chrome.storage.local.remove('gasoline_pending_recording')

      if (!gestureGranted) {
        console.log(LOG, 'GESTURE_TIMEOUT: User did not click the Gasoline icon within 30s')
        chrome.tabs.sendMessage(tab.id, {
          type: 'GASOLINE_ACTION_TOAST',
          text: 'Audio Permission Required',
          detail: 'Click the Gasoline icon (↗ upper right) to grant audio recording permission',
          state: 'audio' as const,
          duration_ms: 8000,
        }).catch(() => {})
        recordingState.active = false // eslint-disable-line require-atomic-updates
        return {
          status: 'error',
          name: '',
          error: 'RECORD_START: Audio recording requires permission. Click the Gasoline icon (↗ upper right) to grant audio recording permission, then try again.',
        }
      }

      // Dismiss the prompt toast with a success confirmation
      chrome.tabs.sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Recording',
        detail: 'Recording started',
        state: 'success' as const,
        duration_ms: 2000,
      }).catch(() => {})

      // activeTab granted — get stream ID with targetTabId
      streamId = await new Promise<string>((resolve, reject) => {
        chrome.tabCapture.getMediaStreamId({ targetTabId: tab.id! }, (id) => {
          if (chrome.runtime.lastError) {
            reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'))
          } else {
            resolve(id)
          }
        })
      })
    } else {
      // MCP video-only: requires activeTab via user gesture, same as audio recording
      // Auto-activate the tracked tab to ensure content script is loaded and user sees the toast
      chrome.tabs.update(tab.id, { active: true })

      // Prompt the user to click the Gasoline extension icon with orange highlight.
      chrome.tabs.sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Video Recording - Click Gasoline Icon',
        detail: 'Click the Gasoline icon (↗ upper right) to grant video recording permission',
        state: 'audio' as const,
        duration_ms: 30000,
      }).catch(() => {})

      await chrome.storage.local.set({
        gasoline_pending_recording: { name, fps, audio, tabId: tab.id, url: tab.url },
      })

      const gestureGranted = await waitForRecordingGesture(30000)
      await chrome.storage.local.remove('gasoline_pending_recording')

      if (!gestureGranted) {
        console.log(LOG, 'GESTURE_TIMEOUT: User did not click the Gasoline icon within 30s')
        chrome.tabs.sendMessage(tab.id, {
          type: 'GASOLINE_ACTION_TOAST',
          text: 'Video Permission Required',
          detail: 'Click the Gasoline icon (↗ upper right) to grant video recording permission',
          state: 'audio' as const,
          duration_ms: 8000,
        }).catch(() => {})
        recordingState.active = false // eslint-disable-line require-atomic-updates
        return {
          status: 'error',
          name: '',
          error: 'RECORD_START: Video recording requires permission. Click the Gasoline icon (↗ upper right) to grant video recording permission, then try again.',
        }
      }

      // Dismiss the prompt toast with a success confirmation
      chrome.tabs.sendMessage(tab.id, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Recording',
        detail: 'Recording started',
        state: 'success' as const,
        duration_ms: 2000,
      }).catch(() => {})

      // activeTab granted — get stream ID with targetTabId
      streamId = await new Promise<string>((resolve, reject) => {
        chrome.tabCapture.getMediaStreamId({ targetTabId: tab.id! }, (id) => {
          if (chrome.runtime.lastError) {
            reject(new Error(chrome.runtime.lastError.message ?? 'getMediaStreamId failed'))
          } else {
            resolve(id)
          }
        })
      })
    }

    if (!streamId) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      console.error(LOG, 'START FAILED: streamId is empty')
      return { status: 'error', name: '', error: 'RECORD_START: getMediaStreamId returned empty. Check tabCapture permission.' }
    }

    // Ensure the offscreen document is running
    console.log(LOG, 'Ensuring offscreen document exists')
    await ensureOffscreenDocument()
    console.log(LOG, 'Offscreen document ready, sending START command')

    // Send start command to offscreen document and wait for confirmation (10s timeout)
    const startResult = await new Promise<OffscreenRecordingStartedMessage>((resolve) => {
      const timeout = setTimeout(() => {
        chrome.runtime.onMessage.removeListener(listener)
        resolve({ target: 'background', type: 'OFFSCREEN_RECORDING_STARTED', success: false, error: 'RECORD_START: Offscreen document timed out.' })
      }, 10000)

      const listener = (message: OffscreenRecordingStartedMessage) => {
        if (message.target === 'background' && message.type === 'OFFSCREEN_RECORDING_STARTED') {
          clearTimeout(timeout)
          chrome.runtime.onMessage.removeListener(listener)
          resolve(message)
        }
      }
      chrome.runtime.onMessage.addListener(listener)

      chrome.runtime.sendMessage({
        target: 'offscreen',
        type: 'OFFSCREEN_START_RECORDING',
        streamId,
        serverUrl: index.serverUrl,
        name,
        fps,
        audioMode: audio,
        tabId: tab.id,
        url: tab.url ?? '',
      })
    })

    console.log(LOG, 'Offscreen START result:', { success: startResult.success, error: startResult.error })

    if (!startResult.success) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      console.error(LOG, 'START FAILED: offscreen rejected:', startResult.error)
      return { status: 'error', name: '', error: startResult.error ?? 'RECORD_START: Offscreen document failed to start recording.' }
    }

    /* eslint-disable require-atomic-updates */
    recordingState = {
      active: true,
      name,
      startTime: Date.now(),
      fps,
      audioMode: audio,
      tabId: tab.id,
      url: tab.url ?? '',
      queryId,
    }
    /* eslint-enable require-atomic-updates */

    // Persist state flag for popup sync
    await chrome.storage.local.set({
      gasoline_recording: { active: true, name, startTime: Date.now() },
    })

    // Show "Recording started" toast (fades after 2s)
    chrome.tabs.sendMessage(tab.id, {
      type: 'GASOLINE_ACTION_TOAST',
      text: 'Recording started',
      state: 'success' as const,
      duration_ms: 2000,
    }).catch(() => {})

    // Show recording watermark overlay in the page
    chrome.tabs.sendMessage(tab.id, { type: 'GASOLINE_RECORDING_WATERMARK', visible: true }).catch(() => {})

    // Re-send watermark when recording tab navigates or content script re-injects
    if (tabUpdateListener) chrome.tabs.onUpdated.removeListener(tabUpdateListener)
    tabUpdateListener = (updatedTabId: number, changeInfo: chrome.tabs.TabChangeInfo) => {
      if (updatedTabId === recordingState.tabId && changeInfo.status === 'complete' && recordingState.active) {
        chrome.tabs.sendMessage(updatedTabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: true }).catch(() => {})
      }
    }
    chrome.tabs.onUpdated.addListener(tabUpdateListener)

    console.log(LOG, 'Recording STARTED successfully', { name, tabId: tab.id, audioMode: audio, fps })
    return { status: 'recording', name, startTime: recordingState.startTime }
  } catch (err) {
    recordingState.active = false // eslint-disable-line require-atomic-updates
    console.error(LOG, 'START EXCEPTION:', (err as Error).message, (err as Error).stack)
    return {
      status: 'error',
      name: '',
      error: `RECORD_START: ${(err as Error).message || 'Failed to start recording.'}`,
    }
  }
}

/**
 * Stop recording and save the video.
 * @param truncated — true if auto-stopped due to memory guard or tab close
 */
export async function stopRecording(truncated: boolean = false): Promise<{
  status: string
  name: string
  duration_seconds?: number
  size_bytes?: number
  truncated?: boolean
  path?: string
  error?: string
}> {
  console.log(LOG, 'stopRecording called', { currentlyActive: recordingState.active, name: recordingState.name, tabId: recordingState.tabId, truncated })

  if (!recordingState.active) {
    // Clean up stale storage in case of zombie recording state (e.g., service worker restarted)
    console.warn(LOG, 'STOP: No active recording in memory — cleaning up zombie storage')
    chrome.storage.local.remove('gasoline_recording').catch(() => {})
    return { status: 'error', name: '', error: 'RECORD_STOP: No active recording.' }
  }

  const { tabId } = recordingState

  // Mark as no longer active immediately to prevent double-stop
  recordingState.active = false
  console.log(LOG, 'Marked active=false, sending STOP to offscreen')

  // Hide recording watermark overlay
  if (tabId) {
    chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: false }).catch(() => {})
  }

  try {
    // Send stop command to offscreen document and wait for result (30s timeout for upload)
    const stopResult = await new Promise<OffscreenRecordingStoppedMessage>((resolve) => {
      const timeout = setTimeout(() => {
        chrome.runtime.onMessage.removeListener(listener)
        resolve({ target: 'background', type: 'OFFSCREEN_RECORDING_STOPPED', status: 'error', name: recordingState.name || '', error: 'RECORD_STOP: Offscreen document timed out during save.' })
      }, 30000)

      const listener = (message: OffscreenRecordingStoppedMessage) => {
        if (message.target === 'background' && message.type === 'OFFSCREEN_RECORDING_STOPPED') {
          clearTimeout(timeout)
          chrome.runtime.onMessage.removeListener(listener)
          resolve(message)
        }
      }
      chrome.runtime.onMessage.addListener(listener)

      chrome.runtime.sendMessage({
        target: 'offscreen',
        type: 'OFFSCREEN_STOP_RECORDING',
      })
    })

    console.log(LOG, 'Offscreen STOP result:', { status: stopResult.status, name: stopResult.name, error: stopResult.error, size: stopResult.size_bytes, path: stopResult.path })

    await clearRecordingState()

    // Show save toast on the recorded tab
    if (tabId && stopResult.status === 'saved') {
      const sizeMB = stopResult.size_bytes ? (stopResult.size_bytes / (1024 * 1024)).toFixed(1) : '?'
      chrome.tabs.sendMessage(tabId, {
        type: 'GASOLINE_ACTION_TOAST',
        text: 'Recording saved',
        detail: `${stopResult.path ?? stopResult.name} (${sizeMB} MB)`,
        state: 'success' as const,
        duration_ms: 5000,
      }).catch(() => {})
    }

    return {
      status: stopResult.status,
      name: stopResult.name,
      duration_seconds: stopResult.duration_seconds,
      size_bytes: stopResult.size_bytes,
      truncated: stopResult.truncated,
      path: stopResult.path,
      error: stopResult.error,
    }
  } catch (err) {
    console.error(LOG, 'STOP EXCEPTION:', (err as Error).message, (err as Error).stack)
    await clearRecordingState()
    return {
      status: 'error',
      name: recordingState.name || '',
      error: `RECORD_STOP: ${(err as Error).message || 'Failed to stop recording.'}`,
    }
  }
}

/**
 * Listen for unsolicited messages from offscreen (auto-stop from memory guard or tab close).
 */
chrome.runtime.onMessage.addListener((message: OffscreenRecordingStoppedMessage, sender: chrome.runtime.MessageSender) => {
  // Only accept messages from the extension itself
  if (sender.id !== chrome.runtime.id) return
  if (message.target !== 'background' || message.type !== 'OFFSCREEN_RECORDING_STOPPED') return
  // Only handle if we think we're still recording (auto-stop case)
  if (!recordingState.active) return

  console.log(LOG, 'Auto-stop from offscreen (memory guard or tab close)', { status: message.status, name: message.name })
  recordingState.active = false
  if (recordingState.tabId) {
    chrome.tabs.sendMessage(recordingState.tabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: false }).catch(() => {})
  }
  clearRecordingState().catch(() => {})
})

/**
 * Handle popup-initiated record_start / record_stop messages.
 * These are direct chrome.runtime messages from the popup, not MCP pending queries.
 */
chrome.runtime.onMessage.addListener(
  (message: { type?: string; audio?: string }, sender: chrome.runtime.MessageSender, sendResponse: (response?: unknown) => void) => {
    // Only accept messages from the extension itself (popup)
    if (sender.id !== chrome.runtime.id) return false
    if (message.type === 'record_start') {
      console.log(LOG, 'Popup record_start received', { audio: message.audio })
      chrome.tabs.query({ active: true, currentWindow: true }).then((tabs) => {
        let slug = 'recording'
        try {
          const hostname = new URL(tabs[0]?.url ?? '').hostname.replace(/^www\./, '')
          slug = hostname.replace(/[^a-z0-9]/gi, '-').replace(/-+/g, '-').replace(/^-|-$/g, '') || 'recording'
        } catch { /* use default */ }
        const audio = message.audio ?? ''
        console.log(LOG, 'Popup record_start → startRecording', { slug, audio, tabUrl: tabs[0]?.url?.substring(0, 60) })
        startRecording(slug, 15, '', audio, true)
          .then((result) => {
            console.log(LOG, 'Popup record_start result:', result)
            sendResponse(result)
          })
          .catch((err) => {
            console.error(LOG, 'Popup record_start EXCEPTION:', err)
            sendResponse({ status: 'error' })
          })
      })
      return true // async response
    }
    if (message.type === 'record_stop') {
      console.log(LOG, 'Popup record_stop received')
      stopRecording()
        .then((result) => {
          console.log(LOG, 'Popup record_stop result:', result)
          sendResponse(result)
        })
        .catch((err) => {
          console.error(LOG, 'Popup record_stop EXCEPTION:', err)
          sendResponse({ status: 'error' })
        })
      return true // async response
    }
    return false
  },
)

/**
 * Handle MIC_GRANTED_CLOSE_TAB from the mic-permission page.
 * Closes the permission tab, activates the original tab, and shows a guidance toast.
 */
chrome.runtime.onMessage.addListener(
  (message: { type?: string }, sender: chrome.runtime.MessageSender) => {
    // Only accept messages from the extension itself
    if (sender.id !== chrome.runtime.id) return false
    if (message.type !== 'MIC_GRANTED_CLOSE_TAB') return false
    console.log(LOG, 'MIC_GRANTED_CLOSE_TAB received from tab', sender.tab?.id)

    // Read the stored return tab before closing the permission tab
    chrome.storage.local.get('gasoline_pending_mic_recording', (result: Record<string, { returnTabId?: number } | undefined>) => {
      const returnTabId = result.gasoline_pending_mic_recording?.returnTabId
      console.log(LOG, 'Pending mic recording intent:', result.gasoline_pending_mic_recording, 'returnTabId:', returnTabId)

      // Close the permission tab
      if (sender.tab?.id) {
        console.log(LOG, 'Closing permission tab', sender.tab.id)
        chrome.tabs.remove(sender.tab.id).catch(() => {})
      }

      // Activate the original tab and show guidance toast
      if (returnTabId) {
        console.log(LOG, 'Activating return tab', returnTabId)
        chrome.tabs.update(returnTabId, { active: true }).then(() => {
          console.log(LOG, 'Return tab activated, sending toast in 300ms')
          // Short delay to let the tab activation settle before sending message
          setTimeout(() => {
            console.log(LOG, 'Sending guidance toast to tab', returnTabId)
            chrome.tabs.sendMessage(returnTabId, {
              type: 'GASOLINE_ACTION_TOAST',
              text: 'Mic permission granted',
              detail: 'Open Gasoline and click Record',
              state: 'success' as const,
              duration_ms: 8000,
            }).catch((err) => {
              console.error(LOG, 'Toast send FAILED to tab', returnTabId, ':', (err as Error).message)
            })
          }, 300)
        }).catch((err) => {
          console.error(LOG, 'Tab activation FAILED for tab', returnTabId, ':', (err as Error).message)
        })
      } else {
        console.warn(LOG, 'No returnTabId found — cannot activate tab or show toast')
      }
    })

    return false
  },
)

/**
 * Handle REVEAL_FILE — opens the file in the OS file manager via the Go server.
 */
chrome.runtime.onMessage.addListener(
  (message: { type?: string; path?: string }, sender: chrome.runtime.MessageSender, sendResponse: (response?: unknown) => void) => {
    // Only accept messages from the extension itself
    if (sender.id !== chrome.runtime.id) return false
    if (message.type !== 'REVEAL_FILE' || !message.path) return false

    fetch(`${index.serverUrl}/recordings/reveal`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
      body: JSON.stringify({ path: message.path }),
    })
      .then((r) => r.json())
      .then((result) => sendResponse(result))
      .catch((err) => sendResponse({ error: (err as Error).message }))

    return true // async response
  },
)

/** Wait for user to click extension icon (popup sends RECORDING_GESTURE_GRANTED). */
function waitForRecordingGesture(timeoutMs: number): Promise<boolean> {
  return new Promise((resolve) => {
    const timeout = setTimeout(() => {
      chrome.runtime.onMessage.removeListener(listener)
      resolve(false)
    }, timeoutMs)

    const listener = (message: { type?: string }) => {
      if (message.type === 'RECORDING_GESTURE_GRANTED') {
        clearTimeout(timeout)
        chrome.runtime.onMessage.removeListener(listener)
        resolve(true)
      }
    }
    chrome.runtime.onMessage.addListener(listener)
  })
}

async function clearRecordingState(): Promise<void> {
  recordingState = { ...defaultState }
  if (tabUpdateListener) {
    chrome.tabs.onUpdated.removeListener(tabUpdateListener)
    tabUpdateListener = null
  }
  await chrome.storage.local.remove('gasoline_recording')
}
