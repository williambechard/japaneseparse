package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the Japanese parser
type Config struct {
	Dictionary DictionaryConfig `yaml:"dictionary"`
	Output     OutputConfig     `yaml:"output"`
	Debug      bool             `yaml:"debug"`
}

// DictionaryConfig holds dictionary file paths
type DictionaryConfig struct {
	JMdictPath   string `yaml:"jmdict_path" env:"JMDICT_PATH"`
	EnamdictPath string `yaml:"enamdict_path" env:"ENAMDICT_PATH"`
	KanjidicPath string `yaml:"kanjidic_path" env:"KANJIDIC_PATH"`
}

// OutputConfig holds output configuration
type OutputConfig struct {
	LogsDir  string `yaml:"logs_dir" env:"LOGS_DIR"`
	SaveLogs bool   `yaml:"save_logs" env:"SAVE_LOGS"`
	Verbose  bool   `yaml:"verbose" env:"VERBOSE"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Dictionary: DictionaryConfig{
			JMdictPath:   "dict/JMdict_e",
			EnamdictPath: "dict/enamdict",
			KanjidicPath: "dict/kanjidic2.xml",
		},
		Output: OutputConfig{
			LogsDir:  "logs",
			SaveLogs: true,
			Verbose:  false,
		},
		Debug: false,
	}
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	config := DefaultConfig()

	// Load from file if specified
	if configPath != "" {
		if err := loadFromFile(config, configPath); err != nil {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	// Override with environment variables
	loadFromEnv(config)

	// Ensure required directories exist
	if err := ensureDirectories(config); err != nil {
		return nil, fmt.Errorf("ensuring directories: %w", err)
	}

	return config, nil
}

// loadFromFile loads configuration from a YAML file
func loadFromFile(config *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	return nil
}

// loadFromEnv loads configuration from environment variables
func loadFromEnv(config *Config) {
	if path := os.Getenv("JMDICT_PATH"); path != "" {
		config.Dictionary.JMdictPath = path
	}
	if path := os.Getenv("ENAMDICT_PATH"); path != "" {
		config.Dictionary.EnamdictPath = path
	}
	if path := os.Getenv("KANJIDIC_PATH"); path != "" {
		config.Dictionary.KanjidicPath = path
	}
	if dir := os.Getenv("LOGS_DIR"); dir != "" {
		config.Output.LogsDir = dir
	}
	if val := os.Getenv("SAVE_LOGS"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Output.SaveLogs = parsed
		}
	}
	if val := os.Getenv("VERBOSE"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Output.Verbose = parsed
		}
	}
	if val := os.Getenv("DEBUG"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			config.Debug = parsed
		}
	}
}

// ensureDirectories creates necessary directories
func ensureDirectories(config *Config) error {
	if config.Output.SaveLogs {
		if err := os.MkdirAll(config.Output.LogsDir, 0755); err != nil {
			return fmt.Errorf("creating logs directory: %w", err)
		}
	}
	return nil
}

// Validate checks that required files and directories exist
func (c *Config) Validate() error {
	// Check dictionary files exist
	dictFiles := map[string]string{
		"JMdict":   c.Dictionary.JMdictPath,
		"ENAMDICT": c.Dictionary.EnamdictPath,
		"Kanjidic": c.Dictionary.KanjidicPath,
	}

	for name, path := range dictFiles {
		if path == "" {
			return fmt.Errorf("%s path not configured", name)
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Errorf("%s file not found: %s", name, path)
		}
	}

	return nil
}

// GetAbsolutePath returns absolute path for a potentially relative path
func (c *Config) GetAbsolutePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}
