// install_id_drift_testseam.go — Test-only introspection seam for the
// install_id drift log fn registration.
//
// HasInstallIDDriftLogFnForTest is a CONTRACT that production code must
// never call. The "ForTest" suffix is Go's recognized convention for
// test-only exports (see runtime/runtime_test.go and similar). Reviewers
// should reject any production-code call site as broken on its face.
//
// The function lives on the public API surface — not in helpers_test.go or
// behind a build tag — because:
//   1. cross-package tests in cmd/browser-agent need to link against it,
//   2. Go's export_test.go pattern only crosses the SAME-package external
//      test boundary (`package telemetry_test`), not arbitrary downstream
//      packages,
//   3. build-tag gating would force every test invocation to pass
//      `-tags=...`, polluting Makefile/CI for one introspection function.
//
// File-naming convention: `_testseam` makes the intent unmistakable at the
// directory level even before opening the file.

package telemetry

// HasInstallIDDriftLogFnForTest reports whether SetInstallIDDriftLogFn has
// installed a non-nil callback. Test-only — see file header.
func HasInstallIDDriftLogFnForTest() bool {
	return loadInstallIDDriftLogFn() != nil
}
