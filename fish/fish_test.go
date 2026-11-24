package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
