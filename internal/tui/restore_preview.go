package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"dbbackup/internal/restore"
)

var (
	previewBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(1, 2)

	checkPassedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2"))

	checkFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1"))

	checkWarningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3"))

	checkPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
)

// SafetyCheck represents a pre-restore safety check
type SafetyCheck struct {
	Name     string
	Status   string // "pending", "checking", "passed", "failed", "warning"
	Message  string
	Critical bool
}

// RestorePreviewModel shows restore preview and safety checks
type RestorePreviewModel struct {
	config       *config.Config
	logger       logger.Logger
	parent       tea.Model
	ctx          context.Context
	archive      ArchiveInfo
	mode         string
	targetDB     string
	cleanFirst   bool
	createIfMissing bool
	cleanClusterFirst bool // For cluster restore: drop all user databases first
	existingDBCount  int    // Number of existing user databases
	existingDBs      []string // List of existing user databases
	safetyChecks []SafetyCheck
	checking     bool
	canProceed   bool
	message      string
}

// NewRestorePreview creates a new restore preview
func NewRestorePreview(cfg *config.Config, log logger.Logger, parent tea.Model, ctx context.Context, archive ArchiveInfo, mode string) RestorePreviewModel {
	// Default target database name from archive
	targetDB := archive.DatabaseName
	if targetDB == "" {
		targetDB = cfg.Database
	}

	return RestorePreviewModel{
		config:       cfg,
		logger:       log,
		parent:       parent,
		ctx:          ctx,
		archive:      archive,
		mode:         mode,
		targetDB:     targetDB,
		cleanFirst:   false,
		createIfMissing: true,
		checking:     true,
		safetyChecks: []SafetyCheck{
			{Name: "Archive integrity", Status: "pending", Critical: true},
			{Name: "Disk space", Status: "pending", Critical: true},
			{Name: "Required tools", Status: "pending", Critical: true},
			{Name: "Target database", Status: "pending", Critical: false},
		},
	}
}

func (m RestorePreviewModel) Init() tea.Cmd {
	return runSafetyChecks(m.config, m.logger, m.archive, m.targetDB)
}

type safetyCheckCompleteMsg struct {
	checks          []SafetyCheck
	canProceed      bool
	existingDBCount int
	existingDBs     []string
}

func runSafetyChecks(cfg *config.Config, log logger.Logger, archive ArchiveInfo, targetDB string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		safety := restore.NewSafety(cfg, log)
		checks := []SafetyCheck{}
		canProceed := true

		// 1. Archive integrity
		check := SafetyCheck{Name: "Archive integrity", Status: "checking", Critical: true}
		if err := safety.ValidateArchive(archive.Path); err != nil {
			check.Status = "failed"
			check.Message = err.Error()
			canProceed = false
		} else {
			check.Status = "passed"
			check.Message = "Valid backup archive"
		}
		checks = append(checks, check)

		// 2. Disk space
		check = SafetyCheck{Name: "Disk space", Status: "checking", Critical: true}
		multiplier := 3.0
		if archive.Format.IsClusterBackup() {
			multiplier = 4.0
		}
		if err := safety.CheckDiskSpace(archive.Path, multiplier); err != nil {
			check.Status = "warning"
			check.Message = err.Error()
			// Not critical - just warning
		} else {
			check.Status = "passed"
			check.Message = "Sufficient space available"
		}
		checks = append(checks, check)

		// 3. Required tools
		check = SafetyCheck{Name: "Required tools", Status: "checking", Critical: true}
		dbType := "postgres"
		if archive.Format.IsMySQL() {
			dbType = "mysql"
		}
		if err := safety.VerifyTools(dbType); err != nil {
			check.Status = "failed"
			check.Message = err.Error()
			canProceed = false
		} else {
			check.Status = "passed"
			check.Message = "All required tools available"
		}
		checks = append(checks, check)

		// 4. Target database check (skip for cluster restores)
		existingDBCount := 0
		existingDBs := []string{}
		
		if !archive.Format.IsClusterBackup() {
			check = SafetyCheck{Name: "Target database", Status: "checking", Critical: false}
			exists, err := safety.CheckDatabaseExists(ctx, targetDB)
			if err != nil {
				check.Status = "warning"
				check.Message = fmt.Sprintf("Cannot check: %v", err)
			} else if exists {
				check.Status = "warning"
				check.Message = fmt.Sprintf("Database '%s' exists - will be overwritten if clean-first enabled", targetDB)
			} else {
				check.Status = "passed"
				check.Message = fmt.Sprintf("Database '%s' does not exist - will be created", targetDB)
			}
			checks = append(checks, check)
		} else {
			// For cluster restores, detect existing user databases
			check = SafetyCheck{Name: "Existing databases", Status: "checking", Critical: false}
			
			// Get list of existing user databases (exclude templates and system DBs)
			dbList, err := safety.ListUserDatabases(ctx)
			if err != nil {
				check.Status = "warning"
				check.Message = fmt.Sprintf("Cannot list databases: %v", err)
			} else {
				existingDBCount = len(dbList)
				existingDBs = dbList
				
				if existingDBCount > 0 {
					check.Status = "warning"
					check.Message = fmt.Sprintf("Found %d existing user database(s) - can be cleaned before restore", existingDBCount)
				} else {
					check.Status = "passed"
					check.Message = "No existing user databases - clean slate"
				}
			}
			checks = append(checks, check)
		}

		return safetyCheckCompleteMsg{
			checks:          checks,
			canProceed:      canProceed,
			existingDBCount: existingDBCount,
			existingDBs:     existingDBs,
		}
	}
}

func (m RestorePreviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case safetyCheckCompleteMsg:
		m.checking = false
		m.safetyChecks = msg.checks
		m.canProceed = msg.canProceed
		m.existingDBCount = msg.existingDBCount
		m.existingDBs = msg.existingDBs
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m.parent, nil

		case "t":
			// Toggle clean-first
			m.cleanFirst = !m.cleanFirst
			m.message = fmt.Sprintf("Clean-first: %v", m.cleanFirst)

		case "c":
			if m.mode == "restore-cluster" {
				// Toggle cluster cleanup
				m.cleanClusterFirst = !m.cleanClusterFirst
				if m.cleanClusterFirst {
					m.message = checkWarningStyle.Render(fmt.Sprintf("⚠️  Will drop %d existing database(s) before restore", m.existingDBCount))
				} else {
					m.message = fmt.Sprintf("Clean cluster first: disabled")
				}
			} else {
				// Toggle create if missing
				m.createIfMissing = !m.createIfMissing
				m.message = fmt.Sprintf("Create if missing: %v", m.createIfMissing)
			}

		case "enter", " ":
			if m.checking {
				m.message = "Please wait for safety checks to complete..."
				return m, nil
			}

			if !m.canProceed {
				m.message = errorStyle.Render("❌ Cannot proceed - critical safety checks failed")
				return m, nil
			}

			// Proceed to restore execution
			exec := NewRestoreExecution(m.config, m.logger, m.parent, m.ctx, m.archive, m.targetDB, m.cleanFirst, m.createIfMissing, m.mode, m.cleanClusterFirst, m.existingDBs)
			return exec, exec.Init()
		}
	}

	return m, nil
}

func (m RestorePreviewModel) View() string {
	var s strings.Builder

	// Title
	title := "🔍 Restore Preview"
	if m.mode == "restore-cluster" {
		title = "🔍 Cluster Restore Preview"
	}
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n\n")

	// Archive Information
	s.WriteString(archiveHeaderStyle.Render("📦 Archive Information"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("  File: %s\n", m.archive.Name))
	s.WriteString(fmt.Sprintf("  Format: %s\n", m.archive.Format.String()))
	s.WriteString(fmt.Sprintf("  Size: %s\n", formatSize(m.archive.Size)))
	s.WriteString(fmt.Sprintf("  Created: %s\n", m.archive.Modified.Format("2006-01-02 15:04:05")))
	if m.archive.DatabaseName != "" {
		s.WriteString(fmt.Sprintf("  Database: %s\n", m.archive.DatabaseName))
	}
	s.WriteString("\n")

	// Target Information
	if m.mode == "restore-single" {
		s.WriteString(archiveHeaderStyle.Render("🎯 Target Information"))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("  Database: %s\n", m.targetDB))
		s.WriteString(fmt.Sprintf("  Host: %s:%d\n", m.config.Host, m.config.Port))
		
		cleanIcon := "✗"
		if m.cleanFirst {
			cleanIcon = "✓"
		}
		s.WriteString(fmt.Sprintf("  Clean First: %s %v\n", cleanIcon, m.cleanFirst))
		
		createIcon := "✗"
		if m.createIfMissing {
			createIcon = "✓"
		}
		s.WriteString(fmt.Sprintf("  Create If Missing: %s %v\n", createIcon, m.createIfMissing))
		s.WriteString("\n")
	} else if m.mode == "restore-cluster" {
		s.WriteString(archiveHeaderStyle.Render("🎯 Cluster Restore Options"))
		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("  Host: %s:%d\n", m.config.Host, m.config.Port))
		
		if m.existingDBCount > 0 {
			s.WriteString(fmt.Sprintf("  Existing Databases: %d found\n", m.existingDBCount))
			
			// Show first few database names
			maxShow := 5
			for i, db := range m.existingDBs {
				if i >= maxShow {
					remaining := len(m.existingDBs) - maxShow
					s.WriteString(fmt.Sprintf("    ... and %d more\n", remaining))
					break
				}
				s.WriteString(fmt.Sprintf("    - %s\n", db))
			}
			
			cleanIcon := "✗"
			cleanStyle := infoStyle
			if m.cleanClusterFirst {
				cleanIcon = "✓"
				cleanStyle = checkWarningStyle
			}
			s.WriteString(cleanStyle.Render(fmt.Sprintf("  Clean All First: %s %v (press 'c' to toggle)\n", cleanIcon, m.cleanClusterFirst)))
		} else {
			s.WriteString("  Existing Databases: None (clean slate)\n")
		}
		s.WriteString("\n")
	}

	// Safety Checks
	s.WriteString(archiveHeaderStyle.Render("🛡️  Safety Checks"))
	s.WriteString("\n")

	if m.checking {
		s.WriteString(infoStyle.Render("  Running safety checks..."))
		s.WriteString("\n")
	} else {
		for _, check := range m.safetyChecks {
			icon := "○"
			style := checkPendingStyle
			
			switch check.Status {
			case "passed":
				icon = "✓"
				style = checkPassedStyle
			case "failed":
				icon = "✗"
				style = checkFailedStyle
			case "warning":
				icon = "⚠"
				style = checkWarningStyle
			case "checking":
				icon = "⟳"
				style = checkPendingStyle
			}

			line := fmt.Sprintf("  %s %s", icon, check.Name)
			if check.Message != "" {
				line += fmt.Sprintf(" ... %s", check.Message)
			}
			s.WriteString(style.Render(line))
			s.WriteString("\n")
		}
	}
	s.WriteString("\n")

	// Warnings
	if m.cleanFirst {
		s.WriteString(checkWarningStyle.Render("⚠️  Warning: Clean-first enabled"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("   All existing data in target database will be dropped!"))
		s.WriteString("\n\n")
	}
	if m.cleanClusterFirst && m.existingDBCount > 0 {
		s.WriteString(checkWarningStyle.Render("🔥 WARNING: Cluster cleanup enabled"))
		s.WriteString("\n")
		s.WriteString(checkWarningStyle.Render(fmt.Sprintf("   %d existing database(s) will be DROPPED before restore!", m.existingDBCount)))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("   This ensures a clean disaster recovery scenario"))
		s.WriteString("\n\n")
	}

	// Message
	if m.message != "" {
		s.WriteString(m.message)
		s.WriteString("\n\n")
	}

	// Footer
	if m.checking {
		s.WriteString(infoStyle.Render("⌨️  Please wait..."))
	} else if m.canProceed {
		s.WriteString(successStyle.Render("✅ Ready to restore"))
		s.WriteString("\n")
		if m.mode == "restore-single" {
			s.WriteString(infoStyle.Render("⌨️  t: Toggle clean-first | c: Toggle create | Enter: Proceed | Esc: Cancel"))
		} else if m.mode == "restore-cluster" {
			if m.existingDBCount > 0 {
				s.WriteString(infoStyle.Render("⌨️  c: Toggle cleanup | Enter: Proceed | Esc: Cancel"))
			} else {
				s.WriteString(infoStyle.Render("⌨️  Enter: Proceed | Esc: Cancel"))
			}
		} else {
			s.WriteString(infoStyle.Render("⌨️  Enter: Proceed | Esc: Cancel"))
		}
	} else {
		s.WriteString(errorStyle.Render("❌ Cannot proceed - please fix errors above"))
		s.WriteString("\n")
		s.WriteString(infoStyle.Render("⌨️  Esc: Go back"))
	}

	return s.String()
}
