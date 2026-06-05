package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/jscyril/golang_music_player/internal/auth"
	"github.com/jscyril/golang_music_player/internal/config"
	"github.com/jscyril/golang_music_player/internal/database"
	"github.com/jscyril/golang_music_player/internal/library"
	"github.com/jscyril/golang_music_player/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal: %v", err)
	}
}

func run() error {
	// Load .env file (if present) so env vars are set automatically
	if err := godotenv.Load(); err != nil {
		log.Println("[ENV] No .env file found, using system environment variables")
	} else {
		log.Println("[ENV] Loaded configuration from .env")
	}

	log.Println("=== GTMPC Server ===")

	// --- Step 1: Load configuration ---
	configPath := config.GetConfigPath()
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		return err
	}
	log.Printf("[CONFIG] Loaded from %s", configPath)

	// --- Step 2: Ensure data directories exist ---
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// --- Step 3: Setup graceful shutdown ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[SIGNAL] Received %v, initiating shutdown...", sig)
		cancel()
	}()

	// --- Step 4: Connect to PostgreSQL ---
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/gtmpc?sslmode=disable"
		log.Println("[WARN] Using default DATABASE_URL. Set DATABASE_URL env var for production.")
	}

	db, err := database.New(ctx, dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// --- Step 5: Initialize repositories ---
	userRepo := database.NewUserRepo(db)
	trackRepo := database.NewTrackRepo(db)
	playlistRepo := database.NewPlaylistRepo(db)

	// --- Step 6: Initialize auth service (bcrypt + PostgreSQL) ---
	authDB := auth.NewDBService(userRepo)
	log.Println("[AUTH] Database-backed auth service initialized")

	// --- Step 7: Load music library (for scanning) ---
	libraryPath := filepath.Join(cfg.DataDir, "library.json")
	lib, err := library.LoadLibrary(libraryPath)
	if err != nil {
		return err
	}
	log.Printf("[LIBRARY] Loaded %d tracks from %s", lib.TotalTracks, libraryPath)

	// Defer library save on shutdown
	defer func() {
		if err := lib.Save(libraryPath); err != nil {
			log.Printf("[LIBRARY] Failed to save: %v", err)
		} else {
			log.Println("[LIBRARY] Saved successfully")
		}
	}()

	// --- Step 8: Resolve Web UI directory ---
	webDir := os.Getenv("GTMPC_WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	log.Printf("[WEB] Serving Web UI from: %s", webDir)

	// --- Step 9: Resolve upload directory ---
	uploadDir := os.Getenv("GTMPC_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = filepath.Join(cfg.DataDir, "uploads")
	}
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("create upload directory: %w", err)
	}
	log.Printf("[UPLOAD] Upload directory: %s", uploadDir)

	// --- Step 10: Start HTTP server ---
	jwtSecret := []byte(os.Getenv("GTMPC_JWT_SECRET"))
	if len(jwtSecret) == 0 {
		jwtSecret = []byte("gtmpc-default-secret-change-me")
		log.Println("[WARN] Using default JWT secret. Set GTMPC_JWT_SECRET in production.")
	}

	addr := os.Getenv("GTMPC_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := server.NewServer(addr, authDB, trackRepo, playlistRepo, jwtSecret, webDir, uploadDir)
	return srv.Start(ctx, lib, trackRepo, cfg.MusicDirectories)
}
