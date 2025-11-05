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
)

// StatusViewModel shows database status
type StatusViewModel struct {
	config    *config.Config
	logger    logger.Logger
	parent    tea.Model
	loading   bool
	status    string
	err       error
	dbCount   int
	dbVersion string
	connected bool
}

func NewStatusView(cfg *config.Config, log logger.Logger, parent tea.Model) StatusViewModel {
	return StatusViewModel{
		config:  cfg,
		logger:  log,
		parent:  parent,
		loading: true,
		status:  "Loading status...",
	}
}

func (m StatusViewModel) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(m.config, m.logger),
		tickCmd(),
	)
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type statusMsg struct {
	status    string
	err       error
	dbCount   int
	dbVersion string
	connected bool
}

func fetchStatus(cfg *config.Config, log logger.Logger) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		dbClient, err := database.New(cfg, log)
		if err != nil {
			return statusMsg{
				status:    "",
				err:       fmt.Errorf("failed to create database client: %w", err),
				connected: false,
			}
		}
		defer dbClient.Close()

		if err := dbClient.Connect(ctx); err != nil {
			return statusMsg{
				status:    "",
				err:       fmt.Errorf("connection failed: %w", err),
				connected: false,
			}
		}

		version, err := dbClient.GetVersion(ctx)
		if err != nil {
			log.Warn("failed to get database version", "error", err)
			version = "Unknown"
		}

		databases, err := dbClient.ListDatabases(ctx)
		if err != nil {
			return statusMsg{
				status:    "Connected, but failed to list databases",
				err:       fmt.Errorf("failed to list databases: %w", err),
				connected: true,
			}
		}

		return statusMsg{
			status:    "Database connection successful",
			err:       nil,
			dbCount:   len(databases),
			dbVersion: version,
			connected: true,
		}
	}
}

func (m StatusViewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		if m.loading {
			return m, tickCmd()
		}
		return m, nil

	case statusMsg:
		m.loading = false
		if msg.status != "" {
			m.status = msg.status
		}
		m.err = msg.err
		m.dbCount = msg.dbCount
		if msg.dbVersion != "" {
			m.dbVersion = msg.dbVersion
		}
		m.connected = msg.connected
		return m, nil

	case tea.KeyMsg:
		if !m.loading {
			switch msg.String() {
			case "ctrl+c", "q", "esc", "enter":
				return m.parent, nil
			}
		}
	}

	return m, nil
}

func (m StatusViewModel) View() string {
	var s strings.Builder

	header := titleStyle.Render("📊 Database Status & Health Check")
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	if m.loading {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := int(time.Now().UnixMilli()/100) % len(spinner)
		s.WriteString(fmt.Sprintf("%s Loading status information...\n", spinner[frame]))
		return s.String()
	}

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v\n", m.err)))
		s.WriteString("\n")
	} else {
		s.WriteString("Connection Status:\n")
		if m.connected {
			s.WriteString(successStyle.Render("  ✓ Connected\n"))
		} else {
			s.WriteString(errorStyle.Render("  ✗ Disconnected\n"))
		}
		s.WriteString("\n")

		s.WriteString(fmt.Sprintf("Database Type: %s (%s)\n", m.config.DisplayDatabaseType(), m.config.DatabaseType))
		s.WriteString(fmt.Sprintf("Host: %s:%d\n", m.config.Host, m.config.Port))
		s.WriteString(fmt.Sprintf("User: %s\n", m.config.User))
		s.WriteString(fmt.Sprintf("Backup Directory: %s\n", m.config.BackupDir))
		s.WriteString(fmt.Sprintf("Version: %s\n\n", m.dbVersion))

		if m.dbCount > 0 {
			s.WriteString(fmt.Sprintf("Databases Found: %s\n", successStyle.Render(fmt.Sprintf("%d", m.dbCount))))
		}

		s.WriteString("\n")
		s.WriteString(successStyle.Render("✓ All systems operational\n"))
	}

	s.WriteString("\n⌨️  Press any key to return to menu\n")
	return s.String()
}
