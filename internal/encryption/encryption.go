package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// AES-256 requires 32-byte keys
	KeySize = 32
	
	// Nonce size for GCM
	NonceSize = 12
	
	// Salt size for key derivation
	SaltSize = 32
	
	// PBKDF2 iterations (100,000 is recommended minimum)
	PBKDF2Iterations = 100000
	
	// Magic header to identify encrypted files
	EncryptedFileMagic = "DBBACKUP_ENCRYPTED_V1"
)

// EncryptionHeader stores metadata for encrypted files
type EncryptionHeader struct {
	Magic      [22]byte // "DBBACKUP_ENCRYPTED_V1" (21 bytes + null)
	Version    uint8    // Version number (1)
	Algorithm  uint8    // Algorithm ID (1 = AES-256-GCM)
	Salt       [32]byte // Salt for key derivation
	Nonce      [12]byte // GCM nonce
	Reserved   [32]byte // Reserved for future use
}

// EncryptionOptions configures encryption behavior
type EncryptionOptions struct {
	// Key is the encryption key (32 bytes for AES-256)
	Key []byte
	
	// Passphrase for key derivation (alternative to direct key)
	Passphrase string
	
	// Salt for key derivation (if empty, will be generated)
	Salt []byte
}

// DeriveKey derives an encryption key from a passphrase using PBKDF2
func DeriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, PBKDF2Iterations, KeySize, sha256.New)
}

// GenerateSalt creates a cryptographically secure random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateKey creates a cryptographically secure random key
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// NewEncryptionWriter creates an encrypted writer that wraps an underlying writer
// Data written to this writer will be encrypted before being written to the underlying writer
func NewEncryptionWriter(w io.Writer, opts EncryptionOptions) (*EncryptionWriter, error) {
	// Derive or validate key
	var key []byte
	var salt []byte
	
	if opts.Passphrase != "" {
		// Derive key from passphrase
		if len(opts.Salt) == 0 {
			var err error
			salt, err = GenerateSalt()
			if err != nil {
				return nil, err
			}
		} else {
			salt = opts.Salt
		}
		key = DeriveKey(opts.Passphrase, salt)
	} else if len(opts.Key) > 0 {
		if len(opts.Key) != KeySize {
			return nil, fmt.Errorf("invalid key size: expected %d bytes, got %d", KeySize, len(opts.Key))
		}
		key = opts.Key
		// Generate salt even when using direct key (for header)
		var err error
		salt, err = GenerateSalt()
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("either Key or Passphrase must be provided")
	}
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	// Generate nonce
	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	// Write header
	header := EncryptionHeader{
		Version:   1,
		Algorithm: 1, // AES-256-GCM
	}
	copy(header.Magic[:], []byte(EncryptedFileMagic))
	copy(header.Salt[:], salt)
	copy(header.Nonce[:], nonce)
	
	if err := writeHeader(w, &header); err != nil {
		return nil, fmt.Errorf("failed to write header: %w", err)
	}
	
	return &EncryptionWriter{
		writer: w,
		gcm:    gcm,
		nonce:  nonce,
		buffer: make([]byte, 0, 64*1024), // 64KB buffer
	}, nil
}

// EncryptionWriter encrypts data written to it
type EncryptionWriter struct {
	writer io.Writer
	gcm    cipher.AEAD
	nonce  []byte
	buffer []byte
	closed bool
}

// Write encrypts and writes data
func (ew *EncryptionWriter) Write(p []byte) (n int, err error) {
	if ew.closed {
		return 0, fmt.Errorf("writer is closed")
	}
	
	// Accumulate data in buffer
	ew.buffer = append(ew.buffer, p...)
	
	// If buffer is large enough, encrypt and write
	const chunkSize = 64 * 1024 // 64KB chunks
	for len(ew.buffer) >= chunkSize {
		chunk := ew.buffer[:chunkSize]
		encrypted := ew.gcm.Seal(nil, ew.nonce, chunk, nil)
		
		// Write encrypted chunk size (4 bytes) then chunk
		size := uint32(len(encrypted))
		sizeBytes := []byte{
			byte(size >> 24),
			byte(size >> 16),
			byte(size >> 8),
			byte(size),
		}
		if _, err := ew.writer.Write(sizeBytes); err != nil {
			return n, err
		}
		if _, err := ew.writer.Write(encrypted); err != nil {
			return n, err
		}
		
		// Move remaining data to start of buffer
		ew.buffer = ew.buffer[chunkSize:]
		n += chunkSize
		
		// Increment nonce for next chunk
		incrementNonce(ew.nonce)
	}
	
	return len(p), nil
}

// Close flushes remaining data and finalizes encryption
func (ew *EncryptionWriter) Close() error {
	if ew.closed {
		return nil
	}
	ew.closed = true
	
	// Encrypt and write remaining buffer
	if len(ew.buffer) > 0 {
		encrypted := ew.gcm.Seal(nil, ew.nonce, ew.buffer, nil)
		
		size := uint32(len(encrypted))
		sizeBytes := []byte{
			byte(size >> 24),
			byte(size >> 16),
			byte(size >> 8),
			byte(size),
		}
		if _, err := ew.writer.Write(sizeBytes); err != nil {
			return err
		}
		if _, err := ew.writer.Write(encrypted); err != nil {
			return err
		}
	}
	
	// Write final zero-length chunk to signal end
	if _, err := ew.writer.Write([]byte{0, 0, 0, 0}); err != nil {
		return err
	}
	
	return nil
}

// NewDecryptionReader creates a decrypted reader from an encrypted stream
func NewDecryptionReader(r io.Reader, opts EncryptionOptions) (*DecryptionReader, error) {
	// Read and parse header
	header, err := readHeader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	
	// Verify magic
	if string(header.Magic[:len(EncryptedFileMagic)]) != EncryptedFileMagic {
		return nil, fmt.Errorf("not an encrypted backup file")
	}
	
	// Verify version
	if header.Version != 1 {
		return nil, fmt.Errorf("unsupported encryption version: %d", header.Version)
	}
	
	// Verify algorithm
	if header.Algorithm != 1 {
		return nil, fmt.Errorf("unsupported encryption algorithm: %d", header.Algorithm)
	}
	
	// Derive or validate key
	var key []byte
	if opts.Passphrase != "" {
		key = DeriveKey(opts.Passphrase, header.Salt[:])
	} else if len(opts.Key) > 0 {
		if len(opts.Key) != KeySize {
			return nil, fmt.Errorf("invalid key size: expected %d bytes, got %d", KeySize, len(opts.Key))
		}
		key = opts.Key
	} else {
		return nil, fmt.Errorf("either Key or Passphrase must be provided")
	}
	
	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonce := make([]byte, NonceSize)
	copy(nonce, header.Nonce[:])
	
	return &DecryptionReader{
		reader: r,
		gcm:    gcm,
		nonce:  nonce,
		buffer: make([]byte, 0),
	}, nil
}

// DecryptionReader decrypts data from an encrypted stream
type DecryptionReader struct {
	reader io.Reader
	gcm    cipher.AEAD
	nonce  []byte
	buffer []byte
	eof    bool
}

// Read decrypts and returns data
func (dr *DecryptionReader) Read(p []byte) (n int, err error) {
	// If we have buffered data, return it first
	if len(dr.buffer) > 0 {
		n = copy(p, dr.buffer)
		dr.buffer = dr.buffer[n:]
		return n, nil
	}
	
	// If EOF reached, return EOF
	if dr.eof {
		return 0, io.EOF
	}
	
	// Read next chunk size
	sizeBytes := make([]byte, 4)
	if _, err := io.ReadFull(dr.reader, sizeBytes); err != nil {
		if err == io.EOF {
			dr.eof = true
			return 0, io.EOF
		}
		return 0, err
	}
	
	size := uint32(sizeBytes[0])<<24 | uint32(sizeBytes[1])<<16 | uint32(sizeBytes[2])<<8 | uint32(sizeBytes[3])
	
	// Zero-length chunk signals end of stream
	if size == 0 {
		dr.eof = true
		return 0, io.EOF
	}
	
	// Read encrypted chunk
	encrypted := make([]byte, size)
	if _, err := io.ReadFull(dr.reader, encrypted); err != nil {
		return 0, err
	}
	
	// Decrypt chunk
	decrypted, err := dr.gcm.Open(nil, dr.nonce, encrypted, nil)
	if err != nil {
		return 0, fmt.Errorf("decryption failed (wrong key?): %w", err)
	}
	
	// Increment nonce for next chunk
	incrementNonce(dr.nonce)
	
	// Return as much as fits in p, buffer the rest
	n = copy(p, decrypted)
	if n < len(decrypted) {
		dr.buffer = decrypted[n:]
	}
	
	return n, nil
}

// Helper functions

func writeHeader(w io.Writer, h *EncryptionHeader) error {
	data := make([]byte, 100) // Total header size
	copy(data[0:22], h.Magic[:])
	data[22] = h.Version
	data[23] = h.Algorithm
	copy(data[24:56], h.Salt[:])
	copy(data[56:68], h.Nonce[:])
	copy(data[68:100], h.Reserved[:])
	
	_, err := w.Write(data)
	return err
}

func readHeader(r io.Reader) (*EncryptionHeader, error) {
	data := make([]byte, 100)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	
	header := &EncryptionHeader{
		Version:   data[22],
		Algorithm: data[23],
	}
	copy(header.Magic[:], data[0:22])
	copy(header.Salt[:], data[24:56])
	copy(header.Nonce[:], data[56:68])
	copy(header.Reserved[:], data[68:100])
	
	return header, nil
}

func incrementNonce(nonce []byte) {
	// Increment nonce as a big-endian counter
	for i := len(nonce) - 1; i >= 0; i-- {
		nonce[i]++
		if nonce[i] != 0 {
			break
		}
	}
}
