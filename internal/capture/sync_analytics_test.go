// sync_analytics_test.go — Tests for analytics fields in sync protocol:
// features_used callback, install_id in response.

package capture

import (
	"net/http"
	"sync"
	"testing"
)

func TestHandleSync_FeaturesUsedInvokesCallback(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	var mu sync.Mutex
	var callbackInvoked bool
	var receivedFeatures map[string]bool

	cap.SetFeaturesCallback(func(features map[string]bool) {
		mu.Lock()
		callbackInvoked = true
		receivedFeatures = features
		mu.Unlock()
	})

	req := SyncRequest{
		ExtSessionID: "analytics_test",
		FeaturesUsed: map[string]bool{
			"screenshot":  true,
			"annotations": true,
			"video":       false,
		},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if !callbackInvoked {
		t.Fatal("Expected featuresCallback to be invoked")
	}
	if !receivedFeatures["screenshot"] {
		t.Error("Expected screenshot=true in callback")
	}
	if !receivedFeatures["annotations"] {
		t.Error("Expected annotations=true in callback")
	}
	if receivedFeatures["video"] {
		t.Error("Expected video=false in callback")
	}
}

func TestHandleSync_FeaturesUsedEmpty_NoCallback(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	var mu sync.Mutex
	var callbackInvoked bool
	cap.SetFeaturesCallback(func(_ map[string]bool) {
		mu.Lock()
		callbackInvoked = true
		mu.Unlock()
	})

	req := SyncRequest{
		ExtSessionID: "analytics_test_empty",
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if callbackInvoked {
		t.Error("Callback should not be invoked when features_used is empty")
	}
}

func TestHandleSync_FeaturesUsedNoCallback_NoPanic(t *testing.T) {
	t.Parallel()
	cap := NewCapture()
	// No callback set — should not panic.

	req := SyncRequest{
		ExtSessionID: "analytics_test_no_cb",
		FeaturesUsed: map[string]bool{"screenshot": true},
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSync_ResponseContainsInstallID(t *testing.T) {
	t.Parallel()
	cap := NewCapture()

	req := SyncRequest{
		ExtSessionID: "install_id_test",
	}

	w := runSyncRequest(t, cap, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	resp := decodeSyncResponse(t, w)
	if resp.InstallID == "" {
		t.Error("Expected install_id to be non-empty in sync response")
	}
}
