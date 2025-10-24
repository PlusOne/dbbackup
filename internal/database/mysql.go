package database

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// MySQL implements Database interface for MySQL
type MySQL struct {
	baseDatabase
}

// NewMySQL creates a new MySQL database instance
func NewMySQL(cfg *config.Config, log logger.Logger) *MySQL {
	return &MySQL{
		baseDatabase: baseDatabase{
			cfg: cfg,
			log: log,
		},
	}
}

// Connect establishes a connection to MySQL
func (m *MySQL) Connect(ctx context.Context) error {
	// Build MySQL DSN
	dsn := m.buildDSN()
	m.dsn = dsn

	m.log.Debug("Connecting to MySQL", "dsn", sanitizeMySQLDSN(dsn))

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(0)

	// Test connection
	timeoutCtx, cancel := buildTimeout(ctx, 0)
	defer cancel()

	if err := db.PingContext(timeoutCtx); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	m.db = db
	m.log.Info("Connected to MySQL successfully")
	return nil
}

// ListDatabases returns list of databases (excluding system databases)
func (m *MySQL) ListDatabases(ctx context.Context) ([]string, error) {
	if m.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}

	query := `SHOW DATABASES`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()

	var databases []string
	systemDbs := map[string]bool{
		"information_schema": true,
		"performance_schema": true,
		"mysql":              true,
		"sys":                true,
	}

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}

		// Skip system databases
		if !systemDbs[name] {
			databases = append(databases, name)
		}
	}

	return databases, rows.Err()
}

// ListTables returns list of tables in a database
func (m *MySQL) ListTables(ctx context.Context, database string) ([]string, error) {
	if m.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}

	query := `SELECT table_name FROM information_schema.tables 
	          WHERE table_schema = ? AND table_type = 'BASE TABLE'
	          ORDER BY table_name`

	rows, err := m.db.QueryContext(ctx, query, database)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// CreateDatabase creates a new database
func (m *MySQL) CreateDatabase(ctx context.Context, name string) error {
	if m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	query := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", name)
	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", name, err)
	}

	m.log.Info("Created database", "name", name)
	return nil
}

// DropDatabase drops a database
func (m *MySQL) DropDatabase(ctx context.Context, name string) error {
	if m.db == nil {
		return fmt.Errorf("not connected to database")
	}

	query := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", name)
	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", name, err)
	}

	m.log.Info("Dropped database", "name", name)
	return nil
}

// DatabaseExists checks if a database exists
func (m *MySQL) DatabaseExists(ctx context.Context, name string) (bool, error) {
	if m.db == nil {
		return false, fmt.Errorf("not connected to database")
	}

	query := `SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?`
	var dbName string
	err := m.db.QueryRowContext(ctx, query, name).Scan(&dbName)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}

	return true, nil
}

// GetVersion returns MySQL version
func (m *MySQL) GetVersion(ctx context.Context) (string, error) {
	if m.db == nil {
		return "", fmt.Errorf("not connected to database")
	}

	var version string
	err := m.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}

	return version, nil
}

// GetDatabaseSize returns database size in bytes
func (m *MySQL) GetDatabaseSize(ctx context.Context, database string) (int64, error) {
	if m.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}

	query := `SELECT COALESCE(SUM(data_length + index_length), 0) as size_bytes
	          FROM information_schema.tables 
	          WHERE table_schema = ?`

	var size int64
	err := m.db.QueryRowContext(ctx, query, database).Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("failed to get database size: %w", err)
	}

	return size, nil
}

// GetTableRowCount returns row count for a table
func (m *MySQL) GetTableRowCount(ctx context.Context, database, table string) (int64, error) {
	if m.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}

	// First try information_schema for approximate count (faster)
	query := `SELECT table_rows FROM information_schema.tables 
	          WHERE table_schema = ? AND table_name = ?`

	var count int64
	err := m.db.QueryRowContext(ctx, query, database, table).Scan(&count)
	if err != nil || count == 0 {
		// Fallback to exact count
		exactQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`.`%s`", database, table)
		err = m.db.QueryRowContext(ctx, exactQuery).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("failed to get table row count: %w", err)
		}
	}

	return count, nil
}

// BuildBackupCommand builds mysqldump command
func (m *MySQL) BuildBackupCommand(database, outputFile string, options BackupOptions) []string {
	cmd := []string{"mysqldump"}

	// Connection parameters
	cmd = append(cmd, "-h", m.cfg.Host)
	cmd = append(cmd, "-P", strconv.Itoa(m.cfg.Port))
	cmd = append(cmd, "-u", m.cfg.User)

	if m.cfg.Password != "" {
		cmd = append(cmd, "-p"+m.cfg.Password)
	}

	// SSL options
	if m.cfg.Insecure {
		cmd = append(cmd, "--skip-ssl")
	} else if mode := strings.ToLower(m.cfg.SSLMode); mode != "" {
		switch mode {
		case "require", "required":
			cmd = append(cmd, "--ssl-mode=REQUIRED")
		case "verify-ca":
			cmd = append(cmd, "--ssl-mode=VERIFY_CA")
		case "verify-full", "verify-identity":
			cmd = append(cmd, "--ssl-mode=VERIFY_IDENTITY")
		case "disable", "disabled":
			cmd = append(cmd, "--skip-ssl")
		}
	}

	// Backup options
	cmd = append(cmd, "--single-transaction") // Consistent backup
	cmd = append(cmd, "--routines")           // Include stored procedures/functions
	cmd = append(cmd, "--triggers")           // Include triggers
	cmd = append(cmd, "--events")             // Include events

	if options.SchemaOnly {
		cmd = append(cmd, "--no-data")
	} else if options.DataOnly {
		cmd = append(cmd, "--no-create-info")
	}

	if options.NoOwner || options.NoPrivileges {
		cmd = append(cmd, "--skip-add-drop-table")
	}

	// Compression (handled externally for MySQL)
	// Output redirection will be handled by caller

	// Database
	cmd = append(cmd, database)

	return cmd
}

// BuildRestoreCommand builds mysql restore command
func (m *MySQL) BuildRestoreCommand(database, inputFile string, options RestoreOptions) []string {
	cmd := []string{"mysql"}

	// Connection parameters
	cmd = append(cmd, "-h", m.cfg.Host)
	cmd = append(cmd, "-P", strconv.Itoa(m.cfg.Port))
	cmd = append(cmd, "-u", m.cfg.User)

	if m.cfg.Password != "" {
		cmd = append(cmd, "-p"+m.cfg.Password)
	}

	// SSL options
	if m.cfg.Insecure {
		cmd = append(cmd, "--skip-ssl")
	}

	// Options
	if options.SingleTransaction {
		cmd = append(cmd, "--single-transaction")
	}

	// Database
	cmd = append(cmd, database)

	// Input file (will be handled via stdin redirection)

	return cmd
}

// BuildSampleQuery builds SQL query for sampling data
func (m *MySQL) BuildSampleQuery(database, table string, strategy SampleStrategy) string {
	switch strategy.Type {
	case "ratio":
		// Every Nth record using row_number (MySQL 8.0+) or modulo
		return fmt.Sprintf("SELECT * FROM (SELECT *, (@row_number:=@row_number + 1) AS rn FROM %s.%s CROSS JOIN (SELECT @row_number:=0) AS t) AS numbered WHERE rn %% %d = 1",
			database, table, strategy.Value)
	case "percent":
		// Percentage sampling using RAND()
		return fmt.Sprintf("SELECT * FROM %s.%s WHERE RAND() <= %f",
			database, table, float64(strategy.Value)/100.0)
	case "count":
		// First N records
		return fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d", database, table, strategy.Value)
	default:
		return fmt.Sprintf("SELECT * FROM %s.%s LIMIT 1000", database, table)
	}
}

// ValidateBackupTools checks if required MySQL tools are available
func (m *MySQL) ValidateBackupTools() error {
	tools := []string{"mysqldump", "mysql"}

	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool not found: %s", tool)
		}
	}

	return nil
}

// buildDSN constructs MySQL connection string
func (m *MySQL) buildDSN() string {
	dsn := ""

	if m.cfg.User != "" {
		dsn += m.cfg.User
	}

	if m.cfg.Password != "" {
		dsn += ":" + m.cfg.Password
	}

	dsn += "@"

	if m.cfg.Host != "" && m.cfg.Host != "localhost" {
		dsn += "tcp(" + m.cfg.Host + ":" + strconv.Itoa(m.cfg.Port) + ")"
	}

	dsn += "/" + m.cfg.Database

	// Add connection parameters
	params := []string{}

	if !m.cfg.Insecure {
		switch strings.ToLower(m.cfg.SSLMode) {
		case "require", "required":
			params = append(params, "tls=true")
		case "verify-ca", "verify-full", "verify-identity":
			params = append(params, "tls=preferred")
		}
	}

	// Add charset
	params = append(params, "charset=utf8mb4")
	params = append(params, "parseTime=true")

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	return dsn
}

// sanitizeMySQLDSN removes password from DSN for logging
func sanitizeMySQLDSN(dsn string) string {
	// Find password part and replace it
	if idx := strings.Index(dsn, ":"); idx != -1 {
		if endIdx := strings.Index(dsn[idx:], "@"); endIdx != -1 {
			return dsn[:idx] + ":***" + dsn[idx+endIdx:]
		}
	}
	return dsn
}
