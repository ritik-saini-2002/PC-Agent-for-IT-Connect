// Package server — connection tracking (port of Python _connected_users).
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ritik-saini/pc-agent/internal/auth"
)

// ConnInfo holds information about a connected device.
type ConnInfo struct {
	DeviceName  string `json:"device_name"`
	DeviceID    string `json:"device_id"`
	UserName    string `json:"user_name"`
	UserEmail   string `json:"user_email"`
	UserRole    string `json:"user_role"`
	UserCompany string `json:"user_company"`
	IP          string `json:"ip"`
	ConnectedAt string `json:"connected_at"`
	LastSeen    string `json:"last_seen"`
}

// ConnectionTracker manages connected devices.
type ConnectionTracker struct {
	users  map[string]*ConnInfo
	mu     sync.RWMutex
	logDir string
}

// GetConnTracker returns the global connection tracker.
func GetConnTracker() *ConnectionTracker {
	return connTracker
}

// NewConnectionTracker creates a new tracker.
func NewConnectionTracker() *ConnectionTracker {
	return &ConnectionTracker{
		users: make(map[string]*ConnInfo),
	}
}

// SetLogDir sets the connection log directory.
func (ct *ConnectionTracker) SetLogDir(dir string) {
	ct.logDir = dir
	os.MkdirAll(dir, 0755)
}

// Register tracks a new connection from an HTTP request.
func (ct *ConnectionTracker) Register(r *http.Request) {
	deviceName := r.Header.Get("X-Device-Name")
	if deviceName == "" {
		deviceName = "Unknown"
	}
	deviceID := r.Header.Get("X-Device-Id")
	if deviceID == "" {
		deviceID = deviceName
	}

	now := time.Now().Format(time.RFC3339)
	ip := auth.GetClientIP(r)

	ct.mu.Lock()
	_, isNew := ct.users[deviceID]
	existing := ct.users[deviceID]
	connAt := now
	if existing != nil {
		connAt = existing.ConnectedAt
	}

	ct.users[deviceID] = &ConnInfo{
		DeviceName:  deviceName,
		DeviceID:    deviceID,
		UserName:    r.Header.Get("X-User-Name"),
		UserEmail:   r.Header.Get("X-User-Email"),
		UserRole:    r.Header.Get("X-User-Role"),
		UserCompany: r.Header.Get("X-User-Company"),
		IP:          ip,
		ConnectedAt: connAt,
		LastSeen:    now,
	}
	ct.mu.Unlock()

	if !isNew {
		ct.writeLog(deviceID, "CONNECTED", map[string]string{
			"device_name": deviceName,
			"user_name":   r.Header.Get("X-User-Name"),
			"ip":          ip,
			"time":        now,
		})
	}
}

// Disconnect removes a device and logs the event.
func (ct *ConnectionTracker) Disconnect(deviceID string) bool {
	ct.mu.Lock()
	user, ok := ct.users[deviceID]
	if ok {
		delete(ct.users, deviceID)
	}
	ct.mu.Unlock()

	if ok && user != nil {
		ct.writeLog(deviceID, "DISCONNECTED", map[string]string{
			"user_name": user.UserName,
			"reason":    "forced_by_master",
		})
	}
	return ok
}

// List returns all currently connected users.
func (ct *ConnectionTracker) List() []*ConnInfo {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	users := make([]*ConnInfo, 0, len(ct.users))
	for _, u := range ct.users {
		users = append(users, u)
	}
	return users
}

// Count returns the number of connected users.
func (ct *ConnectionTracker) Count() int {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	return len(ct.users)
}

// PruneStale removes connections that haven't been seen in over 5 minutes.
func (ct *ConnectionTracker) PruneStale() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for id, info := range ct.users {
		lastSeen, err := time.Parse(time.RFC3339, info.LastSeen)
		if err != nil || lastSeen.Before(cutoff) {
			delete(ct.users, id)
		}
	}
}

// writeLog appends a JSON event to the device's connection log file.
func (ct *ConnectionTracker) writeLog(deviceID, event string, data map[string]string) {
	if ct.logDir == "" {
		return
	}
	// Sanitize device ID for filename
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, deviceID)
	if len(safe) > 80 {
		safe = safe[:80]
	}

	logFile := filepath.Join(ct.logDir, safe+".log")

	entry := map[string]string{
		"event":     event,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	for k, v := range data {
		entry[k] = v
	}

	line, _ := json.Marshal(entry)

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(line)
	f.WriteString("\n")
}
