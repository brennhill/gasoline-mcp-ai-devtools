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
  var DEFAULT_SERVER_URL = "http://localhost:7890";
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
  var RuntimeMessageName = {
    SHOW_TRACKED_HOVER_LAUNCHER: "GASOLINE_SHOW_TRACKED_HOVER_LAUNCHER"
  };
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
    TRACKED_HOVER_LAUNCHER_HIDDEN: "gasoline_tracked_hover_launcher_hidden",
    PENDING_RECORDING: "gasoline_pending_recording",
    PENDING_MIC_RECORDING: "gasoline_pending_mic_recording",
    MIC_GRANTED: "gasoline_mic_granted",
    RECORD_AUDIO_PREF: "gasoline_record_audio_pref",
    TERMINAL_CONFIG: "gasoline_terminal_config",
    TERMINAL_AI_COMMAND: "gasoline_terminal_ai_command",
    TERMINAL_DEV_ROOT: "gasoline_terminal_dev_root",
    POPUP_LAST_STATUS: "gasoline_popup_last_status",
    TERMINAL_SESSION: "gasoline_terminal_session",
    TERMINAL_UI_STATE: "gasoline_terminal_ui_state"
  };

  // extension/popup/ui-utils.js
  function formatFileSize(bytes) {
    if (bytes === 0)
      return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`;
  }
  function isInternalUrl(url) {
    if (!url)
      return true;
    const internalPrefixes = ["chrome://", "chrome-extension://", "about:", "edge://", "brave://", "devtools://"];
    return internalPrefixes.some((prefix) => url.startsWith(prefix));
  }

  // extension/popup/status-display.js
  var DEFAULT_MAX_ENTRIES = 1e3;
  function updateConnectionStatus(status) {
    const statusEl = document.getElementById("status");
    const entriesEl = document.getElementById("entries-count");
    const errorEl = document.getElementById("error-message");
    const serverUrlEl = document.getElementById("server-url");
    const logFileEl = document.getElementById("log-file-path");
    const errorCountEl = document.getElementById("error-count");
    const troubleshootingEl = document.getElementById("troubleshooting");
    if (status.connected) {
      if (statusEl) {
        statusEl.textContent = "Connected";
        statusEl.classList.remove("disconnected");
        statusEl.classList.add("connected");
      }
      const entries = status.entries || 0;
      const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES;
      if (entriesEl) {
        entriesEl.textContent = `${entries} / ${maxEntries}`;
      }
      if (errorEl) {
        errorEl.textContent = "";
      }
      if (troubleshootingEl) {
        troubleshootingEl.style.display = "none";
      }
    } else {
      if (statusEl) {
        statusEl.textContent = "Offline";
        statusEl.classList.remove("connected");
        statusEl.classList.add("disconnected");
      }
      if (errorEl && status.error) {
        errorEl.textContent = status.error;
      }
      if (troubleshootingEl) {
        troubleshootingEl.style.display = "block";
      }
    }
    const versionWarningEl = document.getElementById("version-mismatch");
    if (versionWarningEl) {
      if (status.versionMismatch && status.serverVersion && status.extensionVersion) {
        versionWarningEl.style.display = "block";
        const versionDetail = versionWarningEl.querySelector(".version-detail");
        if (versionDetail) {
          versionDetail.textContent = `Server: v${status.serverVersion} / Extension: v${status.extensionVersion}`;
        }
      } else {
        versionWarningEl.style.display = "none";
      }
    }
    const securityWarningEl = document.getElementById("security-mode-warning");
    const securityDetailEl = document.getElementById("security-mode-detail");
    if (securityWarningEl) {
      if (status.securityMode === "insecure_proxy") {
        securityWarningEl.style.display = "block";
        if (securityDetailEl) {
          const rewrites = status.insecureRewritesApplied && status.insecureRewritesApplied.length > 0 ? status.insecureRewritesApplied.join(", ") : "csp_headers";
          securityDetailEl.textContent = `INSECURE DEBUG MODE active. production_parity=${status.productionParity === false ? "false" : "true"}; rewrites=${rewrites}`;
        }
      } else {
        securityWarningEl.style.display = "none";
        if (securityDetailEl) {
          securityDetailEl.textContent = "";
        }
      }
    }
    if (serverUrlEl && status.serverUrl) {
      serverUrlEl.textContent = status.serverUrl;
    }
    if (logFileEl && status.logFile) {
      logFileEl.textContent = status.logFile;
    }
    if (errorCountEl && status.errorCount !== void 0) {
      errorCountEl.textContent = String(status.errorCount);
    }
    const fileSizeEl = document.getElementById("log-file-size");
    if (fileSizeEl && status.logFileSize !== void 0) {
      fileSizeEl.textContent = formatFileSize(status.logFileSize);
    }
    const healthSection = document.getElementById("health-indicators");
    const cbEl = document.getElementById("health-circuit-breaker");
    const mpEl = document.getElementById("health-memory-pressure");
    if (healthSection && cbEl && mpEl) {
      const cbState = status.circuitBreakerState || "closed";
      const mpState = status.memoryPressure?.memoryPressureLevel || "normal";
      cbEl.classList.remove("health-error", "health-warning");
      if (!status.connected || cbState === "closed") {
        cbEl.style.display = "none";
        cbEl.textContent = "";
      } else if (cbState === "open") {
        cbEl.style.display = "";
        cbEl.classList.add("health-error");
        cbEl.textContent = "Server: paused (recovering from errors)";
      } else if (cbState === "half-open") {
        cbEl.style.display = "";
        cbEl.classList.add("health-warning");
        cbEl.textContent = "Server: recovering";
      }
      mpEl.classList.remove("health-error", "health-warning");
      if (!status.connected || mpState === "normal") {
        mpEl.style.display = "none";
        mpEl.textContent = "";
      } else if (mpState === "soft") {
        mpEl.style.display = "";
        mpEl.classList.add("health-warning");
        mpEl.textContent = "Memory: elevated (some features limited)";
      } else if (mpState === "hard") {
        mpEl.style.display = "";
        mpEl.classList.add("health-error");
        mpEl.textContent = "Memory: critical (network capture disabled)";
      }
      const cbVisible = status.connected && cbState !== "closed";
      const mpVisible = status.connected && mpState !== "normal";
      healthSection.style.display = cbVisible || mpVisible ? "" : "none";
    }
    const contextWarningEl = document.getElementById("context-warning");
    const contextWarningTextEl = document.getElementById("context-warning-text");
    if (contextWarningEl) {
      if (status.connected && status.contextWarning) {
        contextWarningEl.style.display = "block";
        if (contextWarningTextEl) {
          contextWarningTextEl.textContent = `${status.contextWarning.count} recent entries have context annotations averaging ${status.contextWarning.sizeKB}KB. This may consume significant AI context window space.`;
        }
      } else {
        contextWarningEl.style.display = "none";
        if (contextWarningTextEl) {
          contextWarningTextEl.textContent = "";
        }
      }
    }
  }

  // extension/lib/error-utils.js
  function errorMessage(err, fallback = "Unknown error") {
    if (err instanceof Error && err.message)
      return err.message;
    if (typeof err === "string" && err)
      return err;
    return fallback;
  }

  // extension/popup/recording.js
  var START_LABEL = "Record screen";
  var STOP_LABEL = "Stop recording";
  var HIGHLIGHT_LABEL = "\u25CF \xAB Click here to record";
  var RECENT_RECORDING_START_MS = 8e3;
  var TOP_NOTICE_DURATION_MS = 4e3;
  var AUDIO_LABELS = {
    "": "Video only",
    tab: "Video + tab audio",
    mic: "Video + microphone",
    both: "Video + tab + mic"
  };
  var topNoticeTimer = null;
  function getRecordSection(els) {
    const closest = els.row.closest;
    if (typeof closest !== "function")
      return null;
    return closest.call(els.row, ".section");
  }
  function applyRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
      section.classList.add("record-highlight");
    els.label.textContent = HIGHLIGHT_LABEL;
  }
  function removeRecordHighlight(els) {
    const section = getRecordSection(els);
    if (section)
      section.classList.remove("record-highlight");
    if (els.label.textContent === HIGHLIGHT_LABEL) {
      els.label.textContent = START_LABEL;
    }
  }
  function showRecording(els, state, name, startTime) {
    const wasRecording = state.isRecording;
    removeRecordHighlight(els);
    state.isRecording = true;
    els.row.classList.add("is-recording");
    els.label.textContent = STOP_LABEL;
    els.statusEl.textContent = "";
    if (els.optionsEl)
      els.optionsEl.style.display = "none";
    if (state.timerInterval)
      clearInterval(state.timerInterval);
    state.timerInterval = setInterval(() => {
      const elapsed = Math.round((Date.now() - startTime) / 1e3);
      const mins = Math.floor(elapsed / 60);
      const secs = elapsed % 60;
      els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, "0")}`;
    }, 1e3);
    if (!wasRecording && Date.now() - startTime <= RECENT_RECORDING_START_MS) {
      showTopNotice(els, "Recording started");
    }
  }
  function showIdle(els, state) {
    state.isRecording = false;
    removeRecordHighlight(els);
    els.row.classList.remove("is-recording");
    els.label.textContent = START_LABEL;
    els.statusEl.textContent = "";
    if (els.optionsEl)
      els.optionsEl.style.display = "block";
    if (state.timerInterval) {
      clearInterval(state.timerInterval);
      state.timerInterval = null;
    }
  }
  function describePendingRecording(pending) {
    const parts = [];
    if (pending.name)
      parts.push(`Name: ${pending.name}`);
    if (typeof pending.fps === "number")
      parts.push(`FPS: ${pending.fps}`);
    const audioLabel = AUDIO_LABELS[pending.audio ?? ""] ?? AUDIO_LABELS[""];
    parts.push(`Mode: ${audioLabel}`);
    return parts.join(" \xB7 ");
  }
  function setApprovalPendingState(els, approvalEls, state, pending) {
    const setRowAriaDisabled = (value) => {
      const setAttr = els.row.setAttribute;
      const removeAttr = els.row.removeAttribute;
      if (value !== null) {
        if (typeof setAttr === "function")
          setAttr.call(els.row, "aria-disabled", value);
        return;
      }
      if (typeof removeAttr === "function")
        removeAttr.call(els.row, "aria-disabled");
    };
    const approvalPending = Boolean(pending && !pending.highlight && !state.isRecording);
    if (approvalPending) {
      if (approvalEls.detail && pending)
        approvalEls.detail.textContent = describePendingRecording(pending);
      if (approvalEls.card)
        approvalEls.card.style.display = "block";
      els.row.classList.add("is-disabled");
      setRowAriaDisabled("true");
      if (els.optionsEl)
        els.optionsEl.style.display = "none";
      return;
    }
    if (approvalEls.detail)
      approvalEls.detail.textContent = "";
    if (approvalEls.card)
      approvalEls.card.style.display = "none";
    els.row.classList.remove("is-disabled");
    setRowAriaDisabled(null);
    if (!state.isRecording && els.optionsEl)
      els.optionsEl.style.display = "block";
  }
  function sendRecordingGestureDecision(type) {
    chrome.runtime.sendMessage({ type }, () => {
      void chrome.runtime.lastError;
    });
  }
  function showTopNotice(els, text) {
    const notice = els.topNoticeEl;
    if (!notice)
      return;
    notice.textContent = text;
    notice.style.display = "block";
    if (topNoticeTimer)
      clearTimeout(topNoticeTimer);
    topNoticeTimer = setTimeout(() => {
      notice.style.display = "none";
    }, TOP_NOTICE_DURATION_MS);
  }
  function showSavedLink(saveInfoEl, displayName, filePath) {
    saveInfoEl.textContent = "Saved: ";
    const link = document.createElement("a");
    link.href = "#";
    link.id = "reveal-recording";
    link.textContent = displayName;
    link.style.color = "#58a6ff";
    link.style.textDecoration = "underline";
    link.style.cursor = "pointer";
    saveInfoEl.appendChild(link);
    const linkEl = document.getElementById("reveal-recording");
    if (linkEl) {
      linkEl.addEventListener("click", (e) => {
        e.preventDefault();
        chrome.runtime.sendMessage({ type: "REVEAL_FILE", path: filePath }, (result) => {
          if (result?.error) {
            saveInfoEl.textContent = `Could not open folder: ${result.error}`;
            saveInfoEl.style.color = "#f85149";
            setTimeout(() => {
              saveInfoEl.style.display = "none";
            }, 5e3);
          }
        });
      });
    }
  }
  function showSaveResult(saveInfoEl, resp) {
    if (resp?.status !== "saved" || !resp.name || !saveInfoEl)
      return;
    const displayName = resp.name.replace(/--\d{4}-\d{2}-\d{2}-\d{4}(-\d+)?$/, "");
    if (resp.path) {
      showSavedLink(saveInfoEl, displayName, resp.path);
    } else {
      saveInfoEl.textContent = `Saved: ${displayName}`;
    }
    saveInfoEl.style.display = "block";
    setTimeout(() => {
      saveInfoEl.style.display = "none";
    }, 12e3);
  }
  function showStartError(saveInfoEl, errorText) {
    if (!saveInfoEl)
      return;
    saveInfoEl.textContent = errorText;
    saveInfoEl.style.display = "block";
    saveInfoEl.style.background = "rgba(248, 81, 73, 0.1)";
    saveInfoEl.style.color = "#f85149";
    setTimeout(() => {
      saveInfoEl.style.display = "none";
      saveInfoEl.style.background = "rgba(63, 185, 80, 0.1)";
      saveInfoEl.style.color = "#3fb950";
    }, 5e3);
  }
  function showMicPermissionPrompt(saveInfoEl, audioMode) {
    chrome.tabs.query({ active: true, currentWindow: true }, (activeTabs) => {
      chrome.storage.local.set({
        [StorageKey.PENDING_MIC_RECORDING]: { audioMode, returnTabId: activeTabs[0]?.id }
      });
    });
    saveInfoEl.innerHTML = 'Microphone access needed. <a href="#" id="grant-mic-link" style="color: #58a6ff; text-decoration: underline; cursor: pointer">Grant access</a>';
    saveInfoEl.style.display = "block";
    saveInfoEl.style.background = "rgba(248, 81, 73, 0.1)";
    saveInfoEl.style.color = "#f85149";
    const link = document.getElementById("grant-mic-link");
    if (link) {
      link.addEventListener("click", (e) => {
        e.preventDefault();
        chrome.tabs.create({ url: chrome.runtime.getURL("mic-permission.html") });
      });
    }
  }
  function sendRecordStart(els, state, audioMode) {
    console.log("[Gasoline REC] Popup: sendStart() called, sending screen_recording_start with audio:", audioMode);
    chrome.runtime.sendMessage({ type: "screen_recording_start", audio: audioMode }, (resp) => {
      console.log("[Gasoline REC] Popup: screen_recording_start response:", resp);
      if (chrome.runtime.lastError) {
        console.error("[Gasoline REC] Popup: screen_recording_start lastError:", chrome.runtime.lastError.message);
      }
      if (resp?.status === "recording" && resp.name) {
        showRecording(els, state, resp.name, resp.startTime ?? Date.now());
      } else {
        showIdle(els, state);
        if (resp?.error)
          showStartError(els.saveInfoEl, resp.error);
      }
    });
  }
  function tryMicPermissionThenStart(els, state, audioMode) {
    console.log("[Gasoline REC] Popup: trying getUserMedia from popup...");
    navigator.mediaDevices.getUserMedia({ audio: true }).then((micStream) => {
      console.log("[Gasoline REC] Popup: getUserMedia succeeded from popup");
      micStream.getTracks().forEach((t) => t.stop());
      chrome.storage.local.set({ [StorageKey.MIC_GRANTED]: true });
      sendRecordStart(els, state, audioMode);
    }).catch((err) => {
      console.log("[Gasoline REC] Popup: getUserMedia FAILED:", err.name, errorMessage(err));
      chrome.storage.local.remove(StorageKey.MIC_GRANTED);
      showIdle(els, state);
      if (els.saveInfoEl)
        showMicPermissionPrompt(els.saveInfoEl, audioMode);
    });
  }
  function handleStartClick(els, state) {
    const audioSelect = document.getElementById("record-audio-mode");
    const audioMode = audioSelect?.value ?? "";
    chrome.storage.local.set({ [StorageKey.RECORD_AUDIO_PREF]: audioMode });
    if (els.optionsEl)
      els.optionsEl.style.display = "none";
    if (els.saveInfoEl)
      els.saveInfoEl.style.display = "none";
    els.label.textContent = "Starting...";
    if (audioMode === "mic" || audioMode === "both") {
      console.log("[Gasoline REC] Popup: mic/both mode \u2014 checking gasoline_mic_granted");
      tryMicPermissionThenStart(els, state, audioMode);
    } else {
      sendRecordStart(els, state, audioMode);
    }
  }
  function handleStopClick(els, state) {
    els.row.classList.remove("is-recording");
    els.label.textContent = "Saving...";
    console.log("[Gasoline REC] Popup: sending screen_recording_stop");
    chrome.runtime.sendMessage({ type: "screen_recording_stop" }, (resp) => {
      console.log("[Gasoline REC] Popup: screen_recording_stop response:", resp);
      if (chrome.runtime.lastError) {
        console.error("[Gasoline REC] Popup: screen_recording_stop lastError:", chrome.runtime.lastError.message);
      }
      showIdle(els, state);
      showSaveResult(els.saveInfoEl, resp);
    });
  }
  function setupRecordingUI() {
    const row = document.getElementById("record-row");
    const label = document.getElementById("record-label");
    const statusEl = document.getElementById("recording-status");
    if (!row || !label || !statusEl)
      return;
    const els = {
      row,
      label,
      statusEl,
      optionsEl: document.getElementById("record-options"),
      saveInfoEl: document.getElementById("record-save-info"),
      topNoticeEl: document.getElementById("record-top-notice")
    };
    const approvalEls = {
      card: document.getElementById("record-approval-card"),
      detail: document.getElementById("record-approval-detail"),
      approveBtn: document.getElementById("record-approve-btn"),
      denyBtn: document.getElementById("record-deny-btn")
    };
    const state = { isRecording: false, timerInterval: null };
    let pendingRecordingIntent = null;
    const updatePendingRecording = (pendingValue) => {
      const pending = pendingValue;
      if (pending?.highlight && !state.isRecording) {
        applyRecordHighlight(els);
        pendingRecordingIntent = null;
        setApprovalPendingState(els, approvalEls, state, null);
        chrome.storage.local.remove(StorageKey.PENDING_RECORDING);
        return;
      }
      pendingRecordingIntent = pending && !pending.highlight ? pending : null;
      if (!pendingRecordingIntent && !state.isRecording)
        removeRecordHighlight(els);
      setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
    };
    const clearPendingRecordingIntent = () => {
      pendingRecordingIntent = null;
      setApprovalPendingState(els, approvalEls, state, null);
      chrome.storage.local.remove(StorageKey.PENDING_RECORDING);
    };
    row.style.visibility = "hidden";
    chrome.storage.local.get(StorageKey.RECORDING, (result) => {
      const rec = result[StorageKey.RECORDING];
      console.log("[Gasoline REC] Popup: gasoline_recording from storage:", rec);
      if (rec?.active && rec.name && rec.startTime) {
        console.log("[Gasoline REC] Popup: resuming recording UI for", rec.name);
        showRecording(els, state, rec.name, rec.startTime);
      }
      row.style.visibility = "visible";
      chrome.storage.local.get(StorageKey.PENDING_RECORDING, (pendingResult) => {
        void chrome.runtime.lastError;
        updatePendingRecording(pendingResult[StorageKey.PENDING_RECORDING]);
      });
    });
    chrome.storage.onChanged.addListener((changes, areaName) => {
      if (areaName === "local" && changes[StorageKey.RECORDING]) {
        const rec = changes[StorageKey.RECORDING].newValue;
        console.log("[Gasoline REC] Popup: gasoline_recording changed:", rec);
        if (rec?.active && rec.name && rec.startTime) {
          showRecording(els, state, rec.name, rec.startTime);
        } else {
          showIdle(els, state);
        }
        setApprovalPendingState(els, approvalEls, state, pendingRecordingIntent);
        return;
      }
      if (areaName === "local" && changes[StorageKey.PENDING_RECORDING]) {
        updatePendingRecording(changes[StorageKey.PENDING_RECORDING].newValue);
      }
    });
    approvalEls.approveBtn?.addEventListener("click", (event) => {
      event.preventDefault();
      sendRecordingGestureDecision("RECORDING_GESTURE_GRANTED");
      clearPendingRecordingIntent();
    });
    approvalEls.denyBtn?.addEventListener("click", (event) => {
      event.preventDefault();
      sendRecordingGestureDecision("RECORDING_GESTURE_DENIED");
      clearPendingRecordingIntent();
    });
    chrome.storage.local.get(StorageKey.PENDING_MIC_RECORDING, (result) => {
      const intent = result[StorageKey.PENDING_MIC_RECORDING];
      console.log("[Gasoline REC] Popup: pending_mic_recording intent:", intent);
      if (!intent?.audioMode)
        return;
      console.log("[Gasoline REC] Popup: consuming mic intent, pre-selecting audioMode:", intent.audioMode);
      chrome.storage.local.remove(StorageKey.PENDING_MIC_RECORDING);
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        if (tabs[0]?.id) {
          chrome.tabs.sendMessage(tabs[0].id, {
            type: "GASOLINE_ACTION_TOAST",
            text: "",
            detail: "",
            state: "success",
            duration_ms: 1
          }).catch(() => {
          });
        }
      });
      const audioSelect = document.getElementById("record-audio-mode");
      if (audioSelect)
        audioSelect.value = intent.audioMode;
    });
    chrome.storage.local.get(StorageKey.RECORD_AUDIO_PREF, (result) => {
      const saved = result[StorageKey.RECORD_AUDIO_PREF];
      if (saved) {
        const audioSelect = document.getElementById("record-audio-mode");
        if (audioSelect)
          audioSelect.value = saved;
      }
    });
    row.addEventListener("click", () => {
      console.log("[Gasoline REC] Popup: record row clicked, isRecording:", state.isRecording);
      if (pendingRecordingIntent && !state.isRecording) {
        console.log("[Gasoline REC] Popup: record row click ignored while approval is pending");
        return;
      }
      removeRecordHighlight(els);
      if (state.isRecording) {
        handleStopClick(els, state);
      } else {
        handleStartClick(els, state);
      }
    });
  }

  // extension/popup/draw-mode.js
  function showDrawModeError(label, message) {
    label.textContent = message;
    label.style.color = "#f85149";
    setTimeout(() => {
      label.textContent = "Draw";
      label.style.color = "";
    }, 3e3);
  }
  function setupDrawModeButton() {
    const row = document.getElementById("draw-mode-row");
    const label = document.getElementById("draw-mode-label");
    if (!row || !label)
      return;
    const statusEl = document.getElementById("draw-mode-status");
    if (statusEl) {
      const hasNavigator = typeof navigator !== "undefined";
      const isMac = hasNavigator && (navigator.platform?.toUpperCase().includes("MAC") || navigator.userAgentData?.platform === "macOS");
      statusEl.textContent = isMac ? "\u2325\u21E7D" : "Alt+Shift+D";
    }
    row.addEventListener("click", () => {
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const tab = tabs[0];
        if (!tab?.id) {
          showDrawModeError(label, "No active tab");
          return;
        }
        if (tab.url?.startsWith("chrome://") || tab.url?.startsWith("about:") || tab.url?.startsWith("chrome-extension://")) {
          showDrawModeError(label, "Cannot draw on internal pages");
          return;
        }
        label.textContent = "Starting...";
        chrome.tabs.sendMessage(tab.id, { type: "GASOLINE_DRAW_MODE_START", started_by: "user" }, (resp) => {
          if (chrome.runtime.lastError) {
            showDrawModeError(label, "Content script not loaded \u2014 try refreshing the page");
            return;
          }
          if (resp?.error) {
            showDrawModeError(label, resp.message || "Draw mode failed");
            return;
          }
          window.close();
        });
      });
    });
  }

  // extension/lib/daemon-http.js
  var DEFAULT_CLIENT_NAME = "gasoline-extension";
  function buildDaemonHeaders(options = {}) {
    const { clientName = DEFAULT_CLIENT_NAME, extensionVersion, contentType = "application/json", additionalHeaders = {} } = options;
    const normalizedVersion = typeof extensionVersion === "string" && extensionVersion.trim().length > 0 ? extensionVersion.trim() : "";
    const headers = {
      "X-Gasoline-Client": normalizedVersion ? `${clientName}/${normalizedVersion}` : clientName
    };
    if (contentType !== null) {
      headers["Content-Type"] = contentType;
    }
    if (normalizedVersion) {
      headers["X-Gasoline-Extension-Version"] = normalizedVersion;
    }
    return {
      ...headers,
      ...additionalHeaders
    };
  }
  function buildDaemonJSONRequestInit(payload, options = {}) {
    const { method = "POST", signal, ...headerOptions } = options;
    return {
      method,
      headers: buildDaemonHeaders(headerOptions),
      body: JSON.stringify(payload),
      ...signal ? { signal } : {}
    };
  }
  async function postDaemonJSON(url, payload, options = {}) {
    const { timeoutMs, signal, ...requestOptions } = options;
    const effectiveSignal = signal || (typeof timeoutMs === "number" && timeoutMs > 0 && typeof AbortSignal.timeout === "function" ? AbortSignal.timeout(timeoutMs) : void 0);
    return fetch(url, buildDaemonJSONRequestInit(payload, { ...requestOptions, signal: effectiveSignal }));
  }

  // extension/popup/action-recording.js
  var START_LABEL2 = "Record action workflow";
  var STOP_LABEL2 = "Stop recording";
  function showRecording2(els, state) {
    state.isRecording = true;
    els.row.classList.add("is-recording");
    els.label.textContent = STOP_LABEL2;
    els.statusEl.textContent = "";
    if (state.timerInterval)
      clearInterval(state.timerInterval);
    const start = state.startTime ?? Date.now();
    state.timerInterval = setInterval(() => {
      const elapsed = Math.round((Date.now() - start) / 1e3);
      const mins = Math.floor(elapsed / 60);
      const secs = elapsed % 60;
      els.statusEl.textContent = `${mins}:${secs.toString().padStart(2, "0")}`;
    }, 1e3);
  }
  function showIdle2(els, state) {
    state.isRecording = false;
    state.recordingId = null;
    state.startTime = null;
    els.row.classList.remove("is-recording");
    els.label.textContent = START_LABEL2;
    els.statusEl.textContent = "";
    if (state.timerInterval) {
      clearInterval(state.timerInterval);
      state.timerInterval = null;
    }
  }
  function showError(els, message) {
    els.statusEl.textContent = message;
    els.statusEl.style.color = "#f85149";
    setTimeout(() => {
      els.statusEl.textContent = "";
      els.statusEl.style.color = "";
    }, 5e3);
  }
  function getServerUrl() {
    return new Promise((resolve) => {
      chrome.storage.local.get(StorageKey.SERVER_URL, (result) => {
        void chrome.runtime.lastError;
        resolve(result[StorageKey.SERVER_URL] || DEFAULT_SERVER_URL);
      });
    });
  }
  function getConfigureError(data) {
    const message = data.error?.message;
    return typeof message === "string" && message.length > 0 ? message : null;
  }
  function extractRecordingID(data) {
    const text = data.result?.content?.[0]?.text ?? "";
    const idMatch = text.match(/"recording_id"\s*:\s*"([^"]+)"/);
    return idMatch?.[1] ?? null;
  }
  async function callConfigureFromPopup(argumentsPayload) {
    const serverUrl = await getServerUrl();
    const resp = await postDaemonJSON(`${serverUrl}/mcp`, {
      jsonrpc: "2.0",
      id: Date.now(),
      method: "tools/call",
      params: {
        name: "configure",
        arguments: argumentsPayload
      }
    });
    if (!resp.ok) {
      throw new Error(`Server error: HTTP ${resp.status}`);
    }
    return await resp.json();
  }
  async function startActionRecording(els, state) {
    els.label.textContent = "Starting...";
    try {
      const data = await callConfigureFromPopup({
        what: "event_recording_start",
        name: `workflow-${Date.now()}`
      });
      const configureError = getConfigureError(data);
      if (configureError) {
        showIdle2(els, state);
        showError(els, configureError);
        return;
      }
      state.recordingId = extractRecordingID(data);
      state.startTime = Date.now();
      chrome.storage.local.set({
        gasoline_action_recording: {
          active: true,
          recordingId: state.recordingId,
          startTime: state.startTime
        }
      }, () => {
        void chrome.runtime.lastError;
      });
      showRecording2(els, state);
    } catch (err) {
      showIdle2(els, state);
      showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }
  async function stopActionRecording(els, state) {
    els.label.textContent = "Stopping...";
    try {
      const data = await callConfigureFromPopup({
        what: "event_recording_stop",
        recording_id: state.recordingId ?? ""
      });
      const configureError = getConfigureError(data);
      if (configureError) {
        showError(els, configureError);
      }
      chrome.storage.local.remove("gasoline_action_recording", () => {
        void chrome.runtime.lastError;
      });
      showIdle2(els, state);
    } catch (err) {
      showIdle2(els, state);
      showError(els, `Connection failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }
  function setupActionRecordingUI() {
    const row = document.getElementById("action-record-row");
    const label = document.getElementById("action-record-label");
    const statusEl = document.getElementById("action-recording-status");
    if (!row || !label || !statusEl)
      return;
    const els = { row, label, statusEl };
    const state = {
      isRecording: false,
      recordingId: null,
      timerInterval: null,
      startTime: null
    };
    chrome.storage.local.get("gasoline_action_recording", (result) => {
      void chrome.runtime.lastError;
      const saved = result["gasoline_action_recording"];
      if (saved?.active && saved.recordingId) {
        state.recordingId = saved.recordingId;
        state.startTime = saved.startTime ?? Date.now();
        showRecording2(els, state);
      }
    });
    row.addEventListener("click", () => {
      if (state.isRecording) {
        void stopActionRecording(els, state);
      } else {
        void startActionRecording(els, state);
      }
    });
  }

  // extension/popup/feature-toggles.js
  var FEATURE_TOGGLES = [
    {
      id: "toggle-websocket",
      storageKey: StorageKey.WEBSOCKET_CAPTURE_ENABLED,
      messageType: SettingName.WEBSOCKET_CAPTURE,
      default: true
    },
    {
      id: "toggle-network-waterfall",
      storageKey: StorageKey.NETWORK_WATERFALL_ENABLED,
      messageType: SettingName.NETWORK_WATERFALL,
      default: true
    },
    {
      id: "toggle-performance-marks",
      storageKey: StorageKey.PERFORMANCE_MARKS_ENABLED,
      messageType: SettingName.PERFORMANCE_MARKS,
      default: true
    },
    {
      id: "toggle-action-replay",
      storageKey: StorageKey.ACTION_REPLAY_ENABLED,
      messageType: SettingName.ACTION_REPLAY,
      default: true
    },
    {
      id: "toggle-screenshot",
      storageKey: StorageKey.SCREENSHOT_ON_ERROR,
      messageType: "setScreenshotOnError",
      default: true
    },
    {
      id: "toggle-source-maps",
      storageKey: StorageKey.SOURCE_MAP_ENABLED,
      messageType: "setSourceMapEnabled",
      default: true
    },
    {
      id: "toggle-network-body-capture",
      storageKey: StorageKey.NETWORK_BODY_CAPTURE_ENABLED,
      messageType: SettingName.NETWORK_BODY_CAPTURE,
      default: true
    },
    {
      id: "toggle-action-toasts",
      storageKey: StorageKey.ACTION_TOASTS_ENABLED,
      messageType: SettingName.ACTION_TOASTS,
      default: true
    },
    {
      id: "toggle-subtitles",
      storageKey: StorageKey.SUBTITLES_ENABLED,
      messageType: SettingName.SUBTITLES,
      default: true
    }
  ];
  function handleFeatureToggle(storageKey, messageType, enabled) {
    chrome.runtime.sendMessage({ type: messageType, enabled }, (response) => {
      if (chrome.runtime.lastError) {
        console.error(`[Gasoline] Message error for ${messageType}:`, chrome.runtime.lastError.message);
      } else if (response?.success) {
        console.log(`[Gasoline] ${messageType} acknowledged by background`);
      } else {
        console.warn(`[Gasoline] ${messageType} - no response from background`);
      }
    });
  }
  async function initFeatureToggles() {
    const storageKeys = FEATURE_TOGGLES.map((t) => t.storageKey);
    return new Promise((resolve) => {
      chrome.storage.local.get(storageKeys, (result) => {
        for (const toggle of FEATURE_TOGGLES) {
          const checkbox = document.getElementById(toggle.id);
          if (checkbox) {
            const savedValue = result[toggle.storageKey];
            checkbox.checked = savedValue !== void 0 ? savedValue : toggle.default;
            checkbox.addEventListener("change", () => {
              handleFeatureToggle(toggle.storageKey, toggle.messageType, checkbox.checked);
            });
          }
        }
        resolve();
      });
    });
  }

  // extension/popup/tab-tracking.js
  var trackingStorageSyncInstalled = false;
  function showInternalPageState(btn) {
    const trackingBar = document.getElementById("tracking-bar");
    if (trackingBar)
      trackingBar.style.display = "none";
    btn.disabled = true;
    btn.textContent = "Cannot Track Internal Pages";
    btn.title = "Chrome blocks extensions on internal pages like chrome:// and about:";
    Object.assign(btn.style, { opacity: "0.5", background: "#252525", color: "#888", borderColor: "#333" });
  }
  function showTrackingState(btn, trackedTabUrl, trackedTabId) {
    const heroEl = document.getElementById("track-hero");
    if (heroEl)
      heroEl.style.display = "none";
    const noTrackEl = document.getElementById("no-tracking-warning");
    if (noTrackEl)
      noTrackEl.style.display = "none";
    const trackingBar = document.getElementById("tracking-bar");
    const trackingBarUrl = document.getElementById("tracking-bar-url");
    const trackingBarStop = document.getElementById("tracking-bar-stop");
    if (trackingBar)
      trackingBar.style.display = "flex";
    if (trackingBarUrl && trackedTabUrl) {
      trackingBarUrl.textContent = trackedTabUrl;
      trackingBarUrl.onclick = () => {
        void handleUrlClick(trackedTabId);
      };
    }
    if (trackingBarStop) {
      trackingBarStop.onclick = (e) => {
        e.stopPropagation();
        handleStopTracking();
      };
    }
  }
  function showIdleState(btn) {
    const heroEl = document.getElementById("track-hero");
    if (heroEl)
      heroEl.style.display = "";
    btn.textContent = "Track This Tab";
    Object.assign(btn.style, {
      background: "#1a3a5c",
      color: "#58a6ff",
      borderColor: "#58a6ff",
      fontSize: "16px",
      fontWeight: "600",
      padding: "14px 16px",
      borderWidth: "2px"
    });
    const heroDesc = document.getElementById("track-hero-desc");
    if (heroDesc)
      heroDesc.style.display = "";
    const trackingBar = document.getElementById("tracking-bar");
    if (trackingBar)
      trackingBar.style.display = "none";
    const noTrackEl = document.getElementById("no-tracking-warning");
    if (noTrackEl)
      noTrackEl.style.display = "block";
  }
  function syncTrackButtonState(btn) {
    chrome.storage.local.get([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL], (result) => {
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
        const currentUrl = tabs?.[0]?.url;
        if (result.trackedTabId) {
          showTrackingState(btn, result.trackedTabUrl, result.trackedTabId);
        } else if (isInternalUrl(currentUrl)) {
          showInternalPageState(btn);
        } else {
          showIdleState(btn);
        }
      });
    });
  }
  function installTrackingStorageSync(btn) {
    if (trackingStorageSyncInstalled)
      return;
    trackingStorageSyncInstalled = true;
    chrome.storage.onChanged.addListener((changes, areaName) => {
      if (areaName !== "local")
        return;
      if (!changes[StorageKey.TRACKED_TAB_ID] && !changes[StorageKey.TRACKED_TAB_URL])
        return;
      syncTrackButtonState(btn);
    });
  }
  function handleStopTracking() {
    chrome.storage.local.get([StorageKey.TRACKED_TAB_ID], (result) => {
      const prevTabId = result.trackedTabId;
      if (!prevTabId)
        return;
      chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL], () => {
        const btn = document.getElementById("track-page-btn");
        if (btn)
          showIdleState(btn);
        chrome.runtime.sendMessage({ type: "screen_recording_stop" }, () => {
          if (chrome.runtime.lastError) {
          }
        });
        chrome.tabs.sendMessage(prevTabId, {
          type: "trackingStateChanged",
          state: { isTracked: false, aiPilotEnabled: false }
        }).catch(() => {
        });
        console.log("[Gasoline] Stopped tracking via bar stop button");
      });
    });
  }
  function initTrackPageButton() {
    const btn = document.getElementById("track-page-btn");
    if (!btn)
      return;
    syncTrackButtonState(btn);
    installTrackingStorageSync(btn);
    btn.addEventListener("click", handleTrackPageClick);
  }
  async function handleUrlClick(tabId) {
    if (!tabId)
      return;
    try {
      await chrome.tabs.update(tabId, { active: true });
      const tab = await chrome.tabs.get(tabId);
      if (tab.windowId) {
        await chrome.windows.update(tab.windowId, { focused: true });
      }
      console.log("[Gasoline] Switched to tracked tab:", tabId);
    } catch (err) {
      console.error("[Gasoline] Failed to switch to tracked tab:", err);
      chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL]);
    }
  }
  async function handleTrackPageClick() {
    const btn = document.getElementById("track-page-btn");
    chrome.storage.local.get([StorageKey.TRACKED_TAB_ID], async (result) => {
      if (result.trackedTabId) {
        handleStopTracking();
      } else {
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
          if (tabs[0]) {
            const tab = tabs[0];
            if (isInternalUrl(tab.url)) {
              if (btn) {
                btn.disabled = true;
                btn.textContent = "Cannot Track Internal Pages";
                btn.style.opacity = "0.5";
              }
              return;
            }
            chrome.storage.local.set({ trackedTabId: tab.id, trackedTabUrl: tab.url, trackedTabTitle: tab.title || "" }, () => {
              showTrackingState(btn, tab.url, tab.id);
              console.log("[Gasoline] Now tracking tab:", tab.id, tab.url);
              if (tab.id) {
                const tabId = tab.id;
                chrome.tabs.sendMessage(tabId, { type: "GASOLINE_PING" }, (response) => {
                  if (chrome.runtime.lastError || !response?.status) {
                    console.log("[Gasoline] Content script not found, reloading tab", tabId);
                    chrome.tabs.reload(tabId);
                  } else {
                    console.log("[Gasoline] Content script already loaded, skipping reload");
                    chrome.tabs.sendMessage(tabId, {
                      type: "trackingStateChanged",
                      state: { isTracked: true, aiPilotEnabled: false }
                    });
                  }
                });
              }
            });
          }
        });
      }
    });
  }

  // extension/popup/ai-web-pilot.js
  async function initAiWebPilotToggle() {
    const toggle = document.getElementById("aiWebPilotEnabled");
    if (!toggle)
      return;
    return new Promise((resolve) => {
      chrome.storage.local.get([StorageKey.AI_WEB_PILOT_ENABLED], (result) => {
        toggle.checked = result.aiWebPilotEnabled !== false;
        toggle.addEventListener("change", () => {
          handleAiWebPilotToggle(toggle.checked);
        });
        resolve();
      });
    });
  }
  async function handleAiWebPilotToggle(enabled) {
    chrome.runtime.sendMessage({ type: "setAiWebPilotEnabled", enabled }, (response) => {
      if (!response || !response.success) {
        console.error("[Gasoline] Failed to set AI Web Pilot toggle in background");
        const toggle = document.getElementById("aiWebPilotEnabled");
        if (toggle) {
          toggle.checked = !enabled;
        }
      }
    });
  }

  // extension/popup/settings.js
  function handleWebSocketModeChange(mode) {
    chrome.storage.local.set({ webSocketCaptureMode: mode });
    chrome.runtime.sendMessage({ type: SettingName.WEBSOCKET_CAPTURE_MODE, mode });
  }
  async function initWebSocketModeSelector() {
    const modeSelect = document.getElementById("ws-mode");
    if (!modeSelect)
      return;
    return new Promise((resolve) => {
      chrome.storage.local.get(["webSocketCaptureMode"], (result) => {
        modeSelect.value = result.webSocketCaptureMode || "medium";
        resolve();
      });
    });
  }
  var clearConfirmPending = false;
  var clearConfirmTimer = null;
  function resetClearConfirm() {
    clearConfirmPending = false;
    if (clearConfirmTimer) {
      clearTimeout(clearConfirmTimer);
      clearConfirmTimer = null;
    }
  }
  async function handleClearLogs() {
    const clearBtn = document.getElementById("clear-btn");
    const entriesEl = document.getElementById("entries-count");
    if (clearBtn && !clearConfirmPending) {
      clearConfirmPending = true;
      clearBtn.textContent = "Confirm Clear?";
      clearConfirmTimer = setTimeout(() => {
        clearConfirmPending = false;
        if (clearBtn)
          clearBtn.textContent = "Clear Logs";
      }, 3e3);
      return Promise.resolve(null);
    }
    clearConfirmPending = false;
    if (clearConfirmTimer) {
      clearTimeout(clearConfirmTimer);
      clearConfirmTimer = null;
    }
    if (clearBtn) {
      clearBtn.disabled = true;
      clearBtn.textContent = "Clearing...";
    }
    return new Promise((resolve) => {
      chrome.runtime.sendMessage({ type: "clearLogs" }, (response) => {
        if (clearBtn) {
          clearBtn.disabled = false;
          clearBtn.textContent = "Clear Logs";
        }
        if (response?.success) {
          if (entriesEl) {
            entriesEl.textContent = "0 / 1000";
          }
        } else if (response?.error) {
          const errorEl = document.getElementById("error-message");
          if (errorEl) {
            errorEl.textContent = response.error;
          }
        }
        resolve(response || null);
      });
    });
  }

  // extension/popup.js
  try {
    chrome.storage.local.get("theme", (r) => {
      void chrome.runtime.lastError;
      if (r?.["theme"] === "light")
        document.body.classList.add("light-theme");
    });
  } catch {
  }
  var DEFAULT_MAX_ENTRIES2 = 1e3;
  var RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE = {
    type: RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
  };
  function bindToggleVisibility(toggle, target, isVisible) {
    target.style.display = isVisible() ? "block" : "none";
    toggle.addEventListener("change", () => {
      target.style.display = isVisible() ? "block" : "none";
    });
  }
  function setupWebSocketUI() {
    const wsToggle = document.getElementById("toggle-websocket");
    const wsModeContainer = document.getElementById("ws-mode-container");
    if (wsToggle && wsModeContainer) {
      bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked);
    }
    const wsModeSelect = document.getElementById("ws-mode");
    if (wsModeSelect) {
      wsModeSelect.addEventListener("change", (e) => {
        handleWebSocketModeChange(e.target.value);
      });
    }
    const wsMessagesWarning = document.getElementById("ws-messages-warning");
    if (wsModeSelect && wsMessagesWarning) {
      bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === "all");
    }
  }
  function setupToggleWarnings() {
    const toggleWarnings = [
      { toggleId: "toggle-screenshot", warningId: "screenshot-warning" },
      { toggleId: "toggle-network-waterfall", warningId: "waterfall-warning" },
      { toggleId: "toggle-performance-marks", warningId: "perfmarks-warning" }
    ];
    for (const { toggleId, warningId } of toggleWarnings) {
      const toggle = document.getElementById(toggleId);
      const warning = document.getElementById(warningId);
      if (toggle && warning) {
        warning.style.display = toggle.checked ? "block" : "none";
        toggle.addEventListener("change", () => {
          warning.style.display = toggle.checked ? "block" : "none";
        });
      }
    }
  }
  function requestTrackedHoverLauncherReshow() {
    if (!chrome.tabs?.query || !chrome.tabs?.sendMessage)
      return;
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tabId = tabs[0]?.id;
      if (!tabId)
        return;
      chrome.tabs.sendMessage(tabId, RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE, () => {
        void chrome.runtime.lastError;
      });
    });
  }
  function cacheStatus(status) {
    try {
      chrome.storage.session.set({ [StorageKey.POPUP_LAST_STATUS]: status }, () => {
        void chrome.runtime.lastError;
      });
    } catch {
    }
  }
  function initPopup() {
    requestTrackedHoverLauncherReshow();
    try {
      chrome.storage.session.get([StorageKey.POPUP_LAST_STATUS], (result) => {
        void chrome.runtime.lastError;
        const cached = result?.[StorageKey.POPUP_LAST_STATUS];
        if (cached)
          updateConnectionStatus(cached);
      });
    } catch {
    }
    try {
      chrome.runtime.sendMessage({ type: "getStatus" }, (status) => {
        if (chrome.runtime.lastError) {
          updateConnectionStatus({
            connected: false,
            entries: 0,
            maxEntries: DEFAULT_MAX_ENTRIES2,
            errorCount: 0,
            logFile: "",
            error: "Extension restarting \u2014 please wait a moment and reopen popup"
          });
          return;
        }
        if (status) {
          updateConnectionStatus(status);
          cacheStatus(status);
        }
      });
    } catch {
      updateConnectionStatus({
        connected: false,
        entries: 0,
        maxEntries: DEFAULT_MAX_ENTRIES2,
        errorCount: 0,
        logFile: "",
        error: "Extension error \u2014 try reloading the extension"
      });
    }
    setupRecordingUI();
    setupActionRecordingUI();
    initFeatureToggles();
    initWebSocketModeSelector();
    initAiWebPilotToggle();
    initTrackPageButton();
    setupWebSocketUI();
    setupToggleWarnings();
    setupDrawModeButton();
    const clearBtn = document.getElementById("clear-btn");
    if (clearBtn)
      clearBtn.addEventListener("click", handleClearLogs);
    chrome.runtime.onMessage.addListener((message) => {
      if (message.type === "statusUpdate" && message.status) {
        updateConnectionStatus(message.status);
        cacheStatus(message.status);
      } else if (message.type === "pilotStatusChanged") {
        const toggle = document.getElementById("aiWebPilotEnabled");
        if (toggle) {
          toggle.checked = message.enabled === true;
          console.log("[Gasoline] Pilot status confirmed:", message.enabled);
        }
      }
    });
    chrome.storage.onChanged.addListener((changes, areaName) => {
      if (areaName === "local" && changes[StorageKey.TRACKED_TAB_URL]) {
        const urlEl = document.getElementById("tracking-bar-url");
        if (urlEl && changes[StorageKey.TRACKED_TAB_URL].newValue) {
          urlEl.textContent = changes[StorageKey.TRACKED_TAB_URL].newValue;
          console.log("[Gasoline] Tracked tab URL updated in popup:", changes[StorageKey.TRACKED_TAB_URL].newValue);
        }
      }
    });
  }
  if (typeof document !== "undefined" && typeof globalThis.process === "undefined") {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", initPopup);
    } else {
      initPopup();
    }
  }
})();
//# sourceMappingURL=popup.bundled.js.map
