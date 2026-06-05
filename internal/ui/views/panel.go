package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func renderFixedPanel(content string, width, height int, style lipgloss.Style) string {
	if width < 8 {
		width = 8
	}
	if height < 4 {
		height = 4
	}

	innerHeight := height - 4 // border and vertical padding from the shared styles
	if innerHeight < 1 {
		innerHeight = 1
	}

	lines := strings.Split(content, "\n")
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	for len(lines) < innerHeight {
		lines = append(lines, "")
	}

	return style.Width(width - 4).Render(strings.Join(lines, "\n"))
}
