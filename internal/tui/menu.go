package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/cleanup"
	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// Style definitions
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("240")).
			Padding(0, 1)

	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	dbSelectorLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("6")).
				Bold(true)
)

type dbTypeOption struct {
	label string
	value string
}

// MenuModel represents the simple menu state
type MenuModel struct {
	choices         []string
	cursor          int
	config          *config.Config
	logger          logger.Logger
	quitting        bool
	message         string
	dbTypes         []dbTypeOption
	dbTypeCursor    int

	// Background operations
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMenuModel(cfg *config.Config, log logger.Logger) MenuModel {
	ctx, cancel := context.WithCancel(context.Background())

	dbTypes := []dbTypeOption{
		{label: "PostgreSQL", value: "postgres"},
		{label: "MySQL", value: "mysql"},
		{label: "MariaDB", value: "mariadb"},
	}

	dbCursor := 0
	if cfg.DatabaseType == "mysql" {
		dbCursor = 1
	} else if cfg.DatabaseType == "mariadb" {
		dbCursor = 2
	}

	model := MenuModel{
		choices: []string{
			"Single Database Backup",
			"Sample Database Backup (with ratio)",
			"Cluster Backup (all databases)",
			"────────────────────────────────",
			"Restore Single Database",
			"Restore Cluster Backup",
			"List & Manage Backups",
			"────────────────────────────────",
			"View Active Operations",
			"Show Operation History",
			"Database Status & Health Check",
			"Configuration Settings",
			"Clear Operation History",
			"Quit",
		},
		config:       cfg,
		logger:       log,
		ctx:          ctx,
		cancel:       cancel,
		dbTypes:      dbTypes,
		dbTypeCursor: dbCursor,
	}

	return model
}

// Init initializes the model
func (m MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Cancel all running operations
			if m.cancel != nil {
				m.cancel()
			}
			
			// Clean up any orphaned processes before exit
			m.logger.Info("Cleaning up processes before exit")
			if err := cleanup.KillOrphanedProcesses(m.logger); err != nil {
				m.logger.Warn("Failed to clean up all processes", "error", err)
			}
			
			m.quitting = true
			return m, tea.Quit

		case "left", "h":
			if m.dbTypeCursor > 0 {
				m.dbTypeCursor--
				m.applyDatabaseSelection()
			}

		case "right", "l":
			if m.dbTypeCursor < len(m.dbTypes)-1 {
				m.dbTypeCursor++
				m.applyDatabaseSelection()
			}

		case "t":
			if len(m.dbTypes) > 0 {
				m.dbTypeCursor = (m.dbTypeCursor + 1) % len(m.dbTypes)
				m.applyDatabaseSelection()
			}

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			switch m.cursor {
			case 0: // Single Database Backup
				return m.handleSingleBackup()
			case 1: // Sample Database Backup
				return m.handleSampleBackup()
			case 2: // Cluster Backup
				return m.handleClusterBackup()
			case 3: // Separator
				// Do nothing
			case 4: // Restore Single Database
				return m.handleRestoreSingle()
			case 5: // Restore Cluster Backup
				return m.handleRestoreCluster()
			case 6: // List & Manage Backups
				return m.handleBackupManager()
			case 7: // Separator
				// Do nothing
			case 8: // View Active Operations
				return m.handleViewOperations()
			case 9: // Show Operation History
				return m.handleOperationHistory()
			case 10: // Database Status
				return m.handleStatus()
			case 11: // Settings
				return m.handleSettings()
			case 12: // Clear History
				m.message = "🗑️ History cleared"
			case 13: // Quit
				if m.cancel != nil {
					m.cancel()
				}
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

// View renders the simple menu
func (m MenuModel) View() string {
	if m.quitting {
		return "Thanks for using DB Backup Tool!\n"
	}

	var s string

	// Header
	header := titleStyle.Render("🗄️  Database Backup Tool - Interactive Menu")
	s += fmt.Sprintf("\n%s\n\n", header)

	if len(m.dbTypes) > 0 {
		options := make([]string, len(m.dbTypes))
		for i, opt := range m.dbTypes {
			if m.dbTypeCursor == i {
				options[i] = menuSelectedStyle.Render(opt.label)
			} else {
				options[i] = menuStyle.Render(opt.label)
			}
		}
		selector := fmt.Sprintf("Target Engine: %s", strings.Join(options, menuStyle.Render("  |  ")))
		s += dbSelectorLabelStyle.Render(selector) + "\n"
		hint := infoStyle.Render("Switch with ←/→ or t • Cluster backup requires PostgreSQL")
		s += hint + "\n"
	}

	// Database info
	dbInfo := infoStyle.Render(fmt.Sprintf("Database: %s@%s:%d (%s)",
		m.config.User, m.config.Host, m.config.Port, m.config.DisplayDatabaseType()))
	s += fmt.Sprintf("%s\n\n", dbInfo)

	// Menu items
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s += menuSelectedStyle.Render(fmt.Sprintf("%s %s", cursor, choice))
		} else {
			s += menuStyle.Render(fmt.Sprintf("%s %s", cursor, choice))
		}
		s += "\n"
	}

	// Message area
	if m.message != "" {
		s += "\n" + m.message + "\n"
	}

	// Footer
	footer := infoStyle.Render("\n⌨️ Press ↑/↓ to navigate • Enter to select • q to quit")
	s += footer

	return s
}

// handleSingleBackup opens database selector for single backup
func (m MenuModel) handleSingleBackup() (tea.Model, tea.Cmd) {
	selector := NewDatabaseSelector(m.config, m.logger, m, m.ctx, "🗄️  Single Database Backup", "single")
	return selector, selector.Init()
}

// handleSampleBackup opens database selector for sample backup
func (m MenuModel) handleSampleBackup() (tea.Model, tea.Cmd) {
	selector := NewDatabaseSelector(m.config, m.logger, m, m.ctx, "📊 Sample Database Backup", "sample")
	return selector, selector.Init()
}

// handleClusterBackup shows confirmation and executes cluster backup
func (m MenuModel) handleClusterBackup() (tea.Model, tea.Cmd) {
	if !m.config.IsPostgreSQL() {
		m.message = errorStyle.Render("❌ Cluster backup is available only for PostgreSQL targets")
		return m, nil
	}
	confirm := NewConfirmationModelWithAction(m.config, m.logger, m,
		"🗄️  Cluster Backup",
		"This will backup ALL databases in the cluster. Continue?",
		func() (tea.Model, tea.Cmd) {
			executor := NewBackupExecution(m.config, m.logger, m, m.ctx, "cluster", "", 0)
			return executor, executor.Init()
		})
	return confirm, nil
}

// handleViewOperations shows active operations
func (m MenuModel) handleViewOperations() (tea.Model, tea.Cmd) {
	ops := NewOperationsView(m.config, m.logger, m)
	return ops, nil
}

// handleOperationHistory shows operation history
func (m MenuModel) handleOperationHistory() (tea.Model, tea.Cmd) {
	history := NewHistoryView(m.config, m.logger, m)
	return history, nil
}

// handleStatus shows database status
func (m MenuModel) handleStatus() (tea.Model, tea.Cmd) {
	status := NewStatusView(m.config, m.logger, m)
	return status, status.Init()
}

// handleSettings opens settings
func (m MenuModel) handleSettings() (tea.Model, tea.Cmd) {
	// Create and return the settings model
	settingsModel := NewSettingsModel(m.config, m.logger, m)
	return settingsModel, nil
}

// handleRestoreSingle opens archive browser for single restore
func (m MenuModel) handleRestoreSingle() (tea.Model, tea.Cmd) {
	browser := NewArchiveBrowser(m.config, m.logger, m, m.ctx, "restore-single")
	return browser, browser.Init()
}

// handleRestoreCluster opens archive browser for cluster restore
func (m MenuModel) handleRestoreCluster() (tea.Model, tea.Cmd) {
	if !m.config.IsPostgreSQL() {
		m.message = errorStyle.Render("❌ Cluster restore is available only for PostgreSQL")
		return m, nil
	}
	browser := NewArchiveBrowser(m.config, m.logger, m, m.ctx, "restore-cluster")
	return browser, browser.Init()
}

// handleBackupManager opens backup management view
func (m MenuModel) handleBackupManager() (tea.Model, tea.Cmd) {
	manager := NewBackupManager(m.config, m.logger, m, m.ctx)
	return manager, manager.Init()
}

func (m *MenuModel) applyDatabaseSelection() {
	if m == nil || len(m.dbTypes) == 0 {
		return
	}
	if m.dbTypeCursor < 0 || m.dbTypeCursor >= len(m.dbTypes) {
		return
	}

	selection := m.dbTypes[m.dbTypeCursor]
	if err := m.config.SetDatabaseType(selection.value); err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ %v", err))
		return
	}

	// Refresh default port if unchanged
	if m.config.Port == 0 {
		m.config.Port = m.config.GetDefaultPort()
	}

	m.message = successStyle.Render(fmt.Sprintf("🔀 Target database set to %s", m.config.DisplayDatabaseType()))
	if m.logger != nil {
		m.logger.Info("updated target database type", "type", m.config.DatabaseType, "port", m.config.Port)
	}
}

// RunInteractiveMenu starts the simple TUI
func RunInteractiveMenu(cfg *config.Config, log logger.Logger) error {
	m := NewMenuModel(cfg, log)
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running interactive menu: %w", err)
	}

	return nil
}
