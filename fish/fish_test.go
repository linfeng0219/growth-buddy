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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": []map[string]any{
				{"id": "m1", "sender_name": "大宝", "msg_type": "text", "text": "打卡啦"},
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
	msgs, _, err := fetchGroupMessages(ts.URL, lr.Role, lr.Token, grp.ID, 0)
	if err != nil || len(msgs) != 1 || msgs[0].SenderName != "大宝" {
		t.Fatalf("messages mismatch: %+v %v", msgs, err)
	}
	sent, _, err := sendGroupText(ts.URL, lr.Role, lr.Token, grp.ID, "收到")
	if err != nil || sent.Text != "收到" {
		t.Fatalf("send mismatch: %+v %v", sent, err)
	}
}
