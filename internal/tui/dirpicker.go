package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DirectoryPicker is a simple, fast directory and file picker
type DirectoryPicker struct {
	currentPath  string
	items        []FileItem
	cursor       int
	callback     func(string)
	allowFiles   bool // Allow file selection for restore operations
	styles       DirectoryPickerStyles
}

type FileItem struct {
	Name  string
	IsDir bool
	Path  string
}

type DirectoryPickerStyles struct {
	Container lipgloss.Style
	Header    lipgloss.Style
	Item      lipgloss.Style
	Selected  lipgloss.Style
	Help      lipgloss.Style
}

func DefaultDirectoryPickerStyles() DirectoryPickerStyles {
	return DirectoryPickerStyles{
		Container: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true).
			MarginBottom(1),
		Item: lipgloss.NewStyle().
			PaddingLeft(2),
		Selected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("240")).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			MarginTop(1),
	}
}

func NewDirectoryPicker(startPath string, allowFiles bool, callback func(string)) *DirectoryPicker {
	dp := &DirectoryPicker{
		currentPath: startPath,
		allowFiles:  allowFiles,
		callback:    callback,
		styles:      DefaultDirectoryPickerStyles(),
	}
	dp.loadItems()
	return dp
}

func (dp *DirectoryPicker) loadItems() {
	dp.items = []FileItem{}
	dp.cursor = 0

	// Add parent directory option if not at root
	if dp.currentPath != "/" && dp.currentPath != "" {
		dp.items = append(dp.items, FileItem{
			Name:  "..",
			IsDir: true,
			Path:  filepath.Dir(dp.currentPath),
		})
	}

	// Read current directory
	entries, err := os.ReadDir(dp.currentPath)
	if err != nil {
		dp.items = append(dp.items, FileItem{
			Name:  "Error reading directory",
			IsDir: false,
			Path:  "",
		})
		return
	}

	// Collect directories and optionally files
	var dirs []FileItem
	var files []FileItem
	
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue // Skip hidden files
		}
		
		item := FileItem{
			Name:  entry.Name(),
			IsDir: entry.IsDir(),
			Path:  filepath.Join(dp.currentPath, entry.Name()),
		}
		
		if entry.IsDir() {
			dirs = append(dirs, item)
		} else if dp.allowFiles {
			// Only include backup-related files
			if strings.HasSuffix(entry.Name(), ".sql") || 
			   strings.HasSuffix(entry.Name(), ".dump") ||
			   strings.HasSuffix(entry.Name(), ".gz") ||
			   strings.HasSuffix(entry.Name(), ".tar") {
				files = append(files, item)
			}
		}
	}

	// Sort directories and files separately
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	// Add directories first, then files
	dp.items = append(dp.items, dirs...)
	dp.items = append(dp.items, files...)
}

func (dp *DirectoryPicker) Init() tea.Cmd {
	return nil
}

func (dp *DirectoryPicker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
			if dp.callback != nil {
				dp.callback("") // Empty string indicates cancel
			}
			return dp, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if len(dp.items) == 0 {
				return dp, nil
			}

			selected := dp.items[dp.cursor]
			if selected.Name == ".." {
				// Go to parent directory
				dp.currentPath = filepath.Dir(dp.currentPath)
				dp.loadItems()
			} else if selected.Name == "Error reading directory" {
				return dp, nil
			} else if selected.IsDir {
				// Navigate into directory
				dp.currentPath = selected.Path
				dp.loadItems()
			} else {
				// File selected (for restore operations)
				if dp.callback != nil {
					dp.callback(selected.Path)
				}
				return dp, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			// Select current directory
			if dp.callback != nil {
				dp.callback(dp.currentPath)
			}
			return dp, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if dp.cursor > 0 {
				dp.cursor--
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if dp.cursor < len(dp.items)-1 {
				dp.cursor++
			}
		}
	}

	return dp, nil
}

func (dp *DirectoryPicker) View() string {
	if len(dp.items) == 0 {
		return dp.styles.Container.Render("No items found")
	}

	var content strings.Builder

	// Header with current path
	pickerType := "Directory"
	if dp.allowFiles {
		pickerType = "File/Directory"
	}
	header := fmt.Sprintf("📁 %s Picker - %s", pickerType, dp.currentPath)
	content.WriteString(dp.styles.Header.Render(header))
	content.WriteString("\n\n")

	// Items list
	for i, item := range dp.items {
		var prefix string
		if item.Name == ".." {
			prefix = "⬆️ "
		} else if item.Name == "Error reading directory" {
			prefix = "❌ "
		} else if item.IsDir {
			prefix = "📁 "
		} else {
			prefix = "📄 "
		}

		line := prefix + item.Name
		if i == dp.cursor {
			content.WriteString(dp.styles.Selected.Render(line))
		} else {
			content.WriteString(dp.styles.Item.Render(line))
		}
		content.WriteString("\n")
	}

	// Help text
	help := "\n↑/↓: Navigate • Enter: Open/Select File • s: Select Directory • q/Esc: Cancel"
	if !dp.allowFiles {
		help = "\n↑/↓: Navigate • Enter: Open • s: Select Directory • q/Esc: Cancel"
	}
	content.WriteString(dp.styles.Help.Render(help))

	return dp.styles.Container.Render(content.String())
}