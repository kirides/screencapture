# Screencapture

## Motivation

I wanted to learn more about `COM` interop with Go and create a somewhat usable screen sharing tool

## Libaries used

This application uses D3D11 `IDXGIOutputDuplication` to create a somewhat _realtime_ desktop presentation

- `github.com/mattn/go-mjpeg` for mjpeg streaming
- `github.com/kbinani/screenshot` for comparison with GDI `BitBlt` (slightly modified source, to support re-using `image.RGBA`)
- `golang.org/x/exp/shiny/driver/internal/swizzle` for faster BGRA -> RGBA conversion (see [shiny LICENSE](./swizzle/LICENSE))
- `github.com/pixiv/go-libjpeg/jpeg` for fast jpeg encoding
  - enable with `go build -tag jpegturbo`

## Demo

Just build the application and run it.
It should serve all your monitors on an URL like `http://127.0.0.1:8023/watch?screen=N`  
where `screen=N <- N` is the monitor index, starting at zero (`0`).

By changing the lines in `cmd/example/main.go` regarding the streaming, you can switch between GDI `BitBlt` or `IDXGIOutputDuplication`

```go
// ...
framerate := 10
for i := 0; i < n; i++ {
    // ...
    // streamDisplay(ctx, i, framerate, stream)  // <= USE GDI BitBlt
    streamDisplayDXGI(ctx, i, framerate, stream) // <= USE IDXGIOutputDuplication
    // captureScreenTranscode(ctx, i, desiredFps)
    http.HandleFunc(fmt.Sprintf("/mjpeg%d", i), stream.ServeHTTP)
}
// ...
```

### screen recording with ffmpeg

The code contains the function `captureScreenTranscode` which allows you to record the
selected screen directly into ffmpeg and transcode it to h264 in an mp4 container.

## Performance

Performance _is not_ optimized to 100%, there are still thing that could be improved.

- only copying the dirty-rectangles (less GPU<->CPU communication)
- faster swizzle implementation using AVX/2 (less time for converting the BGRA texture)
- _profile ... profile ... profile_

Overall the current implementation is about 2-5x faster than GDI `BitBlt` (depending on the resolution, 
the higher the bigger the difference) and uses a lot less resources for cases where there arent any changes to the screen.

## app.manifest

To make use of `IDXGIOutput5::DuplicateOutput1`, an application has to provide support for `PerMonitorV2` DPI-Awareness (Windows 10 1703+)
This is usually done by providing an my-executable.exe.manifest file either next to the executable, or as an embedded resource.

In this application there are calls to `IsValidDpiAwarenessContext` and `SetThreadDpiAwarenessContext` which circumvent the requirement.
