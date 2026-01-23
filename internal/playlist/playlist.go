package playlist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jscyril/golang_music_player/api"
	playerrors "github.com/jscyril/golang_music_player/pkg/errors"
)

// Manager handles playlist CRUD operations with JSON persistence
type Manager struct {
	playlists map[string]*api.Playlist
	basePath  string
	mu        sync.RWMutex
}

// NewManager creates a new playlist manager
func NewManager(basePath string) *Manager {
	return &Manager{
		playlists: make(map[string]*api.Playlist),
		basePath:  basePath,
	}
}

// Create creates a new playlist
func (m *Manager) Create(name, description string) (*api.Playlist, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generatePlaylistID(name)
	now := time.Now()

	playlist := &api.Playlist{
		ID:          id,
		Name:        name,
		Description: description,
		Tracks:      []api.Track{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	m.playlists[id] = playlist

	if err := m.savePlaylist(playlist); err != nil {
		delete(m.playlists, id)
		return nil, err
	}

	return playlist, nil
}

// GetByID returns a playlist by its ID
func (m *Manager) GetByID(id string) (*api.Playlist, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlist, exists := m.playlists[id]
	if !exists {
		return nil, playerrors.ErrPlaylistNotFound
	}
	return playlist, nil
}

// GetAll returns all playlists
func (m *Manager) GetAll() []*api.Playlist {
	m.mu.RLock()
	defer m.mu.RUnlock()

	playlists := make([]*api.Playlist, 0, len(m.playlists))
	for _, p := range m.playlists {
		playlists = append(playlists, p)
	}
	return playlists
}

// Update updates a playlist's name and description
func (m *Manager) Update(id, name, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, exists := m.playlists[id]
	if !exists {
		return playerrors.ErrPlaylistNotFound
	}

	playlist.Name = name
	playlist.Description = description
	playlist.UpdatedAt = time.Now()

	return m.savePlaylist(playlist)
}

// Delete deletes a playlist
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.playlists[id]; !exists {
		return playerrors.ErrPlaylistNotFound
	}

	// Delete file
	path := filepath.Join(m.basePath, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete playlist file: %w", err)
	}

	delete(m.playlists, id)
	return nil
}

// AddTrack adds a track to a playlist
func (m *Manager) AddTrack(playlistID string, track *api.Track) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, exists := m.playlists[playlistID]
	if !exists {
		return playerrors.ErrPlaylistNotFound
	}

	playlist.Tracks = append(playlist.Tracks, *track)
	playlist.UpdatedAt = time.Now()

	return m.savePlaylist(playlist)
}

// RemoveTrack removes a track from a playlist
func (m *Manager) RemoveTrack(playlistID, trackID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	playlist, exists := m.playlists[playlistID]
	if !exists {
		return playerrors.ErrPlaylistNotFound
	}

	found := false
	for i, t := range playlist.Tracks {
		if t.ID == trackID {
			playlist.Tracks = append(playlist.Tracks[:i], playlist.Tracks[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return playerrors.ErrTrackNotFound
	}

	playlist.UpdatedAt = time.Now()
	return m.savePlaylist(playlist)
}

// savePlaylist saves a playlist to disk
func (m *Manager) savePlaylist(playlist *api.Playlist) error {
	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return fmt.Errorf("create playlist directory: %w", err)
	}

	data, err := json.MarshalIndent(playlist, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal playlist: %w", err)
	}

	path := filepath.Join(m.basePath, playlist.ID+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write playlist file: %w", err)
	}

	return nil
}

// LoadAll loads all playlists from disk
func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return fmt.Errorf("create playlist directory: %w", err)
	}

	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return fmt.Errorf("read playlist directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(m.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var playlist api.Playlist
		if err := json.Unmarshal(data, &playlist); err != nil {
			continue // Skip invalid JSON
		}

		m.playlists[playlist.ID] = &playlist
	}

	return nil
}

// generatePlaylistID generates a unique ID for a playlist
func generatePlaylistID(name string) string {
	return fmt.Sprintf("playlist-%d", time.Now().UnixNano())
}
