package d3d

import "strconv"

type _DXGI_RATIONAL struct {
	Numerator   uint32
	Denominator uint32
}
type _DXGI_MODE_DESC struct {
	Width            uint32
	Height           uint32
	Rational         _DXGI_RATIONAL
	Format           uint32 // DXGI_FORMAT
	ScanlineOrdering uint32 // DXGI_MODE_SCANLINE_ORDER
	Scaling          uint32 // DXGI_MODE_SCALING
}

type _DXGI_OUTDUPL_DESC struct {
	ModeDesc                   _DXGI_MODE_DESC
	Rotation                   uint32 // DXGI_MODE_ROTATION
	DesktopImageInSystemMemory uint32 // BOOL
}

type _DXGI_SAMPLE_DESC struct {
	Count   uint32
	Quality uint32
}

type POINT struct {
	X int32
	Y int32
}
type _DXGI_OUTDUPL_POINTER_POSITION struct {
	Position POINT
	Visible  uint32
}
type _DXGI_OUTDUPL_FRAME_INFO struct {
	LastPresentTime           int64
	LastMouseUpdateTime       int64
	AccumulatedFrames         uint32
	RectsCoalesced            uint32
	ProtectedContentMaskedOut uint32
	PointerPosition           _DXGI_OUTDUPL_POINTER_POSITION
	TotalMetadataBufferSize   uint32
	PointerShapeBufferSize    uint32
}
type DXGI_MAPPED_RECT struct {
	Pitch int32
	PBits uintptr
}

const (
	ERROR_INVALID_ARG            _DXGI_ERROR = 0x80070057
	DXGI_ERROR_ACCESS_LOST       _DXGI_ERROR = 0x887A0026
	DXGI_ERROR_INVALID_CALL      _DXGI_ERROR = 0x887A0001
	DXGI_ERROR_WAIT_TIMEOUT      _DXGI_ERROR = 0x887A0027
	DXGI_ERROR_WAS_STILL_DRAWING _DXGI_ERROR = 0x887A000A
	DXGI_ERROR_UNSUPPORTED       _DXGI_ERROR = 0x887A0004
	DXGI_ERROR_DEVICE_HUNG       _DXGI_ERROR = 0x887A0006
)

type _DXGI_ERROR uint32

func (e _DXGI_ERROR) Error() string {
	switch e {
	case ERROR_INVALID_ARG:
		return "ERROR_INVALID_ARG"
	case DXGI_ERROR_ACCESS_LOST:
		return "DXGI_ERROR_ACCESS_LOST"
	case DXGI_ERROR_INVALID_CALL:
		return "DXGI_ERROR_INVALID_CALL"
	case DXGI_ERROR_WAIT_TIMEOUT:
		return "DXGI_ERROR_WAIT_TIMEOUT"
	case DXGI_ERROR_WAS_STILL_DRAWING:
		return "DXGI_ERROR_WAS_STILL_DRAWING"
	case DXGI_ERROR_UNSUPPORTED:
		return "DXGI_ERROR_UNSUPPORTED"
	case DXGI_ERROR_DEVICE_HUNG:
		return "DXGI_ERROR_DEVICE_HUNG"
	}

	return "0x" + strconv.FormatUint(uint64(e), 16)
}
