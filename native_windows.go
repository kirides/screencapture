package main

import (
	"errors"
	"image"
	"screen-share/swizzle"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

func CaptureImg(img *image.RGBA, x, y, width, height int) error {
	return captureImg(img, x, y, width, height)
}

func captureImg(img *image.RGBA, x, y, width, height int) error {
	hWnd := syscall.Handle(getDesktopWindow())
	hdc := win.GetDC(win.HWND(hWnd))
	if hdc == 0 {
		return errors.New("GetDC failed")
	}
	defer win.ReleaseDC(win.HWND(hWnd), hdc)

	memory_device := win.CreateCompatibleDC(hdc)
	if memory_device == 0 {
		return errors.New("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(memory_device)

	bitmap := win.CreateCompatibleBitmap(hdc, int32(width), int32(height))
	if bitmap == 0 {
		return errors.New("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(bitmap))

	old := win.SelectObject(memory_device, win.HGDIOBJ(bitmap))
	if old == 0 {
		return errors.New("SelectObject failed")
	}
	defer win.SelectObject(memory_device, old)

	if !win.BitBlt(memory_device, 0, 0, int32(width), int32(height), hdc, int32(x), int32(y), win.SRCCOPY) {
		return errors.New("BitBlt failed")
	}
	var bm win.BITMAP
	win.GetObject(win.HGDIOBJ(bitmap), unsafe.Sizeof(win.BITMAP{}), unsafe.Pointer(&bm))

	var header BITMAPINFOHEADER
	header.BiSize = uint32(unsafe.Sizeof(header))
	header.BiPlanes = 1
	header.BiBitCount = 32
	header.BiWidth = bm.BmWidth
	header.BiHeight = -bm.BmHeight
	header.BiCompression = win.BI_RGB

	// GetDIBits balks at using Go memory on some systems.
	bitmapDataSize := int32(((int64(bm.BmWidth)*int64(header.BiBitCount) + 31) / 32) * 4 * int64(bm.BmHeight))

	hHeap, _ := getProcessHeap()
	hMem, _ := heapAlloc(hHeap, 0, uintptr(bitmapDataSize))
	defer heapFree(hHeap, 0, hMem)

	if v, _ := getDIBits(syscall.Handle(hdc), syscall.Handle(bitmap), 0, uint32(height), (*uint8)(unsafe.Pointer(hMem)), (*BITMAPINFO)(unsafe.Pointer(&header)), win.DIB_RGB_COLORS); v == 0 {
		return errors.New("GetDIBits failed")
	}

	// using memory interpretation
	bgra := ((*[1 << 30]byte)(unsafe.Pointer(hMem)))[:bitmapDataSize:bitmapDataSize]
	swizzle.BGRA(bgra)
	copy(img.Pix[:bitmapDataSize], bgra)

	// manual swizzle B <-> R, A = 255

	// for i := int32(0); i < bitmapDataSize; i += 4 {
	// 	v0 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i)))
	// 	v1 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i) + 1))
	// 	v2 := *(*uint8)(unsafe.Pointer(hMem + uintptr(i) + 2))

	// 	// BGRA => RGBA, and set A to 255
	// 	img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v2, v1, v0, 255
	// }
	return nil
}
