// Package config manages agent_config.json — keys, connection settings, etc.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds all persistent agent configuration.
type Config struct {
	SecretKey     string `json:"secret_key,omitempty"`
	SecretKeyHash string `json:"secret_key_hash,omitempty"`
	MasterKey     string `json:"master_key,omitempty"`
	MasterKeyHash string `json:"master_key_hash,omitempty"`
	Updated       string `json:"updated,omitempty"`
}

// Defaults — same as Python agent_v12.py
const (
	DefaultSecretKey = "Saini@2004"
	DefaultMasterKey = "Ritik@2004"
	DefaultPort      = 5000
	DefaultStreamPort = 5001
	DefaultHost      = "0.0.0.0"
	ChunkSize        = 4 * 1024 * 1024        // 4MB
	SocketBufSize    = 16 * 1024 * 1024        // 16MB
	RequestTimeout   = 30                       // seconds
	FlaskThreads     = 64                       // goroutines handle this natively
)

var (
	current  Config
	mu       sync.RWMutex
	confPath string
)

// Load reads config from disk. If the file doesn't exist, defaults are used.
func Load(agentDir string) error {
	confPath = filepath.Join(agentDir, "agent_config.json")

	mu.Lock()
	defer mu.Unlock()

	// Start with defaults
	current = Config{
		SecretKey: DefaultSecretKey,
		MasterKey: DefaultMasterKey,
	}

	data, err := os.ReadFile(confPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config yet — use defaults
		}
		return err
	}

	var disk Config
	if err := json.Unmarshal(data, &disk); err != nil {
		return err
	}

	// Merge disk values over defaults
	if disk.SecretKey != "" {
		current.SecretKey = disk.SecretKey
	}
	if disk.SecretKeyHash != "" {
		current.SecretKeyHash = disk.SecretKeyHash
	}
	if disk.MasterKey != "" {
		current.MasterKey = disk.MasterKey
	}
	if disk.MasterKeyHash != "" {
		current.MasterKeyHash = disk.MasterKeyHash
	}

	return nil
}

// Save writes the current config to disk with 0600 permissions.
func Save() error {
	mu.RLock()
	c := current
	mu.RUnlock()

	c.Updated = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(confPath, data, 0600)
}

// Get returns a copy of the current config.
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetSecretKey updates the secret key and saves to disk.
func SetSecretKey(key string) error {
	mu.Lock()
	current.SecretKey = key
	mu.Unlock()
	return Save()
}

// SetSecretKeyHash updates the PBKDF2 hash of the secret key.
func SetSecretKeyHash(hash string) {
	mu.Lock()
	current.SecretKeyHash = hash
	mu.Unlock()
}

// SetMasterKeyHash updates the PBKDF2 hash of the master key.
func SetMasterKeyHash(hash string) {
	mu.Lock()
	current.MasterKeyHash = hash
	mu.Unlock()
}
