/**
 * Purpose: CRUD operations for persistent state snapshots stored in chrome.storage.local.
 */

// state-snapshots.ts — State snapshot storage for saving/loading browser state.

import type { BrowserStateSnapshot } from '../types/index.js'
import { getLocal, setLocal } from '../lib/storage-utils.js'

// =============================================================================
// TYPES
// =============================================================================

const SNAPSHOT_KEY = 'kaboom_state_snapshots'

interface StoredStateSnapshot extends BrowserStateSnapshot {
  name: string
  size_bytes: number
}

interface StateSnapshotStorage {
  [name: string]: StoredStateSnapshot
}

// =============================================================================
// CRUD OPERATIONS
// =============================================================================

/**
 * Save a state snapshot to persistent storage
 */
export async function saveStateSnapshot(
  name: string,
  state: BrowserStateSnapshot
): Promise<{ success: boolean; snapshot_name: string; size_bytes: number }> {
  const existing = (await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined
  const snapshots: StateSnapshotStorage = existing || {}
  const sizeBytes = JSON.stringify(state).length // nosemgrep: no-stringify-keys
  snapshots[name] = { ...state, name, size_bytes: sizeBytes }
  await setLocal(SNAPSHOT_KEY, snapshots)
  return { success: true, snapshot_name: name, size_bytes: sizeBytes }
}

/**
 * Load a state snapshot from persistent storage
 */
export async function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null> {
  const existing = (await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined
  const snapshots: StateSnapshotStorage = existing || {}
  return snapshots[name] || null
}

/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots(): Promise<
  Array<{ name: string; url: string; timestamp: number; size_bytes: number }>
> {
  const existing = (await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined
  const snapshots: StateSnapshotStorage = existing || {}
  return Object.values(snapshots).map((s) => ({
    name: s.name,
    url: s.url,
    timestamp: s.timestamp,
    size_bytes: s.size_bytes
  }))
}

/**
 * Delete a state snapshot from persistent storage
 */
export async function deleteStateSnapshot(name: string): Promise<{ success: boolean; deleted: string }> {
  const existing = (await getLocal(SNAPSHOT_KEY)) as StateSnapshotStorage | undefined
  const snapshots: StateSnapshotStorage = existing || {}
  delete snapshots[name]
  await setLocal(SNAPSHOT_KEY, snapshots)
  return { success: true, deleted: name }
}
