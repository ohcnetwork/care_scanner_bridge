package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	Port         int    `json:"port"`
	BaudRate     int    `json:"baudRate"`
	AutoConnect  bool   `json:"autoConnect"`
	LastDevice   string `json:"lastDevice"`
	StartMinimized bool `json:"startMinimized"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:         7001,
		BaudRate:     9600,
		AutoConnect:  true,
		StartMinimized: false,
	}
}

// Load loads configuration from file or returns defaults
func Load() *Config {
	cfg := DefaultConfig()
	
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config file doesn't exist, use defaults
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}

	return cfg
}

// Save saves the configuration to file
func (c *Config) Save() error {
	configPath := getConfigPath()
	
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func getConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".care_scanner_bridge", "config.json")
}
