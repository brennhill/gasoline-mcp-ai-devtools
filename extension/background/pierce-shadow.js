// pierce-shadow.ts — Pierce-shadow parameter resolution and auto heuristic.
// Extracted from pending-queries.ts to reduce file size.
import * as index from './index.js';
export function parsePierceShadowInput(value) {
    if (value === undefined || value === null) {
        return { value: 'auto' };
    }
    if (typeof value === 'boolean') {
        return { value };
    }
    if (typeof value === 'string') {
        const normalized = value.trim().toLowerCase();
        if (normalized === 'auto')
            return { value: 'auto' };
    }
    return { error: "Invalid 'pierce_shadow' value. Use true, false, or \"auto\"." };
}
function parseOrigin(url) {
    if (!url)
        return null;
    try {
        return new URL(url).origin;
    }
    catch {
        return null;
    }
}
export function hasActiveDebugIntent(target) {
    if (!target)
        return false;
    if (index.__aiWebPilotEnabledCache !== true)
        return false;
    if (target.source !== 'tracked_tab')
        return false;
    if (!target.trackedTabId || target.tabId !== target.trackedTabId)
        return false;
    const targetOrigin = parseOrigin(target.url);
    const trackedOrigin = parseOrigin(target.trackedTabUrl);
    if (!trackedOrigin || !targetOrigin) {
        return false;
    }
    return targetOrigin === trackedOrigin;
}
export function resolveDOMQueryParams(params, target) {
    const parsed = parsePierceShadowInput(params.pierce_shadow);
    if (parsed.error) {
        return { error: parsed.error };
    }
    const pierceShadow = parsed.value === 'auto' ? hasActiveDebugIntent(target) : parsed.value;
    return {
        params: {
            ...params,
            pierce_shadow: pierceShadow
        }
    };
}
//# sourceMappingURL=pierce-shadow.js.map