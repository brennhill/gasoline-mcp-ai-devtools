// Purpose: Package testing — shared internal test bootstrap helpers for server and capture instances.
// Why: Reduces duplicated test setup logic and keeps fixture initialization consistent across tests.
// Docs: docs/features/feature/self-testing/index.md

/*
Package testing provides shared test helpers for creating pre-configured server
and capture instances used across internal package tests.

Key functions:
  - SetupTestServer: creates a Server with a temporary log file.
  - SetupTestCapture: creates a Capture instance with default test configuration.
*/
package testing
