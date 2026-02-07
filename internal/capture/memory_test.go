package capture

import (
	"testing"
	"time"
)

// ============================================
// Per-Buffer Memory Tracking Tests
// ============================================

// Helper: create a WebSocketEvent with a specific data size
func makeWSEvent(dataSize int) WebSocketEvent {
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = 'x'
	}
	return WebSocketEvent{
		ID:        "conn-1",
		Event:     "message",
		Direction: "incoming",
		Data:      string(data),
		Timestamp: time.Now().Format(time.RFC3339Nano),
	}
}

// Helper: create a NetworkBody with specific body sizes
func makeNetworkBody(reqSize, respSize int) NetworkBody {
	reqBody := make([]byte, reqSize)
	for i := range reqBody {
		reqBody[i] = 'r'
	}
	respBody := make([]byte, respSize)
	for i := range respBody {
		respBody[i] = 'R'
	}
	return NetworkBody{
		Method:       "GET",
		URL:          "http://example.com/api",
		Status:       200,
		RequestBody:  string(reqBody),
		ResponseBody: string(respBody),
	}
}

// Helper: create an EnhancedAction
func makeAction() EnhancedAction {
	return EnhancedAction{
		Type:      "click",
		Timestamp: time.Now().UnixMilli(),
		URL:       "http://example.com",
	}
}

// Helper: recalculate running memory totals from current slices.
// Must be called with lock held.
func recalcMemoryTotals(c *Capture) {
	c.wsMemoryTotal = 0
	for i := range c.wsEvents {
		c.wsMemoryTotal += wsEventMemory(&c.wsEvents[i])
	}
	c.nbMemoryTotal = 0
	for i := range c.networkBodies {
		c.nbMemoryTotal += nbEntryMemory(&c.networkBodies[i])
	}
}

// bruteForceWSMemory recalculates WS memory by iterating all events (reference implementation).
func bruteForceWSMemory(events []WebSocketEvent) int64 {
	var total int64
	for i := range events {
		total += int64(len(events[i].Data)) + wsEventOverhead
	}
	return total
}

// bruteForceNBMemory recalculates NB memory by iterating all bodies (reference implementation).
func bruteForceNBMemory(bodies []NetworkBody) int64 {
	var total int64
	for i := range bodies {
		total += int64(len(bodies[i].RequestBody)+len(bodies[i].ResponseBody)) + networkBodyOverhead
	}
	return total
}

// ============================================
// Per-Entry Memory Estimation
// ============================================

func TestMemory_CalcWSMemory_PerEventEstimate(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	dataSize := 1000
	c.mu.Lock()
	c.wsEvents = append(c.wsEvents, makeWSEvent(dataSize))
	c.wsAddedAt = append(c.wsAddedAt, time.Now())
	recalcMemoryTotals(c)
	c.mu.Unlock()

	c.mu.RLock()
	mem := c.calcWSMemory()
	c.mu.RUnlock()

	expectedMin := int64(dataSize + 100)
	expectedMax := int64(dataSize + 400)

	if mem < expectedMin || mem > expectedMax {
		t.Errorf("calcWSMemory() = %d, expected between %d and %d for %d-byte data",
			mem, expectedMin, expectedMax, dataSize)
	}
}

func TestMemory_CalcNBMemory_PerEntryEstimate(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	reqSize, respSize := 500, 1500
	c.mu.Lock()
	c.networkBodies = append(c.networkBodies, makeNetworkBody(reqSize, respSize))
	c.networkAddedAt = append(c.networkAddedAt, time.Now())
	recalcMemoryTotals(c)
	c.mu.Unlock()

	c.mu.RLock()
	mem := c.calcNBMemory()
	c.mu.RUnlock()

	expectedMin := int64(reqSize + respSize + 50)
	expectedMax := int64(reqSize + respSize + 500)

	if mem < expectedMin || mem > expectedMax {
		t.Errorf("calcNBMemory() = %d, expected between %d and %d for %d+%d byte bodies",
			mem, expectedMin, expectedMax, reqSize, respSize)
	}
}

func TestMemory_CalcActionMemory_PerEntryEstimate(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.mu.Lock()
	c.enhancedActions = append(c.enhancedActions, makeAction())
	c.actionAddedAt = append(c.actionAddedAt, time.Now())
	c.mu.Unlock()

	c.mu.RLock()
	mem := c.calcActionMemory()
	c.mu.RUnlock()

	expected := int64(500)
	if mem != expected {
		t.Errorf("calcActionMemory() = %d, expected %d for 1 action", mem, expected)
	}
}

func TestMemory_EmptyBuffers_ZeroMemory(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.mu.RLock()
	ws := c.calcWSMemory()
	nb := c.calcNBMemory()
	actions := c.calcActionMemory()
	c.mu.RUnlock()

	if ws != 0 || nb != 0 || actions != 0 {
		t.Errorf("expected all zero for empty buffers, got ws=%d nb=%d actions=%d", ws, nb, actions)
	}
}

// ============================================
// Running Total Accuracy
// ============================================

func TestMemory_RunningTotal_WSAccurateAfterAdd(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	events := []WebSocketEvent{
		makeWSEvent(500),
		makeWSEvent(1000),
		makeWSEvent(2000),
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.wsMemoryTotal
	expected := bruteForceWSMemory(c.wsEvents)
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterAdd(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	bodies := []NetworkBody{
		makeNetworkBody(500, 500),
		makeNetworkBody(1000, 2000),
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.nbMemoryTotal
	expected := bruteForceNBMemory(c.networkBodies)
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("nbMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_WSAccurateAfterRotation(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Fill to capacity, then add more to trigger ring buffer rotation
	events := make([]WebSocketEvent, MaxWSEvents+10)
	for i := range events {
		events[i] = makeWSEvent(100 + i)
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.wsMemoryTotal
	expected := bruteForceWSMemory(c.wsEvents)
	count := len(c.wsEvents)
	c.mu.RUnlock()

	if count > MaxWSEvents {
		t.Errorf("expected at most %d events, got %d", MaxWSEvents, count)
	}
	if runningTotal != expected {
		t.Errorf("after rotation: wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterRotation(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Fill to capacity, then add more to trigger ring buffer rotation
	bodies := make([]NetworkBody, MaxNetworkBodies+5)
	for i := range bodies {
		bodies[i] = makeNetworkBody(100+i, 200+i)
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.nbMemoryTotal
	expected := bruteForceNBMemory(c.networkBodies)
	count := len(c.networkBodies)
	c.mu.RUnlock()

	if count > MaxNetworkBodies {
		t.Errorf("expected at most %d bodies, got %d", MaxNetworkBodies, count)
	}
	if runningTotal != expected {
		t.Errorf("after rotation: nbMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_WSAccurateAfterPerBufferEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add events that exceed per-buffer WS memory limit (4MB)
	// Each event with 50KB data = ~50200 bytes; 100 events = ~5MB
	events := make([]WebSocketEvent, 100)
	for i := range events {
		events[i] = makeWSEvent(50000)
	}
	c.AddWebSocketEvents(events)

	c.mu.RLock()
	runningTotal := c.wsMemoryTotal
	expected := bruteForceWSMemory(c.wsEvents)
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer WS eviction: wsMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_NBAccurateAfterPerBufferEviction(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add bodies that exceed per-buffer NB memory limit (8MB)
	bodies := make([]NetworkBody, 100)
	for i := range bodies {
		bodies[i] = makeNetworkBody(maxRequestBodySize, maxResponseBodySize)
	}
	c.AddNetworkBodies(bodies)

	c.mu.RLock()
	runningTotal := c.nbMemoryTotal
	expected := bruteForceNBMemory(c.networkBodies)
	c.mu.RUnlock()

	if runningTotal != expected {
		t.Errorf("after per-buffer NB eviction: nbMemoryTotal = %d, brute force = %d", runningTotal, expected)
	}
}

func TestMemory_RunningTotal_ZeroAfterClearAll(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(1000), makeWSEvent(2000)})
	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(500, 500), makeNetworkBody(1000, 1000)})

	c.mu.RLock()
	wsBefore := c.wsMemoryTotal
	nbBefore := c.nbMemoryTotal
	c.mu.RUnlock()

	if wsBefore == 0 {
		t.Fatal("expected non-zero wsMemoryTotal before ClearAll")
	}
	if nbBefore == 0 {
		t.Fatal("expected non-zero nbMemoryTotal before ClearAll")
	}

	c.ClearAll()

	c.mu.RLock()
	wsAfter := c.wsMemoryTotal
	nbAfter := c.nbMemoryTotal
	c.mu.RUnlock()

	if wsAfter != 0 {
		t.Errorf("expected wsMemoryTotal = 0 after ClearAll, got %d", wsAfter)
	}
	if nbAfter != 0 {
		t.Errorf("expected nbMemoryTotal = 0 after ClearAll, got %d", nbAfter)
	}
}

func TestMemory_CalcWSMemory_ReturnsRunningTotal(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})

	c.mu.RLock()
	calcResult := c.calcWSMemory()
	runningTotal := c.wsMemoryTotal
	c.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcWSMemory() = %d, wsMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_CalcNBMemory_ReturnsRunningTotal(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(500, 500)})

	c.mu.RLock()
	calcResult := c.calcNBMemory()
	runningTotal := c.nbMemoryTotal
	c.mu.RUnlock()

	if calcResult != runningTotal {
		t.Errorf("calcNBMemory() = %d, nbMemoryTotal = %d; expected equal", calcResult, runningTotal)
	}
}

func TestMemory_RunningTotal_MultipleAddEvictCycles(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Cycle 1: add events
	c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(500), makeWSEvent(1000)})
	c.AddNetworkBodies([]NetworkBody{makeNetworkBody(200, 300)})

	// Cycle 2: add more (may trigger rotation if near capacity)
	for i := 0; i < 5; i++ {
		c.AddWebSocketEvents([]WebSocketEvent{makeWSEvent(100 * (i + 1))})
		c.AddNetworkBodies([]NetworkBody{makeNetworkBody(50*(i+1), 75*(i+1))})
	}

	c.mu.RLock()
	wsRunning := c.wsMemoryTotal
	wsExpected := bruteForceWSMemory(c.wsEvents)
	nbRunning := c.nbMemoryTotal
	nbExpected := bruteForceNBMemory(c.networkBodies)
	c.mu.RUnlock()

	if wsRunning != wsExpected {
		t.Errorf("after multiple cycles: wsMemoryTotal = %d, brute force = %d", wsRunning, wsExpected)
	}
	if nbRunning != nbExpected {
		t.Errorf("after multiple cycles: nbMemoryTotal = %d, brute force = %d", nbRunning, nbExpected)
	}
}
