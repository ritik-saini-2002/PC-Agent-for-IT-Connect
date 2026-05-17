// Package logger provides rotating file + stdout logging for the agent.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Log is the global logger used by all packages.
var Log *log.Logger

// logFile holds the open file handle for rotation.
var logFile *os.File

// Init initialises the logger. Logs are written to both stdout and a file
// in the agent directory. The file is simple append — rotation handled at
// the OS level or via external tooling to keep the binary dependency-free.
func Init(agentDir string) {
	logPath := filepath.Join(agentDir, "agent_log.txt")

	var err error
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		// Fall back to stdout only
		Log = log.New(os.Stdout, "", 0)
		Log.Printf("WARN: could not open log file %s: %v", logPath, err)
		return
	}

	multi := io.MultiWriter(os.Stdout, logFile)
	Log = log.New(multi, "", 0)
}

// Close flushes and closes the log file.
func Close() {
	if logFile != nil {
		logFile.Close()
	}
}

// timestamp returns a formatted timestamp matching Python agent format.
func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// Info logs an informational message.
func Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	Log.Printf("%s [INFO] %s", timestamp(), msg)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	Log.Printf("%s [WARN] %s", timestamp(), msg)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	Log.Printf("%s [ERROR] %s", timestamp(), msg)
}
