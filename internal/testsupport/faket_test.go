// faket_test.go — Self-tests for FakeT semantics (cleanup ordering and
// idempotency, Fatalf recording). These tests live next to the
// implementation (faket.go) so a future move/delete of FakeT updates
// both the production and test surface in one place — earlier these
// tests were in internal/telemetry, which left a cross-package coupling
// that grew costly.

package testsupport

import (
	"sync"
	"testing"
)

// TestFakeT_RunCleanupsLIFO pins the LIFO ordering of FakeT.RunCleanups —
// the whole point of the fake (mirroring testing.T.Cleanup semantics).
// A regression that switched the iteration to FIFO would silently flip
// every withRotation self-test's restore order vs. real *testing.T
// behavior, producing a fake that disagrees with the system it
// impersonates.
func TestFakeT_RunCleanupsLIFO(t *testing.T) {
	fake := &FakeT{}
	var order []int
	fake.Cleanup(func() { order = append(order, 1) })
	fake.Cleanup(func() { order = append(order, 2) })
	fake.Cleanup(func() { order = append(order, 3) })

	fake.RunCleanups()

	want := []int{3, 2, 1}
	if len(order) != len(want) {
		t.Fatalf("cleanups ran %d times, want %d", len(order), len(want))
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("cleanup[%d] = %d, want %d (RunCleanups must be LIFO)", i, order[i], want[i])
		}
	}
}

// TestFakeT_RunCleanupsIdempotent pins the divergence-protection contract:
// a second RunCleanups call MUST be a no-op, matching real *testing.T
// (which does not re-fire cleanups). Without this, a future test that
// defers RunCleanups AND calls it explicitly would phantom-restore
// state twice.
func TestFakeT_RunCleanupsIdempotent(t *testing.T) {
	fake := &FakeT{}
	var fires int
	fake.Cleanup(func() { fires++ })

	fake.RunCleanups()
	fake.RunCleanups() // second call must NOT re-fire registered cleanups.

	if fires != 1 {
		t.Errorf("cleanup fired %d times across two RunCleanups calls; want 1 (idempotent)", fires)
	}
}

// TestFakeT_FatalfRecordsAllMessages pins that multiple Fatalf calls
// across the FakeT's lifetime are all retained in Fatals(), with Fatal()
// returning the most recent. Real testing.T aborts the goroutine on the
// first Fatalf, but FakeT can be re-driven (a cleanup may invoke
// Fatalf, then a subsequent body invokes it again), and tests asserting
// the multi-message path need history to be observable.
func TestFakeT_FatalfRecordsAllMessages(t *testing.T) {
	fake := &FakeT{}

	for _, msg := range []string{"first", "second", "third"} {
		// Each Fatalf panics; recover via the package's own helper.
		func(m string) {
			defer recoverFakeFatal()
			fake.Fatalf("%s", m)
		}(msg)
	}

	got := fake.Fatals()
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("Fatals() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Fatals()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if last := fake.Fatal(); last != "third" {
		t.Errorf("Fatal() = %q, want %q (most-recent message)", last, "third")
	}
}

// TestFakeT_FatalEmptyWhenNoFatalfFired pins the zero-value behavior:
// a fresh FakeT returns "" from Fatal() and nil-equivalent from
// Fatals() before any Fatalf has been recorded. ExpectFakeFatal relies
// on Fatal() being "" to detect a body that returned without firing.
func TestFakeT_FatalEmptyWhenNoFatalfFired(t *testing.T) {
	fake := &FakeT{}
	if got := fake.Fatal(); got != "" {
		t.Errorf("Fatal() = %q, want empty before any Fatalf", got)
	}
	if got := fake.Fatals(); len(got) != 0 {
		t.Errorf("Fatals() = %v, want empty before any Fatalf", got)
	}
}

// TestFakeT_ConcurrentRecording is a smoke test for the mutex around
// fatals/cleanups. Drives N goroutines that each invoke Fatalf and
// register a cleanup, then verifies all messages were recorded and all
// cleanups fire LIFO (note: LIFO across concurrent registration is not
// strictly defined, but every registered cleanup must fire exactly
// once — that's the load-bearing property).
func TestFakeT_ConcurrentRecording(t *testing.T) {
	fake := &FakeT{}
	const goroutines = 16

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			defer recoverFakeFatal()
			fake.Cleanup(func() {})
			fake.Fatalf("worker-%d", i)
		}(i)
	}
	wg.Wait()

	if got := len(fake.Fatals()); got != goroutines {
		t.Errorf("Fatals() len = %d, want %d (one per goroutine)", got, goroutines)
	}

	// Every cleanup must fire on RunCleanups; we can't assert order
	// because registration order across goroutines is non-deterministic.
	var firedMu sync.Mutex
	fired := 0
	// Reset cleanups; install N counters.
	fake = &FakeT{}
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			fake.Cleanup(func() {
				firedMu.Lock()
				fired++
				firedMu.Unlock()
			})
		}()
	}
	wg.Wait()

	fake.RunCleanups()
	if fired != goroutines {
		t.Errorf("cleanups fired = %d, want %d", fired, goroutines)
	}
}

// TestExpectFakeFatal_FailsSurroundingTBWhenBodyReturns drives
// ExpectFakeFatal with a body that does NOT panic, asserts the
// surrounding TB receives a Fatal call. Uses a fakeFatalSink that
// captures the Fatal invocation without aborting the real *testing.T.
func TestExpectFakeFatal_FailsSurroundingTBWhenBodyReturns(t *testing.T) {
	sink := &fakeFatalSink{}
	fake := &FakeT{}

	defer func() {
		// ExpectFakeFatal panics when sink.Fatal is called (because
		// our sink stores + panics, mirroring t.FailNow). Recover so
		// the surrounding test continues.
		if r := recover(); r != nil {
			if _, ok := r.(fakeFatalSinkSentinel); !ok {
				panic(r)
			}
		}
	}()

	ExpectFakeFatal(sink, fake, func() {
		// Body returns normally — no Fatalf.
	})

	// Unreachable due to panic; if reached, ExpectFakeFatal failed to
	// signal.
	t.Fatal("ExpectFakeFatal returned normally after a non-panicking body; expected sink.Fatal to fire")
}

// fakeFatalSink is a minimal helperFatalTB that records Fatal
// invocations without aborting the surrounding test.
type fakeFatalSink struct{ args []any }

type fakeFatalSinkSentinel struct{}

func (s *fakeFatalSink) Helper() {}
func (s *fakeFatalSink) Fatal(args ...any) {
	s.args = args
	panic(fakeFatalSinkSentinel{})
}
