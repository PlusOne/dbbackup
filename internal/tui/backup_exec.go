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
	"dbbackup/internal/progress"
)

// BackupExecutionModel handles backup execution with progress
type BackupExecutionModel struct {
	config       *config.Config
	logger       logger.Logger
	parent       tea.Model
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
}

func NewBackupExecution(cfg *config.Config, log logger.Logger, parent tea.Model, backupType, dbName string, ratio int) BackupExecutionModel {
	return BackupExecutionModel{
		config:       cfg,
		logger:       log,
		parent:       parent,
		backupType:   backupType,
		databaseName: dbName,
		ratio:        ratio,
		status:       "Initializing...",
		startTime:    time.Now(),
		details:      []string{},
	}
}

func (m BackupExecutionModel) Init() tea.Cmd {
	reporter := NewTUIProgressReporter()
	reporter.AddCallback(func(ops []progress.OperationStatus) {
		if len(ops) == 0 {
			return
		}

		latest := ops[len(ops)-1]
		tea.Println(backupProgressMsg{
			status:   latest.Message,
			progress: latest.Progress,
			detail:   latest.Status,
		})
	})

	return tea.Batch(
		executeBackupWithTUIProgress(m.config, m.logger, m.backupType, m.databaseName, m.ratio, reporter),
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

func executeBackupWithTUIProgress(cfg *config.Config, log logger.Logger, backupType, dbName string, ratio int, reporter *TUIProgressReporter) tea.Cmd {
	return func() tea.Msg {
			// Use configurable cluster timeout (minutes) from config; default set in config.New()
			clusterTimeout := time.Duration(cfg.ClusterTimeoutMinutes) * time.Minute
			ctx, cancel := context.WithTimeout(context.Background(), clusterTimeout)
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

		engine := backup.NewSilent(cfg, log, dbClient, reporter)

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

	header := titleStyle.Render("🔄 Backup Execution")
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	s.WriteString(fmt.Sprintf("Type: %s\n", m.backupType))
	if m.databaseName != "" {
		s.WriteString(fmt.Sprintf("Database: %s\n", m.databaseName))
	}
	if m.ratio > 0 {
		s.WriteString(fmt.Sprintf("Sample Ratio: %d\n", m.ratio))
	}
	s.WriteString(fmt.Sprintf("Duration: %s\n\n", time.Since(m.startTime).Round(time.Second)))

	s.WriteString(fmt.Sprintf("Status: %s\n", m.status))

	if !m.done {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := int(time.Since(m.startTime).Milliseconds()/100) % len(spinner)
		s.WriteString(fmt.Sprintf("\n%s Processing...\n", spinner[frame]))
	} else {
		s.WriteString("\n")
		if m.err != nil {
			s.WriteString(fmt.Sprintf("Error: %v\n\n", m.err))
		}
		lines := strings.Split(m.result, "\n")
		for _, line := range lines {
			if strings.Contains(line, "✅") || strings.Contains(line, "completed") ||
				strings.Contains(line, "Size:") || strings.Contains(line, "backup_") {
				s.WriteString(line + "\n")
			}
		}
		s.WriteString("\n⌨️  Press Enter or ESC to return to menu\n")
	}

	return s.String()
}
