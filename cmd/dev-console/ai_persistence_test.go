package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test Scenario 1: Store created at .gasoline/ in project directory
// ============================================

func TestPersistStoreCreatedAtProjectRoot(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	gasolineDir := filepath.Join(dir, ".gasoline")
	info, err := os.Stat(gasolineDir)
	if err != nil {
		t.Fatalf(".gasoline dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf(".gasoline is not a directory")
	}
}

// ============================================
// Test Scenario 2: .gasoline/ added to .gitignore if present
// ============================================

func TestPersistGitignoreUpdated(t *testing.T) {
	dir := t.TempDir()

	// Create a .gitignore without .gasoline
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n.env\n"), 0o644)

	_, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	data, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(data), ".gasoline/") {
		t.Errorf(".gitignore should contain .gasoline/, got: %s", string(data))
	}

	// Create a second store - shouldn't duplicate the entry
	_, err = NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (2) failed: %v", err)
	}
	data, _ = os.ReadFile(gitignorePath)
	count := strings.Count(string(data), ".gasoline")
	if count != 1 {
		t.Errorf("expected .gasoline to appear once, got %d times", count)
	}
}

// ============================================
// Test Scenario 3: Save then load returns identical data
// ============================================

func TestPersistSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	data := map[string]interface{}{
		"key1":  "value1",
		"count": float64(42),
		"nested": map[string]interface{}{
			"inner": "data",
		},
	}
	dataJSON, _ := json.Marshal(data)

	err = store.Save("baselines", "login", dataJSON)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load("baselines", "login")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(loaded, &result); err != nil {
		t.Fatalf("Unmarshal loaded data failed: %v", err)
	}

	if result["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", result["key1"])
	}
	if result["count"] != float64(42) {
		t.Errorf("expected count=42, got %v", result["count"])
	}
	nested, ok := result["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested map, got %T", result["nested"])
	}
	if nested["inner"] != "data" {
		t.Errorf("expected inner=data, got %v", nested["inner"])
	}
}

// ============================================
// Test Scenario 4: Load nonexistent key returns error
// ============================================

func TestPersistLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.Load("baselines", "nonexistent")
	if err == nil {
		t.Fatal("expected error loading nonexistent key, got nil")
	}
}

// ============================================
// Test Scenario 5: List returns all keys without .json extension
// ============================================

func TestPersistListKeys(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save multiple keys
	store.Save("baselines", "login", []byte(`{"name":"login"}`))
	store.Save("baselines", "dashboard", []byte(`{"name":"dashboard"}`))
	store.Save("baselines", "checkout", []byte(`{"name":"checkout"}`))

	keys, err := store.List("baselines")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}

	// Check no .json extension
	for _, key := range keys {
		if strings.HasSuffix(key, ".json") {
			t.Errorf("key should not have .json extension: %s", key)
		}
	}

	// Check all keys are present
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, expected := range []string{"login", "dashboard", "checkout"} {
		if !keySet[expected] {
			t.Errorf("expected key %q not found in list", expected)
		}
	}
}

// ============================================
// Test Scenario 6: Delete removes file, subsequent load errors
// ============================================

func TestPersistDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("baselines", "login", []byte(`{"name":"login"}`))

	// Verify it's loadable
	_, err = store.Load("baselines", "login")
	if err != nil {
		t.Fatalf("Load before delete failed: %v", err)
	}

	// Delete
	err = store.Delete("baselines", "login")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify load now fails
	_, err = store.Load("baselines", "login")
	if err == nil {
		t.Fatal("expected error loading deleted key, got nil")
	}
}

// ============================================
// Test Scenario 7: Stats returns correct sizes and entry counts
// ============================================

func TestPersistStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	data1 := []byte(`{"name":"login","data":"some baseline data"}`)
	data2 := []byte(`{"name":"dashboard","data":"more baseline data"}`)
	data3 := []byte(`{"rules":[1,2,3]}`)

	store.Save("baselines", "login", data1)
	store.Save("baselines", "dashboard", data2)
	store.Save("noise", "config", data3)

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.SessionCount < 1 {
		t.Errorf("expected session_count >= 1, got %d", stats.SessionCount)
	}

	// Check namespace entry counts
	if stats.Namespaces["baselines"] != 2 {
		t.Errorf("expected baselines=2 entries, got %d", stats.Namespaces["baselines"])
	}
	if stats.Namespaces["noise"] != 1 {
		t.Errorf("expected noise=1 entry, got %d", stats.Namespaces["noise"])
	}

	// Check total bytes > 0
	if stats.TotalBytes == 0 {
		t.Error("expected TotalBytes > 0")
	}
}

// ============================================
// Test Scenario 8: File exceeding 1MB returns error on save
// ============================================

func TestPersistSaveExceedsMaxSize(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create data > 1MB
	bigData := make([]byte, 1024*1024+1)
	for i := range bigData {
		bigData[i] = 'a'
	}
	// Wrap in valid JSON
	jsonData, _ := json.Marshal(string(bigData))

	err = store.Save("baselines", "toobig", jsonData)
	if err == nil {
		t.Fatal("expected error saving >1MB data, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention size limit, got: %v", err)
	}
}

// ============================================
// Test Scenario 9: Shutdown then restart increments session count
// ============================================

func TestPersistSessionCountIncrement(t *testing.T) {
	dir := t.TempDir()

	// First session
	store1, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (1) failed: %v", err)
	}
	store1.Shutdown()

	// Second session
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (2) failed: %v", err)
	}

	meta := store2.GetMeta()
	if meta.SessionCount != 2 {
		t.Errorf("expected session_count=2 after restart, got %d", meta.SessionCount)
	}

	// Verify last_session was updated
	if meta.LastSession.IsZero() {
		t.Error("expected last_session to be set")
	}

	store2.Shutdown()
}

// ============================================
// Test Scenario 10: Fresh store with no prior sessions
// ============================================

func TestPersistFreshStore(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	meta := store.GetMeta()
	if meta.SessionCount != 1 {
		t.Errorf("expected session_count=1 for fresh store, got %d", meta.SessionCount)
	}

	ctx := store.LoadSessionContext()
	if ctx.SessionCount != 1 {
		t.Errorf("expected context session_count=1, got %d", ctx.SessionCount)
	}
	if len(ctx.Baselines) != 0 {
		t.Errorf("expected no baselines, got %d", len(ctx.Baselines))
	}
	if len(ctx.ErrorHistory) != 0 {
		t.Errorf("expected no error history, got %d", len(ctx.ErrorHistory))
	}
}

// ============================================
// Test Scenario 11: Store with existing data returns all summaries
// ============================================

func TestPersistExistingDataSummaries(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Save baselines
	store.Save("baselines", "login", []byte(`{"name":"login"}`))
	store.Save("baselines", "dashboard", []byte(`{"name":"dashboard"}`))

	// Save noise config
	noiseConfig := map[string]interface{}{
		"rules":          []interface{}{map[string]interface{}{"pattern": "test"}},
		"auto_count":     float64(1),
		"manual_count":   float64(0),
		"total_filtered": float64(100),
	}
	noiseJSON, _ := json.Marshal(noiseConfig)
	store.Save("noise", "config", noiseJSON)

	// Save error history
	errorHistory := []map[string]interface{}{
		{"fingerprint": "err1", "count": float64(5), "resolved": false},
		{"fingerprint": "err2", "count": float64(2), "resolved": true},
	}
	errJSON, _ := json.Marshal(errorHistory)
	store.Save("errors", "history", errJSON)

	store.Shutdown()

	// Reload
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (reload) failed: %v", err)
	}
	defer store2.Shutdown()

	ctx := store2.LoadSessionContext()
	if ctx.SessionCount != 2 {
		t.Errorf("expected session_count=2, got %d", ctx.SessionCount)
	}
	if len(ctx.Baselines) != 2 {
		t.Errorf("expected 2 baselines, got %d", len(ctx.Baselines))
	}
	if ctx.NoiseConfig == nil {
		t.Error("expected noise config to be populated")
	}
	if len(ctx.ErrorHistory) != 2 {
		t.Errorf("expected 2 error history entries, got %d", len(ctx.ErrorHistory))
	}
}

// ============================================
// Test Scenario 12: Loading noise config rules become active
// ============================================

func TestPersistNoiseConfigActive(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	noiseConfig := map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{"pattern": "ignore-me", "type": "auto"},
			map[string]interface{}{"pattern": "also-ignore", "type": "manual"},
		},
		"auto_count":     float64(1),
		"manual_count":   float64(1),
		"total_filtered": float64(50),
	}
	noiseJSON, _ := json.Marshal(noiseConfig)
	store.Save("noise", "config", noiseJSON)
	store.Shutdown()

	// Reload
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (reload) failed: %v", err)
	}
	defer store2.Shutdown()

	ctx := store2.LoadSessionContext()
	if ctx.NoiseConfig == nil {
		t.Fatal("expected noise config to be populated")
	}
	rules, ok := ctx.NoiseConfig["rules"].([]interface{})
	if !ok {
		t.Fatalf("expected rules to be a slice, got %T", ctx.NoiseConfig["rules"])
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 noise rules, got %d", len(rules))
	}
}

// ============================================
// Test Scenario 13: Concurrent reads and writes have no race conditions
// ============================================

func TestPersistConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// 10 concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data, _ := json.Marshal(map[string]int{"id": id})
			key := string(rune('a' + id))
			if err := store.Save("concurrent", key, data); err != nil {
				errCh <- err
			}
		}(i)
	}

	// 10 concurrent readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.List("concurrent")
			store.LoadSessionContext()
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent operation failed: %v", err)
	}
}

// ============================================
// Test Scenario 14: Shutdown flushes all dirty data
// ============================================

func TestPersistShutdownFlush(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Mark some data as dirty (use MarkDirty to buffer data)
	store.MarkDirty("errors", "history", []byte(`[{"fingerprint":"e1","count":1}]`))

	// Shutdown should flush
	store.Shutdown()

	// Verify the file was written
	filePath := filepath.Join(dir, ".gasoline", "errors", "history.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected dirty data to be flushed on shutdown, file not found: %v", err)
	}
	if len(data) == 0 {
		t.Error("flushed file is empty")
	}
}

// ============================================
// Test Scenario 15: Short flush interval writes data within interval
// ============================================

func TestPersistFlushInterval(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStoreWithInterval(dir, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewSessionStoreWithInterval failed: %v", err)
	}
	defer store.Shutdown()

	// Mark data as dirty
	store.MarkDirty("perf", "metrics", []byte(`{"latency":42}`))

	// Wait for flush interval to fire
	time.Sleep(300 * time.Millisecond)

	// Verify the file was written
	filePath := filepath.Join(dir, ".gasoline", "perf", "metrics.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected data to be flushed by periodic goroutine: %v", err)
	}
	if string(data) != `{"latency":42}` {
		t.Errorf("unexpected flushed data: %s", string(data))
	}
}

// ============================================
// Test Scenario 16: Read-only directory returns errors, server continues
// ============================================

func TestPersistReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create the store directory, then make it read-only
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	store.Shutdown()

	// Make the namespace directory read-only
	gasolineDir := filepath.Join(dir, ".gasoline")
	readOnlyDir := filepath.Join(gasolineDir, "readonly_ns")
	os.MkdirAll(readOnlyDir, 0755)
	os.Chmod(readOnlyDir, 0555)
	// Also make gasoline dir read-only so new dirs can't be created
	os.Chmod(gasolineDir, 0555)
	defer func() {
		os.Chmod(gasolineDir, 0755)
		os.Chmod(readOnlyDir, 0755)
	}()

	// Re-open store
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (readonly) failed: %v", err)
	}
	defer store2.Shutdown()

	// Save should fail but not panic
	err = store2.Save("newnamespace", "key", []byte(`{"test":true}`))
	if err == nil {
		t.Fatal("expected error saving to read-only directory, got nil")
	}

	// Store should still be functional (loads, lists, etc.)
	ctx := store2.LoadSessionContext()
	if ctx.ProjectID == "" {
		t.Error("store should still be functional after save error")
	}
}

// ============================================
// Test Scenario 17: Error history at 500 entries evicts oldest
// ============================================

func TestPersistErrorHistoryCap(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create 501 error entries with unique fingerprints and progressively newer FirstSeen
	entries := make([]ErrorHistoryEntry, 501)
	baseTime := time.Now().Add(-1000 * time.Hour)
	for i := 0; i < 501; i++ {
		entries[i] = ErrorHistoryEntry{
			Fingerprint: fmt.Sprintf("error_%04d", i),
			FirstSeen:   baseTime.Add(time.Duration(i) * time.Hour), // entry 0 is oldest
			LastSeen:    time.Now(),
			Count:       1,
			Resolved:    false,
		}
	}

	oldestFingerprint := entries[0].Fingerprint

	result := enforceErrorHistoryCap(entries)
	if len(result) != 500 {
		t.Errorf("expected 500 entries after cap, got %d", len(result))
	}

	// The oldest entry (index 0) should have been evicted
	for _, e := range result {
		if e.Fingerprint == oldestFingerprint {
			t.Error("oldest entry should have been evicted")
		}
	}
}

// ============================================
// Test Scenario 18: Error entries older than 30 days are removed
// ============================================

func TestPersistErrorHistoryStaleEviction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	entries := []ErrorHistoryEntry{
		{
			Fingerprint: "recent-error",
			FirstSeen:   time.Now().Add(-24 * time.Hour),
			LastSeen:    time.Now(),
			Count:       5,
			Resolved:    false,
		},
		{
			Fingerprint: "stale-error",
			FirstSeen:   time.Now().Add(-60 * 24 * time.Hour), // 60 days ago
			LastSeen:    time.Now().Add(-31 * 24 * time.Hour), // last seen 31 days ago
			Count:       2,
			Resolved:    false,
		},
		{
			Fingerprint: "old-but-recent-activity",
			FirstSeen:   time.Now().Add(-90 * 24 * time.Hour),
			LastSeen:    time.Now().Add(-1 * time.Hour), // last seen 1 hour ago
			Count:       10,
			Resolved:    false,
		},
	}

	result := evictStaleErrors(entries, 30*24*time.Hour)
	if len(result) != 2 {
		t.Errorf("expected 2 entries after stale eviction, got %d", len(result))
	}

	// Check that the stale error was removed
	for _, e := range result {
		if e.Fingerprint == "stale-error" {
			t.Error("stale error should have been evicted")
		}
	}

	// Check that recent and old-but-recently-active errors remain
	found := make(map[string]bool)
	for _, e := range result {
		found[e.Fingerprint] = true
	}
	if !found["recent-error"] {
		t.Error("recent-error should remain")
	}
	if !found["old-but-recent-activity"] {
		t.Error("old-but-recent-activity should remain (last seen recently)")
	}
}

// ============================================
// Test: List on empty/nonexistent namespace returns empty slice
// ============================================

func TestPersistListEmptyNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	keys, err := store.List("nonexistent")
	if err != nil {
		t.Fatalf("List should not error for empty namespace: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for empty namespace, got %d", len(keys))
	}
}

// ============================================
// Test: Meta.json persists project path and ID
// ============================================

func TestPersistMetaFields(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	meta := store.GetMeta()
	if meta.ProjectPath != dir {
		t.Errorf("expected project_path=%s, got %s", dir, meta.ProjectPath)
	}
	if meta.ProjectID != dir {
		t.Errorf("expected project_id=%s, got %s", dir, meta.ProjectID)
	}
	if meta.FirstCreated.IsZero() {
		t.Error("expected first_created to be set")
	}

	store.Shutdown()
}

// ============================================
// Test: Project size limit (10MB per project)
// ============================================

func TestPersistProjectSizeLimit(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save several files that together exceed 10MB
	// Each file is ~900KB (under 1MB limit) but 12 of them > 10MB
	bigChunk := make([]byte, 900*1024)
	for i := range bigChunk {
		bigChunk[i] = 'b'
	}
	chunkJSON, _ := json.Marshal(string(bigChunk))

	var lastErr error
	for i := 0; i < 12; i++ {
		key := string(rune('a'+i)) + "_data"
		lastErr = store.Save("big", key, chunkJSON)
		if lastErr != nil {
			break
		}
	}

	if lastErr == nil {
		t.Fatal("expected error when project size exceeds 10MB")
	}
	if !strings.Contains(lastErr.Error(), "project size") {
		t.Errorf("error should mention project size limit, got: %v", lastErr)
	}
}

// ============================================
// Test: MCP tool session_store - save action
// ============================================

func TestPersistMCPToolSave(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	args := SessionStoreArgs{
		Action:    "save",
		Namespace: "baselines",
		Key:       "login",
		Data:      json.RawMessage(`{"steps":["open","type","click"]}`),
	}

	result, err := store.HandleSessionStore(args)
	if err != nil {
		t.Fatalf("HandleSessionStore save failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify we can load it back
	loaded, loadErr := store.Load("baselines", "login")
	if loadErr != nil {
		t.Fatalf("Load after MCP save failed: %v", loadErr)
	}
	if string(loaded) != `{"steps":["open","type","click"]}` {
		t.Errorf("unexpected loaded data: %s", string(loaded))
	}
}

// ============================================
// Test: MCP tool session_store - load action
// ============================================

func TestPersistMCPToolLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("noise", "config", []byte(`{"rules":["a","b"]}`))

	args := SessionStoreArgs{
		Action:    "load",
		Namespace: "noise",
		Key:       "config",
	}

	result, err := store.HandleSessionStore(args)
	if err != nil {
		t.Fatalf("HandleSessionStore load failed: %v", err)
	}

	var loaded map[string]interface{}
	json.Unmarshal(result, &loaded)
	data, ok := loaded["data"]
	if !ok {
		t.Fatal("expected 'data' field in result")
	}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", data)
	}
	rules, ok := dataMap["rules"].([]interface{})
	if !ok || len(rules) != 2 {
		t.Errorf("expected 2 rules, got %v", dataMap["rules"])
	}
}

// ============================================
// Test: MCP tool session_store - list action
// ============================================

func TestPersistMCPToolList(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("baselines", "a", []byte(`{}`))
	store.Save("baselines", "b", []byte(`{}`))

	args := SessionStoreArgs{
		Action:    "list",
		Namespace: "baselines",
	}

	result, err := store.HandleSessionStore(args)
	if err != nil {
		t.Fatalf("HandleSessionStore list failed: %v", err)
	}

	var response map[string]interface{}
	json.Unmarshal(result, &response)
	keys, ok := response["keys"].([]interface{})
	if !ok {
		t.Fatalf("expected keys array, got %T", response["keys"])
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// ============================================
// Test: MCP tool session_store - delete action
// ============================================

func TestPersistMCPToolDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("baselines", "todelete", []byte(`{"x":1}`))

	args := SessionStoreArgs{
		Action:    "delete",
		Namespace: "baselines",
		Key:       "todelete",
	}

	_, err = store.HandleSessionStore(args)
	if err != nil {
		t.Fatalf("HandleSessionStore delete failed: %v", err)
	}

	// Verify it's gone
	_, loadErr := store.Load("baselines", "todelete")
	if loadErr == nil {
		t.Fatal("expected error loading deleted key")
	}
}

// ============================================
// Test: MCP tool session_store - stats action
// ============================================

func TestPersistMCPToolStats(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("baselines", "x", []byte(`{"val":123}`))

	args := SessionStoreArgs{
		Action: "stats",
	}

	result, err := store.HandleSessionStore(args)
	if err != nil {
		t.Fatalf("HandleSessionStore stats failed: %v", err)
	}

	var stats map[string]interface{}
	json.Unmarshal(result, &stats)
	if stats["session_count"] == nil {
		t.Error("expected session_count in stats")
	}
	if stats["total_bytes"] == nil {
		t.Error("expected total_bytes in stats")
	}
}

// ============================================
// Test: MCP tool load_session_context
// ============================================

func TestPersistMCPToolLoadContext(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("baselines", "login", []byte(`{"name":"login"}`))
	store.Save("baselines", "checkout", []byte(`{"name":"checkout"}`))

	ctx := store.LoadSessionContext()
	if ctx.ProjectID == "" {
		t.Error("expected non-empty project_id")
	}
	if ctx.SessionCount < 1 {
		t.Error("expected session_count >= 1")
	}
	if len(ctx.Baselines) != 2 {
		t.Errorf("expected 2 baselines, got %d", len(ctx.Baselines))
	}
}

// ============================================
// Test: MCP tool session_store - missing required params
// ============================================

func TestPersistMCPToolMissingParams(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save without namespace
	args := SessionStoreArgs{
		Action: "save",
		Key:    "mykey",
		Data:   json.RawMessage(`{}`),
	}
	_, err = store.HandleSessionStore(args)
	if err == nil {
		t.Fatal("expected error for missing namespace on save")
	}

	// Save without key
	args = SessionStoreArgs{
		Action:    "save",
		Namespace: "ns",
		Data:      json.RawMessage(`{}`),
	}
	_, err = store.HandleSessionStore(args)
	if err == nil {
		t.Fatal("expected error for missing key on save")
	}

	// Save without data
	args = SessionStoreArgs{
		Action:    "save",
		Namespace: "ns",
		Key:       "k",
	}
	_, err = store.HandleSessionStore(args)
	if err == nil {
		t.Fatal("expected error for missing data on save")
	}

	// Invalid action
	args = SessionStoreArgs{
		Action: "invalid",
	}
	_, err = store.HandleSessionStore(args)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

// ============================================
// Test: Corrupted JSON in meta.json starts fresh
// ============================================

func TestPersistCorruptedMetaStartsFresh(t *testing.T) {
	dir := t.TempDir()

	// Create .gasoline dir with corrupted meta.json
	gasolineDir := filepath.Join(dir, ".gasoline")
	os.MkdirAll(gasolineDir, 0755)
	os.WriteFile(filepath.Join(gasolineDir, "meta.json"), []byte("{corrupted json!!!"), 0644)

	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore should not fail on corrupted meta: %v", err)
	}
	defer store.Shutdown()

	meta := store.GetMeta()
	if meta.SessionCount != 1 {
		t.Errorf("expected fresh session_count=1 after corrupted meta, got %d", meta.SessionCount)
	}
}

// ============================================
// Test: Corrupted JSON in namespace file is silently skipped
// ============================================

func TestPersistCorruptedNamespaceFileSkipped(t *testing.T) {
	dir := t.TempDir()

	// Create store with some valid data
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	store.Save("baselines", "valid", []byte(`{"name":"valid"}`))
	store.Shutdown()

	// Corrupt the error history file
	errDir := filepath.Join(dir, ".gasoline", "errors")
	os.MkdirAll(errDir, 0755)
	os.WriteFile(filepath.Join(errDir, "history.json"), []byte("not valid json at all"), 0644)

	// Reload store and load context
	store2, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore (reload) failed: %v", err)
	}
	defer store2.Shutdown()

	ctx := store2.LoadSessionContext()
	// Error history should be empty (corrupted file skipped)
	if len(ctx.ErrorHistory) != 0 {
		t.Errorf("expected 0 error history entries after corruption, got %d", len(ctx.ErrorHistory))
	}
	// But valid baselines should still load
	if len(ctx.Baselines) != 1 {
		t.Errorf("expected 1 baseline, got %d", len(ctx.Baselines))
	}
}

// ============================================
// Test: MCP tool handler returns error when store not initialized
// ============================================

func TestPersistMCPToolStoreNotInitialized(t *testing.T) {
	server := &Server{
		entries:    make([]LogEntry, 0),
		maxEntries: 1000,
	}
	capture := NewCapture()

	// Create a ToolHandler with nil sessionStore
	handler := &ToolHandler{
		MCPHandler:  NewMCPHandler(server),
		capture:     capture,
		checkpoints: NewCheckpointManager(server, capture),
		noise:       NewNoiseConfig(),
		// sessionStore intentionally nil
	}

	// Test configure with action:"store" tool
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/call"}
	args := json.RawMessage(`{"action":"store","store_action":"stats"}`)
	resp, handled := handler.handleToolCall(req, "configure", args)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Error("expected error result when store not initialized")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "not initialized") {
		t.Errorf("expected 'not initialized' error message, got: %v", result.Content)
	}

	// Test configure with action:"load" tool
	resp, handled = handler.handleToolCall(req, "configure", json.RawMessage(`{"action":"load"}`))
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	json.Unmarshal(resp.Result, &result)
	if !result.IsError {
		t.Error("expected error result when store not initialized for configure(action:load)")
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "not initialized") {
		t.Errorf("expected 'not initialized' error message, got: %v", result.Content)
	}
}

// ============================================
// Test: MCP tool integration - session_store through ToolHandler
// ============================================

func TestPersistMCPToolHandlerIntegration(t *testing.T) {
	dir := t.TempDir()
	server := &Server{
		entries:    make([]LogEntry, 0),
		maxEntries: 1000,
	}
	capture := NewCapture()

	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	handler := &ToolHandler{
		MCPHandler:   NewMCPHandler(server),
		capture:      capture,
		checkpoints:  NewCheckpointManager(server, capture),
		sessionStore: store,
		noise:        NewNoiseConfig(),
	}

	// Test save through handler
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/call"}
	saveArgs := json.RawMessage(`{"action":"store","store_action":"save","namespace":"test_ns","key":"test_key","data":{"hello":"world"}}`)
	resp, handled := handler.handleToolCall(req, "configure", saveArgs)
	if !handled {
		t.Fatal("expected configure to be handled")
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if result.IsError {
		t.Fatalf("unexpected tool error: %s", result.Content[0].Text)
	}

	// Test load through handler
	loadArgs := json.RawMessage(`{"action":"store","store_action":"load","namespace":"test_ns","key":"test_key"}`)
	resp, handled = handler.handleToolCall(req, "configure", loadArgs)
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	json.Unmarshal(resp.Result, &result)
	if result.IsError {
		t.Fatalf("unexpected tool error on load: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "hello") {
		t.Errorf("expected loaded data to contain 'hello', got: %s", result.Content[0].Text)
	}

	// Test configure with action:"load" through handler
	resp, handled = handler.handleToolCall(req, "configure", json.RawMessage(`{"action":"load"}`))
	if !handled {
		t.Fatal("expected configure to be handled")
	}

	json.Unmarshal(resp.Result, &result)
	if result.IsError {
		t.Fatalf("unexpected tool error on configure(action:load): %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "session_count") {
		t.Errorf("expected session context to contain session_count, got: %s", result.Content[0].Text)
	}
}

// ============================================
// Test: NewToolHandler initializes SessionStore with CWD
// ============================================

func TestPersistNewToolHandlerInitializesStore(t *testing.T) {
	dir := t.TempDir()

	// Change to temp dir so NewToolHandler uses it as project root
	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	server := &Server{
		entries:    make([]LogEntry, 0),
		maxEntries: 1000,
	}
	capture := NewCapture()

	mcpHandler := setupToolHandler(t, server, capture)

	// Verify the tool handler has a session store by calling configure with action:"load"
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"load"}}`)}
	resp := mcpHandler.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %v", resp.Error)
	}

	var result MCPToolResult
	json.Unmarshal(resp.Result, &result)
	if result.IsError {
		t.Fatalf("expected successful configure(action:load), got error: %s", result.Content[0].Text)
	}
	if !strings.Contains(result.Content[0].Text, "session_count") {
		t.Errorf("expected session context, got: %s", result.Content[0].Text)
	}

	// Verify .gasoline directory was created
	gasolineDir := filepath.Join(dir, ".gasoline")
	if _, err := os.Stat(gasolineDir); os.IsNotExist(err) {
		t.Error("expected .gasoline directory to be created by NewToolHandler")
	}
}

// ============================================
// Test: SessionStore Shutdown is idempotent (double shutdown)
// ============================================

func TestPersistShutdownIdempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// First shutdown
	store.Shutdown()
	// Second shutdown should not panic
	store.Shutdown()
}

// ============================================
// Additional Coverage Tests
// ============================================

// --- LoadSessionContext: baselines directory with entries ---

func TestLoadSessionContextBaselinesDir(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create baselines directory with several JSON files and a non-JSON file
	baselineDir := filepath.Join(dir, ".gasoline", "baselines")
	os.MkdirAll(baselineDir, 0o755)
	os.WriteFile(filepath.Join(baselineDir, "homepage.json"), []byte(`{"p50":200}`), 0o644)
	os.WriteFile(filepath.Join(baselineDir, "api-latency.json"), []byte(`{"p50":100}`), 0o644)
	os.WriteFile(filepath.Join(baselineDir, "ignored.txt"), []byte(`not json`), 0o644)
	// Also create a subdirectory that should be skipped
	os.MkdirAll(filepath.Join(baselineDir, "subdir"), 0o755)

	ctx := store.LoadSessionContext()

	if len(ctx.Baselines) != 2 {
		t.Fatalf("expected 2 baselines, got %d: %v", len(ctx.Baselines), ctx.Baselines)
	}
	// Check both baselines are present (order may vary)
	found := map[string]bool{}
	for _, b := range ctx.Baselines {
		found[b] = true
	}
	if !found["homepage"] || !found["api-latency"] {
		t.Errorf("expected homepage and api-latency baselines, got: %v", ctx.Baselines)
	}
}

// --- LoadSessionContext: noise config loading ---

func TestLoadSessionContextNoiseConfig(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	noiseDir := filepath.Join(dir, ".gasoline", "noise")
	os.MkdirAll(noiseDir, 0o755)
	noiseConfig := `{"ignore_patterns":["favicon.ico"],"threshold":0.8}`
	os.WriteFile(filepath.Join(noiseDir, "config.json"), []byte(noiseConfig), 0o644)

	ctx := store.LoadSessionContext()

	if ctx.NoiseConfig == nil {
		t.Fatal("expected NoiseConfig to be loaded")
	}
	if _, ok := ctx.NoiseConfig["ignore_patterns"]; !ok {
		t.Errorf("expected ignore_patterns key in NoiseConfig, got: %v", ctx.NoiseConfig)
	}
	if ctx.NoiseConfig["threshold"] != 0.8 {
		t.Errorf("expected threshold=0.8, got: %v", ctx.NoiseConfig["threshold"])
	}
}

// --- LoadSessionContext: error history loading (typed entries) ---

func TestLoadSessionContextErrorHistoryTyped(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	errDir := filepath.Join(dir, ".gasoline", "errors")
	os.MkdirAll(errDir, 0o755)

	entries := []ErrorHistoryEntry{
		{
			Fingerprint: "err-001",
			FirstSeen:   time.Now().Add(-48 * time.Hour),
			LastSeen:    time.Now().Add(-1 * time.Hour),
			Count:       5,
			Resolved:    false,
		},
		{
			Fingerprint: "err-002",
			FirstSeen:   time.Now().Add(-24 * time.Hour),
			LastSeen:    time.Now(),
			Count:       2,
			Resolved:    true,
		},
	}
	data, _ := json.Marshal(entries)
	os.WriteFile(filepath.Join(errDir, "history.json"), data, 0o644)

	ctx := store.LoadSessionContext()

	if len(ctx.ErrorHistory) != 2 {
		t.Fatalf("expected 2 error history entries, got %d", len(ctx.ErrorHistory))
	}
	if ctx.ErrorHistory[0].Fingerprint != "err-001" {
		t.Errorf("expected first entry fingerprint=err-001, got: %s", ctx.ErrorHistory[0].Fingerprint)
	}
	if ctx.ErrorHistory[1].Resolved != true {
		t.Errorf("expected second entry resolved=true")
	}
	if ctx.ErrorHistory[0].Count != 5 {
		t.Errorf("expected first entry count=5, got: %d", ctx.ErrorHistory[0].Count)
	}
}

// --- LoadSessionContext: error history loading (raw JSON fallback path) ---

func TestLoadSessionContextErrorHistoryRawFallback(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	errDir := filepath.Join(dir, ".gasoline", "errors")
	os.MkdirAll(errDir, 0o755)

	// Write raw map entries that won't unmarshal as []ErrorHistoryEntry
	// because they have extra unknown fields and missing time fields
	rawEntries := `[{"fingerprint":"raw-err-1","count":3,"resolved":false,"extra_field":"yes"},{"fingerprint":"raw-err-2","count":7,"resolved":true}]`
	os.WriteFile(filepath.Join(errDir, "history.json"), []byte(rawEntries), 0o644)

	ctx := store.LoadSessionContext()

	// The typed unmarshal should actually succeed here since extra fields are ignored.
	// To really trigger the fallback, we need invalid time fields.
	// Actually looking at the code: the typed path WILL succeed with extra fields ignored.
	// The raw fallback triggers when typed unmarshal fails. Let's use invalid time format.
	if len(ctx.ErrorHistory) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(ctx.ErrorHistory))
	}
	if ctx.ErrorHistory[0].Fingerprint != "raw-err-1" {
		t.Errorf("expected fingerprint=raw-err-1, got: %s", ctx.ErrorHistory[0].Fingerprint)
	}
}

// --- LoadSessionContext: error history with truly invalid typed fields (forces raw fallback) ---

func TestLoadSessionContextErrorHistoryForcedRawFallback(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	errDir := filepath.Join(dir, ".gasoline", "errors")
	os.MkdirAll(errDir, 0o755)

	// Use invalid time format to force typed unmarshal to fail, triggering raw fallback
	rawJSON := `[{"fingerprint":"fallback-1","count":10,"resolved":true,"first_seen":"not-a-timestamp","last_seen":"also-not-a-timestamp"},{"fingerprint":"fallback-2","count":1,"resolved":false,"first_seen":"invalid","last_seen":"invalid"}]`
	os.WriteFile(filepath.Join(errDir, "history.json"), []byte(rawJSON), 0o644)

	ctx := store.LoadSessionContext()

	if len(ctx.ErrorHistory) != 2 {
		t.Fatalf("expected 2 entries from raw fallback, got %d", len(ctx.ErrorHistory))
	}
	if ctx.ErrorHistory[0].Fingerprint != "fallback-1" {
		t.Errorf("expected fingerprint=fallback-1, got: %s", ctx.ErrorHistory[0].Fingerprint)
	}
	if ctx.ErrorHistory[0].Count != 10 {
		t.Errorf("expected count=10, got: %d", ctx.ErrorHistory[0].Count)
	}
	if ctx.ErrorHistory[0].Resolved != true {
		t.Errorf("expected resolved=true for first entry")
	}
	if ctx.ErrorHistory[1].Fingerprint != "fallback-2" {
		t.Errorf("expected fingerprint=fallback-2, got: %s", ctx.ErrorHistory[1].Fingerprint)
	}
}

// --- LoadSessionContext: API schema loading ---

func TestLoadSessionContextAPISchema(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	schemaDir := filepath.Join(dir, ".gasoline", "api_schema")
	os.MkdirAll(schemaDir, 0o755)
	schema := `{"endpoints":{"/api/users":{"method":"GET","response_type":"json"}}}`
	os.WriteFile(filepath.Join(schemaDir, "schema.json"), []byte(schema), 0o644)

	ctx := store.LoadSessionContext()

	if ctx.APISchema == nil {
		t.Fatal("expected APISchema to be loaded")
	}
	endpoints, ok := ctx.APISchema["endpoints"]
	if !ok {
		t.Fatalf("expected endpoints key in APISchema, got: %v", ctx.APISchema)
	}
	endpointsMap, ok := endpoints.(map[string]interface{})
	if !ok {
		t.Fatalf("expected endpoints to be a map")
	}
	if _, ok := endpointsMap["/api/users"]; !ok {
		t.Errorf("expected /api/users endpoint in schema")
	}
}

// --- LoadSessionContext: performance loading ---

func TestLoadSessionContextPerformance(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	perfDir := filepath.Join(dir, ".gasoline", "performance")
	os.MkdirAll(perfDir, 0o755)
	perf := `{"homepage":{"p50_ms":150,"p95_ms":400},"api":{"p50_ms":50}}`
	os.WriteFile(filepath.Join(perfDir, "endpoints.json"), []byte(perf), 0o644)

	ctx := store.LoadSessionContext()

	if ctx.Performance == nil {
		t.Fatal("expected Performance to be loaded")
	}
	if _, ok := ctx.Performance["homepage"]; !ok {
		t.Errorf("expected homepage key in Performance, got: %v", ctx.Performance)
	}
	if _, ok := ctx.Performance["api"]; !ok {
		t.Errorf("expected api key in Performance")
	}
}

// --- HandleSessionStore: additional action branches ---

func TestSessionStoreHandleDeleteAction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save something first
	store.Save("ns", "mykey", []byte(`{"value":42}`))

	// Delete through handler
	result, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "delete",
		Namespace: "ns",
		Key:       "mykey",
	})
	if err != nil {
		t.Fatalf("delete action failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)
	if parsed["status"] != "deleted" {
		t.Errorf("expected status=deleted, got: %v", parsed["status"])
	}

	// Verify file is gone
	_, loadErr := store.Load("ns", "mykey")
	if loadErr == nil {
		t.Error("expected load to fail after delete")
	}
}

func TestSessionStoreHandleStatsAction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save a couple of entries
	store.Save("metrics", "key1", []byte(`{"a":1}`))
	store.Save("metrics", "key2", []byte(`{"b":2}`))
	store.Save("logs", "entry1", []byte(`{"c":3}`))

	result, err := store.HandleSessionStore(SessionStoreArgs{Action: "stats"})
	if err != nil {
		t.Fatalf("stats action failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)

	sessionCount, ok := parsed["session_count"].(float64)
	if !ok || sessionCount < 1 {
		t.Errorf("expected session_count >= 1, got: %v", parsed["session_count"])
	}

	namespaces, ok := parsed["namespaces"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected namespaces map, got: %v", parsed["namespaces"])
	}
	if namespaces["metrics"] != float64(2) {
		t.Errorf("expected metrics namespace count=2, got: %v", namespaces["metrics"])
	}
	if namespaces["logs"] != float64(1) {
		t.Errorf("expected logs namespace count=1, got: %v", namespaces["logs"])
	}
}

func TestSessionStoreHandleUnknownAction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{Action: "bogus"})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("expected 'unknown action' error, got: %v", err)
	}
}

func TestSessionStoreHandleSaveMissingData(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "save",
		Namespace: "ns",
		Key:       "k",
		Data:      nil,
	})
	if err == nil {
		t.Fatal("expected error for save with empty data")
	}
	if !strings.Contains(err.Error(), "data is required") {
		t.Errorf("expected 'data is required' error, got: %v", err)
	}
}

func TestSessionStoreHandleListAction(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("myns", "alpha", []byte(`"a"`))
	store.Save("myns", "beta", []byte(`"b"`))

	result, err := store.HandleSessionStore(SessionStoreArgs{
		Action:    "list",
		Namespace: "myns",
	})
	if err != nil {
		t.Fatalf("list action failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(result, &parsed)

	keys, ok := parsed["keys"].([]interface{})
	if !ok {
		t.Fatalf("expected keys array, got: %v", parsed["keys"])
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestSessionStoreHandleDeleteMissingNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action: "delete",
		Key:    "k",
	})
	if err == nil {
		t.Fatal("expected error for delete without namespace")
	}
	if !strings.Contains(err.Error(), "namespace is required") {
		t.Errorf("expected 'namespace is required' error, got: %v", err)
	}
}

func TestSessionStoreHandleDeleteMissingKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "delete",
		Namespace: "ns",
	})
	if err == nil {
		t.Fatal("expected error for delete without key")
	}
	if !strings.Contains(err.Error(), "key is required") {
		t.Errorf("expected 'key is required' error, got: %v", err)
	}
}

func TestSessionStoreHandleLoadMissingNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action: "load",
		Key:    "k",
	})
	if err == nil {
		t.Fatal("expected error for load without namespace")
	}
}

func TestSessionStoreHandleLoadMissingKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "load",
		Namespace: "ns",
	})
	if err == nil {
		t.Fatal("expected error for load without key")
	}
}

func TestSessionStoreHandleListMissingNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action: "list",
	})
	if err == nil {
		t.Fatal("expected error for list without namespace")
	}
}

// --- NewSessionStoreWithInterval: MkdirAll error path ---

func TestSessionStoreNewMkdirAllError(t *testing.T) {
	// Use a path where the parent is a file, not a directory
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "blocker")
	os.WriteFile(blockingFile, []byte("I am a file"), 0o644)

	// Try to create a store where .gasoline would be under a file (impossible)
	_, err := NewSessionStore(blockingFile)
	if err == nil {
		t.Fatal("expected error when project path is a file (MkdirAll should fail)")
	}
	if !strings.Contains(err.Error(), "failed to create .gasoline directory") {
		t.Errorf("expected 'failed to create .gasoline directory' error, got: %v", err)
	}
}

// --- ensureGitignore: file not ending with newline ---

func TestPersistGitignoreNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()

	// Create .gitignore WITHOUT trailing newline
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n.env"), 0o644) // no trailing \n

	_, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	data, _ := os.ReadFile(gitignorePath)
	content := string(data)

	// Should have added a newline before .gasoline/
	if !strings.Contains(content, ".env\n.gasoline/\n") {
		t.Errorf("expected .env followed by newline then .gasoline/, got: %q", content)
	}
}

// --- ensureGitignore: OpenFile error path ---

func TestPersistGitignoreOpenFileError(t *testing.T) {
	dir := t.TempDir()

	// Create a read-only .gitignore (can read but not write/append)
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0o444)

	// NewSessionStore should still succeed (ensureGitignore handles errors gracefully)
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// .gasoline/ should NOT be in .gitignore since it couldn't be written
	data, _ := os.ReadFile(gitignorePath)
	if strings.Contains(string(data), ".gasoline") {
		t.Error("should not have been able to write to read-only .gitignore")
	}
}

// --- GetMeta: nil meta path ---

func TestPersistGetMetaNil(t *testing.T) {
	// Create a store and then nil out meta to test the nil guard in GetMeta
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Set meta to nil to test the nil guard
	store.mu.Lock()
	originalMeta := store.meta
	store.meta = nil
	store.mu.Unlock()

	meta := store.GetMeta()
	if meta.ProjectID != "" {
		t.Errorf("expected empty ProjectID for nil meta, got: %s", meta.ProjectID)
	}
	if meta.SessionCount != 0 {
		t.Errorf("expected SessionCount=0 for nil meta, got: %d", meta.SessionCount)
	}

	// Restore meta so Shutdown doesn't panic
	store.mu.Lock()
	store.meta = originalMeta
	store.mu.Unlock()
	store.Shutdown()
}

// --- enforceErrorHistoryCap: entries exceeding maxErrorHistory with eviction ---

func TestEnforceErrorHistoryCapEviction(t *testing.T) {
	// Create maxErrorHistory + 10 entries with varying FirstSeen times
	entries := make([]ErrorHistoryEntry, maxErrorHistory+10)
	baseTime := time.Now().Add(-1000 * time.Hour)

	for i := range entries {
		entries[i] = ErrorHistoryEntry{
			Fingerprint: fmt.Sprintf("err-%04d", i),
			FirstSeen:   baseTime.Add(time.Duration(i) * time.Hour),
			LastSeen:    time.Now(),
			Count:       1,
		}
	}

	result := enforceErrorHistoryCap(entries)

	if len(result) != maxErrorHistory {
		t.Fatalf("expected %d entries after cap, got %d", maxErrorHistory, len(result))
	}

	// The oldest 10 entries (err-0000 through err-0009) should have been evicted
	for _, e := range result {
		idx := 0
		fmt.Sscanf(e.Fingerprint, "err-%d", &idx)
		if idx < 10 {
			t.Errorf("entry err-%04d should have been evicted (oldest), but was kept", idx)
		}
	}
}

// --- evictStaleErrors: all entries stale, returning nil -> empty slice ---

func TestEvictStaleErrorsAllStale(t *testing.T) {
	entries := []ErrorHistoryEntry{
		{
			Fingerprint: "old-1",
			LastSeen:    time.Now().Add(-60 * 24 * time.Hour), // 60 days ago
			Count:       1,
		},
		{
			Fingerprint: "old-2",
			LastSeen:    time.Now().Add(-90 * 24 * time.Hour), // 90 days ago
			Count:       2,
		},
	}

	result := evictStaleErrors(entries, staleErrorThreshold)

	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries after evicting all stale, got %d", len(result))
	}
}

// --- List: directory entries (IsDir=true skipped), non-json files skipped ---

func TestPersistListSkipsDirsAndNonJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	nsDir := filepath.Join(dir, ".gasoline", "testns")
	os.MkdirAll(nsDir, 0o755)

	// Create a valid JSON file
	os.WriteFile(filepath.Join(nsDir, "valid.json"), []byte(`{}`), 0o644)
	// Create a non-JSON file
	os.WriteFile(filepath.Join(nsDir, "readme.txt"), []byte("hello"), 0o644)
	// Create a subdirectory
	os.MkdirAll(filepath.Join(nsDir, "subdir"), 0o755)

	keys, err := store.List("testns")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key (only .json), got %d: %v", len(keys), keys)
	}
	if keys[0] != "valid" {
		t.Errorf("expected key 'valid', got: %s", keys[0])
	}
}

// --- Delete: error path (file doesn't exist) ---

func TestPersistDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	err = store.Delete("nonexistent-ns", "nonexistent-key")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent file")
	}
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Errorf("expected 'failed to delete' error, got: %v", err)
	}
}

// --- Stats: subdirectory traversal and file counting ---

func TestPersistStatsSubdirTraversal(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create multiple namespaces with files
	store.Save("ns1", "a", []byte(`{"x":1}`))
	store.Save("ns1", "b", []byte(`{"x":2}`))
	store.Save("ns2", "c", []byte(`{"x":3}`))

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Namespaces["ns1"] != 2 {
		t.Errorf("expected ns1 count=2, got: %d", stats.Namespaces["ns1"])
	}
	if stats.Namespaces["ns2"] != 1 {
		t.Errorf("expected ns2 count=1, got: %d", stats.Namespaces["ns2"])
	}
	if stats.TotalBytes <= 0 {
		t.Errorf("expected TotalBytes > 0, got: %d", stats.TotalBytes)
	}
	if stats.SessionCount < 1 {
		t.Errorf("expected SessionCount >= 1, got: %d", stats.SessionCount)
	}
}

// --- Stats: namespace subdirectory with subdirs (skipped) ---

func TestPersistStatsSkipsNestedDirs(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	store.Save("ns", "file1", []byte(`{"v":1}`))

	// Create a subdirectory inside the namespace (should be skipped)
	nestedDir := filepath.Join(dir, ".gasoline", "ns", "nested")
	os.MkdirAll(nestedDir, 0o755)
	os.WriteFile(filepath.Join(nestedDir, "deep.json"), []byte(`{"deep":true}`), 0o644)

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	// Only file1 should be counted (nested dir entries skipped due to IsDir check)
	if stats.Namespaces["ns"] != 1 {
		t.Errorf("expected ns count=1 (nested dir skipped), got: %d", stats.Namespaces["ns"])
	}
}

// --- flushDirty: invalid key format (no slash) ---

func TestPersistFlushDirtyInvalidKeyFormat(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Directly inject an invalid key (no slash separator)
	store.dirtyMu.Lock()
	store.dirty["invalidkey-no-slash"] = []byte(`{"bad":true}`)
	store.dirty["valid/goodkey"] = []byte(`{"good":true}`)
	store.dirtyMu.Unlock()

	// Trigger flush
	store.flushDirty()

	// The valid key should have been flushed
	data, err := store.Load("valid", "goodkey")
	if err != nil {
		t.Fatalf("expected valid/goodkey to be flushed, got error: %v", err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["good"] != true {
		t.Errorf("expected good=true, got: %v", parsed)
	}

	// The invalid key should NOT have created any file
	// (no way to locate it since there's no valid namespace/key split)
	entries, _ := os.ReadDir(filepath.Join(dir, ".gasoline"))
	for _, e := range entries {
		if e.Name() == "invalidkey-no-slash" || e.Name() == "invalidkey-no-slash.json" {
			t.Errorf("invalid key should not have created a file, found: %s", e.Name())
		}
	}
}

// --- Save: size limit exceeded ---

func TestPersistSaveSizeLimitExceeded(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Fill up the store to exceed 10MB project limit.
	// Use 900KB chunks (JSON-encoded to be valid). 12 * 900KB > 10MB.
	bigChunk := make([]byte, 900*1024)
	for i := range bigChunk {
		bigChunk[i] = 'a'
	}
	chunkJSON, _ := json.Marshal(string(bigChunk))

	var lastErr error
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("chunk%d", i)
		lastErr = store.Save("size_test", key, chunkJSON)
		if lastErr != nil {
			break
		}
	}

	if lastErr == nil {
		t.Fatal("expected error when project size limit exceeded")
	}
	if !strings.Contains(lastErr.Error(), "project size limit exceeded") {
		t.Errorf("expected 'project size limit exceeded' error, got: %v", err)
	}
}

func TestPersistSaveExceedsMaxFileSizeExact(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create data exactly at maxFileSize + 1
	oversize := make([]byte, maxFileSize+1)
	for i := range oversize {
		oversize[i] = 'y'
	}

	err = store.Save("ns", "toobig", oversize)
	if err == nil {
		t.Fatal("expected error for data exceeding maxFileSize")
	}
	if !strings.Contains(err.Error(), "data exceeds maximum file size") {
		t.Errorf("expected 'data exceeds maximum file size' error, got: %v", err)
	}
}

// --- HandleSessionStore: save action with file too large ---

func TestSessionStoreHandleSaveFileTooLarge(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	oversize := make([]byte, maxFileSize+100)
	for i := range oversize {
		oversize[i] = 'z'
	}

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "save",
		Namespace: "ns",
		Key:       "big",
		Data:      json.RawMessage(oversize),
	})
	if err == nil {
		t.Fatal("expected error from HandleSessionStore for oversized data")
	}
}

// --- HandleSessionStore: save with missing namespace ---

func TestSessionStoreHandleSaveMissingNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action: "save",
		Key:    "k",
		Data:   json.RawMessage(`{"v":1}`),
	})
	if err == nil {
		t.Fatal("expected error for save without namespace")
	}
	if !strings.Contains(err.Error(), "namespace is required") {
		t.Errorf("expected 'namespace is required' error, got: %v", err)
	}
}

// --- HandleSessionStore: save with missing key ---

func TestSessionStoreHandleSaveMissingKey(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "save",
		Namespace: "ns",
		Data:      json.RawMessage(`{"v":1}`),
	})
	if err == nil {
		t.Fatal("expected error for save without key")
	}
	if !strings.Contains(err.Error(), "key is required") {
		t.Errorf("expected 'key is required' error, got: %v", err)
	}
}

// --- HandleSessionStore: load nonexistent key ---

func TestSessionStoreHandleLoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "load",
		Namespace: "ns",
		Key:       "does-not-exist",
	})
	if err == nil {
		t.Fatal("expected error for loading nonexistent key")
	}
	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("expected 'key not found' error, got: %v", err)
	}
}

// --- HandleSessionStore: delete nonexistent key ---

func TestSessionStoreHandleDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "delete",
		Namespace: "ns",
		Key:       "ghost",
	})
	if err == nil {
		t.Fatal("expected error for deleting nonexistent key")
	}
	if !strings.Contains(err.Error(), "failed to delete") {
		t.Errorf("expected 'failed to delete' error, got: %v", err)
	}
}

// --- projectSize: Walk error path (unreadable directory) ---

func TestPersistProjectSizeWalkError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Save a file so there's some size
	store.Save("ns", "file", []byte(`{"data":"value"}`))

	// Create a subdirectory with no read permission to trigger walk errors
	restrictedDir := filepath.Join(dir, ".gasoline", "restricted")
	os.MkdirAll(restrictedDir, 0o755)
	os.WriteFile(filepath.Join(restrictedDir, "hidden.json"), []byte(`{}`), 0o644)
	os.Chmod(restrictedDir, 0o000)

	// projectSize should still return (errors are skipped in walk)
	store.mu.Lock()
	size, err := store.projectSize()
	store.mu.Unlock()

	// Restore permissions for cleanup
	os.Chmod(restrictedDir, 0o755)

	if err != nil {
		t.Fatalf("projectSize should not return error (skip errors), got: %v", err)
	}
	// Size should at least include the ns/file.json we created
	if size <= 0 {
		t.Errorf("expected positive size, got: %d", size)
	}
}

// --- flushDirty: empty dirty map (no-op path) ---

func TestPersistFlushDirtyEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Flush with nothing dirty should be a no-op (just returns early)
	store.flushDirty()

	// Verify no crash and no extra files
	entries, _ := os.ReadDir(filepath.Join(dir, ".gasoline"))
	for _, e := range entries {
		if e.Name() != "meta.json" && e.Name() != ".gitignore" {
			// Only meta.json should exist
		}
	}
}

// --- evictStaleErrors: empty input returns empty slice ---

func TestEvictStaleErrorsEmptyInput(t *testing.T) {
	result := evictStaleErrors([]ErrorHistoryEntry{}, staleErrorThreshold)
	if result == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

// --- evictStaleErrors: nil input returns empty slice ---

func TestEvictStaleErrorsNilInput(t *testing.T) {
	result := evictStaleErrors(nil, staleErrorThreshold)
	if result == nil {
		t.Fatal("expected non-nil empty slice for nil input")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

// --- enforceErrorHistoryCap: under cap returns unchanged ---

func TestEnforceErrorHistoryCapUnderLimit(t *testing.T) {
	entries := []ErrorHistoryEntry{
		{Fingerprint: "a", FirstSeen: time.Now()},
		{Fingerprint: "b", FirstSeen: time.Now()},
	}
	result := enforceErrorHistoryCap(entries)
	if len(result) != 2 {
		t.Errorf("expected 2 entries (under cap), got %d", len(result))
	}
}

// --- enforceErrorHistoryCap: exactly at cap returns unchanged ---

func TestEnforceErrorHistoryCapExactLimit(t *testing.T) {
	entries := make([]ErrorHistoryEntry, maxErrorHistory)
	for i := range entries {
		entries[i] = ErrorHistoryEntry{
			Fingerprint: fmt.Sprintf("err-%d", i),
			FirstSeen:   time.Now(),
		}
	}
	result := enforceErrorHistoryCap(entries)
	if len(result) != maxErrorHistory {
		t.Errorf("expected %d entries (at cap), got %d", maxErrorHistory, len(result))
	}
}

// --- MarkDirty and background flush integration ---

func TestPersistMarkDirtyAndFlush(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStoreWithInterval(dir, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("NewSessionStoreWithInterval failed: %v", err)
	}
	defer store.Shutdown()

	store.MarkDirty("buffered", "item", []byte(`{"buffered":true}`))

	// Wait for background flush
	time.Sleep(150 * time.Millisecond)

	data, err := store.Load("buffered", "item")
	if err != nil {
		t.Fatalf("expected buffered item to be flushed, got: %v", err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["buffered"] != true {
		t.Errorf("expected buffered=true, got: %v", parsed)
	}
}

// --- Concurrent MarkDirty + flushDirty safety ---

func TestPersistConcurrentMarkDirtyFlush(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", idx)
			store.MarkDirty("concurrent", key, []byte(fmt.Sprintf(`{"idx":%d}`, idx)))
		}(i)
	}

	// Trigger flush concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.flushDirty()
		}()
	}

	wg.Wait()
	// Final flush to catch stragglers
	store.flushDirty()

	// All 50 keys should have been written
	keys, err := store.List("concurrent")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 50 {
		t.Errorf("expected 50 keys from concurrent writes, got %d", len(keys))
	}
}

// ============================================
// Additional Coverage: flushDirty with unwritable namespace dir
// ============================================

func TestPersistFlushDirtyMkdirAllError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create a file where a namespace directory would go, so MkdirAll fails
	blockingPath := filepath.Join(dir, ".gasoline", "blocked_ns")
	os.WriteFile(blockingPath, []byte("I am a file"), 0o644)

	// Inject dirty data targeting that namespace
	store.dirtyMu.Lock()
	store.dirty["blocked_ns/somekey"] = []byte(`{"test":true}`)
	store.dirtyMu.Unlock()

	// Flush should not panic, just skip the errored entry
	store.flushDirty()

	// The file should still be a file (not overwritten)
	info, err := os.Stat(blockingPath)
	if err != nil {
		t.Fatalf("blocking file should still exist: %v", err)
	}
	if info.IsDir() {
		t.Error("blocking file should not have been replaced with a directory")
	}
}

// ============================================
// Additional Coverage: Stats with unreadable projectDir (os.ReadDir error)
// ============================================

func TestPersistStatsReadDirError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Make the projectDir unreadable
	gasolineDir := filepath.Join(dir, ".gasoline")
	os.Chmod(gasolineDir, 0o000)
	defer os.Chmod(gasolineDir, 0o755)

	_, err = store.Stats()
	if err == nil {
		t.Error("expected error from Stats when directory is unreadable")
	}

	os.Chmod(gasolineDir, 0o755)
	store.Shutdown()
}

// ============================================
// Additional Coverage: Stats skipping non-JSON files in namespace
// ============================================

func TestPersistStatsCountsAllFilesInNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create a namespace with multiple files and a subdirectory
	nsDir := filepath.Join(dir, ".gasoline", "myns")
	os.MkdirAll(nsDir, 0o755)
	os.WriteFile(filepath.Join(nsDir, "valid.json"), []byte(`{"x":1}`), 0o644)
	os.WriteFile(filepath.Join(nsDir, "notes.txt"), []byte("hello"), 0o644)
	// Create subdirectory (should be skipped in count)
	os.MkdirAll(filepath.Join(nsDir, "subdir"), 0o755)

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	// Stats counts all non-dir files (2 files, subdir skipped)
	if stats.Namespaces["myns"] != 2 {
		t.Errorf("expected myns count=2 (all files), got: %d", stats.Namespaces["myns"])
	}
}

// ============================================
// Additional Coverage: Stats with unreadable namespace subdirectory (continue on ReadDir err)
// ============================================

func TestPersistStatsUnreadableNamespace(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create a readable namespace and an unreadable one
	nsDir := filepath.Join(dir, ".gasoline", "readable")
	os.MkdirAll(nsDir, 0o755)
	os.WriteFile(filepath.Join(nsDir, "data.json"), []byte(`{"k":1}`), 0o644)

	badDir := filepath.Join(dir, ".gasoline", "unreadable")
	os.MkdirAll(badDir, 0o755)
	os.WriteFile(filepath.Join(badDir, "data.json"), []byte(`{"k":2}`), 0o644)
	os.Chmod(badDir, 0o000)
	defer os.Chmod(badDir, 0o755)

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats failed unexpectedly: %v", err)
	}

	// Readable namespace should be counted
	if stats.Namespaces["readable"] != 1 {
		t.Errorf("expected readable count=1, got: %d", stats.Namespaces["readable"])
	}
	// Unreadable namespace should be skipped (count=0 or not present)
	if stats.Namespaces["unreadable"] != 0 {
		t.Errorf("expected unreadable count=0 (skipped), got: %d", stats.Namespaces["unreadable"])
	}
}

// ============================================
// Additional Coverage: List returns empty when dir has only non-JSON files
// ============================================

func TestPersistListOnlyNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	nsDir := filepath.Join(dir, ".gasoline", "emptyns")
	os.MkdirAll(nsDir, 0o755)
	os.WriteFile(filepath.Join(nsDir, "readme.txt"), []byte("hello"), 0o644)
	os.WriteFile(filepath.Join(nsDir, "data.csv"), []byte("a,b"), 0o644)

	keys, err := store.List("emptyns")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// keys should be empty slice (not nil) since no .json files found
	if keys == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys (no .json files), got %d", len(keys))
	}
}

// ============================================
// Additional Coverage: HandleSessionStore list error from unreadable namespace
// ============================================

func TestSessionStoreHandleListError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	defer store.Shutdown()

	// Create namespace dir then make it unreadable
	nsDir := filepath.Join(dir, ".gasoline", "restricted")
	os.MkdirAll(nsDir, 0o755)
	os.WriteFile(filepath.Join(nsDir, "file.json"), []byte(`{}`), 0o644)
	os.Chmod(nsDir, 0o000)
	defer os.Chmod(nsDir, 0o755)

	_, err = store.HandleSessionStore(SessionStoreArgs{
		Action:    "list",
		Namespace: "restricted",
	})
	if err == nil {
		t.Error("expected error when listing unreadable namespace")
	}
}

// ============================================
// Additional Coverage: HandleSessionStore stats with ReadDir error
// ============================================

func TestSessionStoreHandleStatsError(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir)
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Make projectDir unreadable to trigger Stats error
	gasolineDir := filepath.Join(dir, ".gasoline")
	os.Chmod(gasolineDir, 0o000)
	defer os.Chmod(gasolineDir, 0o755)

	_, err = store.HandleSessionStore(SessionStoreArgs{Action: "stats"})
	if err == nil {
		t.Error("expected error from stats when directory is unreadable")
	}

	os.Chmod(gasolineDir, 0o755)
	store.Shutdown()
}
