package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jscyril/golang_music_player/api"
)

// PlaylistRepo handles user-owned playlists stored in PostgreSQL.
type PlaylistRepo struct {
	db *DB
}

func NewPlaylistRepo(db *DB) *PlaylistRepo {
	return &PlaylistRepo{db: db}
}

func (r *PlaylistRepo) Create(ctx context.Context, owner, name, description string) (*api.Playlist, error) {
	if name == "" {
		return nil, fmt.Errorf("playlist name is required")
	}

	id, err := randomID("playlist-")
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO playlists (id, owner_username, name, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, created_at, updated_at`

	pl := &api.Playlist{}
	err = r.db.Pool.QueryRow(ctx, query, id, owner, name, description).
		Scan(&pl.ID, &pl.Name, &pl.Description, &pl.CreatedAt, &pl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("playlist_repo: create failed: %w", err)
	}
	pl.Tracks = []api.Track{}
	return pl, nil
}

func (r *PlaylistRepo) GetAll(ctx context.Context, owner string) ([]*api.Playlist, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM playlists
		WHERE owner_username = $1
		ORDER BY updated_at DESC, created_at DESC`

	rows, err := r.db.Pool.Query(ctx, query, owner)
	if err != nil {
		return nil, fmt.Errorf("playlist_repo: list failed: %w", err)
	}
	defer rows.Close()

	playlists := []*api.Playlist{}
	for rows.Next() {
		pl := &api.Playlist{Tracks: []api.Track{}}
		if err := rows.Scan(&pl.ID, &pl.Name, &pl.Description, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("playlist_repo: scan failed: %w", err)
		}
		tracks, err := r.getTracks(ctx, pl.ID)
		if err != nil {
			return nil, err
		}
		pl.Tracks = tracks
		playlists = append(playlists, pl)
	}
	return playlists, rows.Err()
}

func (r *PlaylistRepo) GetByID(ctx context.Context, owner, id string) (*api.Playlist, error) {
	query := `
		SELECT id, name, description, created_at, updated_at
		FROM playlists
		WHERE owner_username = $1 AND id = $2`

	pl := &api.Playlist{}
	err := r.db.Pool.QueryRow(ctx, query, owner, id).
		Scan(&pl.ID, &pl.Name, &pl.Description, &pl.CreatedAt, &pl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("playlist_repo: playlist not found: %w", err)
	}

	tracks, err := r.getTracks(ctx, id)
	if err != nil {
		return nil, err
	}
	pl.Tracks = tracks
	return pl, nil
}

func (r *PlaylistRepo) Update(ctx context.Context, owner, id, name, description string) (*api.Playlist, error) {
	if name == "" {
		return nil, fmt.Errorf("playlist name is required")
	}

	query := `
		UPDATE playlists
		SET name = $3, description = $4, updated_at = NOW()
		WHERE owner_username = $1 AND id = $2
		RETURNING id, name, description, created_at, updated_at`

	pl := &api.Playlist{}
	err := r.db.Pool.QueryRow(ctx, query, owner, id, name, description).
		Scan(&pl.ID, &pl.Name, &pl.Description, &pl.CreatedAt, &pl.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("playlist_repo: update failed: %w", err)
	}

	tracks, err := r.getTracks(ctx, id)
	if err != nil {
		return nil, err
	}
	pl.Tracks = tracks
	return pl, nil
}

func (r *PlaylistRepo) Delete(ctx context.Context, owner, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM playlists WHERE owner_username = $1 AND id = $2`, owner, id)
	if err != nil {
		return fmt.Errorf("playlist_repo: delete failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("playlist not found")
	}
	return nil
}

func (r *PlaylistRepo) AddTrack(ctx context.Context, owner, playlistID, trackID string) (*api.Playlist, error) {
	if err := r.ensureOwned(ctx, owner, playlistID); err != nil {
		return nil, err
	}

	query := `
		INSERT INTO playlist_tracks (playlist_id, track_id, position)
		SELECT $1, $2, COALESCE(MAX(position), -1) + 1
		FROM playlist_tracks
		WHERE playlist_id = $1
		ON CONFLICT (playlist_id, track_id) DO NOTHING`
	if _, err := r.db.Pool.Exec(ctx, query, playlistID, trackID); err != nil {
		return nil, fmt.Errorf("playlist_repo: add track failed: %w", err)
	}

	if _, err := r.db.Pool.Exec(ctx, `UPDATE playlists SET updated_at = NOW() WHERE id = $1`, playlistID); err != nil {
		return nil, fmt.Errorf("playlist_repo: touch playlist failed: %w", err)
	}
	return r.GetByID(ctx, owner, playlistID)
}

func (r *PlaylistRepo) RemoveTrack(ctx context.Context, owner, playlistID, trackID string) (*api.Playlist, error) {
	if err := r.ensureOwned(ctx, owner, playlistID); err != nil {
		return nil, err
	}

	if _, err := r.db.Pool.Exec(ctx, `DELETE FROM playlist_tracks WHERE playlist_id = $1 AND track_id = $2`, playlistID, trackID); err != nil {
		return nil, fmt.Errorf("playlist_repo: remove track failed: %w", err)
	}
	if _, err := r.db.Pool.Exec(ctx, `UPDATE playlists SET updated_at = NOW() WHERE id = $1`, playlistID); err != nil {
		return nil, fmt.Errorf("playlist_repo: touch playlist failed: %w", err)
	}
	return r.GetByID(ctx, owner, playlistID)
}

func (r *PlaylistRepo) ensureOwned(ctx context.Context, owner, id string) error {
	var exists bool
	err := r.db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM playlists WHERE owner_username = $1 AND id = $2)`, owner, id).Scan(&exists)
	if err != nil {
		return fmt.Errorf("playlist_repo: ownership check failed: %w", err)
	}
	if !exists {
		return fmt.Errorf("playlist not found")
	}
	return nil
}

func (r *PlaylistRepo) getTracks(ctx context.Context, playlistID string) ([]api.Track, error) {
	query := `
		SELECT t.id, t.title, t.artist, t.album, t.duration, t.file_path, t.genre, t.year, t.track_num, t.created_at
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		WHERE pt.playlist_id = $1
		ORDER BY pt.position, t.artist, t.album, t.track_num, t.title`

	rows, err := r.db.Pool.Query(ctx, query, playlistID)
	if err != nil {
		return nil, fmt.Errorf("playlist_repo: tracks query failed: %w", err)
	}
	defer rows.Close()

	tracks := []api.Track{}
	for rows.Next() {
		var t api.Track
		var dur int64
		if err := rows.Scan(&t.ID, &t.Title, &t.Artist, &t.Album, &dur, &t.FilePath, &t.Genre, &t.Year, &t.TrackNum, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("playlist_repo: track scan failed: %w", err)
		}
		t.Duration = time.Duration(dur)
		tracks = append(tracks, t)
	}
	return tracks, rows.Err()
}

func randomID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(b), nil
}
