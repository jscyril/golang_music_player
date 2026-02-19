package audio

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/speaker"
	"github.com/jscyril/golang_music_player/api"
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
	sampleRate beep.SampleRate
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

func (e *AudioEngine) Start(ctx context.Context) {
	go e.run(ctx)
	go e.trackPosition(ctx)
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
				if err := e.playTrack(track); err != nil {
					e.events <- api.AudioEvent{Type: api.EventError, Payload: err}
				}

			case api.CmdPause:
				e.mu.Lock()
				if e.ctrl != nil {
					e.ctrl.Paused = true
					e.state.Status = api.StatusPaused
				}
				e.mu.Unlock()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdResume:
				e.mu.Lock()
				if e.ctrl != nil {
					e.ctrl.Paused = false
					e.state.Status = api.StatusPlaying
				}
				e.mu.Unlock()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdStop:
				e.stopPlayback()
				e.events <- api.AudioEvent{Type: api.EventStateChange, Payload: e.state}

			case api.CmdVolume:
				level := cmd.Payload.(float64)
				e.mu.Lock()
				if e.volume != nil {
					// Convert 0-1 range to decibel-like scale
					e.volume.Volume = level*2 - 1 // -1 to 1 range
				}
				e.state.Volume = level
				e.mu.Unlock()

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
			e.mu.RLock()
			if e.state.Status == api.StatusPlaying && e.streamer != nil {
				pos := e.streamer.Position()
				e.state.Position = e.sampleRate.D(pos)
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
	e.stopPlayback()

	file, err := os.Open(track.FilePath)
	if err != nil {
		return playerrors.NewPlayerError("open", track.ID, err)
	}

	streamer, format, err := DecodeAudio(file, track.FilePath)
	if err != nil {
		file.Close()
		return playerrors.NewPlayerError("decode", track.ID, err)
	}

	e.mu.Lock()
	e.streamer = streamer
	e.format = format
	e.sampleRate = format.SampleRate
	e.ctrl = &beep.Ctrl{Streamer: streamer, Paused: false}
	e.volume = &effects.Volume{
		Streamer: e.ctrl,
		Base:     2,
		Volume:   e.state.Volume*2 - 1,
		Silent:   false,
	}
	e.state.CurrentTrack = track
	e.state.Status = api.StatusPlaying
	e.state.Position = 0
	e.mu.Unlock()

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return playerrors.NewPlayerError("speaker_init", track.ID, err)
	}

	speaker.Play(beep.Seq(e.volume, beep.Callback(func() {
		e.events <- api.AudioEvent{Type: api.EventTrackEnded, Payload: track}
	})))

	e.events <- api.AudioEvent{Type: api.EventTrackStarted, Payload: track}
	return nil
}

func (e *AudioEngine) stopPlayback() {
	e.mu.Lock()
	defer e.mu.Unlock()

	speaker.Clear()
	if e.streamer != nil {
		e.streamer.Close()
		e.streamer = nil
	}
	e.ctrl = nil
	e.volume = nil
	e.state.Status = api.StatusStopped
	e.state.Position = 0
}

func (e *AudioEngine) seekTo(pos time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.streamer != nil {
		newPos := e.sampleRate.N(pos)
		if err := e.streamer.Seek(newPos); err == nil {
			e.state.Position = pos
		}
	}
}

func (e *AudioEngine) cleanup() {
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
