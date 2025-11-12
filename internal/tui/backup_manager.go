package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// BackupManagerModel manages backup archives
type BackupManagerModel struct {
	config    *config.Config
	logger    logger.Logger
	parent    tea.Model
	archives  []ArchiveInfo
	cursor    int
	loading   bool
	err       error
	message   string
	totalSize int64
	freeSpace int64
}

// NewBackupManager creates a new backup manager
func NewBackupManager(cfg *config.Config, log logger.Logger, parent tea.Model) BackupManagerModel {
	return BackupManagerModel{
		config:  cfg,
		logger:  log,
		parent:  parent,
		loading: true,
	}
}

func (m BackupManagerModel) Init() tea.Cmd {
	return loadArchives(m.config, m.logger)
}

func (m BackupManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case archiveListMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.archives = msg.archives
		
		// Calculate total size
		m.totalSize = 0
		for _, archive := range m.archives {
			m.totalSize += archive.Size
		}
		
		// Get free space (simplified - just show message)
		m.message = fmt.Sprintf("Loaded %d archive(s)", len(m.archives))
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m.parent, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.archives)-1 {
				m.cursor++
			}

		case "v":
			// Verify archive
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				m.message = fmt.Sprintf("🔍 Verifying %s...", selected.Name)
				// In real implementation, would run verification
			}

		case "d":
			// Delete archive (with confirmation)
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				archivePath := selected.Path
				confirm := NewConfirmationModelWithAction(m.config, m.logger, m,
					"🗑️  Delete Archive",
					fmt.Sprintf("Delete archive '%s'? This cannot be undone.", selected.Name),
					func() (tea.Model, tea.Cmd) {
						// Delete the archive
						err := deleteArchive(archivePath)
						if err != nil {
							m.err = fmt.Errorf("failed to delete archive: %v", err)
							m.message = fmt.Sprintf("❌ Failed to delete: %v", err)
						} else {
							m.message = fmt.Sprintf("✅ Deleted: %s", selected.Name)
						}
						// Refresh the archive list
						m.loading = true
						return m, loadArchives(m.config, m.logger)
					})
				return confirm, nil
			}

		case "i":
			// Show info
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				m.message = fmt.Sprintf("📦 %s | %s | %s | Modified: %s",
					selected.Name,
					selected.Format.String(),
					formatSize(selected.Size),
					selected.Modified.Format("2006-01-02 15:04:05"))
			}

		case "r":
			// Restore selected archive
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				mode := "restore-single"
				if selected.Format.IsClusterBackup() {
					mode = "restore-cluster"
				}
				preview := NewRestorePreview(m.config, m.logger, m.parent, selected, mode)
				return preview, preview.Init()
			}

		case "R":
			// Refresh list
			m.loading = true
			m.message = "Refreshing..."
			return m, loadArchives(m.config, m.logger)
		}
	}

	return m, nil
}

func (m BackupManagerModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🗄️  Backup Archive Manager"))
	s.WriteString("\n\n")

	if m.loading {
		s.WriteString(infoStyle.Render("Loading archives..."))
		return s.String()
	}

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.err)))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Press Esc to go back"))
		return s.String()
	}

	// Summary
	s.WriteString(infoStyle.Render(fmt.Sprintf("Total Archives: %d  |  Total Size: %s",
		len(m.archives), formatSize(m.totalSize))))
	s.WriteString("\n\n")

	// Archives list
	if len(m.archives) == 0 {
		s.WriteString(infoStyle.Render("No backup archives found"))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Press Esc to go back"))
		return s.String()
	}

	// Column headers
	s.WriteString(archiveHeaderStyle.Render(fmt.Sprintf("%-35s %-25s %-12s %-20s",
		"FILENAME", "FORMAT", "SIZE", "MODIFIED")))
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", 95))
	s.WriteString("\n")

	// Show archives (limit to visible area)
	start := m.cursor - 5
	if start < 0 {
		start = 0
	}
	end := start + 12
	if end > len(m.archives) {
		end = len(m.archives)
	}

	for i := start; i < end; i++ {
		archive := m.archives[i]
		cursor := " "
		style := archiveNormalStyle

		if i == m.cursor {
			cursor = ">"
			style = archiveSelectedStyle
		}

		// Status icon
		statusIcon := "✓"
		if !archive.Valid {
			statusIcon = "✗"
			style = archiveInvalidStyle
		} else if time.Since(archive.Modified) > 30*24*time.Hour {
			statusIcon = "⚠"
		}

		filename := truncate(archive.Name, 33)
		format := truncate(archive.Format.String(), 23)

		line := fmt.Sprintf("%s %s %-33s %-23s %-10s %-19s",
			cursor,
			statusIcon,
			filename,
			format,
			formatSize(archive.Size),
			archive.Modified.Format("2006-01-02 15:04"))

		s.WriteString(style.Render(line))
		s.WriteString("\n")
	}

	// Footer
	s.WriteString("\n")
	if m.message != "" {
		s.WriteString(infoStyle.Render(m.message))
		s.WriteString("\n")
	}

	s.WriteString(infoStyle.Render(fmt.Sprintf("Selected: %d/%d", m.cursor+1, len(m.archives))))
	s.WriteString("\n")
	s.WriteString(infoStyle.Render("⌨️  ↑/↓: Navigate | r: Restore | v: Verify | d: Delete | i: Info | R: Refresh | Esc: Back"))

	return s.String()
}

// deleteArchive deletes a backup archive (to be called from confirmation)
func deleteArchive(archivePath string) error {
	return os.Remove(archivePath)
}
