package wal

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dbbackup/internal/logger"
)

// Compressor handles WAL file compression
type Compressor struct {
	log logger.Logger
}

// NewCompressor creates a new WAL compressor
func NewCompressor(log logger.Logger) *Compressor {
	return &Compressor{
		log: log,
	}
}

// CompressWALFile compresses a WAL file using gzip
// Returns the path to the compressed file and the compressed size
func (c *Compressor) CompressWALFile(sourcePath, destPath string, level int) (int64, error) {
	c.log.Debug("Compressing WAL file", "source", sourcePath, "dest", destPath, "level", level)

	// Open source file
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file size for logging
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat source file: %w", err)
	}
	originalSize := srcInfo.Size()

	// Create destination file
	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Create gzip writer with specified compression level
	gzWriter, err := gzip.NewWriterLevel(dstFile, level)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzWriter.Close()

	// Copy and compress
	_, err = io.Copy(gzWriter, srcFile)
	if err != nil {
		return 0, fmt.Errorf("compression failed: %w", err)
	}

	// Close gzip writer to flush buffers
	if err := gzWriter.Close(); err != nil {
		return 0, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Sync to disk
	if err := dstFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync compressed file: %w", err)
	}

	// Get actual compressed size
	dstInfo, err := dstFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat compressed file: %w", err)
	}
	compressedSize := dstInfo.Size()

	compressionRatio := float64(originalSize) / float64(compressedSize)
	c.log.Debug("WAL compression complete",
		"original_size", originalSize,
		"compressed_size", compressedSize,
		"compression_ratio", fmt.Sprintf("%.2fx", compressionRatio),
		"saved_bytes", originalSize-compressedSize)

	return compressedSize, nil
}

// DecompressWALFile decompresses a gzipped WAL file
func (c *Compressor) DecompressWALFile(sourcePath, destPath string) (int64, error) {
	c.log.Debug("Decompressing WAL file", "source", sourcePath, "dest", destPath)

	// Open compressed source file
	srcFile, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open compressed file: %w", err)
	}
	defer srcFile.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(srcFile)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip reader (file may be corrupted): %w", err)
	}
	defer gzReader.Close()

	// Create destination file
	dstFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return 0, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Decompress
	written, err := io.Copy(dstFile, gzReader)
	if err != nil {
		return 0, fmt.Errorf("decompression failed: %w", err)
	}

	// Sync to disk
	if err := dstFile.Sync(); err != nil {
		return 0, fmt.Errorf("failed to sync decompressed file: %w", err)
	}

	c.log.Debug("WAL decompression complete", "decompressed_size", written)
	return written, nil
}

// CompressAndArchive compresses a WAL file and archives it in one operation
func (c *Compressor) CompressAndArchive(walPath, archiveDir string, level int) (archivePath string, compressedSize int64, err error) {
	walFileName := filepath.Base(walPath)
	compressedFileName := walFileName + ".gz"
	archivePath = filepath.Join(archiveDir, compressedFileName)

	// Ensure archive directory exists
	if err := os.MkdirAll(archiveDir, 0700); err != nil {
		return "", 0, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Compress directly to archive location
	compressedSize, err = c.CompressWALFile(walPath, archivePath, level)
	if err != nil {
		// Clean up partial file on error
		os.Remove(archivePath)
		return "", 0, err
	}

	return archivePath, compressedSize, nil
}

// GetCompressionRatio calculates compression ratio between original and compressed files
func (c *Compressor) GetCompressionRatio(originalPath, compressedPath string) (float64, error) {
	origInfo, err := os.Stat(originalPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat original file: %w", err)
	}

	compInfo, err := os.Stat(compressedPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat compressed file: %w", err)
	}

	if compInfo.Size() == 0 {
		return 0, fmt.Errorf("compressed file is empty")
	}

	return float64(origInfo.Size()) / float64(compInfo.Size()), nil
}

// VerifyCompressedFile verifies a compressed WAL file can be decompressed
func (c *Compressor) VerifyCompressedFile(compressedPath string) error {
	file, err := os.Open(compressedPath)
	if err != nil {
		return fmt.Errorf("cannot open compressed file: %w", err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("invalid gzip format: %w", err)
	}
	defer gzReader.Close()

	// Read first few bytes to verify decompression works
	buf := make([]byte, 1024)
	_, err = gzReader.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("decompression verification failed: %w", err)
	}

	return nil
}
