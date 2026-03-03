package capture

// AddPerformanceSnapshots stores performance snapshots from the extension.
// Snapshots are keyed by URL with LRU eviction (max 100 entries).
func (c *Capture) AddPerformanceSnapshots(snapshots []PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.appendSnapshots(snapshots)
}

// GetPerformanceSnapshots returns all stored performance snapshots (thread-safe)
func (c *Capture) GetPerformanceSnapshots() []PerformanceSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotsList()
}

// GetPerformanceSnapshotByURL returns a specific snapshot by URL key (thread-safe).
func (c *Capture) GetPerformanceSnapshotByURL(url string) (PerformanceSnapshot, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.perf.snapshotByURL(url)
}

// StoreBeforeSnapshot stores a performance snapshot keyed by correlation_id
// for later perf_diff computation. Max 50 entries with oldest eviction.
func (c *Capture) StoreBeforeSnapshot(correlationID string, snapshot PerformanceSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.perf.storeBeforeSnapshot(correlationID, snapshot)
}

// GetAndDeleteBeforeSnapshot retrieves and removes a before-snapshot by correlation_id.
// Consume-on-read: the snapshot is deleted after retrieval to prevent memory leaks.
func (c *Capture) GetAndDeleteBeforeSnapshot(correlationID string) (PerformanceSnapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.perf.takeBeforeSnapshot(correlationID)
}
