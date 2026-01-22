package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions
var (
	ErrTrackNotFound    = errors.New("track not found")
	ErrPlaylistNotFound = errors.New("playlist not found")
	ErrInvalidFormat    = errors.New("unsupported audio format")
	ErrPlaybackFailed   = errors.New("playback failed")
	ErrEmptyQueue       = errors.New("playback queue is empty")
	ErrInvalidVolume    = errors.New("volume must be between 0.0 and 1.0")
)

// PlayerError wraps errors with additional context
type PlayerError struct {
	Op    string // Operation that failed
	Track string // Track ID if applicable
	Err   error  // Underlying error
}

func (e *PlayerError) Error() string {
	if e.Track != "" {
		return fmt.Sprintf("%s failed for track %s: %v", e.Op, e.Track, e.Err)
	}
	return fmt.Sprintf("%s failed: %v", e.Op, e.Err)
}

func (e *PlayerError) Unwrap() error {
	return e.Err
}

// NewPlayerError creates a new PlayerError
func NewPlayerError(op, track string, err error) *PlayerError {
	return &PlayerError{Op: op, Track: track, Err: err}
}

// ScanError represents an error during library scanning
type ScanError struct {
	Path string
	Err  error
}

func (e *ScanError) Error() string {
	return fmt.Sprintf("scan error at %s: %v", e.Path, e.Err)
}

func (e *ScanError) Unwrap() error {
	return e.Err
}
