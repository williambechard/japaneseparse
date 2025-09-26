package enamdict

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

// Add tests for parseEnamdictLine, LoadEnamdict, and LookupEnamdictFlexible here.

func TestParseEnamdictLine(t *testing.T) {
	// Adjusted test data to match the expected format for POS extraction
	line := "example_lemma [example_reading] /(POS) Meaning/"
	expected := EnamdictEntry{
		Lemma:   "example_lemma",
		Reading: "example_reading",
		POS:     "POS",
		Meaning: "Meaning",
	}

	entry, ok := parseEnamdictLine(line)
	if !ok {
		t.Fatalf("Expected parseEnamdictLine to succeed, but it failed")
	}

	if entry != expected {
		t.Errorf("Parsed entry does not match expected. Got %+v, expected %+v", entry, expected)
	}

	// Test with an invalid line
	invalidLine := "invalid_line"
	_, ok = parseEnamdictLine(invalidLine)
	if ok {
		t.Errorf("Expected parseEnamdictLine to fail for invalid line, but it succeeded")
	}
}

func TestLoadEnamdict(t *testing.T) {
	// Create a temporary ENAMDICT file
	tempFile, err := ioutil.TempFile("", "enamdict_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Adjusted test content to match the expected format for POS extraction
	testContent := "example_lemma [example_reading] /(POS) Meaning/\n"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	tempFile.Close()

	// Test LoadEnamdict
	entries, err := LoadEnamdict(tempFile.Name())
	if err != nil {
		t.Fatalf("LoadEnamdict failed: %v", err)
	}

	expectedKey := "example_lemma|example_reading"
	expectedEntry := EnamdictEntry{
		Lemma:   "example_lemma",
		Reading: "example_reading",
		POS:     "POS",
		Meaning: "Meaning",
	}

	entry, exists := entries[expectedKey]
	if !exists {
		t.Errorf("Expected key %s not found in entries", expectedKey)
	}

	if entry != expectedEntry {
		t.Errorf("Loaded entry does not match expected. Got %+v, expected %+v", entry, expectedEntry)
	}
}

func TestLookupEnamdictFlexible(t *testing.T) {
	// Create a temporary ENAMDICT file
	tempFile, err := ioutil.TempFile("", "enamdict_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testContent := "example_lemma [example_reading] /POS/ Meaning/\n"
	if _, err := tempFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temporary file: %v", err)
	}
	tempFile.Close()

	// Test LookupEnamdictFlexible
	idxs, lines, err := LookupEnamdictFlexible(tempFile.Name(), "example_lemma", "exact", 10, 1)
	if err != nil {
		t.Fatalf("LookupEnamdictFlexible failed: %v", err)
	}

	if len(idxs) != 1 {
		t.Errorf("Expected 1 match, got %d", len(idxs))
	}

	if len(lines) == 0 || !strings.Contains(lines[0], "example_lemma") {
		t.Errorf("Expected lines to contain 'example_lemma', got %v", lines)
	}
}
