package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jscyril/golang_music_player/internal/auth"
	"github.com/jscyril/golang_music_player/internal/database"
	"github.com/jscyril/golang_music_player/internal/library"
)

// Server wraps the HTTP server with graceful shutdown and route management.
type Server struct {
	httpServer *http.Server
	handlers   *Handlers
	jwtSecret  []byte
}

// NewServer creates a fully configured HTTP server.
// It wires all routes, middleware, and background workers together.
func NewServer(addr string, authDB *auth.DBService, trackRepo *database.TrackRepo, playlistRepo *database.PlaylistRepo, jwtSecret []byte, webDir string, uploadDir string) *Server {
	h := NewHandlers(authDB, trackRepo, playlistRepo, jwtSecret, uploadDir)
	mux := http.NewServeMux()

	// --- Public routes (no auth required) ---
	mux.HandleFunc("/api/health", HandleHealthCheck)
	mux.HandleFunc("/api/auth/register", h.HandleRegister)
	mux.HandleFunc("/api/auth/login", h.HandleLogin)

	// --- Protected routes (JWT auth required) ---
	protected := http.NewServeMux()
	protected.HandleFunc("/api/library/tracks", h.HandleGetTracks)
	protected.HandleFunc("/api/library/search", h.HandleSearchTracks)
	protected.HandleFunc("/api/library/upload", h.HandleUploadTrack)
	protected.HandleFunc("/api/playlists", h.HandlePlaylists)
	protected.HandleFunc("/api/playlists/", h.HandlePlaylistByID)
	protected.HandleFunc("/api/stream/", h.HandleStreamTrack)

	// Apply auth middleware to protected routes
	mux.Handle("/api/library/", Chain(protected, AuthMiddleware(jwtSecret)))
	mux.Handle("/api/playlists", Chain(protected, AuthMiddleware(jwtSecret)))
	mux.Handle("/api/playlists/", Chain(protected, AuthMiddleware(jwtSecret)))
	mux.Handle("/api/stream/", Chain(protected, AuthMiddleware(jwtSecret)))

	// --- Static file serving (Web UI) ---
	if webDir != "" {
		// Serve CSS/JS at /static/
		staticFS := http.FileServer(http.Dir(webDir))
		mux.Handle("/static/", http.StripPrefix("/static/", staticFS))

		// Serve index.html for the root (SPA entry point)
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, webDir+"/index.html")
		})

		log.Printf("[SERVER] Serving Web UI from %s", webDir)
	}

	// Apply global middleware: logging and CORS
	handler := Chain(mux, LoggingMiddleware, CORSMiddleware)

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 60 * time.Second, // longer for audio streaming
			IdleTimeout:  120 * time.Second,
		},
		handlers:  h,
		jwtSecret: jwtSecret,
	}
}

// Start launches the HTTP server and a background library scanner concurrently.
// Demonstrates: goroutine management with WaitGroup and context for graceful shutdown.
func (s *Server) Start(ctx context.Context, lib *library.Library, trackRepo *database.TrackRepo, scanPaths []string) error {
	var wg sync.WaitGroup

	// --- Background goroutine: periodic library scan ---
	// Concurrency requirement: goroutines for async workflows
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.backgroundScanner(ctx, lib, trackRepo, scanPaths)
	}()

	// --- Main goroutine: HTTP server ---
	serverErr := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("[SERVER] Listening on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for context cancellation (e.g., SIGINT)
	select {
	case <-ctx.Done():
		log.Println("[SERVER] Shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	}
}

// backgroundScanner periodically rescans music directories using goroutines
// and syncs discovered tracks to PostgreSQL via TrackRepo.
func (s *Server) backgroundScanner(ctx context.Context, lib *library.Library, trackRepo *database.TrackRepo, paths []string) {
	if len(paths) == 0 {
		log.Println("[SCANNER] No scan paths configured, background scanner disabled")
		return
	}

	// Perform an initial scan at startup
	log.Printf("[SCANNER] Starting initial library scan of %d directories...", len(paths))
	if err := lib.Scan(ctx, paths); err != nil {
		log.Printf("[SCANNER] Initial scan error: %v", err)
	}

	// Sync scanned tracks to PostgreSQL
	syncTracksToDatabase(ctx, lib, trackRepo)

	// Schedule periodic rescans every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[SCANNER] Background scanner stopped")
			return
		case <-ticker.C:
			log.Println("[SCANNER] Starting periodic rescan...")
			if err := lib.Scan(ctx, paths); err != nil {
				log.Printf("[SCANNER] Periodic scan error: %v", err)
			}
			syncTracksToDatabase(ctx, lib, trackRepo)
		}
	}
}

// syncTracksToDatabase upserts all in-memory library tracks into PostgreSQL.
// This runs concurrently as part of the background scanner goroutine.
func syncTracksToDatabase(ctx context.Context, lib *library.Library, trackRepo *database.TrackRepo) {
	tracks := lib.GetAllTracks()
	synced := 0
	for _, t := range tracks {
		if err := trackRepo.Upsert(ctx, t); err != nil {
			log.Printf("[SCANNER] Failed to sync track %q: %v", t.Title, err)
			continue
		}
		synced++
	}
	log.Printf("[SCANNER] Synced %d/%d tracks to PostgreSQL", synced, len(tracks))
}
