package library

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jscyril/golang_music_player/api"
	playerrors "github.com/jscyril/golang_music_player/pkg/errors"
)

// Scanner scans directories concurrently using a worker pool
type Scanner struct {
	workers    int
	formats    []string
	metaReader *MetadataReader
}

// NewScanner creates a new file scanner
func NewScanner(workers int) *Scanner {
	if workers <= 0 {
		workers = 4 // Default worker count
	}
	return &Scanner{
		workers:    workers,
		formats:    []string{".mp3", ".wav", ".flac"},
		metaReader: NewMetadataReader(),
	}
}

// SupportedFormats returns list of supported audio formats
func (s *Scanner) SupportedFormats() []string {
	return s.formats
}

// isSupported checks if a file format is supported
func (s *Scanner) isSupported(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, format := range s.formats {
		if ext == format {
			return true
		}
	}
	return false
}

// Scan scans directories concurrently and returns channels for results and errors
func (s *Scanner) Scan(ctx context.Context, paths []string) (<-chan *api.Track, <-chan error) {
	tracks := make(chan *api.Track, 100)
	errors := make(chan error, 10)
	files := make(chan string, 100)

	var wg sync.WaitGroup

	// Start file discovery goroutine
	go func() {
		defer close(files)
		for _, path := range paths {
			select {
			case <-ctx.Done():
				return
			default:
			}

			err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					select {
					case errors <- &playerrors.ScanError{Path: p, Err: err}:
					default:
					}
					return nil
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if !d.IsDir() && s.isSupported(p) {
					select {
					case files <- p:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			})

			if err != nil && err != context.Canceled {
				select {
				case errors <- &playerrors.ScanError{Path: path, Err: err}:
				default:
				}
			}
		}
	}()

	// Start worker pool
	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range files {
				select {
				case <-ctx.Done():
					return
				default:
				}

				track, err := s.metaReader.Read(filePath)
				if err != nil {
					select {
					case errors <- &playerrors.ScanError{Path: filePath, Err: err}:
					default:
					}
					continue
				}

				select {
				case tracks <- track:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Close channels when done
	go func() {
		wg.Wait()
		close(tracks)
		close(errors)
	}()

	return tracks, errors
}

// ScanFile scans a single file and returns a Track
func (s *Scanner) ScanFile(filePath string) (*api.Track, error) {
	if !s.isSupported(filePath) {
		return nil, playerrors.ErrInvalidFormat
	}
	return s.metaReader.Read(filePath)
}
