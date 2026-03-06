// Purpose: Package export — HAR 1.2 and SARIF 2.1.0 serializers for captured browser data.
// Why: Provides stable export formats consumed by browser DevTools, GitHub Code Scanning, and CI pipelines.
// Docs: docs/features/feature/har-export/index.md

/*
Package export converts captured browser telemetry into standard interchange formats.

Key functions:
  - ExportHAR: converts NetworkBody entries into HAR 1.2 JSON for import into DevTools or Charles Proxy.
  - ExportSARIF: converts axe-core accessibility violations into SARIF 2.1.0 for GitHub Code Scanning.
  - SaveToFile: writes export output to a file path with atomic write semantics.
*/
package export
