/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */
export declare const DEFAULT_SERVER_URL = "http://localhost:7890";
export declare const MAX_STRING_LENGTH = 10240;
export declare const MAX_RESPONSE_LENGTH = 5120;
export declare const MAX_DEPTH = 10;
export declare const MAX_CONTEXT_SIZE = 50;
export declare const MAX_CONTEXT_VALUE_SIZE = 4096;
export declare const SENSITIVE_HEADERS: readonly string[];
export declare const MAX_ACTION_BUFFER_SIZE = 20;
export declare const SCROLL_THROTTLE_MS = 250;
export declare const SENSITIVE_INPUT_TYPES: readonly string[];
export declare const MAX_WATERFALL_ENTRIES = 50;
export declare const WATERFALL_TIME_WINDOW_MS = 30000;
export declare const MAX_PERFORMANCE_ENTRIES = 50;
export declare const PERFORMANCE_TIME_WINDOW_MS = 60000;
export declare const WS_MAX_BODY_SIZE = 4096;
export declare const WS_PREVIEW_LIMIT = 200;
export declare const REQUEST_BODY_MAX = 8192;
export declare const RESPONSE_BODY_MAX = 16384;
export declare const BODY_READ_TIMEOUT_MS = 5;
export declare const SENSITIVE_HEADER_PATTERNS: RegExp;
export declare const BINARY_CONTENT_TYPES: RegExp;
export declare const DOM_QUERY_MAX_ELEMENTS = 50;
export declare const DOM_QUERY_MAX_TEXT = 500;
export declare const DOM_QUERY_MAX_DEPTH = 5;
export declare const DOM_QUERY_MAX_HTML = 200;
export declare const A11Y_MAX_NODES_PER_VIOLATION = 10;
export declare const ASYNC_COMMAND_TIMEOUT_MS: number;
export declare const A11Y_AUDIT_TIMEOUT_MS: number;
export declare const MEMORY_SOFT_LIMIT_MB = 20;
export declare const MEMORY_HARD_LIMIT_MB = 50;
export declare const AI_CONTEXT_SNIPPET_LINES = 5;
export declare const AI_CONTEXT_MAX_LINE_LENGTH = 200;
export declare const AI_CONTEXT_MAX_SNIPPETS_SIZE = 10240;
export declare const AI_CONTEXT_MAX_ANCESTRY_DEPTH = 10;
export declare const AI_CONTEXT_MAX_PROP_KEYS = 20;
export declare const AI_CONTEXT_MAX_STATE_KEYS = 10;
export declare const AI_CONTEXT_MAX_RELEVANT_SLICE = 10;
export declare const AI_CONTEXT_MAX_VALUE_LENGTH = 200;
export declare const AI_CONTEXT_SOURCE_MAP_CACHE_SIZE = 20;
export declare const AI_CONTEXT_PIPELINE_TIMEOUT_MS: number;
export declare const ENHANCED_ACTION_BUFFER_SIZE = 50;
export declare const CSS_PATH_MAX_DEPTH = 5;
export declare const SELECTOR_TEXT_MAX_LENGTH = 50;
export declare const SCRIPT_MAX_SIZE = 51200;
export declare const CLICKABLE_TAGS: ReadonlySet<string>;
export declare const ACTIONABLE_KEYS: ReadonlySet<string>;
export declare const MAX_LONG_TASKS = 50;
export declare const MAX_SLOWEST_REQUESTS = 3;
export declare const MAX_URL_LENGTH = 80;
/**
 * Setting names used across background, content, and inject contexts.
 * Single source of truth — all layers import from here.
 *
 * Note: These are the RUNTIME string values sent as message `type` fields.
 * The TYPE-LEVEL literals in runtime-messages.ts (SetBooleanSettingMessage etc.)
 * are deliberately kept as literal strings for TypeScript discriminated union narrowing.
 */
export declare const SettingName: {
    readonly NETWORK_WATERFALL: "setNetworkWaterfallEnabled";
    readonly PERFORMANCE_MARKS: "setPerformanceMarksEnabled";
    readonly ACTION_REPLAY: "setActionReplayEnabled";
    readonly WEBSOCKET_CAPTURE: "setWebSocketCaptureEnabled";
    readonly WEBSOCKET_CAPTURE_MODE: "setWebSocketCaptureMode";
    readonly PERFORMANCE_SNAPSHOT: "setPerformanceSnapshotEnabled";
    readonly DEFERRAL: "setDeferralEnabled";
    readonly NETWORK_BODY_CAPTURE: "setNetworkBodyCaptureEnabled";
    readonly ACTION_TOASTS: "setActionToastsEnabled";
    readonly SUBTITLES: "setSubtitlesEnabled";
    readonly SERVER_URL: "setServerUrl";
};
export type SettingNameValue = typeof SettingName[keyof typeof SettingName];
/** All valid setting names as a Set (for runtime validation) */
export declare const VALID_SETTING_NAMES: ReadonlySet<string>;
/**
 * Settings forwarded from background -> content -> inject (MAIN world).
 * These are the settings that the inject script knows how to handle.
 * Content-only settings (ACTION_TOASTS, SUBTITLES) are NOT in this set —
 * they are handled directly by the content script runtime-message-listener.
 */
export declare const INJECT_FORWARDED_SETTINGS: ReadonlySet<string>;
/**
 * Chrome storage key strings used in chrome.storage.local.get/set/remove calls.
 * Single source of truth — all layers import from here.
 */
export declare const StorageKey: {
    readonly TRACKED_TAB_ID: "trackedTabId";
    readonly TRACKED_TAB_URL: "trackedTabUrl";
    readonly TRACKED_TAB_TITLE: "trackedTabTitle";
    readonly AI_WEB_PILOT_ENABLED: "aiWebPilotEnabled";
    readonly DEBUG_MODE: "debugMode";
    readonly SERVER_URL: "serverUrl";
    readonly SCREENSHOT_ON_ERROR: "screenshotOnError";
    readonly SOURCE_MAP_ENABLED: "sourceMapEnabled";
    readonly LOG_LEVEL: "logLevel";
    readonly THEME: "theme";
    readonly DEFERRAL_ENABLED: "deferralEnabled";
    readonly WEBSOCKET_CAPTURE_ENABLED: "webSocketCaptureEnabled";
    readonly WEBSOCKET_CAPTURE_MODE: "webSocketCaptureMode";
    readonly NETWORK_WATERFALL_ENABLED: "networkWaterfallEnabled";
    readonly PERFORMANCE_MARKS_ENABLED: "performanceMarksEnabled";
    readonly ACTION_REPLAY_ENABLED: "actionReplayEnabled";
    readonly NETWORK_BODY_CAPTURE_ENABLED: "networkBodyCaptureEnabled";
    readonly ACTION_TOASTS_ENABLED: "actionToastsEnabled";
    readonly SUBTITLES_ENABLED: "subtitlesEnabled";
    readonly RECORDING: "gasoline_recording";
    readonly PENDING_RECORDING: "gasoline_pending_recording";
    readonly PENDING_MIC_RECORDING: "gasoline_pending_mic_recording";
    readonly MIC_GRANTED: "gasoline_mic_granted";
    readonly RECORD_AUDIO_PREF: "gasoline_record_audio_pref";
};
//# sourceMappingURL=constants.d.ts.map