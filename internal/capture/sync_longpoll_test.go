package capture

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

func TestHandleSync_LongPolling(t *testing.T) {
	cap := NewCapture()
	
	// Start a goroutine that will queue a command after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		cap.CreatePendingQuery(queries.PendingQuery{
			Type: "test_cmd",
			Params: json.RawMessage(`{"foo":"bar"}`),
		})
	}()

	reqBody, _ := json.Marshal(SyncRequest{SessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	start := time.Now()
	cap.HandleSync(w, req)
	duration := time.Since(start)

	if duration < 400*time.Millisecond {
		t.Errorf("Sync returned too fast (%v), long-polling should have waited for command", duration)
	}

	var resp SyncResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(resp.Commands) != 1 {
		t.Errorf("Expected 1 command, got %d", len(resp.Commands))
	}
}

func TestHandleSync_TimeoutIfNoCommand(t *testing.T) {
	cap := NewCapture()
	
	reqBody, _ := json.Marshal(SyncRequest{SessionID: "test"})
	req := httptest.NewRequest("POST", "/sync", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	start := time.Now()
	cap.HandleSync(w, req) // Should wait ~5s
	duration := time.Since(start)

	if duration < 4*time.Second {
		t.Errorf("Sync timeout too short (%v), expected ~5s", duration)
	}

	var resp SyncResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Commands) != 0 {
		t.Errorf("Expected 0 commands, got %d", len(resp.Commands))
	}
}
