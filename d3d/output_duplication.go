package d3d

import (
	"errors"
	"fmt"
	"image"

	"unsafe"

	"github.com/kirides/screencapture/swizzle"
)

type PointerInfo struct {
	pos POINT

	size           POINT
	shapeInBuffer  []byte
	shapeOutBuffer *image.RGBA
	visible        bool
}

type OutputDuplicator struct {
	device            *ID3D11Device
	deviceCtx         *ID3D11DeviceContext
	outputDuplication *IDXGIOutputDuplication

	stagedTex  *ID3D11Texture2D
	surface    *IDXGISurface
	mappedRect DXGI_MAPPED_RECT
	size       POINT

	pointerInfo PointerInfo
	DrawPointer bool

	// TODO: handle DPI? Do we need it?
	dirtyRects    []RECT
	movedRects    []_DXGI_OUTDUPL_MOVE_RECT
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

var ErrNoImageYet = errors.New("no image yet")

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
		return nil, nil, nil, fmt.Errorf("failed to get the description. %w", HRESULT(hr))
	}

	if desc.DesktopImageInSystemMemory != 0 {
		// TODO: Figure out WHEN exactly this can occur, and if we can make use of it
		dup.size = POINT{int32(desc.ModeDesc.Width), int32(desc.ModeDesc.Height)}
		hr = dup.outputDuplication.MapDesktopSurface(&dup.mappedRect)
		if !failed(hr) {
			return dup.outputDuplication.UnMapDesktopSurface, &dup.mappedRect, &dup.size, nil
		}
	}

	var desktop *IDXGIResource
	var frameInfo _DXGI_OUTDUPL_FRAME_INFO

	// Release a possible previous frame
	// TODO: Properly use ReleaseFrame...

	dup.ReleaseFrame()
	hrF := dup.outputDuplication.AcquireNextFrame(timeoutMs, &frameInfo, &desktop)
	dup.acquiredFrame = true
	if failed(int32(hrF)) {
		if HRESULT(hrF) == DXGI_ERROR_WAIT_TIMEOUT {
			return nil, nil, nil, ErrNoImageYet
		}
		return nil, nil, nil, fmt.Errorf("failed to AcquireNextFrame. %w", HRESULT(hrF))
	}
	// If we do not release the frame ASAP, we only get FPS / 2 frames :/
	// Something wrong here?
	defer dup.ReleaseFrame()
	defer desktop.Release()

	if dup.DrawPointer {
		if err := dup.updatePointer(&frameInfo); err != nil {
			return nil, nil, nil, err
		}
	}

	if frameInfo.AccumulatedFrames == 0 {
		return nil, nil, nil, ErrNoImageYet
	}
	var desktop2d *ID3D11Texture2D
	hr = desktop.QueryInterface(iid_ID3D11Texture2D, &desktop2d)
	if failed(hr) {
		return nil, nil, nil, fmt.Errorf("failed to QueryInterface(iid_ID3D11Texture2D, ...). %w", HRESULT(hr))
	}
	defer desktop2d.Release()

	if dup.stagedTex == nil {
		hr = dup.initializeStage(desktop2d)
		if failed(hr) {
			return nil, nil, nil, fmt.Errorf("failed to InitializeStage. %w", HRESULT(hr))
		}
	}

	// NOTE: we could use a single, large []byte buffer and use it as storage for moved rects & dirty rects
	if frameInfo.TotalMetadataBufferSize > 0 {
		// Handling moved / dirty rects, to reduce GPU<->CPU memory copying
		moveRectsRequired := uint32(1)
		for {
			if len(dup.movedRects) < int(moveRectsRequired) {
				dup.movedRects = make([]_DXGI_OUTDUPL_MOVE_RECT, moveRectsRequired)
			}
			hr = dup.outputDuplication.GetFrameMoveRects(dup.movedRects, &moveRectsRequired)
			if failed(hr) {
				if HRESULT(hr) == DXGI_ERROR_MORE_DATA {
					continue
				}
				return nil, nil, nil, fmt.Errorf("failed to GetFrameMoveRects. %w", HRESULT(hr))
			}
			dup.movedRects = dup.movedRects[:moveRectsRequired]
			break
		}

		dirtyRectsRequired := uint32(1)
		for {
			if len(dup.dirtyRects) < int(dirtyRectsRequired) {
				dup.dirtyRects = make([]RECT, dirtyRectsRequired)
			}
			hr = dup.outputDuplication.GetFrameDirtyRects(dup.dirtyRects, &dirtyRectsRequired)
			if failed(hr) {
				if HRESULT(hr) == DXGI_ERROR_MORE_DATA {
					continue
				}
				return nil, nil, nil, fmt.Errorf("failed to GetFrameDirtyRects. %w", HRESULT(hr))
			}
			dup.dirtyRects = dup.dirtyRects[:dirtyRectsRequired]
			break
		}

		box := _D3D11_BOX{
			Front: 0,
			Back:  1,
		}
		if len(dup.movedRects) == 0 {
			for i := 0; i < len(dup.dirtyRects); i++ {
				box.Left = uint32(dup.dirtyRects[i].Left)
				box.Top = uint32(dup.dirtyRects[i].Top)
				box.Right = uint32(dup.dirtyRects[i].Right)
				box.Bottom = uint32(dup.dirtyRects[i].Bottom)

				dup.deviceCtx.CopySubresourceRegion2D(dup.stagedTex, 0, box.Left, box.Top, 0, desktop2d, 0, &box)
			}
		} else {
			// TODO: handle moved rects, then dirty rects
			// for now, just update the whole image instead
			dup.deviceCtx.CopyResource2D(dup.stagedTex, desktop2d)
		}
	} else {
		// no frame metadata, copy whole image
		dup.deviceCtx.CopyResource2D(dup.stagedTex, desktop2d)
		if !dup.needsSwizzle {
			dup.needsSwizzle = true
		}
		print("no frame metadata\n")
	}

	hr = dup.surface.Map(&dup.mappedRect, DXGI_MAP_READ)
	if failed(hr) {
		return nil, nil, nil, fmt.Errorf("failed to surface_.Map(...). %v", HRESULT(hr))
	}
	return dup.surface.Unmap, &dup.mappedRect, &dup.size, nil
}

func (dup *OutputDuplicator) GetImage(img *image.RGBA, timeoutMs uint) error {
	unmap, mappedRect, size, err := dup.Snapshot(timeoutMs)
	if err != nil {
		return err
	}
	defer unmap()
	hMem := mappedRect.PBits

	bitmapDataSize := int32(((int64(size.X)*32 + 31) / 32) * 4 * int64(size.Y))

	// copy source bytes into image.RGBA.Pix using memory interpretation
	imageBytes := ((*[1 << 30]byte)(unsafe.Pointer(hMem)))[:bitmapDataSize:bitmapDataSize]
	copy(img.Pix[:bitmapDataSize], imageBytes)
	dup.drawPointer(img)
	if dup.needsSwizzle {
		swizzle.BGRA(img.Pix)
	}

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

func (dup *OutputDuplicator) updatePointer(info *_DXGI_OUTDUPL_FRAME_INFO) error {
	if info.LastMouseUpdateTime == 0 {
		return nil
	}
	dup.pointerInfo.visible = info.PointerPosition.Visible != 0
	dup.pointerInfo.pos = info.PointerPosition.Position

	if info.PointerShapeBufferSize != 0 {
		// new shape
		if len(dup.pointerInfo.shapeInBuffer) < int(info.PointerShapeBufferSize) {
			dup.pointerInfo.shapeInBuffer = make([]byte, info.PointerShapeBufferSize)
		}
		var requiredSize uint32
		var pointerInfo _DXGI_OUTDUPL_POINTER_SHAPE_INFO

		hr := dup.outputDuplication.GetFramePointerShape(info.PointerShapeBufferSize,
			dup.pointerInfo.shapeInBuffer,
			&requiredSize,
			&pointerInfo,
		)
		if hr != 0 {
			return fmt.Errorf("unable to obtain frame pointer shape")
		}
		neededSize := pointerInfo.Width * pointerInfo.Height * 4
		dup.pointerInfo.shapeOutBuffer = image.NewRGBA(image.Rect(0, 0, int(pointerInfo.Width), int(pointerInfo.Height)))
		if len(dup.pointerInfo.shapeOutBuffer.Pix) < int(neededSize) {
			dup.pointerInfo.shapeOutBuffer.Pix = make([]byte, neededSize)
		}

		if pointerInfo.Type == DXGI_OUTDUPL_POINTER_SHAPE_TYPE_MONOCHROME {
			dup.pointerInfo.size = POINT{int32(pointerInfo.Width), int32(pointerInfo.Height)}

			xor_offset := pointerInfo.Pitch * (pointerInfo.Height / 2)
			andMap := dup.pointerInfo.shapeInBuffer
			xorMap := dup.pointerInfo.shapeInBuffer[:xor_offset]
			out_pixels := dup.pointerInfo.shapeOutBuffer.Pix
			widthBytes := (pointerInfo.Width + 7) / 8

			imgHeight := pointerInfo.Height / 2

			for j := 0; j < int(imgHeight); j++ {
				bit := byte(0x80)

				for i := 0; i < int(pointerInfo.Width); i++ {
					andByte := andMap[j*int(widthBytes)+i/8]
					xorByte := xorMap[j*int(widthBytes)+i/8]
					andBit := 0
					if (andByte & bit) != 0 {
						andBit = 1
					}
					xorBit := 0
					if (xorByte & bit) != 0 {
						xorBit = 1
					}
					outDx := j*int(pointerInfo.Width)*4 + i*4
					if andBit == 0 {
						if xorBit == 0 {
							out_pixels[outDx+0] = 0x00
							out_pixels[outDx+1] = 0x00
							out_pixels[outDx+2] = 0x00
							out_pixels[outDx+3] = 0x00
						} else {
							out_pixels[outDx+0] = 0xFF
							out_pixels[outDx+1] = 0xFF
							out_pixels[outDx+2] = 0xFF
							out_pixels[outDx+3] = 0xFF
						}
					} else {
						if xorBit == 0 {
							out_pixels[outDx+0] = 0x00
							out_pixels[outDx+1] = 0x00
							out_pixels[outDx+2] = 0x00
							out_pixels[outDx+3] = 0x00
						} else {
							out_pixels[outDx+0] = 0x00
							out_pixels[outDx+1] = 0x00
							out_pixels[outDx+2] = 0x00
							out_pixels[outDx+3] = 0xFF
						}
					}
					if bit == 0x01 {
						bit = 0x80
					} else {
						bit = bit >> 1
					}
				}
			}
		} else if pointerInfo.Type == DXGI_OUTDUPL_POINTER_SHAPE_TYPE_COLOR {
			dup.pointerInfo.size = POINT{int32(pointerInfo.Width), int32(pointerInfo.Height)}

			out, in := dup.pointerInfo.shapeOutBuffer.Pix, dup.pointerInfo.shapeInBuffer
			for j := 0; j < int(pointerInfo.Height); j++ {
				tout := out[j*int(pointerInfo.Pitch):]
				tin := in[j*int(pointerInfo.Pitch):]
				copy(tout, tin[:pointerInfo.Pitch])
			}
		} else if pointerInfo.Type == DXGI_OUTDUPL_POINTER_SHAPE_TYPE_MASKED_COLOR {
			dup.pointerInfo.size = POINT{int32(pointerInfo.Width), int32(pointerInfo.Height)}

			// TODO: Properly add mask
			out, in := dup.pointerInfo.shapeOutBuffer.Pix, dup.pointerInfo.shapeInBuffer
			for j := 0; j < int(pointerInfo.Height); j++ {
				tout := out[j*int(pointerInfo.Pitch):]
				tin := in[j*int(pointerInfo.Pitch):]
				copy(tout, tin[:pointerInfo.Pitch])
			}
		} else {
			dup.pointerInfo.size = POINT{0, 0}
			return fmt.Errorf("unsupported type %v", pointerInfo.Type)
		}
	}
	return nil
}

func (dup *OutputDuplicator) drawPointer(img *image.RGBA) error {
	if !dup.DrawPointer {
		return nil
	}

	for j := 0; j < int(dup.pointerInfo.size.Y); j++ {
		for i := 0; i < int(dup.pointerInfo.size.X); i++ {
			col := dup.pointerInfo.shapeOutBuffer.At(i, j)
			_, _, _, a := col.RGBA()
			if a == 0 {
				// just dont draw invisible pixel?
				// TODO: correctly apply mask
				continue
			}

			img.Set(int(dup.pointerInfo.pos.X)+i, int(dup.pointerInfo.pos.Y)+j, col)
		}
	}
	return nil
}
