/**
 * Purpose: Owns mixed page-context injection policy for the workspace sidepanel.
 * Why: Keeps auto/manual terminal injections and route-refresh behavior separate from shell rendering.
 * Docs: docs/features/feature/terminal/index.md
 */

import type { WorkspaceStatusSnapshot } from '../types/workspace-status.js'

export interface WorkspaceContextUiState {
  readonly message: string | null
}

interface WorkspaceContextControllerOptions {
  readonly hostTabId?: number
  readonly writeToTerminal: (text: string) => void
  readonly shouldDeferWrite: () => boolean
  readonly onUiStateChange: (state: WorkspaceContextUiState) => void
  readonly refreshWorkspaceStatus: (mode?: 'live' | 'audit') => Promise<WorkspaceStatusSnapshot | undefined>
}

export interface WorkspaceContextController {
  setSnapshot: (snapshot: WorkspaceStatusSnapshot) => void
  handleWorkspaceOpen: (snapshot: WorkspaceStatusSnapshot | undefined) => void
  handleAuditSnapshot: (snapshot: WorkspaceStatusSnapshot) => void
  injectCurrentContext: () => void
  reset: () => void
  dispose: () => void
}

const ROUTE_REFRESH_RETRY_DELAY_MS = 150
const ROUTE_REFRESH_MAX_ATTEMPTS = 3

function buildPageContextText(snapshot: WorkspaceStatusSnapshot): string {
  const title = snapshot.page.title || 'Untitled page'
  const summary = snapshot.page.summary || 'No page summary available.'
  return [
    'Page context',
    `Title: ${title}`,
    `URL: ${snapshot.page.url}`,
    `Summary: ${summary}`,
    `SEO: ${snapshot.seo.score ?? 'unavailable'} (${snapshot.seo.source})`,
    `Accessibility: ${snapshot.accessibility.score ?? 'unavailable'} (${snapshot.accessibility.source})`,
    `Performance: ${snapshot.performance.verdict}`
  ].join('\n')
}

function buildAuditSummaryText(snapshot: WorkspaceStatusSnapshot): string {
  const updatedAt = snapshot.audit.updated_at ?? 'not available'
  return [
    'Audit summary',
    `Updated: ${updatedAt}`,
    `SEO: ${snapshot.seo.score ?? 'unavailable'}`,
    `Accessibility: ${snapshot.accessibility.score ?? 'unavailable'}`,
    `Performance: ${snapshot.performance.verdict}`,
    `Recommendation: ${snapshot.recommendation}`
  ].join('\n')
}

export function createWorkspaceContextController(options: WorkspaceContextControllerOptions): WorkspaceContextController {
  let currentSnapshot: WorkspaceStatusSnapshot | null = null
  let lastAuditInjectionAt: string | null = null
  let removed = false
  let routeRefreshTimer: ReturnType<typeof setTimeout> | null = null
  let routeRefreshRequestId = 0

  function clearRouteRefreshTimer(): void {
    if (routeRefreshTimer === null) return
    clearTimeout(routeRefreshTimer)
    routeRefreshTimer = null
  }

  function refreshRouteContext(targetUrl: string, attempt: number, requestId: number): void {
    void options.refreshWorkspaceStatus('live').then((snapshot) => {
      if (removed || requestId !== routeRefreshRequestId) return
      const resolvedUrl = snapshot?.page.url
      if (!snapshot || !resolvedUrl || resolvedUrl !== targetUrl) {
        if (attempt + 1 >= ROUTE_REFRESH_MAX_ATTEMPTS) {
          options.onUiStateChange({ message: 'Page context unavailable until the new route finishes loading.' })
          return
        }
        routeRefreshTimer = setTimeout(() => {
          routeRefreshTimer = null
          refreshRouteContext(targetUrl, attempt + 1, requestId)
        }, ROUTE_REFRESH_RETRY_DELAY_MS)
        return
      }
      currentSnapshot = snapshot
      injectPageContext(snapshot)
    })
  }

  const routeListener =
    options.hostTabId !== undefined && chrome.tabs?.onUpdated
      ? (tabId: number, changeInfo: { url?: string }, _tab: chrome.tabs.Tab): void => {
          if (removed || tabId !== options.hostTabId) return
          if (!changeInfo.url || changeInfo.url === currentSnapshot?.page.url) return
          routeRefreshRequestId += 1
          clearRouteRefreshTimer()
          options.onUiStateChange({ message: 'Context injection queued for the new route.' })
          refreshRouteContext(changeInfo.url, 0, routeRefreshRequestId)
        }
      : null

  if (routeListener) {
    chrome.tabs.onUpdated.addListener(routeListener)
  }

  function injectText(text: string, queuedMessage: string, sentMessage: string): void {
    options.onUiStateChange({ message: options.shouldDeferWrite() ? queuedMessage : sentMessage })
    options.writeToTerminal(text)
  }

  function injectPageContext(snapshot: WorkspaceStatusSnapshot): void {
    injectText(
      buildPageContextText(snapshot),
      'Context injection queued until the terminal is idle.',
      'Page context injected into the workspace terminal.'
    )
  }

  function injectAuditSummary(snapshot: WorkspaceStatusSnapshot): void {
    injectText(
      buildAuditSummaryText(snapshot),
      'Audit summary queued until the terminal is idle.',
      'Audit summary injected into the workspace terminal.'
    )
  }

  return {
    setSnapshot(snapshot): void {
      currentSnapshot = snapshot
    },
    handleWorkspaceOpen(snapshot): void {
      if (!snapshot) return
      currentSnapshot = snapshot
      injectPageContext(snapshot)
    },
    handleAuditSnapshot(snapshot): void {
      currentSnapshot = snapshot
      const updatedAt = snapshot.audit.updated_at
      if (snapshot.mode !== 'audit' || !updatedAt || updatedAt === lastAuditInjectionAt) return
      lastAuditInjectionAt = updatedAt
      injectAuditSummary(snapshot)
    },
    injectCurrentContext(): void {
      if (!currentSnapshot) {
        options.onUiStateChange({ message: 'Page context unavailable until the workspace loads page status.' })
        return
      }
      injectPageContext(currentSnapshot)
    },
    reset(): void {
      lastAuditInjectionAt = null
      options.onUiStateChange({ message: 'Workspace session reset. Terminal and recording stay active.' })
    },
    dispose(): void {
      removed = true
      clearRouteRefreshTimer()
      if (routeListener) {
        chrome.tabs.onUpdated.removeListener(routeListener)
      }
    }
  }
}
