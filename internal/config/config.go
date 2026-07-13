package config

import (
	"encoding/json"
	"os"
)

// Config holds application configuration.
type Config struct {
	LogLevel string `json:"log_level"`
	Output   string `json:"output"`
}

// Default returns sane default settings.
func Default() Config {
	return Config{
		LogLevel: "info",
		Output:   "text",
	}
}

// Load reads config from a JSON file at path (if it exists), then applies
// MYCLI_LOG_LEVEL / MYCLI_OUTPUT env var overrides on top.
func Load(path string) (Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
				return cfg, jsonErr
			}
		} else if !os.IsNotExist(err) {
			return cfg, err
		}
	}

	if v := os.Getenv("MYCLI_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("MYCLI_OUTPUT"); v != "" {
		cfg.Output = v
	}

	return cfg, nil
}
