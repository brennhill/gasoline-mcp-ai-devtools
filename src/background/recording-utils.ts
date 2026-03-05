/**
 * Purpose: Shared recording helpers used by context menus, keyboard shortcuts, and runtime listeners.
 * Why: Keep recording slug generation consistent across all recording entry points.
 * Docs: docs/features/feature/flow-recording/index.md
 */

/**
 * Build a filesystem-safe recording slug from the current tab URL.
 */
export function buildScreenRecordingSlug(url: string | undefined): string {
  try {
    const hostname = new URL(url ?? '').hostname.replace(/^www\./, '')
    return (
      hostname
        .replace(/[^a-z0-9]/gi, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '') || 'recording'
    )
  } catch {
    return 'recording'
  }
}
