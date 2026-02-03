#!/usr/bin/env node
/**
 * Bundle content and inject scripts using esbuild
 * Chrome content scripts don't support ES modules, so we bundle into single files
 */

import * as esbuild from 'esbuild'

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
    minify: false, // Keep readable for debugging
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
  })
  console.log('✅ Inject script bundled successfully')
} catch (error) {
  console.error('❌ Script bundling failed:', error)
  process.exit(1)
}
