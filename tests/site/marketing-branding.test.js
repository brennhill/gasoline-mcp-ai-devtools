/**
 * @fileoverview marketing-branding.test.js — Stable-site branding regression coverage.
 */

import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

function read(path) {
  return readFileSync(new URL(`../../${path}`, import.meta.url), 'utf8')
}

test('stable marketing surfaces use Gasoline branding and the updated tagline', () => {
  const docsIndex = read('cookwithgasoline.com/src/content/docs/index.mdx')
  const landing = read('cookwithgasoline.com/src/components/Landing.astro')
  const footer = read('cookwithgasoline.com/src/components/Footer.astro')
  const rotatingHero = read('cookwithgasoline.com/src/components/RotatingHero.astro')

  assert.match(docsIndex, /text:\s*Install Gasoline\b/, 'docs splash install CTA should use Gasoline branding')
  assert.doesNotMatch(
    docsIndex,
    /Install Gasoline Agentic Devtools/,
    'docs splash install CTA should not use the longer legacy product name'
  )

  assert.match(landing, />Install Gasoline</, 'landing install CTA should use Gasoline branding')
  assert.doesNotMatch(
    landing,
    /Install Gasoline Agentic Devtools|Install &amp; Automate Now/,
    'landing CTAs should not use the legacy install copy'
  )

  assert.match(footer, /<p>Gasoline &copy; 2025-2026/, 'footer should use the Gasoline brand name')
  assert.match(footer, /Fueling rapid development with AI\./, 'footer should use the updated tagline')
  assert.doesNotMatch(
    footer,
    /Pouring fuel on the AI development fire\./,
    'footer should not use the old tagline'
  )

  assert.match(
    rotatingHero,
    /subtitle:\s*"Fueling rapid development with AI\."/,
    'rotating hero should use the updated tagline'
  )
  assert.doesNotMatch(
    rotatingHero,
    /subtitle:\s*"Pouring fuel on the AI development fire"/,
    'rotating hero should not use the old tagline'
  )
})
