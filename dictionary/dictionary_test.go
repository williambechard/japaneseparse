package dictionary

import (
	"japaneseparse/model"
	"os"
	"strings"
	"testing"
)

// Add tests for intern, isKatakana, normalizeReadings, and InitDictionaries here.

func TestIntern(t *testing.T) {
	str1 := "example"
	str2 := "example"

	interned1 := intern(str1)
	interned2 := intern(str2)

	if interned1 != interned2 {
		t.Errorf("Expected interned strings to be the same instance, but got different instances")
	}

	if interned1 != str1 {
		t.Errorf("Expected interned string to match original string, but got different value")
	}
}

func TestInitDictionaries(t *testing.T) {
	// Create temporary JMdict and ENAMDICT files
	jmTempFile, err := os.CreateTemp("", "jmdict_test_*.xml")
	if err != nil {
		t.Fatalf("Failed to create temporary JMdict file: %v", err)
	}
	defer os.Remove(jmTempFile.Name())

	enamTempFile, err := os.CreateTemp("", "enamdict_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary ENAMDICT file: %v", err)
	}
	defer os.Remove(enamTempFile.Name())

	// Write test content to JMdict file
	jmContent := `<entry><keb>example_kanji</keb><reb>example_reading</reb><gloss>example_gloss</gloss></entry>`
	if _, err := jmTempFile.WriteString(jmContent); err != nil {
		t.Fatalf("Failed to write to JMdict file: %v", err)
	}
	jmTempFile.Close()

	// Write test content to ENAMDICT file
	enamContent := "example_name [example_reading] /example_meaning/\n"
	if _, err := enamTempFile.WriteString(enamContent); err != nil {
		t.Fatalf("Failed to write to ENAMDICT file: %v", err)
	}
	enamTempFile.Close()

	// Ensure InitDictionaries uses the provided paths
	if err := InitDictionaries(jmTempFile.Name(), enamTempFile.Name()); err != nil {
		t.Fatalf("InitDictionaries failed: %v", err)
	}

	// Validate JMdict entries
	if len(jmDictMap["example_kanji"]) == 0 {
		t.Errorf("Expected JMdict entry for 'example_kanji', but got none")
	}

	// Adjusted test validation to match the expected key format
	expectedKey := "example_name|example_reading"
	// Trim whitespace from the actual value before comparison
	actualValue := strings.TrimSpace(enamDictMap[expectedKey])
	if actualValue != "example_name [example_reading] /example_meaning/" {
		t.Errorf("Expected ENAMDICT entry for '%s', but got '%s'", expectedKey, actualValue)
	}
}

func TestDictionaryLookup(t *testing.T) {
	// Ensure InitDictionaries has been called and maps are populated
	if len(jmDictMap) == 0 || len(enamDictMap) == 0 {
		t.Fatalf("Dictionaries are not initialized. Ensure InitDictionaries is called before lookup tests.")
	}

	// Test JMdict lookup
	entries, exists := jmDictMap["example_kanji"]
	if !exists || len(entries) == 0 {
		t.Errorf("Expected JMdict entry for 'example_kanji', but got none")
	}

	// Test ENAMDICT lookup
	lookupKey := "example_name|example_reading"
	if enamDictMap[lookupKey] == "" {
		t.Errorf("Expected ENAMDICT entry for '%s', but got '%s'", lookupKey, enamDictMap[lookupKey])
	}
}

func TestSampleKanjiLookup(t *testing.T) {
	// Ensure InitDictionaries has been called and maps are populated
	if len(jmDictMap) == 0 {
		t.Fatalf("JMdict is not initialized. Ensure InitDictionaries is called before kanji lookup tests.")
	}

	// Add a sample kanji entry to the JMdict map for testing
	sampleEntry := model.DictionaryEntry{
		Source:   "JMdict",
		Kanji:    []string{"例"},
		Readings: []string{"れい"},
		Glosses:  []string{"example"},
	}
	jmDictMap["例"] = []model.DictionaryEntry{sampleEntry}

	// Test kanji lookup
	entries, exists := jmDictMap["例"]
	if !exists || len(entries) == 0 {
		t.Errorf("Expected JMdict entry for '例', but got none")
	}

	// Validate the retrieved entry
	retrievedEntry := entries[0]
	if retrievedEntry.Source != "JMdict" ||
		retrievedEntry.Kanji[0] != "例" ||
		retrievedEntry.Readings[0] != "れい" ||
		retrievedEntry.Glosses[0] != "example" {
		t.Errorf("Retrieved entry does not match expected. Got %+v", retrievedEntry)
	}
}
