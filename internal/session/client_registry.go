// Purpose: Tracks connected MCP clients and registry-level LRU ordering.
// Why: Keeps registry operations isolated from per-client cursor/state methods.
// Docs: docs/features/feature/request-session-correlation/index.md

// client_registry.go — Multi-client session registry management.
// Tracks connected MCP clients with LRU eviction and lookup behavior.
// Thread-safe: all access guarded by RWMutex (see LOCKING.md for ordering).
package session

import (
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxClients        = 10               // Maximum concurrent clients before LRU eviction
	clientIdleTimeout = 30 * time.Minute // Clients inactive for this long may be evicted
	clientIDLength    = 12               // Length of derived client ID (hex chars)
)

// ============================================
// ClientRegistry
// ============================================

// ClientRegistry manages all connected MCP clients.
// Thread-safe with RWMutex. Lock ordering: ClientRegistry.mu before ClientState.mu.
type ClientRegistry struct {
	mu      sync.RWMutex
	clients map[string]*ClientState
	// Track access order for LRU eviction
	accessOrder []string
}

// NewClientRegistry creates a new empty client registry.
func NewClientRegistry() *ClientRegistry {
	return &ClientRegistry{
		clients:     make(map[string]*ClientState),
		accessOrder: make([]string, 0, maxClients),
	}
}

// Register adds or updates a client. Returns the client state.
// If client already exists, updates LastSeenAt and returns existing state.
// If at capacity, evicts the least recently used client first.
func (r *ClientRegistry) Register(cwd string) *ClientState {
	id := DeriveClientID(cwd)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if client already exists.
	if cs, exists := r.clients[id]; exists {
		cs.Touch()
		r.moveToEnd(id)
		return cs
	}

	// Evict if at capacity.
	if len(r.clients) >= maxClients {
		r.evictOldestLocked()
	}

	// Create new client.
	cs := NewClientState(cwd)
	r.clients[id] = cs
	r.accessOrder = append(r.accessOrder, id)
	return cs
}

// Get retrieves a client by ID. Returns nil if not found.
// Updates LastSeenAt if found.
func (r *ClientRegistry) Get(id string) *ClientState {
	r.mu.Lock()
	defer r.mu.Unlock()

	cs, exists := r.clients[id]
	if !exists {
		return nil
	}
	cs.Touch()
	r.moveToEnd(id)
	return cs
}

// GetOrDefault retrieves a client by ID, or returns a default state if not found.
// The default state is not registered and has empty cursors.
// This is useful for backwards compatibility when no client ID is provided.
func (r *ClientRegistry) GetOrDefault(id string) *ClientState {
	if id == "" {
		// Return a default client state with empty cursors.
		now := time.Now()
		return &ClientState{
			ID:               "",
			CWD:              "",
			CreatedAt:        now,
			LastSeenAt:       now,
			CheckpointPrefix: "", // No prefix means global namespace.
		}
	}
	if cs := r.Get(id); cs != nil {
		return cs
	}
	// Client ID provided but not found - return default.
	now := time.Now()
	return &ClientState{
		ID:               id,
		CWD:              "",
		CreatedAt:        now,
		LastSeenAt:       now,
		CheckpointPrefix: id + ":",
	}
}

// Unregister removes a client by ID.
func (r *ClientRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.clients, id)
	r.removeFromOrder(id)
}

// List returns information about all registered clients.
func (r *ClientRegistry) List() []ClientInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ClientInfo, 0, len(r.clients))
	for _, cs := range r.clients {
		cs.mu.RLock()
		info := ClientInfo{
			ID:         cs.ID,
			CWD:        cs.CWD,
			CreatedAt:  cs.CreatedAt.Format(time.RFC3339),
			LastSeenAt: cs.LastSeenAt.Format(time.RFC3339),
			IdleFor:    time.Since(cs.LastSeenAt).Round(time.Second).String(),
		}
		cs.mu.RUnlock()
		result = append(result, info)
	}
	return result
}

// Count returns the number of registered clients.
func (r *ClientRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}

// evictOldestLocked removes the least recently used client.
// Must be called with r.mu held.
func (r *ClientRegistry) evictOldestLocked() {
	if len(r.accessOrder) == 0 {
		return
	}
	oldest := r.accessOrder[0]
	delete(r.clients, oldest)
	// Copy to new slice to allow GC of evicted string entry.
	newOrder := make([]string, len(r.accessOrder)-1)
	copy(newOrder, r.accessOrder[1:])
	r.accessOrder = newOrder
}

// moveToEnd moves a client ID to the end of the access order.
// Must be called with r.mu held.
func (r *ClientRegistry) moveToEnd(id string) {
	r.removeFromOrder(id)
	r.accessOrder = append(r.accessOrder, id)
}

// removeFromOrder removes a client ID from the access order slice.
// Must be called with r.mu held.
func (r *ClientRegistry) removeFromOrder(id string) {
	for i, oid := range r.accessOrder {
		if oid == id {
			newOrder := make([]string, len(r.accessOrder)-1)
			copy(newOrder, r.accessOrder[:i])
			copy(newOrder[i:], r.accessOrder[i+1:])
			r.accessOrder = newOrder
			return
		}
	}
}

// ============================================
// ClientInfo
// ============================================

// ClientInfo is the JSON-serializable view of a client for the /clients endpoint.
type ClientInfo struct {
	ID         string `json:"id"`
	CWD        string `json:"cwd"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	IdleFor    string `json:"idle_for"`
}
