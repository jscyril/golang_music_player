package loader

import (
	"os"
	"path/filepath"
	"strings"
)

// LoadTracks scans a root directory for audio files
func LoadTracks(root string) ([]string, error) {
	var files []string

	// WalkDir is more efficient than Walk
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".mp3" || ext == ".flac" || ext == ".wav" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
