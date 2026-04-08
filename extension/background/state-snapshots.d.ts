/**
 * Purpose: CRUD operations for persistent state snapshots stored in chrome.storage.local.
 */
import type { BrowserStateSnapshot } from '../types/index.js';
interface StoredStateSnapshot extends BrowserStateSnapshot {
    name: string;
    size_bytes: number;
}
/**
 * Save a state snapshot to persistent storage
 */
export declare function saveStateSnapshot(name: string, state: BrowserStateSnapshot): Promise<{
    success: boolean;
    snapshot_name: string;
    size_bytes: number;
}>;
/**
 * Load a state snapshot from persistent storage
 */
export declare function loadStateSnapshot(name: string): Promise<StoredStateSnapshot | null>;
/**
 * List all state snapshots with metadata
 */
export declare function listStateSnapshots(): Promise<Array<{
    name: string;
    url: string;
    timestamp: number;
    size_bytes: number;
}>>;
/**
 * Delete a state snapshot from persistent storage
 */
export declare function deleteStateSnapshot(name: string): Promise<{
    success: boolean;
    deleted: string;
}>;
export {};
//# sourceMappingURL=state-snapshots.d.ts.map