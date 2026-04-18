/**
 * Purpose: Builds the terminal pane used inside the workspace sidebar shell.
 * Why: Keeps the xterm host, header controls, and iframe mounting isolated from workspace chrome.
 * Docs: docs/features/feature/terminal/index.md
 */
import { DISCONNECT_TERMINAL_BUTTON_ID, HEADER_ID, IFRAME_ID, MINIMIZE_TERMINAL_BUTTON_ID, REDRAW_TERMINAL_BUTTON_ID, getTerminalServerUrl } from '../content/ui/terminal-widget-types.js';
function createTerminalHeader(options) {
    const header = document.createElement('div');
    header.id = HEADER_ID;
    Object.assign(header.style, {
        height: '38px',
        background: '#16161e',
        display: 'flex',
        alignItems: 'center',
        padding: '0 10px 0 12px',
        gap: '8px',
        borderBottom: '1px solid #292e42',
        flexShrink: '0'
    });
    const statusDotEl = document.createElement('span');
    statusDotEl.className = 'kaboom-terminal-status-dot';
    Object.assign(statusDotEl.style, {
        width: '8px',
        height: '8px',
        borderRadius: '50%',
        background: '#565f89',
        flexShrink: '0',
        transition: 'background 200ms ease'
    });
    const titleSpan = document.createElement('span');
    titleSpan.textContent = 'KaBOOM! Workspace';
    Object.assign(titleSpan.style, {
        color: '#d8dee9',
        fontSize: '12px',
        fontWeight: '600',
        overflow: 'hidden',
        textOverflow: 'ellipsis',
        whiteSpace: 'nowrap',
        userSelect: 'none'
    });
    const spacer = document.createElement('div');
    spacer.style.flex = '1';
    const disconnectButton = document.createElement('button');
    disconnectButton.id = DISCONNECT_TERMINAL_BUTTON_ID;
    disconnectButton.textContent = '\u23FB';
    disconnectButton.title = 'Disconnect terminal & end session';
    disconnectButton.type = 'button';
    Object.assign(disconnectButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#f7768e',
        fontSize: '12px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    disconnectButton.addEventListener('click', options.onDisconnect);
    const redrawButton = document.createElement('button');
    redrawButton.id = REDRAW_TERMINAL_BUTTON_ID;
    redrawButton.textContent = '\u21BB';
    redrawButton.title = 'Redraw terminal graphics';
    redrawButton.type = 'button';
    Object.assign(redrawButton.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    redrawButton.addEventListener('click', options.onRedraw);
    const minimizeButtonEl = document.createElement('button');
    minimizeButtonEl.id = MINIMIZE_TERMINAL_BUTTON_ID;
    minimizeButtonEl.textContent = '\u2581';
    minimizeButtonEl.title = 'Minimize terminal';
    minimizeButtonEl.type = 'button';
    Object.assign(minimizeButtonEl.style, {
        width: '24px',
        height: '24px',
        border: 'none',
        background: 'transparent',
        color: '#565f89',
        fontSize: '14px',
        cursor: 'pointer',
        borderRadius: '4px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: '0'
    });
    minimizeButtonEl.addEventListener('click', options.onMinimize);
    header.appendChild(statusDotEl);
    header.appendChild(titleSpan);
    header.appendChild(disconnectButton);
    header.appendChild(spacer);
    header.appendChild(redrawButton);
    header.appendChild(minimizeButtonEl);
    return { headerEl: header, statusDotEl, minimizeButtonEl };
}
export function createWorkspaceTerminalPane(options) {
    const shellEl = document.createElement('div');
    shellEl.style.cssText = [
        'flex:1 1 auto',
        'height:100%',
        'min-height:0',
        'display:flex',
        'flex-direction:column',
        'background:#11131a',
        'border:1px solid #292e42',
        'border-radius:12px',
        'overflow:hidden'
    ].join(';');
    const { headerEl, statusDotEl, minimizeButtonEl } = createTerminalHeader(options);
    const bodyEl = document.createElement('div');
    bodyEl.style.cssText = [
        'flex:1',
        'min-height:0',
        'display:block',
        'background:#1a1b26'
    ].join(';');
    let iframeEl = null;
    if (options.token) {
        iframeEl = document.createElement('iframe');
        iframeEl.id = IFRAME_ID;
        iframeEl.src = `${getTerminalServerUrl(options.serverUrl)}/terminal?token=${encodeURIComponent(options.token)}`;
        iframeEl.setAttribute('allow', 'clipboard-write');
        iframeEl.style.cssText = 'width:100%;height:100%;border:none;background:#1a1b26;display:block;';
        bodyEl.appendChild(iframeEl);
    }
    shellEl.appendChild(headerEl);
    shellEl.appendChild(bodyEl);
    return { shellEl, bodyEl, iframeEl, statusDotEl, minimizeButtonEl };
}
//# sourceMappingURL=workspace-terminal-pane.js.map