package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"dbbackup/internal/restore"
)

var (
	archiveHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("240"))

	archiveSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Bold(true)

	archiveNormalStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250"))

	archiveValidStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2"))

	archiveInvalidStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1"))

	archiveOldStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3"))
)

// ArchiveInfo holds information about a backup archive
type ArchiveInfo struct {
	Name         string
	Path         string
	Format       restore.ArchiveFormat
	Size         int64
	Modified     time.Time
	DatabaseName string
	Valid        bool
	ValidationMsg string
}

// ArchiveBrowserModel for browsing and selecting backup archives
type ArchiveBrowserModel struct {
	config     *config.Config
	logger     logger.Logger
	parent     tea.Model
	archives   []ArchiveInfo
	cursor     int
	loading    bool
	err        error
	mode       string // "restore-single", "restore-cluster", "manage"
	filterType string // "all", "postgres", "mysql", "cluster"
	message    string
}

// NewArchiveBrowser creates a new archive browser
func NewArchiveBrowser(cfg *config.Config, log logger.Logger, parent tea.Model, mode string) ArchiveBrowserModel {
	return ArchiveBrowserModel{
		config:     cfg,
		logger:     log,
		parent:     parent,
		loading:    true,
		mode:       mode,
		filterType: "all",
	}
}

func (m ArchiveBrowserModel) Init() tea.Cmd {
	return loadArchives(m.config, m.logger)
}

type archiveListMsg struct {
	archives []ArchiveInfo
	err      error
}

func loadArchives(cfg *config.Config, log logger.Logger) tea.Cmd {
	return func() tea.Msg {
		backupDir := cfg.BackupDir

		// Check if backup directory exists
		if _, err := os.Stat(backupDir); err != nil {
			return archiveListMsg{archives: nil, err: fmt.Errorf("backup directory not found: %s", backupDir)}
		}

		// List all files
		files, err := os.ReadDir(backupDir)
		if err != nil {
			return archiveListMsg{archives: nil, err: fmt.Errorf("cannot read backup directory: %w", err)}
		}

		var archives []ArchiveInfo

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()
			format := restore.DetectArchiveFormat(name)

			if format == restore.FormatUnknown {
				continue // Skip non-backup files
			}

			info, _ := file.Info()
			fullPath := filepath.Join(backupDir, name)

			// Extract database name
			dbName := extractDBNameFromFilename(name)

			// Basic validation (just check if file is readable)
			valid := true
			validationMsg := "Valid"
			if info.Size() == 0 {
				valid = false
				validationMsg = "Empty file"
			}

			archives = append(archives, ArchiveInfo{
				Name:         name,
				Path:         fullPath,
				Format:       format,
				Size:         info.Size(),
				Modified:     info.ModTime(),
				DatabaseName: dbName,
				Valid:        valid,
				ValidationMsg: validationMsg,
			})
		}

		// Sort by modification time (newest first)
		sort.Slice(archives, func(i, j int) bool {
			return archives[i].Modified.After(archives[j].Modified)
		})

		return archiveListMsg{archives: archives, err: nil}
	}
}

func (m ArchiveBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case archiveListMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.archives = m.filterArchives(msg.archives)
		if len(m.archives) == 0 {
			m.message = "No backup archives found"
		}
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

		case "f":
			// Toggle filter
			filters := []string{"all", "postgres", "mysql", "cluster"}
			for i, f := range filters {
				if f == m.filterType {
					m.filterType = filters[(i+1)%len(filters)]
					break
				}
			}
			m.cursor = 0
			return m, loadArchives(m.config, m.logger)

		case "enter", " ":
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				
				// Validate selection based on mode
				if m.mode == "restore-cluster" && !selected.Format.IsClusterBackup() {
					m.message = errorStyle.Render("❌ Please select a cluster backup (.tar.gz)")
					return m, nil
				}
				
				if m.mode == "restore-single" && selected.Format.IsClusterBackup() {
					m.message = errorStyle.Render("❌ Please select a single database backup")
					return m, nil
				}

				// Open restore preview
				preview := NewRestorePreview(m.config, m.logger, m.parent, selected, m.mode)
				return preview, preview.Init()
			}

		case "i":
			// Show detailed info
			if len(m.archives) > 0 && m.cursor < len(m.archives) {
				selected := m.archives[m.cursor]
				m.message = fmt.Sprintf("📦 %s | Format: %s | Size: %s | Modified: %s",
					selected.Name,
					selected.Format.String(),
					formatSize(selected.Size),
					selected.Modified.Format("2006-01-02 15:04:05"))
			}
		}
	}

	return m, nil
}

func (m ArchiveBrowserModel) View() string {
	var s strings.Builder

	// Header
	title := "📦 Backup Archives"
	if m.mode == "restore-single" {
		title = "📦 Select Archive to Restore (Single Database)"
	} else if m.mode == "restore-cluster" {
		title = "📦 Select Archive to Restore (Cluster)"
	}
	
	s.WriteString(titleStyle.Render(title))
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

	// Filter info
	filterLabel := "Filter: " + m.filterType
	s.WriteString(infoStyle.Render(filterLabel))
	s.WriteString(infoStyle.Render("  (Press 'f' to change filter)"))
	s.WriteString("\n\n")

	// Archives list
	if len(m.archives) == 0 {
		s.WriteString(infoStyle.Render(m.message))
		s.WriteString("\n\n")
		s.WriteString(infoStyle.Render("Press Esc to go back"))
		return s.String()
	}

	// Column headers
	s.WriteString(archiveHeaderStyle.Render(fmt.Sprintf("%-40s %-25s %-12s %-20s",
		"FILENAME", "FORMAT", "SIZE", "MODIFIED")))
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", 100))
	s.WriteString("\n")

	// Show archives (limit to visible area)
	start := m.cursor - 5
	if start < 0 {
		start = 0
	}
	end := start + 10
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

		// Color code based on validity and age
		statusIcon := "✓"
		if !archive.Valid {
			statusIcon = "✗"
			style = archiveInvalidStyle
		} else if time.Since(archive.Modified) > 30*24*time.Hour {
			style = archiveOldStyle
			statusIcon = "⚠"
		}

		filename := truncate(archive.Name, 38)
		format := truncate(archive.Format.String(), 23)

		line := fmt.Sprintf("%s %s %-38s %-23s %-10s %-19s",
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
		s.WriteString(m.message)
		s.WriteString("\n")
	}

	s.WriteString(infoStyle.Render(fmt.Sprintf("Total: %d archive(s) | Selected: %d/%d",
		len(m.archives), m.cursor+1, len(m.archives))))
	s.WriteString("\n")
	s.WriteString(infoStyle.Render("⌨️  ↑/↓: Navigate | Enter: Select | f: Filter | i: Info | Esc: Back"))

	return s.String()
}

// filterArchives filters archives based on current filter setting
func (m ArchiveBrowserModel) filterArchives(archives []ArchiveInfo) []ArchiveInfo {
	if m.filterType == "all" {
		return archives
	}

	var filtered []ArchiveInfo
	for _, archive := range archives {
		switch m.filterType {
		case "postgres":
			if archive.Format.IsPostgreSQL() && !archive.Format.IsClusterBackup() {
				filtered = append(filtered, archive)
			}
		case "mysql":
			if archive.Format.IsMySQL() {
				filtered = append(filtered, archive)
			}
		case "cluster":
			if archive.Format.IsClusterBackup() {
				filtered = append(filtered, archive)
			}
		}
	}
	return filtered
}

// extractDBNameFromFilename extracts database name from archive filename
func extractDBNameFromFilename(filename string) string {
	base := filepath.Base(filename)

	// Remove extensions
	base = strings.TrimSuffix(base, ".tar.gz")
	base = strings.TrimSuffix(base, ".dump.gz")
	base = strings.TrimSuffix(base, ".sql.gz")
	base = strings.TrimSuffix(base, ".dump")
	base = strings.TrimSuffix(base, ".sql")

	// Remove timestamp patterns (YYYYMMDD_HHMMSS)
	parts := strings.Split(base, "_")
	for i := len(parts) - 1; i >= 0; i-- {
		// Check if part looks like a date or time
		if len(parts[i]) == 8 || len(parts[i]) == 6 {
			parts = parts[:i]
		} else {
			break
		}
	}

	if len(parts) > 0 {
		return parts[0]
	}

	return base
}

// formatSize formats file size
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncate truncates string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
