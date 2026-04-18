// bridge_push_relay.go -- Polls daemon push inbox and relays events to Claude via stdio.
// Why: Bridge and daemon are separate processes; the daemon cannot write to the bridge's stdout.
// This goroutine bridges the gap by polling /push/drain and emitting sampling/createMessage requests.

package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	internbridge "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/bridge"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

const (
	pushRelayPollInterval = 500 * time.Millisecond
	pushRelayPollTimeout  = 3 * time.Second
)

// startBridgePushRelay starts a goroutine that polls the daemon's /push/drain endpoint
// and relays events to Claude Code via MCP sampling/createMessage or notifications.
// Stops when the done channel is closed (bridge shutdown).
func startBridgePushRelay(client *http.Client, endpoint string, done <-chan struct{}) {
	go func() { // lint:allow-bare-goroutine — lifecycle-tied to done channel
		ticker := time.NewTicker(pushRelayPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				relayPendingPushEvents(client, endpoint)
			}
		}
	}()
}

// relayPendingPushEvents fetches and relays any pending push events from the daemon.
func relayPendingPushEvents(client *http.Client, endpoint string) {
	ctx, cancel := context.WithTimeout(context.Background(), pushRelayPollTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/push/drain", nil)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return // daemon unreachable — will retry next tick
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var drain struct {
		Events []push.PushEvent `json:"events"`
		Count  int              `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&drain); err != nil || drain.Count == 0 {
		return
	}

	framing := deps.GetBridgeFraming()
	for i := range drain.Events {
		relayPushEvent(drain.Events[i], framing)
	}
}

// relayPushEvent sends a single push event to Claude via MCP sampling/createMessage.
func relayPushEvent(ev push.PushEvent, framing internbridge.StdioFraming) {
	samplingReq := push.BuildSamplingRequest(ev)
	payload, err := json.Marshal(samplingReq)
	if err != nil {
		return
	}
	deps.WriteMCPPayload(payload, framing)
	deps.Debugf("push relay: sent %s event (page=%s)", ev.Type, ev.PageURL)
}

