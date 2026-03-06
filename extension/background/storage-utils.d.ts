/**
 * Purpose: Re-exports shared storage utilities for backward compatibility with background service worker imports.
 * Why: The canonical implementation lives in src/lib/storage-utils.ts so both background and popup can share it.
 */
export { setSessionValue, getSessionValue, removeSessionValue, clearSessionStorage, setLocalValue, setLocalValues, getLocalValue, getLocalValues, removeLocalValue, removeLocalValues, setValue, getValue, removeValue, getStorageDiagnostics, wasServiceWorkerRestarted, markStateVersion } from '../lib/storage-utils.js';
//# sourceMappingURL=storage-utils.d.ts.map