/**
 * Purpose: Shared DOM action contracts used by background dispatch and injected primitives.
 * Docs: docs/features/feature/interact-explore/index.md
 */

export interface DOMResult {
  success: boolean
  action: string
  selector: string
  value?: unknown
  error?: string
  message?: string
  dom_summary?: string
  timing?: { total_ms: number }
  dom_changes?: { added: number; removed: number; modified: number; summary: string }
  analysis?: string
}

export interface DOMPrimitiveOptions {
  text?: string
  value?: string
  clear?: boolean
  checked?: boolean
  name?: string
  timeout_ms?: number
  analyze?: boolean
}

export interface DOMActionParams extends DOMPrimitiveOptions {
  action?: string
  selector?: string
  reason?: string
  frame?: string | number
}
