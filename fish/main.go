//go:build windows

package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const (
	wmAppRefresh = WM_APP + 1
	wmAppShow    = WM_APP + 2

	idTimerPoll  = 1
	idHotkeyBoss = 1

	idBtnLogin        = 101
	idBtnSettings     = 102
	idBtnLogout       = 103
	idBtnSend         = 104
	idBtnApplySize    = 105
	idBtnSizeSmall    = 106
	idBtnSizeMed      = 107
	idBtnSizeLarge    = 108
	idBtnPickBg       = 109
	idBtnBgWarm       = 110
	idBtnBgWhite      = 111
	idBtnBgGreen      = 112
	idBtnBgDark       = 113
	idChkBgTrans      = 114
	idBtnAlpha25      = 115
	idBtnAlpha50      = 116
	idBtnAlpha75      = 117
	idBtnAlpha90      = 118
	idBtnAlpha100     = 119
	idBtnPickFont     = 120
	idBtnFontInk      = 121
	idBtnFontBlack    = 122
	idBtnFontWhite    = 123
	idBtnFontGray     = 124
	idBtnFontSize13   = 125
	idBtnFontSize15   = 126
	idBtnFontSize18   = 127
	idBtnCloseSetting = 128
	idEditMsg         = 129
	idEditPass        = 130
	idBtnHide         = 131
	idBtnHKAltH       = 132
	idBtnHKAltQ       = 133
	idBtnHKCtrlAltH   = 134
	idBtnHKF9         = 135
)

const chromaKeyRef uint32 = 0x00030201 // RGB(1, 2, 3)

var pGetDpiForWindow = user32.NewProc("GetDpiForWindow")

type appState struct {
	mu           sync.Mutex
	hwnd         uintptr
	hInst        uintptr
	cfg          FishConfig
	custColors   [16]uint32
	loggedIn     bool
	showSettings bool
	loggingIn    bool
	polling      bool
	sending      bool
	loginStatus  string
	messages     []ChatMsg
	seenIDs      map[string]bool
	lastMsgTs    int64
	scrollY      int32
	maxScrollY   int32
	atBottom     bool

	uiFont   uintptr
	boldFont uintptr
	chatFont uintptr
	bgBrush  uintptr
	altBrush uintptr

	// Login controls
	hLblTitle, hLblSrv, hEditSrv, hLblUser, hEditUser, hLblPass, hEditPass, hBtnLogin, hLblLoginStatus uintptr

	// Chat controls
	hLblGroup, hBtnHide, hBtnSettings, hBtnLogout, hEditMsg, hBtnSend uintptr

	// Settings controls
	hLblSetHead                                                                                       uintptr
	hLblSize, hEditW, hLblX, hEditH, hBtnApplySize, hBtnSzS, hBtnSzM, hBtnSzL                         uintptr
	hLblBg, hBtnPickBg, hBtnBgWarm, hBtnBgWhite, hBtnBgGreen, hBtnBgDark, hChkBgTrans                 uintptr
	hLblAlpha, hBtnA25, hBtnA50, hBtnA75, hBtnA90, hBtnA100                                           uintptr
	hLblFont, hBtnPickFont, hBtnFInk, hBtnFBlack, hBtnFWhite, hBtnFGray, hBtnFS13, hBtnFS15, hBtnFS18 uintptr
	hLblHotkey, hBtnHKAltH, hBtnHKAltQ, hBtnHKCtrlAltH, hBtnHKF9, hLblHKStatus                        uintptr
	hBtnCloseSet                                                                                      uintptr

	hotkeyStatus string

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
	if a.altBrush != 0 {
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
		// Subtle non-chroma surface so child inputs remain visible & clickable in transparent mode
		altRef := uint32(0x0024201E)
		if !isDarkColor(hexToCOLORREF(a.cfg.FontColor, 0)) {
			altRef = 0x00F4EFE6
		}
		a.altBrush, _, _ = pCreateSolidBrush.Call(uintptr(altRef))
	} else {
		a.bgBrush, _, _ = pCreateSolidBrush.Call(uintptr(bgRef))
		a.altBrush, _, _ = pCreateSolidBrush.Call(uintptr(bgRef))
	}

	allCtrls := []uintptr{
		a.hLblTitle, a.hLblSrv, a.hEditSrv, a.hLblUser, a.hEditUser, a.hLblPass, a.hEditPass, a.hBtnLogin, a.hLblLoginStatus,
		a.hLblGroup, a.hBtnHide, a.hBtnSettings, a.hBtnLogout, a.hEditMsg, a.hBtnSend,
		a.hLblSetHead, a.hLblSize, a.hEditW, a.hLblX, a.hEditH, a.hBtnApplySize, a.hBtnSzS, a.hBtnSzM, a.hBtnSzL,
		a.hLblBg, a.hBtnPickBg, a.hBtnBgWarm, a.hBtnBgWhite, a.hBtnBgGreen, a.hBtnBgDark, a.hChkBgTrans,
		a.hLblAlpha, a.hBtnA25, a.hBtnA50, a.hBtnA75, a.hBtnA90, a.hBtnA100,
		a.hLblFont, a.hBtnPickFont, a.hBtnFInk, a.hBtnFBlack, a.hBtnFWhite, a.hBtnFGray, a.hBtnFS13, a.hBtnFS15, a.hBtnFS18,
		a.hLblHotkey, a.hBtnHKAltH, a.hBtnHKAltQ, a.hBtnHKCtrlAltH, a.hBtnHKF9, a.hLblHKStatus,
		a.hBtnCloseSet,
	}
	for _, h := range allCtrls {
		setFont(h, a.uiFont)
	}
	setFont(a.hLblTitle, a.boldFont)
	setFont(a.hLblGroup, a.boldFont)
	setFont(a.hLblSetHead, a.boldFont)
	setFont(a.hEditMsg, a.chatFont)

	if a.hwnd != 0 {
		flags := uint32(LWA_ALPHA)
		if a.cfg.BgTransparent {
			flags |= LWA_COLORKEY
		}
		pSetLayeredWindowAttrs.Call(a.hwnd, uintptr(chromaKeyRef), uintptr(byte(a.cfg.Alpha)), uintptr(flags))
	}
}

func isDarkColor(c uint32) bool {
	r := int(c & 0xFF)
	g := int((c >> 8) & 0xFF)
	b := int((c >> 16) & 0xFF)
	return (r*299 + g*587 + b*114) < 128000
}

func (a *appState) createAllControls() {
	h := a.hwnd
	inst := a.hInst

	stLbl := uint32(WS_CHILD)
	stEdit := uint32(WS_CHILD | WS_TABSTOP | WS_BORDER | ES_LEFT | ES_AUTOHSCROLL)
	stBtn := uint32(WS_CHILD | WS_TABSTOP | BS_PUSHBUTTON)
	stChk := uint32(WS_CHILD | WS_TABSTOP | BS_AUTOCHECKBOX)

	// Login UI
	a.hLblTitle = createCtrl("STATIC", "🐟 fish", stLbl, 0, 0, 0, 100, 24, h, 0, inst)
	a.hLblSrv = createCtrl("STATIC", "服务器", stLbl, 0, 0, 0, 80, 20, h, 0, inst)
	a.hEditSrv = createCtrl("EDIT", a.cfg.ServerURL, stEdit, 0, 0, 0, 200, 26, h, 0, inst)
	a.hLblUser = createCtrl("STATIC", "账号", stLbl, 0, 0, 0, 80, 20, h, 0, inst)
	a.hEditUser = createCtrl("EDIT", "", stEdit, 0, 0, 0, 200, 26, h, 0, inst)
	a.hLblPass = createCtrl("STATIC", "密码", stLbl, 0, 0, 0, 80, 20, h, 0, inst)
	a.hEditPass = createCtrl("EDIT", "", stEdit|ES_PASSWORD, 0, 0, 0, 200, 26, h, idEditPass, inst)
	a.hBtnLogin = createCtrl("BUTTON", "登录家庭群", stBtn, 0, 0, 0, 120, 32, h, idBtnLogin, inst)
	a.hLblLoginStatus = createCtrl("STATIC", "", stLbl, 0, 0, 0, 240, 36, h, 0, inst)

	// Chat UI
	a.hLblGroup = createCtrl("STATIC", "🐟 家庭群", stLbl, 0, 0, 0, 180, 22, h, 0, inst)
	a.hBtnHide = createCtrl("BUTTON", "隐藏", stBtn, 0, 0, 0, 44, 24, h, idBtnHide, inst)
	a.hBtnSettings = createCtrl("BUTTON", "设置", stBtn, 0, 0, 0, 44, 24, h, idBtnSettings, inst)
	a.hBtnLogout = createCtrl("BUTTON", "退出", stBtn, 0, 0, 0, 44, 24, h, idBtnLogout, inst)
	a.hEditMsg = createCtrl("EDIT", "", stEdit, 0, 0, 0, 200, 28, h, idEditMsg, inst)
	a.hBtnSend = createCtrl("BUTTON", "发送", stBtn, 0, 0, 0, 56, 28, h, idBtnSend, inst)

	// Settings UI
	a.hLblSetHead = createCtrl("STATIC", "外观与窗口设置", stLbl, 0, 0, 0, 180, 22, h, 0, inst)
	a.hLblSize = createCtrl("STATIC", "窗口大小：", stLbl, 0, 0, 0, 74, 22, h, 0, inst)
	a.hEditW = createCtrl("EDIT", strconv.Itoa(a.cfg.Width), stEdit|ES_NUMBER, 0, 0, 0, 56, 24, h, 0, inst)
	a.hLblX = createCtrl("STATIC", "×", stLbl, 0, 0, 0, 14, 22, h, 0, inst)
	a.hEditH = createCtrl("EDIT", strconv.Itoa(a.cfg.Height), stEdit|ES_NUMBER, 0, 0, 0, 56, 24, h, 0, inst)
	a.hBtnApplySize = createCtrl("BUTTON", "应用", stBtn, 0, 0, 0, 48, 24, h, idBtnApplySize, inst)
	a.hBtnSzS = createCtrl("BUTTON", "小 300×380", stBtn, 0, 0, 0, 78, 24, h, idBtnSizeSmall, inst)
	a.hBtnSzM = createCtrl("BUTTON", "中 380×520", stBtn, 0, 0, 0, 78, 24, h, idBtnSizeMed, inst)
	a.hBtnSzL = createCtrl("BUTTON", "大 480×620", stBtn, 0, 0, 0, 78, 24, h, idBtnSizeLarge, inst)

	a.hLblBg = createCtrl("STATIC", "背景颜色：", stLbl, 0, 0, 0, 74, 22, h, 0, inst)
	a.hBtnPickBg = createCtrl("BUTTON", "选色...", stBtn, 0, 0, 0, 58, 24, h, idBtnPickBg, inst)
	a.hBtnBgWarm = createCtrl("BUTTON", "暖米", stBtn, 0, 0, 0, 44, 24, h, idBtnBgWarm, inst)
	a.hBtnBgWhite = createCtrl("BUTTON", "纯白", stBtn, 0, 0, 0, 44, 24, h, idBtnBgWhite, inst)
	a.hBtnBgGreen = createCtrl("BUTTON", "浅绿", stBtn, 0, 0, 0, 44, 24, h, idBtnBgGreen, inst)
	a.hBtnBgDark = createCtrl("BUTTON", "暗夜", stBtn, 0, 0, 0, 44, 24, h, idBtnBgDark, inst)
	a.hChkBgTrans = createCtrl("BUTTON", "背景全透明（仅保留文字）", stChk, 0, 0, 0, 210, 22, h, idChkBgTrans, inst)

	a.hLblAlpha = createCtrl("STATIC", "透明度：", stLbl, 0, 0, 0, 64, 22, h, 0, inst)
	a.hBtnA25 = createCtrl("BUTTON", "25%", stBtn, 0, 0, 0, 40, 24, h, idBtnAlpha25, inst)
	a.hBtnA50 = createCtrl("BUTTON", "50%", stBtn, 0, 0, 0, 40, 24, h, idBtnAlpha50, inst)
	a.hBtnA75 = createCtrl("BUTTON", "75%", stBtn, 0, 0, 0, 40, 24, h, idBtnAlpha75, inst)
	a.hBtnA90 = createCtrl("BUTTON", "90%", stBtn, 0, 0, 0, 40, 24, h, idBtnAlpha90, inst)
	a.hBtnA100 = createCtrl("BUTTON", "100%", stBtn, 0, 0, 0, 46, 24, h, idBtnAlpha100, inst)

	a.hLblFont = createCtrl("STATIC", "字体设置：", stLbl, 0, 0, 0, 74, 22, h, 0, inst)
	a.hBtnPickFont = createCtrl("BUTTON", "选色...", stBtn, 0, 0, 0, 58, 24, h, idBtnPickFont, inst)
	a.hBtnFInk = createCtrl("BUTTON", "深墨", stBtn, 0, 0, 0, 44, 24, h, idBtnFontInk, inst)
	a.hBtnFBlack = createCtrl("BUTTON", "纯黑", stBtn, 0, 0, 0, 44, 24, h, idBtnFontBlack, inst)
	a.hBtnFWhite = createCtrl("BUTTON", "纯白", stBtn, 0, 0, 0, 44, 24, h, idBtnFontWhite, inst)
	a.hBtnFGray = createCtrl("BUTTON", "柔灰", stBtn, 0, 0, 0, 44, 24, h, idBtnFontGray, inst)
	a.hBtnFS13 = createCtrl("BUTTON", "小字", stBtn, 0, 0, 0, 48, 24, h, idBtnFontSize13, inst)
	a.hBtnFS15 = createCtrl("BUTTON", "标准", stBtn, 0, 0, 0, 48, 24, h, idBtnFontSize15, inst)
	a.hBtnFS18 = createCtrl("BUTTON", "大字", stBtn, 0, 0, 0, 48, 24, h, idBtnFontSize18, inst)

	a.hLblHotkey = createCtrl("STATIC", "隐藏热键：", stLbl, 0, 0, 0, 68, 22, h, 0, inst)
	a.hBtnHKAltH = createCtrl("BUTTON", "Alt+H", stBtn, 0, 0, 0, 48, 24, h, idBtnHKAltH, inst)
	a.hBtnHKAltQ = createCtrl("BUTTON", "Alt+Q", stBtn, 0, 0, 0, 48, 24, h, idBtnHKAltQ, inst)
	a.hBtnHKCtrlAltH = createCtrl("BUTTON", "Ctrl+Alt+H", stBtn, 0, 0, 0, 74, 24, h, idBtnHKCtrlAltH, inst)
	a.hBtnHKF9 = createCtrl("BUTTON", "F9", stBtn, 0, 0, 0, 34, 24, h, idBtnHKF9, inst)
	a.hLblHKStatus = createCtrl("STATIC", "", stLbl, 0, 0, 0, 260, 18, h, 0, inst)

	a.hBtnCloseSet = createCtrl("BUTTON", "完成设置，返回群聊", stBtn, 0, 0, 0, 160, 28, h, idBtnCloseSetting, inst)

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
	ch := rc.Bottom - rc.Top
	s := a.dpiScale
	pad := s(12)

	showLogin := !a.loggedIn
	showChat := a.loggedIn && !a.showSettings
	showSet := a.loggedIn && a.showSettings

	for _, h := range []uintptr{a.hLblTitle, a.hLblSrv, a.hEditSrv, a.hLblUser, a.hEditUser, a.hLblPass, a.hEditPass, a.hBtnLogin, a.hLblLoginStatus} {
		showCtrl(h, showLogin)
	}
	for _, h := range []uintptr{a.hLblGroup, a.hBtnHide, a.hBtnSettings, a.hBtnLogout} {
		showCtrl(h, a.loggedIn)
	}
	for _, h := range []uintptr{a.hEditMsg, a.hBtnSend} {
		showCtrl(h, showChat)
	}
	for _, h := range []uintptr{
		a.hLblSetHead, a.hLblSize, a.hEditW, a.hLblX, a.hEditH, a.hBtnApplySize, a.hBtnSzS, a.hBtnSzM, a.hBtnSzL,
		a.hLblBg, a.hBtnPickBg, a.hBtnBgWarm, a.hBtnBgWhite, a.hBtnBgGreen, a.hBtnBgDark, a.hChkBgTrans,
		a.hLblAlpha, a.hBtnA25, a.hBtnA50, a.hBtnA75, a.hBtnA90, a.hBtnA100,
		a.hLblFont, a.hBtnPickFont, a.hBtnFInk, a.hBtnFBlack, a.hBtnFWhite, a.hBtnFGray, a.hBtnFS13, a.hBtnFS15, a.hBtnFS18,
		a.hLblHotkey, a.hBtnHKAltH, a.hBtnHKAltQ, a.hBtnHKCtrlAltH, a.hBtnHKF9, a.hLblHKStatus,
		a.hBtnCloseSet,
	} {
		showCtrl(h, showSet)
	}

	if showLogin {
		fw := cw - pad*2
		if fw > s(280) {
			fw = s(280)
		}
		lx := (cw - fw) / 2
		y := s(28)
		moveCtrl(a.hLblTitle, lx, y, fw, s(26))
		y += s(36)
		moveCtrl(a.hLblSrv, lx, y, fw, s(18))
		y += s(20)
		moveCtrl(a.hEditSrv, lx, y, fw, s(28))
		y += s(36)
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

	// Top bar
	btnW := s(42)
	topH := s(25)
	gap := s(5)
	moveCtrl(a.hBtnLogout, cw-pad-btnW, s(8), btnW, topH)
	moveCtrl(a.hBtnSettings, cw-pad-btnW*2-gap, s(8), btnW, topH)
	moveCtrl(a.hBtnHide, cw-pad-btnW*3-gap*2, s(8), btnW, topH)
	moveCtrl(a.hLblGroup, pad, s(10), cw-pad*2-btnW*3-gap*3, s(22))

	if a.cfg.GroupName != "" {
		setCtrlText(a.hLblGroup, "🐟 "+a.cfg.GroupName)
	}

	if showChat {
		sendW := s(58)
		inH := s(30)
		inY := ch - pad - inH
		moveCtrl(a.hEditMsg, pad, inY, cw-pad*2-sendW-gap, inH)
		moveCtrl(a.hBtnSend, cw-pad-sendW, inY, sendW, inH)
		return
	}

	if showSet {
		x := pad
		y := s(40)
		moveCtrl(a.hLblSetHead, x, y, cw-pad*2, s(20))
		y += s(24)

		moveCtrl(a.hLblSize, x, y+s(2), s(64), s(18))
		moveCtrl(a.hEditW, x+s(66), y, s(48), s(22))
		moveCtrl(a.hLblX, x+s(117), y+s(2), s(12), s(18))
		moveCtrl(a.hEditH, x+s(130), y, s(48), s(22))
		moveCtrl(a.hBtnApplySize, x+s(184), y, s(46), s(22))
		y += s(26)

		moveCtrl(a.hBtnSzS, x, y, s(74), s(22))
		moveCtrl(a.hBtnSzM, x+s(78), y, s(74), s(22))
		moveCtrl(a.hBtnSzL, x+s(156), y, s(74), s(22))
		y += s(28)

		moveCtrl(a.hLblBg, x, y+s(2), s(64), s(18))
		moveCtrl(a.hBtnPickBg, x+s(66), y, s(52), s(22))
		moveCtrl(a.hBtnBgWarm, x+s(122), y, s(40), s(22))
		moveCtrl(a.hBtnBgWhite, x+s(166), y, s(40), s(22))
		y += s(25)
		moveCtrl(a.hBtnBgGreen, x+s(66), y, s(40), s(22))
		moveCtrl(a.hBtnBgDark, x+s(110), y, s(40), s(22))
		y += s(25)
		moveCtrl(a.hChkBgTrans, x, y, cw-pad*2, s(20))
		checked := uintptr(0)
		if a.cfg.BgTransparent {
			checked = 1
		}
		pSendMessageW.Call(a.hChkBgTrans, BM_SETCHECK, checked, 0)
		y += s(25)

		moveCtrl(a.hLblAlpha, x, y+s(2), s(58), s(18))
		moveCtrl(a.hBtnA25, x+s(60), y, s(36), s(22))
		moveCtrl(a.hBtnA50, x+s(99), y, s(36), s(22))
		moveCtrl(a.hBtnA75, x+s(138), y, s(36), s(22))
		moveCtrl(a.hBtnA90, x+s(177), y, s(36), s(22))
		moveCtrl(a.hBtnA100, x+s(216), y, s(42), s(22))
		y += s(28)

		moveCtrl(a.hLblFont, x, y+s(2), s(64), s(18))
		moveCtrl(a.hBtnPickFont, x+s(66), y, s(52), s(22))
		moveCtrl(a.hBtnFInk, x+s(122), y, s(40), s(22))
		moveCtrl(a.hBtnFBlack, x+s(166), y, s(40), s(22))
		y += s(25)
		moveCtrl(a.hBtnFWhite, x+s(66), y, s(40), s(22))
		moveCtrl(a.hBtnFGray, x+s(110), y, s(40), s(22))
		y += s(25)
		moveCtrl(a.hBtnFS13, x+s(66), y, s(46), s(22))
		moveCtrl(a.hBtnFS15, x+s(116), y, s(46), s(22))
		moveCtrl(a.hBtnFS18, x+s(166), y, s(46), s(22))
		y += s(28)

		moveCtrl(a.hLblHotkey, x, y+s(2), s(64), s(18))
		moveCtrl(a.hBtnHKAltH, x+s(66), y, s(46), s(22))
		moveCtrl(a.hBtnHKAltQ, x+s(115), y, s(46), s(22))
		moveCtrl(a.hBtnHKCtrlAltH, x+s(164), y, s(70), s(22))
		moveCtrl(a.hBtnHKF9, x+s(237), y, s(32), s(22))
		y += s(24)
		moveCtrl(a.hLblHKStatus, x, y, cw-pad*2, s(18))
		setCtrlText(a.hLblHKStatus, a.hotkeyStatus)
		y += s(22)

		moveCtrl(a.hBtnCloseSet, x, y, cw-pad*2, s(28))
	}
}

func (a *appState) chatRect(rc RECT) RECT {
	s := a.dpiScale
	pad := s(12)
	return RECT{
		Left:   pad,
		Top:    s(42),
		Right:  rc.Right - pad,
		Bottom: rc.Bottom - pad - s(38),
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
	showSettings := a.showSettings
	msgs := make([]ChatMsg, len(a.messages))
	copy(msgs, a.messages)
	a.mu.Unlock()

	if loggedIn && !showSettings {
		cr := a.chatRect(rc)
		viewW := cr.Right - cr.Left
		viewH := cr.Bottom - cr.Top
		if viewW > 20 && viewH > 20 {
			oldFont, _, _ := pSelectObject.Call(memDC, a.chatFont)
			pSetBkMode.Call(memDC, TRANSPARENT)
			fg := hexToCOLORREF(a.cfg.FontColor, 0x00283123)
			pSetTextColor.Call(memDC, uintptr(fg))

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
	if loggedIn && !showSettings {
		cr := a.chatRect(rc)
		topClip := RECT{Left: 0, Top: 0, Right: cw, Bottom: cr.Top}
		botClip := RECT{Left: 0, Top: cr.Bottom, Right: cw, Bottom: ch}
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&topClip)), a.bgBrush)
		pFillRect.Call(memDC, uintptr(unsafe.Pointer(&botClip)), a.bgBrush)
	}

	pBitBlt.Call(hdc, 0, 0, uintptr(cw), uintptr(ch), memDC, 0, 0, SRCCOPY)
	pSelectObject.Call(memDC, oldBmp)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(memDC)
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

	srv := strings.TrimSpace(getCtrlText(a.hEditSrv))
	if srv == "" {
		srv = defaultServerURL
	}
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
			a.showSettings = false
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
	if !a.loggedIn || a.showSettings {
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

func (a *appState) applyWindowSize(w, h int) {
	if w < 260 {
		w = 260
	}
	if h < 240 {
		h = 240
	}
	sw, _, _ := pGetSystemMetrics.Call(SM_CXSCREEN)
	sh, _, _ := pGetSystemMetrics.Call(SM_CYSCREEN)
	if int32(sw) > 0 && w > int(sw) {
		w = int(sw)
	}
	if int32(sh) > 0 && h > int(sh) {
		h = int(sh)
	}
	a.cfg.Width = w
	a.cfg.Height = h
	saveConfig(a.cfg)
	setCtrlText(a.hEditW, strconv.Itoa(w))
	setCtrlText(a.hEditH, strconv.Itoa(h))
	pSetWindowPos.Call(a.hwnd, 0, 0, 0, uintptr(w), uintptr(h), SWP_NOMOVE|SWP_NOZORDER|SWP_NOACTIVATE)
}

func (a *appState) registerBossHotkey() {
	if a.hwnd == 0 {
		return
	}
	pUnregisterHotKey.Call(a.hwnd, idHotkeyBoss)
	mods, vk, label := parseHotkey(a.cfg.Hotkey)
	a.cfg.Hotkey = label
	r, _, _ := pRegisterHotKey.Call(a.hwnd, idHotkeyBoss, uintptr(mods|MOD_NOREPEAT), uintptr(vk))
	if r != 0 {
		a.hotkeyStatus = "全局快捷键已生效：" + label + "（任意界面隐藏/呼出）"
	} else {
		a.hotkeyStatus = "快捷键 " + label + " 可能被其他软件占用，请换一个"
	}
	if a.hLblHKStatus != 0 {
		setCtrlText(a.hLblHKStatus, a.hotkeyStatus)
	}
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
		if a.loggedIn && !a.showSettings && a.hEditMsg != 0 {
			pSetFocus.Call(a.hEditMsg)
		}
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
	case idBtnHide:
		a.toggleBossHide()
	case idBtnSettings:
		a.showSettings = !a.showSettings
		a.layoutControls()
		a.triggerRepaint()
	case idBtnCloseSetting:
		a.showSettings = false
		a.layoutControls()
		a.triggerRepaint()
	case idBtnLogout:
		a.mu.Lock()
		a.loggedIn = false
		a.showSettings = false
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
	case idBtnSend:
		a.handleSendMessage()
	case idBtnApplySize:
		w, _ := strconv.Atoi(strings.TrimSpace(getCtrlText(a.hEditW)))
		h, _ := strconv.Atoi(strings.TrimSpace(getCtrlText(a.hEditH)))
		a.applyWindowSize(w, h)
	case idBtnSizeSmall:
		a.applyWindowSize(300, 380)
	case idBtnSizeMed:
		a.applyWindowSize(380, 520)
	case idBtnSizeLarge:
		a.applyWindowSize(480, 620)
	case idBtnPickBg:
		if c, ok := pickColorDialog(a.hwnd, hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6), &a.custColors); ok {
			a.cfg.BgColor = colorrefToHex(c)
			a.cfg.BgTransparent = false
			a.updateAppearance()
		}
	case idBtnBgWarm:
		a.cfg.BgColor = "#FAF6EE"
		a.cfg.BgTransparent = false
		a.updateAppearance()
	case idBtnBgWhite:
		a.cfg.BgColor = "#FFFFFF"
		a.cfg.BgTransparent = false
		a.updateAppearance()
	case idBtnBgGreen:
		a.cfg.BgColor = "#E7EFE9"
		a.cfg.BgTransparent = false
		a.updateAppearance()
	case idBtnBgDark:
		a.cfg.BgColor = "#1E2226"
		a.cfg.BgTransparent = false
		a.updateAppearance()
	case idChkBgTrans:
		chk, _, _ := pSendMessageW.Call(a.hChkBgTrans, BM_GETCHECK, 0, 0)
		a.cfg.BgTransparent = chk != 0
		a.updateAppearance()
	case idBtnAlpha25:
		a.cfg.Alpha = 64
		a.updateAppearance()
	case idBtnAlpha50:
		a.cfg.Alpha = 128
		a.updateAppearance()
	case idBtnAlpha75:
		a.cfg.Alpha = 192
		a.updateAppearance()
	case idBtnAlpha90:
		a.cfg.Alpha = 230
		a.updateAppearance()
	case idBtnAlpha100:
		a.cfg.Alpha = 255
		a.updateAppearance()
	case idBtnPickFont:
		if c, ok := pickColorDialog(a.hwnd, hexToCOLORREF(a.cfg.FontColor, 0), &a.custColors); ok {
			a.cfg.FontColor = colorrefToHex(c)
			a.updateAppearance()
		}
	case idBtnFontInk:
		a.cfg.FontColor = "#233128"
		a.updateAppearance()
	case idBtnFontBlack:
		a.cfg.FontColor = "#111111"
		a.updateAppearance()
	case idBtnFontWhite:
		a.cfg.FontColor = "#FFFFFF"
		a.updateAppearance()
	case idBtnFontGray:
		a.cfg.FontColor = "#CCCCCC"
		a.updateAppearance()
	case idBtnFontSize13:
		a.cfg.FontSize = 13
		a.updateAppearance()
	case idBtnFontSize15:
		a.cfg.FontSize = 15
		a.updateAppearance()
	case idBtnFontSize18:
		a.cfg.FontSize = 18
		a.updateAppearance()
	case idBtnHKAltH:
		a.cfg.Hotkey = "Alt+H"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idBtnHKAltQ:
		a.cfg.Hotkey = "Alt+Q"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idBtnHKCtrlAltH:
		a.cfg.Hotkey = "Ctrl+Alt+H"
		saveConfig(a.cfg)
		a.registerBossHotkey()
	case idBtnHKF9:
		a.cfg.Hotkey = "F9"
		saveConfig(a.cfg)
		a.registerBossHotkey()
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
		a.layoutControls()
		pSetTimer.Call(hwnd, idTimerPoll, 2500, 0)
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
			if a.showSettings {
				setCtrlText(a.hEditW, strconv.Itoa(a.cfg.Width))
				setCtrlText(a.hEditH, strconv.Itoa(a.cfg.Height))
			}
		}
		a.triggerRepaint()
		return 0

	case WM_MOVE:
		var wr RECT
		if pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); wr.Left > -10000 && wr.Top > -10000 {
			a.cfg.X = int(wr.Left)
			a.cfg.Y = int(wr.Top)
			a.cfg.HasPos = true
		}
		return 0

	case WM_ERASEBKGND:
		return 1

	case WM_PAINT:
		a.paintWindow()
		return 0

	case WM_MOUSEWHEEL:
		if a.loggedIn && !a.showSettings {
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

	case WM_COMMAND:
		id := int(wParam & 0xFFFF)
		a.handleCommand(id)
		return 0

	case WM_CTLCOLORSTATIC, WM_CTLCOLORBTN:
		hdc := wParam
		pSetTextColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.FontColor, 0x00283123)))
		if a.cfg.BgTransparent {
			pSetBkColor.Call(hdc, uintptr(chromaKeyRef))
			return a.bgBrush
		}
		pSetBkColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.BgColor, 0x00EEFAF6)))
		return a.bgBrush

	case WM_CTLCOLOREDIT:
		hdc := wParam
		if a.cfg.BgTransparent {
			isDark := !isDarkColor(hexToCOLORREF(a.cfg.FontColor, 0))
			if isDark {
				pSetTextColor.Call(hdc, 0x00FFFFFF)
				pSetBkColor.Call(hdc, 0x0024201E)
			} else {
				pSetTextColor.Call(hdc, 0x00233128)
				pSetBkColor.Call(hdc, 0x00F4EFE6)
			}
			return a.altBrush
		}
		pSetTextColor.Call(hdc, uintptr(hexToCOLORREF(a.cfg.FontColor, 0x00283123)))
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
		if a.loggedIn && !a.showSettings && a.hEditMsg != 0 {
			pSetFocus.Call(a.hEditMsg)
		}
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
		pUnregisterHotKey.Call(hwnd, idHotkeyBoss)
		pKillTimer.Call(hwnd, idTimerPoll)
		pDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func main() {
	runtime.LockOSThread()
	pSetProcessDPIAware.Call()

	cfg := loadConfig()
	gApp = &appState{
		cfg:      cfg,
		hInst:    getModuleHandle(),
		loggedIn: cfg.Token != "" && cfg.GroupID != "",
		atBottom: true,
		seenIDs:  make(map[string]bool),
	}
	for i := range gApp.custColors {
		gApp.custColors[i] = 0x00FFFFFF
	}

	clsName := u16("GrowthBuddyFishWnd")
	if prev, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(clsName)), 0); prev != 0 {
		pPostMessageW.Call(prev, wmAppShow, 0, 0)
		return
	}
	cur, _, _ := pLoadCursorW.Call(0, IDC_ARROW)
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(mainWndProc),
		HInstance:     gApp.hInst,
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
		WS_EX_LAYERED,
		uintptr(unsafe.Pointer(clsName)),
		uintptr(unsafe.Pointer(u16("fish"))),
		WS_OVERLAPPEDWINDOW|WS_CLIPCHILDREN,
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
