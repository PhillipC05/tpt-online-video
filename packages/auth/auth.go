package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidHash        = errors.New("invalid argon2 hash")
	ErrMismatchedHash     = errors.New("mismatched hash")
	ErrInvalidTokenLength = errors.New("invalid token length")
)

type PasswordHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{
		memory:      64 * 1024,
		iterations:  3,
		parallelism: 2,
		saltLength:  16,
		keyLength:   32,
	}
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	salt, err := randomBytes(h.saltLength)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLength)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		h.memory,
		h.iterations,
		h.parallelism,
		encodedSalt,
		encodedHash,
	), nil
}

func (h *PasswordHasher) Compare(password, encodedHash string) bool {
	parts, err := parseHash(encodedHash)
	if err != nil {
		return false
	}

	decodedSalt, err := base64.RawStdEncoding.DecodeString(parts.salt)
	if err != nil {
		return false
	}

	decodedHash, err := base64.RawStdEncoding.DecodeString(parts.hash)
	if err != nil {
		return false
	}

	otherHash := argon2.IDKey([]byte(password), decodedSalt, parts.iterations, parts.memory, parts.parallelism, uint32(len(decodedHash)))
	decodedHash64 := make([]byte, base64.RawStdEncoding.DecodedLen(len(encodedHash)))
	base64.RawStdEncoding.Decode(decodedHash64, decodedHash)

	return subtle.ConstantTimeCompare(otherHash, decodedHash) == 1
}

type TokenManager struct{}

func NewTokenManager() *TokenManager {
	return &TokenManager{}
}

func (m *TokenManager) NewRandomToken() (string, string, error) {
	token, err := randomHex(32)
	if err != nil {
		return "", "", err
	}
	return token, HashToken(token), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type hashParts struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        string
	hash        string
}

func parseHash(encodedHash string) (*hashParts, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, ErrInvalidHash
	}

	versionParam := strings.TrimPrefix(parts[2], "v=")
	version, err := strconv.Atoi(versionParam)
	if err != nil || version != argon2.Version {
		return nil, ErrInvalidHash
	}

	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return nil, ErrInvalidHash
	}

	memory, err := strconv.ParseUint(strings.TrimPrefix(params[0], "m="), 10, 32)
	if err != nil {
		return nil, ErrInvalidHash
	}
	iterations, err := strconv.ParseUint(strings.TrimPrefix(params[1], "t="), 10, 32)
	if err != nil {
		return nil, ErrInvalidHash
	}
	parallelism, err := strconv.ParseUint(strings.TrimPrefix(params[2], "p="), 10, 8)
	if err != nil {
		return nil, ErrInvalidHash
	}

	return &hashParts{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
		salt:        parts[4],
		hash:        parts[5],
	}, nil
}

func randomBytes(length uint32) ([]byte, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return nil, fmt.Errorf("generate random bytes: %w", err)
	}
	return buffer, nil
}

func randomHex(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}