// Package capture provides screen capture via GDI BitBlt.
package capture

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ritik-saini/pc-agent/internal/logger"
	"github.com/ritik-saini/pc-agent/internal/winapi"
	"golang.org/x/image/draw"
)

var (
	latestFrame []byte
	frameLock   sync.RWMutex
	streamClients atomic.Int64
	defaultFPS    = 20
)

// GetFrame returns the latest captured JPEG frame.
func GetFrame() []byte {
	frameLock.RLock()
	defer frameLock.RUnlock()
	return latestFrame
}

// IncrementClients increments the stream client counter.
func IncrementClients() {
	streamClients.Add(1)
}

// DecrementClients decrements the stream client counter.
func DecrementClients() {
	streamClients.Add(-1)
}

// ClientCount returns the number of active stream clients.
func ClientCount() int64 {
	return streamClients.Load()
}

// StartGrabber starts the background frame grabber goroutine.
// It captures the screen using GDI BitBlt, encodes to JPEG, and stores
// the result in the shared frame slot. Sleeps when no clients connected.
func StartGrabber() {
	go func() {
		interval := time.Duration(float64(time.Second) / float64(defaultFPS))

		for {
			// Sleep when no viewers
			if streamClients.Load() == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			t0 := time.Now()

			frame, err := captureScreen(1920, 75)
			if err != nil {
				logger.Warn("[FrameGrabber] capture error: %v", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			frameLock.Lock()
			latestFrame = frame
			frameLock.Unlock()

			elapsed := time.Since(t0)
			wait := interval - elapsed
			if wait > 0 {
				time.Sleep(wait)
			}
		}
	}()
}

// captureScreen captures the primary screen and returns JPEG bytes.
func captureScreen(targetWidth, quality int) ([]byte, error) {
	sw, sh := winapi.GetScreenSize()
	if sw == 0 || sh == 0 {
		return nil, nil
	}

	// Get screen DC
	hdcScreen := winapi.GetDC(0)
	if hdcScreen == 0 {
		return nil, nil
	}
	defer winapi.ReleaseDC(0, hdcScreen)

	// Create compatible DC and bitmap
	hdcMem := winapi.CreateCompatibleDC(hdcScreen)
	if hdcMem == 0 {
		return nil, nil
	}
	defer winapi.DeleteDC(hdcMem)

	hBitmap := winapi.CreateCompatibleBitmap(hdcScreen, sw, sh)
	if hBitmap == 0 {
		return nil, nil
	}
	defer winapi.DeleteObject(hBitmap)

	old := winapi.SelectObject(hdcMem, hBitmap)
	defer winapi.SelectObject(hdcMem, old)

	// BitBlt — copy screen to memory DC
	if !winapi.BitBlt(hdcMem, 0, 0, sw, sh, hdcScreen, 0, 0, winapi.SRCCOPY) {
		return nil, nil
	}

	// Read bitmap bits
	bi := winapi.BITMAPINFOHEADER{
		BiSize:     uint32(40), // sizeof(BITMAPINFOHEADER)
		BiWidth:    int32(sw),
		BiHeight:   int32(-sh), // top-down
		BiPlanes:   1,
		BiBitCount: 32,
	}

	pixels := make([]byte, sw*sh*4)
	winapi.GetDIBits(hdcMem, hBitmap, 0, uint32(sh), unsafe.Pointer(&pixels[0]), &bi, 0)

	// Convert BGRA to RGBA image
	img := image.NewRGBA(image.Rect(0, 0, sw, sh))
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			offset := (y*sw + x) * 4
			img.SetRGBA(x, y, color.RGBA{
				R: pixels[offset+2],
				G: pixels[offset+1],
				B: pixels[offset+0],
				A: 255,
			})
		}
	}

	// Resize to target width
	if targetWidth > 0 && targetWidth < sw {
		newH := sh * targetWidth / sw
		resized := image.NewRGBA(image.Rect(0, 0, targetWidth, newH))
		draw.BiLinear.Scale(resized, resized.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = resized
	}

	// Encode to JPEG
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// CaptureSnapshot captures a single low-quality screenshot for the /screen/snapshot endpoint.
func CaptureSnapshot(width, quality int) ([]byte, int, int, error) {
	frame, err := captureScreen(width, quality)
	if err != nil {
		return nil, 0, 0, err
	}
	// Decode to get dimensions
	img, err := jpeg.Decode(bytes.NewReader(frame))
	if err != nil {
		return frame, width, 480, nil
	}
	bounds := img.Bounds()
	return frame, bounds.Dx(), bounds.Dy(), nil
}
