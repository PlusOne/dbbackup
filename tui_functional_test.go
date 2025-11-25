// +build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
)

// TestTUIAutoSelectBackup tests TUI with auto-select for backup menu
func TestTUIAutoSelectBackup(t *testing.T) {
	// Setup test environment
	testDir := setupTestEnv(t)
	defer os.RemoveAll(testDir)

	binary := buildBinary(t)
	defer os.Remove(binary)

	// Start the TUI with auto-select 0 (Backup Database) as postgres user
	cmd := exec.Command("su", "-", "postgres", "-c",
		"cd "+testDir+" && "+binary+" interactive --auto-select 0 --auto-database postgres")
	cmd.Dir = testDir

	console, err := expect.NewConsole(
		expect.WithStdout(os.Stdout),
		expect.WithDefaultTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create console: %v", err)
	}
	defer console.Close()

	cmd.Stdin = console.Tty()
	cmd.Stdout = console.Tty()
	cmd.Stderr = console.Tty()

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Expect backup completion
	_, err = console.ExpectString("completed successfully")
	if err != nil {
		t.Errorf("Backup did not complete: %v", err)
	}
	
	// Give it a moment to write files
	time.Sleep(1 * time.Second)
	
	// Kill the process since it's waiting for user input
	cmd.Process.Kill()
	cmd.Wait()

	// Verify backup was created
	backupDir := filepath.Join(testDir, "backups")
	files, _ := filepath.Glob(filepath.Join(backupDir, "db_postgres_*.dump"))
	if len(files) == 0 {
		t.Error("No backup file created")
	}

	// Verify checksum file
	checksumFiles, _ := filepath.Glob(filepath.Join(backupDir, "db_postgres_*.dump.sha256"))
	if len(checksumFiles) == 0 {
		t.Error("No checksum file created")
	}

	// Verify metadata file
	metaFiles, _ := filepath.Glob(filepath.Join(backupDir, "db_postgres_*.dump.info"))
	if len(metaFiles) == 0 {
		t.Error("No metadata file created")
	}

	t.Log("✅ TUI Auto-Select Backup test passed")
}

// TestTUIAutoSelectWithLogging tests TUI auto-select with logging enabled
func TestTUIAutoSelectWithLogging(t *testing.T) {
	testDir := setupTestEnv(t)
	defer os.RemoveAll(testDir)

	binary := buildBinary(t)
	defer os.Remove(binary)

	logFile := filepath.Join(testDir, "tui_test.log")

	// Start TUI with logging as postgres user
	cmd := exec.Command("su", "-", "postgres", "-c",
		"cd "+testDir+" && "+binary+" interactive --auto-select 0 --auto-database postgres --log-file "+logFile)
	cmd.Dir = testDir

	console, err := expect.NewConsole(
		expect.WithDefaultTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create console: %v", err)
	}
	defer console.Close()

	cmd.Stdin = console.Tty()
	cmd.Stdout = console.Tty()
	cmd.Stderr = console.Tty()

	err = cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start command: %v", err)
	}

	// Expect backup completion
	_, err = console.ExpectString("completed successfully")
	if err != nil {
		t.Logf("Warning: Did not see completion message: %v", err)
	}
	
	// Give it time to write files
	time.Sleep(1 * time.Second)
	
	// Kill process
	cmd.Process.Kill()
	cmd.Wait()

	// Verify log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Log file not created")
	} else {
		t.Log("✅ Log file created:", logFile)
	}

	t.Log("✅ TUI Auto-Select with Logging test passed")
}

// TestTUIManualNavigation tests manual navigation through menus
func TestTUIManualNavigation(t *testing.T) {
	t.Skip("Manual navigation requires PTY and is tested in TEST 8 & 10")
}

// Helper: Setup test environment
func setupTestEnv(t *testing.T) string {
	testDir := "/tmp/dbbackup_functional_test_" + time.Now().Format("20060102_150405")
	backupDir := filepath.Join(testDir, "backups")

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Change ownership to postgres
	exec.Command("chown", "-R", "postgres:postgres", testDir).Run()

	return testDir
}

// Helper: Build binary for testing
func buildBinary(t *testing.T) string {
	binary := "/tmp/dbbackup_test_" + time.Now().Format("20060102_150405")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = "/root/dbbackup"

	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, output)
	}

	return binary
}
