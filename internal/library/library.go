package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jscyril/golang_music_player/api"
	playerrors "github.com/jscyril/golang_music_player/pkg/errors"
)

// Library represents the entire music collection
type Library struct {
	Tracks      map[string]*api.Track `json:"tracks"`
	ScanPaths   []string              `json:"scan_paths"`
	LastScanned time.Time             `json:"last_scanned"`
	TotalTracks int                   `json:"total_tracks"`

	// Secondary indices for efficient queries
	artistIndex map[string][]string
	albumIndex  map[string][]string
	genreIndex  map[string][]string

	mu      sync.RWMutex
	scanner *Scanner
}

// NewLibrary creates a new empty library
func NewLibrary() *Library {
	return &Library{
		Tracks:      make(map[string]*api.Track),
		artistIndex: make(map[string][]string),
		albumIndex:  make(map[string][]string),
		genreIndex:  make(map[string][]string),
		scanner:     NewScanner(4),
	}
}

// AddTrack adds a track to the library and updates indices
func (l *Library) AddTrack(track *api.Track) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Tracks[track.ID] = track
	l.TotalTracks = len(l.Tracks)

	// Update indices
	if track.Artist != "" {
		l.artistIndex[track.Artist] = append(l.artistIndex[track.Artist], track.ID)
	}
	if track.Album != "" {
		l.albumIndex[track.Album] = append(l.albumIndex[track.Album], track.ID)
	}
	if track.Genre != "" {
		l.genreIndex[track.Genre] = append(l.genreIndex[track.Genre], track.ID)
	}
}

// GetTrack returns a track by ID
func (l *Library) GetTrack(id string) (*api.Track, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	track, exists := l.Tracks[id]
	if !exists {
		return nil, playerrors.ErrTrackNotFound
	}
	return track, nil
}

// GetAllTracks returns all tracks as a slice
func (l *Library) GetAllTracks() []*api.Track {
	l.mu.RLock()
	defer l.mu.RUnlock()

	tracks := make([]*api.Track, 0, len(l.Tracks))
	for _, track := range l.Tracks {
		tracks = append(tracks, track)
	}

	// Sort by artist, then album, then track number
	sort.Slice(tracks, func(i, j int) bool {
		if tracks[i].Artist != tracks[j].Artist {
			return tracks[i].Artist < tracks[j].Artist
		}
		if tracks[i].Album != tracks[j].Album {
			return tracks[i].Album < tracks[j].Album
		}
		return tracks[i].TrackNum < tracks[j].TrackNum
	})

	return tracks
}

// GetTracksByArtist returns all tracks by a specific artist
func (l *Library) GetTracksByArtist(artist string) []*api.Track {
	l.mu.RLock()
	defer l.mu.RUnlock()

	trackIDs, exists := l.artistIndex[artist]
	if !exists {
		return nil
	}

	tracks := make([]*api.Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		if track, ok := l.Tracks[id]; ok {
			tracks = append(tracks, track)
		}
	}
	return tracks
}

// GetTracksByAlbum returns all tracks from a specific album
func (l *Library) GetTracksByAlbum(album string) []*api.Track {
	l.mu.RLock()
	defer l.mu.RUnlock()

	trackIDs, exists := l.albumIndex[album]
	if !exists {
		return nil
	}

	tracks := make([]*api.Track, 0, len(trackIDs))
	for _, id := range trackIDs {
		if track, ok := l.Tracks[id]; ok {
			tracks = append(tracks, track)
		}
	}
	return tracks
}

// GetArtists returns all unique artists
func (l *Library) GetArtists() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	artists := make([]string, 0, len(l.artistIndex))
	for artist := range l.artistIndex {
		artists = append(artists, artist)
	}
	sort.Strings(artists)
	return artists
}

// GetAlbums returns all unique albums
func (l *Library) GetAlbums() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	albums := make([]string, 0, len(l.albumIndex))
	for album := range l.albumIndex {
		albums = append(albums, album)
	}
	sort.Strings(albums)
	return albums
}

// Search searches tracks by query string (matches title and artist)
func (l *Library) Search(query string) []*api.Track {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]*api.Track, 0, 10)

	for _, track := range l.Tracks {
		titleMatch := strings.Contains(strings.ToLower(track.Title), query)
		artistMatch := strings.Contains(strings.ToLower(track.Artist), query)
		albumMatch := strings.Contains(strings.ToLower(track.Album), query)

		if titleMatch || artistMatch || albumMatch {
			results = append(results, track)
		}
	}

	// Sort by relevance (title matches first)
	sort.Slice(results, func(i, j int) bool {
		iTitle := strings.Contains(strings.ToLower(results[i].Title), query)
		jTitle := strings.Contains(strings.ToLower(results[j].Title), query)
		return iTitle && !jTitle
	})

	return results
}

// RemoveTrack removes a track from the library
func (l *Library) RemoveTrack(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	track, exists := l.Tracks[id]
	if !exists {
		return playerrors.ErrTrackNotFound
	}

	// Remove from indices
	l.removeFromIndex(l.artistIndex, track.Artist, id)
	l.removeFromIndex(l.albumIndex, track.Album, id)
	l.removeFromIndex(l.genreIndex, track.Genre, id)

	delete(l.Tracks, id)
	l.TotalTracks = len(l.Tracks)
	return nil
}

// removeFromIndex removes a track ID from an index
func (l *Library) removeFromIndex(index map[string][]string, key, trackID string) {
	if key == "" {
		return
	}

	ids := index[key]
	for i, id := range ids {
		if id == trackID {
			index[key] = append(ids[:i], ids[i+1:]...)
			break
		}
	}

	// Remove empty keys
	if len(index[key]) == 0 {
		delete(index, key)
	}
}

// Scan scans the configured paths and adds tracks to the library
func (l *Library) Scan(ctx context.Context, paths []string) error {
	l.ScanPaths = paths
	tracks, errors := l.scanner.Scan(ctx, paths)

	// Collect errors
	var scanErrors []error
	go func() {
		for err := range errors {
			scanErrors = append(scanErrors, err)
		}
	}()

	// Add tracks to library
	for track := range tracks {
		l.AddTrack(track)
	}

	l.mu.Lock()
	l.LastScanned = time.Now()
	l.mu.Unlock()

	return nil
}

// Clear removes all tracks from the library
func (l *Library) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Tracks = make(map[string]*api.Track)
	l.artistIndex = make(map[string][]string)
	l.albumIndex = make(map[string][]string)
	l.genreIndex = make(map[string][]string)
	l.TotalTracks = 0
}

// Save persists the library to a JSON file
func (l *Library) Save(path string) error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal library: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write library file: %w", err)
	}

	return nil
}

// LoadLibrary loads a library from a JSON file (or returns empty if not exists)
func LoadLibrary(path string) (*Library, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewLibrary(), nil // First run, return empty library
	}
	if err != nil {
		return nil, fmt.Errorf("read library file: %w", err)
	}

	var lib Library
	if err := json.Unmarshal(data, &lib); err != nil {
		return nil, fmt.Errorf("unmarshal library: %w", err)
	}

	// Initialize non-exported fields
	lib.scanner = NewScanner(4)

	// Rebuild indices from loaded tracks
	lib.rebuildIndices()

	return &lib, nil
}

// rebuildIndices rebuilds the secondary indices from the tracks map
func (l *Library) rebuildIndices() {
	l.artistIndex = make(map[string][]string)
	l.albumIndex = make(map[string][]string)
	l.genreIndex = make(map[string][]string)

	for _, track := range l.Tracks {
		if track.Artist != "" {
			l.artistIndex[track.Artist] = append(l.artistIndex[track.Artist], track.ID)
		}
		if track.Album != "" {
			l.albumIndex[track.Album] = append(l.albumIndex[track.Album], track.ID)
		}
		if track.Genre != "" {
			l.genreIndex[track.Genre] = append(l.genreIndex[track.Genre], track.ID)
		}
	}

	l.TotalTracks = len(l.Tracks)
}

// AddFile adds a single file from any location to the library
func (l *Library) AddFile(filePath string) (*api.Track, error) {
	track, err := l.scanner.ScanFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}
	l.AddTrack(track)
	return track, nil
}
