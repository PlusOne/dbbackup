package restore

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// Safety provides pre-restore validation and safety checks
type Safety struct {
	cfg *config.Config
	log logger.Logger
}

// NewSafety creates a new safety checker
func NewSafety(cfg *config.Config, log logger.Logger) *Safety {
	return &Safety{
		cfg: cfg,
		log: log,
	}
}

// ValidateArchive performs integrity checks on the archive
func (s *Safety) ValidateArchive(archivePath string) error {
	// Check if file exists
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("archive not accessible: %w", err)
	}

	// Check if file is not empty
	if stat.Size() == 0 {
		return fmt.Errorf("archive is empty")
	}

	// Check if file is too small (likely corrupted)
	if stat.Size() < 100 {
		return fmt.Errorf("archive is suspiciously small (%d bytes)", stat.Size())
	}

	// Detect format
	format := DetectArchiveFormat(archivePath)
	if format == FormatUnknown {
		return fmt.Errorf("unknown archive format: %s", archivePath)
	}

	// Validate based on format
	switch format {
	case FormatPostgreSQLDump:
		return s.validatePgDump(archivePath)
	case FormatPostgreSQLDumpGz:
		return s.validatePgDumpGz(archivePath)
	case FormatPostgreSQLSQL, FormatMySQLSQL:
		return s.validateSQLScript(archivePath)
	case FormatPostgreSQLSQLGz, FormatMySQLSQLGz:
		return s.validateSQLScriptGz(archivePath)
	case FormatClusterTarGz:
		return s.validateTarGz(archivePath)
	}

	return nil
}

// validatePgDump validates PostgreSQL dump file
func (s *Safety) validatePgDump(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Read first 512 bytes for signature check
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read file: %w", err)
	}

	if n < 5 {
		return fmt.Errorf("file too small to validate")
	}

	// Check for PGDMP signature
	if string(buffer[:5]) == "PGDMP" {
		return nil
	}

	// Check for PostgreSQL dump indicators
	content := strings.ToLower(string(buffer[:n]))
	if strings.Contains(content, "postgresql") || strings.Contains(content, "pg_dump") {
		return nil
	}

	return fmt.Errorf("does not appear to be a PostgreSQL dump file")
}

// validatePgDumpGz validates compressed PostgreSQL dump
func (s *Safety) validatePgDumpGz(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Open gzip reader
	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	// Read first 512 bytes
	buffer := make([]byte, 512)
	n, err := gz.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read gzip contents: %w", err)
	}

	if n < 5 {
		return fmt.Errorf("gzip archive too small")
	}

	// Check for PGDMP signature
	if string(buffer[:5]) == "PGDMP" {
		return nil
	}

	content := strings.ToLower(string(buffer[:n]))
	if strings.Contains(content, "postgresql") || strings.Contains(content, "pg_dump") {
		return nil
	}

	return fmt.Errorf("does not appear to be a PostgreSQL dump file")
}

// validateSQLScript validates SQL script
func (s *Safety) validateSQLScript(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read file: %w", err)
	}

	content := strings.ToLower(string(buffer[:n]))
	if containsSQLKeywords(content) {
		return nil
	}

	return fmt.Errorf("does not appear to contain SQL content")
}

// validateSQLScriptGz validates compressed SQL script
func (s *Safety) validateSQLScriptGz(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	buffer := make([]byte, 1024)
	n, err := gz.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("cannot read gzip contents: %w", err)
	}

	content := strings.ToLower(string(buffer[:n]))
	if containsSQLKeywords(content) {
		return nil
	}

	return fmt.Errorf("does not appear to contain SQL content")
}

// validateTarGz validates tar.gz archive
func (s *Safety) validateTarGz(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	// Check gzip magic number
	buffer := make([]byte, 3)
	n, err := file.Read(buffer)
	if err != nil || n < 3 {
		return fmt.Errorf("cannot read file header")
	}

	if buffer[0] == 0x1f && buffer[1] == 0x8b {
		return nil // Valid gzip header
	}

	return fmt.Errorf("not a valid gzip file")
}

// containsSQLKeywords checks if content contains SQL keywords
func containsSQLKeywords(content string) bool {
	keywords := []string{
		"select", "insert", "create", "drop", "alter",
		"database", "table", "update", "delete", "from", "where",
	}

	for _, keyword := range keywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

// CheckDiskSpace verifies sufficient disk space for restore
func (s *Safety) CheckDiskSpace(archivePath string, multiplier float64) error {
	// Get archive size
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("cannot stat archive: %w", err)
	}

	archiveSize := stat.Size()
	
	// Estimate required space (archive size * multiplier for decompression/extraction)
	requiredSpace := int64(float64(archiveSize) * multiplier)

	// Get available disk space
	var statfs syscall.Statfs_t
	if err := syscall.Statfs(s.cfg.BackupDir, &statfs); err != nil {
		s.log.Warn("Cannot check disk space", "error", err)
		return nil // Don't fail if we can't check
	}

	availableSpace := int64(statfs.Bavail) * statfs.Bsize

	if availableSpace < requiredSpace {
		return fmt.Errorf("insufficient disk space: need %s, have %s",
			FormatBytes(requiredSpace), FormatBytes(availableSpace))
	}

	s.log.Info("Disk space check passed",
		"required", FormatBytes(requiredSpace),
		"available", FormatBytes(availableSpace))

	return nil
}

// VerifyTools checks if required restore tools are available
func (s *Safety) VerifyTools(dbType string) error {
	var tools []string

	if dbType == "postgres" {
		tools = []string{"pg_restore", "psql"}
	} else if dbType == "mysql" || dbType == "mariadb" {
		tools = []string{"mysql"}
	}

	missing := []string{}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}

	return nil
}

// CheckDatabaseExists verifies if target database exists
func (s *Safety) CheckDatabaseExists(ctx context.Context, dbName string) (bool, error) {
	if s.cfg.DatabaseType == "postgres" {
		return s.checkPostgresDatabaseExists(ctx, dbName)
	} else if s.cfg.DatabaseType == "mysql" || s.cfg.DatabaseType == "mariadb" {
		return s.checkMySQLDatabaseExists(ctx, dbName)
	}

	return false, fmt.Errorf("unsupported database type: %s", s.cfg.DatabaseType)
}

// checkPostgresDatabaseExists checks if PostgreSQL database exists
func (s *Safety) checkPostgresDatabaseExists(ctx context.Context, dbName string) (bool, error) {
	cmd := exec.CommandContext(ctx,
		"psql",
		"-h", s.cfg.Host,
		"-p", fmt.Sprintf("%d", s.cfg.Port),
		"-U", s.cfg.User,
		"-d", "postgres",
		"-tAc", fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", dbName),
	)

	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.cfg.Password))

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}

	return strings.TrimSpace(string(output)) == "1", nil
}

// checkMySQLDatabaseExists checks if MySQL database exists
func (s *Safety) checkMySQLDatabaseExists(ctx context.Context, dbName string) (bool, error) {
	cmd := exec.CommandContext(ctx,
		"mysql",
		"-h", s.cfg.Host,
		"-P", fmt.Sprintf("%d", s.cfg.Port),
		"-u", s.cfg.User,
		"-e", fmt.Sprintf("SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME='%s'", dbName),
	)

	if s.cfg.Password != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("MYSQL_PWD=%s", s.cfg.Password))
	}

	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}

	return strings.Contains(string(output), dbName), nil
}
