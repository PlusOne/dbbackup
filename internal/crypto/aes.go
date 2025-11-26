package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// AES-256 requires 32-byte keys
	KeySize = 32

	// GCM standard nonce size
	NonceSize = 12

	// Salt size for PBKDF2
	SaltSize = 32

	// PBKDF2 iterations (OWASP recommended minimum)
	PBKDF2Iterations = 600000

	// Buffer size for streaming encryption
	BufferSize = 64 * 1024 // 64KB chunks
)

// AESEncryptor implements AES-256-GCM encryption
type AESEncryptor struct{}

// NewAESEncryptor creates a new AES-256-GCM encryptor
func NewAESEncryptor() *AESEncryptor {
	return &AESEncryptor{}
}

// Algorithm returns the algorithm name
func (e *AESEncryptor) Algorithm() EncryptionAlgorithm {
	return AlgorithmAES256GCM
}

// DeriveKey derives a 32-byte key from a password using PBKDF2-SHA256
func DeriveKey(password []byte, salt []byte) []byte {
	return pbkdf2.Key(password, salt, PBKDF2Iterations, KeySize, sha256.New)
}

// GenerateSalt generates a random salt
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

// GenerateNonce generates a random nonce for GCM
func GenerateNonce() ([]byte, error) {
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	return nonce, nil
}

// ValidateKey checks if a key is the correct length
func ValidateKey(key []byte) error {
	if len(key) != KeySize {
		return fmt.Errorf("invalid key length: expected %d bytes, got %d bytes", KeySize, len(key))
	}
	return nil
}

// Encrypt encrypts data from reader and returns an encrypted reader
func (e *AESEncryptor) Encrypt(reader io.Reader, key []byte) (io.Reader, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
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
	nonce, err := GenerateNonce()
	if err != nil {
		return nil, err
	}

	// Create pipe for streaming
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		// Write nonce first (needed for decryption)
		if _, err := pw.Write(nonce); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to write nonce: %w", err))
			return
		}

		// Read plaintext in chunks and encrypt
		buf := make([]byte, BufferSize)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				// Encrypt chunk
				ciphertext := gcm.Seal(nil, nonce, buf[:n], nil)

				// Write encrypted chunk length (4 bytes) + encrypted data
				lengthBuf := []byte{
					byte(len(ciphertext) >> 24),
					byte(len(ciphertext) >> 16),
					byte(len(ciphertext) >> 8),
					byte(len(ciphertext)),
				}
				if _, err := pw.Write(lengthBuf); err != nil {
					pw.CloseWithError(fmt.Errorf("failed to write chunk length: %w", err))
					return
				}
				if _, err := pw.Write(ciphertext); err != nil {
					pw.CloseWithError(fmt.Errorf("failed to write ciphertext: %w", err))
					return
				}

				// Increment nonce for next chunk (simple counter mode)
				for i := len(nonce) - 1; i >= 0; i-- {
					nonce[i]++
					if nonce[i] != 0 {
						break
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				pw.CloseWithError(fmt.Errorf("read error: %w", err))
				return
			}
		}
	}()

	return pr, nil
}

// Decrypt decrypts data from reader and returns a decrypted reader
func (e *AESEncryptor) Decrypt(reader io.Reader, key []byte) (io.Reader, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
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

	// Create pipe for streaming
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		// Read initial nonce
		nonce := make([]byte, NonceSize)
		if _, err := io.ReadFull(reader, nonce); err != nil {
			pw.CloseWithError(fmt.Errorf("failed to read nonce: %w", err))
			return
		}

		// Read and decrypt chunks
		lengthBuf := make([]byte, 4)
		for {
			// Read chunk length
			if _, err := io.ReadFull(reader, lengthBuf); err != nil {
				if err == io.EOF {
					break
				}
				pw.CloseWithError(fmt.Errorf("failed to read chunk length: %w", err))
				return
			}

			chunkLen := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 |
				int(lengthBuf[2])<<8 | int(lengthBuf[3])

			// Read encrypted chunk
			ciphertext := make([]byte, chunkLen)
			if _, err := io.ReadFull(reader, ciphertext); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to read ciphertext: %w", err))
				return
			}

			// Decrypt chunk
			plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("decryption failed (wrong key?): %w", err))
				return
			}

			// Write plaintext
			if _, err := pw.Write(plaintext); err != nil {
				pw.CloseWithError(fmt.Errorf("failed to write plaintext: %w", err))
				return
			}

			// Increment nonce for next chunk
			for i := len(nonce) - 1; i >= 0; i-- {
				nonce[i]++
				if nonce[i] != 0 {
					break
				}
			}
		}
	}()

	return pr, nil
}

// EncryptFile encrypts a file
func (e *AESEncryptor) EncryptFile(inputPath, outputPath string, key []byte) error {
	// Open input file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Encrypt
	encReader, err := e.Encrypt(inFile, key)
	if err != nil {
		return err
	}

	// Copy encrypted data to output file
	if _, err := io.Copy(outFile, encReader); err != nil {
		return fmt.Errorf("failed to write encrypted data: %w", err)
	}

	return nil
}

// DecryptFile decrypts a file
func (e *AESEncryptor) DecryptFile(inputPath, outputPath string, key []byte) error {
	// Open input file
	inFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inFile.Close()

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Decrypt
	decReader, err := e.Decrypt(inFile, key)
	if err != nil {
		return err
	}

	// Copy decrypted data to output file
	if _, err := io.Copy(outFile, decReader); err != nil {
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	return nil
}
