package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestNodeAuthCrossCompatibility(t *testing.T) {
	// Verify Go scrypt & HMAC-SHA256 token match Node.js auth.js byte-for-byte
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not available for cross-compat check")
	}
	cmd := exec.Command(nodeBin, "-e", `
		process.env.SESSION_SECRET = "compat-secret-123";
		const auth = require("/home/antigravity/growth-buddy-server/auth.js");
		const h = auth.hashPassword("kidPass!2026");
		const tok = auth.signToken({ role: "child", childId: "kid_abc", familyId: "fam_xyz" }, 3600);
		console.log(JSON.stringify({ hash: h, token: tok }));
	`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("node auth check failed: %v", err)
	}
	var parsed struct {
		Hash  string `json:"hash"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("parse node output failed: %v", err)
	}

	if !VerifyPassword("kidPass!2026", parsed.Hash) {
		t.Fatalf("Go VerifyPassword failed to verify Node scrypt hash: %s", parsed.Hash)
	}
	if VerifyPassword("wrongPass", parsed.Hash) {
		t.Fatalf("Go VerifyPassword accepted wrong password")
	}

	setSessionSecret("compat-secret-123")
	claims := VerifyToken(parsed.Token)
	if claims == nil || claims.Role != "child" || claims.ChildID != "kid_abc" || claims.FamilyID != "fam_xyz" {
		t.Fatalf("Go VerifyToken failed to verify Node HMAC token: %+v", claims)
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

	// Child direct login (no family code)
	st, cLogin := doJSON(t, ts, "POST", "/api/auth/child-login", "", map[string]any{
		"username": "dabao01",
		"password": "kid1234",
	})
	if st != 200 || cLogin["childId"] != firstChildID {
		t.Fatalf("child-login failed: %d %+v", st, cLogin)
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

	// Child logs in
	st, cLogin := doJSON(t, ts, "POST", "/api/auth/child-login", "", map[string]any{
		"username": "erbao01",
		"password": "kid1234",
	})
	if st != 200 {
		t.Fatalf("child-login failed: %d", st)
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
}
