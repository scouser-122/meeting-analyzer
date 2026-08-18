package tui

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TuiConfig holds configuration parameters for the TUI client.
type TuiConfig struct {
	BaseURL string `yaml:"base_url"`
}

// LoadConfig reads and parses the TUI configuration from a YAML file.
func LoadConfig(path string) (*TuiConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg TuiConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8080"
	}

	return &cfg, nil
}
