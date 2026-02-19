package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jscyril/golang_music_player/internal/audio"
	"github.com/jscyril/golang_music_player/internal/config"
	"github.com/jscyril/golang_music_player/internal/library"
	"github.com/jscyril/golang_music_player/internal/playlist"
	"github.com/jscyril/golang_music_player/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Load configuration
	configPath := config.GetConfigPath()
	cfg, err := config.LoadOrCreate(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Create data directory
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Setup context with graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Initialize audio engine
	audioEngine := audio.NewAudioEngine()
	audioEngine.Start(ctx)

	// Load persisted library (or create empty)
	libraryPath := filepath.Join(cfg.DataDir, "library.json")
	lib, err := library.LoadLibrary(libraryPath)
	if err != nil {
		return fmt.Errorf("load library: %w", err)
	}
	fmt.Printf("Loaded %d tracks from library\n", lib.TotalTracks)

	// Scan only if library is empty and directories are configured
	if lib.TotalTracks == 0 && len(cfg.MusicDirectories) > 0 {
		fmt.Println("Library empty, scanning music directories...")
		if err := lib.Scan(ctx, cfg.MusicDirectories); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: scan error: %v\n", err)
		}
		fmt.Printf("Found %d tracks\n", lib.TotalTracks)
	}

	// Save library on exit
	defer func() {
		if err := lib.Save(libraryPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: save library: %v\n", err)
		}
	}()

	// Initialize playlist manager
	playlistPath := filepath.Join(cfg.DataDir, "playlists")
	plManager := playlist.NewManager(playlistPath)
	if err := plManager.LoadAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: load playlists: %v\n", err)
	}

	// Run UI
	if err := ui.Run(audioEngine, lib, plManager); err != nil {
		return fmt.Errorf("run ui: %w", err)
	}

	return nil
}
