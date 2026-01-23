package library

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dhowden/tag"
	"github.com/jscyril/golang_music_player/api"
)

// MetadataReader extracts metadata from audio files
type MetadataReader struct{}

// NewMetadataReader creates a new metadata reader
func NewMetadataReader() *MetadataReader {
	return &MetadataReader{}
}

// Read extracts metadata from an audio file and returns a Track
func (r *MetadataReader) Read(filePath string) (*api.Track, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Generate unique ID from file path
	id := generateTrackID(filePath)

	// Try to read metadata tags
	metadata, err := tag.ReadFrom(file)
	if err != nil {
		// If no tags, return basic track info from filename
		return &api.Track{
			ID:        id,
			Title:     filepath.Base(filePath),
			FilePath:  filePath,
			CreatedAt: time.Now(),
		}, nil
	}

	// Get duration if available (requires seeking back to start)
	var duration time.Duration

	track := &api.Track{
		ID:        id,
		Title:     getOrDefault(metadata.Title(), filepath.Base(filePath)),
		Artist:    getOrDefault(metadata.Artist(), "Unknown Artist"),
		Album:     getOrDefault(metadata.Album(), "Unknown Album"),
		Genre:     getOrDefault(metadata.Genre(), ""),
		Year:      metadata.Year(),
		Duration:  duration,
		FilePath:  filePath,
		CreatedAt: time.Now(),
	}

	// Get track number
	trackNum, _ := metadata.Track()
	track.TrackNum = trackNum

	return track, nil
}

// ReadCoverArt extracts cover art from an audio file
func (r *MetadataReader) ReadCoverArt(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	metadata, err := tag.ReadFrom(file)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}

	if picture := metadata.Picture(); picture != nil {
		return picture.Data, nil
	}

	return nil, nil
}

// generateTrackID creates a unique ID for a track based on its file path
func generateTrackID(filePath string) string {
	hash := md5.Sum([]byte(filePath))
	return fmt.Sprintf("track-%x", hash[:8])
}

// getOrDefault returns the value if non-empty, otherwise returns the default
func getOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
