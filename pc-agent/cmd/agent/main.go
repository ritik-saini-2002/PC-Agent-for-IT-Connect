package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
	"github.com/ritik-saini/pc-agent/internal/capture"
	"github.com/ritik-saini/pc-agent/internal/config"
	"github.com/ritik-saini/pc-agent/internal/logger"
	"github.com/ritik-saini/pc-agent/internal/server"
)

type agentService struct{}

func (m *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	
	// Start the actual logic in background
	ctx, cancel := context.WithCancel(context.Background())
	go runAgentLogic(ctx)

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cancel()
				break loop
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func main() {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		isInteractive = true
	}

	if !isInteractive {
		svc.Run("PCCommandAgent", &agentService{})
		return
	}

	// Interactive mode (double clicked)
	runAgentLogic(context.Background())
}

func runAgentLogic(ctx context.Context) {
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

	// Auto-install: firewall and autorun
	go autoSetup()

	logger.Info("==============================================================")
	logger.Info("  PC Command Agent v12.0-go  [Single Binary - Zero Dependencies]")
	logger.Info("==============================================================")
	logger.Info("  IP Address   : %s", localIP)
	logger.Info("  Command Port : %d  (HTTP)", config.DefaultPort)
	logger.Info("  Stream Port  : %d  (MJPEG + Audio)", config.DefaultStreamPort)
	logger.Info("  Secret Key   : %s", cfg.SecretKey)
	logger.Info("  Master Key   : %s***", cfg.MasterKey[:4])
	logger.Info("  Chunk Size   : %d MB", config.ChunkSize/1024/1024)
	logger.Info("  Browser view : http://%s:%d/screen/viewer?key=%s", localIP, config.DefaultStreamPort, cfg.SecretKey)
	logger.Info("==============================================================")

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

	// Wait for shutdown signal or context cancellation
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
	case <-ctx.Done():
	}

	logger.Info("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	apiServer.Shutdown(shutdownCtx)
	streamServer.Shutdown(shutdownCtx)
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

func autoSetup() {
	exe, err := os.Executable()
	if err != nil {
		return
	}

	// 1. Add to Windows Defender Firewall (runs silently, works if Admin)
	exec.Command("netsh", "advfirewall", "firewall", "add", "rule", 
		"name=PC Command Agent API", "dir=in", "action=allow", "protocol=TCP", "localport=5000").Run()
	exec.Command("netsh", "advfirewall", "firewall", "add", "rule", 
		"name=PC Command Agent Stream", "dir=in", "action=allow", "protocol=TCP", "localport=5001").Run()

	// 2. Add to Registry for Autorun (Current User - works without Admin)
	psCmd := fmt.Sprintf(`New-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name "PCCommandAgent" -Value '"%s"' -PropertyType String -Force`, exe)
	exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", psCmd).Run()
}
