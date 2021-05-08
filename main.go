package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"runtime"

	// _ "net/http/pprof"
	"os"
	"os/signal"
	"screen-share/d3d"
	"strconv"
	"syscall"
	"time"

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

	const desiredFps = 30
	framerate := time.Second / desiredFps
	for i := 0; i < n-1; i++ {
		fmt.Fprintf(os.Stderr, "Registering stream %d\n", i)
		stream := mjpeg.NewStreamWithInterval(framerate)
		defer stream.Close()
		// streamDisplay(ctx, i, desiredFps, stream)
		// streamDisplayDXGI(ctx, i, desiredFps, stream)
		captureScreenTranscode(ctx, i, desiredFps)
		http.HandleFunc(fmt.Sprintf("/mjpeg%d", i), stream.ServeHTTP)
	}
	go func() {
		http.ListenAndServe("127.0.0.1:8023", nil)

	}()
	<-ctx.Done()
	<-time.After(time.Second)
}

// Capture using "github.com/kbinani/screenshot" (modified to reuse image.RGBA)
func streamDisplay(ctx context.Context, n int, framerate time.Duration, out *mjpeg.Stream) {
	max := screenshot.NumActiveDisplays()
	if n >= max {
		fmt.Printf("Not enough displays\n")
		return
	}
	go func() {
		buf := &bufferFlusher{}
		opts := &jpeg.Options{Quality: 75}
		ticker := time.NewTicker(time.Second / framerate)

		var err error
		finalBounds := screenshot.GetDisplayBounds(n)
		imgBuf := image.NewRGBA(finalBounds)

		lastBounds := finalBounds
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			bounds := screenshot.GetDisplayBounds(n)

			x, y, hw, hh := bounds.Min.X, 0, bounds.Dx(), bounds.Dy()
			newBounds := image.Rect(0, 0, int(hw), int(hh))
			if newBounds != lastBounds {
				lastBounds = newBounds
				imgBuf = image.NewRGBA(lastBounds)
			}
			err = CaptureImg(imgBuf, int(x), int(y), int(hw), int(hh))
			if err != nil {
				fmt.Printf("Err CaptureImg: %v\n", err)
				continue
			}
			buf.Reset()

			jpeg.Encode(buf, imgBuf, opts)
			out.Update(buf.Bytes())
			<-ticker.C
		}
	}()
}

// Capture using IDXGIOutputDuplication
//     https://docs.microsoft.com/en-us/windows/win32/api/dxgi1_2/nn-dxgi1_2-idxgioutputduplication
func streamDisplayDXGI(ctx context.Context, n int, framerate time.Duration, out *mjpeg.Stream) {
	max := screenshot.NumActiveDisplays()
	if n >= max {
		fmt.Printf("Not enough displays\n")
		return
	}

	go func() {
		// Keep this thread, so windows/d3d11/dxgi can use their threadlocal caches, if any
		runtime.LockOSThread()
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

		const maxTimeoutMs = 150
		buf := &bufferFlusher{Buffer: bytes.Buffer{}}
		opts := &jpeg.Options{Quality: 75}
		ticker := time.NewTicker(time.Second / framerate)

		// Create image that can contain the wanted output (desktop)
		finalBounds := screenshot.GetDisplayBounds(n)
		imgBuf := image.NewRGBA(finalBounds)
		lastBounds := finalBounds

		for {
			select {
			case <-ctx.Done():
				return
			default:
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
					<-ticker.C
					continue
				}
			}

			// Grab an image.RGBA from the current output presenter
			err = ddup.GetImage(imgBuf, maxTimeoutMs)
			if err != nil {
				fmt.Printf("Err ddup.GetImage: %v\n", err)
				// Retry with new ddup, can occur when changing resolution
				ddup.Release()
				ddup = nil
				continue
			}
			buf.Reset()

			jpeg.Encode(buf, imgBuf, opts)
			out.Update(buf.Bytes())
			<-ticker.C
		}
	}()
}

// Workaround for jpeg.Encode(), which requires a Flush()
// method to not call `bufio.NewWriter`
type bufferFlusher struct {
	bytes.Buffer
}

func (*bufferFlusher) Flush() error { return nil }
