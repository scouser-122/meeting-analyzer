package telegram

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure loaded from a YAML file.
type TelegramBotConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
	Backend  BackendConfig  `yaml:"backend"`
}

// TelegramConfig holds settings required to connect to Telegram.
type TelegramConfig struct {
	Token string `yaml:"token"`
}

// BackendConfig holds settings required to reach the meeting-processing backend.
type BackendConfig struct {
	BaseURL string        `yaml:"base_url"`
	Timeout time.Duration `yaml:"timeout"`
}

// LoadConfig reads and parses the Telegram bot configuration from a YAML file.
func LoadConfig(path string) (*TelegramBotConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg TelegramBotConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	return &cfg, nil
}
