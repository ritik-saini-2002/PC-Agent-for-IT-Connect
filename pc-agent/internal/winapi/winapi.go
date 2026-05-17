// Package winapi provides raw Windows API proc calls used across the agent.
package winapi

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	powrprof = syscall.NewLazyDLL("powrprof.dll")

	// user32
	procSendInput             = user32.NewProc("SendInput")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput     = user32.NewProc("AttachThreadInput")
	procShowWindow            = user32.NewProc("ShowWindow")
	procBringWindowToTop      = user32.NewProc("BringWindowToTop")
	procIsIconic              = user32.NewProc("IsIconic")
	procEnumWindows           = user32.NewProc("EnumWindows")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procLockWorkStation       = user32.NewProc("LockWorkStation")
	procGetDC                 = user32.NewProc("GetDC")
	procReleaseDC             = user32.NewProc("ReleaseDC")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
	procVkKeyScanW            = user32.NewProc("VkKeyScanW")
	procOpenInputDesktop      = user32.NewProc("OpenInputDesktop")
	procSetThreadDesktop      = user32.NewProc("SetThreadDesktop")
	procCloseDesktop          = user32.NewProc("CloseDesktop")
	procOpenClipboard         = user32.NewProc("OpenClipboard")
	procCloseClipboard        = user32.NewProc("CloseClipboard")
	procEmptyClipboard        = user32.NewProc("EmptyClipboard")
	procGetClipboardData      = user32.NewProc("GetClipboardData")
	procSetClipboardData      = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procMessageBoxW           = user32.NewProc("MessageBoxW")

	// kernel32
	procSetThreadExecutionState = kernel32.NewProc("SetThreadExecutionState")
	procGetCurrentThreadId      = kernel32.NewProc("GetCurrentThreadId")
	procGlobalAlloc             = kernel32.NewProc("GlobalAlloc")
	procGlobalLock              = kernel32.NewProc("GlobalLock")
	procGlobalUnlock            = kernel32.NewProc("GlobalUnlock")
	procGetVolumeInformationW   = kernel32.NewProc("GetVolumeInformationW")
	procGetLogicalDriveStringsW = kernel32.NewProc("GetLogicalDriveStringsW")
	procGetDiskFreeSpaceExW     = kernel32.NewProc("GetDiskFreeSpaceExW")

	// gdi32
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procGetDIBits              = gdi32.NewProc("GetDIBits")

	// shell32
	procShellExecuteW = shell32.NewProc("ShellExecuteW")

	// powrprof
	procSetSuspendState = powrprof.NewProc("SetSuspendState")
)

// ── INPUT STRUCTURES ─────────────────────────────────────────────

const (
	InputMouse    = 0
	InputKeyboard = 1

	KeyEventFKeyUp       = 0x0002
	KeyEventFExtendedKey = 0x0001
	KeyEventFUnicode     = 0x0004

	MouseEventFMove      = 0x0001
	MouseEventFLeftDown  = 0x0002
	MouseEventFLeftUp    = 0x0004
	MouseEventFRightDown = 0x0008
	MouseEventFRightUp   = 0x0010
	MouseEventFWheel     = 0x0800
	MouseEventFHWheel    = 0x1000
	MouseEventFAbsolute  = 0x8000

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	SW_RESTORE  = 9
	SW_SHOW     = 5
	SW_MINIMIZE = 6

	CF_UNICODETEXT = 13

	ES_CONTINUOUS       = 0x80000000
	ES_SYSTEM_REQUIRED  = 0x00000001
	ES_DISPLAY_REQUIRED = 0x00000002

	SRCCOPY = 0x00CC0020
	BI_RGB  = 0

	GMEM_MOVEABLE = 0x0002
)

// MouseInput matches the Windows MOUSEINPUT structure.
type MouseInput struct {
	Dx        int32
	Dy        int32
	MouseData uint32
	DwFlags   uint32
	Time      uint32
	DwExtra   uintptr
}

// KeybdInput matches the Windows KEYBDINPUT structure.
type KeybdInput struct {
	WVk     uint16
	WScan   uint16
	DwFlags uint32
	Time    uint32
	DwExtra uintptr
}

// Input matches the Windows INPUT structure.
type Input struct {
	Type uint32
	_    [4]byte // padding on 64-bit
	Mi   MouseInput
}

// KeyboardInput is INPUT for keyboard events.
type KeyboardInput struct {
	Type uint32
	_    [4]byte
	Ki   KeybdInput
	_pad [16]byte // pad to match INPUT union size
}

// BITMAPINFOHEADER for screen capture.
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// ── LOW-LEVEL FUNCTIONS ──────────────────────────────────────────

// SendInputCall sends one INPUT structure to the system.
func SendInputCall(input unsafe.Pointer, size int) {
	procSendInput.Call(1, uintptr(input), uintptr(size))
}

// SendKeyDown presses a virtual key code.
func SendKeyDown(vk uint16) {
	ki := KeyboardInput{Type: InputKeyboard}
	ki.Ki = KeybdInput{WVk: vk}
	SendInputCall(unsafe.Pointer(&ki), int(unsafe.Sizeof(ki)))
}

// SendKeyUp releases a virtual key code.
func SendKeyUp(vk uint16) {
	ki := KeyboardInput{Type: InputKeyboard}
	ki.Ki = KeybdInput{WVk: vk, DwFlags: KeyEventFKeyUp}
	SendInputCall(unsafe.Pointer(&ki), int(unsafe.Sizeof(ki)))
}

// SendMouseInput sends a mouse input event.
func SendMouseInput(flags uint32, dx, dy int32, data uint32) {
	mi := Input{Type: InputMouse}
	mi.Mi = MouseInput{Dx: dx, Dy: dy, MouseData: data, DwFlags: flags}
	SendInputCall(unsafe.Pointer(&mi), int(unsafe.Sizeof(mi)))
}

// GetScreenSize returns the primary monitor resolution.
func GetScreenSize() (int, int) {
	w, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	h, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	return int(w), int(h)
}

// GetForegroundWindow returns the HWND of the foreground window.
func GetForegroundWindow() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}

// SetForegroundWindow brings a window to the front.
func SetForegroundWindow(hwnd uintptr) {
	procSetForegroundWindow.Call(hwnd)
}

// ShowWindow changes the show state of a window.
func ShowWindow(hwnd uintptr, cmdShow int) {
	procShowWindow.Call(hwnd, uintptr(cmdShow))
}

// BringWindowToTop brings a window to the top of the Z order.
func BringWindowToTop(hwnd uintptr) {
	procBringWindowToTop.Call(hwnd)
}

// IsIconic checks if a window is minimized.
func IsIconic(hwnd uintptr) bool {
	ret, _, _ := procIsIconic.Call(hwnd)
	return ret != 0
}

// IsWindowVisible checks if a window is visible.
func IsWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

// GetWindowThreadProcessId returns the thread and process IDs for a window.
func GetWindowThreadProcessId(hwnd uintptr) (threadID, processID uint32) {
	var pid uint32
	tid, _, _ := procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return uint32(tid), pid
}

// AttachThreadInput attaches or detaches thread input.
func AttachThreadInput(idAttach, idAttachTo uint32, attach bool) {
	var fAttach uintptr
	if attach {
		fAttach = 1
	}
	procAttachThreadInput.Call(uintptr(idAttach), uintptr(idAttachTo), fAttach)
}

// GetCurrentThreadId returns the calling thread's ID.
func GetCurrentThreadId() uint32 {
	tid, _, _ := procGetCurrentThreadId.Call()
	return uint32(tid)
}

// LockWorkStation locks the computer.
func LockWorkStation() {
	procLockWorkStation.Call()
}

// SetThreadExecutionState prevents sleep/screen-off.
func SetThreadExecutionState(flags uint32) {
	procSetThreadExecutionState.Call(uintptr(flags))
}

// GetWindowText returns the title of a window.
func GetWindowText(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}

// EnumWindows enumerates all top-level windows.
func EnumWindows(callback func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd, lParam uintptr) uintptr {
		if callback(hwnd) {
			return 1 // continue
		}
		return 0 // stop
	})
	procEnumWindows.Call(cb, 0)
}

// VkKeyScanW returns the virtual key code + shift state for a character.
func VkKeyScanW(ch uint16) int16 {
	ret, _, _ := procVkKeyScanW.Call(uintptr(ch))
	return int16(ret)
}

// ShellExecuteW opens a file, URL, or app.
func ShellExecuteW(hwnd uintptr, verb, file, params, dir string, showCmd int) uintptr {
	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	filePtr, _ := syscall.UTF16PtrFromString(file)
	var paramsPtr, dirPtr *uint16
	if params != "" {
		paramsPtr, _ = syscall.UTF16PtrFromString(params)
	}
	if dir != "" {
		dirPtr, _ = syscall.UTF16PtrFromString(dir)
	}
	ret, _, _ := procShellExecuteW.Call(
		hwnd,
		uintptr(unsafe.Pointer(verbPtr)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramsPtr)),
		uintptr(unsafe.Pointer(dirPtr)),
		uintptr(showCmd),
	)
	return ret
}

// GetDC retrieves a handle to a device context for the entire screen.
func GetDC(hwnd uintptr) uintptr {
	hdc, _, _ := procGetDC.Call(hwnd)
	return hdc
}

// ReleaseDC releases a device context.
func ReleaseDC(hwnd, hdc uintptr) {
	procReleaseDC.Call(hwnd, hdc)
}

// CreateCompatibleDC creates a memory device context.
func CreateCompatibleDC(hdc uintptr) uintptr {
	mdc, _, _ := procCreateCompatibleDC.Call(hdc)
	return mdc
}

// CreateCompatibleBitmap creates a bitmap compatible with a device context.
func CreateCompatibleBitmap(hdc uintptr, width, height int) uintptr {
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	return bmp
}

// SelectObject selects an object into a device context.
func SelectObject(hdc, obj uintptr) uintptr {
	old, _, _ := procSelectObject.Call(hdc, obj)
	return old
}

// BitBlt performs a bit-block transfer.
func BitBlt(hdcDest uintptr, x, y, w, h int, hdcSrc uintptr, srcX, srcY int, rop uint32) bool {
	ret, _, _ := procBitBlt.Call(
		hdcDest, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hdcSrc, uintptr(srcX), uintptr(srcY), uintptr(rop),
	)
	return ret != 0
}

// DeleteDC deletes a device context.
func DeleteDC(hdc uintptr) {
	procDeleteDC.Call(hdc)
}

// DeleteObject deletes a GDI object.
func DeleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

// GetDIBits retrieves bitmap bits.
func GetDIBits(hdc, hbmp uintptr, start, lines uint32, bits unsafe.Pointer, bi *BITMAPINFOHEADER, usage uint32) int {
	ret, _, _ := procGetDIBits.Call(
		hdc, hbmp, uintptr(start), uintptr(lines),
		uintptr(bits), uintptr(unsafe.Pointer(bi)), uintptr(usage),
	)
	return int(ret)
}

// SetSuspendState puts the computer to sleep.
func SetSuspendState(hibernate, force, wakeEvents bool) {
	var h, f, w uintptr
	if hibernate {
		h = 1
	}
	if force {
		f = 1
	}
	if wakeEvents {
		w = 1
	}
	procSetSuspendState.Call(h, f, w)
}

// GetLogicalDriveStrings returns all drive letters.
func GetLogicalDriveStrings() []string {
	buf := make([]uint16, 256)
	n, _, _ := procGetLogicalDriveStringsW.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 {
		return nil
	}
	var drives []string
	s := buf[:n]
	for {
		idx := 0
		for idx < len(s) && s[idx] != 0 {
			idx++
		}
		if idx == 0 {
			break
		}
		drives = append(drives, syscall.UTF16ToString(s[:idx]))
		if idx+1 >= len(s) {
			break
		}
		s = s[idx+1:]
	}
	return drives
}

// GetDiskFreeSpaceEx returns free/total bytes for a drive.
func GetDiskFreeSpaceEx(path string) (free, total, totalFree uint64, err error) {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	r, _, e := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		err = e
	}
	return
}

// GetVolumeInformation returns the label and filesystem type for a drive.
func GetVolumeInformation(rootPath string) (label string, err error) {
	rootPtr, _ := syscall.UTF16PtrFromString(rootPath)
	labelBuf := make([]uint16, 256)
	r, _, e := procGetVolumeInformationW.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&labelBuf[0])), 256,
		0, 0, 0, 0, 0,
	)
	if r == 0 {
		return "Local Disk", e
	}
	label = syscall.UTF16ToString(labelBuf)
	if label == "" {
		label = "Local Disk"
	}
	return label, nil
}

// OpenInputDesktop opens the desktop that receives user input.
func OpenInputDesktop() uintptr {
	h, _, _ := procOpenInputDesktop.Call(0, 0, 0x10000000) // GENERIC_ALL
	return h
}

// SetThreadDesktop sets the desktop for the calling thread.
func SetThreadDesktop(hDesktop uintptr) bool {
	r, _, _ := procSetThreadDesktop.Call(hDesktop)
	return r != 0
}

// CloseDesktop closes a desktop handle.
func CloseDesktop(hDesktop uintptr) {
	procCloseDesktop.Call(hDesktop)
}

// SwitchToInteractiveDesktop switches the calling thread to the input desktop.
// Must be called before SendInput on a Windows Service.
func SwitchToInteractiveDesktop() {
	hDesk := OpenInputDesktop()
	if hDesk != 0 {
		SetThreadDesktop(hDesk)
		CloseDesktop(hDesk)
	}
}
