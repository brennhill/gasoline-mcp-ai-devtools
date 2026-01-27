/**
 * Global teardown: Clean up after E2E tests
 */

export default async function globalTeardown() {
  // Nothing to clean up globally - server processes are managed per-test by the fixture
  console.log('[e2e] Tests complete.')
}
