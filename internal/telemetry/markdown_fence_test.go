// markdown_fence_test.go — Minimal CommonMark fence-walker primitives used
// by TestContract_DefaultEndpointMatchesDocs to detect fenced code blocks
// without pulling in a Markdown parser dependency. CLAUDE.md rule: zero
// production deps; tests stay in-tree.

package telemetry

import "testing"

// markdownFenceOpenInfo reports whether trimmed starts with 3+ of '`' or '~' and
// returns the fence character.
//
// Input contract: `trimmed` MUST already be left-trimmed of whitespace by
// the caller. Per CommonMark §4.5, fences may have up to 3 leading spaces
// (4+ becomes an indented code block); callers handle the indent rule
// before invoking. This helper assumes any leading run of '`' or '~' is
// candidate fence content, not an indent.
func markdownFenceOpenInfo(trimmed string) (bool, byte) {
	for _, ch := range []byte{'`', '~'} {
		if markdownCountLeadingByte(trimmed, ch) >= 3 {
			return true, ch
		}
	}
	return false, 0
}

// markdownCountLeadingByte returns the number of consecutive `ch` bytes at
// the start of s. Returns 0 for empty input or first-byte mismatch.
func markdownCountLeadingByte(s string, ch byte) int {
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return n
}

func TestMarkdownCountLeadingByte(t *testing.T) {
	cases := []struct {
		s    string
		ch   byte
		want int
	}{
		{"", '`', 0},
		{"x", '`', 0},
		{"`", '`', 1},
		{"```", '`', 3},
		{"````x", '`', 4},
		{"```~~~", '~', 0},
		{"~~~", '~', 3},
	}
	for _, tc := range cases {
		if got := markdownCountLeadingByte(tc.s, tc.ch); got != tc.want {
			t.Errorf("markdownCountLeadingByte(%q, %q) = %d, want %d", tc.s, tc.ch, got, tc.want)
		}
	}
}

func TestMarkdownFenceOpenInfo(t *testing.T) {
	cases := []struct {
		name   string
		s      string
		wantOK bool
		wantCh byte
	}{
		{"empty", "", false, 0},
		{"two backticks", "``", false, 0},
		{"three backticks", "```", true, '`'},
		{"four backticks", "````", true, '`'},
		{"three backticks with info", "```go", true, '`'},
		{"three tildes", "~~~", true, '~'},
		{"five tildes", "~~~~~", true, '~'},
		{"two tildes", "~~", false, 0},
		{"mixed prefix", "`~~", false, 0},
		{"plain text", "hello", false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, ch := markdownFenceOpenInfo(tc.s)
			if ok != tc.wantOK || ch != tc.wantCh {
				t.Errorf("markdownFenceOpenInfo(%q) = (%v, %q), want (%v, %q)", tc.s, ok, ch, tc.wantOK, tc.wantCh)
			}
		})
	}
}
