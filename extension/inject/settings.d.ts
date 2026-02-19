import type { BrowserStateSnapshot, StateAction } from '../types/index';
/**
 * Valid setting names from content script
 */
export declare const VALID_SETTINGS: Set<string>;
export declare const VALID_STATE_ACTIONS: Set<StateAction>;
/**
 * Setting message from content script
 */
export interface SettingMessageData {
    type: 'GASOLINE_SETTING';
    setting: string;
    enabled?: boolean;
    mode?: string;
    url?: string;
}
/**
 * State command message from content script
 */
export interface StateCommandMessageData {
    type: 'GASOLINE_STATE_COMMAND';
    messageId: string;
    action: StateAction;
    state?: BrowserStateSnapshot;
    include_url?: boolean;
}
export declare function isValidSettingPayload(data: SettingMessageData): boolean;
export declare function handleSetting(data: SettingMessageData): void;
export declare function handleStateCommand(data: StateCommandMessageData, captureStateFn: () => BrowserStateSnapshot, restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown): void;
//# sourceMappingURL=settings.d.ts.map