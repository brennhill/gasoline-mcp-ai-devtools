/**
 * Purpose: CSP-safe execution subsystem barrel — structured command parsing and execution for Content Security Policy restricted pages.
 * Why: Groups all CSP-related functionality into a cohesive module.
 */

// Types
export type { StructuredStep, StructuredValue, StructuredCommand, ParseResult } from './types.js'

// Parser
export { parseExpression } from './parser.js'

// Executor
export { cspSafeExecutor } from './executor.js'
