package d3d

type iDXGIObjectVtbl struct {
	iUnknownVtbl

	SetPrivateData          uintptr
	SetPrivateDataInterface uintptr
	GetPrivateData          uintptr
	GetParent               uintptr
}

type IDXGIAdapterVtbl struct {
	iDXGIObjectVtbl

	EnumOutputs           uintptr
	GetDesc               uintptr
	CheckInterfaceSupport uintptr
}
type IDXGIAdapter1Vtbl struct {
	IDXGIAdapterVtbl

	GetDesc1 uintptr
}

type iDXGIDeviceVtbl struct {
	iDXGIObjectVtbl

	CreateSurface          uintptr
	GetAdapter             uintptr
	GetGPUThreadPriority   uintptr
	QueryResourceResidency uintptr
	SetGPUThreadPriority   uintptr
}

type iDXGIDevice1Vtbl struct {
	iDXGIDeviceVtbl

	GetMaximumFrameLatency uintptr
	SetMaximumFrameLatency uintptr
}

type iDXGIDeviceSubObjectVtbl struct {
	iDXGIObjectVtbl

	GetDevice uintptr
}

type iDXGISurfaceVtbl struct {
	iDXGIDeviceSubObjectVtbl

	GetDesc uintptr
	Map     uintptr
	Unmap   uintptr
}

type iDXGIResourceVtbl struct {
	iDXGIDeviceSubObjectVtbl

	GetSharedHandle     uintptr
	GetUsage            uintptr
	SetEvictionPriority uintptr
	GetEvictionPriority uintptr
}

type iDXGIOutputVtbl struct {
	iDXGIObjectVtbl

	GetDesc                     uintptr
	GetDisplayModeList          uintptr
	FindClosestMatchingMode     uintptr
	WaitForVBlank               uintptr
	TakeOwnership               uintptr
	ReleaseOwnership            uintptr
	GetGammaControlCapabilities uintptr
	SetGammaControl             uintptr
	GetGammaControl             uintptr
	SetDisplaySurface           uintptr
	GetDisplaySurfaceData       uintptr
	GetFrameStatistics          uintptr
}

type iDXGIOutput1Vtbl struct {
	iDXGIOutputVtbl

	GetDisplayModeList1      uintptr
	FindClosestMatchingMode1 uintptr
	GetDisplaySurfaceData1   uintptr
	DuplicateOutput          uintptr
}

type iDXGIOutputDuplicationVtbl struct {
	iDXGIObjectVtbl

	GetDesc              uintptr
	AcquireNextFrame     uintptr
	GetFrameDirtyRects   uintptr
	GetFrameMoveRects    uintptr
	GetFramePointerShape uintptr
	MapDesktopSurface    uintptr
	UnMapDesktopSurface  uintptr
	ReleaseFrame         uintptr
}
