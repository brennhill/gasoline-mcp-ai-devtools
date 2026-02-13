// extension/lib/constants.js
var MAX_STRING_LENGTH = 10240;
var MAX_RESPONSE_LENGTH = 5120;
var MAX_DEPTH = 10;
var MAX_CONTEXT_SIZE = 50;
var MAX_CONTEXT_VALUE_SIZE = 4096;
var SENSITIVE_HEADERS = [
  "authorization",
  "cookie",
  "set-cookie",
  "x-auth-token",
  "x-api-key",
  "x-csrf-token",
  "proxy-authorization"
];
var MAX_ACTION_BUFFER_SIZE = 20;
var SCROLL_THROTTLE_MS = 250;
var SENSITIVE_INPUT_TYPES = ["password", "credit-card", "cc-number", "cc-exp", "cc-csc"];
var MAX_WATERFALL_ENTRIES = 50;
var WATERFALL_TIME_WINDOW_MS = 3e4;
var MAX_PERFORMANCE_ENTRIES = 50;
var PERFORMANCE_TIME_WINDOW_MS = 6e4;
var WS_MAX_BODY_SIZE = 4096;
var WS_PREVIEW_LIMIT = 200;
var REQUEST_BODY_MAX = 8192;
var RESPONSE_BODY_MAX = 16384;
var BODY_READ_TIMEOUT_MS = 5;
var SENSITIVE_HEADER_PATTERNS = /^(authorization|cookie|set-cookie|x-api-key|x-auth-token|x-secret|x-password|.*token.*|.*secret.*|.*key.*|.*password.*)$/i;
var BINARY_CONTENT_TYPES = /^(image|video|audio|font)\/|^application\/(wasm|octet-stream|zip|gzip|pdf)/;
var DOM_QUERY_MAX_ELEMENTS = 50;
var DOM_QUERY_MAX_TEXT = 500;
var DOM_QUERY_MAX_DEPTH = 5;
var DOM_QUERY_MAX_HTML = 200;
var A11Y_MAX_NODES_PER_VIOLATION = 10;
var ASYNC_COMMAND_TIMEOUT_MS = 6e4;
var A11Y_AUDIT_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS;
var MEMORY_SOFT_LIMIT_MB = 20;
var MEMORY_HARD_LIMIT_MB = 50;
var AI_CONTEXT_SNIPPET_LINES = 5;
var AI_CONTEXT_MAX_LINE_LENGTH = 200;
var AI_CONTEXT_MAX_SNIPPETS_SIZE = 10240;
var AI_CONTEXT_MAX_ANCESTRY_DEPTH = 10;
var AI_CONTEXT_MAX_PROP_KEYS = 20;
var AI_CONTEXT_MAX_STATE_KEYS = 10;
var AI_CONTEXT_MAX_RELEVANT_SLICE = 10;
var AI_CONTEXT_MAX_VALUE_LENGTH = 200;
var AI_CONTEXT_SOURCE_MAP_CACHE_SIZE = 20;
var AI_CONTEXT_PIPELINE_TIMEOUT_MS = 3e3;
var ENHANCED_ACTION_BUFFER_SIZE = 50;
var CSS_PATH_MAX_DEPTH = 5;
var SELECTOR_TEXT_MAX_LENGTH = 50;
var SCRIPT_MAX_SIZE = 51200;
var CLICKABLE_TAGS = /* @__PURE__ */ new Set(["BUTTON", "A", "SUMMARY"]);
var ACTIONABLE_KEYS = /* @__PURE__ */ new Set([
  "Enter",
  "Escape",
  "Tab",
  "ArrowUp",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "Backspace",
  "Delete"
]);
var MAX_LONG_TASKS = 50;
var MAX_SLOWEST_REQUESTS = 3;
var MAX_URL_LENGTH = 80;

// extension/lib/serialize.js
function serializePrimitive(value, type) {
  if (type === "string") {
    const s = value;
    return s.length > MAX_STRING_LENGTH ? s.slice(0, MAX_STRING_LENGTH) + "... [truncated]" : s;
  }
  if (type === "number")
    return value;
  if (type === "boolean")
    return value;
  if (type === "function")
    return `[Function: ${value.name || "anonymous"}]`;
  return void 0;
}
function serializeDOMNode(value) {
  const tag = value.tagName ? value.tagName.toLowerCase() : "node";
  const id = value.id ? `#${value.id}` : "";
  const cn = value.className;
  const className = typeof cn === "string" && cn ? `.${cn.split(" ").join(".")}` : "";
  return `[${tag}${id}${className}]`;
}
function serializeObject(value, depth, seen) {
  if (seen.has(value))
    return "[Circular]";
  seen.add(value);
  if (value.nodeType)
    return serializeDOMNode(value);
  if (Array.isArray(value))
    return value.slice(0, 100).map((item) => safeSerialize(item, depth + 1, seen));
  const result = {};
  for (const key of Object.keys(value).slice(0, 50)) {
    try {
      result[key] = safeSerialize(value[key], depth + 1, seen);
    } catch {
      result[key] = "[Unserializable]";
    }
  }
  return result;
}
function safeSerialize(value, depth = 0, seen = /* @__PURE__ */ new WeakSet()) {
  if (value === null || value === void 0)
    return null;
  const type = typeof value;
  const primitive = serializePrimitive(value, type);
  if (primitive !== void 0)
    return primitive;
  if (value instanceof Error) {
    return { name: value.name, message: value.message, stack: value.stack || null };
  }
  if (depth >= MAX_DEPTH)
    return "[Max depth exceeded]";
  if (type === "object")
    return serializeObject(value, depth, seen);
  return String(value);
}
function getElementSelector(element) {
  if (!element || !element.tagName)
    return "";
  const tag = element.tagName.toLowerCase();
  const id = element.id ? `#${element.id}` : "";
  let classes = "";
  const classNameValue = element.className;
  if (classNameValue && typeof classNameValue === "string") {
    classes = "." + classNameValue.trim().split(/\s+/).slice(0, 2).join(".");
  }
  const testId = element.getAttribute("data-testid");
  const testIdStr = testId ? `[data-testid="${testId}"]` : "";
  return `${tag}${id}${classes}${testIdStr}`.slice(0, 100);
}
var SENSITIVE_AUTOCOMPLETE_PATTERNS = ["password", "cc-", "credit-card"];
var SENSITIVE_NAME_PATTERNS = ["password", "passwd", "secret", "token", "credit", "card", "cvv", "cvc", "ssn"];
function matchesAny(value, patterns) {
  return patterns.some((p) => value.includes(p));
}
function isSensitiveInput(element) {
  if (!element)
    return false;
  const inputElement = element;
  const type = (inputElement.type || "").toLowerCase();
  const autocomplete = (inputElement.autocomplete || "").toLowerCase();
  const name = (inputElement.name || "").toLowerCase();
  return SENSITIVE_INPUT_TYPES.includes(type) || matchesAny(autocomplete, SENSITIVE_AUTOCOMPLETE_PATTERNS) || matchesAny(name, SENSITIVE_NAME_PATTERNS);
}

// extension/lib/context.js
var contextAnnotations = /* @__PURE__ */ new Map();
function getContextAnnotations() {
  if (contextAnnotations.size === 0)
    return null;
  const result = {};
  for (const [key, value] of contextAnnotations) {
    result[key] = value;
  }
  return result;
}
function setContextAnnotation(key, value) {
  if (typeof key !== "string" || key.length === 0) {
    console.warn("[Gasoline] annotate() requires a non-empty string key");
    return false;
  }
  if (key.length > 100) {
    console.warn("[Gasoline] annotate() key must be 100 characters or less");
    return false;
  }
  if (!contextAnnotations.has(key) && contextAnnotations.size >= MAX_CONTEXT_SIZE) {
    console.warn(`[Gasoline] Maximum context annotations (${MAX_CONTEXT_SIZE}) reached`);
    return false;
  }
  const serialized = safeSerialize(value);
  const serializedStr = JSON.stringify(serialized);
  if (serializedStr.length > MAX_CONTEXT_VALUE_SIZE) {
    console.warn(`[Gasoline] Context value for "${key}" exceeds max size, truncating`);
    contextAnnotations.set(key, "[Value too large]");
    return false;
  }
  contextAnnotations.set(key, serialized);
  return true;
}
function removeContextAnnotation(key) {
  return contextAnnotations.delete(key);
}
function clearContextAnnotations() {
  contextAnnotations.clear();
}

// extension/lib/reproduction.js
var enhancedActionBuffer = [];
var TAG_TO_ROLE = {
  button: "button",
  textarea: "textbox",
  select: "combobox",
  nav: "navigation",
  main: "main",
  header: "banner",
  footer: "contentinfo"
};
var INPUT_TYPE_TO_ROLE = {
  text: "textbox",
  email: "textbox",
  password: "textbox",
  tel: "textbox",
  url: "textbox",
  checkbox: "checkbox",
  radio: "radio",
  search: "searchbox",
  number: "spinbutton",
  range: "slider"
};
function getImplicitRole(element) {
  if (!element || !element.tagName)
    return null;
  const tag = element.tagName.toLowerCase();
  const el = element;
  if (tag === "a") {
    return el.getAttribute && el.getAttribute("href") !== null ? "link" : null;
  }
  if (tag === "input") {
    const type = el.getAttribute ? el.getAttribute("type") : null;
    return INPUT_TYPE_TO_ROLE[type || "text"] ?? "textbox";
  }
  return TAG_TO_ROLE[tag] ?? null;
}
function isDynamicClass(className) {
  if (!className)
    return false;
  if (/^(css|sc|emotion|styled|chakra)-/.test(className))
    return true;
  if (/^[a-z]{5,8}$/.test(className))
    return true;
  return false;
}
function computeCssPath(element) {
  if (!element)
    return "";
  const parts = [];
  let current = element;
  while (current && parts.length < CSS_PATH_MAX_DEPTH) {
    let selector = current.tagName ? current.tagName.toLowerCase() : "";
    if (current.id) {
      selector = `#${current.id}`;
      parts.unshift(selector);
      break;
    }
    const classNameValue = current.className;
    const classList = classNameValue && typeof classNameValue === "string" ? classNameValue.trim().split(/\s+/).filter((c) => c && !isDynamicClass(c)) : [];
    if (classList.length > 0) {
      selector += "." + classList.slice(0, 2).join(".");
    }
    parts.unshift(selector);
    current = current.parentElement;
  }
  return parts.join(" > ");
}
function computeSelectors(element) {
  if (!element)
    return { cssPath: "" };
  const selectors = {};
  const el = element;
  const testId = el.getAttribute && (el.getAttribute("data-testid") || el.getAttribute("data-test-id") || el.getAttribute("data-cy")) || void 0;
  if (testId)
    selectors.testId = testId;
  const ariaLabel = el.getAttribute && el.getAttribute("aria-label");
  if (ariaLabel)
    selectors.ariaLabel = ariaLabel;
  const explicitRole = el.getAttribute && el.getAttribute("role");
  const role = explicitRole || getImplicitRole(element);
  const name = ariaLabel || el.textContent && el.textContent.trim().slice(0, SELECTOR_TEXT_MAX_LENGTH);
  if (role && name) {
    selectors.role = { role, name: ariaLabel || name };
  }
  if (element.id)
    selectors.id = element.id;
  const isClickable = element.tagName && CLICKABLE_TAGS.has(element.tagName.toUpperCase()) || el.getAttribute && el.getAttribute("role") === "button";
  if (isClickable) {
    const text = (el.textContent || el.innerText || "").trim();
    if (text)
      selectors.text = text.slice(0, SELECTOR_TEXT_MAX_LENGTH);
  }
  selectors.cssPath = computeCssPath(element);
  return selectors;
}
var ACTION_DATA_ENRICHERS = {
  input: (a, el, o) => {
    const typedEl = el;
    const inputType = typedEl && typedEl.getAttribute ? typedEl.getAttribute("type") : "text";
    a.input_type = inputType || "text";
    a.value = inputType === "password" || el && isSensitiveInput(el) ? "[redacted]" : o.value || "";
  },
  keypress: (a, _el, o) => {
    a.key = o.key || "";
  },
  navigate: (a, _el, o) => {
    a.from_url = o.from_url || "";
    a.to_url = o.to_url || "";
  },
  select: (a, _el, o) => {
    a.selected_value = o.selected_value || "";
    a.selected_text = o.selected_text || "";
  },
  scroll: (a, _el, o) => {
    a.scroll_y = o.scroll_y || 0;
  }
};
function recordEnhancedAction(type, element, opts = {}) {
  const action = {
    type,
    timestamp: Date.now(),
    url: typeof window !== "undefined" && window.location ? window.location.href : ""
  };
  if (element) {
    action.selectors = computeSelectors(element);
  }
  const enricher = ACTION_DATA_ENRICHERS[type];
  if (enricher)
    enricher(action, element, opts);
  enhancedActionBuffer.push(action);
  if (enhancedActionBuffer.length > ENHANCED_ACTION_BUFFER_SIZE) {
    enhancedActionBuffer.shift();
  }
  if (typeof window !== "undefined" && window.postMessage) {
    window.postMessage({ type: "GASOLINE_ENHANCED_ACTION", payload: action }, window.location.origin);
  }
  return action;
}
function getEnhancedActionBuffer() {
  return [...enhancedActionBuffer];
}
function clearEnhancedActionBuffer() {
  enhancedActionBuffer = [];
}
function rebaseUrl(url, baseUrl) {
  if (!baseUrl || !url)
    return url;
  try {
    return baseUrl + new URL(url).pathname;
  } catch {
    return url;
  }
}
var ACTION_STEP_GENERATORS = {
  click: (_action, locator) => locator ? `  await page.${locator}.click();` : `  // click action - no selector available`,
  input: (action, locator) => {
    if (!locator)
      return null;
    const value = action.value === "[redacted]" ? "[user-provided]" : action.value || "";
    return `  await page.${locator}.fill('${escapeString(value)}');`;
  },
  keypress: (action) => `  await page.keyboard.press('${escapeString(action.key || "")}');`,
  navigate: (action, _locator, baseUrl) => `  await page.waitForURL('${escapeString(rebaseUrl(action.to_url || "", baseUrl))}');`,
  select: (action, locator) => locator ? `  await page.${locator}.selectOption('${escapeString(action.selected_value || "")}');` : null,
  scroll: (action) => `  // User scrolled to y=${action.scroll_y || 0}`
};
function actionToPlaywrightStep(action, baseUrl) {
  const locator = getPlaywrightLocator(action.selectors || { cssPath: "" });
  const generator = ACTION_STEP_GENERATORS[action.type];
  return generator ? generator(action, locator, baseUrl) : null;
}
function generatePlaywrightScript(actions, opts = {}) {
  const { errorMessage, baseUrl, lastNActions } = opts;
  let filteredActions = actions;
  if (lastNActions && lastNActions > 0 && actions.length > lastNActions) {
    filteredActions = actions.slice(-lastNActions);
  }
  let startUrl = "";
  if (filteredActions.length > 0) {
    const firstAction = filteredActions[0];
    if (firstAction) {
      startUrl = firstAction.url || "";
    }
  }
  if (baseUrl && startUrl) {
    try {
      const parsed = new URL(startUrl);
      startUrl = baseUrl + parsed.pathname;
    } catch {
      startUrl = baseUrl;
    }
  }
  const testName = errorMessage ? `reproduction: ${errorMessage.slice(0, 80)}` : "reproduction: captured user actions";
  const steps = [];
  let prevTimestamp = null;
  for (const action of filteredActions) {
    if (prevTimestamp && action.timestamp - prevTimestamp > 2e3) {
      const gap = Math.round((action.timestamp - prevTimestamp) / 1e3);
      steps.push(`  // [${gap}s pause]`);
    }
    prevTimestamp = action.timestamp;
    const step = actionToPlaywrightStep(action, baseUrl);
    if (step)
      steps.push(step);
  }
  let script = `import { test, expect } from '@playwright/test';

`;
  script += `test('${escapeString(testName)}', async ({ page }) => {
`;
  if (startUrl) {
    script += `  await page.goto('${escapeString(startUrl)}');

`;
  }
  script += steps.join("\n");
  if (steps.length > 0)
    script += "\n";
  if (errorMessage) {
    script += `
  // Error occurred here: ${errorMessage}
`;
  }
  script += `});
`;
  if (script.length > SCRIPT_MAX_SIZE) {
    script = script.slice(0, SCRIPT_MAX_SIZE);
  }
  return script;
}
function getPlaywrightLocator(selectors) {
  if (selectors.testId)
    return `getByTestId('${escapeString(selectors.testId)}')`;
  if (selectors.role && selectors.role.role) {
    const escaped = escapeString(selectors.role.role);
    return selectors.role.name ? `getByRole('${escaped}', { name: '${escapeString(selectors.role.name)}' })` : `getByRole('${escaped}')`;
  }
  if (selectors.ariaLabel)
    return `getByLabel('${escapeString(selectors.ariaLabel)}')`;
  if (selectors.text)
    return `getByText('${escapeString(selectors.text)}')`;
  if (selectors.id)
    return `locator('#${escapeString(selectors.id)}')`;
  if (selectors.cssPath)
    return `locator('${escapeString(selectors.cssPath)}')`;
  return null;
}
function escapeString(str) {
  if (!str)
    return "";
  return str.replace(/\\/g, "\\\\").replace(/'/g, "\\'").replace(/\n/g, "\\n").replace(/\r/g, "\\r").replace(/\t/g, "\\t").replace(/`/g, "\\`");
}

// extension/lib/actions.js
var actionBuffer = [];
var lastScrollTime = 0;
var actionCaptureEnabled = true;
var clickHandler = null;
var inputHandler = null;
var scrollHandler = null;
var keydownHandler = null;
var changeHandler = null;
function recordAction(action) {
  if (!actionCaptureEnabled)
    return;
  actionBuffer.push({
    ts: (/* @__PURE__ */ new Date()).toISOString(),
    ...action
  });
  if (actionBuffer.length > MAX_ACTION_BUFFER_SIZE) {
    actionBuffer.shift();
  }
}
function getActionBuffer() {
  return [...actionBuffer];
}
function clearActionBuffer() {
  actionBuffer = [];
}
function handleClick(event) {
  const target = event.target;
  if (!target)
    return;
  const action = {
    type: "click",
    target: getElementSelector(target),
    x: event.clientX,
    y: event.clientY
  };
  const text = target.textContent || target.innerText || "";
  if (text && text.length > 0) {
    action.text = text.trim().slice(0, 50);
  }
  recordAction(action);
  recordEnhancedAction("click", target);
}
function handleInput(event) {
  const target = event.target;
  if (!target)
    return;
  const action = {
    type: "input",
    target: getElementSelector(target),
    inputType: target.type || "text"
  };
  if (!isSensitiveInput(target)) {
    const value = target.value || "";
    action.value = value.slice(0, 100);
    action.length = value.length;
  } else {
    action.value = "[redacted]";
    action.length = (target.value || "").length;
  }
  recordAction(action);
  recordEnhancedAction("input", target, { value: action.value });
}
function handleScroll(event) {
  const now = Date.now();
  if (now - lastScrollTime < SCROLL_THROTTLE_MS)
    return;
  lastScrollTime = now;
  const target = event.target;
  recordAction({
    type: "scroll",
    scrollX: Math.round(window.scrollX),
    scrollY: Math.round(window.scrollY),
    target: target === document ? "document" : getElementSelector(target)
  });
  recordEnhancedAction("scroll", null, { scroll_y: Math.round(window.scrollY) });
}
function handleKeydown(event) {
  if (!ACTIONABLE_KEYS.has(event.key))
    return;
  const target = event.target;
  recordEnhancedAction("keypress", target, { key: event.key });
}
function handleChange(event) {
  const target = event.target;
  if (!target || !target.tagName || target.tagName.toUpperCase() !== "SELECT")
    return;
  const selectedOption = target.options && target.options[target.selectedIndex];
  const selectedValue = target.value || "";
  const selectedText = selectedOption ? selectedOption.text || "" : "";
  recordEnhancedAction("select", target, { selected_value: selectedValue, selected_text: selectedText });
}
function installActionCapture() {
  if (typeof window === "undefined" || typeof document === "undefined")
    return;
  if (typeof document.addEventListener !== "function")
    return;
  clickHandler = handleClick;
  inputHandler = handleInput;
  scrollHandler = handleScroll;
  keydownHandler = handleKeydown;
  changeHandler = handleChange;
  document.addEventListener("click", clickHandler, { capture: true, passive: true });
  document.addEventListener("input", inputHandler, { capture: true, passive: true });
  document.addEventListener("keydown", keydownHandler, { capture: true, passive: true });
  document.addEventListener("change", changeHandler, { capture: true, passive: true });
  window.addEventListener("scroll", scrollHandler, { capture: true, passive: true });
}
function uninstallActionCapture() {
  if (clickHandler) {
    document.removeEventListener("click", clickHandler, { capture: true });
    clickHandler = null;
  }
  if (inputHandler) {
    document.removeEventListener("input", inputHandler, { capture: true });
    inputHandler = null;
  }
  if (keydownHandler) {
    document.removeEventListener("keydown", keydownHandler, { capture: true });
    keydownHandler = null;
  }
  if (changeHandler) {
    document.removeEventListener("change", changeHandler, { capture: true });
    changeHandler = null;
  }
  if (scrollHandler) {
    window.removeEventListener("scroll", scrollHandler, { capture: true });
    scrollHandler = null;
  }
  clearActionBuffer();
}
function setActionCaptureEnabled(enabled) {
  actionCaptureEnabled = enabled;
  if (!enabled) {
    clearActionBuffer();
  }
}
var navigationPopstateHandler = null;
var originalPushState = null;
var originalReplaceState = null;
function installNavigationCapture() {
  if (typeof window === "undefined")
    return;
  let lastUrl = window.location.href;
  navigationPopstateHandler = function() {
    const toUrl = window.location.href;
    recordEnhancedAction("navigate", null, { from_url: lastUrl, to_url: toUrl });
    lastUrl = toUrl;
  };
  window.addEventListener("popstate", navigationPopstateHandler);
  if (window.history && window.history.pushState) {
    originalPushState = window.history.pushState;
    window.history.pushState = function(state, title, url) {
      const fromUrl = lastUrl;
      originalPushState.call(this, state, title, url);
      const toUrl = url || window.location.href;
      recordEnhancedAction("navigate", null, { from_url: fromUrl, to_url: String(toUrl) });
      lastUrl = window.location.href;
    };
  }
  if (window.history && window.history.replaceState) {
    originalReplaceState = window.history.replaceState;
    window.history.replaceState = function(state, title, url) {
      const fromUrl = lastUrl;
      originalReplaceState.call(this, state, title, url);
      const toUrl = url || window.location.href;
      recordEnhancedAction("navigate", null, { from_url: fromUrl, to_url: String(toUrl) });
      lastUrl = window.location.href;
    };
  }
}
function uninstallNavigationCapture() {
  if (navigationPopstateHandler) {
    window.removeEventListener("popstate", navigationPopstateHandler);
    navigationPopstateHandler = null;
  }
  if (originalPushState && window.history) {
    window.history.pushState = originalPushState;
    originalPushState = null;
  }
  if (originalReplaceState && window.history) {
    window.history.replaceState = originalReplaceState;
    originalReplaceState = null;
  }
}

// extension/lib/network.js
var configuredServerUrl = "";
var networkWaterfallEnabled = false;
var pendingRequests = /* @__PURE__ */ new Map();
var requestIdCounter = 0;
var networkBodyCaptureEnabled = true;
var SENSITIVE_URL_PATTERNS = /\/(auth|login|signin|signup|token|oauth|session|api[_-]?key|password|register)\b/i;
function parseResourceTiming(timing) {
  const phases = {
    dns: Math.max(0, timing.domainLookupEnd - timing.domainLookupStart),
    connect: Math.max(0, timing.connectEnd - timing.connectStart),
    tls: timing.secureConnectionStart > 0 ? Math.max(0, timing.connectEnd - timing.secureConnectionStart) : 0,
    ttfb: Math.max(0, timing.responseStart - timing.requestStart),
    download: Math.max(0, timing.responseEnd - timing.responseStart)
  };
  const result = {
    url: timing.name,
    initiatorType: timing.initiatorType,
    startTime: timing.startTime,
    duration: timing.duration,
    phases,
    transferSize: timing.transferSize || 0,
    encodedBodySize: timing.encodedBodySize || 0,
    decodedBodySize: timing.decodedBodySize || 0
  };
  if (timing.transferSize === 0 && timing.encodedBodySize > 0) {
    ;
    result.cached = true;
  }
  return result;
}
function getNetworkWaterfall(options = {}) {
  if (typeof performance === "undefined" || !performance)
    return [];
  try {
    let entries = performance.getEntriesByType("resource") || [];
    if (options.since) {
      entries = entries.filter((e) => e.startTime >= options.since);
    }
    if (options.initiatorTypes) {
      entries = entries.filter((e) => options.initiatorTypes.includes(e.initiatorType));
    }
    entries = entries.filter((e) => !e.name.startsWith("data:"));
    entries.sort((a, b) => a.startTime - b.startTime);
    if (entries.length > MAX_WATERFALL_ENTRIES) {
      entries = entries.slice(-MAX_WATERFALL_ENTRIES);
    }
    return entries.map(parseResourceTiming);
  } catch {
    return [];
  }
}
function trackPendingRequest(request) {
  const id = `req_${++requestIdCounter}`;
  pendingRequests.set(id, {
    ...request,
    id
  });
  return id;
}
function completePendingRequest(requestId) {
  pendingRequests.delete(requestId);
}
function getPendingRequests() {
  return Array.from(pendingRequests.values());
}
function clearPendingRequests() {
  pendingRequests.clear();
}
async function getNetworkWaterfallForError(errorEntry) {
  if (!networkWaterfallEnabled)
    return null;
  const now = typeof performance !== "undefined" && performance?.now ? performance.now() : 0;
  const since = Math.max(0, now - WATERFALL_TIME_WINDOW_MS);
  const entries = getNetworkWaterfall({ since });
  const pending = getPendingRequests();
  return {
    type: "network_waterfall",
    ts: (/* @__PURE__ */ new Date()).toISOString(),
    _errorTs: errorEntry.ts,
    entries,
    pending
  };
}
function setNetworkWaterfallEnabled(enabled) {
  networkWaterfallEnabled = enabled;
}
function isNetworkWaterfallEnabled() {
  return networkWaterfallEnabled;
}
function setNetworkBodyCaptureEnabled(enabled) {
  networkBodyCaptureEnabled = enabled;
}
function isNetworkBodyCaptureEnabled() {
  return networkBodyCaptureEnabled;
}
function setServerUrl(url) {
  configuredServerUrl = url || "";
}
function shouldCaptureUrl(url) {
  if (!url)
    return true;
  if (configuredServerUrl) {
    try {
      const serverParsed = new URL(configuredServerUrl);
      const hostPort = serverParsed.host;
      if (url.includes(hostPort))
        return false;
    } catch {
    }
  }
  if (url.includes("localhost:7890") || url.includes("127.0.0.1:7890"))
    return false;
  if (url.startsWith("chrome-extension://"))
    return false;
  return true;
}
function sanitizeHeaders(headers) {
  if (!headers)
    return {};
  const result = {};
  if (headers instanceof Headers || typeof headers.forEach === "function") {
    ;
    headers.forEach((value, key) => {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value;
      }
    });
  } else if (typeof headers.entries === "function") {
    for (const [key, value] of headers.entries()) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value;
      }
    }
  } else if (typeof headers === "object") {
    for (const [key, value] of Object.entries(headers)) {
      if (!SENSITIVE_HEADER_PATTERNS.test(key)) {
        result[key] = value;
      }
    }
  }
  return result;
}
function truncateRequestBody(body) {
  if (body === null || body === void 0)
    return { body: null, truncated: false };
  if (body.length <= REQUEST_BODY_MAX)
    return { body, truncated: false };
  return { body: body.slice(0, REQUEST_BODY_MAX), truncated: true };
}
function truncateResponseBody(body) {
  if (body === null || body === void 0)
    return { body: null, truncated: false };
  if (body.length <= RESPONSE_BODY_MAX)
    return { body, truncated: false };
  return { body: body.slice(0, RESPONSE_BODY_MAX), truncated: true };
}
async function readResponseBody(response) {
  const contentType = response.headers?.get?.("content-type") || "";
  if (BINARY_CONTENT_TYPES.test(contentType)) {
    const blob = await response.blob();
    return `[Binary: ${blob.size} bytes, ${contentType}]`;
  }
  return await response.text();
}
async function readResponseBodyWithTimeout(response, timeoutMs = BODY_READ_TIMEOUT_MS) {
  return Promise.race([
    readResponseBody(response),
    new Promise((resolve) => {
      setTimeout(() => resolve("[Skipped: body read timeout]"), timeoutMs);
    })
  ]);
}
function extractFetchInfo(input, init) {
  let url = "";
  let method = "GET";
  if (typeof input === "string") {
    url = input;
  } else if (input && input.url) {
    url = input.url;
    method = input.method || "GET";
  }
  if (init) {
    method = init.method || method;
  }
  return { url, method, requestBody: init?.body || null };
}
async function readCapturedBody(url, cloned, contentType) {
  if (SENSITIVE_URL_PATTERNS.test(url))
    return "[REDACTED: auth endpoint]";
  if (!cloned)
    return "";
  if (BINARY_CONTENT_TYPES.test(contentType)) {
    const blob = await cloned.blob();
    return `[Binary: ${blob.size} bytes, ${contentType}]`;
  }
  return readResponseBodyWithTimeout(cloned);
}
function postNetworkBody(win, url, method, response, contentType, requestBody, duration, truncResp, truncReq) {
  const message = {
    type: "GASOLINE_NETWORK_BODY",
    payload: {
      url,
      method,
      status: response.status,
      contentType,
      requestBody: truncReq || (typeof requestBody === "string" ? requestBody : void 0),
      responseBody: truncResp,
      duration
    }
  };
  win.postMessage(message, window.location.origin);
}
function wrapFetchWithBodies(fetchFn) {
  return async function(input, init) {
    const { url, method, requestBody } = extractFetchInfo(input, init);
    if (!shouldCaptureUrl(url))
      return fetchFn(input, init);
    const startTime = Date.now();
    const response = await fetchFn(input, init);
    const duration = Date.now() - startTime;
    const contentType = response.headers?.get?.("content-type") || "";
    const cloned = response.clone ? response.clone() : null;
    const win = typeof window !== "undefined" ? window : null;
    Promise.resolve().then(async () => {
      try {
        const responseBody = await readCapturedBody(url, cloned, contentType);
        const { body: truncResp } = truncateResponseBody(responseBody);
        const rawReq = SENSITIVE_URL_PATTERNS.test(url) ? "[REDACTED: auth endpoint]" : typeof requestBody === "string" ? requestBody : null;
        const { body: truncReq } = truncateRequestBody(rawReq);
        if (win && networkBodyCaptureEnabled) {
          postNetworkBody(win, url, method, response, contentType, requestBody, duration, truncResp || responseBody, truncReq);
        }
      } catch {
      }
    }).catch((err) => {
      console.debug("[Gasoline] Network body capture error:", err);
    });
    return response;
  };
}

// extension/lib/perf-snapshot.js
var perfSnapshotEnabled = true;
var longTaskEntries = [];
var longTaskObserver = null;
var paintObserver = null;
var lcpObserver = null;
var clsObserver = null;
var inpObserver = null;
var fcpValue = null;
var lcpValue = null;
var clsValue = 0;
var inpValue = null;
function mapInitiatorType(type) {
  switch (type) {
    case "script":
      return "script";
    case "link":
    case "css":
      return "style";
    case "img":
      return "image";
    case "fetch":
    case "xmlhttprequest":
      return "fetch";
    case "font":
      return "font";
    default:
      return "other";
  }
}
function aggregateResourceTiming() {
  const resources = performance.getEntriesByType("resource") || [];
  const byType = {};
  let transferSize = 0;
  let decodedSize = 0;
  for (const entry of resources) {
    const category = mapInitiatorType(entry.initiatorType);
    if (!byType[category]) {
      byType[category] = { count: 0, size: 0 };
    }
    byType[category].count++;
    byType[category].size += entry.transferSize || 0;
    transferSize += entry.transferSize || 0;
    decodedSize += entry.decodedBodySize || 0;
  }
  const sorted = [...resources].sort((a, b) => b.duration - a.duration);
  const slowestRequests = sorted.slice(0, MAX_SLOWEST_REQUESTS).map((r) => ({
    url: r.name.length > MAX_URL_LENGTH ? r.name.slice(0, MAX_URL_LENGTH) : r.name,
    duration: r.duration,
    size: r.transferSize || 0
  }));
  return {
    request_count: resources.length,
    transfer_size: transferSize,
    decoded_size: decodedSize,
    by_type: byType,
    slowest_requests: slowestRequests
  };
}
function capturePerformanceSnapshot() {
  const navEntries = performance.getEntriesByType("navigation") || [];
  if (!navEntries || navEntries.length === 0)
    return null;
  const nav = navEntries[0];
  if (!nav)
    return null;
  const timing = {
    dom_content_loaded: nav.domContentLoadedEventEnd,
    load: nav.loadEventEnd,
    first_contentful_paint: getFCP(),
    largest_contentful_paint: getLCP(),
    interaction_to_next_paint: getINP(),
    time_to_first_byte: nav.responseStart - nav.requestStart,
    dom_interactive: nav.domInteractive
  };
  const network = aggregateResourceTiming();
  const longTasks = getLongTaskMetrics();
  const marks = performance.getEntriesByType("mark") || [];
  const measures = performance.getEntriesByType("measure") || [];
  const userTiming = marks.length > 0 || measures.length > 0 ? {
    marks: marks.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime })),
    measures: measures.slice(-50).map((m) => ({ name: m.name, start_time: m.startTime, duration: m.duration }))
  } : void 0;
  return {
    url: window.location.pathname,
    timestamp: (/* @__PURE__ */ new Date()).toISOString(),
    timing,
    network,
    long_tasks: longTasks,
    cumulative_layout_shift: getCLS(),
    user_timing: userTiming
  };
}
function installPerfObservers() {
  longTaskEntries = [];
  fcpValue = null;
  lcpValue = null;
  clsValue = 0;
  inpValue = null;
  longTaskObserver = new PerformanceObserver((list) => {
    const entries = list.getEntries();
    for (const entry of entries) {
      if (longTaskEntries.length < MAX_LONG_TASKS) {
        longTaskEntries.push(entry);
      }
    }
  });
  longTaskObserver.observe({ type: "longtask" });
  paintObserver = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      if (entry.name === "first-contentful-paint") {
        fcpValue = entry.startTime;
      }
    }
  });
  paintObserver.observe({ type: "paint", buffered: true });
  lcpObserver = new PerformanceObserver((list) => {
    const entries = list.getEntries();
    if (entries.length > 0) {
      const lastEntry = entries[entries.length - 1];
      if (lastEntry) {
        lcpValue = lastEntry.startTime;
      }
    }
  });
  lcpObserver.observe({ type: "largest-contentful-paint", buffered: true });
  clsObserver = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const clsEntry = entry;
      if (!clsEntry.hadRecentInput) {
        clsValue += clsEntry.value || 0;
      }
    }
  });
  clsObserver.observe({ type: "layout-shift", buffered: true });
  inpObserver = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
      const inpEntry = entry;
      if (inpEntry.interactionId) {
        if (inpValue === null || inpEntry.duration > inpValue) {
          inpValue = inpEntry.duration;
        }
      }
    }
  });
  inpObserver.observe({ type: "event", durationThreshold: 40, buffered: true });
}
function uninstallPerfObservers() {
  if (longTaskObserver) {
    longTaskObserver.disconnect();
    longTaskObserver = null;
  }
  if (paintObserver) {
    paintObserver.disconnect();
    paintObserver = null;
  }
  if (lcpObserver) {
    lcpObserver.disconnect();
    lcpObserver = null;
  }
  if (clsObserver) {
    clsObserver.disconnect();
    clsObserver = null;
  }
  if (inpObserver) {
    inpObserver.disconnect();
    inpObserver = null;
  }
  longTaskEntries = [];
}
function getLongTaskMetrics() {
  let totalBlockingTime = 0;
  let longest = 0;
  for (const entry of longTaskEntries) {
    const blocking = entry.duration - 50;
    if (blocking > 0)
      totalBlockingTime += blocking;
    if (entry.duration > longest)
      longest = entry.duration;
  }
  return {
    count: longTaskEntries.length,
    total_blocking_time: totalBlockingTime,
    longest
  };
}
function getFCP() {
  return fcpValue;
}
function getLCP() {
  return lcpValue;
}
function getCLS() {
  return clsValue;
}
function getINP() {
  return inpValue;
}
function sendPerformanceSnapshot() {
  if (!perfSnapshotEnabled)
    return;
  const snapshot = capturePerformanceSnapshot();
  if (!snapshot)
    return;
  window.postMessage({ type: "GASOLINE_PERFORMANCE_SNAPSHOT", payload: snapshot }, window.location.origin);
}
var snapshotResendTimer = null;
function scheduleSnapshotResend() {
  if (!perfSnapshotEnabled)
    return;
  if (snapshotResendTimer)
    clearTimeout(snapshotResendTimer);
  snapshotResendTimer = setTimeout(() => {
    snapshotResendTimer = null;
    sendPerformanceSnapshot();
  }, 500);
}
function isPerformanceSnapshotEnabled() {
  return perfSnapshotEnabled;
}
function setPerformanceSnapshotEnabled(enabled) {
  perfSnapshotEnabled = enabled;
}

// extension/lib/performance.js
var performanceMarksEnabled = false;
var capturedMarks = [];
var capturedMeasures = [];
var originalPerformanceMark = null;
var originalPerformanceMeasure = null;
var performanceObserver = null;
var performanceCaptureActive = false;
function getPerformanceMarks(options = {}) {
  if (typeof performance === "undefined" || !performance)
    return [];
  try {
    let marks = performance.getEntriesByType("mark") || [];
    if (options.since) {
      marks = marks.filter((m) => m.startTime >= options.since);
    }
    marks.sort((a, b) => a.startTime - b.startTime);
    if (marks.length > MAX_PERFORMANCE_ENTRIES) {
      marks = marks.slice(-MAX_PERFORMANCE_ENTRIES);
    }
    return marks.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      detail: m.detail || null
    }));
  } catch {
    return [];
  }
}
function getPerformanceMeasures(options = {}) {
  if (typeof performance === "undefined" || !performance)
    return [];
  try {
    let measures = performance.getEntriesByType("measure") || [];
    if (options.since) {
      measures = measures.filter((m) => m.startTime >= options.since);
    }
    measures.sort((a, b) => a.startTime - b.startTime);
    if (measures.length > MAX_PERFORMANCE_ENTRIES) {
      measures = measures.slice(-MAX_PERFORMANCE_ENTRIES);
    }
    return measures.map((m) => ({
      name: m.name,
      startTime: m.startTime,
      duration: m.duration,
      ...m.detail !== void 0 ? { detail: m.detail } : {}
    }));
  } catch {
    return [];
  }
}
function getCapturedMarks() {
  return [...capturedMarks];
}
function getCapturedMeasures() {
  return [...capturedMeasures];
}
function installPerformanceCapture() {
  if (typeof performance === "undefined" || !performance)
    return;
  if (performanceCaptureActive) {
    console.warn("[Gasoline] Performance capture already installed, skipping");
    return;
  }
  capturedMarks = [];
  capturedMeasures = [];
  originalPerformanceMark = performance.mark.bind(performance);
  originalPerformanceMeasure = performance.measure.bind(performance);
  const wrappedMark = function(name, options) {
    const result = originalPerformanceMark.call(performance, name, options);
    capturedMarks.push({
      name,
      startTime: result.startTime || performance.now(),
      entryType: "mark",
      detail: options?.detail || void 0,
      capturedAt: (/* @__PURE__ */ new Date()).toISOString()
    });
    if (capturedMarks.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMarks.shift();
    }
    scheduleSnapshotResend();
    return result;
  };
  Object.defineProperty(performance, "mark", { value: wrappedMark, writable: true, configurable: true });
  const wrappedMeasure = function(name, startMark, endMark) {
    const result = originalPerformanceMeasure.call(performance, name, startMark, endMark);
    capturedMeasures.push({
      name,
      startTime: result.startTime || 0,
      duration: result.duration || 0,
      entryType: "measure",
      capturedAt: (/* @__PURE__ */ new Date()).toISOString()
    });
    if (capturedMeasures.length > MAX_PERFORMANCE_ENTRIES) {
      capturedMeasures.shift();
    }
    scheduleSnapshotResend();
    return result;
  };
  Object.defineProperty(performance, "measure", { value: wrappedMeasure, writable: true, configurable: true });
  performanceCaptureActive = true;
  if (typeof window !== "undefined" && typeof PerformanceObserver !== "undefined") {
    try {
      performanceObserver = new PerformanceObserver((list) => {
        for (const entry of list.getEntries()) {
          if (entry.entryType === "mark") {
            if (!capturedMarks.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMarks.push({
                name: entry.name,
                startTime: entry.startTime,
                entryType: "mark",
                detail: entry.detail || void 0,
                capturedAt: (/* @__PURE__ */ new Date()).toISOString()
              });
            }
          } else if (entry.entryType === "measure") {
            if (!capturedMeasures.some((m) => m.name === entry.name && m.startTime === entry.startTime)) {
              capturedMeasures.push({
                name: entry.name,
                startTime: entry.startTime,
                duration: entry.duration,
                entryType: "measure",
                capturedAt: (/* @__PURE__ */ new Date()).toISOString()
              });
            }
          }
        }
      });
      if (performanceObserver) {
        performanceObserver.observe({ entryTypes: ["mark", "measure"] });
      }
    } catch {
    }
  }
}
function uninstallPerformanceCapture() {
  if (typeof performance === "undefined" || !performance)
    return;
  if (originalPerformanceMark) {
    Object.defineProperty(performance, "mark", { value: originalPerformanceMark, writable: true, configurable: true });
    originalPerformanceMark = null;
  }
  if (originalPerformanceMeasure) {
    Object.defineProperty(performance, "measure", {
      value: originalPerformanceMeasure,
      writable: true,
      configurable: true
    });
    originalPerformanceMeasure = null;
  }
  if (performanceObserver) {
    performanceObserver.disconnect();
    performanceObserver = null;
  }
  capturedMarks = [];
  capturedMeasures = [];
  performanceCaptureActive = false;
}
function isPerformanceCaptureActive() {
  return performanceCaptureActive;
}
async function getPerformanceSnapshotForError(errorEntry) {
  if (!performanceMarksEnabled)
    return null;
  const now = typeof performance !== "undefined" && performance?.now ? performance.now() : 0;
  const since = Math.max(0, now - PERFORMANCE_TIME_WINDOW_MS);
  const marks = getPerformanceMarks({ since });
  const measures = getPerformanceMeasures({ since });
  let navigation = null;
  if (typeof performance !== "undefined" && performance) {
    try {
      const navEntries = performance.getEntriesByType("navigation") || [];
      if (navEntries && navEntries.length > 0) {
        const nav = navEntries[0];
        if (nav) {
          navigation = {
            type: nav.type,
            startTime: nav.startTime,
            domContentLoadedEventEnd: nav.domContentLoadedEventEnd,
            loadEventEnd: nav.loadEventEnd
          };
        }
      }
    } catch {
    }
  }
  return {
    type: "performance",
    ts: (/* @__PURE__ */ new Date()).toISOString(),
    _enrichments: ["performanceMarks"],
    _errorTs: errorEntry.ts,
    marks,
    measures,
    navigation
  };
}
function setPerformanceMarksEnabled(enabled) {
  performanceMarksEnabled = enabled;
}
function isPerformanceMarksEnabled() {
  return performanceMarksEnabled;
}

// extension/lib/bridge.js
function postLog(payload) {
  const context = getContextAnnotations();
  const actions = payload.level === "error" ? getActionBuffer() : null;
  const enrichments = [];
  if (context && payload.level === "error")
    enrichments.push("context");
  if (actions && actions.length > 0)
    enrichments.push("userActions");
  const { level, type, args, error, stack, ...otherFields } = payload;
  window.postMessage({
    type: "GASOLINE_LOG",
    payload: {
      // Enriched fields (these are the source of truth)
      ts: (/* @__PURE__ */ new Date()).toISOString(),
      url: window.location.href,
      message: payload.message || payload.error || (payload.args?.[0] !== null && payload.args?.[0] !== void 0 ? String(payload.args[0]) : ""),
      source: payload.filename ? `${payload.filename}:${payload.lineno || 0}` : "",
      // Core fields from payload
      level,
      ...type ? { type } : {},
      ...args ? { args } : {},
      ...error ? { error } : {},
      ...stack ? { stack } : {},
      // Optional enrichments
      ...enrichments.length > 0 ? { _enrichments: enrichments } : {},
      ...context && payload.level === "error" ? { _context: context } : {},
      ...actions && actions.length > 0 ? { _actions: actions } : {},
      // Any other fields from payload (excluding the ones we destructured)
      ...otherFields
    }
  }, window.location.origin);
}

// extension/lib/console.js
var originalConsole = {};
function installConsoleCapture() {
  const methods = ["log", "warn", "error", "info", "debug"];
  methods.forEach((method) => {
    originalConsole[method] = console[method];
    console[method] = function(...args) {
      postLog({
        level: method,
        type: "console",
        args: args.map((arg) => safeSerialize(arg))
      });
      originalConsole[method].apply(console, args);
    };
  });
}
function uninstallConsoleCapture() {
  Object.keys(originalConsole).forEach((method) => {
    console[method] = originalConsole[method];
  });
  originalConsole = {};
}

// extension/lib/ai-context.js
var aiContextEnabled = true;
var aiContextStateSnapshotEnabled = false;
var aiSourceMapCache = /* @__PURE__ */ new Map();
var CHROME_FRAME_RE = /^at\s+(?:(.+?)\s+\()?(.+?):(\d+):(\d+)\)?$/;
var FIREFOX_FRAME_RE = /^(.+?)@(.+?):(\d+):(\d+)$/;
function parseChromeFrame(line) {
  const m = line.match(CHROME_FRAME_RE);
  if (!m)
    return null;
  const filename = m[2];
  if (!filename || filename.includes("<anonymous>"))
    return null;
  if (!m[3] || !m[4])
    return null;
  return { functionName: m[1] || null, filename, lineno: parseInt(m[3], 10), colno: parseInt(m[4], 10) };
}
function parseFirefoxFrame(line) {
  const m = line.match(FIREFOX_FRAME_RE);
  if (!m)
    return null;
  const filename = m[2];
  if (!filename || filename.includes("<anonymous>"))
    return null;
  if (!m[3] || !m[4])
    return null;
  return { functionName: m[1] || null, filename, lineno: parseInt(m[3], 10), colno: parseInt(m[4], 10) };
}
var FRAME_PARSERS = [parseChromeFrame, parseFirefoxFrame];
function parseStackFrames(stack) {
  if (!stack)
    return [];
  const frames = [];
  for (const line of stack.split("\n")) {
    const trimmed = line.trim();
    for (const parser of FRAME_PARSERS) {
      const frame = parser(trimmed);
      if (frame) {
        frames.push(frame);
        break;
      }
    }
  }
  return frames;
}
function parseSourceMap(dataUrl) {
  if (!dataUrl || typeof dataUrl !== "string")
    return null;
  if (!dataUrl.startsWith("data:"))
    return null;
  try {
    const base64Match = dataUrl.match(/;base64,(.+)$/);
    if (!base64Match || !base64Match[1])
      return null;
    const decoded = atob(base64Match[1]);
    const parsed = JSON.parse(decoded);
    if (!parsed.sourcesContent || parsed.sourcesContent.length === 0)
      return null;
    return parsed;
  } catch {
    return null;
  }
}
function extractSnippet(sourceContent, line) {
  if (!sourceContent || typeof sourceContent !== "string")
    return null;
  if (!line || line < 1)
    return null;
  const lines = sourceContent.split("\n");
  if (line > lines.length)
    return null;
  const start = Math.max(0, line - 1 - AI_CONTEXT_SNIPPET_LINES);
  const end = Math.min(lines.length, line + AI_CONTEXT_SNIPPET_LINES);
  const snippet = [];
  for (let i = start; i < end; i++) {
    let text = lines[i];
    if (!text)
      continue;
    if (text.length > AI_CONTEXT_MAX_LINE_LENGTH) {
      text = text.slice(0, AI_CONTEXT_MAX_LINE_LENGTH);
    }
    const entry = { line: i + 1, text };
    if (i + 1 === line)
      entry.isError = true;
    snippet.push(entry);
  }
  return snippet;
}
async function extractSourceSnippets(frames, mockSourceMaps) {
  const snippets = [];
  let totalSize = 0;
  for (const frame of frames.slice(0, 3)) {
    if (totalSize >= AI_CONTEXT_MAX_SNIPPETS_SIZE)
      break;
    const sourceMap = mockSourceMaps[frame.filename];
    if (!sourceMap || !sourceMap.sourcesContent || !sourceMap.sourcesContent[0])
      continue;
    const snippet = extractSnippet(sourceMap.sourcesContent[0], frame.lineno);
    if (!snippet)
      continue;
    const snippetObj = { file: frame.filename, line: frame.lineno, snippet };
    const snippetSize = JSON.stringify(snippetObj).length;
    if (totalSize + snippetSize > AI_CONTEXT_MAX_SNIPPETS_SIZE)
      break;
    totalSize += snippetSize;
    snippets.push(snippetObj);
  }
  return snippets;
}
function detectFramework(element) {
  if (!element || typeof element !== "object")
    return null;
  const keys = Object.keys(element);
  const reactKey = keys.find((k) => k.startsWith("__reactFiber$") || k.startsWith("__reactInternalInstance$"));
  if (reactKey)
    return { framework: "react", key: reactKey };
  if (element.__vueParentComponent || element.__vue_app__) {
    return { framework: "vue" };
  }
  if (element.__svelte_meta) {
    return { framework: "svelte" };
  }
  return null;
}
function getReactComponentAncestry(fiber) {
  if (!fiber)
    return null;
  const ancestry = [];
  let current = fiber;
  let depth = 0;
  while (current && depth < AI_CONTEXT_MAX_ANCESTRY_DEPTH) {
    depth++;
    if (current.type && typeof current.type !== "string") {
      const typeObj = current.type;
      const name = typeObj.displayName || typeObj.name || "Anonymous";
      const entry = { name };
      if (current.memoizedProps && typeof current.memoizedProps === "object") {
        entry.propKeys = Object.keys(current.memoizedProps).filter((k) => k !== "children").slice(0, AI_CONTEXT_MAX_PROP_KEYS);
      }
      if (current.memoizedState && typeof current.memoizedState === "object" && !Array.isArray(current.memoizedState)) {
        entry.hasState = true;
        entry.stateKeys = Object.keys(current.memoizedState).slice(0, AI_CONTEXT_MAX_STATE_KEYS);
      }
      ancestry.push(entry);
    }
    current = current.return;
  }
  return ancestry.reverse();
}
function classifyValueType(value) {
  if (Array.isArray(value))
    return "array";
  if (value === null)
    return "null";
  return typeof value;
}
var RELEVANT_STATE_KEYS = ["error", "loading", "status", "failed"];
function buildRelevantSlice(state, errorWords) {
  const relevantSlice = {};
  let sliceCount = 0;
  for (const [key, value] of Object.entries(state)) {
    if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE)
      break;
    if (typeof value !== "object" || value === null || Array.isArray(value))
      continue;
    for (const [subKey, subValue] of Object.entries(value)) {
      if (sliceCount >= AI_CONTEXT_MAX_RELEVANT_SLICE)
        break;
      const isRelevantKey = RELEVANT_STATE_KEYS.some((k) => subKey.toLowerCase().includes(k));
      const isKeywordMatch = errorWords.some((w) => key.toLowerCase().includes(w));
      if (!isRelevantKey && !isKeywordMatch)
        continue;
      let val = subValue;
      if (typeof val === "string" && val.length > AI_CONTEXT_MAX_VALUE_LENGTH) {
        val = val.slice(0, AI_CONTEXT_MAX_VALUE_LENGTH);
      }
      relevantSlice[`${key}.${subKey}`] = val;
      sliceCount++;
    }
  }
  return relevantSlice;
}
function captureStateSnapshot(errorMessage) {
  if (typeof window === "undefined")
    return null;
  try {
    const store = window.__REDUX_STORE__;
    if (!store || typeof store.getState !== "function")
      return null;
    const state = store.getState();
    if (!state || typeof state !== "object")
      return null;
    const keys = {};
    for (const [key, value] of Object.entries(state)) {
      keys[key] = { type: classifyValueType(value) };
    }
    const errorWords = (errorMessage || "").toLowerCase().split(/\W+/).filter((w) => w.length > 2);
    const relevantSlice = buildRelevantSlice(state, errorWords);
    return { source: "redux", keys, relevantSlice };
  } catch {
    return null;
  }
}
function generateAiSummary(data) {
  const parts = [];
  if (data.file && data.line) {
    parts.push(`${data.errorType} in ${data.file}:${data.line} \u2014 ${data.message}`);
  } else {
    parts.push(`${data.errorType}: ${data.message}`);
  }
  if (data.componentAncestry && data.componentAncestry.components) {
    const path = data.componentAncestry.components.map((c) => c.name).join(" > ");
    parts.push(`Component tree: ${path}.`);
  }
  if (data.stateSnapshot && data.stateSnapshot.relevantSlice) {
    const sliceKeys = Object.keys(data.stateSnapshot.relevantSlice);
    if (sliceKeys.length > 0) {
      const stateInfo = sliceKeys.map((k) => `${k}=${JSON.stringify(data.stateSnapshot.relevantSlice[k])}`).join(", ");
      parts.push(`State: ${stateInfo}.`);
    }
  }
  return parts.join(" ");
}
async function buildAiContext(error) {
  const result = {};
  const frames = parseStackFrames(error.stack);
  if (frames.length === 0)
    return { summary: error.message || "Unknown error" };
  const topFrame = frames[0];
  if (topFrame) {
    const cached = getSourceMapCache(topFrame.filename);
    if (cached) {
      const snippets = await extractSourceSnippets(frames, { [topFrame.filename]: cached });
      if (snippets.length > 0)
        result.sourceSnippets = snippets;
    }
  }
  result.componentAncestry = extractComponentAncestry() || void 0;
  if (aiContextStateSnapshotEnabled) {
    const snapshot = captureStateSnapshot(error.message || "");
    if (snapshot)
      result.stateSnapshot = snapshot;
  }
  result.summary = generateAiSummary({
    errorType: error.message?.split(":")[0] || "Error",
    message: error.message || "",
    file: topFrame?.filename || null,
    line: topFrame?.lineno || null,
    componentAncestry: result.componentAncestry || null,
    stateSnapshot: result.stateSnapshot || null
  });
  return result;
}
function extractComponentAncestry() {
  if (typeof document === "undefined" || !document.activeElement)
    return null;
  const framework = detectFramework(document.activeElement);
  if (!framework || framework.framework !== "react" || !framework.key)
    return null;
  const fiber = document.activeElement[framework.key];
  const components = getReactComponentAncestry(fiber);
  if (!components || components.length === 0)
    return null;
  return { framework: "react", components };
}
function applyAiContext(enriched, context) {
  enriched._aiContext = context;
  if (!enriched._enrichments)
    enriched._enrichments = [];
  enriched._enrichments.push("aiContext");
}
async function enrichErrorWithAiContext(error) {
  if (!aiContextEnabled)
    return error;
  const enriched = { ...error };
  try {
    const context = await Promise.race([
      buildAiContext(error),
      new Promise((resolve) => {
        setTimeout(() => resolve({ summary: `${error.message || "Error"}` }), AI_CONTEXT_PIPELINE_TIMEOUT_MS);
      })
    ]);
    applyAiContext(enriched, context);
  } catch {
    applyAiContext(enriched, { summary: error.message || "Unknown error" });
  }
  return enriched;
}
function setAiContextEnabled(enabled) {
  aiContextEnabled = enabled;
}
function setAiContextStateSnapshot(enabled) {
  aiContextStateSnapshotEnabled = enabled;
}
function setSourceMapCache(url, map) {
  if (!aiSourceMapCache.has(url) && aiSourceMapCache.size >= AI_CONTEXT_SOURCE_MAP_CACHE_SIZE) {
    const firstKey = aiSourceMapCache.keys().next().value;
    if (firstKey) {
      aiSourceMapCache.delete(firstKey);
    }
  }
  aiSourceMapCache.delete(url);
  aiSourceMapCache.set(url, map);
}
function getSourceMapCache(url) {
  return aiSourceMapCache.get(url) || null;
}
function getSourceMapCacheSize() {
  return aiSourceMapCache.size;
}

// extension/lib/exceptions.js
var originalOnerror = null;
var unhandledrejectionHandler = null;
function enrichAndPost(entry) {
  void (async () => {
    try {
      const enriched = await enrichErrorWithAiContext(entry);
      postLog(enriched);
    } catch {
      postLog(entry);
    }
  })().catch((err) => {
    console.error("[Gasoline] Exception enrichment error:", err);
    try {
      postLog(entry);
    } catch (postErr) {
      console.error("[Gasoline] Failed to log entry:", postErr);
    }
  });
}
function extractRejectionInfo(reason) {
  if (reason instanceof Error)
    return { message: reason.message, stack: reason.stack || "" };
  if (typeof reason === "string")
    return { message: reason, stack: "" };
  return { message: String(reason), stack: "" };
}
function installExceptionCapture() {
  originalOnerror = window.onerror;
  window.onerror = function(message, filename, lineno, colno, error) {
    const messageStr = typeof message === "string" ? message : message.type || "Error";
    const entry = {
      level: "error",
      type: "exception",
      message: messageStr,
      source: filename ? `${filename}:${lineno || 0}` : "",
      filename: filename || "",
      lineno: lineno || 0,
      colno: colno || 0,
      stack: error?.stack || ""
    };
    enrichAndPost(entry);
    if (originalOnerror)
      return originalOnerror(message, filename, lineno, colno, error);
    return false;
  };
  unhandledrejectionHandler = function(event) {
    const { message, stack } = extractRejectionInfo(event.reason);
    enrichAndPost({
      level: "error",
      type: "exception",
      message: `Unhandled Promise Rejection: ${message}`,
      stack
    });
  };
  window.addEventListener("unhandledrejection", unhandledrejectionHandler);
}
function uninstallExceptionCapture() {
  if (originalOnerror !== null) {
    window.onerror = originalOnerror;
    originalOnerror = null;
  }
  if (unhandledrejectionHandler) {
    window.removeEventListener("unhandledrejection", unhandledrejectionHandler);
    unhandledrejectionHandler = null;
  }
}

// extension/lib/websocket.js
var _textEncoder = typeof TextEncoder !== "undefined" ? new TextEncoder() : null;
var originalWebSocket = null;
var webSocketCaptureEnabled = true;
var webSocketCaptureMode = "medium";
function getSize(data) {
  if (typeof data === "string") {
    return _textEncoder ? _textEncoder.encode(data).length : data.length;
  }
  if (data instanceof ArrayBuffer)
    return data.byteLength;
  if (data && typeof data === "object" && "size" in data)
    return data.size;
  return 0;
}
function formatPayload(data) {
  if (typeof data === "string")
    return data;
  if (data instanceof ArrayBuffer) {
    const bytes = new Uint8Array(data);
    if (data.byteLength < 256) {
      let hex = "";
      for (let i = 0; i < bytes.length; i++) {
        const byte = bytes[i];
        if (byte !== void 0) {
          hex += byte.toString(16).padStart(2, "0");
        }
      }
      return `[Binary: ${data.byteLength}B] ${hex}`;
    } else {
      let magic = "";
      for (let i = 0; i < Math.min(4, bytes.length); i++) {
        const byte = bytes[i];
        if (byte !== void 0) {
          magic += byte.toString(16).padStart(2, "0");
        }
      }
      return `[Binary: ${data.byteLength}B, magic:${magic}]`;
    }
  }
  if (data && typeof data === "object" && "size" in data) {
    return `[Binary: ${data.size}B]`;
  }
  return String(data);
}
function truncateWsMessage(message) {
  if (typeof message === "string" && message.length > WS_MAX_BODY_SIZE) {
    return { data: message.slice(0, WS_MAX_BODY_SIZE), truncated: true };
  }
  return { data: message, truncated: false };
}
function createConnectionTracker(id, url) {
  const tracker = {
    id,
    url,
    messageCount: 0,
    _sampleCounter: 0,
    _messageRate: 0,
    _messageTimestamps: [],
    _schemaKeys: [],
    _schemaVariants: /* @__PURE__ */ new Map(),
    _schemaConsistent: true,
    _schemaDetected: false,
    stats: {
      incoming: { count: 0, bytes: 0, lastPreview: null, lastAt: null },
      outgoing: { count: 0, bytes: 0, lastPreview: null, lastAt: null }
    },
    /**
     * Record a message for stats and schema detection
     *
     * WEBSOCKET PAYLOAD SCHEMA INFERENCE LOGIC:
     *
     * This method implements a three-phase schema detection strategy to identify the
     * shape of JSON messages flowing over a WebSocket connection. Understanding the
     * schema is crucial for debugging: it reveals whether messages are uniform (good
     * for testing) or polymorphic (suggests different message types or errors).
     *
     * PHASE 1: BOOTSTRAP DETECTION (messages 1-5)
     *   Purpose: Quickly infer the "canonical" schema from the first JSON messages.
     *   Strategy:
     *     - Extract sorted object keys from each incoming JSON message
     *     - Stop after 5 messages (samples are enough to detect schema; balance between
     *       coverage and memory/CPU cost)
     *     - Compute consistency: if all 5 messages have identical key sets, mark as
     *       consistent=true
     *     - Store key strings as comma-separated sorted lists (e.g., "id,status,timestamp")
     *   Why 5: Statistically sufficient for most API patterns. First message might be
     *     special (connection ACK). By message 5, the pattern is clear.
     *   Early exit: If not JSON or message is array, skip (only track object schemas).
     *
     * PHASE 2: CONSISTENCY CHECKING (after first 2 messages)
     *   Trigger: Once _schemaKeys.length >= 2, begin checking if all keys match the first.
     *   Result: Sets _schemaConsistent = boolean indicating if messages have uniform schema.
     *   Why check early: Detect schema changes immediately without waiting for all 5 messages.
     *   Performance: O(n) single pass over _schemaKeys array; no redundant comparisons.
     *
     * PHASE 3: VARIANT TRACKING (messages 6+)
     *   Purpose: After bootstrap, track schema variants without resetting detection.
     *   Strategy:
     *     - Continue parsing incoming JSON messages after _schemaDetected = true
     *     - Build variants Map: key -> count (e.g., "id,status" -> 5 occurrences)
     *     - Memory bound: Cap Map at 50 entries. Only add new variants if under cap;
     *       always increment existing keys (ensures frequent patterns stay tracked).
     *     - This bounds memory to ~50KB even on long-lived connections.
     *   Why variants matter: Detects polymorphic message types (e.g., "id,status,data"
     *     vs "id,error,code"). Useful for debugging API versioning issues.
     *   Why cap variants: Long-running connections might emit hundreds of unique schemas.
     *     Capping prevents unbounded growth while keeping the 50 most frequent variants.
     *
     * SAMPLING RATE DECISION:
     *   The schema info (keys, consistency, variants) flows to getSchema() which returns:
     *     - detectedKeys: union of all seen keys (for understanding message structure)
     *     - consistent: boolean (true if all bootstrap messages matched)
     *     - variants: array of key strings (top variants seen after bootstrap)
     *   MCP observe handler uses this to emit SchemaInfo in WebSocket capture events,
     *   helping users understand payload patterns without logging every message.
     *
     * MESSAGE RATE TRACKING:
     *   Maintains _messageTimestamps for the last 5 seconds (sliding window). This powers
     *   shouldSample() which implements adaptive sampling: high-frequency connections
     *   (>200 msg/s) sample at 1-in-100; low-frequency (<2 msg/s) capture all messages.
     *   This ensures detailed visibility on slow links without bloating on high-volume.
     */
    recordMessage(direction, data) {
      this.messageCount++;
      const size = data ? typeof data === "string" ? data.length : getSize(data) : 0;
      const now = Date.now();
      this.stats[direction].count++;
      this.stats[direction].bytes += size;
      this.stats[direction].lastAt = now;
      if (data && typeof data === "string") {
        this.stats[direction].lastPreview = data.length > WS_PREVIEW_LIMIT ? data.slice(0, WS_PREVIEW_LIMIT) : data;
      }
      this._messageTimestamps.push(now);
      const cutoff = now - 5e3;
      this._messageTimestamps = this._messageTimestamps.filter((t) => t >= cutoff);
      if (direction === "incoming" && data && typeof data === "string" && this._schemaKeys.length < 5) {
        try {
          const parsed = JSON.parse(data);
          if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed).sort();
            const keyStr = keys.join(",");
            this._schemaKeys.push(keyStr);
            this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1);
            if (this._schemaKeys.length >= 2) {
              const first = this._schemaKeys[0];
              this._schemaConsistent = this._schemaKeys.every((k) => k === first);
            }
            if (this._schemaKeys.length >= 5) {
              this._schemaDetected = true;
            }
          }
        } catch {
        }
      }
      if (direction === "incoming" && data && typeof data === "string" && this._schemaDetected) {
        try {
          const parsed = JSON.parse(data);
          if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
            const keys = Object.keys(parsed).sort();
            const keyStr = keys.join(",");
            if (this._schemaVariants.has(keyStr) || this._schemaVariants.size < 50) {
              this._schemaVariants.set(keyStr, (this._schemaVariants.get(keyStr) || 0) + 1);
            }
          }
        } catch {
        }
      }
    },
    /**
     * Determine if a message should be sampled (logged)
     */
    shouldSample(_direction) {
      this._sampleCounter++;
      if (webSocketCaptureMode === "all")
        return true;
      if (this.messageCount > 0 && this.messageCount <= 5)
        return true;
      const rate = this._messageRate || this.getMessageRate();
      const targetRate = webSocketCaptureMode === "high" ? 10 : webSocketCaptureMode === "medium" ? 5 : 2;
      if (rate <= targetRate)
        return true;
      const n = Math.max(1, Math.round(rate / targetRate));
      return this._sampleCounter % n === 0;
    },
    /**
     * Lifecycle events should always be logged
     */
    shouldLogLifecycle() {
      return true;
    },
    /**
     * Get sampling info
     */
    getSamplingInfo() {
      const rate = this._messageRate || this.getMessageRate();
      let targetRate = rate;
      if (rate >= 10 && rate < 50)
        targetRate = 10;
      else if (rate >= 50 && rate < 200)
        targetRate = 5;
      else if (rate >= 200)
        targetRate = 2;
      return {
        rate: `${rate}/s`,
        logged: `${targetRate}/${Math.round(rate)}`,
        window: "5s"
      };
    },
    /**
     * Get the current message rate (messages per second)
     */
    getMessageRate() {
      if (this._messageTimestamps.length < 2)
        return this._messageTimestamps.length;
      const lastTime = this._messageTimestamps[this._messageTimestamps.length - 1];
      const firstTime = this._messageTimestamps[0];
      if (lastTime === void 0 || firstTime === void 0)
        return this._messageTimestamps.length;
      const window2 = (lastTime - firstTime) / 1e3;
      return window2 > 0 ? this._messageTimestamps.length / window2 : this._messageTimestamps.length;
    },
    /**
     * Set the message rate manually (for testing)
     */
    setMessageRate(rate) {
      this._messageRate = rate;
    },
    /**
     * Get the detected schema info
     */
    getSchema() {
      if (this._schemaKeys.length === 0) {
        return { detectedKeys: null, consistent: true };
      }
      const allKeys = /* @__PURE__ */ new Set();
      for (const keyStr of this._schemaKeys) {
        for (const k of keyStr.split(",")) {
          if (k)
            allKeys.add(k);
        }
      }
      const variants = [];
      for (const [keyStr, count] of this._schemaVariants) {
        if (count > 0)
          variants.push(keyStr);
      }
      return {
        detectedKeys: allKeys.size > 0 ? Array.from(allKeys).sort() : null,
        consistent: this._schemaConsistent,
        variants: variants.length > 1 ? variants : void 0
      };
    },
    /**
     * Check if a message represents a schema change
     */
    isSchemaChange(data) {
      if (!this._schemaDetected || !data || typeof data !== "string")
        return false;
      try {
        const parsed = JSON.parse(data);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
          return false;
        const keys = Object.keys(parsed).sort().join(",");
        return !this._schemaKeys.includes(keys);
      } catch {
        return false;
      }
    }
  };
  return tracker;
}
function installWebSocketCapture() {
  if (typeof window === "undefined")
    return;
  if (!window.WebSocket)
    return;
  if (originalWebSocket)
    return;
  webSocketCaptureEnabled = true;
  const earlyOriginal = window.__GASOLINE_ORIGINAL_WS__;
  originalWebSocket = earlyOriginal || window.WebSocket;
  const OriginalWS = originalWebSocket;
  function GasolineWebSocket(url, protocols) {
    const ws = new OriginalWS(url, protocols);
    const connectionId = crypto.randomUUID();
    const urlString = url.toString();
    const tracker = createConnectionTracker(connectionId, urlString);
    ws.addEventListener("open", () => {
      if (!webSocketCaptureEnabled)
        return;
      window.postMessage({
        type: "GASOLINE_WS",
        payload: { type: "websocket", event: "open", id: connectionId, url: urlString, ts: (/* @__PURE__ */ new Date()).toISOString() }
      }, window.location.origin);
    });
    ws.addEventListener("close", (event) => {
      if (!webSocketCaptureEnabled)
        return;
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "close",
          id: connectionId,
          url: urlString,
          code: event.code,
          reason: event.reason,
          ts: (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    });
    ws.addEventListener("error", () => {
      if (!webSocketCaptureEnabled)
        return;
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "error",
          id: connectionId,
          url: urlString,
          ts: (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    });
    ws.addEventListener("message", (event) => {
      if (!webSocketCaptureEnabled)
        return;
      tracker.recordMessage("incoming", event.data);
      if (!tracker.shouldSample("incoming"))
        return;
      const data = event.data;
      const size = getSize(data);
      const formatted = formatPayload(data);
      const { data: truncatedData, truncated } = truncateWsMessage(formatted);
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "message",
          id: connectionId,
          url: urlString,
          direction: "incoming",
          data: truncatedData,
          size,
          truncated: truncated || void 0,
          ts: (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    });
    const originalSend = ws.send.bind(ws);
    ws.send = function(data) {
      if (webSocketCaptureEnabled) {
        tracker.recordMessage("outgoing", data);
      }
      if (webSocketCaptureEnabled && tracker.shouldSample("outgoing")) {
        const size = getSize(data);
        const formatted = formatPayload(data);
        const { data: truncatedData, truncated } = truncateWsMessage(formatted);
        window.postMessage({
          type: "GASOLINE_WS",
          payload: {
            type: "websocket",
            event: "message",
            id: connectionId,
            url: urlString,
            direction: "outgoing",
            data: truncatedData,
            size,
            truncated: truncated || void 0,
            ts: (/* @__PURE__ */ new Date()).toISOString()
          }
        }, window.location.origin);
      }
      return originalSend(data);
    };
    return ws;
  }
  GasolineWebSocket.prototype = OriginalWS.prototype;
  Object.defineProperty(GasolineWebSocket, "CONNECTING", { value: OriginalWS.CONNECTING, writable: false });
  Object.defineProperty(GasolineWebSocket, "OPEN", { value: OriginalWS.OPEN, writable: false });
  Object.defineProperty(GasolineWebSocket, "CLOSING", { value: OriginalWS.CLOSING, writable: false });
  Object.defineProperty(GasolineWebSocket, "CLOSED", { value: OriginalWS.CLOSED, writable: false });
  window.WebSocket = GasolineWebSocket;
  adoptEarlyConnections();
}
function adoptEarlyConnections() {
  const earlyConnections = window.__GASOLINE_EARLY_WS__;
  if (!earlyConnections || earlyConnections.length === 0) {
    delete window.__GASOLINE_ORIGINAL_WS__;
    delete window.__GASOLINE_EARLY_WS__;
    return;
  }
  let adopted = 0;
  for (const conn of earlyConnections) {
    const ws = conn.ws;
    if (ws.readyState === WebSocket.CLOSED)
      continue;
    adopted++;
    const connectionId = crypto.randomUUID();
    const urlString = conn.url;
    const tracker = createConnectionTracker(connectionId, urlString);
    const hasOpened = conn.events.some((e) => e.type === "open");
    if (hasOpened && webSocketCaptureEnabled) {
      const openEvent = conn.events.find((e) => e.type === "open");
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "open",
          id: connectionId,
          url: urlString,
          ts: openEvent ? new Date(openEvent.ts).toISOString() : (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    }
    ws.addEventListener("close", (event) => {
      if (!webSocketCaptureEnabled)
        return;
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "close",
          id: connectionId,
          url: urlString,
          code: event.code,
          reason: event.reason,
          ts: (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    });
    ws.addEventListener("error", () => {
      if (!webSocketCaptureEnabled)
        return;
      window.postMessage({
        type: "GASOLINE_WS",
        payload: { type: "websocket", event: "error", id: connectionId, url: urlString, ts: (/* @__PURE__ */ new Date()).toISOString() }
      }, window.location.origin);
    });
    ws.addEventListener("message", (event) => {
      if (!webSocketCaptureEnabled)
        return;
      tracker.recordMessage("incoming", event.data);
      if (!tracker.shouldSample("incoming"))
        return;
      const data = event.data;
      const size = getSize(data);
      const formatted = formatPayload(data);
      const { data: truncatedData, truncated } = truncateWsMessage(formatted);
      window.postMessage({
        type: "GASOLINE_WS",
        payload: {
          type: "websocket",
          event: "message",
          id: connectionId,
          url: urlString,
          direction: "incoming",
          data: truncatedData,
          size,
          truncated: truncated || void 0,
          ts: (/* @__PURE__ */ new Date()).toISOString()
        }
      }, window.location.origin);
    });
    const originalSend = ws.send.bind(ws);
    ws.send = function(data) {
      if (webSocketCaptureEnabled) {
        tracker.recordMessage("outgoing", data);
      }
      if (webSocketCaptureEnabled && tracker.shouldSample("outgoing")) {
        const size = getSize(data);
        const formatted = formatPayload(data);
        const { data: truncatedData, truncated } = truncateWsMessage(formatted);
        window.postMessage({
          type: "GASOLINE_WS",
          payload: {
            type: "websocket",
            event: "message",
            id: connectionId,
            url: urlString,
            direction: "outgoing",
            data: truncatedData,
            size,
            truncated: truncated || void 0,
            ts: (/* @__PURE__ */ new Date()).toISOString()
          }
        }, window.location.origin);
      }
      return originalSend(data);
    };
  }
  if (adopted > 0) {
    console.log(`[Gasoline] Adopted ${adopted} early WebSocket connection(s)`);
  }
  delete window.__GASOLINE_ORIGINAL_WS__;
  delete window.__GASOLINE_EARLY_WS__;
}
function setWebSocketCaptureMode(mode) {
  webSocketCaptureMode = mode;
}
function setWebSocketCaptureEnabled(enabled) {
  webSocketCaptureEnabled = enabled;
}
function getWebSocketCaptureMode() {
  return webSocketCaptureMode;
}
function uninstallWebSocketCapture() {
  if (typeof window === "undefined")
    return;
  if (originalWebSocket) {
    window.WebSocket = originalWebSocket;
    originalWebSocket = null;
  }
}
function resetForTesting() {
  uninstallWebSocketCapture();
  webSocketCaptureEnabled = false;
  webSocketCaptureMode = "medium";
  originalWebSocket = null;
  if (typeof window !== "undefined") {
    delete window.__GASOLINE_ORIGINAL_WS__;
    delete window.__GASOLINE_EARLY_WS__;
  }
}

// extension/lib/dom-queries.js
async function executeDOMQuery(params) {
  const { selector, include_styles, properties, include_children, max_depth } = params;
  const elements = document.querySelectorAll(selector);
  const matchCount = elements.length;
  const cappedDepth = Math.min(max_depth || 3, DOM_QUERY_MAX_DEPTH);
  const matches = [];
  for (let i = 0; i < Math.min(elements.length, DOM_QUERY_MAX_ELEMENTS); i++) {
    const el = elements[i];
    if (!el)
      continue;
    const entry = serializeDOMElement(el, include_styles, properties, include_children, cappedDepth, 0);
    matches.push(entry);
  }
  return {
    url: window.location.href,
    title: document.title,
    matchCount,
    returnedCount: matches.length,
    matches
  };
}
function collectAttributes(el) {
  if (!el.attributes || el.attributes.length === 0)
    return void 0;
  const attrs = {};
  for (const attr of el.attributes) {
    attrs[attr.name] = attr.value;
  }
  return attrs;
}
function collectBoundingBox(el) {
  if (!el.getBoundingClientRect)
    return void 0;
  const rect = el.getBoundingClientRect();
  return { x: rect.x, y: rect.y, width: rect.width, height: rect.height };
}
function collectStyles(el, includeStyles, styleProps) {
  if (!includeStyles || typeof window.getComputedStyle !== "function")
    return void 0;
  const computed = window.getComputedStyle(el);
  if (styleProps && styleProps.length > 0) {
    const styles = {};
    for (const prop of styleProps) {
      styles[prop] = computed.getPropertyValue(prop);
    }
    return styles;
  }
  return { display: computed.display, color: computed.color, position: computed.position };
}
function collectChildren(el, includeChildren, maxDepth, currentDepth) {
  if (!includeChildren || currentDepth >= maxDepth || !el.children || el.children.length === 0)
    return void 0;
  const children = [];
  const maxChildren = Math.min(el.children.length, DOM_QUERY_MAX_ELEMENTS);
  for (let i = 0; i < maxChildren; i++) {
    const child = el.children[i];
    if (child) {
      children.push(serializeDOMElement(child, false, void 0, true, maxDepth, currentDepth + 1));
    }
  }
  return children;
}
function serializeDOMElement(el, includeStyles, styleProps, includeChildren, maxDepth, currentDepth) {
  const entry = {
    tag: el.tagName ? el.tagName.toLowerCase() : "",
    text: (el.textContent || "").slice(0, DOM_QUERY_MAX_TEXT),
    visible: el.offsetParent !== null || el.getBoundingClientRect && el.getBoundingClientRect().width > 0
  };
  entry.attributes = collectAttributes(el);
  entry.boundingBox = collectBoundingBox(el);
  entry.styles = collectStyles(el, includeStyles, styleProps);
  entry.children = collectChildren(el, includeChildren, maxDepth, currentDepth);
  return entry;
}
async function getPageInfo() {
  const headings = [];
  const headingEls = document.querySelectorAll("h1,h2,h3,h4,h5,h6");
  for (const h of headingEls) {
    headings.push((h.textContent || "").slice(0, DOM_QUERY_MAX_TEXT));
  }
  const forms = [];
  const formEls = document.querySelectorAll("form");
  for (const form of formEls) {
    const fields = [];
    const inputs = form.querySelectorAll("input,select,textarea");
    for (const input of inputs) {
      const inputEl = input;
      if (inputEl.name)
        fields.push(inputEl.name);
    }
    forms.push({
      id: form.id || void 0,
      action: form.action || void 0,
      fields
    });
  }
  return {
    url: window.location.href,
    title: document.title,
    viewport: { width: window.innerWidth, height: window.innerHeight },
    scroll: { x: window.scrollX, y: window.scrollY },
    documentHeight: document.documentElement.scrollHeight,
    headings,
    links: document.querySelectorAll("a").length,
    images: document.querySelectorAll("img").length,
    interactiveElements: document.querySelectorAll("button,input,select,textarea,a[href]").length,
    forms
  };
}
function loadAxeCore() {
  return new Promise((resolve, reject) => {
    if (window.axe) {
      resolve();
      return;
    }
    const checkInterval = setInterval(() => {
      if (window.axe) {
        clearInterval(checkInterval);
        resolve();
      }
    }, 100);
    setTimeout(() => {
      clearInterval(checkInterval);
      reject(new Error("Accessibility audit failed: axe-core library not loaded (5s timeout). The extension content script may not have been injected on this page. Try reloading the tab and re-running the audit."));
    }, 5e3);
  });
}
async function runAxeAudit(params) {
  await loadAxeCore();
  const context = params.scope ? { include: [params.scope] } : document;
  const config = {};
  if (params.tags && params.tags.length > 0) {
    config.runOnly = params.tags;
  }
  if (params.include_passes) {
    config.resultTypes = ["violations", "passes", "incomplete", "inapplicable"];
  } else {
    config.resultTypes = ["violations", "incomplete"];
  }
  const results = await window.axe.run(context, config);
  return formatAxeResults(results);
}
async function runAxeAuditWithTimeout(params, timeoutMs = A11Y_AUDIT_TIMEOUT_MS) {
  return Promise.race([
    runAxeAudit(params),
    new Promise((resolve) => {
      setTimeout(() => resolve({
        violations: [],
        summary: { violations: 0, passes: 0, incomplete: 0, inapplicable: 0 },
        error: "Accessibility audit timeout"
      }), timeoutMs);
    })
  ]);
}
function formatAxeResults(axeResult) {
  const formatViolation = (v) => {
    const formatted = {
      id: v.id,
      impact: v.impact,
      description: v.description,
      helpUrl: v.helpUrl,
      nodes: []
    };
    if (v.tags) {
      formatted.wcag = v.tags.filter((t) => t.startsWith("wcag"));
    }
    formatted.nodes = (v.nodes || []).slice(0, A11Y_MAX_NODES_PER_VIOLATION).map((node) => {
      const selector = Array.isArray(node.target) ? node.target[0] : node.target;
      return {
        selector: selector || "",
        html: (node.html || "").slice(0, DOM_QUERY_MAX_HTML),
        ...node.failureSummary ? { failureSummary: node.failureSummary } : {}
      };
    });
    if (v.nodes && v.nodes.length > A11Y_MAX_NODES_PER_VIOLATION) {
      formatted.nodeCount = v.nodes.length;
    }
    return formatted;
  };
  return {
    violations: (axeResult.violations || []).map(formatViolation),
    summary: {
      violations: (axeResult.violations || []).length,
      passes: (axeResult.passes || []).length,
      incomplete: (axeResult.incomplete || []).length,
      inapplicable: (axeResult.inapplicable || []).length
    }
  };
}

// extension/inject/api.js
function setWithNativeSetter(element, proto, prop, val) {
  const setter = Object.getOwnPropertyDescriptor(proto.prototype, prop)?.set;
  if (setter)
    setter.call(element, val);
  else
    element[prop] = val;
}
function setNativeValue(element, value) {
  if (element instanceof HTMLInputElement) {
    if (element.type === "checkbox" || element.type === "radio") {
      setWithNativeSetter(element, HTMLInputElement, "checked", Boolean(value));
    } else {
      setWithNativeSetter(element, HTMLInputElement, "value", String(value));
    }
    return true;
  }
  if (element instanceof HTMLTextAreaElement) {
    setWithNativeSetter(element, HTMLTextAreaElement, "value", String(value));
    return true;
  }
  if (element instanceof HTMLSelectElement) {
    setWithNativeSetter(element, HTMLSelectElement, "value", String(value));
    return true;
  }
  return false;
}
function installGasolineAPI() {
  if (typeof window === "undefined")
    return;
  window.__gasoline = {
    /**
     * Add a context annotation that will be included with errors
     * @param key - Annotation key (e.g., 'checkout-flow', 'user')
     * @param value - Annotation value
     * @example
     * window.__gasoline.annotate('checkout-flow', { step: 'payment', items: 3 })
     */
    annotate(key, value) {
      return setContextAnnotation(key, value);
    },
    /**
     * Remove a context annotation
     * @param key - Annotation key to remove
     */
    removeAnnotation(key) {
      return removeContextAnnotation(key);
    },
    /**
     * Clear all context annotations
     */
    clearAnnotations() {
      clearContextAnnotations();
    },
    /**
     * Get current context annotations
     * @returns Current annotations or null if none
     */
    getContext() {
      return getContextAnnotations();
    },
    /**
     * Get the user action replay buffer
     * @returns Recent user actions
     */
    getActions() {
      return getActionBuffer();
    },
    /**
     * Clear the user action replay buffer
     */
    clearActions() {
      clearActionBuffer();
    },
    /**
     * Enable or disable action capture
     * @param enabled - Whether to capture user actions
     */
    setActionCapture(enabled) {
      setActionCaptureEnabled(enabled);
    },
    /**
     * Enable or disable network waterfall capture
     * @param enabled - Whether to capture network waterfall
     */
    setNetworkWaterfall(enabled) {
      setNetworkWaterfallEnabled(enabled);
    },
    /**
     * Get current network waterfall
     * @param options - Filter options
     * @returns Network waterfall entries
     */
    getNetworkWaterfall(options) {
      return getNetworkWaterfall(options);
    },
    /**
     * Enable or disable performance marks capture
     * @param enabled - Whether to capture performance marks
     */
    setPerformanceMarks(enabled) {
      setPerformanceMarksEnabled(enabled);
    },
    /**
     * Get performance marks
     * @param options - Filter options
     * @returns Performance mark entries
     */
    getMarks(options) {
      return getPerformanceMarks(options);
    },
    /**
     * Get performance measures
     * @param options - Filter options
     * @returns Performance measure entries
     */
    getMeasures(options) {
      return getPerformanceMeasures(options);
    },
    // === AI Context ===
    /**
     * Enrich an error entry with AI context
     * @param error - Error entry to enrich
     * @returns Enriched error entry
     */
    enrichError(error) {
      return enrichErrorWithAiContext(error);
    },
    /**
     * Enable or disable AI context enrichment
     * @param enabled
     */
    setAiContext(enabled) {
      setAiContextEnabled(enabled);
    },
    /**
     * Enable or disable state snapshot in AI context
     * @param enabled
     */
    setStateSnapshot(enabled) {
      setAiContextStateSnapshot(enabled);
    },
    // === Reproduction Scripts ===
    /**
     * Record an enhanced action (for testing)
     * @param type - Action type (click, input, keypress, navigate, select, scroll)
     * @param element - Target element
     * @param opts - Options
     */
    recordAction(type, element, opts) {
      recordEnhancedAction(type, element, opts);
    },
    /**
     * Get the enhanced action buffer
     * @returns
     */
    getEnhancedActions() {
      return getEnhancedActionBuffer();
    },
    /**
     * Clear the enhanced action buffer
     */
    clearEnhancedActions() {
      clearEnhancedActionBuffer();
    },
    /**
     * Generate a Playwright reproduction script
     * @param opts - Generation options
     * @returns Playwright test script
     */
    generateScript(opts) {
      return generatePlaywrightScript(getEnhancedActionBuffer(), opts);
    },
    /**
     * Compute multi-strategy selectors for an element
     * @param element
     * @returns
     */
    getSelectors(element) {
      return computeSelectors(element);
    },
    /**
     * Set input value and trigger React/Vue/Svelte change events
     * Works with frameworks that track form state internally by dispatching
     * the events that frameworks listen for.
     *
     * @param selector - CSS selector for the input element
     * @param value - Value to set (string for text inputs, boolean for checkboxes)
     * @returns true if successful, false if element not found
     *
     * @example
     * // Text input
     * window.__gasoline.setInputValue('input[name="email"]', 'test@example.com')
     *
     * // Checkbox
     * window.__gasoline.setInputValue('input[type="checkbox"]', true)
     *
     * // Select dropdown
     * window.__gasoline.setInputValue('select[name="country"]', 'US')
     */
    setInputValue(selector, value) {
      const element = document.querySelector(selector);
      if (!element) {
        console.error("[Gasoline] Element not found:", selector);
        return false;
      }
      try {
        if (!setNativeValue(element, value)) {
          console.error("[Gasoline] Element is not a form input:", selector);
          return false;
        }
        element.dispatchEvent(new Event("input", { bubbles: true }));
        element.dispatchEvent(new Event("change", { bubbles: true }));
        element.dispatchEvent(new Event("blur", { bubbles: true }));
        return true;
      } catch (err) {
        console.error("[Gasoline] Failed to set input value:", err);
        return false;
      }
    },
    /**
     * Version of the Gasoline API
     */
    version: "6.1.0"
  };
}
function uninstallGasolineAPI() {
  if (typeof window !== "undefined" && window.__gasoline) {
    delete window.__gasoline;
  }
}

// extension/inject/observers.js
var originalFetch = null;
var deferralEnabled = true;
var phase2Installed = false;
var injectionTimestamp = 0;
var phase2Timestamp = 0;
function wrapFetch(originalFetchFn) {
  return async function(input, init) {
    const startTime = Date.now();
    const url = typeof input === "string" ? input : input.url;
    const method = init?.method || (typeof input === "object" && "method" in input ? input.method : "GET") || "GET";
    try {
      const response = await originalFetchFn(input, init);
      const duration = Date.now() - startTime;
      if (!response.ok) {
        let responseBody = "";
        try {
          const cloned = response.clone();
          responseBody = await cloned.text();
          if (responseBody.length > MAX_RESPONSE_LENGTH) {
            responseBody = responseBody.slice(0, MAX_RESPONSE_LENGTH) + "... [truncated]";
          }
        } catch {
          responseBody = "[Could not read response]";
        }
        const safeHeaders = {};
        const rawHeaders = init?.headers || (typeof input === "object" && "headers" in input ? input.headers : null);
        if (rawHeaders) {
          const headers = rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : rawHeaders;
          Object.keys(headers).forEach((key) => {
            const value = headers[key];
            if (value && !SENSITIVE_HEADERS.includes(key.toLowerCase())) {
              safeHeaders[key] = value;
            }
          });
        }
        const logPayload = {
          level: "error",
          type: "network",
          method: method.toUpperCase(),
          url,
          status: response.status,
          statusText: response.statusText,
          duration,
          response: responseBody,
          ...Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}
        };
        postLog(logPayload);
      }
      return response;
    } catch (error) {
      const duration = Date.now() - startTime;
      const safeHeaders = {};
      const rawHeaders = init?.headers || (typeof input === "object" && "headers" in input ? input.headers : null);
      if (rawHeaders) {
        const headers = rawHeaders instanceof Headers ? Object.fromEntries(rawHeaders) : rawHeaders;
        Object.keys(headers).forEach((key) => {
          const value = headers[key];
          if (value && !SENSITIVE_HEADERS.includes(key.toLowerCase())) {
            safeHeaders[key] = value;
          }
        });
      }
      const logPayload = {
        level: "error",
        type: "network",
        method: method.toUpperCase(),
        url,
        error: error.message,
        duration,
        ...Object.keys(safeHeaders).length > 0 ? { headers: safeHeaders } : {}
      };
      postLog(logPayload);
      throw error;
    }
  };
}
function installFetchCapture() {
  originalFetch = window.fetch;
  const wrappedWithBodies = wrapFetchWithBodies(originalFetch);
  window.fetch = wrapFetch(wrappedWithBodies);
}
function uninstallFetchCapture() {
  if (originalFetch) {
    window.fetch = originalFetch;
    originalFetch = null;
  }
}
function install() {
  installConsoleCapture();
  installFetchCapture();
  installExceptionCapture();
  installActionCapture();
  installNavigationCapture();
  installWebSocketCapture();
  installPerformanceCapture();
}
function uninstall() {
  uninstallConsoleCapture();
  uninstallFetchCapture();
  uninstallExceptionCapture();
  uninstallActionCapture();
  uninstallNavigationCapture();
  uninstallWebSocketCapture();
  uninstallPerformanceCapture();
}
function shouldDeferIntercepts() {
  if (typeof document === "undefined")
    return false;
  return document.readyState === "loading";
}
function checkMemoryPressure(state) {
  const result = { ...state };
  if (state.memoryUsageMB >= MEMORY_HARD_LIMIT_MB) {
    result.networkBodiesEnabled = false;
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.25);
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.25);
  } else if (state.memoryUsageMB >= MEMORY_SOFT_LIMIT_MB) {
    result.wsBufferCapacity = Math.floor(state.wsBufferCapacity * 0.5);
    result.networkBufferCapacity = Math.floor(state.networkBufferCapacity * 0.5);
  }
  return result;
}
function installPhase1() {
  console.log("[Gasoline] Phase 1 installing (lightweight API + perf observers)");
  injectionTimestamp = performance.now();
  phase2Installed = false;
  phase2Timestamp = 0;
  installPerformanceCapture();
  if (!deferralEnabled) {
    installPhase2();
  } else {
    const installDeferred = () => {
      if (!phase2Installed)
        setTimeout(installPhase2, 100);
    };
    if (document.readyState === "complete") {
      installDeferred();
    } else {
      window.addEventListener("load", installDeferred, { once: true });
      setTimeout(() => {
        if (!phase2Installed)
          installPhase2();
      }, 1e4);
    }
  }
}
function installPhase2() {
  if (phase2Installed)
    return;
  if (typeof window === "undefined" || typeof document === "undefined")
    return;
  console.log("[Gasoline] Phase 2 installing (heavy interceptors: console, fetch, WS, errors, actions)");
  phase2Timestamp = performance.now();
  phase2Installed = true;
  install();
  installPerfObservers();
}
function getDeferralState() {
  return {
    deferralEnabled,
    phase2Installed,
    injectionTimestamp,
    phase2Timestamp
  };
}
function setDeferralEnabled(enabled) {
  deferralEnabled = enabled;
}

// extension/lib/timeout-utils.js
function createDeferredPromise() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

// extension/lib/link-health.js
function extractUniqueLinks() {
  const linkElements = document.querySelectorAll("a[href]");
  const urls = /* @__PURE__ */ new Set();
  for (const elem of linkElements) {
    const href = elem.href;
    if (href && !isIgnoredLink(href))
      urls.add(href);
  }
  return Array.from(urls);
}
function aggregateResults(results) {
  const summary = { totalLinks: results.length, ok: 0, redirect: 0, requiresAuth: 0, broken: 0, timeout: 0, corsBlocked: 0, needsServerVerification: 0 };
  const codeToField = {
    ok: "ok",
    redirect: "redirect",
    requires_auth: "requiresAuth",
    broken: "broken",
    timeout: "timeout",
    cors_blocked: "corsBlocked"
  };
  for (const result of results) {
    const field = codeToField[result.code];
    if (field)
      summary[field]++;
    if (result.code === "cors_blocked" && result.needsServerVerification)
      summary.needsServerVerification++;
  }
  return summary;
}
async function checkLinkHealth(params) {
  const timeout_ms = params.timeout_ms || 15e3;
  const max_workers = params.max_workers || 20;
  const uniqueLinks = extractUniqueLinks();
  const results = [];
  const chunks = chunkArray(uniqueLinks, max_workers);
  for (const chunk of chunks) {
    const batchResults = await Promise.allSettled(chunk.map((url) => checkLink(url, timeout_ms)));
    for (const result of batchResults) {
      if (result.status === "fulfilled" && result.value)
        results.push(result.value);
    }
  }
  return { summary: aggregateResults(results), results };
}
async function checkLink(url, timeout_ms) {
  const startTime = performance.now();
  const isExternal = new URL(url).origin !== window.location.origin;
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout_ms);
    try {
      const response = await fetch(url, {
        method: "HEAD",
        signal: controller.signal,
        redirect: "follow"
      });
      clearTimeout(timeoutId);
      const timeMs = Math.round(performance.now() - startTime);
      if (response.status === 0) {
        return {
          url,
          status: null,
          code: "cors_blocked",
          timeMs,
          isExternal,
          error: "CORS policy blocked the request",
          needsServerVerification: isExternal
          // Only external links need server verification
        };
      }
      let code;
      if (response.status >= 200 && response.status < 300) {
        code = "ok";
      } else if (response.status >= 300 && response.status < 400) {
        code = "redirect";
      } else if (response.status === 401 || response.status === 403) {
        code = "requires_auth";
      } else if (response.status >= 400) {
        code = "broken";
      } else {
        code = "broken";
      }
      return {
        url,
        status: response.status,
        code,
        timeMs,
        isExternal,
        redirectTo: response.redirected ? response.url : void 0
      };
    } finally {
      clearTimeout(timeoutId);
    }
  } catch (error) {
    const timeMs = Math.round(performance.now() - startTime);
    const isTimeout = error.name === "AbortError";
    return {
      url,
      status: null,
      code: isTimeout ? "timeout" : "broken",
      timeMs,
      isExternal,
      error: isTimeout ? "timeout" : error.message
    };
  }
}
function isIgnoredLink(href) {
  if (href.startsWith("javascript:"))
    return true;
  if (href.startsWith("mailto:"))
    return true;
  if (href.startsWith("tel:"))
    return true;
  if (href.startsWith("#"))
    return true;
  if (href === "")
    return true;
  return false;
}
function chunkArray(arr, chunkSize) {
  const chunks = [];
  for (let i = 0; i < arr.length; i += chunkSize) {
    chunks.push(arr.slice(i, i + chunkSize));
  }
  return chunks;
}

// extension/inject/message-handlers.js
var pageNonce = "";
if (typeof document !== "undefined" && typeof document.querySelector === "function") {
  const nonceEl = document.querySelector("script[data-gasoline-nonce]");
  if (nonceEl) {
    pageNonce = nonceEl.getAttribute("data-gasoline-nonce") || "";
  }
}
var VALID_SETTINGS = /* @__PURE__ */ new Set([
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
var VALID_STATE_ACTIONS = /* @__PURE__ */ new Set(["capture", "restore"]);
function serializeObject2(obj, depth, seen) {
  if (seen.has(obj))
    return "[Circular]";
  seen.add(obj);
  if (Array.isArray(obj))
    return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen));
  if (obj instanceof Error)
    return { error: obj.message, stack: obj.stack };
  if (obj instanceof Date)
    return obj.toISOString();
  if (obj instanceof RegExp)
    return obj.toString();
  if (typeof Node !== "undefined" && obj instanceof Node) {
    const node = obj;
    return `[${node.nodeName}${node.id ? "#" + node.id : ""}]`;
  }
  const result = {};
  const keys = Object.keys(obj).slice(0, 50);
  for (const key of keys) {
    try {
      result[key] = safeSerializeForExecute(obj[key], depth + 1, seen);
    } catch {
      result[key] = "[unserializable]";
    }
  }
  if (Object.keys(obj).length > 50) {
    result["..."] = `[${Object.keys(obj).length - 50} more keys]`;
  }
  return result;
}
function safeSerializeForExecute(value, depth = 0, seen = /* @__PURE__ */ new WeakSet()) {
  if (depth > 10)
    return "[max depth exceeded]";
  if (value === null || value === void 0)
    return value;
  const type = typeof value;
  if (type === "string" || type === "number" || type === "boolean")
    return value;
  if (type === "function")
    return `[Function: ${value.name || "anonymous"}]`;
  if (type === "symbol")
    return value.toString();
  if (type === "object")
    return serializeObject2(value, depth, seen);
  return String(value);
}
function executeJavaScript(script, timeoutMs = 5e3) {
  const deferred = createDeferredPromise();
  const executeWithTimeoutProtection = async () => {
    const timeoutHandle = setTimeout(() => {
      deferred.resolve({
        success: false,
        error: "execution_timeout",
        message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
      });
    }, timeoutMs);
    try {
      const cleanScript = script.trim();
      let fn;
      try {
        fn = new Function(`"use strict"; return (${cleanScript});`);
      } catch {
        fn = new Function(`"use strict"; ${cleanScript}`);
      }
      const result = fn();
      if (result && typeof result.then === "function") {
        ;
        result.then((value) => {
          clearTimeout(timeoutHandle);
          deferred.resolve({ success: true, result: safeSerializeForExecute(value) });
        }).catch((err) => {
          clearTimeout(timeoutHandle);
          deferred.resolve({
            success: false,
            error: "promise_rejected",
            message: err.message,
            stack: err.stack
          });
        });
      } else {
        clearTimeout(timeoutHandle);
        deferred.resolve({ success: true, result: safeSerializeForExecute(result) });
      }
    } catch (err) {
      clearTimeout(timeoutHandle);
      const error = err;
      if (error.message && (error.message.includes("Content Security Policy") || error.message.includes("unsafe-eval") || error.message.includes("Trusted Type"))) {
        deferred.resolve({
          success: false,
          error: "csp_blocked",
          message: 'This page has a Content Security Policy that blocks script execution in the MAIN world. Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). With world: "auto" (default), this fallback happens automatically.'
        });
      } else {
        deferred.resolve({
          success: false,
          error: "execution_error",
          message: error.message,
          stack: error.stack
        });
      }
    }
  };
  executeWithTimeoutProtection().catch((err) => {
    console.error("[Gasoline] Unexpected error in executeJavaScript:", err);
    deferred.resolve({
      success: false,
      error: "execution_error",
      message: "Unexpected error during script execution"
    });
  });
  return deferred.promise;
}
async function handleLinkHealthQuery(data) {
  try {
    const params = data.params || {};
    const result = await checkLinkHealth(params);
    return result;
  } catch (err) {
    return {
      error: "link_health_error",
      message: err.message || "Failed to check link health"
    };
  }
}
function isValidSettingPayload(data) {
  if (!VALID_SETTINGS.has(data.setting)) {
    console.warn("[Gasoline] Invalid setting:", data.setting);
    return false;
  }
  if (data.setting === "setWebSocketCaptureMode")
    return typeof data.mode === "string";
  if (data.setting === "setServerUrl")
    return typeof data.url === "string";
  if (typeof data.enabled !== "boolean") {
    console.warn("[Gasoline] Invalid enabled value type");
    return false;
  }
  return true;
}
function handleLinkHealthMessage(data) {
  handleLinkHealthQuery(data).then((result) => {
    window.postMessage({ type: "GASOLINE_LINK_HEALTH_RESPONSE", requestId: data.requestId, result }, window.location.origin);
  }).catch((err) => {
    window.postMessage({
      type: "GASOLINE_LINK_HEALTH_RESPONSE",
      requestId: data.requestId,
      result: { error: "link_health_error", message: err.message || "Failed to check link health" }
    }, window.location.origin);
  });
}
function installMessageListener(captureStateFn, restoreStateFn) {
  if (typeof window === "undefined")
    return;
  const messageHandlers = {
    GASOLINE_SETTING: (data) => {
      const settingData = data;
      if (isValidSettingPayload(settingData))
        handleSetting(settingData);
    },
    GASOLINE_STATE_COMMAND: (data) => handleStateCommand(data, captureStateFn, restoreStateFn),
    GASOLINE_EXECUTE_JS: (data) => handleExecuteJs(data),
    GASOLINE_A11Y_QUERY: (data) => handleA11yQuery(data),
    GASOLINE_DOM_QUERY: (data) => handleDomQuery(data),
    GASOLINE_GET_WATERFALL: (data) => handleGetWaterfall(data),
    GASOLINE_LINK_HEALTH_QUERY: (data) => handleLinkHealthMessage(data)
  };
  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== window.location.origin)
      return;
    if (pageNonce && event.data?._nonce !== pageNonce)
      return;
    const msgType = event.data?.type;
    if (!msgType)
      return;
    const handler = messageHandlers[msgType];
    if (handler)
      handler(event.data);
  });
}
var SETTING_HANDLERS = {
  setNetworkWaterfallEnabled: (data) => setNetworkWaterfallEnabled(data.enabled),
  setPerformanceMarksEnabled: (data) => {
    setPerformanceMarksEnabled(data.enabled);
    if (data.enabled)
      installPerformanceCapture();
    else
      uninstallPerformanceCapture();
  },
  setActionReplayEnabled: (data) => setActionCaptureEnabled(data.enabled),
  setWebSocketCaptureEnabled: (data) => {
    setWebSocketCaptureEnabled(data.enabled);
    if (data.enabled)
      installWebSocketCapture();
    else
      uninstallWebSocketCapture();
  },
  setWebSocketCaptureMode: (data) => setWebSocketCaptureMode(data.mode || "medium"),
  setPerformanceSnapshotEnabled: (data) => setPerformanceSnapshotEnabled(data.enabled),
  setDeferralEnabled: (data) => setDeferralEnabled(data.enabled),
  setNetworkBodyCaptureEnabled: (data) => setNetworkBodyCaptureEnabled(data.enabled),
  setServerUrl: (data) => setServerUrl(data.url)
};
function handleSetting(data) {
  const handler = SETTING_HANDLERS[data.setting];
  if (handler)
    handler(data);
}
function handleStateCommand(data, captureStateFn, restoreStateFn) {
  const { messageId, action, state } = data;
  if (!VALID_STATE_ACTIONS.has(action)) {
    console.warn("[Gasoline] Invalid state action:", action);
    window.postMessage({
      type: "GASOLINE_STATE_RESPONSE",
      messageId,
      result: { error: `Invalid action: ${action}` }
    }, window.location.origin);
    return;
  }
  if (action === "restore" && (!state || typeof state !== "object")) {
    console.warn("[Gasoline] Invalid state object for restore");
    window.postMessage({
      type: "GASOLINE_STATE_RESPONSE",
      messageId,
      result: { error: "Invalid state object" }
    }, window.location.origin);
    return;
  }
  let result;
  try {
    if (action === "capture") {
      result = captureStateFn();
    } else if (action === "restore") {
      const includeUrl = data.include_url !== false;
      result = restoreStateFn(state, includeUrl);
    } else {
      result = { error: `Unknown action: ${action}` };
    }
  } catch (err) {
    result = { error: err.message };
  }
  window.postMessage({
    type: "GASOLINE_STATE_RESPONSE",
    messageId,
    result
  }, window.location.origin);
}
function handleExecuteJs(data) {
  const { requestId, script, timeoutMs } = data;
  if (typeof script !== "string") {
    console.warn("[Gasoline] Script must be a string");
    window.postMessage({
      type: "GASOLINE_EXECUTE_JS_RESULT",
      requestId,
      result: { success: false, error: "invalid_script", message: "Script must be a string" }
    }, window.location.origin);
    return;
  }
  if (typeof requestId !== "number" && typeof requestId !== "string") {
    console.warn("[Gasoline] Invalid requestId type");
    return;
  }
  executeJavaScript(script, timeoutMs).then((result) => {
    window.postMessage({
      type: "GASOLINE_EXECUTE_JS_RESULT",
      requestId,
      result
    }, window.location.origin);
  }).catch((err) => {
    console.error("[Gasoline] Failed to execute JS:", err);
    window.postMessage({
      type: "GASOLINE_EXECUTE_JS_RESULT",
      requestId,
      result: { success: false, error: "execution_failed", message: err.message }
    }, window.location.origin);
  });
}
function handleA11yQuery(data) {
  const { requestId, params } = data;
  if (typeof runAxeAuditWithTimeout !== "function") {
    window.postMessage({
      type: "GASOLINE_A11Y_QUERY_RESPONSE",
      requestId,
      result: {
        error: "runAxeAuditWithTimeout not available - try reloading the extension"
      }
    }, window.location.origin);
    return;
  }
  try {
    runAxeAuditWithTimeout(params || {}).then((result) => {
      window.postMessage({
        type: "GASOLINE_A11Y_QUERY_RESPONSE",
        requestId,
        result
      }, window.location.origin);
    }).catch((err) => {
      console.error("[Gasoline] Accessibility audit error:", err);
      window.postMessage({
        type: "GASOLINE_A11Y_QUERY_RESPONSE",
        requestId,
        result: { error: err.message || "Accessibility audit failed" }
      }, window.location.origin);
    });
  } catch (err) {
    console.error("[Gasoline] Failed to run accessibility audit:", err);
    window.postMessage({
      type: "GASOLINE_A11Y_QUERY_RESPONSE",
      requestId,
      result: { error: err.message || "Failed to run accessibility audit" }
    }, window.location.origin);
  }
}
function handleDomQuery(data) {
  const { requestId, params } = data;
  if (typeof executeDOMQuery !== "function") {
    window.postMessage({
      type: "GASOLINE_DOM_QUERY_RESPONSE",
      requestId,
      result: {
        error: "executeDOMQuery not available - try reloading the extension"
      }
    }, window.location.origin);
    return;
  }
  try {
    executeDOMQuery(params || {}).then((result) => {
      window.postMessage({
        type: "GASOLINE_DOM_QUERY_RESPONSE",
        requestId,
        result
      }, window.location.origin);
    }).catch((err) => {
      console.error("[Gasoline] DOM query error:", err);
      window.postMessage({
        type: "GASOLINE_DOM_QUERY_RESPONSE",
        requestId,
        result: { error: err.message || "DOM query failed" }
      }, window.location.origin);
    });
  } catch (err) {
    console.error("[Gasoline] Failed to run DOM query:", err);
    window.postMessage({
      type: "GASOLINE_DOM_QUERY_RESPONSE",
      requestId,
      result: { error: err.message || "Failed to run DOM query" }
    }, window.location.origin);
  }
}
function handleGetWaterfall(data) {
  const { requestId } = data;
  try {
    const entries = getNetworkWaterfall({});
    const snakeEntries = (entries || []).map((e) => ({
      url: e.url,
      name: e.url,
      initiator_type: e.initiatorType,
      start_time: e.startTime,
      duration: e.duration,
      transfer_size: e.transferSize,
      encoded_body_size: e.encodedBodySize,
      decoded_body_size: e.decodedBodySize
    }));
    window.postMessage({
      type: "GASOLINE_WATERFALL_RESPONSE",
      requestId,
      entries: snakeEntries,
      page_url: window.location.href
    }, window.location.origin);
  } catch (err) {
    console.error("[Gasoline] Failed to get network waterfall:", err);
    window.postMessage({
      type: "GASOLINE_WATERFALL_RESPONSE",
      requestId,
      entries: []
    }, window.location.origin);
  }
}

// extension/inject/state.js
var pageNonce2 = "";
if (typeof document !== "undefined" && typeof document.querySelector === "function") {
  const nonceEl = document.querySelector("script[data-gasoline-nonce]");
  if (nonceEl) {
    pageNonce2 = nonceEl.getAttribute("data-gasoline-nonce") || "";
  }
}
var SENSITIVE_KEY_PATTERNS = /token|secret|password|api.?key|auth|session.?id|csrf|jwt/i;
var gasolineHighlighter = null;
function captureState() {
  const state = {
    url: window.location.href,
    timestamp: Date.now(),
    localStorage: {},
    sessionStorage: {},
    cookies: document.cookie.split(";").map((c) => {
      const [name, ...rest] = c.split("=");
      if (name && SENSITIVE_KEY_PATTERNS.test(name.trim())) {
        return `${name}=[REDACTED]`;
      }
      return c;
    }).join(";")
  };
  const localStorageData = {};
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key) {
      localStorageData[key] = SENSITIVE_KEY_PATTERNS.test(key) ? "[REDACTED]" : localStorage.getItem(key) || "";
    }
  }
  ;
  state.localStorage = localStorageData;
  const sessionStorageData = {};
  for (let i = 0; i < sessionStorage.length; i++) {
    const key = sessionStorage.key(i);
    if (key) {
      sessionStorageData[key] = SENSITIVE_KEY_PATTERNS.test(key) ? "[REDACTED]" : sessionStorage.getItem(key) || "";
    }
  }
  ;
  state.sessionStorage = sessionStorageData;
  return state;
}
function isValidStorageKey(key) {
  if (typeof key !== "string")
    return false;
  if (key.length === 0 || key.length > 256)
    return false;
  const dangerous = ["__proto__", "constructor", "prototype"];
  const lowerKey = key.toLowerCase();
  for (const pattern of dangerous) {
    if (lowerKey.includes(pattern))
      return false;
  }
  return true;
}
var MAX_STORAGE_VALUE_SIZE = 10 * 1024 * 1024;
function restoreStorageEntries(storage, entries, label) {
  let skipped = 0;
  for (const [key, value] of Object.entries(entries)) {
    if (!isValidStorageKey(key)) {
      skipped++;
      console.warn(`[gasoline] Skipped ${label} key with invalid pattern:`, key);
      continue;
    }
    if (typeof value === "string" && value.length > MAX_STORAGE_VALUE_SIZE) {
      skipped++;
      console.warn(`[gasoline] Skipped ${label} value exceeding 10MB:`, key);
      continue;
    }
    storage.setItem(key, value);
  }
  return skipped;
}
function clearAllCookies() {
  const isSecure = window.location.protocol === "https:";
  document.cookie.split(";").forEach((c) => {
    const name = (c.split("=")[0] || "").trim();
    if (!name)
      return;
    let deleteCookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/`;
    if (isSecure)
      deleteCookie += "; Secure";
    deleteCookie += "; SameSite=Strict";
    document.cookie = deleteCookie;
  });
}
function restoreCookies(cookieString) {
  const isSecure = window.location.protocol === "https:";
  cookieString.split(";").forEach((c) => {
    const trimmed = c.trim();
    if (!trimmed)
      return;
    let securedCookie = trimmed;
    if (isSecure && !securedCookie.toLowerCase().includes("secure"))
      securedCookie += "; Secure";
    if (!securedCookie.toLowerCase().includes("samesite"))
      securedCookie += "; SameSite=Strict";
    document.cookie = securedCookie;
  });
}
function navigateSameOrigin(url) {
  if (url === window.location.href)
    return;
  try {
    const parsed = new URL(url);
    if ((parsed.protocol === "http:" || parsed.protocol === "https:") && parsed.origin === window.location.origin) {
      window.location.href = url;
    } else {
      console.warn("[gasoline] Skipped navigation: URL must be same origin", url, "current:", window.location.origin);
    }
  } catch (e) {
    console.warn("[gasoline] Invalid URL for navigation:", url, e);
  }
}
function restoreState(state, includeUrl = true) {
  if (!state || typeof state !== "object") {
    return { success: false, error: "Invalid state object" };
  }
  let skipped = restoreStorageEntries(localStorage, state.localStorage || {}, "localStorage");
  skipped += restoreStorageEntries(sessionStorage, state.sessionStorage || {}, "sessionStorage");
  clearAllCookies();
  if (state.cookies)
    restoreCookies(state.cookies);
  const restored = {
    localStorage: Object.keys(state.localStorage || {}).length - skipped,
    sessionStorage: Object.keys(state.sessionStorage || {}).length,
    cookies: (state.cookies || "").split(";").filter((c) => c.trim()).length,
    skipped
  };
  if (includeUrl && state.url)
    navigateSameOrigin(state.url);
  if (skipped > 0)
    console.warn(`[gasoline] restoreState completed with ${skipped} skipped item(s)`);
  return { success: true, restored };
}
function highlightElement(selector, durationMs = 5e3) {
  if (gasolineHighlighter) {
    gasolineHighlighter.remove();
    gasolineHighlighter = null;
  }
  const element = document.querySelector(selector);
  if (!element) {
    return { success: false, error: "element_not_found", selector };
  }
  const rect = element.getBoundingClientRect();
  gasolineHighlighter = document.createElement("div");
  gasolineHighlighter.id = "gasoline-highlighter";
  gasolineHighlighter.dataset.selector = selector;
  Object.assign(gasolineHighlighter.style, {
    position: "fixed",
    top: `${rect.top}px`,
    left: `${rect.left}px`,
    width: `${rect.width}px`,
    height: `${rect.height}px`,
    border: "2px solid rgba(59, 130, 246, 0.7)",
    borderRadius: "4px",
    backgroundColor: "rgba(59, 130, 246, 0.08)",
    boxShadow: "0 0 12px rgba(59, 130, 246, 0.5)",
    zIndex: "2147483647",
    pointerEvents: "none",
    boxSizing: "border-box"
  });
  const targetElement = document.body || document.documentElement;
  if (targetElement) {
    targetElement.appendChild(gasolineHighlighter);
  } else {
    console.warn("[Gasoline] No document body available for highlighter injection");
    return;
  }
  setTimeout(() => {
    if (gasolineHighlighter) {
      gasolineHighlighter.remove();
      gasolineHighlighter = null;
    }
  }, durationMs);
  return {
    success: true,
    selector,
    bounds: { x: rect.x, y: rect.y, width: rect.width, height: rect.height }
  };
}
function clearHighlight() {
  if (gasolineHighlighter) {
    gasolineHighlighter.remove();
    gasolineHighlighter = null;
  }
}
if (typeof window !== "undefined") {
  window.addEventListener("scroll", () => {
    if (gasolineHighlighter) {
      const selector = gasolineHighlighter.dataset.selector;
      if (selector) {
        const el = document.querySelector(selector);
        if (el) {
          const rect = el.getBoundingClientRect();
          gasolineHighlighter.style.top = `${rect.top}px`;
          gasolineHighlighter.style.left = `${rect.left}px`;
        }
      }
    }
  }, { passive: true });
}
if (typeof window !== "undefined") {
  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== window.location.origin)
      return;
    if (pageNonce2 && event.data?._nonce !== pageNonce2)
      return;
    if (event.data?.type === "GASOLINE_HIGHLIGHT_REQUEST") {
      const { requestId, params } = event.data;
      const { selector, duration_ms } = params || { selector: "" };
      const result = highlightElement(selector, duration_ms);
      window.postMessage({
        type: "GASOLINE_HIGHLIGHT_RESPONSE",
        requestId,
        result
      }, window.location.origin);
    }
  });
}

// extension/inject/index.js
if (typeof window !== "undefined" && typeof document !== "undefined" && typeof globalThis.process === "undefined") {
  installPhase1();
  installMessageListener(captureState, restoreState);
  installGasolineAPI();
  window.addEventListener("load", () => {
    setTimeout(() => {
      sendPerformanceSnapshot();
    }, 2e3);
  });
}
export {
  MAX_PERFORMANCE_ENTRIES,
  MAX_WATERFALL_ENTRIES,
  SENSITIVE_HEADERS,
  aggregateResourceTiming,
  capturePerformanceSnapshot,
  captureState,
  captureStateSnapshot,
  checkMemoryPressure,
  clearActionBuffer,
  clearContextAnnotations,
  clearEnhancedActionBuffer,
  clearHighlight,
  clearPendingRequests,
  completePendingRequest,
  computeCssPath,
  computeSelectors,
  createConnectionTracker,
  detectFramework,
  enrichErrorWithAiContext,
  executeDOMQuery,
  executeJavaScript,
  extractSnippet,
  extractSourceSnippets,
  formatAxeResults,
  formatPayload,
  generateAiSummary,
  generatePlaywrightScript,
  getActionBuffer,
  getCLS,
  getCapturedMarks,
  getCapturedMeasures,
  getContextAnnotations,
  getDeferralState,
  getElementSelector,
  getEnhancedActionBuffer,
  getFCP,
  getINP,
  getImplicitRole,
  getLCP,
  getLongTaskMetrics,
  getNetworkWaterfall,
  getNetworkWaterfallForError,
  getPageInfo,
  getPendingRequests,
  getPerformanceMarks,
  getPerformanceMeasures,
  getPerformanceSnapshotForError,
  getReactComponentAncestry,
  getSize,
  getSourceMapCache,
  getSourceMapCacheSize,
  getWebSocketCaptureMode,
  handleChange,
  handleClick,
  handleInput,
  handleKeydown,
  handleScroll,
  highlightElement,
  install,
  installActionCapture,
  installConsoleCapture,
  installExceptionCapture,
  installFetchCapture,
  installGasolineAPI,
  installMessageListener,
  installNavigationCapture,
  installPerfObservers,
  installPerformanceCapture,
  installPhase1,
  installPhase2,
  installWebSocketCapture,
  isDynamicClass,
  isNetworkBodyCaptureEnabled,
  isNetworkWaterfallEnabled,
  isPerformanceCaptureActive,
  isPerformanceMarksEnabled,
  isPerformanceSnapshotEnabled,
  isSensitiveInput,
  mapInitiatorType,
  parseResourceTiming,
  parseSourceMap,
  parseStackFrames,
  postLog,
  readResponseBody,
  readResponseBodyWithTimeout,
  recordAction,
  recordEnhancedAction,
  removeContextAnnotation,
  resetForTesting,
  restoreState,
  runAxeAudit,
  runAxeAuditWithTimeout,
  safeSerialize,
  safeSerializeForExecute,
  sanitizeHeaders,
  sendPerformanceSnapshot,
  setActionCaptureEnabled,
  setAiContextEnabled,
  setAiContextStateSnapshot,
  setContextAnnotation,
  setDeferralEnabled,
  setNetworkBodyCaptureEnabled,
  setNetworkWaterfallEnabled,
  setPerformanceMarksEnabled,
  setPerformanceSnapshotEnabled,
  setServerUrl,
  setSourceMapCache,
  setWebSocketCaptureEnabled,
  setWebSocketCaptureMode,
  shouldCaptureUrl,
  shouldDeferIntercepts,
  trackPendingRequest,
  truncateRequestBody,
  truncateResponseBody,
  truncateWsMessage,
  uninstall,
  uninstallActionCapture,
  uninstallConsoleCapture,
  uninstallExceptionCapture,
  uninstallFetchCapture,
  uninstallGasolineAPI,
  uninstallNavigationCapture,
  uninstallPerfObservers,
  uninstallPerformanceCapture,
  uninstallWebSocketCapture,
  wrapFetch,
  wrapFetchWithBodies
};
//# sourceMappingURL=inject.bundled.js.map
