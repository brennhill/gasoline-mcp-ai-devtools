/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 */

import type { PendingQuery } from '../types';
import * as communication from './communication';
import * as eventListeners from './event-listeners';
import * as index from './index';
import { DebugCategory } from './debug';
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './message-handlers';

// Extract values from index for easier reference (but NOT DebugCategory - imported directly above)
const { debugLog, diagnosticLog } = index;

// =============================================================================
// PENDING QUERY HANDLING
// =============================================================================

export async function handlePendingQuery(query: PendingQuery): Promise<void> {
  try {
    if (query.type.startsWith('state_')) {
      await handleStateQuery(query);
      return;
    }

    const storage = await eventListeners.getTrackedTabInfo();
    let tabId: number | undefined;

    if (storage.trackedTabId) {
      diagnosticLog(`[Diagnostic] Using tracked tab ${storage.trackedTabId} for query ${query.type}`);
      try {
        await chrome.tabs.get(storage.trackedTabId);
        tabId = storage.trackedTabId;
      } catch {
        diagnosticLog(`[Diagnostic] Tracked tab ${storage.trackedTabId} no longer exists, clearing tracking`);
        eventListeners.clearTrackedTab();
        const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstActiveTab = activeTabs[0];
        if (!firstActiveTab?.id) return;
        tabId = firstActiveTab.id;
      }
    } else {
      const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true });
      const firstActiveTab = activeTabs[0];
      if (!firstActiveTab?.id) return;
      tabId = firstActiveTab.id;
    }

    if (!tabId) return;

    if (query.type === 'browser_action') {
      const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
      if (query.correlation_id) {
        await handleAsyncBrowserAction(query, tabId, params);
      } else {
        const result = await handleBrowserAction(tabId, params);
        await communication.postQueryResult(index.serverUrl, query.id, 'browser_action', result);
      }
      return;
    }

    if (query.type === 'highlight') {
      const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
      const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params);
      await communication.postQueryResult(index.serverUrl, query.id, 'highlight', result);
      return;
    }

    if (query.type === 'page_info') {
      const tab = await chrome.tabs.get(tabId);
      const result = {
        url: tab.url,
        title: tab.title,
        favicon: tab.favIconUrl,
        status: tab.status,
        viewport: {
          width: tab.width,
          height: tab.height,
        },
      };
      await communication.postQueryResult(index.serverUrl, query.id, 'page_info', result);
      return;
    }

    if (query.type === 'tabs') {
      const allTabs = await chrome.tabs.query({});
      const tabsList = allTabs.map((tab) => ({
        id: tab.id,
        url: tab.url,
        title: tab.title,
        active: tab.active,
        windowId: tab.windowId,
        index: tab.index,
      }));
      await communication.postQueryResult(index.serverUrl, query.id, 'dom', { tabs: tabsList });
      return;
    }

    if (query.type === 'dom') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'DOM_QUERY',
          params: query.params,
        });
        await communication.postQueryResult(index.serverUrl, query.id, 'dom', result);
      } catch (err) {
        await communication.postQueryResult(index.serverUrl, query.id, 'dom', {
          error: 'dom_query_failed',
          message: (err as Error).message || 'Failed to execute DOM query',
        });
      }
      return;
    }

    if (query.type === 'a11y') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'A11Y_QUERY',
          params: query.params,
        });
        await communication.postQueryResult(index.serverUrl, query.id, 'a11y', result);
      } catch (err) {
        await communication.postQueryResult(index.serverUrl, query.id, 'a11y', {
          error: 'a11y_audit_failed',
          message: (err as Error).message || 'Failed to execute accessibility audit',
        });
      }
      return;
    }

    if (query.type === 'execute') {
      if (!index.__aiWebPilotEnabledCache) {
        if (query.correlation_id) {
          await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled');
        } else {
          await communication.postQueryResult(index.serverUrl, query.id, 'execute', {
            success: false,
            error: 'ai_web_pilot_disabled',
            message: 'AI Web Pilot is not enabled in the extension popup',
          });
        }
        return;
      }

      if (query.correlation_id) {
        await handleAsyncExecuteCommand(query, tabId);
      } else {
        try {
          const result = await chrome.tabs.sendMessage(tabId, {
            type: 'GASOLINE_EXECUTE_QUERY',
            queryId: query.id,
            params: query.params,
          });
          await communication.postQueryResult(index.serverUrl, query.id, 'execute', result);
        } catch (err) {
          let message = (err as Error).message || 'Tab communication failed';
          if (message.includes('Receiving end does not exist')) {
            message = 'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({action: "refresh"})\n\nThen retry your command.';
          }
          await communication.postQueryResult(index.serverUrl, query.id, 'execute', {
            success: false,
            error: 'content_script_not_loaded',
            message,
          });
        }
      }
      return;
    }
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
      type: query.type,
      id: query.id,
      error: (err as Error).message,
    });
  }
}

async function handleStateQuery(query: PendingQuery): Promise<void> {
  if (!index.__aiWebPilotEnabledCache) {
    await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: 'ai_web_pilot_disabled' });
    return;
  }

  const params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params;
  const action = params.action as string;

  try {
    let result: unknown;

    switch (action) {
      case 'capture': {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        if (!firstTab?.id) {
          await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: 'no_active_tab' });
          return;
        }
        result = await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' },
        });
        break;
      }

      case 'save': {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        if (!firstTab?.id) {
          await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: 'no_active_tab' });
          return;
        }
        const captureResult = await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' },
        }) as { error?: string } & { url: string; timestamp: number; localStorage: Record<string, string>; sessionStorage: Record<string, string>; cookies: string };
        if (captureResult.error) {
          await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: captureResult.error });
          return;
        }
        result = await saveStateSnapshot(params.name as string, captureResult);
        break;
      }

      case 'load': {
        const snapshot = await loadStateSnapshot(params.name as string);
        if (!snapshot) {
          await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: `Snapshot '${params.name}' not found` });
          return;
        }
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        const firstTab = tabs[0];
        if (!firstTab?.id) {
          await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: 'no_active_tab' });
          return;
        }
        result = await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: {
            action: 'restore',
            state: snapshot,
            include_url: params.include_url !== false,
          },
        });
        break;
      }

      case 'list':
        result = { snapshots: await listStateSnapshots() };
        break;

      case 'delete':
        result = await deleteStateSnapshot(params.name as string);
        break;

      default:
        result = { error: `Unknown action: ${action}` };
    }

    await communication.postQueryResult(index.serverUrl, query.id, 'state', result);
  } catch (err) {
    await communication.postQueryResult(index.serverUrl, query.id, 'state', { error: (err as Error).message });
  }
}

async function handleBrowserAction(tabId: number, params: { action?: string; url?: string }): Promise<{
  success: boolean;
  action?: string;
  url?: string;
  content_script_status?: string;
  message?: string;
  error?: string;
}> {
  const { action, url } = params || {};

  if (!index.__aiWebPilotEnabledCache) {
    return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' };
  }

  try {
    switch (action) {
      case 'refresh':
        await chrome.tabs.reload(tabId);
        await eventListeners.waitForTabLoad(tabId);
        return { success: true, action: 'refresh' };

      case 'navigate': {
        if (!url) {
          return { success: false, error: 'missing_url', message: 'URL required for navigate action' };
        }

        if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
          return {
            success: false,
            error: 'restricted_url',
            message: 'Cannot navigate to Chrome internal pages',
          };
        }

        await chrome.tabs.update(tabId, { url });
        await eventListeners.waitForTabLoad(tabId);
        await new Promise((r) => setTimeout(r, 500));

        const contentScriptLoaded = await eventListeners.pingContentScript(tabId);

        if (contentScriptLoaded) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'loaded',
            message: 'Content script ready',
          };
        }

        const tab = await chrome.tabs.get(tabId);
        if (tab.url?.startsWith('file://')) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'unavailable',
            message: 'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings.',
          };
        }

        debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url });
        await chrome.tabs.reload(tabId);
        await eventListeners.waitForTabLoad(tabId);
        await new Promise((r) => setTimeout(r, 1000));

        const loadedAfterRefresh = await eventListeners.pingContentScript(tabId);

        if (loadedAfterRefresh) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'refreshed',
            message: 'Page refreshed to load content script',
          };
        }

        return {
          success: true,
          action: 'navigate',
          url,
          content_script_status: 'failed',
          message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.',
        };
      }

      case 'back':
        await chrome.tabs.goBack(tabId);
        return { success: true, action: 'back' };

      case 'forward':
        await chrome.tabs.goForward(tabId);
        return { success: true, action: 'forward' };

      default:
        return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` };
    }
  } catch (err) {
    return { success: false, error: 'browser_action_failed', message: (err as Error).message };
  }
}

async function handleAsyncExecuteCommand(query: PendingQuery, tabId: number): Promise<void> {
  const startTime = Date.now();
  let completed = false;
  let pendingPosted = false;

  type ExecSuccess = { success: true; result: unknown };
  type ExecFailure = { success: false; error: string; message: string };
  type ExecResult = ExecSuccess | ExecFailure;

  const executionPromise: Promise<ExecResult> = chrome.tabs
    .sendMessage(tabId, {
      type: 'GASOLINE_EXECUTE_QUERY',
      queryId: query.id,
      params: query.params,
    })
    .then((result): ExecSuccess => {
      completed = true;
      return { success: true, result };
    })
    .catch((err: Error): ExecFailure => {
      completed = true;
      let message = err.message || 'Tab communication failed';
      if (message.includes('Receiving end does not exist')) {
        message = 'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({action: "refresh"})\n\nThen retry your command.';
      }
      return {
        success: false,
        error: 'content_script_not_loaded',
        message,
      };
    });

  const pendingTimer = setTimeout(async () => {
    if (!completed && !pendingPosted) {
      pendingPosted = true;
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'pending');
      debugLog(DebugCategory.CONNECTION, 'Posted pending status for async command', {
        correlationId: query.correlation_id,
        elapsed: Date.now() - startTime,
      });
    }
  }, 3000);

  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), 10000);
      }),
    ]);

    clearTimeout(pendingTimer);

    if (execResult.success) {
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'complete', execResult.result);
    } else {
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'complete', null, execResult.error || execResult.message);
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async command', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success,
    });
  } catch {
    clearTimeout(pendingTimer);

    const timeoutMessage = `JavaScript execution exceeded 10s timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < 2s for best results
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope`;

    await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'timeout', null, timeoutMessage);

    debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    });
  }
}

async function handleAsyncBrowserAction(query: PendingQuery, tabId: number, params: { action?: string; url?: string }): Promise<void> {
  const startTime = Date.now();
  let completed = false;
  let pendingPosted = false;

  const executionPromise = handleBrowserAction(tabId, params)
    .then((result) => {
      completed = true;
      return result;
    })
    .catch((err: Error) => {
      completed = true;
      return {
        success: false,
        error: err.message || 'Browser action failed',
      };
    });

  const pendingTimer = setTimeout(async () => {
    if (!completed && !pendingPosted) {
      pendingPosted = true;
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'pending');
      debugLog(DebugCategory.CONNECTION, 'Posted pending status for async browser action', {
        correlationId: query.correlation_id,
        elapsed: Date.now() - startTime,
      });
    }
  }, 3000);

  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), 10000);
      }),
    ]);

    clearTimeout(pendingTimer);

    if (execResult.success !== false) {
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'complete', execResult);
    } else {
      await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'complete', null, execResult.error);
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success !== false,
    });
  } catch {
    clearTimeout(pendingTimer);

    const timeoutMessage = `Browser action exceeded 10s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network', status_min: 400})`;

    await communication.postAsyncCommandResult(index.serverUrl, query.correlation_id!, 'timeout', null, timeoutMessage);

    debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    });
  }
}

export async function handlePilotCommand(command: string, params: unknown): Promise<unknown> {
  if (!index.__aiWebPilotEnabledCache) {
    if (typeof chrome !== 'undefined' && chrome.storage) {
      const localResult = await new Promise<{ aiWebPilotEnabled?: boolean }>((resolve) => {
        chrome.storage.local.get(['aiWebPilotEnabled'], (result: { aiWebPilotEnabled?: boolean }) => {
          resolve(result);
        });
      });
      if (localResult.aiWebPilotEnabled === true) {
        // Update cache (note: this module imports from index.ts which has the state)
        // We can't directly update it, so we return the error
      }
    }
  }

  if (!index.__aiWebPilotEnabledCache) {
    return { error: 'ai_web_pilot_disabled' };
  }

  try {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const firstTab = tabs[0];

    if (!firstTab?.id) {
      return { error: 'no_active_tab' };
    }

    const tabId = firstTab.id;

    const result = await chrome.tabs.sendMessage(tabId, {
      type: command,
      params,
    });

    return result || { success: true };
  } catch (err) {
    return { error: (err as Error).message || 'command_failed' };
  }
}
