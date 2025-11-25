package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirectoryBrowser is an integrated directory browser for the settings
type DirectoryBrowser struct {
	CurrentPath string
	items       []string
	cursor      int
	visible     bool
}

func NewDirectoryBrowser(startPath string) *DirectoryBrowser {
	db := &DirectoryBrowser{
		CurrentPath: startPath,
		visible:     false,
	}
	db.LoadItems()
	return db
}

func (db *DirectoryBrowser) LoadItems() {
	db.items = []string{}
	db.cursor = 0

	// Add parent directory if not at root
	if db.CurrentPath != "/" && db.CurrentPath != "" {
		db.items = append(db.items, "..")
	}

	// Read current directory
	entries, err := os.ReadDir(db.CurrentPath)
	if err != nil {
		db.items = append(db.items, "[Error reading directory]")
		return
	}

	// Collect directories only
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	// Sort directories
	sort.Strings(dirs)
	db.items = append(db.items, dirs...)
}

func (db *DirectoryBrowser) Show() {
	db.visible = true
}

func (db *DirectoryBrowser) Hide() {
	db.visible = false
}

func (db *DirectoryBrowser) IsVisible() bool {
	return db.visible
}

func (db *DirectoryBrowser) GetCurrentPath() string {
	return db.CurrentPath
}

func (db *DirectoryBrowser) Navigate(direction int) {
	if direction < 0 && db.cursor > 0 {
		db.cursor--
	} else if direction > 0 && db.cursor < len(db.items)-1 {
		db.cursor++
	}
}

func (db *DirectoryBrowser) Enter() bool {
	if len(db.items) == 0 || db.cursor >= len(db.items) {
		return false
	}

	selected := db.items[db.cursor]
	if selected == ".." {
		db.CurrentPath = filepath.Dir(db.CurrentPath)
		db.LoadItems()
		return false
	} else if selected == "[Error reading directory]" {
		return false
	} else {
		// Navigate into directory
		newPath := filepath.Join(db.CurrentPath, selected)
		if stat, err := os.Stat(newPath); err == nil && stat.IsDir() {
			db.CurrentPath = newPath
			db.LoadItems()
		}
		return false
	}
}

func (db *DirectoryBrowser) Select() string {
	return db.CurrentPath
}

func (db *DirectoryBrowser) Render() string {
	if !db.visible {
		return ""
	}

	var lines []string
	
	// Header
	lines = append(lines, fmt.Sprintf("    Current: %s", db.CurrentPath))
	lines = append(lines, fmt.Sprintf("    Found %d directories (cursor: %d)", len(db.items), db.cursor))
	lines = append(lines, "    Directories:")

	// Show directories
	maxItems := 5 // Show max 5 items to keep it compact
	start := 0
	end := len(db.items)
	
	if len(db.items) > maxItems {
		// Center the cursor in the view
		start = db.cursor - maxItems/2
		if start < 0 {
			start = 0
		}
		end = start + maxItems
		if end > len(db.items) {
			end = len(db.items)
			start = end - maxItems
			if start < 0 {
				start = 0
			}
		}
	}

	for i := start; i < end; i++ {
		item := db.items[i]
		prefix := "      "
		if i == db.cursor {
			prefix = "   >> "
		}
		
		displayName := item
		if item == ".." {
			displayName = "../ (parent directory)"
		} else if item != "[Error reading directory]" {
			displayName = item + "/"
		}
		
		lines = append(lines, prefix+displayName)
	}

	// Show navigation info if there are more items
	if len(db.items) > maxItems {
		lines = append(lines, fmt.Sprintf("    (%d of %d directories)", db.cursor+1, len(db.items)))
	}

	lines = append(lines, "")
	lines = append(lines, "    ↑/↓: Navigate | Enter/→: Open | ←: Parent | Space: Select | Esc: Cancel")

	return strings.Join(lines, "\n")
}