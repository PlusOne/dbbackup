package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// Style definitions
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	menuSelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF75B7")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF6B6B")).
			Bold(true)
)

// MenuModel represents the simple menu state
type MenuModel struct {
	choices  []string
	cursor   int
	config   *config.Config
	logger   logger.Logger
	quitting bool
	message  string
	
	// Background operations
	ctx    context.Context
	cancel context.CancelFunc
}

func NewMenuModel(cfg *config.Config, log logger.Logger) MenuModel {
	ctx, cancel := context.WithCancel(context.Background())
	
	model := MenuModel{
		choices: []string{
			"Single Database Backup",
			"Sample Database Backup (with ratio)",
			"Cluster Backup (all databases)",
			"View Active Operations",
			"Show Operation History",
			"Database Status & Health Check",
			"Configuration Settings",
			"Clear Operation History",
			"Quit",
		},
		config: cfg,
		logger: log,
		ctx:    ctx,
		cancel: cancel,
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
			if m.cancel != nil {
				m.cancel()
			}
			m.quitting = true
			return m, tea.Quit

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
			case 3: // View Active Operations
				return m.handleViewOperations()
			case 4: // Show Operation History
				return m.handleOperationHistory()
			case 5: // Database Status
				return m.handleStatus()
			case 6: // Settings
				return m.handleSettings()
			case 7: // Clear History
				m.message = "🗑️ History cleared"
			case 8: // Quit
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

	// Database info
	dbInfo := infoStyle.Render(fmt.Sprintf("Database: %s@%s:%d (%s)", 
		m.config.User, m.config.Host, m.config.Port, m.config.DatabaseType))
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
	selector := NewDatabaseSelector(m.config, m.logger, m, "🗄️  Single Database Backup", "single")
	return selector, selector.Init()
}

// handleSampleBackup opens database selector for sample backup
func (m MenuModel) handleSampleBackup() (tea.Model, tea.Cmd) {
	selector := NewDatabaseSelector(m.config, m.logger, m, "📊 Sample Database Backup", "sample")
	return selector, selector.Init()
}

// handleClusterBackup shows confirmation and executes cluster backup
func (m MenuModel) handleClusterBackup() (tea.Model, tea.Cmd) {
	confirm := NewConfirmationModel(m.config, m.logger, m,
		"🗄️  Cluster Backup",
		"This will backup ALL databases in the cluster. Continue?")
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

// RunInteractiveMenu starts the simple TUI
func RunInteractiveMenu(cfg *config.Config, log logger.Logger) error {
	m := NewMenuModel(cfg, log)
	p := tea.NewProgram(m)
	
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running interactive menu: %w", err)
	}
	
	return nil
}