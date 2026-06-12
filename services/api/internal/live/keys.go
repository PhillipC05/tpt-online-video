package live

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// GenerateStreamKey generates a cryptographically random stream key.
// Returns the plaintext key (to show the user once) and its SHA-256 hash.
func GenerateStreamKey() (plaintext string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate stream key: %w", err)
	}
	plaintext = hex.EncodeToString(buf)

	// Hash the key for storage
	sum := sha256.Sum256([]byte(plaintext))
	hash = hex.EncodeToString(sum[:])

	return plaintext, hash, nil
}

// HashStreamKey hashes a plaintext stream key for comparison.
func HashStreamKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}