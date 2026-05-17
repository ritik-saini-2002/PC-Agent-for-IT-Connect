// Package input provides keyboard control via Windows SendInput API.
package input

import (
	"strings"
	"sync"
	"time"

	"github.com/ritik-saini/pc-agent/internal/winapi"
)

// ── VIRTUAL KEY CODE MAP ─────────────────────────────────────────
// Exact port of the Python VK dictionary from agent_v12.py

var VK = map[string]uint16{
	"WIN": 0x5B, "LWIN": 0x5B, "RWIN": 0x5C,
	"CTRL": 0x11, "ALT": 0x12, "SHIFT": 0x10,
	"ENTER": 0x0D, "ESC": 0x1B, "SPACE": 0x20,
	"TAB": 0x09, "BACK": 0x08, "BACKSPACE": 0x08, "DEL": 0x2E, "DELETE": 0x2E,
	"UP": 0x26, "DOWN": 0x28, "LEFT": 0x25, "RIGHT": 0x27,
	"HOME": 0x24, "END": 0x23, "PGUP": 0x21, "PGDN": 0x22,
	"PAGEUP": 0x21, "PAGEDOWN": 0x22,
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73,
	"F5": 0x74, "F6": 0x75, "F7": 0x76, "F8": 0x77,
	"F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,
	"INSERT": 0x2D, "PRINTSCREEN": 0x2C, "PAUSE": 0x13, "NUMLOCK": 0x90,
	"VOLUP": 0xAF, "VOLDN": 0xAE, "MUTE": 0xAD,
	"PLUS": 0xBB, "MINUS": 0xBD, "OEM_PLUS": 0xBB, "OEM_MINUS": 0xBD, "EQUALS": 0xBB,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"A": 0x41, "B": 0x42, "C": 0x43, "D": 0x44, "E": 0x45, "F": 0x46,
	"G": 0x47, "H": 0x48, "I": 0x49, "J": 0x4A, "K": 0x4B, "L": 0x4C,
	"M": 0x4D, "N": 0x4E, "O": 0x4F, "P": 0x50, "Q": 0x51, "R": 0x52,
	"S": 0x53, "T": 0x54, "U": 0x55, "V": 0x56, "W": 0x57, "X": 0x58,
	"Y": 0x59, "Z": 0x5A, "ALTGR": 0xA5,
}

// ── PREDEFINED KEY COMBOS ────────────────────────────────────────
// Port of Python COMBOS dict — maps key strings to VK code sequences.

var Combos = map[string][]uint16{
	// Windows combos
	"WIN+L": {0x5B, 0x4C}, "WIN+D": {0x5B, 0x44}, "WIN+E": {0x5B, 0x45},
	"WIN+R": {0x5B, 0x52}, "WIN+I": {0x5B, 0x49}, "WIN+A": {0x5B, 0x41},
	"WIN+S": {0x5B, 0x53}, "WIN+X": {0x5B, 0x58}, "WIN+P": {0x5B, 0x50},
	"WIN+M": {0x5B, 0x4D}, "WIN+V": {0x5B, 0x56}, "WIN+G": {0x5B, 0x47},
	"WIN+TAB": {0x5B, 0x09}, "WIN+UP": {0x5B, 0x26}, "WIN+DOWN": {0x5B, 0x28},
	"WIN+LEFT": {0x5B, 0x25}, "WIN+RIGHT": {0x5B, 0x27},
	"WIN+SHIFT+S": {0x5B, 0x10, 0x53}, "WIN": {0x5B},
	"WIN+.": {0x5B, 0xBE},
	"WIN+1": {0x5B, 0x31}, "WIN+2": {0x5B, 0x32}, "WIN+3": {0x5B, 0x33},

	// Ctrl combos
	"CTRL+C": {0x11, 0x43}, "CTRL+V": {0x11, 0x56}, "CTRL+Z": {0x11, 0x5A},
	"CTRL+Y": {0x11, 0x59}, "CTRL+S": {0x11, 0x53}, "CTRL+A": {0x11, 0x41},
	"CTRL+X": {0x11, 0x58}, "CTRL+W": {0x11, 0x57}, "CTRL+N": {0x11, 0x4E},
	"CTRL+T": {0x11, 0x54}, "CTRL+F": {0x11, 0x46}, "CTRL+P": {0x11, 0x50},
	"CTRL+O": {0x11, 0x4F}, "CTRL+R": {0x11, 0x52}, "CTRL+L": {0x11, 0x4C},
	"CTRL+D": {0x11, 0x44}, "CTRL+H": {0x11, 0x48}, "CTRL+B": {0x11, 0x42},
	"CTRL+I": {0x11, 0x49}, "CTRL+U": {0x11, 0x55}, "CTRL+G": {0x11, 0x47},
	"CTRL+K": {0x11, 0x4B}, "CTRL+E": {0x11, 0x45}, "CTRL+J": {0x11, 0x4A},
	"CTRL+Q": {0x11, 0x51},
	"CTRL+PLUS": {0x11, 0xBB}, "CTRL+MINUS": {0x11, 0xBD}, "CTRL+0": {0x11, 0x30},
	"CTRL+TAB": {0x11, 0x09}, "CTRL+END": {0x11, 0x23}, "CTRL+HOME": {0x11, 0x24},

	// Ctrl+Shift combos
	"CTRL+SHIFT+ESC": {0x11, 0x10, 0x1B}, "CTRL+SHIFT+N": {0x11, 0x10, 0x4E},
	"CTRL+SHIFT+T": {0x11, 0x10, 0x54}, "CTRL+SHIFT+V": {0x11, 0x10, 0x56},
	"CTRL+SHIFT+S": {0x11, 0x10, 0x53}, "CTRL+SHIFT+F": {0x11, 0x10, 0x46},
	"CTRL+SHIFT+TAB": {0x11, 0x10, 0x09}, "CTRL+SHIFT+DELETE": {0x11, 0x10, 0x2E},
	"CTRL+ALT+DEL": {0x11, 0x12, 0x2E},

	// Alt combos
	"ALT+F4": {0x12, 0x73}, "ALT+TAB": {0x12, 0x09}, "ALT+ENTER": {0x12, 0x0D},
	"ALT+ESC": {0x12, 0x1B}, "ALT+F": {0x12, 0x46}, "ALT+E": {0x12, 0x45},
	"ALT+V": {0x12, 0x56}, "ALT+D": {0x12, 0x44}, "ALT+SPACE": {0x12, 0x20},
	"ALT+LEFT": {0x12, 0x25}, "ALT+RIGHT": {0x12, 0x27}, "ALT+UP": {0x12, 0x26},
	"ALT+PRINTSCREEN": {0x12, 0x2C},

	// Shift combos
	"SHIFT+DELETE": {0x10, 0x2E}, "SHIFT+TAB": {0x10, 0x09},
	"SHIFT+F10": {0x10, 0x79}, "SHIFT+F3": {0x10, 0x72},
	"SHIFT+INSERT": {0x10, 0x2D}, "SHIFT+HOME": {0x10, 0x24},
	"SHIFT+END": {0x10, 0x23}, "SHIFT+UP": {0x10, 0x26},
	"SHIFT+DOWN": {0x10, 0x28},

	// Single keys
	"ENTER": {0x0D}, "ESC": {0x1B}, "SPACE": {0x20}, "TAB": {0x09},
	"BACKSPACE": {0x08}, "UP": {0x26}, "DOWN": {0x28}, "LEFT": {0x25}, "RIGHT": {0x27},
	"HOME": {0x24}, "END": {0x23}, "PAGE_UP": {0x21}, "PAGE_DOWN": {0x22},
	"PAGEUP": {0x21}, "PAGEDOWN": {0x22}, "PRINTSCREEN": {0x2C}, "INSERT": {0x2D},
	"SHIFT": {0x10}, "CTRL": {0x11}, "ALT": {0x12}, "ALTGR": {0xA5},
	"VOLUME_UP": {0xAF}, "VOLUME_DOWN": {0xAE},
	"F1": {0x70}, "F2": {0x71}, "F3": {0x72}, "F4": {0x73},
	"F5": {0x74}, "F6": {0x75}, "F7": {0x76}, "F8": {0x77},
	"F9": {0x78}, "F10": {0x79}, "F11": {0x7A}, "F12": {0x7B},
}

// ── HELD KEYS ────────────────────────────────────────────────────

var (
	heldKeys  = make(map[uint16]bool)
	heldMutex sync.Mutex
)

// SendKey presses and releases a virtual key.
func SendKey(vk uint16) {
	winapi.SwitchToInteractiveDesktop()
	winapi.SendKeyDown(vk)
	time.Sleep(10 * time.Millisecond)
	winapi.SendKeyUp(vk)
}

// SendCombo presses all keys in order then releases in reverse — matches Python _send_combo.
func SendCombo(vks ...uint16) {
	winapi.SwitchToInteractiveDesktop()
	for _, vk := range vks {
		winapi.SendKeyDown(vk)
	}
	for i := len(vks) - 1; i >= 0; i-- {
		winapi.SendKeyUp(vks[i])
	}
	time.Sleep(50 * time.Millisecond)
}

// HoldKey presses and holds a key (doesn't release).
func HoldKey(vk uint16) {
	heldMutex.Lock()
	defer heldMutex.Unlock()
	if !heldKeys[vk] {
		winapi.SwitchToInteractiveDesktop()
		winapi.SendKeyDown(vk)
		heldKeys[vk] = true
	}
}

// ReleaseKey releases a held key.
func ReleaseKey(vk uint16) {
	heldMutex.Lock()
	defer heldMutex.Unlock()
	if heldKeys[vk] {
		winapi.SendKeyUp(vk)
		delete(heldKeys, vk)
	}
}

// ReleaseAllHeld releases all currently held keys.
func ReleaseAllHeld() {
	heldMutex.Lock()
	defer heldMutex.Unlock()
	for vk := range heldKeys {
		winapi.SendKeyUp(vk)
	}
	heldKeys = make(map[uint16]bool)
}

// TypeString types a string character by character using SendInput.
func TypeString(text string) {
	winapi.SwitchToInteractiveDesktop()
	for _, ch := range text {
		vkScan := winapi.VkKeyScanW(uint16(ch))
		vk := uint16(vkScan & 0xFF)
		shift := (vkScan >> 8) & 0xFF

		if vk != 0xFF {
			if shift&1 != 0 {
				winapi.SendKeyDown(VK["SHIFT"])
			}
			winapi.SendKeyDown(vk)
			winapi.SendKeyUp(vk)
			if shift&1 != 0 {
				winapi.SendKeyUp(VK["SHIFT"])
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ExecuteKeyPress handles a key string (single key or combo like "CTRL+C").
// Returns a description of what was pressed.
func ExecuteKeyPress(keyStr string) string {
	keyStr = strings.ToUpper(strings.TrimSpace(keyStr))

	// Check predefined combos first
	if combo, ok := Combos[keyStr]; ok {
		SendCombo(combo...)
		return "Key: " + keyStr
	}

	// Dynamic combo parsing (e.g., "CTRL+SHIFT+Q")
	if strings.Contains(keyStr, "+") {
		parts := strings.Split(keyStr, "+")
		var vks []uint16
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if vk, ok := VK[part]; ok {
				vks = append(vks, vk)
			} else if len(part) == 1 {
				if part[0] >= 'A' && part[0] <= 'Z' {
					vks = append(vks, uint16(part[0]))
				} else if part[0] >= '0' && part[0] <= '9' {
					vks = append(vks, uint16(part[0]))
				}
			}
		}
		if len(vks) > 0 {
			SendCombo(vks...)
			return "Key: " + keyStr + " (dynamic)"
		}
	}

	// Single character
	if len(keyStr) == 1 {
		vkScan := winapi.VkKeyScanW(uint16(keyStr[0]))
		if vkScan != -1 {
			SendCombo(uint16(vkScan & 0xFF))
			return "Key: " + keyStr
		}
	}

	return "Key unknown: " + keyStr
}
