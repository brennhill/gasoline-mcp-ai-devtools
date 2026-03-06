/**
 * Purpose: Implements popup recording controls, mic-permission flow, and saved-recording reveal behavior.
 * Why: Gives users reliable start/stop control with explicit permission/error handling for tab capture sessions.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */

/**
 * @fileoverview Recording UI Module for Popup
 * Manages recording controls, timer display, and mic permission flow.
 */

import { StorageKey } from '../lib/constants.js'
import { errorMessage } from '../lib/error-utils.js'
import { getLocalValue, setLocalValue, removeLocalValue } from '../lib/storage-utils.js'

interface RecordingElements {
  row: HTMLElement
  label: HTMLElement
  statusEl: HTMLElement
  optionsEl: HTMLElement | null
  saveInfoEl: HTMLElement | null
  topNoticeEl: HTMLElement | null
}

interface RecordingState {
  isRecording: boolean
  timerInterval: ReturnType<typeof setInterval> | null
}

interface PendingRecordingIntent {
  highlight?: boolean
  name?: string
  fps?: number
  audio?: string
  tabId?: number
  url?: string
}

interface ApprovalElements {
  card: HTMLElement | null
  detail: HTMLElement | null
  approveBtn: HTMLButtonElement | null
  denyBtn: HTMLButtonElement | null
}

const START_LABEL = 'Record screen'
const STOP_LABEL = 'Stop recording'
const HIGHLIGHT_LABEL = '\u25CF \u00AB Click here to record'
const RECENT_RECORDING_START_MS = 8000
const TOP_NOTICE_DURATION_MS = 4000
const AUDIO_LABELS: Record<string, string> = {
  '': 'Video only',
  tab: 'Video + tab audio',
  mic: 'Video + microphone',
  both: 'Video + tab + mic'
}

let topNoticeTimer: ReturnType<typeof setTimeout> | null = null

function getRecordSection(els: RecordingElements): Element | null {
  const closest = (els.row as { closest?: unknown }).closest
  if (typeof closest !== 'function') return null
  return closest.call(els.row, '.section') as Element | null
}

function applyRecordHighlight(els: RecordingElements): void {
  const section = getRecordSection(els)
  if (section) section.classList.add('record-highlight')
  els.label.textContent = HIGHLIGHT_LABEL
}

function removeRecordHighlight(els: RecordingElements): void {
  const section = getRecordSection(els)
  if (section) section.classList.remove('record-highlight')
  if (els.label.textContent === HIGHLIGHT_LABEL) {
    els.label.textContent = START_LABEL
  }
}

// #lizard forgives
function showRecording(els: RecordingElements, state: RecordingState, name: string, startTime: number): void {
  const wasRecording = state.isRecording
  removeRecordHighlight(els)
  state.isRecording = true
  els.row.classList.add('is-recording')
  els.label.textContent = STOP_LABEL
  els.statusEl.textContent = ''
  if (els.optionsEl) els.optionsEl.style.display = 'none'

  if (state.timerInterval) clearInterval(state.timerInterval)
  state.timerInterval = setInterval(() => {
    const elapsed = Math.round((Date.now() - startTime) / 1000)
    const mins = Math.floor(elapsed / 60)
    const secs = elapsed % 60
    els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, '0')}`
  }, 1000)

  if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
    showTopNotice(els, 'Recording started')
  }
}

function showIdle(els: RecordingElements, state: RecordingState): void {
  state.isRecording = false
  removeRecordHighlight(els)
  els.row.classList.remove('is-recording')
  els.label.textContent = START_LABEL
  els.statusEl.textContent = ''
  if (els.optionsEl) els.optionsEl.style.display = 'block'
  if (state.timerInterval) {
    clearInterval(state.timerInterval)
    state.timerInterval = null
  }
}

function describePendingRecording(pending: PendingRecordingIntent): string {
  const parts: string[] = []
  if (pending.name) parts.push(`Name: ${pending.name}`)
  if (typeof pending.fps === 'number') parts.push(`FPS: ${pending.fps}`)
  const audioLabel = AUDIO_LABELS[pending.audio ?? ''] ?? AUDIO_LABELS['']
  parts.push(`Mode: ${audioLabel}`)
  return parts.join(' \u00b7 ')
}

function setApprovalPendingState(
  els: RecordingElements,
  approvalEls: ApprovalElements,
  state: RecordingState,
  pending: PendingRecordingIntent | null
): void {
  const setRowAriaDisabled = (value: string | null): void => {
    const setAttr = (els.row as { setAttribute?: unknown }).setAttribute
    const removeAttr = (els.row as { removeAttribute?: unknown }).removeAttribute
    if (value !== null) {
      if (typeof setAttr === 'function') setAttr.call(els.row, 'aria-disabled', value)
      return
    }
    if (typeof removeAttr === 'function') removeAttr.call(els.row, 'aria-disabled')
  }

  const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording)
  if (approvalPending) {
    if (approvalEls.detail && pending) approvalEls.detail.textContent = describePendingRecording(pending)
    if (approvalEls.card) approvalEls.card.style.display = 'block'
    els.row.classList.add('is-disabled')
    setRowAriaDisabled('true')
    if (els.optionsEl) els.optionsEl.style.display = 'none'
    return
  }
  if (approvalEls.detail) approvalEls.detail.textContent = ''
  if (approvalEls.card) approvalEls.card.style.display = 'none'
  els.row.classList.remove('is-disabled')
  setRowAriaDisabled(null)
  if (!state.isRecording && els.optionsEl) els.optionsEl.style.display = 'block'
}

function sendRecordingGestureDecision(type: 'RECORDING_GESTURE_GRANTED' | 'RECORDING_GESTURE_DENIED'): void {
  chrome.runtime.sendMessage({ type }, () => {
    void chrome.runtime.lastError
  })
}

function showTopNotice(els: RecordingElements, text: string): void {
  const notice = els.topNoticeEl
  if (!notice) return
  notice.textContent = text
  notice.style.display = 'block'
  if (topNoticeTimer) clearTimeout(topNoticeTimer)
  topNoticeTimer = setTimeout(() => {
    notice.style.display = 'none'
  }, TOP_NOTICE_DURATION_MS)
}

function showSavedLink(saveInfoEl: HTMLElement, displayName: string, filePath: string): void {
  saveInfoEl.textContent = 'Saved: '
  const link = document.createElement('a')
  link.href = '#'
  link.id = 'reveal-recording'
  link.textContent = displayName
  link.style.color = '#58a6ff'
  link.style.textDecoration = 'underline'
  link.style.cursor = 'pointer'
  saveInfoEl.appendChild(link)
  const linkEl = document.getElementById('reveal-recording')
  if (linkEl) {
    linkEl.addEventListener('click', (e) => {
      e.preventDefault()
      chrome.runtime.sendMessage({ type: 'REVEAL_FILE', path: filePath }, (result: { error?: string } | undefined) => {
        if (result?.error) {
          saveInfoEl.textContent = `Could not open folder: ${result.error}`
          saveInfoEl.style.color = '#f85149'
          setTimeout(() => {
            saveInfoEl.style.display = 'none'
          }, 5000)
        }
      })
    })
  }
}

function showSaveResult(
  saveInfoEl: HTMLElement | null,
  resp: { status?: string; name?: string; path?: string; error?: string } | undefined
): void {
  if (resp?.status !== 'saved' || !resp.name || !saveInfoEl) return
  const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, '')
  if (resp.path) {
    showSavedLink(saveInfoEl, displayName, resp.path)
  } else {
    saveInfoEl.textContent = `Saved: ${displayName}`
  }
  saveInfoEl.style.display = 'block'
  setTimeout(() => {
    saveInfoEl.style.display = 'none'
  }, 12000)
}

function showStartError(saveInfoEl: HTMLElement | null, errorText: string): void {
  if (!saveInfoEl) return
  saveInfoEl.textContent = errorText
  saveInfoEl.style.display = 'block'
  saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)'
  saveInfoEl.style.color = '#f85149'
  setTimeout(() => {
    saveInfoEl.style.display = 'none'
    saveInfoEl.style.background = 'rgba(63, 185, 80, 0.1)'
    saveInfoEl.style.color = '#3fb950'
  }, 5000)
}

function showMicPermissionPrompt(saveInfoEl: HTMLElement, audioMode: string): void {
  chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
    setLocalValue(StorageKey.PENDING_MIC_RECORDING, { audioMode, returnTabId: activeTabs[0]?.id })
  })
  saveInfoEl.innerHTML =
    'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>'
  saveInfoEl.style.display = 'block'
  saveInfoEl.style.background = 'rgba(248, 81, 73, 0.1)'
  saveInfoEl.style.color = '#f85149'
  const link = document.getElementById('grant-mic-link')
  if (link) {
    link.addEventListener('click', (e) => {
      e.preventDefault()
      chrome.tabs.create({ url: chrome.runtime.getURL('mic-permission.html') })
    })
  }
}

function sendRecordStart(els: RecordingElements, state: RecordingState, audioMode: string): void {
  console.log('[Gasoline REC] Popup: sendStart() called, sending screen_recording_start with audio:', audioMode)
  chrome.runtime.sendMessage(
    { type: 'screen_recording_start', audio: audioMode },
    (resp: { status?: string; name?: string; startTime?: number; error?: string } | undefined) => {
      console.log('[Gasoline REC] Popup: screen_recording_start response:', resp)
      if (chrome.runtime.lastError) {
        console.error('[Gasoline REC] Popup: screen_recording_start lastError:', chrome.runtime.lastError.message)
      }
      if (resp?.status === 'recording' && resp.name) {
        showRecording(els, state, resp.name, resp.startTime ?? Date.now())
      } else {
        showIdle(els, state)
        if (resp?.error) showStartError(els.saveInfoEl, resp.error)
      }
    }
  )
}

// #lizard forgives
function tryMicPermissionThenStart(els: RecordingElements, state: RecordingState, audioMode: string): void {
  console.log('[Gasoline REC] Popup: trying getUserMedia from popup...')
  navigator.mediaDevices
    .getUserMedia({ audio: true })
    .then((micStream) => {
      console.log('[Gasoline REC] Popup: getUserMedia succeeded from popup')
      micStream.getTracks().forEach((t) => t.stop())
      setLocalValue(StorageKey.MIC_GRANTED, true)
      sendRecordStart(els, state, audioMode)
    })
    .catch((err) => {
      console.log('[Gasoline REC] Popup: getUserMedia FAILED:', (err as Error).name, errorMessage(err))
      removeLocalValue(StorageKey.MIC_GRANTED)
      showIdle(els, state)
      if (els.saveInfoEl) showMicPermissionPrompt(els.saveInfoEl, audioMode)
    })
}

function handleStartClick(els: RecordingElements, state: RecordingState): void {
  const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
  const audioMode = audioSelect?.value ?? ''
  setLocalValue(StorageKey.RECORD_AUDIO_PREF, audioMode)
  if (els.optionsEl) els.optionsEl.style.display = 'none'
  if (els.saveInfoEl) els.saveInfoEl.style.display = 'none'
  els.label.textContent = 'Starting...'

  if (audioMode === 'mic' || audioMode === 'both') {
    console.log('[Gasoline REC] Popup: mic/both mode — checking gasoline_mic_granted')
    tryMicPermissionThenStart(els, state, audioMode)
  } else {
    sendRecordStart(els, state, audioMode)
  }
}

function handleStopClick(els: RecordingElements, state: RecordingState): void {
  els.row.classList.remove('is-recording')
  els.label.textContent = 'Saving...'
  console.log('[Gasoline REC] Popup: sending screen_recording_stop')
  chrome.runtime.sendMessage(
    { type: 'screen_recording_stop' },
    (resp: { status?: string; name?: string; path?: string; error?: string } | undefined) => {
      console.log('[Gasoline REC] Popup: screen_recording_stop response:', resp)
      if (chrome.runtime.lastError) {
        console.error('[Gasoline REC] Popup: screen_recording_stop lastError:', chrome.runtime.lastError.message)
      }
      showIdle(els, state)
      showSaveResult(els.saveInfoEl, resp)
    }
  )
}

export function setupRecordingUI(): void {
  const row = document.getElementById('record-row')
  const label = document.getElementById('record-label')
  const statusEl = document.getElementById('recording-status')
  if (!row || !label || !statusEl) return

  const els: RecordingElements = {
    row,
    label,
    statusEl,
    optionsEl: document.getElementById('record-options'),
    saveInfoEl: document.getElementById('record-save-info'),
    topNoticeEl: document.getElementById('record-top-notice')
  }
  const approvalEls: ApprovalElements = {
    card: document.getElementById('record-approval-card'),
    detail: document.getElementById('record-approval-detail'),
    approveBtn: document.getElementById('record-approve-btn') as HTMLButtonElement | null,
    denyBtn: document.getElementById('record-deny-btn') as HTMLButtonElement | null
  }

  const state: RecordingState = { isRecording: false, timerInterval: null }
  let pendingRecordingIntent: PendingRecordingIntent | null = null

  const updatePendingRecording = (pendingValue: unknown): void => {
    const pending = pendingValue as PendingRecordingIntent | undefined
    if (pending?.highlight && !state.isRecording) {
      applyRecordHighlight(els)
      pendingRecordingIntent = null
      setApprovalPendingState(els, approvalEls, state, null)
      removeLocalValue(StorageKey.PENDING_RECORDING)
      return
    }
    pendingRecordingIntent = pending && !pending.highlight ? pending : null
    if (!pendingRecordingIntent && !state.isRecording) removeRecordHighlight(els)
    setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent)
  }

  const clearPendingRecordingIntent = (): void => {
    pendingRecordingIntent = null
    setApprovalPendingState(els, approvalEls, state, null)
    removeLocalValue(StorageKey.PENDING_RECORDING)
  }

  row.style.visibility = 'hidden'

  getLocalValue(StorageKey.RECORDING, (value: unknown) => {
    const rec = value as { active?: boolean; name?: string; startTime?: number } | undefined
    console.log('[Gasoline REC] Popup: gasoline_recording from storage:', rec)
    if (rec?.active && rec.name && rec.startTime) {
      console.log('[Gasoline REC] Popup: resuming recording UI for', rec.name)
      showRecording(els, state, rec.name, rec.startTime)
    }
    row.style.visibility = 'visible'

    // Check for highlight request from hover launcher
    getLocalValue(StorageKey.PENDING_RECORDING, (pendingValue: unknown) => {
      updatePendingRecording(pendingValue)
    })
  })

  chrome.storage.onChanged.addListener((changes, areaName) => {
    if (areaName === 'local' && changes[StorageKey.RECORDING]) {
      const rec = changes[StorageKey.RECORDING]!.newValue as
        | { active?: boolean; name?: string; startTime?: number }
        | undefined
      console.log('[Gasoline REC] Popup: gasoline_recording changed:', rec)
      if (rec?.active && rec.name && rec.startTime) {
        showRecording(els, state, rec.name, rec.startTime)
      } else {
        showIdle(els, state)
      }
      setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent)
      return
    }
    if (areaName === 'local' && changes[StorageKey.PENDING_RECORDING]) {
      updatePendingRecording(changes[StorageKey.PENDING_RECORDING]!.newValue)
    }
  })

  approvalEls.approveBtn?.addEventListener('click', (event) => {
    event.preventDefault()
    sendRecordingGestureDecision('RECORDING_GESTURE_GRANTED')
    clearPendingRecordingIntent()
  })

  approvalEls.denyBtn?.addEventListener('click', (event) => {
    event.preventDefault()
    sendRecordingGestureDecision('RECORDING_GESTURE_DENIED')
    clearPendingRecordingIntent()
  })

  getLocalValue(StorageKey.PENDING_MIC_RECORDING, (value: unknown) => {
    const intent = value as { audioMode?: string } | undefined
    console.log('[Gasoline REC] Popup: pending_mic_recording intent:', intent)
    if (!intent?.audioMode) return

    console.log('[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:', intent.audioMode)
    removeLocalValue(StorageKey.PENDING_MIC_RECORDING)

    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (tabs[0]?.id) {
        chrome.tabs
          .sendMessage(tabs[0].id, {
            type: 'GASOLINE_ACTION_TOAST',
            text: '',
            detail: '',
            state: 'success' as const,
            duration_ms: 1
          })
          .catch(() => {})
      }
    })

    const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
    if (audioSelect) audioSelect.value = intent.audioMode
  })

  getLocalValue(StorageKey.RECORD_AUDIO_PREF, (value: unknown) => {
    const saved = value as string | undefined
    if (saved) {
      const audioSelect = document.getElementById('record-audio-mode') as HTMLSelectElement | null
      if (audioSelect) audioSelect.value = saved
    }
  })

  row.addEventListener('click', () => {
    console.log('[Gasoline REC] Popup: record row clicked, isRecording:', state.isRecording)
    if (pendingRecordingIntent && !state.isRecording) {
      console.log('[Gasoline REC] Popup: record row click ignored while approval is pending')
      return
    }
    removeRecordHighlight(els)
    if (state.isRecording) {
      handleStopClick(els, state)
    } else {
      handleStartClick(els, state)
    }
  })
}
