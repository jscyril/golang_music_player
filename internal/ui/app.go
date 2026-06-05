package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/audio"
	"github.com/jscyril/golang_music_player/internal/library"
	"github.com/jscyril/golang_music_player/internal/logger"
	"github.com/jscyril/golang_music_player/internal/playlist"
	"github.com/jscyril/golang_music_player/internal/ui/views"
)

var ErrBackground = errors.New("music player running in background")

// ViewType represents the current active view
type ViewType int

const (
	ViewPlayer ViewType = iota
	ViewLibrary
	ViewPlaylist
	ViewQueue
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
	queueView    views.QueueView

	// Components
	audioEngine     *audio.AudioEngine
	library         *library.Library
	playlistManager *playlist.Manager
	queue           *playlist.Queue

	// State
	ctx                 context.Context
	cancel              context.CancelFunc
	err                 error
	background          bool
	playlistPickerTrack *api.Track
	playlistPickerIndex int

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

// TrackEndedMsg is sent when a track finishes playing
type TrackEndedMsg struct{}

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
	m.playerView = views.NewPlayerView(m.width, 11)
	m.libraryView = views.NewLibraryView(m.width, m.contentPanelHeight())
	m.playlistView = views.NewPlaylistView(m.width, m.contentPanelHeight())
	m.queueView = views.NewQueueView(m.width, m.contentPanelHeight())

	// Load library tracks into view
	m.libraryView.SetTracks(lib.GetAllTracks())

	// Load playlists
	m.playlistView.SetPlaylists(plManager.GetAll())
	m.refreshQueueView()

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
		case event, ok := <-m.audioEngine.Events():
			if !ok {
				return nil
			}
			switch event.Type {
			case api.EventStateChange, api.EventTrackStarted, api.EventPositionUpdate:
				return StateUpdateMsg{State: m.currentPlaybackState()}
			case api.EventTrackEnded:
				return TrackEndedMsg{}
			case api.EventError:
				return StateUpdateMsg{State: m.currentPlaybackState()}
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
		state := m.currentPlaybackState()
		m.playerView.SetState(state)
		cmds = append(cmds, tickCmd())

	case StateUpdateMsg:
		m.playerView.SetState(msg.State)
		cmds = append(cmds, m.listenForEvents())

	case TrackEndedMsg:
		// Auto-advance to next track (handled inside Update for thread safety)
		logger.Debug("TrackEndedMsg received, advancing to next track")
		if next := m.queue.Next(); next != nil {
			logger.Info("Auto-advancing to next track: %q", next.Title)
			m.audioEngine.Play(next)
			m.refreshQueueView()
		} else {
			logger.Info("Queue exhausted, no next track")
		}
		state := m.currentPlaybackState()
		m.playerView.SetState(state)
		cmds = append(cmds, m.listenForEvents())

	case views.FileAddedMsg:
		// Add file to library
		logger.Info("Adding file to library: %s", msg.Path)
		track, err := m.library.AddFile(msg.Path)
		if err != nil {
			logger.Error("Failed to add file %s: %v", msg.Path, err)
			m.err = err
		} else {
			logger.Info("Added track: %q by %s", track.Title, track.Artist)
			// Update the library view with the new track
			m.libraryView.AddTrack(track)
		}

	case tea.KeyMsg:
		if m.playlistPickerTrack != nil {
			switch msg.String() {
			case "esc":
				m.playlistPickerTrack = nil
				return m, tea.Batch(cmds...)
			case "up", "k":
				if m.playlistPickerIndex > 0 {
					m.playlistPickerIndex--
				}
				return m, tea.Batch(cmds...)
			case "down", "j":
				playlists := m.playlistManager.GetAll()
				if m.playlistPickerIndex < len(playlists)-1 {
					m.playlistPickerIndex++
				}
				return m, tea.Batch(cmds...)
			case "enter":
				playlists := m.playlistManager.GetAll()
				if m.playlistPickerIndex >= 0 && m.playlistPickerIndex < len(playlists) {
					if err := m.playlistManager.AddTrack(playlists[m.playlistPickerIndex].ID, m.playlistPickerTrack); err != nil {
						m.err = err
					} else {
						m.refreshPlaylists()
						m.err = nil
					}
				}
				m.playlistPickerTrack = nil
				return m, tea.Batch(cmds...)
			}
		}

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

		if m.activeView == ViewPlaylist && m.playlistView.Creating {
			switch msg.String() {
			case "ctrl+c":
				m.cancel()
				return m, tea.Quit
			case "enter":
				name := m.playlistView.CreateName()
				if name == "" {
					m.err = fmt.Errorf("playlist name cannot be empty")
				} else if _, err := m.playlistManager.Create(name, ""); err != nil {
					m.err = err
				} else {
					m.err = nil
					m.playlistView.CancelCreate()
					m.refreshPlaylists()
				}
			default:
				m.playlistView, _ = m.playlistView.Update(msg)
			}
			return m, tea.Batch(cmds...)
		}

		if m.activeView == ViewPlaylist && m.playlistView.ShowingList && msg.String() == "enter" {
			m.playlistView, _ = m.playlistView.Update(msg)
			return m, tea.Batch(cmds...)
		}

		// Global keybindings (only active when not searching)
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "b":
			m.background = true
			m.cancel()
			return m, tea.Quit

		case "1":
			m.activeView = ViewPlayer
		case "2":
			m.activeView = ViewLibrary
		case "3":
			m.activeView = ViewPlaylist
		case "4":
			m.activeView = ViewQueue

		case "tab":
			m.activeView = (m.activeView + 1) % 4

		case " ": // Space - play/pause
			state := m.audioEngine.GetState()
			if state.Status == api.StatusPlaying {
				logger.Debug("User paused playback")
				m.audioEngine.Pause()
			} else if state.Status == api.StatusPaused {
				logger.Debug("User resumed playback")
				m.audioEngine.Resume()
			} else if m.queue.Current() != nil {
				logger.Debug("User started playback from stopped state")
				m.audioEngine.Play(m.queue.Current())
			}

		case "s": // Stop
			logger.Debug("User stopped playback")
			m.audioEngine.Stop()

		case "n": // Next
			if next := m.queue.Next(); next != nil {
				logger.Info("User skipped to next track: %q", next.Title)
				m.audioEngine.Play(next)
				m.refreshQueueView()
			}

		case "p": // Previous
			if prev := m.previousTrack(); prev != nil {
				m.audioEngine.Play(prev)
				m.refreshQueueView()
			}

		case "right": // Seek forward 5 seconds
			state := m.audioEngine.GetState()
			if state.Status == api.StatusPlaying || state.Status == api.StatusPaused {
				newPos := state.Position + 5*time.Second
				if state.CurrentTrack != nil && newPos > state.CurrentTrack.Duration {
					newPos = state.CurrentTrack.Duration
				}
				m.audioEngine.Seek(newPos)
			}

		case "left": // Seek backward 5 seconds
			state := m.audioEngine.GetState()
			if state.Status == api.StatusPlaying || state.Status == api.StatusPaused {
				newPos := state.Position - 5*time.Second
				if newPos < 0 {
					newPos = 0
				}
				m.audioEngine.Seek(newPos)
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
			m.playerView.SetState(m.currentPlaybackState())
			m.refreshQueueView()

		case "S": // Toggle shuffle
			if m.queue.IsShuffled() {
				m.queue.Unshuffle()
			} else {
				m.queue.Shuffle()
			}
			m.playerView.SetState(m.currentPlaybackState())
			m.refreshQueueView()

		case "A": // Add selected library track to a playlist
			if m.activeView == ViewLibrary {
				track := m.libraryView.SelectedTrack()
				playlists := m.playlistManager.GetAll()
				if track == nil {
					m.err = fmt.Errorf("select a track before adding to a playlist")
				} else if len(playlists) == 0 {
					m.err = fmt.Errorf("create a playlist first in the Playlist tab")
				} else {
					m.err = nil
					m.playlistPickerTrack = track
					m.playlistPickerIndex = 0
				}
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
					m.refreshQueueView()
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
						m.refreshQueueView()
					}
				}
			case ViewQueue:
				if idx := m.queueView.SelectedIndex(); idx >= 0 {
					if err := m.queue.JumpTo(idx); err != nil {
						m.err = err
					} else if track = m.queue.Current(); track != nil {
						m.refreshQueueView()
					}
				}
			}
			if track != nil {
				logger.Info("User selected track: %q by %s", track.Title, track.Artist)
				m.audioEngine.Play(track)
			}

		default:
			// Pass to active view
			switch m.activeView {
			case ViewLibrary:
				m.libraryView, _ = m.libraryView.Update(msg)
			case ViewPlaylist:
				switch msg.String() {
				case "c":
					if m.playlistView.ShowingList {
						m.playlistView.BeginCreate()
					}
				case "d":
					if m.playlistView.ShowingList {
						if pl := m.playlistView.SelectedPlaylist(); pl != nil {
							if err := m.playlistManager.Delete(pl.ID); err != nil {
								m.err = err
							} else {
								m.err = nil
								m.refreshPlaylists()
							}
						}
					}
				case "x":
					if !m.playlistView.ShowingList {
						pl := m.playlistView.SelectedPlaylist()
						track := m.playlistView.SelectedTrack()
						if pl != nil && track != nil {
							if err := m.playlistManager.RemoveTrack(pl.ID, track.ID); err != nil {
								m.err = err
							} else {
								m.err = nil
								m.refreshPlaylists()
							}
						}
					}
				default:
					m.playlistView, _ = m.playlistView.Update(msg)
				}
			case ViewQueue:
				switch msg.String() {
				case "x":
					m.removeQueueSelection()
				case "C":
					m.queue.Clear()
					m.audioEngine.Stop()
					m.err = nil
					m.refreshQueueView()
					m.playerView.SetState(m.currentPlaybackState())
				case "u":
					m.moveQueueSelection(-1)
				case "d":
					m.moveQueueSelection(1)
				default:
					m.queueView, _ = m.queueView.Update(msg)
				}
			}
		}

	case tea.MouseMsg:
		// Handle click-to-seek on progress bar
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			state := m.audioEngine.GetState()
			if state.Status == api.StatusPlaying || state.Status == api.StatusPaused {
				// The progress bar row is at a fixed offset from the top:
				// tab bar (1) + newline gap (1) + player border top (1) + padding (1)
				// + status/title (1) + artist (1) + album (1) + blank (1) = row 8 (0-indexed: 7)
				progressRow := 1 + m.playerView.ProgressBarRow() // tab + player offset
				if msg.Y == progressRow {
					barOffsetX := m.playerView.ProgressBarColumn()
					seekPos := m.playerView.ProgressBarClickSeek(msg.X, barOffsetX)
					m.audioEngine.Seek(seekPos)
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

// updateViewSizes updates view dimensions
func (m *Model) updateViewSizes() {
	m.playerView.Width = m.width
	m.playerView.Height = 11
	m.libraryView.Width = m.width
	m.libraryView.Height = m.contentPanelHeight()
	m.playlistView.Width = m.width
	m.playlistView.Height = m.contentPanelHeight()
	m.queueView.Width = m.width
	m.queueView.Height = m.contentPanelHeight()
}

func (m Model) contentPanelHeight() int {
	// One row for tabs and one row between player/content.
	h := m.height - m.playerView.Height - 2
	if h < 8 {
		return 8
	}
	return h
}

func (m *Model) refreshPlaylists() {
	m.playlistView.SetPlaylists(m.playlistManager.GetAll())
}

func (m *Model) refreshQueueView() {
	m.queueView.SetQueue(m.queue.GetAll(), m.queue.Index())
}

func (m *Model) removeQueueSelection() {
	idx := m.queueView.SelectedIndex()
	if idx < 0 {
		return
	}
	wasCurrent := idx == m.queue.Index()
	if err := m.queue.Remove(idx); err != nil {
		m.err = err
		return
	}
	m.err = nil
	if wasCurrent {
		if next := m.queue.Current(); next != nil {
			m.audioEngine.Play(next)
		} else {
			m.audioEngine.Stop()
		}
	}
	m.refreshQueueView()
	m.playerView.SetState(m.currentPlaybackState())
}

func (m *Model) moveQueueSelection(delta int) {
	from := m.queueView.SelectedIndex()
	to := from + delta
	if from < 0 || to < 0 || to >= m.queue.Len() {
		return
	}
	if err := m.queue.Move(from, to); err != nil {
		m.err = err
		return
	}
	m.err = nil
	m.queueView.Selected = to
	m.refreshQueueView()
	m.playerView.SetState(m.currentPlaybackState())
}

func (m *Model) previousTrack() *api.Track {
	state := m.audioEngine.GetState()
	if state.Position > 3*time.Second {
		if current := m.queue.Current(); current != nil {
			m.audioEngine.Seek(0)
			return nil
		}
	}
	return m.queue.Previous()
}

func (m Model) currentPlaybackState() *api.PlaybackState {
	state := m.audioEngine.GetState()
	state.Repeat = m.queue.GetRepeatMode()
	state.Shuffle = m.queue.IsShuffled()
	state.Queue = m.queue.GetAll()
	state.QueueIndex = m.queue.Index()
	return state
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
	case ViewQueue:
		sb += m.playerView.View()
		sb += "\n"
		sb += m.queueView.View()
	}

	// Error display
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)
		sb += "\n" + errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if m.playlistPickerTrack != nil {
		sb += "\n" + m.renderPlaylistPicker()
	}

	return sb
}

func (m Model) renderPlaylistPicker() string {
	playlists := m.playlistManager.GetAll()
	var b strings.Builder
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230")).Padding(0, 1)
	normalStyle := lipgloss.NewStyle().Padding(0, 1)
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(m.width - 4)

	b.WriteString(titleStyle.Render(fmt.Sprintf("Add %q to playlist", m.playlistPickerTrack.Title)))
	b.WriteString("\n\n")
	for i, pl := range playlists {
		line := fmt.Sprintf("%s (%d tracks)", pl.Name, len(pl.Tracks))
		if i == m.playlistPickerIndex {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(normalStyle.Render(line))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("[Enter] Add  [Esc] Cancel  [↑↓] Choose"))
	return boxStyle.Render(b.String())
}

// renderTabs renders the tab bar
func (m Model) renderTabs() string {
	tabs := []string{"[1] Player", "[2] Library", "[3] Playlist", "[4] Queue"}

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
	logger.Info("Starting UI")
	model := NewModel(engine, lib, plManager)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	finalModel, err := p.Run()
	if err != nil {
		logger.Error("UI exited with error: %v", err)
	} else {
		logger.Info("UI exited cleanly")
	}
	if m, ok := finalModel.(Model); ok && m.background {
		return ErrBackground
	}
	return err
}
