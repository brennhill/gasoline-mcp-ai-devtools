/**
 * Purpose: Acquires tab capture streams, manages offscreen documents, and handles user gesture flow for video recording.
 * Docs: docs/features/feature/flow-recording/index.md
 */

// recording-capture.ts — Tab capture stream acquisition, offscreen document management, and user gesture flow.
// Extracted from recording.ts to separate media plumbing from recording lifecycle.

import { scaleTimeout } from '../lib/timeouts.js'
import { StorageKey } from '../lib/constants.js'
import { sendTabToast } from './event-listeners.js'
import { errorMessage } from '../lib/error-utils.js'
import { delay } from '../lib/timeout-utils.js'

const LOG = '[Gasoline REC]'

/** Ensure the offscreen document exists for recording. */
export async function ensureOffscreenDocument(): Promise<void> {
  // Check if an offscreen document already exists
  const contexts = await chrome.runtime.getContexts({
    contextTypes: [chrome.runtime.ContextType.OFFSCREEN_DOCUMENT]
  })
  if (contexts.length > 0) return

  await chrome.offscreen.createDocument({
    url: 'offscreen.html',
    reasons: [chrome.offscreen.Reason.USER_MEDIA],
    justification: 'Tab video recording via MediaRecorder'
  })
}

/**
 * Get a media stream ID, recovering from "active stream" errors by closing the
 * stale offscreen document (which releases leaked streams) and retrying once.
 */
export async function getStreamIdWithRecovery(tabId: number): Promise<string> {
  try {
    return await getStreamId(tabId)
  } catch (err) {
    if (errorMessage(err)?.includes('active stream')) {
      console.warn(LOG, 'Active stream detected — closing offscreen document to release leaked streams')
      try {
        await chrome.offscreen.closeDocument()
      } catch {
        /* might not exist */
      }
      // Brief pause to let Chrome release the capture
      await delay(scaleTimeout(200))
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
 * Request user gesture for recording permission (used for MCP-initiated recordings).
 * Shows a toast prompting the user to open the Gasoline popup and approve.
 */
export async function requestRecordingGesture(
  tab: chrome.tabs.Tab,
  name: string,
  fps: number,
  audio: string,
  mediaType: string
): Promise<{ status: string; name: string; error?: string }> {
  chrome.tabs.update(tab.id!, { active: true })
  sendTabToast(
    tab.id!,
    `\u2191 Open Gasoline Popup`,
    `Approve ${mediaType.toLowerCase()} recording request`,
    'audio',
    scaleTimeout(30000)
  )

  await chrome.storage.local.set({ [StorageKey.PENDING_RECORDING]: { name, fps, audio, tabId: tab.id, url: tab.url } })
  const gestureResult = await waitForRecordingGesture(scaleTimeout(30000))
  await chrome.storage.local.remove(StorageKey.PENDING_RECORDING)

  if (gestureResult === 'denied') {
    console.log(LOG, 'GESTURE_DENIED: User denied recording request from popup')
    return {
      status: 'error',
      name: '',
      error: `RECORD_START: ${mediaType} recording request was denied in the Gasoline popup.`
    }
  }

  if (gestureResult !== 'granted') {
    console.log(LOG, 'GESTURE_TIMEOUT: User did not approve recording request within 30s')
    sendTabToast(
      tab.id!,
      `\u2191 Open Gasoline Popup`,
      `Approve ${mediaType.toLowerCase()} recording request`,
      'audio',
      scaleTimeout(8000)
    )
    return {
      status: 'error',
      name: '',
      error: `RECORD_START: ${mediaType} recording requires popup approval. Open the Gasoline popup, click Approve, then try again.`
    }
  }

  sendTabToast(tab.id!, 'Recording', 'Recording started', 'success', scaleTimeout(2000))

  return { status: 'ok', name }
}

/** Wait for popup approval decision (grant/deny) with timeout fallback. */
function waitForRecordingGesture(timeoutMs: number): Promise<'granted' | 'denied' | 'timeout'> {
  return new Promise((resolve) => {
    const timeout = setTimeout(() => {
      chrome.runtime.onMessage.removeListener(listener)
      resolve('timeout')
    }, timeoutMs)

    const listener = (message: { type?: string }) => {
      if (message.type === 'RECORDING_GESTURE_GRANTED') {
        clearTimeout(timeout)
        chrome.runtime.onMessage.removeListener(listener)
        resolve('granted')
        return
      }
      if (message.type === 'RECORDING_GESTURE_DENIED') {
        clearTimeout(timeout)
        chrome.runtime.onMessage.removeListener(listener)
        resolve('denied')
      }
    }
    chrome.runtime.onMessage.addListener(listener)
  })
}
