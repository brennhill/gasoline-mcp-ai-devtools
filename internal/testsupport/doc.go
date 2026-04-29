// Package testsupport provides utilities shared across test files in the
// internal/* package tree.
//
// Helpers belong here only when (a) used by 2+ test packages and (b)
// cannot live in `*_test.go` due to Go's per-package test compilation.
// The bar is intentionally high to keep the package small and focused.
//
// Production code MUST NOT import this package — the helpers here are
// test-only and there is no API stability guarantee. Cross-package
// imports from `*_test.go` are expected and supported. The
// "MUST NOT import" rule is enforced by
// TestPackageNotImportedByProductionCode in package_isolation_test.go.
//
// Files in this package, by concern:
//   - doc.go      — this file (package overview)
//   - tb.go       — canonical *testing.T-shaped interfaces (HelperFatalfTB, HelperFatalTB)
//   - faket.go    — *testing.T fake (FakeT, ExpectFakeFatal)
//   - repo.go     — repo-root walk + go.mod parser (RepoRoot, ExpectedModulePath)
//   - paths.go    — filesystem path helpers (AssertPathResolvesTo)
//   - astutil.go  — Go AST traversal helpers (ImportQualifiers, ImportFacts)
package testsupport
