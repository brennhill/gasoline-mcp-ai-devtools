/**
 * Purpose: Defines the typed QA workspace status contracts shared by background, content, and sidepanel code.
 * Why: Keeps live heuristics, audit results, and session state synchronized across surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */

export type WorkspaceStatusMode = 'live' | 'audit'

export type WorkspaceMetricState = 'healthy' | 'needs_attention' | 'unavailable'

export type WorkspaceMetricSource = 'heuristic' | 'audit' | 'heuristic_only' | 'unavailable'

export type WorkspacePerformanceVerdict = 'good' | 'mixed' | 'poor' | 'not_measured'

export interface WorkspaceMetric {
  readonly label: string
  readonly score: number | null
  readonly state: WorkspaceMetricState
  readonly source: WorkspaceMetricSource
}

export interface WorkspacePerformanceStatus {
  readonly verdict: WorkspacePerformanceVerdict
  readonly source: Exclude<WorkspaceMetricSource, 'heuristic_only'>
}

export interface WorkspaceSessionStatus {
  readonly recording_active: boolean
  readonly screenshot_count: number
  readonly note_count: number
}

export interface WorkspaceAuditStatus {
  readonly updated_at: string | null
  readonly state: 'idle' | 'available' | 'unavailable'
}

export interface WorkspacePageStatus {
  readonly title: string
  readonly url: string
  readonly summary: string
}

export interface WorkspaceStatusSnapshot {
  readonly mode: WorkspaceStatusMode
  readonly seo: WorkspaceMetric
  readonly accessibility: WorkspaceMetric
  readonly performance: WorkspacePerformanceStatus
  readonly session: WorkspaceSessionStatus
  readonly audit: WorkspaceAuditStatus
  readonly page: WorkspacePageStatus
  readonly recommendation: string
}

export interface WorkspaceStatusHeuristicInput {
  readonly title: string
  readonly url: string
  readonly metaDescription?: string
  readonly canonicalUrl?: string
  readonly headings: readonly string[]
  readonly images: ReadonlyArray<{ readonly alt?: string }>
  readonly interactiveLabels: readonly string[]
  readonly navigationTiming?: {
    readonly domContentLoadedMs?: number
    readonly loadMs?: number
  }
}

export interface WorkspaceContentStatusPayload {
  readonly seo: WorkspaceMetric
  readonly accessibility: WorkspaceMetric
  readonly performance: WorkspacePerformanceStatus
  readonly page: WorkspacePageStatus
  readonly recommendation: string
}
