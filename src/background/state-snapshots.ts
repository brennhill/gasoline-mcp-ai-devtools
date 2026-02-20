// state-snapshots.ts — Chrome storage persistence for browser state snapshots.

/**
 * @fileoverview State Snapshot Storage
 * Provides CRUD operations for saving/loading/listing/deleting browser state
 * snapshots in chrome.storage.local. Used by the interact tool's state_*
 * actions (save_state, load_state, list_states, delete_state).
 */

import type { BrowserStateSnapshot } from '../types'

// =============================================================================
// CONSTANTS & TYPES
// =============================================================================

const SNAPSHOT_KEY = 'gasoline_state_snapshots'

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
 * Save a state snapshot to chrome.storage.local
 */
export async function saveStateSnapshot(
  name: string,
  state: BrowserStateSnapshot
): Promise<{ success: boolean; snapshot_name: string; size_bytes: number }> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      const sizeBytes = JSON.stringify(state).length // nosemgrep: no-stringify-keys
      snapshots[name] = {
        ...state,
        name,
        size_bytes: sizeBytes
      }
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({
          success: true,
          snapshot_name: name,
          size_bytes: sizeBytes
        })
      })
    })
  })
}

/**
 * Load a state snapshot from chrome.storage.local
 */
export async function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      resolve(snapshots[name] || null)
    })
  })
}

/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots(): Promise<
  Array<{ name: string; url: string; timestamp: number; size_bytes: number }>
> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      const list = Object.values(snapshots).map((s) => ({
        name: s.name,
        url: s.url,
        timestamp: s.timestamp,
        size_bytes: s.size_bytes
      }))
      resolve(list)
    })
  })
}

/**
 * Delete a state snapshot from chrome.storage.local
 */
export async function deleteStateSnapshot(name: string): Promise<{ success: boolean; deleted: string }> {
  return new Promise((resolve) => {
    chrome.storage.local.get(SNAPSHOT_KEY, (result: { [key: string]: StateSnapshotStorage }) => {
      const snapshots: StateSnapshotStorage = result[SNAPSHOT_KEY] || {}
      delete snapshots[name]
      chrome.storage.local.set({ [SNAPSHOT_KEY]: snapshots }, () => {
        resolve({ success: true, deleted: name })
      })
    })
  })
}
