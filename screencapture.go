package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os/exec"
	"runtime"
	"screen-share/d3d"
	"time"

	"github.com/kbinani/screenshot"
)

func captureScreenTranscode(ctx context.Context, n int, framerate int) {
	max := screenshot.NumActiveDisplays()
	if n >= max {
		fmt.Printf("Not enough displays\n")
		return
	}
	screenBounds := screenshot.GetDisplayBounds(n)
	transcoder := newVideotranscoder(fmt.Sprintf("screen_%d.mp4", n), screenBounds.Dx(), screenBounds.Dy(), float32(framerate))

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

	ddup, err := d3d.NewIDXGIOutputDuplication(device, deviceCtx, uint(n))
	if err != nil {
		fmt.Printf("Err NewIDXGIOutputDuplication: %v\n", err)
		return
	}
	defer ddup.Release()

	limiter := newFrameLimiter(framerate)

	// Create image that can contain the wanted output (desktop)
	imgBuf := image.NewRGBA(screenBounds)

	defer transcoder.Close()
	t1 := time.Now()
	numFrames := 0
	for {
		if time.Since(t1).Seconds() >= 1 {
			fmt.Printf("%d: written %d frames in 1s\n", n, numFrames)
			t1 = time.Now()
			numFrames = 0
		}
		select {
		case <-ctx.Done():
			return
		default:
			limiter.Wait()
		}
		// Grab an image.RGBA from the current output presenter
		err = ddup.GetImage(imgBuf, 0)
		if err != nil && !errors.Is(err, d3d.ErrNoImageYet) {
			fmt.Printf("Err ddup.GetImage: %v\n", err)
			return
		}

		numFrames++

		n, err := transcoder.Write(imgBuf.Pix)
		if err != nil || n != len(imgBuf.Pix) {
			fmt.Printf("Failed to write image: %v\n", err)
			return
		}
	}
}

type videotranscoder struct {
	cmd *exec.Cmd

	in io.WriteCloser
}

func newVideotranscoder(filePath string, width, height int, framerate float32) *videotranscoder {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-vsync", "0",
		"-f", "rawvideo",
		"-video_size", fmt.Sprintf("%dx%d", width, height),
		"-pixel_format", "rgba",
		"-framerate", fmt.Sprintf("%f", framerate),
		"-i", "-",
		// "-vf", "scale=-1:1080",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-crf", "26",
		"-tune", "zerolatency",
		filePath,
	)

	wc, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	return &videotranscoder{
		cmd: cmd,
		in:  wc,
	}
}
func (v *videotranscoder) Write(buf []byte) (int, error) {
	return v.in.Write(buf)
}
func (v *videotranscoder) Close() error {
	// v.out.Close()
	v.in.Close()
	return nil
}
