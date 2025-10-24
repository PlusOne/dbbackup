package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// InputModel for getting user input
type InputModel struct {
	config   *config.Config
	logger   logger.Logger
	parent   tea.Model
	title    string
	prompt   string
	value    string
	cursor   int
	done     bool
	err      error
	validate func(string) error
}

func NewInputModel(cfg *config.Config, log logger.Logger, parent tea.Model, title, prompt, defaultValue string, validate func(string) error) InputModel {
	return InputModel{
		config:   cfg,
		logger:   log,
		parent:   parent,
		title:    title,
		prompt:   prompt,
		value:    defaultValue,
		validate: validate,
		cursor:   len(defaultValue),
	}
}

func (m InputModel) Init() tea.Cmd {
	return nil
}

func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			// Return to grandparent (menu) not immediate parent (selector)
			if selector, ok := m.parent.(DatabaseSelectorModel); ok {
				return selector.parent, nil
			}
			return m.parent, nil

		case "enter":
			if m.validate != nil {
				if err := m.validate(m.value); err != nil {
					m.err = err
					return m, nil
				}
			}
			m.done = true
			
			// If this is from database selector, execute backup with ratio
			if selector, ok := m.parent.(DatabaseSelectorModel); ok {
				ratio, _ := strconv.Atoi(m.value)
				executor := NewBackupExecution(selector.config, selector.logger, selector.parent, 
					selector.backupType, selector.selected, ratio)
				return executor, executor.Init()
			}
			return m, nil

		case "backspace":
			if len(m.value) > 0 && m.cursor > 0 {
				m.value = m.value[:m.cursor-1] + m.value[m.cursor:]
				m.cursor--
			}

		case "left":
			if m.cursor > 0 {
				m.cursor--
			}

		case "right":
			if m.cursor < len(m.value) {
				m.cursor++
			}

		default:
			// Add character
			if len(msg.String()) == 1 {
				m.value = m.value[:m.cursor] + msg.String() + m.value[m.cursor:]
				m.cursor++
				m.err = nil
			}
		}
	}

	return m, nil
}

func (m InputModel) View() string {
	var s strings.Builder

	header := titleStyle.Render(m.title)
	s.WriteString(fmt.Sprintf("\n%s\n\n", header))

	s.WriteString(fmt.Sprintf("%s\n\n", m.prompt))

	// Show input with cursor
	before := m.value[:m.cursor]
	after := ""
	if m.cursor < len(m.value) {
		after = m.value[m.cursor:]
	}
	s.WriteString(inputStyle.Render(fmt.Sprintf("> %s▎%s", before, after)))
	s.WriteString("\n\n")

	if m.err != nil {
		s.WriteString(errorStyle.Render(fmt.Sprintf("❌ Error: %v\n\n", m.err)))
	}

	s.WriteString("⌨️  Type value • Enter: Confirm • ESC: Cancel\n")

	return s.String()
}

func (m InputModel) GetValue() string {
	return m.value
}

func (m InputModel) GetIntValue() (int, error) {
	return strconv.Atoi(m.value)
}

func (m InputModel) IsDone() bool {
	return m.done
}

// Validation functions
func ValidateInt(min, max int) func(string) error {
	return func(s string) error {
		val, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		if val < min || val > max {
			return fmt.Errorf("must be between %d and %d", min, max)
		}
		return nil
	}
}

func ValidateNotEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("value cannot be empty")
	}
	return nil
}
