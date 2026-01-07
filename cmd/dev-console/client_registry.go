// client_registry.go — Multi-client session management.
// Tracks connected MCP clients, their buffer cursors, and per-client state isolation.
// Supports up to 10 concurrent clients with LRU eviction of idle clients.
// Thread-safe: all access guarded by RWMutex (see LOCKING.md for ordering).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	maxClients         = 10               // Maximum concurrent clients before LRU eviction
	clientIdleTimeout  = 30 * time.Minute // Clients inactive for this long may be evicted
	clientIDLength     = 12               // Length of derived client ID (hex chars)
)

// ============================================
// BufferCursor
// ============================================

// BufferCursor tracks a client's read position in a ring buffer.
// The timestamp allows detecting when the buffer has wrapped and
// the position is no longer valid (all data at that position has been evicted).
type BufferCursor struct {
	Position  int64     // Monotonic position in the buffer (total items ever added)
	Timestamp time.Time // When this position was last valid
}

// ============================================
// ClientState
// ============================================

// ClientState holds per-client isolated state.
// Each client gets its own cursors, checkpoint namespace, and noise rules.
type ClientState struct {
	mu sync.RWMutex

	// Client identification
	ID        string    // SHA256(CWD)[:12]
	CWD       string    // Original working directory
	CreatedAt time.Time // When client first connected
	LastSeenAt time.Time // Last activity (for LRU eviction)

	// Buffer cursors - track where each client has read up to
	WSEventCursor      BufferCursor
	NetworkBodyCursor  BufferCursor
	EnhancedActionCursor BufferCursor

	// Per-client checkpoint namespace prefix (clientId + ":")
	// Checkpoints are stored as "clientId:checkpointName" in the global store
	CheckpointPrefix string
}

// NewClientState creates a new client state for the given CWD.
func NewClientState(cwd string) *ClientState {
	now := time.Now()
	id := DeriveClientID(cwd)
	return &ClientState{
		ID:               id,
		CWD:              cwd,
		CreatedAt:        now,
		LastSeenAt:       now,
		CheckpointPrefix: id + ":",
	}
}

// Touch updates LastSeenAt to current time.
func (cs *ClientState) Touch() {
	cs.mu.Lock()
	cs.LastSeenAt = time.Now()
	cs.mu.Unlock()
}

// GetLastSeen returns when this client was last active.
func (cs *ClientState) GetLastSeen() time.Time {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.LastSeenAt
}

// UpdateWSCursor updates the WebSocket event cursor.
func (cs *ClientState) UpdateWSCursor(cursor BufferCursor) {
	cs.mu.Lock()
	cs.WSEventCursor = cursor
	cs.mu.Unlock()
}

// GetWSCursor returns the current WebSocket event cursor.
func (cs *ClientState) GetWSCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.WSEventCursor
}

// UpdateNetworkCursor updates the network body cursor.
func (cs *ClientState) UpdateNetworkCursor(cursor BufferCursor) {
	cs.mu.Lock()
	cs.NetworkBodyCursor = cursor
	cs.mu.Unlock()
}

// GetNetworkCursor returns the current network body cursor.
func (cs *ClientState) GetNetworkCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.NetworkBodyCursor
}

// UpdateActionCursor updates the enhanced action cursor.
func (cs *ClientState) UpdateActionCursor(cursor BufferCursor) {
	cs.mu.Lock()
	cs.EnhancedActionCursor = cursor
	cs.mu.Unlock()
}

// GetActionCursor returns the current enhanced action cursor.
func (cs *ClientState) GetActionCursor() BufferCursor {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.EnhancedActionCursor
}

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

	// Check if client already exists
	if cs, exists := r.clients[id]; exists {
		cs.Touch()
		r.moveToEnd(id)
		return cs
	}

	// Evict if at capacity
	if len(r.clients) >= maxClients {
		r.evictOldestLocked()
	}

	// Create new client
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
		// Return a default client state with empty cursors
		return &ClientState{
			ID:               "",
			CWD:              "",
			CreatedAt:        time.Now(),
			LastSeenAt:       time.Now(),
			CheckpointPrefix: "", // No prefix means global namespace
		}
	}
	if cs := r.Get(id); cs != nil {
		return cs
	}
	// Client ID provided but not found - return default
	return &ClientState{
		ID:               id,
		CWD:              "",
		CreatedAt:        time.Now(),
		LastSeenAt:       time.Now(),
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
	// Copy to new slice to allow GC of evicted string entry
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

// ============================================
// Helper Functions
// ============================================

// DeriveClientID generates a stable client ID from the working directory.
// Uses SHA256 hash truncated to 12 hex characters for uniqueness while
// remaining human-readable.
func DeriveClientID(cwd string) string {
	if cwd == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(cwd))
	return hex.EncodeToString(hash[:])[:clientIDLength]
}
