/**
 * Purpose: Renders workspace QA summary and status regions from typed snapshots.
 * Why: Keeps sidepanel display logic separate from terminal-session orchestration.
 * Docs: docs/features/feature/terminal/index.md
 */
function createChip(label, value) {
    const chip = document.createElement('div');
    Object.assign(chip.style, {
        display: 'inline-flex',
        alignItems: 'center',
        gap: '6px',
        padding: '8px 10px',
        borderRadius: '999px',
        background: '#16161e',
        border: '1px solid #292e42',
        color: '#c0caf5',
        fontSize: '12px',
        whiteSpace: 'nowrap'
    });
    const labelEl = document.createElement('span');
    labelEl.textContent = label;
    labelEl.style.color = '#7aa2f7';
    const valueEl = document.createElement('span');
    valueEl.textContent = value;
    chip.appendChild(labelEl);
    chip.appendChild(valueEl);
    return chip;
}
function createStatusLine(text, muted = false) {
    const line = document.createElement('div');
    line.textContent = text;
    Object.assign(line.style, {
        fontSize: '12px',
        color: muted ? '#787c99' : '#a9b1d6'
    });
    return line;
}
function metricValue(score) {
    return score === null ? 'unavailable' : String(score);
}
function performanceValue(snapshot) {
    return snapshot.performance.verdict.replace('_', ' ');
}
export function renderWorkspaceStatus(summaryStripEl, statusAreaEl, snapshot, contextMessage = null) {
    summaryStripEl.replaceChildren(createChip('SEO', metricValue(snapshot.seo.score)), createChip('Accessibility', metricValue(snapshot.accessibility.score)), createChip('Performance', performanceValue(snapshot)), createChip('Recording', snapshot.session.recording_active ? 'active' : 'idle'));
    const updatedAt = snapshot.audit.updated_at ? snapshot.audit.updated_at.slice(0, 10) : 'not yet run';
    statusAreaEl.replaceChildren(createStatusLine(snapshot.page.summary || snapshot.page.title || snapshot.page.url), createStatusLine(`Recording: ${snapshot.session.recording_active ? 'active' : 'idle'} • Screenshots: ${snapshot.session.screenshot_count} • Notes: ${snapshot.session.note_count}`), createStatusLine(`Latest audit: ${updatedAt}`), createStatusLine(snapshot.recommendation, true), ...(contextMessage ? [createStatusLine(contextMessage)] : []));
}
//# sourceMappingURL=workspace-status.js.map