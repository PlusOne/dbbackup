package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
)

// PostgreSQL implements Database interface for PostgreSQL
type PostgreSQL struct {
	baseDatabase
}

// NewPostgreSQL creates a new PostgreSQL database instance
func NewPostgreSQL(cfg *config.Config, log logger.Logger) *PostgreSQL {
	return &PostgreSQL{
		baseDatabase: baseDatabase{
			cfg: cfg,
			log: log,
		},
	}
}

// Connect establishes a connection to PostgreSQL
func (p *PostgreSQL) Connect(ctx context.Context) error {
	// Build PostgreSQL DSN
	dsn := p.buildDSN()
	p.dsn = dsn
	
	p.log.Debug("Connecting to PostgreSQL", "dsn", sanitizeDSN(dsn))
	
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to open PostgreSQL connection: %w", err)
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
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}
	
	p.db = db
	p.log.Info("Connected to PostgreSQL successfully")
	return nil
}

// ListDatabases returns list of non-template databases
func (p *PostgreSQL) ListDatabases(ctx context.Context) ([]string, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	
	query := `SELECT datname FROM pg_database 
	          WHERE datistemplate = false 
	          ORDER BY datname`
	
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query databases: %w", err)
	}
	defer rows.Close()
	
	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan database name: %w", err)
		}
		databases = append(databases, name)
	}
	
	return databases, rows.Err()
}

// ListTables returns list of tables in a database
func (p *PostgreSQL) ListTables(ctx context.Context, database string) ([]string, error) {
	if p.db == nil {
		return nil, fmt.Errorf("not connected to database")
	}
	
	query := `SELECT schemaname||'.'||tablename as full_name
	          FROM pg_tables 
	          WHERE schemaname NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
	          ORDER BY schemaname, tablename`
	
	rows, err := p.db.QueryContext(ctx, query)
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
func (p *PostgreSQL) CreateDatabase(ctx context.Context, name string) error {
	if p.db == nil {
		return fmt.Errorf("not connected to database")
	}
	
	// PostgreSQL doesn't support CREATE DATABASE in transactions or prepared statements
	query := fmt.Sprintf("CREATE DATABASE %s", name)
	_, err := p.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", name, err)
	}
	
	p.log.Info("Created database", "name", name)
	return nil
}

// DropDatabase drops a database
func (p *PostgreSQL) DropDatabase(ctx context.Context, name string) error {
	if p.db == nil {
		return fmt.Errorf("not connected to database")
	}
	
	// Force drop connections and drop database
	query := fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)
	_, err := p.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to drop database %s: %w", name, err)
	}
	
	p.log.Info("Dropped database", "name", name)
	return nil
}

// DatabaseExists checks if a database exists
func (p *PostgreSQL) DatabaseExists(ctx context.Context, name string) (bool, error) {
	if p.db == nil {
		return false, fmt.Errorf("not connected to database")
	}
	
	query := `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`
	var exists bool
	err := p.db.QueryRowContext(ctx, query, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check database existence: %w", err)
	}
	
	return exists, nil
}

// GetVersion returns PostgreSQL version
func (p *PostgreSQL) GetVersion(ctx context.Context) (string, error) {
	if p.db == nil {
		return "", fmt.Errorf("not connected to database")
	}
	
	var version string
	err := p.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	
	return version, nil
}

// GetDatabaseSize returns database size in bytes
func (p *PostgreSQL) GetDatabaseSize(ctx context.Context, database string) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}
	
	query := `SELECT pg_database_size($1)`
	var size int64
	err := p.db.QueryRowContext(ctx, query, database).Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("failed to get database size: %w", err)
	}
	
	return size, nil
}

// GetTableRowCount returns approximate row count for a table
func (p *PostgreSQL) GetTableRowCount(ctx context.Context, database, table string) (int64, error) {
	if p.db == nil {
		return 0, fmt.Errorf("not connected to database")
	}
	
	// Use pg_stat_user_tables for approximate count (faster)
	parts := strings.Split(table, ".")
	if len(parts) != 2 {
		return 0, fmt.Errorf("table name must be in format schema.table")
	}
	
	query := `SELECT COALESCE(n_tup_ins, 0) FROM pg_stat_user_tables 
	          WHERE schemaname = $1 AND relname = $2`
	
	var count int64
	err := p.db.QueryRowContext(ctx, query, parts[0], parts[1]).Scan(&count)
	if err != nil {
		// Fallback to exact count if stats not available
		exactQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		err = p.db.QueryRowContext(ctx, exactQuery).Scan(&count)
		if err != nil {
			return 0, fmt.Errorf("failed to get table row count: %w", err)
		}
	}
	
	return count, nil
}

// BuildBackupCommand builds pg_dump command
func (p *PostgreSQL) BuildBackupCommand(database, outputFile string, options BackupOptions) []string {
	cmd := []string{"pg_dump"}
	
	// Connection parameters
	if p.cfg.Host != "localhost" {
		cmd = append(cmd, "-h", p.cfg.Host)
		cmd = append(cmd, "-p", strconv.Itoa(p.cfg.Port))
		cmd = append(cmd, "--no-password")
	}
	cmd = append(cmd, "-U", p.cfg.User)
	
	// Format and compression
	if options.Format != "" {
		cmd = append(cmd, "--format="+options.Format)
	} else {
		cmd = append(cmd, "--format=custom")
	}
	
	if options.Compression > 0 {
		cmd = append(cmd, "--compress="+strconv.Itoa(options.Compression))
	}
	
	// Parallel jobs (only for directory format)
	if options.Parallel > 1 && options.Format == "directory" {
		cmd = append(cmd, "--jobs="+strconv.Itoa(options.Parallel))
	}
	
	// Options
	if options.Blobs {
		cmd = append(cmd, "--blobs")
	}
	if options.SchemaOnly {
		cmd = append(cmd, "--schema-only")
	}
	if options.DataOnly {
		cmd = append(cmd, "--data-only")
	}
	if options.NoOwner {
		cmd = append(cmd, "--no-owner")
	}
	if options.NoPrivileges {
		cmd = append(cmd, "--no-privileges")
	}
	if options.Role != "" {
		cmd = append(cmd, "--role="+options.Role)
	}
	
	// Database and output
	cmd = append(cmd, "--dbname="+database)
	cmd = append(cmd, "--file="+outputFile)
	
	return cmd
}

// BuildRestoreCommand builds pg_restore command
func (p *PostgreSQL) BuildRestoreCommand(database, inputFile string, options RestoreOptions) []string {
	cmd := []string{"pg_restore"}
	
	// Connection parameters
	if p.cfg.Host != "localhost" {
		cmd = append(cmd, "-h", p.cfg.Host)
		cmd = append(cmd, "-p", strconv.Itoa(p.cfg.Port))
		cmd = append(cmd, "--no-password")
	}
	cmd = append(cmd, "-U", p.cfg.User)
	
	// Parallel jobs
	if options.Parallel > 1 {
		cmd = append(cmd, "--jobs="+strconv.Itoa(options.Parallel))
	}
	
	// Options
	if options.Clean {
		cmd = append(cmd, "--clean")
	}
	if options.IfExists {
		cmd = append(cmd, "--if-exists")
	}
	if options.NoOwner {
		cmd = append(cmd, "--no-owner")
	}
	if options.NoPrivileges {
		cmd = append(cmd, "--no-privileges")
	}
	if options.SingleTransaction {
		cmd = append(cmd, "--single-transaction")
	}
	
	// Database and input
	cmd = append(cmd, "--dbname="+database)
	cmd = append(cmd, inputFile)
	
	return cmd
}

// BuildSampleQuery builds SQL query for sampling data
func (p *PostgreSQL) BuildSampleQuery(database, table string, strategy SampleStrategy) string {
	switch strategy.Type {
	case "ratio":
		// Every Nth record using row_number
		return fmt.Sprintf("SELECT * FROM (SELECT *, row_number() OVER () as rn FROM %s) t WHERE rn %% %d = 1", 
			table, strategy.Value)
	case "percent":
		// Percentage sampling using TABLESAMPLE (PostgreSQL 9.5+)
		return fmt.Sprintf("SELECT * FROM %s TABLESAMPLE BERNOULLI(%d)", table, strategy.Value)
	case "count":
		// First N records
		return fmt.Sprintf("SELECT * FROM %s LIMIT %d", table, strategy.Value)
	default:
		return fmt.Sprintf("SELECT * FROM %s LIMIT 1000", table)
	}
}

// ValidateBackupTools checks if required PostgreSQL tools are available
func (p *PostgreSQL) ValidateBackupTools() error {
	tools := []string{"pg_dump", "pg_restore", "pg_dumpall", "psql"}
	
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool not found: %s", tool)
		}
	}
	
	return nil
}

// buildDSN constructs PostgreSQL connection string
func (p *PostgreSQL) buildDSN() string {
	dsn := fmt.Sprintf("user=%s dbname=%s", p.cfg.User, p.cfg.Database)
	
	if p.cfg.Password != "" {
		dsn += " password=" + p.cfg.Password
	}
	
	// For localhost connections, try socket first for peer auth
	if p.cfg.Host == "localhost" && p.cfg.Password == "" {
		// Try Unix socket connection for peer authentication
		// Common PostgreSQL socket locations
		socketDirs := []string{
			"/var/run/postgresql",
			"/tmp",
			"/var/lib/pgsql",
		}
		
		for _, dir := range socketDirs {
			socketPath := fmt.Sprintf("%s/.s.PGSQL.%d", dir, p.cfg.Port)
			if _, err := os.Stat(socketPath); err == nil {
				dsn += " host=" + dir
				p.log.Debug("Using PostgreSQL socket", "path", socketPath)
				break
			}
		}
	} else if p.cfg.Host != "localhost" || p.cfg.Password != "" {
		// Use TCP connection
		dsn += " host=" + p.cfg.Host
		dsn += " port=" + strconv.Itoa(p.cfg.Port)
	}
	
	if p.cfg.SSLMode != "" && !p.cfg.Insecure {
		// Map SSL modes to supported values for lib/pq
		switch strings.ToLower(p.cfg.SSLMode) {
		case "prefer", "preferred":
			dsn += " sslmode=require" // lib/pq default, closest to prefer
		case "require", "required":
			dsn += " sslmode=require"
		case "verify-ca":
			dsn += " sslmode=verify-ca"
		case "verify-full", "verify-identity":
			dsn += " sslmode=verify-full"
		case "disable", "disabled":
			dsn += " sslmode=disable"
		default:
			dsn += " sslmode=require" // Safe default
		}
	} else if p.cfg.Insecure {
		dsn += " sslmode=disable"
	}
	
	return dsn
}

// sanitizeDSN removes password from DSN for logging
func sanitizeDSN(dsn string) string {
	// Simple password removal for logging
	parts := strings.Split(dsn, " ")
	var sanitized []string
	
	for _, part := range parts {
		if strings.HasPrefix(part, "password=") {
			sanitized = append(sanitized, "password=***")
		} else {
			sanitized = append(sanitized, part)
		}
	}
	
	return strings.Join(sanitized, " ")
}