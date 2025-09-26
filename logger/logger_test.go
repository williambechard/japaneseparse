package logger

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestLogf(t *testing.T) {
	// Temporarily enable logging
	os.Setenv("JAPARSE_LOG", "1")
	defer os.Setenv("JAPARSE_LOG", "0")

	// Capture log output
	logFile := "test_log.txt"
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	defer os.Remove(logFile)
	defer file.Close()

	log.SetOutput(file)
	Logf("Test message: %s", "Hello, Logger!")

	file.Close()
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if string(content) == "" {
		t.Errorf("Expected log output, but got empty file")
	}
}

func TestInitLogs(t *testing.T) {
	testDir := "test_logs"
	os.Mkdir(testDir, 0755)
	defer os.RemoveAll(testDir)

	// Create dummy .json files
	os.WriteFile(filepath.Join(testDir, "test1.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(testDir, "test2.json"), []byte("{}"), 0644)

	err := InitLogs(testDir)
	if err != nil {
		t.Fatalf("InitLogs failed: %v", err)
	}

	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read test directory: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Expected all .json files to be cleared, but found %d files", len(files))
	}
}

func TestLogJSON(t *testing.T) {
	testDir := "test_logs"
	os.Mkdir(testDir, 0755)
	defer os.RemoveAll(testDir)

	data := map[string]string{"key": "value"}
	err := LogJSON(testDir, "test", data)
	if err != nil {
		t.Fatalf("LogJSON failed: %v", err)
	}

	filePath := filepath.Join(testDir, "test.json")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read JSON file: %v", err)
	}

	var result map[string]string
	err = json.Unmarshal(content, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if result["key"] != "value" {
		t.Errorf("Expected key 'value', but got '%s'", result["key"])
	}
}
