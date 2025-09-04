package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// InitLogs ensures the logs directory exists and removes any existing .json files
// so the program starts with a clean logs directory.
func InitLogs(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	pattern := filepath.Join(dir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	for _, f := range files {
		// ignore individual remove errors but continue trying to clean others
		_ = os.Remove(f)
	}
	return nil
}

// LogJSON writes the provided value as pretty JSON to logs/<name>.json. It writes to
// a temporary file first and renames to the final path to reduce chance of partial files.
func LogJSON(dir, name string, v interface{}) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	safe := filepath.Base(name)
	final := filepath.Join(dir, safe+".json")
	tmp := final + ".tmp"
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		// try cleanup of tmp on failure
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
