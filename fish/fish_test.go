package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unicode/utf16"
)

func TestFishVersionAndOTAUpdate(t *testing.T) {
	vName, vCode := currentFishVersion()
	if vName == "" || vCode <= 0 {
		t.Fatalf("expected non-empty fish version, got %q (%d)", vName, vCode)
	}
	if got := autoFishVersionCode("2026.11.24-1620"); got != 611241620 {
		t.Fatalf("unexpected autoFishVersionCode: %d", got)
	}

	payload := []byte("MZ-fake-new-fish-binary")
	sum := sha256.Sum256(payload)
	expectedSHA := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/api/app/version", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("platform") != "fish" {
			t.Fatalf("expected platform=fish, got %s", r.URL.Query().Get("platform"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"hasRelease":  true,
			"hasUpdate":   true,
			"forceUpdate": true,
			"release": map[string]any{
				"id":            "rel_fish_1",
				"platform":      "fish",
				"version_code":  999999999,
				"version_name":  "2099.01.01-0000",
				"release_notes": "新版上线",
				"force_update":  true,
				"sha256":        expectedSHA,
				"download_url":  "/api/app/download/rel_fish_1.exe",
			},
		})
	})
	mux.HandleFunc("/api/app/download/rel_fish_1.exe", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	verRes, err := checkFishUpdate(ts.URL, vCode)
	if err != nil || !verRes.HasUpdate || verRes.Release == nil || verRes.Release.VersionName != "2099.01.01-0000" {
		t.Fatalf("checkFishUpdate mismatch: %+v err=%v", verRes, err)
	}

	dest := filepath.Join(t.TempDir(), "fish.exe.new")
	if err := downloadFishUpdateToFile(ts.URL, *verRes.Release, dest); err != nil {
		t.Fatalf("downloadFishUpdateToFile failed: %v", err)
	}
	gotBytes, err := os.ReadFile(dest)
	if err != nil || string(gotBytes) != string(payload) {
		t.Fatalf("downloaded bytes mismatch: %v %q", err, string(gotBytes))
	}

	// Bad SHA should be rejected
	badRel := *verRes.Release
	badRel.SHA256 = "0000000000000000"
	badDest := filepath.Join(t.TempDir(), "fish-bad.exe.new")
	if err := downloadFishUpdateToFile(ts.URL, badRel, badDest); err == nil {
		t.Fatalf("expected SHA mismatch error, got nil")
	}
}

func TestColorAndConfigHelpers(t *testing.T) {
	if c := hexToCOLORREF("#112233", 0); c != 0x00332211 {
		t.Fatalf("unexpected COLORREF: 0x%08x", c)
	}
	if s := colorrefToHex(0x00332211); s != "#112233" {
		t.Fatalf("unexpected hex: %s", s)
	}
	if !isHexColor("#FAF6EE") || isHexColor("#ZZZZZZ") || isHexColor("123456") {
		t.Fatalf("hex validation mismatch")
	}
	if m, v, c := parseHotkey("ctrl+alt+h"); m != 0x0003 || v != 0x48 || c != "Ctrl+Alt+H" {
		t.Fatalf("parseHotkey ctrl+alt+h mismatch: %x %x %s", m, v, c)
	}
	if m, v, c := parseHotkey("unknown"); m != 0x0001 || v != 0x48 || c != "Alt+H" {
		t.Fatalf("parseHotkey default mismatch: %x %x %s", m, v, c)
	}
}

func TestAutoInvertTextColorAndStripAverage(t *testing.T) {
	if got := invertTextColorForBg(0x00000000); got != 0x00FFFFFF {
		t.Fatalf("expected white on black, got 0x%08x", got)
	}
	if got := invertTextColorForBg(0x00FFFFFF); got != 0x00000000 {
		t.Fatalf("expected black on white, got 0x%08x", got)
	}
	// 滞回稳定测试：轻微亮度波动(126 <-> 130)保持当前明暗极性，不反复黑白闪烁
	fg1, ref1 := invertTextColorStable(0x007E7E7E, 0, 0, false) // lum ~126 -> 0x00FFFFFF
	if fg1 != 0x00FFFFFF {
		t.Fatalf("expected white on 126 gray, got 0x%08x", fg1)
	}
	fg2, _ := invertTextColorStable(0x00828282, ref1, fg1, true) // lum ~130 -> 仍在死区/滞回区间内，维持白色
	if fg2 != 0x00FFFFFF {
		t.Fatalf("expected stable white across mid-gray ripple 126->130, got 0x%08x", fg2)
	}
	// 真正切到浅色背景 (>152) 才切换为深色文字
	fg3, ref3 := invertTextColorStable(0x00B8B8B8, ref1, fg2, true)
	if colorLuminance(fg3) >= 128 {
		t.Fatalf("expected dark text on bright gray bg, got 0x%08x", fg3)
	}
	// 从浅色回退到中灰 (130) 时维持深色，不再抖回白色
	fg4, _ := invertTextColorStable(0x007E7E7E, ref3, fg3, true)
	if colorLuminance(fg4) >= 128 {
		t.Fatalf("expected dark text to hold inside hysteresis band, got 0x%08x", fg4)
	}
	if d := colorChannelMaxDiff(0x00102030, 0x0015223E); d != 14 {
		t.Fatalf("expected max channel diff 14, got %d", d)
	}
	// Mid-gray (#808080) should avoid gray-on-gray and produce high contrast
	grayInv := invertTextColorForBg(0x00808080)
	if absInt(colorLuminance(0x00808080)-colorLuminance(grayInv)) < 85 {
		t.Fatalf("expected high contrast on mid-gray, got 0x%08x", grayInv)
	}
	// Inversion should never produce exact chromaKeyRef
	almostChromaBg := uint32(0x00FCFDFE) // 255-254=1(R), 255-253=2(G), 255-252=3(B) -> chromaKeyRef
	if got := invertTextColorForBg(almostChromaBg); got == chromaKeyRef {
		t.Fatalf("invertTextColorForBg must avoid chromaKeyRef")
	}
	if got := brightenColor(0x001E1E1E, 28); got != 0x003A3A3A {
		t.Fatalf("expected brightened dark color 0x003A3A3A, got 0x%08x", got)
	}
	if got := brightenColor(0x00FFFFFF, 28); got != 0x00FFFFFF {
		t.Fatalf("expected white clamped to white, got 0x%08x", got)
	}
	btnBg := darkenColorForButton(0x00EEFAF6, false)
	if !isDarkColor(btnBg) || btnBg == chromaKeyRef {
		t.Fatalf("expected dark button color, got 0x%08x", btnBg)
	}
	if pressedBg := darkenColorForButton(0x00EEFAF6, true); colorLuminance(pressedBg) > colorLuminance(btnBg) {
		t.Fatalf("expected pressed button color darker or equal, got %d > %d", colorLuminance(pressedBg), colorLuminance(btnBg))
	}
	for _, validLim := range []int{0, 100, 500} {
		if cfg := normalizeMaxMessages(validLim); cfg != validLim {
			t.Fatalf("expected %d preserved, got %d", validLim, cfg)
		}
	}
	if cfg := normalizeMaxMessages(999); cfg != 100 {
		t.Fatalf("expected invalid maxMessages normalized to 100, got %d", cfg)
	}
	def := defaultConfig()
	if !def.BgTransparent || !def.AutoInvertText {
		t.Fatalf("expected defaultConfig BgTransparent=true and AutoInvertText=true, got %+v", def)
	}

	// Build 10x10 top-down BGRA image: top half pure red (0x00FF0000), bottom half dark navy (0x00001020)
	pix := make([]uint32, 10*10)
	for y := 0; y < 5; y++ {
		for x := 0; x < 10; x++ {
			pix[y*10+x] = 0xFFFF0000
		}
	}
	for y := 5; y < 10; y++ {
		for x := 0; x < 10; x++ {
			pix[y*10+x] = 0xFF001020
		}
	}
	topCol, ok := averageBGRAStripColor(pix, 10, 10, 0, 5)
	if !ok || topCol != 0x000000FF { // COLORREF 0x00BBGGRR for R=255
		t.Fatalf("unexpected top strip color: %v 0x%08x", ok, topCol)
	}
	botCol, ok := averageBGRAStripColor(pix, 10, 10, 5, 10)
	if !ok || botCol != 0x00201000 { // R=0,G=16,B=32 -> 0x00201000
		t.Fatalf("unexpected bottom strip color: %v 0x%08x", ok, botCol)
	}
}

func TestFishAPIFlow(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"role": "parent", "token": "tok_p",
			"parent": map[string]any{"id": "par_1", "display_name": "妈妈"},
		})
	})
	mux.HandleFunc("/api/parent/friends", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"chat_groups": []map[string]any{
				{"id": "grp_other", "name": "其他群", "is_family_default": false},
				{"id": "grp_fam", "name": "阳光家", "is_family_default": true},
			},
		})
	})
	mux.HandleFunc("/api/parent/chat/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"message": map[string]any{"id": "m2", "sender_name": "妈妈", "msg_type": "text", "text": "收到"},
			})
			return
		}
		gotLim := r.URL.Query().Get("limit")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]any{
				{"id": "m1", "sender_name": "大宝", "msg_type": "text", "text": "打卡啦:" + gotLim},
			},
		})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	lr, err := doLogin(ts.URL, "mom", "pass")
	if err != nil || lr.Role != "parent" || lr.Token != "tok_p" || lr.DisplayName != "妈妈" {
		t.Fatalf("login mismatch: %+v %v", lr, err)
	}
	grp, code, err := fetchDefaultFamilyGroup(ts.URL, lr.Role, lr.Token)
	if err != nil || code != 200 || grp.ID != "grp_fam" || grp.Name != "阳光家" {
		t.Fatalf("group mismatch: %+v %d %v", grp, code, err)
	}
	msgs, _, err := fetchGroupMessages(ts.URL, lr.Role, lr.Token, grp.ID, 0, 500)
	if err != nil || len(msgs) != 1 || msgs[0].SenderName != "大宝" || msgs[0].Text != "打卡啦:500" {
		t.Fatalf("messages mismatch: %+v %v", msgs, err)
	}
	msgsAll, _, err := fetchGroupMessages(ts.URL, lr.Role, lr.Token, grp.ID, 0, 0)
	if err != nil || len(msgsAll) != 1 || msgsAll[0].Text != "打卡啦:all" {
		t.Fatalf("messages unlimited mismatch: %+v %v", msgsAll, err)
	}
	sent, _, err := sendGroupText(ts.URL, lr.Role, lr.Token, grp.ID, "收到")
	if err != nil || sent.Text != "收到" {
		t.Fatalf("send mismatch: %+v %v", sent, err)
	}
}

func TestConversationGroupingTimestampAndTrayTip(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 11, 24, 17, 30, 0, 0, loc)
	t1 := now.Add(-10 * time.Minute).UnixMilli()
	t2Same := t1 + (2*60+59)*1000 // < 3 分钟：归为同一次对话
	t3New := t2Same + 3*60*1000   // == 3 分钟：新一次对话

	if !shouldShowConversationTimestamp(0, t1, true) {
		t.Fatalf("expected first message to show timestamp")
	}
	if shouldShowConversationTimestamp(t1, t2Same, false) {
		t.Fatalf("expected message within 3 minutes to share previous conversation without timestamp")
	}
	if !shouldShowConversationTimestamp(t2Same, t3New, false) {
		t.Fatalf("expected message >= 3 minutes apart to start a new conversation timestamp")
	}
	if got := formatConversationTime(now.UnixMilli(), now); got != "17:30" {
		t.Fatalf("expected today format 17:30, got %q", got)
	}
	yest := now.AddDate(0, 0, -1).UnixMilli()
	if got := formatConversationTime(yest, now); got != "昨天 17:30" {
		t.Fatalf("expected yesterday format 昨天 17:30, got %q", got)
	}
	if got := formatTrayUnreadTip(0); got != "未读信息0条" {
		t.Fatalf("unexpected tray tip for 0: %q", got)
	}
	if got := formatTrayUnreadTip(5); got != "未读信息5条" {
		t.Fatalf("unexpected tray tip for 5: %q", got)
	}
}

func TestSavedAccountsManagement(t *testing.T) {
	var cfg FishConfig
	cfg.upsertSavedAccount(SavedAccount{
		Username:    "mom",
		Role:        "parent",
		ActorID:     "u_mom",
		DisplayName: "妈妈",
		Token:       "tok_mom_1",
		GroupID:     "grp_1",
		GroupName:   "阳光家",
		LastLoginAt: 1000,
	})
	cfg.upsertSavedAccount(SavedAccount{
		Username:    "kid1",
		Role:        "child",
		ActorID:     "u_kid1",
		DisplayName: "大宝",
		Token:       "tok_kid_1",
		GroupID:     "grp_1",
		GroupName:   "阳光家",
		LastLoginAt: 2000,
	})
	if len(cfg.SavedAccounts) != 2 || cfg.SavedAccounts[0].ActorID != "u_kid1" {
		t.Fatalf("expected kid1 at front, got %+v", cfg.SavedAccounts)
	}

	// Switch back / update mom without overwriting username
	cfg.upsertSavedAccount(SavedAccount{
		Role:        "parent",
		ActorID:     "u_mom",
		DisplayName: "妈妈",
		Token:       "tok_mom_2",
		GroupID:     "grp_1",
		GroupName:   "阳光家",
		LastLoginAt: 3000,
	})
	if len(cfg.SavedAccounts) != 2 || cfg.SavedAccounts[0].ActorID != "u_mom" || cfg.SavedAccounts[0].Username != "mom" || cfg.SavedAccounts[0].Token != "tok_mom_2" {
		t.Fatalf("expected mom updated cleanly at front, got %+v", cfg.SavedAccounts[0])
	}
	if cfg.SavedAccounts[1].Token != "tok_kid_1" {
		t.Fatalf("expected kid1 token preserved, got %q", cfg.SavedAccounts[1].Token)
	}

	cfg.Token = "tok_mom_2"
	cfg.Role = "parent"
	cfg.ActorID = "u_mom"
	cfg.Username = "mom"
	if !cfg.isActiveSavedAccount(cfg.SavedAccounts[0]) || cfg.isActiveSavedAccount(cfg.SavedAccounts[1]) {
		t.Fatalf("isActiveSavedAccount mismatch")
	}

	cfg.clearSavedAccountToken("parent", "u_mom", "mom")
	if cfg.SavedAccounts[0].Token != "" || cfg.SavedAccounts[1].Token != "tok_kid_1" {
		t.Fatalf("expected only mom token cleared, got %+v", cfg.SavedAccounts)
	}
	if got := savedAccountLabel(cfg.SavedAccounts[0]); got != "妈妈 (家长 · mom · 需登录)" {
		t.Fatalf("unexpected cleared label: %q", got)
	}
	if got := savedAccountLabel(cfg.SavedAccounts[1]); got != "大宝 (孩子 · kid1)" {
		t.Fatalf("unexpected active label: %q", got)
	}
}

func TestChatTextSelectionAndExtraction(t *testing.T) {
	l0 := visualLine{
		paraIdx: 0,
		u16:     utf16.Encode([]rune("妈妈: 你")),
		charX:   []int32{0, 14, 28, 34, 40, 54},
		y:       0,
		h:       20,
	}
	l1 := visualLine{
		paraIdx: 0,
		u16:     utf16.Encode([]rune("好呀")),
		charX:   []int32{0, 14, 28},
		y:       20,
		h:       20,
	}
	l2 := visualLine{
		paraIdx: 1,
		u16:     utf16.Encode([]rune("大宝: 🙂棒")),
		charX:   []int32{0, 14, 28, 34, 40, 54, 54, 68}, // surrogate pair shares x advance at boundary
		y:       48,
		h:       20,
	}
	lines := []visualLine{l0, l1, l2}

	// Hit test midpoint of '好' on line 1 (y=20..40, crTop=10, scrollY=0 -> clientY=35, x near 7+crLeft(8))
	pos := hitTestChatLines(lines, 0, 8, 10, 8+13, 10+25)
	if pos != (textPos{line: 1, col: 1}) {
		t.Fatalf("expected hit (1,1), got %+v", pos)
	}

	// Surrogate pair boundary check: should snap cleanly
	posEmoji := hitTestChatLines(lines, 0, 8, 10, 8+45, 10+52)
	if posEmoji.line != 2 || isLowSurrogate(lines[2].u16[posEmoji.col]) {
		t.Fatalf("hit test landed inside surrogate pair: %+v", posEmoji)
	}

	// Double-click paragraph selection on wrapped line 1 should cover whole paragraph 0
	pLo, pHi, ok := paragraphBoundsAtLine(lines, 1)
	if !ok || pLo != (textPos{line: 0, col: 0}) || pHi != (textPos{line: 1, col: len(l1.u16)}) {
		t.Fatalf("paragraphBoundsAtLine mismatch: lo=%+v hi=%+v ok=%v", pLo, pHi, ok)
	}
	if got := utf16ToString(extractSelectedUTF16(lines, pLo, pHi)); got != "妈妈: 你好呀" {
		t.Fatalf("expected wrapped paragraph join without newline, got %q", got)
	}

	// Reverse selection across paragraphs adds \r\n between paragraph 0 and paragraph 1
	lo, hi, hasSel := normalizedSelection(textPos{line: 2, col: len(l2.u16)}, textPos{line: 0, col: 4})
	if !hasSel {
		t.Fatalf("expected valid reverse selection")
	}
	if got := utf16ToString(extractSelectedUTF16(lines, lo, hi)); got != "你好呀\r\n大宝: 🙂棒" {
		t.Fatalf("expected cross-paragraph selection, got %q", got)
	}
}

func TestClearHotkeyFilteringAndAdaptiveInputBorder(t *testing.T) {
	if m, vk, canon := parseClearHotkey(""); m != 0x0001 || vk != 0x4C || canon != "Alt+L" {
		t.Fatalf("default parseClearHotkey mismatch: %x %x %s", m, vk, canon)
	}
	if m, vk, canon := parseClearHotkey("ctrl+alt+l"); m != 0x0003 || vk != 0x4C || canon != "Ctrl+Alt+L" {
		t.Fatalf("ctrl+alt+l parseClearHotkey mismatch: %x %x %s", m, vk, canon)
	}
	if m, vk, canon := parseClearHotkey("f8"); m != 0 || vk != 0x77 || canon != "F8" {
		t.Fatalf("f8 parseClearHotkey mismatch: %x %x %s", m, vk, canon)
	}

	msgs := []ChatMsg{
		{ID: "m1", CreatedAt: 1000, Text: "old-1"},
		{ID: "m2", CreatedAt: 2000, Text: "old-at-cutoff"},
		{ID: "m3", CreatedAt: 2001, Text: "after-clear"},
	}
	filtered := filterMessagesAfterClear(msgs, 2000)
	if len(filtered) != 1 || filtered[0].ID != "m3" {
		t.Fatalf("expected only m3 after clear cutoff 2000, got %+v", filtered)
	}
	if all := filterMessagesAfterClear(msgs, 0); len(all) != 3 {
		t.Fatalf("expected all messages when clearBeforeTs=0, got %d", len(all))
	}

	// 白色背景下，边框颜色必须比纯白深且肉眼可见（不能钳位成白色）
	whiteBorder := adaptiveInputBorderColor(0x00FFFFFF)
	if whiteBorder == 0x00FFFFFF || whiteBorder == chromaKeyRef {
		t.Fatalf("white background border must be visible, got 0x%08x", whiteBorder)
	}
	if colorLuminance(whiteBorder) > 215 {
		t.Fatalf("expected darkened border luminance <= 215 on pure white, got %d (0x%08x)", colorLuminance(whiteBorder), whiteBorder)
	}

	// 深色背景下，边框颜色应适当提亮
	darkBg := uint32(0x00202020)
	darkBorder := adaptiveInputBorderColor(darkBg)
	if darkBorder == chromaKeyRef || colorLuminance(darkBorder) <= colorLuminance(darkBg)+25 {
		t.Fatalf("expected brightened border on dark bg, got 0x%08x", darkBorder)
	}
}
