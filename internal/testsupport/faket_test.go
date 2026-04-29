// faket_test.go — Self-tests for FakeT semantics. Live next to the
// implementation (faket.go) so a future move/delete of FakeT updates
// both surfaces in one place.

package testsupport

import (
	"reflect"
	"sync"
	"sync/atomic"
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

// TestFakeT_LastFatalReturnsMostRecent pins the LastFatal() accessor
// contract: returns "" before any Fatalf, and the most-recent message
// after each subsequent Fatalf. The rename from Fatal() to LastFatal()
// disambiguates from testing.TB.Fatal(args ...any), which is a verb
// (fire a failure), not a noun (read recorded message).
func TestFakeT_LastFatalReturnsMostRecent(t *testing.T) {
	fake := &FakeT{}
	if got := fake.LastFatal(); got != "" {
		t.Errorf("LastFatal() on fresh fake = %q, want empty", got)
	}

	for _, msg := range []string{"first", "second", "third"} {
		func(m string) {
			defer recoverFakeFatal()
			fake.Fatalf("%s", m)
		}(msg)
		if got := fake.LastFatal(); got != msg {
			t.Errorf("after Fatalf(%q): LastFatal() = %q, want %q", msg, got, msg)
		}
	}
}

// TestFakeT_FatalfRecordsAllMessages pins that multiple Fatalf calls
// across the FakeT's lifetime are all retained in Fatals(). Real
// testing.T aborts the goroutine on the first Fatalf, but FakeT can be
// re-driven (a cleanup may invoke Fatalf, then a subsequent body invokes
// it again), and tests asserting the multi-message path need history to
// be observable.
func TestFakeT_FatalfRecordsAllMessages(t *testing.T) {
	fake := &FakeT{}

	for _, msg := range []string{"first", "second", "third"} {
		func(m string) {
			defer recoverFakeFatal()
			fake.Fatalf("%s", m)
		}(msg)
	}

	got := fake.Fatals()
	want := []string{"first", "second", "third"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fatals() = %v, want %v", got, want)
	}
	if last := fake.LastFatal(); last != "third" {
		t.Errorf("LastFatal() = %q, want %q (most-recent message)", last, "third")
	}
}

// TestFakeT_FatalfFromCleanupRecorded is the load-bearing test for the
// motivating case the FakeT.Fatalf doc cites: "real code can invoke a
// fake's Fatalf from a deferred cleanup, in which case both messages
// must be visible." A registered cleanup calls Fatalf; we then drive
// RunCleanups and assert both the body's message AND the cleanup's
// message are recorded in source order.
//
// Without this test, the doc's promise is unverified — a regression
// that swapped the order, dropped the cleanup-side message, or dead-
// locked on cleanup-reentry would slip through.
func TestFakeT_FatalfFromCleanupRecorded(t *testing.T) {
	fake := &FakeT{}

	fake.Cleanup(func() {
		defer recoverFakeFatal()
		fake.Fatalf("from-cleanup")
	})

	// Body: fire Fatalf, recover the sentinel.
	func() {
		defer recoverFakeFatal()
		fake.Fatalf("from-body")
	}()

	// Cleanup also panics; recover at the RunCleanups call site.
	func() {
		defer recoverFakeFatal()
		fake.RunCleanups()
	}()

	got := fake.Fatals()
	want := []string{"from-body", "from-cleanup"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Fatals() = %v, want %v (both body and cleanup messages, in order)", got, want)
	}
	if last := fake.LastFatal(); last != "from-cleanup" {
		t.Errorf("LastFatal() = %q, want %q (cleanup ran AFTER body)", last, "from-cleanup")
	}
}

// TestFakeT_CleanupCanReenterFakeT pins the documented contract that
// "Cleanups run WITHOUT the internal mutex held — a cleanup is allowed
// to call back into FakeT methods (e.g., register another cleanup, or
// invoke Fatalf) without deadlocking." A regression that moved the
// cleanup invocation INSIDE the mutex window would deadlock here, not
// in production where the path is rare.
func TestFakeT_CleanupCanReenterFakeT(t *testing.T) {
	fake := &FakeT{}
	var bodyRan, reentrantRan bool

	fake.Cleanup(func() {
		bodyRan = true
		// Reenter to register another cleanup. This MUST NOT deadlock.
		fake.Cleanup(func() {
			reentrantRan = true
		})
		// Reenter to record a Fatalf. This MUST NOT deadlock either —
		// recover the sentinel so RunCleanups can finish the LIFO
		// sweep.
		func() {
			defer recoverFakeFatal()
			fake.Fatalf("from-reentrant")
		}()
	})

	fake.RunCleanups()

	if !bodyRan {
		t.Fatal("outer cleanup did not run")
	}
	if last := fake.LastFatal(); last != "from-reentrant" {
		t.Errorf("LastFatal() = %q, want %q (cleanup-side Fatalf was lost)", last, "from-reentrant")
	}
	// Reentrant Cleanup should be recorded but NOT auto-fired by the
	// outer RunCleanups (its slice was already drained). Test the
	// idempotency-respecting semantics by re-RunCleanups; it MUST
	// fire exactly once, the new cleanup.
	fake.RunCleanups()
	if !reentrantRan {
		t.Error("reentrant cleanup did not run on the follow-up RunCleanups")
	}
}

// TestFakeT_ConcurrentInterleavedWritesAndReads is the load-bearing
// concurrency test for FakeT.mu. Earlier versions of this test forked
// two separate fakes (one for Fatalf, one for Cleanup) — never tripping
// the mutex on interleaved writes against a SINGLE fake, because the
// race lived between the two halves of the test.
//
// This rewrite drives N writer goroutines that EACH call Cleanup +
// Fatalf on the same fake AND a reader goroutine that loops Fatals()
// during the writers' lifetime. Without the mutex, the read-side
// `make([]string, len) + copy` races the write-side append; the race
// detector trips. With the mutex, the test passes cleanly.
//
// To verify the mutex is load-bearing: temporarily comment out the
// f.mu.Lock/Unlock calls in faket.go, run `go test -race
// ./internal/testsupport/ -run ConcurrentInterleavedWritesAndReads` —
// the race detector should fire.
func TestFakeT_ConcurrentInterleavedWritesAndReads(t *testing.T) {
	fake := &FakeT{}
	const writers = 16

	var cleanupFires atomic.Int32

	var writersWG sync.WaitGroup
	writersWG.Add(writers)

	stop := make(chan struct{})
	var readerWG sync.WaitGroup
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = fake.Fatals()
				_ = fake.LastFatal()
			}
		}
	}()

	for i := range writers {
		go func(i int) {
			defer writersWG.Done()
			defer recoverFakeFatal()
			fake.Cleanup(func() { cleanupFires.Add(1) })
			fake.Fatalf("worker-%d", i)
		}(i)
	}

	writersWG.Wait()
	close(stop)
	readerWG.Wait()

	// Verify every writer's Fatalf landed in the slice. The race
	// detector is the actual contract; this count is a sanity floor.
	if got := len(fake.Fatals()); got != writers {
		t.Errorf("Fatals() len = %d, want %d (writers all recorded)", got, writers)
	}

	// Verify every writer's cleanup was registered AND fires on
	// RunCleanups. Without the mutex, a registration could be lost
	// to an append-aliasing race; this assertion is a count-floor
	// for that scenario in addition to the race-detector signal.
	fake.RunCleanups()
	if got := cleanupFires.Load(); got != writers {
		t.Errorf("cleanups fired = %d, want %d", got, writers)
	}
}

// TestFakeT_FatalsReturnsNonNilEmptySlice pins the documented Fatals()
// contract: a fresh FakeT returns a NON-NIL empty slice, not nil. Tests
// using `len(got) == 0` work either way; tests using `got == nil` would
// silently break if this guarantee were ever flipped. The contract is
// load-bearing because reflect.DeepEqual([]string(nil), []string{}) ==
// false — a JSON-encoded test fixture comparison would diverge.
func TestFakeT_FatalsReturnsNonNilEmptySlice(t *testing.T) {
	fake := &FakeT{}
	got := fake.Fatals()
	if got == nil {
		t.Errorf("Fatals() = nil, want non-nil empty slice (documented contract)")
	}
	if len(got) != 0 {
		t.Errorf("Fatals() = %v, want empty before any Fatalf", got)
	}
	if last := fake.LastFatal(); last != "" {
		t.Errorf("LastFatal() = %q, want empty before any Fatalf", last)
	}
}

// TestExpectFakeFatal_FailsSurroundingTBWhenBodyReturns drives
// ExpectFakeFatal with a body that does NOT panic and asserts the
// surrounding TB receives a Fatal call. The earlier version used a
// panic-on-Fatal sink to trip a deferred recover, but the dance was
// theatrical: ExpectFakeFatal already returns normally after invoking
// t.Fatal (testing.T's Fatal calls FailNow → Goexit, but that's the
// real-T path; we just need to observe the call here). The simpler
// test records the args and asserts non-empty.
func TestExpectFakeFatal_FailsSurroundingTBWhenBodyReturns(t *testing.T) {
	sink := &recordingFatalTB{}
	fake := &FakeT{}

	ExpectFakeFatal(sink, fake, func() {
		// Body returns normally — no Fatalf.
	})

	if len(sink.args) == 0 {
		t.Fatal("ExpectFakeFatal did not call sink.Fatal when the body returned without firing")
	}
}

// recordingFatalTB satisfies HelperFatalTB by recording the Fatal
// invocation and returning normally. Lets self-tests observe whether
// ExpectFakeFatal called Fatal without aborting the surrounding test.
type recordingFatalTB struct{ args []any }

func (r *recordingFatalTB) Helper()                {}
func (r *recordingFatalTB) Fatal(args ...any)      { r.args = args }
