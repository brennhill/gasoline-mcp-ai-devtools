// recording.ts — Tab video recording pipeline.
// Captures the active tab via chrome.tabCapture, holds chunks in memory,
// and POSTs the final blob to the Go server on stop.

import * as index from './index'

/** Maximum recording size in bytes before auto-stop (100MB). */
const MAX_RECORDING_BYTES = 100 * 1024 * 1024

interface RecordingState {
  active: boolean
  name: string
  startTime: number
  fps: number
  audioMode: string
  recorder: MediaRecorder | null
  stream: MediaStream | null
  chunks: Blob[]
  totalBytes: number
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
  recorder: null,
  stream: null,
  chunks: [],
  totalBytes: 0,
  tabId: 0,
  url: '',
  queryId: '',
}

let recordingState: RecordingState = { ...defaultState }

/** Listener to re-send watermark when recording tab navigates or content script re-injects. */
let tabUpdateListener: ((tabId: number, changeInfo: chrome.tabs.TabChangeInfo) => void) | null = null

// Clear stale recording state from previous session (e.g., browser crash during recording)
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

/**
 * Start recording the active tab.
 * @param name — Pre-generated filename from the Go server (e.g., "checkout-bug--2026-02-07-1423")
 * @param fps — Framerate (5–60, default 15)
 * @param queryId — PendingQuery ID for result resolution
 * @param audio — Audio mode: 'tab', 'mic', 'both', or '' (no audio)
 */
export async function startRecording(
  name: string,
  fps: number = 15,
  queryId: string = '',
  audio: string = '',
): Promise<{ status: string; name: string; error?: string }> {
  if (recordingState.active) {
    return { status: 'error', name: '', error: 'RECORD_START: Already recording. Stop current recording first.' }
  }

  // Mark active immediately to prevent TOCTOU race across awaits
  recordingState.active = true // eslint-disable-line require-atomic-updates

  // Clamp fps
  fps = Math.max(5, Math.min(60, fps))

  // Scale bitrate proportionally: 500kbps at 15fps baseline
  const bitrate = Math.round((fps / 15) * 500_000)

  // Tab audio capture (Phase 1: 'tab' and 'both' modes enable tab audio)
  const hasAudio = audio === 'tab' || audio === 'both'

  try {
    // Get active tab
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    const tab = tabs[0]
    if (!tab?.id) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      return { status: 'error', name: '', error: 'RECORD_START: No active tab found.' }
    }

    // Capture the tab — chrome.tabCapture.capture uses callback API
    const stream: MediaStream | null = await new Promise((resolve) => {
      chrome.tabCapture.capture(
        {
          video: true,
          videoConstraints: {
            mandatory: {
              minWidth: 1280,
              minHeight: 720,
              maxWidth: 1920,
              maxHeight: 1080,
              maxFrameRate: fps,
            },
          },
          audio: hasAudio,
        } as chrome.tabCapture.CaptureOptions,
        (s) => resolve(s ?? null),
      )
    })

    if (!stream) {
      recordingState.active = false // eslint-disable-line require-atomic-updates
      return { status: 'error', name: '', error: 'RECORD_START: Tab capture returned null stream. Check tabCapture permission.' }
    }

    const mimeType = hasAudio ? 'video/webm;codecs=vp8,opus' : 'video/webm;codecs=vp8'
    const recorder = new MediaRecorder(stream, {
      mimeType,
      videoBitsPerSecond: bitrate,
    })

    const chunks: Blob[] = []
    let totalBytes = 0
    let autoStopping = false

    recorder.ondataavailable = (e: BlobEvent) => {
      if (e.data.size > 0) {
        chunks.push(e.data)
        totalBytes += e.data.size
        recordingState.totalBytes = totalBytes

        // Memory guard: auto-stop if approaching limit
        if (totalBytes >= MAX_RECORDING_BYTES && !autoStopping) {
          autoStopping = true
          stopRecording(true).catch(() => {})
        }
      }
    }

    // Listen for stream ending (tab closed)
    const videoTrack = stream.getVideoTracks()[0]
    if (videoTrack) {
      videoTrack.onended = () => {
        if (recordingState.active && !autoStopping) {
          autoStopping = true
          stopRecording(true).catch(() => {})
        }
      }
    }

    recorder.start(1000) // Collect chunks every 1s

    /* eslint-disable require-atomic-updates */
    recordingState = {
      active: true,
      name,
      startTime: Date.now(),
      fps,
      audioMode: audio,
      recorder,
      stream,
      chunks,
      totalBytes: 0,
      tabId: tab.id,
      url: tab.url ?? '',
      queryId,
    }
    /* eslint-enable require-atomic-updates */

    // Persist state flag for popup sync
    await chrome.storage.local.set({
      gasoline_recording: { active: true, name, startTime: Date.now() },
    })

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

    return { status: 'recording', name }
  } catch (err) {
    recordingState.active = false // eslint-disable-line require-atomic-updates
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
  error?: string
}> {
  if (!recordingState.active) {
    return { status: 'error', name: '', error: 'RECORD_STOP: No active recording.' }
  }

  const { name, startTime, recorder, stream, chunks, url, tabId, fps, audioMode } = recordingState

  // Mark as no longer active immediately to prevent double-stop
  recordingState.active = false

  // Hide recording watermark overlay
  if (tabId) {
    chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_RECORDING_WATERMARK', visible: false }).catch(() => {})
  }

  return new Promise((resolve) => {
    if (!recorder || recorder.state === 'inactive') {
      // Already stopped (edge case) — still clean up stream tracks
      if (stream) {
        stream.getTracks().forEach((t) => t.stop())
      }
      clearRecordingState()
      resolve({ status: 'error', name: '', error: 'RECORD_STOP: Recorder already inactive.' })
      return
    }

    recorder.onstop = async () => {
      try {
        const blob = new Blob(chunks, { type: 'video/webm' })
        const duration = Math.round((Date.now() - startTime) / 1000)

        // Stop media stream tracks
        if (stream) {
          stream.getTracks().forEach((t) => t.stop())
        }

        // Build display name from the slug
        const displayName = name
          .replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '')
          .replace(/-/g, ' ')

        // POST to Go server
        const formData = new FormData()
        formData.append('video', blob, `${name}.webm`)
        formData.append(
          'metadata',
          JSON.stringify({
            name,
            display_name: displayName,
            created_at: new Date(startTime).toISOString(),
            duration_seconds: duration,
            size_bytes: blob.size,
            url,
            tab_id: tabId,
            resolution: '1920x1080',
            format: audioMode === 'tab' || audioMode === 'both' ? 'video/webm;codecs=vp8,opus' : 'video/webm;codecs=vp8',
            fps,
            has_audio: audioMode === 'tab' || audioMode === 'both',
            audio_mode: audioMode || undefined,
            truncated,
          }),
        )

        const response = await fetch(`${index.serverUrl}/recordings/save`, {
          method: 'POST',
          body: formData,
        })

        await clearRecordingState()

        if (!response.ok) {
          resolve({
            status: 'error',
            name,
            error: `RECORD_STOP: Server returned ${response.status}.`,
          })
          return
        }

        resolve({
          status: 'saved',
          name,
          duration_seconds: duration,
          size_bytes: blob.size,
          truncated: truncated || undefined,
        })
      } catch (err) {
        await clearRecordingState()
        resolve({
          status: 'error',
          name,
          error: `RECORD_STOP: ${(err as Error).message || 'Save failed.'}`,
        })
      }
    }

    recorder.stop()
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
