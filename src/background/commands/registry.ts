// registry.ts — Command registry and dispatch loop.
// Replaces the monolithic if-chain in pending-queries.ts with a Map-based registry.

import type { PendingQuery } from '../../types'
import type { SyncClient } from '../sync-client'
import * as index from '../index'
import { DebugCategory } from '../debug'
import type { SendAsyncResultFn, QueryParamsObject, TargetResolution } from './helpers'
import {
  sendResult,
  sendAsyncResult,
  requiresTargetTab,
  resolveTargetTab,
  parseQueryParamsObject,
  withTargetContext,
  actionToast
} from './helpers'

const { debugLog } = index

// =============================================================================
// COMMAND CONTEXT
// =============================================================================

export interface CommandContext {
  query: PendingQuery
  syncClient: SyncClient
  tabId: number
  params: QueryParamsObject
  target: TargetResolution | undefined

  /** Send a sync result, wrapped with target context */
  sendResult: (result: unknown) => void

  /** Send an async result, wrapped with target context */
  sendAsyncResult: SendAsyncResultFn

  /** Show action toast on the target tab */
  actionToast: typeof actionToast
}

export type CommandHandler = (ctx: CommandContext) => Promise<void>

// =============================================================================
// REGISTRY
// =============================================================================

const handlers = new Map<string, CommandHandler>()

export function registerCommand(type: string, handler: CommandHandler): void {
  handlers.set(type, handler)
}

// =============================================================================
// DISPATCH
// =============================================================================

export async function dispatch(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  // Wait for initialization to complete (max 2s) so pilot cache is populated
  await Promise.race([index.initReady, new Promise((r) => setTimeout(r, 2000))])

  debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
    id: query.id,
    type: query.type,
    correlation_id: query.correlation_id || null,
    hasSyncClient: !!syncClient
  })

  // Normalize state_* types to a wildcard key
  let queryType: string = query.type
  if (queryType.startsWith('state_')) {
    queryType = 'state_*'
  }

  const handler = handlers.get(queryType)
  if (!handler) {
    debugLog(DebugCategory.CONNECTION, 'Unknown query type', { type: query.type })
    sendResult(syncClient, query.id, {
      error: 'unknown_query_type',
      message: `Unknown query type: ${query.type}`
    })
    return
  }

  // Target resolution
  let target: TargetResolution | undefined
  const paramsObj = parseQueryParamsObject(query.params)
  const needsTarget = requiresTargetTab(query.type)

  if (needsTarget) {
    const resolved = await resolveTargetTab(query, paramsObj)
    if (resolved.error) {
      if (query.correlation_id) {
        sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', resolved.error.payload, resolved.error.message)
      } else {
        sendResult(syncClient, query.id, resolved.error.payload)
      }
      return
    }
    target = resolved.target
  }

  const tabId = target?.tabId ?? 0
  if (needsTarget && !tabId) {
    const payload = {
      success: false,
      error: 'missing_target',
      message: 'No target tab resolved for query'
    }
    if (query.correlation_id) {
      sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', payload, payload.message)
    } else {
      sendResult(syncClient, query.id, payload)
    }
    return
  }

  // Build result wrappers that include target context
  const wrapResult = (result: unknown): unknown => {
    if (!target) return result
    return withTargetContext(result, target)
  }

  const wrappedSendResult = (result: unknown): void => {
    sendResult(syncClient, query.id, wrapResult(result))
  }

  const wrappedSendAsyncResult: SendAsyncResultFn = (
    client, queryId, correlationId, status, result, error
  ): void => {
    sendAsyncResult(client, queryId, correlationId, status, wrapResult(result), error)
  }

  const ctx: CommandContext = {
    query,
    syncClient,
    tabId,
    params: paramsObj,
    target,
    sendResult: wrappedSendResult,
    sendAsyncResult: wrappedSendAsyncResult,
    actionToast
  }

  try {
    await handler(ctx)
  } catch (err) {
    const errMsg = (err as Error).message || 'Unexpected error handling query'
    debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
      type: query.type,
      id: query.id,
      error: errMsg
    })
    if (query.correlation_id) {
      wrappedSendAsyncResult(syncClient, query.id, query.correlation_id, 'error', null, errMsg)
    } else {
      wrappedSendResult({ error: 'query_handler_error', message: errMsg })
    }
  }
}
