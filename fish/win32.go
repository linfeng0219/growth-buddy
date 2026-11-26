//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
	pGlobalLock             = kernel32.NewProc("GlobalLock")
	pGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
	pSetProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	pOpenClipboard          = user32.NewProc("OpenClipboard")
	pEmptyClipboard         = user32.NewProc("EmptyClipboard")
	pSetClipboardData       = user32.NewProc("SetClipboardData")
	pCloseClipboard         = user32.NewProc("CloseClipboard")
	pGetKeyState            = user32.NewProc("GetKeyState")
	pRegisterClassExW       = user32.NewProc("RegisterClassExW")
	pCreateWindowExW        = user32.NewProc("CreateWindowExW")
	pDefWindowProcW         = user32.NewProc("DefWindowProcW")
	pDestroyWindow          = user32.NewProc("DestroyWindow")
	pShowWindow             = user32.NewProc("ShowWindow")
	pUpdateWindow           = user32.NewProc("UpdateWindow")
	pIsWindowVisible        = user32.NewProc("IsWindowVisible")
	pGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	pSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	pFindWindowW            = user32.NewProc("FindWindowW")
	pRegisterHotKey         = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey       = user32.NewProc("UnregisterHotKey")
	pGetMessageW            = user32.NewProc("GetMessageW")
	pIsDialogMessageW       = user32.NewProc("IsDialogMessageW")
	pTranslateMessage       = user32.NewProc("TranslateMessage")
	pDispatchMessageW       = user32.NewProc("DispatchMessageW")
	pPostQuitMessage        = user32.NewProc("PostQuitMessage")
	pPostMessageW           = user32.NewProc("PostMessageW")
	pSendMessageW           = user32.NewProc("SendMessageW")
	pGetClientRect          = user32.NewProc("GetClientRect")
	pGetWindowRect          = user32.NewProc("GetWindowRect")
	pSetWindowPos           = user32.NewProc("SetWindowPos")
	pMoveWindow             = user32.NewProc("MoveWindow")
	pInvalidateRect         = user32.NewProc("InvalidateRect")
	pBeginPaint             = user32.NewProc("BeginPaint")
	pEndPaint               = user32.NewProc("EndPaint")
	pFillRect               = user32.NewProc("FillRect")
	pFrameRect              = user32.NewProc("FrameRect")
	pSetLayeredWindowAttrs  = user32.NewProc("SetLayeredWindowAttributes")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	pDestroyMenu            = user32.NewProc("DestroyMenu")
	pGetCursorPos           = user32.NewProc("GetCursorPos")
	pScreenToClient         = user32.NewProc("ScreenToClient")
	pCreateIconIndirect     = user32.NewProc("CreateIconIndirect")
	pDestroyIcon            = user32.NewProc("DestroyIcon")
	pShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	pSetWindowTextW         = user32.NewProc("SetWindowTextW")
	pGetWindowTextW         = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	pSetFocus               = user32.NewProc("SetFocus")
	pSetTimer               = user32.NewProc("SetTimer")
	pKillTimer              = user32.NewProc("KillTimer")
	pLoadCursorW            = user32.NewProc("LoadCursorW")
	pSetCursor              = user32.NewProc("SetCursor")
	pSetCapture             = user32.NewProc("SetCapture")
	pReleaseCapture         = user32.NewProc("ReleaseCapture")
	pGetDC                  = user32.NewProc("GetDC")
	pReleaseDC              = user32.NewProc("ReleaseDC")
	pMessageBoxW            = user32.NewProc("MessageBoxW")
	pSetWindowLongPtrW      = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW        = user32.NewProc("CallWindowProcW")
	pDrawTextW              = user32.NewProc("DrawTextW")
	pGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC               = gdi32.NewProc("DeleteDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pCreateBitmap           = gdi32.NewProc("CreateBitmap")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pGetDIBits              = gdi32.NewProc("GetDIBits")
	pGetPixel               = gdi32.NewProc("GetPixel")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkColor             = gdi32.NewProc("SetBkColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pGetTextExtentExPointW  = gdi32.NewProc("GetTextExtentExPointW")
	pExtTextOutW            = gdi32.NewProc("ExtTextOutW")
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_POPUP            = 0x80000000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_TABSTOP          = 0x00010000
	WS_BORDER           = 0x00800000
	WS_VSCROLL          = 0x00200000
	WS_CLIPCHILDREN     = 0x02000000
	WS_EX_TRANSPARENT   = 0x00000020
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_EX_TOPMOST       = 0x00000008
	WS_EX_LAYERED       = 0x00080000
	WS_EX_NOACTIVATE    = 0x08000000

	ES_LEFT         = 0x0000
	ES_AUTOHSCROLL  = 0x0080
	ES_AUTOVSCROLL  = 0x0040
	ES_MULTILINE    = 0x0004
	ES_PASSWORD     = 0x0020
	ES_NUMBER       = 0x2000
	BS_PUSHBUTTON   = 0x0000
	BS_AUTOCHECKBOX = 0x0003
	BS_OWNERDRAW    = 0x000B

	LWA_COLORKEY = 0x00000001
	LWA_ALPHA    = 0x00000002

	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_MOVE           = 0x0003
	WM_SIZE           = 0x0005
	WM_ACTIVATE       = 0x0006
	WM_SETFONT        = 0x0030
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_GETMINMAXINFO  = 0x0024
	WM_DRAWITEM       = 0x002B
	WM_KEYDOWN        = 0x0100
	WM_CHAR           = 0x0102
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_VSCROLL        = 0x0115
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORBTN    = 0x0135
	WM_CTLCOLORSTATIC = 0x0138
	WM_MOUSEWHEEL     = 0x020A
	WM_NCHITTEST      = 0x0084
	WM_SETCURSOR      = 0x0020
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_LBUTTONDBLCLK  = 0x0203
	WM_RBUTTONDOWN    = 0x0204
	WM_RBUTTONUP      = 0x0205
	WM_HOTKEY         = 0x0312
	WM_APP            = 0x8000

	HTCAPTION     = 2
	HTLEFT        = 10
	HTRIGHT       = 11
	HTTOP         = 12
	HTTOPLEFT     = 13
	HTTOPRIGHT    = 14
	HTBOTTOM      = 15
	HTBOTTOMLEFT  = 16
	HTBOTTOMRIGHT = 17

	NIM_ADD     = 0x00000000
	NIM_MODIFY  = 0x00000001
	NIM_DELETE  = 0x00000002
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	MF_STRING       = 0x00000000
	MF_CHECKED      = 0x00000008
	MF_POPUP        = 0x00000010
	MF_SEPARATOR    = 0x00000800
	TPM_LEFTALIGN   = 0x0000
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	MOD_NOREPEAT = 0x4000

	EM_SETSEL   = 0x00B1
	BM_GETCHECK = 0x00F0
	BM_SETCHECK = 0x00F1

	SW_SHOW        = 5
	SW_HIDE        = 0
	SWP_NOSIZE     = 0x0001
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	HWND_TOPMOST   = ^uintptr(0)
	HWND_NOTOPMOST = ^uintptr(1)

	DT_LEFT        = 0x00000000
	DT_CENTER      = 0x00000001
	DT_VCENTER     = 0x00000004
	DT_WORDBREAK   = 0x00000010
	DT_SINGLELINE  = 0x00000020
	DT_CALCRECT    = 0x00000400
	DT_NOPREFIX    = 0x00000800
	DT_EDITCONTROL = 0x00002000

	MB_OK              = 0x00000000
	MB_OKCANCEL        = 0x00000001
	MB_YESNO           = 0x00000004
	MB_ICONERROR       = 0x00000010
	MB_ICONINFORMATION = 0x00000040
	IDOK               = 1
	IDYES              = 6
	ODS_SELECTED       = 0x0001

	SRCCOPY     = 0x00CC0020
	CAPTUREBLT  = 0x40000000
	TRANSPARENT = 1
	OPAQUE      = 2

	VK_RETURN  = 0x0D
	VK_CONTROL = 0x11
	VK_ESCAPE  = 0x1B
	VK_PRIOR   = 0x21
	VK_NEXT    = 0x22
	VK_END     = 0x23
	VK_HOME    = 0x24
	VK_UP      = 0x26
	VK_DOWN    = 0x28

	CS_DBLCLKS = 0x0008

	IDC_ARROW = 32512
	IDC_IBEAM = 32513
	IDC_CROSS = 32515

	ETO_OPAQUE             = 0x0002
	ETO_CLIPPED            = 0x0004
	CF_UNICODETEXT         = 13
	GMEM_MOVEABLE          = 0x0002
	DEFAULT_CHARSET        = 1
	FW_NORMAL              = 400
	FW_BOLD                = 700
	NONANTIALIASED_QUALITY = 3
	CLEARTYPE_QUALITY      = 5

	GWLP_WNDPROC       = ^uintptr(3) // -4 as uintptr
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
)

type POINT struct {
	X, Y int32
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type SIZE struct {
	Cx, Cy int32
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type WNDCLASSEXW struct {
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

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type MINMAXINFO struct {
	PtReserved     POINT
	PtMaxSize      POINT
	PtMaxPosition  POINT
	PtMinTrackSize POINT
	PtMaxTrackSize POINT
}

type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     RECT
	ItemData   uintptr
}

type NOTIFYICONDATAW struct {
	CbSize           uint32
	Hwnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type ICONINFO struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

func createFishTrayIcon(unread bool) uintptr {
	const sz = 16
	pixels := make([]uint32, sz*sz)
	// Cute teal-green badge (#2F6B4F) with a crisp white fish silhouette (BGRA byte order in uint32 little-endian: 0xAARRGGBB)
	const bg uint32 = 0xFF2F6B4F
	const fg uint32 = 0xFFFFFFFF
	const eye uint32 = 0xFF2F6B4F
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx := float64(x) - 7.5
			dy := float64(y) - 7.5
			if dx*dx+dy*dy <= 7.5*7.5 {
				pixels[y*sz+x] = bg
			}
		}
	}
	// Fish body ellipse centered at (6.5, 7.5) + tail at right (x=11..13)
	for y := 4; y <= 11; y++ {
		for x := 2; x <= 13; x++ {
			dx := (float64(x) - 6.5) / 4.0
			dy := (float64(y) - 7.5) / 3.0
			inBody := dx*dx+dy*dy <= 1.0
			inTail := x >= 11 && x <= 13 && (y >= 7-(x-10) && y <= 8+(x-10))
			if inBody || inTail {
				pixels[y*sz+x] = fg
			}
		}
	}
	pixels[7*sz+4] = eye
	if unread {
		// Unread notification red dot at top-right corner with crisp 1px white halo
		const dotHalo uint32 = 0xFFFFFFFF
		const dotRed uint32 = 0xFFFF3B30
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				dx := float64(x) - 12.2
				dy := float64(y) - 3.6
				d2 := dx*dx + dy*dy
				if d2 <= 3.1*3.1 {
					pixels[y*sz+x] = dotRed
				} else if d2 <= 4.1*4.1 {
					pixels[y*sz+x] = dotHalo
				}
			}
		}
	}
	mask := make([]byte, (sz/8)*sz)
	hColor, _, _ := pCreateBitmap.Call(sz, sz, 1, 32, uintptr(unsafe.Pointer(&pixels[0])))
	hMask, _, _ := pCreateBitmap.Call(sz, sz, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	if hColor == 0 || hMask == 0 {
		if hColor != 0 {
			pDeleteObject.Call(hColor)
		}
		if hMask != 0 {
			pDeleteObject.Call(hMask)
		}
		return 0
	}
	ii := ICONINFO{FIcon: 1, HbmMask: hMask, HbmColor: hColor}
	icon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(hColor)
	pDeleteObject.Call(hMask)
	return icon
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func getModuleHandle() uintptr {
	r, _, _ := pGetModuleHandleW.Call(0)
	return r
}

func createCtrl(className, text string, style, exStyle uint32, x, y, w, h int32, parent, id, hInst uintptr) uintptr {
	hwnd, _, _ := pCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(u16(className))),
		uintptr(unsafe.Pointer(u16(text))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, id, hInst, 0,
	)
	return hwnd
}

func setCtrlText(hwnd uintptr, s string) {
	pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(u16(s))))
}

func getCtrlText(hwnd uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func showCtrl(hwnd uintptr, visible bool) {
	if hwnd == 0 {
		return
	}
	if visible {
		pShowWindow.Call(hwnd, SW_SHOW)
	} else {
		pShowWindow.Call(hwnd, SW_HIDE)
	}
}

func moveCtrl(hwnd uintptr, x, y, w, h int32) {
	if hwnd != 0 {
		pMoveWindow.Call(hwnd, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
	}
}

func setFont(hwnd, hFont uintptr) {
	if hwnd != 0 && hFont != 0 {
		pSendMessageW.Call(hwnd, WM_SETFONT, hFont, 1)
	}
}

func createUIFont(height int32, bold bool, quality uint32) uintptr {
	weight := int32(FW_NORMAL)
	if bold {
		weight = FW_BOLD
	}
	f, _, _ := pCreateFontW.Call(
		uintptr(uint32(-height)), 0, 0, 0,
		uintptr(weight), 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, uintptr(quality), 0,
		uintptr(unsafe.Pointer(u16("Microsoft YaHei UI"))),
	)
	return f
}
