package components

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileEntry represents a file or directory in the browser
type FileEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// FileBrowser is a component for navigating the filesystem
type FileBrowser struct {
	Width       int
	Height      int
	CurrentPath string
	Entries     []FileEntry
	Selected    int
	Offset      int
	Extensions  []string // Supported file extensions
	Err         error

	// Styles
	DirStyle      lipgloss.Style
	FileStyle     lipgloss.Style
	SelectedStyle lipgloss.Style
	PathStyle     lipgloss.Style
	BorderStyle   lipgloss.Style
}

// NewFileBrowser creates a new file browser starting at the given path
func NewFileBrowser(startPath string, width, height int) FileBrowser {
	fb := FileBrowser{
		Width:      width,
		Height:     height,
		Extensions: []string{".mp3", ".wav", ".flac"},
		DirStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Bold(true),
		FileStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")),
		SelectedStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("255")).
			Bold(true),
		PathStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true),
		BorderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2),
	}

	// If startPath is empty, use home directory
	if startPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			startPath = "/"
		} else {
			startPath = home
		}
	}

	fb.Navigate(startPath)
	return fb
}

// Navigate changes to the specified directory
func (fb *FileBrowser) Navigate(path string) {
	fb.CurrentPath = path
	fb.Selected = 0
	fb.Offset = 0
	fb.Err = nil

	entries, err := os.ReadDir(path)
	if err != nil {
		fb.Err = err
		fb.Entries = nil
		return
	}

	fb.Entries = make([]FileEntry, 0)

	// Add parent directory entry (unless at root)
	if path != "/" {
		fb.Entries = append(fb.Entries, FileEntry{
			Name:  "..",
			Path:  filepath.Dir(path),
			IsDir: true,
		})
	}

	// Separate dirs and files
	var dirs, files []FileEntry

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())

		if entry.IsDir() {
			dirs = append(dirs, FileEntry{
				Name:  entry.Name(),
				Path:  fullPath,
				IsDir: true,
			})
		} else {
			// Only show supported audio files
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			for _, supportedExt := range fb.Extensions {
				if ext == supportedExt {
					files = append(files, FileEntry{
						Name:  entry.Name(),
						Path:  fullPath,
						IsDir: false,
					})
					break
				}
			}
		}
	}

	// Sort directories and files alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Add directories first, then files
	fb.Entries = append(fb.Entries, dirs...)
	fb.Entries = append(fb.Entries, files...)
}

// Update handles input messages
func (fb FileBrowser) Update(msg tea.Msg) (FileBrowser, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if fb.Selected > 0 {
				fb.Selected--
				fb.ensureVisible()
			}
		case "down", "j":
			if fb.Selected < len(fb.Entries)-1 {
				fb.Selected++
				fb.ensureVisible()
			}
		case "pgup":
			fb.Selected -= fb.visibleHeight()
			if fb.Selected < 0 {
				fb.Selected = 0
			}
			fb.ensureVisible()
		case "pgdown":
			fb.Selected += fb.visibleHeight()
			if fb.Selected >= len(fb.Entries) {
				fb.Selected = len(fb.Entries) - 1
			}
			fb.ensureVisible()
		case "home":
			fb.Selected = 0
			fb.ensureVisible()
		case "end":
			fb.Selected = len(fb.Entries) - 1
			fb.ensureVisible()
		case "backspace":
			// Go to parent directory
			if fb.CurrentPath != "/" {
				fb.Navigate(filepath.Dir(fb.CurrentPath))
			}
		case "~":
			// Go to home directory
			if home, err := os.UserHomeDir(); err == nil {
				fb.Navigate(home)
			}
		}
	}
	return fb, nil
}

// SelectedEntry returns the currently selected entry, or nil if none
func (fb *FileBrowser) SelectedEntry() *FileEntry {
	if fb.Selected >= 0 && fb.Selected < len(fb.Entries) {
		return &fb.Entries[fb.Selected]
	}
	return nil
}

// EnterSelected handles Enter on the selected entry
// Returns the file path if a file was selected, empty string if navigated to dir
func (fb *FileBrowser) EnterSelected() string {
	entry := fb.SelectedEntry()
	if entry == nil {
		return ""
	}

	if entry.IsDir {
		fb.Navigate(entry.Path)
		return ""
	}

	// It's a file, return the path
	return entry.Path
}

// visibleHeight returns the number of visible items
func (fb *FileBrowser) visibleHeight() int {
	h := fb.Height - 6 // Account for border, path, help
	if h < 1 {
		return 1
	}
	return h
}

// ensureVisible ensures the selected item is visible
func (fb *FileBrowser) ensureVisible() {
	visible := fb.visibleHeight()
	if fb.Selected < fb.Offset {
		fb.Offset = fb.Selected
	} else if fb.Selected >= fb.Offset+visible {
		fb.Offset = fb.Selected - visible + 1
	}
}

// View renders the file browser
func (fb FileBrowser) View() string {
	var sb strings.Builder

	// Current path
	sb.WriteString(fb.PathStyle.Render("📁 " + fb.CurrentPath))
	sb.WriteString("\n\n")

	// Error display
	if fb.Err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		sb.WriteString(errorStyle.Render("Error: " + fb.Err.Error()))
		sb.WriteString("\n")
	}

	// File list
	visible := fb.visibleHeight()
	end := fb.Offset + visible
	if end > len(fb.Entries) {
		end = len(fb.Entries)
	}

	for i := fb.Offset; i < end; i++ {
		entry := fb.Entries[i]

		var line string
		if entry.IsDir {
			line = "📂 " + entry.Name
		} else {
			line = "🎵 " + entry.Name
		}

		// Truncate if too long
		maxWidth := fb.Width - 10
		if len(line) > maxWidth {
			line = line[:maxWidth-3] + "..."
		}

		if i == fb.Selected {
			sb.WriteString(fb.SelectedStyle.Render(line))
		} else if entry.IsDir {
			sb.WriteString(fb.DirStyle.Render(line))
		} else {
			sb.WriteString(fb.FileStyle.Render(line))
		}
		sb.WriteString("\n")
	}

	// Padding if not enough entries
	for i := end - fb.Offset; i < visible; i++ {
		sb.WriteString("\n")
	}

	// Count info
	fileCount := 0
	for _, e := range fb.Entries {
		if !e.IsDir {
			fileCount++
		}
	}
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(countStyle.Render(
		strings.Repeat("─", 20) + "\n" +
			"Files: " + string(rune('0'+fileCount/100%10)) + string(rune('0'+fileCount/10%10)) + string(rune('0'+fileCount%10))))

	// Help text
	sb.WriteString("\n\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(helpStyle.Render("[Enter] Open/Add  [Backspace] Up  [~] Home  [Esc] Cancel"))

	return fb.BorderStyle.Width(fb.Width - 4).Render(sb.String())
}
