package main

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

func TestSocialFriendsAndMultimediaChatFlow(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. 注册两个独立家庭：家庭 A（大宝 + 二宝 + 妈妈）与 家庭 B（小明）
	st, regA := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "阳光家", "displayName": "妈妈", "username": "mom_a", "password": "pass1234",
		"childName": "大宝", "childUsername": "dabao_a", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register family A failed: %d %+v", st, regA)
	}
	parentATok := regA["token"].(string)
	st, _ = doJSON(t, ts, "POST", "/api/parent/children", parentATok, map[string]any{
		"name": "二宝", "username": "erbao_a", "password": "pass1234", "age": 6,
	})
	if st != 201 {
		t.Fatalf("create sibling failed: %d", st)
	}

	st, regB := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "快乐家", "displayName": "爸爸", "username": "dad_b", "password": "pass1234",
		"childName": "小明", "childUsername": "xiaoming_b", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register family B failed: %d %+v", st, regB)
	}

	// 登录三个孩子端账号
	st, loginDa := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "dabao_a", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login dabao failed: %d", st)
	}
	tokDa := loginDa["token"].(string)
	idDa := loginDa["child"].(map[string]any)["id"].(string)

	st, loginEr := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "erbao_a", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login erbao failed: %d", st)
	}
	tokEr := loginEr["token"].(string)

	st, loginXiao := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "xiaoming_b", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login xiaoming failed: %d", st)
	}
	tokXiao := loginXiao["token"].(string)
	idXiao := loginXiao["child"].(map[string]any)["id"].(string)

	// 2. 验证同家庭自动成为「家庭」分组好友，默认也有「朋友」分组
	st, ovDa := doJSON(t, ts, "GET", "/api/child/friends", tokDa, nil)
	if st != 200 {
		t.Fatalf("get friends failed: %d", st)
	}
	friendsDa := ovDa["friends"].([]any)
	hasMom, hasErbao := false, false
	for _, f := range friendsDa {
		m := f.(map[string]any)
		if m["group_name"] != "家庭" {
			t.Fatalf("expected family group, got %+v", m)
		}
		if m["peer_type"] == "parent" && m["name"] == "妈妈" {
			hasMom = true
		}
		if m["peer_type"] == "child" && m["name"] == "二宝" {
			hasErbao = true
		}
	}
	if !hasMom || !hasErbao {
		t.Fatalf("expected mom and erbao in 家庭 group, got %+v", friendsDa)
	}

	// 3. 陌生人未同意前不可发消息
	st, _ = doJSON(t, ts, "POST", "/api/child/chat/messages", tokDa, map[string]any{
		"peerType": "child", "peerId": idXiao, "msgType": "text", "text": "未经同意能发吗",
	})
	if st != 400 {
		t.Fatalf("expected 400 when messaging non-friend, got %d", st)
	}

	// 4. 小明获取自己的分享码 & 二维码载荷，大宝通过二维码载荷查询并发送好友申请
	st, ovXiao := doJSON(t, ts, "GET", "/api/child/friends", tokXiao, nil)
	if st != 200 {
		t.Fatalf("get xiaoming overview failed: %d", st)
	}
	qrPayload := ovXiao["self"].(map[string]any)["qr_payload"].(string)
	if !strings.HasPrefix(qrPayload, "growthbuddy://add-friend?code=") {
		t.Fatalf("unexpected qr payload: %s", qrPayload)
	}

	st, lookupRes := doJSON(t, ts, "GET", "/api/child/friends/lookup?q="+qrPayload, tokDa, nil)
	if st != 200 {
		t.Fatalf("lookup by QR failed: %d %+v", st, lookupRes)
	}
	cand := lookupRes["candidate"].(map[string]any)
	if cand["child_id"] != idXiao || cand["relation_status"] != "none" {
		t.Fatalf("unexpected lookup result: %+v", cand)
	}

	// 用账号或 QR 发送申请都需要对方同意
	st, reqRes := doJSON(t, ts, "POST", "/api/child/friends/request", tokDa, map[string]any{
		"target": "xiaoming_b", "message": "我是大宝，交个朋友吧",
	})
	if st != 200 || reqRes["status"] != "pending" {
		t.Fatalf("expected pending friend request, got %d %+v", st, reqRes)
	}

	// 小明收到待处理申请并同意
	st, ovXiao2 := doJSON(t, ts, "GET", "/api/child/friends", tokXiao, nil)
	if st != 200 {
		t.Fatalf("get xiaoming requests failed: %d", st)
	}
	inc := ovXiao2["incoming_requests"].([]any)
	if len(inc) != 1 {
		t.Fatalf("expected 1 incoming request, got %d", len(inc))
	}
	reqID := inc[0].(map[string]any)["id"].(string)
	st, _ = doJSON(t, ts, "POST", "/api/child/friends/requests/respond", tokXiao, map[string]any{
		"requestId": reqID, "action": "accept",
	})
	if st != 200 {
		t.Fatalf("accept friend request failed: %d", st)
	}

	// 5. 同意后双方均出现在对方的「朋友」分组，并可收发文字、语音、图片
	st, ovDa2 := doJSON(t, ts, "GET", "/api/child/friends", tokDa, nil)
	if st != 200 {
		t.Fatalf("reload dabao friends failed: %d", st)
	}
	foundFriendInGroup := false
	for _, f := range ovDa2["friends"].([]any) {
		m := f.(map[string]any)
		if m["peer_id"] == idXiao && m["group_name"] == "朋友" {
			foundFriendInGroup = true
		}
	}
	if !foundFriendInGroup {
		t.Fatalf("expected xiaoming in 朋友 group")
	}

	fakeImg := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{137, 80, 78, 71, 1, 2, 3, 4})
	fakeVoice := "data:audio/mp4;base64," + base64.StdEncoding.EncodeToString([]byte("FAKE_M4A_BYTES"))

	st, _ = doJSON(t, ts, "POST", "/api/child/chat/messages", tokDa, map[string]any{
		"peerType": "child", "peerId": idXiao, "msgType": "text", "text": "你好呀小明！",
	})
	if st != 201 {
		t.Fatalf("send text failed: %d", st)
	}
	st, imgMsgRes := doJSON(t, ts, "POST", "/api/child/chat/messages", tokDa, map[string]any{
		"peerType": "child", "peerId": idXiao, "msgType": "image", "mediaData": fakeImg,
	})
	if st != 201 {
		t.Fatalf("send image failed: %d", st)
	}
	imgURL := imgMsgRes["message"].(map[string]any)["media_url"].(string)

	st, voiceMsgRes := doJSON(t, ts, "POST", "/api/child/chat/messages", tokXiao, map[string]any{
		"peerType": "child", "peerId": idDa, "msgType": "voice", "mediaData": fakeVoice, "durationSec": 6,
	})
	if st != 201 {
		t.Fatalf("send voice failed: %d", st)
	}
	voiceURL := voiceMsgRes["message"].(map[string]any)["media_url"].(string)

	// 6. 验证媒体隐私隔离：二宝（无关第三方）不可访问大宝与小明的聊天图片/语音
	reqUnauth, _ := http.NewRequest("GET", ts.URL+imgURL, nil)
	reqUnauth.Header.Set("Authorization", "Bearer "+tokEr)
	respUnauth, err := http.DefaultClient.Do(reqUnauth)
	if err != nil {
		t.Fatalf("media unauth request failed: %v", err)
	}
	_ = respUnauth.Body.Close()
	if respUnauth.StatusCode != 403 {
		t.Fatalf("expected 403 for unrelated third party accessing chat media, got %d", respUnauth.StatusCode)
	}

	reqAuth, _ := http.NewRequest("GET", ts.URL+voiceURL, nil)
	reqAuth.Header.Set("Authorization", "Bearer "+tokDa)
	respAuth, err := http.DefaultClient.Do(reqAuth)
	if err != nil {
		t.Fatalf("media auth request failed: %v", err)
	}
	_ = respAuth.Body.Close()
	if respAuth.StatusCode != 200 {
		t.Fatalf("expected 200 for chat participant accessing voice, got %d", respAuth.StatusCode)
	}

	// 7. 验证家长端家庭聊天接口可用
	st, _ = doJSON(t, ts, "POST", "/api/parent/chat/messages", parentATok, map[string]any{
		"childId": idDa, "msgType": "text", "text": "妈妈收到啦，加油！",
	})
	if st != 201 {
		t.Fatalf("parent chat send failed: %d", st)
	}
}
