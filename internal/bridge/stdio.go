// stdio.go — MCP stdio message reader supporting line-delimited and Content-Length framing.
package bridge

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
)

// ReadStdioMessage reads one MCP message from a buffered reader.
// Supports both line-delimited JSON and Content-Length framed messages.
// maxBodySize caps the Content-Length value to prevent memory exhaustion.
func ReadStdioMessage(reader *bufio.Reader, maxBodySize int) ([]byte, error) {
	for {
		firstLineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				trimmed := strings.TrimSpace(string(firstLineBytes))
				if trimmed == "" {
					return nil, io.EOF
				}
				return []byte(trimmed), nil
			}
			return nil, err
		}

		firstLine := strings.TrimSpace(string(firstLineBytes))
		if firstLine == "" {
			continue
		}

		if !strings.HasPrefix(strings.ToLower(firstLine), "content-length:") {
			return []byte(firstLine), nil
		}

		parts := strings.SplitN(firstLine, ":", 2)
		if len(parts) != 2 {
			return []byte(firstLine), nil
		}
		contentLength, convErr := strconv.Atoi(strings.TrimSpace(parts[1]))
		if convErr != nil || contentLength < 0 || contentLength > maxBodySize {
			return []byte(firstLine), nil
		}

		// Consume remaining headers until blank line.
		for {
			headerLine, headerErr := reader.ReadBytes('\n')
			if headerErr != nil {
				if errors.Is(headerErr, io.EOF) {
					return nil, io.EOF
				}
				return nil, headerErr
			}
			if strings.TrimSpace(string(headerLine)) == "" {
				break
			}
		}

		payload := make([]byte, contentLength)
		if _, readErr := io.ReadFull(reader, payload); readErr != nil {
			return nil, readErr
		}
		return bytes.TrimSpace(payload), nil
	}
}
