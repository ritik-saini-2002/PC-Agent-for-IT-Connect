package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ritik-saini/pc-agent/internal/capture"
	"github.com/ritik-saini/pc-agent/internal/config"
	"github.com/ritik-saini/pc-agent/internal/logger"
	"github.com/ritik-saini/pc-agent/internal/server"
)

func main() {
	// Determine agent directory
	exe, _ := os.Executable()
	agentDir := filepath.Dir(exe)

	// Init logger
	logger.Init(agentDir)
	defer logger.Close()

	// Load config
	if err := config.Load(agentDir); err != nil {
		logger.Error("Config load failed: %v", err)
	}
	cfg := config.Get()

	// Get local IP
	localIP := getLocalIP()

	logger.Info("══════════════════════════════════════════════════════════════")
	logger.Info("  PC Command Agent v12.0-go  [Single Binary — Zero Dependencies]")
	logger.Info("══════════════════════════════════════════════════════════════")
	logger.Info("  IP Address   : %s", localIP)
	logger.Info("  Command Port : %d  (HTTP)", config.DefaultPort)
	logger.Info("  Stream Port  : %d  (MJPEG + Audio)", config.DefaultStreamPort)
	logger.Info("  Secret Key   : %s", cfg.SecretKey)
	logger.Info("  Master Key   : %s***", cfg.MasterKey[:4])
	logger.Info("  Chunk Size   : %d MB", config.ChunkSize/1024/1024)
	logger.Info("  Browser view : http://%s:%d/screen/viewer?key=%s", localIP, config.DefaultStreamPort, cfg.SecretKey)
	logger.Info("══════════════════════════════════════════════════════════════")

	// Start background frame grabber
	capture.StartGrabber()

	// Setup connection log directory
	server.GetConnTracker().SetLogDir(filepath.Join(agentDir, "connection_logs"))

	// Port 5000 — Command API
	apiMux := http.NewServeMux()
	server.RegisterAPIRoutes(apiMux)
	apiServer := &http.Server{
		Addr:         ":5000",
		Handler:      server.ChainMiddleware(apiMux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Port 5001 — Stream + Audio
	streamMux := http.NewServeMux()
	server.RegisterStreamRoutes(streamMux)
	streamServer := &http.Server{
		Addr:         ":5001",
		Handler:      server.ChainStreamMiddleware(streamMux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // no timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	// Start both servers
	go func() {
		logger.Info("Starting Command API on %s:%d", config.DefaultHost, config.DefaultPort)
		ln, err := net.Listen("tcp", apiServer.Addr)
		if err != nil {
			logger.Error("Port 5000 listen failed: %v", err)
			os.Exit(1)
		}
		tuneSocket(ln)
		if err := apiServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("API server crashed: %v", err)
		}
	}()

	go func() {
		logger.Info("Starting Stream server on %s:%d", config.DefaultHost, config.DefaultStreamPort)
		ln, err := net.Listen("tcp", streamServer.Addr)
		if err != nil {
			logger.Error("Port 5001 listen failed: %v", err)
			os.Exit(1)
		}
		tuneSocket(ln)
		if err := streamServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error("Stream server crashed: %v", err)
		}
	}()

	// Start keep-alive worker
	go keepAliveWorker()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	apiServer.Shutdown(ctx)
	streamServer.Shutdown(ctx)
	logger.Info("Agent stopped.")
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func tuneSocket(ln net.Listener) {
	if tcpLn, ok := ln.(*net.TCPListener); ok {
		if raw, err := tcpLn.SyscallConn(); err == nil {
			raw.Control(func(fd uintptr) {
				syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 16*1024*1024)
				syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, 16*1024*1024)
			})
		}
	}
}

func keepAliveWorker() {
	for {
		time.Sleep(30 * time.Second)
		server.GetConnTracker().PruneStale()
		logger.Info("[HEARTBEAT] Uptime: %ds  IP:%s  Connected: %d",
			int(time.Since(time.Now()).Seconds()), getLocalIP(), server.GetConnTracker().Count())
	}
}
