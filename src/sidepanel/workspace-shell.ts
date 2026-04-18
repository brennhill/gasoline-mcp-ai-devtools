/**
 * Purpose: Builds the QA workspace shell that wraps the existing terminal pane.
 * Why: Keeps the sidepanel layout and placeholder workspace chrome separate from terminal session logic.
 * Docs: docs/features/feature/terminal/index.md
 */

import { WIDGET_ID } from '../content/ui/terminal-widget-types.js'

export const WORKSPACE_SUMMARY_STRIP_ID = 'kaboom-workspace-summary-strip'
export const WORKSPACE_ACTION_ROW_ID = 'kaboom-workspace-action-row'
export const WORKSPACE_TERMINAL_REGION_ID = 'kaboom-workspace-terminal-region'
export const WORKSPACE_STATUS_AREA_ID = 'kaboom-workspace-status-area'

export interface WorkspaceShellElements {
  readonly rootEl: HTMLDivElement
  readonly terminalRegionEl: HTMLDivElement
  readonly summaryStripEl: HTMLDivElement
  readonly actionRowEl: HTMLDivElement
  readonly statusAreaEl: HTMLDivElement
}

export interface WorkspaceShellActions {
  readonly onToggleRecording: () => void
  readonly onScreenshot: () => void
  readonly onRunAudit: () => void
  readonly onAddNote: () => void
  readonly onInjectContext: () => void
  readonly onResetWorkspace: () => void
}

function createMetricChip(label: string, value: string): HTMLDivElement {
  const chip = document.createElement('div')
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
  })

  const labelEl = document.createElement('span')
  labelEl.textContent = label
  labelEl.style.color = '#7aa2f7'

  const valueEl = document.createElement('span')
  valueEl.textContent = value

  chip.appendChild(labelEl)
  chip.appendChild(valueEl)
  return chip
}

function createActionButton(label: string, onClick: () => void): HTMLButtonElement {
  const button = document.createElement('button')
  button.type = 'button'
  button.textContent = label
  Object.assign(button.style, {
    border: '1px solid #2f354d',
    background: '#16161e',
    color: '#d8dee9',
    borderRadius: '8px',
    padding: '8px 10px',
    fontSize: '12px',
    cursor: 'pointer'
  })
  button.addEventListener('click', (event) => {
    event.preventDefault()
    event.stopPropagation()
    onClick()
  })
  return button
}

export function createWorkspaceShell(terminalPaneEl: HTMLDivElement, actions: WorkspaceShellActions): WorkspaceShellElements {
  const rootEl = document.createElement('div')
  rootEl.id = WIDGET_ID
  Object.assign(rootEl.style, {
    position: 'fixed',
    inset: '0',
    zIndex: '2147483644',
    display: 'flex',
    flexDirection: 'column',
    gap: '12px',
    padding: '12px',
    background: '#0f1117',
    color: '#e5e7eb',
    opacity: '1',
    pointerEvents: 'auto',
    transition: 'opacity 180ms ease'
  })

  const summaryStripEl = document.createElement('div')
  summaryStripEl.id = WORKSPACE_SUMMARY_STRIP_ID
  Object.assign(summaryStripEl.style, {
    display: 'flex',
    gap: '8px',
    flexWrap: 'wrap',
    flexShrink: '0'
  })
  summaryStripEl.appendChild(createMetricChip('SEO', 'unavailable'))
  summaryStripEl.appendChild(createMetricChip('Accessibility', 'unavailable'))
  summaryStripEl.appendChild(createMetricChip('Performance', 'not measured'))
  summaryStripEl.appendChild(createMetricChip('Session', '0 screenshots'))

  const actionRowEl = document.createElement('div')
  actionRowEl.id = WORKSPACE_ACTION_ROW_ID
  Object.assign(actionRowEl.style, {
    display: 'flex',
    gap: '8px',
    flexWrap: 'wrap',
    flexShrink: '0'
  })
  actionRowEl.appendChild(createActionButton('Record', actions.onToggleRecording))
  actionRowEl.appendChild(createActionButton('Screenshot', actions.onScreenshot))
  actionRowEl.appendChild(createActionButton('Run audit', actions.onRunAudit))
  actionRowEl.appendChild(createActionButton('Add note', actions.onAddNote))
  actionRowEl.appendChild(createActionButton('Inject context', actions.onInjectContext))
  actionRowEl.appendChild(createActionButton('Reset workspace', actions.onResetWorkspace))

  const terminalRegionEl = document.createElement('div')
  terminalRegionEl.id = WORKSPACE_TERMINAL_REGION_ID
  Object.assign(terminalRegionEl.style, {
    flex: '1 1 auto',
    minHeight: '0',
    display: 'flex'
  })
  terminalRegionEl.appendChild(terminalPaneEl)

  const statusAreaEl = document.createElement('div')
  statusAreaEl.id = WORKSPACE_STATUS_AREA_ID
  Object.assign(statusAreaEl.style, {
    flexShrink: '0',
    borderRadius: '12px',
    border: '1px solid #292e42',
    background: '#16161e',
    padding: '12px',
    color: '#a9b1d6',
    fontSize: '12px'
  })
  statusAreaEl.textContent = 'Workspace status will appear here once the page is connected.'

  rootEl.appendChild(summaryStripEl)
  rootEl.appendChild(actionRowEl)
  rootEl.appendChild(terminalRegionEl)
  rootEl.appendChild(statusAreaEl)

  return { rootEl, terminalRegionEl, summaryStripEl, actionRowEl, statusAreaEl }
}
