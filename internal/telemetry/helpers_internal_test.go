// helpers_internal_test.go — Self-tests for the withRotation primitive.
// Verify save/restore ordering, t.Cleanup wiring, mutex semantics, and the
// nested-misuse t.Fatalf branch. Uses a private mutex + counter so these
// tests cannot interfere with the real package-level rotation mutexes
// (lockBudgetMu, homeDirFnMu, secondaryDirStateMu).

package telemetry

import (
	"fmt"
	"sync"
	"testing"
)

// fakeRotationT satisfies the rotationT interface without aborting the
// surrounding *testing.T. Fatalf records into fatal and panics with a
// sentinel so the calling goroutine unwinds; the test then recovers and
// inspects the captured message. Cleanup is collected so the test can
// invoke it manually after the body returns (mirroring t.Cleanup's
// post-test ordering without requiring a real *testing.T).
type fakeRotationT struct {
	fatal    string
	cleanups []func()
}

func (f *fakeRotationT) Helper() {}

func (f *fakeRotationT) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	// Use a sentinel panic so the caller unwinds — t.Fatalf semantics
	// require "abort the current goroutine"; our fake mimics that with
	// recover() at the call site.
	panic(fakeFatalSentinel{})
}

func (f *fakeRotationT) Cleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

func (f *fakeRotationT) runCleanups() {
	// LIFO, matching testing.T.Cleanup semantics.
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

type fakeFatalSentinel struct{}

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
// still held. Driven via fakeRotationT so the failure does not propagate
// to the surrounding test — we want to ASSERT that Fatalf was called, not
// have its call abort us.
func TestWithRotation_NestedCallFiresFatalf(t *testing.T) {
	var mu sync.Mutex
	mu.Lock() // simulate "another caller already holds the rotation lock"
	defer mu.Unlock()

	fake := &fakeRotationT{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(fakeFatalSentinel); !ok {
					panic(r) // re-raise unexpected panics
				}
			}
		}()
		withRotation(fake, &mu, "myLabel",
			func() int { return 0 },
			func(int) {})
		t.Fatal("expected fakeRotationT.Fatalf panic; did not occur")
	}()
	if fake.fatal == "" {
		t.Fatal("Fatalf was not called when mutex was already held")
	}
	// Label must appear in the misuse message verbatim — operators rely
	// on it to identify which helper was misused.
	const wantSubstr = "myLabel"
	if !contains(fake.fatal, wantSubstr) {
		t.Errorf("Fatalf message = %q, want substring %q", fake.fatal, wantSubstr)
	}
	// Save/restore must NOT have run (the call should bail before save).
	if len(fake.cleanups) != 0 {
		t.Errorf("Cleanup registered %d funcs after misuse; want 0", len(fake.cleanups))
	}
}

// TestWithRotation_RestoreRunsViaCleanup verifies the restore runs when
// the registered cleanup fires (driven via fakeRotationT so we can invoke
// cleanups deterministically without spinning up a child *testing.T).
func TestWithRotation_RestoreRunsViaCleanup(t *testing.T) {
	var mu sync.Mutex
	state := 7

	fake := &fakeRotationT{}
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

	fake.runCleanups()
	if state != 7 {
		t.Errorf("state after cleanup = %d, want 7", state)
	}
	// Mutex must be unlocked after cleanup.
	if !mu.TryLock() {
		t.Error("mutex still held after cleanup ran")
	} else {
		mu.Unlock()
	}
}

// contains is a tiny strings.Contains stand-in to avoid pulling in the
// strings package solely for this self-test (helpers_test.go's existing
// imports are unaffected).
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
