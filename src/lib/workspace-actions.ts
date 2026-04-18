/**
 * Purpose: Centralizes workspace-related runtime and tab action helpers shared by popup, hover, and sidepanel UI.
 * Why: Keeps QA action entrypoints aligned so audit, screenshot, and note capture do not drift across surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */

export async function openWorkspace(): Promise<void> {
  await chrome.runtime.sendMessage({ type: 'open_terminal_panel' })
}

export async function requestWorkspaceAudit(pageUrl?: string): Promise<void> {
  try {
    await openWorkspace()
  } catch {
    // Best effort: still request the audit workflow even if the side panel failed to open.
  }

  await chrome.runtime.sendMessage({ type: 'qa_scan_requested', page_url: pageUrl })
}

export async function requestWorkspaceScreenshot(): Promise<unknown> {
  return await chrome.runtime.sendMessage({ type: 'capture_screenshot' })
}

export async function requestWorkspaceNoteMode(tabId: number): Promise<unknown> {
  return await chrome.tabs.sendMessage(tabId, {
    type: 'kaboom_draw_mode_start',
    started_by: 'user'
  })
}

export async function toggleWorkspaceRecording(recordingActive: boolean): Promise<unknown> {
  if (recordingActive) {
    return await chrome.runtime.sendMessage({ type: 'screen_recording_stop' })
  }
  return await chrome.runtime.sendMessage({ type: 'screen_recording_start', audio: '' })
}
