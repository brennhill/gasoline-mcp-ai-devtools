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
// Test Scenario 1: Same path always produces the same hash
// ============================================

func TestPersistHashDeterministic(t *testing.T) {
	hash1 := projectHash("/home/user/my-project")
	hash2 := projectHash("/home/user/my-project")
	if hash1 != hash2 {
		t.Errorf("same path produced different hashes: %s vs %s", hash1, hash2)
	}
	if len(hash1) != 16 {
		t.Errorf("expected 16-char hash, got %d chars: %s", len(hash1), hash1)
	}
}

// ============================================
// Test Scenario 2: Different paths produce different hashes
// ============================================

func TestPersistHashUnique(t *testing.T) {
	hash1 := projectHash("/home/user/project-a")
	hash2 := projectHash("/home/user/project-b")
	if hash1 == hash2 {
		t.Errorf("different paths produced same hash: %s", hash1)
	}
}

// ============================================
// Test Scenario 3: Save then load returns identical data
// ============================================

func TestPersistSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store1, err := NewSessionStore(dir, "/fake/project")
	if err != nil {
		t.Fatalf("NewSessionStore (1) failed: %v", err)
	}
	store1.Shutdown()

	// Second session
	store2, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store2, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store2, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	// Mark some data as dirty (use MarkDirty to buffer data)
	store.MarkDirty("errors", "history", []byte(`[{"fingerprint":"e1","count":1}]`))

	// Shutdown should flush
	store.Shutdown()

	// Verify the file was written
	hash := projectHash("/fake/project")
	filePath := filepath.Join(dir, hash, "errors", "history.json")
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
	store, err := NewSessionStoreWithInterval(dir, "/fake/project", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("NewSessionStoreWithInterval failed: %v", err)
	}
	defer store.Shutdown()

	// Mark data as dirty
	store.MarkDirty("perf", "metrics", []byte(`{"latency":42}`))

	// Wait for flush interval to fire
	time.Sleep(300 * time.Millisecond)

	// Verify the file was written
	hash := projectHash("/fake/project")
	filePath := filepath.Join(dir, hash, "perf", "metrics.json")
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
	store, err := NewSessionStore(dir, "/fake/project")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}
	store.Shutdown()

	// Make the namespace directory read-only
	hash := projectHash("/fake/project")
	projectDir := filepath.Join(dir, hash)
	readOnlyDir := filepath.Join(projectDir, "readonly_ns")
	os.MkdirAll(readOnlyDir, 0755)
	os.Chmod(readOnlyDir, 0555)
	// Also make project dir read-only so new dirs can't be created
	os.Chmod(projectDir, 0555)
	defer func() {
		os.Chmod(projectDir, 0755)
		os.Chmod(readOnlyDir, 0755)
	}()

	// Re-open store
	store2, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/my/cool/project")
	if err != nil {
		t.Fatalf("NewSessionStore failed: %v", err)
	}

	meta := store.GetMeta()
	if meta.ProjectPath != "/my/cool/project" {
		t.Errorf("expected project_path=/my/cool/project, got %s", meta.ProjectPath)
	}
	expectedHash := projectHash("/my/cool/project")
	if meta.ProjectID != expectedHash {
		t.Errorf("expected project_id=%s, got %s", expectedHash, meta.ProjectID)
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
	store, err := NewSessionStore(dir, "/fake/project")
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
