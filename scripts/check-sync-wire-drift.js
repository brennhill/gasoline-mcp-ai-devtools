#!/usr/bin/env node
// check-sync-wire-drift.js — Validates /sync protocol types stay aligned between Go and TypeScript.
// Why: Sync types are hand-maintained in both languages; this catches field name,
//      optionality, and missing-field drift before it reaches production.
//
// Usage:
//   node scripts/check-sync-wire-drift.js        # Check for drift (exit non-zero if found)

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const ROOT = path.resolve(__dirname, '..')

const GO_FILE = 'internal/capture/sync.go'
const TS_FILE = 'src/background/sync-client.ts'

// Additional Go/TS file pairs for types defined outside the primary sync files.
const EXTRA_TYPE_FILES = [
  {
    goFile: 'internal/types/log.go',
    tsFile: 'src/background/sync-client.ts',
    goTypeName: 'ExtensionLog',
    tsTypeName: 'SyncExtensionLog'
  },
  // Upload wire types (internal/upload/types.go <-> src/types/wire-upload.ts)
  {
    goFile: 'internal/upload/types.go',
    tsFile: 'src/types/wire-upload.ts',
    goTypeName: 'FileReadResponse',
    tsTypeName: 'FileReadResponse'
  },
  {
    goFile: 'internal/upload/types.go',
    tsFile: 'src/types/wire-upload.ts',
    goTypeName: 'StageResponse',
    tsTypeName: 'StageResponse'
  },
  // Push wire types (cmd/browser-agent/push_handlers.go <-> src/types/wire-push.ts)
  {
    goFile: 'cmd/browser-agent/push_handlers.go',
    tsFile: 'src/types/wire-push.ts',
    goTypeName: 'PushScreenshotRequest',
    tsTypeName: 'PushScreenshotRequest'
  },
  {
    goFile: 'cmd/browser-agent/push_handlers.go',
    tsFile: 'src/types/wire-push.ts',
    goTypeName: 'PushMessageRequest',
    tsTypeName: 'PushMessageRequest'
  },
  {
    goFile: 'cmd/browser-agent/push_handlers.go',
    tsFile: 'src/types/wire-push.ts',
    goTypeName: 'PushCapabilitiesResponse',
    tsTypeName: 'PushCapabilities'
  },
  {
    goFile: 'cmd/browser-agent/push_handlers.go',
    tsFile: 'src/types/wire-push.ts',
    goTypeName: 'PushResponse',
    tsTypeName: 'PushResponse'
  }
]

// Sync types that must stay aligned between Go and TS.
// Only types that cross the wire (sent/received over HTTP) are checked.
const SYNC_TYPES = [
  'SyncRequest',
  'SyncResponse',
  'SyncSettings',
  'SyncCommand',
  'SyncCommandResult',
  'SyncInProgress'
]

/**
 * Intentional optionality overrides. Keys are "TypeName.json_field".
 * When set, the checker allows TS to be more permissive (optional) than Go.
 * Each entry MUST have a justification comment.
 */
const OPTIONALITY_OVERRIDES = {
  // Go always sends capture_overrides (even as {}), but TS treats it as optional
  // for defensive parsing. The TS consumer already guards with `if (data.capture_overrides)`.
  'SyncResponse.capture_overrides': 'ts-optional-ok',
  // Go marks category as omitempty (empty string omitted), but TS always sends it.
  // The extension always populates category before sending; Go omitempty only matters
  // for server→client serialization where empty category is dropped.
  'ExtensionLog/SyncExtensionLog.category': 'ts-required-ok'
}

// ============================================
// Go Parser
// ============================================

/**
 * Parse a Go struct and return { name, fields: [{ jsonTag, optional }] }.
 * Optional means the json tag has ",omitempty" or the Go type is a pointer/slice/map.
 */
function parseGoStruct(content, typeName) {
  const structRegex = new RegExp(`type\\s+${typeName}\\s+struct\\s*\\{([^}]*)\\}`, 's')
  const match = content.match(structRegex)
  if (!match) return null

  const body = match[1]
  const fields = []

  for (const line of body.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('//')) continue

    const fieldMatch = trimmed.match(/^\w+\s+\S+\s+`json:"([^"]+)"`/)
    if (!fieldMatch) continue

    const jsonTagRaw = fieldMatch[1]
    if (jsonTagRaw === '-') continue

    const parts = jsonTagRaw.split(',')
    const jsonTag = parts[0]
    const omitempty = parts.includes('omitempty')

    // Pointer types are effectively optional in JSON (nil -> omitted or null).
    // Slices and maps without omitempty are always present (nil -> null or []/{}),
    // so they count as required from a wire perspective.
    const typeMatch = trimmed.match(/^\w+\s+(\S+)\s+`json:/)
    const goType = typeMatch ? typeMatch[1] : ''
    const isPointer = goType.startsWith('*')

    fields.push({
      jsonTag,
      optional: omitempty || isPointer
    })
  }

  return { name: typeName, fields }
}

// ============================================
// TS Parser
// ============================================

/**
 * Parse a TS interface and return { name, fields: [{ jsonTag, optional }] }.
 */
function parseTSInterface(content, typeName) {
  // Match: export interface TypeName { ... } or interface TypeName { ... }
  const ifaceRegex = new RegExp(`(?:export\\s+)?interface\\s+${typeName}\\s*\\{([^}]*)\\}`, 's')
  const match = content.match(ifaceRegex)
  if (!match) return null

  const body = match[1]
  const fields = []

  for (const line of body.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('//') || trimmed.startsWith('*')) continue

    // Match: readonly field_name?: type  OR  field_name: type
    const fieldMatch = trimmed.match(/^(?:readonly\s+)?(\w+)(\??)\s*:/)
    if (!fieldMatch) continue

    const [, fieldName, optMarker] = fieldMatch
    fields.push({
      jsonTag: fieldName,
      optional: optMarker === '?'
    })
  }

  return { name: typeName, fields }
}

// ============================================
// Comparison
// ============================================

function compareType(goStruct, tsInterface) {
  const errors = []
  const typeName = goStruct.name

  const goFields = new Map(goStruct.fields.map((f) => [f.jsonTag, f]))
  const tsFields = new Map(tsInterface.fields.map((f) => [f.jsonTag, f]))

  // Check for fields in Go but missing from TS
  for (const [tag, goField] of goFields) {
    if (!tsFields.has(tag)) {
      errors.push(`${typeName}: field "${tag}" exists in Go but missing from TS`)
      continue
    }

    const tsField = tsFields.get(tag)

    // Check optionality alignment
    // Go optional (omitempty/pointer/slice/map) should be TS optional (?)
    // Go required should be TS required
    if (goField.optional && !tsField.optional) {
      const overrideKey = `${typeName}.${tag}`
      if (!OPTIONALITY_OVERRIDES[overrideKey]) {
        errors.push(
          `${typeName}.${tag}: Go is optional (omitempty/pointer) but TS declares it required — add to OPTIONALITY_OVERRIDES if intentional`
        )
      }
    }
    if (!goField.optional && tsField.optional) {
      const overrideKey = `${typeName}.${tag}`
      if (!OPTIONALITY_OVERRIDES[overrideKey]) {
        errors.push(
          `${typeName}.${tag}: Go is required but TS declares it optional — add to OPTIONALITY_OVERRIDES if intentional`
        )
      }
    }
  }

  // Check for fields in TS but missing from Go
  for (const [tag] of tsFields) {
    if (!goFields.has(tag)) {
      errors.push(`${typeName}: field "${tag}" exists in TS but missing from Go`)
    }
  }

  return errors
}

// ============================================
// Main
// ============================================

const goPath = path.join(ROOT, GO_FILE)
const tsPath = path.join(ROOT, TS_FILE)

if (!fs.existsSync(goPath)) {
  console.error(`MISSING: ${GO_FILE}`)
  process.exit(1)
}
if (!fs.existsSync(tsPath)) {
  console.error(`MISSING: ${TS_FILE}`)
  process.exit(1)
}

const goContent = fs.readFileSync(goPath, 'utf-8')
const tsContent = fs.readFileSync(tsPath, 'utf-8')

let allErrors = []
let checkedCount = 0

for (const typeName of SYNC_TYPES) {
  const goStruct = parseGoStruct(goContent, typeName)
  if (!goStruct) {
    allErrors.push(`${typeName}: not found in ${GO_FILE}`)
    continue
  }

  const tsInterface = parseTSInterface(tsContent, typeName)
  if (!tsInterface) {
    allErrors.push(`${typeName}: not found in ${TS_FILE}`)
    continue
  }

  const errors = compareType(goStruct, tsInterface)
  allErrors = allErrors.concat(errors)
  checkedCount++
}

// Check extra type pairs defined in separate files.
for (const extra of EXTRA_TYPE_FILES) {
  const extraGoPath = path.join(ROOT, extra.goFile)
  const extraTsPath = path.join(ROOT, extra.tsFile)

  if (!fs.existsSync(extraGoPath)) {
    allErrors.push(`${extra.goTypeName}: Go file not found: ${extra.goFile}`)
    continue
  }
  if (!fs.existsSync(extraTsPath)) {
    allErrors.push(`${extra.tsTypeName}: TS file not found: ${extra.tsFile}`)
    continue
  }

  const extraGoContent = fs.readFileSync(extraGoPath, 'utf-8')
  const extraTsContent = fs.readFileSync(extraTsPath, 'utf-8')

  const goStruct = parseGoStruct(extraGoContent, extra.goTypeName)
  if (!goStruct) {
    allErrors.push(`${extra.goTypeName}: not found in ${extra.goFile}`)
    continue
  }

  const tsInterface = parseTSInterface(extraTsContent, extra.tsTypeName)
  if (!tsInterface) {
    allErrors.push(`${extra.tsTypeName}: not found in ${extra.tsFile}`)
    continue
  }

  // Compare using the Go type name for error messages, noting the TS alias.
  const crossErrors = compareType(
    { name: `${extra.goTypeName}/${extra.tsTypeName}`, fields: goStruct.fields },
    { name: `${extra.goTypeName}/${extra.tsTypeName}`, fields: tsInterface.fields }
  )
  allErrors = allErrors.concat(crossErrors)
  checkedCount++
}

if (allErrors.length > 0) {
  console.error('SYNC WIRE DRIFT DETECTED:')
  for (const err of allErrors) {
    console.error(`  - ${err}`)
  }
  console.error(`\nChecked ${checkedCount}/${SYNC_TYPES.length + EXTRA_TYPE_FILES.length} sync types`)
  console.error(`Fix the drift in the relevant Go or TS files`)
  process.exit(1)
} else {
  console.log(`OK: ${checkedCount} sync wire types verified, zero drift`)
}
