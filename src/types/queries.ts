/**
 * @fileoverview Pending Query Types
 * Query types, browser actions, and tabs information
 */

/**
 * Query types from server
 */
export type QueryType =
  | 'dom'
  | 'a11y'
  | 'execute'
  | 'highlight'
  | 'page_info'
  | 'tabs'
  | 'browser_action'
  | 'waterfall'
  | 'dom_action'
  | 'state_capture'
  | 'state_save'
  | 'state_load'
  | 'state_list'
  | 'state_delete'
  | 'subtitle'
  | 'screenshot'
  | 'record_start'
  | 'record_stop'
  | 'link_health'

/**
 * Pending query from server
 */
export interface PendingQuery {
  readonly id: string
  readonly type: QueryType
  readonly params: string | Record<string, unknown>
  readonly correlation_id?: string
}

/**
 * Browser action parameters
 */
export interface BrowserActionParams {
  readonly action: 'refresh' | 'navigate' | 'back' | 'forward'
  readonly url?: string
}

/**
 * Browser action result
 */
export interface BrowserActionResult {
  readonly success: boolean
  readonly action?: string
  readonly url?: string
  readonly content_script_status?: 'loaded' | 'refreshed' | 'failed' | 'unavailable'
  readonly message?: string
  readonly error?: string
}

/**
 * Tabs query result
 */
export interface TabInfo {
  readonly id: number
  readonly url: string
  readonly title: string
  readonly active: boolean
  readonly windowId: number
  readonly index: number
}
