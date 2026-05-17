package capture

import (
	"context"
	"os/exec"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
	"github.com/ritik-saini/pc-agent/internal/logger"
)

var (
	audioEnabled      bool
	audioEnabledMutex sync.RWMutex
	audioSubscribers  []chan []byte
	audioSubMutex     sync.Mutex
	audioLoopRunning  bool
)

func SetAudioEnabled(enabled bool) {
	audioEnabledMutex.Lock()
	audioEnabled = enabled
	audioEnabledMutex.Unlock()

	if enabled && !audioLoopRunning {
		go runAudioLoopback()
	}
}

func IsAudioEnabled() bool {
	audioEnabledMutex.RLock()
	defer audioEnabledMutex.RUnlock()
	return audioEnabled
}

func SubscribeAudio() chan []byte {
	ch := make(chan []byte, 100)
	audioSubMutex.Lock()
	audioSubscribers = append(audioSubscribers, ch)
	audioSubMutex.Unlock()
	return ch
}

func UnsubscribeAudio(ch chan []byte) {
	audioSubMutex.Lock()
	defer audioSubMutex.Unlock()
	for i, c := range audioSubscribers {
		if c == ch {
			audioSubscribers = append(audioSubscribers[:i], audioSubscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

func broadcastAudio(data []byte) {
	audioSubMutex.Lock()
	defer audioSubMutex.Unlock()
	for _, ch := range audioSubscribers {
		select {
		case ch <- data:
		default:
			// queue full, drop frame
		}
	}
}

func GetAudioSubscribersCount() int {
	audioSubMutex.Lock()
	defer audioSubMutex.Unlock()
	return len(audioSubscribers)
}

func runAudioLoopback() {
	audioEnabledMutex.Lock()
	if audioLoopRunning {
		audioEnabledMutex.Unlock()
		return
	}
	audioLoopRunning = true
	audioEnabledMutex.Unlock()

	defer func() {
		audioEnabledMutex.Lock()
		audioLoopRunning = false
		audioEnabledMutex.Unlock()
	}()

	for {
		if !IsAudioEnabled() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		err := captureWASAPI()
		if err != nil {
			logger.Warn("[Audio] WASAPI loopback error: %v", err)
			time.Sleep(1 * time.Second)
		}
	}
}

func captureWASAPI() error {
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return err
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &mmd); err != nil {
		return err
	}
	defer mmd.Release()

	var ac *wca.IAudioClient
	if err := mmd.Activate(wca.IID_IAudioClient, wca.CLSCTX_ALL, nil, &ac); err != nil {
		return err
	}
	defer ac.Release()

	var wfx *wca.WAVEFORMATEX
	if err := ac.GetMixFormat(&wfx); err != nil {
		return err
	}
	defer ole.CoTaskMemFree(uintptr(unsafe.Pointer(wfx)))

	// We want to initialize it in loopback mode
	wfx.WFormatTag = 1 // WAVE_FORMAT_PCM
	wfx.NChannels = 2
	wfx.NSamplesPerSec = 44100
	wfx.WBitsPerSample = 16
	wfx.NBlockAlign = (wfx.WBitsPerSample / 8) * wfx.NChannels
	wfx.NAvgBytesPerSec = wfx.NSamplesPerSec * uint32(wfx.NBlockAlign)
	wfx.CbSize = 0

	var defaultPeriod, minimumPeriod wca.REFERENCE_TIME
	if err := ac.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		return err
	}

	// 100ms latency
	latency := wca.REFERENCE_TIME(1000000)

	if err := ac.Initialize(wca.AUDCLNT_SHAREMODE_SHARED, wca.AUDCLNT_STREAMFLAGS_LOOPBACK, latency, 0, wfx, nil); err != nil {
		return err
	}

	var acc *wca.IAudioCaptureClient
	if err := ac.GetService(wca.IID_IAudioCaptureClient, &acc); err != nil {
		return err
	}
	defer acc.Release()

	if err := ac.Start(); err != nil {
		return err
	}
	defer ac.Stop()

	var bufferFrameCount uint32
	if err := ac.GetBufferSize(&bufferFrameCount); err != nil {
		return err
	}

	logger.Info("[Audio] WASAPI loopback started (44100Hz, 16-bit, stereo)")

	for IsAudioEnabled() {
		var packetLength uint32
		if err := acc.GetNextPacketSize(&packetLength); err != nil {
			return err
		}

		if packetLength == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		var data *byte
		var numFramesAvailable uint32
		var flags uint32
		var devicePosition uint64
		var qpcPosition uint64

		if err := acc.GetBuffer(&data, &numFramesAvailable, &flags, &devicePosition, &qpcPosition); err != nil {
			return err
		}

		if numFramesAvailable > 0 {
			// Copy data
			size := numFramesAvailable * uint32(wfx.NBlockAlign)
			rawBytes := (*[1 << 30]byte)(unsafe.Pointer(data))[:size:size]
			buf := make([]byte, size)
			copy(buf, rawBytes)

			if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT != 0 {
				for i := range buf {
					buf[i] = 0
				}
			}

			broadcastAudio(buf)
		}

		if err := acc.ReleaseBuffer(numFramesAvailable); err != nil {
			return err
		}
	}

	return nil
}

func ConvertToMP3(ctx context.Context, pcmData <-chan []byte) (<-chan []byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-loglevel", "quiet",
		"-f", "s16le",
		"-ar", "44100",
		"-ac", "2",
		"-i", "pipe:0",
		"-f", "mp3",
		"-ab", "192k",
		"-flush_packets", "1",
		"pipe:1")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	mp3Chan := make(chan []byte, 100)

	// Writer goroutine
	go func() {
		defer stdin.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-pcmData:
				if !ok {
					return
				}
				stdin.Write(chunk)
			}
		}
	}()

	// Reader goroutine
	go func() {
		defer close(mp3Chan)
		defer cmd.Wait()
		buf := make([]byte, 4096)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n, err := stdout.Read(buf)
				if n > 0 {
					b := make([]byte, n)
					copy(b, buf[:n])
					mp3Chan <- b
				}
				if err != nil {
					return
				}
			}
		}
	}()

	return mp3Chan, nil
}
