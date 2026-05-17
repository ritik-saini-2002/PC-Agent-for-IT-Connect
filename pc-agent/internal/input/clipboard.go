// Package input — clipboard operations via Windows API.
package input

import (
	"syscall"
	"unsafe"

	"github.com/ritik-saini/pc-agent/internal/winapi"
)

// ReadClipboard returns the current clipboard text (CF_UNICODETEXT).
func ReadClipboard() (string, error) {
	r, _, err := syscall.NewLazyDLL("user32.dll").NewProc("OpenClipboard").Call(0)
	if r == 0 {
		return "", err
	}
	defer syscall.NewLazyDLL("user32.dll").NewProc("CloseClipboard").Call()

	// Check if Unicode text is available
	avail, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("IsClipboardFormatAvailable").Call(winapi.CF_UNICODETEXT)
	if avail == 0 {
		return "", nil
	}

	h, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("GetClipboardData").Call(winapi.CF_UNICODETEXT)
	if h == 0 {
		return "", nil
	}

	ptr, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalLock").Call(h)
	if ptr == 0 {
		return "", nil
	}
	defer syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalUnlock").Call(h)

	// Read UTF-16 string
	var u16 []uint16
	for i := 0; ; i++ {
		val := *(*uint16)(unsafe.Pointer(ptr + uintptr(i*2)))
		if val == 0 {
			break
		}
		u16 = append(u16, val)
	}
	text := syscall.UTF16ToString(u16)
	return text, nil
}

// WriteClipboard sets the clipboard text (CF_UNICODETEXT).
func WriteClipboard(text string) error {
	r, _, err := syscall.NewLazyDLL("user32.dll").NewProc("OpenClipboard").Call(0)
	if r == 0 {
		return err
	}
	defer syscall.NewLazyDLL("user32.dll").NewProc("CloseClipboard").Call()

	syscall.NewLazyDLL("user32.dll").NewProc("EmptyClipboard").Call()

	// Convert text to UTF-16
	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	size := len(utf16) * 2 // 2 bytes per uint16
	hMem, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalAlloc").Call(
		winapi.GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return syscall.ENOMEM
	}

	ptr, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalLock").Call(hMem)
	if ptr == 0 {
		return syscall.ENOMEM
	}

	// Copy UTF-16 data
	for i := 0; i < size; i++ {
		*(*byte)(unsafe.Pointer(ptr + uintptr(i))) = *(*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(&utf16[0])) + uintptr(i)))
	}

	syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalUnlock").Call(hMem)

	syscall.NewLazyDLL("user32.dll").NewProc("SetClipboardData").Call(winapi.CF_UNICODETEXT, hMem)

	return nil
}

// TypeViaClipboard copies text to clipboard then pastes with Ctrl+V — fast path.
func TypeViaClipboard(text string) {
	if err := WriteClipboard(text); err != nil {
		// Fallback to SendInput character-by-character
		TypeString(text)
		return
	}
	SendCombo(VK["CTRL"], VK["V"])
}
