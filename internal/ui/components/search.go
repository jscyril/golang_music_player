package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchInput represents a search input component
type SearchInput struct {
	Value       string
	Placeholder string
	Focused     bool
	Width       int
	CursorPos   int
	Style       lipgloss.Style
	FocusStyle  lipgloss.Style
	Prompt      string
}

// NewSearchInput creates a new search input
func NewSearchInput(width int) SearchInput {
	return SearchInput{
		Placeholder: "Search...",
		Width:       width,
		Prompt:      "🔍 ",
		Style: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1),
		FocusStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("212")).
			Padding(0, 1),
	}
}

// Focus sets focus on the input
func (s *SearchInput) Focus() {
	s.Focused = true
}

// Blur removes focus from the input
func (s *SearchInput) Blur() {
	s.Focused = false
}

// SetValue sets the input value
func (s *SearchInput) SetValue(value string) {
	s.Value = value
	s.CursorPos = len(value)
}

// Clear clears the input
func (s *SearchInput) Clear() {
	s.Value = ""
	s.CursorPos = 0
}

// Update handles messages for the search input
func (s SearchInput) Update(msg tea.Msg) (SearchInput, tea.Cmd) {
	if !s.Focused {
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyBackspace:
			if len(s.Value) > 0 && s.CursorPos > 0 {
				s.Value = s.Value[:s.CursorPos-1] + s.Value[s.CursorPos:]
				s.CursorPos--
			}
		case tea.KeyDelete:
			if s.CursorPos < len(s.Value) {
				s.Value = s.Value[:s.CursorPos] + s.Value[s.CursorPos+1:]
			}
		case tea.KeyLeft:
			if s.CursorPos > 0 {
				s.CursorPos--
			}
		case tea.KeyRight:
			if s.CursorPos < len(s.Value) {
				s.CursorPos++
			}
		case tea.KeyHome:
			s.CursorPos = 0
		case tea.KeyEnd:
			s.CursorPos = len(s.Value)
		case tea.KeyRunes:
			// Insert character at cursor position
			char := string(msg.Runes)
			s.Value = s.Value[:s.CursorPos] + char + s.Value[s.CursorPos:]
			s.CursorPos += len(char)
		}
	}

	return s, nil
}

// View renders the search input
func (s SearchInput) View() string {
	var content string

	if s.Value == "" && !s.Focused {
		content = s.Prompt + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(s.Placeholder)
	} else {
		// Show value with cursor
		if s.Focused {
			before := s.Value[:s.CursorPos]
			after := s.Value[s.CursorPos:]
			cursor := lipgloss.NewStyle().Background(lipgloss.Color("212")).Render(" ")
			content = s.Prompt + before + cursor + after
		} else {
			content = s.Prompt + s.Value
		}
	}

	// Truncate if too long
	maxWidth := s.Width - 4
	if len(content) > maxWidth {
		content = content[:maxWidth]
	}

	if s.Focused {
		return s.FocusStyle.Width(s.Width).Render(content)
	}
	return s.Style.Width(s.Width).Render(content)
}
