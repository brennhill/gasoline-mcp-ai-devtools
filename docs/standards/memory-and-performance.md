# Memory and Performance Standards

> **Memory safety, performance optimization, and Gasoline-specific 3-tier memory limits**

**Scope:** Memory management, performance requirements, ring buffers, and Gasoline's specific memory constraints as a browser extension.

**Related Standards:**
- [data-design.md](data-design.md) — Data structure design
- [error-and-recovery.md](error-and-recovery.md) — Resource management and cleanup
- [code-quality.md](code-quality.md) — Code organization

---

## ⚡ Performance

### Complexity Analysis

- ✅ **Document time complexity:** O(n), O(n log n), etc.
- ✅ **Document space complexity:** Memory usage characteristics
- ✅ **Avoid O(n²) in hot paths:** Use better algorithms

### Memory Management

- ✅ **Minimize allocations:** Reuse buffers, pre-allocate slices
- ✅ **Avoid string concatenation in loops:** Use strings.Builder
- ✅ **Pool expensive objects:** sync.Pool for frequently allocated objects
- ✅ **Bounded data structures:** All buffers/queues have max capacity

### Performance Requirements

- ✅ **Define performance budgets:**
  ```go
  // Performance: Must complete in < 0.5ms for typical request (< 10KB)
  // Memory: Allocates ~200 bytes per entry
  ```
- ✅ **Add benchmarks for hot paths:**
  ```go
  func BenchmarkAddNetworkBodies(b *testing.B) {
      // Benchmark implementation
  }
  ```
- ✅ **Track performance:** Compare benchmarks before/after changes

---

## 🛡️ Memory Safety (Gasoline-Specific)

### Ring Buffer Safety (Critical for Gasoline)

- ✅ **All buffers have capacity limits:**
  ```go
  const (
      MaxWebSocketEvents = 500
      MaxNetworkBodies = 100
      MaxActions = 500
  )

  // Initialize with explicit capacity
  wsEvents := make([]WebSocketEvent, 0, MaxWebSocketEvents)
  ```

- ✅ **Parallel arrays stay in sync:** Add defensive length checks
  ```go
  // Before adding to parallel arrays, verify they match
  if len(c.wsEvents) != len(c.wsAddedAt) {
      fmt.Fprintf(os.Stderr, "WARNING: wsEvents/wsAddedAt mismatch: %d != %d\n",
          len(c.wsEvents), len(c.wsAddedAt))
      // Recover by truncating to shorter length
      minLen := min(len(c.wsEvents), len(c.wsAddedAt))
      c.wsEvents = c.wsEvents[:minLen]
      c.wsAddedAt = c.wsAddedAt[:minLen]
  }
  ```

- ✅ **Ring buffer eviction strategy documented:**
  - FIFO (oldest first) for fairness
  - Document eviction trigger (capacity exceeded)
  - Log warnings when evicting

### Memory Limits (Gasoline 3-Tier System)

```go
const (
    SoftMemoryLimit     = 50 * 1024 * 1024  // 50MB - Evict 25%
    HardMemoryLimit     = 100 * 1024 * 1024 // 100MB - Evict 50%
    CriticalMemoryLimit = 150 * 1024 * 1024 // 150MB - Clear all
)

// Check memory before adding data
if c.totalMemory > CriticalMemoryLimit {
    c.clearAll() // Enter minimal mode
} else if c.totalMemory > HardMemoryLimit {
    c.evictPercent(50) // Aggressive eviction
} else if c.totalMemory > SoftMemoryLimit {
    c.evictPercent(25) // Normal eviction
}
```

The 3-tier system ensures:
1. **Soft limit** (50MB): Gentle eviction keeps memory manageable
2. **Hard limit** (100MB): Aggressive eviction when approaching danger
3. **Critical limit** (150MB): Full clear to prevent crashes

### String Truncation (Prevent Unbounded Growth)

- ✅ **Truncate large strings:** Max 10KB for captured data
  ```go
  const MaxStringLength = 10 * 1024 // 10KB

  if len(data) > MaxStringLength {
      data = data[:MaxStringLength] + "...[truncated]"
      wasTruncated = true
  }
  ```

- ✅ **Document truncation:** Include flag in data structure
  ```go
  type NetworkBody struct {
      ResponseBody string `json:"response_body"`
      ResponseTruncated bool `json:"response_truncated"` // True if body was truncated
  }
  ```

### Memory Leak Prevention

- ✅ **Clear references when done:**
  ```go
  // Bad - keeps reference
  oldEvents := c.wsEvents
  c.wsEvents = newEvents

  // Good - clears reference for GC
  c.wsEvents = nil
  c.wsEvents = newEvents
  ```

- ✅ **Close goroutine channels:** Prevent goroutine leaks
- ✅ **Profile regularly:** Run `go test -memprofile=mem.prof`
- ✅ **Monitor memory usage:** Track in `/diagnostics` endpoint

### Slice Management

- ✅ **Pre-allocate when size known:**
  ```go
  // Good - pre-allocate
  results := make([]Entry, 0, expectedSize)

  // Bad - grows dynamically
  results := []Entry{}
  ```

- ✅ **Avoid slice leaks:**
  ```go
  // When slicing, consider making copy if original is large
  subset := make([]Event, len(events)-100)
  copy(subset, events[100:]) // Don't hold reference to large original
  ```

### Map Cleanup

- ✅ **Delete old entries:**
  ```go
  // Cleanup TTL-expired entries
  for id, entry := range c.queryResults {
      if time.Since(entry.createdAt) > queryTTL {
          delete(c.queryResults, id) // Free memory
      }
  }
  ```

- ✅ **Bounded map sizes:** Enforce max entries, LRU eviction

### Defensive Copying

- ✅ **Copy when ownership unclear:**
  ```go
  // If caller might modify the slice after passing it
  func AddEvents(events []Event) {
      c.mu.Lock()
      defer c.mu.Unlock()

      // Defensive copy - caller can't mutate our data
      for _, event := range events {
          c.events = append(c.events, event) // Copy each event
      }
  }
  ```

- ✅ **Document ownership:** Comment who owns the data after function returns

---

**Last updated:** 2026-02-03
**See also:** [README.md](README.md) — Navigation and index

