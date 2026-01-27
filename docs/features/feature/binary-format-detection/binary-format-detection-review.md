# Binary Format Detection -- Engineering Review

## Executive Summary

The spec proposes a valuable feature: decode binary protocols (protobuf, MessagePack, CBOR, etc.) in captured network/WebSocket traffic so AI agents see structured data instead of hex noise. However, the spec significantly overscopes the implementation relative to Gasoline's zero-dependency Go constraint, underspecifies critical safety bounds on recursive parsing, and the existing codebase already ships a `binary.go` detection layer that the spec does not acknowledge. The gap between what the spec describes (full schemaless decoders for 6+ formats, entropy analysis, format caching, gRPC-Web framing) and what should ship first (Content-Type awareness + the already-implemented magic-byte detection) needs to be closed before implementation begins.

---

## Critical Issues (Must Fix)

### C1. Spec ignores existing implementation

The codebase already has `cmd/dev-console/binary.go` and `cmd/dev-console/binary_test.go` with a working `DetectBinaryFormat()` function that handles MessagePack, protobuf, CBOR, and BSON via magic bytes. The `NetworkBody` and `WebSocketEvent` structs already carry `BinaryFormat` and `FormatConfidence` fields (see `cmd/dev-console/types.go:31-32, 145-146`). Detection is already wired into the ingestion paths in `network.go:41-46` and `websocket.go:32-37`.

The spec proposes `binary_detect.go` as the file location (Section: File Locations), which would create a parallel implementation. The spec must be updated to build on the existing foundation, not replace it.

**Fix:** Audit `binary.go` against the spec. Identify what is already done (magic-byte detection for 4 formats), what needs enhancement (Content-Type checks, structural protobuf validation, entropy analysis), and scope the delta.

### C2. Recursive protobuf decoding without depth/size limits is a denial-of-service vector

Section "Partial Decoding -- Protocol Buffers" says: "Nested messages: recursively parse length-delimited fields that look like valid protobuf." No recursion depth limit is specified. A crafted payload with deeply nested length-delimited fields (e.g., field 1 containing field 1 containing field 1...) will cause unbounded stack growth or CPU spin.

The spec's 10KB decode limit (Section: Edge Cases, "Large binary payloads") bounds the input size but not the recursion depth. A 10KB payload with 5000 levels of nesting at 2 bytes per level is trivially constructable.

**Fix:** Specify a maximum recursion depth (8-16 levels is sufficient for real-world protobuf). Specify a maximum field count per message (e.g., 1000). Both must be enforced at parse time with immediate bailout, not after-the-fact.

### C3. No data contract for the `decoded` / `decoded_content` response fields

Section "MCP Integration" says binary messages will include a `decoded` field in `get_websocket_events` and `content_format` / `decoded_content` in `get_network_bodies`. But no struct definition is provided, and the existing `WebSocketEvent` and `NetworkBody` structs have no such fields. The existing fields are `BinaryFormat string` and `FormatConfidence float64` -- not the richer structure the spec describes.

Adding `decoded_content` as a `json.RawMessage` or `interface{}` to these structs affects every consumer: the MCP tool responses, the redaction engine, the noise filter, the session snapshot comparisons, and the HAR exporter.

**Fix:** Define the exact struct fields that will be added. Use a typed struct, not `interface{}`. Example:

```go
type BinaryDecodeResult struct {
    Format     string          `json:"format"`
    Confidence float64         `json:"confidence"`
    Method     string          `json:"detection_method"` // "content_type", "magic_bytes", "structural", "entropy"
    Decoded    json.RawMessage `json:"decoded,omitempty"`
    Error      string          `json:"decode_error,omitempty"`
    RawSize    int             `json:"raw_size"`
    DecodedSize int            `json:"decoded_size,omitempty"`
}
```

### C4. Zero-dependency constraint vs. full MessagePack/CBOR decoders

Gasoline's core rule (CLAUDE.md, Rule 2): "Go server: stdlib only. Extension: no frameworks, no build tools." The spec proposes full MessagePack and CBOR decoders (Section: Partial Decoding). Writing a correct, fuzz-resistant MessagePack decoder from scratch is non-trivial -- MessagePack has 30+ type codes, extension types, and encoding ambiguities. CBOR is equally complex with tagged values, indefinite-length containers, and canonical encoding considerations.

Building these from scratch without pulling in `github.com/vmihailenco/msgpack` or `github.com/fxamacker/cbor` means accepting the risk of subtle decoding bugs that produce incorrect JSON output, which the AI agent will then reason about incorrectly.

**Fix:** Either (a) scope Phase 1 to detection-only (already implemented) and structural summary (field count, types, sizes) without full decode, or (b) accept that full decoders will be 500-1000 lines each and require extensive fuzz testing. Option (a) is strongly recommended. The AI already gets value from "this is MessagePack with 3 map entries" without needing to see every decoded value.

### C5. Entropy calculation is underspecified and provides marginal value

Section "Detection Hierarchy -- Entropy analysis" proposes Shannon entropy calculation as a fallback classifier. The thresholds (7.5, 6.5, 4.0 bits/byte) are stated without justification. In practice:
- Compressed JSON (gzip) has entropy ~7.8-7.95. The spec's "encrypted or compressed" threshold of 7.5 would misclassify many gzip payloads.
- Base64-encoded data has entropy ~5.95-6.0, which falls in the "unknown binary" range -- not particularly useful.
- The only reliable entropy-based detection is gzip/zlib via magic bytes (1f 8b / 78 xx), which is already a magic-byte check, not an entropy check.

**Fix:** Drop entropy analysis from the spec. Replace with explicit gzip/zlib magic-byte detection (which the spec already mentions as a magic-byte check in the same section). Entropy adds complexity without reliable signal in this context.

---

## Recommendations (Should Consider)

### R1. Content-Type detection should be Phase 1, not bundled with structural detection

The spec's "Detection Hierarchy" puts explicit signals (Content-Type headers) first, which is correct. But the existing `binary.go` implementation skips Content-Type entirely and jumps straight to magic bytes. The highest-value change is to check `NetworkBody.ContentType` before calling `DetectBinaryFormat()`. When the server receives `Content-Type: application/protobuf`, detection is trivial and 1.0 confidence. This should ship independently.

### R2. Format cache invalidation needs a time-based expiry

Section "Format Cache" describes invalidation only on decode failure. If a WebSocket connection sends protobuf for 5 minutes, switches to JSON for 30 minutes, then the cache never invalidates because JSON text won't "fail to decode" -- it just won't match. Add a TTL (e.g., 60s) or a sliding window (cache revalidates every N messages).

### R3. The 100-byte structural detection window for protobuf is too small

Section "Structural detection" says: "The detector parses the first 100 bytes as protobuf wire format." The existing `detectProtobuf()` in `binary.go` only examines the first byte's field tag and wire type. The spec proposes parsing 100 bytes, which is a reasonable improvement, but 100 bytes is often just one or two fields. Consider 256 bytes as the analysis window to get at least 3-5 fields for more reliable detection. This matters because a single valid field tag is also valid in many other formats.

### R4. gRPC-Web framing is a separate concern

Section "Edge Cases -- gRPC-Web streaming" describes parsing gRPC frame headers within HTTP response bodies. This is effectively a transport-layer decoder, not a binary format detector. It should be a separate function (`DetectGRPCWebFrame`) called before `DetectBinaryFormat`, with the extracted payload then passed to the protobuf detector. Mixing framing logic into the format detector creates tight coupling.

### R5. The spec should define behavior when detection disagrees with Content-Type

Example: `Content-Type: application/json` but the body is actually MessagePack (misconfigured server). The spec doesn't say which wins. Recommendation: Content-Type takes priority for format naming, but if the body fails to parse as the declared format, fall back to magic-byte detection and report the mismatch as a warning.

### R6. Test scenario gaps

The 21 test scenarios cover the happy paths well but miss:
- Adversarial inputs: payloads designed to trigger worst-case performance (deeply nested protobuf, maximum-length varint chains, BSON with length pointing past end of buffer)
- Fuzz testing: the spec should require `go test -fuzz` targets for each decoder
- Concurrency: format cache accessed from multiple goroutines (detection runs during `AddNetworkBodies` which holds `capture.mu`, but cache access patterns are unclear)
- Memory: decoded JSON can be 2-3x the binary size (spec Section: Performance Constraints). For a 10KB protobuf message, decoded output is 20-30KB. With 100 network bodies and 500 WebSocket events, worst case is 30KB * 600 = 18MB of decoded content in memory. This should be bounded.

### R7. Performance constraints need benchmarks, not just targets

Section "Performance Constraints" lists targets (e.g., "protobuf schemaless decode under 2ms") but the spec doesn't require benchmark tests. The existing `binary_test.go` has a `BenchmarkDetectBinaryFormat` but it only covers detection, not decoding. Add benchmark requirements for each decoder at 1KB, 10KB, and 100KB payloads.

---

## Implementation Roadmap

### Phase 1: Content-Type awareness (low risk, high value)
1. Add Content-Type-based detection in `AddNetworkBodies()` before the existing `DetectBinaryFormat()` call
2. Map known Content-Types to format names with confidence 1.0
3. Set `detection_method: "content_type"` on the result
4. Add gzip/zlib magic byte detection (0x1f 0x8b / 0x78)
5. Tests: Content-Type mapping, gzip detection, priority over magic bytes
6. **Estimated effort: 1 day**

### Phase 2: Enhanced structural detection (medium risk)
1. Extend `detectProtobuf()` to parse first 256 bytes validating multiple field tags
2. Increase confidence to 0.8 when 3+ valid fields with incrementing field numbers are found
3. Add field-count and wire-type summary to `BinaryFormat.Details` (e.g., "3 fields: varint, length-delimited, varint")
4. Add `BinaryDecodeResult` struct to `types.go` with the fields defined in C3
5. Tests: multi-field protobuf, invalid wire types, fuzz targets
6. **Estimated effort: 2 days**

### Phase 3: Format cache for WebSocket connections (medium risk)
1. Add per-connection format cache in `connectionState`
2. Cache after 3 consecutive same-format detections
3. Invalidate on: decode mismatch, 60-second TTL, connection close
4. Tests: cache hit, invalidation on mismatch, TTL expiry, concurrent access
5. **Estimated effort: 1 day**

### Phase 4: Schemaless protobuf decoder (high risk)
1. Implement varint, fixed32, fixed64, length-delimited field extraction
2. Enforce depth limit (16 levels), field count limit (1000), input size limit (10KB)
3. Attempt UTF-8 decode on length-delimited fields; show byte length otherwise
4. Output as JSON: `{"1": 42, "2": "Hello", "3": {"1": true}}`
5. Add decode result to `NetworkBody` and `WebSocketEvent` via `BinaryDecodeResult`
6. Fuzz tests with `go test -fuzz`, benchmark at 1KB/10KB
7. **Estimated effort: 3-4 days**

### Phase 5: MessagePack/CBOR decoders (high risk, defer)
1. Only pursue if Phase 4 demonstrates clear user value
2. Implement self-describing format decoders with same safety limits
3. Each decoder is 300-500 lines with full type coverage
4. Requires extensive fuzz testing due to format complexity
5. **Estimated effort: 5-7 days total**

### Phase 6: gRPC-Web framing (medium risk, defer)
1. Detect gRPC-Web frame prefix (0x00 data, 0x80 trailers)
2. Extract protobuf payload from frames
3. Chain into Phase 4 protobuf decoder
4. Handle streaming responses with multiple frames
5. **Estimated effort: 2 days**

**Skip entirely:** Entropy analysis, FlatBuffers/Avro decoders (schema-required formats provide minimal value without schema files).

---

## Summary Table

| Issue | Severity | Section | Action |
|-------|----------|---------|--------|
| C1: Ignores existing binary.go | Critical | File Locations | Update spec to reference existing code |
| C2: Unbounded recursion | Critical | Partial Decoding | Add depth + field count limits |
| C3: No response struct defined | Critical | MCP Integration | Define BinaryDecodeResult struct |
| C4: Full decoders vs zero-deps | Critical | Partial Decoding | Phase detection-only first |
| C5: Entropy analysis unreliable | Critical | Detection Hierarchy | Remove; use magic bytes for gzip |
| R1: Content-Type first | High | Detection Hierarchy | Ship as Phase 1 |
| R2: Cache TTL missing | Medium | Format Cache | Add time-based expiry |
| R3: 100-byte window too small | Medium | Structural detection | Increase to 256 bytes |
| R4: gRPC-Web as separate concern | Medium | Edge Cases | Separate function |
| R5: Content-Type mismatch | Low | (missing) | Define precedence rules |
| R6: Test gaps | Medium | Test Scenarios | Add adversarial + fuzz tests |
| R7: Benchmark requirements | Low | Performance | Add benchmark test requirements |
