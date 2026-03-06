// Purpose: Analyzes test files for selectors, repairs broken selectors, and runs batch healing across directories.
// Docs: docs/features/feature/test-generation/index.md
//
// Layout:
// - heal_paths.go: path/directory validation and resolution
// - heal_selectors.go: selector extraction and validation
// - heal_repair.go: selector healing and classification
// - heal_batch.go: batch file walking and aggregate healing
// - heal_summary.go: user-facing summary formatting

package testgen
