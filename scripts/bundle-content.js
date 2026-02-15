#!/usr/bin/env node
/**
 * Bundle content and inject scripts using esbuild
 * Chrome content scripts don't support ES modules, so we bundle into single files
 */

import * as esbuild from 'esbuild'
import { readFileSync } from 'fs'

const version = readFileSync('VERSION', 'utf-8').trim()

try {
  // Bundle content.js (IIFE for content script context)
  await esbuild.build({
    entryPoints: ['extension/content.js'],
    bundle: true,
    format: 'iife',
    outfile: 'extension/content.bundled.js',
    platform: 'browser',
    target: ['chrome120'],
    sourcemap: true,
    minify: false // Keep readable for debugging
  })
  console.log('✅ Content script bundled successfully')

  // Bundle inject.js (ESM for page context module script)
  await esbuild.build({
    entryPoints: ['extension/inject.js'],
    bundle: true,
    format: 'esm', // Keep as ESM since it's loaded as type="module"
    outfile: 'extension/inject.bundled.js',
    platform: 'browser',
    target: ['chrome120'],
    sourcemap: true,
    minify: false,
    define: { __GASOLINE_VERSION__: JSON.stringify(version) }
  })
  console.log('✅ Inject script bundled successfully')

  // Bundle early-patch.js (IIFE for MAIN world content script)
  // Runs on ALL pages at document_start — minified to minimize overhead
  await esbuild.build({
    entryPoints: ['extension/early-patch.js'],
    bundle: true,
    format: 'iife',
    outfile: 'extension/early-patch.bundled.js',
    platform: 'browser',
    target: ['chrome120'],
    sourcemap: true,
    minify: true
  })
  console.log('✅ Early-patch script bundled successfully')

  // Bundle offscreen.js (IIFE for offscreen document — recording engine)
  await esbuild.build({
    entryPoints: ['extension/offscreen.js'],
    bundle: true,
    format: 'iife',
    outfile: 'extension/offscreen.bundled.js',
    platform: 'browser',
    target: ['chrome120'],
    sourcemap: true,
    minify: false
  })
  console.log('✅ Offscreen recording script bundled successfully')
} catch (error) {
  console.error('❌ Script bundling failed:', error)
  // eslint-disable-next-line n/no-process-exit -- CLI script exits with error status on build failure
  process.exit(1)
}
