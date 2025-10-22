package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"dbbackup/internal/config"
	"dbbackup/internal/logger"
	
	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// Database represents a database connection and operations
type Database interface {
	// Connection management
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	
	// Database discovery
	ListDatabases(ctx context.Context) ([]string, error)
	ListTables(ctx context.Context, database string) ([]string, error)
	
	// Database operations
	CreateDatabase(ctx context.Context, name string) error
	DropDatabase(ctx context.Context, name string) error
	DatabaseExists(ctx context.Context, name string) (bool, error)
	
	// Information
	GetVersion(ctx context.Context) (string, error)
	GetDatabaseSize(ctx context.Context, database string) (int64, error)
	GetTableRowCount(ctx context.Context, database, table string) (int64, error)
	
	// Backup/Restore command building
	BuildBackupCommand(database, outputFile string, options BackupOptions) []string
	BuildRestoreCommand(database, inputFile string, options RestoreOptions) []string
	BuildSampleQuery(database, table string, strategy SampleStrategy) string
	
	// Validation
	ValidateBackupTools() error
}

// BackupOptions holds options for backup operations
type BackupOptions struct {
	Compression     int
	Parallel        int
	Format          string  // "custom", "plain", "directory"
	Blobs          bool
	SchemaOnly     bool
	DataOnly       bool
	NoOwner        bool
	NoPrivileges   bool
	Clean          bool
	IfExists       bool
	Role           string
}

// RestoreOptions holds options for restore operations
type RestoreOptions struct {
	Parallel     int
	Clean        bool
	IfExists     bool
	NoOwner      bool
	NoPrivileges bool
	SingleTransaction bool
}

// SampleStrategy defines how to sample data
type SampleStrategy struct {
	Type  string // "ratio", "percent", "count"
	Value int
}

// DatabaseInfo holds database metadata
type DatabaseInfo struct {
	Name         string
	Size         int64
	Owner        string
	Encoding     string
	Collation    string
	Tables       []TableInfo
}

// TableInfo holds table metadata
type TableInfo struct {
	Schema   string
	Name     string
	RowCount int64
	Size     int64
}

// New creates a new database instance based on configuration
func New(cfg *config.Config, log logger.Logger) (Database, error) {
	if cfg.IsPostgreSQL() {
		return NewPostgreSQL(cfg, log), nil
	} else if cfg.IsMySQL() {
		return NewMySQL(cfg, log), nil
	}
	return nil, fmt.Errorf("unsupported database type: %s", cfg.DatabaseType)
}

// Common database implementation
type baseDatabase struct {
	cfg  *config.Config
	log  logger.Logger
	db   *sql.DB
	dsn  string
}

func (b *baseDatabase) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

func (b *baseDatabase) Ping(ctx context.Context) error {
	if b.db == nil {
		return fmt.Errorf("database not connected")
	}
	return b.db.PingContext(ctx)
}

// buildTimeout creates a context with timeout for database operations
func buildTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}