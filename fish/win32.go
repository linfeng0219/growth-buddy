//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	pSetProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	pRegisterClassExW       = user32.NewProc("RegisterClassExW")
	pCreateWindowExW        = user32.NewProc("CreateWindowExW")
	pDefWindowProcW         = user32.NewProc("DefWindowProcW")
	pDestroyWindow          = user32.NewProc("DestroyWindow")
	pShowWindow             = user32.NewProc("ShowWindow")
	pUpdateWindow           = user32.NewProc("UpdateWindow")
	pIsWindowVisible        = user32.NewProc("IsWindowVisible")
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
	pSetLayeredWindowAttrs  = user32.NewProc("SetLayeredWindowAttributes")
	pSetWindowTextW         = user32.NewProc("SetWindowTextW")
	pGetWindowTextW         = user32.NewProc("GetWindowTextW")
	pGetWindowTextLengthW   = user32.NewProc("GetWindowTextLengthW")
	pSetFocus               = user32.NewProc("SetFocus")
	pSetTimer               = user32.NewProc("SetTimer")
	pKillTimer              = user32.NewProc("KillTimer")
	pLoadCursorW            = user32.NewProc("LoadCursorW")
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
	pSelectObject           = gdi32.NewProc("SelectObject")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkColor             = gdi32.NewProc("SetBkColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pChooseColorW           = comdlg32.NewProc("ChooseColorW")
)

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_TABSTOP          = 0x00010000
	WS_BORDER           = 0x00800000
	WS_VSCROLL          = 0x00200000
	WS_CLIPCHILDREN     = 0x02000000
	WS_EX_LAYERED       = 0x00080000

	ES_LEFT         = 0x0000
	ES_AUTOHSCROLL  = 0x0080
	ES_AUTOVSCROLL  = 0x0040
	ES_MULTILINE    = 0x0004
	ES_PASSWORD     = 0x0020
	ES_NUMBER       = 0x2000
	BS_PUSHBUTTON   = 0x0000
	BS_AUTOCHECKBOX = 0x0003

	LWA_COLORKEY = 0x00000001
	LWA_ALPHA    = 0x00000002

	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_MOVE           = 0x0003
	WM_SIZE           = 0x0005
	WM_SETFONT        = 0x0030
	WM_PAINT          = 0x000F
	WM_CLOSE          = 0x0010
	WM_ERASEBKGND     = 0x0014
	WM_GETMINMAXINFO  = 0x0024
	WM_KEYDOWN        = 0x0100
	WM_CHAR           = 0x0102
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_VSCROLL        = 0x0115
	WM_CTLCOLOREDIT   = 0x0133
	WM_CTLCOLORBTN    = 0x0135
	WM_CTLCOLORSTATIC = 0x0138
	WM_MOUSEWHEEL     = 0x020A
	WM_HOTKEY         = 0x0312
	WM_APP            = 0x8000

	MOD_NOREPEAT = 0x4000

	EM_SETSEL   = 0x00B1
	BM_GETCHECK = 0x00F0
	BM_SETCHECK = 0x00F1

	SW_SHOW        = 5
	SW_HIDE        = 0
	SWP_NOMOVE     = 0x0002
	SWP_NOZORDER   = 0x0004
	SWP_NOACTIVATE = 0x0010

	DT_LEFT        = 0x00000000
	DT_WORDBREAK   = 0x00000010
	DT_CALCRECT    = 0x00000400
	DT_NOPREFIX    = 0x00000800
	DT_EDITCONTROL = 0x00002000

	SRCCOPY     = 0x00CC0020
	TRANSPARENT = 1
	OPAQUE      = 2

	VK_RETURN = 0x0D

	IDC_ARROW              = 32512
	DEFAULT_CHARSET        = 1
	FW_NORMAL              = 400
	FW_BOLD                = 700
	NONANTIALIASED_QUALITY = 3
	CLEARTYPE_QUALITY      = 5

	CC_RGBINIT  = 0x00000001
	CC_FULLOPEN = 0x00000002

	GWLP_WNDPROC = ^uintptr(3) // -4 as uintptr
	SM_CXSCREEN  = 0
	SM_CYSCREEN  = 1
)

type POINT struct {
	X, Y int32
}

type RECT struct {
	Left, Top, Right, Bottom int32
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

type CHOOSECOLORW struct {
	LStructSize    uint32
	HwndOwner      uintptr
	HInstance      uintptr
	RgbResult      uint32
	LpCustColors   *uint32
	Flags          uint32
	LCustData      uintptr
	LpfnHook       uintptr
	LpTemplateName *uint16
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

func pickColorDialog(owner uintptr, initial uint32, cust *[16]uint32) (uint32, bool) {
	cc := CHOOSECOLORW{
		LStructSize:  uint32(unsafe.Sizeof(CHOOSECOLORW{})),
		HwndOwner:    owner,
		RgbResult:    initial,
		LpCustColors: &cust[0],
		Flags:        CC_RGBINIT | CC_FULLOPEN,
	}
	r, _, _ := pChooseColorW.Call(uintptr(unsafe.Pointer(&cc)))
	return cc.RgbResult, r != 0
}
