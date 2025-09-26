package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

var logEnabled = os.Getenv("JAPARSE_LOG") != "0" && os.Getenv("JAPARSE_LOG") != "false"

func Logf(format string, v ...interface{}) {
	if logEnabled {
		log.Printf(format, v...)
	}
}

func InitLogs(path string) error {
	// Clear all .json files in the logs directory
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, f := range files {
		if !f.IsDir() && len(f.Name()) > 5 && f.Name()[len(f.Name())-5:] == ".json" {
			_ = os.Remove(path + "/" + f.Name())
		}
	}
	return nil
}

func LogJSON(path, id string, data interface{}) error {
	// Debugging: Log the path and id values
	fmt.Printf("DEBUG: LogJSON called with path: %s, id: %s\n", path, id)

	file := fmt.Sprintf("%s/%s.json", path, id)
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Ensure the directory exists before writing
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Add debugging to verify the file path being written
	fmt.Printf("DEBUG: Writing JSON to file: %s\n", file)
	return os.WriteFile(file, bytes, 0644)
}
