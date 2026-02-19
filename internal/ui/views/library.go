package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/ui/components"
)

// FileAddedMsg is sent when a file is added via the file browser
type FileAddedMsg struct {
	Path string
}

// LibraryView displays the music library
type LibraryView struct {
	Width       int
	Height      int
	TrackList   components.TrackList
	SearchBar   components.SearchInput
	FileBrowser components.FileBrowser
	Searching   bool
	Browsing    bool // True when file browser is open
	AllTracks   []*api.Track
	BorderStyle lipgloss.Style
	TitleStyle  lipgloss.Style
}

// NewLibraryView creates a new library view
func NewLibraryView(width, height int) LibraryView {
	trackList := components.NewTrackList(height-8, width-6)
	trackList.Title = "🎵 Library"

	return LibraryView{
		Width:       width,
		Height:      height,
		TrackList:   trackList,
		SearchBar:   components.NewSearchInput(width - 6),
		FileBrowser: components.NewFileBrowser("", width, height),
		AllTracks:   make([]*api.Track, 0),
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
		TitleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
	}
}

// SetTracks sets the library tracks
func (v *LibraryView) SetTracks(tracks []*api.Track) {
	v.AllTracks = tracks
	v.TrackList.SetItems(tracks)
}

// AddTrack adds a track to the view
func (v *LibraryView) AddTrack(track *api.Track) {
	v.AllTracks = append(v.AllTracks, track)
	v.TrackList.SetItems(v.AllTracks)
}

// Update handles messages
func (v LibraryView) Update(msg tea.Msg) (LibraryView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle file browser mode
		if v.Browsing {
			switch msg.String() {
			case "esc":
				v.Browsing = false
				return v, nil
			case "enter":
				// Try to open/add the selected entry
				filePath := v.FileBrowser.EnterSelected()
				if filePath != "" {
					// File was selected, send message to add it
					v.Browsing = false
					return v, func() tea.Msg {
						return FileAddedMsg{Path: filePath}
					}
				}
				// Otherwise it was a directory navigation, stay in browser
				return v, nil
			default:
				v.FileBrowser, _ = v.FileBrowser.Update(msg)
			}
			return v, nil
		}

		// Handle search mode
		if v.Searching {
			switch msg.String() {
			case "enter", "esc":
				v.Searching = false
				v.SearchBar.Blur()
				// Filter tracks based on search
				if v.SearchBar.Value != "" {
					v.filterTracks(v.SearchBar.Value)
				} else {
					v.TrackList.SetItems(v.AllTracks)
				}
				return v, nil
			default:
				v.SearchBar, _ = v.SearchBar.Update(msg)
				// Live filtering
				v.filterTracks(v.SearchBar.Value)
			}
		} else {
			// Normal mode
			switch msg.String() {
			case "/":
				v.Searching = true
				v.SearchBar.Focus()
				return v, nil
			case "a":
				// Open file browser
				v.Browsing = true
				v.FileBrowser = components.NewFileBrowser("", v.Width, v.Height)
				return v, nil
			default:
				v.TrackList, _ = v.TrackList.Update(msg)
			}
		}
	}
	return v, nil
}

// filterTracks filters tracks based on search query
func (v *LibraryView) filterTracks(query string) {
	if query == "" {
		v.TrackList.SetItems(v.AllTracks)
		return
	}

	query = strings.ToLower(query)
	filtered := make([]*api.Track, 0)
	for _, track := range v.AllTracks {
		if strings.Contains(strings.ToLower(track.Title), query) ||
			strings.Contains(strings.ToLower(track.Artist), query) ||
			strings.Contains(strings.ToLower(track.Album), query) {
			filtered = append(filtered, track)
		}
	}
	v.TrackList.SetItems(filtered)
}

// SelectedTrack returns the currently selected track
func (v *LibraryView) SelectedTrack() *api.Track {
	return v.TrackList.SelectedItem()
}

// View renders the library view
func (v LibraryView) View() string {
	// If browsing, show file browser instead
	if v.Browsing {
		return v.FileBrowser.View()
	}

	var sb strings.Builder

	// Search bar
	sb.WriteString(v.SearchBar.View())
	sb.WriteString("\n\n")

	// Track list
	sb.WriteString(v.TrackList.View())

	// Help
	sb.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	if v.Searching {
		sb.WriteString(helpStyle.Render("[Enter] Confirm  [Esc] Cancel"))
	} else {
		sb.WriteString(helpStyle.Render("[/] Search  [a] Add Files  [Enter] Play  [↑↓] Navigate"))
	}

	return v.BorderStyle.Width(v.Width - 4).Render(sb.String())
}
