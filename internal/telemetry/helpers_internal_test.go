// helpers_internal_test.go — Self-tests for the withRotation primitive.
// Verify save/restore ordering, t.Cleanup wiring, mutex semantics, and the
// nested-misuse t.Fatalf branch. Uses a private mutex + counter so these
// tests cannot interfere with the real package-level rotation mutexes
// (lockBudgetMu, homeDirFnMu, secondaryDirStateMu).
//
// The `_internal_test.go` filename is convention only — Go's build system
// treats every `*_test.go` file in this package identically. The suffix
// signals to humans that this file tests un-exported package internals,
// matching the same intent the standard library uses (e.g.,
// `time/internal_test.go`).
//
// FakeT-specific semantic tests (LIFO cleanup, idempotency, multi-Fatalf
// recording) live in internal/testsupport/faket_test.go next to the
// implementation. This file restricts itself to withRotation behavior.
//
// CONSTRAINT — no t.Parallel() in package telemetry: see doc.go for the
// package-wide explanation. New tests in this package must NOT call
// t.Parallel().

package telemetry

import (
	"strings"
	"sync"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsupport"
)

// TestWithRotation_SaveRunsBeforeRestore exercises the happy path: save
// captures pre-state, body mutates state, t.Cleanup restores the snapshot.
// Verifies the call order is exactly save → (body) → restore.
func TestWithRotation_SaveRunsBeforeRestore(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	state := 1

	t.Run("scoped", func(t *testing.T) {
		withRotation(t, &mu, "test",
			func() int { calls = append(calls, "save"); return state },
			func(v int) { calls = append(calls, "restore"); state = v })
		state = 99 // simulate body mutation
	})
	// After the subtest's cleanup runs, restore must have fired.
	if state != 1 {
		t.Errorf("state after cleanup = %d, want 1 (restore did not run)", state)
	}
	if len(calls) != 2 || calls[0] != "save" || calls[1] != "restore" {
		t.Errorf("call order = %v, want [save restore]", calls)
	}
}

// TestWithRotation_MutexUnlockedAfterCleanup verifies that the mutex is
// released by t.Cleanup, so a follow-up withRotation in a sibling test
// using the same mutex succeeds rather than fatal'ing on the misuse guard.
func TestWithRotation_MutexUnlockedAfterCleanup(t *testing.T) {
	var mu sync.Mutex

	t.Run("first", func(t *testing.T) {
		withRotation(t, &mu, "first",
			func() int { return 0 },
			func(int) {})
	})
	// If cleanup didn't unlock, TryLock would fail here and t.Fatalf would
	// fire — surfacing as the second subtest panicking with the misuse
	// message instead of running.
	t.Run("second", func(t *testing.T) {
		withRotation(t, &mu, "second",
			func() int { return 0 },
			func(int) {})
	})
}

// TestWithRotation_NestedCallFiresFatalf verifies the misuse guard fires
// t.Fatalf with the label when a second call occurs while the mutex is
// still held. Driven via testsupport.ExpectFakeFatal so the failure does
// not propagate to the surrounding test — we want to ASSERT that Fatalf
// was called, not have its call abort us.
func TestWithRotation_NestedCallFiresFatalf(t *testing.T) {
	var mu sync.Mutex
	mu.Lock() // simulate "another caller already holds the rotation lock"
	defer mu.Unlock()

	fake := &testsupport.FakeT{}
	testsupport.ExpectFakeFatal(t, fake, func() {
		withRotation(fake, &mu, "myLabel",
			func() int { return 0 },
			func(int) {})
	})
	if fake.LastFatal() == "" {
		t.Fatal("Fatalf was not called when mutex was already held")
	}
	// Label must appear in the misuse message verbatim — operators rely
	// on it to identify which helper was misused.
	const wantSubstr = "myLabel"
	if !strings.Contains(fake.LastFatal(), wantSubstr) {
		t.Errorf("Fatalf message = %q, want substring %q", fake.LastFatal(), wantSubstr)
	}
}

// TestWithRotation_RestoreRunsViaCleanup verifies the restore runs when
// the registered cleanup fires (driven via testsupport.FakeT so we can
// invoke cleanups deterministically without spinning up a child
// *testing.T).
func TestWithRotation_RestoreRunsViaCleanup(t *testing.T) {
	var mu sync.Mutex
	state := 7

	fake := &testsupport.FakeT{}
	withRotation(fake, &mu, "label",
		func() int { return state },
		func(v int) { state = v })
	state = 999

	// mu must be HELD at this point — withRotation locked it and hasn't
	// unlocked yet (cleanup hasn't fired). Confirm via a TryLock that
	// must fail.
	if mu.TryLock() {
		mu.Unlock()
		t.Fatal("withRotation did not hold the mutex post-call")
	}

	fake.RunCleanups()
	if state != 7 {
		t.Errorf("state after cleanup = %d, want 7", state)
	}
	// Mutex must be unlocked after cleanup ran.
	if !mu.TryLock() {
		t.Error("mutex still held after cleanup ran")
	} else {
		mu.Unlock()
	}
}
