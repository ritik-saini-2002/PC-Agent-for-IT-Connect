// Package auth provides PBKDF2 key verification and master key checks.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/pbkdf2"

	"github.com/ritik-saini/pc-agent/internal/config"
)

const (
	pbkdf2Iterations = 260000
	pbkdf2KeyLen     = 32
	saltLen          = 16
)

// VerifyPBKDF2 checks a password against a stored "salt_hex:dk_hex" hash.
func VerifyPBKDF2(password, storedHash string) bool {
	parts := strings.SplitN(storedHash, ":", 2)
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedDK, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	actualDK := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, pbkdf2KeyLen, sha256.New)
	return hmac.Equal(actualDK, expectedDK)
}

// HashPBKDF2 generates a "salt_hex:dk_hex" hash for a password.
func HashPBKDF2(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	dk := pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, pbkdf2KeyLen, sha256.New)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(dk), nil
}

// IsKeyValid checks if a key matches the secret key, master key, or their PBKDF2 hashes.
func IsKeyValid(key string) bool {
	cfg := config.Get()

	// Plain text match
	if key == cfg.SecretKey || key == cfg.MasterKey {
		return true
	}

	// PBKDF2 hash match
	if cfg.SecretKeyHash != "" && VerifyPBKDF2(key, cfg.SecretKeyHash) {
		return true
	}
	if cfg.MasterKeyHash != "" && VerifyPBKDF2(key, cfg.MasterKeyHash) {
		return true
	}

	return false
}

// IsMasterKey checks if a key is the master key (plain or hashed).
func IsMasterKey(key string) bool {
	cfg := config.Get()

	if key == cfg.MasterKey {
		return true
	}
	if cfg.MasterKeyHash != "" && VerifyPBKDF2(key, cfg.MasterKeyHash) {
		return true
	}
	return false
}
