package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/ui/components"
)

// PlaylistView displays playlist management
type PlaylistView struct {
	Width       int
	Height      int
	TrackList   components.TrackList
	Playlists   []*api.Playlist
	Current     *api.Playlist
	ShowingList bool // true = showing playlists, false = showing tracks
	Selected    int
	BorderStyle lipgloss.Style
	TitleStyle  lipgloss.Style
}

// NewPlaylistView creates a new playlist view
func NewPlaylistView(width, height int) PlaylistView {
	trackList := components.NewTrackList(height-8, width-6)
	trackList.Title = "📋 Playlist"

	return PlaylistView{
		Width:       width,
		Height:      height,
		TrackList:   trackList,
		Playlists:   make([]*api.Playlist, 0),
		ShowingList: true,
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
		TitleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
	}
}

// SetPlaylists sets the available playlists
func (v *PlaylistView) SetPlaylists(playlists []*api.Playlist) {
	v.Playlists = playlists
}

// SetCurrentPlaylist sets the current playlist to display
func (v *PlaylistView) SetCurrentPlaylist(playlist *api.Playlist) {
	v.Current = playlist
	v.ShowingList = false
	if playlist != nil {
		tracks := make([]*api.Track, len(playlist.Tracks))
		for i := range playlist.Tracks {
			tracks[i] = &playlist.Tracks[i]
		}
		v.TrackList.SetItems(tracks)
		v.TrackList.Title = "📋 " + playlist.Name
	}
}

// Update handles messages
func (v PlaylistView) Update(msg tea.Msg) (PlaylistView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if v.ShowingList {
			switch msg.String() {
			case "up", "k":
				if v.Selected > 0 {
					v.Selected--
				}
			case "down", "j":
				if v.Selected < len(v.Playlists)-1 {
					v.Selected++
				}
			case "enter":
				if v.Selected < len(v.Playlists) {
					v.SetCurrentPlaylist(v.Playlists[v.Selected])
				}
			}
		} else {
			switch msg.String() {
			case "backspace", "esc":
				v.ShowingList = true
				v.Current = nil
				return v, nil
			default:
				v.TrackList, _ = v.TrackList.Update(msg)
			}
		}
	}
	return v, nil
}

// SelectedTrack returns the currently selected track
func (v *PlaylistView) SelectedTrack() *api.Track {
	if v.ShowingList {
		return nil
	}
	return v.TrackList.SelectedItem()
}

// SelectedPlaylist returns the currently selected playlist
func (v *PlaylistView) SelectedPlaylist() *api.Playlist {
	if v.ShowingList && v.Selected < len(v.Playlists) {
		return v.Playlists[v.Selected]
	}
	return v.Current
}

// View renders the playlist view
func (v PlaylistView) View() string {
	var sb strings.Builder

	if v.ShowingList {
		// Show playlist list
		sb.WriteString(v.TitleStyle.Render("📋 Playlists"))
		sb.WriteString("\n\n")

		if len(v.Playlists) == 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No playlists yet"))
		} else {
			selectedStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230")).
				Bold(true).
				Padding(0, 1)
			normalStyle := lipgloss.NewStyle().Padding(0, 1)

			for i, pl := range v.Playlists {
				line := pl.Name
				if pl.Description != "" {
					line += " - " + pl.Description
				}
				line += lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
					" (" + string(rune('0'+len(pl.Tracks))) + " tracks)")

				if i == v.Selected {
					sb.WriteString(selectedStyle.Render(line))
				} else {
					sb.WriteString(normalStyle.Render(line))
				}
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
			"[Enter] Open  [↑↓] Navigate"))
	} else {
		// Show playlist tracks
		sb.WriteString(v.TrackList.View())
		sb.WriteString("\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
			"[Backspace/Esc] Back  [Enter] Play  [↑↓] Navigate"))
	}

	return v.BorderStyle.Width(v.Width - 4).Render(sb.String())
}
