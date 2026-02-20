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
	HeadStyle   lipgloss.Style

	// Layout info for click-to-seek (set during View)
	barWidth  int
	timeWidth int
}

// NewProgressBar creates a new progress bar
func NewProgressBar(width int) ProgressBar {
	return ProgressBar{
		Width:       width,
		BarChar:     "━",
		EmptyChar:   "─",
		ShowTime:    true,
		Style:       lipgloss.NewStyle(),
		FilledStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("212")),
		EmptyStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		HeadStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true),
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

// BarWidth returns the computed bar width (available after View is called)
func (p ProgressBar) BarWidth() int {
	return p.barWidth
}

// HandleClick converts a click X position (relative to the start of the bar)
// into a seek position. barOffsetX is the X offset of the bar within the
// parent container (e.g. border padding). Returns the target duration.
func (p ProgressBar) HandleClick(clickX, barOffsetX int) time.Duration {
	relX := clickX - barOffsetX
	if relX < 0 {
		relX = 0
	}
	if p.barWidth <= 0 || p.Total <= 0 {
		return 0
	}
	if relX > p.barWidth {
		relX = p.barWidth
	}
	percent := float64(relX) / float64(p.barWidth)
	return time.Duration(float64(p.Total) * percent)
}

// View renders the progress bar
func (p *ProgressBar) View() string {
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
	// Time display takes "MM:SS/MM:SS " = 12 chars + 2 spaces = 14
	p.timeWidth = 14
	p.barWidth = p.Width - p.timeWidth
	if p.barWidth < 10 {
		p.barWidth = 10
	}

	headPos := int(float64(p.barWidth) * percent)
	if headPos >= p.barWidth {
		headPos = p.barWidth - 1
	}

	filled := headPos
	empty := p.barWidth - headPos - 1

	// Build progress bar with seek head
	filledBar := p.FilledStyle.Render(strings.Repeat(p.BarChar, filled))
	head := p.HeadStyle.Render("●")
	emptyBar := p.EmptyStyle.Render(strings.Repeat(p.EmptyChar, empty))

	sb.WriteString(filledBar)
	sb.WriteString(head)
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
