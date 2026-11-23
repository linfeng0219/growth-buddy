package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed all:public
var embeddedPublicFS embed.FS

// SystemVersion 默认取编译注入时间（YYYY.MM.DD-HHMM），未注入时取程序文件最后修改时间。
var SystemVersion = ""

func init() {
	if SystemVersion != "" {
		return
	}
	loc := time.FixedZone("CST", 8*3600)
	t := time.Now().In(loc)
	if exe, err := os.Executable(); err == nil {
		if info, err := os.Stat(exe); err == nil {
			t = info.ModTime().In(loc)
		}
	}
	SystemVersion = t.Format("2006.01.02-1504")
}

func versionCodeFromTime(t time.Time) int {
	loc := time.FixedZone("CST", 8*3600)
	lt := t.In(loc)
	return (lt.Year()-2020)*100_000_000 + int(lt.Month())*1_000_000 + lt.Day()*10_000 + lt.Hour()*100 + lt.Minute()
}

func autoVersionCodeFromName(vName string) int {
	loc := time.FixedZone("CST", 8*3600)
	for _, layout := range []string{"2006.01.02-1504", "2006-01-02 15:04", "200601021504"} {
		if t, err := time.ParseInLocation(layout, strings.TrimSpace(vName), loc); err == nil {
			return versionCodeFromTime(t)
		}
	}
	return versionCodeFromTime(time.Now())
}

var dateRegexp = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type AppServer struct {
	store     *Store
	publicFS  fs.FS
	publicDir string
}

func NewAppServer(store *Store, publicDir string) *AppServer {
	sub, _ := fs.Sub(embeddedPublicFS, "public")
	return &AppServer{
		store:     store,
		publicFS:  sub,
		publicDir: publicDir,
	}
}

func serverToday() string {
	return time.Now().Format("2006-01-02")
}

func sanitizeDate(in string) string {
	if dateRegexp.MatchString(in) {
		return in
	}
	return serverToday()
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func readJSONMap(r *http.Request, maxBytes int64) (map[string]any, error) {
	if maxBytes <= 0 {
		maxBytes = 512 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return nil, errors.New("读取请求体失败")
	}
	if int64(len(body)) > maxBytes {
		return nil, errors.New("请求体过大")
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, errors.New("无效的 JSON 请求体")
	}
	return m, nil
}

func mapStr(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(t)
	}
}

func mapOptStr(m map[string]any, key string) *string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	if v == nil {
		empty := ""
		return &empty
	}
	s := mapStr(m, key)
	return &s
}

func mapInt(m map[string]any, key string, def int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err == nil {
			return n
		}
		return def
	default:
		return def
	}
}

func mapOptInt(m map[string]any, key string) *int {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	n := mapInt(m, key, 0)
	return &n
}

func mapBool(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	default:
		return false
	}
}

func mapOptBool(m map[string]any, key string) *bool {
	_, ok := m[key]
	if !ok {
		return nil
	}
	b := mapBool(m, key)
	return &b
}

func mapStrSlice(m map[string]any, key string) ([]string, bool) {
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out, true
}

func getAuth(r *http.Request) *TokenClaims {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil
	}
	return VerifyToken(strings.TrimSpace(h[7:]))
}

func (a *AppServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	p := r.URL.Path
	if !strings.HasPrefix(p, "/api/") {
		a.serveStatic(w, r)
		return
	}

	a.handleAPI(w, r)
}

func (a *AppServer) serveStatic(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Path
	if rel == "/" || rel == "" {
		rel = "/index.html"
	}
	clean := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if strings.Contains(clean, "..") {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
		return
	}

	var data []byte
	var err error
	if a.publicDir != "" {
		full := filepath.Join(a.publicDir, filepath.FromSlash(clean))
		absPub, _ := filepath.Abs(a.publicDir)
		absFull, _ := filepath.Abs(full)
		if !strings.HasPrefix(absFull, absPub) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
			return
		}
		data, err = os.ReadFile(absFull)
	} else {
		data, err = fs.ReadFile(a.publicFS, clean)
	}
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Not Found"})
		return
	}
	ext := strings.ToLower(path.Ext(clean))
	ct := mime.TypeByExtension(ext)
	switch ext {
	case ".html":
		ct = "text/html; charset=utf-8"
	case ".js":
		ct = "application/javascript; charset=utf-8"
	case ".css":
		ct = "text/css; charset=utf-8"
	case ".json":
		ct = "application/json; charset=utf-8"
	case ".svg":
		ct = "image/svg+xml"
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *AppServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	m := r.Method

	if m == http.MethodGet && p == "/api/health" {
		writeJSON(w, 200, map[string]any{"ok": true, "engine": "go-sqlite", "version": SystemVersion, "time": nowMs()})
		return
	}

	if m == http.MethodGet && p == "/api/changelog" {
		raw, err := fs.ReadFile(embeddedPublicFS, "public/shared/changelog.json")
		if err != nil {
			writeJSON(w, 200, map[string]any{"version": SystemVersion, "changelog": []any{}})
			return
		}
		var list []any
		_ = json.Unmarshal(raw, &list)
		writeJSON(w, 200, map[string]any{"version": SystemVersion, "changelog": list})
		return
	}

	/* ===== 0. 公开 App 远程升级接口 ===== */
	if m == http.MethodGet && p == "/api/app/version" {
		platform := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("platform")))
		if platform == "" {
			platform = "android"
		}
		curCode, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("currentVersionCode")))
		rel, err := a.store.GetLatestRelease(platform)
		if err != nil {
			writeJSON(w, 200, map[string]any{
				"hasRelease":  false,
				"hasUpdate":   false,
				"forceUpdate": false,
				"release":     nil,
			})
			return
		}
		hasUpdate := curCode > 0 && rel.VersionCode > curCode
		writeJSON(w, 200, map[string]any{
			"hasRelease":  true,
			"hasUpdate":   hasUpdate,
			"forceUpdate": hasUpdate && rel.ForceUpdate,
			"release":     rel,
		})
		return
	}

	if (m == http.MethodGet || m == http.MethodHead) && p == "/api/app/latest.apk" {
		rel, err := a.store.GetLatestRelease("android")
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "暂未发布安卓安装包"})
			return
		}
		a.serveReleaseAPK(w, r, rel)
		return
	}

	if (m == http.MethodGet || m == http.MethodHead) && strings.HasPrefix(p, "/api/app/download/") {
		id := strings.TrimPrefix(p, "/api/app/download/")
		id = strings.TrimSuffix(id, ".apk")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, 404, map[string]string{"error": "版本不存在"})
			return
		}
		rel, err := a.store.GetReleaseByID(id)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "版本不存在"})
			return
		}
		a.serveReleaseAPK(w, r, rel)
		return
	}

	if m == http.MethodGet && strings.HasPrefix(p, "/api/avatar/") {
		rest := strings.TrimPrefix(p, "/api/avatar/")
		parts := strings.Split(rest, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeJSON(w, 404, map[string]string{"error": "头像不存在"})
			return
		}
		mimeType, raw, err := a.store.GetAvatarImage(parts[0], parts[1])
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "头像不存在"})
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
		return
	}

	if m == http.MethodGet && strings.HasPrefix(p, "/api/task-icon/") {
		iconID := strings.TrimPrefix(p, "/api/task-icon/")
		if iconID != "" && !strings.Contains(iconID, "/") {
			mimeType, raw, err := a.store.GetTaskIconData(iconID)
			if err != nil {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			_, _ = w.Write(raw)
			return
		}
	}

	if m == http.MethodGet && strings.HasPrefix(p, "/api/checkin-photo/") {
		logID := strings.TrimPrefix(p, "/api/checkin-photo/")
		if logID == "" || strings.Contains(logID, "/") {
			writeJSON(w, 404, map[string]string{"error": "打卡照片不存在"})
			return
		}
		mimeType, raw, err := a.store.GetCheckinPhoto(logID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "打卡照片不存在"})
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
		return
	}

	/* ===== 1. 认证与家庭绑定 ===== */
	if m == http.MethodPost && p == "/api/auth/register-family" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		created, err := a.store.RegisterFamily(RegisterFamilyInput{
			FamilyName:    mapStr(body, "familyName"),
			Username:      mapStr(body, "username"),
			DisplayName:   mapStr(body, "displayName"),
			Password:      mapStr(body, "password"),
			ChildName:     mapStr(body, "childName"),
			ChildUsername: mapStr(body, "childUsername"),
			ChildPassword: mapStr(body, "childPassword"),
		})
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		tok := SignToken(TokenClaims{Role: "parent", ParentID: created.Parent.ID, FamilyID: created.Family.ID}, 0)
		writeJSON(w, 201, map[string]any{
			"token":        tok,
			"family":       created.Family,
			"parent":       created.Parent,
			"firstChildId": created.FirstChildID,
		})
		return
	}

	if m == http.MethodPost && p == "/api/auth/admin-login" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		adm := a.store.VerifyAdminLogin(mapStr(body, "username"), mapStr(body, "password"))
		if adm == nil {
			writeJSON(w, 401, map[string]string{"error": "管理员账号或密码不正确"})
			return
		}
		tok := SignToken(TokenClaims{Role: "admin", AdminID: adm.ID}, 0)
		writeJSON(w, 200, map[string]any{"token": tok, "admin": adm})
		return
	}

	if m == http.MethodPost && p == "/api/auth/parent-login" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		parent, fam, err := a.store.VerifyParentLogin(mapStr(body, "username"), mapStr(body, "password"))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "账号或密码不正确"})
			return
		}
		tok := SignToken(TokenClaims{Role: "parent", ParentID: parent.ID, FamilyID: fam.ID}, 0)
		writeJSON(w, 200, map[string]any{
			"token":  tok,
			"parent": parent,
			"family": fam,
		})
		return
	}

	if m == http.MethodGet && p == "/api/family/roster" {
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			writeJSON(w, 400, map[string]string{"error": "请输入家庭码"})
			return
		}
		fam, kids, err := a.store.GetFamilyRoster(code)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "未找到该家庭码，请向家长确认"})
			return
		}
		writeJSON(w, 200, map[string]any{"family": fam, "children": kids})
		return
	}

	if m == http.MethodPost && p == "/api/auth/child-login" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if _, hasUser := body["username"]; hasUser {
			u := strings.TrimSpace(mapStr(body, "username"))
			pw := mapStr(body, "password")
			if u == "" || pw == "" {
				writeJSON(w, 400, map[string]string{"error": "请输入孩子账号和密码"})
				return
			}
			cid, fid, fcode, err := a.store.LoginChildByAccount(u, pw)
			if err != nil {
				writeJSON(w, 401, map[string]string{"error": err.Error()})
				return
			}
			tok := SignToken(TokenClaims{Role: "child", ChildID: cid, FamilyID: fid}, 0)
			writeJSON(w, 200, map[string]any{
				"token":      tok,
				"childId":    cid,
				"familyId":   fid,
				"familyCode": fcode,
				"loginMode":  "account",
			})
			return
		}

		cid := strings.TrimSpace(mapStr(body, "childId"))
		fid, stCode, err := a.store.LoginChildByRoster(mapStr(body, "familyCode"), cid, mapStr(body, "pin"))
		if err != nil {
			if stCode == 0 {
				stCode = 400
			}
			writeJSON(w, stCode, map[string]string{"error": err.Error()})
			return
		}
		tok := SignToken(TokenClaims{Role: "child", ChildID: cid, FamilyID: fid}, 0)
		writeJSON(w, 200, map[string]any{"token": tok, "childId": cid, "familyId": fid, "loginMode": "roster"})
		return
	}

	/* ===== 2. 孩子端接口 ===== */
	if strings.HasPrefix(p, "/api/child/") {
		a.handleChildAPI(w, r)
		return
	}

	/* ===== 3. 家长端接口 ===== */
	if strings.HasPrefix(p, "/api/parent/") {
		a.handleParentAPI(w, r)
		return
	}

	/* ===== 4. 管理端接口 ===== */
	if strings.HasPrefix(p, "/api/admin/") {
		a.handleAdminAPI(w, r)
		return
	}

	writeJSON(w, 404, map[string]string{"error": "接口不存在"})
}

func (a *AppServer) serveReleaseAPK(w http.ResponseWriter, r *http.Request, rel *AppReleaseDTO) {
	if rel.FileName != "" && a.store.releasesDir != "" {
		localPath := filepath.Join(a.store.releasesDir, rel.ID+".apk")
		if _, err := os.Stat(localPath); err == nil {
			dlName := rel.FileName
			if !strings.HasSuffix(strings.ToLower(dlName), ".apk") {
				dlName = fmt.Sprintf("GrowthBuddyChild-%s.apk", rel.VersionName)
			}
			w.Header().Set("Content-Type", "application/vnd.android.package-archive")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, dlName))
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, localPath)
			return
		}
	}
	if strings.HasPrefix(rel.DownloadURL, "http://") || strings.HasPrefix(rel.DownloadURL, "https://") {
		http.Redirect(w, r, rel.DownloadURL, http.StatusFound)
		return
	}
	writeJSON(w, 404, map[string]string{"error": "APK 文件不存在"})
}

func (a *AppServer) handleChildAPI(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	if auth == nil {
		writeJSON(w, 401, map[string]string{"error": "请先选择小朋友身份登录"})
		return
	}
	targetChildID := auth.ChildID
	if auth.Role != "child" {
		targetChildID = strings.TrimSpace(r.URL.Query().Get("childId"))
	}
	if targetChildID == "" {
		writeJSON(w, 400, map[string]string{"error": "缺少 childId"})
		return
	}

	p := r.URL.Path
	m := r.Method

	if m == http.MethodGet && p == "/api/child/questions" {
		age, diff := a.store.GetChildQuizProfile(auth.FamilyID, targetChildID)
		if rawAge := strings.TrimSpace(r.URL.Query().Get("age")); rawAge != "" {
			if parsedAge, err := strconv.Atoi(rawAge); err == nil {
				age = NormalizeAge(parsedAge)
			}
		}
		if rawDiff := strings.TrimSpace(r.URL.Query().Get("difficulty")); rawDiff != "" {
			diff = NormalizeDifficulty(rawDiff)
		}
		list := QueryOnlineQuestions(age, diff, 24)
		ageGroup, ageLabel := AgeGroupForAge(age)
		writeJSON(w, 200, map[string]any{
			"age":              age,
			"age_group":        ageGroup,
			"age_label":        ageLabel,
			"difficulty":       diff,
			"difficulty_label": DifficultyLabelZh(diff),
			"questions":        list,
		})
		return
	}

	if m == http.MethodGet && p == "/api/child/state" {
		todayStr := sanitizeDate(r.URL.Query().Get("date"))
		st, err := a.store.GetChildState(auth.FamilyID, targetChildID, todayStr)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "孩子档案不存在"})
			return
		}
		writeJSON(w, 200, st)
		return
	}

	if m == http.MethodGet && p == "/api/child/letters" {
		list, unread := a.store.ListLettersForChild(auth.FamilyID, targetChildID)
		writeJSON(w, 200, map[string]any{"letters": list, "unread_count": unread})
		return
	}

	if m == http.MethodPost && p == "/api/child/letters" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		dto, err := a.store.SendChildLetter(
			auth.FamilyID,
			targetChildID,
			mapStr(body, "stamp"),
			mapStr(body, "title"),
			mapStr(body, "content"),
		)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"letter": dto})
		return
	}

	if m == http.MethodPost && p == "/api/child/letters/read" {
		body, _ := readJSONMap(r, 0)
		claimedXP, newXP, err := a.store.MarkChildLetterReadAndClaim(auth.FamilyID, targetChildID, mapStr(body, "letterId"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "claimed_xp": claimedXP, "new_xp": newXP})
		return
	}

	if m == http.MethodPost && p == "/api/child/checkin" {
		body, err := readJSONMap(r, 4*1024*1024)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		todayStr := sanitizeDate(mapStr(body, "date"))
		res, err := a.store.CheckinTask(auth.FamilyID, targetChildID, mapStr(body, "taskId"), todayStr, mapOptInt(body, "actualCount"), mapOptInt(body, "actualDuration"), mapStr(body, "photoData"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, res)
		return
	}

	if m == http.MethodPost && p == "/api/child/undo" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		todayStr := sanitizeDate(mapStr(body, "date"))
		undone, st, err := a.store.UndoLastCheckin(auth.FamilyID, targetChildID, todayStr)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"undone": undone, "state": st})
		return
	}

	if m == http.MethodPatch && p == "/api/child/profile" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		_, compSet := body["active_companion"]
		var compVal *string
		if compSet && body["active_companion"] != nil {
			s := mapStr(body, "active_companion")
			compVal = &s
		}
		err = a.store.UpdateChildProfile(auth.FamilyID, targetChildID, mapOptStr(body, "pet_name"), compSet, compVal, mapOptBool(body, "muted"))
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "孩子档案不存在"})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	// 纯原生安卓客户端便捷接口：静音 / 改宠名 / 切换伴宠
	if m == http.MethodPatch && p == "/api/child/mute" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		muted := mapBool(body, "muted")
		if err := a.store.UpdateChildProfile(auth.FamilyID, targetChildID, nil, false, nil, &muted); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodPatch && p == "/api/child/pet-name" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		name := mapStr(body, "petName")
		if err := a.store.UpdateChildProfile(auth.FamilyID, targetChildID, &name, false, nil, nil); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodPost && p == "/api/child/companion" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		spID := mapStr(body, "speciesId")
		var comp *string
		if spID != "" {
			comp = &spID
		}
		if err := a.store.UpdateChildProfile(auth.FamilyID, targetChildID, nil, true, comp, nil); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		st, _ := a.store.GetChildState(auth.FamilyID, targetChildID, serverToday())
		writeJSON(w, 200, map[string]any{"ok": true, "state": st})
		return
	}

	if m == http.MethodPost && p == "/api/child/adopt" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		todayStr := sanitizeDate(mapStr(body, "date"))
		sp, st, err := a.store.AdoptSpecies(auth.FamilyID, targetChildID, mapStr(body, "speciesId"), mapBool(body, "mystery"), todayStr)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"adopted": sp, "state": st})
		return
	}

	if m == http.MethodPost && p == "/api/child/avatar" {
		body, err := readJSONMap(r, 2*1024*1024)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		st, err := a.store.UpdateChildAvatar(auth.FamilyID, targetChildID, mapOptInt(body, "avatarIdx"), mapOptStr(body, "avatarData"), serverToday())
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "孩子档案不存在"})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "state": st})
		return
	}

	if m == http.MethodPost && p == "/api/child/polaroid" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		todayStr := sanitizeDate(mapStr(body, "date"))
		photo, st, err := a.store.TakePolaroid(auth.FamilyID, targetChildID, mapInt(body, "avatarIdx", 0), mapStr(body, "note"), todayStr)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"photo": photo, "state": st})
		return
	}

	writeJSON(w, 404, map[string]string{"error": "接口不存在"})
}

func (a *AppServer) handleParentAPI(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	if auth == nil || auth.Role != "parent" {
		writeJSON(w, 403, map[string]string{"error": "需要家长账号登录才能操作"})
		return
	}
	fid := auth.FamilyID
	p := r.URL.Path
	m := r.Method

	if m == http.MethodGet && p == "/api/parent/overview" {
		todayStr := sanitizeDate(r.URL.Query().Get("date"))
		ov, err := a.store.GetParentOverview(fid, auth.ParentID, todayStr)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, ov)
		return
	}

	if m == http.MethodPost && p == "/api/parent/avatar" {
		body, err := readJSONMap(r, 2*1024*1024)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		parent, err := a.store.UpdateParentAvatar(fid, auth.ParentID, mapOptInt(body, "avatarIdx"), mapOptStr(body, "avatarData"))
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "家长账号不存在"})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "parent": parent})
		return
	}

	if m == http.MethodPost && p == "/api/parent/children" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		created, err := a.store.CreateChild(fid, mapStr(body, "name"), mapInt(body, "avatar_idx", 0), mapStr(body, "pin"), mapStr(body, "username"), mapStr(body, "password"), mapInt(body, "age", 7), mapStr(body, "quiz_difficulty"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	if strings.HasPrefix(p, "/api/parent/children/") {
		cid := strings.TrimPrefix(p, "/api/parent/children/")
		if !strings.Contains(cid, "/") && cid != "" {
			if m == http.MethodPatch {
				body, err := readJSONMap(r, 2*1024*1024)
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				avIdx := mapOptInt(body, "avatar_idx")
				if avIdx == nil {
					avIdx = mapOptInt(body, "avatarIdx")
				}
				avData := mapOptStr(body, "avatar_data")
				if avData == nil {
					avData = mapOptStr(body, "avatarData")
				}
				updated, err := a.store.UpdateChild(fid, cid, UpdateChildInput{
					Name:           mapOptStr(body, "name"),
					AvatarIdx:      avIdx,
					AvatarData:     avData,
					Pin:            mapOptStr(body, "pin"),
					PetName:        mapOptStr(body, "pet_name"),
					Username:       mapOptStr(body, "username"),
					Password:       mapOptStr(body, "password"),
					Age:            mapOptInt(body, "age"),
					QuizDifficulty: mapOptStr(body, "quiz_difficulty"),
				})
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "孩子档案不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true, "child": updated})
				return
			}
			if m == http.MethodDelete {
				err := a.store.DeleteChild(fid, cid)
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "孩子档案不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
		}
	}

	if m == http.MethodPost && p == "/api/parent/parents" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		created, err := a.store.AddParent(fid, mapStr(body, "username"), mapStr(body, "displayName"), mapStr(body, "password"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	if m == http.MethodPost && p == "/api/parent/tasks/batch" {
		body, err := readJSONMap(r, 512*1024)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		rawTasks, _ := body["tasks"].([]any)
		inputs := make([]TaskInput, 0, len(rawTasks))
		for _, item := range rawTasks {
			tm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			cids, hasCids := mapStrSlice(tm, "childIds")
			inputs = append(inputs, TaskInput{
				Emoji:        mapOptStr(tm, "emoji"),
				Label:        mapOptStr(tm, "label"),
				Pts:          mapOptInt(tm, "pts"),
				Scope:        mapOptStr(tm, "scope"),
				TaskType:     mapOptStr(tm, "task_type"),
				DurationSec:  mapOptInt(tm, "duration_sec"),
				TargetCount:  mapOptInt(tm, "target_count"),
				TimerMode:    mapOptStr(tm, "timer_mode"),
				RequirePhoto: mapOptBool(tm, "require_photo"),
				RepeatMode:   mapOptStr(tm, "repeat_mode"),
				RepeatDays:   mapOptStr(tm, "repeat_days"),
				DueDate:      mapOptStr(tm, "due_date"),
				ChildIDs:     cids,
				HasChildIDs:  hasCids,
			})
		}
		ids, err := a.store.BatchCreateTasks(fid, inputs)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"ids": ids, "count": len(ids)})
		return
	}

	if m == http.MethodPost && p == "/api/parent/tasks" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		cids, hasCids := mapStrSlice(body, "childIds")
		id, err := a.store.CreateTask(fid, TaskInput{
			Emoji:        mapOptStr(body, "emoji"),
			IconURL:      mapOptStr(body, "icon_url"),
			Label:        mapOptStr(body, "label"),
			Pts:          mapOptInt(body, "pts"),
			Scope:        mapOptStr(body, "scope"),
			TaskType:     mapOptStr(body, "task_type"),
			DurationSec:  mapOptInt(body, "duration_sec"),
			TargetCount:  mapOptInt(body, "target_count"),
			TimerMode:    mapOptStr(body, "timer_mode"),
			RequirePhoto: mapOptBool(body, "require_photo"),
			RepeatMode:   mapOptStr(body, "repeat_mode"),
			RepeatDays:   mapOptStr(body, "repeat_days"),
			DueDate:      mapOptStr(body, "due_date"),
			ChildIDs:     cids,
			HasChildIDs:  hasCids,
		})
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"id": id})
		return
	}

	if strings.HasPrefix(p, "/api/parent/tasks/") {
		tid := strings.TrimPrefix(p, "/api/parent/tasks/")
		if !strings.Contains(tid, "/") && tid != "" {
			if m == http.MethodPatch {
				body, err := readJSONMap(r, 0)
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				cids, hasCids := mapStrSlice(body, "childIds")
				err = a.store.UpdateTask(fid, tid, TaskInput{
					Emoji:        mapOptStr(body, "emoji"),
					IconURL:      mapOptStr(body, "icon_url"),
					Label:        mapOptStr(body, "label"),
					Pts:          mapOptInt(body, "pts"),
					Scope:        mapOptStr(body, "scope"),
					TaskType:     mapOptStr(body, "task_type"),
					DurationSec:  mapOptInt(body, "duration_sec"),
					TargetCount:  mapOptInt(body, "target_count"),
					TimerMode:    mapOptStr(body, "timer_mode"),
					RequirePhoto: mapOptBool(body, "require_photo"),
					RepeatMode:   mapOptStr(body, "repeat_mode"),
					RepeatDays:   mapOptStr(body, "repeat_days"),
					DueDate:      mapOptStr(body, "due_date"),
					Enabled:      mapOptBool(body, "enabled"),
					ChildIDs:     cids,
					HasChildIDs:  hasCids,
				})
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
			if m == http.MethodDelete {
				if err := a.store.DeleteTask(fid, tid); err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
		}
	}

	if m == http.MethodPost && p == "/api/parent/grant-egg" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		res, err := a.store.GrantEgg(fid, auth.ParentID, mapStr(body, "childId"), mapInt(body, "amount", 1), mapStr(body, "reason"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, res)
		return
	}

	if m == http.MethodGet && p == "/api/parent/checkin-logs" {
		q := r.URL.Query()
		res, err := a.store.GetParentCheckinLogs(
			fid,
			strings.TrimSpace(q.Get("childId")),
			strings.TrimSpace(q.Get("period")),
			sanitizeDate(q.Get("date")),
		)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, res)
		return
	}

	if m == http.MethodPost && p == "/api/parent/checkin" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		todayStr := sanitizeDate(mapStr(body, "date"))
		if err := a.store.ParentCheckin(fid, mapStr(body, "childId"), mapStr(body, "taskId"), todayStr); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodPost && p == "/api/parent/undo-checkin" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		taskID := mapStr(body, "taskId")
		var undone *CheckinLogDTO
		if taskID != "" {
			undone, err = a.store.UndoTaskCheckin(fid, mapStr(body, "childId"), taskID, sanitizeDate(mapStr(body, "date")))
		} else {
			undone, _, err = a.store.UndoLastCheckin(fid, mapStr(body, "childId"), "")
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"undone": undone})
		return
	}

	if m == http.MethodPost && p == "/api/parent/species" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		var targetChild *string
		if s := strings.TrimSpace(mapStr(body, "target_child_id")); s != "" {
			targetChild = &s
		}
		res, err := a.store.CreateCustomSpecies(fid, Species{
			Name:          mapStr(body, "name"),
			Color:         mapStr(body, "color"),
			Ear:           mapStr(body, "ear"),
			Acc:           mapStr(body, "acc"),
			Tail:          mapStr(body, "tail"),
			Mouth:         mapStr(body, "mouth"),
			Base:          mapStr(body, "base"),
			Hint:          mapStr(body, "hint"),
			TargetChildID: targetChild,
		})
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, res)
		return
	}

	if strings.HasPrefix(p, "/api/parent/species/") && m == http.MethodDelete {
		sid := strings.TrimPrefix(p, "/api/parent/species/")
		err := a.store.DeleteCustomSpecies(fid, sid)
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "自定义伙伴不存在（内置伙伴不可下架）"})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodGet && p == "/api/parent/task-icons" {
		list, err := a.store.ListFamilyTaskIcons(fid)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"icons": list})
		return
	}

	if m == http.MethodPost && p == "/api/parent/task-icons" {
		body, err := readJSONMap(r, 2*1024*1024)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		ico, err := a.store.AddFamilyTaskIcon(fid, mapStr(body, "label"), mapStr(body, "category"), mapStr(body, "image_data"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"icon": ico})
		return
	}

	if strings.HasPrefix(p, "/api/parent/task-icons/") && m == http.MethodDelete {
		iconID := strings.TrimPrefix(p, "/api/parent/task-icons/")
		if err := a.store.DeleteFamilyTaskIcon(fid, iconID); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodGet && p == "/api/parent/questions" {
		age := 7
		if rawAge := strings.TrimSpace(r.URL.Query().Get("age")); rawAge != "" {
			if parsedAge, err := strconv.Atoi(rawAge); err == nil {
				age = NormalizeAge(parsedAge)
			}
		}
		diff := NormalizeDifficulty(r.URL.Query().Get("difficulty"))
		list := QueryOnlineQuestions(age, diff, 18)
		ageGroup, ageLabel := AgeGroupForAge(age)
		writeJSON(w, 200, map[string]any{
			"age":              age,
			"age_group":        ageGroup,
			"age_label":        ageLabel,
			"difficulty":       diff,
			"difficulty_label": DifficultyLabelZh(diff),
			"total_curated":    len(onlineCuratedBank),
			"questions":        list,
		})
		return
	}

	if m == http.MethodPost && p == "/api/parent/questions" {
		var q QuizQuestion
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q); err != nil {
			writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
			return
		}
		created, err := a.store.AddFamilyQuestion(fid, q)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"question": created})
		return
	}

	if strings.HasPrefix(p, "/api/parent/questions/") && m == http.MethodDelete {
		qid := strings.TrimPrefix(p, "/api/parent/questions/")
		if err := a.store.DeleteFamilyQuestion(fid, qid); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodGet && p == "/api/parent/letters" {
		cid := strings.TrimSpace(r.URL.Query().Get("childId"))
		list, unread := a.store.ListLettersForParent(fid, cid)
		writeJSON(w, 200, map[string]any{"letters": list, "unread_from_child": unread})
		return
	}

	if m == http.MethodPost && p == "/api/parent/letters" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		created, err := a.store.SendParentLetter(
			fid,
			auth.ParentID,
			mapStr(body, "childId"),
			mapStr(body, "stamp"),
			mapStr(body, "title"),
			mapStr(body, "content"),
			mapInt(body, "bonus_xp", 0),
		)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"letters": created, "count": len(created)})
		return
	}

	if m == http.MethodPost && p == "/api/parent/letters/read" {
		body, _ := readJSONMap(r, 0)
		if err := a.store.MarkParentLettersRead(fid, mapStr(body, "letterId"), mapStr(body, "childId")); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if strings.HasPrefix(p, "/api/parent/letters/") && m == http.MethodDelete {
		lid := strings.TrimPrefix(p, "/api/parent/letters/")
		if err := a.store.DeleteFamilyLetter(fid, lid); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodPost && p == "/api/parent/import-backup" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		res, err := a.store.ImportLegacyBackup(fid, mapStr(body, "childId"), mapStr(body, "backupCode"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, res)
		return
	}

	writeJSON(w, 404, map[string]string{"error": "接口不存在"})
}

func (a *AppServer) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	auth := getAuth(r)
	if auth == nil || auth.Role != "admin" {
		writeJSON(w, 403, map[string]string{"error": "需要系统管理员身份权限"})
		return
	}
	p := r.URL.Path
	m := r.Method

	if m == http.MethodGet && p == "/api/admin/overview" {
		todayStr := sanitizeDate(r.URL.Query().Get("date"))
		ov, err := a.store.GetAdminOverview(todayStr)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		ov["admin"] = a.store.GetAdminByID(auth.AdminID)
		writeJSON(w, 200, ov)
		return
	}

	if m == http.MethodPost && p == "/api/admin/change-password" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if err := a.store.ChangeAdminPassword(auth.AdminID, mapStr(body, "oldPassword"), mapStr(body, "newPassword")); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	if m == http.MethodPost && p == "/api/admin/families" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		created, err := a.store.RegisterFamily(RegisterFamilyInput{
			FamilyName:    mapStr(body, "familyName"),
			Username:      mapStr(body, "username"),
			DisplayName:   mapStr(body, "displayName"),
			Password:      mapStr(body, "password"),
			ChildName:     mapStr(body, "childName"),
			ChildUsername: mapStr(body, "childUsername"),
			ChildPassword: mapStr(body, "childPassword"),
		})
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	if strings.HasPrefix(p, "/api/admin/families/") {
		famID := strings.TrimPrefix(p, "/api/admin/families/")
		if !strings.Contains(famID, "/") && famID != "" {
			if m == http.MethodPatch {
				body, err := readJSONMap(r, 0)
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				updated, err := a.store.AdminUpdateFamily(famID, mapStr(body, "name"))
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "家庭不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, updated)
				return
			}
			if m == http.MethodDelete {
				err := a.store.AdminDeleteFamily(famID)
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "家庭不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
		}
	}

	if m == http.MethodPost && p == "/api/admin/parents" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		famID := strings.TrimSpace(mapStr(body, "familyId"))
		if !a.store.FamilyExists(famID) {
			writeJSON(w, 404, map[string]string{"error": "目标家庭不存在"})
			return
		}
		created, err := a.store.AddParent(famID, mapStr(body, "username"), mapStr(body, "displayName"), mapStr(body, "password"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	if strings.HasPrefix(p, "/api/admin/parents/") {
		pid := strings.TrimPrefix(p, "/api/admin/parents/")
		if !strings.Contains(pid, "/") && pid != "" {
			if m == http.MethodPatch {
				body, err := readJSONMap(r, 0)
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				updated, err := a.store.AdminUpdateParent(pid, mapOptStr(body, "username"), mapOptStr(body, "displayName"), mapOptStr(body, "password"))
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "家长账号不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, updated)
				return
			}
			if m == http.MethodDelete {
				err := a.store.AdminDeleteParent(pid)
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "家长账号不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
		}
	}

	if m == http.MethodPost && p == "/api/admin/children" {
		body, err := readJSONMap(r, 0)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		famID := strings.TrimSpace(mapStr(body, "familyId"))
		if !a.store.FamilyExists(famID) {
			writeJSON(w, 404, map[string]string{"error": "目标家庭不存在"})
			return
		}
		created, err := a.store.CreateChild(famID, mapStr(body, "name"), mapInt(body, "avatar_idx", 0), mapStr(body, "pin"), mapStr(body, "username"), mapStr(body, "password"), mapInt(body, "age", 7), mapStr(body, "quiz_difficulty"))
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	if strings.HasPrefix(p, "/api/admin/children/") {
		cid := strings.TrimPrefix(p, "/api/admin/children/")
		if !strings.Contains(cid, "/") && cid != "" {
			if m == http.MethodPatch {
				body, err := readJSONMap(r, 0)
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				updated, err := a.store.AdminUpdateChild(cid, UpdateChildInput{
					Name:           mapOptStr(body, "name"),
					AvatarIdx:      mapOptInt(body, "avatar_idx"),
					Pin:            mapOptStr(body, "pin"),
					PetName:        mapOptStr(body, "pet_name"),
					Username:       mapOptStr(body, "username"),
					Password:       mapOptStr(body, "password"),
					Age:            mapOptInt(body, "age"),
					QuizDifficulty: mapOptStr(body, "quiz_difficulty"),
					XP:             mapOptInt(body, "xp"),
					Coupons:        mapOptInt(body, "coupons"),
				})
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "儿童档案不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true, "child": updated})
				return
			}
			if m == http.MethodDelete {
				err := a.store.AdminDeleteChild(cid)
				if errors.Is(err, ErrNotFound) {
					writeJSON(w, 404, map[string]string{"error": "儿童档案不存在"})
					return
				}
				if err != nil {
					writeJSON(w, 400, map[string]string{"error": err.Error()})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
		}
	}

	/* ===== 5. 管理端：App 版本发布与 OTA 远程升级管理 ===== */
	if m == http.MethodGet && p == "/api/admin/releases" {
		list, err := a.store.ListReleases("")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"releases": list})
		return
	}

	if m == http.MethodPost && p == "/api/admin/releases" {
		a.handleAdminCreateRelease(w, r)
		return
	}

	if strings.HasPrefix(p, "/api/admin/releases/") && m == http.MethodDelete {
		id := strings.TrimPrefix(p, "/api/admin/releases/")
		err := a.store.DeleteRelease(id)
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "版本记录不存在"})
			return
		}
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}

	writeJSON(w, 404, map[string]string{"error": "接口不存在"})
}

func (a *AppServer) handleAdminCreateRelease(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		const maxUpload = 150 << 20 // 150 MB
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			writeJSON(w, 400, map[string]string{"error": "上传包解析失败（单文件最大 150 MB）"})
			return
		}
		defer func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}()

		vCode, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("versionCode")))
		vName := strings.TrimSpace(r.FormValue("versionName"))
		notes := strings.TrimSpace(r.FormValue("releaseNotes"))
		force := strings.EqualFold(r.FormValue("forceUpdate"), "true") || r.FormValue("forceUpdate") == "1" || r.FormValue("forceUpdate") == "on"
		if vName == "" {
			loc := time.FixedZone("CST", 8*3600)
			vName = time.Now().In(loc).Format("2006.01.02-1504")
		}
		if vCode <= 0 {
			vCode = autoVersionCodeFromName(vName)
		}

		file, header, err := r.FormFile("apk")
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "请选择要上传的 .apk 安装包文件"})
			return
		}
		defer file.Close()

		relID := GenID("rel_")
		if err := os.MkdirAll(a.store.releasesDir, 0o755); err != nil {
			writeJSON(w, 500, map[string]string{"error": "无法创建安装包存储目录"})
			return
		}
		destPath := filepath.Join(a.store.releasesDir, relID+".apk")
		out, err := os.Create(destPath)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "无法保存安装包文件"})
			return
		}
		hasher := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(out, hasher), file)
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			_ = os.Remove(destPath)
			writeJSON(w, 500, map[string]string{"error": "写入安装包失败"})
			return
		}

		origName := filepath.Base(header.Filename)
		if origName == "" || origName == "." || !strings.HasSuffix(strings.ToLower(origName), ".apk") {
			origName = fmt.Sprintf("GrowthBuddyChild-%s.apk", vName)
		}

		created, err := a.store.CreateRelease(AppReleaseDTO{
			ID:           relID,
			Platform:     "android",
			VersionCode:  vCode,
			VersionName:  vName,
			ReleaseNotes: notes,
			ForceUpdate:  force,
			FileName:     origName,
			FileSize:     written,
			SHA256:       hex.EncodeToString(hasher.Sum(nil)),
			DownloadURL:  "/api/app/download/" + relID,
		})
		if err != nil {
			_ = os.Remove(destPath)
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, created)
		return
	}

	// JSON 模式：支持外部链接发布（如 OSS / CDN / Google Drive 直链）
	body, err := readJSONMap(r, 0)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	dlURL := strings.TrimSpace(mapStr(body, "downloadUrl"))
	if !strings.HasPrefix(dlURL, "http://") && !strings.HasPrefix(dlURL, "https://") {
		writeJSON(w, 400, map[string]string{"error": "请上传 APK 文件，或填写以 http(s):// 开头的外部下载地址"})
		return
	}
	vName := strings.TrimSpace(mapStr(body, "versionName"))
	if vName == "" {
		loc := time.FixedZone("CST", 8*3600)
		vName = time.Now().In(loc).Format("2006.01.02-1504")
	}
	vCode := mapInt(body, "versionCode", 0)
	if vCode <= 0 {
		vCode = autoVersionCodeFromName(vName)
	}
	created, err := a.store.CreateRelease(AppReleaseDTO{
		Platform:     "android",
		VersionCode:  vCode,
		VersionName:  vName,
		ReleaseNotes: mapStr(body, "releaseNotes"),
		ForceUpdate:  mapBool(body, "forceUpdate"),
		FileSize:     int64(mapInt(body, "fileSize", 0)),
		SHA256:       strings.ToLower(strings.TrimSpace(mapStr(body, "sha256"))),
		DownloadURL:  dlURL,
	})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, created)
}
