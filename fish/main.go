//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	wmAppRefresh      = WM_APP + 1
	wmAppShow         = WM_APP + 2
	wmAppTray         = WM_APP + 3
	wmAppUpdateResult = WM_APP + 4

	idTimerPoll     = 1
	idTimerContrast = 2
	idHotkeyBoss    = 1
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
)

var pGetDpiForWindow = user32.NewProc("GetDpiForWindow")

type appState struct {
	mu          sync.Mutex
	hwnd        uintptr
	hInst       uintptr
	hIcon       uintptr
	trayAdded   bool
	cfg         FishConfig
	loggedIn    bool
	loggingIn   bool
	polling     bool
	sending     bool
	loginStatus string
	messages    []ChatMsg
	seenIDs     map[string]bool
	lastMsgTs   int64
	scrollY     int32
	maxScrollY  int32
	atBottom    bool

	uiFont   uintptr
	boldFont uintptr
	chatFont uintptr
	bgBrush  uintptr
	altBrush uintptr

	// Login controls
	hLblTitle, hBtnLoginQuit, hLblUser, hEditUser, hLblPass, hEditPass, hBtnLogin, hLblLoginStatus uintptr

	// Chat controls (minimal: no buttons in window, Enter to send)
	hLblGroup, hEditMsg uintptr

	checkingUpdate   bool
	pendingUpdateRes *FishVersionResponse
	pendingUpdateErr string
	pendingUpdateMan bool

	backdropBuf  []uint32
	backdropW    int32
	backdropH    int32
	backdropSig  uint64
	autoHeaderFg uint32
	inputBgColor uint32

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
	if a.bgBrush != 0 {
		pDeleteObject.Call(a.bgBrush)
	}
	if a.altBrush != 0 && a.altBrush != a.bgBrush {
		pDeleteObject.Call(a.altBrush)
	}

	qual := uint32(CLEARTYPE_QUALITY)
	if a.cfg.BgTransparent {
		qual = NONANTIALIASED_QUALITY
	}
	a.uiFont = createUIFont(a.dpiScale(14), false, qual)
	a.boldFont = createUIFont(a.dpiScale(15), true, qual)
	a.chatFont = createUIFont(a.dpiScale(int32(a.cfg.FontSize)), false, qual)

	bgRef := hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
	if a.cfg.BgTransparent {
		a.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(chromaKeyRef))
		// Default initial fill to bgRef (never chromaKeyRef so input stays clickable) until backdrop sample refreshes it
		initBoxBg := a.inputBgColor
		if initBoxBg == 0 || initBoxBg == chromaKeyRef {
			initBoxBg = bgRef
			if initBoxBg == chromaKeyRef {
				initBoxBg = 0x001E1E1E
			}
		}
		a.inputBgColor = initBoxBg
		a.altBrush, _, _ = pCreateSolidBrush.Call(uintptr(initBoxBg))
	} else {
		a.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(bgRef))
		a.altBrush = a.bgBrush
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
	hdrEnd := a.dpiScale(36)
	if hdrEnd > ch {
		hdrEnd = ch
	}
	var newHdrFg uint32
	if hdrBg, ok := averageBGRAStripColor(buf, cw, ch, 4, hdrEnd); ok {
		newHdrFg = invertTextColorForBg(hdrBg)
	} else {
		newHdrFg = invertTextColorForBg(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6))
	}

	bx := a.inputBoxRect(rc)
	newBoxBg := a.inputBgColor
	if sampledBoxBg, ok := averageBGRAStripColor(buf, cw, ch, bx.Top, bx.Bottom); ok {
		if sampledBoxBg == chromaKeyRef {
			sampledBoxBg = 0x00020202
		}
		newBoxBg = sampledBoxBg
	}

	hdrChanged := newHdrFg != a.autoHeaderFg
	boxChanged := newBoxBg != a.inputBgColor
	sigChanged := sig != a.backdropSig || a.backdropW != cw || a.backdropH != ch

	a.backdropBuf = buf
	a.backdropW = cw
	a.backdropH = ch
	a.backdropSig = sig
	a.autoHeaderFg = newHdrFg

	if boxChanged {
		a.inputBgColor = newBoxBg
		if newBr, _, _ := pCreateSolidBrush.Call(uintptr(newBoxBg)); newBr != 0 {
			if a.altBrush != 0 && a.altBrush != a.bgBrush {
				pDeleteObject.Call(a.altBrush)
			}
			a.altBrush = newBr
		}
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
	}

	if hdrChanged && a.cfg.AutoInvertText {
		if a.hLblGroup != 0 {
			pInvalidateRect.Call(a.hLblGroup, 0, 1)
		}
		if !a.loggedIn {
			pInvalidateRect.Call(a.hwnd, 0, 1)
		}
	}
	if force || sigChanged || boxChanged || (hdrChanged && a.cfg.AutoInvertText) {
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

	// Subclass Enter key on message & password inputs
	msgCb := syscall.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if msg == WM_KEYDOWN && wParam == VK_RETURN {
			gApp.handleSendMessage()
			return 0
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
			type measured struct {
				text string
				h    int32
			}
			items := make([]measured, 0, len(msgs))
			var totalH int32

			if len(msgs) == 0 {
				items = append(items, measured{text: "暂无群聊消息"})
			} else {
				for _, m := range msgs {
					items = append(items, measured{text: a.formatMsgLine(m)})
				}
			}

			for i := range items {
				calc := RECT{Left: 0, Top: 0, Right: viewW - a.dpiScale(6), Bottom: 0}
				pDrawTextW.Call(memDC, uintptr(unsafe.Pointer(u16(items[i].text))), ^uintptr(0), uintptr(unsafe.Pointer(&calc)), DT_LEFT|DT_WORDBREAK|DT_CALCRECT|DT_NOPREFIX|DT_EDITCONTROL)
				items[i].h = calc.Bottom - calc.Top
				totalH += items[i].h + gapY
			}
			if totalH > gapY {
				totalH -= gapY
			}

			maxScroll := totalH - viewH
			if maxScroll < 0 {
				maxScroll = 0
			}
			a.mu.Lock()
			a.maxScrollY = maxScroll
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
			a.mu.Unlock()

			y := cr.Top - curScroll
			for _, it := range items {
				if y+it.h >= cr.Top && y <= cr.Bottom {
					dr := RECT{Left: cr.Left, Top: y, Right: cr.Right - a.dpiScale(4), Bottom: y + it.h}
					if dr.Top < cr.Top {
						dr.Top = cr.Top
					}
					if dr.Bottom > cr.Bottom {
						dr.Bottom = cr.Bottom
					}
					if dr.Bottom > dr.Top {
						lineFg := baseFg
						if a.cfg.AutoInvertText && a.cfg.BgTransparent && a.backdropW == cw && len(a.backdropBuf) == int(a.backdropW*a.backdropH) {
							if stripBg, ok := averageBGRAStripColor(a.backdropBuf, a.backdropW, a.backdropH, dr.Top, dr.Bottom); ok {
								lineFg = invertTextColorForBg(stripBg)
							}
						}
						pSetTextColor.Call(memDC, uintptr(lineFg))
						drawRc := RECT{Left: cr.Left, Top: y, Right: cr.Right - a.dpiScale(4), Bottom: y + it.h}
						pDrawTextW.Call(memDC, uintptr(unsafe.Pointer(u16(it.text))), ^uintptr(0), uintptr(unsafe.Pointer(&drawRc)), DT_LEFT|DT_WORDBREAK|DT_NOPREFIX|DT_EDITCONTROL)
					}
				}
				y += it.h + gapY
			}
			pSelectObject.Call(memDC, oldFont)
		}
	}

	// Re-fill header & input padding with bgBrush so scrolled text never bleeds outside chat viewport
	if loggedIn {
		cr := a.chatRect(rc)
		topClip := RECT{Left: 0, Top: 0, Right: cw, Bottom: cr.Top}
		botClip := RECT{Left: 0, Top: cr.Bottom, Right: cw, Bottom: ch}
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&topClip)), a.bgBrush)
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&botClip)), a.bgBrush)

		// Draw custom chat input box filled with input surface and a slightly brighter border
		bx := a.inputBoxRect(rc)
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&bx)), a.altBrush)
		surfaceRef := hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)
		if a.cfg.BgTransparent && a.inputBgColor != 0 {
			surfaceRef = a.inputBgColor
		}
		borderRef := brightenColor(surfaceRef, 28)
		if borderBr, _, _ := pCreateSolidBrush.Call(uintptr(borderRef)); borderBr != 0 {
			pFrameRect.Call(memDC, uintptr(unsafe.Pointer(&bx)), borderBr)
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
			a.cfg.Token = loginRes.Token
			a.cfg.Role = loginRes.Role
			a.cfg.ActorID = loginRes.ActorID
			a.cfg.DisplayName = loginRes.DisplayName
			a.cfg.GroupID = grp.ID
			a.cfg.GroupName = grp.Name
			saveConfig(a.cfg)
			a.loggedIn = true
			a.messages = nil
			a.seenIDs = make(map[string]bool)
			a.lastMsgTs = 0
			a.atBottom = true
			a.loginStatus = ""
		}
		a.mu.Unlock()
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
	a.mu.Unlock()

	go func() {
		if gid == "" {
			a.mu.Lock()
			a.polling = false
			a.mu.Unlock()
			return
		}
		msgs, code, err := fetchGroupMessages(srv, role, tok, gid, after)
		a.mu.Lock()
		a.polling = false
		if code == 401 || code == 403 {
			a.loggedIn = false
			a.cfg.Token = ""
			saveConfig(a.cfg)
			a.loginStatus = "登录已过期，请重新登录"
			a.mu.Unlock()
			pPostMessageW.Call(a.hwnd, wmAppRefresh, 1, 0)
			return
		}
		changed := false
		if err == nil {
			if a.seenIDs == nil {
				a.seenIDs = make(map[string]bool)
			}
			for _, m := range msgs {
				if !a.seenIDs[m.ID] {
					a.seenIDs[m.ID] = true
					a.messages = append(a.messages, m)
					if m.CreatedAt > a.lastMsgTs {
						a.lastMsgTs = m.CreatedAt
					}
					changed = true
				}
			}
			if len(a.messages) > 200 {
				a.messages = a.messages[len(a.messages)-200:]
			}
		}
		a.mu.Unlock()
		if changed {
			pPostMessageW.Call(a.hwnd, wmAppRefresh, 0, 0)
		}
	}()
}

func (a *appState) handleSendMessage() {
	if !a.loggedIn {
		return
	}
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
	tip, _ := syscall.UTF16FromString("fish · 家庭群聊")
	copy(nid.SzTip[:], tip)
	r, _, _ := pShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if r != 0 {
		a.trayAdded = true
	}
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

	hkMenu, _, _ := pCreatePopupMenu.Call()
	if hkMenu != 0 {
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Alt+H"), idMenuHKAltH, uintptr(unsafe.Pointer(u16("Alt+H"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Alt+Q"), idMenuHKAltQ, uintptr(unsafe.Pointer(u16("Alt+Q"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "Ctrl+Alt+H"), idMenuHKCtrlAltH, uintptr(unsafe.Pointer(u16("Ctrl+Alt+H"))))
		pAppendMenuW.Call(hkMenu, menuFlags(a.cfg.Hotkey == "F9"), idMenuHKF9, uintptr(unsafe.Pointer(u16("F9"))))
		pAppendMenuW.Call(menu, MF_POPUP, hkMenu, uintptr(unsafe.Pointer(u16("隐藏热键 ("+a.cfg.Hotkey+")"))))
	}

	pAppendMenuW.Call(menu, MF_SEPARATOR, 0, 0)
	verName, _ := currentFishVersion()
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

func (a *appState) toggleBossHide() {
	if a.hwnd == 0 {
		return
	}
	vis, _, _ := pIsWindowVisible.Call(a.hwnd)
	if vis != 0 {
		pShowWindow.Call(a.hwnd, SW_HIDE)
	} else {
		pShowWindow.Call(a.hwnd, SW_SHOW)
		pSetForegroundWindow.Call(a.hwnd)
		if a.loggedIn && a.hEditMsg != 0 {
			pSetFocus.Call(a.hEditMsg)
		}
		a.sampleUnderWindow(true)
	}
}

func (a *appState) updateAppearance() {
	saveConfig(a.cfg)
	a.refreshResources()
	a.layoutControls()
	a.triggerRepaint()
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
		}
	case idMenuToggleInvert:
		a.cfg.AutoInvertText = !a.cfg.AutoInvertText
		if a.cfg.AutoInvertText && a.cfg.BgTransparent {
			a.sampleUnderWindow(true)
		}
		a.updateAppearance()
	case idMenuPickBgColor:
		if c, ok := a.pickScreenColor("背景"); ok {
			a.cfg.BgColor = colorrefToHex(c)
			a.cfg.BgTransparent = false
			a.updateAppearance()
		}
	case idMenuToggleTransparent:
		a.cfg.BgTransparent = !a.cfg.BgTransparent
		a.updateAppearance()
		if a.cfg.BgTransparent {
			a.sampleUnderWindow(true)
		}
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
	case idMenuCheckUpdate:
		a.checkUpdateAsync(true)
	case idMenuLogout:
		a.mu.Lock()
		a.loggedIn = false
		a.cfg.Token = ""
		a.cfg.GroupID = ""
		a.messages = nil
		a.seenIDs = nil
		a.lastMsgTs = 0
		saveConfig(a.cfg)
		a.mu.Unlock()
		setCtrlText(a.hLblLoginStatus, "")
		a.layoutControls()
		a.triggerRepaint()
	case idBtnQuit, idMenuQuit:
		pPostMessageW.Call(a.hwnd, WM_CLOSE, 0, 0)
	}
}

func mainWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := gApp
	switch msg {
	case WM_CREATE:
		a.hwnd = hwnd
		a.createAllControls()
		a.refreshResources()
		a.registerBossHotkey()
		a.addTrayIcon()
		a.layoutControls()
		pSetTimer.Call(hwnd, idTimerPoll, 2500, 0)
		pSetTimer.Call(hwnd, idTimerContrast, 260, 0)
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
			a.toggleBossHide()
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

	case WM_LBUTTONDOWN:
		if a.loggedIn && a.hEditMsg != 0 {
			var rc RECT
			pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
			bx := a.inputBoxRect(rc)
			cx := int32(int16(lParam & 0xFFFF))
			cy := int32(int16((lParam >> 16) & 0xFFFF))
			if cx >= bx.Left && cx <= bx.Right && cy >= bx.Top && cy <= bx.Bottom {
				pSetFocus.Call(a.hEditMsg)
				return 0
			}
		}

	case WM_PAINT:
		a.paintWindow()
		return 0

	case WM_MOUSEWHEEL:
		if a.loggedIn {
			delta := int16(wParam >> 16)
			step := a.dpiScale(44)
			a.mu.Lock()
			if delta > 0 {
				a.scrollY -= step
			} else if delta < 0 {
				a.scrollY += step
			}
			if a.scrollY < 0 {
				a.scrollY = 0
			}
			if a.scrollY > a.maxScrollY {
				a.scrollY = a.maxScrollY
			}
			a.atBottom = (a.maxScrollY - a.scrollY) <= a.dpiScale(24)
			a.mu.Unlock()
			a.triggerRepaint()
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
		a.triggerRepaint()
		return 0

	case wmAppUpdateResult:
		a.handleUpdateMessage(wParam)
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
		if a.cfg.BgTransparent {
			txtCol := hexToCOLORREF(a.cfg.FontColor, 0x00283123)
			if a.cfg.AutoInvertText {
				txtCol = invertTextColorForBg(a.inputBgColor)
			}
			pSetTextColor.Call(hdc, uintptr(txtCol))
			pSetBkColor.Call(hdc, uintptr(a.inputBgColor))
			return a.altBrush
		}
		pSetTextColor.Call(hdc, uintptr(a.effectiveTextColor()))
		pSetBkColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)))
		return a.bgBrush

	case WM_HOTKEY:
		if wParam == idHotkeyBoss {
			a.toggleBossHide()
		}
		return 0

	case wmAppShow:
		pShowWindow.Call(hwnd, SW_SHOW)
		pSetForegroundWindow.Call(hwnd)
		if a.loggedIn && a.hEditMsg != 0 {
			pSetFocus.Call(a.hEditMsg)
		}
		a.sampleUnderWindow(true)
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
		pKillTimer.Call(hwnd, idTimerPoll)
		pKillTimer.Call(hwnd, idTimerContrast)
		pDestroyWindow.Call(hwnd)
		if a.hIcon != 0 {
			pDestroyIcon.Call(a.hIcon)
			a.hIcon = 0
		}
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
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
	gApp.hIcon = createFishTrayIcon()
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
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
	gApp.sampleUnderWindow(true)
	pUpdateWindow.Call(hwnd)

	var m MSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		if !(m.Message == WM_KEYDOWN && m.WParam == VK_RETURN) {
			if handled, _, _ := pIsDialogMessageW.Call(hwnd, uintptr(unsafe.Pointer(&m))); handled != 0 {
				continue
			}
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
