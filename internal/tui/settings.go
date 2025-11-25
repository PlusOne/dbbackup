package tui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("250")).Padding(1, 2)
	inputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	buttonStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("240")).Padding(0, 2)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("240")).Bold(true)
	detailStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
)

// SettingsModel represents the settings configuration state
type SettingsModel struct {
	config       *config.Config
	logger       logger.Logger
	cursor       int
	editing      bool
	editingField string
	editingValue string
	settings     []SettingItem
	quitting     bool
	message      string
	parent       tea.Model
	dirBrowser   *DirectoryBrowser
	browsingDir  bool
}

// SettingItem represents a configurable setting
type SettingItem struct {
	Key         string
	DisplayName string
	Value       func(*config.Config) string
	Update      func(*config.Config, string) error
	Type        string // "string", "int", "bool", "path"
	Description string
}

// Initialize settings model
func NewSettingsModel(cfg *config.Config, log logger.Logger, parent tea.Model) SettingsModel {
	settings := []SettingItem{
		{
			Key:         "database_type",
			DisplayName: "Database Type",
			Value:       func(c *config.Config) string { return c.DatabaseType },
			Update: func(c *config.Config, v string) error {
				return c.SetDatabaseType(v)
			},
			Type:        "selector",
			Description: "Target database engine (press Enter to cycle: PostgreSQL → MySQL → MariaDB)",
		},
		{
			Key:         "cpu_workload",
			DisplayName: "CPU Workload Type",
			Value:       func(c *config.Config) string { return c.CPUWorkloadType },
			Update: func(c *config.Config, v string) error {
				workloads := []string{"balanced", "cpu-intensive", "io-intensive"}
				currentIdx := 0
				for i, w := range workloads {
					if c.CPUWorkloadType == w {
						currentIdx = i
						break
					}
				}
				nextIdx := (currentIdx + 1) % len(workloads)
				c.CPUWorkloadType = workloads[nextIdx]
				
				// Recalculate Jobs and DumpJobs based on workload type
				if c.CPUInfo != nil && c.AutoDetectCores {
					switch c.CPUWorkloadType {
					case "cpu-intensive":
						c.Jobs = c.CPUInfo.PhysicalCores * 2
						c.DumpJobs = c.CPUInfo.PhysicalCores
					case "io-intensive":
						c.Jobs = c.CPUInfo.PhysicalCores / 2
						if c.Jobs < 1 {
							c.Jobs = 1
						}
						c.DumpJobs = 2
					default: // balanced
						c.Jobs = c.CPUInfo.PhysicalCores
						c.DumpJobs = c.CPUInfo.PhysicalCores / 2
						if c.DumpJobs < 2 {
							c.DumpJobs = 2
						}
					}
				}
				return nil
			},
			Type:        "selector",
			Description: "CPU workload profile (press Enter to cycle: Balanced → CPU-Intensive → I/O-Intensive)",
		},
		{
			Key:         "backup_dir",
			DisplayName: "Backup Directory",
			Value:       func(c *config.Config) string { return c.BackupDir },
			Update: func(c *config.Config, v string) error {
				if v == "" {
					return fmt.Errorf("backup directory cannot be empty")
				}
				c.BackupDir = filepath.Clean(v)
				return nil
			},
			Type:        "path",
			Description: "Directory where backup files will be stored",
		},
		{
			Key:         "compression_level",
			DisplayName: "Compression Level",
			Value:       func(c *config.Config) string { return fmt.Sprintf("%d", c.CompressionLevel) },
			Update: func(c *config.Config, v string) error {
				val, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("compression level must be a number")
				}
				if val < 0 || val > 9 {
					return fmt.Errorf("compression level must be between 0-9")
				}
				c.CompressionLevel = val
				return nil
			},
			Type:        "int",
			Description: "Compression level (0=fastest, 9=smallest)",
		},
		{
			Key:         "jobs",
			DisplayName: "Parallel Jobs",
			Value:       func(c *config.Config) string { return fmt.Sprintf("%d", c.Jobs) },
			Update: func(c *config.Config, v string) error {
				val, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("jobs must be a number")
				}
				if val < 1 {
					return fmt.Errorf("jobs must be at least 1")
				}
				c.Jobs = val
				return nil
			},
			Type:        "int",
			Description: "Number of parallel jobs for backup operations",
		},
		{
			Key:         "dump_jobs",
			DisplayName: "Dump Jobs",
			Value:       func(c *config.Config) string { return fmt.Sprintf("%d", c.DumpJobs) },
			Update: func(c *config.Config, v string) error {
				val, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("dump jobs must be a number")
				}
				if val < 1 {
					return fmt.Errorf("dump jobs must be at least 1")
				}
				c.DumpJobs = val
				return nil
			},
			Type:        "int",
			Description: "Number of parallel jobs for database dumps",
		},
		{
			Key:         "host",
			DisplayName: "Database Host",
			Value:       func(c *config.Config) string { return c.Host },
			Update: func(c *config.Config, v string) error {
				if v == "" {
					return fmt.Errorf("host cannot be empty")
				}
				c.Host = v
				return nil
			},
			Type:        "string",
			Description: "Database server hostname or IP address",
		},
		{
			Key:         "port",
			DisplayName: "Database Port",
			Value:       func(c *config.Config) string { return fmt.Sprintf("%d", c.Port) },
			Update: func(c *config.Config, v string) error {
				val, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("port must be a number")
				}
				if val < 1 || val > 65535 {
					return fmt.Errorf("port must be between 1-65535")
				}
				c.Port = val
				return nil
			},
			Type:        "int",
			Description: "Database server port number",
		},
		{
			Key:         "user",
			DisplayName: "Database User",
			Value:       func(c *config.Config) string { return c.User },
			Update: func(c *config.Config, v string) error {
				if v == "" {
					return fmt.Errorf("user cannot be empty")
				}
				c.User = v
				return nil
			},
			Type:        "string",
			Description: "Database username for connections",
		},
		{
			Key:         "database",
			DisplayName: "Default Database",
			Value:       func(c *config.Config) string { return c.Database },
			Update: func(c *config.Config, v string) error {
				c.Database = v // Can be empty for cluster operations
				return nil
			},
			Type:        "string",
			Description: "Default database name (optional)",
		},
		{
			Key:         "ssl_mode",
			DisplayName: "SSL Mode",
			Value:       func(c *config.Config) string { return c.SSLMode },
			Update: func(c *config.Config, v string) error {
				validModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}
				for _, mode := range validModes {
					if v == mode {
						c.SSLMode = v
						return nil
					}
				}
				return fmt.Errorf("invalid SSL mode. Valid options: %s", strings.Join(validModes, ", "))
			},
			Type:        "string",
			Description: "SSL connection mode (disable, allow, prefer, require, verify-ca, verify-full)",
		},
		{
			Key:         "auto_detect_cores",
			DisplayName: "Auto Detect CPU Cores",
			Value: func(c *config.Config) string {
				if c.AutoDetectCores {
					return "true"
				} else {
					return "false"
				}
			},
			Update: func(c *config.Config, v string) error {
				val, err := strconv.ParseBool(v)
				if err != nil {
					return fmt.Errorf("must be true or false")
				}
				c.AutoDetectCores = val
				return nil
			},
			Type:        "bool",
			Description: "Automatically detect and optimize for CPU cores",
		},
	}

	return SettingsModel{
		config:   cfg,
		logger:   log,
		settings: settings,
		parent:   parent,
	}
}

// Init initializes the settings model
func (m SettingsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle directory browsing mode
		if m.browsingDir && m.dirBrowser != nil {
			switch msg.String() {
			case "esc":
				m.browsingDir = false
				m.dirBrowser.Hide()
				return m, nil
			case "up", "k":
				m.dirBrowser.Navigate(-1)
				return m, nil
			case "down", "j":
				m.dirBrowser.Navigate(1)
				return m, nil
			case "enter", "right", "l":
				m.dirBrowser.Enter()
				return m, nil
			case "left", "h":
				// Go up one level (same as selecting ".." and entering)
				parentPath := filepath.Dir(m.dirBrowser.CurrentPath)
				if parentPath != m.dirBrowser.CurrentPath { // Avoid infinite loop at root
					m.dirBrowser.CurrentPath = parentPath
					m.dirBrowser.LoadItems()
				}
				return m, nil
			case " ":
				// Select current directory
				selectedPath := m.dirBrowser.Select()
				if m.cursor < len(m.settings) {
					setting := m.settings[m.cursor]
					if err := setting.Update(m.config, selectedPath); err != nil {
						m.message = "❌ Error: " + err.Error()
					} else {
						m.message = "✅ Directory updated: " + selectedPath
					}
				}
				m.browsingDir = false
				m.dirBrowser.Hide()
				return m, nil
			case "tab":
				// Toggle back to settings
				m.browsingDir = false
				m.dirBrowser.Hide()
				return m, nil
			}
			return m, nil
		}

		if m.editing {
			return m.handleEditingInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m.parent, nil

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.settings)-1 {
				m.cursor++
			}

		case "enter", " ":
			// For database_type, cycle through options instead of typing
			if m.cursor >= 0 && m.cursor < len(m.settings) && m.settings[m.cursor].Key == "database_type" {
				return m.cycleDatabaseType()
			}
			return m.startEditing()

		case "tab":
			// Directory browser for path fields
			if m.cursor >= 0 && m.cursor < len(m.settings) {
				if m.settings[m.cursor].Type == "path" {
					return m.openDirectoryBrowser()
				} else {
					m.message = "❌ Tab key only works on directory path fields"
					return m, nil
				}
			} else {
				m.message = "❌ Invalid selection"
				return m, nil
			}

		case "r":
			return m.resetToDefaults()

		case "s":
			return m.saveSettings()
		}
	}

	return m, nil
}

// handleEditingInput handles input when editing a setting
func (m SettingsModel) handleEditingInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m.parent, nil

	case "esc":
		m.editing = false
		m.editingField = ""
		m.editingValue = ""
		m.message = ""
		return m, nil

	case "enter":
		return m.saveEditedValue()

	case "backspace", "ctrl+h":
		if len(m.editingValue) > 0 {
			m.editingValue = m.editingValue[:len(m.editingValue)-1]
		}

	default:
		// Add character to editing value
		if len(msg.String()) == 1 {
			m.editingValue += msg.String()
		}
	}

	return m, nil
}

// startEditing begins editing a setting
func (m SettingsModel) startEditing() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.settings) {
		return m, nil
	}

	setting := m.settings[m.cursor]
	m.editing = true
	m.editingField = setting.Key
	m.editingValue = setting.Value(m.config)
	m.message = ""

	return m, nil
}

// saveEditedValue saves the currently edited value
func (m SettingsModel) saveEditedValue() (tea.Model, tea.Cmd) {
	if m.editingField == "" {
		return m, nil
	}

	// Find the setting being edited
	var setting *SettingItem
	for i := range m.settings {
		if m.settings[i].Key == m.editingField {
			setting = &m.settings[i]
			break
		}
	}

	if setting == nil {
		m.message = errorStyle.Render("❌ Setting not found")
		m.editing = false
		return m, nil
	}

	// Update the configuration
	if err := setting.Update(m.config, m.editingValue); err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ %s", err.Error()))
		return m, nil
	}

	m.message = successStyle.Render(fmt.Sprintf("✅ Updated %s", setting.DisplayName))
	m.editing = false
	m.editingField = ""
	m.editingValue = ""

	return m, nil
}

// resetToDefaults resets configuration to default values
func (m SettingsModel) resetToDefaults() (tea.Model, tea.Cmd) {
	newConfig := config.New()

	// Copy important connection details
	newConfig.Host = m.config.Host
	newConfig.Port = m.config.Port
	newConfig.User = m.config.User
	newConfig.Database = m.config.Database
	newConfig.DatabaseType = m.config.DatabaseType

	*m.config = *newConfig
	m.message = successStyle.Render("✅ Settings reset to defaults")

	return m, nil
}

// saveSettings validates and saves current settings
func (m SettingsModel) saveSettings() (tea.Model, tea.Cmd) {
	if err := m.config.Validate(); err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Validation failed: %s", err.Error()))
		return m, nil
	}

	// Optimize CPU settings if auto-detect is enabled
	if m.config.AutoDetectCores {
		if err := m.config.OptimizeForCPU(); err != nil {
			m.message = errorStyle.Render(fmt.Sprintf("❌ CPU optimization failed: %s", err.Error()))
			return m, nil
		}
	}

	m.message = successStyle.Render("✅ Settings validated and saved")
	return m, nil
}

// cycleDatabaseType cycles through database type options
func (m SettingsModel) cycleDatabaseType() (tea.Model, tea.Cmd) {
	dbTypes := []string{"postgres", "mysql", "mariadb"}
	
	// Find current index
	currentIdx := 0
	for i, dbType := range dbTypes {
		if m.config.DatabaseType == dbType {
			currentIdx = i
			break
		}
	}
	
	// Cycle to next
	nextIdx := (currentIdx + 1) % len(dbTypes)
	newType := dbTypes[nextIdx]
	
	// Update config
	if err := m.config.SetDatabaseType(newType); err != nil {
		m.message = errorStyle.Render(fmt.Sprintf("❌ Failed to set database type: %s", err.Error()))
		return m, nil
	}
	
	m.message = successStyle.Render(fmt.Sprintf("✅ Database type set to %s", m.config.DisplayDatabaseType()))
	return m, nil
}

// View renders the settings interface
func (m SettingsModel) View() string {
	if m.quitting {
		return "Returning to main menu...\n"
	}

	var b strings.Builder

	// Header
	header := titleStyle.Render("⚙️  Configuration Settings")
	b.WriteString(fmt.Sprintf("\n%s\n\n", header))

	// Settings list
	for i, setting := range m.settings {
		cursor := " "
		value := setting.Value(m.config)
		displayValue := value
		if setting.Key == "database_type" {
			displayValue = fmt.Sprintf("%s (%s)", value, m.config.DisplayDatabaseType())
		}

		if m.cursor == i {
			cursor = ">"
			if m.editing && m.editingField == setting.Key {
				// Show editing interface
				editValue := m.editingValue
				if setting.Type == "bool" {
					editValue += " (true/false)"
				}
				line := fmt.Sprintf("%s %s: %s", cursor, setting.DisplayName, editValue)
				b.WriteString(selectedStyle.Render(line))
				b.WriteString(" ✏️")
			} else {
				line := fmt.Sprintf("%s %s: %s", cursor, setting.DisplayName, displayValue)
				b.WriteString(selectedStyle.Render(line))
			}
		} else {
			line := fmt.Sprintf("%s %s: %s", cursor, setting.DisplayName, displayValue)
			b.WriteString(menuStyle.Render(line))
		}
		b.WriteString("\n")

		// Show description for selected item
		if m.cursor == i && !m.editing {
			desc := detailStyle.Render(fmt.Sprintf("    %s", setting.Description))
			b.WriteString(desc)
			b.WriteString("\n")
		}

		// Show directory browser for current path field
		if m.cursor == i && m.browsingDir && m.dirBrowser != nil && setting.Type == "path" {
			b.WriteString("\n")
			browserView := m.dirBrowser.Render()
			b.WriteString(browserView)
			b.WriteString("\n")
		}
	}

	// Message area
	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(m.message)
		b.WriteString("\n")
	}

	// Current configuration summary
	if !m.editing {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render("📋 Current Configuration:"))
		b.WriteString("\n")

		summary := []string{
			fmt.Sprintf("Target DB: %s (%s)", m.config.DisplayDatabaseType(), m.config.DatabaseType),
			fmt.Sprintf("Database: %s@%s:%d", m.config.User, m.config.Host, m.config.Port),
			fmt.Sprintf("Backup Dir: %s", m.config.BackupDir),
			fmt.Sprintf("Compression: Level %d", m.config.CompressionLevel),
			fmt.Sprintf("Jobs: %d parallel, %d dump", m.config.Jobs, m.config.DumpJobs),
		}

		for _, line := range summary {
			b.WriteString(detailStyle.Render(fmt.Sprintf("  %s", line)))
			b.WriteString("\n")
		}
	}

	// Footer with instructions
	var footer string
	if m.editing {
		footer = infoStyle.Render("\n⌨️  Type new value • Enter to save • Esc to cancel")
	} else {
		if m.browsingDir {
			footer = infoStyle.Render("\n⌨️  ↑/↓ navigate directories • Enter open • Space select • Tab/Esc back to settings")
		} else {
			// Show different help based on current selection
			if m.cursor >= 0 && m.cursor < len(m.settings) && m.settings[m.cursor].Type == "path" {
				footer = infoStyle.Render("\n⌨️  ↑/↓ navigate • Enter edit • Tab browse directories • 's' save • 'r' reset • 'q' menu")
			} else {
				footer = infoStyle.Render("\n⌨️  ↑/↓ navigate • Enter edit • 's' save • 'r' reset • 'q' menu • Tab=dirs on path fields only")
			}
		}
	}
	b.WriteString(footer)

	return b.String()
}

func (m SettingsModel) openDirectoryBrowser() (tea.Model, tea.Cmd) {
	if m.cursor >= len(m.settings) {
		return m, nil
	}

	setting := m.settings[m.cursor]
	currentValue := setting.Value(m.config)
	if currentValue == "" {
		currentValue = "/tmp"
	}

	if m.dirBrowser == nil {
		m.dirBrowser = NewDirectoryBrowser(currentValue)
	} else {
		// Update the browser to start from the current value
		m.dirBrowser.CurrentPath = currentValue
		m.dirBrowser.LoadItems()
	}

	m.dirBrowser.Show()
	m.browsingDir = true

	return m, nil
}

// RunSettingsMenu starts the settings configuration interface
func RunSettingsMenu(cfg *config.Config, log logger.Logger, parent tea.Model) error {
	m := NewSettingsModel(cfg, log, parent)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running settings menu: %w", err)
	}

	return nil
}
