package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"net/http"
	"runtime"

	// _ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/kirides/screencapture/d3d"
	forkscreenshot "github.com/kirides/screencapture/screenshot"
	"github.com/kirides/screencapture/win"
	"github.com/nfnt/resize"

	"github.com/kbinani/screenshot"
	"github.com/mattn/go-mjpeg"
)

func main() {
	n := screenshot.NumActiveDisplays()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	http.HandleFunc("/watch", func(w http.ResponseWriter, r *http.Request) {
		screen := r.URL.Query().Get("screen")
		if screen == "" {
			screen = "0"
		}
		screenNo, err := strconv.Atoi(screen)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		if screenNo >= n || screenNo < 0 {
			screenNo = 0
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<head>
		<meta charset="UTF-8">
		<meta http-equiv="X-UA-Compatible" content="IE=edge">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title> Screen ` + strconv.Itoa(screenNo) + `</title>
	</head>
		<body style="margin:0">
	<img src="/mjpeg` + strconv.Itoa(screenNo) + `" style="max-width: 100vw; max-height: 100vh;object-fit: contain;display: block;margin: 0 auto;" />
</body>`))
	})

	framerate := 15
	for i := 0; i < n; i++ {
		fmt.Fprintf(os.Stderr, "Registering stream %d\n", i)
		stream := mjpeg.NewStream()
		defer stream.Close()
		// go streamDisplay(ctx, i, framerate, stream)
		go streamDisplayDXGI(ctx, i, framerate, stream)
		// go captureScreenTranscode(ctx, i, framerate)
		http.HandleFunc(fmt.Sprintf("/mjpeg%d", i), stream.ServeHTTP)
	}
	go func() {
		http.ListenAndServe("0.0.0.0:8023", nil)

	}()
	<-ctx.Done()
	<-time.After(time.Second)
}

// Capture using "github.com/kbinani/screenshot" (modified to reuse image.RGBA)
func streamDisplay(ctx context.Context, n int, framerate int, out *mjpeg.Stream) {
	max := screenshot.NumActiveDisplays()
	if n >= max {
		fmt.Printf("Not enough displays\n")
		return
	}
	buf := &bufferFlusher{}
	opts := jpegQuality(75)
	limiter := NewFrameLimiter(framerate)

	var err error
	finalBounds := screenshot.GetDisplayBounds(n)
	imgBuf := image.NewRGBA(finalBounds)

	lastBounds := finalBounds
	for {
		select {
		case <-ctx.Done():
			return
		default:
			limiter.Wait()
		}
		bounds := screenshot.GetDisplayBounds(n)

		x, y, hw, hh := bounds.Min.X, 0, bounds.Dx(), bounds.Dy()
		newBounds := image.Rect(0, 0, int(hw), int(hh))
		if newBounds != lastBounds {
			lastBounds = newBounds
			imgBuf = image.NewRGBA(lastBounds)
		}
		err = forkscreenshot.CaptureImg(imgBuf, int(x), int(y), int(hw), int(hh))
		if err != nil {
			fmt.Printf("Err CaptureImg: %v\n", err)
			continue
		}
		buf.Reset()

		encodeJpeg(buf, imgBuf, opts)
		out.Update(buf.Bytes())
	}
}

// Capture using IDXGIOutputDuplication
//     https://docs.microsoft.com/en-us/windows/win32/api/dxgi1_2/nn-dxgi1_2-idxgioutputduplication
func streamDisplayDXGI(ctx context.Context, n int, framerate int, out *mjpeg.Stream) {
	max := screenshot.NumActiveDisplays()
	if n >= max {
		fmt.Printf("Not enough displays\n")
		return
	}

	// Keep this thread, so windows/d3d11/dxgi can use their threadlocal caches, if any
	runtime.LockOSThread()

	// Make thread PerMonitorV2 Dpi aware if supported on OS
	// allows to let windows handle BGRA -> RGBA conversion and possibly more things
	if win.IsValidDpiAwarenessContext(win.DpiAwarenessContextPerMonitorAwareV2) {
		_, err := win.SetThreadDpiAwarenessContext(win.DpiAwarenessContextPerMonitorAwareV2)
		if err != nil {
			fmt.Printf("Could not set thread DPI awareness to PerMonitorAwareV2. %v\n", err)
		} else {
			fmt.Printf("Enabled PerMonitorAwareV2 DPI awareness.\n")
		}
	}

	// Setup D3D11 stuff
	device, deviceCtx, err := d3d.NewD3D11Device()
	if err != nil {
		fmt.Printf("Could not create D3D11 Device. %v\n", err)
		return
	}
	defer device.Release()
	defer deviceCtx.Release()

	var ddup *d3d.OutputDuplicator
	defer func() {
		if ddup != nil {
			ddup.Release()
			ddup = nil
		}
	}()

	buf := &bufferFlusher{Buffer: bytes.Buffer{}}
	opts := jpegQuality(50)
	limiter := NewFrameLimiter(framerate)
	// Create image that can contain the wanted output (desktop)
	finalBounds := screenshot.GetDisplayBounds(n)
	imgBuf := image.NewRGBA(finalBounds)
	lastBounds := finalBounds

	for {
		select {
		case <-ctx.Done():
			return
		default:
			limiter.Wait()
		}
		bounds := screenshot.GetDisplayBounds(n)
		newBounds := image.Rect(0, 0, int(bounds.Dx()), int(bounds.Dy()))
		if newBounds != lastBounds {
			lastBounds = newBounds
			imgBuf = image.NewRGBA(lastBounds)

			// Throw away old ddup
			if ddup != nil {
				ddup.Release()
				ddup = nil
			}
		}
		// create output duplication if doesn't exist yet (maybe due to resolution change)
		if ddup == nil {
			ddup, err = d3d.NewIDXGIOutputDuplication(device, deviceCtx, uint(n))
			if err != nil {
				fmt.Printf("err: %v\n", err)
				continue
			}
		}

		// Grab an image.RGBA from the current output presenter
		err = ddup.GetImage(imgBuf, 0)
		if err != nil {
			if errors.Is(err, d3d.ErrNoImageYet) {
				// don't update
				continue
			}
			fmt.Printf("Err ddup.GetImage: %v\n", err)
			// Retry with new ddup, can occur when changing resolution
			ddup.Release()
			ddup = nil
			continue
		}
		buf.Reset()
		resized := resize.Resize(1920, 1080, imgBuf, resize.Bilinear)
		encodeJpeg(buf, resized, opts)

		// encodeJpeg(buf, imgBuf, opts)
		out.Update(buf.Bytes())
	}
}

// Workaround for jpeg.Encode(), which requires a Flush()
// method to not call `bufio.NewWriter`
type bufferFlusher struct {
	bytes.Buffer
}

func (*bufferFlusher) Flush() error { return nil }
