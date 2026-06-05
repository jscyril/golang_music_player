# GTMPC

GTMPC is a Go-based music player with a terminal-first interface, local audio playback, playlist management, queue editing, library indexing, metadata extraction, and an optional HTTP backend. The project is designed to demonstrate practical Go engineering across concurrency, terminal UI design, audio processing, persistence, authentication, PostgreSQL, and REST APIs.

The primary application is the terminal music player. It provides a keyboard-driven interface for browsing a local music library, playing tracks, managing playlists, editing the playback queue, viewing embedded album art, and controlling playback without leaving the terminal.

## Screenshots

Add screenshots of the application here.

### Player View

Image placeholder.

### Library View

Image placeholder.

### Playlist View

Image placeholder.

### Queue View

Image placeholder.

### Web Interface

Image placeholder.

## Core Features

### Terminal Music Player

The TUI is built with Bubble Tea and Lip Gloss. It includes four main views:

| View | Purpose |
| --- | --- |
| Player | Displays the current track, playback state, progress, volume, queue position, next track, repeat mode, shuffle mode, and album art when available. |
| Library | Lists indexed tracks, supports search, file browsing, and playback from the selected track. |
| Playlist | Creates, opens, deletes, and edits persisted playlists. |
| Queue | Displays the active playback queue and supports queue editing while music is playing. |

### Audio Playback

GTMPC supports local playback for:

| Format | Support |
| --- | --- |
| MP3 | Supported |
| WAV | Supported |
| FLAC | Supported |

Playback is handled through the Go audio stack using `beep` and `speaker`. The audio engine supports:

- Play, pause, resume, and stop
- Next and previous track navigation
- Seek forward and backward
- Mouse-based progress-bar seeking
- Volume control
- Shuffle mode
- Repeat off, repeat one, and repeat all
- Automatic queue advancement when a track ends
- Background playback after closing the TUI

### Album Art

The player attempts to read embedded cover art from audio metadata. When cover art is available, it is rendered directly in the terminal using ANSI truecolor blocks. When no cover art is available, the player displays a fallback terminal tile.

### Library Management

The library system scans configured directories, extracts metadata, and stores an indexed library on disk. It supports:

- Recursive directory scanning
- Concurrent scanning through worker goroutines
- Metadata extraction for title, artist, album, genre, year, track number, duration, and cover art
- Search by title, artist, and album
- Artist, album, and genre indexing
- Manual file addition through the integrated terminal file browser
- JSON persistence for the local library cache

### Playlist Management

The terminal player includes local playlist management backed by JSON persistence. It supports:

- Create playlists
- Delete playlists
- Open playlists
- Add selected library tracks to playlists
- Remove tracks from playlists
- Play directly from a playlist
- Persist playlists across application restarts
- Prevent duplicate tracks in a playlist

The server mode also includes PostgreSQL-backed playlist support with authenticated, user-owned playlists.

### Queue Management

The queue view provides direct control over the active playback queue. It supports:

- View the full queue
- See the currently playing track
- Jump to a selected queue item
- Remove queue items
- Move queue items up or down
- Clear the queue
- Preserve the current track correctly during queue edits

### Background Playback

Pressing `b` closes the terminal UI while keeping the process and audio playback running. The process remains alive until it receives `Ctrl+C` or `SIGTERM`.

This is useful when the user wants to start playback from the TUI and then return to the shell while music continues.

### HTTP Backend

The optional server mode exposes a REST API and a static web interface. It includes:

- PostgreSQL connection pooling with `pgxpool`
- Automatic schema creation for users, tracks, playlists, and playlist-track mappings
- User registration and login
- Bcrypt password hashing
- JWT-based authentication
- Track listing and search
- Audio upload
- Authenticated audio streaming
- User-owned playlist APIs
- CORS and request logging middleware
- Background library scanning and PostgreSQL synchronization

### Web Interface

The web UI is secondary to the TUI, but it provides a browser-based client for the server mode. It supports:

- Login and registration
- Track listing
- Search
- Uploading music
- Playback through the browser audio element
- Playlist creation
- Playlist deletion
- Adding tracks to playlists
- Removing tracks from playlists

## Architecture

The codebase is organized into clear packages:

| Path | Responsibility |
| --- | --- |
| `cmd/player` | Terminal player entry point |
| `cmd/server` | HTTP server entry point |
| `api` | Shared API and domain types |
| `internal/audio` | Audio decoding, playback, seeking, volume, and playback state |
| `internal/library` | Library scanning, metadata extraction, indexing, and persistence |
| `internal/playlist` | Local playlist persistence and playback queue logic |
| `internal/ui` | Bubble Tea application model and TUI orchestration |
| `internal/ui/views` | Player, library, playlist, and queue views |
| `internal/ui/components` | Reusable terminal UI components |
| `internal/auth` | Authentication services, bcrypt, and JWT handling |
| `internal/database` | PostgreSQL connection and repositories |
| `internal/server` | HTTP routing, handlers, middleware, and background scanner |
| `pkg/errors` | Shared error types |
| `pkg/events` | Event bus implementation |
| `web` | Static browser client |

## Application Flow

### Terminal Player

1. The player loads configuration from the configured path.
2. It creates the data directory if needed.
3. It starts the audio engine and initializes the speaker.
4. It loads the persisted library from `library.json`.
5. If the library is empty and music directories are configured, it scans those directories.
6. It loads local playlists from disk.
7. It starts the Bubble Tea TUI.
8. User input is translated into library, playlist, queue, or audio actions.
9. The library is saved when the process exits.

### Server

1. The server loads environment variables and configuration.
2. It connects to PostgreSQL.
3. It runs idempotent schema creation.
4. It initializes repositories and authentication services.
5. It serves public auth routes and protected library, playlist, upload, and stream routes.
6. It optionally scans configured music directories in the background and syncs tracks to PostgreSQL.

## Installation

### Requirements

- Go 1.25 or newer
- A terminal with truecolor support for best album-art rendering
- Linux audio dependencies required by the Go audio backend
- PostgreSQL if using server mode

On Linux, the audio backend may require system packages such as:

```bash
sudo apt install libasound2-dev
```

### Build the Terminal Player

```bash
go mod download
go build -o gtmpc cmd/player/main.go
```

Run the player:

```bash
./gtmpc
```

### Build the HTTP Server

```bash
go build -o gtmpc-server cmd/server/main.go
```

Run the server:

```bash
./gtmpc-server
```

## Configuration

The terminal player uses a JSON configuration file.

Configuration path resolution:

1. `MUSIC_PLAYER_CONFIG`
2. `$XDG_CONFIG_HOME/musicplayer/config.json`
3. `~/.config/musicplayer/config.json`
4. `./config.json` if the home directory cannot be resolved

Default configuration includes:

- Music directories
- Default volume
- Theme name
- Key bindings
- Cache settings
- Data directory

The default data directory is:

```text
./data
```

The data directory stores:

- `library.json`
- playlist JSON files
- uploaded audio files when configured for server mode

## Terminal Keybindings

### Global

| Key | Action |
| --- | --- |
| `Tab` | Cycle through Player, Library, Playlist, and Queue views |
| `1` | Open Player view |
| `2` | Open Library view |
| `3` | Open Playlist view |
| `4` | Open Queue view |
| `q` | Quit and stop playback |
| `Ctrl+C` | Quit and stop playback |
| `b` | Close the TUI and keep playback running in the background |

### Playback

| Key | Action |
| --- | --- |
| `Space` | Toggle play and pause |
| `s` | Stop playback |
| `n` | Play next track |
| `p` | Restart current track or play previous track |
| `Right Arrow` | Seek forward 5 seconds |
| `Left Arrow` | Seek backward 5 seconds |
| `+` or `=` | Increase volume |
| `-` | Decrease volume |
| `S` | Toggle shuffle |
| `r` | Cycle repeat mode |

### Library

| Key | Action |
| --- | --- |
| `Up` or `k` | Move selection up |
| `Down` or `j` | Move selection down |
| `Enter` | Play selected track |
| `/` | Search library |
| `Esc` | Exit search or file browser |
| `a` | Open file browser to add a track |
| `A` | Add selected track to a playlist |

### Playlist

| Key | Action |
| --- | --- |
| `c` | Create playlist |
| `d` | Delete selected playlist |
| `Enter` | Open selected playlist or play selected playlist track |
| `x` | Remove selected track from an open playlist |
| `Backspace` or `Esc` | Return to playlist list |

### Queue

| Key | Action |
| --- | --- |
| `Enter` | Jump to selected queue item |
| `x` | Remove selected queue item |
| `u` | Move selected queue item up |
| `d` | Move selected queue item down |
| `C` | Clear queue |
| `Up` or `k` | Move selection up |
| `Down` or `j` | Move selection down |

## Server Environment Variables

| Variable | Purpose | Default |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://postgres:postgres@localhost:5432/gtmpc?sslmode=disable` |
| `GTMPC_JWT_SECRET` | JWT signing secret | Development fallback secret |
| `GTMPC_ADDR` | HTTP server address | `:8080` |
| `GTMPC_WEB_DIR` | Static web directory | `web` |
| `GTMPC_UPLOAD_DIR` | Upload directory | Configured data directory uploads path |

## HTTP API Summary

### Public Routes

| Method | Route | Description |
| --- | --- | --- |
| `GET` | `/api/health` | Health check |
| `POST` | `/api/auth/register` | Register a user |
| `POST` | `/api/auth/login` | Log in and receive a JWT |

### Protected Routes

These routes require a Bearer token.

| Method | Route | Description |
| --- | --- | --- |
| `GET` | `/api/library/tracks` | List tracks |
| `GET` | `/api/library/search?q=term` | Search tracks |
| `POST` | `/api/library/upload` | Upload an audio file |
| `GET` | `/api/stream/{trackID}` | Stream an audio file |
| `GET` | `/api/playlists` | List user playlists |
| `POST` | `/api/playlists` | Create playlist |
| `GET` | `/api/playlists/{id}` | Get playlist |
| `PUT` | `/api/playlists/{id}` | Update playlist |
| `DELETE` | `/api/playlists/{id}` | Delete playlist |
| `POST` | `/api/playlists/{id}/tracks` | Add track to playlist |
| `DELETE` | `/api/playlists/{id}/tracks/{trackID}` | Remove track from playlist |

## Testing

Run the full test suite:

```bash
go test ./...
```

The test suite currently covers:

- Configuration serialization and file loading
- Authentication and password hashing
- JWT generation and validation
- Audio engine initialization and basic command validation
- Queue movement and removal behavior

## Project Highlights

This project demonstrates:

- Go concurrency through scanner workers, audio engine goroutines, server goroutines, and graceful shutdown
- Clean package separation between UI, audio, library, playlist, auth, database, and server layers
- Practical terminal UI design with Bubble Tea
- Audio playback and seeking in Go
- Metadata extraction from local audio files
- JSON persistence for local-first workflows
- PostgreSQL-backed server mode
- REST API design with authentication
- Bcrypt password hashing and JWT validation
- Queue and playlist state management

## Current Status

The TUI is the primary interface and is actively developed. The backend and web UI are available as secondary interfaces for demonstrating API, authentication, upload, streaming, and PostgreSQL-backed playlist functionality.
