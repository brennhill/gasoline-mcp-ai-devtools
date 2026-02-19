// stdio_test.go — Tests for ReadStdioMessage.
package bridge

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
)

const testMaxBodySize = 10 * 1024 * 1024

func frameMessage(payload string) string {
	return fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(payload), payload)
}

func TestReadStdioMessage_LineDelimitedJSON(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	r := bufio.NewReader(strings.NewReader(input))

	msg, err := ReadStdioMessage(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessage returned error: %v", err)
	}
	if got, want := string(msg), `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestReadStdioMessage_ContentLengthFramedJSON(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	r := bufio.NewReader(strings.NewReader(frameMessage(payload)))

	msg, err := ReadStdioMessage(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessage returned error: %v", err)
	}
	if got := string(msg); got != payload {
		t.Fatalf("message = %q, want %q", got, payload)
	}
}

func TestReadStdioMessage_BackToBackFramedMessages(t *testing.T) {
	first := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	second := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	input := frameMessage(first) + frameMessage(second)
	r := bufio.NewReader(strings.NewReader(input))

	msg1, err := ReadStdioMessage(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessage first returned error: %v", err)
	}
	if got := string(msg1); got != first {
		t.Fatalf("first message = %q, want %q", got, first)
	}

	msg2, err := ReadStdioMessage(r, testMaxBodySize)
	if err != nil {
		t.Fatalf("ReadStdioMessage second returned error: %v", err)
	}
	if got := string(msg2); got != second {
		t.Fatalf("second message = %q, want %q", got, second)
	}

	_, err = ReadStdioMessage(r, testMaxBodySize)
	if err == nil {
		t.Fatal("expected EOF after reading all messages, got nil")
	}
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}
