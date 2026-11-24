//go:build windows

package main

import (
	"sync"
	"syscall"
	"unsafe"
)

type eyedropperState struct {
	overlayHwnd uintptr
	tipHwnd     uintptr
	snapDC      uintptr
	vx, vy      int32
	vw, vh      int32
	curColor    uint32
	curHex      string
	label       string
	done        bool
	ok          bool
}

var (
	dropperRegOnce sync.Once
	activeDropper  *eyedropperState
	dropperFont    uintptr
)

func (a *appState) pickScreenColor(label string) (uint32, bool) {
	if pGetDC.Find() != nil || pGetPixel.Find() != nil {
		return 0, false
	}

	dropperRegOnce.Do(func() {
		crossCur, _, _ := pLoadCursorW.Call(0, IDC_CROSS)
		wcOverlay := WNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
			LpfnWndProc:   syscall.NewCallback(dropperOverlayProc),
			HInstance:     a.hInst,
			HCursor:       crossCur,
			LpszClassName: u16("GrowthBuddyFishDropper"),
		}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcOverlay)))

		arrowCur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
		wcTip := WNDCLASSEXW{
			CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
			LpfnWndProc:   syscall.NewCallback(dropperTipProc),
			HInstance:     a.hInst,
			HCursor:       arrowCur,
			LpszClassName: u16("GrowthBuddyFishDropperTip"),
		}
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcTip)))
		dropperFont = createUIFont(a.dpiScale(13), false, CLEARTYPE_QUALITY)
	})

	vxM, _, _ := pGetSystemMetrics.Call(SM_XVIRTUALSCREEN)
	vyM, _, _ := pGetSystemMetrics.Call(SM_YVIRTUALSCREEN)
	vwM, _, _ := pGetSystemMetrics.Call(SM_CXVIRTUALSCREEN)
	vhM, _, _ := pGetSystemMetrics.Call(SM_CYVIRTUALSCREEN)
	vx, vy := int32(vxM), int32(vyM)
	vw, vh := int32(vwM), int32(vhM)
	if vw <= 0 || vh <= 0 {
		w0, _, _ := pGetSystemMetrics.Call(SM_CXSCREEN)
		h0, _, _ := pGetSystemMetrics.Call(SM_CYSCREEN)
		vx, vy, vw, vh = 0, 0, int32(w0), int32(h0)
	}

	scrDC, _, _ := pGetDC.Call(0)
	if scrDC == 0 {
		return 0, false
	}
	snapDC, _, _ := pCreateCompatibleDC.Call(scrDC)
	snapBmp, _, _ := pCreateCompatibleBitmap.Call(scrDC, uintptr(vw), uintptr(vh))
	oldBmp, _, _ := pSelectObject.Call(snapDC, snapBmp)
	pBitBlt.Call(snapDC, 0, 0, uintptr(vw), uintptr(vh), scrDC, uintptr(vx), uintptr(vy), SRCCOPY|CAPTUREBLT)
	pReleaseDC.Call(0, scrDC)
	defer func() {
		if snapDC != 0 {
			pSelectObject.Call(snapDC, oldBmp)
			pDeleteDC.Call(snapDC)
		}
		if snapBmp != 0 {
			pDeleteObject.Call(snapBmp)
		}
	}()

	st := &eyedropperState{
		snapDC:   snapDC,
		vx:       vx,
		vy:       vy,
		vw:       vw,
		vh:       vh,
		curColor: 0x00FFFFFF,
		curHex:   "#FFFFFF",
		label:    label,
	}
	activeDropper = st
	defer func() {
		activeDropper = nil
	}()

	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	st.sampleAt(pt.X, pt.Y)

	tipW := a.dpiScale(188)
	tipH := a.dpiScale(48)
	tx, ty := st.calcTipPos(pt.X, pt.Y, tipW, tipH)
	st.tipHwnd, _, _ = pCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE|WS_EX_TRANSPARENT,
		uintptr(unsafe.Pointer(u16("GrowthBuddyFishDropperTip"))),
		uintptr(unsafe.Pointer(u16("DropperTip"))),
		WS_POPUP,
		uintptr(tx), uintptr(ty), uintptr(tipW), uintptr(tipH),
		0, 0, a.hInst, 0,
	)

	st.overlayHwnd, _, _ = pCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_LAYERED,
		uintptr(unsafe.Pointer(u16("GrowthBuddyFishDropper"))),
		uintptr(unsafe.Pointer(u16("DropperOverlay"))),
		WS_POPUP,
		uintptr(vx), uintptr(vy), uintptr(vw), uintptr(vh),
		0, 0, a.hInst, 0,
	)
	if st.overlayHwnd == 0 {
		if st.tipHwnd != 0 {
			pDestroyWindow.Call(st.tipHwnd)
		}
		return 0, false
	}

	// Minimal non-zero alpha captures mouse clicks cleanly across the full virtual desktop
	pSetLayeredWindowAttrs.Call(st.overlayHwnd, 0, 1, LWA_ALPHA)
	pShowWindow.Call(st.overlayHwnd, SW_SHOW)
	if st.tipHwnd != 0 {
		pShowWindow.Call(st.tipHwnd, SW_SHOW)
	}
	pSetForegroundWindow.Call(st.overlayHwnd)
	pSetCapture.Call(st.overlayHwnd)
	crossCur, _, _ := pLoadCursorW.Call(0, IDC_CROSS)
	if crossCur != 0 {
		pSetCursor.Call(crossCur)
	}

	var msg MSG
	for !st.done {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	pReleaseCapture.Call()
	if st.tipHwnd != 0 {
		pDestroyWindow.Call(st.tipHwnd)
	}
	pDestroyWindow.Call(st.overlayHwnd)
	return st.curColor, st.ok
}

func (st *eyedropperState) sampleAt(sx, sy int32) {
	px := sx - st.vx
	py := sy - st.vy
	if px < 0 || py < 0 || px >= st.vw || py >= st.vh {
		return
	}
	c, _, _ := pGetPixel.Call(st.snapDC, uintptr(px), uintptr(py))
	if uint32(c) != 0xFFFFFFFF {
		st.curColor = uint32(c) & 0x00FFFFFF
		st.curHex = colorrefToHex(st.curColor)
	}
}

func (st *eyedropperState) calcTipPos(sx, sy, tipW, tipH int32) (int32, int32) {
	off := int32(18)
	tx := sx + off
	ty := sy + off
	if tx+tipW > st.vx+st.vw-8 {
		tx = sx - tipW - off
	}
	if ty+tipH > st.vy+st.vh-8 {
		ty = sy - tipH - off
	}
	return tx, ty
}

func dropperOverlayProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	st := activeDropper
	if st == nil {
		r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return r
	}
	switch msg {
	case WM_MOUSEMOVE:
		var pt POINT
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		st.sampleAt(pt.X, pt.Y)
		if st.tipHwnd != 0 {
			var tr RECT
			pGetWindowRect.Call(st.tipHwnd, uintptr(unsafe.Pointer(&tr)))
			tw := tr.Right - tr.Left
			th := tr.Bottom - tr.Top
			tx, ty := st.calcTipPos(pt.X, pt.Y, tw, th)
			pSetWindowPos.Call(st.tipHwnd, 0, uintptr(tx), uintptr(ty), 0, 0, SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE)
			pInvalidateRect.Call(st.tipHwnd, 0, 0)
		}
		return 0
	case WM_LBUTTONDOWN:
		var pt POINT
		pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		st.sampleAt(pt.X, pt.Y)
		st.ok = true
		st.done = true
		return 0
	case WM_RBUTTONDOWN:
		st.done = true
		return 0
	case WM_KEYDOWN:
		if wParam == VK_ESCAPE {
			st.done = true
			return 0
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func dropperTipProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	st := activeDropper
	switch msg {
	case WM_ERASEBKGND:
		return 1
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc == 0 {
			return 0
		}
		defer pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if st == nil {
			return 0
		}
		var rc RECT
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))

		bgBr, _, _ := pCreateSolidBrush.Call(0x0024201E)
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), bgBr)
		pDeleteObject.Call(bgBr)

		borderBr, _, _ := pCreateSolidBrush.Call(0x00888888)
		pFrameRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), borderBr)
		pDeleteObject.Call(borderBr)

		pad := int32(8)
		swSize := (rc.Bottom - rc.Top) - pad*2
		swRc := RECT{Left: pad, Top: pad, Right: pad + swSize, Bottom: pad + swSize}
		swBr, _, _ := pCreateSolidBrush.Call(uintptr(st.curColor))
		pFillRect.Call(hdc, uintptr(unsafe.Pointer(&swRc)), swBr)
		pDeleteObject.Call(swBr)

		swFr, _, _ := pCreateSolidBrush.Call(0x00FFFFFF)
		pFrameRect.Call(hdc, uintptr(unsafe.Pointer(&swRc)), swFr)
		pDeleteObject.Call(swFr)

		if dropperFont != 0 {
			oldF, _, _ := pSelectObject.Call(hdc, dropperFont)
			defer pSelectObject.Call(hdc, oldF)
		}
		pSetBkMode.Call(hdc, TRANSPARENT)
		pSetTextColor.Call(hdc, 0x00FFFFFF)
		txtRc := RECT{Left: swRc.Right + 8, Top: 5, Right: rc.Right - 6, Bottom: rc.Bottom - 4}
		txt := st.label + " " + st.curHex + "\n左键取色 · 右键/Esc取消"
		pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(u16(txt))), ^uintptr(0), uintptr(unsafe.Pointer(&txtRc)), DT_LEFT|DT_WORDBREAK|DT_NOPREFIX)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}
