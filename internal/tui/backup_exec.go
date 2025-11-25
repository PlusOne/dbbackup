package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/backup"
	"dbbackup/internal/config"
	"dbbackup/internal/database"
	"dbbackup/internal/logger"
)

// BackupExecutionModel handles backup execution with progress
type BackupExecutionModel struct {
	config       *config.Config
	logger       logger.Logger
	parent       tea.Model
	ctx          context.Context
	backupType   string
	databaseName string
	ratio        int
	status       string
	progress     int
	done         bool
	err          error
	result       string
	startTime    time.Time
	details      []string
	spinnerFrame int
}

func NewBackupExecution(cfg *config.Config, log logger.Logger, parent tea.Model, ctx context.Context, backupType, dbName string, ratio int) BackupExecutionModel {
	return BackupExecutionModel{
		config:       cfg,
		logger:       log,
		parent:       parent,
		ctx:          ctx,
		backupType:   backupType,
		databaseName: dbName,
		ratio:        ratio,
		status:       "Initializing...",
		startTime:    time.Now(),
		details:      []string{},
		spinnerFrame: 0,
	}
}

func (m BackupExecutionModel) Init() tea.Cmd {
	// TUI handles all display through View() - no progress callbacks needed
	return tea.Batch(
		executeBackupWithTUIProgress(m.ctx, m.config, m.logger, m.backupType, m.databaseName, m.ratio),
		backupTickCmd(),
	)
}

type backupTickMsg time.Time

func backupTickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return backupTickMsg(t)
	})
}

type backupProgressMsg struct {
	status   string
	progress int
	detail   string
}

type backupCompleteMsg struct {
	result string
	err    error
}

func executeBackupWithTUIProgress(parentCtx context.Context, cfg *config.Config, log logger.Logger, backupType, dbName string, ratio int) tea.Cmd {
	return func() tea.Msg {
			// Use configurable cluster timeout (minutes) from config; default set in config.New()
			// Use parent context to inherit cancellation from TUI
			clusterTimeout := time.Duration(cfg.ClusterTimeoutMinutes) * time.Minute
			ctx, cancel := context.WithTimeout(parentCtx, clusterTimeout)
		defer cancel()

		start := time.Now()

		dbClient, err := database.New(cfg, log)
		if err != nil {
			return backupCompleteMsg{
				result: "",
				err:    fmt.Errorf("failed to create database client: %w", err),
			}
		}
		defer dbClient.Close()

		if err := dbClient.Connect(ctx); err != nil {
			return backupCompleteMsg{
				result: "",
				err:    fmt.Errorf("database connection failed: %w", err),
			}
		}

		// Pass nil as indicator - TUI itself handles all display, no stdout printing
		engine := backup.NewSilent(cfg, log, dbClient, nil)

		var backupErr error
		switch backupType {
		case "single":
			backupErr = engine.BackupSingle(ctx, dbName)
		case "sample":
			cfg.SampleStrategy = "ratio"
			cfg.SampleValue = ratio
			backupErr = engine.BackupSample(ctx, dbName)
		case "cluster":
			backupErr = engine.BackupCluster(ctx)
		default:
			return backupCompleteMsg{err: fmt.Errorf("unknown backup type: %s", backupType)}
		}

		if backupErr != nil {
			return backupCompleteMsg{
				result: "",
				err:    fmt.Errorf("backup failed: %w", backupErr),
			}
		}

		elapsed := time.Since(start).Round(time.Second)

		var result string
		switch backupType {
		case "single":
			result = fmt.Sprintf("✓ Single database backup of '%s' completed successfully in %v", dbName, elapsed)
		case "sample":
			result = fmt.Sprintf("✓ Sample backup of '%s' (ratio: %d) completed successfully in %v", dbName, ratio, elapsed)
		case "cluster":
			result = fmt.Sprintf("✓ Cluster backup completed successfully in %v", elapsed)
		}

		return backupCompleteMsg{
			result: result,
			err:    nil,
		}
	}
}

func (m BackupExecutionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case backupTickMsg:
		if !m.done {
			// Increment spinner frame for smooth animation
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			
			// Update status based on elapsed time to show progress
			elapsedSec := int(time.Since(m.startTime).Seconds())
			
			if elapsedSec < 2 {
				m.status = "Initializing backup..."
			} else if elapsedSec < 5 {
				if m.backupType == "cluster" {
					m.status = "Connecting to database cluster..."
				} else {
					m.status = fmt.Sprintf("Connecting to database '%s'...", m.databaseName)
				}
			} else if elapsedSec < 10 {
				if m.backupType == "cluster" {
					m.status = "Backing up global objects (roles, tablespaces)..."
				} else if m.backupType == "sample" {
					m.status = fmt.Sprintf("Analyzing tables for sampling (ratio: %d)...", m.ratio)
				} else {
					m.status = fmt.Sprintf("Dumping database '%s'...", m.databaseName)
				}
			} else {
				if m.backupType == "cluster" {
					m.status = "Backing up cluster databases..."
				} else if m.backupType == "sample" {
					m.status = fmt.Sprintf("Creating sample backup of '%s'...", m.databaseName)
				} else {
					m.status = fmt.Sprintf("Backing up database '%s'...", m.databaseName)
				}
			}
			
			return m, backupTickCmd()
		}
		return m, nil

	case backupProgressMsg:
		m.status = msg.status
		m.progress = msg.progress
		return m, nil

	case backupCompleteMsg:
		m.done = true
		m.err = msg.err
		m.result = msg.result
		if m.err == nil {
			m.status = "✅ Backup completed successfully!"
		} else {
			m.status = fmt.Sprintf("❌ Backup failed: %v", m.err)
		}
		return m, nil

	case tea.KeyMsg:
		if m.done {
			switch msg.String() {
			case "enter", "esc", "q":
				return m.parent, nil
			}
		}
	}

	return m, nil
}

func (m BackupExecutionModel) View() string {
	var s strings.Builder
	s.Grow(512) // Pre-allocate estimated capacity for better performance

	// Clear screen with newlines and render header
	s.WriteString("\n\n")
	header := titleStyle.Render("🔄 Backup Execution")
	s.WriteString(header)
	s.WriteString("\n\n")

	// Backup details - properly aligned
	s.WriteString(fmt.Sprintf("  %-10s %s\n", "Type:", m.backupType))
	if m.databaseName != "" {
		s.WriteString(fmt.Sprintf("  %-10s %s\n", "Database:", m.databaseName))
	}
	if m.ratio > 0 {
		s.WriteString(fmt.Sprintf("  %-10s %d\n", "Sample:", m.ratio))
	}
	s.WriteString(fmt.Sprintf("  %-10s %s\n", "Duration:", time.Since(m.startTime).Round(time.Second)))
	s.WriteString("\n")

	// Status with spinner
	if !m.done {
		s.WriteString(fmt.Sprintf("  %s %s\n", spinnerFrames[m.spinnerFrame], m.status))
	} else {
		s.WriteString(fmt.Sprintf("  %s\n\n", m.status))
		
		if m.err != nil {
			s.WriteString(fmt.Sprintf("  ❌ Error: %v\n", m.err))
		} else if m.result != "" {
			// Parse and display result cleanly
			lines := strings.Split(m.result, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					s.WriteString("  " + line + "\n")
				}
			}
		}
		s.WriteString("\n  ⌨️  Press Enter or ESC to return to menu\n")
	}

	return s.String()
}
