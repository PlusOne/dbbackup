package restore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectArchiveFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     ArchiveFormat
	}{
		{
			name:     "PostgreSQL custom dump",
			filename: "backup.dump",
			want:     FormatPostgreSQLDump,
		},
		{
			name:     "PostgreSQL custom dump compressed",
			filename: "backup.dump.gz",
			want:     FormatPostgreSQLDumpGz,
		},
		{
			name:     "PostgreSQL SQL script",
			filename: "backup.sql",
			want:     FormatPostgreSQLSQL,
		},
		{
			name:     "PostgreSQL SQL compressed",
			filename: "backup.sql.gz",
			want:     FormatPostgreSQLSQLGz,
		},
		{
			name:     "Cluster backup",
			filename: "cluster_backup_20241107.tar.gz",
			want:     FormatClusterTarGz,
		},
		{
			name:     "MySQL SQL script",
			filename: "mydb.sql",
			want:     FormatPostgreSQLSQL, // Note: Could be MySQL or PostgreSQL
		},
		{
			name:     "Unknown format",
			filename: "backup.txt",
			want:     FormatUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectArchiveFormat(tt.filename)
			if got != tt.want {
				t.Errorf("DetectArchiveFormat(%s) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestArchiveFormat_String(t *testing.T) {
	tests := []struct {
		format ArchiveFormat
		want   string
	}{
		{FormatPostgreSQLDump, "PostgreSQL Dump"},
		{FormatPostgreSQLDumpGz, "PostgreSQL Dump (gzip)"},
		{FormatPostgreSQLSQL, "PostgreSQL SQL"},
		{FormatPostgreSQLSQLGz, "PostgreSQL SQL (gzip)"},
		{FormatMySQLSQL, "MySQL SQL"},
		{FormatMySQLSQLGz, "MySQL SQL (gzip)"},
		{FormatClusterTarGz, "Cluster Archive (tar.gz)"},
		{FormatUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.format.String()
			if got != tt.want {
				t.Errorf("Format.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestArchiveFormat_IsCompressed(t *testing.T) {
	tests := []struct {
		format ArchiveFormat
		want   bool
	}{
		{FormatPostgreSQLDump, false},
		{FormatPostgreSQLDumpGz, true},
		{FormatPostgreSQLSQL, false},
		{FormatPostgreSQLSQLGz, true},
		{FormatMySQLSQL, false},
		{FormatMySQLSQLGz, true},
		{FormatClusterTarGz, true},
		{FormatUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			got := tt.format.IsCompressed()
			if got != tt.want {
				t.Errorf("%s.IsCompressed() = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestArchiveFormat_IsClusterBackup(t *testing.T) {
	tests := []struct {
		format ArchiveFormat
		want   bool
	}{
		{FormatPostgreSQLDump, false},
		{FormatPostgreSQLDumpGz, false},
		{FormatPostgreSQLSQL, false},
		{FormatPostgreSQLSQLGz, false},
		{FormatMySQLSQL, false},
		{FormatMySQLSQLGz, false},
		{FormatClusterTarGz, true},
		{FormatUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.format.String(), func(t *testing.T) {
			got := tt.format.IsClusterBackup()
			if got != tt.want {
				t.Errorf("%s.IsClusterBackup() = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "bytes",
			bytes: 500,
			want:  "500 B",
		},
		{
			name:  "kilobytes",
			bytes: 2048,
			want:  "2.0 KB",
		},
		{
			name:  "megabytes",
			bytes: 5242880,
			want:  "5.0 MB",
		},
		{
			name:  "gigabytes",
			bytes: 2147483648,
			want:  "2.0 GB",
		},
		{
			name:  "terabytes",
			bytes: 1099511627776,
			want:  "1.0 TB",
		},
		{
			name:  "zero",
			bytes: 0,
			want:  "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestDetectArchiveFormatWithRealFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	testCases := []struct {
		name     string
		filename string
		content  []byte
		want     ArchiveFormat
	}{
		{
			name:     "PostgreSQL dump with magic bytes",
			filename: "test.dump",
			content:  []byte("PGDMP"),
			want:     FormatPostgreSQLDump,
		},
		{
			name:     "Gzipped file",
			filename: "test.gz",
			content:  []byte{0x1f, 0x8b, 0x08},
			want:     FormatUnknown, // .gz without proper extension
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, tc.filename)
			if err := os.WriteFile(filePath, tc.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			got := DetectArchiveFormat(filePath)
			if got != tc.want {
				t.Errorf("DetectArchiveFormat(%s) = %v, want %v", tc.filename, got, tc.want)
			}
		})
	}
}
