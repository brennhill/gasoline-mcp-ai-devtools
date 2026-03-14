/**
 * Purpose: Shared recording helpers: slug generation, toast labels, and badge timer lifecycle.
 * Why: Consolidates all recording utility functions so callers have a single import point.
 * Docs: docs/features/feature/flow-recording/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 */
// =============================================================================
// SLUG & LABEL GENERATION
// =============================================================================
/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export function buildScreenRecordingSlug(url) {
    try {
        const hostname = new URL(url ?? '').hostname.replace(/^www\./, '');
        return (hostname
            .replace(/[^a-z0-9]/gi, '-')
            .replace(/-+/g, '-')
            .replace(/^-|-$/g, '') || 'recording');
    }
    catch {
        return 'recording';
    }
}
/**
 * Build a short human-readable recording toast label from a tab URL.
 */
export function buildRecordingToastLabel(url) {
    try {
        const parsed = new URL(url ?? '');
        const host = parsed.hostname.replace(/^www\./, '');
        const path = parsed.pathname === '/' ? '' : parsed.pathname;
        const base = `${host}${path}`;
        const clipped = base.length > 42 ? `${base.slice(0, 39)}...` : base;
        return `Recording ${clipped}`;
    }
    catch {
        return 'Recording started';
    }
}
// =============================================================================
// RECORDING BADGE TIMER
// =============================================================================
let badgeTimerInterval = null;
let badgeStartTime = null;
function updateRecordingBadge() {
    if (!chrome?.action || !badgeStartTime)
        return;
    const elapsed = Math.round((Date.now() - badgeStartTime) / 1000);
    const mins = Math.floor(elapsed / 60);
    const secs = elapsed % 60;
    const text = mins > 0 ? `${mins}:${secs.toString().padStart(2, '0')}` : `${secs}s`;
    try {
        chrome.action.setBadgeText({ text });
    }
    catch {
        // Badge updates are best-effort.
    }
}
export function startRecordingBadgeTimer(startTime) {
    stopRecordingBadgeTimer();
    badgeStartTime = startTime;
    if (!chrome?.action)
        return;
    try {
        chrome.action.setBadgeBackgroundColor({ color: '#dc2626' });
    }
    catch {
        // Badge updates are best-effort.
    }
    updateRecordingBadge();
    badgeTimerInterval = setInterval(updateRecordingBadge, 1000);
}
export function stopRecordingBadgeTimer() {
    if (badgeTimerInterval) {
        clearInterval(badgeTimerInterval);
        badgeTimerInterval = null;
    }
    badgeStartTime = null;
    if (!chrome?.action)
        return;
    try {
        chrome.action.setBadgeText({ text: '' });
    }
    catch {
        // Badge updates are best-effort.
    }
}
//# sourceMappingURL=recording-utils.js.map