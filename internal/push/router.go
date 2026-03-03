// router.go — Push delivery router with sampling → notification → inbox fallback.
package push

import (
	"sync"
)

// SamplingSender sends a sampling/createMessage request to the MCP client.
type SamplingSender interface {
	SendSampling(req SamplingRequest) error
}

// Notifier sends a log-level notification to the MCP client.
type Notifier interface {
	SendNotification(method string, params map[string]any)
}

// Router delivers push events using the best available channel.
type Router struct {
	mu       sync.RWMutex
	inbox    *PushInbox
	sender   SamplingSender
	notifier Notifier
	caps     ClientCapabilities
}

// NewRouter creates a router with the given delivery backends.
func NewRouter(inbox *PushInbox, sender SamplingSender, notifier Notifier, caps ClientCapabilities) *Router {
	return &Router{
		inbox:    inbox,
		sender:   sender,
		notifier: notifier,
		caps:     caps,
	}
}

// DeliveryMethod indicates how a push event was delivered.
type DeliveryMethod string

const (
	DeliveredViaSampling     DeliveryMethod = "sampling"
	DeliveredViaNotification DeliveryMethod = "notification"
	DeliveredViaInbox        DeliveryMethod = "inbox"
)

// DeliveryResult describes how a push event was routed.
type DeliveryResult struct {
	Method DeliveryMethod
}

// DeliverPush routes an event through sampling → notification → inbox.
// Returns the actual delivery method used.
func (r *Router) DeliverPush(ev PushEvent) (DeliveryResult, error) {
	r.mu.RLock()
	caps := r.caps
	r.mu.RUnlock()

	// Try sampling first (richest channel)
	if caps.SupportsSampling && r.sender != nil {
		req := BuildSamplingRequest(ev)
		if err := r.sender.SendSampling(req); err == nil {
			return DeliveryResult{Method: DeliveredViaSampling}, nil
		}
		// Fall through to notification
	}

	return r.notifyAndQueue(ev), nil
}

// DeliverPushWithRequest routes a pre-built sampling request, falling back to notification/inbox.
// Use this when you need to track the request ID for response correlation.
// Returns the actual delivery method used so the caller can detect sampling failures.
func (r *Router) DeliverPushWithRequest(ev PushEvent, req SamplingRequest) (DeliveryResult, error) {
	r.mu.RLock()
	caps := r.caps
	r.mu.RUnlock()

	// Try sampling first with the pre-built request
	if caps.SupportsSampling && r.sender != nil {
		if err := r.sender.SendSampling(req); err == nil {
			return DeliveryResult{Method: DeliveredViaSampling}, nil
		}
	}

	return r.notifyAndQueue(ev), nil
}

// notifyAndQueue sends a notification (if available) and queues in inbox.
func (r *Router) notifyAndQueue(ev PushEvent) DeliveryResult {
	r.mu.RLock()
	caps := r.caps
	r.mu.RUnlock()

	notified := false
	if caps.SupportsNotifications && r.notifier != nil {
		r.notifier.SendNotification("notifications/message", map[string]any{
			"level":  "info",
			"logger": "gasoline-push",
			"data": map[string]any{
				"type":     ev.Type,
				"page_url": ev.PageURL,
				"message":  "New " + ev.Type + " push from browser",
			},
		})
		notified = true
	}

	r.inbox.Enqueue(ev)
	if notified {
		return DeliveryResult{Method: DeliveredViaNotification}
	}
	return DeliveryResult{Method: DeliveredViaInbox}
}

// UpdateCapabilities updates the client's delivery capabilities.
func (r *Router) UpdateCapabilities(caps ClientCapabilities) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.caps = caps
}

// GetCapabilities returns the current client capabilities.
func (r *Router) GetCapabilities() ClientCapabilities {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.caps
}
