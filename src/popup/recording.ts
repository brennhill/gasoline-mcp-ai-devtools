/**
 * Purpose: Implements popup recording controls setup, storage-driven state sync, and approval flow wiring.
 * Why: Owns the wiring/lifecycle for recording UI while delegating rendering to recording-ui-state.ts.
 * Docs: docs/features/feature/playback-engine/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */

import { StorageKey } from '../lib/constants.js'
import { getLocal, removeLocal, onStorageChanged } from '../lib/storage-utils.js'
import {
  sendRecordingGestureDecision,
  handleStartClick,
  handleStopClick,
  type RecordingElements,
  type RecordingState
} from './recording-io.js'
import {
  applyRecordHighlight,
  removeRecordHighlight,
  showRecording,
  showIdle,
  showSaveResult,
  showStartError,
  setApprovalPendingState,
  type PendingRecordingIntent,
  type ApprovalElements
} from './recording-ui-state.js'

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
      void removeLocal(StorageKey.PENDING_RECORDING)
      return
    }
    pendingRecordingIntent = pending && !pending.highlight ? pending : null
    if (!pendingRecordingIntent && !state.isRecording) removeRecordHighlight(els)
    setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent)
  }

  const clearPendingRecordingIntent = (): void => {
    pendingRecordingIntent = null
    setApprovalPendingState(els, approvalEls, state, null)
    void removeLocal(StorageKey.PENDING_RECORDING)
  }

  row.style.visibility = 'hidden'

  void getLocal(StorageKey.RECORDING).then(async (value: unknown) => {
    const rec = value as { active?: boolean; name?: string; startTime?: number } | undefined
    console.log('[Gasoline REC] Popup: gasoline_recording from storage:', rec)
    if (rec?.active && rec.name && rec.startTime) {
      console.log('[Gasoline REC] Popup: resuming recording UI for', rec.name)
      showRecording(els, state, rec.name, rec.startTime)
    }
    row.style.visibility = 'visible'

    // Check for highlight request from hover launcher
    const pendingValue = await getLocal(StorageKey.PENDING_RECORDING)
    updatePendingRecording(pendingValue)
  })

  onStorageChanged((changes, areaName) => {
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
    sendRecordingGestureDecision('recording_gesture_granted')
    clearPendingRecordingIntent()
  })

  approvalEls.denyBtn?.addEventListener('click', (event) => {
    event.preventDefault()
    sendRecordingGestureDecision('recording_gesture_denied')
    clearPendingRecordingIntent()
  })

  void getLocal(StorageKey.PENDING_MIC_RECORDING).then(async (value: unknown) => {
    const intent = value as { audioMode?: string } | undefined
    console.log('[Gasoline REC] Popup: pending_mic_recording intent:', intent)
    if (!intent?.audioMode) return

    console.log('[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:', intent.audioMode)
    await removeLocal(StorageKey.PENDING_MIC_RECORDING)

    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (tabs[0]?.id) {
        chrome.tabs
          .sendMessage(tabs[0].id, {
            type: 'gasoline_action_toast',
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

  void getLocal(StorageKey.RECORD_AUDIO_PREF).then((value: unknown) => {
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
      handleStopClick(els, state, showIdle, showSaveResult)
    } else {
      handleStartClick(els, state, showRecording, showIdle, showStartError)
    }
  })
}
