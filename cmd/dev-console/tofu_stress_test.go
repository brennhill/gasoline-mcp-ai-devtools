// tofu_stress_test.go — Concurrent stress tests for TOFU extension pairing.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestStressTOFUConcurrentPairing verifies that concurrent TOFU pairing attempts
// result in exactly one winner due to proper mutex usage. Launches 100 goroutines
// that all attempt to pair different extension IDs simultaneously. Only the first
// one to acquire the lock should succeed in pairing.
//
// Designed to be run with -race to detect data races in the TOFU pairing system.
func TestStressTOFUConcurrentPairing(t *testing.T) {
	t.Run("concurrent_pairing_chrome", func(t *testing.T) {
		const numGoroutines = 100

		// Use a temporary directory for the trust file
		tmpDir := t.TempDir()
		settingsDir := filepath.Join(tmpDir, "settings")
		if err := os.MkdirAll(settingsDir, 0o750); err != nil {
			t.Fatalf("Failed to create temp settings dir: %v", err)
		}

		// Override HOME to use temp directory (state.InRoot uses HOME)
		t.Setenv("HOME", tmpDir)

		// Reset trust state before test
		extensionTrustMu.Lock()
		extensionTrustLoaded = false
		extensionTrust = extensionTrustConfig{}
		extensionTrustMu.Unlock()

		var wg sync.WaitGroup
		results := make([]bool, numGoroutines)

		// Launch concurrent pairing attempts
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Each goroutine tries to pair a unique extension ID
				id := fmt.Sprintf("ext-id-%d-abcdefghijklmnop", idx)
				results[idx] = checkOrPairExtensionID(id, true)
			}(i)
		}

		wg.Wait()

		// Count winners: exactly one should have paired successfully
		winners := 0
		for _, ok := range results {
			if ok {
				winners++
			}
		}

		// Only the first goroutine to acquire the lock should have paired.
		// All others should be rejected because the paired ID doesn't match theirs.
		if winners != 1 {
			t.Errorf("Expected exactly 1 TOFU winner, got %d", winners)
		}

		// Verify that the trust file was written
		extensionTrustMu.Lock()
		pairedID := extensionTrust.ChromeID
		extensionTrustMu.Unlock()

		if pairedID == "" {
			t.Error("No Chrome extension ID was paired")
		}

		t.Logf("TOFU stress test completed: %d goroutines, 1 winner (paired ID: %s)", numGoroutines, pairedID)
	})
}

// TestStressTOFUMixedBrowsers verifies concurrent pairing for both Chrome and Firefox.
func TestStressTOFUMixedBrowsers(t *testing.T) {
	t.Run("concurrent_mixed_browsers", func(t *testing.T) {
		const numChromeGoroutines = 50
		const numFirefoxGoroutines = 50

		// Use a temporary directory for the trust file
		tmpDir := t.TempDir()
		settingsDir := filepath.Join(tmpDir, "settings")
		if err := os.MkdirAll(settingsDir, 0o750); err != nil {
			t.Fatalf("Failed to create temp settings dir: %v", err)
		}

		t.Setenv("HOME", tmpDir)

		// Reset trust state
		extensionTrustMu.Lock()
		extensionTrustLoaded = false
		extensionTrust = extensionTrustConfig{}
		extensionTrustMu.Unlock()

		var wg sync.WaitGroup
		chromeResults := make([]bool, numChromeGoroutines)
		firefoxResults := make([]bool, numFirefoxGoroutines)

		// Launch Chrome pairing attempts
		for i := 0; i < numChromeGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				id := fmt.Sprintf("chrome-ext-%d-abcdefghijklmnop", idx)
				chromeResults[idx] = checkOrPairExtensionID(id, true)
			}(i)
		}

		// Launch Firefox pairing attempts
		for i := 0; i < numFirefoxGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				id := fmt.Sprintf("firefox-ext-%d-abcdefghijklmnop", idx)
				firefoxResults[idx] = checkOrPairExtensionID(id, false)
			}(i)
		}

		wg.Wait()

		// Count winners for each browser
		chromeWinners := 0
		for _, ok := range chromeResults {
			if ok {
				chromeWinners++
			}
		}

		firefoxWinners := 0
		for _, ok := range firefoxResults {
			if ok {
				firefoxWinners++
			}
		}

		// Each browser family should have exactly one winner
		if chromeWinners != 1 {
			t.Errorf("Expected exactly 1 Chrome TOFU winner, got %d", chromeWinners)
		}
		if firefoxWinners != 1 {
			t.Errorf("Expected exactly 1 Firefox TOFU winner, got %d", firefoxWinners)
		}

		// Verify both IDs were paired
		extensionTrustMu.Lock()
		chromeID := extensionTrust.ChromeID
		firefoxID := extensionTrust.FirefoxID
		extensionTrustMu.Unlock()

		if chromeID == "" {
			t.Error("No Chrome extension ID was paired")
		}
		if firefoxID == "" {
			t.Error("No Firefox extension ID was paired")
		}

		t.Logf("Mixed browser stress test: Chrome winner: %s, Firefox winner: %s", chromeID, firefoxID)
	})
}

// TestStressTOFURepeatedPairing verifies that once an ID is paired, repeated calls
// with the same ID succeed, while different IDs are rejected.
func TestStressTOFURepeatedPairing(t *testing.T) {
	t.Run("repeated_pairing_stress", func(t *testing.T) {
		const numRepeats = 100
		const numRejected = 50

		tmpDir := t.TempDir()
		settingsDir := filepath.Join(tmpDir, "settings")
		if err := os.MkdirAll(settingsDir, 0o750); err != nil {
			t.Fatalf("Failed to create temp settings dir: %v", err)
		}

		t.Setenv("HOME", tmpDir)

		// Reset trust state
		extensionTrustMu.Lock()
		extensionTrustLoaded = false
		extensionTrust = extensionTrustConfig{}
		extensionTrustMu.Unlock()

		// First pairing
		pairedID := "paired-extension-id-abc123"
		if !checkOrPairExtensionID(pairedID, true) {
			t.Fatal("Initial pairing should succeed")
		}

		var wg sync.WaitGroup
		successResults := make([]bool, numRepeats)
		rejectResults := make([]bool, numRejected)

		// Launch goroutines with the paired ID (should all succeed)
		for i := 0; i < numRepeats; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				successResults[idx] = checkOrPairExtensionID(pairedID, true)
			}(i)
		}

		// Launch goroutines with different IDs (should all be rejected)
		for i := 0; i < numRejected; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				differentID := fmt.Sprintf("different-ext-%d-xyz789", idx)
				rejectResults[idx] = checkOrPairExtensionID(differentID, true)
			}(i)
		}

		wg.Wait()

		// All paired ID calls should succeed
		successCount := 0
		for _, ok := range successResults {
			if ok {
				successCount++
			}
		}
		if successCount != numRepeats {
			t.Errorf("Expected all %d paired ID calls to succeed, got %d", numRepeats, successCount)
		}

		// All different ID calls should be rejected
		rejectCount := 0
		for _, ok := range rejectResults {
			if !ok {
				rejectCount++
			}
		}
		if rejectCount != numRejected {
			t.Errorf("Expected all %d different ID calls to be rejected, got %d", numRejected, rejectCount)
		}

		t.Logf("Repeated pairing stress test: %d/%d paired ID successes, %d/%d different ID rejections",
			successCount, numRepeats, rejectCount, numRejected)
	})
}

// TestStressTOFUFileSystemRace verifies that concurrent file I/O during TOFU pairing
// doesn't cause corruption or data races.
func TestStressTOFUFileSystemRace(t *testing.T) {
	t.Run("filesystem_race", func(t *testing.T) {
		const numIterations = 50

		tmpDir := t.TempDir()
		settingsDir := filepath.Join(tmpDir, "settings")
		if err := os.MkdirAll(settingsDir, 0o750); err != nil {
			t.Fatalf("Failed to create temp settings dir: %v", err)
		}

		t.Setenv("HOME", tmpDir)

		// Each iteration resets and pairs
		for i := 0; i < numIterations; i++ {
			// Reset trust state AND delete persisted trust file so
			// loadExtensionTrustLocked won't reload a stale winner.
			trustFile := filepath.Join(tmpDir, ".gasoline", "settings", trustedExtensionFileName)
			os.Remove(trustFile)

			extensionTrustMu.Lock()
			extensionTrustLoaded = false
			extensionTrust = extensionTrustConfig{}
			extensionTrustMu.Unlock()

			var wg sync.WaitGroup
			results := make([]bool, 20)

			// Launch concurrent pairing attempts
			for j := 0; j < 20; j++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					id := fmt.Sprintf("iter-%d-ext-%d-abc", i, idx)
					results[idx] = checkOrPairExtensionID(id, true)
				}(j)
			}

			wg.Wait()

			// Verify exactly one winner
			winners := 0
			for _, ok := range results {
				if ok {
					winners++
				}
			}
			if winners != 1 {
				t.Errorf("Iteration %d: expected 1 winner, got %d", i, winners)
			}
		}

		t.Logf("Filesystem race test completed: %d iterations", numIterations)
	})
}
