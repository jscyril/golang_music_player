package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestConfigMarshal tests JSON marshalling of Config struct
func TestConfigMarshal(t *testing.T) {
	config := &Config{
		MusicDirectories: []string{"/home/user/Music", "/mnt/external/songs"},
		DefaultVolume:    0.75,
		Theme:            "dark",
		EnableCache:      true,
		CachePath:        ".cache/player",
		KeyBindings: KeyMap{
			PlayPause:  " ",
			Stop:       "s",
			Next:       "n",
			Previous:   "p",
			VolumeUp:   "+",
			VolumeDown: "-",
			Quit:       "q",
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	// Verify JSON contains expected fields
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result["default_volume"].(float64) != 0.75 {
		t.Errorf("Expected volume 0.75, got %v", result["default_volume"])
	}

	if result["theme"].(string) != "dark" {
		t.Errorf("Expected theme 'dark', got %v", result["theme"])
	}

	dirs := result["music_directories"].([]interface{})
	if len(dirs) != 2 {
		t.Errorf("Expected 2 music directories, got %d", len(dirs))
	}
}

// TestConfigUnmarshal tests JSON unmarshalling of Config struct
func TestConfigUnmarshal(t *testing.T) {
	jsonData := `{
        "music_directories": ["/home/user/Music"],
        "default_volume": 0.8,
        "theme": "light",
        "enable_cache": false,
        "cache_path": "/tmp/cache",
        "key_bindings": {
            "play_pause": "p",
            "stop": "x",
            "next": ">",
            "previous": "<",
            "volume_up": "=",
            "volume_down": "-",
            "quit": "q"
        }
    }`

	var config Config
	if err := json.Unmarshal([]byte(jsonData), &config); err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	if config.DefaultVolume != 0.8 {
		t.Errorf("Expected volume 0.8, got %f", config.DefaultVolume)
	}

	if config.Theme != "light" {
		t.Errorf("Expected theme 'light', got %s", config.Theme)
	}

	if config.EnableCache != false {
		t.Errorf("Expected cache disabled, got enabled")
	}

	if len(config.MusicDirectories) != 1 {
		t.Errorf("Expected 1 directory, got %d", len(config.MusicDirectories))
	}

	if config.KeyBindings.PlayPause != "p" {
		t.Errorf("Expected play_pause 'p', got %s", config.KeyBindings.PlayPause)
	}
}

// TestConfigRoundTrip tests marshal -> unmarshal preserves data
func TestConfigRoundTrip(t *testing.T) {
	original := GetDefaultConfig()
	original.MusicDirectories = []string{"/test/path"}
	original.DefaultVolume = 0.65

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var restored Config
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Compare
	if original.DefaultVolume != restored.DefaultVolume {
		t.Errorf("Volume mismatch: %f != %f", original.DefaultVolume, restored.DefaultVolume)
	}

	if original.Theme != restored.Theme {
		t.Errorf("Theme mismatch: %s != %s", original.Theme, restored.Theme)
	}

	if len(original.MusicDirectories) != len(restored.MusicDirectories) {
		t.Errorf("Directories count mismatch")
	}
}

// TestSaveLoadConfig tests file operations
func TestSaveLoadConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	original := &Config{
		MusicDirectories: []string{"/test/music"},
		DefaultVolume:    0.9,
		Theme:            "custom",
		EnableCache:      true,
	}

	// Save
	if err := SaveConfig(original, configPath); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load
	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.DefaultVolume != original.DefaultVolume {
		t.Errorf("Volume mismatch after load")
	}

	if loaded.Theme != original.Theme {
		t.Errorf("Theme mismatch after load")
	}
}

// TestLoadConfigNotExists tests loading non-existent config
func TestLoadConfigNotExists(t *testing.T) {
	config, err := LoadConfig("/non/existent/path.json")
	if err != nil {
		t.Fatalf("Should return default on missing file: %v", err)
	}

	expected := GetDefaultConfig()
	if config.DefaultVolume != expected.DefaultVolume {
		t.Error("Should return default config values")
	}
}

// TestInvalidJSON tests error handling for malformed JSON
func TestInvalidJSON(t *testing.T) {
	invalidJSON := `{"music_directories": [1, 2, 3], "volume":}`

	var config Config
	err := json.Unmarshal([]byte(invalidJSON), &config)

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

// TestGetDefaultConfig verifies default config values
func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config.DefaultVolume != 0.5 {
		t.Errorf("Expected default volume 0.5, got %f", config.DefaultVolume)
	}

	if config.Theme != "dark" {
		t.Errorf("Expected default theme 'dark', got %s", config.Theme)
	}

	if config.KeyBindings.PlayPause != " " {
		t.Errorf("Expected default play_pause ' ', got %s", config.KeyBindings.PlayPause)
	}

	if config.KeyBindings.Quit != "q" {
		t.Errorf("Expected default quit 'q', got %s", config.KeyBindings.Quit)
	}
}
