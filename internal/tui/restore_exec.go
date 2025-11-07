package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
	"dbbackup/internal/restore"
)

// RestoreExecutionModel handles restore execution with progress
type RestoreExecutionModel struct {
	config       *config.Config
	logger       logger.Logger
	parent       tea.Model
	archive      ArchiveInfo
	targetDB     string
	cleanFirst   bool
	createIfMissing bool
	restoreType  string
	
	// Progress tracking
	status       string
	phase        string
	progress     int
	details      []string
	startTime    time.Time
	spinnerFrame int
	spinnerFrames []string
	
	// Results
	done         bool
	err          error
	result       string
	elapsed      time.Duration
}

// NewRestoreExecution creates a new restore execution model
func NewRestoreExecution(cfg *config.Config, log logger.Logger, parent tea.Model, archive ArchiveInfo, targetDB string, cleanFirst, createIfMissing bool, restoreType string) RestoreExecutionModel {
	return RestoreExecutionModel{
		config:       cfg,
		logger:       log,
		parent:       parent,
		archive:      archive,
		targetDB:     targetDB,
		cleanFirst:   cleanFirst,
		createIfMissing: createIfMissing,
		restoreType:  restoreType,
		status:       "Initializing...",
		phase:        "Starting",
		startTime:    time.Now(),
		details:      []string{},
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		spinnerFrame: 0,
	}
}

func (m RestoreExecutionModel) Init() tea.Cmd {
	return tea.Batch(
		executeRestoreWithTUIProgress(m.config, m.logger, m.archive, m.targetDB, m.cleanFirst, m.createIfMissing, m.restoreType),
		restoreTickCmd(),
	)
}

type restoreTickMsg time.Time

func restoreTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return restoreTickMsg(t)
	})
}

type restoreProgressMsg struct {
	status   string
	phase    string
	progress int
	detail   string
}

type restoreCompleteMsg struct {
	result  string
	err     error
	elapsed time.Duration
}

func executeRestoreWithTUIProgress(cfg *config.Config, log logger.Logger, archive ArchiveInfo, targetDB string, cleanFirst, createIfMissing bool, restoreType string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()

		start := time.Now()

		// Create database instance
		dbClient, err := database.New(cfg, log)
		if err != nil {
			return restoreCompleteMsg{
				result:  "",
				err:     fmt.Errorf("failed to create database client: %w", err),
				elapsed: time.Since(start),
			}
		}
		defer dbClient.Close()

		// Create restore engine with silent progress (no stdout interference with TUI)
		engine := restore.NewSilent(cfg, log, dbClient)

		// Execute restore based on type
		var restoreErr error
		if restoreType == "restore-cluster" {
			restoreErr = engine.RestoreCluster(ctx, archive.Path)
		} else {
			restoreErr = engine.RestoreSingle(ctx, archive.Path, targetDB, cleanFirst, createIfMissing)
		}

		if restoreErr != nil {
			return restoreCompleteMsg{
				result:  "",
				err:     restoreErr,
				elapsed: time.Since(start),
			}
		}

		result := fmt.Sprintf("Successfully restored from %s", archive.Name)
		if restoreType == "restore-single" {
			result = fmt.Sprintf("Successfully restored '%s' from %s", targetDB, archive.Name)
		}

		return restoreCompleteMsg{
			result:  result,
			err:     nil,
			elapsed: time.Since(start),
		}
	}
}

func (m RestoreExecutionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case restoreTickMsg:
		if !m.done {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(m.spinnerFrames)
			m.elapsed = time.Since(m.startTime)
			return m, restoreTickCmd()
		}
		return m, nil

	case restoreProgressMsg:
		m.status = msg.status
		m.phase = msg.phase
		m.progress = msg.progress
		if msg.detail != "" {
			m.details = append(m.details, msg.detail)
			// Keep only last 5 details
			if len(m.details) > 5 {
				m.details = m.details[len(m.details)-5:]
			}
		}
		return m, nil

	case restoreCompleteMsg:
		m.done = true
		m.err = msg.err
		m.result = msg.result
		m.elapsed = msg.elapsed
		
		if m.err == nil {
			m.status = "Completed"
			m.phase = "Done"
			m.progress = 100
		} else {
			m.status = "Failed"
			m.phase = "Error"
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Always allow quitting
			return m.parent, tea.Quit

		case "enter", " ", "esc":
			if m.done {
				return m.parent, nil
			}
		}
	}

	return m, nil
}

func (m RestoreExecutionModel) View() string {
	var s strings.Builder

	// Title
	title := "💾 Restoring Database"
	if m.restoreType == "restore-cluster" {
		title = "💾 Restoring Cluster"
	}
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n\n")

	// Archive info
	s.WriteString(fmt.Sprintf("Archive: %s\n", m.archive.Name))
	if m.restoreType == "restore-single" {
		s.WriteString(fmt.Sprintf("Target: %s\n", m.targetDB))
	}
	s.WriteString("\n")

	if m.done {
		// Show result
		if m.err != nil {
			s.WriteString(errorStyle.Render("❌ Restore Failed"))
			s.WriteString("\n\n")
			s.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
			s.WriteString("\n")
		} else {
			s.WriteString(successStyle.Render("✅ Restore Completed Successfully"))
			s.WriteString("\n\n")
			s.WriteString(successStyle.Render(m.result))
			s.WriteString("\n")
		}

		s.WriteString(fmt.Sprintf("\nElapsed Time: %s\n", formatDuration(m.elapsed)))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("⌨️  Press Enter to continue"))
	} else {
		// Show progress
		s.WriteString(fmt.Sprintf("Phase: %s\n", m.phase))
		
		// Show status with rotating spinner
		spinner := m.spinnerFrames[m.spinnerFrame]
		s.WriteString(fmt.Sprintf("Status: %s %s\n", spinner, m.status))
		s.WriteString("\n")

		// Progress bar
		progressBar := renderProgressBar(m.progress)
		s.WriteString(progressBar)
		s.WriteString(fmt.Sprintf("  %d%%\n", m.progress))
		s.WriteString("\n")

		// Details
		if len(m.details) > 0 {
			s.WriteString(infoStyle.Render("Recent activity:"))
			s.WriteString("\n")
			for _, detail := range m.details {
				s.WriteString(fmt.Sprintf("  • %s\n", detail))
			}
			s.WriteString("\n")
		}

		// Elapsed time
		s.WriteString(fmt.Sprintf("Elapsed: %s\n", formatDuration(m.elapsed)))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("⌨️  Press Ctrl+C to cancel"))
	}

	return s.String()
}

// renderProgressBar renders a text progress bar
func renderProgressBar(percent int) string {
	width := 40
	filled := (percent * width) / 100
	
	bar := strings.Repeat("█", filled)
	empty := strings.Repeat("░", width-filled)
	
	return successStyle.Render(bar) + infoStyle.Render(empty)
}

// formatDuration formats duration in human readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
