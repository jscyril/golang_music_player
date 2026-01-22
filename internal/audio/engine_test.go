package audio

import (
	"testing"

	"github.com/jscyril/golang_music_player/api"
)

func TestNewAudioEngine(t *testing.T) {
	engine := NewAudioEngine()

	if engine == nil {
		t.Fatal("NewAudioEngine returned nil")
	}

	if engine.state == nil {
		t.Error("Engine state is nil")
	}

	if engine.state.Status != api.StatusStopped {
		t.Errorf("Expected status StatusStopped, got %v", engine.state.Status)
	}

	if engine.state.Volume != 0.5 {
		t.Errorf("Expected volume 0.5, got %f", engine.state.Volume)
	}

	if engine.commands == nil {
		t.Error("Commands channel is nil")
	}

	if engine.events == nil {
		t.Error("Events channel is nil")
	}
}

func TestSetVolume_Valid(t *testing.T) {
	engine := NewAudioEngine()

	tests := []struct {
		name    string
		volume  float64
		wantErr bool
	}{
		{"zero volume", 0.0, false},
		{"half volume", 0.5, false},
		{"full volume", 1.0, false},
		{"below zero", -0.1, true},
		{"above one", 1.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.SetVolume(tt.volume)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetVolume(%f) error = %v, wantErr %v", tt.volume, err, tt.wantErr)
			}
		})
	}
}

func TestGetState(t *testing.T) {
	engine := NewAudioEngine()
	state := engine.GetState()

	if state == nil {
		t.Fatal("GetState returned nil")
	}

	if state.Status != api.StatusStopped {
		t.Errorf("Expected StatusStopped, got %v", state.Status)
	}

	// Verify it's a copy, not the original
	state.Volume = 0.99
	originalState := engine.GetState()
	if originalState.Volume == 0.99 {
		t.Error("GetState should return a copy, not the original")
	}
}

func TestPlay_NilTrack(t *testing.T) {
	engine := NewAudioEngine()

	err := engine.Play(nil)
	if err == nil {
		t.Error("Play(nil) should return an error")
	}
}

func TestIsSupported(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/music/song.mp3", true},
		{"/music/song.MP3", true},
		{"/music/song.wav", true},
		{"/music/song.flac", true},
		{"/music/song.ogg", false},
		{"/music/song.aac", false},
		{"/music/song.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsSupported(tt.path)
			if result != tt.expected {
				t.Errorf("IsSupported(%s) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSupportedFormats(t *testing.T) {
	formats := SupportedFormats()

	if len(formats) == 0 {
		t.Error("SupportedFormats should return at least one format")
	}

	expected := map[string]bool{".mp3": true, ".wav": true, ".flac": true}
	for _, f := range formats {
		if !expected[f] {
			t.Errorf("Unexpected format: %s", f)
		}
	}
}
