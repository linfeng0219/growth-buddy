//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmAppRefresh         = WM_APP + 1
	wmAppShow            = WM_APP + 2
	wmAppTray            = WM_APP + 3
	wmAppUpdateResult    = WM_APP + 4
	wmAppChangelogResult = WM_APP + 5

	idTimerPoll     = 1
	idTimerContrast = 2
	idHotkeyBoss    = 1
	idHotkeyClear   = 2
	idTrayIcon      = 1

	idBtnLogin = 101
	idEditMsg  = 129
	idEditPass = 130
	idBtnQuit  = 136

	idMenuToggle            = 201
	idMenuPickFontColor     = 202
	idMenuToggleInvert      = 217
	idMenuFont12            = 203
	idMenuFont14            = 204
	idMenuFont15            = 205
	idMenuFont18            = 206
	idMenuFont20            = 207
	idMenuToggleTransparent = 208
	idMenuPickBgColor       = 209
	idMenuHKAltH            = 210
	idMenuHKAltQ            = 211
	idMenuHKCtrlAltH        = 212
	idMenuHKF9              = 213
	idMenuCheckUpdate       = 214
	idMenuLogout            = 215
	idMenuQuit              = 216
	idMenuLimit100          = 218
	idMenuLimit500          = 219
	idMenuLimitAll          = 220
	idMenuAddAccount        = 221
	idMenuClearNow          = 222
	idMenuClearHKAltL       = 223
	idMenuClearHKAltC       = 224
	idMenuClearHKCtrlAltL   = 225
	idMenuClearHKF8         = 226
	idMenuUnclearHistory    = 227
	idMenuChangelog         = 228
	idMenuAccountBase       = 300
	maxMenuSavedAccounts    = 15
	idCtxCopy               = 401
	idCtxSelectAll          = 402
)

var pGetDpiForWindow = user32.NewProc("GetDpiForWindow")

type contrastSample struct {
	bg uint32
	fg uint32
}

type appState struct {
	mu              sync.Mutex
	hwnd            uintptr
	hInst           uintptr
	hIcon           uintptr
	hIconUnread     uintptr
	trayAdded       bool
	trayUnreadCount int
	unreadCount     int
	initialSynced   bool
	cfg             FishConfig
	loggedIn        bool
	loggingIn       bool
	polling         bool
	sending         bool
	loginStatus     string
	messages        []ChatMsg
	seenIDs         map[string]bool
	lastMsgTs       int64
	scrollY         int32
	maxScrollY      int32
	atBottom        bool
	visLines        []visualLine
	selecting       bool
	draggingScroll  bool
	selAnchor       textPos
	selFocus        textPos
	hookTransparent int32 // atomic: 1 when loggedIn && BgTransparent
	hookOverflow    int32 // atomic: 1 when loggedIn && maxScrollY > 0
	hookLoggedIn    int32 // atomic: 1 when loggedIn
	hMouseHook      uintptr
	mouseHookCbPtr  uintptr

	uiFont   uintptr
	boldFont uintptr
	chatFont uintptr
	tsFont   uintptr
	bgBrush  uintptr

	// Login controls
	hLblTitle, hBtnLoginQuit, hLblUser, hEditUser, hLblPass, hEditPass, hBtnLogin, hLblLoginStatus uintptr

	// Chat controls (minimal: no buttons in window, Enter to send)
	hLblGroup, hEditMsg uintptr

	checkingUpdate   bool
	pendingUpdateRes *FishVersionResponse
	pendingUpdateErr string
	pendingUpdateMan bool
	pendingChangelog string

	backdropBuf    []uint32
	backdropW      int32
	backdropH      int32
	backdropSig    uint64
	autoHeaderFg   uint32
	autoHeaderBg   uint32
	hasHdrSample   bool
	inputBgColor   uint32
	inputTextFg    uint32
	hasInputSample bool

	origMsgEditProc  uintptr
	origPassEditProc uintptr
}

var gApp *appState

func (a *appState) dpiScale(px int32) int32 {
	dpi := uint32(96)
	if a.hwnd != 0 && pGetDpiForWindow.Find() == nil {
		if d, _, _ := pGetDpiForWindow.Call(a.hwnd); d >= 96 {
			dpi = uint32(d)
		}
	}
	return int32(int64(px) * int64(dpi) / 96)
}

func (a *appState) effectiveTextColor() uint32 {
	if !a.cfg.AutoInvertText {
		return hexToCOLORREF(a.cfg.FontColor, 0x00283123)
	}
	if !a.cfg.BgTransparent {
		return invertTextColorForBg(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6))
	}
	if a.autoHeaderFg != 0 {
		return a.autoHeaderFg
	}
	return invertTextColorForBg(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6))
}

func (a *appState) refreshResources() {
	if a.uiFont != 0 {
		pDeleteObject.Call(a.uiFont)
	}
	if a.boldFont != 0 {
		pDeleteObject.Call(a.boldFont)
	}
	if a.chatFont != 0 {
		pDeleteObject.Call(a.chatFont)
	}
	if a.tsFont != 0 {
		pDeleteObject.Call(a.tsFont)
	}
	if a.bgBrush != 0 {
		pDeleteObject.Call(a.bgBrush)
	}

	qual := uint32(CLEARTYPE_QUALITY)
	if a.cfg.BgTransparent {
		qual = NONANTIALIASED_QUALITY
	}
	a.uiFont = createUIFont(a.dpiScale(14), false, qual)
	a.boldFont = createUIFont(a.dpiScale(15), true, qual)
	a.chatFont = createUIFont(a.dpiScale(int32(a.cfg.FontSize)), false, qual)
	tsPx := int32(a.cfg.FontSize) - 3
	if tsPx < 11 {
		tsPx = 11
	}
	a.tsFont = createUIFont(a.dpiScale(tsPx), false, qual)

	bgRef := hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
	if a.cfg.BgTransparent {
		a.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(chromaKeyRef))
	} else {
		a.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(bgRef))
		a.inputBgColor = bgRef
	}

	allCtrls := []uintptr{
		a.hLblTitle, a.hBtnLoginQuit, a.hLblUser, a.hEditUser, a.hLblPass, a.hEditPass, a.hBtnLogin, a.hLblLoginStatus,
		a.hLblGroup, a.hEditMsg,
	}
	for _, h := range allCtrls {
		setFont(h, a.uiFont)
	}
	setFont(a.hLblTitle, a.boldFont)
	setFont(a.hLblGroup, a.boldFont)
	setFont(a.hEditMsg, a.chatFont)

	if a.hwnd != 0 {
		flags := uint32(LWA_ALPHA)
		if a.cfg.BgTransparent {
			flags |= LWA_COLORKEY
		}
		pSetLayeredWindowAttrs.Call(a.hwnd, uintptr(chromaKeyRef), 255, uintptr(flags))
	}
}

func (a *appState) sampleUnderWindow(force bool) {
	if a.hwnd == 0 || !a.cfg.BgTransparent {
		return
	}
	vis, _, _ := pIsWindowVisible.Call(a.hwnd)
	if vis == 0 {
		return
	}
	var wr, rc RECT
	pGetWindowRect.Call(hwndPtr(a.hwnd), uintptr(unsafe.Pointer(&wr)))
	pGetClientRect.Call(hwndPtr(a.hwnd), uintptr(unsafe.Pointer(&rc)))
	cw := rc.Right - rc.Left
	ch := rc.Bottom - rc.Top
	if cw <= 0 || ch <= 0 {
		return
	}
	buf, ok := captureScreenRectBGRA(wr.Left, wr.Top, cw, ch)
	if !ok {
		return
	}
	sig := computeBackdropSignature(buf, cw, ch)
	cr := a.chatRect(rc)
	sampleEnd := cr.Bottom
	if sampleEnd <= a.dpiScale(36) {
		sampleEnd = ch
	}
	var rawWinBg uint32
	if winBg, ok := averageBGRAStripColor(buf, cw, ch, 4, sampleEnd); ok {
		rawWinBg = winBg
	} else {
		rawWinBg = hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
	}
	newUnifiedFg, newUnifiedBg := invertTextColorStable(rawWinBg, a.autoHeaderBg, a.autoHeaderFg, a.hasHdrSample)

	bx := a.inputBoxRect(rc)
	bxTop, bxBot := bx.Top+a.dpiScale(3), bx.Bottom-a.dpiScale(3)
	if bxBot <= bxTop {
		bxTop, bxBot = bx.Top, bx.Bottom
	}
	rawBoxBg := a.inputBgColor
	if rawBoxBg == 0 {
		rawBoxBg = hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
	}
	if sampledBoxBg, ok := averageBGRAStripColor(buf, cw, ch, bxTop, bxBot); ok {
		if sampledBoxBg == chromaKeyRef {
			sampledBoxBg = 0x00020202
		}
		rawBoxBg = sampledBoxBg
	}
	prevBoxBg := a.inputBgColor
	wasInputSampled := a.hasInputSample

	hdrChanged := !a.hasHdrSample || newUnifiedFg != a.autoHeaderFg
	boxBorderChanged := !wasInputSampled || colorChannelMaxDiff(rawBoxBg, prevBoxBg) >= 18

	a.backdropBuf = buf
	a.backdropW = cw
	a.backdropH = ch
	a.backdropSig = sig
	a.autoHeaderFg = newUnifiedFg
	a.autoHeaderBg = newUnifiedBg
	a.hasHdrSample = true
	a.inputTextFg = newUnifiedFg
	a.hasInputSample = true
	if boxBorderChanged {
		a.inputBgColor = rawBoxBg
	}

	if hdrChanged && a.cfg.AutoInvertText {
		if a.hEditMsg != 0 {
			pInvalidateRect.Call(a.hEditMsg, 0, 1)
		}
		if !a.loggedIn {
			for _, h := range []uintptr{a.hEditUser, a.hEditPass} {
				if h != 0 {
					pInvalidateRect.Call(h, 0, 1)
				}
			}
		}
		if a.hLblGroup != 0 {
			pInvalidateRect.Call(a.hLblGroup, 0, 1)
		}
		if !a.loggedIn {
			pInvalidateRect.Call(a.hwnd, 0, 1)
		}
	}
	if force || boxBorderChanged || (hdrChanged && a.cfg.AutoInvertText) {
		a.triggerRepaint()
	}
}

func hwndPtr(h uintptr) uintptr { return h }

func (a *appState) createAllControls() {
	h := a.hwnd
	inst := a.hInst

	stLbl := uint32(WS_CHILD)
	stEdit := uint32(WS_CHILD | WS_TABSTOP | WS_BORDER | ES_LEFT | ES_AUTOHSCROLL)
	stOwnerBtn := uint32(WS_CHILD | WS_TABSTOP | BS_OWNERDRAW)

	// Login UI
	a.hLblTitle = createCtrl("STATIC", "🐟 fish", stLbl, 0, 0, 0, 100, 24, h, 0, inst)
	a.hBtnLoginQuit = createCtrl("BUTTON", "×", stOwnerBtn, 0, 0, 0, 26, 24, h, idBtnQuit, inst)
	a.hLblUser = createCtrl("STATIC", "账号", stLbl, 0, 0, 0, 80, 20, h, 0, inst)
	a.hEditUser = createCtrl("EDIT", "", stEdit, 0, 0, 0, 200, 26, h, 0, inst)
	a.hLblPass = createCtrl("STATIC", "密码", stLbl, 0, 0, 0, 80, 20, h, 0, inst)
	a.hEditPass = createCtrl("EDIT", "", stEdit|ES_PASSWORD, 0, 0, 0, 200, 26, h, idEditPass, inst)
	a.hBtnLogin = createCtrl("BUTTON", "进入", stOwnerBtn, 0, 0, 0, 120, 32, h, idBtnLogin, inst)
	a.hLblLoginStatus = createCtrl("STATIC", "", stLbl, 0, 0, 0, 240, 36, h, 0, inst)

	// Chat UI (no Send/Hide/Settings/Logout buttons on page; Enter sends message)
	a.hLblGroup = createCtrl("STATIC", "🐟 家庭群", stLbl, 0, 0, 0, 220, 22, h, 0, inst)
	stMsgEdit := uint32(WS_CHILD | WS_TABSTOP | ES_LEFT | ES_AUTOHSCROLL)
	a.hEditMsg = createCtrl("EDIT", "", stMsgEdit, 0, 0, 0, 200, 28, h, idEditMsg, inst)

	// Subclass Enter key, Ctrl+L clear shortcut, and scroll forwarding on message input
	msgCb := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if msg == WM_MOUSEWHEEL {
			pPostMessageW.Call(gApp.hwnd, uintptr(msg), wParam, lParam)
			return 0
		}
		if msg == WM_KEYDOWN && wParam == VK_RETURN {
			gApp.handleSendMessage()
			return 0
		}
		if msg == WM_KEYDOWN && (wParam == VK_PRIOR || wParam == VK_NEXT || wParam == VK_UP || wParam == VK_DOWN) {
			pPostMessageW.Call(gApp.hwnd, uintptr(msg), wParam, lParam)
			return 0
		}
		if msg == WM_KEYDOWN && (wParam == 'L' || wParam == 'l') {
			if st, _, _ := pGetKeyState.Call(VK_CONTROL); (uint16(st) & 0x8000) != 0 {
				gApp.clearMessagesNow()
				return 0
			}
		}
		if msg == WM_CHAR && wParam == VK_RETURN {
			return 0
		}
		r, _, _ := pCallWindowProcW.Call(gApp.origMsgEditProc, hwnd, uintptr(msg), wParam, lParam)
		return r
	})
	a.origMsgEditProc, _, _ = pSetWindowLongPtrW.Call(a.hEditMsg, GWLP_WNDPROC, msgCb)

	passCb := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if msg == WM_KEYDOWN && wParam == VK_RETURN {
			gApp.handleLoginClick()
			return 0
		}
		if msg == WM_CHAR && wParam == VK_RETURN {
			return 0
		}
		r, _, _ := pCallWindowProcW.Call(gApp.origPassEditProc, hwnd, uintptr(msg), wParam, lParam)
		return r
	})
	a.origPassEditProc, _, _ = pSetWindowLongPtrW.Call(a.hEditPass, GWLP_WNDPROC, passCb)
}

func (a *appState) layoutControls() {
	if a.hwnd == 0 {
		return
	}
	var rc RECT
	pGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc)))
	cw := rc.Right - rc.Left
	s := a.dpiScale
	pad := s(12)

	showLogin := !a.loggedIn
	showChat := a.loggedIn
	a.syncHookAtomicStateLocked()

	for _, h := range []uintptr{a.hLblTitle, a.hBtnLoginQuit, a.hLblUser, a.hEditUser, a.hLblPass, a.hEditPass, a.hBtnLogin, a.hLblLoginStatus} {
		showCtrl(h, showLogin)
	}
	for _, h := range []uintptr{a.hLblGroup, a.hEditMsg} {
		showCtrl(h, showChat)
	}

	if showLogin {
		moveCtrl(a.hBtnLoginQuit, cw-pad-s(26), s(8), s(26), s(24))
		fw := cw - pad*2
		if fw > s(280) {
			fw = s(280)
		}
		lx := (cw - fw) / 2
		y := s(44)
		moveCtrl(a.hLblTitle, lx, y, fw-s(32), s(26))
		y += s(44)
		moveCtrl(a.hLblUser, lx, y, fw, s(18))
		y += s(20)
		moveCtrl(a.hEditUser, lx, y, fw, s(28))
		y += s(36)
		moveCtrl(a.hLblPass, lx, y, fw, s(18))
		y += s(20)
		moveCtrl(a.hEditPass, lx, y, fw, s(28))
		y += s(40)
		moveCtrl(a.hBtnLogin, lx, y, fw, s(32))
		y += s(40)
		moveCtrl(a.hLblLoginStatus, lx, y, fw, s(40))
		return
	}

	moveCtrl(a.hLblGroup, pad, s(10), cw-pad*2, s(22))
	if a.cfg.GroupName != "" {
		setCtrlText(a.hLblGroup, "🐟 "+a.cfg.GroupName)
	}

	bx := a.inputBoxRect(rc)
	insetX := s(6)
	insetY := s(5)
	if bx.Right-bx.Left > insetX*2+10 && bx.Bottom-bx.Top > insetY*2+6 {
		moveCtrl(a.hEditMsg, bx.Left+insetX, bx.Top+insetY, (bx.Right-bx.Left)-insetX*2, (bx.Bottom-bx.Top)-insetY*2)
	}
}

func (a *appState) inputBoxRect(rc RECT) RECT {
	s := a.dpiScale
	pad := s(12)
	inH := s(int32(a.cfg.FontSize) + 15)
	minH := s(30)
	if inH < minH {
		inH = minH
	}
	return RECT{
		Left:   pad,
		Top:    rc.Bottom - pad - inH,
		Right:  rc.Right - pad,
		Bottom: rc.Bottom - pad,
	}
}

func (a *appState) chatRect(rc RECT) RECT {
	s := a.dpiScale
	pad := s(12)
	bx := a.inputBoxRect(rc)
	return RECT{
		Left:   pad,
		Top:    s(42),
		Right:  rc.Right - pad,
		Bottom: bx.Top - s(8),
	}
}

func (a *appState) formatMsgLine(m ChatMsg) string {
	nick := strings.TrimSpace(m.SenderName)
	if nick == "" {
		if m.IsSelf && a.cfg.DisplayName != "" {
			nick = a.cfg.DisplayName
		} else {
			nick = "成员"
		}
	}
	content := strings.TrimSpace(m.Text)
	if m.MsgType == "image" {
		content = "[图片]"
	} else if m.MsgType == "voice" {
		content = "[语音]"
	}
	return nick + "：" + content
}

func (a *appState) paintWindow() {
	var ps PAINTSTRUCT
	hdc, _, _ := pBeginPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer pEndPaint.Call(a.hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc RECT
	pGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc)))
	cw := rc.Right - rc.Left
	ch := rc.Bottom - rc.Top
	if cw <= 0 || ch <= 0 {
		return
	}

	memDC, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(cw), uintptr(ch))
	oldBmp, _, _ := pSelectObject.Call(memDC, bmp)

	pFillRect.Call(memDC, uintptr(unsafe.Pointer(&rc)), a.bgBrush)

	a.mu.Lock()
	a.syncHookAtomicStateLocked()
	loggedIn := a.loggedIn
	msgs := make([]ChatMsg, len(a.messages))
	copy(msgs, a.messages)
	a.mu.Unlock()

	if loggedIn {
		cr := a.chatRect(rc)
		viewW := cr.Right - cr.Left
		viewH := cr.Bottom - cr.Top
		if viewW > 20 && viewH > 20 {
			oldFont, _, _ := pSelectObject.Call(memDC, a.chatFont)
			pSetBkMode.Call(memDC, TRANSPARENT)
			baseFg := a.effectiveTextColor()
			pSetTextColor.Call(memDC, uintptr(baseFg))

			gapY := a.dpiScale(8)
			tsGapY := a.dpiScale(2)
			type para struct {
				text        string
				isTimestamp bool
				gapAfter    int32
			}
			paras := make([]para, 0, len(msgs)+8)

			if len(msgs) == 0 {
				emptyTxt := "暂无群聊消息"
				if a.cfg.ClearBeforeTs > 0 {
					emptyTxt = "已清屏（等待新消息）"
				}
				paras = append(paras, para{text: emptyTxt, gapAfter: gapY})
			} else {
				nowTime := time.Now()
				for i, m := range msgs {
					var prevTs int64
					if i > 0 {
						prevTs = msgs[i-1].CreatedAt
					}
					if shouldShowConversationTimestamp(prevTs, m.CreatedAt, i == 0) {
						if tsStr := formatConversationTime(m.CreatedAt, nowTime); tsStr != "" {
							paras = append(paras, para{text: tsStr, isTimestamp: true, gapAfter: tsGapY})
						}
					}
					paras = append(paras, para{text: a.formatMsgLine(m), gapAfter: gapY})
				}
			}

			maxTextW := viewW - a.dpiScale(6)
			lines := make([]visualLine, 0, len(paras)*2)
			var totalH int32
			for pIdx, p := range paras {
				font := a.chatFont
				if p.isTimestamp && a.tsFont != 0 {
					font = a.tsFont
				}
				pLines := wrapTextWithFont(memDC, font, p.text, maxTextW, pIdx, p.isTimestamp)
				for i := range pLines {
					pLines[i].y = totalH
					totalH += pLines[i].h
				}
				lines = append(lines, pLines...)
				if len(pLines) > 0 && pIdx < len(paras)-1 {
					totalH += p.gapAfter
				}
			}

			maxScroll := totalH - viewH
			if maxScroll < 0 {
				maxScroll = 0
			}
			a.mu.Lock()
			a.visLines = lines
			a.maxScrollY = maxScroll
			if maxScroll > 0 {
				atomic.StoreInt32(&a.hookOverflow, 1)
			} else {
				atomic.StoreInt32(&a.hookOverflow, 0)
			}
			a.syncHookAtomicStateLocked()
			if a.atBottom {
				a.scrollY = maxScroll
			}
			if a.scrollY > maxScroll {
				a.scrollY = maxScroll
			}
			if a.scrollY < 0 {
				a.scrollY = 0
			}
			curScroll := a.scrollY
			a.selAnchor = clampTextPos(lines, a.selAnchor)
			a.selFocus = clampTextPos(lines, a.selFocus)
			selLo, selHi, hasSel := normalizedSelection(a.selAnchor, a.selFocus)
			a.mu.Unlock()

			for li, ln := range lines {
				screenY := cr.Top + ln.y - curScroll
				if screenY+ln.h < cr.Top || screenY > cr.Bottom {
					continue
				}
				clipTop := screenY
				if clipTop < cr.Top {
					clipTop = cr.Top
				}
				clipBottom := screenY + ln.h
				if clipBottom > cr.Bottom {
					clipBottom = cr.Bottom
				}
				if clipBottom <= clipTop {
					continue
				}
				if ln.isTimestamp && a.tsFont != 0 {
					pSelectObject.Call(memDC, a.tsFont)
				} else {
					pSelectObject.Call(memDC, a.chatFont)
				}
				drawVisualLine(memDC, ln, cr.Left, screenY, cr.Top, cr.Bottom, baseFg, hasSel, selLo, selHi, li)
			}
			pSelectObject.Call(memDC, oldFont)

			if maxScroll > 0 && viewH > 12 {
				totalRange := maxScroll + viewH
				thumbH := (viewH * viewH) / totalRange
				if minH := a.dpiScale(24); thumbH < minH {
					thumbH = minH
				}
				if thumbH > viewH {
					thumbH = viewH
				}
				avail := viewH - thumbH
				thumbY := cr.Top
				if avail > 0 {
					thumbY += (curScroll * avail) / maxScroll
				}
				sbW := a.dpiScale(6)
				if sbW < 4 {
					sbW = 4
				}
				// Subtle non-colorkey scrollbar track so clicking/dragging the track also catches the mouse in transparent mode
				trackW := sbW / 2
				if trackW < 2 {
					trackW = 2
				}
				trackX := cr.Right - sbW + (sbW-trackW)/2
				trackRc := RECT{Left: trackX, Top: cr.Top, Right: trackX + trackW, Bottom: cr.Bottom}
				trackCol := adaptiveInputBorderColor(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6))
				if a.cfg.BgTransparent && a.inputBgColor != 0 {
					trackCol = adaptiveInputBorderColor(a.inputBgColor)
				}
				if trBr, _, _ := pCreateSolidBrush.Call(uintptr(trackCol)); trBr != 0 {
					pFillRect.Call(memDC, uintptr(unsafe.Pointer(&trackRc)), trBr)
					pDeleteObject.Call(trBr)
				}
				sbRc := RECT{Left: cr.Right - sbW, Top: thumbY, Right: cr.Right, Bottom: thumbY + thumbH}
				if sbBr, _, _ := pCreateSolidBrush.Call(uintptr(baseFg)); sbBr != 0 {
					pFillRect.Call(memDC, uintptr(unsafe.Pointer(&sbRc)), sbBr)
					pDeleteObject.Call(sbBr)
				}
			}
		}
	}

	// Re-fill header & input padding with bgBrush so scrolled text never bleeds outside chat viewport
	if loggedIn {
		cr := a.chatRect(rc)
		topClip := RECT{Left: 0, Top: 0, Right: cw, Bottom: cr.Top}
		botClip := RECT{Left: 0, Top: cr.Bottom, Right: cw, Bottom: ch}
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&topClip)), a.bgBrush)
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&botClip)), a.bgBrush)

		// Draw custom chat input box (transparent in transparent mode; matches background color when opaque) with adaptive visible border
		bx := a.inputBoxRect(rc)
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&bx)), a.bgBrush)
		surfaceRef := hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
		if a.cfg.BgTransparent && a.inputBgColor != 0 {
			surfaceRef = a.inputBgColor
		}
		borderRef := adaptiveInputBorderColor(surfaceRef)
		if borderBr, _, _ := pCreateSolidBrush.Call(uintptr(borderRef)); borderBr != 0 {
			pFrameRect.Call(memDC, uintptr(unsafe.Pointer(&bx)), borderBr)
			if a.dpiScale(2) >= 2 && bx.Right-bx.Left > 6 && bx.Bottom-bx.Top > 6 {
				innerBx := RECT{Left: bx.Left + 1, Top: bx.Top + 1, Right: bx.Right - 1, Bottom: bx.Bottom - 1}
				pFrameRect.Call(memDC, uintptr(unsafe.Pointer(&innerBx)), borderBr)
			}
			pDeleteObject.Call(borderBr)
		}
	}

	pBitBlt.Call(hdc, 0, 0, uintptr(cw), uintptr(ch), memDC, 0, 0, SRCCOPY)
	pSelectObject.Call(memDC, oldBmp)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(memDC)
}

func (a *appState) drawDarkButton(dis *DRAWITEMSTRUCT) {
	if dis == nil || dis.HDC == 0 {
		return
	}
	pressed := (dis.ItemState & ODS_SELECTED) != 0
	bgRef := hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
	fillCol := darkenColorForButton(bgRef, pressed)
	borderCol := brightenColor(fillCol, 22)

	rc := dis.RcItem
	if br, _, _ := pCreateSolidBrush.Call(uintptr(fillCol)); br != 0 {
		pFillRect.Call(dis.HDC, uintptr(unsafe.Pointer(&rc)), br)
		pDeleteObject.Call(br)
	}
	if fbr, _, _ := pCreateSolidBrush.Call(uintptr(borderCol)); fbr != 0 {
		pFrameRect.Call(dis.HDC, uintptr(unsafe.Pointer(&rc)), fbr)
		pDeleteObject.Call(fbr)
	}

	txt := getCtrlText(dis.HwndItem)
	if txt != "" {
		font := a.boldFont
		if dis.HwndItem == a.hBtnLoginQuit {
			font = a.uiFont
		}
		if font != 0 {
			oldF, _, _ := pSelectObject.Call(dis.HDC, font)
			defer pSelectObject.Call(dis.HDC, oldF)
		}
		pSetBkMode.Call(dis.HDC, TRANSPARENT)
		pSetTextColor.Call(dis.HDC, 0x00F6F8F5) // bright off-white text
		pDrawTextW.Call(dis.HDC, uintptr(unsafe.Pointer(u16(txt))), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_NOPREFIX)
	}
}

func (a *appState) triggerRepaint() {
	if a.hwnd != 0 {
		pInvalidateRect.Call(a.hwnd, 0, 0)
	}
}

func (a *appState) handleLoginClick() {
	a.mu.Lock()
	if a.loggingIn {
		a.mu.Unlock()
		return
	}
	a.loggingIn = true
	a.mu.Unlock()

	srv := defaultServerURL
	usr := strings.TrimSpace(getCtrlText(a.hEditUser))
	pwd := getCtrlText(a.hEditPass)
	if usr == "" || pwd == "" {
		setCtrlText(a.hLblLoginStatus, "请输入账号和密码")
		a.mu.Lock()
		a.loggingIn = false
		a.mu.Unlock()
		return
	}

	setCtrlText(a.hLblLoginStatus, "正在登录...")
	go func() {
		loginRes, err := doLogin(srv, usr, pwd)
		var grp *FamilyGroupInfo
		if err == nil {
			grp, _, err = fetchDefaultFamilyGroup(srv, loginRes.Role, loginRes.Token)
		}
		a.mu.Lock()
		a.loggingIn = false
		if err != nil {
			a.loginStatus = err.Error()
		} else {
			a.cfg.ServerURL = srv
			a.cfg.Username = usr
			a.cfg.Token = loginRes.Token
			a.cfg.Role = loginRes.Role
			a.cfg.ActorID = loginRes.ActorID
			a.cfg.DisplayName = loginRes.DisplayName
			a.cfg.GroupID = grp.ID
			a.cfg.GroupName = grp.Name
			a.cfg.upsertSavedAccount(SavedAccount{
				Username:    usr,
				Role:        loginRes.Role,
				ActorID:     loginRes.ActorID,
				DisplayName: loginRes.DisplayName,
				Token:       loginRes.Token,
				GroupID:     grp.ID,
				GroupName:   grp.Name,
				LastLoginAt: time.Now().UnixMilli(),
			})
			saveConfig(a.cfg)
			a.loggedIn = true
			a.messages = nil
			a.seenIDs = make(map[string]bool)
			a.lastMsgTs = 0
			a.initialSynced = false
			a.atBottom = true
			a.visLines = nil
			a.selecting = false
			a.selAnchor = textPos{}
			a.selFocus = textPos{}
			a.loginStatus = ""
		}
		a.mu.Unlock()
		a.setUnreadCount(0)
		pPostMessageW.Call(a.hwnd, wmAppRefresh, 1, 0)
	}()
}

func (a *appState) pollMessagesAsync() {
	a.mu.Lock()
	if !a.loggedIn || a.cfg.Token == "" || a.cfg.GroupID == "" || a.polling {
		a.mu.Unlock()
		return
	}
	a.polling = true
	srv := a.cfg.ServerURL
	role := a.cfg.Role
	tok := a.cfg.Token
	gid := a.cfg.GroupID
	after := a.lastMsgTs
	clearBefore := a.cfg.ClearBeforeTs
	if clearBefore > after {
		after = clearBefore
	}
	maxLim := a.cfg.MaxMessages
	a.mu.Unlock()

	go func() {
		if gid == "" {
			a.mu.Lock()
			a.polling = false
			a.mu.Unlock()
			return
		}
		msgs, code, err := fetchGroupMessages(srv, role, tok, gid, after, maxLim)
		a.mu.Lock()
		a.polling = false
		if code == 401 || code == 403 {
			a.loggedIn = false
			a.cfg.clearSavedAccountToken(a.cfg.Role, a.cfg.ActorID, a.cfg.Username)
			a.cfg.Token = ""
			saveConfig(a.cfg)
			a.loginStatus = "登录已过期，请重新登录"
			a.mu.Unlock()
			pPostMessageW.Call(a.hwnd, wmAppRefresh, 1, 0)
			return
		}
		changed := false
		incomingOtherCount := 0
		wasInitial := !a.initialSynced
		if err == nil {
			if a.seenIDs == nil {
				a.seenIDs = make(map[string]bool)
			}
			msgs = filterMessagesAfterClear(msgs, a.cfg.ClearBeforeTs)
			for _, m := range msgs {
				if !a.seenIDs[m.ID] {
					a.seenIDs[m.ID] = true
					a.messages = append(a.messages, m)
					if m.CreatedAt > a.lastMsgTs {
						a.lastMsgTs = m.CreatedAt
					}
					changed = true
					if !wasInitial && !m.IsSelf {
						incomingOtherCount++
					}
				}
			}
			a.initialSynced = true
			if a.cfg.MaxMessages > 0 && len(a.messages) > a.cfg.MaxMessages {
				a.messages = a.messages[len(a.messages)-a.cfg.MaxMessages:]
			}
		}
		a.mu.Unlock()
		if changed {
			pPostMessageW.Call(a.hwnd, wmAppRefresh, 0, uintptr(incomingOtherCount))
		}
	}()
}

func (a *appState) handleSendMessage() {
	if !a.loggedIn {
		return
	}
	a.setUnreadCount(0)
	txt := strings.TrimSpace(getCtrlText(a.hEditMsg))
	if txt == "" {
		return
	}
	a.mu.Lock()
	if a.sending {
		a.mu.Unlock()
		return
	}
	a.sending = true
	srv := a.cfg.ServerURL
	role := a.cfg.Role
	tok := a.cfg.Token
	gid := a.cfg.GroupID
	a.mu.Unlock()

	setCtrlText(a.hEditMsg, "")
	go func() {
		msg, code, err := sendGroupText(srv, role, tok, gid, txt)
		a.mu.Lock()
		a.sending = false
		if code == 401 || code == 403 {
			a.loggedIn = false
			a.cfg.Token = ""
			saveConfig(a.cfg)
			a.mu.Unlock()
			pPostMessageW.Call(a.hwnd, wmAppRefresh, 1, 0)
			return
		}
		if err == nil && msg != nil {
			if a.seenIDs == nil {
				a.seenIDs = make(map[string]bool)
			}
			if !a.seenIDs[msg.ID] {
				a.seenIDs[msg.ID] = true
				a.messages = append(a.messages, *msg)
				if a.cfg.ClearBeforeTs > msg.CreatedAt && a.cfg.ClearBeforeTs-msg.CreatedAt < 300000 {
					a.cfg.ClearBeforeTs = msg.CreatedAt - 1
					saveConfig(a.cfg)
				}
				if msg.CreatedAt > a.lastMsgTs {
					a.lastMsgTs = msg.CreatedAt
				}
			}
			a.atBottom = true
		}
		a.mu.Unlock()
		pPostMessageW.Call(a.hwnd, wmAppRefresh, 0, 0)
	}()
}

func (a *appState) addTrayIcon() {
	if a.hwnd == 0 || a.trayAdded {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = a.hwnd
	nid.UID = idTrayIcon
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = wmAppTray
	nid.HIcon = a.hIcon
	tip, _ := syscall.UTF16FromString("未读信息0条")
	copy(nid.SzTip[:], tip)
	r, _, _ := pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if r != 0 {
		a.trayAdded = true
		a.unreadCount = 0
		a.trayUnreadCount = 0
	}
}

func (a *appState) setUnreadCount(count int) {
	if count < 0 {
		count = 0
	}
	a.unreadCount = count
	if a.hwnd == 0 || !a.trayAdded || a.trayUnreadCount == count {
		return
	}
	icon := a.hIcon
	if count > 0 && a.hIconUnread != 0 {
		icon = a.hIconUnread
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = a.hwnd
	nid.UID = idTrayIcon
	nid.UFlags = NIF_ICON | NIF_TIP
	nid.HIcon = icon
	tip, _ := syscall.UTF16FromString(formatTrayUnreadTip(count))
	copy(nid.SzTip[:], tip)
	pShellNotifyIconW.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&nid)))
	a.trayUnreadCount = count
}

func (a *appState) isActivelyViewing() bool {
	if a.hwnd == 0 {
		return false
	}
	vis, _, _ := pIsWindowVisible.Call(a.hwnd)
	if vis == 0 {
		return false
	}
	fg, _, _ := pGetForegroundWindow.Call()
	return fg == a.hwnd
}

func (a *appState) removeTrayIcon() {
	if !a.trayAdded || a.hwnd == 0 {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.Hwnd = a.hwnd
	nid.UID = idTrayIcon
	pShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	a.trayAdded = false
}

func menuFlags(checked bool) uintptr {
	if checked {
		return MF_STRING | MF_CHECKED
	}
	return MF_STRING
}

func (a *appState) showTrayMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}

	vis, _, _ := pIsWindowVisible.Call(a.hwnd)
	toggleLabel := "显示窗口 (" + a.cfg.Hotkey + ")"
	if vis != 0 {
		toggleLabel = "隐藏窗口 (" + a.cfg.Hotkey + ")"
	}
	pAppendMenuW.Call(menu, MF_STRING, idMenuToggle, uintptr(unsafe.Pointer(u16(toggleLabel))))
	if a.loggedIn {
		pAppendMenuW.Call(menu, MF_STRING, idMenuClearNow, uintptr(unsafe.Pointer(u16("清空屏幕 ("+a.cfg.ClearHotkey+" / Ctrl+L)"))))
	}
	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)

	fontSettingsMenu, _, _ := pCreatePopupMenu.Call()
	if fontSettingsMenu != 0 {
		fontLabel := "字体颜色（屏幕吸管） [" + a.cfg.FontColor + "]"
		if a.cfg.AutoInvertText {
			fontLabel = "字体颜色（屏幕吸管） [已开反显]"
		}
		pAppendMenuW.Call(fontSettingsMenu, MF_STRING, idMenuPickFontColor, uintptr(unsafe.Pointer(u16(fontLabel))))
		pAppendMenuW.Call(fontSettingsMenu, menuFlags(a.cfg.AutoInvertText), idMenuToggleInvert, uintptr(unsafe.Pointer(u16("文字反显（随背景自动取反）"))))

		fontSizeMenu, _, _ := pCreatePopupMenu.Call()
		if fontSizeMenu != 0 {
			pAppendMenuW.Call(fontSizeMenu, menuFlags(a.cfg.FontSize == 12), idMenuFont12, uintptr(unsafe.Pointer(u16("12 px（超小）"))))
			pAppendMenuW.Call(fontSizeMenu, menuFlags(a.cfg.FontSize == 14), idMenuFont14, uintptr(unsafe.Pointer(u16("14 px（较小）"))))
			pAppendMenuW.Call(fontSizeMenu, menuFlags(a.cfg.FontSize == 15), idMenuFont15, uintptr(unsafe.Pointer(u16("15 px（标准）"))))
			pAppendMenuW.Call(fontSizeMenu, menuFlags(a.cfg.FontSize == 18), idMenuFont18, uintptr(unsafe.Pointer(u16("18 px（大字）"))))
			pAppendMenuW.Call(fontSizeMenu, menuFlags(a.cfg.FontSize == 20), idMenuFont20, uintptr(unsafe.Pointer(u16("20 px（超大）"))))
			pAppendMenuW.Call(fontSettingsMenu, MF_POPUP, fontSizeMenu, uintptr(unsafe.Pointer(u16(fmt.Sprintf("字体大小 (%dpx)", a.cfg.FontSize)))))
		}

		fontSummary := fmt.Sprintf("字体设置 (%s / %dpx)", a.cfg.FontColor, a.cfg.FontSize)
		if a.cfg.AutoInvertText {
			fontSummary = fmt.Sprintf("字体设置 (反显 / %dpx)", a.cfg.FontSize)
		}
		pAppendMenuW.Call(menu, MF_POPUP, fontSettingsMenu, uintptr(unsafe.Pointer(u16(fontSummary))))
	}

	bgMenu, _, _ := pCreatePopupMenu.Call()
	if bgMenu != 0 {
		pAppendMenuW.Call(bgMenu, menuFlags(a.cfg.BgTransparent), idMenuToggleTransparent, uintptr(unsafe.Pointer(u16("开启透明背景（仅保留文字）"))))
		pAppendMenuW.Call(bgMenu, MF_STRING, idMenuPickBgColor, uintptr(unsafe.Pointer(u16("背景颜色（屏幕吸管） ["+a.cfg.BgColor+"]"))))
		bgSummary := "背景设置 (" + a.cfg.BgColor + ")"
		if a.cfg.BgTransparent {
			bgSummary = "背景设置 (透明已开启)"
		}
		pAppendMenuW.Call(menu, MF_POPUP, bgMenu, uintptr(unsafe.Pointer(u16(bgSummary))))
	}

	limitMenu, _, _ := pCreatePopupMenu.Call()
	if limitMenu != 0 {
		pAppendMenuW.Call(limitMenu, menuFlags(a.cfg.MaxMessages == 100), idMenuLimit100, uintptr(unsafe.Pointer(u16("最近 100 条"))))
		pAppendMenuW.Call(limitMenu, menuFlags(a.cfg.MaxMessages == 500), idMenuLimit500, uintptr(unsafe.Pointer(u16("最近 500 条"))))
		pAppendMenuW.Call(limitMenu, menuFlags(a.cfg.MaxMessages == 0), idMenuLimitAll, uintptr(unsafe.Pointer(u16("不限制"))))
		limLabel := "显示条数 (不限制)"
		if a.cfg.MaxMessages == 100 {
			limLabel = "显示条数 (100条)"
		} else if a.cfg.MaxMessages == 500 {
			limLabel = "显示条数 (500条)"
		}
		pAppendMenuW.Call(menu, MF_POPUP, limitMenu, uintptr(unsafe.Pointer(u16(limLabel))))
	}

	hkMenu, _, _ := pCreatePopupMenu.Call()
	if hkMenu != 0 {
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Alt+H"), idMenuHKAltH, uintptr(unsafe.Pointer(u16("Alt+H"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Alt+Q"), idMenuHKAltQ, uintptr(unsafe.Pointer(u16("Alt+Q"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Ctrl+Alt+H"), idMenuHKCtrlAltH, uintptr(unsafe.Pointer(u16("Ctrl+Alt+H"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "F9"), idMenuHKF9, uintptr(unsafe.Pointer(u16("F9"))))
		pAppendMenuW.Call(menu, MF_POPUP, hkMenu, uintptr(unsafe.Pointer(u16("隐藏热键 ("+a.cfg.Hotkey+")"))))
	}

	clearHkMenu, _, _ := pCreatePopupMenu.Call()
	if clearHkMenu != 0 {
		pAppendMenuW.Call(clearHkMenu, menuFlags(a.cfg.ClearHotkey == "Alt+L"), idMenuClearHKAltL, uintptr(unsafe.Pointer(u16("Alt+L"))))
		pAppendMenuW.Call(clearHkMenu, menuFlags(a.cfg.ClearHotkey == "Alt+C"), idMenuClearHKAltC, uintptr(unsafe.Pointer(u16("Alt+C"))))
		pAppendMenuW.Call(clearHkMenu, menuFlags(a.cfg.ClearHotkey == "Ctrl+Alt+L"), idMenuClearHKCtrlAltL, uintptr(unsafe.Pointer(u16("Ctrl+Alt+L"))))
		pAppendMenuW.Call(clearHkMenu, menuFlags(a.cfg.ClearHotkey == "F8"), idMenuClearHKF8, uintptr(unsafe.Pointer(u16("F8"))))
		if a.cfg.ClearBeforeTs > 0 {
			pAppendMenuW.Call(clearHkMenu, MF_SEPARATOR, 0, 0)
			pAppendMenuW.Call(clearHkMenu, MF_STRING, idMenuUnclearHistory, uintptr(unsafe.Pointer(u16("恢复显示清屏前消息"))))
		}
		pAppendMenuW.Call(menu, MF_POPUP, clearHkMenu, uintptr(unsafe.Pointer(u16("清屏热键 ("+a.cfg.ClearHotkey+")"))))
	}

	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	accMenu, _, _ := pCreatePopupMenu.Call()
	if accMenu != 0 {
		a.mu.Lock()
		accounts := append([]SavedAccount(nil), a.cfg.SavedAccounts...)
		activeCfg := a.cfg
		loggedIn := a.loggedIn
		a.mu.Unlock()
		for idx, acc := range accounts {
			if idx >= maxMenuSavedAccounts {
				break
			}
			label := savedAccountLabel(acc)
			pAppendMenuW.Call(accMenu, menuFlags(loggedIn && activeCfg.isActiveSavedAccount(acc)), uintptr(idMenuAccountBase+idx), uintptr(unsafe.Pointer(u16(label))))
		}
		if len(accounts) > 0 {
			pAppendMenuW.Call(accMenu, MF_SEPARATOR, 0, 0)
		}
		pAppendMenuW.Call(accMenu, MF_STRING, idMenuAddAccount, uintptr(unsafe.Pointer(u16("+ 登录其他账号..."))))
		curAccLabel := "切换账号"
		if curName := strings.TrimSpace(activeCfg.DisplayName); curName != "" {
			curAccLabel = "切换账号 (" + curName + ")"
		} else if activeCfg.Username != "" {
			curAccLabel = "切换账号 (" + activeCfg.Username + ")"
		}
		pAppendMenuW.Call(menu, MF_POPUP, accMenu, uintptr(unsafe.Pointer(u16(curAccLabel))))
	}
	verName, _ := currentFishVersion()
	pAppendMenuW.Call(menu, MF_STRING, idMenuChangelog, uintptr(unsafe.Pointer(u16("更新内容"))))
	pAppendMenuW.Call(menu, MF_STRING, idMenuCheckUpdate, uintptr(unsafe.Pointer(u16("检查更新 (v"+verName+")"))))
	if a.loggedIn {
		pAppendMenuW.Call(menu, MF_STRING, idMenuLogout, uintptr(unsafe.Pointer(u16("注销账号"))))
	}
	pAppendMenuW.Call(menu, MF_STRING, idMenuQuit, uintptr(unsafe.Pointer(u16("退出程序"))))

	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(a.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(menu, TPM_LEFTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, a.hwnd, 0)
	pDestroyMenu.Call(menu)

	if cmd != 0 {
		if cmd == idMenuPickFontColor || cmd == idMenuPickBgColor {
			// Allow popup menu fade animation to finish before capturing screen pixels
			time.Sleep(80 * time.Millisecond)
		}
		a.handleCommand(int(cmd))
	}
}

func (a *appState) registerBossHotkey() {
	if a.hwnd == 0 {
		return
	}
	pUnregisterHotKey.Call(a.hwnd, idHotkeyBoss)
	mods, vk, label := parseHotkey(a.cfg.Hotkey)
	a.cfg.Hotkey = label
	_, _, _ = pRegisterHotKey.Call(a.hwnd, idHotkeyBoss, uintptr(mods|MOD_NOREPEAT), uintptr(vk))
}

func (a *appState) registerClearHotkey() {
	if a.hwnd == 0 {
		return
	}
	pUnregisterHotKey.Call(a.hwnd, idHotkeyClear)
	mods, vk, label := parseClearHotkey(a.cfg.ClearHotkey)
	a.cfg.ClearHotkey = label
	_, _, _ = pRegisterHotKey.Call(a.hwnd, idHotkeyClear, uintptr(mods|MOD_NOREPEAT), uintptr(vk))
}

func (a *appState) clearMessagesNow() {
	if !a.loggedIn {
		return
	}
	a.mu.Lock()
	cutoff := time.Now().UnixMilli()
	if a.lastMsgTs > cutoff {
		cutoff = a.lastMsgTs
	}
	for _, m := range a.messages {
		if m.CreatedAt > cutoff {
			cutoff = m.CreatedAt
		}
	}
	a.cfg.ClearBeforeTs = cutoff
	a.messages = nil
	a.seenIDs = make(map[string]bool)
	a.lastMsgTs = cutoff
	a.scrollY = 0
	a.maxScrollY = 0
	a.atBottom = true
	a.visLines = nil
	a.selecting = false
	a.selAnchor = textPos{}
	a.selFocus = textPos{}
	saveConfig(a.cfg)
	a.mu.Unlock()
	a.setUnreadCount(0)
	a.triggerRepaint()
}

func (a *appState) restoreClearedMessages() {
	a.mu.Lock()
	a.cfg.ClearBeforeTs = 0
	a.messages = nil
	a.seenIDs = make(map[string]bool)
	a.lastMsgTs = 0
	a.initialSynced = false
	a.atBottom = true
	a.visLines = nil
	a.selecting = false
	a.selAnchor = textPos{}
	a.selFocus = textPos{}
	saveConfig(a.cfg)
	loggedIn := a.loggedIn
	a.mu.Unlock()
	if loggedIn {
		a.pollMessagesAsync()
	}
	a.triggerRepaint()
}

func (a *appState) showAndBringToFront(focusInput bool) {
	if a.hwnd == 0 {
		return
	}
	a.setUnreadCount(0)
	pShowWindow.Call(a.hwnd, SW_SHOW)
	pSetWindowPos.Call(a.hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	pSetWindowPos.Call(a.hwnd, HWND_NOTOPMOST, 0, 0, 0, 0, SWP_NOMOVE|SWP_NOSIZE|SWP_SHOWWINDOW)
	pSetForegroundWindow.Call(a.hwnd)
	if focusInput && a.loggedIn && a.hEditMsg != 0 {
		pSetFocus.Call(a.hEditMsg)
	}
	a.sampleUnderWindow(true)
	a.triggerRepaint()
	if a.hLblGroup != 0 {
		pInvalidateRect.Call(a.hLblGroup, 0, 1)
	}
	if a.hEditMsg != 0 {
		pInvalidateRect.Call(a.hEditMsg, 0, 1)
	}
	pUpdateWindow.Call(a.hwnd)
}

func (a *appState) toggleBossHide() {
	if a.hwnd == 0 {
		return
	}
	vis, _, _ := pIsWindowVisible.Call(a.hwnd)
	if vis != 0 {
		pShowWindow.Call(a.hwnd, SW_HIDE)
	} else {
		a.showAndBringToFront(true)
	}
}

func (a *appState) updateAppearance() {
	saveConfig(a.cfg)
	a.refreshResources()
	a.layoutControls()
	a.showAndBringToFront(true)
}

func (a *appState) handleCommand(id int) {
	switch id {
	case idBtnLogin:
		a.handleLoginClick()
	case idMenuToggle:
		a.toggleBossHide()
	case idMenuPickFontColor:
		if c, ok := a.pickScreenColor("字体"); ok {
			a.cfg.FontColor = colorrefToHex(c)
			a.cfg.AutoInvertText = false
			a.updateAppearance()
		} else {
			a.showAndBringToFront(false)
		}
	case idMenuToggleInvert:
		a.cfg.AutoInvertText = !a.cfg.AutoInvertText
		a.updateAppearance()
	case idMenuPickBgColor:
		if c, ok := a.pickScreenColor("背景"); ok {
			a.cfg.BgColor = colorrefToHex(c)
			a.cfg.BgTransparent = false
			a.updateAppearance()
		} else {
			a.showAndBringToFront(false)
		}
	case idMenuToggleTransparent:
		a.cfg.BgTransparent = !a.cfg.BgTransparent
		a.updateAppearance()
	case idMenuFont12:
		a.cfg.FontSize = 12
		a.updateAppearance()
	case idMenuFont14:
		a.cfg.FontSize = 14
		a.updateAppearance()
	case idMenuFont15:
		a.cfg.FontSize = 15
		a.updateAppearance()
	case idMenuFont18:
		a.cfg.FontSize = 18
		a.updateAppearance()
	case idMenuFont20:
		a.cfg.FontSize = 20
		a.updateAppearance()
	case idMenuLimit100:
		a.applyMessageLimit(100)
	case idMenuLimit500:
		a.applyMessageLimit(500)
	case idMenuLimitAll:
		a.applyMessageLimit(0)
	case idMenuHKAltH:
		a.cfg.Hotkey = "Alt+H"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idMenuHKAltQ:
		a.cfg.Hotkey = "Alt+Q"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idMenuHKCtrlAltH:
		a.cfg.Hotkey = "Ctrl+Alt+H"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idMenuHKF9:
		a.cfg.Hotkey = "F9"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idMenuClearNow:
		a.clearMessagesNow()
	case idMenuClearHKAltL:
		a.cfg.ClearHotkey = "Alt+L"
		saveConfig(a.cfg)
		a.registerClearHotkey()
	case idMenuClearHKAltC:
		a.cfg.ClearHotkey = "Alt+C"
		saveConfig(a.cfg)
		a.registerClearHotkey()
	case idMenuClearHKCtrlAltL:
		a.cfg.ClearHotkey = "Ctrl+Alt+L"
		saveConfig(a.cfg)
		a.registerClearHotkey()
	case idMenuClearHKF8:
		a.cfg.ClearHotkey = "F8"
		saveConfig(a.cfg)
		a.registerClearHotkey()
	case idMenuUnclearHistory:
		a.restoreClearedMessages()
	case idMenuAddAccount:
		a.startAddAccount()
	case idMenuChangelog:
		a.showChangelogAsync()
	case idMenuCheckUpdate:
		a.checkUpdateAsync(true)
	case idMenuLogout:
		a.mu.Lock()
		a.loggedIn = false
		a.cfg.clearSavedAccountToken(a.cfg.Role, a.cfg.ActorID, a.cfg.Username)
		prevUser := a.cfg.Username
		a.cfg.Token = ""
		a.cfg.GroupID = ""
		a.messages = nil
		a.seenIDs = nil
		a.lastMsgTs = 0
		a.initialSynced = false
		a.visLines = nil
		a.selecting = false
		a.selAnchor = textPos{}
		a.selFocus = textPos{}
		saveConfig(a.cfg)
		a.mu.Unlock()
		a.setUnreadCount(0)
		setCtrlText(a.hEditUser, prevUser)
		setCtrlText(a.hEditPass, "")
		setCtrlText(a.hLblLoginStatus, "")
		a.layoutControls()
		a.triggerRepaint()
	case idBtnQuit, idMenuQuit:
		pPostMessageW.Call(a.hwnd, WM_CLOSE, 0, 0)
	default:
		if id >= idMenuAccountBase && id < idMenuAccountBase+maxMenuSavedAccounts {
			a.switchToSavedAccount(id - idMenuAccountBase)
		}
	}
}

func (a *appState) startAddAccount() {
	a.mu.Lock()
	a.loggedIn = false
	a.cfg.Token = ""
	a.cfg.GroupID = ""
	a.cfg.GroupName = ""
	a.cfg.Role = ""
	a.cfg.ActorID = ""
	a.cfg.DisplayName = ""
	a.cfg.Username = ""
	a.messages = nil
	a.seenIDs = nil
	a.lastMsgTs = 0
	a.initialSynced = false
	a.scrollY = 0
	a.maxScrollY = 0
	a.atBottom = true
	a.visLines = nil
	a.selecting = false
	a.selAnchor = textPos{}
	a.selFocus = textPos{}
	a.loginStatus = ""
	saveConfig(a.cfg)
	a.mu.Unlock()

	a.setUnreadCount(0)
	setCtrlText(a.hEditUser, "")
	setCtrlText(a.hEditPass, "")
	setCtrlText(a.hLblLoginStatus, "")
	a.layoutControls()
	a.showAndBringToFront(false)
	pSetFocus.Call(a.hEditUser)
	a.triggerRepaint()
}

func (a *appState) switchToSavedAccount(idx int) {
	a.mu.Lock()
	if idx < 0 || idx >= len(a.cfg.SavedAccounts) {
		a.mu.Unlock()
		return
	}
	acc := a.cfg.SavedAccounts[idx]
	alreadyActive := a.loggedIn && a.cfg.isActiveSavedAccount(acc)
	if alreadyActive {
		a.mu.Unlock()
		a.showAndBringToFront(false)
		return
	}
	if acc.Token == "" || acc.GroupID == "" {
		a.loggedIn = false
		a.cfg.Token = ""
		a.cfg.Username = acc.Username
		a.cfg.Role = acc.Role
		a.cfg.ActorID = acc.ActorID
		a.cfg.DisplayName = acc.DisplayName
		a.cfg.GroupID = acc.GroupID
		a.cfg.GroupName = acc.GroupName
		a.messages = nil
		a.seenIDs = nil
		a.lastMsgTs = 0
		a.initialSynced = false
		a.scrollY = 0
		a.maxScrollY = 0
		a.atBottom = true
		a.visLines = nil
		a.selecting = false
		a.selAnchor = textPos{}
		a.selFocus = textPos{}
		a.loginStatus = "该账号登录已过期，请输入密码重新登录"
		saveConfig(a.cfg)
		a.mu.Unlock()

		a.setUnreadCount(0)
		setCtrlText(a.hEditUser, acc.Username)
		setCtrlText(a.hEditPass, "")
		setCtrlText(a.hLblLoginStatus, "该账号登录已过期，请输入密码重新登录")
		a.layoutControls()
		a.showAndBringToFront(false)
		pSetFocus.Call(a.hEditPass)
		a.triggerRepaint()
		return
	}

	acc.LastLoginAt = time.Now().UnixMilli()
	a.cfg.Username = acc.Username
	a.cfg.Token = acc.Token
	a.cfg.Role = acc.Role
	a.cfg.ActorID = acc.ActorID
	a.cfg.DisplayName = acc.DisplayName
	a.cfg.GroupID = acc.GroupID
	a.cfg.GroupName = acc.GroupName
	a.cfg.upsertSavedAccount(acc)
	saveConfig(a.cfg)
	a.loggedIn = true
	a.messages = nil
	a.seenIDs = make(map[string]bool)
	a.lastMsgTs = 0
	a.initialSynced = false
	a.scrollY = 0
	a.maxScrollY = 0
	a.atBottom = true
	a.visLines = nil
	a.selecting = false
	a.selAnchor = textPos{}
	a.selFocus = textPos{}
	a.loginStatus = ""
	a.mu.Unlock()

	a.setUnreadCount(0)
	setCtrlText(a.hEditMsg, "")
	a.layoutControls()
	a.showAndBringToFront(false)
	a.pollMessagesAsync()
	a.triggerRepaint()
}

func (a *appState) chatPageStep() int32 {
	if a.hwnd == 0 {
		return 160
	}
	var rc RECT
	pGetClientRect.Call(a.hwnd, uintptr(unsafe.Pointer(&rc)))
	cr := a.chatRect(rc)
	step := ((cr.Bottom - cr.Top) * 4) / 5
	if minStep := a.dpiScale(64); step < minStep {
		step = minStep
	}
	return step
}

func (a *appState) scrollByDelta(dy int32) {
	if !a.loggedIn || dy == 0 {
		return
	}
	a.mu.Lock()
	nextY, atBot := computeScrollY(a.scrollY, a.maxScrollY, dy, a.dpiScale(24))
	changed := nextY != a.scrollY || atBot != a.atBottom
	a.scrollY = nextY
	a.atBottom = atBot
	a.mu.Unlock()
	if changed {
		a.triggerRepaint()
	}
}

func (a *appState) scrollToExtreme(bottom bool) {
	if !a.loggedIn {
		return
	}
	a.mu.Lock()
	target := int32(0)
	if bottom {
		target = a.maxScrollY
	}
	changed := target != a.scrollY || a.atBottom != bottom
	a.scrollY = target
	a.atBottom = bottom
	a.mu.Unlock()
	if changed {
		a.triggerRepaint()
	}
}

func (a *appState) clearSelection() bool {
	a.mu.Lock()
	_, _, hadSel := normalizedSelection(a.selAnchor, a.selFocus)
	a.selecting = false
	a.selAnchor = textPos{}
	a.selFocus = textPos{}
	a.mu.Unlock()
	if hadSel {
		a.triggerRepaint()
	}
	return hadSel
}

func (a *appState) selectAllChat() {
	a.mu.Lock()
	if len(a.visLines) == 0 {
		a.mu.Unlock()
		return
	}
	lastIdx := len(a.visLines) - 1
	a.selecting = false
	a.selAnchor = textPos{line: 0, col: 0}
	a.selFocus = textPos{line: lastIdx, col: len(a.visLines[lastIdx].u16)}
	a.mu.Unlock()
	a.triggerRepaint()
}

func (a *appState) copySelection() bool {
	a.mu.Lock()
	lo, hi, ok := normalizedSelection(a.selAnchor, a.selFocus)
	var u []uint16
	if ok && len(a.visLines) > 0 {
		u = extractSelectedUTF16(a.visLines, lo, hi)
	}
	a.mu.Unlock()
	if len(u) == 0 {
		return false
	}
	return copyUTF16ToClipboard(a.hwnd, u)
}

func (a *appState) showChatContextMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer pDestroyMenu.Call(menu)

	a.mu.Lock()
	_, _, hasSel := normalizedSelection(a.selAnchor, a.selFocus)
	hasLines := len(a.visLines) > 0
	a.mu.Unlock()

	if hasSel {
		pAppendMenuW.Call(menu, MF_STRING, idCtxCopy, uintptr(unsafe.Pointer(u16("复制 (Ctrl+C)"))))
	}
	if hasLines {
		pAppendMenuW.Call(menu, MF_STRING, idCtxSelectAll, uintptr(unsafe.Pointer(u16("全选 (Ctrl+A)"))))
	}
	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(a.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(menu, TPM_LEFTALIGN|TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, a.hwnd, 0)
	switch int(cmd) {
	case idCtxCopy:
		a.copySelection()
	case idCtxSelectAll:
		a.selectAllChat()
	}
}

func (a *appState) applyMessageLimit(newLim int) {
	a.mu.Lock()
	oldLim := a.cfg.MaxMessages
	a.cfg.MaxMessages = newLim
	saveConfig(a.cfg)
	trimmed := false
	if newLim > 0 && len(a.messages) > newLim {
		a.messages = a.messages[len(a.messages)-newLim:]
		trimmed = true
	}
	needRefetch := a.loggedIn && ((oldLim > 0 && newLim == 0) || (oldLim > 0 && newLim > oldLim))
	if needRefetch {
		a.messages = nil
		a.seenIDs = make(map[string]bool)
		a.lastMsgTs = 0
		a.initialSynced = false
		a.atBottom = true
	}
	a.mu.Unlock()
	if needRefetch {
		a.pollMessagesAsync()
	} else if trimmed {
		a.triggerRepaint()
	}
	a.showAndBringToFront(false)
}

func mainWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	switch msg {
	case WM_CREATE:
		a.hwnd = hwnd
		a.createAllControls()
		a.refreshResources()
		a.registerBossHotkey()
		a.registerClearHotkey()
		a.addTrayIcon()
		a.layoutControls()
		pSetTimer.Call(hwnd, idTimerPoll, 2500, 0)
		pSetTimer.Call(hwnd, idTimerContrast, 500, 0)
		a.checkUpdateAsync(false)
		if a.loggedIn {
			a.pollMessagesAsync()
			go func() {
				if grp, code, err := fetchDefaultFamilyGroup(a.cfg.ServerURL, a.cfg.Role, a.cfg.Token); err == nil && code == 200 {
					a.mu.Lock()
					a.cfg.GroupID = grp.ID
					a.cfg.GroupName = grp.Name
					saveConfig(a.cfg)
					a.mu.Unlock()
					pPostMessageW.Call(hwnd, wmAppRefresh, 1, 0)
				}
			}()
		}
		return 0

	case WM_GETMINMAXINFO:
		mmi := (*MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize.X = 260
		mmi.PtMinTrackSize.Y = 240
		return 0

	case WM_SIZE:
		a.layoutControls()
		var wr RECT
		if pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); wr.Right > wr.Left && wr.Bottom > wr.Top {
			a.cfg.Width = int(wr.Right - wr.Left)
			a.cfg.Height = int(wr.Bottom - wr.Top)
		}
		a.sampleUnderWindow(true)
		a.triggerRepaint()
		return 0

	case WM_NCHITTEST:
		var wr RECT
		pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
		sx := int32(int16(lParam & 0xFFFF))
		sy := int32(int16((lParam >> 16) & 0xFFFF))
		b := a.dpiScale(7)
		onLeft := sx >= wr.Left && sx < wr.Left+b
		onRight := sx <= wr.Right && sx >= wr.Right-b
		onTop := sy >= wr.Top && sy < wr.Top+b
		onBottom := sy <= wr.Bottom && sy >= wr.Bottom-b
		switch {
		case onTop && onLeft:
			return HTTOPLEFT
		case onTop && onRight:
			return HTTOPRIGHT
		case onBottom && onLeft:
			return HTBOTTOMLEFT
		case onBottom && onRight:
			return HTBOTTOMRIGHT
		case onLeft:
			return HTLEFT
		case onRight:
			return HTRIGHT
		case onTop:
			return HTTOP
		case onBottom:
			return HTBOTTOM
		}
		if sy-wr.Top < a.dpiScale(38) || !a.loggedIn {
			return HTCAPTION
		}
		return 1 // HTCLIENT

	case wmAppTray:
		ev := uint32(lParam & 0xFFFF)
		if ev == WM_LBUTTONUP || ev == WM_LBUTTONDBLCLK {
			a.showAndBringToFront(true)
		} else if ev == WM_RBUTTONUP {
			a.showTrayMenu()
		}
		return 0

	case WM_MOVE:
		var wr RECT
		if pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); wr.Left > -10000 && wr.Top > -10000 {
			a.cfg.X = int(wr.Left)
			a.cfg.Y = int(wr.Top)
			a.cfg.HasPos = true
		}
		a.sampleUnderWindow(true)
		return 0

	case WM_ERASEBKGND:
		return 1

	case WM_SETCURSOR:
		if a.loggedIn && uint32(lParam&0xFFFF) == 1 {
			var pt POINT
			var rc RECT
			pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
			pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cr := a.chatRect(rc)
			if pt.X >= cr.Left && pt.X <= cr.Right && pt.Y >= cr.Top && pt.Y <= cr.Bottom {
				if atomic.LoadInt32(&a.hookOverflow) == 1 && pt.X >= cr.Right-a.dpiScale(14) {
					arrow, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
					if arrow != 0 {
						pSetCursor.Call(arrow)
						return 1
					}
				} else {
					ibeam, _, _ := pLoadCursorW.Call(0, IDC_IBEAM)
					if ibeam != 0 {
						pSetCursor.Call(ibeam)
						return 1
					}
				}
			}
		}

	case WM_LBUTTONDOWN:
		if a.loggedIn {
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cx := int32(int16(lParam & 0xFFFF))
			cy := int32(int16((lParam >> 16) & 0xFFFF))
			if a.hEditMsg != 0 {
				bx := a.inputBoxRect(rc)
				if cx >= bx.Left && cx <= bx.Right && cy >= bx.Top && cy <= bx.Bottom {
					pSetFocus.Call(a.hEditMsg)
					return 0
				}
			}
			cr := a.chatRect(rc)
			if cx >= cr.Left && cx <= cr.Right && cy >= cr.Top && cy <= cr.Bottom {
				a.mu.Lock()
				hasLines := len(a.visLines) > 0
				maxScr := a.maxScrollY
				if maxScr > 0 && cx >= cr.Right-a.dpiScale(14) && cr.Bottom > cr.Top {
					ratio := float64(cy-cr.Top) / float64(cr.Bottom-cr.Top)
					if ratio < 0 {
						ratio = 0
					} else if ratio > 1 {
						ratio = 1
					}
					targetY := int32(ratio * float64(maxScr))
					a.scrollY = targetY
					a.atBottom = (maxScr - targetY) <= a.dpiScale(24)
					a.draggingScroll = true
					a.mu.Unlock()
					pSetCapture.Call(hwnd)
					pSetFocus.Call(hwnd)
					a.triggerRepaint()
					return 0
				}
				if hasLines {
					pos := hitTestChatLines(a.visLines, a.scrollY, cr.Left, cr.Top, cx, cy)
					a.selAnchor = pos
					a.selFocus = pos
					a.selecting = true
				}
				a.mu.Unlock()
				if hasLines {
					pSetCapture.Call(hwnd)
					pSetFocus.Call(hwnd)
					a.triggerRepaint()
				}
				return 0
			}
		}

	case WM_MOUSEMOVE:
		if a.loggedIn {
			a.mu.Lock()
			dragScr := a.draggingScroll
			selecting := a.selecting
			maxScr := a.maxScrollY
			a.mu.Unlock()
			if dragScr {
				var rc RECT
				pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
				cr := a.chatRect(rc)
				cy := int32(int16((lParam >> 16) & 0xFFFF))
				if cr.Bottom > cr.Top && maxScr > 0 {
					ratio := float64(cy-cr.Top) / float64(cr.Bottom-cr.Top)
					if ratio < 0 {
						ratio = 0
					} else if ratio > 1 {
						ratio = 1
					}
					targetY := int32(ratio * float64(maxScr))
					a.mu.Lock()
					a.scrollY = targetY
					a.atBottom = (a.maxScrollY - targetY) <= a.dpiScale(24)
					a.mu.Unlock()
					a.triggerRepaint()
				}
				return 0
			}
			if selecting {
				var rc RECT
				pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
				cr := a.chatRect(rc)
				cx := int32(int16(lParam & 0xFFFF))
				cy := int32(int16((lParam >> 16) & 0xFFFF))
				a.mu.Lock()
				if cy < cr.Top && a.scrollY > 0 {
					step := a.dpiScale(12)
					a.scrollY -= step
					if a.scrollY < 0 {
						a.scrollY = 0
					}
					a.atBottom = false
				} else if cy > cr.Bottom && a.scrollY < a.maxScrollY {
					step := a.dpiScale(12)
					a.scrollY += step
					if a.scrollY > a.maxScrollY {
						a.scrollY = a.maxScrollY
					}
					a.atBottom = (a.maxScrollY - a.scrollY) <= a.dpiScale(24)
				}
				if len(a.visLines) > 0 {
					a.selFocus = hitTestChatLines(a.visLines, a.scrollY, cr.Left, cr.Top, cx, cy)
				}
				a.mu.Unlock()
				a.triggerRepaint()
				return 0
			}
		}

	case WM_LBUTTONUP:
		a.mu.Lock()
		dragScr := a.draggingScroll
		selecting := a.selecting
		a.draggingScroll = false
		if selecting {
			a.selecting = false
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cr := a.chatRect(rc)
			cx := int32(int16(lParam & 0xFFFF))
			cy := int32(int16((lParam >> 16) & 0xFFFF))
			if len(a.visLines) > 0 {
				a.selFocus = hitTestChatLines(a.visLines, a.scrollY, cr.Left, cr.Top, cx, cy)
			}
		}
		a.mu.Unlock()
		if dragScr || selecting {
			pReleaseCapture.Call()
			a.triggerRepaint()
			return 0
		}

	case WM_LBUTTONDBLCLK:
		if a.loggedIn {
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cr := a.chatRect(rc)
			cx := int32(int16(lParam & 0xFFFF))
			cy := int32(int16((lParam >> 16) & 0xFFFF))
			if cx >= cr.Left && cx <= cr.Right && cy >= cr.Top && cy <= cr.Bottom {
				a.mu.Lock()
				selected := false
				if len(a.visLines) > 0 {
					pos := hitTestChatLines(a.visLines, a.scrollY, cr.Left, cr.Top, cx, cy)
					if lo, hi, ok := paragraphBoundsAtLine(a.visLines, pos.line); ok {
						a.selAnchor = lo
						a.selFocus = hi
						selected = true
					}
				}
				a.mu.Unlock()
				if selected {
					pSetFocus.Call(hwnd)
					a.triggerRepaint()
				}
				return 0
			}
		}

	case WM_RBUTTONUP:
		if a.loggedIn {
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			cr := a.chatRect(rc)
			cx := int32(int16(lParam & 0xFFFF))
			cy := int32(int16((lParam >> 16) & 0xFFFF))
			if cx >= cr.Left && cx <= cr.Right && cy >= cr.Top && cy <= cr.Bottom {
				a.showChatContextMenu()
				return 0
			}
		}

	case WM_KEYDOWN:
		if a.loggedIn {
			ctrlDown := false
			st, _, _ := pGetKeyState.Call(VK_CONTROL)
			if (uint16(st) & 0x8000) != 0 {
				ctrlDown = true
			}
			if ctrlDown && (wParam == 'C' || wParam == 'c') {
				a.copySelection()
				return 0
			}
			if ctrlDown && (wParam == 'A' || wParam == 'a') {
				a.selectAllChat()
				return 0
			}
			if ctrlDown && (wParam == 'L' || wParam == 'l') {
				a.clearMessagesNow()
				return 0
			}
			if wParam == VK_ESCAPE && a.clearSelection() {
				return 0
			}
			switch wParam {
			case VK_PRIOR:
				a.scrollByDelta(-a.chatPageStep())
				return 0
			case VK_NEXT:
				a.scrollByDelta(a.chatPageStep())
				return 0
			case VK_UP:
				a.scrollByDelta(-a.dpiScale(36))
				return 0
			case VK_DOWN:
				a.scrollByDelta(a.dpiScale(36))
				return 0
			case VK_HOME:
				a.scrollToExtreme(false)
				return 0
			case VK_END:
				a.scrollToExtreme(true)
				return 0
			}
		}

	case WM_PAINT:
		a.paintWindow()
		return 0

	case WM_ACTIVATE:
		if (wParam & 0xFFFF) != 0 {
			a.setUnreadCount(0)
		}
		return 0

	case WM_MOUSEWHEEL:
		if a.loggedIn {
			delta := int16((wParam >> 16) & 0xFFFF)
			a.scrollByDelta(wheelScrollDeltaY(delta, a.dpiScale(48)))
		}
		return 0

	case WM_TIMER:
		if wParam == idTimerPoll && a.loggedIn {
			a.pollMessagesAsync()
		} else if wParam == idTimerContrast && a.cfg.BgTransparent {
			a.sampleUnderWindow(false)
		}
		return 0

	case WM_APP + 1:
		a.mu.Lock()
		status := a.loginStatus
		needLayout := wParam == 1
		a.mu.Unlock()
		if !a.loggedIn && status != "" {
			setCtrlText(a.hLblLoginStatus, status)
		}
		if needLayout {
			a.layoutControls()
			if a.loggedIn {
				a.pollMessagesAsync()
			}
		}
		if lParam > 0 && !a.isActivelyViewing() {
			a.setUnreadCount(a.unreadCount + int(lParam))
		}
		a.triggerRepaint()
		return 0

	case wmAppUpdateResult:
		a.handleUpdateMessage(wParam)
		return 0

	case wmAppChangelogResult:
		a.mu.Lock()
		txt := a.pendingChangelog
		a.pendingChangelog = ""
		a.mu.Unlock()
		if txt != "" {
			verName, _ := currentFishVersion()
			pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(u16(txt))), uintptr(unsafe.Pointer(u16("fish 更新内容 (v"+verName+")"))), MB_OK|MB_ICONINFORMATION)
		}
		return 0

	case WM_COMMAND:
		id := int(wParam & 0xFFFF)
		a.handleCommand(id)
		return 0

	case WM_DRAWITEM:
		if lParam != 0 {
			dis := (*DRAWITEMSTRUCT)(unsafe.Pointer(lParam))
			a.drawDarkButton(dis)
			return 1
		}
		return 0

	case WM_CTLCOLORSTATIC, WM_CTLCOLORBTN:
		hdc := wParam
		pSetTextColor.Call(hdc, uintptr(a.effectiveTextColor()))
		if a.cfg.BgTransparent {
			pSetBkColor.Call(hdc, uintptr(chromaKeyRef))
			return a.bgBrush
		}
		pSetBkColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)))
		return a.bgBrush

	case WM_CTLCOLOREDIT:
		hdc := wParam
		pSetTextColor.Call(hdc, uintptr(a.effectiveTextColor()))
		if a.cfg.BgTransparent {
			pSetBkColor.Call(hdc, uintptr(chromaKeyRef))
			return a.bgBrush
		}
		pSetBkColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)))
		return a.bgBrush

	case WM_HOTKEY:
		if wParam == idHotkeyBoss {
			a.toggleBossHide()
		} else if wParam == idHotkeyClear {
			a.clearMessagesNow()
		}
		return 0

	case wmAppShow:
		a.showAndBringToFront(true)
		return 0

	case WM_CLOSE:
		var wr RECT
		pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
		if wr.Right > wr.Left && wr.Bottom > wr.Top {
			a.cfg.Width = int(wr.Right - wr.Left)
			a.cfg.Height = int(wr.Bottom - wr.Top)
			a.cfg.X = int(wr.Left)
			a.cfg.Y = int(wr.Top)
			a.cfg.HasPos = true
		}
		saveConfig(a.cfg)
		a.removeTrayIcon()
		pUnregisterHotKey.Call(hwnd, idHotkeyBoss)
		pUnregisterHotKey.Call(hwnd, idHotkeyClear)
		pKillTimer.Call(hwnd, idTimerPoll)
		pKillTimer.Call(hwnd, idTimerContrast)
		pDestroyWindow.Call(hwnd)
		if a.hIcon != 0 {
			pDestroyIcon.Call(a.hIcon)
			a.hIcon = 0
		}
		if a.hIconUnread != 0 {
			pDestroyIcon.Call(a.hIconUnread)
			a.hIconUnread = 0
		}
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *appState) showChangelogAsync() {
	go func() {
		txt, err := fetchFishChangelogText(defaultServerURL)
		if err != nil {
			txt = "获取更新内容失败: " + err.Error()
		}
		a.mu.Lock()
		a.pendingChangelog = txt
		hwnd := a.hwnd
		a.mu.Unlock()
		if hwnd != 0 {
			pPostMessageW.Call(hwnd, wmAppChangelogResult, 0, 0)
		}
	}()
}

func (a *appState) checkUpdateAsync(manual bool) {
	a.mu.Lock()
	if a.checkingUpdate {
		a.mu.Unlock()
		return
	}
	a.checkingUpdate = true
	srv := defaultServerURL
	a.mu.Unlock()

	_, verCode := currentFishVersion()
	go func() {
		res, err := checkFishUpdate(srv, verCode)
		a.mu.Lock()
		a.checkingUpdate = false
		a.pendingUpdateRes = res
		a.pendingUpdateMan = manual
		if err != nil {
			a.pendingUpdateErr = err.Error()
		} else {
			a.pendingUpdateErr = ""
		}
		hwnd := a.hwnd
		a.mu.Unlock()
		if hwnd != 0 {
			pPostMessageW.Call(hwnd, wmAppUpdateResult, 1, 0)
		}
	}()
}

func (a *appState) handleUpdateMessage(kind uintptr) {
	if kind == 2 {
		pPostMessageW.Call(a.hwnd, WM_CLOSE, 0, 0)
		return
	}

	a.mu.Lock()
	res := a.pendingUpdateRes
	errMsg := a.pendingUpdateErr
	manual := a.pendingUpdateMan
	srv := a.cfg.ServerURL
	a.pendingUpdateRes = nil
	a.pendingUpdateErr = ""
	a.mu.Unlock()

	if errMsg != "" {
		if manual || kind == 3 {
			pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(u16(errMsg))), uintptr(unsafe.Pointer(u16("fish 更新"))), MB_OK|MB_ICONERROR)
		}
		return
	}
	if res == nil || !res.HasRelease || res.Release == nil || !res.HasUpdate {
		if manual {
			verName, _ := currentFishVersion()
			pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(u16("当前已是最新版本（v"+verName+"）"))), uintptr(unsafe.Pointer(u16("fish 检查更新"))), MB_OK|MB_ICONINFORMATION)
		}
		return
	}

	rel := *res.Release
	verName, _ := currentFishVersion()
	prompt := fmt.Sprintf("发现新版本 v%s（当前 v%s）\n\n%s\n\n是否立即下载并重启更新？",
		rel.VersionName, verName, strings.TrimSpace(rel.ReleaseNotes))
	title := "fish 发现新版本"
	if res.ForceUpdate {
		title = "fish 发现重要更新"
	}
	ans, _, _ := pMessageBoxW.Call(a.hwnd, uintptr(unsafe.Pointer(u16(prompt))), uintptr(unsafe.Pointer(u16(title))), MB_YESNO|MB_ICONINFORMATION)
	if ans != IDYES {
		return
	}

	a.mu.Lock()
	if a.checkingUpdate {
		a.mu.Unlock()
		return
	}
	a.checkingUpdate = true
	a.mu.Unlock()

	go func() {
		err := downloadAndApplyFishUpdate(srv, rel)
		a.mu.Lock()
		a.checkingUpdate = false
		if err != nil {
			a.pendingUpdateErr = "自动更新失败：" + err.Error()
			hwnd := a.hwnd
			a.mu.Unlock()
			if hwnd != 0 {
				pPostMessageW.Call(hwnd, wmAppUpdateResult, 3, 0)
			}
			return
		}
		hwnd := a.hwnd
		a.mu.Unlock()
		if hwnd != 0 {
			pPostMessageW.Call(hwnd, wmAppUpdateResult, 2, 0)
		}
	}()
}

func (a *appState) syncHookAtomicStateLocked() {
	if a.loggedIn {
		atomic.StoreInt32(&a.hookLoggedIn, 1)
	} else {
		atomic.StoreInt32(&a.hookLoggedIn, 0)
		atomic.StoreInt32(&a.hookOverflow, 0)
	}
	if a.loggedIn && a.cfg.BgTransparent {
		atomic.StoreInt32(&a.hookTransparent, 1)
	} else {
		atomic.StoreInt32(&a.hookTransparent, 0)
	}
}

func pointInRect(pt POINT, rc RECT) bool {
	return pt.X >= rc.Left && pt.X <= rc.Right && pt.Y >= rc.Top && pt.Y <= rc.Bottom
}

func isFishObscuredAt(hwnd uintptr, pt POINT) bool {
	prev := hwnd
	for {
		next, _, _ := pGetWindow.Call(prev, GW_HWNDPREV)
		if next == 0 || next == prev {
			return false
		}
		prev = next
		vis, _, _ := pIsWindowVisible.Call(prev)
		if vis == 0 {
			continue
		}
		iconic, _, _ := pIsIconic.Call(prev)
		if iconic != 0 {
			continue
		}
		ex, _, _ := pGetWindowLongW.Call(prev, GWL_EXSTYLE)
		if (ex & WS_EX_TRANSPARENT) != 0 {
			continue
		}
		var r RECT
		if pGetWindowRect.Call(prev, uintptr(unsafe.Pointer(&r))); r.Right > r.Left && r.Bottom > r.Top && pointInRect(pt, r) {
			return true
		}
	}
}

func mouseLLHookProc(nCode, wParam, lParam uintptr) uintptr {
	if int32(nCode) >= 0 && uint32(wParam) == WM_MOUSEWHEEL && lParam != 0 && gApp != nil {
		hwnd := gApp.hwnd
		if hwnd != 0 && atomic.LoadInt32(&gApp.hookTransparent) == 1 {
			if vis, _, _ := pIsWindowVisible.Call(hwnd); vis != 0 {
				ms := (*MSLLHOOKSTRUCT)(unsafe.Pointer(lParam))
				var wr RECT
				pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
				if pointInRect(ms.Pt, wr) {
					fg, _, _ := pGetForegroundWindow.Call()
					capture := (fg == hwnd)
					if !capture && atomic.LoadInt32(&gApp.hookOverflow) == 1 && !isFishObscuredAt(hwnd, ms.Pt) {
						cPt := ms.Pt
						pScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&cPt)))
						var rc RECT
						pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
						cr := gApp.chatRect(rc)
						if pointInRect(cPt, cr) {
							capture = true
						}
					}
					if capture {
						delta := int16((ms.MouseData >> 16) & 0xFFFF)
						if delta != 0 {
							wp := uintptr(uint32(uint16(delta)) << 16)
							lp := uintptr((uint32(uint16(ms.Pt.Y)) << 16) | uint32(uint16(ms.Pt.X)))
							pPostMessageW.Call(hwnd, WM_MOUSEWHEEL, wp, lp)
							return 1
						}
					}
				}
			}
		}
	}
	ret, _, _ := pCallNextHookEx.Call(0, nCode, wParam, lParam)
	return ret
}

func (a *appState) installMouseWheelHook() {
	if a.hMouseHook != 0 {
		return
	}
	if a.mouseHookCbPtr == 0 {
		a.mouseHookCbPtr = syscall.NewCallback(mouseLLHookProc)
	}
	hh, _, _ := pSetWindowsHookExW.Call(WH_MOUSE_LL, a.mouseHookCbPtr, a.hInst, 0)
	a.hMouseHook = hh
}

func (a *appState) uninstallMouseWheelHook() {
	if a.hMouseHook != 0 {
		pUnhookWindowsHookEx.Call(a.hMouseHook)
		a.hMouseHook = 0
	}
}

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()
	cleanupPendingUpdateFiles()

	cfg := loadConfig()
	gApp = &appState{
		cfg:      cfg,
		hInst:    getModuleHandle(),
		loggedIn: cfg.Token != "" && cfg.GroupID != "",
		atBottom: true,
		seenIDs:  make(map[string]bool),
	}

	clsName := u16("GrowthBuddyFishWnd")
	if prev, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(clsName)), 0); prev != 0 {
		pPostMessageW.Call(prev, wmAppShow, 0, 0)
		return
	}
	cur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	gApp.hIcon = createFishTrayIcon(false)
	gApp.hIconUnread = createFishTrayIcon(true)
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_DBLCLKS,
		LpfnWndProc:   syscall.NewCallback(mainWndProc),
		HInstance:     gApp.hInst,
		HIcon:         gApp.hIcon,
		HIconSm:       gApp.hIcon,
		HCursor:       cur,
		LpszClassName: clsName,
	}
	atom, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		msg := fmt.Sprintf("窗口注册失败: %v", err)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(msg))), uintptr(unsafe.Pointer(u16("fish"))), 0)
		return
	}

	posX, posY := int32(160), int32(120)
	if cfg.HasPos {
		posX, posY = int32(cfg.X), int32(cfg.Y)
	}
	hwnd, _, err := pCreateWindowExW.Call(
		WS_EX_LAYERED|WS_EX_TOOLWINDOW,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(u16("fish"))),
		WS_POPUP|WS_CLIPCHILDREN,
		uintptr(posX), uintptr(posY),
		uintptr(cfg.Width), uintptr(cfg.Height),
		0, 0, gApp.hInst, 0,
	)
	if hwnd == 0 {
		msg := fmt.Sprintf("窗口创建失败: %v", err)
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(msg))), uintptr(unsafe.Pointer(u16("fish"))), 0)
		return
	}

	pShowWindow.Call(hwnd, SW_SHOW)
	gApp.mu.Lock()
	gApp.syncHookAtomicStateLocked()
	gApp.mu.Unlock()
	gApp.installMouseWheelHook()
	defer gApp.uninstallMouseWheelHook()
	gApp.sampleUnderWindow(true)
	pUpdateWindow.Call(hwnd)

	var m MSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		if atomic.LoadInt32(&gApp.hookLoggedIn) == 0 {
			if !(m.Message == WM_KEYDOWN && (m.WParam == VK_RETURN || m.Hwnd == hwnd)) {
				if handled, _, _ := pIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&m))); handled != 0 {
					continue
				}
			}
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
