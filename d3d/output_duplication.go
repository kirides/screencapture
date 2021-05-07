package d3d

import (
	"errors"
	"fmt"
	"image"
	"screen-share/swizzle"
	"unsafe"
)

type OutputDuplicator struct {
	device            *ID3D11Device
	deviceCtx         *ID3D11DeviceContext
	outputDuplication *IDXGIOutputDuplication

	stagedTex  *ID3D11Texture2D
	surface    *IDXGISurface
	mappedRect DXGI_MAPPED_RECT
	size       POINT

	// TODO: gther info about update regions and only update those
	// TODO: handle DPI? Do we need it?

	acquiredFrame bool
	needsSwizzle  bool // in case we use DuplicateOutput1, swizzle is not neccessery
}

func (dup *OutputDuplicator) initializeStage(texture *ID3D11Texture2D) int32 {

	/*
		TODO: Only do this on changes!
	*/
	var hr int32
	desc := _D3D11_TEXTURE2D_DESC{}
	hr = texture.GetDesc(&desc)
	if failed(hr) {
		return hr
	}

	desc.Usage = D3D11_USAGE_STAGING
	desc.CPUAccessFlags = D3D11_CPU_ACCESS_READ
	desc.BindFlags = 0
	desc.MipLevels = 1
	desc.ArraySize = 1
	desc.MiscFlags = 0
	desc.SampleDesc.Count = 1

	hr = dup.device.CreateTexture2D(&desc, &dup.stagedTex)
	if failed(hr) {
		return hr
	}

	hr = dup.stagedTex.QueryInterface(iid_IDXGISurface, &dup.surface)
	if failed(hr) {
		return hr
	}
	dup.size = POINT{X: int32(desc.Width), Y: int32(desc.Height)}

	return 0
}

func (dup *OutputDuplicator) Release() {
	if dup.stagedTex != nil {
		dup.stagedTex.Release()
		dup.stagedTex = nil
	}
	if dup.surface != nil {
		dup.surface.Release()
		dup.surface = nil
	}
	if dup.outputDuplication != nil {
		dup.outputDuplication.Release()
		dup.outputDuplication = nil
	}
}

var errNoImageYet = errors.New("no image yet")

type unmapFn func() int32

func (dup *OutputDuplicator) ReleaseFrame() {
	if dup.acquiredFrame {
		dup.outputDuplication.ReleaseFrame()
		dup.acquiredFrame = false
	}
}

// returns DXGI_FORMAT_B8G8R8A8_UNORM data
func (dup *OutputDuplicator) Snapshot(timeoutMs uint) (unmapFn, *DXGI_MAPPED_RECT, *POINT, error) {
	var hr int32
	desc := _DXGI_OUTDUPL_DESC{}
	hr = dup.outputDuplication.GetDesc(&desc)
	if failed(hr) {
		return nil, nil, nil, fmt.Errorf("failed to get the description. %w", _DXGI_ERROR(hr))
	}

	if desc.DesktopImageInSystemMemory != 0 {
		// TODO: Figure out WHEN exactly this cann occur, and if we can make use of it
		dup.size = POINT{int32(desc.ModeDesc.Width), int32(desc.ModeDesc.Height)}
		hr = dup.outputDuplication.MapDesktopSurface(&dup.mappedRect)
		if !failed(hr) {
			return dup.outputDuplication.UnMapDesktopSurface, &dup.mappedRect, &dup.size, nil
		}
	}

	var desktop *IDXGIResource
	var frameInfo _DXGI_OUTDUPL_FRAME_INFO
	// for {
	dup.ReleaseFrame()
	hrF := dup.outputDuplication.AcquireNextFrame(timeoutMs, &frameInfo, &desktop)
	if failed(int32(hrF)) {
		if _DXGI_ERROR(hrF) == DXGI_ERROR_WAIT_TIMEOUT {
			dup.acquiredFrame = true
			return nil, nil, nil, errNoImageYet
		}
		return nil, nil, nil, fmt.Errorf("failed to AcquireNextFrame. %w", _DXGI_ERROR(hrF))
	}
	dup.acquiredFrame = true
	if frameInfo.AccumulatedFrames == 0 {
		desktop.Release()
		return nil, nil, nil, errNoImageYet
	}

	// }

	var desktop2d *ID3D11Texture2D
	hr = desktop.QueryInterface(iid_ID3D11Texture2D, &desktop2d)
	desktop.Release()
	if failed(hr) {
		return nil, nil, nil, fmt.Errorf("failed to QueryInterface(iid_ID3D11Texture2D, ...). %w", _DXGI_ERROR(hr))
	}

	if dup.stagedTex == nil {
		hr = dup.initializeStage(desktop2d)
		if failed(hr) {
			return nil, nil, nil, fmt.Errorf("failed to InitializeStage. %w", _DXGI_ERROR(hr))
		}
	}

	// TODO: Optimize by only using dirty rects and CopySubresourceRegion2D

	// box := _D3D11_BOX{
	// 	Left:   0,
	// 	Top:    0,
	// 	Right:  desc.ModeDesc.Width,
	// 	Bottom: desc.ModeDesc.Height,
	// 	Front:  0,
	// 	Back:   1,
	// }

	// dup.deviceCtx.CopySubresourceRegion2D(dup.stage_, 0, 0, 0, 0, desktop2d, 0, &box)

	dup.deviceCtx.CopyResource2D(dup.stagedTex, desktop2d)

	hr = dup.surface.Map(&dup.mappedRect, DXGI_MAP_READ)
	if failed(hr) {
		return nil, nil, nil, fmt.Errorf("failed to surface_.Map(...). %v", _DXGI_ERROR(hr))
	}
	return dup.surface.Unmap, &dup.mappedRect, &dup.size, nil
}

// func byteSliceFromUintptr(v uintptr, len int) []byte {
// 	res := []byte{}
// 	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&res))
// 	hdr.Data = v
// 	hdr.Cap = len
// 	hdr.Len = len
// 	return res
// }

func (dup *OutputDuplicator) GetImage(img *image.RGBA, timeoutMs uint) error {
	unmap, mappedRect, size, err := dup.Snapshot(timeoutMs)
	if err != nil {
		if errors.Is(err, errNoImageYet) {
			return nil
		}
		return err
	}
	defer unmap()
	hMem := mappedRect.PBits

	bitmapDataSize := int32(((int64(size.X)*32 + 31) / 32) * 4 * int64(size.Y))
	// using slice header tricks
	// bgra := byteSliceFromUintptr(hMem, int(bitmapDataSize))

	// using memory interpretation
	bgra := ((*[1 << 30]byte)(unsafe.Pointer(hMem)))[:bitmapDataSize:bitmapDataSize]
	if dup.needsSwizzle {
		swizzle.BGRA(bgra)
	}
	copy(img.Pix[:bitmapDataSize], bgra)

	// manual swizzle B <-> R

	// for i := int32(0); i < bitmapDataSize; i += 4 {
	// 	v0 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i)))
	// 	v1 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i) + 1))
	// 	v2 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i) + 2))

	// 	// BGRA => RGBA, no need to read alpha, always 255.
	// 	img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255
	// }
	return nil
}
