package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultServerURL = "https://linuu.xyz"

type SavedAccount struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	ActorID     string `json:"actor_id"`
	DisplayName string `json:"display_name"`
	Token       string `json:"token"`
	GroupID     string `json:"group_id"`
	GroupName   string `json:"group_name"`
	LastLoginAt int64  `json:"last_login_at"`
}

type FishConfig struct {
	ServerURL      string         `json:"server_url"`
	Username       string         `json:"username"`
	Token          string         `json:"token"`
	Role           string         `json:"role"`
	DisplayName    string         `json:"display_name"`
	ActorID        string         `json:"actor_id"`
	GroupID        string         `json:"group_id"`
	GroupName      string         `json:"group_name"`
	SavedAccounts  []SavedAccount `json:"saved_accounts,omitempty"`
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	X              int            `json:"x"`
	Y              int            `json:"y"`
	HasPos         bool           `json:"has_pos"`
	BgColor        string         `json:"bg_color"`
	BgTransparent  bool           `json:"bg_transparent"`
	AutoInvertText bool           `json:"auto_invert_text"`
	FontColor      string         `json:"font_color"`
	FontSize       int            `json:"font_size"`
	MaxMessages    int            `json:"max_messages"` // 100, 500, or 0 (unlimited)
	Hotkey         string         `json:"hotkey"`
	ClearHotkey    string         `json:"clear_hotkey"`
	ClearBeforeTs  int64          `json:"clear_before_ts,omitempty"`
}

func defaultConfig() FishConfig {
	return FishConfig{
		ServerURL:      defaultServerURL,
		Width:          380,
		Height:         520,
		BgColor:        "#FAF6EE",
		BgTransparent:  true,
		AutoInvertText: true,
		FontColor:      "#233128",
		FontSize:       15,
		MaxMessages:    100,
		Hotkey:         "Alt+H",
		ClearHotkey:    "Alt+L",
	}
}

func configPath() string {
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "GrowthBuddyFish", "config.json")
}

func loadConfig() FishConfig {
	cfg := defaultConfig()
	raw, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(raw, &cfg)
	cfg.ServerURL = defaultServerURL
	if cfg.Width < 260 {
		cfg.Width = 380
	}
	if cfg.Height < 240 {
		cfg.Height = 520
	}
	if cfg.FontSize < 12 || cfg.FontSize > 26 {
		cfg.FontSize = 15
	}
	if !isHexColor(cfg.BgColor) {
		cfg.BgColor = "#FAF6EE"
	}
	if !isHexColor(cfg.FontColor) {
		cfg.FontColor = "#233128"
	}
	cfg.MaxMessages = normalizeMaxMessages(cfg.MaxMessages)
	_, _, cfg.Hotkey = parseHotkey(cfg.Hotkey)
	_, _, cfg.ClearHotkey = parseClearHotkey(cfg.ClearHotkey)
	if cfg.ClearBeforeTs < 0 {
		cfg.ClearBeforeTs = 0
	}
	if cfg.Token != "" && cfg.GroupID != "" && (cfg.ActorID != "" || cfg.Username != "") {
		cfg.upsertSavedAccount(SavedAccount{
			Username:    cfg.Username,
			Role:        cfg.Role,
			ActorID:     cfg.ActorID,
			DisplayName: cfg.DisplayName,
			Token:       cfg.Token,
			GroupID:     cfg.GroupID,
			GroupName:   cfg.GroupName,
		})
	}
	return cfg
}

func sameSavedAccount(a, b SavedAccount) bool {
	if a.ActorID != "" && b.ActorID != "" && a.Role == b.Role && a.ActorID == b.ActorID {
		return true
	}
	if a.Username != "" && b.Username != "" && strings.EqualFold(a.Username, b.Username) {
		return true
	}
	return false
}

func (c *FishConfig) upsertSavedAccount(acc SavedAccount) {
	acc.Username = strings.TrimSpace(acc.Username)
	acc.DisplayName = strings.TrimSpace(acc.DisplayName)
	if acc.ActorID == "" && acc.Username == "" {
		return
	}
	next := make([]SavedAccount, 0, len(c.SavedAccounts)+1)
	for _, old := range c.SavedAccounts {
		if sameSavedAccount(old, acc) {
			if acc.Username == "" {
				acc.Username = old.Username
			}
			if acc.DisplayName == "" {
				acc.DisplayName = old.DisplayName
			}
			if acc.LastLoginAt == 0 {
				acc.LastLoginAt = old.LastLoginAt
			}
			continue
		}
		next = append(next, old)
	}
	// Put updated active account at front
	out := append([]SavedAccount{acc}, next...)
	if len(out) > 15 {
		out = out[:15]
	}
	c.SavedAccounts = out
}

func (c *FishConfig) clearSavedAccountToken(role, actorID, username string) {
	target := SavedAccount{Role: role, ActorID: actorID, Username: strings.TrimSpace(username)}
	for i := range c.SavedAccounts {
		if sameSavedAccount(c.SavedAccounts[i], target) {
			c.SavedAccounts[i].Token = ""
		}
	}
}

func (c *FishConfig) isActiveSavedAccount(acc SavedAccount) bool {
	if c.Token == "" {
		return false
	}
	return sameSavedAccount(acc, SavedAccount{
		Role:     c.Role,
		ActorID:  c.ActorID,
		Username: c.Username,
	})
}

func savedAccountLabel(acc SavedAccount) string {
	name := strings.TrimSpace(acc.DisplayName)
	if name == "" {
		name = strings.TrimSpace(acc.Username)
	}
	if name == "" {
		name = "已保存账号"
	}
	roleName := "成员"
	if acc.Role == "parent" {
		roleName = "家长"
	} else if acc.Role == "child" {
		roleName = "孩子"
	}
	suffix := roleName
	if acc.Username != "" && !strings.EqualFold(acc.Username, name) {
		suffix += " · " + acc.Username
	}
	if acc.Token == "" {
		suffix += " · 需登录"
	}
	return name + " (" + suffix + ")"
}

func normalizeMaxMessages(v int) int {
	switch v {
	case 0, 100, 500:
		return v
	default:
		return 100
	}
}

func parseHotkey(raw string) (mods uint32, vk uint32, canonical string) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "ALT+Q":
		return 0x0001, 0x51, "Alt+Q"
	case "CTRL+ALT+H":
		return 0x0001 | 0x0002, 0x48, "Ctrl+Alt+H"
	case "CTRL+SHIFT+H":
		return 0x0002 | 0x0004, 0x48, "Ctrl+Shift+H"
	case "F9":
		return 0, 0x78, "F9"
	default:
		return 0x0001, 0x48, "Alt+H"
	}
}

func parseClearHotkey(raw string) (mods uint32, vk uint32, canonical string) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "ALT+C":
		return 0x0001, 0x43, "Alt+C"
	case "CTRL+ALT+L":
		return 0x0001 | 0x0002, 0x4C, "Ctrl+Alt+L"
	case "F8":
		return 0, 0x77, "F8"
	default:
		return 0x0001, 0x4C, "Alt+L"
	}
}

func filterMessagesAfterClear(msgs []ChatMsg, clearBeforeTs int64) []ChatMsg {
	if clearBeforeTs <= 0 || len(msgs) == 0 {
		return msgs
	}
	out := make([]ChatMsg, 0, len(msgs))
	for _, m := range msgs {
		if m.CreatedAt > clearBeforeTs {
			out = append(out, m)
		}
	}
	return out
}

func saveConfig(cfg FishConfig) {
	p := configPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0600)
}

func isHexColor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(s[1:], 16, 32)
	return err == nil
}

// hexToCOLORREF converts "#RRGGBB" to Win32 COLORREF (0x00BBGGRR).
func hexToCOLORREF(s string, fallback uint32) uint32 {
	if !isHexColor(s) {
		return fallback
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return fallback
	}
	r := (v >> 16) & 0xFF
	g := (v >> 8) & 0xFF
	b := v & 0xFF
	return uint32((b << 16) | (g << 8) | r)
}

func colorrefToHex(c uint32) string {
	r := c & 0xFF
	g := (c >> 8) & 0xFF
	b := (c >> 16) & 0xFF
	return "#" + padHex2(r) + padHex2(g) + padHex2(b)
}

func padHex2(v uint32) string {
	s := strings.ToUpper(strconv.FormatUint(uint64(v&0xFF), 16))
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
