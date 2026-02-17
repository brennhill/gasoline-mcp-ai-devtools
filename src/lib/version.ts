/**
 * Purpose: Provides shared runtime utilities used by extension and server workflows.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Version - Utilities for semver comparison and version checking
 */

/**
 * Parse a semantic version string into components
 * @param version - Version string like "5.2.5"
 * @returns Object with major, minor, patch components, or null if invalid
 */
export function parseVersion(version: string): {
  major: number
  minor: number
  patch: number
} | null {
  const match = version.match(/^(\d+)\.(\d+)\.(\d+)/)
  if (!match || !match[1] || !match[2] || !match[3]) {
    return null
  }
  return {
    major: parseInt(match[1], 10),
    minor: parseInt(match[2], 10),
    patch: parseInt(match[3], 10)
  }
}

/**
 * Compare two semantic versions
 * @param versionA - First version string
 * @param versionB - Second version string
 * @returns -1 if A < B, 0 if A == B, 1 if A > B, null if either is invalid
 */
export function compareVersions(versionA: string, versionB: string): -1 | 0 | 1 | null {
  const a = parseVersion(versionA)
  const b = parseVersion(versionB)

  if (!a || !b) {
    return null
  }

  if (a.major !== b.major) {
    return a.major < b.major ? -1 : 1
  }
  if (a.minor !== b.minor) {
    return a.minor < b.minor ? -1 : 1
  }
  if (a.patch !== b.patch) {
    return a.patch < b.patch ? -1 : 1
  }

  return 0
}

/**
 * Check if a version is newer than another
 * @param newer - Version that might be newer
 * @param older - Version that might be older
 * @returns true if newer > older
 */
export function isVersionNewer(newer: string, older: string): boolean {
  const result = compareVersions(newer, older)
  return result === 1
}

/**
 * Check if a version is same or newer than another
 * @param version - Version to check
 * @param minimum - Minimum required version
 * @returns true if version >= minimum
 */
export function isVersionSameOrNewer(version: string, minimum: string): boolean {
  const result = compareVersions(version, minimum)
  return result === 1 || result === 0
}

/**
 * Format version for display (e.g., "v5.2.5")
 */
export function formatVersionDisplay(version: string): string {
  return `v${version}`
}
