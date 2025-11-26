package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dbbackup/internal/logger"
)

// TestIncrementalBackupRestore tests the full incremental backup workflow
func TestIncrementalBackupRestore(t *testing.T) {
	// Create test directories
	tempDir, err := os.MkdirTemp("", "incremental_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dataDir := filepath.Join(tempDir, "pgdata")
	backupDir := filepath.Join(tempDir, "backups")
	restoreDir := filepath.Join(tempDir, "restore")

	// Create directories
	for _, dir := range []string{dataDir, backupDir, restoreDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Initialize logger
	log := logger.New("info", "text")

	// Create incremental engine
	engine := &PostgresIncrementalEngine{
		log: log,
	}

	ctx := context.Background()

	// Step 1: Create test data files (simulate PostgreSQL data directory)
	t.Log("Step 1: Creating test data files...")
	testFiles := map[string]string{
		"base/12345/1234":     "Original table data file",
		"base/12345/1235":     "Another table file",
		"base/12345/1236":     "Third table file",
		"global/pg_control":   "PostgreSQL control file",
		"pg_wal/000000010000": "WAL file (should be excluded)",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(dataDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", relPath, err)
		}
	}

	// Wait a moment to ensure timestamps differ
	time.Sleep(100 * time.Millisecond)

	// Step 2: Create base (full) backup
	t.Log("Step 2: Creating base backup...")
	baseBackupPath := filepath.Join(backupDir, "testdb_base.tar.gz")
	
	// Manually create base backup for testing
	baseConfig := &IncrementalBackupConfig{
		DataDirectory:    dataDir,
		CompressionLevel: 6,
	}

	// Create a simple tar.gz of the data directory (simulating full backup)
	changedFiles := []ChangedFile{}
	err = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(dataDir, path)
		if err != nil {
			return err
		}
		changedFiles = append(changedFiles, ChangedFile{
			RelativePath: relPath,
			AbsolutePath: path,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
		})
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk data directory: %v", err)
	}

	// Create base backup using tar
	if err := engine.createTarGz(ctx, baseBackupPath, changedFiles, baseConfig); err != nil {
		t.Fatalf("Failed to create base backup: %v", err)
	}

	// Calculate checksum for base backup
	baseChecksum, err := engine.CalculateFileChecksum(baseBackupPath)
	if err != nil {
		t.Fatalf("Failed to calculate base backup checksum: %v", err)
	}
	t.Logf("Base backup created: %s (checksum: %s)", baseBackupPath, baseChecksum[:16])

	// Create base backup metadata
	baseStat, _ := os.Stat(baseBackupPath)
	baseMetadata := createTestMetadata("testdb", baseBackupPath, baseStat.Size(), baseChecksum, "full", nil)
	if err := saveTestMetadata(baseBackupPath, baseMetadata); err != nil {
		t.Fatalf("Failed to save base metadata: %v", err)
	}

	// Wait to ensure different timestamps
	time.Sleep(200 * time.Millisecond)

	// Step 3: Modify data files (simulate database changes)
	t.Log("Step 3: Modifying data files...")
	modifiedFiles := map[string]string{
		"base/12345/1234": "MODIFIED table data - incremental will capture this",
		"base/12345/1237": "NEW table file added after base backup",
	}

	for relPath, content := range modifiedFiles {
		fullPath := filepath.Join(dataDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write modified file %s: %v", relPath, err)
		}
	}

	// Wait to ensure different timestamps
	time.Sleep(100 * time.Millisecond)

	// Step 4: Find changed files
	t.Log("Step 4: Finding changed files...")
	incrConfig := &IncrementalBackupConfig{
		BaseBackupPath:   baseBackupPath,
		DataDirectory:    dataDir,
		CompressionLevel: 6,
	}

	changedFilesList, err := engine.FindChangedFiles(ctx, incrConfig)
	if err != nil {
		t.Fatalf("Failed to find changed files: %v", err)
	}

	t.Logf("Found %d changed files", len(changedFilesList))
	if len(changedFilesList) == 0 {
		t.Fatal("Expected changed files but found none")
	}

	// Verify we found the modified files
	foundModified := false
	foundNew := false
	for _, cf := range changedFilesList {
		if cf.RelativePath == "base/12345/1234" {
			foundModified = true
		}
		if cf.RelativePath == "base/12345/1237" {
			foundNew = true
		}
	}

	if !foundModified {
		t.Error("Did not find modified file base/12345/1234")
	}
	if !foundNew {
		t.Error("Did not find new file base/12345/1237")
	}

	// Step 5: Create incremental backup
	t.Log("Step 5: Creating incremental backup...")
	if err := engine.CreateIncrementalBackup(ctx, incrConfig, changedFilesList); err != nil {
		t.Fatalf("Failed to create incremental backup: %v", err)
	}

	// Find the incremental backup (has _incr_ in filename)
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("Failed to read backup directory: %v", err)
	}

	var incrementalBackupPath string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".gz" && 
			entry.Name() != filepath.Base(baseBackupPath) {
			incrementalBackupPath = filepath.Join(backupDir, entry.Name())
			break
		}
	}

	if incrementalBackupPath == "" {
		t.Fatal("Incremental backup file not found")
	}

	t.Logf("Incremental backup created: %s", incrementalBackupPath)

	// Verify incremental backup was created
	incrStat, _ := os.Stat(incrementalBackupPath)
	t.Logf("Base backup size: %d bytes", baseStat.Size())
	t.Logf("Incremental backup size: %d bytes", incrStat.Size())
	
	// Note: For tiny test files, incremental might be larger due to tar.gz overhead
	// In real-world scenarios with larger files, incremental would be much smaller
	t.Logf("Incremental contains %d changed files out of %d total",
		len(changedFilesList), len(testFiles))

	// Step 6: Restore incremental backup
	t.Log("Step 6: Restoring incremental backup...")
	if err := engine.RestoreIncremental(ctx, baseBackupPath, incrementalBackupPath, restoreDir); err != nil {
		t.Fatalf("Failed to restore incremental backup: %v", err)
	}

	// Step 7: Verify restored files
	t.Log("Step 7: Verifying restored files...")
	for relPath, expectedContent := range modifiedFiles {
		restoredPath := filepath.Join(restoreDir, relPath)
		content, err := os.ReadFile(restoredPath)
		if err != nil {
			t.Errorf("Failed to read restored file %s: %v", relPath, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("File %s content mismatch:\nExpected: %s\nGot: %s",
				relPath, expectedContent, string(content))
		}
	}

	// Verify unchanged files still exist
	unchangedFile := filepath.Join(restoreDir, "base/12345/1235")
	if _, err := os.Stat(unchangedFile); err != nil {
		t.Errorf("Unchanged file base/12345/1235 not found in restore: %v", err)
	}

	t.Log("✅ Incremental backup and restore test completed successfully")
}

// TestIncrementalBackupErrors tests error handling
func TestIncrementalBackupErrors(t *testing.T) {
	log := logger.New("info", "text")
	engine := &PostgresIncrementalEngine{log: log}
	ctx := context.Background()

	tempDir, err := os.MkdirTemp("", "incremental_error_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	t.Run("Missing base backup", func(t *testing.T) {
		config := &IncrementalBackupConfig{
			BaseBackupPath:   filepath.Join(tempDir, "nonexistent.tar.gz"),
			DataDirectory:    tempDir,
			CompressionLevel: 6,
		}
		_, err := engine.FindChangedFiles(ctx, config)
		if err == nil {
			t.Error("Expected error for missing base backup, got nil")
		}
	})

	t.Run("No changed files", func(t *testing.T) {
		// Create a dummy base backup
		baseBackupPath := filepath.Join(tempDir, "base.tar.gz")
		os.WriteFile(baseBackupPath, []byte("dummy"), 0644)
		
		// Create metadata with current timestamp
		baseMetadata := createTestMetadata("testdb", baseBackupPath, 100, "dummychecksum", "full", nil)
		saveTestMetadata(baseBackupPath, baseMetadata)

		config := &IncrementalBackupConfig{
			BaseBackupPath:   baseBackupPath,
			DataDirectory:    tempDir,
			CompressionLevel: 6,
		}

		// This should find no changed files (empty directory)
		err := engine.CreateIncrementalBackup(ctx, config, []ChangedFile{})
		if err == nil {
			t.Error("Expected error for no changed files, got nil")
		}
	})
}

// Helper function to create test metadata
func createTestMetadata(database, backupFile string, size int64, checksum, backupType string, incremental *IncrementalMetadata) map[string]interface{} {
	metadata := map[string]interface{}{
		"database":    database,
		"backup_file": backupFile,
		"size":        size,
		"sha256":      checksum,
		"timestamp":   time.Now().Format(time.RFC3339),
		"backup_type": backupType,
	}
	if incremental != nil {
		metadata["incremental"] = incremental
	}
	return metadata
}

// Helper function to save test metadata
func saveTestMetadata(backupPath string, metadata map[string]interface{}) error {
	metaPath := backupPath + ".meta.json"
	file, err := os.Create(metaPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Simple JSON encoding
	content := fmt.Sprintf(`{
		"database": "%s",
		"backup_file": "%s",
		"size": %d,
		"sha256": "%s",
		"timestamp": "%s",
		"backup_type": "%s"
	}`,
		metadata["database"],
		metadata["backup_file"],
		metadata["size"],
		metadata["sha256"],
		metadata["timestamp"],
		metadata["backup_type"],
	)
	
	_, err = file.WriteString(content)
	return err
}
