/**
 * @fileoverview Error Grouping and Deduplication
 * Manages error group tracking, deduplication within configurable windows,
 * and cleanup of stale error groups.
 */

import type { LogEntry } from '../types';

// =============================================================================
// CONSTANTS
// =============================================================================

/** Error deduplication window in milliseconds */
const ERROR_DEDUP_WINDOW_MS = 5000;

/** Error group flush interval in milliseconds */
const ERROR_GROUP_FLUSH_MS = 10000;

/** Maximum tracked error groups */
const MAX_TRACKED_ERRORS = 100;

/** Error group max age - cleanup after 1 hour */
export const ERROR_GROUP_MAX_AGE_MS = 3600000;

// =============================================================================
// TYPE DEFINITIONS
// =============================================================================

/** Internal error group structure */
interface InternalErrorGroup {
  entry: LogEntry;
  count: number;
  firstSeen: number;
  lastSeen: number;
}

/** Process error group result */
interface ProcessErrorGroupResult {
  shouldSend: boolean;
  entry?: LogEntry & { _previousOccurrences?: number };
}

/** Processed log entry with aggregation metadata */
export type ProcessedLogEntry = LogEntry & {
  _aggregatedCount?: number;
  _firstSeen?: string;
  _lastSeen?: string;
};

// =============================================================================
// STATE
// =============================================================================

/** Error grouping state */
const errorGroups = new Map<string, InternalErrorGroup>();

// =============================================================================
// ERROR GROUPING
// =============================================================================

/**
 * Create a signature for an error to identify duplicates
 */
export function createErrorSignature(entry: LogEntry): string {
  const parts: string[] = [];

  parts.push((entry as { type?: string }).type || 'unknown');
  parts.push(entry.level || 'error');

  if ((entry as { type?: string }).type === 'exception') {
    const exEntry = entry as { message?: string; stack?: string };
    parts.push(exEntry.message || '');
    if (exEntry.stack) {
      const firstFrame = exEntry.stack.split('\n')[1] || '';
      parts.push(firstFrame.trim());
    }
  } else if ((entry as { type?: string }).type === 'network') {
    const netEntry = entry as { method?: string; url?: string; status?: number };
    parts.push(netEntry.method || 'GET');
    try {
      const url = new URL(netEntry.url || '', 'http://localhost');
      parts.push(url.pathname);
    } catch {
      parts.push(netEntry.url || '');
    }
    parts.push(String(netEntry.status || 0));
  } else if ((entry as { type?: string }).type === 'console') {
    const consEntry = entry as { args?: unknown[] };
    if (consEntry.args && consEntry.args.length > 0) {
      const firstArg = consEntry.args[0];
      parts.push(
        typeof firstArg === 'string' ? firstArg.slice(0, 200) : JSON.stringify(firstArg).slice(0, 200)
      );
    }
  }

  return parts.join('|');
}

/**
 * Process an error through the grouping system
 */
export function processErrorGroup(entry: LogEntry): ProcessErrorGroupResult {
  if (entry.level !== 'error' && entry.level !== 'warn') {
    return { shouldSend: true, entry };
  }

  const signature = createErrorSignature(entry);
  const now = Date.now();

  if (errorGroups.has(signature)) {
    const group = errorGroups.get(signature)!;

    if (now - group.lastSeen < ERROR_DEDUP_WINDOW_MS) {
      group.count++;
      group.lastSeen = now;
      return { shouldSend: false };
    }

    const countToReport = group.count;
    group.count = 1;
    group.lastSeen = now;
    group.firstSeen = now;

    if (countToReport > 1) {
      return {
        shouldSend: true,
        entry: { ...entry, _previousOccurrences: countToReport - 1 } as LogEntry & {
          _previousOccurrences: number;
        },
      };
    }
    return { shouldSend: true, entry };
  }

  if (errorGroups.size >= MAX_TRACKED_ERRORS) {
    let oldestSig: string | null = null;
    let oldestTime = Infinity;
    for (const [sig, group] of errorGroups) {
      if (group.lastSeen < oldestTime) {
        oldestTime = group.lastSeen;
        oldestSig = sig;
      }
    }
    if (oldestSig) {
      errorGroups.delete(oldestSig);
    }
  }

  errorGroups.set(signature, {
    entry,
    count: 1,
    firstSeen: now,
    lastSeen: now,
  });

  return { shouldSend: true, entry };
}

/**
 * Get current state of error groups (for testing)
 */
export function getErrorGroupsState(): Map<string, InternalErrorGroup> {
  return errorGroups;
}

/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export function cleanupStaleErrorGroups(
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): void {
  const now = Date.now();
  for (const [signature, group] of errorGroups) {
    if (now - group.lastSeen > ERROR_GROUP_MAX_AGE_MS) {
      errorGroups.delete(signature);
      if (debugLogFn) {
        debugLogFn('error', 'Cleaned up stale error group', {
          signature: signature.slice(0, 50) + '...',
          age: Math.round((now - group.lastSeen) / 60000) + ' min',
        });
      }
    }
  }
}

/**
 * Flush error groups - send any accumulated counts
 */
export function flushErrorGroups(): ProcessedLogEntry[] {
  const now = Date.now();
  const entriesToSend: ProcessedLogEntry[] = [];

  for (const [signature, group] of errorGroups) {
    if (group.count > 1) {
      // Spread the readonly entry and add our mutable properties
      const processedEntry = {
        ...group.entry,
        _aggregatedCount: group.count,
        _firstSeen: new Date(group.firstSeen).toISOString(),
        _lastSeen: new Date(group.lastSeen).toISOString(),
      };
      // Override ts with fresh timestamp (cast to mutable)
      (processedEntry as { ts: string }).ts = new Date().toISOString();
      entriesToSend.push(processedEntry as unknown as ProcessedLogEntry);
      group.count = 0;
    }

    if (now - group.lastSeen > ERROR_GROUP_FLUSH_MS * 2) {
      errorGroups.delete(signature);
    }
  }

  return entriesToSend;
}
