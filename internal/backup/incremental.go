package backup

import (
	"context"
	"time"
)

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeFull        BackupType = "full"        // Complete backup of all data
	BackupTypeIncremental BackupType = "incremental" // Only changed files since base backup
)

// IncrementalMetadata contains metadata for incremental backups
type IncrementalMetadata struct {
	// BaseBackupID is the SHA-256 checksum of the base backup this incremental depends on
	BaseBackupID string `json:"base_backup_id"`
	
	// BaseBackupPath is the filename of the base backup (e.g., "mydb_20250126_120000.tar.gz")
	BaseBackupPath string `json:"base_backup_path"`
	
	// BaseBackupTimestamp is when the base backup was created
	BaseBackupTimestamp time.Time `json:"base_backup_timestamp"`
	
	// IncrementalFiles is the number of changed files included in this backup
	IncrementalFiles int `json:"incremental_files"`
	
	// TotalSize is the total size of changed files (bytes)
	TotalSize int64 `json:"total_size"`
	
	// BackupChain is the list of all backups needed for restore (base + incrementals)
	// Ordered from oldest to newest: [base, incr1, incr2, ...]
	BackupChain []string `json:"backup_chain"`
}

// ChangedFile represents a file that changed since the base backup
type ChangedFile struct {
	// RelativePath is the path relative to PostgreSQL data directory
	RelativePath string
	
	// AbsolutePath is the full filesystem path
	AbsolutePath string
	
	// Size is the file size in bytes
	Size int64
	
	// ModTime is the last modification time
	ModTime time.Time
	
	// Checksum is the SHA-256 hash of the file content (optional)
	Checksum string
}

// IncrementalBackupConfig holds configuration for incremental backups
type IncrementalBackupConfig struct {
	// BaseBackupPath is the path to the base backup archive
	BaseBackupPath string
	
	// DataDirectory is the PostgreSQL data directory to scan
	DataDirectory string
	
	// IncludeWAL determines if WAL files should be included
	IncludeWAL bool
	
	// CompressionLevel for the incremental archive (0-9)
	CompressionLevel int
}

// BackupChainResolver resolves the chain of backups needed for restore
type BackupChainResolver interface {
	// FindBaseBackup locates the base backup for an incremental backup
	FindBaseBackup(ctx context.Context, incrementalBackupID string) (*BackupInfo, error)
	
	// ResolveChain returns the complete chain of backups needed for restore
	// Returned in order: [base, incr1, incr2, ..., target]
	ResolveChain(ctx context.Context, targetBackupID string) ([]*BackupInfo, error)
	
	// ValidateChain verifies all backups in the chain exist and are valid
	ValidateChain(ctx context.Context, chain []*BackupInfo) error
}

// IncrementalBackupEngine handles incremental backup operations
type IncrementalBackupEngine interface {
	// FindChangedFiles identifies files changed since the base backup
	FindChangedFiles(ctx context.Context, config *IncrementalBackupConfig) ([]ChangedFile, error)
	
	// CreateIncrementalBackup creates a new incremental backup
	CreateIncrementalBackup(ctx context.Context, config *IncrementalBackupConfig, changedFiles []ChangedFile) error
	
	// RestoreIncremental restores an incremental backup on top of a base backup
	RestoreIncremental(ctx context.Context, baseBackupPath, incrementalPath, targetDir string) error
}

// BackupInfo extends the existing Info struct with incremental metadata
// This will be integrated into the existing backup.Info struct
type BackupInfo struct {
	// Existing fields from backup.Info...
	Database  string    `json:"database"`
	Timestamp time.Time `json:"timestamp"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	
	// New fields for incremental support
	BackupType BackupType           `json:"backup_type"`           // "full" or "incremental"
	Incremental *IncrementalMetadata `json:"incremental,omitempty"` // Only present for incremental backups
}
