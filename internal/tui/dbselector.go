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

// DatabaseSelectorModel for selecting a database
type DatabaseSelectorModel struct {
	config     *config.Config
	logger     logger.Logger
	parent     tea.Model
	ctx        context.Context
	databases  []string
	cursor     int
	selected   string
	loading    bool
	err        error
	title      string
	message    string
	backupType string // "single" or "sample"
}

func NewDatabaseSelector(cfg *config.Config, log logger.Logger, parent tea.Model, ctx context.Context, title string, backupType string) DatabaseSelectorModel {
	return DatabaseSelectorModel{
		config:     cfg,
		logger:     log,
		parent:     parent,
		ctx:        ctx,
		databases:  []string{"Loading databases..."},
		title:      title,
		loading:    true,
		backupType: backupType,
	}
}

func (m DatabaseSelectorModel) Init() tea.Cmd {
	return fetchDatabases(m.config, m.logger)
}

type databaseListMsg struct {
	databases []string
	err       error
}

func fetchDatabases(cfg *config.Config, log logger.Logger) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		dbClient, err := database.New(cfg, log)
		if err != nil {
			return databaseListMsg{databases: nil, err: fmt.Errorf("failed to create database client: %w", err)}
		}
		defer dbClient.Close()

		if err := dbClient.Connect(ctx); err != nil {
			return databaseListMsg{databases: nil, err: fmt.Errorf("connection failed: %w", err)}
		}

		databases, err := dbClient.ListDatabases(ctx)
		if err != nil {
			return databaseListMsg{databases: nil, err: fmt.Errorf("failed to list databases: %w", err)}
		}

		return databaseListMsg{databases: databases, err: nil}
	}
}

func (m DatabaseSelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case databaseListMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			m.databases = []string{"Error loading databases"}
		} else {
			m.databases = msg.databases
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
			if m.cursor < len(m.databases)-1 {
				m.cursor++
			}

		case "enter":
			if !m.loading && m.err == nil && len(m.databases) > 0 {
				m.selected = m.databases[m.cursor]
				
				// If sample backup, ask for ratio first
				if m.backupType == "sample" {
					inputModel := NewInputModel(m.config, m.logger, m,
						"📊 Sample Ratio",
						"Enter sample ratio (1-100):",
						"10",
						ValidateInt(1, 100))
					return inputModel, nil
				}
				
				// For single backup, go directly to execution
				executor := NewBackupExecution(m.config, m.logger, m.parent, m.ctx, m.backupType, m.selected, 0)
				return executor, executor.Init()
			}
		}
	}

	return m, nil
}

func (m DatabaseSelectorModel) View() string {
	var s strings.Builder

	header := titleStyle.Render(m.title)
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	if m.loading {
		s.WriteString("⏳ Loading databases...\n")
		return s.String()
	}

	if m.err != nil {
		s.WriteString(fmt.Sprintf("❌ Error: %v\n", m.err))
		s.WriteString("\nPress ESC to go back\n")
		return s.String()
	}

	s.WriteString("Select a database:\n\n")

	for i, db := range m.databases {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s", cursor, db)))
		} else {
			s.WriteString(fmt.Sprintf("%s %s", cursor, db))
		}
		s.WriteString("\n")
	}

	if m.message != "" {
		s.WriteString(fmt.Sprintf("\n%s\n", m.message))
	}

	s.WriteString("\n⌨️  ↑/↓: Navigate • Enter: Select • ESC: Back • q: Quit\n")

	return s.String()
}

func (m DatabaseSelectorModel) GetSelected() string {
	return m.selected
}
