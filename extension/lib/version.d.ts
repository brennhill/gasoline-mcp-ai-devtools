/**
 * @fileoverview Version - Utilities for semver comparison and version checking
 */
/**
 * Parse a semantic version string into components
 * @param version - Version string like "5.2.5"
 * @returns Object with major, minor, patch components, or null if invalid
 */
export declare function parseVersion(version: string): {
  major: number
  minor: number
  patch: number
} | null
/**
 * Compare two semantic versions
 * @param versionA - First version string
 * @param versionB - Second version string
 * @returns -1 if A < B, 0 if A == B, 1 if A > B, null if either is invalid
 */
export declare function compareVersions(versionA: string, versionB: string): -1 | 0 | 1 | null
/**
 * Check if a version is newer than another
 * @param newer - Version that might be newer
 * @param older - Version that might be older
 * @returns true if newer > older
 */
export declare function isVersionNewer(newer: string, older: string): boolean
/**
 * Check if a version is same or newer than another
 * @param version - Version to check
 * @param minimum - Minimum required version
 * @returns true if version >= minimum
 */
export declare function isVersionSameOrNewer(version: string, minimum: string): boolean
/**
 * Format version for display (e.g., "v5.2.5")
 */
export declare function formatVersionDisplay(version: string): string
//# sourceMappingURL=version.d.ts.map
