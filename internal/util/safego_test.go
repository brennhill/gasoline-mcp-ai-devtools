// safego_test.go — Tests for SafeGo panic recovery wrapper.
package util

import (
	"sync"
	"testing"
	"time"
)

func TestSafeGoNormalExecution(t *testing.T) {
	var done sync.WaitGroup
	done.Add(1)
	executed := false

	SafeGo(func() {
		executed = true
		done.Done()
	})

	done.Wait()
	if !executed {
		t.Error("SafeGo did not execute the function")
	}
}

func TestSafeGoPanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	SafeGo(func() {
		defer func() { recovered <- true }()
		panic("test panic")
	})

	select {
	case <-recovered:
		// Goroutine survived the panic — success
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo goroutine did not recover from panic within timeout")
	}
}

func TestSafeGoNilPanicRecovery(t *testing.T) {
	recovered := make(chan bool, 1)

	SafeGo(func() {
		defer func() { recovered <- true }()
		panic(nil)
	})

	select {
	case <-recovered:
		// Goroutine survived nil panic — success
	case <-time.After(2 * time.Second):
		t.Fatal("SafeGo goroutine did not recover from nil panic within timeout")
	}
}
