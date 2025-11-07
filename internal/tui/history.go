package tui

import (
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// HistoryViewModel shows operation history
type HistoryViewModel struct {
	config     *config.Config
	logger     logger.Logger
	parent     tea.Model
	history    []HistoryEntry
	cursor     int
	viewOffset int // For scrolling large lists
}

type HistoryEntry struct {
	Type      string
	Database  string
	Timestamp time.Time
	Status    string
	Filename  string
}

func NewHistoryView(cfg *config.Config, log logger.Logger, parent tea.Model) HistoryViewModel {
	history := loadHistory(cfg)
	// Start at the last item (most recent backup at bottom)
	lastIndex := len(history) - 1
	if lastIndex < 0 {
		lastIndex = 0
	}
	
	// Calculate initial viewport to show the last item
	maxVisible := 15
	viewOffset := lastIndex - maxVisible + 1
	if viewOffset < 0 {
		viewOffset = 0
	}
	
	return HistoryViewModel{
		config:     cfg,
		logger:     log,
		parent:     parent,
		history:    history,
		cursor:     lastIndex, // Start at most recent backup
		viewOffset: viewOffset,
	}
}

func loadHistory(cfg *config.Config) []HistoryEntry {
	var entries []HistoryEntry

	// Read backup files from backup directory
	files, err := ioutil.ReadDir(cfg.BackupDir)
	if err != nil {
		return entries
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		name := file.Name()
		if strings.HasSuffix(name, ".info") {
			continue
		}

		var backupType string
		var database string

		if strings.Contains(name, "cluster") {
			backupType = "Cluster Backup"
			database = "All Databases"
		} else if strings.Contains(name, "sample") {
			backupType = "Sample Backup"
			parts := strings.Split(name, "_")
			if len(parts) > 2 {
				database = parts[2]
			}
		} else {
			backupType = "Single Backup"
			parts := strings.Split(name, "_")
			if len(parts) > 2 {
				database = parts[2]
			}
		}

		entries = append(entries, HistoryEntry{
			Type:      backupType,
			Database:  database,
			Timestamp: file.ModTime(),
			Status:    "✅ Completed",
			Filename:  name,
		})
	}

	return entries
}

func (m HistoryViewModel) Init() tea.Cmd {
	return nil
}

func (m HistoryViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	maxVisible := 15 // Show max 15 items at once
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m.parent, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				// Scroll viewport up if cursor moves above visible area
				if m.cursor < m.viewOffset {
					m.viewOffset = m.cursor
				}
			}

		case "down", "j":
			if m.cursor < len(m.history)-1 {
				m.cursor++
				// Scroll viewport down if cursor moves below visible area
				if m.cursor >= m.viewOffset+maxVisible {
					m.viewOffset = m.cursor - maxVisible + 1
				}
			}
			
		case "pgup":
			// Page up - jump by maxVisible items
			m.cursor -= maxVisible
			if m.cursor < 0 {
				m.cursor = 0
			}
			// Adjust viewport
			if m.cursor < m.viewOffset {
				m.viewOffset = m.cursor
			}
			
		case "pgdown":
			// Page down - jump by maxVisible items
			m.cursor += maxVisible
			if m.cursor >= len(m.history) {
				m.cursor = len(m.history) - 1
			}
			// Adjust viewport
			if m.cursor >= m.viewOffset+maxVisible {
				m.viewOffset = m.cursor - maxVisible + 1
			}
			
		case "home", "g":
			// Jump to first item
			m.cursor = 0
			m.viewOffset = 0
			
		case "end", "G":
			// Jump to last item
			m.cursor = len(m.history) - 1
			m.viewOffset = m.cursor - maxVisible + 1
			if m.viewOffset < 0 {
				m.viewOffset = 0
			}
		}
	}

	return m, nil
}

func (m HistoryViewModel) View() string {
	var s strings.Builder

	header := titleStyle.Render("📜 Operation History")
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	if len(m.history) == 0 {
		s.WriteString(infoStyle.Render("📭 No backup history found"))
		s.WriteString("\n\n")
	} else {
		maxVisible := 15 // Show max 15 items at once
		
		// Calculate visible range
		start := m.viewOffset
		end := start + maxVisible
		if end > len(m.history) {
			end = len(m.history)
		}
		
		s.WriteString(fmt.Sprintf("Found %d backup operations (Viewing %d/%d):\n\n", 
			len(m.history), m.cursor+1, len(m.history)))

		// Show scroll indicators
		if start > 0 {
			s.WriteString(infoStyle.Render("  ▲ More entries above...\n"))
		}

		// Display only visible entries
		for i := start; i < end; i++ {
			entry := m.history[i]
			line := fmt.Sprintf("[%s] %s - %s (%s)",
				entry.Timestamp.Format("2006-01-02 15:04"),
				entry.Type,
				entry.Database,
				entry.Status)

			if m.cursor == i {
				// Highlighted selection
				s.WriteString(selectedStyle.Render("→ " + line) + "\n")
			} else {
				s.WriteString("  " + line + "\n")
			}
		}
		
		// Show scroll indicator if more entries below
		if end < len(m.history) {
			s.WriteString(infoStyle.Render(fmt.Sprintf("  ▼ %d more entries below...\n", len(m.history)-end)))
		}
		
		s.WriteString("\n")
	}

	s.WriteString("⌨️  ↑/↓: Navigate • PgUp/PgDn: Jump • Home/End: First/Last • ESC: Back • q: Quit\n")

	return s.String()
}
