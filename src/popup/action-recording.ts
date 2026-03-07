/**
 * Purpose: Popup UI module for action workflow (event) recording — start/stop via daemon HTTP API.
 * Why: Separates event recording controls from screen recording, keeping each feature self-contained.
 * Docs: docs/features/feature/flow-recording/index.md
 */

import { DEFAULT_SERVER_URL, StorageKey } from '../lib/constants.js'
import { postDaemonJSON } from '../lib/daemon-http.js'
import { getLocal, setLocal, removeLocal } from '../lib/storage-utils.js'

interface ActionRecordingElements {
  row: HTMLElement
  label: HTMLElement
  statusEl: HTMLElement
}

interface ActionRecordingState {
  isRecording: boolean
  recordingId: string | null
  timerInterval: ReturnType<typeof setInterval> | null
  startTime: number | null
}

interface ConfigureCallResponse {
  result?: { content?: Array<{ text?: string }> }
  error?: { message?: string }
}

const START_LABEL = 'Record action workflow'
const STOP_LABEL = 'Stop recording'

function showRecording(els: ActionRecordingElements, state: ActionRecordingState): void {
  state.isRecording = true
  els.row.classList.add('is-recording')
  els.label.textContent = STOP_LABEL
  els.statusEl.textContent = ''

  if (state.timerInterval) clearInterval(state.timerInterval)
  const start = state.startTime ?? Date.now()
  state.timerInterval = setInterval(() => {
    const elapsed = Math.round((Date.now() - start) / 1000)
    const mins = Math.floor(elapsed / 60)
    const secs = elapsed % 60
    els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`
  }, 1000)
}

function showIdle(els: ActionRecordingElements, state: ActionRecordingState): void {
  state.isRecording = false
  state.recordingId = null
  state.startTime = null
  els.row.classList.remove('is-recording')
  els.label.textContent = START_LABEL
  els.statusEl.textContent = ''
  if (state.timerInterval) {
    clearInterval(state.timerInterval)
    state.timerInterval = null
  }
}

function showError(els: ActionRecordingElements, message: string): void {
  els.statusEl.textContent = message
  els.statusEl.style.color = '#f85149'
  setTimeout(() => {
    els.statusEl.textContent = ''
    els.statusEl.style.color = ''
  }, 5000)
}

async function getServerUrl(): Promise<string> {
  const value = await getLocal(StorageKey.SERVER_URL)
  return (value as string) || DEFAULT_SERVER_URL
}

function getConfigureError(data: ConfigureCallResponse): string | null {
  const message = data.error?.message
  return typeof message === 'string' && message.length > 0 ? message : null
}

function extractRecordingID(data: ConfigureCallResponse): string | null {
  const text = data.result?.content?.[0]?.text ?? ''
  const idMatch = text.match(/"recording_id"\s*:\s*"([^"]+)"/)
  return idMatch?.[1] ?? null
}

async function callConfigureFromPopup(argumentsPayload: Record<string, unknown>): Promise<ConfigureCallResponse> {
  const serverUrl = await getServerUrl()
  const resp = await postDaemonJSON(`${serverUrl}/mcp`, {
    jsonrpc: '2.0',
    id: Date.now(),
    method: 'tools/call',
    params: {
      name: 'configure',
      arguments: argumentsPayload
    }
  })
  if (!resp.ok) {
    throw new Error(`Server error: HTTP ${resp.status}`)
  }
  return (await resp.json()) as ConfigureCallResponse
}

async function startActionRecording(els: ActionRecordingElements, state: ActionRecordingState): Promise<void> {
  els.label.textContent = 'Starting...'

  try {
    const data = await callConfigureFromPopup({
      what: 'event_recording_start',
      name: `workflow-${Date.now()}`
    })

    const configureError = getConfigureError(data)
    if (configureError) {
      showIdle(els, state)
      showError(els, configureError)
      return
    }

    state.recordingId = extractRecordingID(data)
    state.startTime = Date.now()

    // Persist state so reopening popup shows recording in progress
    void setLocal(StorageKey.ACTION_RECORDING, {
      active: true,
      recordingId: state.recordingId,
      startTime: state.startTime
    })

    showRecording(els, state)
  } catch (err) {
    showIdle(els, state)
    showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`)
  }
}

async function stopActionRecording(els: ActionRecordingElements, state: ActionRecordingState): Promise<void> {
  els.label.textContent = 'Stopping...'

  try {
    const data = await callConfigureFromPopup({
      what: 'event_recording_stop',
      recording_id: state.recordingId ?? ''
    })

    const configureError = getConfigureError(data)
    if (configureError) {
      showError(els, configureError)
    }

    void removeLocal(StorageKey.ACTION_RECORDING)

    showIdle(els, state)
  } catch (err) {
    showIdle(els, state)
    showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`)
  }
}

export function setupActionRecordingUI(): void {
  const row = document.getElementById('action-record-row')
  const label = document.getElementById('action-record-label')
  const statusEl = document.getElementById('action-recording-status')
  if (!row || !label || !statusEl) return

  const els: ActionRecordingElements = { row, label, statusEl }
  const state: ActionRecordingState = {
    isRecording: false,
    recordingId: null,
    timerInterval: null,
    startTime: null
  }

  // Restore state if popup was closed during recording
  void getLocal(StorageKey.ACTION_RECORDING).then((value: unknown) => {
    const saved = value as {
      active?: boolean
      recordingId?: string
      startTime?: number
    } | undefined
    if (saved?.active && saved.recordingId) {
      state.recordingId = saved.recordingId
      state.startTime = saved.startTime ?? Date.now()
      showRecording(els, state)
    }
  })

  row.addEventListener('click', () => {
    if (state.isRecording) {
      void stopActionRecording(els, state)
    } else {
      void startActionRecording(els, state)
    }
  })
}
