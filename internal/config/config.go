package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds application configuration
type Config struct {
	MusicDirectories []string `json:"music_directories"`
	DefaultVolume    float64  `json:"default_volume"`
	Theme            string   `json:"theme"`
	KeyBindings      KeyMap   `json:"key_bindings"`
	EnableCache      bool     `json:"enable_cache"`
	CachePath        string   `json:"cache_path"`
	DataDir          string   `json:"data_dir"`
}

// KeyMap defines keyboard shortcuts
type KeyMap struct {
	PlayPause   string `json:"play_pause"`
	Stop        string `json:"stop"`
	Next        string `json:"next"`
	Previous    string `json:"previous"`
	VolumeUp    string `json:"volume_up"`
	VolumeDown  string `json:"volume_down"`
	SeekForward string `json:"seek_forward"`
	SeekBack    string `json:"seek_back"`
	Quit        string `json:"quit"`
	Search      string `json:"search"`
	Library     string `json:"library"`
	Playlist    string `json:"playlist"`
}

// GetDefaultConfig returns default configuration
func GetDefaultConfig() *Config {
	return &Config{
		MusicDirectories: []string{},
		DefaultVolume:    0.5,
		Theme:            "dark",
		EnableCache:      true,
		CachePath:        ".cache/musicplayer",
		DataDir:          "./data",
		KeyBindings: KeyMap{
			PlayPause:   " ",
			Stop:        "s",
			Next:        "n",
			Previous:    "p",
			VolumeUp:    "+",
			VolumeDown:  "-",
			SeekForward: "right",
			SeekBack:    "left",
			Quit:        "q",
			Search:      "/",
			Library:     "l",
			Playlist:    "P",
		},
	}
}

// LoadConfig reads and unmarshals configuration from file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GetDefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// SaveConfig marshals and saves configuration to file
func SaveConfig(config *Config, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadOrCreate loads config from path or creates default if not exists
func LoadOrCreate(path string) (*Config, error) {
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}

	// Save default config if file didn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := SaveConfig(config, path); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
	}

	return config, nil
}

// GetConfigPath returns the default config file path
func GetConfigPath() string {
	// Check environment variable first
	if path := os.Getenv("MUSIC_PLAYER_CONFIG"); path != "" {
		return path
	}

	// Use XDG config directory if available
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "musicplayer", "config.json")
	}

	// Fall back to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "./config.json"
	}

	return filepath.Join(home, ".config", "musicplayer", "config.json")
}
