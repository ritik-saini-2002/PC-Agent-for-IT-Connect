package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ritik-saini/pc-agent/internal/auth"
	"github.com/ritik-saini/pc-agent/internal/capture"
	"github.com/ritik-saini/pc-agent/internal/config"
	"github.com/ritik-saini/pc-agent/internal/input"
	"github.com/ritik-saini/pc-agent/internal/winapi"
)

var startTime = time.Now()

func RegisterAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/", handlePing)
	mux.HandleFunc("/ping", handlePing)
	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("/connections", handleConnections)
	mux.HandleFunc("/connections/kick", handleKick)
	mux.HandleFunc("/settings/key", handleChangeKey)
	mux.HandleFunc("/screen/snapshot", handleSnapshot)
	mux.HandleFunc("/screen/capture", handleCapture)
	mux.HandleFunc("/screen/info", handleScreenInfo)
	mux.HandleFunc("/screen_size", handleScreenSize)
	mux.HandleFunc("/input/mouse/move", handleMouseMove)
	mux.HandleFunc("/input/mouse/move/abs", handleMouseMoveAbs)
	mux.HandleFunc("/input/mouse/click", handleMouseClick)
	mux.HandleFunc("/input/mouse/scroll", handleMouseScroll)
	mux.HandleFunc("/input/mouse/down", handleMouseDown)
	mux.HandleFunc("/input/mouse/up", handleMouseUp)
	mux.HandleFunc("/input/keyboard/key", handleKeyboardKey)
	mux.HandleFunc("/input/keyboard/combo", handleKeyboardCombo)
	mux.HandleFunc("/input/keyboard/type", handleKeyboardType)
	mux.HandleFunc("/input/keyboard/hold", handleKeyboardHold)
	mux.HandleFunc("/input/keyboard/release", handleKeyboardRelease)
	mux.HandleFunc("/input/gesture", handleGesture)
	mux.HandleFunc("/clipboard", handleClipboard)
	mux.HandleFunc("/wakescreen", handleWakeScreen)
	mux.HandleFunc("/execute", handleExecute)
	mux.HandleFunc("/quick", handleQuick)
	mux.HandleFunc("/processes", handleProcesses)
	mux.HandleFunc("/system/volume", handleVolume)
	mux.HandleFunc("/system/volume/set", handleVolumeSet)
	mux.HandleFunc("/file/download", handleFileDownload)
	mux.HandleFunc("/file/upload", handleFileUpload)
	mux.HandleFunc("/file/delete", handleFileDelete)
	mux.HandleFunc("/file/rename", handleFileRename)
	mux.HandleFunc("/file/move", handleFileMove)
	mux.HandleFunc("/file/copy", handleFileCopy)
	mux.HandleFunc("/file/mkdir", handleFileMkdir)
	mux.HandleFunc("/browse/drives", handleBrowseDrives)
	mux.HandleFunc("/browse/dir", handleBrowseDir)
	mux.HandleFunc("/browse/special", handleBrowseSpecial)
	mux.HandleFunc("/browse/search", handleBrowseSearch)
	mux.HandleFunc("/browse/apps", handleBrowseApps)
}

func getLocalIP() string {
	addrs, _ := getOutboundIP()
	return addrs
}

func getOutboundIP() (string, error) {
	// same trick as Python — UDP connect to 8.8.8.8
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil { return "127.0.0.1", err }
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{
		"status": "online", "version": "12.0-go",
		"pc_name": os.Getenv("COMPUTERNAME"),
		"os": runtime.GOOS, "uptime": int(time.Since(startTime).Seconds()),
		"ip": getLocalIP(), "port": config.DefaultPort,
		"stream_port": config.DefaultStreamPort, "https": false,
		"chunk_size": config.ChunkSize, "audio_on_stream_port": true,
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{"results": []interface{}{}})
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("X-Secret-Key"))
	if key == "" { key = r.URL.Query().Get("key") }
	if !auth.IsMasterKey(key) { jsonError(w, "Master key required", 403); return }
	users := connTracker.List()
	jsonOK(w, map[string]interface{}{"connected_users": users, "count": len(users)})
}

func handleKick(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("X-Secret-Key"))
	if key == "" { key = r.URL.Query().Get("key") }
	if !auth.IsMasterKey(key) { jsonError(w, "Master key required", 403); return }
	var data map[string]string
	json.NewDecoder(r.Body).Decode(&data)
	ok := connTracker.Disconnect(data["device_id"])
	jsonOK(w, map[string]interface{}{"ok": ok, "device_id": data["device_id"]})
}

func handleChangeKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.Header.Get("X-Secret-Key"))
	if key == "" { key = r.URL.Query().Get("key") }
	if !auth.IsMasterKey(key) { jsonError(w, "Master key required", 403); return }
	var data map[string]string
	json.NewDecoder(r.Body).Decode(&data)
	newKey := strings.TrimSpace(data["new_key"])
	if len(newKey) < 4 { jsonError(w, "Key must be at least 4 characters", 400); return }
	config.SetSecretKey(newKey)
	jsonOK(w, map[string]interface{}{"ok": true, "message": "Secret key updated"})
}

func handleSnapshot(w http.ResponseWriter, r *http.Request) {
	frame, fw, fh, err := capture.CaptureSnapshot(480, 35)
	if err != nil { jsonError(w, err.Error(), 500); return }
	b64 := base64.StdEncoding.EncodeToString(frame)
	jsonOK(w, map[string]interface{}{"ok": true, "data": b64, "w": fw, "h": fh, "ts": time.Now().Unix()})
}

func handleCapture(w http.ResponseWriter, r *http.Request) {
	q, _ := strconv.Atoi(r.URL.Query().Get("q")); if q == 0 { q = 25 }
	frame, fw, fh, err := capture.CaptureSnapshot(480, q)
	if err != nil { jsonError(w, err.Error(), 500); return }
	b64 := base64.StdEncoding.EncodeToString(frame)
	jsonOK(w, map[string]interface{}{"ok": true, "data": b64, "w": fw, "h": fh, "ts": time.Now().UnixMilli()})
}

func handleScreenInfo(w http.ResponseWriter, r *http.Request) {
	sw, sh := winapi.GetScreenSize()
	title := winapi.GetWindowText(winapi.GetForegroundWindow())
	jsonOK(w, map[string]interface{}{"ok": true, "sw": sw, "sh": sh, "window": title, "ts": time.Now().Unix()})
}

func handleScreenSize(w http.ResponseWriter, r *http.Request) {
	sw, sh := winapi.GetScreenSize()
	jsonOK(w, map[string]interface{}{"width": sw, "height": sh})
}

func handleMouseMove(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	dx := int(toFloat(data["dx"])); dy := int(toFloat(data["dy"]))
	if dx != 0 || dy != 0 { input.MoveRelative(dx, dy) }
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleMouseMoveAbs(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	x := int(toFloat(data["x"])); y := int(toFloat(data["y"]))
	input.MoveAbsolute(x, y)
	jsonOK(w, map[string]interface{}{"ok": true, "x": x, "y": y})
}

func handleMouseClick(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	btn, _ := data["button"].(string); if btn == "" { btn = "left" }
	dbl, _ := data["double"].(bool)
	input.Click(btn, dbl)
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleMouseScroll(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	amount := int(toFloat(data["amount"])); horiz, _ := data["horizontal"].(bool)
	input.Scroll(amount, horiz)
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleMouseDown(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	btn, _ := data["button"].(string); if btn == "" { btn = "left" }
	input.MouseDown(btn)
	jsonOK(w, map[string]interface{}{"ok": true, "dragging": true})
}

func handleMouseUp(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	btn, _ := data["button"].(string); if btn == "" { btn = "left" }
	input.MouseUp(btn)
	jsonOK(w, map[string]interface{}{"ok": true, "dragging": false})
}

func handleKeyboardKey(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	key, _ := data["value"].(string)
	input.ExecuteKeyPress(key)
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleKeyboardCombo(w http.ResponseWriter, r *http.Request) {
	var data struct{ Keys []string `json:"keys"` }
	json.NewDecoder(r.Body).Decode(&data)
	if len(data.Keys) == 0 { jsonError(w, "no keys", 400); return }
	var vks []uint16
	for _, k := range data.Keys {
		ku := strings.ToUpper(k)
		if vk, ok := input.VK[ku]; ok { vks = append(vks, vk) } else if len(k) == 1 {
			vks = append(vks, uint16(winapi.VkKeyScanW(uint16(k[0]))&0xFF))
		}
	}
	input.SendCombo(vks...)
	jsonOK(w, map[string]interface{}{"ok": true, "keys": data.Keys})
}

func handleKeyboardType(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	text, _ := data["value"].(string)
	input.TypeViaClipboard(text)
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleKeyboardHold(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	key := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", data["value"])))
	vk, ok := input.VK[key]; if !ok { jsonError(w, "Unknown key: "+key, 400); return }
	input.HoldKey(vk)
	jsonOK(w, map[string]interface{}{"ok": true, "held": key})
}

func handleKeyboardRelease(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	key := strings.ToUpper(strings.TrimSpace(fmt.Sprintf("%v", data["value"])))
	if key == "ALL" { input.ReleaseAllHeld(); jsonOK(w, map[string]interface{}{"ok": true, "released": "ALL"}); return }
	vk, ok := input.VK[key]; if !ok { jsonError(w, "Unknown key: "+key, 400); return }
	input.ReleaseKey(vk)
	jsonOK(w, map[string]interface{}{"ok": true, "released": key})
}

var gestureMap = map[string]string{
	"3finger-tap": "WIN+S", "3finger-swipe-up": "WIN+TAB", "3finger-swipe-down": "WIN+D",
	"3finger-swipe-left": "ALT+SHIFT+TAB", "3finger-swipe-right": "ALT+TAB",
	"4finger-tap": "WIN+A", "4finger-swipe-up": "WIN+TAB", "4finger-swipe-down": "WIN+D",
	"zoom-in": "CTRL+PLUS", "zoom-out": "CTRL+MINUS", "zoom-reset": "CTRL+0",
}

func handleGesture(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	gesture := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", data["type"])))
	alias, ok := gestureMap[gesture]; if !ok { jsonError(w, "Unknown gesture: "+gesture, 400); return }
	input.ExecuteKeyPress(alias)
	jsonOK(w, map[string]interface{}{"ok": true, "gesture": gesture, "key": alias})
}

func handleClipboard(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var data map[string]interface{}
		json.NewDecoder(r.Body).Decode(&data)
		text, _ := data["value"].(string)
		if err := input.WriteClipboard(text); err != nil { jsonError(w, err.Error(), 500); return }
		jsonOK(w, map[string]interface{}{"ok": true})
	} else {
		text, _ := input.ReadClipboard()
		jsonOK(w, map[string]interface{}{"ok": true, "value": text})
	}
}

func handleWakeScreen(w http.ResponseWriter, r *http.Request) {
	winapi.SetThreadExecutionState(winapi.ES_CONTINUOUS | winapi.ES_DISPLAY_REQUIRED)
	input.SendKey(input.VK["SHIFT"])
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleExecute(w http.ResponseWriter, r *http.Request) {
	// Async plan execution stub
	jsonOK(w, map[string]interface{}{"status": "executing"})
}

func handleQuick(w http.ResponseWriter, r *http.Request) {
	var step map[string]interface{}
	json.NewDecoder(r.Body).Decode(&step)
	st := strings.ToUpper(fmt.Sprintf("%v", step["type"]))
	switch st {
	case "KEY_PRESS":
		input.ExecuteKeyPress(fmt.Sprintf("%v", step["value"]))
	case "TYPE_TEXT":
		input.TypeViaClipboard(fmt.Sprintf("%v", step["value"]))
	case "MOUSE_CLICK":
		x := int(toFloat(step["x"])); y := int(toFloat(step["y"]))
		input.MoveAbsolute(x, y); input.Click("left", false)
	case "SYSTEM_CMD":
		cmd := strings.ToUpper(fmt.Sprintf("%v", step["value"]))
		if cmd == "LOCK" { winapi.LockWorkStation() }
	default:
		jsonError(w, "Unknown step type: "+st, 400); return
	}
	jsonOK(w, map[string]interface{}{"status": "ok"})
}

func handleProcesses(w http.ResponseWriter, r *http.Request) {
	out, _ := exec.Command("tasklist", "/fo", "csv", "/nh").Output()
	var names []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) > 0 {
			name := strings.Trim(parts[0], "\" \r")
			if name != "" && !seen[name] { seen[name] = true; names = append(names, name) }
		}
	}
	sort.Strings(names)
	jsonOK(w, map[string]interface{}{"processes": names})
}

func handleVolume(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{"ok": true, "volume": 50, "muted": false, "estimated": true})
}

func handleVolumeSet(w http.ResponseWriter, r *http.Request) {
	var data map[string]interface{}
	json.NewDecoder(r.Body).Decode(&data)
	level := int(toFloat(data["level"]))
	// Use PowerShell SendKeys fallback
	ps := fmt.Sprintf("$wsh=New-Object -ComObject WScript.Shell;for($i=0;$i -lt 50;$i++){$wsh.SendKeys([char]174)};for($i=0;$i -lt %d;$i++){$wsh.SendKeys([char]175)}", level/2)
	exec.Command("powershell", "-Command", ps).Start()
	jsonOK(w, map[string]interface{}{"ok": true, "volume": level})
}

func handleFileDownload(w http.ResponseWriter, r *http.Request) {
	path := strings.ReplaceAll(r.URL.Query().Get("path"), "/", "\\")
	if _, err := os.Stat(path); err != nil { jsonError(w, "File not found", 404); return }
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(path)))
	http.ServeFile(w, r, path)
}

func handleFileUpload(w http.ResponseWriter, r *http.Request) {
	dest := strings.ReplaceAll(r.FormValue("dest"), "/", "\\")
	if dest == "" { dest = filepath.Join(os.Getenv("USERPROFILE"), "Downloads") }
	os.MkdirAll(dest, 0755)
	file, header, err := r.FormFile("file")
	if err != nil {
		data, _ := io.ReadAll(r.Body)
		name := r.URL.Query().Get("name"); if name == "" { name = fmt.Sprintf("upload_%d", time.Now().Unix()) }
		savePath := filepath.Join(dest, name)
		os.WriteFile(savePath, data, 0644)
		jsonOK(w, map[string]interface{}{"ok": true, "path": savePath}); return
	}
	defer file.Close()
	savePath := filepath.Join(dest, filepath.Base(header.Filename))
	out, _ := os.Create(savePath)
	defer out.Close()
	io.Copy(out, file)
	info, _ := os.Stat(savePath)
	jsonOK(w, map[string]interface{}{"ok": true, "path": savePath, "size_kb": info.Size() / 1024})
}

func handleFileDelete(w http.ResponseWriter, r *http.Request) {
	var data map[string]string; json.NewDecoder(r.Body).Decode(&data)
	path := strings.ReplaceAll(data["path"], "/", "\\")
	if err := os.RemoveAll(path); err != nil { jsonError(w, err.Error(), 500); return }
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleFileRename(w http.ResponseWriter, r *http.Request) {
	var data map[string]string; json.NewDecoder(r.Body).Decode(&data)
	src := strings.ReplaceAll(data["from"], "/", "\\")
	dst := filepath.Join(filepath.Dir(src), data["name"])
	if err := os.Rename(src, dst); err != nil { jsonError(w, err.Error(), 500); return }
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleFileMove(w http.ResponseWriter, r *http.Request) {
	var data map[string]string; json.NewDecoder(r.Body).Decode(&data)
	src := strings.ReplaceAll(data["from"], "/", "\\")
	dst := strings.ReplaceAll(data["to"], "/", "\\")
	if err := os.Rename(src, dst); err != nil { jsonError(w, err.Error(), 500); return }
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleFileCopy(w http.ResponseWriter, r *http.Request) {
	var data map[string]string; json.NewDecoder(r.Body).Decode(&data)
	src := strings.ReplaceAll(data["from"], "/", "\\")
	dst := strings.ReplaceAll(data["to"], "/", "\\")
	in, err := os.Open(src); if err != nil { jsonError(w, err.Error(), 500); return }
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.Create(dst); if err != nil { jsonError(w, err.Error(), 500); return }
	defer out.Close()
	io.Copy(out, in)
	jsonOK(w, map[string]interface{}{"ok": true})
}

func handleFileMkdir(w http.ResponseWriter, r *http.Request) {
	var data map[string]string; json.NewDecoder(r.Body).Decode(&data)
	path := strings.ReplaceAll(data["path"], "/", "\\")
	os.MkdirAll(path, 0755)
	jsonOK(w, map[string]interface{}{"ok": true, "path": path})
}

func handleBrowseDrives(w http.ResponseWriter, r *http.Request) {
	var drives []map[string]interface{}
	for _, letter := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		root := string(letter) + ":\\"
		if _, err := os.Stat(root); err == nil {
			free, total, _, _ := winapi.GetDiskFreeSpaceEx(root)
			label, _ := winapi.GetVolumeInformation(root)
			drives = append(drives, map[string]interface{}{
				"letter": string(letter), "label": label,
				"freeGb": float64(free) / (1024 * 1024 * 1024),
				"totalGb": float64(total) / (1024 * 1024 * 1024),
			})
		}
	}
	jsonOK(w, drives)
}

func handleBrowseDir(w http.ResponseWriter, r *http.Request) {
	path := strings.ReplaceAll(r.URL.Query().Get("path"), "/", "\\")
	if path == "" { path = "C:\\" }
	entries, err := os.ReadDir(path)
	if err != nil { jsonError(w, err.Error(), 403); return }
	var items []map[string]interface{}
	for _, e := range entries {
		info, _ := e.Info()
		ext := ""; if !e.IsDir() { ext = strings.TrimPrefix(filepath.Ext(e.Name()), ".") }
		sizeKb := int64(0); modTime := int64(0)
		if info != nil { sizeKb = info.Size() / 1024; modTime = info.ModTime().Unix() }
		items = append(items, map[string]interface{}{
			"name": e.Name(), "path": strings.ReplaceAll(filepath.Join(path, e.Name()), "\\", "/"),
			"isDir": e.IsDir(), "sizeKb": sizeKb, "extension": ext, "modTime": modTime,
		})
	}
	jsonOK(w, items)
}

func handleBrowseSpecial(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	specials := []struct{ Name, Sub, Icon string }{
		{"Desktop", "Desktop", "🖥️"}, {"Downloads", "Downloads", "⬇️"},
		{"Documents", "Documents", "📄"}, {"Pictures", "Pictures", "🖼️"},
		{"Videos", "Videos", "🎬"}, {"Music", "Music", "🎵"},
	}
	var folders []map[string]interface{}
	for _, s := range specials {
		p := filepath.Join(home, s.Sub)
		if _, err := os.Stat(p); err == nil {
			entries, _ := os.ReadDir(p); count := len(entries)
			folders = append(folders, map[string]interface{}{
				"name": s.Name, "path": strings.ReplaceAll(p, "\\", "/"), "icon": s.Icon, "count": count,
			})
		}
	}
	jsonOK(w, folders)
}

func handleBrowseSearch(w http.ResponseWriter, r *http.Request) {
	root := strings.ReplaceAll(r.URL.Query().Get("path"), "/", "\\")
	query := strings.ToLower(r.URL.Query().Get("q"))
	if query == "" { jsonError(w, "q required", 400); return }
	maxResults := 50
	var results []map[string]interface{}
	deadline := time.Now().Add(8 * time.Second)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || time.Now().After(deadline) || len(results) >= maxResults { return filepath.SkipDir }
		if strings.Contains(strings.ToLower(info.Name()), query) {
			ext := ""; if !info.IsDir() { ext = strings.TrimPrefix(filepath.Ext(info.Name()), ".") }
			results = append(results, map[string]interface{}{
				"name": info.Name(), "path": strings.ReplaceAll(path, "\\", "/"),
				"isDir": info.IsDir(), "sizeKb": info.Size() / 1024, "extension": ext,
			})
		}
		return nil
	})
	jsonOK(w, results)
}

func handleBrowseApps(w http.ResponseWriter, r *http.Request) {
	apps := []map[string]interface{}{
		{"name": "Notepad", "exePath": "notepad.exe", "icon": "📄"},
		{"name": "Calculator", "exePath": "calc.exe", "icon": "🔢"},
		{"name": "File Explorer", "exePath": "explorer.exe", "icon": "📁"},
		{"name": "Command Prompt", "exePath": "cmd.exe", "icon": "⬛"},
		{"name": "Task Manager", "exePath": "taskmgr.exe", "icon": "⚙️"},
	}
	jsonOK(w, apps)
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64: return n
	case string: f, _ := strconv.ParseFloat(n, 64); return f
	default: return 0
	}
}
