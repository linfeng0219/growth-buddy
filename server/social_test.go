package main

import (
	"encoding/base64"
	"net/http"
	"net/url"
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
	idDa := loginDa["childId"].(string)

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
	idXiao := loginXiao["childId"].(string)

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

	st, lookupRes := doJSON(t, ts, "GET", "/api/child/friends/lookup?q="+url.QueryEscape(qrPayload), tokDa, nil)
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

func TestGroupChatAndFamilyDefaultGroupFlow(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. 注册两个家庭：阳光家（妈妈+大宝+二宝）、快乐家（爸爸+小明）
	st, regA := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "阳光家", "displayName": "妈妈", "username": "mom_grp", "password": "pass1234",
		"childName": "大宝", "childUsername": "da_grp", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register A failed: %d", st)
	}
	momTok := regA["token"].(string)

	st, erCreated := doJSON(t, ts, "POST", "/api/parent/children", momTok, map[string]any{
		"name": "二宝", "username": "er_grp", "password": "pass1234", "age": 7,
	})
	if st != 201 {
		t.Fatalf("create sibling failed: %d", st)
	}
	idEr := erCreated["child"].(map[string]any)["id"].(string)

	st, regB := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "快乐家", "displayName": "爸爸", "username": "dad_grp", "password": "pass1234",
		"childName": "小明", "childUsername": "xiao_grp", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register B failed: %d %+v", st, regB)
	}

	st, lDa := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "da_grp", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login da failed: %d", st)
	}
	tokDa, idDa := lDa["token"].(string), lDa["childId"].(string)

	st, lEr := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "er_grp", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login er failed: %d", st)
	}
	tokEr := lEr["token"].(string)

	st, lXiao := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "xiao_grp", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login xiao failed: %d", st)
	}
	tokXiao, idXiao := lXiao["token"].(string), lXiao["childId"].(string)

	// 2. 验证同家庭自动成为一个默认群，且群名为家庭名
	st, ovDa := doJSON(t, ts, "GET", "/api/child/friends", tokDa, nil)
	if st != 200 {
		t.Fatalf("child overview failed: %d", st)
	}
	groupsDa := ovDa["chat_groups"].([]any)
	if len(groupsDa) != 1 {
		t.Fatalf("expected 1 default family group, got %d", len(groupsDa))
	}
	famGrp := groupsDa[0].(map[string]any)
	famGrpID := famGrp["id"].(string)
	if famGrp["name"] != "阳光家" || famGrp["is_family_default"] != true {
		t.Fatalf("unexpected default family group: %+v", famGrp)
	}
	if int(famGrp["member_count"].(float64)) != 3 {
		t.Fatalf("expected 3 members in family A group (mom+da+er), got %v", famGrp["member_count"])
	}

	// 家长端也能看到同一个家庭默认群
	st, pContacts := doJSON(t, ts, "GET", "/api/parent/chat/contacts", momTok, nil)
	if st != 200 {
		t.Fatalf("parent contacts failed: %d", st)
	}
	pGroups := pContacts["groups"].([]any)
	if len(pGroups) != 1 || pGroups[0].(map[string]any)["id"] != famGrpID {
		t.Fatalf("expected parent to see family group %s, got %+v", famGrpID, pGroups)
	}

	// 3. 妈妈在家庭群发消息、大宝发图片，二宝可见未读并读取发送者名称，家庭外的小明无权访问
	st, _ = doJSON(t, ts, "POST", "/api/parent/chat/messages", momTok, map[string]any{
		"peerType": "group", "peerId": famGrpID, "msgType": "text", "text": "今晚一起跳绳打卡！",
	})
	if st != 201 {
		t.Fatalf("parent send group msg failed: %d", st)
	}
	imgData := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1, 2, 3})
	st, grpImgRes := doJSON(t, ts, "POST", "/api/child/chat/messages", tokDa, map[string]any{
		"peerType": "group", "peerId": famGrpID, "msgType": "image", "mediaData": imgData,
	})
	if st != 201 {
		t.Fatalf("child send group image failed: %d", st)
	}
	grpImgURL := grpImgRes["message"].(map[string]any)["media_url"].(string)

	// 二宝查看概览应有 2 条群聊未读
	st, ovEr := doJSON(t, ts, "GET", "/api/child/friends", tokEr, nil)
	if st != 200 {
		t.Fatalf("er overview failed: %d", st)
	}
	erFamGrp := ovEr["chat_groups"].([]any)[0].(map[string]any)
	if int(erFamGrp["unread_count"].(float64)) != 2 {
		t.Fatalf("expected 2 unread group msgs for er, got %v", erFamGrp["unread_count"])
	}

	// 二宝拉取群消息应带 sender_name
	st, gMsgsRes := doJSON(t, ts, "GET", "/api/child/chat/messages?peerType=group&peerId="+url.QueryEscape(famGrpID), tokEr, nil)
	if st != 200 {
		t.Fatalf("list group msgs failed: %d", st)
	}
	gMsgs := gMsgsRes["messages"].([]any)
	if len(gMsgs) != 2 || gMsgs[0].(map[string]any)["sender_name"] != "妈妈" || gMsgs[1].(map[string]any)["sender_name"] != "大宝" {
		t.Fatalf("unexpected group messages/senders: %+v", gMsgs)
	}

	// 小明（非家庭成员）不能查看或发送家庭群消息、也不能访问群图片
	st, _ = doJSON(t, ts, "GET", "/api/child/chat/messages?peerType=group&peerId="+url.QueryEscape(famGrpID), tokXiao, nil)
	if st != 400 {
		t.Fatalf("expected 400 for outsider reading family group, got %d", st)
	}
	reqOutsider, _ := http.NewRequest("GET", ts.URL+grpImgURL, nil)
	reqOutsider.Header.Set("Authorization", "Bearer "+tokXiao)
	respOutsider, _ := http.DefaultClient.Do(reqOutsider)
	_ = respOutsider.Body.Close()
	if respOutsider.StatusCode != 403 {
		t.Fatalf("expected 403 for outsider fetching group media, got %d", respOutsider.StatusCode)
	}

	// 默认家庭群不可退出
	st, _ = doJSON(t, ts, "POST", "/api/child/groups/"+famGrpID+"/leave", tokDa, nil)
	if st != 400 {
		t.Fatalf("expected 400 leaving default family group, got %d", st)
	}

	// 4. 大宝与小明加为好友后自建群聊，并测试群内通信与退群
	st, _ = doJSON(t, ts, "POST", "/api/child/friends/request", tokDa, map[string]any{"target": "xiao_grp", "message": "一起建群"})
	if st != 200 {
		t.Fatalf("friend req failed: %d", st)
	}
	st, ovXiao := doJSON(t, ts, "GET", "/api/child/friends", tokXiao, nil)
	reqID := ovXiao["incoming_requests"].([]any)[0].(map[string]any)["id"].(string)
	st, _ = doJSON(t, ts, "POST", "/api/child/friends/requests/respond", tokXiao, map[string]any{"requestId": reqID, "action": "accept"})
	if st != 200 {
		t.Fatalf("accept friend failed: %d", st)
	}

	st, createGrpRes := doJSON(t, ts, "POST", "/api/child/groups", tokDa, map[string]any{
		"name": "运动小分队",
		"members": []map[string]any{
			{"memberType": "child", "memberId": idEr},
			{"memberType": "child", "memberId": idXiao},
		},
	})
	if st != 201 {
		t.Fatalf("create custom group failed: %d %+v", st, createGrpRes)
	}
	customGrp := createGrpRes["group"].(map[string]any)
	customGrpID := customGrp["id"].(string)
	if customGrp["name"] != "运动小分队" || int(customGrp["member_count"].(float64)) != 3 {
		t.Fatalf("unexpected custom group: %+v", customGrp)
	}

	// 小明发群消息，大宝查看详情与消息
	st, _ = doJSON(t, ts, "POST", "/api/child/chat/messages", tokXiao, map[string]any{
		"peerType": "group", "peerId": customGrpID, "msgType": "text", "text": "大家好，我加入啦！",
	})
	if st != 201 {
		t.Fatalf("xiao send custom group msg failed: %d", st)
	}
	st, detailRes := doJSON(t, ts, "GET", "/api/child/groups/"+customGrpID, tokDa, nil)
	if st != 200 || len(detailRes["group"].(map[string]any)["members"].([]any)) != 3 {
		t.Fatalf("unexpected group detail: %d %+v", st, detailRes)
	}

	// 小明退出自定义群聊后不可再发消息
	st, _ = doJSON(t, ts, "POST", "/api/child/groups/"+customGrpID+"/leave", tokXiao, nil)
	if st != 200 {
		t.Fatalf("xiao leave custom group failed: %d", st)
	}
	st, _ = doJSON(t, ts, "POST", "/api/child/chat/messages", tokXiao, map[string]any{
		"peerType": "group", "peerId": customGrpID, "msgType": "text", "text": "退群后还能发吗",
	})
	if st != 400 {
		t.Fatalf("expected 400 after leaving custom group, got %d", st)
	}
	_ = idDa
}

func TestRenderQRSvg(t *testing.T) {
	svg := RenderQRSvg("growthbuddy://add-parent?code=ABCD2345", 200)
	if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, "<path ") {
		t.Fatalf("unexpected QR SVG output: %s", svg)
	}
}

func TestParentSocialSafetyAndChildOversightFlow(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// 1. 注册两个独立家庭：家庭 A（妈妈 + 爸爸 + 大宝）与 家庭 B（爸爸B + 小明）
	st, regA := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "阳光家", "displayName": "妈妈A", "username": "moma_p", "password": "pass1234",
		"childName": "大宝", "childUsername": "dabao_p", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register A failed: %d", st)
	}
	momTok := regA["token"].(string)
	st, dadA := doJSON(t, ts, "POST", "/api/parent/parents", momTok, map[string]any{
		"username": "dada_p", "displayName": "爸爸A", "password": "pass1234",
	})
	if st != 201 {
		t.Fatalf("create second parent failed: %d", st)
	}
	dadAID := dadA["id"].(string)

	st, regB := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "快乐家", "displayName": "爸爸B", "username": "dadb_p", "password": "pass1234",
		"childName": "小明", "childUsername": "xiaoming_p", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register B failed: %d", st)
	}
	dadBTok := regB["token"].(string)
	dadBID := regB["parent"].(map[string]any)["id"].(string)

	st, lDa := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "dabao_p", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login dabao failed: %d", st)
	}
	tokDa, idDa := lDa["token"].(string), lDa["childId"].(string)

	st, lXiao := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "xiaoming_p", "password": "pass1234"})
	if st != 200 {
		t.Fatalf("login xiaoming failed: %d", st)
	}
	tokXiao, idXiao := lXiao["token"].(string), lXiao["childId"].(string)

	// 2. 验证家长概览包含同家庭配偶与孩子、默认家庭群与二维码名片
	st, pOvA := doJSON(t, ts, "GET", "/api/parent/friends", momTok, nil)
	if st != 200 {
		t.Fatalf("parent overview failed: %d", st)
	}
	selfA := pOvA["self"].(map[string]any)
	if !strings.Contains(selfA["qr_svg"].(string), "<svg") || selfA["friend_code"].(string) == "" {
		t.Fatalf("expected parent qr_svg and friend_code, got %+v", selfA)
	}
	friendsA := pOvA["friends"].([]any)
	hasDadA, hasDa := false, false
	for _, f := range friendsA {
		m := f.(map[string]any)
		if m["peer_type"] == "parent" && m["peer_id"] == dadAID {
			hasDadA = true
		}
		if m["peer_type"] == "child" && m["peer_id"] == idDa {
			hasDa = true
		}
	}
	if !hasDadA || !hasDa {
		t.Fatalf("expected same-family spouse & child in parent friends, got %+v", friendsA)
	}

	// 3. 安全边界校验：家长不可跨家庭搜/加别人孩子；孩子不可搜/加别人家长；不可跨家庭私聊孩子
	st, _ = doJSON(t, ts, "GET", "/api/parent/friends/lookup?q=xiaoming_p", momTok, nil)
	if st != 404 {
		t.Fatalf("expected 404 when parent looks up other family's child, got %d", st)
	}
	st, _ = doJSON(t, ts, "POST", "/api/parent/friends/request", momTok, map[string]any{"target": "xiaoming_p"})
	if st != 400 {
		t.Fatalf("expected 400 when parent sends friend req to other family's child, got %d", st)
	}
	st, _ = doJSON(t, ts, "GET", "/api/child/friends/lookup?q=dadb_p", tokDa, nil)
	if st != 404 {
		t.Fatalf("expected 404 when child looks up other family's parent, got %d", st)
	}
	st, _ = doJSON(t, ts, "POST", "/api/parent/chat/messages", momTok, map[string]any{
		"peerType": "child", "peerId": idXiao, "msgType": "text", "text": "越界测试",
	})
	if st != 400 {
		t.Fatalf("expected 400 when parent messages unrelated child, got %d", st)
	}

	// 4. 家长 <-> 家长 跨家庭互加好友、自建家长群（且禁止跨家庭群混入孩子）
	st, pOvB := doJSON(t, ts, "GET", "/api/parent/friends", dadBTok, nil)
	if st != 200 {
		t.Fatalf("dad B overview failed: %d", st)
	}
	dadBCode := pOvB["self"].(map[string]any)["friend_code"].(string)
	st, lookB := doJSON(t, ts, "GET", "/api/parent/friends/lookup?q="+url.QueryEscape(dadBCode), momTok, nil)
	if st != 200 || lookB["candidate"].(map[string]any)["parent_id"] != dadBID {
		t.Fatalf("parent lookup by code failed: %d %+v", st, lookB)
	}
	st, _ = doJSON(t, ts, "POST", "/api/parent/friends/request", momTok, map[string]any{
		"target": dadBCode, "message": "您好，我是大宝妈妈！",
	})
	if st != 200 {
		t.Fatalf("send parent friend req failed: %d", st)
	}
	st, pOvB2 := doJSON(t, ts, "GET", "/api/parent/friends", dadBTok, nil)
	pReqID := pOvB2["incoming_requests"].([]any)[0].(map[string]any)["id"].(string)
	st, _ = doJSON(t, ts, "POST", "/api/parent/friends/requests/respond", dadBTok, map[string]any{
		"requestId": pReqID, "action": "accept",
	})
	if st != 200 {
		t.Fatalf("accept parent req failed: %d", st)
	}

	// 验证跨家庭混合群聊被安全策略拦截（妈妈A不能在同一自定义群里混入爸爸B与自家大宝）
	st, _ = doJSON(t, ts, "POST", "/api/parent/groups", momTok, map[string]any{
		"name": "不允许混入孩子",
		"members": []map[string]any{
			{"memberType": "parent", "memberId": dadBID},
			{"memberType": "child", "memberId": idDa},
		},
	})
	if st != 400 {
		t.Fatalf("expected 400 mixing cross-family adult + minor in custom group, got %d", st)
	}

	// 纯家长跨家庭自建群与聊天正常通过
	st, pGrpRes := doJSON(t, ts, "POST", "/api/parent/groups", momTok, map[string]any{
		"name": "家校互通家长群",
		"members": []map[string]any{
			{"memberType": "parent", "memberId": dadAID},
			{"memberType": "parent", "memberId": dadBID},
		},
	})
	if st != 201 {
		t.Fatalf("create parent custom group failed: %d %+v", st, pGrpRes)
	}

	// 5. 大宝与小明加好友发图片聊天，妈妈A可在家长端只读查看大宝好友与聊天记录，且不消除大宝未读数
	st, _ = doJSON(t, ts, "POST", "/api/child/friends/request", tokDa, map[string]any{"target": "xiaoming_p"})
	st, ovXiao := doJSON(t, ts, "GET", "/api/child/friends", tokXiao, nil)
	cReqID := ovXiao["incoming_requests"].([]any)[0].(map[string]any)["id"].(string)
	st, _ = doJSON(t, ts, "POST", "/api/child/friends/requests/respond", tokXiao, map[string]any{"requestId": cReqID, "action": "accept"})
	if st != 200 {
		t.Fatalf("child accept failed: %d", st)
	}
	imgData := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 9, 8, 7})
	st, xMsgRes := doJSON(t, ts, "POST", "/api/child/chat/messages", tokXiao, map[string]any{
		"peerType": "child", "peerId": idDa, "msgType": "image", "mediaData": imgData,
	})
	if st != 201 {
		t.Fatalf("xiao send image failed: %d", st)
	}
	cMediaURL := xMsgRes["message"].(map[string]any)["media_url"].(string)

	// 妈妈A查看大宝好友列表与聊天记录
	st, ovKids := doJSON(t, ts, "GET", "/api/parent/children/"+idDa+"/social", momTok, nil)
	if st != 200 {
		t.Fatalf("parent oversight child social failed: %d %+v", st, ovKids)
	}
	st, ovChat := doJSON(t, ts, "GET", "/api/parent/children/"+idDa+"/chat/messages?peerType=child&peerId="+url.QueryEscape(idXiao), momTok, nil)
	if st != 200 || len(ovChat["messages"].([]any)) != 1 {
		t.Fatalf("parent oversight child chat failed: %d %+v", st, ovChat)
	}

	// 妈妈A可查看孩子聊天媒体（家庭B的爸爸不能访问，家庭C也不能；Wait: 爸爸B是小明家长也能看小明的，但不能看与自己无关的）
	reqMomMedia, _ := http.NewRequest("GET", ts.URL+cMediaURL, nil)
	reqMomMedia.Header.Set("Authorization", "Bearer "+momTok)
	respMomMedia, _ := http.DefaultClient.Do(reqMomMedia)
	_ = respMomMedia.Body.Close()
	if respMomMedia.StatusCode != 200 {
		t.Fatalf("expected 200 for parent inspecting own child's chat media, got %d", respMomMedia.StatusCode)
	}

	// 爸爸B不能越权查看大宝的好友列表
	st, _ = doJSON(t, ts, "GET", "/api/parent/children/"+idDa+"/social", dadBTok, nil)
	if st != 404 {
		t.Fatalf("expected 404 when unrelated parent inspects child A social, got %d", st)
	}

	// 验证大宝未读消息没有因为妈妈只读查看而清零
	st, daState := doJSON(t, ts, "GET", "/api/child/state", tokDa, nil)
	if st != 200 || int(daState["child"].(map[string]any)["unread_chat"].(float64)) < 1 {
		t.Fatalf("expected dabao unread_chat >= 1 after parent read-only oversight, got %+v", daState)
	}
}
