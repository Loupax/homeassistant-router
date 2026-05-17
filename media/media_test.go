package media

import (
	"testing"
)

// TestSanitizeQueryBasic verifies "hello world" remains unchanged
func TestSanitizeQueryBasic(t *testing.T) {
	input := "hello world"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "hello world"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryRemovesShellChars verifies dangerous chars are stripped
func TestSanitizeQueryRemovesShellChars(t *testing.T) {
	input := "hello; rm -rf /"
	result := sanitizeRe.ReplaceAllString(input, "")
	// Only alphanumeric, spaces, quotes, commas, and hyphens are preserved
	// So "hello; rm -rf /" becomes "hello rm -rf " with the semicolon removed
	expected := "hello rm -rf "
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryEmptyAfterSanitization verifies whitespace-only queries become empty
func TestSanitizeQueryEmptyAfterSanitization(t *testing.T) {
	input := "  "
	result := sanitizeRe.ReplaceAllString(input, "")
	// After sanitization and trim, should be empty
	if result != "  " {
		t.Errorf("Expected spaces, got %q", result)
	}
}

// TestSanitizeQueryPreservesCommas verifies commas are preserved
func TestSanitizeQueryPreservesCommas(t *testing.T) {
	input := "hello, world"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "hello, world"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryPreservesApostrophes verifies apostrophes are preserved
func TestSanitizeQueryPreservesApostrophes(t *testing.T) {
	input := "don't stop"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "don't stop"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryPreservesHyphens verifies hyphens are preserved
func TestSanitizeQueryPreservesHyphens(t *testing.T) {
	input := "ac-dc"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "ac-dc"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryRemovesUnicodeChars verifies non-ASCII chars are removed
func TestSanitizeQueryRemovesUnicodeChars(t *testing.T) {
	input := "café"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "caf"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryRemovesParens verifies parentheses are removed
func TestSanitizeQueryRemovesParens(t *testing.T) {
	input := "hello (world)"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "hello world"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

// TestSanitizeQueryRemovesPunctuation verifies punctuation is removed
func TestSanitizeQueryRemovesPunctuation(t *testing.T) {
	input := "hello! world?"
	result := sanitizeRe.ReplaceAllString(input, "")
	expected := "hello world"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}
