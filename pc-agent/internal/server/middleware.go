// Package server provides the HTTP middleware chain for the agent.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ritik-saini/pc-agent/internal/auth"
	"github.com/ritik-saini/pc-agent/internal/logger"
)

// Global rate limiter — 1 req/s with burst of 60 (≈ 60 req/min)
var limiter = auth.NewRateLimiter(1.0, 60)

// connTracker is the connection tracking manager
var connTracker = NewConnectionTracker()

// jsonError writes a JSON error response.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": msg,
	})
}

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(data)
}

// authMiddleware checks API key authentication.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public endpoints — no auth required
		path := r.URL.Path
		if path == "/" || path == "/ping" || path == "/status" || path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract key from header or query param
		key := r.Header.Get("X-Secret-Key")
		if key == "" {
			key = r.URL.Query().Get("key")
		}
		key = strings.TrimSpace(key)

		clientIP := auth.GetClientIP(r)

		// Check if IP is blocked
		if limiter.IsBlocked(clientIP) {
			jsonError(w, "Too many auth failures — IP blocked for 15 minutes", 429)
			return
		}

		if !auth.IsKeyValid(key) {
			blocked := limiter.RecordAuthFailure(clientIP)
			if blocked {
				logger.Warn("IP %s blocked after 10 auth failures", clientIP)
			}
			jsonError(w, "Unauthorized — invalid key", 401)
			return
		}

		// Successful auth — reset failure counter and track connection
		limiter.ResetAuthFailures(clientIP)
		connTracker.Register(r)

		next.ServeHTTP(w, r)
	})
}

// rateLimitMiddleware enforces per-IP rate limiting.
func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exempt streaming endpoints from rate limiting
		path := r.URL.Path
		if strings.HasPrefix(path, "/screen/stream") ||
			strings.HasPrefix(path, "/audio/stream") ||
			strings.HasPrefix(path, "/clipboard/stream") ||
			strings.HasPrefix(path, "/screen/viewer") {
			next.ServeHTTP(w, r)
			return
		}

		clientIP := auth.GetClientIP(r)
		if !limiter.Allow(clientIP) {
			jsonError(w, "Rate limit exceeded", 429)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// panicRecoveryMiddleware catches panics in handlers and returns 500.
func panicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				stack := debug.Stack()
				logger.Error("PANIC in %s %s: %v\n%s", r.Method, r.URL.Path, rec, string(stack))
				jsonError(w, fmt.Sprintf("Internal server error: %v", rec), 500)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// requestLogMiddleware logs each request with timing.
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		next.ServeHTTP(w, r)
		elapsed := time.Since(t0)

		// Skip noisy streaming endpoints in logs
		path := r.URL.Path
		if strings.HasPrefix(path, "/screen/stream") ||
			strings.HasPrefix(path, "/audio/stream") {
			return
		}

		logger.Info("%s %s from %s (%dms)",
			r.Method, path, auth.GetClientIP(r), elapsed.Milliseconds())
	})
}

// corsMiddleware adds CORS headers for cross-origin access from admin viewer.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Secret-Key, X-Device-Name, X-Device-Id, X-User-Name, X-User-Email, X-User-Role, X-User-Company")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ChainMiddleware applies all middleware in order.
func ChainMiddleware(handler http.Handler) http.Handler {
	return panicRecoveryMiddleware(
		corsMiddleware(
			requestLogMiddleware(
				authMiddleware(
					rateLimitMiddleware(handler)))))
}

// ChainStreamMiddleware applies middleware for the stream server (auth + panic + CORS only).
func ChainStreamMiddleware(handler http.Handler) http.Handler {
	return panicRecoveryMiddleware(
		corsMiddleware(
			authMiddleware(handler)))
}
