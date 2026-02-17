---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Agent Assignment: Binary Format Detection

**Branch:** `feature/binary-detection`
**Worktree:** `../gasoline-binary-detection`
**Priority:** P5 (nice-to-have, parallel)

---

## Objective

Detect binary formats (protobuf, MessagePack, CBOR, etc.) in network bodies and WebSocket messages by magic bytes instead of showing hex dumps.

---

## Deliverables

### 1. Format Detection

**File:** `cmd/dev-console/binary.go` (new)

```go
// binary.go — Binary format detection via magic bytes.
// Identifies protobuf, MessagePack, CBOR, BSON, Avro, Thrift.

package main

type BinaryFormat struct {
    Name       string
    Confidence float64 // 0.0 - 1.0
    Details    string  // e.g., "protobuf field 1, varint"
}

// DetectBinaryFormat analyzes bytes and returns detected format
func DetectBinaryFormat(data []byte) *BinaryFormat {
    if len(data) == 0 {
        return nil
    }

    // MessagePack: 0x80-0x8f (fixmap), 0x90-0x9f (fixarray), 0xa0-0xbf (fixstr)
    // or specific type markers
    if isMessagePack(data) {
        return &BinaryFormat{Name: "messagepack", Confidence: 0.8}
    }

    // CBOR: similar to MessagePack but different markers
    if isCBOR(data) {
        return &BinaryFormat{Name: "cbor", Confidence: 0.8}
    }

    // Protobuf: field tags with wire types
    if isProtobuf(data) {
        return &BinaryFormat{Name: "protobuf", Confidence: 0.7}
    }

    // BSON: starts with int32 length
    if isBSON(data) {
        return &BinaryFormat{Name: "bson", Confidence: 0.6}
    }

    return nil
}
```

### 2. Integration Points

**File:** `cmd/dev-console/network.go`

When body is binary (not UTF-8), call `DetectBinaryFormat()` and include in response:
```go
if format := DetectBinaryFormat(body); format != nil {
    entry["binary_format"] = format.Name
    entry["format_confidence"] = format.Confidence
}
```

**File:** `cmd/dev-console/websocket.go`

Same for WebSocket binary messages.

---

## Tests

**File:** `cmd/dev-console/binary_test.go` (new)

1. MessagePack detection with sample payloads
2. Protobuf detection with wire-type patterns
3. CBOR detection
4. Unknown binary returns nil
5. Empty input returns nil
6. Text content returns nil

---

## Verification

```bash
go test -v ./cmd/dev-console/ -run Binary
```

---

## Files Modified

| File | Change |
|------|--------|
| `cmd/dev-console/binary.go` | New file — format detection |
| `cmd/dev-console/network.go` | Call DetectBinaryFormat |
| `cmd/dev-console/websocket.go` | Call DetectBinaryFormat |
| `cmd/dev-console/binary_test.go` | New file |
