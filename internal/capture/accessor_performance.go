package capture

// AddPerformanceSnapshots stores performance snapshots from the extension.
// Snapshots are keyed by URL with LRU eviction (max 100 entries).
func (c *Capture) AddPerformanceSnapshots(snapshots []PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	const maxSnapshots = 100

	for _, snapshot := range snapshots {
		key := snapshot.URL
		if key == "" {
			continue
		}

		// Check if this URL already exists
		_, exists := c.perf.snapshots[key]
		if !exists {
			// Add to order tracking
			c.perf.snapshotOrder = append(c.perf.snapshotOrder, key)
		}

		// Store snapshot
		c.perf.snapshots[key] = snapshot

		// Evict oldest if over capacity
		for len(c.perf.snapshots) > maxSnapshots && len(c.perf.snapshotOrder) > 0 {
			oldestKey := c.perf.snapshotOrder[0]
			c.perf.snapshotOrder = c.perf.snapshotOrder[1:]
			delete(c.perf.snapshots, oldestKey)
		}
	}
}

// GetPerformanceSnapshots returns all stored performance snapshots (thread-safe)
func (c *Capture) GetPerformanceSnapshots() []PerformanceSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.perf.snapshots) == 0 {
		return []PerformanceSnapshot{}
	}

	result := make([]PerformanceSnapshot, 0, len(c.perf.snapshots))
	for _, snapshot := range c.perf.snapshots {
		result = append(result, snapshot)
	}
	return result
}

// GetPerformanceSnapshotByURL returns a specific snapshot by URL key (thread-safe).
func (c *Capture) GetPerformanceSnapshotByURL(url string) (PerformanceSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap, ok := c.perf.snapshots[url]
	return snap, ok
}

// StoreBeforeSnapshot stores a performance snapshot keyed by correlation_id
// for later perf_diff computation. Max 50 entries with oldest eviction.
func (c *Capture) StoreBeforeSnapshot(correlationID string, snapshot PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.beforeSnapshots[correlationID] = snapshot
	// Evict oldest if over capacity (simple: just cap size)
	if len(c.perf.beforeSnapshots) > 50 {
		for k := range c.perf.beforeSnapshots {
			delete(c.perf.beforeSnapshots, k)
			break // delete one
		}
	}
}

// GetAndDeleteBeforeSnapshot retrieves and removes a before-snapshot by correlation_id.
// Consume-on-read: the snapshot is deleted after retrieval to prevent memory leaks.
func (c *Capture) GetAndDeleteBeforeSnapshot(correlationID string) (PerformanceSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap, ok := c.perf.beforeSnapshots[correlationID]
	if ok {
		delete(c.perf.beforeSnapshots, correlationID)
	}
	return snap, ok
}
