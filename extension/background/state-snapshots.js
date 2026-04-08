/**
 * Purpose: CRUD operations for persistent state snapshots stored in chrome.storage.local.
 */
import { getLocal, setLocal } from '../lib/storage-utils.js';
// =============================================================================
// TYPES
// =============================================================================
const SNAPSHOT_KEY = 'kaboom_state_snapshots';
// =============================================================================
// CRUD OPERATIONS
// =============================================================================
/**
 * Save a state snapshot to persistent storage
 */
export async function saveStateSnapshot(name, state) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    const sizeBytes = JSON.stringify(state).length; // nosemgrep: no-stringify-keys
    snapshots[name] = { ...state, name, size_bytes: sizeBytes };
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, snapshot_name: name, size_bytes: sizeBytes };
}
/**
 * Load a state snapshot from persistent storage
 */
export async function loadStateSnapshot(name) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    return snapshots[name] || null;
}
/**
 * List all state snapshots with metadata
 */
export async function listStateSnapshots() {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    return Object.values(snapshots).map((s) => ({
        name: s.name,
        url: s.url,
        timestamp: s.timestamp,
        size_bytes: s.size_bytes
    }));
}
/**
 * Delete a state snapshot from persistent storage
 */
export async function deleteStateSnapshot(name) {
    const existing = (await getLocal(SNAPSHOT_KEY));
    const snapshots = existing || {};
    delete snapshots[name];
    await setLocal(SNAPSHOT_KEY, snapshots);
    return { success: true, deleted: name };
}
//# sourceMappingURL=state-snapshots.js.map