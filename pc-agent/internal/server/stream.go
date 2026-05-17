package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ritik-saini/pc-agent/internal/auth"
	"github.com/ritik-saini/pc-agent/internal/capture"
	"github.com/ritik-saini/pc-agent/internal/config"
	agentEmbed "github.com/ritik-saini/pc-agent/internal/embed"
)

// RegisterStreamRoutes sets up port 5001 routes.
func RegisterStreamRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/screen/stream", handleScreenStream)
	mux.HandleFunc("/screen/viewer", handleScreenViewer)
	mux.HandleFunc("/screen/viewer/admincontrol", handleAdminControl)
	mux.HandleFunc("/audio/stream", handleAudioStream)
	mux.HandleFunc("/audio/toggle", handleAudioToggle)
	mux.HandleFunc("/audio/status", handleAudioStatus)
}

func handleScreenStream(w http.ResponseWriter, r *http.Request) {
	q, _ := strconv.Atoi(r.URL.Query().Get("q"))
	if q == 0 { q = 75 }
	fps, _ := strconv.Atoi(r.URL.Query().Get("fps"))
	if fps == 0 { fps = 20 }

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary=frame")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("X-Accel-Buffering", "no")

	capture.IncrementClients()
	defer capture.DecrementClients()

	interval := time.Duration(float64(time.Second) / float64(fps))
	flusher, _ := w.(http.Flusher)

	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		t0 := time.Now()
		frame := capture.GetFrame()
		if frame == nil {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		_, err := fmt.Fprintf(w, "--frame\r\nContent-Type: image/jpeg\r\n\r\n")
		if err != nil { return }
		_, err = w.Write(frame)
		if err != nil { return }
		_, err = w.Write([]byte("\r\n"))
		if err != nil { return }
		if flusher != nil { flusher.Flush() }

		elapsed := time.Since(t0)
		if wait := interval - elapsed; wait > 0 {
			time.Sleep(wait)
		}
	}
}

func handleScreenViewer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(agentEmbed.ViewerHTML)
}

func handleAdminControl(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	cfg := config.Get()
	if !auth.IsMasterKey(key) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(403)
		w.Write([]byte(`<html><body style="background:#111;color:#f55;font-family:monospace;display:flex;align-items:center;justify-content:center;height:100vh;font-size:20px">⛔ Access Denied — Invalid Master Key</body></html>`))
		return
	}
	_ = cfg
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(agentEmbed.AdminControlHTML)
}

func handleAudioStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Audio-Source", "wasapi-loopback")
	// Audio WASAPI implementation will be added in Phase 5
	// For now return empty to maintain API compatibility
	<-r.Context().Done()
}

func handleAudioToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonError(w, "POST required", 405)
		return
	}
	// Placeholder — full WASAPI toggle in Phase 5
	jsonOK(w, map[string]interface{}{"ok": true, "enabled": false})
}

func handleAudioStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{"ok": true, "enabled": false, "streaming_clients": 0})
}
