# Go Terminal Music Player

A robust, terminal-based music player application developed in Go. It offers a feature-rich text user interface (TUI) for managing and playing local audio libraries. Designed for efficiency and ease of use, it supports common audio formats and provides essential playback controls within a modern command-line environment.

## Features

- **Audio Format Support:** Native playback for MP3, WAV, and FLAC formats.
- **Interactive TUI:** Built with Bubble Tea to provide a responsive, windowed interface within the terminal.
- **Library Management:**
  - Automatic directory scanning.
  - Metadata extraction and indexing (Artist, Album, Title).
  - Real-time search functionality.
- **Playlist System:** Create, manage, and persist playlists.
- **Playback Controls:**
  - Standard transport controls (Play, Pause, Stop, Next, Previous).
  - Seek functionality.
  - Volume control.
  - Shuffle and Repeat modes.
- **File Browser:** Integrated file system navigation to locate and add tracks manually.
- **Mouse Support:** functionality for navigation and timeline seeking.

## Installation

### Prerequisites

- Go 1.25 or higher
- Audio dependencies (on Linux, `libasound2-dev` is often required for the underlying audio library).

### Build from Source

```bash
# Clone the repository
git clone https://github.com/jscyril/golang_music_player.git
cd golang_music_player

# Download dependencies
go mod download

# Build the application
go build -o gtmpc cmd/player/main.go
```

## Usage

Run the compiled binary to start the application:

```bash
./gtmpc
```

On the first run, the application will initialize its configuration and data directories.

### Keybindings

**Global Controls**

- `Tab`: Cycle between Player, Library, and Playlist views.
- `1` / `2` / `3`: Switch directly to Player / Library / Playlist views.
- `q` or `Ctrl+C`: Quit the application.

**Playback**

- `Space`: Toggle Play/Pause.
- `s`: Stop playback.
- `n`: Next track.
- `p`: Previous track.
- `Right Arrow`: Seek forward 5 seconds.
- `Left Arrow`: Seek backward 5 seconds.
- `+` / `=`: Increase volume.
- `-`: Decrease volume.
- `S`: Toggle Shuffle mode.
- `r`: Cycle Repeat modes (Off, One, All).

**Library & Navigation**

- `Up` / `Down`: Navigate lists.
- `Enter`: Play selected track or add to queue.
- `/`: Activate search mode (in Library view).
- `Esc`: Exit search or browse mode.

## Configuration

The application adheres to standard configuration paths:

- **Configuration File:** `~/.config/musicplayer/config.json` (or defined by `$XDG_CONFIG_HOME`)
- **Data Directory:** Stores the library index and playlists (typically in `~/.local/share` or similar, depending on OS).

## Architecture

This project follows a modular architecture separating the UI, Audio Engine, and Data layers. For a detailed technical walkthrough of the application execution flow and component interaction, please refer to [APPLICATION_FLOW.md](APPLICATION_FLOW.md).
