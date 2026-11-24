package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultServerURL = "https://linuu.xyz"

type FishConfig struct {
	ServerURL      string `json:"server_url"`
	Token          string `json:"token"`
	Role           string `json:"role"`
	DisplayName    string `json:"display_name"`
	ActorID        string `json:"actor_id"`
	GroupID        string `json:"group_id"`
	GroupName      string `json:"group_name"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	X              int    `json:"x"`
	Y              int    `json:"y"`
	HasPos         bool   `json:"has_pos"`
	BgColor        string `json:"bg_color"`
	BgTransparent  bool   `json:"bg_transparent"`
	AutoInvertText bool   `json:"auto_invert_text"`
	FontColor      string `json:"font_color"`
	FontSize       int    `json:"font_size"`
	MaxMessages    int    `json:"max_messages"` // 100, 500, or 0 (unlimited)
	Hotkey         string `json:"hotkey"`
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
	return cfg
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
