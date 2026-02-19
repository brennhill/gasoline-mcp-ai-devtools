#!/usr/bin/env node
/**
 * generate-dom-primitives.js
 *
 * Generates src/background/dom-primitives.ts from a single template source:
 * scripts/templates/dom-primitives.ts.tpl
 *
 * Usage:
 *   node scripts/generate-dom-primitives.js         # write/update generated file
 *   node scripts/generate-dom-primitives.js --check # exit non-zero if out of date
 */

import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.join(__dirname, '..')

const TEMPLATE_PATH = path.join(ROOT, 'scripts', 'templates', 'dom-primitives.ts.tpl')
const OUTPUT_PATH = path.join(ROOT, 'src', 'background', 'dom-primitives.ts')
const CHECK_ONLY = process.argv.includes('--check')

const GENERATED_BANNER = `// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source template: scripts/templates/dom-primitives.ts.tpl
// Generator: scripts/generate-dom-primitives.js

`

function normalize(content) {
  return content.replace(/\r\n/g, '\n').trimEnd() + '\n'
}

function buildOutput(templateContent) {
  return GENERATED_BANNER + normalize(templateContent)
}

function main() {
  if (!fs.existsSync(TEMPLATE_PATH)) {
    console.error(`Template not found: ${TEMPLATE_PATH}`)
    process.exit(1)
  }

  const templateContent = fs.readFileSync(TEMPLATE_PATH, 'utf8')
  const generatedContent = buildOutput(templateContent)
  const existingContent = fs.existsSync(OUTPUT_PATH) ? fs.readFileSync(OUTPUT_PATH, 'utf8') : ''
  const isDrifted = normalize(existingContent) !== normalize(generatedContent)

  if (CHECK_ONLY) {
    if (isDrifted) {
      console.error('dom-primitives.ts is out of date.')
      console.error('Run: node scripts/generate-dom-primitives.js')
      process.exit(1)
    }
    console.log('dom-primitives.ts is up to date.')
    return
  }

  fs.writeFileSync(OUTPUT_PATH, generatedContent, 'utf8')
  if (isDrifted) {
    console.log('Generated src/background/dom-primitives.ts from template.')
  } else {
    console.log('dom-primitives.ts already current (rewritten for normalized line endings).')
  }
}

main()
