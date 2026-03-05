/**
 * Purpose: Safe error-message extraction from unknown caught values.
 */

/**
 * Extract a message string from an unknown caught value.
 * Returns the Error.message if available, otherwise the fallback.
 */
export function errorMessage(err: unknown, fallback = 'Unknown error'): string {
  if (err instanceof Error && err.message) return err.message
  if (typeof err === 'string' && err) return err
  return fallback
}
