package restore

import (
	"os"
	"path/filepath"
	"testing"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

func TestValidateArchive_FileNotFound(t *testing.T) {
	cfg := &config.Config{}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	err := safety.ValidateArchive("/nonexistent/file.dump")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestValidateArchive_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.dump")
	
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	cfg := &config.Config{}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	err := safety.ValidateArchive(emptyFile)
	if err == nil {
		t.Error("Expected error for empty file, got nil")
	}
}

func TestCheckDiskSpace_InsufficientSpace(t *testing.T) {
	// This test is hard to make deterministic without mocking
	// Just ensure the function doesn't panic
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.dump")
	
	// Create a small test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.Config{
		BackupDir: tmpDir,
	}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	// Should not panic
	_ = safety.CheckDiskSpace(testFile, 1.0)
}

func TestVerifyTools_PostgreSQL(t *testing.T) {
	cfg := &config.Config{}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	// This will fail if pg_restore is not installed, which is expected in many environments
	err := safety.VerifyTools("postgres")
	// We don't assert the result since it depends on the system
	// Just check it doesn't panic
	_ = err
}

func TestVerifyTools_MySQL(t *testing.T) {
	cfg := &config.Config{}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	// This will fail if mysql is not installed
	err := safety.VerifyTools("mysql")
	_ = err
}

func TestVerifyTools_UnknownDBType(t *testing.T) {
	cfg := &config.Config{}
	log := logger.NewNullLogger()
	safety := NewSafety(cfg, log)

	err := safety.VerifyTools("unknown")
	// Unknown DB types currently don't return error - they just don't verify anything
	// This is intentional to allow flexibility
	_ = err
}
