/**
 * @fileoverview User Action Types
 * User interactions for action replay and test generation
 */

/**
 * Action types for user action replay
 */
export type ActionType = 'click' | 'input' | 'scroll' | 'keydown' | 'change' | 'navigate'

/**
 * Basic action entry
 */
export interface ActionEntry {
  readonly type: ActionType
  readonly target: string
  readonly timestamp: string
  readonly value?: string
}

/**
 * Multi-strategy selector result
 */
export interface SelectorStrategies {
  readonly testId?: string
  readonly aria?: string
  readonly role?: string
  readonly cssPath?: string
  readonly xpath?: string
  readonly text?: string
}

/**
 * Enhanced action entry with multiple selector strategies
 */
export interface EnhancedAction {
  readonly type: ActionType
  readonly ts: string
  readonly url: string
  readonly selectors: SelectorStrategies
  readonly value?: string
  readonly key?: string
  readonly modifiers?: {
    readonly ctrl?: boolean
    readonly alt?: boolean
    readonly shift?: boolean
    readonly meta?: boolean
  }
  readonly scrollPosition?: {
    readonly x: number
    readonly y: number
  }
}
