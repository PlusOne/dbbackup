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
	config  *config.Config
	logger  logger.Logger
	parent  tea.Model
	history []HistoryEntry
	cursor  int
}

type HistoryEntry struct {
	Type      string
	Database  string
	Timestamp time.Time
	Status    string
	Filename  string
}

func NewHistoryView(cfg *config.Config, log logger.Logger, parent tea.Model) HistoryViewModel {
	return HistoryViewModel{
		config:  cfg,
		logger:  log,
		parent:  parent,
		history: loadHistory(cfg),
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
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m.parent, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.history)-1 {
				m.cursor++
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
		s.WriteString(fmt.Sprintf("Found %d backup operations:\n\n", len(m.history)))

		for i, entry := range m.history {
			cursor := " "
			line := fmt.Sprintf("%s [%s] %s - %s (%s)",
				cursor,
				entry.Timestamp.Format("2006-01-02 15:04"),
				entry.Type,
				entry.Database,
				entry.Status)

			if m.cursor == i {
				s.WriteString(selectedStyle.Render("> " + line))
			} else {
				s.WriteString("  " + line)
			}
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	s.WriteString("⌨️  ↑/↓: Navigate • ESC: Back • q: Quit\n")

	return s.String()
}
