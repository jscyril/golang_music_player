package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jscyril/golang_music_player/api"
)

// TrackList represents a scrollable list of tracks
type TrackList struct {
	Items         []*api.Track
	Selected      int
	Height        int
	Width         int
	Offset        int
	Title         string
	ShowNumbers   bool
	SelectedStyle lipgloss.Style
	NormalStyle   lipgloss.Style
	TitleStyle    lipgloss.Style
}

// NewTrackList creates a new track list
func NewTrackList(height, width int) TrackList {
	return TrackList{
		Items:    make([]*api.Track, 0),
		Selected: 0,
		Height:   height,
		Width:    width,
		Offset:   0,
		SelectedStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Bold(true).
			Padding(0, 1),
		NormalStyle: lipgloss.NewStyle().
			Padding(0, 1),
		TitleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginBottom(1),
		ShowNumbers: true,
	}
}

// SetItems sets the list items
func (l *TrackList) SetItems(items []*api.Track) {
	l.Items = items
	l.Selected = 0
	l.Offset = 0
}

// Update handles messages for the track list
func (l TrackList) Update(msg tea.Msg) (TrackList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			l.MoveUp()
		case "down", "j":
			l.MoveDown()
		case "home":
			l.Selected = 0
			l.Offset = 0
		case "end":
			if len(l.Items) > 0 {
				l.Selected = len(l.Items) - 1
				l.ensureVisible()
			}
		case "pgup":
			l.PageUp()
		case "pgdown":
			l.PageDown()
		}
	}
	return l, nil
}

// MoveUp moves selection up
func (l *TrackList) MoveUp() {
	if l.Selected > 0 {
		l.Selected--
		l.ensureVisible()
	}
}

// MoveDown moves selection down
func (l *TrackList) MoveDown() {
	if l.Selected < len(l.Items)-1 {
		l.Selected++
		l.ensureVisible()
	}
}

// PageUp moves selection up by a page
func (l *TrackList) PageUp() {
	l.Selected -= l.Height - 2
	if l.Selected < 0 {
		l.Selected = 0
	}
	l.ensureVisible()
}

// PageDown moves selection down by a page
func (l *TrackList) PageDown() {
	l.Selected += l.Height - 2
	if l.Selected >= len(l.Items) {
		l.Selected = len(l.Items) - 1
	}
	l.ensureVisible()
}

// ensureVisible ensures the selected item is visible
func (l *TrackList) ensureVisible() {
	visibleHeight := l.Height - 2 // Account for title and border
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	if l.Selected < l.Offset {
		l.Offset = l.Selected
	} else if l.Selected >= l.Offset+visibleHeight {
		l.Offset = l.Selected - visibleHeight + 1
	}
}

// SelectedItem returns the currently selected track
func (l *TrackList) SelectedItem() *api.Track {
	if l.Selected >= 0 && l.Selected < len(l.Items) {
		return l.Items[l.Selected]
	}
	return nil
}

// View renders the track list
func (l TrackList) View() string {
	var sb strings.Builder

	// Title
	if l.Title != "" {
		sb.WriteString(l.TitleStyle.Render(l.Title))
		sb.WriteString("\n")
	}

	if len(l.Items) == 0 {
		sb.WriteString(l.NormalStyle.Render("No tracks"))
		return sb.String()
	}

	// Calculate visible range
	visibleHeight := l.Height - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	end := l.Offset + visibleHeight
	if end > len(l.Items) {
		end = len(l.Items)
	}

	// Render visible items
	for i := l.Offset; i < end; i++ {
		track := l.Items[i]
		var line string

		if l.ShowNumbers {
			line = fmt.Sprintf("%3d. %s - %s", i+1, truncate(track.Artist, 20), truncate(track.Title, 30))
		} else {
			line = fmt.Sprintf("%s - %s", truncate(track.Artist, 20), truncate(track.Title, 35))
		}

		// Truncate to width
		if len(line) > l.Width-2 {
			line = line[:l.Width-5] + "..."
		}

		if i == l.Selected {
			sb.WriteString(l.SelectedStyle.Render(line))
		} else {
			sb.WriteString(l.NormalStyle.Render(line))
		}

		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	// Scrollbar indicator
	if len(l.Items) > visibleHeight {
		sb.WriteString("\n")
		sb.WriteString(l.NormalStyle.Render(fmt.Sprintf("  [%d/%d]", l.Selected+1, len(l.Items))))
	}

	return sb.String()
}

// truncate truncates a string to the specified length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
