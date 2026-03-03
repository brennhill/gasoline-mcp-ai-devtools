"use strict";
(() => {
  // extension/lib/timeouts.js
  function readTestScale() {
    const globalScale = typeof globalThis !== "undefined" && typeof globalThis.GASOLINE_TEST_TIMEOUT_SCALE === "number" ? globalThis.GASOLINE_TEST_TIMEOUT_SCALE : null;
    if (globalScale !== null)
      return globalScale;
    if (typeof process !== "undefined" && process.env) {
      const raw = process.env.GASOLINE_TEST_TIMEOUT_SCALE || process.env.GASOLINE_TEST_TIME_SCALE;
      if (raw) {
        const parsed = Number(raw);
        if (Number.isFinite(parsed))
          return parsed;
      }
    }
    return 1;
  }
  function scaleTimeout(ms) {
    const scale = readTestScale();
    if (!Number.isFinite(scale) || scale <= 0 || scale === 1) {
      return ms;
    }
    return Math.max(5, Math.round(ms * scale));
  }

  // extension/lib/constants.js
  var ASYNC_COMMAND_TIMEOUT_MS = scaleTimeout(6e4);
  var AI_CONTEXT_PIPELINE_TIMEOUT_MS = scaleTimeout(3e3);
  var SettingName = {
    NETWORK_WATERFALL: "setNetworkWaterfallEnabled",
    PERFORMANCE_MARKS: "setPerformanceMarksEnabled",
    ACTION_REPLAY: "setActionReplayEnabled",
    WEBSOCKET_CAPTURE: "setWebSocketCaptureEnabled",
    WEBSOCKET_CAPTURE_MODE: "setWebSocketCaptureMode",
    PERFORMANCE_SNAPSHOT: "setPerformanceSnapshotEnabled",
    DEFERRAL: "setDeferralEnabled",
    NETWORK_BODY_CAPTURE: "setNetworkBodyCaptureEnabled",
    ACTION_TOASTS: "setActionToastsEnabled",
    SUBTITLES: "setSubtitlesEnabled",
    SERVER_URL: "setServerUrl"
  };
  var VALID_SETTING_NAMES = new Set(Object.values(SettingName));
  var INJECT_FORWARDED_SETTINGS = /* @__PURE__ */ new Set([
    SettingName.NETWORK_WATERFALL,
    SettingName.PERFORMANCE_MARKS,
    SettingName.ACTION_REPLAY,
    SettingName.WEBSOCKET_CAPTURE,
    SettingName.WEBSOCKET_CAPTURE_MODE,
    SettingName.PERFORMANCE_SNAPSHOT,
    SettingName.DEFERRAL,
    SettingName.NETWORK_BODY_CAPTURE,
    SettingName.SERVER_URL
  ]);
  var StorageKey = {
    TRACKED_TAB_ID: "trackedTabId",
    TRACKED_TAB_URL: "trackedTabUrl",
    TRACKED_TAB_TITLE: "trackedTabTitle",
    AI_WEB_PILOT_ENABLED: "aiWebPilotEnabled",
    DEBUG_MODE: "debugMode",
    SERVER_URL: "serverUrl",
    SCREENSHOT_ON_ERROR: "screenshotOnError",
    SOURCE_MAP_ENABLED: "sourceMapEnabled",
    LOG_LEVEL: "logLevel",
    THEME: "theme",
    DEFERRAL_ENABLED: "deferralEnabled",
    WEBSOCKET_CAPTURE_ENABLED: "webSocketCaptureEnabled",
    WEBSOCKET_CAPTURE_MODE: "webSocketCaptureMode",
    NETWORK_WATERFALL_ENABLED: "networkWaterfallEnabled",
    PERFORMANCE_MARKS_ENABLED: "performanceMarksEnabled",
    ACTION_REPLAY_ENABLED: "actionReplayEnabled",
    NETWORK_BODY_CAPTURE_ENABLED: "networkBodyCaptureEnabled",
    ACTION_TOASTS_ENABLED: "actionToastsEnabled",
    SUBTITLES_ENABLED: "subtitlesEnabled",
    RECORDING: "gasoline_recording",
    PENDING_RECORDING: "gasoline_pending_recording",
    PENDING_MIC_RECORDING: "gasoline_pending_mic_recording",
    MIC_GRANTED: "gasoline_mic_granted",
    RECORD_AUDIO_PREF: "gasoline_record_audio_pref"
  };

  // extension/content/tab-tracking.js
  var isTrackedTab = false;
  var currentTabId = null;
  async function updateTrackingStatus() {
    try {
      const storage = await chrome.storage.local.get([StorageKey.TRACKED_TAB_ID]);
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
      if (changes[StorageKey.TRACKED_TAB_ID]) {
        await updateTrackingStatus();
        onChange?.(isTrackedTab);
      }
    });
    return ready;
  }

  // extension/content/script-injection.js
  var injected = false;
  var bridgeReady = false;
  var injectionPromise = null;
  var bridgeProbePromise = null;
  var bridgeProbeCounter = 0;
  var NONCE_ATTR = "data-gasoline-nonce";
  var pageNonce = crypto.getRandomValues(new Uint8Array(16)).reduce((s, b) => s + b.toString(16).padStart(2, "0"), "");
  function getPageNonce() {
    return pageNonce;
  }
  function isInjectScriptLoaded() {
    return injected;
  }
  var SYNC_SETTINGS = [
    { storageKey: "webSocketCaptureEnabled", messageType: SettingName.WEBSOCKET_CAPTURE },
    { storageKey: "webSocketCaptureMode", messageType: SettingName.WEBSOCKET_CAPTURE_MODE, isMode: true },
    { storageKey: "networkWaterfallEnabled", messageType: SettingName.NETWORK_WATERFALL },
    { storageKey: "performanceMarksEnabled", messageType: SettingName.PERFORMANCE_MARKS },
    { storageKey: "actionReplayEnabled", messageType: SettingName.ACTION_REPLAY },
    { storageKey: "networkBodyCaptureEnabled", messageType: SettingName.NETWORK_BODY_CAPTURE }
  ];
  function syncStoredSettings() {
    const storageKeys = SYNC_SETTINGS.map((s) => s.storageKey);
    chrome.storage.local.get(storageKeys, (result) => {
      for (const setting of SYNC_SETTINGS) {
        const value = result[setting.storageKey];
        if (value === void 0)
          continue;
        if (setting.isMode) {
          window.postMessage({
            type: "GASOLINE_SETTING",
            setting: setting.messageType,
            mode: value,
            _nonce: pageNonce
          }, window.location.origin);
        } else {
          window.postMessage({ type: "GASOLINE_SETTING", setting: setting.messageType, enabled: value, _nonce: pageNonce }, window.location.origin);
        }
      }
    });
  }
  function injectAxeCore() {
    if (document.getElementById("gasoline-axe-loader"))
      return;
    const script = document.createElement("script");
    script.id = "gasoline-axe-loader";
    script.src = chrome.runtime.getURL("lib/axe.min.js");
    script.onload = () => script.remove();
    (document.head || document.documentElement).appendChild(script);
  }
  function injectScript() {
    document.querySelectorAll(`script[${NONCE_ATTR}]`).forEach((el) => {
      if (typeof el.remove === "function")
        el.remove();
    });
    document.documentElement?.setAttribute?.(NONCE_ATTR, pageNonce);
    const script = document.createElement("script");
    script.src = chrome.runtime.getURL("inject.bundled.js");
    script.type = "module";
    script.dataset.gasolineNonce = pageNonce;
    return new Promise((resolve) => {
      script.onload = () => {
        script.remove();
        injected = true;
        bridgeReady = false;
        setTimeout(syncStoredSettings, 50);
        resolve(true);
      };
      script.onerror = () => {
        script.remove();
        injected = false;
        bridgeReady = false;
        resolve(false);
      };
      (document.head || document.documentElement).appendChild(script);
    });
  }
  function beginInjection(force = false) {
    if (!force) {
      if (injected)
        return Promise.resolve(true);
      if (injectionPromise)
        return injectionPromise;
    } else if (injectionPromise) {
      return injectionPromise;
    }
    injectionPromise = new Promise((resolve) => {
      const runInjection = () => {
        injectAxeCore();
        injectScript().then((ok) => resolve(ok)).finally(() => {
          injectionPromise = null;
        });
      };
      if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", runInjection, { once: true });
        return;
      }
      runInjection();
    });
    return injectionPromise;
  }
  async function ensureInjectScriptReady(timeoutMs = 2e3, force = false) {
    if (!force && injected)
      return true;
    const injection = beginInjection(force);
    if (timeoutMs <= 0)
      return injection;
    return Promise.race([
      injection,
      new Promise((resolve) => {
        setTimeout(() => resolve(injected), timeoutMs);
      })
    ]);
  }
  async function ensureInjectBridgeReady(timeoutMs = 350) {
    if (bridgeReady)
      return true;
    const injectReady = await ensureInjectScriptReady(timeoutMs);
    if (!injectReady)
      return false;
    if (bridgeReady)
      return true;
    if (bridgeProbePromise)
      return bridgeProbePromise;
    bridgeProbePromise = new Promise((resolve) => {
      const requestId = `inject_bridge_${Date.now()}_${++bridgeProbeCounter}`;
      let settled = false;
      let timer;
      const cleanup = () => {
        if (timer)
          clearTimeout(timer);
        window.removeEventListener("message", onMessage);
        bridgeProbePromise = null;
      };
      const finish = (ok) => {
        if (settled)
          return;
        settled = true;
        if (ok)
          bridgeReady = true;
        cleanup();
        resolve(ok);
      };
      const onMessage = (event) => {
        if (event.source !== window || event.origin !== window.location.origin)
          return;
        if (event.data?.type !== "GASOLINE_INJECT_BRIDGE_PONG")
          return;
        if (event.data?.requestId !== requestId)
          return;
        if (event.data?._nonce && event.data._nonce !== pageNonce)
          return;
        finish(true);
      };
      window.addEventListener("message", onMessage);
      timer = setTimeout(() => finish(false), Math.max(25, timeoutMs));
      try {
        window.postMessage({
          type: "GASOLINE_INJECT_BRIDGE_PING",
          requestId,
          _nonce: pageNonce
        }, window.location.origin);
      } catch {
        finish(false);
      }
    });
    return bridgeProbePromise;
  }
  function initScriptInjection(force = false) {
    void beginInjection(force);
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
  function hasExecuteRequest(requestId) {
    return pendingExecuteRequests.has(requestId);
  }
  function deleteExecuteRequest(requestId) {
    pendingExecuteRequests.delete(requestId);
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
  function hasA11yRequest(requestId) {
    return pendingA11yRequests.has(requestId);
  }
  function deleteA11yRequest(requestId) {
    pendingA11yRequests.delete(requestId);
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
  function hasDomRequest(requestId) {
    return pendingDomRequests.has(requestId);
  }
  function deleteDomRequest(requestId) {
    pendingDomRequests.delete(requestId);
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
  var RESPONSE_HANDLERS = {
    GASOLINE_HIGHLIGHT_RESPONSE: (id, result) => resolveHighlightRequest(id, result),
    GASOLINE_EXECUTE_JS_RESULT: (id, result) => resolveExecuteRequest(id, result),
    GASOLINE_A11Y_QUERY_RESPONSE: (id, result) => resolveA11yRequest(id, result),
    GASOLINE_DOM_QUERY_RESPONSE: (id, result) => resolveDomRequest(id, result)
  };
  function initWindowMessageListener() {
    window.addEventListener("message", (event) => {
      if (event.source !== window || event.origin !== window.location.origin)
        return;
      const { type: messageType, requestId, result, payload } = event.data || {};
      const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : void 0;
      if (responseHandler) {
        const nonce = event.data?._nonce;
        if (nonce && nonce !== getPageNonce())
          return;
        if (requestId !== void 0)
          responseHandler(requestId, result);
        return;
      }
      if (!getIsTrackedTab())
        return;
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

  // extension/content/extractors/shared.js
  var MAIN_CONTENT_SELECTORS = [
    "main",
    "article",
    '[role="main"]',
    "#main",
    ".main",
    ".post-content",
    ".entry-content",
    ".article-body",
    ".article-content",
    ".story-body",
    ".article",
    ".post",
    "#content",
    ".content",
    ".results"
  ];
  function findMainContentElement(minTextLength = 100) {
    for (const sel of MAIN_CONTENT_SELECTORS) {
      const el = document.querySelector(sel);
      if (!el)
        continue;
      const text = (el.innerText || el.textContent || "").trim();
      if (text.length > minTextLength)
        return el;
    }
    return document.body || document.documentElement;
  }

  // extension/content/extractors/readable.js
  var REMOVE_SELECTORS = [
    "nav",
    "header",
    "footer",
    "aside",
    "script",
    "style",
    "noscript",
    "svg",
    '[role="navigation"]',
    '[role="banner"]',
    '[role="contentinfo"]',
    '[aria-hidden="true"]',
    ".ad",
    ".ads",
    ".advertisement",
    ".social-share",
    ".comments",
    ".sidebar",
    ".related-posts",
    ".newsletter"
  ];
  function cleanText(el) {
    if (!el)
      return "";
    const clone = el.cloneNode(true);
    for (const sel of REMOVE_SELECTORS) {
      const els = clone.querySelectorAll(sel);
      for (const child of Array.from(els))
        child.remove();
    }
    return (clone.innerText || clone.textContent || "").replace(/\s+/g, " ").trim();
  }
  function getByline() {
    const selectors = [".author", '[rel="author"]', ".byline", ".post-author", 'meta[name="author"]'];
    for (const sel of selectors) {
      const el = document.querySelector(sel);
      if (el) {
        const text = (el.getAttribute("content") || el.innerText || "").trim();
        if (text.length > 0 && text.length < 200)
          return text;
      }
    }
    return "";
  }
  function extractReadable() {
    const main = findMainContentElement(100);
    const content = cleanText(main);
    const excerpt = content.slice(0, 300);
    const words = content.split(/\s+/).filter(Boolean);
    return {
      title: document.title || "",
      content,
      excerpt,
      byline: getByline(),
      word_count: words.length,
      url: window.location.href
    };
  }

  // extension/content/extractors/markdown.js
  var MAX_OUTPUT_CHARS = 2e5;
  var SKIP_TAGS = ["nav", "header", "footer", "aside", "script", "style", "noscript", "svg"];
  function tableToMarkdown(table) {
    const rows = table.querySelectorAll("tr");
    if (rows.length === 0)
      return "";
    let md = "";
    for (let r = 0; r < rows.length; r++) {
      const rowEl = rows[r];
      if (!rowEl)
        continue;
      const cells = rowEl.querySelectorAll("th,td");
      let row = "|";
      for (let c = 0; c < cells.length; c++) {
        row += " " + (cells[c].innerText || "").trim().replace(/\|/g, "\\|").replace(/\n/g, " ") + " |";
      }
      md += row + "\n";
      if (r === 0 && rowEl.querySelector("th")) {
        md += "|";
        for (let c2 = 0; c2 < cells.length; c2++)
          md += " --- |";
        md += "\n";
      }
    }
    return md;
  }
  function nodeToMarkdown(node, depth, budget) {
    if (!node || budget.remaining <= 0)
      return "";
    if (depth > 20)
      return "";
    if (node.nodeType === 3) {
      const text = node.textContent || "";
      budget.remaining -= text.length;
      return text;
    }
    if (node.nodeType !== 1)
      return "";
    const el = node;
    const tag = el.tagName.toLowerCase();
    if (SKIP_TAGS.includes(tag))
      return "";
    if (el.getAttribute("role") === "navigation")
      return "";
    if (el.getAttribute("aria-hidden") === "true")
      return "";
    let children = "";
    for (let i = 0; i < el.childNodes.length; i++) {
      if (budget.remaining <= 0)
        break;
      const child = el.childNodes[i];
      if (child)
        children += nodeToMarkdown(child, depth + 1, budget);
    }
    children = children.replace(/\n{3,}/g, "\n\n");
    switch (tag) {
      case "h1":
        return "\n# " + children.trim() + "\n\n";
      case "h2":
        return "\n## " + children.trim() + "\n\n";
      case "h3":
        return "\n### " + children.trim() + "\n\n";
      case "h4":
        return "\n#### " + children.trim() + "\n\n";
      case "h5":
        return "\n##### " + children.trim() + "\n\n";
      case "h6":
        return "\n###### " + children.trim() + "\n\n";
      case "p":
        return "\n" + children.trim() + "\n\n";
      case "br":
        return "\n";
      case "hr":
        return "\n---\n\n";
      case "strong":
      case "b":
        return "**" + children.trim() + "**";
      case "em":
      case "i":
        return "*" + children.trim() + "*";
      case "code":
        return "`" + children.trim() + "`";
      case "pre":
        return "\n```\n" + (el.innerText || "").trim() + "\n```\n\n";
      case "a": {
        let href = el.getAttribute("href") || "";
        if (href && href !== "#" && !href.startsWith("javascript:")) {
          try {
            href = new URL(href, window.location.href).href;
          } catch {
          }
          return "[" + children.trim() + "](" + href + ")";
        }
        return children;
      }
      case "img": {
        let src = el.getAttribute("src") || "";
        const alt = el.getAttribute("alt") || "";
        if (src) {
          try {
            src = new URL(src, window.location.href).href;
          } catch {
          }
          return "![" + alt + "](" + src + ")";
        }
        return "";
      }
      case "ul":
      case "ol":
        return "\n" + children + "\n";
      case "li": {
        const parent = el.parentElement;
        if (parent && parent.tagName.toLowerCase() === "ol") {
          const idx = Array.from(parent.children).indexOf(el) + 1;
          return idx + ". " + children.trim() + "\n";
        }
        return "- " + children.trim() + "\n";
      }
      case "blockquote":
        return "\n> " + children.trim().replace(/\n/g, "\n> ") + "\n\n";
      case "table":
        return "\n" + tableToMarkdown(el) + "\n\n";
      case "div":
      case "section":
      case "article":
      case "main":
        return children;
      default:
        return children;
    }
  }
  function extractMarkdown() {
    const main = findMainContentElement(100);
    const budget = { remaining: MAX_OUTPUT_CHARS };
    let markdown = nodeToMarkdown(main, 0, budget).trim();
    const truncated = budget.remaining <= 0;
    if (truncated) {
      markdown = markdown.slice(0, MAX_OUTPUT_CHARS) + "\n\n[...truncated]";
    }
    const words = markdown.replace(/[#*[\]()`|>-]/g, " ").split(/\s+/).filter(Boolean);
    return {
      title: document.title || "",
      markdown,
      word_count: words.length,
      url: window.location.href,
      ...truncated ? { truncated: true } : {}
    };
  }

  // extension/content/extractors/page-summary.js
  function cleanText2(value, maxLen) {
    let text = (value || "").replace(/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]/g, "").replace(/\s+/g, " ").trim();
    if (maxLen > 0 && text.length > maxLen) {
      text = text.slice(0, maxLen);
    }
    return text;
  }
  function absoluteHref(value) {
    try {
      return new URL(value || "", window.location.href).href;
    } catch {
      return value || "";
    }
  }
  function visibleInteractiveCount() {
    const nodes = document.querySelectorAll('a[href],button,input:not([type="hidden"]),select,textarea,[role="button"],[role="link"],[tabindex]');
    let count = 0;
    for (const node of Array.from(nodes)) {
      if (node.disabled)
        continue;
      const style = window.getComputedStyle(node);
      if (style.display === "none" || style.visibility === "hidden")
        continue;
      const rect = node.getBoundingClientRect();
      if (rect.width <= 0 || rect.height <= 0)
        continue;
      count += 1;
    }
    return count;
  }
  function findMainNode() {
    return findMainContentElement(120);
  }
  function classifyPage(forms, interactiveCount, linkCount, paragraphCount, headingCount, previewText) {
    const hasSearchInput = !!document.querySelector('input[type="search"], input[name*="search" i], input[placeholder*="search" i]');
    const likelySearchURL = /[?&](q|query|search)=/i.test(window.location.search);
    const hasArticle = document.querySelectorAll("article").length > 0;
    const hasTable = document.querySelectorAll("table").length > 0;
    let totalFormFields = 0;
    for (const form of forms) {
      totalFormFields += form.fields.length;
    }
    if (hasSearchInput && (likelySearchURL || linkCount > 10))
      return "search_results";
    if (forms.length > 0 && totalFormFields >= 3 && paragraphCount < 8)
      return "form";
    if (hasArticle || paragraphCount >= 8 && linkCount < paragraphCount * 2)
      return "article";
    if (hasTable || interactiveCount > 25 && headingCount >= 2)
      return "dashboard";
    if (linkCount > 30 && paragraphCount < 10)
      return "link_list";
    if (previewText.length < 80 && interactiveCount > 10)
      return "app";
    return "generic";
  }
  function extractPageSummary() {
    const headingNodes = document.querySelectorAll("h1, h2, h3");
    const headings = [];
    for (const heading of Array.from(headingNodes)) {
      if (headings.length >= 30)
        break;
      const text = cleanText2(heading.innerText || heading.textContent || "", 200);
      if (!text)
        continue;
      headings.push(heading.tagName.toLowerCase() + ": " + text);
    }
    const navCandidates = document.querySelectorAll('nav a[href], header a[href], [role="navigation"] a[href]');
    const navLinks = [];
    const seenNav = {};
    for (const link of Array.from(navCandidates)) {
      if (navLinks.length >= 25)
        break;
      const linkText = cleanText2(link.innerText || link.textContent || "", 80);
      const href = absoluteHref(link.getAttribute("href") || "");
      if (!href)
        continue;
      const key = linkText + "|" + href;
      if (seenNav[key])
        continue;
      seenNav[key] = true;
      navLinks.push({ text: linkText, href });
    }
    const forms = [];
    const formNodes = document.querySelectorAll("form");
    for (const form of Array.from(formNodes)) {
      if (forms.length >= 10)
        break;
      const fieldNodes = form.querySelectorAll("input, select, textarea");
      const fields = [];
      const seenFields = {};
      for (const field of Array.from(fieldNodes)) {
        if (fields.length >= 25)
          break;
        const candidate = field.getAttribute("name") || field.getAttribute("id") || field.getAttribute("aria-label") || field.getAttribute("type") || field.tagName.toLowerCase();
        const cleaned = cleanText2(candidate || "", 60);
        if (!cleaned || seenFields[cleaned])
          continue;
        seenFields[cleaned] = true;
        fields.push(cleaned);
      }
      forms.push({
        action: absoluteHref(form.getAttribute("action") || window.location.href),
        method: (form.getAttribute("method") || "GET").toUpperCase(),
        fields
      });
    }
    const mainNode = findMainNode();
    const mainText = cleanText2(mainNode ? mainNode.innerText || mainNode.textContent || "" : "", 2e4);
    const preview = mainText.slice(0, 500);
    const wordCount = mainText ? mainText.split(/\s+/).filter(Boolean).length : 0;
    const linkCount = document.querySelectorAll("a[href]").length;
    const paragraphCount = document.querySelectorAll("p").length;
    const interactiveCount = visibleInteractiveCount();
    const pageType = classifyPage(forms, interactiveCount, linkCount, paragraphCount, headings.length, preview);
    return {
      url: window.location.href,
      title: document.title || "",
      type: pageType,
      headings,
      nav_links: navLinks,
      forms,
      interactive_element_count: interactiveCount,
      main_content_preview: preview,
      word_count: wordCount
    };
  }

  // extension/content/message-handlers.js
  var nextRequestId = 1;
  function parseQueryParams(params) {
    if (typeof params === "string") {
      try {
        return JSON.parse(params);
      } catch {
        return {};
      }
    }
    return typeof params === "object" ? params : {};
  }
  function postToInject(data) {
    window.postMessage({ ...data, _nonce: getPageNonce() }, window.location.origin);
  }
  var TOGGLE_MESSAGES = INJECT_FORWARDED_SETTINGS;
  function isValidBackgroundSender(sender) {
    return typeof sender.id === "string" && sender.id === chrome.runtime.id;
  }
  function forwardHighlightMessage(message) {
    return ensureInjectBridgeReady(1500).then((ready) => {
      if (!ready) {
        return {
          success: false,
          error: isInjectScriptLoaded() ? "inject_not_responding" : "inject_not_loaded"
        };
      }
      const requestId = registerHighlightRequest((result) => deferred.resolve(result));
      const deferred = createDeferredPromise();
      postToInject({
        type: "GASOLINE_HIGHLIGHT_REQUEST",
        requestId,
        params: message.params
      });
      return promiseRaceWithCleanup(deferred.promise, 3e4, { success: false, error: "timeout" }, () => {
        if (hasHighlightRequest(requestId)) {
          deleteHighlightRequest(requestId);
        }
      });
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
    postToInject({
      type: "GASOLINE_STATE_COMMAND",
      messageId,
      action,
      name,
      state,
      include_url
    });
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
    if (message.type === SettingName.WEBSOCKET_CAPTURE_MODE) {
      payload.mode = message.mode;
    } else if (message.type === SettingName.SERVER_URL) {
      payload.url = message.url;
    } else {
      payload.enabled = message.enabled;
    }
    window.postMessage({ ...payload, _nonce: getPageNonce() }, window.location.origin);
  }
  function executeInMainWorld(params, sendResponse) {
    const timeoutMs = params.timeout_ms || 5e3;
    const requestId = registerExecuteRequest(sendResponse);
    const safetyTimeoutMs = timeoutMs + 2e3;
    setTimeout(() => {
      if (hasExecuteRequest(requestId)) {
        deleteExecuteRequest(requestId);
        sendResponse({
          success: false,
          error: "inject_not_responding",
          message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`
        });
      }
    }, safetyTimeoutMs);
    postToInject({
      type: "GASOLINE_EXECUTE_JS",
      requestId,
      script: params.script || "",
      timeoutMs
    });
  }
  function handleExecuteJs(params, sendResponse) {
    const injectReadyWaitMs = Math.max(750, Math.min(3e3, (params.timeout_ms || 5e3) + 500));
    void ensureInjectBridgeReady(injectReadyWaitMs).then((ready) => {
      if (!ready) {
        const fallbackError = isInjectScriptLoaded() ? "inject_not_responding" : "inject_not_loaded";
        sendResponse({
          success: false,
          error: fallbackError,
          message: fallbackError === "inject_not_loaded" ? "Inject script not loaded in page context. Tab may not be tracked." : `Inject script did not respond within ${injectReadyWaitMs}ms. The tab may not be tracked or the inject script failed to load.`
        });
        return;
      }
      executeInMainWorld(params, sendResponse);
    });
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
    const parsedParams = parseQueryParams(params);
    const requestId = registerA11yRequest(sendResponse);
    setTimeout(() => {
      if (hasA11yRequest(requestId)) {
        deleteA11yRequest(requestId);
        sendResponse({ error: "Accessibility audit timeout" });
      }
    }, ASYNC_COMMAND_TIMEOUT_MS);
    postToInject({
      type: "GASOLINE_A11Y_QUERY",
      requestId,
      params: parsedParams
    });
    return true;
  }
  function handleDomQuery(params, sendResponse) {
    const parsedParams = parseQueryParams(params);
    const requestId = registerDomRequest(sendResponse);
    setTimeout(() => {
      if (hasDomRequest(requestId)) {
        deleteDomRequest(requestId);
        sendResponse({ error: "DOM query timeout" });
      }
    }, ASYNC_COMMAND_TIMEOUT_MS);
    postToInject({
      type: "GASOLINE_DOM_QUERY",
      requestId,
      params: parsedParams
    });
    return true;
  }
  function handleGetNetworkWaterfall(sendResponse) {
    const requestId = nextRequestId++;
    const deferred = createDeferredPromise();
    const responseHandler = (event) => {
      if (event.source !== window)
        return;
      const nonce = event.data?._nonce;
      if (nonce && nonce !== getPageNonce())
        return;
      if (event.data?.type === "GASOLINE_WATERFALL_RESPONSE" && event.data?.requestId === requestId) {
        window.removeEventListener("message", responseHandler);
        deferred.resolve({ entries: event.data.entries || [] });
      }
    };
    window.addEventListener("message", responseHandler);
    postToInject({
      type: "GASOLINE_GET_WATERFALL",
      requestId
    });
    promiseRaceWithCleanup(deferred.promise, 5e3, { entries: [] }, () => {
      window.removeEventListener("message", responseHandler);
    }).then((result) => {
      sendResponse(result);
    }, () => {
      sendResponse({ entries: [] });
    });
    return true;
  }
  function forwardInjectQuery(queryType, responseType, label, params, sendResponse) {
    const parsedParams = parseQueryParams(params);
    const requestId = nextRequestId++;
    const deferred = createDeferredPromise();
    const responseHandler = (event) => {
      if (event.source !== window)
        return;
      const nonce = event.data?._nonce;
      if (nonce && nonce !== getPageNonce())
        return;
      if (event.data?.type === responseType && event.data?.requestId === requestId) {
        window.removeEventListener("message", responseHandler);
        deferred.resolve(event.data.result || { error: `No result from ${label}` });
      }
    };
    window.addEventListener("message", responseHandler);
    postToInject({ type: queryType, requestId, params: parsedParams });
    promiseRaceWithCleanup(deferred.promise, ASYNC_COMMAND_TIMEOUT_MS, { error: `${label} timeout` }, () => {
      window.removeEventListener("message", responseHandler);
    }).then((result) => sendResponse(result), () => sendResponse({ error: `${label} failed` }));
    return true;
  }
  function handleComputedStylesQuery(params, sendResponse) {
    return forwardInjectQuery("GASOLINE_COMPUTED_STYLES_QUERY", "GASOLINE_COMPUTED_STYLES_RESPONSE", "Computed styles query", params, sendResponse);
  }
  function handleFormDiscoveryQuery(params, sendResponse) {
    return forwardInjectQuery("GASOLINE_FORM_DISCOVERY_QUERY", "GASOLINE_FORM_DISCOVERY_RESPONSE", "Form discovery", params, sendResponse);
  }
  function handleFormStateQuery(params, sendResponse) {
    return forwardInjectQuery("GASOLINE_FORM_STATE_QUERY", "GASOLINE_FORM_STATE_RESPONSE", "Form state", params, sendResponse);
  }
  function handleDataTableQuery(params, sendResponse) {
    return forwardInjectQuery("GASOLINE_DATA_TABLE_QUERY", "GASOLINE_DATA_TABLE_RESPONSE", "Data table extraction", params, sendResponse);
  }
  function handleLinkHealthQuery(params, sendResponse) {
    return forwardInjectQuery("GASOLINE_LINK_HEALTH_QUERY", "GASOLINE_LINK_HEALTH_RESPONSE", "Link health check", params, sendResponse);
  }
  function handleGetReadable(sendResponse) {
    try {
      sendResponse(extractReadable());
    } catch (err) {
      sendResponse({ error: "get_readable_failed", message: err.message || "Readable extraction failed" });
    }
    return false;
  }
  function handleGetMarkdown(sendResponse) {
    try {
      sendResponse(extractMarkdown());
    } catch (err) {
      sendResponse({ error: "get_markdown_failed", message: err.message || "Markdown extraction failed" });
    }
    return false;
  }
  function handlePageSummary(sendResponse) {
    try {
      sendResponse(extractPageSummary());
    } catch (err) {
      sendResponse({ error: "page_summary_failed", message: err.message || "Page summary extraction failed" });
    }
    return false;
  }

  // extension/content/ui/toast.js
  var TOAST_THEMES = {
    trying: { bg: "linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)", shadow: "rgba(59, 130, 246, 0.4)" },
    success: { bg: "linear-gradient(135deg, #22c55e 0%, #16a34a 100%)", shadow: "rgba(34, 197, 94, 0.4)" },
    warning: { bg: "linear-gradient(135deg, #f59e0b 0%, #d97706 100%)", shadow: "rgba(245, 158, 11, 0.4)" },
    error: { bg: "linear-gradient(135deg, #ef4444 0%, #dc2626 100%)", shadow: "rgba(239, 68, 68, 0.4)" },
    audio: { bg: "linear-gradient(135deg, #f97316 0%, #ea580c 100%)", shadow: "rgba(249, 115, 22, 0.5)" }
  };
  var TOAST_ANIMATION_CSS = [
    "@keyframes gasolineArrowBounceUp {",
    "  0%, 100% { transform: translateY(0); opacity: 1; }",
    "  50% { transform: translateY(-6px); opacity: 0.7; }",
    "}",
    "@keyframes gasolineToastPulse {",
    "  0%, 100% { box-shadow: 0 4px 20px var(--toast-shadow); }",
    "  50% { box-shadow: 0 8px 32px var(--toast-shadow-intense); }",
    "}",
    ".gasoline-toast-arrow {",
    "  display: inline-block; margin-left: 8px;",
    "  animation: gasolineArrowBounceUp 1.5s ease-in-out infinite;",
    "}",
    ".gasoline-toast-pulse { animation: gasolineToastPulse 2s ease-in-out infinite; }"
  ].join("\n");
  function injectToastAnimationStyles() {
    if (document.getElementById("gasoline-toast-animations"))
      return;
    const style = document.createElement("style");
    style.id = "gasoline-toast-animations";
    style.textContent = TOAST_ANIMATION_CSS;
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
    const arrowChar = "\u2191";
    const toast = document.createElement("div");
    toast.id = "gasoline-action-toast";
    if (isAudioPrompt) {
      toast.className = "gasoline-toast-pulse";
    }
    if (isAudioPrompt) {
      const icon = document.createElement("img");
      icon.src = chrome.runtime.getURL("icons/icon-48.png");
      Object.assign(icon.style, {
        width: "20px",
        height: "20px",
        marginRight: "8px",
        flexShrink: "0"
      });
      toast.appendChild(icon);
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
      right: isAudioPrompt ? "80px" : "auto",
      left: isAudioPrompt ? "auto" : "50%",
      transform: isAudioPrompt ? "none" : "translateX(-50%)",
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

  // extension/content/ui/subtitle.js
  var subtitleEscapeHandler = null;
  var SUBTITLE_AUTO_TIMEOUT_MS = 6e4;
  var subtitleAutoTimer = null;
  function clearAutoTimer() {
    if (subtitleAutoTimer) {
      clearTimeout(subtitleAutoTimer);
      subtitleAutoTimer = null;
    }
  }
  function fadeOutAndRemove(elementId, delayMs) {
    const el = document.getElementById(elementId);
    if (!el)
      return;
    el.style.opacity = "0";
    setTimeout(() => el.remove(), delayMs);
  }
  function detachEscapeListener() {
    if (!subtitleEscapeHandler)
      return;
    document.removeEventListener("keydown", subtitleEscapeHandler);
    subtitleEscapeHandler = null;
  }
  function clearSubtitle() {
    clearAutoTimer();
    fadeOutAndRemove("gasoline-subtitle", 200);
    detachEscapeListener();
  }
  function showSubtitle(text) {
    const ELEMENT_ID = "gasoline-subtitle";
    const CLOSE_BTN_ID = "gasoline-subtitle-close";
    if (!text) {
      clearSubtitle();
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
        pointerEvents: "auto",
        opacity: "0",
        transition: "opacity 0.2s ease-in",
        maxHeight: "4.2em",
        // ~3 lines
        overflow: "hidden",
        textOverflow: "ellipsis",
        boxSizing: "border-box"
      });
      const closeBtn2 = document.createElement("button");
      closeBtn2.id = CLOSE_BTN_ID;
      closeBtn2.textContent = "\xD7";
      Object.assign(closeBtn2.style, {
        position: "absolute",
        top: "-6px",
        right: "-6px",
        width: "16px",
        height: "16px",
        padding: "0",
        margin: "0",
        border: "none",
        borderRadius: "50%",
        background: "rgba(255, 255, 255, 0.25)",
        color: "#fff",
        fontSize: "12px",
        lineHeight: "16px",
        textAlign: "center",
        cursor: "pointer",
        pointerEvents: "auto",
        opacity: "0",
        transition: "opacity 0.15s ease-in",
        fontFamily: "sans-serif"
      });
      closeBtn2.addEventListener("click", (e) => {
        e.stopPropagation();
        clearSubtitle();
      });
      bar.appendChild(closeBtn2);
      bar.addEventListener("mouseenter", () => {
        const btn = document.getElementById(CLOSE_BTN_ID);
        if (btn)
          btn.style.opacity = "1";
      });
      bar.addEventListener("mouseleave", () => {
        const btn = document.getElementById(CLOSE_BTN_ID);
        if (btn)
          btn.style.opacity = "0";
      });
      const target = document.body || document.documentElement;
      if (!target)
        return;
      target.appendChild(bar);
    }
    const closeBtn = document.getElementById(CLOSE_BTN_ID);
    bar.textContent = text;
    if (closeBtn) {
      bar.appendChild(closeBtn);
    }
    if (subtitleEscapeHandler) {
      document.removeEventListener("keydown", subtitleEscapeHandler);
    }
    subtitleEscapeHandler = (e) => {
      if (e.key === "Escape") {
        clearSubtitle();
      }
    };
    document.addEventListener("keydown", subtitleEscapeHandler);
    void bar.offsetHeight;
    bar.style.opacity = "1";
    clearAutoTimer();
    subtitleAutoTimer = setTimeout(() => {
      clearSubtitle();
    }, SUBTITLE_AUTO_TIMEOUT_MS);
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

  // extension/content/ui/chat-widget.js
  var WIDGET_ID = "gasoline-chat-widget";
  var INPUT_ID = "gasoline-chat-input";
  var PIN_ID = "gasoline-chat-pin";
  var STATUS_ID = "gasoline-chat-status";
  var isPinned = false;
  var currentClientName = "AI";
  var chatEscapeHandler = null;
  var isRemoving = false;
  function toggleChatWidget(clientName) {
    if (clientName)
      currentClientName = clientName;
    const existing = document.getElementById(WIDGET_ID);
    if (existing && !isRemoving) {
      removeChatWidget();
    } else if (!existing && !isRemoving) {
      showChatWidget();
    }
  }
  function showChatWidget() {
    if (document.getElementById(WIDGET_ID))
      return;
    const widget = document.createElement("div");
    widget.id = WIDGET_ID;
    widget.setAttribute("role", "dialog");
    widget.setAttribute("aria-label", `Push message to ${currentClientName}`);
    Object.assign(widget.style, {
      position: "fixed",
      bottom: "20px",
      right: "20px",
      width: "340px",
      background: "#1a1a2e",
      borderRadius: "12px",
      boxShadow: "0 8px 32px rgba(0, 0, 0, 0.4), 0 0 0 1px rgba(255, 255, 255, 0.08)",
      zIndex: "2147483643",
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      overflow: "hidden",
      opacity: "0",
      transform: "translateY(10px)",
      transition: "opacity 0.15s ease-out, transform 0.15s ease-out"
    });
    widget.addEventListener("keydown", (e) => {
      e.stopPropagation();
    });
    const header = document.createElement("div");
    Object.assign(header.style, {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      padding: "10px 14px",
      background: "rgba(255, 255, 255, 0.04)",
      borderBottom: "1px solid rgba(255, 255, 255, 0.06)"
    });
    const headerLeft = document.createElement("div");
    Object.assign(headerLeft.style, { display: "flex", alignItems: "center", gap: "8px" });
    const label = document.createElement("span");
    label.textContent = `Push to ${currentClientName}`;
    Object.assign(label.style, {
      color: "#e0e0e0",
      fontSize: "12px",
      fontWeight: "600",
      letterSpacing: "0.3px"
    });
    headerLeft.appendChild(label);
    header.appendChild(headerLeft);
    const headerRight = document.createElement("div");
    Object.assign(headerRight.style, { display: "flex", alignItems: "center", gap: "6px" });
    const pinBtn = document.createElement("button");
    pinBtn.id = PIN_ID;
    pinBtn.title = isPinned ? "Unpin (close after sending)" : "Pin (keep open after sending)";
    pinBtn.textContent = "Pin";
    pinBtn.setAttribute("aria-pressed", String(isPinned));
    Object.assign(pinBtn.style, {
      background: isPinned ? "rgba(59, 130, 246, 0.3)" : "transparent",
      border: "1px solid " + (isPinned ? "rgba(59, 130, 246, 0.5)" : "rgba(255, 255, 255, 0.1)"),
      borderRadius: "4px",
      color: isPinned ? "#60a5fa" : "#999",
      fontSize: "11px",
      cursor: "pointer",
      padding: "2px 8px",
      transition: "all 0.15s ease"
    });
    pinBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      isPinned = !isPinned;
      pinBtn.setAttribute("aria-pressed", String(isPinned));
      pinBtn.title = isPinned ? "Unpin (close after sending)" : "Pin (keep open after sending)";
      Object.assign(pinBtn.style, {
        background: isPinned ? "rgba(59, 130, 246, 0.3)" : "transparent",
        border: "1px solid " + (isPinned ? "rgba(59, 130, 246, 0.5)" : "rgba(255, 255, 255, 0.1)"),
        color: isPinned ? "#60a5fa" : "#999"
      });
    });
    headerRight.appendChild(pinBtn);
    const closeBtn = document.createElement("button");
    closeBtn.textContent = "\xD7";
    closeBtn.title = "Close";
    closeBtn.setAttribute("aria-label", "Close chat widget");
    Object.assign(closeBtn.style, {
      background: "transparent",
      border: "none",
      color: "#999",
      fontSize: "16px",
      cursor: "pointer",
      padding: "0 2px",
      lineHeight: "1"
    });
    closeBtn.addEventListener("click", (e) => {
      e.stopPropagation();
      removeChatWidget();
    });
    headerRight.appendChild(closeBtn);
    header.appendChild(headerRight);
    widget.appendChild(header);
    const inputWrap = document.createElement("div");
    Object.assign(inputWrap.style, { padding: "12px 14px" });
    const input = document.createElement("textarea");
    input.id = INPUT_ID;
    input.placeholder = "Type a message to push...";
    input.rows = 2;
    input.maxLength = 1e4;
    input.setAttribute("aria-label", "Message to push");
    Object.assign(input.style, {
      width: "100%",
      background: "rgba(255, 255, 255, 0.06)",
      border: "1px solid rgba(255, 255, 255, 0.1)",
      borderRadius: "8px",
      color: "#e0e0e0",
      fontSize: "13px",
      lineHeight: "1.5",
      padding: "10px 12px",
      resize: "none",
      outline: "none",
      fontFamily: "inherit",
      boxSizing: "border-box",
      minHeight: "44px",
      maxHeight: "120px",
      transition: "border-color 0.15s ease"
    });
    input.addEventListener("focus", () => {
      input.style.borderColor = "rgba(59, 130, 246, 0.5)";
    });
    input.addEventListener("blur", () => {
      input.style.borderColor = "rgba(255, 255, 255, 0.1)";
    });
    input.addEventListener("keydown", (e) => {
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendChatMessage();
      } else if (e.key === "Escape") {
        e.preventDefault();
        e.stopImmediatePropagation();
        removeChatWidget();
      }
    });
    inputWrap.appendChild(input);
    widget.appendChild(inputWrap);
    const footer = document.createElement("div");
    Object.assign(footer.style, {
      display: "flex",
      alignItems: "center",
      justifyContent: "space-between",
      padding: "0 14px 10px",
      fontSize: "11px",
      color: "#999"
    });
    const status = document.createElement("span");
    status.id = STATUS_ID;
    status.setAttribute("aria-live", "polite");
    status.textContent = "";
    footer.appendChild(status);
    const hint = document.createElement("span");
    hint.textContent = "Enter send | Shift+Enter newline | Esc close";
    Object.assign(hint.style, { color: "#aaa" });
    footer.appendChild(hint);
    widget.appendChild(footer);
    const target = document.body || document.documentElement;
    if (!target)
      return;
    target.appendChild(widget);
    if (chatEscapeHandler) {
      document.removeEventListener("keydown", chatEscapeHandler);
    }
    chatEscapeHandler = (e) => {
      if (e.key === "Escape") {
        e.stopImmediatePropagation();
        removeChatWidget();
      }
    };
    document.addEventListener("keydown", chatEscapeHandler, { capture: true });
    const focusable = [input, pinBtn, closeBtn];
    widget.addEventListener("keydown", (e) => {
      if (e.key !== "Tab")
        return;
      const focused = document.activeElement;
      if (!focused)
        return;
      const idx = focusable.indexOf(focused);
      if (idx < 0)
        return;
      e.preventDefault();
      const next = e.shiftKey ? (idx - 1 + focusable.length) % focusable.length : (idx + 1) % focusable.length;
      const el = focusable[next];
      if (el)
        el.focus();
    });
    requestAnimationFrame(() => {
      widget.style.opacity = "1";
      widget.style.transform = "translateY(0)";
      input.focus();
    });
  }
  function removeChatWidget() {
    if (isRemoving)
      return;
    const widget = document.getElementById(WIDGET_ID);
    if (!widget)
      return;
    isRemoving = true;
    widget.style.opacity = "0";
    widget.style.transform = "translateY(10px)";
    setTimeout(() => {
      widget.remove();
      isRemoving = false;
    }, 150);
    if (chatEscapeHandler) {
      document.removeEventListener("keydown", chatEscapeHandler, { capture: true });
      chatEscapeHandler = null;
    }
  }
  function sendChatMessage() {
    const input = document.getElementById(INPUT_ID);
    if (!input)
      return;
    const message = input.value.trim();
    if (!message) {
      input.style.borderColor = "rgba(239, 68, 68, 0.5)";
      setTimeout(() => {
        input.style.borderColor = "rgba(59, 130, 246, 0.5)";
      }, 600);
      return;
    }
    const statusEl = document.getElementById(STATUS_ID);
    input.disabled = true;
    if (statusEl) {
      statusEl.textContent = "Sending...";
      statusEl.style.color = "#60a5fa";
    }
    chrome.runtime.sendMessage({
      type: "GASOLINE_PUSH_CHAT",
      message,
      page_url: window.location.href
    }, (response) => {
      if (chrome.runtime.lastError || !response?.success) {
        if (statusEl) {
          statusEl.textContent = response?.error || "Send failed";
          statusEl.style.color = "#f87171";
        }
        input.disabled = false;
        return;
      }
      if (statusEl) {
        const deliveryText = response.status === "delivered" ? "Sent" : "Queued";
        statusEl.textContent = deliveryText;
        statusEl.style.color = "#22c55e";
      }
      input.value = "";
      input.disabled = false;
      if (!isPinned) {
        setTimeout(() => removeChatWidget(), 1200);
      } else {
        input.focus();
        setTimeout(() => {
          if (statusEl) {
            statusEl.textContent = "";
          }
        }, 2e3);
      }
    });
  }

  // extension/content/runtime-message-listener.js
  var actionToastsEnabled = true;
  var subtitlesEnabled = true;
  function initRuntimeMessageListener() {
    chrome.storage.local.get(["actionToastsEnabled", "subtitlesEnabled"], (result) => {
      if (result.actionToastsEnabled !== void 0)
        actionToastsEnabled = result.actionToastsEnabled;
      if (result.subtitlesEnabled !== void 0)
        subtitlesEnabled = result.subtitlesEnabled;
    });
    const syncHandlers = {
      GASOLINE_PING: () => {
      },
      GASOLINE_ACTION_TOAST: (msg) => {
        if (!actionToastsEnabled)
          return false;
        const m = msg;
        if (m.text)
          showActionToast(m.text, m.detail, m.state || "trying", m.duration_ms);
        return false;
      },
      GASOLINE_TOGGLE_CHAT: (msg) => {
        toggleChatWidget(msg.client_name);
        return false;
      },
      GASOLINE_RECORDING_WATERMARK: (msg) => {
        toggleRecordingWatermark(msg.visible ?? false);
        return false;
      },
      GASOLINE_SUBTITLE: (msg) => {
        if (!subtitlesEnabled)
          return false;
        showSubtitle(msg.text ?? "");
        return false;
      },
      [SettingName.ACTION_TOASTS]: (msg) => {
        actionToastsEnabled = msg.enabled;
        return false;
      },
      [SettingName.SUBTITLES]: (msg) => {
        subtitlesEnabled = msg.enabled;
        return false;
      }
    };
    const delegatedHandlers = {
      GASOLINE_DRAW_MODE_START: (msg, sr) => {
        const m = msg;
        import(
          /* webpackIgnore: true */
          chrome.runtime.getURL("content/draw-mode.js")
        ).then((mod) => {
          const result = mod.activateDrawMode(m.started_by || "user", m.annot_session_name || "", m.correlation_id || "");
          sr(result);
        }).catch((e) => sr({ error: "draw_mode_load_failed", message: e.message }));
        return true;
      },
      GASOLINE_DRAW_MODE_STOP: (_msg, sr) => {
        import(
          /* webpackIgnore: true */
          chrome.runtime.getURL("content/draw-mode.js")
        ).then((mod) => {
          const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.();
          sr(result || { status: "stopped" });
        }).catch((e) => sr({ error: "draw_mode_load_failed", message: e.message }));
        return true;
      },
      GASOLINE_GET_ANNOTATIONS: (_msg, sr) => {
        import(
          /* webpackIgnore: true */
          chrome.runtime.getURL("content/draw-mode.js")
        ).then((mod) => {
          sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false });
        }).catch(() => sr({ draw_mode_active: false }));
        return true;
      },
      GASOLINE_HIGHLIGHT: (msg, sr) => {
        forwardHighlightMessage({ params: msg.params }).then((r) => sr(r)).catch((e) => sr({ success: false, error: e.message }));
        return true;
      },
      GASOLINE_MANAGE_STATE: (msg, sr) => {
        handleStateCommand(msg.params).then((r) => sr(r)).catch((e) => sr({ error: e.message }));
        return true;
      },
      GASOLINE_EXECUTE_JS: (msg, sr) => handleExecuteJs(msg.params || {}, sr),
      GASOLINE_EXECUTE_QUERY: (msg, sr) => handleExecuteQuery(msg.params || {}, sr),
      A11Y_QUERY: (msg, sr) => handleA11yQuery(msg.params || {}, sr),
      DOM_QUERY: (msg, sr) => handleDomQuery(msg.params || {}, sr),
      GET_NETWORK_WATERFALL: (_msg, sr) => handleGetNetworkWaterfall(sr),
      LINK_HEALTH_QUERY: (msg, sr) => handleLinkHealthQuery(msg.params ?? {}, sr),
      COMPUTED_STYLES_QUERY: (msg, sr) => handleComputedStylesQuery(msg.params ?? {}, sr),
      FORM_DISCOVERY_QUERY: (msg, sr) => handleFormDiscoveryQuery(msg.params ?? {}, sr),
      FORM_STATE_QUERY: (msg, sr) => handleFormStateQuery(msg.params ?? {}, sr),
      DATA_TABLE_QUERY: (msg, sr) => handleDataTableQuery(msg.params ?? {}, sr),
      GASOLINE_GET_READABLE: (_msg, sr) => handleGetReadable(sr),
      GASOLINE_GET_MARKDOWN: (_msg, sr) => handleGetMarkdown(sr),
      GASOLINE_PAGE_SUMMARY: (_msg, sr) => handlePageSummary(sr)
    };
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      if (!isValidBackgroundSender(sender)) {
        console.warn("[Gasoline] Rejected message from untrusted sender:", sender.id);
        return false;
      }
      if (message.type === "GASOLINE_PING")
        return handlePing(sendResponse);
      const syncHandler = syncHandlers[message.type];
      if (syncHandler) {
        syncHandler(message);
        return false;
      }
      handleToggleMessage(message);
      const delegated = delegatedHandlers[message.type];
      if (delegated)
        return delegated(message, sendResponse);
      return void 0;
    });
  }

  // extension/content/favicon-replacer.js
  var originalFaviconHref = null;
  var flickerInterval = null;
  function initFaviconReplacer() {
    chrome.runtime.onMessage.addListener((message, sender, _sendResponse) => {
      if (sender.id !== chrome.runtime.id)
        return false;
      if (message.type === "trackingStateChanged") {
        const newState = message.state;
        updateFavicon(newState);
      }
      return false;
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

  // extension/content/ui/tracked-hover-launcher.js
  var ROOT_ID = "gasoline-tracked-hover-launcher";
  var PANEL_ID = "gasoline-tracked-hover-panel";
  var TOGGLE_ID = "gasoline-tracked-hover-toggle";
  var rootEl = null;
  var panelEl = null;
  var recordButtonEl = null;
  var recordingActive = false;
  var panelPinned = false;
  var hideTimer = null;
  var recordingStorageListener = null;
  function clearHideTimer() {
    if (!hideTimer)
      return;
    clearTimeout(hideTimer);
    hideTimer = null;
  }
  function setPanelOpen(open) {
    if (!panelEl)
      return;
    panelEl.style.opacity = open ? "1" : "0";
    panelEl.style.transform = open ? "translateX(0)" : "translateX(8px)";
    panelEl.style.pointerEvents = open ? "auto" : "none";
  }
  function updateRecordButtonState(active) {
    recordingActive = active;
    if (!recordButtonEl)
      return;
    recordButtonEl.textContent = active ? "Stop" : "Rec";
    recordButtonEl.title = active ? "Stop recording" : "Start recording";
    recordButtonEl.style.background = active ? "#c0392b" : "#f3f4f6";
    recordButtonEl.style.color = active ? "#fff" : "#1f2937";
    recordButtonEl.style.borderColor = active ? "#a93226" : "#d1d5db";
  }
  function readRecordingActive(value) {
    if (!value || typeof value !== "object")
      return false;
    return Boolean(value.active);
  }
  function syncRecordingStateFromStorage() {
    chrome.storage.local.get([StorageKey.RECORDING], (result) => {
      updateRecordButtonState(readRecordingActive(result[StorageKey.RECORDING]));
    });
  }
  function installRecordingStorageSync() {
    if (recordingStorageListener)
      return;
    recordingStorageListener = (changes, areaName) => {
      if (areaName !== "local")
        return;
      const recordingChange = changes[StorageKey.RECORDING];
      if (!recordingChange)
        return;
      updateRecordButtonState(readRecordingActive(recordingChange.newValue));
    };
    chrome.storage.onChanged.addListener(recordingStorageListener);
  }
  function uninstallRecordingStorageSync() {
    if (!recordingStorageListener)
      return;
    chrome.storage.onChanged.removeListener(recordingStorageListener);
    recordingStorageListener = null;
  }
  async function startDrawMode() {
    try {
      const drawModeModule = await import(
        /* webpackIgnore: true */
        chrome.runtime.getURL("content/draw-mode.js")
      );
      if (typeof drawModeModule.activateDrawMode === "function") {
        drawModeModule.activateDrawMode("user");
      }
    } catch {
    }
  }
  function runScreenshotCapture() {
    chrome.runtime.sendMessage({ type: "captureScreenshot" }, () => {
      void chrome.runtime.lastError;
    });
  }
  function toggleRecordingAction() {
    const wasActive = recordingActive;
    const message = wasActive ? { type: "record_stop" } : { type: "record_start", audio: "" };
    const button = recordButtonEl;
    if (button)
      button.disabled = true;
    chrome.runtime.sendMessage(message, (response) => {
      if (button)
        button.disabled = false;
      if (chrome.runtime.lastError)
        return;
      const responseStatus = response?.status;
      if (wasActive) {
        if (responseStatus !== "saved") {
          syncRecordingStateFromStorage();
          return;
        }
        updateRecordButtonState(false);
        return;
      }
      if (responseStatus === "recording") {
        updateRecordButtonState(true);
        return;
      }
      syncRecordingStateFromStorage();
    });
  }
  function createActionButton(label, title, onClick) {
    const button = document.createElement("button");
    button.textContent = label;
    button.title = title;
    button.type = "button";
    Object.assign(button.style, {
      height: "34px",
      minWidth: "54px",
      borderRadius: "10px",
      border: "1px solid #d1d5db",
      background: "#f3f4f6",
      color: "#1f2937",
      fontSize: "12px",
      fontWeight: "600",
      cursor: "pointer",
      padding: "0 10px"
    });
    button.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      onClick();
    });
    return button;
  }
  function createLauncherUi() {
    const root = document.createElement("div");
    root.id = ROOT_ID;
    Object.assign(root.style, {
      position: "fixed",
      top: "18px",
      right: "18px",
      zIndex: "2147483643",
      display: "flex",
      alignItems: "center",
      gap: "8px",
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif'
    });
    const panel = document.createElement("div");
    panel.id = PANEL_ID;
    Object.assign(panel.style, {
      display: "flex",
      alignItems: "center",
      gap: "6px",
      padding: "7px",
      borderRadius: "18px",
      background: "#ffffff",
      border: "1px solid rgba(15, 23, 42, 0.12)",
      boxShadow: "0 8px 24px rgba(15, 23, 42, 0.2)",
      opacity: "0",
      transform: "translateX(8px)",
      transition: "opacity 0.16s ease, transform 0.16s ease",
      pointerEvents: "none"
    });
    const drawButton = createActionButton("Draw", "Start annotation draw mode", () => {
      panelPinned = false;
      setPanelOpen(false);
      void startDrawMode();
    });
    const recordButton = createActionButton("Rec", "Start recording", () => {
      panelPinned = true;
      toggleRecordingAction();
    });
    recordButtonEl = recordButton;
    const screenshotButton = createActionButton("Shot", "Capture screenshot", () => {
      panelPinned = false;
      setPanelOpen(false);
      runScreenshotCapture();
    });
    panel.appendChild(drawButton);
    panel.appendChild(recordButton);
    panel.appendChild(screenshotButton);
    const toggle = document.createElement("button");
    toggle.id = TOGGLE_ID;
    toggle.type = "button";
    toggle.textContent = "G";
    toggle.title = "Gasoline quick actions";
    Object.assign(toggle.style, {
      width: "44px",
      height: "44px",
      borderRadius: "22px",
      border: "2px solid #2563eb",
      background: "#ffffff",
      color: "#1d4ed8",
      fontSize: "16px",
      fontWeight: "700",
      cursor: "pointer",
      boxShadow: "0 8px 24px rgba(15, 23, 42, 0.25)"
    });
    toggle.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      panelPinned = !panelPinned;
      clearHideTimer();
      setPanelOpen(panelPinned);
    });
    root.addEventListener("mouseenter", () => {
      clearHideTimer();
      setPanelOpen(true);
    });
    root.addEventListener("mouseleave", () => {
      if (panelPinned)
        return;
      clearHideTimer();
      hideTimer = setTimeout(() => setPanelOpen(false), 120);
    });
    root.appendChild(panel);
    root.appendChild(toggle);
    panelEl = panel;
    syncRecordingStateFromStorage();
    return root;
  }
  function mountLauncher() {
    if (rootEl || document.getElementById(ROOT_ID))
      return;
    rootEl = createLauncherUi();
    const target = document.body || document.documentElement;
    if (!target || !rootEl)
      return;
    target.appendChild(rootEl);
    installRecordingStorageSync();
  }
  function unmountLauncher() {
    clearHideTimer();
    panelPinned = false;
    panelEl = null;
    recordButtonEl = null;
    if (rootEl) {
      rootEl.remove();
      rootEl = null;
    }
    uninstallRecordingStorageSync();
  }
  function setTrackedHoverLauncherEnabled(enabled) {
    if (enabled) {
      mountLauncher();
      return;
    }
    unmountLauncher();
  }

  // extension/content.js
  var scriptsInjected = false;
  initTabTracking((tracked) => {
    if (tracked && !scriptsInjected) {
      initScriptInjection();
      scriptsInjected = true;
    }
    setTrackedHoverLauncherEnabled(tracked);
  });
  initRequestTracking();
  initWindowMessageListener();
  initRuntimeMessageListener();
  initFaviconReplacer();
})();
//# sourceMappingURL=content.bundled.js.map
