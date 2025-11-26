package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	"dbbackup/internal/pitr"
	"dbbackup/internal/wal"
)

// TestRecoveryTargetValidation tests recovery target parsing and validation
func TestRecoveryTargetValidation(t *testing.T) {
	tests := []struct {
		name        string
		targetTime  string
		targetXID   string
		targetLSN   string
		targetName  string
		immediate   bool
		action      string
		timeline    string
		inclusive   bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid time target",
			targetTime:  "2024-11-26 12:00:00",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: false,
		},
		{
			name:        "Valid XID target",
			targetXID:   "1000000",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: false,
		},
		{
			name:        "Valid LSN target",
			targetLSN:   "0/3000000",
			action:      "pause",
			timeline:    "latest",
			inclusive:   false,
			expectError: false,
		},
		{
			name:        "Valid name target",
			targetName:  "my_restore_point",
			action:      "promote",
			timeline:    "2",
			inclusive:   true,
			expectError: false,
		},
		{
			name:        "Valid immediate target",
			immediate:   true,
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: false,
		},
		{
			name:        "No target specified",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "no recovery target specified",
		},
		{
			name:        "Multiple targets",
			targetTime:  "2024-11-26 12:00:00",
			targetXID:   "1000000",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "multiple recovery targets",
		},
		{
			name:        "Invalid time format",
			targetTime:  "invalid-time",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "invalid timestamp format",
		},
		{
			name:        "Invalid XID (negative)",
			targetXID:   "-1000",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "invalid transaction ID",
		},
		{
			name:        "Invalid LSN format",
			targetLSN:   "invalid-lsn",
			action:      "promote",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "invalid LSN format",
		},
		{
			name:        "Invalid action",
			targetTime:  "2024-11-26 12:00:00",
			action:      "invalid",
			timeline:    "latest",
			inclusive:   true,
			expectError: true,
			errorMsg:    "invalid recovery action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := pitr.ParseRecoveryTarget(
				tt.targetTime,
				tt.targetXID,
				tt.targetLSN,
				tt.targetName,
				tt.immediate,
				tt.action,
				tt.timeline,
				tt.inclusive,
			)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if target == nil {
					t.Error("Expected target, got nil")
				}
			}
		})
	}
}

// TestRecoveryTargetToConfig tests conversion to PostgreSQL config
func TestRecoveryTargetToConfig(t *testing.T) {
	tests := []struct {
		name           string
		target         *pitr.RecoveryTarget
		expectedKeys   []string
		expectedValues map[string]string
	}{
		{
			name: "Time target",
			target: &pitr.RecoveryTarget{
				Type:      "time",
				Value:     "2024-11-26 12:00:00",
				Action:    "promote",
				Timeline:  "latest",
				Inclusive: true,
			},
			expectedKeys: []string{"recovery_target_time", "recovery_target_action", "recovery_target_timeline", "recovery_target_inclusive"},
			expectedValues: map[string]string{
				"recovery_target_time":      "2024-11-26 12:00:00",
				"recovery_target_action":    "promote",
				"recovery_target_timeline":  "latest",
				"recovery_target_inclusive": "true",
			},
		},
		{
			name: "XID target",
			target: &pitr.RecoveryTarget{
				Type:      "xid",
				Value:     "1000000",
				Action:    "pause",
				Timeline:  "2",
				Inclusive: false,
			},
			expectedKeys: []string{"recovery_target_xid", "recovery_target_action", "recovery_target_timeline", "recovery_target_inclusive"},
			expectedValues: map[string]string{
				"recovery_target_xid":       "1000000",
				"recovery_target_action":    "pause",
				"recovery_target_timeline":  "2",
				"recovery_target_inclusive": "false",
			},
		},
		{
			name: "Immediate target",
			target: &pitr.RecoveryTarget{
				Type:     "immediate",
				Value:    "immediate",
				Action:   "promote",
				Timeline: "latest",
			},
			expectedKeys: []string{"recovery_target", "recovery_target_action", "recovery_target_timeline"},
			expectedValues: map[string]string{
				"recovery_target":          "immediate",
				"recovery_target_action":   "promote",
				"recovery_target_timeline": "latest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.target.ToPostgreSQLConfig()

			// Check all expected keys are present
			for _, key := range tt.expectedKeys {
				if _, exists := config[key]; !exists {
					t.Errorf("Missing expected key: %s", key)
				}
			}

			// Check expected values
			for key, expectedValue := range tt.expectedValues {
				if actualValue, exists := config[key]; !exists {
					t.Errorf("Missing key: %s", key)
				} else if actualValue != expectedValue {
					t.Errorf("Key %s: expected '%s', got '%s'", key, expectedValue, actualValue)
				}
			}
		})
	}
}

// TestWALArchiving tests WAL file archiving
func TestWALArchiving(t *testing.T) {
	// Create temp directories
	tempDir := t.TempDir()
	walArchiveDir := filepath.Join(tempDir, "wal_archive")
	if err := os.MkdirAll(walArchiveDir, 0700); err != nil {
		t.Fatalf("Failed to create WAL archive dir: %v", err)
	}

	// Create a mock WAL file
	walDir := filepath.Join(tempDir, "wal")
	if err := os.MkdirAll(walDir, 0700); err != nil {
		t.Fatalf("Failed to create WAL dir: %v", err)
	}
	
	walFileName := "000000010000000000000001"
	walFilePath := filepath.Join(walDir, walFileName)
	walContent := []byte("mock WAL file content for testing")
	if err := os.WriteFile(walFilePath, walContent, 0600); err != nil {
		t.Fatalf("Failed to create mock WAL file: %v", err)
	}

	// Create archiver
	cfg := &config.Config{}
	log := logger.New("info", "text")
	archiver := wal.NewArchiver(cfg, log)

	// Test plain archiving
	t.Run("Plain archiving", func(t *testing.T) {
		archiveConfig := wal.ArchiveConfig{
			ArchiveDir:  walArchiveDir,
			CompressWAL: false,
			EncryptWAL:  false,
		}

		ctx := context.Background()
		info, err := archiver.ArchiveWALFile(ctx, walFilePath, walFileName, archiveConfig)
		if err != nil {
			t.Fatalf("Archiving failed: %v", err)
		}

		if info.WALFileName != walFileName {
			t.Errorf("Expected WAL filename %s, got %s", walFileName, info.WALFileName)
		}

		if info.OriginalSize != int64(len(walContent)) {
			t.Errorf("Expected size %d, got %d", len(walContent), info.OriginalSize)
		}

		// Verify archived file exists
		archivedPath := filepath.Join(walArchiveDir, walFileName)
		if _, err := os.Stat(archivedPath); err != nil {
			t.Errorf("Archived file not found: %v", err)
		}
	})

	// Test compressed archiving
	t.Run("Compressed archiving", func(t *testing.T) {
		walFileName2 := "000000010000000000000002"
		walFilePath2 := filepath.Join(walDir, walFileName2)
		if err := os.WriteFile(walFilePath2, walContent, 0600); err != nil {
			t.Fatalf("Failed to create mock WAL file: %v", err)
		}

		archiveConfig := wal.ArchiveConfig{
			ArchiveDir:  walArchiveDir,
			CompressWAL: true,
			EncryptWAL:  false,
		}

		ctx := context.Background()
		info, err := archiver.ArchiveWALFile(ctx, walFilePath2, walFileName2, archiveConfig)
		if err != nil {
			t.Fatalf("Compressed archiving failed: %v", err)
		}

		if !info.Compressed {
			t.Error("Expected compressed flag to be true")
		}

		// Verify compressed file exists
		archivedPath := filepath.Join(walArchiveDir, walFileName2+".gz")
		if _, err := os.Stat(archivedPath); err != nil {
			t.Errorf("Compressed archived file not found: %v", err)
		}
	})
}

// TestWALParsing tests WAL filename parsing
func TestWALParsing(t *testing.T) {
	tests := []struct {
		name             string
		walFileName      string
		expectedTimeline uint32
		expectedSegment  uint64
		expectError      bool
	}{
		{
			name:             "Valid WAL filename",
			walFileName:      "000000010000000000000001",
			expectedTimeline: 1,
			expectedSegment:  1,
			expectError:      false,
		},
		{
			name:             "Timeline 2",
			walFileName:      "000000020000000000000005",
			expectedTimeline: 2,
			expectedSegment:  5,
			expectError:      false,
		},
		{
			name:             "High segment number",
			walFileName:      "00000001000000000000FFFF",
			expectedTimeline: 1,
			expectedSegment:  0xFFFF,
			expectError:      false,
		},
		{
			name:        "Too short",
			walFileName: "00000001",
			expectError: true,
		},
		{
			name:        "Invalid hex",
			walFileName: "GGGGGGGGGGGGGGGGGGGGGGGG",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeline, segment, err := wal.ParseWALFileName(tt.walFileName)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if timeline != tt.expectedTimeline {
					t.Errorf("Expected timeline %d, got %d", tt.expectedTimeline, timeline)
				}
				if segment != tt.expectedSegment {
					t.Errorf("Expected segment %d, got %d", tt.expectedSegment, segment)
				}
			}
		})
	}
}

// TestTimelineManagement tests timeline parsing and validation
func TestTimelineManagement(t *testing.T) {
	// Create temp directory with mock timeline files
	tempDir := t.TempDir()

	// Create timeline history files
	history2 := "1\t0/3000000\tno recovery target specified\n"
	if err := os.WriteFile(filepath.Join(tempDir, "00000002.history"), []byte(history2), 0600); err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}

	history3 := "2\t0/5000000\trecovery target reached\n"
	if err := os.WriteFile(filepath.Join(tempDir, "00000003.history"), []byte(history3), 0600); err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}

	// Create mock WAL files
	walFiles := []string{
		"000000010000000000000001",
		"000000010000000000000002",
		"000000020000000000000001",
		"000000030000000000000001",
	}
	for _, walFile := range walFiles {
		if err := os.WriteFile(filepath.Join(tempDir, walFile), []byte("mock"), 0600); err != nil {
			t.Fatalf("Failed to create WAL file: %v", err)
		}
	}

	// Create timeline manager
	log := logger.New("info", "text")
	tm := wal.NewTimelineManager(log)

	// Parse timeline history
	ctx := context.Background()
	history, err := tm.ParseTimelineHistory(ctx, tempDir)
	if err != nil {
		t.Fatalf("Failed to parse timeline history: %v", err)
	}

	// Validate timeline count
	if len(history.Timelines) < 3 {
		t.Errorf("Expected at least 3 timelines, got %d", len(history.Timelines))
	}

	// Validate timeline 2
	tl2, exists := history.TimelineMap[2]
	if !exists {
		t.Fatal("Timeline 2 not found")
	}
	if tl2.ParentTimeline != 1 {
		t.Errorf("Expected timeline 2 parent to be 1, got %d", tl2.ParentTimeline)
	}
	if tl2.SwitchPoint != "0/3000000" {
		t.Errorf("Expected switch point '0/3000000', got '%s'", tl2.SwitchPoint)
	}

	// Validate timeline 3
	tl3, exists := history.TimelineMap[3]
	if !exists {
		t.Fatal("Timeline 3 not found")
	}
	if tl3.ParentTimeline != 2 {
		t.Errorf("Expected timeline 3 parent to be 2, got %d", tl3.ParentTimeline)
	}

	// Validate consistency
	if err := tm.ValidateTimelineConsistency(ctx, history); err != nil {
		t.Errorf("Timeline consistency validation failed: %v", err)
	}

	// Test timeline path
	path, err := tm.GetTimelinePath(history, 3)
	if err != nil {
		t.Fatalf("Failed to get timeline path: %v", err)
	}
	if len(path) != 3 {
		t.Errorf("Expected timeline path length 3, got %d", len(path))
	}
	if path[0].TimelineID != 1 || path[1].TimelineID != 2 || path[2].TimelineID != 3 {
		t.Error("Timeline path order incorrect")
	}
}

// TestRecoveryConfigGeneration tests recovery configuration file generation
func TestRecoveryConfigGeneration(t *testing.T) {
	tempDir := t.TempDir()

	// Create mock PostgreSQL data directory
	dataDir := filepath.Join(tempDir, "pgdata")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Create PG_VERSION file
	if err := os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("14\n"), 0600); err != nil {
		t.Fatalf("Failed to create PG_VERSION: %v", err)
	}

	log := logger.New("info", "text")
	configGen := pitr.NewRecoveryConfigGenerator(log)

	// Test version detection
	t.Run("Version detection", func(t *testing.T) {
		version, err := configGen.DetectPostgreSQLVersion(dataDir)
		if err != nil {
			t.Fatalf("Version detection failed: %v", err)
		}
		if version != 14 {
			t.Errorf("Expected version 14, got %d", version)
		}
	})

	// Test modern config generation (PG 12+)
	t.Run("Modern config generation", func(t *testing.T) {
		target := &pitr.RecoveryTarget{
			Type:      "time",
			Value:     "2024-11-26 12:00:00",
			Action:    "promote",
			Timeline:  "latest",
			Inclusive: true,
		}

		config := &pitr.RecoveryConfig{
			Target:            target,
			WALArchiveDir:     "/tmp/wal",
			PostgreSQLVersion: 14,
			DataDir:           dataDir,
		}

		err := configGen.GenerateRecoveryConfig(config)
		if err != nil {
			t.Fatalf("Config generation failed: %v", err)
		}

		// Verify recovery.signal exists
		recoverySignal := filepath.Join(dataDir, "recovery.signal")
		if _, err := os.Stat(recoverySignal); err != nil {
			t.Errorf("recovery.signal not created: %v", err)
		}

		// Verify postgresql.auto.conf exists
		autoConf := filepath.Join(dataDir, "postgresql.auto.conf")
		if _, err := os.Stat(autoConf); err != nil {
			t.Errorf("postgresql.auto.conf not created: %v", err)
		}

		// Read and verify content
		content, err := os.ReadFile(autoConf)
		if err != nil {
			t.Fatalf("Failed to read postgresql.auto.conf: %v", err)
		}

		contentStr := string(content)
		if !contains(contentStr, "recovery_target_time") {
			t.Error("Config missing recovery_target_time")
		}
		if !contains(contentStr, "recovery_target_action") {
			t.Error("Config missing recovery_target_action")
		}
	})

	// Test legacy config generation (PG < 12)
	t.Run("Legacy config generation", func(t *testing.T) {
		dataDir11 := filepath.Join(tempDir, "pgdata11")
		if err := os.MkdirAll(dataDir11, 0700); err != nil {
			t.Fatalf("Failed to create data dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dataDir11, "PG_VERSION"), []byte("11\n"), 0600); err != nil {
			t.Fatalf("Failed to create PG_VERSION: %v", err)
		}

		target := &pitr.RecoveryTarget{
			Type:      "xid",
			Value:     "1000000",
			Action:    "pause",
			Timeline:  "latest",
			Inclusive: false,
		}

		config := &pitr.RecoveryConfig{
			Target:            target,
			WALArchiveDir:     "/tmp/wal",
			PostgreSQLVersion: 11,
			DataDir:           dataDir11,
		}

		err := configGen.GenerateRecoveryConfig(config)
		if err != nil {
			t.Fatalf("Legacy config generation failed: %v", err)
		}

		// Verify recovery.conf exists
		recoveryConf := filepath.Join(dataDir11, "recovery.conf")
		if _, err := os.Stat(recoveryConf); err != nil {
			t.Errorf("recovery.conf not created: %v", err)
		}

		// Read and verify content
		content, err := os.ReadFile(recoveryConf)
		if err != nil {
			t.Fatalf("Failed to read recovery.conf: %v", err)
		}

		contentStr := string(content)
		if !contains(contentStr, "recovery_target_xid") {
			t.Error("Config missing recovery_target_xid")
		}
		if !contains(contentStr, "1000000") {
			t.Error("Config missing XID value")
		}
	})
}

// TestDataDirectoryValidation tests data directory validation
func TestDataDirectoryValidation(t *testing.T) {
	log := logger.New("info", "text")
	configGen := pitr.NewRecoveryConfigGenerator(log)

	t.Run("Valid empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, "pgdata")
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			t.Fatalf("Failed to create data dir: %v", err)
		}

		// Create PG_VERSION to make it look like a PG directory
		if err := os.WriteFile(filepath.Join(dataDir, "PG_VERSION"), []byte("14\n"), 0600); err != nil {
			t.Fatalf("Failed to create PG_VERSION: %v", err)
		}

		err := configGen.ValidateDataDirectory(dataDir)
		if err != nil {
			t.Errorf("Validation failed for valid directory: %v", err)
		}
	})

	t.Run("Non-existent directory", func(t *testing.T) {
		err := configGen.ValidateDataDirectory("/nonexistent/path")
		if err == nil {
			t.Error("Expected error for non-existent directory")
		}
	})

	t.Run("PostgreSQL running", func(t *testing.T) {
		tempDir := t.TempDir()
		dataDir := filepath.Join(tempDir, "pgdata_running")
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			t.Fatalf("Failed to create data dir: %v", err)
		}

		// Create postmaster.pid to simulate running PostgreSQL
		if err := os.WriteFile(filepath.Join(dataDir, "postmaster.pid"), []byte("12345\n"), 0600); err != nil {
			t.Fatalf("Failed to create postmaster.pid: %v", err)
		}

		err := configGen.ValidateDataDirectory(dataDir)
		if err == nil {
			t.Error("Expected error for running PostgreSQL")
		}
		if !contains(err.Error(), "currently running") {
			t.Errorf("Expected 'currently running' error, got: %v", err)
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		len(s) > len(substr)+1 && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests
func BenchmarkWALArchiving(b *testing.B) {
	tempDir := b.TempDir()
	walArchiveDir := filepath.Join(tempDir, "wal_archive")
	os.MkdirAll(walArchiveDir, 0700)

	walDir := filepath.Join(tempDir, "wal")
	os.MkdirAll(walDir, 0700)

	// Create a 16MB mock WAL file (typical size)
	walContent := make([]byte, 16*1024*1024)
	walFilePath := filepath.Join(walDir, "000000010000000000000001")
	os.WriteFile(walFilePath, walContent, 0600)

	cfg := &config.Config{}
	log := logger.New("info", "text")
	archiver := wal.NewArchiver(cfg, log)

	archiveConfig := wal.ArchiveConfig{
		ArchiveDir:  walArchiveDir,
		CompressWAL: false,
		EncryptWAL:  false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		archiver.ArchiveWALFile(ctx, walFilePath, "000000010000000000000001", archiveConfig)
	}
}

func BenchmarkRecoveryTargetParsing(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pitr.ParseRecoveryTarget(
			"2024-11-26 12:00:00",
			"",
			"",
			"",
			false,
			"promote",
			"latest",
			true,
		)
	}
}
