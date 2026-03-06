// Purpose: Implements low-level WebSocket frame codec helpers for the test harness.
// Why: Separates RFC 6455 wire parsing/serialization from connection lifecycle and echo dispatch.

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
)

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
