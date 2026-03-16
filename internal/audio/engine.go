package audio

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/logger"
	playerrors "github.com/jscyril/golang_music_player/pkg/errors"
)

var _ api.Player = (*AudioEngine)(nil)

type AudioEngine struct {
	state      *api.PlaybackState
	commands   chan api.AudioCommand
	events     chan api.AudioEvent
	mu         sync.RWMutex
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	volume     *effects.Volume
	format     beep.Format
	done       chan struct{}
	sampleRate beep.SampleRate // speaker sample rate (fixed at init)
	trackRate  beep.SampleRate // current track's native sample rate
}

func NewAudioEngine() *AudioEngine {
	return &AudioEngine{
		state: &api.PlaybackState{
			Status: api.StatusStopped,
			Volume: 0.5,
			Repeat: api.RepeatNone,
		},
		commands: make(chan api.AudioCommand, 10),
		events:   make(chan api.AudioEvent, 20),
		done:     make(chan struct{}),
	}
}

func (e *AudioEngine) Start(ctx context.Context) error {
	// Initialize the speaker ONCE with a standard sample rate.
	// Calling speaker.Init() more than once causes the oto backend to panic.
	e.sampleRate = beep.SampleRate(44100)
	if err := speaker.Init(e.sampleRate, e.sampleRate.N(time.Second/10)); err != nil {
		logger.Error("Speaker init failed: %v", err)
		return fmt.Errorf("speaker init: %w", err)
	}
	logger.Info("Audio engine started (sample_rate=%d)", e.sampleRate)
	go e.run(ctx)
	go e.trackPosition(ctx)
	return nil
}

func (e *AudioEngine) Events() <-chan api.AudioEvent {
	return e.events
}

func (e *AudioEngine) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			e.cleanup()
			return

		case cmd := <-e.commands:
			switch cmd.Type {
			case api.CmdPlay:
				track := cmd.Payload.(*api.Track)
				logger.Info("Play command received: %q by %s (%s)", track.Title, track.Artist, track.FilePath)
				if err := e.playTrack(track); err != nil {
					logger.Error("Failed to play track %q: %v", track.Title, err)
					e.events <- api.AudioEvent{Type: api.EventError, Payload: err}
				}

			case api.CmdPause:
				logger.Debug("Pause command received")
				speaker.Lock()
				e.mu.Lock()
				if e.ctrl != nil {
					e.ctrl.Paused = true
					e.state.Status = api.StatusPaused
				}
				e.mu.Unlock()
				speaker.Unlock()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdResume:
				speaker.Lock()
				e.mu.Lock()
				if e.ctrl != nil {
					e.ctrl.Paused = false
					e.state.Status = api.StatusPlaying
				}
				e.mu.Unlock()
				speaker.Unlock()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdStop:
				e.stopPlayback()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdVolume:
				level := cmd.Payload.(float64)
				speaker.Lock()
				e.mu.Lock()
				if e.volume != nil {
					// Convert 0-1 range to decibel-like scale
					e.volume.Volume = level*2 - 1 // -1 to 1 range
				}
				e.state.Volume = level
				e.mu.Unlock()
				speaker.Unlock()

			case api.CmdSeek:
				pos := cmd.Payload.(time.Duration)
				e.seekTo(pos)
			}
		}
	}
}

func (e *AudioEngine) trackPosition(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			speaker.Lock()
			e.mu.RLock()
			if e.state.Status == api.StatusPlaying && e.streamer != nil {
				pos := e.streamer.Position()
				e.state.Position = e.trackRate.D(pos)
			}
			e.mu.RUnlock()
			speaker.Unlock()

			// Send event outside of locks to avoid blocking
			e.mu.RLock()
			if e.state.Status == api.StatusPlaying {
				e.events <- api.AudioEvent{
					Type:    api.EventPositionUpdate,
					Payload: e.state.Position,
				}
			}
			e.mu.RUnlock()
		}
	}
}

func (e *AudioEngine) playTrack(track *api.Track) error {
	logger.Debug("Stopping previous playback before starting new track")
	e.stopPlayback()

	file, err := os.Open(track.FilePath)
	if err != nil {
		logger.Error("Failed to open file %s: %v", track.FilePath, err)
		return playerrors.NewPlayerError("open", track.ID, err)
	}

	streamer, format, err := DecodeAudio(file, track.FilePath)
	if err != nil {
		file.Close()
		logger.Error("Failed to decode %s: %v", track.FilePath, err)
		return playerrors.NewPlayerError("decode", track.ID, err)
	}

	logger.Debug("Decoded track: sample_rate=%d, channels=%d", format.SampleRate, format.NumChannels)

	// If the track's sample rate differs from the speaker's initialized rate,
	// wrap it in a resampler so we never need to call speaker.Init() again.
	var src beep.Streamer = streamer
	if format.SampleRate != e.sampleRate {
		logger.Info("Resampling track from %d to %d Hz", format.SampleRate, e.sampleRate)
		src = beep.Resample(4, format.SampleRate, e.sampleRate, streamer)
	}

	e.mu.Lock()
	e.streamer = streamer
	e.format = format
	e.trackRate = format.SampleRate
	e.ctrl = &beep.Ctrl{Streamer: src, Paused: false}
	e.volume = &effects.Volume{
		Streamer: e.ctrl,
		Base:     2,
		Volume:   e.state.Volume*2 - 1,
		Silent:   false,
	}
	e.state.CurrentTrack = track
	// Backfill duration from the decoded stream if the track was scanned
	// before duration computation was added (e.g. loaded from a cached library).
	if track.Duration == 0 && format.SampleRate > 0 && streamer.Len() > 0 {
		track.Duration = format.SampleRate.D(streamer.Len())
	}
	e.state.Status = api.StatusPlaying
	e.state.Position = 0
	e.mu.Unlock()

	speaker.Play(beep.Seq(e.volume, beep.Callback(func() {
		logger.Info("Track ended: %q", track.Title)
		e.events <- api.AudioEvent{Type: api.EventTrackEnded, Payload: track}
	})))

	logger.Info("Track started: %q by %s", track.Title, track.Artist)
	e.events <- api.AudioEvent{Type: api.EventTrackStarted, Payload: track}
	return nil
}

func (e *AudioEngine) stopPlayback() {
	logger.Debug("Stopping playback: clearing speaker")
	// speaker.Clear() has its own internal lock, call it first
	speaker.Clear()

	e.mu.Lock()
	streamer := e.streamer
	e.streamer = nil
	e.ctrl = nil
	e.volume = nil
	e.state.Status = api.StatusStopped
	e.state.Position = 0
	e.mu.Unlock()

	// Close streamer outside of locks
	if streamer != nil {
		streamer.Close()
	}
}

func (e *AudioEngine) seekTo(pos time.Duration) {
	speaker.Lock()
	e.mu.Lock()
	defer e.mu.Unlock()
	defer speaker.Unlock()

	if e.streamer != nil {
		newPos := e.trackRate.N(pos)
		if newPos < 0 {
			newPos = 0
		}
		if length := e.streamer.Len(); newPos >= length {
			newPos = length - 1
		}
		if err := e.streamer.Seek(newPos); err == nil {
			e.state.Position = pos
		}
	}
}

func (e *AudioEngine) cleanup() {
	logger.Info("Audio engine shutting down")
	e.stopPlayback()
	close(e.events)
}

func (e *AudioEngine) Play(track *api.Track) error {
	if track == nil {
		return playerrors.ErrTrackNotFound
	}
	e.commands <- api.AudioCommand{Type: api.CmdPlay, Payload: track}
	return nil
}

func (e *AudioEngine) Pause() error {
	e.commands <- api.AudioCommand{Type: api.CmdPause}
	return nil
}
func (e *AudioEngine) Resume() error {
	e.commands <- api.AudioCommand{Type: api.CmdResume}
	return nil
}

func (e *AudioEngine) Stop() error {
	e.commands <- api.AudioCommand{Type: api.CmdStop}
	return nil
}

func (e *AudioEngine) Seek(position time.Duration) error {
	e.commands <- api.AudioCommand{Type: api.CmdSeek, Payload: position}
	return nil
}

func (e *AudioEngine) SetVolume(level float64) error {
	if level < 0 || level > 1 {
		return playerrors.ErrInvalidVolume
	}
	e.commands <- api.AudioCommand{Type: api.CmdVolume, Payload: level}
	return nil
}

func (e *AudioEngine) GetState() *api.PlaybackState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state := *e.state
	if e.state.CurrentTrack != nil {
		track := *e.state.CurrentTrack
		state.CurrentTrack = &track
	}
	return &state
}
