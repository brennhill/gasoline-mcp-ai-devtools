// pierce-shadow.ts — Pierce-shadow parameter resolution and auto heuristic.
// Extracted from pending-queries.ts to reduce file size.

import * as index from './index'

// Re-exported types used by pending-queries.ts
export type QueryParamsObject = Record<string, unknown>
export type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab'

export interface TargetResolution {
  tabId: number
  url: string
  source: TargetResolutionSource
  requestedTabId?: number
  trackedTabId?: number | null
  trackedTabUrl?: string | null
  useActiveTab: boolean
}

type PierceShadowInput = boolean | 'auto'

export function parsePierceShadowInput(value: unknown): { value?: PierceShadowInput; error?: string } {
  if (value === undefined || value === null) {
    return { value: 'auto' }
  }
  if (typeof value === 'boolean') {
    return { value }
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'auto') return { value: 'auto' }
  }
  return { error: "Invalid 'pierce_shadow' value. Use true, false, or \"auto\"." }
}

function parseOrigin(url: string | null | undefined): string | null {
  if (!url) return null
  try {
    return new URL(url).origin
  } catch {
    return null
  }
}

export function hasActiveDebugIntent(target: TargetResolution | undefined): boolean {
  if (!target) return false
  if (index.__aiWebPilotEnabledCache !== true) return false
  if (target.source !== 'tracked_tab') return false
  if (!target.trackedTabId || target.tabId !== target.trackedTabId) return false

  const targetOrigin = parseOrigin(target.url)
  const trackedOrigin = parseOrigin(target.trackedTabUrl)
  if (!trackedOrigin || !targetOrigin) {
    return false
  }
  return targetOrigin === trackedOrigin
}

export function resolveDOMQueryParams(
  params: QueryParamsObject,
  target: TargetResolution | undefined
): { params?: QueryParamsObject; error?: string } {
  const parsed = parsePierceShadowInput(params.pierce_shadow)
  if (parsed.error) {
    return { error: parsed.error }
  }

  const pierceShadow = parsed.value === 'auto' ? hasActiveDebugIntent(target) : parsed.value
  return {
    params: {
      ...params,
      pierce_shadow: pierceShadow
    }
  }
}
