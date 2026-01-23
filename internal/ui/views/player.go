package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/ui/components"
)

// PlayerView displays the current playback state
type PlayerView struct {
	Width       int
	Height      int
	State       *api.PlaybackState
	ProgressBar components.ProgressBar

	// Styles
	TitleStyle    lipgloss.Style
	ArtistStyle   lipgloss.Style
	AlbumStyle    lipgloss.Style
	StatusStyle   lipgloss.Style
	ControlsStyle lipgloss.Style
	BorderStyle   lipgloss.Style
}

// NewPlayerView creates a new player view
func NewPlayerView(width, height int) PlayerView {
	return PlayerView{
		Width:       width,
		Height:      height,
		ProgressBar: components.NewProgressBar(width - 4),
		TitleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1),
		ArtistStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")),
		AlbumStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true),
		StatusStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true),
		ControlsStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1),
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
	}
}

// SetState updates the playback state
func (v *PlayerView) SetState(state *api.PlaybackState) {
	v.State = state
	if state != nil && state.CurrentTrack != nil {
		v.ProgressBar.SetProgress(state.Position, state.CurrentTrack.Duration)
	}
}

// Update handles messages
func (v PlayerView) Update(msg tea.Msg) (PlayerView, tea.Cmd) {
	return v, nil
}

// View renders the player view
func (v PlayerView) View() string {
	var sb strings.Builder

	if v.State == nil || v.State.CurrentTrack == nil {
		sb.WriteString(v.TitleStyle.Render("♪ No track playing"))
		sb.WriteString("\n\n")
		sb.WriteString(v.ControlsStyle.Render("Press Enter on a track to play"))
	} else {
		track := v.State.CurrentTrack

		// Status icon
		var statusIcon string
		switch v.State.Status {
		case api.StatusPlaying:
			statusIcon = "▶"
		case api.StatusPaused:
			statusIcon = "⏸"
		default:
			statusIcon = "⏹"
		}

		// Track info
		sb.WriteString(v.StatusStyle.Render(statusIcon + " "))
		sb.WriteString(v.TitleStyle.Render(track.Title))
		sb.WriteString("\n")
		sb.WriteString(v.ArtistStyle.Render(track.Artist))
		sb.WriteString("\n")
		sb.WriteString(v.AlbumStyle.Render(track.Album))
		sb.WriteString("\n\n")

		// Progress bar
		sb.WriteString(v.ProgressBar.View())
		sb.WriteString("\n\n")

		// Volume
		volumeBar := renderVolumeBar(v.State.Volume)
		sb.WriteString(fmt.Sprintf("Volume: %s %d%%", volumeBar, int(v.State.Volume*100)))
		sb.WriteString("\n")

		// Repeat/Shuffle status
		var modes []string
		switch v.State.Repeat {
		case api.RepeatOne:
			modes = append(modes, "🔂 Repeat One")
		case api.RepeatAll:
			modes = append(modes, "🔁 Repeat All")
		}
		if v.State.Shuffle {
			modes = append(modes, "🔀 Shuffle")
		}
		if len(modes) > 0 {
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(strings.Join(modes, " | ")))
		}
	}

	sb.WriteString("\n\n")
	sb.WriteString(v.ControlsStyle.Render(
		"[Space] Play/Pause  [s] Stop  [n] Next  [p] Prev  [+/-] Volume  [q] Quit",
	))

	return v.BorderStyle.Width(v.Width - 4).Render(sb.String())
}

// renderVolumeBar renders a volume bar
func renderVolumeBar(volume float64) string {
	filled := int(volume * 10)
	empty := 10 - filled

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return filledStyle.Render(strings.Repeat("●", filled)) + emptyStyle.Render(strings.Repeat("○", empty))
}
