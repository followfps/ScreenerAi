//go:build windows

package main

import (
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	pSetTextColor = gdi32w.NewProc("SetTextColor")
	pSetBkMode    = gdi32w.NewProc("SetBkMode")
	pDrawTextW    = user32w.NewProc("DrawTextW")
)

var notificationClassRegistered bool
var notificationMessage string

func showNotification(msg string) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := pGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("NotificationWindow")

	if !notificationClassRegistered {
		wcex := wWNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(wWNDCLASSEXW{})),
			Style:         csHRedraw | csVRedraw,
			LpfnWndProc:   syscall.NewCallback(notificationWndProc),
			CbClsExtra:    0,
			CbWndExtra:    0,
			HInstance:     hInstance,
			HCursor:       0,
			HbrBackground: 0,
			LpszMenuName:  nil,
			LpszClassName: className,
		}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))
		notificationClassRegistered = true
	}

	screenWidth, _, _ := pGetSystemMetrics.Call(smCxScreen)
	screenHeight, _, _ := pGetSystemMetrics.Call(smCyScreen)

	winWidth := int32(600)
	winHeight := int32(60)

	x := (int32(screenWidth) - winWidth) / 2
	y := (int32(screenHeight) / 2) + 150

	windowName, _ := syscall.UTF16PtrFromString("ScreanerAiNotification")
	hwnd, _, _ := pCreateWindowExW.Call(
		wsExTopmost|wsExLayered|wsExToolWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		wsPopup,
		uintptr(x), uintptr(y),
		uintptr(winWidth), uintptr(winHeight),
		0, 0, hInstance, 0,
	)

	if hwnd == 0 {
		return
	}

	pSetLayeredWindowAttributes.Call(hwnd, 0, 220, lwaAlpha)

	notificationMessage = msg
	pShowWindow.Call(hwnd, swShow)

	start := time.Now()
	for time.Since(start) < 4*time.Second {
		var m wMSG
		pPeekMessage := user32w.NewProc("PeekMessageW")
		ret, _, _ := pPeekMessage.Call(uintptr(unsafe.Pointer(&m)), hwnd, 0, 0, 1)
		if ret != 0 {
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		time.Sleep(50 * time.Millisecond)
	}

	pDestroyWindow.Call(hwnd)
}

func notificationWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps wPAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

		var rc wRECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))

		brush, _, _ := pCreateSolidBrush.Call(0x333333)
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), brush)
		pDeleteObject.Call(brush)

		pSetTextColor.Call(hdc, 0xFFFFFF)
		pSetBkMode.Call(hdc, 1)

		textPtr, _ := syscall.UTF16PtrFromString(notificationMessage)
		pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(textPtr)), uintptr(len(notificationMessage)), uintptr(unsafe.Pointer(&rc)), 0x25)

		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmDestroy:
		return 0
	default:
		ret, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}
}
