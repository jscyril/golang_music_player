package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgxpool connection pool with application-specific functionality.
// Uses pgxpool for efficient concurrent connection management — each HTTP request
// goroutine gets its own connection from the pool automatically.
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new database connection pool and runs migrations.
// The connStr should be a PostgreSQL connection string, e.g.:
//
//	"postgres://user:pass@localhost:5432/gtmpc?sslmode=disable"
func New(ctx context.Context, connStr string) (*DB, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("database: invalid connection string: %w", err)
	}

	// Connection pool tuning for concurrent HTTP server workloads
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("database: failed to create pool: %w", err)
	}

	// Verify the connection is alive
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	db := &DB{Pool: pool}

	// Run schema migrations
	if err := db.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database: migration failed: %w", err)
	}

	log.Println("[DB] Connected to PostgreSQL and migrations applied")
	return db, nil
}

// Close gracefully shuts down the connection pool
func (db *DB) Close() {
	db.Pool.Close()
	log.Println("[DB] Connection pool closed")
}

// migrate creates the application tables if they don't already exist.
// Uses IF NOT EXISTS so migrations are idempotent.
func (db *DB) migrate(ctx context.Context) error {
	migrations := []string{
		// Users table — stores bcrypt-hashed credentials
		`CREATE TABLE IF NOT EXISTS users (
			id         TEXT PRIMARY KEY,
			username   TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role       TEXT NOT NULL DEFAULT 'user',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Tracks table — stores music library metadata
		`CREATE TABLE IF NOT EXISTS tracks (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			artist     TEXT NOT NULL DEFAULT '',
			album      TEXT NOT NULL DEFAULT '',
			duration   BIGINT NOT NULL DEFAULT 0,
			file_path  TEXT NOT NULL,
			genre      TEXT NOT NULL DEFAULT '',
			year       INTEGER NOT NULL DEFAULT 0,
			track_num  INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		// Playlists table
		`CREATE TABLE IF NOT EXISTS playlists (
			id          TEXT PRIMARY KEY,
			owner_username TEXT NOT NULL DEFAULT '',
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,

		`ALTER TABLE playlists ADD COLUMN IF NOT EXISTS owner_username TEXT NOT NULL DEFAULT ''`,

		// Playlist-Track junction table (many-to-many)
		`CREATE TABLE IF NOT EXISTS playlist_tracks (
			playlist_id TEXT NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
			track_id    TEXT NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
			position    INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (playlist_id, track_id)
		)`,

		// Index for fast artist/album lookups
		`CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_album  ON tracks(album)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_playlists_owner ON playlists(owner_username)`,
	}

	for _, m := range migrations {
		if _, err := db.Pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	return nil
}
