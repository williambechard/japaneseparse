package logger

import (
	"encoding/json"
	"fmt"
	"os"
)

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
	file := fmt.Sprintf("%s/%s.json", path, id)
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, bytes, 0644)
}
