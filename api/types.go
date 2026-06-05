package api

import "time"

type Track struct {
	ID        string        `json:"id"`
	Title     string        `json:"title"`
	Artist    string        `json:"artist"`
	Album     string        `json:"album"`
	Duration  time.Duration `json:"duration"`
	FilePath  string        `json:"file_path"`
	Genre     string        `json:"genre"`
	Year      int           `json:"year"`
	TrackNum  int           `json:"track_number"`
	CoverArt  []byte        `json:"-"`
	CreatedAt time.Time     `json:"created_at"`
}

type Playlist struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tracks      []Track   `json:"tracks"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type PlayerStatus int

const (
	StatusStopped PlayerStatus = iota
	StatusPlaying
	StatusPaused
)

// RepeatMode represents repeat options
type RepeatMode int

const (
	RepeatNone RepeatMode = iota
	RepeatOne
	RepeatAll
)

// PlaybackState represents the current player state
type PlaybackState struct {
	CurrentTrack *Track        `json:"current_track"`
	Status       PlayerStatus  `json:"status"`
	Position     time.Duration `json:"position"`
	Volume       float64       `json:"volume"` // 0.0 to 1.0
	Repeat       RepeatMode    `json:"repeat"`
	Shuffle      bool          `json:"shuffle"`
	Queue        []*Track      `json:"queue"`
	QueueIndex   int           `json:"queue_index"`
}

// CommandType enumerates audio commands
type CommandType int

const (
	CmdPlay CommandType = iota
	CmdPause
	CmdResume
	CmdStop
	CmdSeek
	CmdVolume
	CmdNext
	CmdPrevious
)

// AudioCommand represents commands sent to the audio engine
type AudioCommand struct {
	Type    CommandType
	Payload interface{} // Can be *Track, float64 for volume, time.Duration for seek
}

// EventType enumerates audio events
type EventType int

const (
	EventTrackStarted EventType = iota
	EventTrackEnded
	EventPositionUpdate
	EventError
	EventStateChange
)

// AudioEvent represents events emitted by the audio engine
type AudioEvent struct {
	Type    EventType
	Payload interface{}
}

// Player defines the core playback interface
type Player interface {
	Play(track *Track) error
	Pause() error
	Resume() error
	Stop() error
	Seek(position time.Duration) error
	SetVolume(level float64) error
	GetState() *PlaybackState
}

// --- Server / Auth Types ---

// User represents a registered user with a bcrypt-hashed password
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never serialized in JSON responses
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// RegisterRequest is the payload for user registration
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// LoginRequest is the payload for user login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is returned on successful authentication
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// PlaylistRequest is used to create or update a playlist.
type PlaylistRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PlaylistTrackRequest adds a track to a playlist.
type PlaylistTrackRequest struct {
	TrackID string `json:"track_id"`
}

// APIResponse is a generic JSON envelope for all API responses
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
