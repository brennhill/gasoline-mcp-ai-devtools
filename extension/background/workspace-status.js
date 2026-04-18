/**
 * Purpose: Assembles workspace status snapshots for the sidepanel from content heuristics and background session state.
 * Why: Keeps workspace QA state in one typed place instead of duplicating logic across hover, popup, and sidepanel surfaces.
 * Docs: docs/features/feature/terminal/index.md
 */
import { StorageKey } from '../lib/constants.js';
import { getLocal } from '../lib/storage-utils.js';
function unavailableMetric(label) {
    return {
        label,
        score: null,
        state: 'unavailable',
        source: 'unavailable'
    };
}
function unavailablePerformance() {
    return {
        verdict: 'not_measured',
        source: 'unavailable'
    };
}
function buildSessionStatus(recordingState) {
    return {
        recording_active: recordingState?.active === true,
        screenshot_count: 0,
        note_count: 0
    };
}
function buildAuditStatus(mode, audit) {
    if (mode === 'audit' && audit?.updated_at) {
        return {
            updated_at: audit.updated_at,
            state: 'available'
        };
    }
    return {
        updated_at: null,
        state: mode === 'audit' ? 'unavailable' : 'idle'
    };
}
function fallbackSnapshot(options) {
    return {
        mode: options.mode,
        seo: unavailableMetric('SEO'),
        accessibility: unavailableMetric('Accessibility'),
        performance: unavailablePerformance(),
        session: buildSessionStatus(options.recordingState),
        audit: buildAuditStatus(options.mode, options.audit),
        page: {
            title: options.tab.title || '',
            url: options.tab.url || '',
            summary: options.tab.title || options.tab.url || 'Workspace status unavailable.'
        },
        recommendation: 'Workspace status is unavailable. Reopen the page context and run an audit.'
    };
}
export async function buildWorkspaceStatusSnapshot(options) {
    try {
        const contentStatus = await options.queryContentStatus();
        return {
            mode: options.mode,
            seo: contentStatus.seo,
            accessibility: contentStatus.accessibility,
            performance: contentStatus.performance,
            session: buildSessionStatus(options.recordingState),
            audit: buildAuditStatus(options.mode, options.audit),
            page: {
                title: contentStatus.page.title || options.tab.title || '',
                url: contentStatus.page.url || options.tab.url || '',
                summary: contentStatus.page.summary || options.tab.title || options.tab.url || ''
            },
            recommendation: contentStatus.recommendation
        };
    }
    catch {
        return fallbackSnapshot(options);
    }
}
async function getWorkspaceHostTab(tabId) {
    if (tabId !== undefined && chrome.tabs?.get) {
        try {
            const tab = await chrome.tabs.get(tabId);
            return { id: tab.id, title: tab.title, url: tab.url };
        }
        catch {
            // Fall through to active-tab lookup.
        }
    }
    if (chrome.tabs?.query) {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        return { id: tab?.id, title: tab?.title, url: tab?.url };
    }
    return { id: tabId };
}
async function queryWorkspaceStatusFromContent(tabId) {
    const response = await chrome.tabs.sendMessage(tabId, { type: 'kaboom_get_workspace_status' });
    if (!response ||
        ('error' in response && response.error) ||
        !('seo' in response && 'accessibility' in response && 'performance' in response && 'page' in response)) {
        throw new Error('workspace status unavailable');
    }
    return response;
}
export async function getWorkspaceStatusSnapshot(options = {}) {
    const tab = await getWorkspaceHostTab(options.tabId);
    const recordingState = (await getLocal(StorageKey.RECORDING));
    const mode = options.mode || 'live';
    return buildWorkspaceStatusSnapshot({
        mode,
        tab,
        recordingState,
        queryContentStatus: async () => {
            if (tab.id === undefined)
                throw new Error('missing workspace tab');
            return queryWorkspaceStatusFromContent(tab.id);
        }
    });
}
//# sourceMappingURL=workspace-status.js.map