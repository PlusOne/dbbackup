package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// ConfirmationModel for yes/no confirmations
type ConfirmationModel struct {
	config    *config.Config
	logger    logger.Logger
	parent    tea.Model
	ctx       context.Context
	title     string
	message   string
	cursor    int
	choices   []string
	confirmed bool
	onConfirm func() (tea.Model, tea.Cmd) // Callback when confirmed
}

func NewConfirmationModel(cfg *config.Config, log logger.Logger, parent tea.Model, title, message string) ConfirmationModel {
	return ConfirmationModel{
		config:  cfg,
		logger:  log,
		parent:  parent,
		title:   title,
		message: message,
		choices: []string{"Yes", "No"},
	}
}

func NewConfirmationModelWithAction(cfg *config.Config, log logger.Logger, parent tea.Model, title, message string, onConfirm func() (tea.Model, tea.Cmd)) ConfirmationModel {
	return ConfirmationModel{
		config:    cfg,
		logger:    log,
		parent:    parent,
		title:     title,
		message:   message,
		choices:   []string{"Yes", "No"},
		onConfirm: onConfirm,
	}
}

func (m ConfirmationModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmationModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "n":
			return m.parent, nil

		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right", "l":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}

		case "enter", "y":
			if msg.String() == "y" || m.cursor == 0 {
				m.confirmed = true
				// Execute the onConfirm callback if provided
				if m.onConfirm != nil {
					return m.onConfirm()
				}
				// Default: execute cluster backup for backward compatibility
				executor := NewBackupExecution(m.config, m.logger, m.parent, m.ctx, "cluster", "", 0)
				return executor, executor.Init()
			}
			return m.parent, nil
		}
	}

	return m, nil
}

func (m ConfirmationModel) View() string {
	var s strings.Builder

	header := titleStyle.Render(m.title)
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	s.WriteString(fmt.Sprintf("%s\n\n", m.message))

	// Show choices
	for i, choice := range m.choices {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			s.WriteString(selectedStyle.Render(fmt.Sprintf("%s [%s]", cursor, choice)))
		} else {
			s.WriteString(fmt.Sprintf("%s [%s]", cursor, choice))
		}
		s.WriteString("  ")
	}

	s.WriteString("\n\n⌨️  ←/→: Select • Enter/y: Confirm • n/ESC: Cancel\n")

	return s.String()
}

func (m ConfirmationModel) IsConfirmed() bool {
	return m.confirmed
}
