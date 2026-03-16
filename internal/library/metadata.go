package library

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
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
		// If no tags, compute duration from the audio stream and return basic track info.
		file.Seek(0, 0)
		duration := computeAudioDuration(filePath, file)
		return &api.Track{
			ID:        id,
			Title:     filepath.Base(filePath),
			Duration:  duration,
			FilePath:  filePath,
			CreatedAt: time.Now(),
		}, nil
	}

	// Compute duration by decoding the audio stream.
	// Seek back to the start first (tag.ReadFrom may have advanced the cursor).
	file.Seek(0, 0)
	duration := computeAudioDuration(filePath, file)

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

// computeAudioDuration decodes the audio file to determine its total duration.
// r must be seeked to position 0 before calling. Returns 0 on any error.
func computeAudioDuration(filePath string, r interface {
	Read([]byte) (int, error)
	Seek(int64, int) (int64, error)
	Close() error
}) time.Duration {
	ext := strings.ToLower(filepath.Ext(filePath))

	var streamer beep.StreamSeekCloser
	var format beep.Format
	var err error

	switch ext {
	case ".mp3":
		streamer, format, err = mp3.Decode(r)
	case ".wav":
		streamer, format, err = wav.Decode(r)
	case ".flac":
		streamer, format, err = flac.Decode(r)
	default:
		return 0
	}
	if err != nil {
		return 0
	}
	defer streamer.Close()

	if format.SampleRate <= 0 || streamer.Len() <= 0 {
		return 0
	}
	return format.SampleRate.D(streamer.Len())
}
