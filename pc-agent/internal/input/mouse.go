// Package input — mouse control via Windows SendInput API.
package input

import (
	"sync"
	"sync/atomic"

	"github.com/ritik-saini/pc-agent/internal/winapi"
)

var (
	dragActive atomic.Bool
	dragButton string // "left" or "right"
	dragMu     sync.Mutex
)

// MoveRelative moves the mouse by dx, dy pixels.
func MoveRelative(dx, dy int) {
	winapi.SwitchToInteractiveDesktop()
	winapi.SendMouseInput(winapi.MouseEventFMove, int32(dx), int32(dy), 0)
}

// MoveAbsolute moves the mouse to absolute screen coordinates.
func MoveAbsolute(x, y int) {
	winapi.SwitchToInteractiveDesktop()
	sw, sh := winapi.GetScreenSize()
	if sw == 0 || sh == 0 {
		return
	}
	absX := int32(x * 65535 / sw)
	absY := int32(y * 65535 / sh)
	winapi.SendMouseInput(winapi.MouseEventFMove|winapi.MouseEventFAbsolute, absX, absY, 0)
}

// Click performs a mouse click at the current position.
func Click(button string, double bool) {
	winapi.SwitchToInteractiveDesktop()
	if button == "right" {
		winapi.SendMouseInput(winapi.MouseEventFRightDown, 0, 0, 0)
		winapi.SendMouseInput(winapi.MouseEventFRightUp, 0, 0, 0)
		if double {
			winapi.SendMouseInput(winapi.MouseEventFRightDown, 0, 0, 0)
			winapi.SendMouseInput(winapi.MouseEventFRightUp, 0, 0, 0)
		}
	} else {
		winapi.SendMouseInput(winapi.MouseEventFLeftDown, 0, 0, 0)
		winapi.SendMouseInput(winapi.MouseEventFLeftUp, 0, 0, 0)
		if double {
			winapi.SendMouseInput(winapi.MouseEventFLeftDown, 0, 0, 0)
			winapi.SendMouseInput(winapi.MouseEventFLeftUp, 0, 0, 0)
		}
	}
}

// MouseDown presses a mouse button and starts a drag.
func MouseDown(button string) {
	winapi.SwitchToInteractiveDesktop()
	dragMu.Lock()
	dragButton = button
	dragMu.Unlock()
	dragActive.Store(true)

	if button == "right" {
		winapi.SendMouseInput(winapi.MouseEventFRightDown, 0, 0, 0)
	} else {
		winapi.SendMouseInput(winapi.MouseEventFLeftDown, 0, 0, 0)
	}
}

// MouseUp releases a mouse button and ends a drag.
func MouseUp(button string) {
	winapi.SwitchToInteractiveDesktop()
	if dragActive.Load() {
		dragMu.Lock()
		btn := dragButton
		dragMu.Unlock()

		if btn == "right" {
			winapi.SendMouseInput(winapi.MouseEventFRightUp, 0, 0, 0)
		} else {
			winapi.SendMouseInput(winapi.MouseEventFLeftUp, 0, 0, 0)
		}
		dragActive.Store(false)
	}
}

// Scroll performs a mouse wheel scroll. amount > 0 = up, < 0 = down.
func Scroll(amount int, horizontal bool) {
	winapi.SwitchToInteractiveDesktop()
	data := uint32(amount * 120)
	if horizontal {
		winapi.SendMouseInput(winapi.MouseEventFHWheel, 0, 0, data)
	} else {
		winapi.SendMouseInput(winapi.MouseEventFWheel, 0, 0, data)
	}
}

// IsDragging returns whether a drag operation is in progress.
func IsDragging() bool {
	return dragActive.Load()
}
