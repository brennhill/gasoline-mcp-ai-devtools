/**
 * @fileoverview Reproduction script generation and enhanced action recording.
 * Captures user interactions with multi-strategy selectors (testId, role, aria,
 * text, CSS path) and generates Playwright test scripts for reproducing issues.
 */
type EnhancedActionType = 'click' | 'input' | 'keypress' | 'navigate' | 'select' | 'scroll'
interface RoleSelector {
  role: string
  name?: string
}
interface SelectorStrategies {
  testId?: string
  ariaLabel?: string
  role?: RoleSelector
  id?: string
  text?: string
  cssPath: string
}
interface EnhancedActionRecord {
  type: EnhancedActionType
  timestamp: number
  url: string
  selectors?: SelectorStrategies
  inputType?: string
  value?: string
  key?: string
  fromUrl?: string
  toUrl?: string
  selectedValue?: string
  selectedText?: string
  scrollY?: number
}
interface ScriptOptions {
  errorMessage?: string
  baseUrl?: string
  lastNActions?: number
}
/**
 * Get the implicit ARIA role for an element
 */
export declare function getImplicitRole(element: Element | null): string | null
/**
 * Detect if a CSS class name is dynamically generated (CSS-in-JS)
 */
export declare function isDynamicClass(className: string | null): boolean
/**
 * Compute a CSS path for an element
 */
export declare function computeCssPath(element: Element | null): string
/**
 * Compute multi-strategy selectors for an element
 */
export declare function computeSelectors(element: Element | null): SelectorStrategies
interface RecordActionOptions {
  value?: string
  key?: string
  fromUrl?: string
  toUrl?: string
  selectedValue?: string
  selectedText?: string
  scrollY?: number
}
/**
 * Record an enhanced action with multi-strategy selectors
 */
export declare function recordEnhancedAction(
  type: EnhancedActionType,
  element: Element | null,
  opts?: RecordActionOptions,
): EnhancedActionRecord
/**
 * Get the enhanced action buffer
 */
export declare function getEnhancedActionBuffer(): EnhancedActionRecord[]
/**
 * Clear the enhanced action buffer
 */
export declare function clearEnhancedActionBuffer(): void
/**
 * Generate a Playwright test script from captured actions
 */
export declare function generatePlaywrightScript(actions: EnhancedActionRecord[], opts?: ScriptOptions): string
export {}
//# sourceMappingURL=reproduction.d.ts.map
