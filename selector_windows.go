//go:build windows

package main

import (
	"image"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32w   = syscall.NewLazyDLL("user32.dll")
	gdi32w    = syscall.NewLazyDLL("gdi32.dll")
	kernel32w = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW           = user32w.NewProc("RegisterClassExW")
	pCreateWindowExW            = user32w.NewProc("CreateWindowExW")
	pDestroyWindow              = user32w.NewProc("DestroyWindow")
	pShowWindow                 = user32w.NewProc("ShowWindow")
	pDefWindowProcW             = user32w.NewProc("DefWindowProcW")
	pGetMessageW                = user32w.NewProc("GetMessageW")
	pTranslateMessage           = user32w.NewProc("TranslateMessage")
	pDispatchMessageW           = user32w.NewProc("DispatchMessageW")
	pPostQuitMessage            = user32w.NewProc("PostQuitMessage")
	pBeginPaint                 = user32w.NewProc("BeginPaint")
	pEndPaint                   = user32w.NewProc("EndPaint")
	pInvalidateRect             = user32w.NewProc("InvalidateRect")
	pGetClientRect              = user32w.NewProc("GetClientRect")
	pFillRect                   = user32w.NewProc("FillRect")
	pSetCapture                 = user32w.NewProc("SetCapture")
	pReleaseCapture             = user32w.NewProc("ReleaseCapture")
	pSetForegroundWindow        = user32w.NewProc("SetForegroundWindow")
	pSetCursor                  = user32w.NewProc("SetCursor")
	pLoadCursorW                = user32w.NewProc("LoadCursorW")
	pSetLayeredWindowAttributes = user32w.NewProc("SetLayeredWindowAttributes")
	pGetSystemMetrics           = user32w.NewProc("GetSystemMetrics")

	pCreateSolidBrush       = gdi32w.NewProc("CreateSolidBrush")
	pCreatePen              = gdi32w.NewProc("CreatePen")
	pSelectObject           = gdi32w.NewProc("SelectObject")
	pDeleteObject           = gdi32w.NewProc("DeleteObject")
	pGetStockObject         = gdi32w.NewProc("GetStockObject")
	pRectangleW             = gdi32w.NewProc("Rectangle")
	pCreateCompatibleDC     = gdi32w.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32w.NewProc("CreateCompatibleBitmap")
	pDeleteDC               = gdi32w.NewProc("DeleteDC")
	pBitBlt                 = gdi32w.NewProc("BitBlt")
	pGetDC                  = user32w.NewProc("GetDC")
	pReleaseDC              = user32w.NewProc("ReleaseDC")
	pGetDesktopWindow       = user32w.NewProc("GetDesktopWindow")
	pGetWindowDC            = user32w.NewProc("GetWindowDC")
	pSetWindowPos           = user32w.NewProc("SetWindowPos")
	pMoveWindow             = user32w.NewProc("MoveWindow")

	pGetModuleHandleW = kernel32w.NewProc("GetModuleHandleW")

	pCreateFontW = gdi32w.NewProc("CreateFontW")

	shell32w              = syscall.NewLazyDLL("shell32.dll")
	pSHBrowseForFolderW   = shell32w.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW = shell32w.NewProc("SHGetPathFromIDListW")

	ole32w        = syscall.NewLazyDLL("ole32.dll")
	pCoInitialize = ole32w.NewProc("CoInitialize")

	dwmapiw                = syscall.NewLazyDLL("dwmapi.dll")
	pDwmSetWindowAttribute = dwmapiw.NewProc("DwmSetWindowAttribute")

	uxthemew        = syscall.NewLazyDLL("uxtheme.dll")
	pSetWindowTheme = uxthemew.NewProc("SetWindowTheme")

	pGetWindowLongPtrW = user32w.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW = user32w.NewProc("SetWindowLongPtrW")
	pSendMessageW      = user32w.NewProc("SendMessageW")

	msimg32w    = syscall.NewLazyDLL("msimg32.dll")
	pAlphaBlend = msimg32w.NewProc("AlphaBlend")
)

const (
	wsPopup        = 0x80000000
	wsExTopmost    = 0x00000008
	wsExLayered    = 0x00080000
	wsExToolWindow = 0x00000080

	gwlStyle = ^uintptr(15)

	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmSetFont       = 0x0030
	wmCtlColorBtn   = 0x0135
	wmKeyDown       = 0x0100
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmMouseMove     = 0x0200
	wmRButtonDown   = 0x0204
	wmNcLButtonDown = 0x00A1

	htCaption = 2

	dwmwaUseImmersiveDarkMode = 20
	swpNoZOrder               = 0x0004

	vkEscape = 0x1B
	swShow   = 5

	smCxScreen = 0
	smCyScreen = 1

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	idcCross = 32515

	lwaColorKey = 0x00000001
	lwaAlpha    = 0x00000002

	psSolid   = 0
	nullBrush = 5

	colorMagenta = 0x00FF00FF
	colorOverlay = 0x00000000
	colorBorder  = 0x0000FF00

	overlayAlpha = 255
	dimAlpha     = 140

	acSrcOver = 0x00
)

type wRECT struct{ Left, Top, Right, Bottom int32 }
type wPOINT struct{ X, Y int32 }

type wMSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      wPOINT
	_       uint32
}

type wWNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type wPAINTSTRUCT struct {
	HDC         uintptr
	FErase      int32
	RcPaint     wRECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

var sel struct {
	startX, startY int32
	endX, endY     int32
	dragging       bool
	done           bool
	cancelled      bool
	hwnd           uintptr

	darkBrush uintptr
	magBrush  uintptr
	borderPen uintptr
	cursor    uintptr

	bgBitmap uintptr
	bgDC     uintptr

	dimmedDC     uintptr
	dimmedBitmap uintptr

	overlayAlpha byte
	selectionClr uint32
}

func getXLParam(lp uintptr) int32 { return int32(int16(lp & 0xFFFF)) }
func getYLParam(lp uintptr) int32 { return int32(int16((lp >> 16) & 0xFFFF)) }

func parseColor(hex string) uint32 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0x0000FF00
	}
	r, _ := strconv.ParseUint(hex[0:2], 16, 8)
	g, _ := strconv.ParseUint(hex[2:4], 16, 8)
	b, _ := strconv.ParseUint(hex[4:6], 16, 8)
	return uint32(r) | (uint32(g) << 8) | (uint32(b) << 16)
}

func minI32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func selectorWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmEraseBkgnd:
		return 1

	case wmPaint:
		var ps wPAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

		var rc wRECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		width, height := rc.Right-rc.Left, rc.Bottom-rc.Top

		memDC, _, _ := pCreateCompatibleDC.Call(hdc)
		memBitmap, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
		oldBitmap, _, _ := pSelectObject.Call(memDC, memBitmap)

		const srccopy = 0x00CC0020
		pBitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), sel.dimmedDC, 0, 0, srccopy)

		if sel.dragging || sel.done {
			sr := wRECT{
				Left:   minI32(sel.startX, sel.endX),
				Top:    minI32(sel.startY, sel.endY),
				Right:  maxI32(sel.startX, sel.endX),
				Bottom: maxI32(sel.startY, sel.endY),
			}

			selW := sr.Right - sr.Left
			selH := sr.Bottom - sr.Top
			if selW > 0 && selH > 0 {
				pBitBlt.Call(memDC, uintptr(sr.Left), uintptr(sr.Top), uintptr(selW), uintptr(selH), sel.bgDC, uintptr(sr.Left), uintptr(sr.Top), srccopy)
			}

			oldPen, _, _ := pSelectObject.Call(memDC, sel.borderPen)
			nb, _, _ := pGetStockObject.Call(nullBrush)
			oldBrush, _, _ := pSelectObject.Call(memDC, nb)
			pRectangleW.Call(memDC, uintptr(sr.Left), uintptr(sr.Top), uintptr(sr.Right), uintptr(sr.Bottom))
			pSelectObject.Call(memDC, oldPen)
			pSelectObject.Call(memDC, oldBrush)
		}

		pBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), memDC, 0, 0, srccopy)

		pSelectObject.Call(memDC, oldBitmap)
		pDeleteObject.Call(memBitmap)
		pDeleteDC.Call(memDC)

		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0

	case wmLButtonDown:
		sel.startX = getXLParam(lParam)
		sel.startY = getYLParam(lParam)
		sel.endX = sel.startX
		sel.endY = sel.startY
		sel.dragging = true
		pSetCapture.Call(hwnd)
		pInvalidateRect.Call(hwnd, 0, 0)
		return 0

	case wmMouseMove:
		if sel.dragging {
			sel.endX = getXLParam(lParam)
			sel.endY = getYLParam(lParam)
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0

	case wmLButtonUp:
		if sel.dragging {
			sel.endX = getXLParam(lParam)
			sel.endY = getYLParam(lParam)
			sel.dragging = false
			sel.done = true
			pReleaseCapture.Call()
			pDestroyWindow.Call(hwnd)
		}
		return 0

	case wmRButtonDown:
		sel.cancelled = true
		pReleaseCapture.Call()
		pDestroyWindow.Call(hwnd)
		return 0

	case wmKeyDown:
		if wParam == vkEscape {
			sel.cancelled = true
			pReleaseCapture.Call()
			pDestroyWindow.Call(hwnd)
		}
		return 0

	case wmSetCursor:
		pSetCursor.Call(sel.cursor)
		return 1

	case wmDestroy:
		pDeleteObject.Call(sel.darkBrush)
		pDeleteObject.Call(sel.magBrush)
		pDeleteObject.Call(sel.borderPen)
		pDeleteObject.Call(sel.bgBitmap)
		pDeleteDC.Call(sel.bgDC)
		pDeleteObject.Call(sel.dimmedBitmap)
		pDeleteDC.Call(sel.dimmedDC)
		pPostQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

func selectRegion(vRect image.Rectangle, opacity int, colorHex string) (image.Rectangle, bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if opacity <= 0 {
		opacity = 140
	}
	sel.overlayAlpha = byte(opacity)
	sel.selectionClr = parseColor(colorHex)

	sel.startX, sel.startY = 0, 0
	sel.endX, sel.endY = 0, 0
	sel.dragging = false
	sel.done = false
	sel.cancelled = false
	sel.hwnd = 0

	hdcScreen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, hdcScreen)

	width := uintptr(vRect.Dx())
	height := uintptr(vRect.Dy())

	sel.bgDC, _, _ = pCreateCompatibleDC.Call(hdcScreen)
	sel.bgBitmap, _, _ = pCreateCompatibleBitmap.Call(hdcScreen, width, height)
	pSelectObject.Call(sel.bgDC, sel.bgBitmap)

	const srccopy = 0x00CC0020
	pBitBlt.Call(sel.bgDC, 0, 0, width, height, hdcScreen, uintptr(vRect.Min.X), uintptr(vRect.Min.Y), srccopy)

	sel.darkBrush, _, _ = pCreateSolidBrush.Call(colorOverlay)
	sel.magBrush, _, _ = pCreateSolidBrush.Call(colorMagenta)
	sel.borderPen, _, _ = pCreatePen.Call(psSolid, 2, uintptr(sel.selectionClr))
	sel.cursor, _, _ = pLoadCursorW.Call(0, idcCross)

	sel.dimmedDC, _, _ = pCreateCompatibleDC.Call(hdcScreen)
	sel.dimmedBitmap, _, _ = pCreateCompatibleBitmap.Call(hdcScreen, width, height)
	pSelectObject.Call(sel.dimmedDC, sel.dimmedBitmap)

	pBitBlt.Call(sel.dimmedDC, 0, 0, width, height, sel.bgDC, 0, 0, srccopy)

	dimDC, _, _ := pCreateCompatibleDC.Call(hdcScreen)
	dimBitmap, _, _ := pCreateCompatibleBitmap.Call(hdcScreen, width, height)
	pSelectObject.Call(dimDC, dimBitmap)
	pFillRect.Call(dimDC, uintptr(unsafe.Pointer(&wRECT{0, 0, int32(width), int32(height)})), sel.darkBrush)

	blendVal := uint32(acSrcOver) | (uint32(0) << 8) | (uint32(sel.overlayAlpha) << 16) | (uint32(0) << 24)
	pAlphaBlend.Call(
		sel.dimmedDC, 0, 0, width, height,
		dimDC, 0, 0, width, height,
		uintptr(blendVal),
	)

	pDeleteObject.Call(dimBitmap)
	pDeleteDC.Call(dimDC)

	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := syscall.StringToUTF16Ptr("ScreenSelectorOverlay")
	cb := syscall.NewCallback(selectorWndProc)

	var wcex wWNDCLASSEXW
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))
	wcex.Style = csHRedraw | csVRedraw
	wcex.LpfnWndProc = cb
	wcex.HInstance = hInst
	wcex.LpszClassName = className

	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	hwnd, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExLayered|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		0,
		wsPopup,
		uintptr(vRect.Min.X), uintptr(vRect.Min.Y), width, height,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return image.Rectangle{}, false
	}
	sel.hwnd = hwnd

	pSetLayeredWindowAttributes.Call(hwnd, 0, overlayAlpha, lwaAlpha)

	pShowWindow.Call(hwnd, swShow)
	pSetForegroundWindow.Call(hwnd)

	var msg wMSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || int32(ret) == -1 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if sel.cancelled {
		return image.Rectangle{}, false
	}

	x1 := int(minI32(sel.startX, sel.endX))
	y1 := int(minI32(sel.startY, sel.endY))
	x2 := int(maxI32(sel.startX, sel.endX))
	y2 := int(maxI32(sel.startY, sel.endY))

	if x2-x1 < 10 || y2-y1 < 10 {
		return image.Rectangle{}, false
	}

	return image.Rect(x1+vRect.Min.X, y1+vRect.Min.Y, x2+vRect.Min.X, y2+vRect.Min.Y), true
}
