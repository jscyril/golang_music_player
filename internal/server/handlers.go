package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/auth"
	"github.com/jscyril/golang_music_player/internal/database"
	"github.com/jscyril/golang_music_player/internal/library"
)

// Handlers holds all dependencies needed by HTTP handlers.
// This is the dependency-injection root for the server layer.
type Handlers struct {
	AuthDB       *auth.DBService
	TrackRepo    *database.TrackRepo
	PlaylistRepo *database.PlaylistRepo
	JWTSecret    []byte
	TokenTTL     time.Duration
	UploadDir    string // directory where uploaded files are stored
}

// NewHandlers creates a Handlers struct with the given dependencies.
func NewHandlers(authDB *auth.DBService, trackRepo *database.TrackRepo, playlistRepo *database.PlaylistRepo, secret []byte, uploadDir string) *Handlers {
	_ = os.MkdirAll(uploadDir, 0755)
	return &Handlers{
		AuthDB:       authDB,
		TrackRepo:    trackRepo,
		PlaylistRepo: playlistRepo,
		JWTSecret:    secret,
		TokenTTL:     24 * time.Hour,
		UploadDir:    uploadDir,
	}
}

// --- Auth Handlers ---

// HandleRegister processes POST /api/auth/register
func (h *Handlers) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	var req api.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid request body"})
		return
	}

	user, err := h.AuthDB.Register(r.Context(), req)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			status = http.StatusConflict
		} else if errors.Is(err, auth.ErrEmptyCredentials) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, api.APIResponse{Success: false, Error: err.Error()})
		return
	}

	log.Printf("[AUTH] User registered: %s (role: %s)", user.Username, user.Role)
	writeJSON(w, http.StatusCreated, api.APIResponse{Success: true, Data: user})
}

// HandleLogin processes POST /api/auth/login
func (h *Handlers) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	var req api.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid request body"})
		return
	}

	user, err := h.AuthDB.Authenticate(r.Context(), req)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, auth.ErrEmptyCredentials) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, api.APIResponse{Success: false, Error: err.Error()})
		return
	}

	// Generate JWT token upon successful bcrypt verification
	token, err := auth.GenerateToken(user.Username, user.Role, h.JWTSecret, h.TokenTTL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to generate token"})
		return
	}

	log.Printf("[AUTH] User logged in: %s", user.Username)
	writeJSON(w, http.StatusOK, api.APIResponse{
		Success: true,
		Data:    api.LoginResponse{Token: token, User: *user},
	})
}

// --- Library Handlers (Protected, PostgreSQL-backed) ---

// HandleGetTracks processes GET /api/library/tracks
func (h *Handlers) HandleGetTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	tracks, err := h.TrackRepo.GetAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to fetch tracks"})
		return
	}

	log.Printf("[LIBRARY] Serving %d tracks to user %s", len(tracks), r.Header.Get("X-User"))
	writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: tracks})
}

// HandleSearchTracks processes GET /api/library/search?q=...
func (h *Handlers) HandleSearchTracks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "query parameter 'q' is required"})
		return
	}

	results, err := h.TrackRepo.Search(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "search failed"})
		return
	}

	writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: results})
}

// --- Playlist Handlers (Protected, user-owned) ---

// HandlePlaylists processes /api/playlists for listing and creation.
func (h *Handlers) HandlePlaylists(w http.ResponseWriter, r *http.Request) {
	owner := currentUser(r)
	if owner == "" {
		writeJSON(w, http.StatusUnauthorized, api.APIResponse{Success: false, Error: "missing authenticated user"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		playlists, err := h.PlaylistRepo.GetAll(r.Context(), owner)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to fetch playlists"})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: playlists})

	case http.MethodPost:
		var req api.PlaylistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid request body"})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		pl, err := h.PlaylistRepo.Create(r.Context(), owner, req.Name, req.Description)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, api.APIResponse{Success: true, Data: pl})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
	}
}

// HandlePlaylistByID processes /api/playlists/{id} and /api/playlists/{id}/tracks.
func (h *Handlers) HandlePlaylistByID(w http.ResponseWriter, r *http.Request) {
	owner := currentUser(r)
	if owner == "" {
		writeJSON(w, http.StatusUnauthorized, api.APIResponse{Success: false, Error: "missing authenticated user"})
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/playlists/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "playlist ID is required"})
		return
	}
	playlistID := parts[0]

	if len(parts) == 1 {
		h.handlePlaylistResource(w, r, owner, playlistID)
		return
	}

	if parts[1] != "tracks" {
		writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: "not found"})
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		var req api.PlaylistTrackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid request body"})
			return
		}
		req.TrackID = strings.TrimSpace(req.TrackID)
		if req.TrackID == "" {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "track_id is required"})
			return
		}
		if _, err := h.TrackRepo.GetByID(r.Context(), req.TrackID); err != nil {
			writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: "track not found"})
			return
		}
		pl, err := h.PlaylistRepo.AddTrack(r.Context(), owner, playlistID, req.TrackID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: pl})
		return
	}

	if len(parts) == 3 && r.Method == http.MethodDelete {
		pl, err := h.PlaylistRepo.RemoveTrack(r.Context(), owner, playlistID, parts[2])
		if err != nil {
			writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: pl})
		return
	}

	writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
}

func (h *Handlers) handlePlaylistResource(w http.ResponseWriter, r *http.Request, owner, playlistID string) {
	switch r.Method {
	case http.MethodGet:
		pl, err := h.PlaylistRepo.GetByID(r.Context(), owner, playlistID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: "playlist not found"})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: pl})

	case http.MethodPut:
		var req api.PlaylistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid request body"})
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Description = strings.TrimSpace(req.Description)
		pl, err := h.PlaylistRepo.Update(r.Context(), owner, playlistID, req.Name, req.Description)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true, Data: pl})

	case http.MethodDelete:
		if err := h.PlaylistRepo.Delete(r.Context(), owner, playlistID); err != nil {
			writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, api.APIResponse{Success: true})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
	}
}

// HandleStreamTrack processes GET /api/stream/{trackID}
// Streams the audio file using http.ServeFile for efficient range-request support.
func (h *Handlers) HandleStreamTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	// Extract track ID from URL path: /api/stream/{id}
	trackID := strings.TrimPrefix(r.URL.Path, "/api/stream/")
	if trackID == "" {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "track ID is required"})
		return
	}

	track, err := h.TrackRepo.GetByID(r.Context(), trackID)
	if err != nil || track == nil {
		writeJSON(w, http.StatusNotFound, api.APIResponse{Success: false, Error: "track not found"})
		return
	}

	// Set content type based on file extension
	ext := strings.ToLower(filepath.Ext(track.FilePath))
	switch ext {
	case ".mp3":
		w.Header().Set("Content-Type", "audio/mpeg")
	case ".wav":
		w.Header().Set("Content-Type", "audio/wav")
	case ".flac":
		w.Header().Set("Content-Type", "audio/flac")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	log.Printf("[STREAM] Streaming track %q (%s) to user %s", track.Title, trackID, r.Header.Get("X-User"))

	// http.ServeFile handles Range requests, Content-Length, and caching automatically
	http.ServeFile(w, r, track.FilePath)
}

// HandleHealthCheck processes GET /api/health — unauthenticated health probe
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.APIResponse{
		Success: true,
		Data: map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		},
	})
}

// --- Upload Handler (Protected) ---

// HandleUploadTrack processes POST /api/library/upload
// Accepts multipart form file uploads and saves to the upload directory.
func (h *Handlers) HandleUploadTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, api.APIResponse{Success: false, Error: "method not allowed"})
		return
	}

	// Limit upload size to 50MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "invalid multipart form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{Success: false, Error: "file is required"})
		return
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".mp3" && ext != ".wav" && ext != ".flac" {
		writeJSON(w, http.StatusBadRequest, api.APIResponse{
			Success: false,
			Error:   "unsupported format: only .mp3, .wav, .flac allowed",
		})
		return
	}

	// Generate unique ID for the track
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to generate track ID"})
		return
	}
	trackID := hex.EncodeToString(idBytes)

	// Save the file to the upload directory
	savePath := filepath.Join(h.UploadDir, trackID+ext)
	dst, err := os.Create(savePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to save file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: "failed to write file"})
		return
	}

	// Extract title from filename (strip extension)
	title := strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename))
	artist := r.FormValue("artist")
	album := r.FormValue("album")
	if artist == "" {
		artist = "Unknown Artist"
	}
	if album == "" {
		album = "Uploads"
	}

	metadataReader := library.NewMetadataReader()
	track, err := metadataReader.Read(savePath)
	if err != nil {
		track = &api.Track{Title: title, CreatedAt: time.Now()}
	}
	track.ID = trackID
	track.FilePath = savePath
	if strings.TrimSpace(r.FormValue("artist")) != "" {
		track.Artist = artist
	}
	if strings.TrimSpace(r.FormValue("album")) != "" {
		track.Album = album
	}
	if strings.TrimSpace(r.FormValue("genre")) != "" {
		track.Genre = r.FormValue("genre")
	}
	if track.Title == "" {
		track.Title = title
	}
	if track.Artist == "" {
		track.Artist = artist
	}
	if track.Album == "" {
		track.Album = album
	}

	if err := h.TrackRepo.Upsert(r.Context(), track); err != nil {
		writeJSON(w, http.StatusInternalServerError, api.APIResponse{Success: false, Error: fmt.Sprintf("failed to save track: %v", err)})
		return
	}

	log.Printf("[UPLOAD] Track uploaded: %q by %s (%s)", track.Title, track.Artist, trackID)
	writeJSON(w, http.StatusCreated, api.APIResponse{Success: true, Data: track})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func currentUser(r *http.Request) string {
	return r.Header.Get("X-User")
}
