package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
	"dbbackup/internal/progress"
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

	selectedStyle = lipgloss.NewStyle().
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

	progressStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD93D")).
			Bold(true)

	stepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6BCF7F")).
			MarginLeft(2)

	detailStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8A8A8")).
			MarginLeft(4).
			Italic(true)
)

// MenuModel represents the enhanced menu state with progress tracking
type MenuModel struct {
	choices  []string
	cursor   int
	config   *config.Config
	logger   logger.Logger
	quitting bool
	message  string
	
	// Progress tracking
	showProgress         bool
	showCompletion       bool
	completionMessage    string
	completionDismissed  bool  // Track if user manually dismissed completion
	currentOperation     *progress.OperationStatus
	allOperations        []progress.OperationStatus
	lastUpdate           time.Time
	spinner              spinner.Model
	
	// Background operations
	ctx    context.Context
	cancel context.CancelFunc
	
	// TUI Progress Reporter
	progressReporter *TUIProgressReporter
}

// completionMsg carries completion status
type completionMsg struct {
	success bool
	message string
}

// operationUpdateMsg carries operation updates
type operationUpdateMsg struct {
	operations []progress.OperationStatus
}

// operationCompleteMsg signals operation completion
type operationCompleteMsg struct {
	operation *progress.OperationStatus
	success   bool
}

// Initialize the menu model
func NewMenuModel(cfg *config.Config, log logger.Logger) MenuModel {
	ctx, cancel := context.WithCancel(context.Background())
	
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD93D"))
	
	// Create TUI progress reporter
	progressReporter := NewTUIProgressReporter()
	
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
		config:           cfg,
		logger:           log,
		ctx:              ctx,
		cancel:           cancel,
		spinner:          s,
		lastUpdate:       time.Now(),
		progressReporter: progressReporter,
	}
	
	// Set up progress callback
	progressReporter.AddCallback(func(operations []progress.OperationStatus) {
		// This will be called when operations update
		// The TUI will pick up these updates in the pollOperations method
	})
	
	return model
}

// Init initializes the model
func (m MenuModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.pollOperations(),
	)
}

// pollOperations periodically checks for operation updates
func (m MenuModel) pollOperations() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		// Get operations from our TUI progress reporter
		operations := m.progressReporter.GetOperations()
		return operationUpdateMsg{operations: operations}
	})
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
			// Clear completion status and allow navigation
			if m.showCompletion {
				m.showCompletion = false
				m.completionMessage = ""
				m.message = ""
				m.completionDismissed = true  // Mark as manually dismissed
			}
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			// Clear completion status and allow navigation
			if m.showCompletion {
				m.showCompletion = false
				m.completionMessage = ""
				m.message = ""
				m.completionDismissed = true  // Mark as manually dismissed
			}
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", " ":
			// Clear completion status and allow selection
			if m.showCompletion {
				m.showCompletion = false
				m.completionMessage = ""
				m.message = ""
				m.completionDismissed = true  // Mark as manually dismissed
				return m, m.pollOperations()
			}
			
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
				return m.handleClearHistory()
			case 8: // Quit
				if m.cancel != nil {
					m.cancel()
				}
				m.quitting = true
				return m, tea.Quit
			}
			
		case "esc":
			// Clear completion status on escape
			if m.showCompletion {
				m.showCompletion = false
				m.completionMessage = ""
				m.message = ""
				m.completionDismissed = true  // Mark as manually dismissed
			}
		}
		
	case operationUpdateMsg:
		m.allOperations = msg.operations
		if len(msg.operations) > 0 {
			latest := msg.operations[len(msg.operations)-1]
			if latest.Status == "running" {
				m.currentOperation = &latest
				m.showProgress = true
				m.showCompletion = false
				m.completionDismissed = false  // Reset dismissal flag for new operation
			} else if m.currentOperation != nil && latest.ID == m.currentOperation.ID {
				m.currentOperation = &latest
				m.showProgress = false
				// Only show completion status if user hasn't manually dismissed it
				if !m.completionDismissed {
					if latest.Status == "completed" {
						m.showCompletion = true
						m.completionMessage = fmt.Sprintf("✅ %s", latest.Message)
					} else if latest.Status == "failed" {
						m.showCompletion = true
						m.completionMessage = fmt.Sprintf("❌ %s", latest.Message)
					}
				}
			}
		}
		return m, m.pollOperations()
		
	case completionMsg:
		m.showProgress = false
		m.showCompletion = true
		if msg.success {
			m.completionMessage = fmt.Sprintf("✅ %s", msg.message)
		} else {
			m.completionMessage = fmt.Sprintf("❌ %s", msg.message)
		}
		return m, m.pollOperations()
		
	case operationCompleteMsg:
		m.currentOperation = msg.operation
		m.showProgress = false
		if msg.success {
			m.message = fmt.Sprintf("✅ Operation completed: %s", msg.operation.Message)
		} else {
			m.message = fmt.Sprintf("❌ Operation failed: %s", msg.operation.Message)
		}
		return m, m.pollOperations()
		
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the enhanced menu with progress tracking
func (m MenuModel) View() string {
	if m.quitting {
		return "Thanks for using DB Backup Tool!\n"
	}

	var b strings.Builder

	// Header
	header := titleStyle.Render("🗄️  Database Backup Tool - Interactive Menu")
	b.WriteString(fmt.Sprintf("\n%s\n\n", header))

	// Database info
	dbInfo := infoStyle.Render(fmt.Sprintf("Database: %s@%s:%d (%s)", 
		m.config.User, m.config.Host, m.config.Port, m.config.DatabaseType))
	b.WriteString(fmt.Sprintf("%s\n\n", dbInfo))

	// Menu items
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			b.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s", cursor, choice)))
		} else {
			b.WriteString(menuStyle.Render(fmt.Sprintf("%s %s", cursor, choice)))
		}
		b.WriteString("\n")
	}

	// Current operation progress
	if m.showProgress && m.currentOperation != nil {
		b.WriteString("\n")
		b.WriteString(m.renderOperationProgress(m.currentOperation))
		b.WriteString("\n")
	}

	// Completion status (persistent until key press)
	if m.showCompletion {
		b.WriteString("\n")
		b.WriteString(successStyle.Render(m.completionMessage))
		b.WriteString("\n")
		b.WriteString(infoStyle.Render("💡 Press any key to continue..."))
		b.WriteString("\n")
	}

	// Message area
	if m.message != "" && !m.showCompletion {
		b.WriteString("\n")
		b.WriteString(m.message)
		b.WriteString("\n")
	}

	// Operations summary
	if len(m.allOperations) > 0 {
		b.WriteString("\n")
		b.WriteString(m.renderOperationsSummary())
		b.WriteString("\n")
	}

	// Footer
	var footer string
	if m.showCompletion {
		footer = infoStyle.Render("\n⌨️ Press Enter, ↑/↓ arrows, or Esc to continue...")
	} else {
		footer = infoStyle.Render("\n⌨️ Press ↑/↓ to navigate • Enter to select • q to quit")
	}
	b.WriteString(footer)

	return b.String()
}

// renderOperationProgress renders detailed progress for the current operation
func (m MenuModel) renderOperationProgress(op *progress.OperationStatus) string {
	var b strings.Builder
	
	// Operation header with spinner
	spinnerView := ""
	if op.Status == "running" {
		spinnerView = m.spinner.View() + " "
	}
	
	status := "🔄"
	if op.Status == "completed" {
		status = "✅"
	} else if op.Status == "failed" {
		status = "❌"
	}
	
	b.WriteString(progressStyle.Render(fmt.Sprintf("%s%s %s [%d%%]", 
		spinnerView, status, strings.Title(op.Type), op.Progress)))
	b.WriteString("\n")
	
	// Progress bar
	barWidth := 40
	filledWidth := (op.Progress * barWidth) / 100
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	bar := strings.Repeat("█", filledWidth) + strings.Repeat("░", barWidth-filledWidth)
	b.WriteString(detailStyle.Render(fmt.Sprintf("[%s] %s", bar, op.Message)))
	b.WriteString("\n")
	
	// Time and details
	elapsed := time.Since(op.StartTime)
	timeInfo := fmt.Sprintf("Elapsed: %s", formatDuration(elapsed))
	if op.EndTime != nil {
		timeInfo = fmt.Sprintf("Duration: %s", op.Duration.String())
	}
	b.WriteString(detailStyle.Render(timeInfo))
	b.WriteString("\n")
	
	// File/byte progress
	if op.FilesTotal > 0 {
		b.WriteString(detailStyle.Render(fmt.Sprintf("Files: %d/%d", op.FilesDone, op.FilesTotal)))
		b.WriteString("\n")
	}
	if op.BytesTotal > 0 {
		b.WriteString(detailStyle.Render(fmt.Sprintf("Data: %s/%s", 
			formatBytes(op.BytesDone), formatBytes(op.BytesTotal))))
		b.WriteString("\n")
	}
	
	// Current steps
	if len(op.Steps) > 0 {
		b.WriteString(stepStyle.Render("Steps:"))
		b.WriteString("\n")
		for _, step := range op.Steps {
			stepStatus := "⏳"
			if step.Status == "completed" {
				stepStatus = "✅"
			} else if step.Status == "failed" {
				stepStatus = "❌"
			}
			b.WriteString(detailStyle.Render(fmt.Sprintf("  %s %s", stepStatus, step.Name)))
			b.WriteString("\n")
		}
	}
	
	return b.String()
}

// renderOperationsSummary renders a summary of all operations
func (m MenuModel) renderOperationsSummary() string {
	if len(m.allOperations) == 0 {
		return ""
	}
	
	completed := 0
	failed := 0
	running := 0
	
	for _, op := range m.allOperations {
		switch op.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		case "running":
			running++
		}
	}
	
	summary := fmt.Sprintf("📊 Operations: %d total | %d completed | %d failed | %d running", 
		len(m.allOperations), completed, failed, running)
	
	return infoStyle.Render(summary)
}

// Enhanced backup handlers with progress tracking

// Handle single database backup with progress
func (m MenuModel) handleSingleBackup() (tea.Model, tea.Cmd) {
	if m.config.Database == "" {
		m.message = errorStyle.Render("❌ No database specified. Use --database flag or set in config.")
		return m, nil
	}

	m.message = progressStyle.Render(fmt.Sprintf("🔄 Starting single backup for: %s", m.config.Database))
	m.showProgress = true
	m.showCompletion = false
	
	// Start backup and return polling command
	go func() {
		err := RunBackupInTUI(m.ctx, m.config, m.logger, "single", m.config.Database, m.progressReporter)
		// The completion will be handled by the progress reporter callback system
		_ = err // Handle error in the progress reporter
	}()
	
	return m, m.pollOperations()
}

// Handle sample backup with progress
func (m MenuModel) handleSampleBackup() (tea.Model, tea.Cmd) {
	m.message = progressStyle.Render("🔄 Starting sample backup...")
	m.showProgress = true
	m.showCompletion = false
	m.completionDismissed = false  // Reset for new operation
	
	// Start backup and return polling command
	go func() {
		err := RunBackupInTUI(m.ctx, m.config, m.logger, "sample", "", m.progressReporter)
		// The completion will be handled by the progress reporter callback system
		_ = err // Handle error in the progress reporter
	}()
	
	return m, m.pollOperations()
}

// Handle cluster backup with progress
func (m MenuModel) handleClusterBackup() (tea.Model, tea.Cmd) {
	m.message = progressStyle.Render("🔄 Starting cluster backup (all databases)...")
	m.showProgress = true
	m.showCompletion = false
	m.completionDismissed = false  // Reset for new operation
	
	// Start backup and return polling command
	go func() {
		err := RunBackupInTUI(m.ctx, m.config, m.logger, "cluster", "", m.progressReporter)
		// The completion will be handled by the progress reporter callback system
		_ = err // Handle error in the progress reporter
	}()
	
	return m, m.pollOperations()
}

// Handle viewing active operations
func (m MenuModel) handleViewOperations() (tea.Model, tea.Cmd) {
	if len(m.allOperations) == 0 {
		m.message = infoStyle.Render("ℹ️  No operations currently running or completed")
		return m, nil
	}
	
	var activeOps []progress.OperationStatus
	for _, op := range m.allOperations {
		if op.Status == "running" {
			activeOps = append(activeOps, op)
		}
	}
	
	if len(activeOps) == 0 {
		m.message = infoStyle.Render("ℹ️  No operations currently running")
	} else {
		m.message = progressStyle.Render(fmt.Sprintf("🔄 %d active operations", len(activeOps)))
	}
	
	return m, nil
}

// Handle showing operation history
func (m MenuModel) handleOperationHistory() (tea.Model, tea.Cmd) {
	if len(m.allOperations) == 0 {
		m.message = infoStyle.Render("ℹ️  No operation history available")
		return m, nil
	}
	
	var history strings.Builder
	history.WriteString("📋 Operation History:\n")
	
	for i, op := range m.allOperations {
		if i >= 5 { // Show last 5 operations
			break
		}
		
		status := "🔄"
		if op.Status == "completed" {
			status = "✅"
		} else if op.Status == "failed" {
			status = "❌"
		}
		
		history.WriteString(fmt.Sprintf("%s %s - %s (%s)\n", 
			status, op.Name, op.Type, op.StartTime.Format("15:04:05")))
	}
	
	m.message = history.String()
	return m, nil
}

// Handle status check
func (m MenuModel) handleStatus() (tea.Model, tea.Cmd) {
	db, err := database.New(m.config, m.logger)
	if err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Connection failed: %v", err))
		return m, nil
	}
	defer db.Close()

	err = db.Connect(m.ctx)
	if err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Connection failed: %v", err))
		return m, nil
	}

	err = db.Ping(m.ctx)
	if err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Ping failed: %v", err))
		return m, nil
	}

	version, err := db.GetVersion(m.ctx)
	if err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Failed to get version: %v", err))
		return m, nil
	}

	m.message = successStyle.Render(fmt.Sprintf("✅ Connected successfully!\nVersion: %s", version))
	return m, nil
}

// Handle settings display
func (m MenuModel) handleSettings() (tea.Model, tea.Cmd) {
	// Create and switch to settings model
	settingsModel := NewSettingsModel(m.config, m.logger, m)
	return settingsModel, settingsModel.Init()
}

// Handle clearing operation history
func (m MenuModel) handleClearHistory() (tea.Model, tea.Cmd) {
	m.allOperations = []progress.OperationStatus{}
	m.currentOperation = nil
	m.showProgress = false
	m.message = successStyle.Render("✅ Operation history cleared")
	return m, nil
}

// Utility functions

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// formatBytes formats byte count in human-readable format
func formatBytes(bytes int64) string {
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

// RunInteractiveMenu starts the enhanced TUI with progress tracking
func RunInteractiveMenu(cfg *config.Config, log logger.Logger) error {
	m := NewMenuModel(cfg, log)
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running interactive menu: %w", err)
	}
	
	return nil
}