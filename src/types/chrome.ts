/**
 * @fileoverview Chrome API Wrapper Types
 * Chrome-specific types for message passing and storage
 */

/**
 * Chrome message sender info
 */
export interface ChromeMessageSender {
  readonly tab?: {
    readonly id?: number;
    readonly url?: string;
    readonly windowId?: number;
  };
  readonly frameId?: number;
  readonly url?: string;
}

/**
 * Chrome tab info
 */
export interface ChromeTabInfo {
  readonly id?: number;
  readonly url?: string;
  readonly title?: string;
  readonly windowId?: number;
  readonly status?: string;
  readonly active?: boolean;
  readonly favIconUrl?: string;
  readonly width?: number;
  readonly height?: number;
}

/**
 * Chrome storage change info
 */
export interface StorageChange<T = unknown> {
  readonly oldValue?: T;
  readonly newValue?: T;
}

/**
 * Storage area name
 */
export type StorageAreaName = 'sync' | 'local' | 'session';
