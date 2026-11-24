package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 12 * time.Second}
var downloadClient = &http.Client{Timeout: 5 * time.Minute}

type FishReleaseDTO struct {
	ID           string `json:"id"`
	Platform     string `json:"platform"`
	VersionCode  int    `json:"version_code"`
	VersionName  string `json:"version_name"`
	ReleaseNotes string `json:"release_notes"`
	ForceUpdate  bool   `json:"force_update"`
	FileName     string `json:"file_name"`
	FileSize     int64  `json:"file_size"`
	SHA256       string `json:"sha256"`
	DownloadURL  string `json:"download_url"`
	CreatedAt    int64  `json:"created_at"`
}

type FishVersionResponse struct {
	HasRelease  bool            `json:"hasRelease"`
	HasUpdate   bool            `json:"hasUpdate"`
	ForceUpdate bool            `json:"forceUpdate"`
	Release     *FishReleaseDTO `json:"release"`
}

type LoginResult struct {
	Token       string
	Role        string
	ActorID     string
	DisplayName string
}

type FamilyGroupInfo struct {
	ID   string
	Name string
}

type ChatMsg struct {
	ID         string `json:"id"`
	SenderName string `json:"sender_name"`
	IsSelf     bool   `json:"is_self"`
	MsgType    string `json:"msg_type"`
	Text       string `json:"text"`
	CreatedAt  int64  `json:"created_at"`
}

func apiRequest(serverURL, method, path, token string, body any) (int, []byte, error) {
	base := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if base == "" {
		base = defaultServerURL
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024*1024))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if resp.StatusCode >= 400 {
		var errObj struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errObj)
		if errObj.Error != "" {
			return resp.StatusCode, raw, errors.New(errObj.Error)
		}
		return resp.StatusCode, raw, errors.New("请求失败 (" + strconv.Itoa(resp.StatusCode) + ")")
	}
	return resp.StatusCode, raw, nil
}

func doLogin(serverURL, username, password string) (*LoginResult, error) {
	_, raw, err := apiRequest(serverURL, "POST", "/api/auth/login", "", map[string]string{
		"username": strings.TrimSpace(username),
		"password": password,
	})
	if err != nil {
		return nil, err
	}
	var res struct {
		Role      string `json:"role"`
		Token     string `json:"token"`
		ChildID   string `json:"childId"`
		ChildName string `json:"childName"`
		Parent    *struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			Name        string `json:"name"`
		} `json:"parent"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	if res.Token == "" {
		return nil, errors.New("登录未返回有效凭证")
	}
	out := &LoginResult{
		Token: res.Token,
		Role:  res.Role,
	}
	if res.Role == "parent" {
		if res.Parent != nil {
			out.ActorID = res.Parent.ID
			out.DisplayName = res.Parent.DisplayName
			if out.DisplayName == "" {
				out.DisplayName = res.Parent.Name
			}
		}
		if out.DisplayName == "" {
			out.DisplayName = "家长"
		}
	} else {
		out.Role = "child"
		out.ActorID = res.ChildID
		out.DisplayName = res.ChildName
		if out.DisplayName == "" {
			out.DisplayName = "孩子"
		}
	}
	return out, nil
}

func fetchDefaultFamilyGroup(serverURL, role, token string) (*FamilyGroupInfo, int, error) {
	endpoint := "/api/child/friends"
	if role == "parent" {
		endpoint = "/api/parent/friends"
	}
	code, raw, err := apiRequest(serverURL, "GET", endpoint, token, nil)
	if err != nil {
		return nil, code, err
	}
	var ov struct {
		ChatGroups []struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			IsFamilyDefault bool   `json:"is_family_default"`
		} `json:"chat_groups"`
	}
	if err := json.Unmarshal(raw, &ov); err != nil {
		return nil, code, err
	}
	if len(ov.ChatGroups) == 0 {
		return nil, code, errors.New("未找到家庭群聊")
	}
	chosen := ov.ChatGroups[0]
	for _, g := range ov.ChatGroups {
		if g.IsFamilyDefault {
			chosen = g
			break
		}
	}
	return &FamilyGroupInfo{ID: chosen.ID, Name: chosen.Name}, code, nil
}

func fetchGroupMessages(serverURL, role, token, groupID string, afterMs int64, maxMessages ...int) ([]ChatMsg, int, error) {
	basePath := "/api/child/chat/messages"
	if role == "parent" {
		basePath = "/api/parent/chat/messages"
	}
	q := basePath + "?peerType=group&peerId=" + url.QueryEscape(groupID)
	if afterMs > 0 {
		q += "&after=" + strconv.FormatInt(afterMs, 10)
	}
	if len(maxMessages) > 0 {
		if maxMessages[0] <= 0 {
			q += "&limit=all"
		} else {
			q += "&limit=" + strconv.Itoa(maxMessages[0])
		}
	}
	code, raw, err := apiRequest(serverURL, "GET", q, token, nil)
	if err != nil {
		return nil, code, err
	}
	var res struct {
		Messages []ChatMsg `json:"messages"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, code, err
	}
	return res.Messages, code, nil
}

func sendGroupText(serverURL, role, token, groupID, text string) (*ChatMsg, int, error) {
	basePath := "/api/child/chat/messages"
	if role == "parent" {
		basePath = "/api/parent/chat/messages"
	}
	code, raw, err := apiRequest(serverURL, "POST", basePath, token, map[string]any{
		"peerType": "group",
		"peerId":   groupID,
		"msgType":  "text",
		"text":     text,
	})
	if err != nil {
		return nil, code, err
	}
	var res struct {
		Message ChatMsg `json:"message"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, code, err
	}
	return &res.Message, code, nil
}

func resolveDownloadURL(serverURL, rawURL string) string {
	u := strings.TrimSpace(rawURL)
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	base := strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if base == "" {
		base = defaultServerURL
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return base + u
}

func checkFishUpdate(serverURL string, currentVersionCode int) (*FishVersionResponse, error) {
	q := "/api/app/version?platform=fish&currentVersionCode=" + strconv.Itoa(currentVersionCode)
	_, raw, err := apiRequest(serverURL, "GET", q, "", nil)
	if err != nil {
		return nil, err
	}
	var res FishVersionResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func downloadFishUpdateToFile(serverURL string, rel FishReleaseDTO, destPath string) error {
	dlURL := resolveDownloadURL(serverURL, rel.DownloadURL)
	req, err := http.NewRequest("GET", dlURL, nil)
	if err != nil {
		return err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return errors.New("下载更新包失败 (" + strconv.Itoa(resp.StatusCode) + ")")
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmpPath := destPath + ".part"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(resp.Body, 200<<20))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmpPath)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}

	gotSHA := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	expectedSHA := strings.ToLower(strings.TrimSpace(rel.SHA256))
	if expectedSHA != "" && gotSHA != expectedSHA {
		_ = os.Remove(tmpPath)
		return errors.New("更新包 SHA-256 校验失败")
	}

	_ = os.Remove(destPath)
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}
