package crypto

import (
	"io"
)

// EncryptionAlgorithm represents the encryption algorithm used
type EncryptionAlgorithm string

const (
	AlgorithmAES256GCM EncryptionAlgorithm = "aes-256-gcm"
)

// EncryptionConfig holds encryption configuration
type EncryptionConfig struct {
	// Enabled indicates whether encryption is enabled
	Enabled bool

	// KeyFile is the path to a file containing the encryption key
	KeyFile string

	// KeyEnvVar is the name of an environment variable containing the key
	KeyEnvVar string

	// Algorithm specifies the encryption algorithm to use
	Algorithm EncryptionAlgorithm

	// Key is the actual encryption key (derived from KeyFile or KeyEnvVar)
	Key []byte
}

// Encryptor provides encryption and decryption capabilities
type Encryptor interface {
	// Encrypt encrypts data from reader and returns an encrypted reader
	// The returned reader streams encrypted data without loading everything into memory
	Encrypt(reader io.Reader, key []byte) (io.Reader, error)

	// Decrypt decrypts data from reader and returns a decrypted reader
	// The returned reader streams decrypted data without loading everything into memory
	Decrypt(reader io.Reader, key []byte) (io.Reader, error)

	// EncryptFile encrypts a file in-place or to a new file
	EncryptFile(inputPath, outputPath string, key []byte) error

	// DecryptFile decrypts a file in-place or to a new file
	DecryptFile(inputPath, outputPath string, key []byte) error

	// Algorithm returns the encryption algorithm used by this encryptor
	Algorithm() EncryptionAlgorithm
}

// KeyDeriver derives encryption keys from passwords/passphrases
type KeyDeriver interface {
	// DeriveKey derives a key from a password using PBKDF2 or similar
	DeriveKey(password []byte, salt []byte, keyLength int) ([]byte, error)

	// GenerateSalt generates a random salt for key derivation
	GenerateSalt() ([]byte, error)
}

// EncryptionMetadata contains metadata about encrypted backups
type EncryptionMetadata struct {
	// Algorithm used for encryption
	Algorithm string `json:"algorithm"`

	// KeyDerivation method used (e.g., "pbkdf2-sha256")
	KeyDerivation string `json:"key_derivation,omitempty"`

	// Salt used for key derivation (base64 encoded)
	Salt string `json:"salt,omitempty"`

	// Nonce/IV used for encryption (base64 encoded)
	Nonce string `json:"nonce,omitempty"`

	// Version of encryption format
	Version int `json:"version"`
}

// DefaultConfig returns a default encryption configuration
func DefaultConfig() *EncryptionConfig {
	return &EncryptionConfig{
		Enabled:   false,
		Algorithm: AlgorithmAES256GCM,
		KeyEnvVar: "DBBACKUP_ENCRYPTION_KEY",
	}
}
