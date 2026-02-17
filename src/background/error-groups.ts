/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Error Grouping and Deduplication
 * Manages error group tracking, deduplication within configurable windows,
 * and cleanup of stale error groups.
 */

import type { LogEntry } from '../types'

// =============================================================================
// CONSTANTS
// =============================================================================

/** Error deduplication window in milliseconds */
const ERROR_DEDUP_WINDOW_MS = 5000

/** Error group flush interval in milliseconds */
const ERROR_GROUP_FLUSH_MS = 10000

/** Maximum tracked error groups */
const MAX_TRACKED_ERRORS = 100

/** Error group max age - cleanup after 1 hour */
export const ERROR_GROUP_MAX_AGE_MS = 3600000

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/** Internal error group structure */
interface InternalErrorGroup {
  entry: LogEntry
  count: number
  firstSeen: number
  lastSeen: number
}

/** Process error group result */
interface ProcessErrorGroupResult {
  shouldSend: boolean
  entry?: LogEntry & { _previousOccurrences?: number }
}

/** Processed log entry with aggregation metadata */
export type ProcessedLogEntry = LogEntry & {
  _aggregatedCount?: number
  _firstSeen?: string
  _lastSeen?: string
}

// =============================================================================
// STATE
// =============================================================================

/** Error grouping state */
const errorGroups = new Map<string, InternalErrorGroup>()

// =============================================================================
// ERROR GROUPING
// =============================================================================

/**
 * Create a signature for an error to identify duplicates
 */
type SignatureExtractor = (entry: LogEntry) => string[]

const SIGNATURE_EXTRACTORS: Record<string, SignatureExtractor> = {
  exception: (entry) => {
    const exEntry = entry as { message?: string; stack?: string }
    const parts = [exEntry.message || '']
    if (exEntry.stack) {
      const firstFrame = exEntry.stack.split('\n')[1] || ''
      parts.push(firstFrame.trim())
    }
    return parts
  },
  network: (entry) => {
    const netEntry = entry as { method?: string; url?: string; status?: number }
    const parts = [netEntry.method || 'GET']
    try {
      const url = new URL(netEntry.url || '', 'http://localhost')
      parts.push(url.pathname)
    } catch {
      parts.push(netEntry.url || '')
    }
    parts.push(String(netEntry.status || 0))
    return parts
  },
  console: (entry) => {
    const consEntry = entry as { args?: unknown[] }
    if (consEntry.args && consEntry.args.length > 0) {
      const firstArg = consEntry.args[0]
      return [typeof firstArg === 'string' ? firstArg.slice(0, 200) : JSON.stringify(firstArg).slice(0, 200)]
    }
    return []
  }
}

export function createErrorSignature(entry: LogEntry): string {
  const entryType = (entry as { type?: string }).type || 'unknown'
  const parts: string[] = [entryType, entry.level || 'error']

  const extractor = SIGNATURE_EXTRACTORS[entryType]
  if (extractor) parts.push(...extractor(entry))

  return parts.join('|')
}

/**
 * Process an error through the grouping system
 */
function handleExistingGroup(group: InternalErrorGroup, entry: LogEntry, now: number): ProcessErrorGroupResult {
  if (now - group.lastSeen < ERROR_DEDUP_WINDOW_MS) {
    group.count++
    group.lastSeen = now
    return { shouldSend: false }
  }

  const countToReport = group.count
  group.count = 1
  group.lastSeen = now
  group.firstSeen = now

  if (countToReport > 1) {
    return {
      shouldSend: true,
      entry: { ...entry, _previousOccurrences: countToReport - 1 } as LogEntry & { _previousOccurrences: number }
    }
  }
  return { shouldSend: true, entry }
}

function evictOldestGroup(): void {
  if (errorGroups.size < MAX_TRACKED_ERRORS) return
  let oldestSig: string | null = null
  let oldestTime = Infinity
  for (const [sig, group] of errorGroups) {
    if (group.lastSeen < oldestTime) {
      oldestTime = group.lastSeen
      oldestSig = sig
    }
  }
  if (oldestSig) errorGroups.delete(oldestSig)
}

export function processErrorGroup(entry: LogEntry): ProcessErrorGroupResult {
  if (entry.level !== 'error' && entry.level !== 'warn') {
    return { shouldSend: true, entry }
  }

  const signature = createErrorSignature(entry)
  const now = Date.now()
  const existing = errorGroups.get(signature)

  if (existing) return handleExistingGroup(existing, entry, now)

  evictOldestGroup()
  errorGroups.set(signature, { entry, count: 1, firstSeen: now, lastSeen: now })
  return { shouldSend: true, entry }
}

/**
 * Get current state of error groups (for testing)
 */
export function getErrorGroupsState(): Map<string, InternalErrorGroup> {
  return errorGroups
}

/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export function cleanupStaleErrorGroups(
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): void {
  const now = Date.now()
  for (const [signature, group] of errorGroups) {
    if (now - group.lastSeen > ERROR_GROUP_MAX_AGE_MS) {
      errorGroups.delete(signature)
      if (debugLogFn) {
        debugLogFn('error', 'Cleaned up stale error group', {
          signature: signature.slice(0, 50) + '...',
          age: Math.round((now - group.lastSeen) / 60000) + ' min'
        })
      }
    }
  }
}

/**
 * Flush error groups - send any accumulated counts
 */
export function flushErrorGroups(): ProcessedLogEntry[] {
  const now = Date.now()
  const entriesToSend: ProcessedLogEntry[] = []

  for (const [signature, group] of errorGroups) {
    if (group.count > 1) {
      // Spread the readonly entry and add our mutable properties
      const processedEntry = {
        ...group.entry,
        _aggregatedCount: group.count,
        _firstSeen: new Date(group.firstSeen).toISOString(),
        _lastSeen: new Date(group.lastSeen).toISOString()
      }
      // Override ts with fresh timestamp (cast to mutable)
      ;(processedEntry as { ts: string }).ts = new Date().toISOString()
      entriesToSend.push(processedEntry as unknown as ProcessedLogEntry)
      group.count = 0
    }

    if (now - group.lastSeen > ERROR_GROUP_FLUSH_MS * 2) {
      errorGroups.delete(signature)
    }
  }

  return entriesToSend
}
