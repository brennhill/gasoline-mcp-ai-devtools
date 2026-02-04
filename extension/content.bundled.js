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
  function initTabTracking() {
    updateTrackingStatus();
    chrome.storage.onChanged.addListener((changes) => {
      if (changes.trackedTabId) {
        updateTrackingStatus();
      }
    });
  }

  // extension/content/script-injection.js
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
    script.onload = () => script.remove();
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
  function clearPendingRequests() {
    pendingHighlightRequests.clear();
    pendingExecuteRequests.clear();
    pendingA11yRequests.clear();
    pendingDomRequests.clear();
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
  function initRequestTracking() {
    window.addEventListener("pagehide", clearPendingRequests);
    window.addEventListener("beforeunload", clearPendingRequests);
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
      if (event.source !== window)
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
  function handleExecuteJs(params, sendResponse) {
    const requestId = registerExecuteRequest(sendResponse);
    setTimeout(createRequestTimeoutCleanup(requestId, /* @__PURE__ */ new Map([[requestId, sendResponse]]), {
      success: false,
      error: "timeout",
      message: "Execute request timed out after 30s"
    }), 3e4);
    window.postMessage({
      type: "GASOLINE_EXECUTE_JS",
      requestId,
      script: params.script || "",
      timeoutMs: params.timeout_ms || 5e3
    }, window.location.origin);
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
  function initRuntimeMessageListener() {
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      if (!isValidBackgroundSender(sender)) {
        console.warn("[Gasoline] Rejected message from untrusted sender:", sender.id);
        return false;
      }
      if (message.type === "GASOLINE_PING") {
        return handlePing(sendResponse);
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
    chrome.runtime.onMessage.addListener((message, _sender, _sendResponse) => {
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
  initTabTracking();
  initRequestTracking();
  initWindowMessageListener();
  initRuntimeMessageListener();
  initFaviconReplacer();
  chrome.storage.onChanged.addListener((changes) => {
    if (changes.trackedTabId) {
      if (getIsTrackedTab() && !scriptsInjected) {
        initScriptInjection();
        scriptsInjected = true;
      }
    }
  });
  setTimeout(() => {
    if (getIsTrackedTab() && !scriptsInjected) {
      initScriptInjection();
      scriptsInjected = true;
    }
  }, 100);
})();
//# sourceMappingURL=content.bundled.js.map
