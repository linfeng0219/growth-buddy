package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func newTestServer(t *testing.T) (*httptest.Server, *Store, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	relDir := filepath.Join(dir, "releases")
	store, err := NewStore(dbPath, relDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	app := NewAppServer(store, "")
	ts := httptest.NewServer(app)
	t.Cleanup(ts.Close)
	return ts, store, relDir
}

func doJSON(t *testing.T, ts *httptest.Server, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.URL+path, rdr)
	if err != nil {
		t.Fatalf("new req failed: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("http %s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func TestUnifiedLoginAndLegacyEndpointsRemoved(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// Health check reports unified protocolVersion
	st, h := doJSON(t, ts, "GET", "/api/health", "", nil)
	if st != 200 || int(h["protocolVersion"].(float64)) != ProtocolVersion {
		t.Fatalf("unexpected protocolVersion in health: %d %+v", st, h)
	}

	// Register family
	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName":    "协议升级测试家",
		"displayName":   "妈妈",
		"username":      "proto_mom",
		"password":      "pass1234",
		"childName":     "小测",
		"childUsername": "proto_kid",
		"childPassword": "kid1234",
	})
	if st != 201 {
		t.Fatalf("register-family failed: %d %+v", st, reg)
	}
	parentTok := reg["token"].(string)

	// Parent unified login
	st, pLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "proto_mom",
		"password": "pass1234",
	})
	if st != 200 || pLogin["role"] != "parent" || pLogin["token"] == "" {
		t.Fatalf("unified parent login failed: %d %+v", st, pLogin)
	}

	// Child unified login
	st, cLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "proto_kid",
		"password": "kid1234",
	})
	if st != 200 || cLogin["role"] != "child" || cLogin["token"] == "" {
		t.Fatalf("unified child login failed: %d %+v", st, cLogin)
	}
	childTok := cLogin["token"].(string)

	// Legacy endpoints are completely removed (404)
	for _, tc := range []struct {
		method string
		path   string
		tok    string
	}{
		{"POST", "/api/auth/parent-login", ""},
		{"POST", "/api/auth/child-login", ""},
		{"GET", "/api/family/roster?code=123456", ""},
		{"POST", "/api/parent/questions", parentTok},
		{"POST", "/api/parent/import-backup", parentTok},
	} {
		code, _ := doJSON(t, ts, tc.method, tc.path, tc.tok, map[string]any{})
		if code != 404 {
			t.Fatalf("expected legacy endpoint %s %s removed (404), got %d", tc.method, tc.path, code)
		}
	}

	// Parent token cannot call /api/child/* directly anymore
	st, _ = doJSON(t, ts, "GET", "/api/child/state", parentTok, nil)
	if st != 403 {
		t.Fatalf("expected 403 for parent on /api/child/state, got %d", st)
	}
	_ = childTok
}

func TestOTAForceUpdateFloor(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, aLogin := doJSON(t, ts, "POST", "/api/auth/admin-login", "", map[string]any{
		"username": "admin", "password": "admin1234",
	})
	if st != 200 {
		t.Fatalf("admin login failed: %d", st)
	}
	adminTok := aLogin["token"].(string)

	// Mandatory baseline v100
	st, _ = doJSON(t, ts, "POST", "/api/admin/releases", adminTok, map[string]any{
		"versionCode":  100,
		"versionName":  "2026.11.23-1800",
		"releaseNotes": "统一协议强制升级基准版",
		"forceUpdate":  true,
		"downloadUrl":  "https://example.com/v100.apk",
	})
	if st != 201 {
		t.Fatalf("create rel v100 failed: %d", st)
	}

	// Subsequent optional patch v120
	st, _ = doJSON(t, ts, "POST", "/api/admin/releases", adminTok, map[string]any{
		"versionCode":  120,
		"versionName":  "2026.11.23-1810",
		"releaseNotes": "小修小补",
		"forceUpdate":  false,
		"downloadUrl":  "https://example.com/v120.apk",
	})
	if st != 201 {
		t.Fatalf("create rel v120 failed: %d", st)
	}

	// Client v90 (< minForced=100) must still be forced to update to v120
	st, v90 := doJSON(t, ts, "GET", "/api/app/version?platform=android&currentVersionCode=90", "", nil)
	if st != 200 || v90["hasUpdate"] != true || v90["forceUpdate"] != true || int(v90["minSupportedVersionCode"].(float64)) != 100 {
		t.Fatalf("expected v90 forced update: %d %+v", st, v90)
	}

	// Client v105 (>= minForced=100, < 120) gets optional update
	st, v105 := doJSON(t, ts, "GET", "/api/app/version?platform=android&currentVersionCode=105", "", nil)
	if st != 200 || v105["hasUpdate"] != true || v105["forceUpdate"] != false {
		t.Fatalf("expected v105 optional update: %d %+v", st, v105)
	}
}

func TestFullThreePortalAndNativeFlows(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// Health & embedded static pages
	st, h := doJSON(t, ts, "GET", "/api/health", "", nil)
	if st != 200 || h["engine"] != "go-sqlite" || h["version"] != SystemVersion {
		t.Fatalf("unexpected health: %d %+v", st, h)
	}
	resp, err := http.Get(ts.URL + "/admin.html")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("embedded admin.html failed: %v", err)
	}
	_ = resp.Body.Close()

	// Register family with first child username/password
	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName":    "林家小院",
		"displayName":   "妈妈",
		"username":      "linmama",
		"password":      "pass1234",
		"childName":     "大宝",
		"childUsername": "dabao01",
		"childPassword": "kid1234",
	})
	if st != 201 {
		t.Fatalf("register-family failed: %d %+v", st, reg)
	}
	parentTok := reg["token"].(string)
	firstChildID := reg["firstChildId"].(string)
	famID := reg["family"].(map[string]any)["id"].(string)

	// Child direct login via unified /api/auth/login
	st, cLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "dabao01",
		"password": "kid1234",
	})
	if st != 200 || cLogin["role"] != "child" || cLogin["childId"] != firstChildID {
		t.Fatalf("child unified login failed: %d %+v", st, cLogin)
	}
	childTok := cLogin["token"].(string)

	// Child cannot access parent or admin APIs
	st, _ = doJSON(t, ts, "GET", "/api/parent/overview", childTok, nil)
	if st != 403 {
		t.Fatalf("expected 403 for child on parent overview, got %d", st)
	}
	st, _ = doJSON(t, ts, "GET", "/api/admin/overview", childTok, nil)
	if st != 403 {
		t.Fatalf("expected 403 for child on admin overview, got %d", st)
	}

	// Parent creates a rope_jump task
	st, taskRes := doJSON(t, ts, "POST", "/api/parent/tasks", parentTok, map[string]any{
		"label":        "一分钟跳绳",
		"pts":          6,
		"scope":        "selected",
		"task_type":    "rope_jump",
		"duration_sec": 60,
		"target_count": 80,
		"childIds":     []string{firstChildID},
	})
	if st != 201 {
		t.Fatalf("create rope task failed: %d %+v", st, taskRes)
	}
	ropeTaskID := taskRes["id"].(string)

	// Rope jump below target should fail
	st, _ = doJSON(t, ts, "POST", "/api/child/checkin", childTok, map[string]any{
		"taskId":         ropeTaskID,
		"date":           "2026-11-18",
		"actualCount":    50,
		"actualDuration": 60,
	})
	if st != 400 {
		t.Fatalf("expected 400 for insufficient rope count, got %d", st)
	}

	// Rope jump meeting target should succeed & include pts top-level and optional photoData
	samplePhotoBytes := []byte("\xff\xd8\xff\xe0\x00\x10JFIF-FAKE-JPEG-PHOTO-PAYLOAD")
	samplePhotoData := "data:image/jpeg;base64," + base64Encode(samplePhotoBytes)
	st, chk := doJSON(t, ts, "POST", "/api/child/checkin", childTok, map[string]any{
		"taskId":         ropeTaskID,
		"date":           "2026-11-18",
		"actualCount":    95,
		"actualDuration": 58,
		"photoData":      samplePhotoData,
	})
	if st != 200 || int(chk["pts"].(float64)) != 6 || int(chk["newXp"].(float64)) != 6 || chk["hasPhoto"] != true {
		t.Fatalf("rope checkin with photo failed: %d %+v", st, chk)
	}
	photoURL, _ := chk["photoUrl"].(string)
	if !strings.HasPrefix(photoURL, "/api/checkin-photo/") {
		t.Fatalf("expected photoUrl in checkin response, got: %+v", chk)
	}
	photoResp, err := http.Get(ts.URL + photoURL)
	if err != nil || photoResp.StatusCode != 200 {
		t.Fatalf("fetch checkin photo failed: %v", err)
	}
	gotPhotoBytes, _ := io.ReadAll(photoResp.Body)
	_ = photoResp.Body.Close()
	if !bytes.Equal(gotPhotoBytes, samplePhotoBytes) || photoResp.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("checkin photo content mismatch: ct=%s len=%d", photoResp.Header.Get("Content-Type"), len(gotPhotoBytes))
	}

	// Verify parent overview includes recent_logs with has_photo and photo_url
	st, pOv := doJSON(t, ts, "GET", "/api/parent/overview?date=2026-11-18", parentTok, nil)
	if st != 200 {
		t.Fatalf("parent overview failed: %d", st)
	}
	pKids := pOv["children"].([]any)
	pLogs := pKids[0].(map[string]any)["recent_logs"].([]any)
	if len(pLogs) == 0 || pLogs[0].(map[string]any)["has_photo"] != true || pLogs[0].(map[string]any)["photo_url"] != photoURL {
		t.Fatalf("parent overview missing photo info in recent_logs: %+v", pLogs)
	}

	// Native Android convenience routes: mute, pet-name, grant egg, adopt, companion, polaroid
	st, _ = doJSON(t, ts, "PATCH", "/api/child/mute", childTok, map[string]any{"muted": true})
	if st != 200 {
		t.Fatalf("mute failed: %d", st)
	}
	st, _ = doJSON(t, ts, "PATCH", "/api/child/pet-name", childTok, map[string]any{"petName": "糯米团"})
	if st != 200 {
		t.Fatalf("pet-name failed: %d", st)
	}
	st, _ = doJSON(t, ts, "POST", "/api/parent/grant-egg", parentTok, map[string]any{"childId": firstChildID, "amount": 2})
	if st != 200 {
		t.Fatalf("grant-egg failed: %d", st)
	}
	st, _ = doJSON(t, ts, "POST", "/api/child/adopt", childTok, map[string]any{"speciesId": "b2", "date": "2026-11-18"})
	if st != 200 {
		t.Fatalf("adopt failed: %d", st)
	}
	st, compRes := doJSON(t, ts, "POST", "/api/child/companion", childTok, map[string]any{"speciesId": "b2"})
	if st != 200 || compRes["state"] == nil {
		t.Fatalf("companion failed: %d %+v", st, compRes)
	}
	st, _ = doJSON(t, ts, "POST", "/api/child/polaroid", childTok, map[string]any{"avatarIdx": 1, "note": "开心打卡", "date": "2026-11-18"})
	if st != 200 {
		t.Fatalf("polaroid failed: %d", st)
	}

	// Verify state persisted correctly
	st, cState := doJSON(t, ts, "GET", "/api/child/state?date=2026-11-18", childTok, nil)
	if st != 200 {
		t.Fatalf("child state failed: %d", st)
	}
	childObj := cState["child"].(map[string]any)
	if childObj["pet_name"] != "糯米团" || childObj["muted"] != true || childObj["active_companion"] != "b2" {
		t.Fatalf("unexpected child state: %+v", childObj)
	}

	// Admin login & CRUD
	st, aLogin := doJSON(t, ts, "POST", "/api/auth/admin-login", "", map[string]any{
		"username": "admin",
		"password": "admin1234",
	})
	if st != 200 {
		t.Fatalf("admin login failed: %d %+v", st, aLogin)
	}
	adminTok := aLogin["token"].(string)

	st, ov := doJSON(t, ts, "GET", "/api/admin/overview?date=2026-11-18", adminTok, nil)
	if st != 200 {
		t.Fatalf("admin overview failed: %d", st)
	}
	stats := ov["stats"].(map[string]any)
	if int(stats["family_count"].(float64)) != 1 || int(stats["today_checkins"].(float64)) != 1 {
		t.Fatalf("unexpected admin stats: %+v", stats)
	}

	// Admin updates family name
	st, _ = doJSON(t, ts, "PATCH", "/api/admin/families/"+famID, adminTok, map[string]any{"name": "林家大院"})
	if st != 200 {
		t.Fatalf("admin update family failed: %d", st)
	}
}

func TestAvatarUploadBuiltinAndPolaroidSnapshot(t *testing.T) {
	ts, _, _ := newTestServer(t)

	// Register family
	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName":    "测试之家",
		"displayName":   "爸爸",
		"username":      "testbaba",
		"password":      "pass1234",
		"childName":     "二宝",
		"childUsername": "erbao01",
		"childPassword": "kid1234",
	})
	if st != 201 {
		t.Fatalf("register-family failed: %d %+v", st, reg)
	}
	parentTok := reg["token"].(string)
	parentID := reg["parent"].(map[string]any)["id"].(string)
	childID := reg["firstChildId"].(string)

	// Child logs in via unified /api/auth/login
	st, cLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "erbao01",
		"password": "kid1234",
	})
	if st != 200 || cLogin["role"] != "child" {
		t.Fatalf("child unified login failed: %d", st)
	}
	childTok := cLogin["token"].(string)

	// 1. Parent picks built-in avatar index 8 (up to 11 allowed)
	st, pAv1 := doJSON(t, ts, "POST", "/api/parent/avatar", parentTok, map[string]any{"avatarIdx": 8})
	if st != 200 || int(pAv1["parent"].(map[string]any)["avatar_idx"].(float64)) != 8 {
		t.Fatalf("parent built-in avatar failed: %d %+v", st, pAv1)
	}

	// 2. Parent uploads custom JPEG avatar
	parentImgBytes := []byte("\xff\xd8\xff\xe0\x00\x10JFIF-PARENT-AVATAR")
	st, pAv2 := doJSON(t, ts, "POST", "/api/parent/avatar", parentTok, map[string]any{
		"avatarData": "data:image/jpeg;base64," + base64Encode(parentImgBytes),
	})
	if st != 200 {
		t.Fatalf("parent upload avatar failed: %d %+v", st, pAv2)
	}
	pURL, _ := pAv2["parent"].(map[string]any)["avatar_url"].(string)
	if !strings.HasPrefix(pURL, "/api/avatar/parent/"+parentID) {
		t.Fatalf("expected parent avatar_url prefix, got %q", pURL)
	}
	resp, err := http.Get(ts.URL + pURL)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET parent avatar failed: %v", err)
	}
	gotP, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !bytes.Equal(gotP, parentImgBytes) {
		t.Fatalf("parent avatar bytes mismatch")
	}

	// 3. Child selects built-in avatar 10, takes polaroid (server must use child's avatarIdx=10, ignoring body avatarIdx=2)
	st, cAv1 := doJSON(t, ts, "POST", "/api/child/avatar", childTok, map[string]any{"avatarIdx": 10})
	if st != 200 {
		t.Fatalf("child built-in avatar failed: %d %+v", st, cAv1)
	}
	st, pol1 := doJSON(t, ts, "POST", "/api/child/polaroid", childTok, map[string]any{
		"avatarIdx": 2,
		"note":      "内置头像合影",
		"date":      "2026-11-18",
	})
	if st != 200 {
		t.Fatalf("polaroid 1 failed: %d %+v", st, pol1)
	}
	pols1 := pol1["state"].(map[string]any)["polaroids"].([]any)
	if int(pols1[0].(map[string]any)["avatar_idx"].(float64)) != 10 {
		t.Fatalf("expected polaroid to use child's avatar_idx=10, got %+v", pols1[0])
	}

	// 4. Child uploads custom PNG avatar, takes polaroid -> polaroid snapshots custom image via /api/avatar/polaroid/{id}
	childImgBytes := []byte("\x89PNG\r\n\x1a\nFAKE-CHILD-PNG")
	st, cAv2 := doJSON(t, ts, "POST", "/api/child/avatar", childTok, map[string]any{
		"avatarData": "data:image/png;base64," + base64Encode(childImgBytes),
	})
	if st != 200 {
		t.Fatalf("child upload avatar failed: %d %+v", st, cAv2)
	}
	cURL, _ := cAv2["state"].(map[string]any)["child"].(map[string]any)["avatar_url"].(string)
	if !strings.HasPrefix(cURL, "/api/avatar/child/"+childID) {
		t.Fatalf("expected child avatar_url, got %q", cURL)
	}
	st, pol2 := doJSON(t, ts, "POST", "/api/child/polaroid", childTok, map[string]any{
		"note": "自定义头像合影",
		"date": "2026-11-19",
	})
	if st != 200 {
		t.Fatalf("polaroid 2 failed: %d %+v", st, pol2)
	}
	pols2 := pol2["state"].(map[string]any)["polaroids"].([]any)
	polURL, _ := pols2[0].(map[string]any)["avatar_url"].(string)
	if !strings.HasPrefix(polURL, "/api/avatar/polaroid/") {
		t.Fatalf("expected polaroid avatar_url, got %+v", pols2[0])
	}
	polResp, err := http.Get(ts.URL + polURL)
	if err != nil || polResp.StatusCode != 200 {
		t.Fatalf("GET polaroid avatar failed: %v", err)
	}
	gotPol, _ := io.ReadAll(polResp.Body)
	_ = polResp.Body.Close()
	if !bytes.Equal(gotPol, childImgBytes) || polResp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("polaroid avatar bytes/ct mismatch: ct=%s", polResp.Header.Get("Content-Type"))
	}

	// 5. If child switches back to built-in avatar 3, previously taken polaroid keeps its snapshot!
	st, cAv3 := doJSON(t, ts, "POST", "/api/child/avatar", childTok, map[string]any{"avatarIdx": 3})
	if st != 200 {
		t.Fatalf("child revert avatar failed: %d", st)
	}
	child3 := cAv3["state"].(map[string]any)["child"].(map[string]any)
	if int(child3["avatar_idx"].(float64)) != 3 || child3["avatar_url"].(string) != "" {
		t.Fatalf("expected child reverted to idx 3 with empty url, got %+v", child3)
	}
	pols3 := cAv3["state"].(map[string]any)["polaroids"].([]any)
	if pols3[0].(map[string]any)["avatar_url"].(string) != polURL {
		t.Fatalf("expected previous polaroid snapshot to remain intact, got %+v", pols3[0])
	}
}

func TestAppOTAReleaseLifecycle(t *testing.T) {
	ts, _, relDir := newTestServer(t)

	// No release yet
	st, v0 := doJSON(t, ts, "GET", "/api/app/version?platform=android&currentVersionCode=2", "", nil)
	if st != 200 || v0["hasRelease"] != false || v0["hasUpdate"] != false {
		t.Fatalf("expected no release yet: %d %+v", st, v0)
	}

	// Admin login
	st, aLogin := doJSON(t, ts, "POST", "/api/auth/admin-login", "", map[string]any{
		"username": "admin",
		"password": "admin1234",
	})
	if st != 200 {
		t.Fatalf("admin login failed: %d", st)
	}
	adminTok := aLogin["token"].(string)

	// Upload a mock APK via multipart/form-data
	fakeAPK := []byte("PK\x03\x04FAKE-ANDROID-APK-PAYLOAD-FOR-OTA-TEST-v2.1.0")
	expectedHash := sha256.Sum256(fakeAPK)
	expectedSHA := hex.EncodeToString(expectedHash[:])

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("versionCode", "3")
	_ = mw.WriteField("versionName", "2.1.0")
	_ = mw.WriteField("releaseNotes", "新增远程在线升级与 Go 后端")
	_ = mw.WriteField("forceUpdate", "true")
	fw, _ := mw.CreateFormFile("apk", "GrowthBuddyChild-2.1.0.apk")
	_, _ = fw.Write(fakeAPK)
	_ = mw.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/admin/releases", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminTok)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload release failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201 on release upload, got %d: %s", resp.StatusCode, string(b))
	}
	var created map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&created)
	relID := created["id"].(string)
	if created["sha256"] != expectedSHA || int(created["file_size"].(float64)) != len(fakeAPK) {
		t.Fatalf("unexpected release metadata: %+v", created)
	}

	// Client on versionCode=2 sees update + forceUpdate
	st, v1 := doJSON(t, ts, "GET", "/api/app/version?platform=android&currentVersionCode=2", "", nil)
	if st != 200 || v1["hasUpdate"] != true || v1["forceUpdate"] != true {
		t.Fatalf("expected hasUpdate=true, forceUpdate=true: %d %+v", st, v1)
	}

	// Client already on versionCode=3 sees no update
	st, v2 := doJSON(t, ts, "GET", "/api/app/version?platform=android&currentVersionCode=3", "", nil)
	if st != 200 || v2["hasUpdate"] != false || v2["forceUpdate"] != false {
		t.Fatalf("expected hasUpdate=false for current version: %d %+v", st, v2)
	}

	// Download via /api/app/download/{id} and /api/app/latest.apk
	for _, dlPath := range []string{"/api/app/download/" + relID, "/api/app/latest.apk"} {
		dlResp, err := http.Get(ts.URL + dlPath)
		if err != nil || dlResp.StatusCode != 200 {
			t.Fatalf("download %s failed: %v", dlPath, err)
		}
		ct := dlResp.Header.Get("Content-Type")
		bodyBytes, _ := io.ReadAll(dlResp.Body)
		_ = dlResp.Body.Close()
		if !strings.Contains(ct, "application/vnd.android.package-archive") {
			t.Fatalf("unexpected content-type on %s: %s", dlPath, ct)
		}
		if !bytes.Equal(bodyBytes, fakeAPK) {
			t.Fatalf("downloaded APK bytes mismatch on %s", dlPath)
		}
	}

	// Admin deletes release -> DB record and file removed
	st, _ = doJSON(t, ts, "DELETE", "/api/admin/releases/"+relID, adminTok, nil)
	if st != 200 {
		t.Fatalf("delete release failed: %d", st)
	}
	if _, err := os.Stat(filepath.Join(relDir, relID+".apk")); !os.IsNotExist(err) {
		t.Fatalf("expected local APK file to be deleted")
	}

	// Blank releaseNotes auto-fills from changelog.json matching version date
	st, autoRel := doJSON(t, ts, "POST", "/api/admin/releases", adminTok, map[string]any{
		"versionName":  "2026.11.24-1115",
		"releaseNotes": "",
		"forceUpdate":  false,
		"downloadUrl":  "https://example.com/auto.apk",
	})
	if st != 201 {
		t.Fatalf("auto release notes create failed: %d %+v", st, autoRel)
	}
	notes, _ := autoRel["release_notes"].(string)
	if !strings.Contains(notes, "好友") || !strings.Contains(notes, "• ") || strings.Contains(notes, "732 题") {
		t.Fatalf("expected auto-filled App-only release_notes from 2026-11-24 changelog, got %q", notes)
	}

	// Fish Windows .exe release upload, /api/app/version?platform=fish, /api/app/latest.exe, delete
	fakeEXE := []byte("MZ-fake-fish-exe-payload-67890")
	exeSum := sha256.Sum256(fakeEXE)
	expectedExeSHA := hex.EncodeToString(exeSum[:])

	var fishBuf bytes.Buffer
	fmw := multipart.NewWriter(&fishBuf)
	_ = fmw.WriteField("platform", "fish")
	_ = fmw.WriteField("versionCode", "20")
	_ = fmw.WriteField("versionName", "2026.11.24-1620")
	_ = fmw.WriteField("releaseNotes", "桌面端 fish 首个 OTA 版本")
	_ = fmw.WriteField("forceUpdate", "true")
	fPart, _ := fmw.CreateFormFile("file", "fish-2026.11.24-1620.exe")
	_, _ = fPart.Write(fakeEXE)
	_ = fmw.Close()

	fReq, _ := http.NewRequest("POST", ts.URL+"/api/admin/releases", &fishBuf)
	fReq.Header.Set("Authorization", "Bearer "+adminTok)
	fReq.Header.Set("Content-Type", fmw.FormDataContentType())
	fResp, err := http.DefaultClient.Do(fReq)
	if err != nil {
		t.Fatalf("fish release upload failed: %v", err)
	}
	defer fResp.Body.Close()
	if fResp.StatusCode != 201 {
		b, _ := io.ReadAll(fResp.Body)
		t.Fatalf("expected 201 on fish release upload, got %d: %s", fResp.StatusCode, string(b))
	}
	var fishCreated map[string]any
	_ = json.NewDecoder(fResp.Body).Decode(&fishCreated)
	fishID := fishCreated["id"].(string)
	if fishCreated["platform"] != "fish" || fishCreated["sha256"] != expectedExeSHA {
		t.Fatalf("unexpected fish release metadata: %+v", fishCreated)
	}

	st, fv1 := doJSON(t, ts, "GET", "/api/app/version?platform=fish&currentVersionCode=10", "", nil)
	if st != 200 || fv1["hasUpdate"] != true || fv1["forceUpdate"] != true {
		t.Fatalf("expected fish hasUpdate=true: %d %+v", st, fv1)
	}
	st, fv2 := doJSON(t, ts, "GET", "/api/app/version?platform=fish&currentVersionCode=20", "", nil)
	if st != 200 || fv2["hasUpdate"] != false {
		t.Fatalf("expected fish hasUpdate=false at 20: %d %+v", st, fv2)
	}

	for _, dlPath := range []string{"/api/app/download/" + fishID + ".exe", "/api/app/latest.exe"} {
		dlResp, err := http.Get(ts.URL + dlPath)
		if err != nil || dlResp.StatusCode != 200 {
			t.Fatalf("fish download %s failed: %v", dlPath, err)
		}
		bodyBytes, _ := io.ReadAll(dlResp.Body)
		_ = dlResp.Body.Close()
		if !bytes.Equal(bodyBytes, fakeEXE) {
			t.Fatalf("downloaded EXE bytes mismatch on %s", dlPath)
		}
	}

	st, _ = doJSON(t, ts, "DELETE", "/api/admin/releases/"+fishID, adminTok, nil)
	if st != 200 {
		t.Fatalf("delete fish release failed: %d", st)
	}
	if _, err := os.Stat(filepath.Join(relDir, fishID+".exe")); !os.IsNotExist(err) {
		t.Fatalf("expected local EXE file to be deleted")
	}
}

func TestChangelogLatestDate(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, res := doJSON(t, ts, "GET", "/api/changelog", "", nil)
	if st != 200 {
		t.Fatalf("changelog endpoint failed: %d", st)
	}
	arr, ok := res["changelog"].([]any)
	if !ok || len(arr) < 2 {
		t.Fatalf("expected at least 2 changelog entries, got %+v", res["changelog"])
	}
	first, ok := arr[0].(map[string]any)
	if !ok || first["date"] != "2026-11-24" {
		t.Fatalf("expected newest changelog entry date 2026-11-24, got %+v", arr[0])
	}
}

func TestAdventureWinDailyCap(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "测试家", "displayName": "妈妈", "username": "momadv", "password": "pass1234",
		"childName": "大宝", "childUsername": "kid_adv", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register failed: %d %+v", st, reg)
	}
	st, login := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "kid_adv", "password": "pass1234",
	})
	if st != 200 || login["role"] != "child" {
		t.Fatalf("child unified login failed: %d %+v", st, login)
	}
	tok, _ := login["token"].(string)

	for i := 1; i <= 10; i++ {
		st, res := doJSON(t, ts, "POST", "/api/child/adventure-win", tok, map[string]any{"date": "2026-11-23"})
		if st != 200 || int(res["earned_xp"].(float64)) != 1 || int(res["today_adventure_xp"].(float64)) != i {
			t.Fatalf("win %d failed: %d %+v", i, st, res)
		}
	}
	st, capped := doJSON(t, ts, "POST", "/api/child/adventure-win", tok, map[string]any{"date": "2026-11-23"})
	if st != 200 || int(capped["earned_xp"].(float64)) != 0 || capped["reached_limit"] != true || int(capped["new_xp"].(float64)) != 10 {
		t.Fatalf("expected daily cap at 10: %d %+v", st, capped)
	}

	// Next day resets daily cap
	st, nextDay := doJSON(t, ts, "POST", "/api/child/adventure-win", tok, map[string]any{"date": "2026-11-24"})
	if st != 200 || int(nextDay["earned_xp"].(float64)) != 1 || int(nextDay["today_adventure_xp"].(float64)) != 1 || int(nextDay["new_xp"].(float64)) != 11 {
		t.Fatalf("expected next day reset: %d %+v", st, nextDay)
	}
}

func TestParentCoManagement(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "家长协作之家", "displayName": "妈妈", "username": "coop_mom", "password": "pass1234",
		"childName": "大宝", "childUsername": "coop_kid", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register failed: %d %+v", st, reg)
	}
	momTok := reg["token"].(string)
	momID := reg["parent"].(map[string]any)["id"].(string)

	// Add co-parent
	st, coCreated := doJSON(t, ts, "POST", "/api/parent/parents", momTok, map[string]any{
		"username": "coop_dad", "displayName": "爸爸", "password": "dad1234", "avatar_idx": 8,
	})
	if st != 201 {
		t.Fatalf("add co-parent failed: %d %+v", st, coCreated)
	}
	dadID := coCreated["id"].(string)

	// Overview returns both parents
	st, ov := doJSON(t, ts, "GET", "/api/parent/overview", momTok, nil)
	if st != 200 {
		t.Fatalf("overview failed: %d", st)
	}
	parentsList, _ := ov["parents"].([]any)
	if len(parentsList) != 2 {
		t.Fatalf("expected 2 parents in overview, got %d (%+v)", len(parentsList), parentsList)
	}

	// Update co-parent displayName, avatar_idx and password
	st, upd := doJSON(t, ts, "PATCH", "/api/parent/parents/"+dadID, momTok, map[string]any{
		"displayName": "超人爸爸", "avatar_idx": 7, "password": "newdad123",
	})
	if st != 200 || upd["parent"].(map[string]any)["display_name"] != "超人爸爸" {
		t.Fatalf("update co-parent failed: %d %+v", st, upd)
	}

	// Updated password works via unified login
	st, dadLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "coop_dad", "password": "newdad123",
	})
	if st != 200 || dadLogin["role"] != "parent" {
		t.Fatalf("updated co-parent login failed: %d %+v", st, dadLogin)
	}

	// Cannot delete currently logged-in parent itself
	st, _ = doJSON(t, ts, "DELETE", "/api/parent/parents/"+momID, momTok, nil)
	if st != 400 {
		t.Fatalf("expected 400 when deleting self, got %d", st)
	}

	// Can delete other co-parent in family
	st, _ = doJSON(t, ts, "DELETE", "/api/parent/parents/"+dadID, momTok, nil)
	if st != 200 {
		t.Fatalf("expected 200 deleting co-parent, got %d", st)
	}
	st, ovAfter := doJSON(t, ts, "GET", "/api/parent/overview", momTok, nil)
	if st != 200 || len(ovAfter["parents"].([]any)) != 1 {
		t.Fatalf("expected 1 parent remaining after delete: %d %+v", st, ovAfter)
	}
}

func TestAdminCredentialsAndPasswordReset(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "密码查看之家", "displayName": "妈妈", "username": "vis_mom", "password": "momPass1",
		"childName": "大宝", "childUsername": "vis_kid", "childPassword": "kidPass1",
	})
	if st != 201 {
		t.Fatalf("register failed: %d %+v", st, reg)
	}
	parentID := reg["parent"].(map[string]any)["id"].(string)
	childID := reg["firstChildId"].(string)
	momTok := reg["token"].(string)

	// Parent overview must NOT leak password_plain
	st, pOv := doJSON(t, ts, "GET", "/api/parent/overview", momTok, nil)
	if st != 200 {
		t.Fatalf("parent overview failed: %d", st)
	}
	if _, leaked := pOv["parents"].([]any)[0].(map[string]any)["password_plain"]; leaked {
		t.Fatalf("parent overview should not expose password_plain")
	}

	// Admin login and inspect credentials
	st, aLogin := doJSON(t, ts, "POST", "/api/auth/admin-login", "", map[string]any{
		"username": "admin", "password": "admin1234",
	})
	if st != 200 {
		t.Fatalf("admin login failed: %d", st)
	}
	adminTok := aLogin["token"].(string)

	st, aOv := doJSON(t, ts, "GET", "/api/admin/overview", adminTok, nil)
	if st != 200 {
		t.Fatalf("admin overview failed: %d", st)
	}
	fam := aOv["families"].([]any)[0].(map[string]any)
	admPar := fam["parents"].([]any)[0].(map[string]any)
	admKid := fam["children"].([]any)[0].(map[string]any)
	if admPar["username"] != "vis_mom" || admPar["password_plain"] != "momPass1" {
		t.Fatalf("expected admin to see parent credentials, got %+v", admPar)
	}
	if admKid["username"] != "vis_kid" || admKid["password_plain"] != "kidPass1" {
		t.Fatalf("expected admin to see child credentials, got %+v", admKid)
	}

	// Admin resets both passwords
	st, _ = doJSON(t, ts, "PATCH", "/api/admin/parents/"+parentID, adminTok, map[string]any{"password": "momNew99"})
	if st != 200 {
		t.Fatalf("admin reset parent password failed: %d", st)
	}
	st, _ = doJSON(t, ts, "PATCH", "/api/admin/children/"+childID, adminTok, map[string]any{"password": "kidNew99"})
	if st != 200 {
		t.Fatalf("admin reset child password failed: %d", st)
	}

	// New passwords visible in admin overview and usable for login
	st, aOv2 := doJSON(t, ts, "GET", "/api/admin/overview", adminTok, nil)
	if st != 200 {
		t.Fatalf("admin overview2 failed: %d", st)
	}
	fam2 := aOv2["families"].([]any)[0].(map[string]any)
	if fam2["parents"].([]any)[0].(map[string]any)["password_plain"] != "momNew99" {
		t.Fatalf("expected updated parent password_plain")
	}
	if fam2["children"].([]any)[0].(map[string]any)["password_plain"] != "kidNew99" {
		t.Fatalf("expected updated child password_plain")
	}
	st, pl := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "vis_mom", "password": "momNew99"})
	if st != 200 || pl["role"] != "parent" {
		t.Fatalf("parent login with reset password failed: %d %+v", st, pl)
	}
	st, cl := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{"username": "vis_kid", "password": "kidNew99"})
	if st != 200 || cl["role"] != "child" {
		t.Fatalf("child login with reset password failed: %d %+v", st, cl)
	}
}

func TestGroupChatHistoryLimitParam(t *testing.T) {
	ts, _, _ := newTestServer(t)

	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "历史条数之家", "displayName": "妈妈", "username": "lim_mom", "password": "pass1234",
		"childName": "小宝", "childUsername": "lim_kid", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register failed: %d %+v", st, reg)
	}
	momTok := reg["token"].(string)

	st, ov := doJSON(t, ts, "GET", "/api/parent/friends", momTok, nil)
	if st != 200 {
		t.Fatalf("friends failed: %d", st)
	}
	groups := ov["chat_groups"].([]any)
	grpID := groups[0].(map[string]any)["id"].(string)

	for i := 0; i < 5; i++ {
		st, _ = doJSON(t, ts, "POST", "/api/parent/chat/messages", momTok, map[string]any{
			"peerType": "group", "peerId": grpID, "msgType": "text", "text": fmt.Sprintf("msg-%d", i+1),
		})
		if st != 201 {
			t.Fatalf("send msg %d failed: %d", i+1, st)
		}
	}

	st, lim2 := doJSON(t, ts, "GET", "/api/parent/chat/messages?peerType=group&peerId="+grpID+"&limit=2", momTok, nil)
	if st != 200 {
		t.Fatalf("limit=2 failed: %d", st)
	}
	arr2 := lim2["messages"].([]any)
	if len(arr2) != 2 || arr2[0].(map[string]any)["text"] != "msg-4" || arr2[1].(map[string]any)["text"] != "msg-5" {
		t.Fatalf("expected latest 2 messages (msg-4, msg-5), got %+v", arr2)
	}

	st, limAll := doJSON(t, ts, "GET", "/api/parent/chat/messages?peerType=group&peerId="+grpID+"&limit=all", momTok, nil)
	if st != 200 {
		t.Fatalf("limit=all failed: %d", st)
	}
	arrAll := limAll["messages"].([]any)
	if len(arrAll) != 5 {
		t.Fatalf("expected all 5 messages with limit=all, got %d", len(arrAll))
	}
}

func TestPetHealthRecoveryBeforeGrowth(t *testing.T) {
	ts, store, _ := newTestServer(t)

	st, reg := doJSON(t, ts, "POST", "/api/auth/register-family", "", map[string]any{
		"familyName": "康复成长之家", "displayName": "妈妈", "username": "heal_mom", "password": "pass1234",
		"childName": "糖糖", "childUsername": "heal_kid", "childPassword": "pass1234",
	})
	if st != 201 {
		t.Fatalf("register failed: %d %+v", st, reg)
	}
	momTok := reg["token"].(string)
	childID := reg["firstChildId"].(string)

	st, cLogin := doJSON(t, ts, "POST", "/api/auth/login", "", map[string]any{
		"username": "heal_kid", "password": "pass1234",
	})
	if st != 200 {
		t.Fatalf("child login failed: %d", st)
	}
	kidTok := cLogin["token"].(string)

	// 创建两个任务：6 分、8 分
	st, t1Res := doJSON(t, ts, "POST", "/api/parent/tasks", momTok, map[string]any{
		"label": "练字", "pts": 6, "emoji": "✍️", "scope": "all",
	})
	if st != 201 {
		t.Fatalf("create task1 failed: %d %+v", st, t1Res)
	}
	t1ID := t1Res["id"].(string)

	st, t2Res := doJSON(t, ts, "POST", "/api/parent/tasks", momTok, map[string]any{
		"label": "朗读", "pts": 8, "emoji": "📖", "scope": "all",
	})
	if st != 201 {
		t.Fatalf("create task2 failed: %d %+v", st, t2Res)
	}
	t2ID := t2Res["id"].(string)

	today := time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02")
	// 设为虚弱状态(需10点恢复)，初始 XP = 5
	if err := store.db.WithTx(func(tx *Tx) error {
		_, err := tx.Exec("UPDATE children SET xp = 5, pet_health = 'weak', heal_progress = 0, last_health_date = ? WHERE id = ?", today, childID)
		return err
	}); err != nil {
		t.Fatalf("seed weak state failed: %v", err)
	}

	// 1. 完成 6 分任务：应全部用于恢复 (0->6/10)，XP 仍为 5
	st, c1 := doJSON(t, ts, "POST", "/api/child/checkin", kidTok, map[string]any{
		"taskId": t1ID, "date": today,
	})
	if st != 200 {
		t.Fatalf("checkin1 failed: %d %+v", st, c1)
	}
	if int(c1["healUsed"].(float64)) != 6 || int(c1["grownXp"].(float64)) != 0 || int(c1["newXp"].(float64)) != 5 {
		t.Fatalf("expected 6 heal / 0 growth / 5 xp, got %+v", c1)
	}
	ch1 := c1["state"].(map[string]any)["child"].(map[string]any)
	if ch1["pet_health"] != "weak" || int(ch1["heal_progress"].(float64)) != 6 || int(ch1["xp"].(float64)) != 5 {
		t.Fatalf("expected still weak 6/10 with xp=5, got %+v", ch1)
	}

	// 2. 完成 8 分任务：前 4 点恢复健康 (6->10->normal)，剩余 4 点用于成长 (5->9)
	st, c2 := doJSON(t, ts, "POST", "/api/child/checkin", kidTok, map[string]any{
		"taskId": t2ID, "date": today,
	})
	if st != 200 {
		t.Fatalf("checkin2 failed: %d %+v", st, c2)
	}
	if int(c2["healUsed"].(float64)) != 4 || int(c2["grownXp"].(float64)) != 4 || int(c2["newXp"].(float64)) != 9 {
		t.Fatalf("expected 4 heal / 4 growth / 9 xp, got %+v", c2)
	}
	ch2 := c2["state"].(map[string]any)["child"].(map[string]any)
	if ch2["pet_health"] != "normal" || int(ch2["heal_progress"].(float64)) != 0 || int(ch2["xp"].(float64)) != 9 {
		t.Fatalf("expected recovered normal with xp=9, got %+v", ch2)
	}

	// 3. 设回生病状态(需20点恢复，已有进度19)，冒险胜利(+1)：恰好恢复健康，XP不额外增加
	if err := store.db.WithTx(func(tx *Tx) error {
		_, err := tx.Exec("UPDATE children SET xp = 9, pet_health = 'sick', heal_progress = 19, last_health_date = ? WHERE id = ?", today, childID)
		return err
	}); err != nil {
		t.Fatalf("seed sick state failed: %v", err)
	}
	st, adv := doJSON(t, ts, "POST", "/api/child/adventure-win", kidTok, map[string]any{"date": today})
	if st != 200 {
		t.Fatalf("adventure-win failed: %d %+v", st, adv)
	}
	if int(adv["heal_used"].(float64)) != 1 || int(adv["grown_xp"].(float64)) != 0 || int(adv["new_xp"].(float64)) != 9 || adv["pet_health"] != "normal" {
		t.Fatalf("expected adventure 1pt to finish healing sick pet with xp=9, got %+v", adv)
	}
}
