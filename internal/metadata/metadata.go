package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupMetadata contains comprehensive information about a backup
type BackupMetadata struct {
	Version         string            `json:"version"`
	Timestamp       time.Time         `json:"timestamp"`
	Database        string            `json:"database"`
	DatabaseType    string            `json:"database_type"`    // postgresql, mysql, mariadb
	DatabaseVersion string            `json:"database_version"` // e.g., "PostgreSQL 15.3"
	Host            string            `json:"host"`
	Port            int               `json:"port"`
	User            string            `json:"user"`
	BackupFile      string            `json:"backup_file"`
	SizeBytes       int64             `json:"size_bytes"`
	SHA256          string            `json:"sha256"`
	Compression     string            `json:"compression"` // none, gzip, pigz
	BackupType      string            `json:"backup_type"` // full, incremental (for v2.0)
	BaseBackup      string            `json:"base_backup,omitempty"`
	Duration        float64           `json:"duration_seconds"`
	ExtraInfo       map[string]string `json:"extra_info,omitempty"`
}

// ClusterMetadata contains metadata for cluster backups
type ClusterMetadata struct {
	Version      string             `json:"version"`
	Timestamp    time.Time          `json:"timestamp"`
	ClusterName  string             `json:"cluster_name"`
	DatabaseType string             `json:"database_type"`
	Host         string             `json:"host"`
	Port         int                `json:"port"`
	Databases    []BackupMetadata   `json:"databases"`
	TotalSize    int64              `json:"total_size_bytes"`
	Duration     float64            `json:"duration_seconds"`
	ExtraInfo    map[string]string  `json:"extra_info,omitempty"`
}

// CalculateSHA256 computes the SHA-256 checksum of a file
func CalculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to calculate checksum: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Save writes metadata to a .meta.json file
func (m *BackupMetadata) Save() error {
	metaPath := m.BackupFile + ".meta.json"
	
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// Load reads metadata from a .meta.json file
func Load(backupFile string) (*BackupMetadata, error) {
	metaPath := backupFile + ".meta.json"
	
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta BackupMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// SaveCluster writes cluster metadata to a .meta.json file
func (m *ClusterMetadata) Save(targetFile string) error {
	metaPath := targetFile + ".meta.json"
	
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cluster metadata file: %w", err)
	}

	return nil
}

// LoadCluster reads cluster metadata from a .meta.json file
func LoadCluster(targetFile string) (*ClusterMetadata, error) {
	metaPath := targetFile + ".meta.json"
	
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster metadata file: %w", err)
	}

	var meta ClusterMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse cluster metadata: %w", err)
	}

	return &meta, nil
}

// ListBackups scans a directory for backup files and returns their metadata
func ListBackups(dir string) ([]*BackupMetadata, error) {
	pattern := filepath.Join(dir, "*.meta.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	var backups []*BackupMetadata
	for _, metaFile := range matches {
		// Extract backup file path (remove .meta.json suffix)
		backupFile := metaFile[:len(metaFile)-len(".meta.json")]
		
		meta, err := Load(backupFile)
		if err != nil {
			// Skip invalid metadata files
			continue
		}
		
		backups = append(backups, meta)
	}

	return backups, nil
}

// FormatSize returns human-readable size
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
