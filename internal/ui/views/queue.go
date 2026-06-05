package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
)

// QueueView displays and edits the active playback queue.
type QueueView struct {
	Width         int
	Height        int
	Tracks        []*api.Track
	CurrentIndex  int
	Selected      int
	Offset        int
	BorderStyle   lipgloss.Style
	TitleStyle    lipgloss.Style
	SelectedStyle lipgloss.Style
	CurrentStyle  lipgloss.Style
	NormalStyle   lipgloss.Style
	HelpStyle     lipgloss.Style
}

func NewQueueView(width, height int) QueueView {
	return QueueView{
		Width:        width,
		Height:       height,
		Tracks:       []*api.Track{},
		CurrentIndex: -1,
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
		TitleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")),
		SelectedStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true).
			Padding(0, 1),
		CurrentStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Padding(0, 1),
		NormalStyle: lipgloss.NewStyle().Padding(0, 1),
		HelpStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

func (v *QueueView) SetQueue(tracks []*api.Track, currentIndex int) {
	v.Tracks = tracks
	v.CurrentIndex = currentIndex
	if len(v.Tracks) == 0 {
		v.Selected = 0
		v.Offset = 0
		return
	}
	if v.Selected >= len(v.Tracks) {
		v.Selected = len(v.Tracks) - 1
	}
	if v.Selected < 0 {
		v.Selected = 0
	}
	v.ensureVisible()
}

func (v *QueueView) SelectedIndex() int {
	if v.Selected < 0 || v.Selected >= len(v.Tracks) {
		return -1
	}
	return v.Selected
}

func (v QueueView) Update(msg tea.Msg) (QueueView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.Selected > 0 {
				v.Selected--
				v.ensureVisible()
			}
		case "down", "j":
			if v.Selected < len(v.Tracks)-1 {
				v.Selected++
				v.ensureVisible()
			}
		case "home":
			v.Selected = 0
			v.Offset = 0
		case "end":
			if len(v.Tracks) > 0 {
				v.Selected = len(v.Tracks) - 1
				v.ensureVisible()
			}
		case "pgup":
			v.Selected -= v.visibleHeight()
			if v.Selected < 0 {
				v.Selected = 0
			}
			v.ensureVisible()
		case "pgdown":
			v.Selected += v.visibleHeight()
			if v.Selected >= len(v.Tracks) {
				v.Selected = len(v.Tracks) - 1
			}
			v.ensureVisible()
		}
	}
	return v, nil
}

func (v QueueView) View() string {
	var sb strings.Builder
	sb.WriteString(v.TitleStyle.Render(fmt.Sprintf("Queue (%d tracks)", len(v.Tracks))))
	sb.WriteString("\n\n")

	if len(v.Tracks) == 0 {
		sb.WriteString(v.NormalStyle.Render("Queue is empty. Press Enter on a library or playlist track to start one."))
		sb.WriteString("\n\n")
		sb.WriteString(v.HelpStyle.Render("[Enter] Jump  [x] Remove  [u/d] Move  [C] Clear  [↑↓] Navigate"))
		return renderFixedPanel(sb.String(), v.Width, v.Height, v.BorderStyle)
	}

	end := v.Offset + v.visibleHeight()
	if end > len(v.Tracks) {
		end = len(v.Tracks)
	}

	for i := v.Offset; i < end; i++ {
		track := v.Tracks[i]
		prefix := "   "
		if i == v.CurrentIndex {
			prefix = "▶  "
		}
		line := fmt.Sprintf("%s%3d. %-22s  %s", prefix, i+1, truncateForQueue(track.Artist, 22), truncateForQueue(track.Title, 36))
		if len(line) > v.Width-8 {
			line = line[:v.Width-11] + "..."
		}

		switch {
		case i == v.Selected:
			sb.WriteString(v.SelectedStyle.Render(line))
		case i == v.CurrentIndex:
			sb.WriteString(v.CurrentStyle.Render(line))
		default:
			sb.WriteString(v.NormalStyle.Render(line))
		}
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	if len(v.Tracks) > v.visibleHeight() {
		sb.WriteString("\n")
		sb.WriteString(v.HelpStyle.Render(fmt.Sprintf("  [%d/%d]", v.Selected+1, len(v.Tracks))))
	}

	sb.WriteString("\n\n")
	sb.WriteString(v.HelpStyle.Render("[Enter] Jump  [x] Remove  [u/d] Move  [C] Clear  [↑↓] Navigate"))
	return renderFixedPanel(sb.String(), v.Width, v.Height, v.BorderStyle)
}

func (v QueueView) visibleHeight() int {
	h := v.Height - 12
	if h < 1 {
		return 1
	}
	return h
}

func (v *QueueView) ensureVisible() {
	visible := v.visibleHeight()
	if v.Selected < v.Offset {
		v.Offset = v.Selected
	} else if v.Selected >= v.Offset+visible {
		v.Offset = v.Selected - visible + 1
	}
}

func truncateForQueue(s string, maxLen int) string {
	if s == "" {
		s = "Unknown"
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
