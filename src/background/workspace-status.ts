/**
 * Purpose: Assembles workspace status snapshots for the sidepanel from content heuristics and background session state.
 * Why: Keeps workspace QA state in one typed place instead of duplicating logic across hover, popup, and sidepanel surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */

import { StorageKey } from '../lib/constants.js'
import { getLocal } from '../lib/storage-utils.js'
import type {
  WorkspaceAuditStatus,
  WorkspaceContentStatusPayload,
  WorkspaceMetric,
  WorkspacePerformanceStatus,
  WorkspaceSessionStatus,
  WorkspaceStatusMode,
  WorkspaceStatusSnapshot
} from '../types/workspace-status.js'

interface WorkspaceStatusTab {
  readonly id?: number
  readonly title?: string
  readonly url?: string
}

interface BuildWorkspaceStatusSnapshotOptions {
  readonly mode: WorkspaceStatusMode
  readonly tab: WorkspaceStatusTab
  readonly recordingState?: { readonly active?: boolean } | null
  readonly audit?: { readonly updated_at?: string } | null
  readonly queryContentStatus: () => Promise<WorkspaceContentStatusPayload>
}

function unavailableMetric(label: string): WorkspaceMetric {
  return {
    label,
    score: null,
    state: 'unavailable',
    source: 'unavailable'
  }
}

function unavailablePerformance(): WorkspacePerformanceStatus {
  return {
    verdict: 'not_measured',
    source: 'unavailable'
  }
}

function buildSessionStatus(recordingState?: { readonly active?: boolean } | null): WorkspaceSessionStatus {
  return {
    recording_active: recordingState?.active === true,
    screenshot_count: 0,
    note_count: 0
  }
}

function buildAuditStatus(
  mode: WorkspaceStatusMode,
  audit?: { readonly updated_at?: string } | null
): WorkspaceAuditStatus {
  if (mode === 'audit' && audit?.updated_at) {
    return {
      updated_at: audit.updated_at,
      state: 'available'
    }
  }
  return {
    updated_at: null,
    state: mode === 'audit' ? 'unavailable' : 'idle'
  }
}

function fallbackSnapshot(
  options: Omit<BuildWorkspaceStatusSnapshotOptions, 'queryContentStatus'>
): WorkspaceStatusSnapshot {
  return {
    mode: options.mode,
    seo: unavailableMetric('SEO'),
    accessibility: unavailableMetric('Accessibility'),
    performance: unavailablePerformance(),
    session: buildSessionStatus(options.recordingState),
    audit: buildAuditStatus(options.mode, options.audit),
    page: {
      title: options.tab.title || '',
      url: options.tab.url || '',
      summary: options.tab.title || options.tab.url || 'Workspace status unavailable.'
    },
    recommendation: 'Workspace status is unavailable. Reopen the page context and run an audit.'
  }
}

export async function buildWorkspaceStatusSnapshot(
  options: BuildWorkspaceStatusSnapshotOptions
): Promise<WorkspaceStatusSnapshot> {
  try {
    const contentStatus = await options.queryContentStatus()
    return {
      mode: options.mode,
      seo: contentStatus.seo,
      accessibility: contentStatus.accessibility,
      performance: contentStatus.performance,
      session: buildSessionStatus(options.recordingState),
      audit: buildAuditStatus(options.mode, options.audit),
      page: {
        title: contentStatus.page.title || options.tab.title || '',
        url: contentStatus.page.url || options.tab.url || '',
        summary: contentStatus.page.summary || options.tab.title || options.tab.url || ''
      },
      recommendation: contentStatus.recommendation
    }
  } catch {
    return fallbackSnapshot(options)
  }
}

async function getWorkspaceHostTab(tabId?: number): Promise<WorkspaceStatusTab> {
  if (tabId !== undefined && chrome.tabs?.get) {
    try {
      const tab = await chrome.tabs.get(tabId)
      return { id: tab.id, title: tab.title, url: tab.url }
    } catch {
      // Fall through to active-tab lookup.
    }
  }

  if (chrome.tabs?.query) {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true })
    return { id: tab?.id, title: tab?.title, url: tab?.url }
  }

  return { id: tabId }
}

async function queryWorkspaceStatusFromContent(tabId: number): Promise<WorkspaceContentStatusPayload> {
  const response = await chrome.tabs.sendMessage(tabId, { type: 'kaboom_get_workspace_status' }) as
    | WorkspaceContentStatusPayload
    | { error?: string; message?: string }
    | undefined

  if (
    !response ||
    ('error' in response && response.error) ||
    !('seo' in response && 'accessibility' in response && 'performance' in response && 'page' in response)
  ) {
    throw new Error('workspace status unavailable')
  }
  return response
}

export async function getWorkspaceStatusSnapshot(options: {
  readonly mode?: WorkspaceStatusMode
  readonly tabId?: number
} = {}): Promise<WorkspaceStatusSnapshot> {
  const tab = await getWorkspaceHostTab(options.tabId)
  const recordingState = (await getLocal(StorageKey.RECORDING)) as { active?: boolean } | undefined
  const mode = options.mode || 'live'

  return buildWorkspaceStatusSnapshot({
    mode,
    tab,
    recordingState,
    queryContentStatus: async () => {
      if (tab.id === undefined) throw new Error('missing workspace tab')
      return queryWorkspaceStatusFromContent(tab.id)
    }
  })
}
