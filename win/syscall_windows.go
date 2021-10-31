package win

//go:generate mkwinsyscall -output zsyscall_windows.go syscall_windows.go

type (
	BOOL          uint32
	BOOLEAN       byte
	BYTE          byte
	DWORD         uint32
	DWORD64       uint64
	HANDLE        uintptr
	HLOCAL        uintptr
	LARGE_INTEGER int64
	LONG          int32
	LPVOID        uintptr
	SIZE_T        uintptr
	UINT          uint32
	ULONG_PTR     uintptr
	ULONGLONG     uint64
	WORD          uint16

	HWND uintptr
)

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}
type RGBQUAD struct {
	RgbBlue     byte
	RgbGreen    byte
	RgbRed      byte
	RgbReserved byte
}

type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors *RGBQUAD
}

const (
	OBJ_BITMAP = 7
)

//sys	GetDesktopWindow() (h HWND) = User32.GetDesktopWindow
//sys	GetDIBits(hdc syscall.Handle, hbmp syscall.Handle, uStartScan uint32, cScanLines uint32, lpvBits *byte, lpbi *BITMAPINFO, uUsage uint32) (v int32, err error) = Gdi32.GetDIBits
//sys	GetCurrentObject(hdc syscall.Handle, typ uint16) (h syscall.Handle) = Gdi32.GetCurrentObject

//sys	setWindowsHookExW(idHook int32, lpfn unsafe.Pointer, hmod syscall.Handle, dwThreadId uint32) (h syscall.Handle, err error) = User32.SetWindowsHookExW

//sys	openClipboard(h syscall.Handle) (err error) = User32.OpenClipboard
//sys	closeClipboard() (err error) = User32.CloseClipboard
//sys	emptyClipboard() (err error) = User32.EmptyClipboard
//sys	registerClipboardFormat(name string) (id uint32, err error) = User32.RegisterClipboardFormatW
//sys	enumClipboardFormats(format uint32) (id uint32, err error) = User32.EnumClipboardFormats
//sys	getClipboardFormatName(format uint32, lpszFormatName *uint16, cchMaxCount int32) (len int32, err error) = User32.GetClipboardFormatNameW
//sys	getClipboardData(uFormat uint32) (h syscall.Handle, err error) = User32.GetClipboardData
//sys	setClipboardData(uFormat uint32, hMem syscall.Handle) (h syscall.Handle, err error) = User32.SetClipboardData
//sys	isClipboardFormatAvailable(uFormat uint32) (err error) = User32.IsClipboardFormatAvailable
//sys	AddClipboardFormatListener(hWnd syscall.Handle) (err error) = User32.AddClipboardFormatListener
//sys	RemoveClipboardFormatListener(hWnd syscall.Handle) (err error) = User32.RemoveClipboardFormatListener

//sys	GetProcessHeap() (hHeap syscall.Handle, err error) = Kernel32.GetProcessHeap
//sys	HeapAlloc(hHeap syscall.Handle, dwFlags uint32, dwSize uintptr) (lpMem uintptr, err error) = Kernel32.HeapAlloc
//sys	HeapFree(hHeap syscall.Handle, dwFlags uint32, lpMem uintptr) (err error) = Kernel32.HeapFree
//sys	heapSize(hHeap syscall.Handle, dwFlags uint32, lpMem uintptr) (size uintptr, err error) [failretval==^uintptr(r0)] = Kernel32.HeapSize

//sys	dragQueryFile(hDrop syscall.Handle, iFile int, buf *uint16, len uint32) (n int, err error) = Shell32.DragQueryFileW

const (
	DpiAwarenessContextUndefined         = 0
	DpiAwarenessContextUnaware           = -1
	DpiAwarenessContextSystemAware       = -2
	DpiAwarenessContextPerMonitorAware   = -3
	DpiAwarenessContextPerMonitorAwareV2 = -4
	DpiAwarenessContextUnawareGdiScaled  = -5
)

//sys	SetThreadDpiAwarenessContext(value int32) (n int, err error) = User32.SetThreadDpiAwarenessContext
//sys	IsValidDpiAwarenessContext(value int32) (n bool) = User32.IsValidDpiAwarenessContext
