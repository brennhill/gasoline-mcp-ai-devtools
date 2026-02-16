/**
 * @fileoverview Flushes early-patch diagnostics into the standard log pipeline.
 */
import { postLog } from '../lib/bridge.js';
function normalizeLevel(level) {
    if (level === 'debug' || level === 'info' || level === 'warn' || level === 'error') {
        return level;
    }
    return 'warn';
}
export function flushEarlyPatchLogs() {
    if (typeof window === 'undefined')
        return 0;
    const queue = window.__GASOLINE_EARLY_LOGS__;
    if (!Array.isArray(queue) || queue.length === 0)
        return 0;
    const pending = queue.splice(0, queue.length);
    let forwarded = 0;
    for (const raw of pending) {
        if (!raw || typeof raw !== 'object')
            continue;
        const entry = raw;
        const message = typeof entry.message === 'string' && entry.message.length > 0 ? entry.message : 'early-patch event';
        const source = typeof entry.source === 'string' && entry.source.length > 0 ? entry.source : 'early-patch';
        const category = typeof entry.category === 'string' && entry.category.length > 0 ? entry.category : 'early_patch';
        const earlyTs = typeof entry.ts === 'string' ? entry.ts : undefined;
        postLog({
            level: normalizeLevel(entry.level),
            type: 'early_patch',
            message,
            source,
            category,
            ...(entry.data !== undefined ? { data: entry.data } : {}),
            ...(earlyTs ? { early_patch_ts: earlyTs } : {})
        });
        forwarded += 1;
    }
    return forwarded;
}
//# sourceMappingURL=early-patch-logs.js.map