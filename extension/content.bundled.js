"use strict";
(() => {
  // extension/content/tab-tracking.js
  var isTrackedTab = false;
  var currentTabId = null;
  async function updateTrackingStatus() {
    try {
      const storage = await chrome.storage.local.get(["trackedTabId"]);
      const response = await chrome.runtime.sendMessage({ type: "GET_TAB_ID" });
      currentTabId = response?.tabId ?? null;
      isTrackedTab = currentTabId !== null && currentTabId !== void 0 && currentTabId === storage.trackedTabId;
    } catch {
      isTrackedTab = false;
    }
  }
  function getIsTrackedTab() {
    return isTrackedTab;
  }
  function getCurrentTabId() {
    return currentTabId;
  }
  function initTabTracking(onChange) {
    const ready = updateTrackingStatus().then(() => {
      onChange?.(isTrackedTab);
    });
    chrome.storage.onChanged.addListener(async (changes) => {
      if (changes.trackedTabId) {
        await updateTrackingStatus();
        onChange?.(isTrackedTab);
      }
    });
    return ready;
  }

  // extension/content/script-injection.js
  var injected = false;
  function isInjectScriptLoaded() {
    return injected;
  }
  var SYNC_SETTINGS = [
    { storageKey: "webSocketCaptureEnabled", messageType: "setWebSocketCaptureEnabled" },
    { storageKey: "webSocketCaptureMode", messageType: "setWebSocketCaptureMode", isMode: true },
    { storageKey: "networkWaterfallEnabled", messageType: "setNetworkWaterfallEnabled" },
    { storageKey: "performanceMarksEnabled", messageType: "setPerformanceMarksEnabled" },
    { storageKey: "actionReplayEnabled", messageType: "setActionReplayEnabled" },
    { storageKey: "networkBodyCaptureEnabled", messageType: "setNetworkBodyCaptureEnabled" }
  ];
  function syncStoredSettings() {
    const storageKeys = SYNC_SETTINGS.map((s) => s.storageKey);
    chrome.storage.local.get(storageKeys, (result) => {
      for (const setting of SYNC_SETTINGS) {
        const value = result[setting.storageKey];
        if (value === void 0)
          continue;
        if (setting.isMode) {
          window.postMessage({ type: "GASOLINE_SETTING", setting: setting.messageType, mode: value }, window.location.origin);
        } else {
          window.postMessage({ type: "GASOLINE_SETTING", setting: setting.messageType, enabled: value }, window.location.origin);
        }
      }
    });
  }
  function injectAxeCore() {
    const script = document.createElement("script");
    script.src = chrome.runtime.getURL("lib/axe.min.js");
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
  }
  function injectScript() {
    const script = document.createElement("script");
    script.src = chrome.runtime.getURL("inject.bundled.js");
    script.type = "module";
    script.onload = () => {
      script.remove();
      injected = true;
      setTimeout(syncStoredSettings, 50);
    };
    (document.head || document.documentElement).appendChild(script);
  }
  function initScriptInjection() {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", () => {
        injectAxeCore();
        injectScript();
      }, { once: true });
    } else {
      injectAxeCore();
      injectScript();
    }
  }

  // extension/content/request-tracking.js
  var pendingHighlightRequests = /* @__PURE__ */ new Map();
  var highlightRequestId = 0;
  var pendingExecuteRequests = /* @__PURE__ */ new Map();
  var executeRequestId = 0;
  var pendingA11yRequests = /* @__PURE__ */ new Map();
  var a11yRequestId = 0;
  var pendingDomRequests = /* @__PURE__ */ new Map();
  var domRequestId = 0;
  var CLEANUP_INTERVAL_MS = 3e4;
  var cleanupTimer = null;
  var requestTimestamps = /* @__PURE__ */ new Map();
  function getRequestTimestamps() {
    const timestamps = [];
    for (const [id, timestamp] of requestTimestamps) {
      timestamps.push([id, timestamp]);
    }
    return timestamps;
  }
  function clearPendingRequests() {
    pendingHighlightRequests.clear();
    pendingExecuteRequests.clear();
    pendingA11yRequests.clear();
    pendingDomRequests.clear();
    requestTimestamps.clear();
  }
  function performPeriodicCleanup() {
    const now = Date.now();
    const staleThreshold = 6e4;
    for (const [id, timestamp] of getRequestTimestamps()) {
      if (now - timestamp > staleThreshold) {
        pendingHighlightRequests.delete(id);
        pendingExecuteRequests.delete(id);
        pendingA11yRequests.delete(id);
        pendingDomRequests.delete(id);
        requestTimestamps.delete(id);
      }
    }
  }
  function getPendingRequestStats() {
    return {
      highlight: pendingHighlightRequests.size,
      execute: pendingExecuteRequests.size,
      a11y: pendingA11yRequests.size,
      dom: pendingDomRequests.size
    };
  }
  function registerHighlightRequest(resolve) {
    const requestId = ++highlightRequestId;
    pendingHighlightRequests.set(requestId, resolve);
    return requestId;
  }
  function resolveHighlightRequest(requestId, result) {
    const resolve = pendingHighlightRequests.get(requestId);
    if (resolve) {
      pendingHighlightRequests.delete(requestId);
      resolve(result);
    }
  }
  function hasHighlightRequest(requestId) {
    return pendingHighlightRequests.has(requestId);
  }
  function deleteHighlightRequest(requestId) {
    pendingHighlightRequests.delete(requestId);
  }
  function registerExecuteRequest(resolve) {
    const requestId = ++executeRequestId;
    pendingExecuteRequests.set(requestId, resolve);
    return requestId;
  }
  function resolveExecuteRequest(requestId, result) {
    const resolve = pendingExecuteRequests.get(requestId);
    if (resolve) {
      pendingExecuteRequests.delete(requestId);
      resolve(result);
    }
  }
  function registerA11yRequest(resolve) {
    const requestId = ++a11yRequestId;
    pendingA11yRequests.set(requestId, resolve);
    return requestId;
  }
  function resolveA11yRequest(requestId, result) {
    const resolve = pendingA11yRequests.get(requestId);
    if (resolve) {
      pendingA11yRequests.delete(requestId);
      resolve(result);
    }
  }
  function registerDomRequest(resolve) {
    const requestId = ++domRequestId;
    pendingDomRequests.set(requestId, resolve);
    return requestId;
  }
  function resolveDomRequest(requestId, result) {
    const resolve = pendingDomRequests.get(requestId);
    if (resolve) {
      pendingDomRequests.delete(requestId);
      resolve(result);
    }
  }
  function cleanupRequestTracking() {
    if (cleanupTimer) {
      clearInterval(cleanupTimer);
      cleanupTimer = null;
    }
    clearPendingRequests();
  }
  function initRequestTracking() {
    window.addEventListener("pagehide", clearPendingRequests);
    window.addEventListener("beforeunload", clearPendingRequests);
    cleanupTimer = setInterval(performPeriodicCleanup, CLEANUP_INTERVAL_MS);
  }

  // extension/content/message-forwarding.js
  var MESSAGE_MAP = {
    GASOLINE_LOG: "log",
    GASOLINE_WS: "ws_event",
    GASOLINE_NETWORK_BODY: "network_body",
    GASOLINE_ENHANCED_ACTION: "enhanced_action",
    GASOLINE_PERFORMANCE_SNAPSHOT: "performance_snapshot"
  };
  var contextValid = true;
  function safeSendMessage(msg) {
    if (!contextValid)
      return;
    try {
      chrome.runtime.sendMessage(msg);
    } catch (e) {
      if (e instanceof Error && e.message?.includes("Extension context invalidated")) {
        contextValid = false;
        console.warn("[Gasoline] Please refresh this page. The Gasoline extension was reloaded and this page still has the old content script. A page refresh will reconnect capture automatically.");
      }
    }
  }

  // extension/content/window-message-listener.js
  function initWindowMessageListener() {
    window.addEventListener("message", (event) => {
      if (event.source !== window || event.origin !== window.location.origin)
        return;
      const { type: messageType, requestId, result, payload } = event.data || {};
      if (messageType === "GASOLINE_HIGHLIGHT_RESPONSE") {
        if (requestId !== void 0) {
          resolveHighlightRequest(requestId, result);
        }
        return;
      }
      if (messageType === "GASOLINE_EXECUTE_JS_RESULT") {
        if (requestId !== void 0) {
          resolveExecuteRequest(requestId, result);
        }
        return;
      }
      if (messageType === "GASOLINE_A11Y_QUERY_RESPONSE") {
        if (requestId !== void 0) {
          resolveA11yRequest(requestId, result);
        }
        return;
      }
      if (messageType === "GASOLINE_DOM_QUERY_RESPONSE") {
        if (requestId !== void 0) {
          resolveDomRequest(requestId, result);
        }
        return;
      }
      if (!getIsTrackedTab()) {
        return;
      }
      if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === "object") {
        const mappedType = MESSAGE_MAP[messageType];
        if (mappedType) {
          safeSendMessage({
            type: mappedType,
            payload,
            tabId: getCurrentTabId()
          });
        }
      }
    });
  }

  // extension/content/timeout-utils.js
  var TimeoutError = class extends Error {
    fallback;
    constructor(message, fallback) {
      super(message);
      this.fallback = fallback;
      this.name = "TimeoutError";
    }
  };
  function createDeferredPromise() {
    let resolve;
    let reject;
    const promise = new Promise((res, rej) => {
      resolve = res;
      reject = rej;
    });
    return { promise, resolve, reject };
  }
  async function promiseRaceWithCleanup(promise, timeoutMs, timeoutFallback, cleanup) {
    try {
      return await Promise.race([
        promise,
        new Promise((_, reject) => setTimeout(() => {
          cleanup?.();
          if (timeoutFallback !== void 0) {
            reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`, timeoutFallback));
          } else {
            reject(new TimeoutError(`Operation timed out after ${timeoutMs}ms`));
          }
        }, timeoutMs))
      ]);
    } catch (err) {
      if (err instanceof TimeoutError && err.fallback !== void 0) {
        return err.fallback;
      }
      throw err;
    }
  }

  // extension/content/message-handlers.js
  var TOGGLE_MESSAGES = /* @__PURE__ */ new Set([
    "setNetworkWaterfallEnabled",
    "setPerformanceMarksEnabled",
    "setActionReplayEnabled",
    "setWebSocketCaptureEnabled",
    "setWebSocketCaptureMode",
    "setPerformanceSnapshotEnabled",
    "setDeferralEnabled",
    "setNetworkBodyCaptureEnabled",
    "setServerUrl"
  ]);
  function isValidBackgroundSender(sender) {
    return typeof sender.id === "string" && sender.id === chrome.runtime.id;
  }
  function createRequestTimeoutCleanup(requestId, pendingMap, errorResponse) {
    return () => {
      if (pendingMap.has(requestId)) {
        const cb = pendingMap.get(requestId);
        pendingMap.delete(requestId);
        if (cb) {
          cb(errorResponse);
        }
      }
    };
  }
  function forwardHighlightMessage(message) {
    const requestId = registerHighlightRequest((result) => deferred.resolve(result));
    const deferred = createDeferredPromise();
    window.postMessage({
      type: "GASOLINE_HIGHLIGHT_REQUEST",
      requestId,
      params: message.params
    }, window.location.origin);
    return promiseRaceWithCleanup(deferred.promise, 3e4, { success: false, error: "timeout" }, () => {
      if (hasHighlightRequest(requestId)) {
        deleteHighlightRequest(requestId);
      }
    });
  }
  async function handleStateCommand(params) {
    const { action, name, state, include_url } = params || {};
    const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`;
    const deferred = createDeferredPromise();
    const responseHandler = (event) => {
      if (event.source !== window)
        return;
      if (event.data?.type === "GASOLINE_STATE_RESPONSE" && event.data?.messageId === messageId) {
        window.removeEventListener("message", responseHandler);
        deferred.resolve(event.data.result || { error: "No result from state command" });
      }
    };
    window.addEventListener("message", responseHandler);
    window.postMessage({
      type: "GASOLINE_STATE_COMMAND",
      messageId,
      action,
      name,
      state,
      include_url
    }, window.location.origin);
    return promiseRaceWithCleanup(deferred.promise, 5e3, { error: "State command timeout" }, () => window.removeEventListener("message", responseHandler));
  }
  function handlePing(sendResponse) {
    sendResponse({ status: "alive", timestamp: Date.now() });
    return true;
  }
  function handleToggleMessage(message) {
    if (!TOGGLE_MESSAGES.has(message.type))
      return;
    const payload = { type: "GASOLINE_SETTING", setting: message.type };
    if (message.type === "setWebSocketCaptureMode") {
      payload.mode = message.mode;
    } else if (message.type === "setServerUrl") {
      payload.url = message.url;
    } else {
      payload.enabled = message.enabled;
    }
    window.postMessage(payload, window.location.origin);
  }
  function executeInMainWorld(params, sendResponse) {
    const timeoutMs = params.timeout_ms || 5e3;
    const requestId = registerExecuteRequest(sendResponse);
    const safetyTimeoutMs = timeoutMs + 2e3;
    setTimeout(createRequestTimeoutCleanup(requestId, /* @__PURE__ */ new Map([[requestId, sendResponse]]), {
      success: false,
      error: "inject_not_responding",
      message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`
    }), safetyTimeoutMs);
    window.postMessage({
      type: "GASOLINE_EXECUTE_JS",
      requestId,
      script: params.script || "",
      timeoutMs
    }, window.location.origin);
  }
  function handleExecuteJs(params, sendResponse) {
    if (!isInjectScriptLoaded()) {
      sendResponse({
        success: false,
        error: "inject_not_loaded",
        message: "Inject script not loaded in page context. Tab may not be tracked."
      });
      return true;
    }
    executeInMainWorld(params, sendResponse);
    return true;
  }
  function handleExecuteQuery(params, sendResponse) {
    let parsedParams = {};
    if (typeof params === "string") {
      try {
        parsedParams = JSON.parse(params);
      } catch {
        parsedParams = {};
      }
    } else if (typeof params === "object") {
      parsedParams = params;
    }
    return handleExecuteJs(parsedParams, sendResponse);
  }
  function handleA11yQuery(params, sendResponse) {
    let parsedParams = {};
    if (typeof params === "string") {
      try {
        parsedParams = JSON.parse(params);
      } catch {
        parsedParams = {};
      }
    } else if (typeof params === "object") {
      parsedParams = params;
    }
    const requestId = registerA11yRequest(sendResponse);
    setTimeout(createRequestTimeoutCleanup(requestId, /* @__PURE__ */ new Map([[requestId, sendResponse]]), {
      error: "Accessibility audit timeout"
    }), 3e4);
    window.postMessage({
      type: "GASOLINE_A11Y_QUERY",
      requestId,
      params: parsedParams
    }, window.location.origin);
    return true;
  }
  function handleDomQuery(params, sendResponse) {
    let parsedParams = {};
    if (typeof params === "string") {
      try {
        parsedParams = JSON.parse(params);
      } catch {
        parsedParams = {};
      }
    } else if (typeof params === "object") {
      parsedParams = params;
    }
    const requestId = registerDomRequest(sendResponse);
    setTimeout(createRequestTimeoutCleanup(requestId, /* @__PURE__ */ new Map([[requestId, sendResponse]]), { error: "DOM query timeout" }), 3e4);
    window.postMessage({
      type: "GASOLINE_DOM_QUERY",
      requestId,
      params: parsedParams
    }, window.location.origin);
    return true;
  }
  function handleGetNetworkWaterfall(sendResponse) {
    const requestId = Date.now();
    const deferred = createDeferredPromise();
    const responseHandler = (event) => {
      if (event.source !== window)
        return;
      if (event.data?.type === "GASOLINE_WATERFALL_RESPONSE") {
        window.removeEventListener("message", responseHandler);
        deferred.resolve({ entries: event.data.entries || [] });
      }
    };
    window.addEventListener("message", responseHandler);
    window.postMessage({
      type: "GASOLINE_GET_WATERFALL",
      requestId
    }, window.location.origin);
    promiseRaceWithCleanup(deferred.promise, 5e3, { entries: [] }, () => {
      window.removeEventListener("message", responseHandler);
    }).then((result) => {
      sendResponse(result);
    });
    return true;
  }

  // extension/content/runtime-message-listener.js
  var TOAST_THEMES = {
    trying: { bg: "linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)", shadow: "rgba(59, 130, 246, 0.4)" },
    success: { bg: "linear-gradient(135deg, #22c55e 0%, #16a34a 100%)", shadow: "rgba(34, 197, 94, 0.4)" },
    warning: { bg: "linear-gradient(135deg, #f59e0b 0%, #d97706 100%)", shadow: "rgba(245, 158, 11, 0.4)" },
    error: { bg: "linear-gradient(135deg, #ef4444 0%, #dc2626 100%)", shadow: "rgba(239, 68, 68, 0.4)" },
    audio: { bg: "linear-gradient(135deg, #f97316 0%, #ea580c 100%)", shadow: "rgba(249, 115, 22, 0.5)" }
  };
  function injectToastAnimationStyles() {
    if (document.getElementById("gasoline-toast-animations"))
      return;
    const style = document.createElement("style");
    style.id = "gasoline-toast-animations";
    style.textContent = `
    @keyframes gasolineArrowBounce {
      0%, 100% { transform: translateY(0) translateX(0); opacity: 1; }
      50% { transform: translateY(-4px) translateX(4px); opacity: 0.7; }
    }
    @keyframes gasolineArrowBounceUp {
      0%, 100% { transform: translateY(0); opacity: 1; }
      50% { transform: translateY(-6px); opacity: 0.7; }
    }
    @keyframes gasolineToastPulse {
      0%, 100% { box-shadow: 0 4px 20px var(--toast-shadow); }
      50% { box-shadow: 0 8px 32px var(--toast-shadow-intense); }
    }
    .gasoline-toast-arrow {
      display: inline-block;
      margin-left: 8px;
      animation: gasolineArrowBounce 1.5s ease-in-out infinite;
    }
    @media (max-width: 767px) {
      .gasoline-toast-arrow {
        animation: gasolineArrowBounceUp 1.5s ease-in-out infinite;
      }
    }
    .gasoline-toast-pulse {
      animation: gasolineToastPulse 2s ease-in-out infinite;
    }
  `;
    document.head.appendChild(style);
  }
  function truncateText(text, maxLen) {
    if (text.length <= maxLen)
      return text;
    return text.slice(0, maxLen - 1) + "\u2026";
  }
  function showActionToast(text, detail, state = "trying", durationMs = 3e3) {
    const existing = document.getElementById("gasoline-action-toast");
    if (existing)
      existing.remove();
    injectToastAnimationStyles();
    const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying;
    const isAudioPrompt = state === "audio" || detail && detail.toLowerCase().includes("audio") && detail.toLowerCase().includes("click");
    const isSmallScreen = typeof window !== "undefined" && window.innerWidth < 768;
    const arrowChar = isSmallScreen ? "\u2191" : "\u2197";
    const toast = document.createElement("div");
    toast.id = "gasoline-action-toast";
    if (isAudioPrompt) {
      toast.className = "gasoline-toast-pulse";
    }
    const label = document.createElement("span");
    label.textContent = truncateText(text, 30);
    Object.assign(label.style, { fontWeight: "700" });
    toast.appendChild(label);
    if (detail) {
      const sep = document.createElement("span");
      sep.textContent = "  ";
      Object.assign(sep.style, { opacity: "0.6", margin: "0 4px" });
      toast.appendChild(sep);
      const det = document.createElement("span");
      det.textContent = truncateText(detail, 50);
      Object.assign(det.style, { fontWeight: "400", opacity: "0.9" });
      toast.appendChild(det);
    }
    if (isAudioPrompt) {
      const arrow = document.createElement("span");
      arrow.className = "gasoline-toast-arrow";
      arrow.textContent = arrowChar;
      Object.assign(arrow.style, {
        fontSize: "16px",
        fontWeight: "700",
        marginLeft: "12px",
        display: "inline-block"
      });
      toast.appendChild(arrow);
    }
    Object.assign(toast.style, {
      position: "fixed",
      top: "16px",
      right: isAudioPrompt && !isSmallScreen ? "16px" : "auto",
      left: isAudioPrompt && isSmallScreen ? "50%" : isAudioPrompt ? "auto" : "50%",
      transform: isAudioPrompt && isSmallScreen ? "translateX(-50%)" : isAudioPrompt ? "none" : "translateX(-50%)",
      padding: isAudioPrompt ? "12px 24px" : "8px 20px",
      background: theme.bg,
      color: "#fff",
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      fontSize: isAudioPrompt ? "14px" : "13px",
      fontWeight: isAudioPrompt ? "600" : "400",
      borderRadius: "8px",
      boxShadow: `0 4px 20px ${theme.shadow}`,
      zIndex: "2147483647",
      pointerEvents: "none",
      opacity: "0",
      transition: "opacity 0.2s ease-in",
      maxWidth: isAudioPrompt ? "320px" : "500px",
      whiteSpace: isAudioPrompt ? "normal" : "nowrap",
      overflow: isAudioPrompt ? "visible" : "hidden",
      display: "flex",
      alignItems: "center",
      gap: "0",
      "--toast-shadow": theme.shadow,
      "--toast-shadow-intense": theme.shadow.replace("0.4)", "0.7)")
    });
    const target = document.body || document.documentElement;
    if (!target)
      return;
    target.appendChild(toast);
    requestAnimationFrame(() => {
      toast.style.opacity = "1";
    });
    setTimeout(() => {
      toast.style.opacity = "0";
      setTimeout(() => toast.remove(), 300);
    }, durationMs);
  }
  var actionToastsEnabled = true;
  var subtitlesEnabled = true;
  function showSubtitle(text) {
    const ELEMENT_ID = "gasoline-subtitle";
    if (!text) {
      const existing = document.getElementById(ELEMENT_ID);
      if (existing) {
        existing.style.opacity = "0";
        setTimeout(() => existing.remove(), 200);
      }
      return;
    }
    let bar = document.getElementById(ELEMENT_ID);
    if (!bar) {
      bar = document.createElement("div");
      bar.id = ELEMENT_ID;
      Object.assign(bar.style, {
        position: "fixed",
        bottom: "24px",
        left: "50%",
        transform: "translateX(-50%)",
        width: "auto",
        maxWidth: "80%",
        padding: "12px 20px",
        background: "rgba(0, 0, 0, 0.85)",
        color: "#fff",
        fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        fontSize: "16px",
        lineHeight: "1.4",
        textAlign: "center",
        borderRadius: "4px",
        zIndex: "2147483646",
        pointerEvents: "none",
        opacity: "0",
        transition: "opacity 0.2s ease-in",
        maxHeight: "4.2em",
        // ~3 lines
        overflow: "hidden",
        textOverflow: "ellipsis",
        boxSizing: "border-box"
      });
      const target = document.body || document.documentElement;
      if (!target)
        return;
      target.appendChild(bar);
    }
    bar.textContent = text;
    void bar.offsetHeight;
    bar.style.opacity = "1";
  }
  function toggleRecordingWatermark(visible) {
    const ELEMENT_ID = "gasoline-recording-watermark";
    if (!visible) {
      const existing = document.getElementById(ELEMENT_ID);
      if (existing) {
        existing.style.opacity = "0";
        setTimeout(() => existing.remove(), 300);
      }
      return;
    }
    if (document.getElementById(ELEMENT_ID))
      return;
    const container = document.createElement("div");
    container.id = ELEMENT_ID;
    Object.assign(container.style, {
      position: "fixed",
      bottom: "16px",
      right: "16px",
      width: "64px",
      height: "64px",
      opacity: "0",
      transition: "opacity 0.3s ease-in",
      zIndex: "2147483645",
      pointerEvents: "none"
    });
    const img = document.createElement("img");
    img.src = chrome.runtime.getURL("icons/icon.svg");
    Object.assign(img.style, { width: "100%", height: "100%", opacity: "0.5" });
    container.appendChild(img);
    const target = document.body || document.documentElement;
    if (!target)
      return;
    target.appendChild(container);
    void container.offsetHeight;
    container.style.opacity = "1";
  }
  function initRuntimeMessageListener() {
    chrome.storage.local.get(["actionToastsEnabled", "subtitlesEnabled"], (result) => {
      if (result.actionToastsEnabled !== void 0)
        actionToastsEnabled = result.actionToastsEnabled;
      if (result.subtitlesEnabled !== void 0)
        subtitlesEnabled = result.subtitlesEnabled;
    });
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      if (!isValidBackgroundSender(sender)) {
        console.warn("[Gasoline] Rejected message from untrusted sender:", sender.id);
        return false;
      }
      if (message.type === "GASOLINE_PING") {
        return handlePing(sendResponse);
      }
      if (message.type === "GASOLINE_ACTION_TOAST") {
        if (!actionToastsEnabled)
          return false;
        const msg = message;
        if (msg.text)
          showActionToast(msg.text, msg.detail, msg.state || "trying", msg.duration_ms);
        return false;
      }
      if (message.type === "GASOLINE_RECORDING_WATERMARK") {
        const msg = message;
        toggleRecordingWatermark(msg.visible ?? false);
        return false;
      }
      if (message.type === "GASOLINE_SUBTITLE") {
        if (!subtitlesEnabled)
          return false;
        const msg = message;
        showSubtitle(msg.text ?? "");
        return false;
      }
      if (message.type === "setActionToastsEnabled") {
        actionToastsEnabled = message.enabled;
        return false;
      }
      if (message.type === "setSubtitlesEnabled") {
        subtitlesEnabled = message.enabled;
        return false;
      }
      handleToggleMessage(message);
      if (message.type === "GASOLINE_HIGHLIGHT") {
        forwardHighlightMessage(message).then((result) => {
          sendResponse(result);
        }).catch((err) => {
          sendResponse({ success: false, error: err.message });
        });
        return true;
      }
      if (message.type === "GASOLINE_MANAGE_STATE") {
        handleStateCommand(message.params).then((result) => sendResponse(result)).catch((err) => sendResponse({ error: err.message }));
        return true;
      }
      if (message.type === "GASOLINE_EXECUTE_JS") {
        const params = message.params || {};
        return handleExecuteJs(params, sendResponse);
      }
      if (message.type === "GASOLINE_EXECUTE_QUERY") {
        return handleExecuteQuery(message.params || {}, sendResponse);
      }
      if (message.type === "A11Y_QUERY") {
        return handleA11yQuery(message.params || {}, sendResponse);
      }
      if (message.type === "DOM_QUERY") {
        return handleDomQuery(message.params || {}, sendResponse);
      }
      if (message.type === "GET_NETWORK_WATERFALL") {
        return handleGetNetworkWaterfall(sendResponse);
      }
      return void 0;
    });
  }

  // extension/content/favicon-replacer.js
  var originalFaviconHref = null;
  var flickerInterval = null;
  function initFaviconReplacer() {
    chrome.runtime.onMessage.addListener((message, sender, _sendResponse) => {
      if (sender.id !== chrome.runtime.id)
        return;
      if (message.type === "trackingStateChanged") {
        const newState = message.state;
        updateFavicon(newState);
      }
    });
    chrome.runtime.sendMessage({ type: "getTrackingState" }, (response) => {
      if (response && response.state) {
        updateFavicon(response.state);
      }
    });
  }
  function updateFavicon(state) {
    if (!state.isTracked) {
      restoreOriginalFavicon();
      stopFlicker();
    } else if (state.aiPilotEnabled) {
      replaceFaviconWithFlame(true);
      startFlicker();
    } else {
      replaceFaviconWithFlame(false);
      stopFlicker();
    }
  }
  function replaceFaviconWithFlame(withGlow) {
    if (!originalFaviconHref) {
      const existingLink = document.querySelector('link[rel*="icon"]');
      originalFaviconHref = existingLink?.href || "";
    }
    const existingIcons = document.querySelectorAll('link[rel*="icon"]');
    existingIcons.forEach((icon) => icon.remove());
    const link = document.createElement("link");
    link.rel = "icon";
    link.type = "image/svg+xml";
    link.id = "gasoline-favicon";
    const iconPath = withGlow ? "icons/icon-glow.svg" : "icons/icon.svg";
    link.href = chrome.runtime.getURL(iconPath);
    document.head.appendChild(link);
  }
  function restoreOriginalFavicon() {
    const gasolineIcon = document.getElementById("gasoline-favicon");
    if (gasolineIcon) {
      gasolineIcon.remove();
    }
    if (originalFaviconHref) {
      const link = document.createElement("link");
      link.rel = "icon";
      link.href = originalFaviconHref;
      document.head.appendChild(link);
    }
  }
  function startFlicker() {
    if (flickerInterval !== null) {
      return;
    }
    const flameFrames = [
      "icon-flicker-1-tiny.svg",
      // 85% - dark red/orange (coolest) + small dark ring
      "icon-flicker-2-small.svg",
      // 92% - orange + small orange ring
      "icon-flicker-3-normal.svg",
      // 100% - orange-yellow (base) + medium orange ring
      "icon-flicker-4-medium.svg",
      // 105% - yellow + medium yellow ring
      "icon-flicker-5-large.svg",
      // 112% - yellow/white (PEAK - hottest) + large bright ring
      "icon-flicker-6-medium.svg",
      // 105% - yellow + medium yellow ring (shrinking)
      "icon-flicker-7-smallmed.svg",
      // 96% - orange-yellow + medium ring
      "icon-flicker-8-small.svg"
      // 92% - orange + small orange ring (back to small)
    ];
    let currentFrameIndex = 0;
    flickerInterval = window.setInterval(() => {
      currentFrameIndex = (currentFrameIndex + 1) % flameFrames.length;
      const gasolineIcon = document.getElementById("gasoline-favicon");
      if (gasolineIcon) {
        const iconPath = `icons/${flameFrames[currentFrameIndex]}`;
        gasolineIcon.href = chrome.runtime.getURL(iconPath);
      }
    }, 150);
  }
  function stopFlicker() {
    if (flickerInterval !== null) {
      clearInterval(flickerInterval);
      flickerInterval = null;
    }
  }

  // extension/content.js
  var scriptsInjected = false;
  initTabTracking((tracked) => {
    if (tracked && !scriptsInjected) {
      initScriptInjection();
      scriptsInjected = true;
    }
  });
  initRequestTracking();
  initWindowMessageListener();
  initRuntimeMessageListener();
  initFaviconReplacer();
})();
//# sourceMappingURL=content.bundled.js.map
