// Package config handles persistent configuration for claude-usage-monitor.
// Settings are stored at ~/.claude-top/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user-configurable settings for the application.
type Config struct {
	// Source is the data source filter: "all", "claude", or "codex".
	// Defaults to "all" when not set.
	Source string `json:"source,omitempty"`
	// CodexPath is the path to the Codex sessions directory.
	// Defaults to ~/.codex/sessions when empty.
	CodexPath string `json:"codex_path,omitempty"`
}

// configPath returns the path to the config file: ~/.claude-top/config.json.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude-top", "config.json"), nil
}

// Load reads the config from disk. Returns defaults (all sources, default paths)
// when no config file exists or on read errors.
func Load() Config {
	path, err := configPath()
	if err != nil {
		return Config{Source: "all"}
	}
	f, err := os.Open(path)
	if err != nil {
		return Config{Source: "all"}
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return Config{Source: "all"}
	}
	// Validate source field.
	switch cfg.Source {
	case "all", "claude", "codex":
		// valid
	default:
		cfg.Source = "all"
	}
	return cfg
}

// Save writes cfg to ~/.claude-top/config.json, creating parent dirs as needed.
// Errors are returned to the caller; a missing config just means defaults are used.
func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cfg)
}
