package views

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
	"github.com/jscyril/golang_music_player/internal/library"
	"github.com/jscyril/golang_music_player/internal/ui/components"
)

// PlayerView displays the current playback state
type PlayerView struct {
	Width        int
	Height       int
	State        *api.PlaybackState
	ProgressBar  components.ProgressBar
	CoverArt     []string
	coverTrackID string

	// Styles
	TitleStyle    lipgloss.Style
	ArtistStyle   lipgloss.Style
	AlbumStyle    lipgloss.Style
	StatusStyle   lipgloss.Style
	ControlsStyle lipgloss.Style
	BorderStyle   lipgloss.Style
	MetaStyle     lipgloss.Style
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
		MetaStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")),
	}
}

// SetState updates the playback state
func (v *PlayerView) SetState(state *api.PlaybackState) {
	v.State = state
	if state != nil && state.CurrentTrack != nil {
		v.ProgressBar.SetProgress(state.Position, state.CurrentTrack.Duration)
		v.loadCoverArt(state.CurrentTrack)
	} else {
		v.coverTrackID = ""
		v.CoverArt = fallbackCoverArt()
	}
}

// Update handles messages
func (v PlayerView) Update(msg tea.Msg) (PlayerView, tea.Cmd) {
	return v, nil
}

// ProgressBarRow returns the screen row offset of the progress bar
// within the player view (relative to the top of the player view content).
// Layout: title, artist/status, album, progress.
// Plus border top and padding, so the progress line is five rows from the box top.
func (v *PlayerView) ProgressBarRow() int {
	return 5
}

// ProgressBarClickSeek converts a mouse click X position to a seek duration.
// barOffsetX is the X offset of the bar within the terminal (border + padding).
func (v *PlayerView) ProgressBarClickSeek(clickX, barOffsetX int) time.Duration {
	return v.ProgressBar.HandleClick(clickX, barOffsetX)
}

// ProgressBarColumn returns the terminal column where the progress bar starts
// inside the rendered player panel.
func (v *PlayerView) ProgressBarColumn() int {
	// border + horizontal padding + cover width + spacing between cover/details.
	return 1 + 2 + 14 + 2
}

// View renders the player view
func (v *PlayerView) View() string {
	var sb strings.Builder

	innerWidth := v.Width - 8
	if innerWidth < 40 {
		innerWidth = 40
	}

	if v.State == nil || v.State.CurrentTrack == nil {
		empty := strings.Join(fallbackCoverArt(), "\n")
		copy := v.StatusStyle.Render("Ready") + "\n" +
			v.TitleStyle.Render("No track playing") + "\n" +
			v.MetaStyle.Render("Press Enter on any track to start playback.") + "\n\n" +
			v.ControlsStyle.Render("[2] Library  [3] Playlist  [4] Queue")
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, empty, "  ", copy))
	} else {
		track := v.State.CurrentTrack

		var statusText string
		switch v.State.Status {
		case api.StatusPlaying:
			statusText = "Playing"
		case api.StatusPaused:
			statusText = "Paused"
		default:
			statusText = "Stopped"
		}

		rightWidth := innerWidth - 20
		if rightWidth < 32 {
			rightWidth = innerWidth
		}
		v.ProgressBar.Width = rightWidth - 2

		statusLine := v.StatusStyle.Render(statusText)
		if len(v.State.Queue) > 0 {
			statusLine += v.MetaStyle.Render(fmt.Sprintf("  Queue %d/%d", v.State.QueueIndex+1, len(v.State.Queue)))
		}

		var details strings.Builder
		details.WriteString(v.TitleStyle.Width(rightWidth).Render(track.Title))
		details.WriteString("\n")
		details.WriteString(v.ArtistStyle.Render(track.Artist))
		details.WriteString(v.MetaStyle.Render("  "))
		details.WriteString(statusLine)
		details.WriteString("\n")
		if track.Album != "" {
			details.WriteString(v.AlbumStyle.Render(track.Album))
		} else {
			details.WriteString(v.AlbumStyle.Render("Unknown Album"))
		}
		details.WriteString("\n")
		details.WriteString(v.ProgressBar.View())
		details.WriteString("\n")

		volumeBar := renderVolumeBar(v.State.Volume)
		details.WriteString(fmt.Sprintf("Vol %s %d%%", volumeBar, int(v.State.Volume*100)))

		var modes []string
		switch v.State.Repeat {
		case api.RepeatOne:
			modes = append(modes, "Repeat One")
		case api.RepeatAll:
			modes = append(modes, "Repeat All")
		}
		if v.State.Shuffle {
			modes = append(modes, "Shuffle")
		}
		if len(modes) > 0 {
			details.WriteString(v.MetaStyle.Render("  " + strings.Join(modes, " | ")))
		}
		if next := nextQueuedTrack(v.State); next != nil {
			details.WriteString("\n")
			details.WriteString(v.MetaStyle.Render("Next: "))
			details.WriteString(truncatePlayerText(fmt.Sprintf("%s - %s", next.Artist, next.Title), rightWidth-6))
		}

		cover := strings.Join(v.CoverArt, "\n")
		sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cover, "  ", details.String()))
	}

	sb.WriteString("\n")
	sb.WriteString(v.ControlsStyle.Render(
		"[Space] Play/Pause  [s] Stop  [n] Next  [p] Prev  [←/→] Seek  [b] Background  [q] Quit",
	))

	return renderFixedPanel(sb.String(), v.Width, v.Height, v.BorderStyle)
}

func (v *PlayerView) loadCoverArt(track *api.Track) {
	if track == nil || track.ID == v.coverTrackID {
		return
	}
	v.coverTrackID = track.ID
	v.CoverArt = fallbackCoverArt()

	data, err := library.NewMetadataReader().ReadCoverArt(track.FilePath)
	if err != nil || len(data) == 0 {
		return
	}
	lines, err := renderCoverArt(data, 14, 6)
	if err != nil {
		return
	}
	v.CoverArt = lines
}

func renderCoverArt(data []byte, width, height int) ([]string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return nil, fmt.Errorf("empty cover art")
	}

	lines := make([]string, height)
	for y := 0; y < height; y++ {
		var b strings.Builder
		for x := 0; x < width; x++ {
			upperY := bounds.Min.Y + ((y * 2) * srcH / (height * 2))
			lowerY := bounds.Min.Y + (((y * 2) + 1) * srcH / (height * 2))
			srcX := bounds.Min.X + (x * srcW / width)
			ur, ug, ub, _ := img.At(srcX, upperY).RGBA()
			lr, lg, lb, _ := img.At(srcX, lowerY).RGBA()
			b.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				ur>>8, ug>>8, ub>>8, lr>>8, lg>>8, lb>>8))
		}
		b.WriteString("\x1b[0m")
		lines[y] = b.String()
	}
	return lines, nil
}

func fallbackCoverArt() []string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true)
	return []string{
		style.Render("┌────────────┐"),
		style.Render("│            │"),
		style.Render("│     ") + accent.Render("♪") + style.Render("      │"),
		style.Render("│   GTMPC    │"),
		style.Render("│            │"),
		style.Render("└────────────┘"),
	}
}

// renderVolumeBar renders a volume bar
func renderVolumeBar(volume float64) string {
	filled := int(volume * 10)
	empty := 10 - filled

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	return filledStyle.Render(strings.Repeat("●", filled)) + emptyStyle.Render(strings.Repeat("○", empty))
}

func nextQueuedTrack(state *api.PlaybackState) *api.Track {
	if state == nil || len(state.Queue) == 0 {
		return nil
	}
	nextIndex := state.QueueIndex + 1
	if nextIndex >= len(state.Queue) {
		return nil
	}
	return state.Queue[nextIndex]
}

func truncatePlayerText(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
