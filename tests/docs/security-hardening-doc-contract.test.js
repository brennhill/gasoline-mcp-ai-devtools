// @ts-nocheck

import { describe, test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const TEST_DIR = path.dirname(fileURLToPath(import.meta.url))
const REPO_ROOT = path.resolve(TEST_DIR, '../..')

function read(relativePath) {
  return fs.readFileSync(path.join(REPO_ROOT, relativePath), 'utf8')
}

describe('security hardening docs contract', () => {
  test('feature docs describe the live security surfaces instead of deleted security_config internals', () => {
    const featureIndex = read('docs/features/feature-index.md')
    const indexDoc = read('docs/features/feature/security-hardening/index.md')
    const productSpec = read('docs/features/feature/security-hardening/product-spec.md')
    const flowMap = read('docs/features/feature/security-hardening/flow-map.md')
    const canonicalFlowMap = read('docs/architecture/flow-maps/security-hardening-active-surfaces.md')
    const flowMapIndex = read('docs/architecture/flow-maps/README.md')

    assert.doesNotMatch(indexDoc, /security_config_(policy|mode|audit|unit_test|path_test)\.go|security_config_(unit_test|path_test)\.go/)
    assert.doesNotMatch(indexDoc, /Mode\/Action:\s+security config/i)

    assert.match(indexDoc, /Tool:\s+analyze, generate, configure/i)
    assert.match(indexDoc, /Mode\/Action:\s+security_audit, third_party_audit, csp, sri, security_mode/i)
    assert.match(indexDoc, /cmd\/browser-agent\/internal\/toolanalyze\/security\.go/)
    assert.match(indexDoc, /cmd\/browser-agent\/internal\/toolgenerate\/artifacts_security_impl\.go/)
    assert.match(indexDoc, /cmd\/browser-agent\/internal\/toolconfigure\/security_mode\.go/)

    assert.match(productSpec, /^tool:\s+analyze, generate, configure$/m)
    assert.match(productSpec, /^mode:\s+security_audit, third_party_audit, csp, sri, security_mode$/m)

    assert.match(flowMap, /security-hardening-active-surfaces\.md/)
    assert.match(canonicalFlowMap, /docs\/features\/feature\/security-hardening\/index\.md/)
    assert.match(canonicalFlowMap, /cmd\/browser-agent\/internal\/toolanalyze\/security\.go/)
    assert.match(canonicalFlowMap, /tests\/docs\/security-hardening-doc-contract\.test\.js/)
    assert.match(flowMapIndex, /Security Hardening Active Surfaces/)

    assert.match(featureIndex, /\| Security Hardening \| shipped \| analyze, generate, configure \| security_audit, third_party_audit, csp, sri, security_mode \|/)
  })
})
