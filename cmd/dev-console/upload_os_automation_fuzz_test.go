// upload_os_automation_fuzz_test.go — Fuzz tests for OS automation path validation and sanitizers.

package main

import (
	"strings"
	"testing"
)

func FuzzValidatePathForOSAutomation(f *testing.F) {
	// Normal paths
	f.Add("/Users/test/file.pdf")
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("/tmp/upload.txt")
	f.Add("./relative/path.doc")

	// Injection attempts
	f.Add("path\x00evil")
	f.Add("path\nevil")
	f.Add("path\revil")
	f.Add("path`evil")
	f.Add("\"; do shell script \"rm -rf /\"")
	f.Add("$(whoami)")
	f.Add("path\r\nevil")
	f.Add("`rm -rf /`")
	f.Add("path\x00\x00evil")

	f.Fuzz(func(t *testing.T, input string) {
		err := validatePathForOSAutomation(input)

		// Invariant: if validation passes, input must not contain dangerous chars
		if err == nil {
			if strings.ContainsRune(input, '\x00') {
				t.Errorf("validatePathForOSAutomation accepted path with null byte: %q", input)
			}
			if strings.ContainsRune(input, '\n') {
				t.Errorf("validatePathForOSAutomation accepted path with newline: %q", input)
			}
			if strings.ContainsRune(input, '\r') {
				t.Errorf("validatePathForOSAutomation accepted path with carriage return: %q", input)
			}
			if strings.ContainsRune(input, '`') {
				t.Errorf("validatePathForOSAutomation accepted path with backtick: %q", input)
			}
		}

		// Converse: if input contains dangerous chars, validation must fail
		hasDangerousChars := strings.ContainsAny(input, "\x00\n\r`")
		if hasDangerousChars && err == nil {
			t.Errorf("validatePathForOSAutomation should reject path with dangerous chars: %q", input)
		}
	})
}

func FuzzSanitizeForAppleScript(f *testing.F) {
	// Normal paths
	f.Add("/Users/test/file.pdf")
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("simple-filename.txt")

	// Injection attempts
	f.Add("\"; do shell script \"rm -rf /\"")
	f.Add("path\\\\evil")
	f.Add("path\"evil")
	f.Add("\\\"test\\\"")
	f.Add("quote\"and\\backslash")
	f.Add("\\")
	f.Add("\"")
	f.Add("\"\"\"")
	f.Add("\\\\\\")

	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeForAppleScript(input)

		// Invariant: no bare double quotes in output
		// Remove all properly escaped quotes, then check for remaining quotes
		cleaned := strings.ReplaceAll(result, `\"`, "")
		cleaned = strings.ReplaceAll(cleaned, `\\`, "") // Also remove escaped backslashes

		if strings.Contains(cleaned, `"`) {
			t.Errorf("sanitizeForAppleScript left bare quote in output\ninput:  %q\noutput: %q\ncleaned: %q",
				input, result, cleaned)
		}

		// Additional check: every quote should be preceded by backslash
		for i := 0; i < len(result); i++ {
			if result[i] == '"' {
				if i == 0 || result[i-1] != '\\' {
					t.Errorf("sanitizeForAppleScript has unescaped quote at position %d\ninput:  %q\noutput: %q",
						i, input, result)
					break
				}
			}
		}
	})
}

func FuzzSanitizeForSendKeys(f *testing.F) {
	// Normal paths
	f.Add("C:\\Users\\test\\file.pdf")
	f.Add("D:\\Documents\\report.docx")
	f.Add("simple-filename.txt")

	// Special chars that need escaping
	f.Add("file+name")
	f.Add("file^name")
	f.Add("file%name")
	f.Add("file~name")
	f.Add("file(name)")
	f.Add("file{name}")
	f.Add("all+^%~(){}specials")
	f.Add("+++")
	f.Add("^^^")
	f.Add("{{{}}")
	f.Add("()()")

	f.Fuzz(func(t *testing.T, input string) {
		result := sanitizeForSendKeys(input)

		// Invariant: special chars must be wrapped in braces
		// Remove all properly escaped sequences, then check for bare specials
		cleaned := result
		cleaned = strings.ReplaceAll(cleaned, "{+}", "")
		cleaned = strings.ReplaceAll(cleaned, "{^}", "")
		cleaned = strings.ReplaceAll(cleaned, "{%}", "")
		cleaned = strings.ReplaceAll(cleaned, "{~}", "")
		cleaned = strings.ReplaceAll(cleaned, "{(}", "")
		cleaned = strings.ReplaceAll(cleaned, "{)}", "")
		cleaned = strings.ReplaceAll(cleaned, "{{}", "")
		cleaned = strings.ReplaceAll(cleaned, "{}}", "")

		// Check for bare special characters
		bareSpecials := []struct {
			char string
			name string
		}{
			{"+", "plus"},
			{"^", "caret"},
			{"%", "percent"},
			{"~", "tilde"},
			{"(", "open paren"},
			{")", "close paren"},
			{"{", "open brace"},
			{"}", "close brace"},
		}

		for _, spec := range bareSpecials {
			if strings.Contains(cleaned, spec.char) {
				t.Errorf("sanitizeForSendKeys left bare %s (%q) in output\ninput:  %q\noutput: %q\ncleaned: %q",
					spec.name, spec.char, input, result, cleaned)
				break
			}
		}

		// Additional check: every occurrence of special char in input should appear escaped in output
		specialChars := "+^%~(){}‌"
		for _, char := range specialChars {
			inputCount := strings.Count(input, string(char))
			if inputCount > 0 {
				escapedForm := "{" + string(char) + "}"
				outputCount := strings.Count(result, escapedForm)

				// Output should have at least as many escaped forms as input had bare chars
				if outputCount < inputCount {
					t.Errorf("sanitizeForSendKeys may have lost %q escaping\ninput count: %d, escaped output count: %d\ninput:  %q\noutput: %q",
						string(char), inputCount, outputCount, input, result)
				}
			}
		}
	})
}
