package audio

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/wav"
	playerrors "github.com/jscyril/golang_music_player/pkg/errors"
)

// SupportedFormats returns list of supported audio formats
func SupportedFormats() []string {
	return []string{".mp3", ".wav", ".flac"}
}

// IsSupported checks if a file format is supported
func IsSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, format := range SupportedFormats() {
		if ext == format {
			return true
		}
	}
	return false
}

// DecodeAudio decodes an audio file based on its extension
func DecodeAudio(r io.ReadSeekCloser, filePath string) (beep.StreamSeekCloser, beep.Format, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".mp3":
		return mp3.Decode(r)
	case ".wav":
		return wav.Decode(r)
	case ".flac":
		return flac.Decode(r)
	default:
		return nil, beep.Format{}, fmt.Errorf("%w: %s", playerrors.ErrInvalidFormat, ext)
	}
}
