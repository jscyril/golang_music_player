package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressBar represents a progress bar component
type ProgressBar struct {
	Width       int
	Current     time.Duration
	Total       time.Duration
	BarChar     string
	EmptyChar   string
	ShowTime    bool
	Style       lipgloss.Style
	FilledStyle lipgloss.Style
	EmptyStyle  lipgloss.Style
}

// NewProgressBar creates a new progress bar
func NewProgressBar(width int) ProgressBar {
	return ProgressBar{
		Width:       width,
		BarChar:     "█",
		EmptyChar:   "░",
		ShowTime:    true,
		Style:       lipgloss.NewStyle(),
		FilledStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
		EmptyStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

// Update handles messages for the progress bar
func (p ProgressBar) Update(msg tea.Msg) (ProgressBar, tea.Cmd) {
	return p, nil
}

// SetProgress sets the current position
func (p *ProgressBar) SetProgress(current, total time.Duration) {
	p.Current = current
	p.Total = total
}

// View renders the progress bar
func (p ProgressBar) View() string {
	var sb strings.Builder

	// Calculate progress percentage
	var percent float64
	if p.Total > 0 {
		percent = float64(p.Current) / float64(p.Total)
	}
	if percent > 1 {
		percent = 1
	}

	// Calculate bar segments
	barWidth := p.Width - 14 // Leave room for time display
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * percent)
	empty := barWidth - filled

	// Build progress bar
	filledBar := p.FilledStyle.Render(strings.Repeat(p.BarChar, filled))
	emptyBar := p.EmptyStyle.Render(strings.Repeat(p.EmptyChar, empty))

	sb.WriteString(filledBar)
	sb.WriteString(emptyBar)

	// Add time display
	if p.ShowTime {
		sb.WriteString(" ")
		sb.WriteString(formatDuration(p.Current))
		sb.WriteString("/")
		sb.WriteString(formatDuration(p.Total))
	}

	return p.Style.Render(sb.String())
}

// formatDuration formats a duration as MM:SS
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	return fmt.Sprintf("%02d:%02d", m, s)
}
