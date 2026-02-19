package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/audio"
	"github.com/jscyril/golang_music_player/internal/library"
	"github.com/jscyril/golang_music_player/internal/playlist"
	"github.com/jscyril/golang_music_player/internal/ui/views"
)

// ViewType represents the current active view
type ViewType int

const (
	ViewPlayer ViewType = iota
	ViewLibrary
	ViewPlaylist
)

// Model is the main bubbletea model
type Model struct {
	// Dimensions
	width  int
	height int

	// Current view
	activeView ViewType

	// Views
	playerView   views.PlayerView
	libraryView  views.LibraryView
	playlistView views.PlaylistView

	// Components
	audioEngine     *audio.AudioEngine
	library         *library.Library
	playlistManager *playlist.Manager
	queue           *playlist.Queue

	// State
	ctx    context.Context
	cancel context.CancelFunc
	err    error

	// Styles
	tabStyle       lipgloss.Style
	activeTabStyle lipgloss.Style
	headerStyle    lipgloss.Style
}

// TickMsg is sent periodically to update the UI
type TickMsg time.Time

// StateUpdateMsg is sent when playback state changes
type StateUpdateMsg struct {
	State *api.PlaybackState
}

// NewModel creates a new application model
func NewModel(engine *audio.AudioEngine, lib *library.Library, plManager *playlist.Manager) Model {
	ctx, cancel := context.WithCancel(context.Background())

	m := Model{
		width:           80,
		height:          24,
		activeView:      ViewLibrary,
		audioEngine:     engine,
		library:         lib,
		playlistManager: plManager,
		queue:           playlist.NewQueue(),
		ctx:             ctx,
		cancel:          cancel,
		tabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Foreground(lipgloss.Color("240")),
		activeTabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Background(lipgloss.Color("236")),
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1),
	}

	// Initialize views
	m.playerView = views.NewPlayerView(m.width, m.height/3)
	m.libraryView = views.NewLibraryView(m.width, m.height-10)
	m.playlistView = views.NewPlaylistView(m.width, m.height-10)

	// Load library tracks into view
	m.libraryView.SetTracks(lib.GetAllTracks())

	// Load playlists
	m.playlistView.SetPlaylists(plManager.GetAll())

	return m
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		m.listenForEvents(),
	)
}

// tickCmd returns a command that ticks every 500ms
func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// listenForEvents returns a command that listens for audio events
func (m Model) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.audioEngine.Events():
			switch event.Type {
			case api.EventStateChange, api.EventTrackStarted, api.EventPositionUpdate:
				return StateUpdateMsg{State: m.audioEngine.GetState()}
			case api.EventTrackEnded:
				// Auto-advance to next track
				if next := m.queue.Next(); next != nil {
					m.audioEngine.Play(next)
				}
				return StateUpdateMsg{State: m.audioEngine.GetState()}
			}
		case <-m.ctx.Done():
			return nil
		}
		return nil
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateViewSizes()

	case TickMsg:
		// Update playback state
		state := m.audioEngine.GetState()
		m.playerView.SetState(state)
		cmds = append(cmds, tickCmd())

	case StateUpdateMsg:
		m.playerView.SetState(msg.State)
		cmds = append(cmds, m.listenForEvents())

	case views.FileAddedMsg:
		// Add file to library
		track, err := m.library.AddFile(msg.Path)
		if err != nil {
			m.err = err
		} else {
			// Update the library view with the new track
			m.libraryView.AddTrack(track)
		}

	case tea.KeyMsg:
		// If library view is in search mode, pass keys directly to it
		// (except for critical global keys like quit)
		if m.activeView == ViewLibrary && (m.libraryView.Searching || m.libraryView.Browsing) {
			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				return m, tea.Quit
			default:
				m.libraryView, _ = m.libraryView.Update(msg)
				return m, tea.Batch(cmds...)
			}
		}

		// Global keybindings (only active when not searching)
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit

		case "1":
			m.activeView = ViewPlayer
		case "2":
			m.activeView = ViewLibrary
		case "3":
			m.activeView = ViewPlaylist

		case "tab":
			m.activeView = (m.activeView + 1) % 3

		case " ": // Space - play/pause
			state := m.audioEngine.GetState()
			if state.Status == api.StatusPlaying {
				m.audioEngine.Pause()
			} else if state.Status == api.StatusPaused {
				m.audioEngine.Resume()
			} else if m.queue.Current() != nil {
				m.audioEngine.Play(m.queue.Current())
			}

		case "s": // Stop
			m.audioEngine.Stop()

		case "n": // Next
			if next := m.queue.Next(); next != nil {
				m.audioEngine.Play(next)
			}

		case "p": // Previous (only in player view)
			if m.activeView == ViewPlayer {
				if prev := m.queue.Previous(); prev != nil {
					m.audioEngine.Play(prev)
				}
			}

		case "+", "=": // Volume up
			state := m.audioEngine.GetState()
			newVol := state.Volume + 0.1
			if newVol > 1 {
				newVol = 1
			}
			m.audioEngine.SetVolume(newVol)

		case "-": // Volume down
			state := m.audioEngine.GetState()
			newVol := state.Volume - 0.1
			if newVol < 0 {
				newVol = 0
			}
			m.audioEngine.SetVolume(newVol)

		case "r": // Toggle repeat
			mode := m.queue.GetRepeatMode()
			newMode := (mode + 1) % 3
			m.queue.SetRepeatMode(newMode)

		case "S": // Toggle shuffle
			if m.queue.IsShuffled() {
				m.queue.Unshuffle()
			} else {
				m.queue.Shuffle()
			}

		case "enter":
			// Play selected track
			var track *api.Track
			switch m.activeView {
			case ViewLibrary:
				track = m.libraryView.SelectedTrack()
				if track != nil {
					// Set queue to all library tracks starting from selected
					tracks := m.library.GetAllTracks()
					m.queue.Set(tracks)
					for i, t := range tracks {
						if t.ID == track.ID {
							m.queue.JumpTo(i)
							break
						}
					}
				}
			case ViewPlaylist:
				track = m.playlistView.SelectedTrack()
				if track != nil {
					// Set queue to playlist tracks
					pl := m.playlistView.SelectedPlaylist()
					if pl != nil {
						tracks := make([]*api.Track, len(pl.Tracks))
						for i := range pl.Tracks {
							tracks[i] = &pl.Tracks[i]
						}
						m.queue.Set(tracks)
						for i, t := range tracks {
							if t.ID == track.ID {
								m.queue.JumpTo(i)
								break
							}
						}
					}
				}
			}
			if track != nil {
				m.audioEngine.Play(track)
			}

		default:
			// Pass to active view
			switch m.activeView {
			case ViewLibrary:
				m.libraryView, _ = m.libraryView.Update(msg)
			case ViewPlaylist:
				m.playlistView, _ = m.playlistView.Update(msg)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// updateViewSizes updates view dimensions
func (m *Model) updateViewSizes() {
	m.playerView.Width = m.width
	m.playerView.Height = 10
	m.libraryView.Width = m.width
	m.libraryView.Height = m.height - 12
	m.playlistView.Width = m.width
	m.playlistView.Height = m.height - 12
}

// View renders the UI
func (m Model) View() string {
	var sb string

	// Header with tabs
	sb += m.renderTabs()
	sb += "\n"

	// Main content
	switch m.activeView {
	case ViewPlayer:
		sb += m.playerView.View()
	case ViewLibrary:
		sb += m.playerView.View()
		sb += "\n"
		sb += m.libraryView.View()
	case ViewPlaylist:
		sb += m.playerView.View()
		sb += "\n"
		sb += m.playlistView.View()
	}

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sb += "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	return sb
}

// renderTabs renders the tab bar
func (m Model) renderTabs() string {
	tabs := []string{"[1] Player", "[2] Library", "[3] Playlist"}

	var rendered []string
	for i, tab := range tabs {
		if ViewType(i) == m.activeView {
			rendered = append(rendered, m.activeTabStyle.Render(tab))
		} else {
			rendered = append(rendered, m.tabStyle.Render(tab))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// Run starts the bubbletea program
func Run(engine *audio.AudioEngine, lib *library.Library, plManager *playlist.Manager) error {
	model := NewModel(engine, lib, plManager)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
