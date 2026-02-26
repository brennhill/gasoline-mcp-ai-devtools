// testpages.go — Embedded test/demo pages served at /tests/.
// Includes a WebSocket echo server at /tests/ws and deterministic error
// endpoints at /tests/404, /tests/500, /tests/cors-test, /tests/slow
// for use by the Gasoline smoke-test harness.
package main

import (
	"bufio"
	"crypto/sha1"
	"embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed testpages
var testPagesFS embed.FS

// maxWSPayload caps incoming frame payloads to prevent DoS via oversized allocation.
const maxWSPayload = 1 << 20 // 1 MiB

// wsIdleTimeout is the per-read deadline, reset after every successful frame.
// This is an idle timeout — an active connection that keeps sending frames will
// never be cut. An idle connection is closed after this duration.
const wsIdleTimeout = 60 * time.Second

// handleTestPages serves embedded test pages at /tests/.
// GET /tests/          → branded HTML index listing all pages.
// GET /tests/{file}    → embedded HTML/CSS file.
// GET /tests/404       → 404 response (network error test).
// GET /tests/500       → 500 response (network error test).
// GET /tests/cors-test → 200 JSON, no CORS headers (CORS-block test).
// GET /tests/slow      → 200 after 3 s delay (latency/waterfall test).
func handleTestPages() http.HandlerFunc {
	sub, err := fs.Sub(testPagesFS, "testpages")
	if err != nil {
		panic(fmt.Sprintf("testpages: embed misconfigured: %v", err))
	}
	fileServer := http.StripPrefix("/tests/", http.FileServerFS(sub))

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		trimmed := strings.TrimPrefix(r.URL.Path, "/tests")
		trimmed = strings.TrimPrefix(trimmed, "/")

		switch trimmed {
		case "":
			serveTestIndex(w)
		case "404":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintln(w, `{"error":"not_found","endpoint":"/tests/404","test":"network_error"}`)
		case "500":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintln(w, `{"error":"internal_server_error","endpoint":"/tests/500","test":"network_error"}`)
		case "cors-test":
			// Intentionally omit CORS headers — browser blocks cross-origin fetch.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"cors":"no_headers","test":"cors_block"}`)
		case "slow":
			time.Sleep(3 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintln(w, `{"delayed":true,"delay_ms":3000,"test":"slow_response"}`)
		default:
			fileServer.ServeHTTP(w, r)
		}
	}
}

// handleTestHarnessWS upgrades a GET /tests/ws request to a WebSocket echo
// server implemented with zero external dependencies (net/http hijacking).
// Note: CORS headers set by corsMiddleware are buffered in http.ResponseWriter
// but are not included in the 101 handshake written directly to the hijacked
// connection. This is intentional — WebSocket upgrade bypasses HTTP CORS.
func handleTestHarnessWS(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" || strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		http.Error(w, "websocket upgrade required", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "server does not support hijacking", http.StatusInternalServerError)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Send the 101 handshake.
	accept := wsAcceptKey(key)
	handshake := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n\r\n",
		accept,
	)
	if _, err := bufrw.WriteString(handshake); err != nil {
		return
	}
	if err := bufrw.Flush(); err != nil {
		return
	}

	wsEchoLoop(conn, bufrw)
}

// wsEchoLoop reads WebSocket frames and echoes data frames.
// Text frames are echoed as a JSON envelope; binary frames are echoed as-is.
// The per-read deadline is refreshed after every successful frame, implementing
// an idle timeout: active connections are never cut; idle ones time out after
// wsIdleTimeout.
func wsEchoLoop(conn net.Conn, rw *bufio.ReadWriter) {
	// fragOpcode and fragBuf accumulate fragments for reassembly (RFC 6455 §5.4).
	var fragOpcode byte
	var fragBuf []byte

	for {
		// Reset the idle deadline before each read. A connection that sends
		// at least one frame per wsIdleTimeout period is never disconnected.
		_ = conn.SetReadDeadline(time.Now().Add(wsIdleTimeout))

		fin, opcode, payload, err := wsReadFrame(rw)
		if err != nil {
			return
		}

		// Control frames (0x8–0xF) must not be fragmented (RFC 6455 §5.5) and
		// are dispatched immediately regardless of any in-progress fragment sequence.
		if opcode >= 0x8 {
			switch opcode {
			case 0x8: // Close — echo close frame and exit.
				_ = wsWriteFrame(rw, 0x8, nil)
				return
			case 0x9: // Ping → Pong
				if err := wsWriteFrame(rw, 0xA, payload); err != nil {
					return
				}
			}
			continue
		}

		// Data frame fragmentation reassembly (RFC 6455 §5.4).
		if opcode != 0x0 {
			// Non-zero opcode = first (or only) frame of a data message.
			if !fin {
				// First fragment: record opcode and begin accumulation.
				fragOpcode = opcode
				fragBuf = append(fragBuf[:0], payload...)
				continue
			}
			// FIN=1 with non-continuation opcode = single unfragmented message.
		} else {
			// Continuation frame (opcode 0x0): append to accumulation buffer.
			fragBuf = append(fragBuf, payload...)
			if !fin {
				continue // More fragments incoming.
			}
			// Final fragment — reassemble the full message.
			opcode = fragOpcode
			payload = fragBuf
			fragBuf = fragBuf[:0]
		}

		// Dispatch complete (possibly reassembled) message.
		switch opcode {
		case 0x1: // Text → echo as JSON envelope
			reply, _ := json.Marshal(map[string]any{
				"type":   "echo",
				"echo":   string(payload),
				"server": "gasoline-test-harness",
				"ts":     time.Now().UnixMilli(),
			})
			if err := wsWriteFrame(rw, 0x1, reply); err != nil {
				return
			}
		case 0x2: // Binary → echo binary
			if err := wsWriteFrame(rw, 0x2, payload); err != nil {
				return
			}
		}
	}
}

// wsReadFrame reads one complete WebSocket frame, handling masking.
// Returns the FIN bit, opcode, unmasked payload, and any I/O error.
// Payloads larger than maxWSPayload are rejected to prevent DoS.
func wsReadFrame(r io.Reader) (fin bool, opcode byte, payload []byte, err error) {
	header := make([]byte, 2)
	if _, err = io.ReadFull(r, header); err != nil {
		return
	}
	fin = header[0]&0x80 != 0
	opcode = header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err = io.ReadFull(r, ext); err != nil {
			return
		}
		length = uint64(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err = io.ReadFull(r, ext); err != nil {
			return
		}
		length = binary.BigEndian.Uint64(ext)
	}

	if length > maxWSPayload {
		err = fmt.Errorf("ws: frame payload %d bytes exceeds limit %d", length, uint64(maxWSPayload))
		return
	}

	var mask [4]byte
	if masked {
		if _, err = io.ReadFull(r, mask[:]); err != nil {
			return
		}
	}

	payload = make([]byte, length)
	if _, err = io.ReadFull(r, payload); err != nil {
		return
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

// wsWriteFrame writes one unmasked WebSocket frame (FIN=1, server→client).
// Payload length is encoded per RFC 6455 §5.2, including the full 8-byte
// big-endian form for payloads ≥ 65536 bytes.
func wsWriteFrame(w *bufio.ReadWriter, opcode byte, payload []byte) error {
	length := uint64(len(payload))
	header := []byte{0x80 | opcode}
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length < 65536:
		header = append(header, 126,
			byte(length>>8), byte(length))
	default:
		// Full 8-byte big-endian uint64 per RFC 6455 §5.2.
		header = append(header, 127,
			byte(length>>56), byte(length>>48), byte(length>>40), byte(length>>32),
			byte(length>>24), byte(length>>16), byte(length>>8), byte(length))
	}
	if _, err := w.Write(append(header, payload...)); err != nil {
		return err
	}
	return w.Flush()
}

// wsAcceptKey computes the Sec-WebSocket-Accept value per RFC 6455.
func wsAcceptKey(key string) string {
	const guid = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	h := sha1.New()
	h.Write([]byte(key + guid))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// serveTestIndex generates a branded HTML index of available test pages.
// File names and labels are HTML-escaped to prevent injection from any
// unexpected entries in the embedded filesystem.
func serveTestIndex(w http.ResponseWriter) {
	entries, err := fs.ReadDir(testPagesFS, "testpages")
	if err != nil {
		http.Error(w, "failed to read test pages", http.StatusInternalServerError)
		return
	}

	var links []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".html") {
			continue
		}
		safeName := html.EscapeString(name)
		safeLabel := html.EscapeString(strings.TrimSuffix(name, path.Ext(name)))
		links = append(links, fmt.Sprintf(`<li><a href="/tests/%s">%s</a></li>`, safeName, safeLabel))
	}

	body := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>Gasoline Test Pages</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;
     background:#1a1a1a;color:#e0e0e0;padding:40px;max-width:600px;margin:0 auto}
h1{font-size:20px;color:#fff;margin-bottom:4px}
p{font-size:13px;color:#888;margin-bottom:24px}
ul{list-style:none;padding:0}
li{margin:6px 0}
a{color:#58a6ff;text-decoration:none;font-size:14px}
a:hover{text-decoration:underline}
.ep{font-family:monospace;font-size:12px;color:#888;margin-top:20px;padding-top:16px;border-top:1px solid #333}
.ep a{color:#d29922}
</style></head>
<body>
<h1>🔥 Gasoline — Test Harness</h1>
<p>Deterministic smoke-test pages served by the Gasoline Go daemon.</p>
<ul>%s</ul>
<div class="ep">
  <div>Test endpoints:</div>
  <div><a href="/tests/404">/tests/404</a> — 404 Not Found</div>
  <div><a href="/tests/500">/tests/500</a> — 500 Server Error</div>
  <div><a href="/tests/cors-test">/tests/cors-test</a> — no CORS headers</div>
  <div><a href="/tests/slow">/tests/slow</a> — 3 s delay</div>
  <div>ws://{host}/tests/ws — WebSocket echo</div>
</div>
</body></html>`, strings.Join(links, "\n"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, body)
}
