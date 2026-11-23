package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type LevelSpec struct {
	XP   int    `json:"xp"`
	Name string `json:"name"`
	Talk string `json:"talk"`
}

var Levels = []LevelSpec{
	{XP: 0, Name: "一颗蛋", Talk: "抱抱我，完成一件小事我就会慢慢破壳哦。"},
	{XP: 10, Name: "破壳啦", Talk: "哇，外面的世界好暖和！快带我去看看吧。"},
	{XP: 30, Name: "幼年体", Talk: "我已经长出小耳朵啦！今天你完成了什么呀？"},
	{XP: 60, Name: "少年体", Talk: "听说图鉴里有好多好朋友，带一位出来陪我玩吧！"},
	{XP: 120, Name: "成年体", Talk: "你好呀老朋友！神秘蛋也解锁啦，一起去看看惊喜吧。"},
	{XP: 200, Name: "守护兽", Talk: "守护兽达成！我们一起记录每一天的成长闪光吧！"},
}

type DefaultTaskSpec struct {
	Emoji string
	Label string
	Pts   int
}

var DefaultTasks = []DefaultTaskSpec{
	{Emoji: "📚", Label: "完成学习块", Pts: 3},
	{Emoji: "📖", Label: "阅读 20 分钟", Pts: 5},
	{Emoji: "🌳", Label: "户外 1 小时", Pts: 5},
	{Emoji: "🌙", Label: "按时上床", Pts: 3},
	{Emoji: "🎒", Label: "自己整理书包", Pts: 2},
	{Emoji: "🌟", Label: "额外好表现（家长自定）", Pts: 5},
}

type Species struct {
	ID            string  `json:"id"`
	FamilyID      string  `json:"family_id,omitempty"`
	Name          string  `json:"name"`
	Color         string  `json:"color"`
	Ear           string  `json:"ear"`
	Acc           string  `json:"acc"`
	Tail          string  `json:"tail"`
	Mouth         string  `json:"mouth"`
	Base          string  `json:"base"`
	Ahoge         int     `json:"ahoge"`
	Hint          string  `json:"hint"`
	TargetChildID *string `json:"target_child_id,omitempty"`
	AddedAt       int64   `json:"added_at"`
}

var BuiltinSpecies = []Species{
	{ID: "b1", Name: "小团子", Color: "#ec9844", Ear: "round", Acc: "none", Tail: "puff", Mouth: "smile", Base: "grass", Ahoge: 1, Hint: "最初的伙伴", AddedAt: 0},
	{ID: "b2", Name: "雪球", Color: "#f2eee4", Ear: "tall", Acc: "scarf", Tail: "puff", Mouth: "buck", Base: "snow", Ahoge: 0, Hint: "冬天上线", AddedAt: 0},
	{ID: "b3", Name: "桔子", Color: "#ec9139", Ear: "pointy", Acc: "none", Tail: "fox", Mouth: "cat", Base: "grass", Ahoge: 0, Hint: "机灵鬼", AddedAt: 0},
	{ID: "b4", Name: "墨墨", Color: "#7f818f", Ear: "pointy", Acc: "star", Tail: "fox", Mouth: "smile", Base: "cloud", Ahoge: 0, Hint: "夜行侠", AddedAt: 0},
	{ID: "b5", Name: "果果", Color: "#6ea854", Ear: "horn", Acc: "none", Tail: "dragon", Mouth: "smile", Base: "grass", Ahoge: 0, Hint: "史前来客", AddedAt: 0},
	{ID: "b6", Name: "汤圆", Color: "#4c5261", Ear: "none", Acc: "bow", Tail: "puff", Mouth: "smile", Base: "snow", Ahoge: 0, Hint: "南极选民", AddedAt: 0},
	{ID: "b7", Name: "糖糖", Color: "#f2a8bc", Ear: "unicorn", Acc: "star", Tail: "wing", Mouth: "smile", Base: "cloud", Ahoge: 0, Hint: "梦幻限定", AddedAt: 0},
	{ID: "b8", Name: "焰焰", Color: "#d96338", Ear: "horn", Acc: "crown", Tail: "wing", Mouth: "fang", Base: "sand", Ahoge: 0, Hint: "传说守护", AddedAt: 0},
}

func LevelForXP(xp int) int {
	lv := 0
	for i, l := range Levels {
		if xp >= l.XP {
			lv = i
		}
	}
	return lv
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}

func clampInt(v, minV, maxV int) int {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func normalizeChildUsername(raw string) (string, error) {
	clean := strings.ToLower(strings.TrimSpace(raw))
	if clean == "" {
		return "", nil
	}
	rLen := len([]rune(clean))
	if rLen < 3 || rLen > 32 {
		return "", errors.New("孩子登录账号需为 3~32 个字符")
	}
	for _, ch := range clean {
		if unicode.IsSpace(ch) {
			return "", errors.New("孩子登录账号不能包含空格")
		}
	}
	return clean, nil
}

type Store struct {
	db          *SQLiteDB
	releasesDir string
}

func NewStore(dbPath, releasesDir string) (*Store, error) {
	if dbPath != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
			return nil, err
		}
	}
	if releasesDir != "" {
		if err := os.MkdirAll(releasesDir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := OpenSQLite(dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, releasesDir: releasesDir}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema() error {
	ddl := `
		PRAGMA journal_mode = WAL;
		PRAGMA foreign_keys = ON;

		CREATE TABLE IF NOT EXISTS admins (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS families (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			family_code TEXT NOT NULL UNIQUE,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS parents (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			username TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS children (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			avatar_idx INTEGER NOT NULL DEFAULT 0,
			username TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL DEFAULT '',
			pin TEXT NOT NULL DEFAULT '',
			pet_name TEXT NOT NULL DEFAULT '',
			xp INTEGER NOT NULL DEFAULT 0,
			coupons INTEGER NOT NULL DEFAULT 0,
			active_companion TEXT,
			muted INTEGER NOT NULL DEFAULT 0,
			pet_health TEXT NOT NULL DEFAULT 'normal',
			heal_progress INTEGER NOT NULL DEFAULT 0,
			last_health_date TEXT NOT NULL DEFAULT '',
			age INTEGER NOT NULL DEFAULT 7,
			quiz_difficulty TEXT NOT NULL DEFAULT 'normal',
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			emoji TEXT NOT NULL DEFAULT '✨',
			icon_url TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL,
			pts INTEGER NOT NULL DEFAULT 3,
			scope TEXT NOT NULL DEFAULT 'all',
			task_type TEXT NOT NULL DEFAULT 'normal',
			duration_sec INTEGER NOT NULL DEFAULT 60,
			target_count INTEGER NOT NULL DEFAULT 60,
			timer_mode TEXT NOT NULL DEFAULT 'none',
			require_photo INTEGER NOT NULL DEFAULT 0,
			repeat_mode TEXT NOT NULL DEFAULT 'daily',
			repeat_days TEXT NOT NULL DEFAULT '1,2,3,4,5,6,7',
			due_date TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS task_assignments (
			task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
			child_id TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
			PRIMARY KEY (task_id, child_id)
		);

		CREATE TABLE IF NOT EXISTS checkin_logs (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			child_id TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
			task_id TEXT NOT NULL,
			task_label TEXT NOT NULL,
			pts INTEGER NOT NULL,
			actual_count INTEGER,
			actual_duration INTEGER,
			checkin_date TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			UNIQUE(child_id, task_id, checkin_date)
		);

		CREATE TABLE IF NOT EXISTS custom_species (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			color TEXT NOT NULL,
			ear TEXT NOT NULL,
			acc TEXT NOT NULL,
			tail TEXT NOT NULL,
			mouth TEXT NOT NULL,
			base TEXT NOT NULL,
			hint TEXT NOT NULL DEFAULT '',
			target_child_id TEXT REFERENCES children(id) ON DELETE SET NULL,
			added_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS child_owned_species (
			child_id TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
			species_id TEXT NOT NULL,
			adopted_date TEXT NOT NULL,
			PRIMARY KEY (child_id, species_id)
		);

		CREATE TABLE IF NOT EXISTS polaroids (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			child_id TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
			avatar_idx INTEGER NOT NULL DEFAULT 0,
			pet_ref TEXT NOT NULL,
			pet_lv INTEGER NOT NULL DEFAULT 0,
			photo_date TEXT NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			UNIQUE(child_id, photo_date)
		);

		CREATE TABLE IF NOT EXISTS egg_grants (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			child_id TEXT NOT NULL REFERENCES children(id) ON DELETE CASCADE,
			parent_id TEXT NOT NULL REFERENCES parents(id) ON DELETE CASCADE,
			amount INTEGER NOT NULL DEFAULT 1,
			reason TEXT NOT NULL DEFAULT '25积分兑换确认',
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS app_releases (
			id TEXT PRIMARY KEY,
			platform TEXT NOT NULL DEFAULT 'android',
			version_code INTEGER NOT NULL,
			version_name TEXT NOT NULL,
			release_notes TEXT NOT NULL DEFAULT '',
			force_update INTEGER NOT NULL DEFAULT 0,
			file_name TEXT NOT NULL DEFAULT '',
			file_size INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			download_url TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS quiz_questions (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			category TEXT NOT NULL,
			question TEXT NOT NULL,
			options_json TEXT NOT NULL,
			answer_idx INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS family_task_icons (
			id TEXT PRIMARY KEY,
			family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE,
			category TEXT NOT NULL DEFAULT '自定义',
			label TEXT NOT NULL,
			image_data TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);
	`
	if err := s.db.ExecRaw(ddl); err != nil {
		return err
	}

	migrations := [][3]string{
		{"parents", "avatar_idx", "avatar_idx INTEGER NOT NULL DEFAULT 0"},
		{"parents", "avatar_data", "avatar_data TEXT NOT NULL DEFAULT ''"},
		{"parents", "avatar_updated_at", "avatar_updated_at INTEGER NOT NULL DEFAULT 0"},
		{"children", "username", "username TEXT NOT NULL DEFAULT ''"},
		{"children", "password_hash", "password_hash TEXT NOT NULL DEFAULT ''"},
		{"children", "avatar_data", "avatar_data TEXT NOT NULL DEFAULT ''"},
		{"children", "avatar_updated_at", "avatar_updated_at INTEGER NOT NULL DEFAULT 0"},
		{"children", "pet_health", "pet_health TEXT NOT NULL DEFAULT 'normal'"},
		{"children", "heal_progress", "heal_progress INTEGER NOT NULL DEFAULT 0"},
		{"children", "last_health_date", "last_health_date TEXT NOT NULL DEFAULT ''"},
		{"children", "age", "age INTEGER NOT NULL DEFAULT 7"},
		{"children", "quiz_difficulty", "quiz_difficulty TEXT NOT NULL DEFAULT 'normal'"},
		{"polaroids", "avatar_data", "avatar_data TEXT NOT NULL DEFAULT ''"},
		{"tasks", "icon_url", "icon_url TEXT NOT NULL DEFAULT ''"},
		{"tasks", "task_type", "task_type TEXT NOT NULL DEFAULT 'normal'"},
		{"tasks", "duration_sec", "duration_sec INTEGER NOT NULL DEFAULT 60"},
		{"tasks", "target_count", "target_count INTEGER NOT NULL DEFAULT 60"},
		{"tasks", "timer_mode", "timer_mode TEXT NOT NULL DEFAULT 'none'"},
		{"tasks", "require_photo", "require_photo INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "repeat_mode", "repeat_mode TEXT NOT NULL DEFAULT 'daily'"},
		{"tasks", "repeat_days", "repeat_days TEXT NOT NULL DEFAULT '1,2,3,4,5,6,7'"},
		{"tasks", "due_date", "due_date TEXT NOT NULL DEFAULT ''"},
		{"checkin_logs", "actual_count", "actual_count INTEGER"},
		{"checkin_logs", "actual_duration", "actual_duration INTEGER"},
		{"checkin_logs", "photo_data", "photo_data TEXT NOT NULL DEFAULT ''"},
	}
	for _, m := range migrations {
		if err := s.ensureColumn(m[0], m[1], m[2]); err != nil {
			return err
		}
	}
	if err := s.db.ExecRaw("CREATE UNIQUE INDEX IF NOT EXISTS idx_children_username ON children(username) WHERE username != ''"); err != nil {
		return err
	}

	return s.db.WithLock(func(tx *Tx) error {
		var count int
		_ = tx.QueryRow("SELECT COUNT(*) FROM admins", func(r SQLiteRow) error {
			count = r.Int(0)
			return nil
		})
		if count == 0 {
			initUser := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_USERNAME")))
			if initUser == "" {
				initUser = "admin"
			}
			initPass := os.Getenv("ADMIN_PASSWORD")
			if initPass == "" {
				initPass = "admin1234"
			}
			_, err := tx.Exec(
				"INSERT INTO admins (id, username, display_name, password_hash, created_at) VALUES (?, ?, ?, ?, ?)",
				GenID("adm_"), initUser, "系统管理员", HashPassword(initPass), nowMs(),
			)
			return err
		}
		return nil
	})
}

func (s *Store) ensureColumn(table, col, ddl string) error {
	var exists bool
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.Query(fmt.Sprintf("PRAGMA table_info(%s)", table), func(r SQLiteRow) error {
			if r.String(1) == col {
				exists = true
			}
			return nil
		})
	})
	if err != nil {
		return err
	}
	if !exists {
		return s.db.ExecRaw(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", table, ddl))
	}
	return nil
}

func (s *Store) uniqueFamilyCodeLocked(tx *Tx) string {
	for i := 0; i < 10; i++ {
		code := GenFamilyCode()
		var dummy int
		err := tx.QueryRow("SELECT 1 FROM families WHERE family_code = ?", func(r SQLiteRow) error {
			dummy = r.Int(0)
			return nil
		}, code)
		_ = dummy
		if errors.Is(err, ErrNotFound) {
			return code
		}
	}
	return fmt.Sprintf("%s%d", GenFamilyCode(), rand.Intn(9))
}

const BuiltinAvatarCount = 12

func clampAvatarIdx(idx int) int {
	if idx < 0 || idx >= BuiltinAvatarCount {
		return 0
	}
	return idx
}

func avatarURL(kind, id string, hasData bool, updatedAt int64) string {
	if !hasData || id == "" {
		return ""
	}
	return fmt.Sprintf("/api/avatar/%s/%s?v=%d", kind, id, updatedAt)
}

func sanitizeAvatarData(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", nil
	}
	if len(clean) > 1*1024*1024 {
		return "", errors.New("头像图片过大，请裁剪或压缩后再试")
	}
	if strings.HasPrefix(clean, "data:image/") {
		comma := strings.IndexByte(clean, ',')
		if comma <= 0 || comma+1 >= len(clean) {
			return "", errors.New("头像图片格式无效")
		}
		b64 := strings.TrimSpace(clean[comma+1:])
		if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return "", errors.New("头像图片 Base64 解码失败")
		}
		return clean[:comma+1] + b64, nil
	}
	if _, err := base64.StdEncoding.DecodeString(clean); err != nil {
		return "", errors.New("头像图片数据无效")
	}
	return "data:image/jpeg;base64," + clean, nil
}

func parseDataURLImage(raw string) (string, []byte, error) {
	b64 := strings.TrimSpace(raw)
	if b64 == "" {
		return "", nil, ErrNotFound
	}
	mimeType := "image/jpeg"
	if strings.HasPrefix(b64, "data:") {
		comma := strings.IndexByte(b64, ',')
		if comma > 5 {
			meta := b64[5:comma]
			if semi := strings.IndexByte(meta, ';'); semi > 0 {
				mimeType = meta[:semi]
			} else if meta != "" {
				mimeType = meta
			}
			b64 = b64[comma+1:]
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil || len(decoded) == 0 {
		return "", nil, ErrNotFound
	}
	return mimeType, decoded, nil
}

/* ==================== Data Models ==================== */

type FamilyDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	FamilyCode string `json:"family_code"`
	CreatedAt  int64  `json:"created_at,omitempty"`
}

type ParentDTO struct {
	ID          string `json:"id"`
	FamilyID    string `json:"family_id,omitempty"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarIdx   int    `json:"avatar_idx"`
	AvatarURL   string `json:"avatar_url"`
	CreatedAt   int64  `json:"created_at,omitempty"`
}

type AdminDTO struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type TaskDTO struct {
	ID               string   `json:"id"`
	FamilyID         string   `json:"family_id,omitempty"`
	Emoji            string   `json:"emoji"`
	IconURL          string   `json:"icon_url"`
	Label            string   `json:"label"`
	Pts              int      `json:"pts"`
	Scope            string   `json:"scope"`
	TaskType         string   `json:"task_type"`
	DurationSec      int      `json:"duration_sec"`
	TargetCount      int      `json:"target_count"`
	TimerMode        string   `json:"timer_mode"`
	RequirePhoto     bool     `json:"require_photo"`
	RepeatMode       string   `json:"repeat_mode"`
	RepeatDays       string   `json:"repeat_days"`
	DueDate          string   `json:"due_date"`
	Enabled          bool     `json:"enabled"`
	SortOrder        int      `json:"sort_order"`
	CreatedAt        int64    `json:"created_at,omitempty"`
	AssignedChildIDs []string `json:"assigned_child_ids,omitempty"`
}

type CheckinLogDTO struct {
	ID             string `json:"id,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	TaskLabel      string `json:"task_label"`
	Pts            int    `json:"pts"`
	ActualCount    *int   `json:"actual_count"`
	ActualDuration *int   `json:"actual_duration"`
	HasPhoto       bool   `json:"has_photo"`
	PhotoURL       string `json:"photo_url,omitempty"`
	CheckinDate    string `json:"checkin_date,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

type PolaroidDTO struct {
	ID        string `json:"id"`
	AvatarIdx int    `json:"avatar_idx"`
	AvatarURL string `json:"avatar_url"`
	PetRef    string `json:"pet_ref"`
	PetLv     int    `json:"pet_lv"`
	PhotoDate string `json:"photo_date"`
	Note      string `json:"note"`
}

type ChildSummaryDTO struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	AvatarIdx       int     `json:"avatar_idx"`
	AvatarURL       string  `json:"avatar_url"`
	PetName         string  `json:"pet_name"`
	XP              int     `json:"xp"`
	Coupons         int     `json:"coupons"`
	ActiveCompanion *string `json:"active_companion"`
	Muted           bool    `json:"muted"`
	HasPin          bool    `json:"has_pin"`
	PetHealth       string  `json:"pet_health"`
	HealProgress    int     `json:"heal_progress"`
	HealTarget      int     `json:"heal_target"`
	Age             int     `json:"age"`
	QuizDifficulty  string  `json:"quiz_difficulty"`
}

type ChildStateDTO struct {
	Family       FamilyDTO         `json:"family"`
	ChildCount   int               `json:"child_count"`
	Child        ChildSummaryDTO   `json:"child"`
	Tasks        []TaskDTO         `json:"tasks"`
	TodayDoneIDs []string          `json:"today_done_ids"`
	Logs         []CheckinLogDTO   `json:"logs"`
	Owned        map[string]string `json:"owned"`
	Catalog      []Species         `json:"catalog"`
	Polaroids    []PolaroidDTO     `json:"polaroids"`
	Today        string            `json:"today"`
}

type AppReleaseDTO struct {
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

/* ==================== Auth & Registration ==================== */

type RegisterFamilyInput struct {
	FamilyName    string `json:"familyName"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	Password      string `json:"password"`
	ChildName     string `json:"childName"`
	ChildUsername string `json:"childUsername"`
	ChildPassword string `json:"childPassword"`
}

type RegisterFamilyResult struct {
	Family       FamilyDTO `json:"family"`
	Parent       ParentDTO `json:"parent"`
	FirstChildID string    `json:"firstChildId"`
}

func (s *Store) RegisterFamily(in RegisterFamilyInput) (*RegisterFamilyResult, error) {
	cleanUser := strings.ToLower(strings.TrimSpace(in.Username))
	if len([]rune(cleanUser)) < 3 {
		return nil, errors.New("账号至少 3 个字符")
	}
	if len(in.Password) < 4 {
		return nil, errors.New("密码至少 4 位")
	}
	cleanChildUser, err := normalizeChildUsername(in.ChildUsername)
	if err != nil {
		return nil, err
	}
	if cleanChildUser != "" {
		if len(in.ChildPassword) < 4 {
			return nil, errors.New("孩子登录密码至少 4 位")
		}
	} else if in.ChildPassword != "" {
		return nil, errors.New("请先填写孩子登录账号")
	}

	var result *RegisterFamilyResult
	err = s.db.WithTx(func(tx *Tx) error {
		var existing string
		if e := tx.QueryRow("SELECT id FROM parents WHERE username = ?", func(r SQLiteRow) error {
			existing = r.String(0)
			return nil
		}, cleanUser); e == nil && existing != "" {
			return errors.New("该家长账号已被注册")
		}
		if cleanChildUser != "" {
			var dup string
			if e := tx.QueryRow("SELECT id FROM children WHERE username = ?", func(r SQLiteRow) error {
				dup = r.String(0)
				return nil
			}, cleanChildUser); e == nil && dup != "" {
				return errors.New("该孩子登录账号已被使用")
			}
		}

		now := nowMs()
		familyID := GenID("fam_")
		familyCode := s.uniqueFamilyCodeLocked(tx)
		parentID := GenID("par_")
		displayName := strings.TrimSpace(in.DisplayName)
		if displayName == "" {
			displayName = cleanUser
		}
		fName := strings.TrimSpace(in.FamilyName)
		if fName == "" {
			fName = displayName + "的家"
		}
		if _, err := tx.Exec(
			"INSERT INTO families (id, name, family_code, created_at) VALUES (?, ?, ?, ?)",
			familyID, fName, familyCode, now,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT INTO parents (id, family_id, username, display_name, password_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			parentID, familyID, cleanUser, displayName, HashPassword(in.Password), now,
		); err != nil {
			return err
		}

		firstChildName := strings.TrimSpace(in.ChildName)
		if firstChildName == "" {
			firstChildName = "大宝"
		}
		childID := GenID("kid_")
		childPassHash := ""
		if cleanChildUser != "" {
			childPassHash = HashPassword(in.ChildPassword)
		}
		if _, err := tx.Exec(
			"INSERT INTO children (id, family_id, name, avatar_idx, username, password_hash, pin, pet_name, xp, coupons, created_at) VALUES (?, ?, ?, 0, ?, ?, '', '', 0, 0, ?)",
			childID, familyID, firstChildName, cleanChildUser, childPassHash, now,
		); err != nil {
			return err
		}

		for idx, t := range DefaultTasks {
			if _, err := tx.Exec(
				"INSERT INTO tasks (id, family_id, emoji, label, pts, scope, enabled, sort_order, created_at) VALUES (?, ?, ?, ?, ?, 'all', 1, ?, ?)",
				GenID("tsk_"), familyID, t.Emoji, t.Label, t.Pts, idx, now,
			); err != nil {
				return err
			}
		}

		result = &RegisterFamilyResult{
			Family:       FamilyDTO{ID: familyID, Name: fName, FamilyCode: familyCode},
			Parent:       ParentDTO{ID: parentID, Username: cleanUser, DisplayName: displayName, FamilyID: familyID},
			FirstChildID: childID,
		}
		return nil
	})
	return result, err
}

func (s *Store) VerifyAdminLogin(username, password string) *AdminDTO {
	clean := strings.ToLower(strings.TrimSpace(username))
	var adm AdminDTO
	var hash string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow("SELECT id, username, display_name, password_hash FROM admins WHERE username = ?", func(r SQLiteRow) error {
			adm = AdminDTO{ID: r.String(0), Username: r.String(1), DisplayName: r.String(2)}
			hash = r.String(3)
			return nil
		}, clean)
	})
	if err != nil || !VerifyPassword(password, hash) {
		return nil
	}
	return &adm
}

func (s *Store) ChangeAdminPassword(adminID, oldPassword, newPassword string) error {
	if len(newPassword) < 4 {
		return errors.New("新密码至少 4 位")
	}
	return s.db.WithTx(func(tx *Tx) error {
		var hash string
		if err := tx.QueryRow("SELECT password_hash FROM admins WHERE id = ?", func(r SQLiteRow) error {
			hash = r.String(0)
			return nil
		}, adminID); err != nil {
			return errors.New("管理员不存在")
		}
		if !VerifyPassword(oldPassword, hash) {
			return errors.New("原密码不正确")
		}
		_, err := tx.Exec("UPDATE admins SET password_hash = ? WHERE id = ?", HashPassword(newPassword), adminID)
		return err
	})
}

func (s *Store) VerifyParentLogin(username, password string) (*ParentDTO, *FamilyDTO, error) {
	clean := strings.ToLower(strings.TrimSpace(username))
	var p ParentDTO
	var fam FamilyDTO
	var hash string
	err := s.db.WithLock(func(tx *Tx) error {
		if err := tx.QueryRow("SELECT id, family_id, username, display_name, password_hash, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at FROM parents WHERE username = ?", func(r SQLiteRow) error {
			pid := r.String(0)
			p = ParentDTO{
				ID:          pid,
				FamilyID:    r.String(1),
				Username:    r.String(2),
				DisplayName: r.String(3),
				AvatarIdx:   clampAvatarIdx(r.Int(5)),
				AvatarURL:   avatarURL("parent", pid, r.Int(6) != 0, r.Int64(7)),
			}
			hash = r.String(4)
			return nil
		}, clean); err != nil {
			return ErrNotFound
		}
		return tx.QueryRow("SELECT id, name, family_code FROM families WHERE id = ?", func(r SQLiteRow) error {
			fam = FamilyDTO{ID: r.String(0), Name: r.String(1), FamilyCode: r.String(2)}
			return nil
		}, p.FamilyID)
	})
	if err != nil || !VerifyPassword(password, hash) {
		return nil, nil, errors.New("账号或密码不正确")
	}
	return &p, &fam, nil
}

type RosterChildDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarIdx int    `json:"avatar_idx"`
	AvatarURL string `json:"avatar_url"`
	PetName   string `json:"pet_name"`
	XP        int    `json:"xp"`
	HasPin    bool   `json:"has_pin"`
}

func (s *Store) GetFamilyRoster(code string) (*FamilyDTO, []RosterChildDTO, error) {
	clean := strings.ToUpper(strings.TrimSpace(code))
	var fam FamilyDTO
	kids := make([]RosterChildDTO, 0)
	err := s.db.WithLock(func(tx *Tx) error {
		if err := tx.QueryRow("SELECT id, name, family_code FROM families WHERE family_code = ?", func(r SQLiteRow) error {
			fam = FamilyDTO{ID: r.String(0), Name: r.String(1), FamilyCode: r.String(2)}
			return nil
		}, clean); err != nil {
			return ErrNotFound
		}
		return tx.Query("SELECT id, name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, pin, pet_name, xp FROM children WHERE family_id = ? ORDER BY created_at ASC", func(r SQLiteRow) error {
			cid := r.String(0)
			kids = append(kids, RosterChildDTO{
				ID:        cid,
				Name:      r.String(1),
				AvatarIdx: clampAvatarIdx(r.Int(2)),
				AvatarURL: avatarURL("child", cid, r.Int(3) != 0, r.Int64(4)),
				HasPin:    r.String(5) != "",
				PetName:   r.String(6),
				XP:        r.Int(7),
			})
			return nil
		}, fam.ID)
	})
	if err != nil {
		return nil, nil, err
	}
	return &fam, kids, nil
}

func (s *Store) LoginChildByAccount(username, password string) (childID, familyID, familyCode string, err error) {
	clean := strings.ToLower(strings.TrimSpace(username))
	var hash string
	e := s.db.WithLock(func(tx *Tx) error {
		if err := tx.QueryRow("SELECT id, family_id, password_hash FROM children WHERE username = ?", func(r SQLiteRow) error {
			childID = r.String(0)
			familyID = r.String(1)
			hash = r.String(2)
			return nil
		}, clean); err != nil {
			return ErrNotFound
		}
		_ = tx.QueryRow("SELECT family_code FROM families WHERE id = ?", func(r SQLiteRow) error {
			familyCode = r.String(0)
			return nil
		}, familyID)
		return nil
	})
	if e != nil || hash == "" || !VerifyPassword(password, hash) {
		return "", "", "", errors.New("孩子账号或密码不正确")
	}
	return childID, familyID, familyCode, nil
}

func (s *Store) LoginChildByRoster(code, childID, pin string) (familyID string, status int, err error) {
	cleanCode := strings.ToUpper(strings.TrimSpace(code))
	cleanPin := strings.TrimSpace(pin)
	var storedPin string
	e := s.db.WithLock(func(tx *Tx) error {
		if err := tx.QueryRow("SELECT id FROM families WHERE family_code = ?", func(r SQLiteRow) error {
			familyID = r.String(0)
			return nil
		}, cleanCode); err != nil {
			status = 404
			return errors.New("家庭码无效")
		}
		if err := tx.QueryRow("SELECT pin FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			storedPin = r.String(0)
			return nil
		}, childID, familyID); err != nil {
			status = 404
			return errors.New("孩子档案不存在")
		}
		if storedPin != "" && storedPin != cleanPin {
			status = 401
			return errors.New("孩子密码（PIN）不对哦")
		}
		return nil
	})
	return familyID, status, e
}

/* ==================== Child & Task Operations ==================== */

func isTaskActiveOnDate(t TaskDTO, dateStr string) bool {
	if t.RepeatMode == "once" {
		return t.DueDate == "" || t.DueDate == dateStr
	}
	if strings.TrimSpace(t.RepeatDays) == "" {
		return true
	}
	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return true
	}
	wd := int(dt.Weekday())
	if wd == 0 {
		wd = 7
	}
	target := fmt.Sprintf("%d", wd)
	for _, d := range strings.Split(t.RepeatDays, ",") {
		if strings.TrimSpace(d) == target {
			return true
		}
	}
	return false
}

func (s *Store) getTasksForChildLocked(tx *Tx, familyID, childID, todayStr string) ([]TaskDTO, error) {
	tasks := make([]TaskDTO, 0)
	err := tx.Query(`
		SELECT t.id, t.emoji, t.icon_url, t.label, t.pts, t.scope, t.task_type, t.duration_sec, t.target_count, t.timer_mode, t.require_photo, t.repeat_mode, t.repeat_days, t.due_date, t.enabled, t.sort_order
		FROM tasks t
		WHERE t.family_id = ? AND t.enabled = 1
		  AND (
		    t.scope = 'all'
		    OR EXISTS (SELECT 1 FROM task_assignments ta WHERE ta.task_id = t.id AND ta.child_id = ?)
		  )
		ORDER BY t.sort_order ASC, t.created_at ASC
	`, func(r SQLiteRow) error {
		t := TaskDTO{
			ID:           r.String(0),
			Emoji:        r.String(1),
			IconURL:      r.String(2),
			Label:        r.String(3),
			Pts:          r.Int(4),
			Scope:        r.String(5),
			TaskType:     r.String(6),
			DurationSec:  r.Int(7),
			TargetCount:  r.Int(8),
			TimerMode:    r.String(9),
			RequirePhoto: r.Int(10) != 0,
			RepeatMode:   r.String(11),
			RepeatDays:   r.String(12),
			DueDate:      r.String(13),
			Enabled:      r.Int(14) != 0,
			SortOrder:    r.Int(15),
		}
		if todayStr == "" || isTaskActiveOnDate(t, todayStr) {
			tasks = append(tasks, t)
		}
		return nil
	}, familyID, childID)
	return tasks, err
}

func (s *Store) getCatalogForChildLocked(tx *Tx, familyID, childID string) ([]Species, error) {
	catalog := make([]Species, 0, len(BuiltinSpecies)+8)
	catalog = append(catalog, BuiltinSpecies...)
	err := tx.Query(`
		SELECT id, name, color, ear, acc, tail, mouth, base, hint, target_child_id, added_at
		FROM custom_species
		WHERE family_id = ? AND (target_child_id IS NULL OR target_child_id = ?)
		ORDER BY added_at ASC
	`, func(r SQLiteRow) error {
		catalog = append(catalog, Species{
			ID:            r.String(0),
			Name:          r.String(1),
			Color:         r.String(2),
			Ear:           r.String(3),
			Acc:           r.String(4),
			Tail:          r.String(5),
			Mouth:         r.String(6),
			Base:          r.String(7),
			Hint:          r.String(8),
			TargetChildID: r.NullableString(9),
			AddedAt:       r.Int64(10),
			Ahoge:         0,
		})
		return nil
	}, familyID, childID)
	return catalog, err
}

func (s *Store) getChildStateLocked(tx *Tx, familyID, childID, todayStr string) (*ChildStateDTO, error) {
	s.syncChildHealthLocked(tx, childID, todayStr)
	var child ChildSummaryDTO
	err := tx.QueryRow(`
		SELECT id, name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, pet_name, xp, coupons, active_companion, muted, pin, pet_health, heal_progress, age, quiz_difficulty
		FROM children WHERE id = ? AND family_id = ?
	`, func(r SQLiteRow) error {
		cid := r.String(0)
		h := r.String(11)
		if h == "" {
			h = "normal"
		}
		child = ChildSummaryDTO{
			ID:              cid,
			Name:            r.String(1),
			AvatarIdx:       clampAvatarIdx(r.Int(2)),
			AvatarURL:       avatarURL("child", cid, r.Int(3) != 0, r.Int64(4)),
			PetName:         r.String(5),
			XP:              r.Int(6),
			Coupons:         r.Int(7),
			ActiveCompanion: r.NullableString(8),
			Muted:           r.Int(9) != 0,
			HasPin:          r.String(10) != "",
			PetHealth:       h,
			HealProgress:    r.Int(12),
			HealTarget:      healTargetForHealth(h),
			Age:             NormalizeAge(r.Int(13)),
			QuizDifficulty:  NormalizeDifficulty(r.String(14)),
		}
		return nil
	}, childID, familyID)
	if err != nil {
		return nil, err
	}

	var fam FamilyDTO
	_ = tx.QueryRow("SELECT id, name, family_code FROM families WHERE id = ?", func(r SQLiteRow) error {
		fam = FamilyDTO{ID: r.String(0), Name: r.String(1), FamilyCode: r.String(2)}
		return nil
	}, familyID)

	tasks, err := s.getTasksForChildLocked(tx, familyID, childID, todayStr)
	if err != nil {
		return nil, err
	}

	logs := make([]CheckinLogDTO, 0)
	todayDone := make([]string, 0)
	_ = tx.Query(`
		SELECT id, task_id, task_label, pts, actual_count, actual_duration, LENGTH(photo_data) > 0, checkin_date, created_at
		FROM checkin_logs WHERE child_id = ? ORDER BY created_at ASC
	`, func(r SQLiteRow) error {
		id := r.String(0)
		tid := r.String(1)
		hasPhoto := r.Int(6) != 0
		photoURL := ""
		if hasPhoto {
			photoURL = "/api/checkin-photo/" + id
		}
		cDate := r.String(7)
		if cDate == todayStr && tid != "" {
			todayDone = append(todayDone, tid)
		}
		logs = append(logs, CheckinLogDTO{
			ID:             id,
			TaskID:         tid,
			TaskLabel:      r.String(2),
			Pts:            r.Int(3),
			ActualCount:    r.NullableInt(4),
			ActualDuration: r.NullableInt(5),
			HasPhoto:       hasPhoto,
			PhotoURL:       photoURL,
			CheckinDate:    cDate,
			CreatedAt:      r.Int64(8),
		})
		return nil
	}, childID)

	owned := make(map[string]string)
	_ = tx.Query("SELECT species_id, adopted_date FROM child_owned_species WHERE child_id = ?", func(r SQLiteRow) error {
		owned[r.String(0)] = r.String(1)
		return nil
	}, childID)

	catalog, err := s.getCatalogForChildLocked(tx, familyID, childID)
	if err != nil {
		return nil, err
	}

	polaroids := make([]PolaroidDTO, 0)
	_ = tx.Query(`
		SELECT id, avatar_idx, LENGTH(avatar_data) > 0, created_at, pet_ref, pet_lv, photo_date, note
		FROM polaroids WHERE child_id = ? ORDER BY photo_date DESC, created_at DESC
	`, func(r SQLiteRow) error {
		pid := r.String(0)
		polaroids = append(polaroids, PolaroidDTO{
			ID:        pid,
			AvatarIdx: clampAvatarIdx(r.Int(1)),
			AvatarURL: avatarURL("polaroid", pid, r.Int(2) != 0, r.Int64(3)),
			PetRef:    r.String(4),
			PetLv:     r.Int(5),
			PhotoDate: r.String(6),
			Note:      r.String(7),
		})
		return nil
	}, childID)

	childCount := 1
	_ = tx.QueryRow("SELECT COUNT(*) FROM children WHERE family_id = ?", func(r SQLiteRow) error {
		childCount = r.Int(0)
		return nil
	}, familyID)

	return &ChildStateDTO{
		Family:       fam,
		ChildCount:   childCount,
		Child:        child,
		Tasks:        tasks,
		TodayDoneIDs: todayDone,
		Logs:         logs,
		Owned:        owned,
		Catalog:      catalog,
		Polaroids:    polaroids,
		Today:        todayStr,
	}, nil
}

func (s *Store) GetChildState(familyID, childID, todayStr string) (*ChildStateDTO, error) {
	var st *ChildStateDTO
	err := s.db.WithLock(func(tx *Tx) error {
		var e error
		st, e = s.getChildStateLocked(tx, familyID, childID, todayStr)
		return e
	})
	return st, err
}

type CheckinResult struct {
	LogID          string         `json:"logId"`
	Task           TaskDTO        `json:"task"`
	Pts            int            `json:"pts"`
	PrevLv         int            `json:"prevLv"`
	NewLv          int            `json:"newLv"`
	NewXP          int            `json:"newXp"`
	ActualCount    *int           `json:"actualCount"`
	ActualDuration *int           `json:"actualDuration"`
	HasPhoto       bool           `json:"hasPhoto"`
	PhotoURL       string         `json:"photoUrl,omitempty"`
	State          *ChildStateDTO `json:"state"`
}

func sanitizeCheckinPhotoData(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", nil
	}
	if len(clean) > 3*1024*1024 {
		return "", errors.New("打卡照片过大，请压缩后再试")
	}
	if strings.HasPrefix(clean, "data:image/") {
		comma := strings.IndexByte(clean, ',')
		if comma <= 0 || comma+1 >= len(clean) {
			return "", errors.New("打卡照片格式无效")
		}
		b64 := strings.TrimSpace(clean[comma+1:])
		if _, err := base64.StdEncoding.DecodeString(b64); err != nil {
			return "", errors.New("打卡照片 Base64 解码失败")
		}
		return clean[:comma+1] + b64, nil
	}
	// 兼容纯 base64 JPEG 字符串
	if _, err := base64.StdEncoding.DecodeString(clean); err != nil {
		return "", errors.New("打卡照片数据无效")
	}
	return "data:image/jpeg;base64," + clean, nil
}

func (s *Store) GetCheckinPhoto(logID string) (string, []byte, error) {
	var photoData string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow("SELECT photo_data FROM checkin_logs WHERE id = ?", func(r SQLiteRow) error {
			photoData = r.String(0)
			return nil
		}, logID)
	})
	if err != nil || strings.TrimSpace(photoData) == "" {
		return "", nil, ErrNotFound
	}
	return parseDataURLImage(photoData)
}

func (s *Store) GetAvatarImage(kind, id string) (string, []byte, error) {
	var q string
	switch kind {
	case "parent":
		q = "SELECT avatar_data FROM parents WHERE id = ?"
	case "child":
		q = "SELECT avatar_data FROM children WHERE id = ?"
	case "polaroid":
		q = "SELECT avatar_data FROM polaroids WHERE id = ?"
	default:
		return "", nil, ErrNotFound
	}
	var avatarData string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow(q, func(r SQLiteRow) error {
			avatarData = r.String(0)
			return nil
		}, id)
	})
	if err != nil || strings.TrimSpace(avatarData) == "" {
		return "", nil, ErrNotFound
	}
	return parseDataURLImage(avatarData)
}

func (s *Store) CheckinTask(familyID, childID, taskID, todayStr string, actualCountIn, actualDurationIn *int, rawPhotoData string) (*CheckinResult, error) {
	cleanPhoto, err := sanitizeCheckinPhotoData(rawPhotoData)
	if err != nil {
		return nil, err
	}
	var res *CheckinResult
	err = s.db.WithTx(func(tx *Tx) error {
		var curXP int
		if err := tx.QueryRow("SELECT xp FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			curXP = r.Int(0)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("孩子档案不存在")
		}

		var task TaskDTO
		if err := tx.QueryRow(`
			SELECT t.id, t.family_id, t.emoji, t.label, t.pts, t.scope, t.task_type, t.duration_sec, t.target_count, t.timer_mode, t.require_photo, t.enabled, t.sort_order
			FROM tasks t
			WHERE t.id = ? AND t.family_id = ? AND t.enabled = 1
			  AND (
			    t.scope = 'all'
			    OR EXISTS (SELECT 1 FROM task_assignments ta WHERE ta.task_id = t.id AND ta.child_id = ?)
			  )
		`, func(r SQLiteRow) error {
			task = TaskDTO{
				ID:           r.String(0),
				FamilyID:     r.String(1),
				Emoji:        r.String(2),
				Label:        r.String(3),
				Pts:          r.Int(4),
				Scope:        r.String(5),
				TaskType:     r.String(6),
				DurationSec:  r.Int(7),
				TargetCount:  r.Int(8),
				TimerMode:    r.String(9),
				RequirePhoto: r.Int(10) != 0,
				Enabled:      r.Int(11) != 0,
				SortOrder:    r.Int(12),
			}
			return nil
		}, taskID, familyID, childID); err != nil {
			return errors.New("该任务未分配给你或已停用")
		}

		var dupID string
		if err := tx.QueryRow("SELECT id FROM checkin_logs WHERE child_id = ? AND task_id = ? AND checkin_date = ?", func(r SQLiteRow) error {
			dupID = r.String(0)
			return nil
		}, childID, taskID, todayStr); err == nil && dupID != "" {
			return errors.New("今天这项已经打过卡啦")
		}

		var actualCount *int
		var actualDuration *int
		if task.TaskType == "rope_jump" {
			cnt := 0
			if actualCountIn != nil && *actualCountIn > 0 {
				cnt = *actualCountIn
			}
			dur := task.DurationSec
			if actualDurationIn != nil && *actualDurationIn > 0 {
				dur = *actualDurationIn
			}
			if dur < 1 {
				dur = 1
			}
			if cnt < task.TargetCount {
				return fmt.Errorf("这次跳了 %d 个，还差 %d 个达标哦！", cnt, task.TargetCount-cnt)
			}
			actualCount = &cnt
			actualDuration = &dur
		} else if actualDurationIn != nil && *actualDurationIn > 0 {
			dur := *actualDurationIn
			actualDuration = &dur
		}
		if task.RequirePhoto && cleanPhoto == "" {
			return errors.New("这项任务要求先拍照留证哦！")
		}

		prevLv := LevelForXP(curXP)
		newXP := curXP + task.Pts
		newLv := LevelForXP(newXP)
		logID := GenID("log_")

		if _, err := tx.Exec(
			"INSERT INTO checkin_logs (id, family_id, child_id, task_id, task_label, pts, actual_count, actual_duration, photo_data, checkin_date, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			logID, familyID, childID, task.ID, task.Label, task.Pts, actualCount, actualDuration, cleanPhoto, todayStr, nowMs(),
		); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET xp = ? WHERE id = ?", newXP, childID); err != nil {
			return err
		}
		s.syncChildHealthLocked(tx, childID, todayStr)
		s.applyHealProgressLocked(tx, childID, task.Pts)

		st, err := s.getChildStateLocked(tx, familyID, childID, todayStr)
		if err != nil {
			return err
		}
		photoURL := ""
		if cleanPhoto != "" {
			photoURL = "/api/checkin-photo/" + logID
		}
		res = &CheckinResult{
			LogID:          logID,
			Task:           task,
			Pts:            task.Pts,
			PrevLv:         prevLv,
			NewLv:          newLv,
			NewXP:          newXP,
			ActualCount:    actualCount,
			ActualDuration: actualDuration,
			HasPhoto:       cleanPhoto != "",
			PhotoURL:       photoURL,
			State:          st,
		}
		return nil
	})
	return res, err
}

func (s *Store) UndoLastCheckin(familyID, childID, todayStr string) (*CheckinLogDTO, *ChildStateDTO, error) {
	var undone *CheckinLogDTO
	var st *ChildStateDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var last CheckinLogDTO
		if err := tx.QueryRow(
			"SELECT id, task_id, task_label, pts, actual_count, actual_duration, checkin_date, created_at FROM checkin_logs WHERE child_id = ? AND family_id = ? ORDER BY created_at DESC LIMIT 1",
			func(r SQLiteRow) error {
				last = CheckinLogDTO{
					ID:             r.String(0),
					TaskID:         r.String(1),
					TaskLabel:      r.String(2),
					Pts:            r.Int(3),
					ActualCount:    r.NullableInt(4),
					ActualDuration: r.NullableInt(5),
					CheckinDate:    r.String(6),
					CreatedAt:      r.Int64(7),
				}
				return nil
			}, childID, familyID,
		); err != nil {
			return errors.New("还没有可撤销的打卡记录")
		}
		if _, err := tx.Exec("DELETE FROM checkin_logs WHERE id = ?", last.ID); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET xp = MAX(0, xp - ?) WHERE id = ?", last.Pts, childID); err != nil {
			return err
		}
		undone = &last
		if todayStr != "" {
			st, _ = s.getChildStateLocked(tx, familyID, childID, todayStr)
		}
		return nil
	})
	return undone, st, err
}

func (s *Store) ParentCheckin(familyID, childID, taskID, todayStr string) error {
	return s.db.WithTx(func(tx *Tx) error {
		var curXP int
		if err := tx.QueryRow("SELECT xp FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			curXP = r.Int(0)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("孩子档案不存在")
		}
		var label string
		var pts int
		if err := tx.QueryRow(`
			SELECT t.label, t.pts
			FROM tasks t
			WHERE t.id = ? AND t.family_id = ? AND t.enabled = 1
			  AND (
			    t.scope = 'all'
			    OR EXISTS (SELECT 1 FROM task_assignments ta WHERE ta.task_id = t.id AND ta.child_id = ?)
			  )
		`, func(r SQLiteRow) error {
			label = r.String(0)
			pts = r.Int(1)
			return nil
		}, taskID, familyID, childID); err != nil {
			return errors.New("该任务未分配给该孩子或已停用")
		}
		var dupID string
		if err := tx.QueryRow("SELECT id FROM checkin_logs WHERE child_id = ? AND task_id = ? AND checkin_date = ?", func(r SQLiteRow) error {
			dupID = r.String(0)
			return nil
		}, childID, taskID, todayStr); err == nil && dupID != "" {
			return errors.New("今天这项已经打过卡啦")
		}
		logID := GenID("log_")
		if _, err := tx.Exec(
			"INSERT INTO checkin_logs (id, family_id, child_id, task_id, task_label, pts, actual_count, actual_duration, photo_data, checkin_date, created_at) VALUES (?, ?, ?, ?, ?, ?, NULL, NULL, '', ?, ?)",
			logID, familyID, childID, taskID, label, pts, todayStr, nowMs(),
		); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET xp = ? WHERE id = ?", curXP+pts, childID); err != nil {
			return err
		}
		s.syncChildHealthLocked(tx, childID, todayStr)
		s.applyHealProgressLocked(tx, childID, pts)
		return nil
	})
}

func (s *Store) UndoTaskCheckin(familyID, childID, taskID, todayStr string) (*CheckinLogDTO, error) {
	var undone *CheckinLogDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var last CheckinLogDTO
		if err := tx.QueryRow(
			"SELECT id, task_id, task_label, pts, checkin_date, created_at FROM checkin_logs WHERE child_id = ? AND family_id = ? AND task_id = ? AND checkin_date = ? ORDER BY created_at DESC LIMIT 1",
			func(r SQLiteRow) error {
				last = CheckinLogDTO{
					ID:          r.String(0),
					TaskID:      r.String(1),
					TaskLabel:   r.String(2),
					Pts:         r.Int(3),
					CheckinDate: r.String(4),
					CreatedAt:   r.Int64(5),
				}
				return nil
			}, childID, familyID, taskID, todayStr,
		); err != nil {
			return errors.New("今日尚未打卡该任务")
		}
		if _, err := tx.Exec("DELETE FROM checkin_logs WHERE id = ?", last.ID); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE children SET xp = MAX(0, xp - ?) WHERE id = ?", last.Pts, childID); err != nil {
			return err
		}
		undone = &last
		return nil
	})
	return undone, err
}

func periodDateRange(period, dateStr string) (string, string) {
	dt, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		dt = time.Now()
	}
	switch period {
	case "week":
		wd := int(dt.Weekday())
		if wd == 0 {
			wd = 7
		}
		start := dt.AddDate(0, 0, -(wd - 1))
		end := start.AddDate(0, 0, 6)
		return start.Format("2006-01-02"), end.Format("2006-01-02")
	case "month":
		start := time.Date(dt.Year(), dt.Month(), 1, 0, 0, 0, 0, dt.Location())
		end := start.AddDate(0, 1, -1)
		return start.Format("2006-01-02"), end.Format("2006-01-02")
	case "year":
		start := time.Date(dt.Year(), 1, 1, 0, 0, 0, 0, dt.Location())
		end := time.Date(dt.Year(), 12, 31, 0, 0, 0, 0, dt.Location())
		return start.Format("2006-01-02"), end.Format("2006-01-02")
	default:
		s := dt.Format("2006-01-02")
		return s, s
	}
}

func (s *Store) GetParentCheckinLogs(familyID, childID, period, dateStr string) (map[string]any, error) {
	startDate, endDate := periodDateRange(period, dateStr)
	type logRow struct {
		ID             string `json:"id"`
		ChildID        string `json:"child_id"`
		ChildName      string `json:"child_name"`
		TaskID         string `json:"task_id"`
		TaskLabel      string `json:"task_label"`
		Pts            int    `json:"pts"`
		ActualCount    *int   `json:"actual_count"`
		ActualDuration *int   `json:"actual_duration"`
		HasPhoto       bool   `json:"has_photo"`
		PhotoURL       string `json:"photo_url,omitempty"`
		CheckinDate    string `json:"checkin_date"`
		CreatedAt      int64  `json:"created_at"`
	}
	type summaryRow struct {
		TaskLabel     string `json:"task_label"`
		Count         int    `json:"count"`
		TotalPts      int    `json:"total_pts"`
		TotalDuration int    `json:"total_duration"`
		TotalCount    int    `json:"total_count"`
	}
	logs := make([]logRow, 0)
	summaryMap := make(map[string]*summaryRow)
	summaryOrder := make([]string, 0)

	err := s.db.WithLock(func(tx *Tx) error {
		q := `
			SELECT cl.id, cl.child_id, COALESCE(c.name, '孩子'), cl.task_id, cl.task_label, cl.pts,
			       cl.actual_count, cl.actual_duration, LENGTH(cl.photo_data) > 0, cl.checkin_date, cl.created_at
			FROM checkin_logs cl
			LEFT JOIN children c ON c.id = cl.child_id
			WHERE cl.family_id = ? AND cl.checkin_date >= ? AND cl.checkin_date <= ?`
		args := []any{familyID, startDate, endDate}
		if childID != "" {
			q += " AND cl.child_id = ?"
			args = append(args, childID)
		}
		q += " ORDER BY cl.checkin_date DESC, cl.created_at DESC LIMIT 500"

		return tx.Query(q, func(r SQLiteRow) error {
			id := r.String(0)
			hasPhoto := r.Int(8) != 0
			photoURL := ""
			if hasPhoto {
				photoURL = "/api/checkin-photo/" + id
			}
			row := logRow{
				ID:             id,
				ChildID:        r.String(1),
				ChildName:      r.String(2),
				TaskID:         r.String(3),
				TaskLabel:      r.String(4),
				Pts:            r.Int(5),
				ActualCount:    r.NullableInt(6),
				ActualDuration: r.NullableInt(7),
				HasPhoto:       hasPhoto,
				PhotoURL:       photoURL,
				CheckinDate:    r.String(9),
				CreatedAt:      r.Int64(10),
			}
			logs = append(logs, row)

			sm, ok := summaryMap[row.TaskLabel]
			if !ok {
				sm = &summaryRow{TaskLabel: row.TaskLabel}
				summaryMap[row.TaskLabel] = sm
				summaryOrder = append(summaryOrder, row.TaskLabel)
			}
			sm.Count++
			sm.TotalPts += row.Pts
			if row.ActualDuration != nil {
				sm.TotalDuration += *row.ActualDuration
			}
			if row.ActualCount != nil {
				sm.TotalCount += *row.ActualCount
			}
			return nil
		}, args...)
	})
	if err != nil {
		return nil, err
	}
	summary := make([]summaryRow, 0, len(summaryOrder))
	for _, k := range summaryOrder {
		summary = append(summary, *summaryMap[k])
	}
	return map[string]any{
		"start_date": startDate,
		"end_date":   endDate,
		"logs":       logs,
		"summary":    summary,
	}, nil
}

func (s *Store) UpdateChildProfile(familyID, childID string, petName *string, activeCompSet bool, activeComp *string, muted *bool) error {
	return s.db.WithTx(func(tx *Tx) error {
		var curPet string
		var curComp *string
		var curMuted int
		if err := tx.QueryRow("SELECT pet_name, active_companion, muted FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			curPet = r.String(0)
			curComp = r.NullableString(1)
			curMuted = r.Int(2)
			return nil
		}, childID, familyID); err != nil {
			return ErrNotFound
		}
		if petName != nil {
			curPet = truncateRunes(strings.TrimSpace(*petName), 12)
		}
		if activeCompSet {
			if activeComp != nil && strings.TrimSpace(*activeComp) != "" {
				v := strings.TrimSpace(*activeComp)
				curComp = &v
			} else {
				curComp = nil
			}
		}
		if muted != nil {
			if *muted {
				curMuted = 1
			} else {
				curMuted = 0
			}
		}
		_, err := tx.Exec("UPDATE children SET pet_name = ?, active_companion = ?, muted = ? WHERE id = ? AND family_id = ?",
			curPet, curComp, curMuted, childID, familyID)
		return err
	})
}

func (s *Store) AdoptSpecies(familyID, childID, speciesID string, mystery bool, todayStr string) (*Species, *ChildStateDTO, error) {
	var adopted *Species
	var st *ChildStateDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var xp, coupons int
		if err := tx.QueryRow("SELECT xp, coupons FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			xp = r.Int(0)
			coupons = r.Int(1)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("孩子档案不存在")
		}
		const matureXP = 200
		const eggCost = 25
		surplusXP := xp - matureXP
		useCoupon := false
		if coupons > 0 {
			useCoupon = true
		} else if surplusXP >= eggCost {
			useCoupon = false
		} else if xp < matureXP {
			return fmt.Errorf("主宠达到成熟期（%d 成长值）后，富余成长值满 %d 点即可直接兑换宠物蛋", matureXP, eggCost)
		} else {
			return fmt.Errorf("富余成长值不足 %d 点（当前富余 %d 点），继续打卡攒成长值吧", eggCost, surplusXP)
		}
		if mystery && xp < 120 {
			return errors.New("神秘蛋需要主宠达到「成年体（120 成长值）」才解锁哦")
		}
		catalog, err := s.getCatalogForChildLocked(tx, familyID, childID)
		if err != nil {
			return err
		}
		owned := make(map[string]bool)
		_ = tx.Query("SELECT species_id FROM child_owned_species WHERE child_id = ?", func(r SQLiteRow) error {
			owned[r.String(0)] = true
			return nil
		}, childID)

		var target *Species
		if mystery {
			unowned := make([]Species, 0)
			for _, sp := range catalog {
				if !owned[sp.ID] {
					unowned = append(unowned, sp)
				}
			}
			if len(unowned) == 0 {
				return errors.New("图鉴已经全部集齐啦！")
			}
			pick := unowned[rand.Intn(len(unowned))]
			target = &pick
		} else {
			for i := range catalog {
				if catalog[i].ID == speciesID {
					target = &catalog[i]
					break
				}
			}
			if target == nil {
				return errors.New("图鉴伙伴不存在")
			}
			if owned[target.ID] {
				return errors.New("已经领养过这位伙伴啦")
			}
		}

		if useCoupon {
			if _, err := tx.Exec("UPDATE children SET coupons = coupons - 1, active_companion = COALESCE(active_companion, ?) WHERE id = ?", target.ID, childID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("UPDATE children SET xp = MAX(?, xp - ?), active_companion = COALESCE(active_companion, ?) WHERE id = ?", matureXP, eggCost, target.ID, childID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("INSERT INTO child_owned_species (child_id, species_id, adopted_date) VALUES (?, ?, ?)", childID, target.ID, todayStr); err != nil {
			return err
		}
		adopted = target
		st, err = s.getChildStateLocked(tx, familyID, childID, todayStr)
		return err
	})
	return adopted, st, err
}

func (s *Store) UpdateParentAvatar(familyID, parentID string, avatarIdx *int, avatarData *string) (*ParentDTO, error) {
	var cleanData string
	var setData bool
	if avatarData != nil {
		var err error
		cleanData, err = sanitizeAvatarData(*avatarData)
		if err != nil {
			return nil, err
		}
		setData = true
	}
	var out ParentDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var curIdx int
		var curData, username, displayName string
		var updatedAt int64
		if err := tx.QueryRow(
			"SELECT username, display_name, avatar_idx, avatar_data, avatar_updated_at FROM parents WHERE id = ? AND family_id = ?",
			func(r SQLiteRow) error {
				username = r.String(0)
				displayName = r.String(1)
				curIdx = clampAvatarIdx(r.Int(2))
				curData = r.String(3)
				updatedAt = r.Int64(4)
				return nil
			}, parentID, familyID,
		); err != nil {
			return ErrNotFound
		}
		if avatarIdx != nil {
			curIdx = clampAvatarIdx(*avatarIdx)
			if !setData || cleanData == "" {
				curData = ""
				updatedAt = nowMs()
			}
		}
		if setData {
			curData = cleanData
			updatedAt = nowMs()
		}
		if _, err := tx.Exec(
			"UPDATE parents SET avatar_idx = ?, avatar_data = ?, avatar_updated_at = ? WHERE id = ? AND family_id = ?",
			curIdx, curData, updatedAt, parentID, familyID,
		); err != nil {
			return err
		}
		out = ParentDTO{
			ID:          parentID,
			FamilyID:    familyID,
			Username:    username,
			DisplayName: displayName,
			AvatarIdx:   curIdx,
			AvatarURL:   avatarURL("parent", parentID, curData != "", updatedAt),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) UpdateChildAvatar(familyID, childID string, avatarIdx *int, avatarData *string, todayStr string) (*ChildStateDTO, error) {
	var cleanData string
	var setData bool
	if avatarData != nil {
		var err error
		cleanData, err = sanitizeAvatarData(*avatarData)
		if err != nil {
			return nil, err
		}
		setData = true
	}
	var st *ChildStateDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var curIdx int
		var curData string
		var updatedAt int64
		if err := tx.QueryRow(
			"SELECT avatar_idx, avatar_data, avatar_updated_at FROM children WHERE id = ? AND family_id = ?",
			func(r SQLiteRow) error {
				curIdx = clampAvatarIdx(r.Int(0))
				curData = r.String(1)
				updatedAt = r.Int64(2)
				return nil
			}, childID, familyID,
		); err != nil {
			return ErrNotFound
		}
		if avatarIdx != nil {
			curIdx = clampAvatarIdx(*avatarIdx)
			if !setData || cleanData == "" {
				curData = ""
				updatedAt = nowMs()
			}
		}
		if setData {
			curData = cleanData
			updatedAt = nowMs()
		}
		if _, err := tx.Exec(
			"UPDATE children SET avatar_idx = ?, avatar_data = ?, avatar_updated_at = ? WHERE id = ? AND family_id = ?",
			curIdx, curData, updatedAt, childID, familyID,
		); err != nil {
			return err
		}
		var e error
		st, e = s.getChildStateLocked(tx, familyID, childID, todayStr)
		return e
	})
	return st, err
}

func (s *Store) TakePolaroid(familyID, childID string, avatarIdx int, noteText, todayStr string) (map[string]any, *ChildStateDTO, error) {
	var photo map[string]any
	var st *ChildStateDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var xp, kidAvatarIdx int
		var kidAvatarData string
		var activeComp *string
		if err := tx.QueryRow("SELECT xp, active_companion, avatar_idx, avatar_data FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			xp = r.Int(0)
			activeComp = r.NullableString(1)
			kidAvatarIdx = clampAvatarIdx(r.Int(2))
			kidAvatarData = r.String(3)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("孩子档案不存在")
		}
		var dup string
		if err := tx.QueryRow("SELECT id FROM polaroids WHERE child_id = ? AND photo_date = ?", func(r SQLiteRow) error {
			dup = r.String(0)
			return nil
		}, childID, todayStr); err == nil && dup != "" {
			return errors.New("今天的合影已经拍过啦，明天再来拍一张吧！")
		}
		petRef := "main"
		if activeComp != nil && *activeComp != "" {
			petRef = *activeComp
		}
		id := GenID("pho_")
		note := truncateRunes(strings.TrimSpace(noteText), 24)
		if _, err := tx.Exec(
			"INSERT INTO polaroids (id, family_id, child_id, avatar_idx, avatar_data, pet_ref, pet_lv, photo_date, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, familyID, childID, kidAvatarIdx, kidAvatarData, petRef, LevelForXP(xp), todayStr, note, nowMs(),
		); err != nil {
			return err
		}
		photo = map[string]any{"id": id, "photo_date": todayStr}
		var e error
		st, e = s.getChildStateLocked(tx, familyID, childID, todayStr)
		return e
	})
	return photo, st, err
}

/* ==================== Parent Operations ==================== */

func (s *Store) getFamilyTasksWithAssignmentsLocked(tx *Tx, familyID string) ([]TaskDTO, error) {
	tasks := make([]TaskDTO, 0)
	if err := tx.Query(
		"SELECT id, family_id, emoji, icon_url, label, pts, scope, task_type, duration_sec, target_count, timer_mode, require_photo, repeat_mode, repeat_days, due_date, enabled, sort_order, created_at FROM tasks WHERE family_id = ? ORDER BY sort_order ASC, created_at ASC",
		func(r SQLiteRow) error {
			tasks = append(tasks, TaskDTO{
				ID:               r.String(0),
				FamilyID:         r.String(1),
				Emoji:            r.String(2),
				IconURL:          r.String(3),
				Label:            r.String(4),
				Pts:              r.Int(5),
				Scope:            r.String(6),
				TaskType:         r.String(7),
				DurationSec:      r.Int(8),
				TargetCount:      r.Int(9),
				TimerMode:        r.String(10),
				RequirePhoto:     r.Int(11) != 0,
				RepeatMode:       r.String(12),
				RepeatDays:       r.String(13),
				DueDate:          r.String(14),
				Enabled:          r.Int(15) != 0,
				SortOrder:        r.Int(16),
				CreatedAt:        r.Int64(17),
				AssignedChildIDs: make([]string, 0),
			})
			return nil
		}, familyID,
	); err != nil {
		return nil, err
	}
	assignMap := make(map[string][]string)
	_ = tx.Query(`
		SELECT ta.task_id, ta.child_id
		FROM task_assignments ta
		JOIN tasks t ON t.id = ta.task_id
		WHERE t.family_id = ?
	`, func(r SQLiteRow) error {
		tid := r.String(0)
		assignMap[tid] = append(assignMap[tid], r.String(1))
		return nil
	}, familyID)

	for i := range tasks {
		if ids, ok := assignMap[tasks[i].ID]; ok {
			tasks[i].AssignedChildIDs = ids
		}
	}
	return tasks, nil
}

func (s *Store) GetParentOverview(familyID, parentID, todayStr string) (map[string]any, error) {
	var out map[string]any
	err := s.db.WithLock(func(tx *Tx) error {
		var fam FamilyDTO
		if err := tx.QueryRow("SELECT id, name, family_code, created_at FROM families WHERE id = ?", func(r SQLiteRow) error {
			fam = FamilyDTO{ID: r.String(0), Name: r.String(1), FamilyCode: r.String(2), CreatedAt: r.Int64(3)}
			return nil
		}, familyID); err != nil {
			return err
		}
		var parent ParentDTO
		_ = tx.QueryRow("SELECT id, username, display_name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at FROM parents WHERE id = ?", func(r SQLiteRow) error {
			pid := r.String(0)
			parent = ParentDTO{
				ID:          pid,
				Username:    r.String(1),
				DisplayName: r.String(2),
				AvatarIdx:   clampAvatarIdx(r.Int(3)),
				AvatarURL:   avatarURL("parent", pid, r.Int(4) != 0, r.Int64(5)),
			}
			return nil
		}, parentID)

		parents := make([]ParentDTO, 0)
		_ = tx.Query("SELECT id, username, display_name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, created_at FROM parents WHERE family_id = ? ORDER BY created_at ASC", func(r SQLiteRow) error {
			pid := r.String(0)
			parents = append(parents, ParentDTO{
				ID:          pid,
				Username:    r.String(1),
				DisplayName: r.String(2),
				AvatarIdx:   clampAvatarIdx(r.Int(3)),
				AvatarURL:   avatarURL("parent", pid, r.Int(4) != 0, r.Int64(5)),
				CreatedAt:   r.Int64(6),
			})
			return nil
		}, familyID)

		type rawKid struct {
			id, name, username, passHash, pin, petName, petHealth, quizDiff string
			avatarIdx, xp, coupons, healProgress, age                       int
			hasAvatarData                                                   bool
			avatarUpdatedAt                                                 int64
			activeComp                                                      *string
		}
		kidIDs := make([]string, 0)
		_ = tx.Query("SELECT id FROM children WHERE family_id = ? ORDER BY created_at ASC", func(r SQLiteRow) error {
			kidIDs = append(kidIDs, r.String(0))
			return nil
		}, familyID)
		for _, kidID := range kidIDs {
			s.syncChildHealthLocked(tx, kidID, todayStr)
		}
		rawKids := make([]rawKid, 0)
		_ = tx.Query("SELECT id, name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, username, password_hash, pin, pet_name, xp, coupons, active_companion, pet_health, heal_progress, age, quiz_difficulty FROM children WHERE family_id = ? ORDER BY created_at ASC", func(r SQLiteRow) error {
			h := r.String(12)
			if h == "" {
				h = "normal"
			}
			rawKids = append(rawKids, rawKid{
				id: r.String(0), name: r.String(1), avatarIdx: clampAvatarIdx(r.Int(2)),
				hasAvatarData: r.Int(3) != 0, avatarUpdatedAt: r.Int64(4),
				username: r.String(5), passHash: r.String(6), pin: r.String(7),
				petName: r.String(8), xp: r.Int(9), coupons: r.Int(10), activeComp: r.NullableString(11),
				petHealth: h, healProgress: r.Int(13),
				age: NormalizeAge(r.Int(14)), quizDiff: NormalizeDifficulty(r.String(15)),
			})
			return nil
		}, familyID)

		childrenEnriched := make([]map[string]any, 0, len(rawKids))
		for _, c := range rawKids {
			myTasks, _ := s.getTasksForChildLocked(tx, familyID, c.id, todayStr)
			todayDone := make([]string, 0)
			_ = tx.Query("SELECT task_id FROM checkin_logs WHERE child_id = ? AND checkin_date = ?", func(r SQLiteRow) error {
				todayDone = append(todayDone, r.String(0))
				return nil
			}, c.id, todayStr)

			recentLogs := make([]CheckinLogDTO, 0)
			_ = tx.Query("SELECT id, task_id, task_label, pts, actual_count, actual_duration, LENGTH(photo_data) > 0, checkin_date, created_at FROM checkin_logs WHERE child_id = ? ORDER BY created_at DESC LIMIT 12", func(r SQLiteRow) error {
				id := r.String(0)
				hasPhoto := r.Int(6) != 0
				photoURL := ""
				if hasPhoto {
					photoURL = "/api/checkin-photo/" + id
				}
				recentLogs = append(recentLogs, CheckinLogDTO{
					ID: id, TaskID: r.String(1), TaskLabel: r.String(2), Pts: r.Int(3),
					ActualCount: r.NullableInt(4), ActualDuration: r.NullableInt(5),
					HasPhoto: hasPhoto, PhotoURL: photoURL,
					CheckinDate: r.String(7), CreatedAt: r.Int64(8),
				})
				return nil
			}, c.id)

			var totalDays, ownedCount int
			_ = tx.QueryRow("SELECT COUNT(DISTINCT checkin_date) FROM checkin_logs WHERE child_id = ?", func(r SQLiteRow) error {
				totalDays = r.Int(0)
				return nil
			}, c.id)
			_ = tx.QueryRow("SELECT COUNT(*) FROM child_owned_species WHERE child_id = ?", func(r SQLiteRow) error {
				ownedCount = r.Int(0)
				return nil
			}, c.id)

			childrenEnriched = append(childrenEnriched, map[string]any{
				"id":               c.id,
				"name":             c.name,
				"avatar_idx":       c.avatarIdx,
				"avatar_url":       avatarURL("child", c.id, c.hasAvatarData, c.avatarUpdatedAt),
				"username":         c.username,
				"has_password":     c.passHash != "",
				"has_pin":          c.pin != "",
				"pet_name":         c.petName,
				"xp":               c.xp,
				"level":            LevelForXP(c.xp),
				"coupons":          c.coupons,
				"pet_health":       c.petHealth,
				"heal_progress":    c.healProgress,
				"heal_target":      healTargetForHealth(c.petHealth),
				"age":              c.age,
				"quiz_difficulty":  c.quizDiff,
				"active_companion": c.activeComp,
				"tasks":            myTasks,
				"today_done_ids":   todayDone,
				"recent_logs":      recentLogs,
				"total_days":       totalDays,
				"owned_count":      ownedCount,
			})
		}

		tasks, err := s.getFamilyTasksWithAssignmentsLocked(tx, familyID)
		if err != nil {
			return err
		}

		catalog := make([]Species, 0, len(BuiltinSpecies)+8)
		catalog = append(catalog, BuiltinSpecies...)
		_ = tx.Query("SELECT id, family_id, name, color, ear, acc, tail, mouth, base, hint, target_child_id, added_at FROM custom_species WHERE family_id = ? ORDER BY added_at ASC", func(r SQLiteRow) error {
			catalog = append(catalog, Species{
				ID: r.String(0), FamilyID: r.String(1), Name: r.String(2), Color: r.String(3),
				Ear: r.String(4), Acc: r.String(5), Tail: r.String(6), Mouth: r.String(7),
				Base: r.String(8), Hint: r.String(9), TargetChildID: r.NullableString(10), AddedAt: r.Int64(11),
			})
			return nil
		}, familyID)

		grants := make([]map[string]any, 0)
		_ = tx.Query(`
			SELECT eg.id, eg.family_id, eg.child_id, eg.parent_id, eg.amount, eg.reason, eg.created_at,
			       c.name AS child_name, p.display_name AS parent_name
			FROM egg_grants eg
			JOIN children c ON c.id = eg.child_id
			JOIN parents p ON p.id = eg.parent_id
			WHERE eg.family_id = ?
			ORDER BY eg.created_at DESC
			LIMIT 20
		`, func(r SQLiteRow) error {
			grants = append(grants, map[string]any{
				"id":          r.String(0),
				"family_id":   r.String(1),
				"child_id":    r.String(2),
				"parent_id":   r.String(3),
				"amount":      r.Int(4),
				"reason":      r.String(5),
				"created_at":  r.Int64(6),
				"child_name":  r.String(7),
				"parent_name": r.String(8),
			})
			return nil
		}, familyID)

		out = map[string]any{
			"today":      todayStr,
			"family":     fam,
			"parent":     parent,
			"parents":    parents,
			"children":   childrenEnriched,
			"tasks":      tasks,
			"catalog":    catalog,
			"egg_grants": grants,
		}
		return nil
	})
	return out, err
}

func (s *Store) AddParent(familyID, username, displayName, password string) (*ParentDTO, error) {
	cleanUser := strings.ToLower(strings.TrimSpace(username))
	if len([]rune(cleanUser)) < 3 {
		return nil, errors.New("账号至少 3 个字符")
	}
	if len(password) < 4 {
		return nil, errors.New("密码至少 4 位")
	}
	disp := strings.TrimSpace(displayName)
	if disp == "" {
		disp = cleanUser
	}
	var out *ParentDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var dup string
		if e := tx.QueryRow("SELECT id FROM parents WHERE username = ?", func(r SQLiteRow) error {
			dup = r.String(0)
			return nil
		}, cleanUser); e == nil && dup != "" {
			return errors.New("该家长账号已被注册")
		}
		id := GenID("par_")
		if _, err := tx.Exec(
			"INSERT INTO parents (id, family_id, username, display_name, password_hash, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			id, familyID, cleanUser, disp, HashPassword(password), nowMs(),
		); err != nil {
			return err
		}
		out = &ParentDTO{ID: id, Username: cleanUser, DisplayName: disp}
		return nil
	})
	return out, err
}

func (s *Store) GetChildQuizProfile(familyID, childID string) (int, string) {
	age := 7
	diff := "normal"
	_ = s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow(
			"SELECT age, quiz_difficulty FROM children WHERE id = ? AND family_id = ?",
			func(r SQLiteRow) error {
				age = NormalizeAge(r.Int(0))
				diff = NormalizeDifficulty(r.String(1))
				return nil
			}, childID, familyID,
		)
	})
	return age, diff
}

func (s *Store) CreateChild(familyID, name string, avatarIdx int, pin, username, password string, age int, quizDiff string) (map[string]any, error) {
	cleanName := truncateRunes(strings.TrimSpace(name), 12)
	if cleanName == "" {
		return nil, errors.New("请输入孩子昵称")
	}
	normAge := NormalizeAge(age)
	normDiff := NormalizeDifficulty(quizDiff)
	cleanPin := truncateRunes(strings.TrimSpace(pin), 6)
	cleanUser, err := normalizeChildUsername(username)
	if err != nil {
		return nil, err
	}
	if cleanUser != "" {
		if len(password) < 4 {
			return nil, errors.New("孩子登录密码至少 4 位")
		}
	} else if password != "" {
		return nil, errors.New("请先填写孩子登录账号")
	}
	var res map[string]any
	err = s.db.WithTx(func(tx *Tx) error {
		if cleanUser != "" {
			var dup string
			if e := tx.QueryRow("SELECT id FROM children WHERE username = ?", func(r SQLiteRow) error {
				dup = r.String(0)
				return nil
			}, cleanUser); e == nil && dup != "" {
				return errors.New("该孩子登录账号已被占用，请换一个")
			}
		}
		id := GenID("kid_")
		passHash := ""
		if cleanUser != "" {
			passHash = HashPassword(password)
		}
		if _, err := tx.Exec(
			"INSERT INTO children (id, family_id, name, avatar_idx, username, password_hash, pin, pet_name, xp, coupons, age, quiz_difficulty, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, '', 0, 0, ?, ?, ?)",
			id, familyID, cleanName, clampAvatarIdx(avatarIdx), cleanUser, passHash, cleanPin, normAge, normDiff, nowMs(),
		); err != nil {
			return err
		}
		res = map[string]any{"id": id, "name": cleanName, "username": cleanUser, "age": normAge, "quiz_difficulty": normDiff}
		return nil
	})
	return res, err
}

type UpdateChildInput struct {
	Name           *string `json:"name"`
	AvatarIdx      *int    `json:"avatar_idx"`
	AvatarData     *string `json:"avatar_data"`
	Pin            *string `json:"pin"`
	PetName        *string `json:"pet_name"`
	Username       *string `json:"username"`
	Password       *string `json:"password"`
	Age            *int    `json:"age"`
	QuizDifficulty *string `json:"quiz_difficulty"`
	XP             *int    `json:"xp"`
	Coupons        *int    `json:"coupons"`
}

func (s *Store) updateChildLocked(tx *Tx, familyID, childID string, in UpdateChildInput) (map[string]any, error) {
	var curName, curUser, curPassHash, curPin, curPet, curAvatarData, curDiff string
	var curAvatar, curAge int
	var curAvatarUpdated int64
	if err := tx.QueryRow("SELECT name, avatar_idx, avatar_data, avatar_updated_at, username, password_hash, pin, pet_name, age, quiz_difficulty FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
		curName = r.String(0)
		curAvatar = clampAvatarIdx(r.Int(1))
		curAvatarData = r.String(2)
		curAvatarUpdated = r.Int64(3)
		curUser = r.String(4)
		curPassHash = r.String(5)
		curPin = r.String(6)
		curPet = r.String(7)
		curAge = NormalizeAge(r.Int(8))
		curDiff = NormalizeDifficulty(r.String(9))
		return nil
	}, childID, familyID); err != nil {
		return nil, ErrNotFound
	}

	if in.Name != nil {
		if n := truncateRunes(strings.TrimSpace(*in.Name), 12); n != "" {
			curName = n
		}
	}
	if in.AvatarIdx != nil {
		curAvatar = clampAvatarIdx(*in.AvatarIdx)
		if in.AvatarData == nil || strings.TrimSpace(*in.AvatarData) == "" {
			curAvatarData = ""
			curAvatarUpdated = nowMs()
		}
	}
	if in.AvatarData != nil {
		cleanData, err := sanitizeAvatarData(*in.AvatarData)
		if err != nil {
			return nil, err
		}
		curAvatarData = cleanData
		curAvatarUpdated = nowMs()
	}
	if in.Pin != nil {
		curPin = truncateRunes(strings.TrimSpace(*in.Pin), 6)
	}
	if in.PetName != nil {
		curPet = truncateRunes(strings.TrimSpace(*in.PetName), 12)
	}
	if in.Username != nil {
		u, err := normalizeChildUsername(*in.Username)
		if err != nil {
			return nil, err
		}
		curUser = u
		if curUser != "" {
			var dup string
			if e := tx.QueryRow("SELECT id FROM children WHERE username = ? AND id != ?", func(r SQLiteRow) error {
				dup = r.String(0)
				return nil
			}, curUser, childID); e == nil && dup != "" {
				return nil, errors.New("该孩子登录账号已被占用，请换一个")
			}
		} else {
			curPassHash = ""
		}
	}
	if in.Password != nil && *in.Password != "" {
		if len(*in.Password) < 4 {
			return nil, errors.New("孩子登录密码至少 4 位")
		}
		if curUser == "" {
			return nil, errors.New("请先设置孩子登录账号")
		}
		curPassHash = HashPassword(*in.Password)
	} else if curUser != "" && curPassHash == "" {
		return nil, errors.New("首次设置独立登录账号时请同时设置至少 4 位密码")
	}

	if in.Age != nil {
		curAge = NormalizeAge(*in.Age)
	}
	if in.QuizDifficulty != nil {
		curDiff = NormalizeDifficulty(*in.QuizDifficulty)
	}

	if _, err := tx.Exec(
		"UPDATE children SET name = ?, avatar_idx = ?, avatar_data = ?, avatar_updated_at = ?, username = ?, password_hash = ?, pin = ?, pet_name = ?, age = ?, quiz_difficulty = ? WHERE id = ? AND family_id = ?",
		curName, curAvatar, curAvatarData, curAvatarUpdated, curUser, curPassHash, curPin, curPet, curAge, curDiff, childID, familyID,
	); err != nil {
		return nil, err
	}
	return map[string]any{
		"id":              childID,
		"name":            curName,
		"avatar_idx":      curAvatar,
		"avatar_url":      avatarURL("child", childID, curAvatarData != "", curAvatarUpdated),
		"username":        curUser,
		"has_password":    curPassHash != "",
		"age":             curAge,
		"quiz_difficulty": curDiff,
	}, nil
}

func (s *Store) UpdateChild(familyID, childID string, in UpdateChildInput) (map[string]any, error) {
	var out map[string]any
	err := s.db.WithTx(func(tx *Tx) error {
		var e error
		out, e = s.updateChildLocked(tx, familyID, childID, in)
		return e
	})
	return out, err
}

func (s *Store) DeleteChild(familyID, childID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		var exists string
		if err := tx.QueryRow("SELECT id FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			exists = r.String(0)
			return nil
		}, childID, familyID); err != nil || exists == "" {
			return ErrNotFound
		}
		var cnt int
		_ = tx.QueryRow("SELECT COUNT(*) FROM children WHERE family_id = ?", func(r SQLiteRow) error {
			cnt = r.Int(0)
			return nil
		}, familyID)
		if cnt <= 1 {
			return errors.New("家庭至少保留 1 位孩子档案")
		}
		_, err := tx.Exec("DELETE FROM children WHERE id = ? AND family_id = ?", childID, familyID)
		return err
	})
}

type TaskInput struct {
	Emoji        *string  `json:"emoji"`
	IconURL      *string  `json:"icon_url"`
	Label        *string  `json:"label"`
	Pts          *int     `json:"pts"`
	Scope        *string  `json:"scope"`
	TaskType     *string  `json:"task_type"`
	DurationSec  *int     `json:"duration_sec"`
	TargetCount  *int     `json:"target_count"`
	TimerMode    *string  `json:"timer_mode"`
	RequirePhoto *bool    `json:"require_photo"`
	RepeatMode   *string  `json:"repeat_mode"`
	RepeatDays   *string  `json:"repeat_days"`
	DueDate      *string  `json:"due_date"`
	Enabled      *bool    `json:"enabled"`
	ChildIDs     []string `json:"childIds"`
	HasChildIDs  bool     `json:"-"`
}

func (s *Store) CreateTask(familyID string, in TaskInput) (string, error) {
	label := ""
	if in.Label != nil {
		label = strings.TrimSpace(*in.Label)
	}
	if label == "" {
		return "", errors.New("任务名称不能为空")
	}
	pts := 3
	if in.Pts != nil && *in.Pts > 0 {
		pts = clampInt(*in.Pts, 1, 50)
	}
	scope := "all"
	if in.Scope != nil && *in.Scope == "selected" {
		scope = "selected"
	}
	taskType := "normal"
	if in.TaskType != nil && *in.TaskType == "rope_jump" {
		taskType = "rope_jump"
	}
	duration := 60
	if in.DurationSec != nil && *in.DurationSec > 0 {
		duration = clampInt(*in.DurationSec, 10, 7200)
	}
	target := 60
	if in.TargetCount != nil && *in.TargetCount > 0 {
		target = clampInt(*in.TargetCount, 1, 500)
	}
	timerMode := "none"
	if in.TimerMode != nil && (*in.TimerMode == "countdown" || *in.TimerMode == "countup") {
		timerMode = *in.TimerMode
	}
	requirePhoto := 0
	if in.RequirePhoto != nil && *in.RequirePhoto {
		requirePhoto = 1
	}
	repeatMode := "daily"
	if in.RepeatMode != nil && (*in.RepeatMode == "once" || *in.RepeatMode == "weekly" || *in.RepeatMode == "daily") {
		repeatMode = *in.RepeatMode
	}
	repeatDays := "1,2,3,4,5,6,7"
	if in.RepeatDays != nil && strings.TrimSpace(*in.RepeatDays) != "" {
		repeatDays = strings.TrimSpace(*in.RepeatDays)
	}
	dueDate := ""
	if repeatMode == "once" {
		if in.DueDate != nil && strings.TrimSpace(*in.DueDate) != "" {
			dueDate = strings.TrimSpace(*in.DueDate)
		} else {
			dueDate = time.Now().Format("2006-01-02")
		}
	}
	emoji := "✨"
	if taskType == "rope_jump" {
		emoji = "🪢"
	}
	if in.Emoji != nil && strings.TrimSpace(*in.Emoji) != "" {
		emoji = strings.TrimSpace(*in.Emoji)
	}
	iconURL := ""
	if in.IconURL != nil {
		iconURL = strings.TrimSpace(*in.IconURL)
	}
	assigned := make([]string, 0)
	for _, cid := range in.ChildIDs {
		if cid != "" {
			assigned = append(assigned, cid)
		}
	}
	if scope == "selected" && len(assigned) == 0 {
		return "", errors.New("指定任务请至少选择 1 位孩子")
	}

	var id string
	err := s.db.WithTx(func(tx *Tx) error {
		id = GenID("tsk_")
		var maxOrder int
		_ = tx.QueryRow("SELECT COALESCE(MAX(sort_order), 0) FROM tasks WHERE family_id = ?", func(r SQLiteRow) error {
			maxOrder = r.Int(0)
			return nil
		}, familyID)
		if _, err := tx.Exec(
			"INSERT INTO tasks (id, family_id, emoji, icon_url, label, pts, scope, task_type, duration_sec, target_count, timer_mode, require_photo, repeat_mode, repeat_days, due_date, enabled, sort_order, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)",
			id, familyID, emoji, iconURL, label, pts, scope, taskType, duration, target, timerMode, requirePhoto, repeatMode, repeatDays, dueDate, maxOrder+1, nowMs(),
		); err != nil {
			return err
		}
		if scope == "selected" {
			for _, cid := range assigned {
				var ok string
				if e := tx.QueryRow("SELECT id FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
					ok = r.String(0)
					return nil
				}, cid, familyID); e == nil && ok != "" {
					_, _ = tx.Exec("INSERT OR IGNORE INTO task_assignments (task_id, child_id) VALUES (?, ?)", id, cid)
				}
			}
		}
		return nil
	})
	return id, err
}

func (s *Store) UpdateTask(familyID, taskID string, in TaskInput) error {
	return s.db.WithTx(func(tx *Tx) error {
		var curEmoji, curIconURL, curLabel, curScope, curType, curTimerMode, curRepeatMode, curRepeatDays, curDueDate string
		var curPts, curDur, curTarget, curRequirePhoto, curEnabled int
		if err := tx.QueryRow(
			"SELECT emoji, icon_url, label, pts, scope, task_type, duration_sec, target_count, timer_mode, require_photo, repeat_mode, repeat_days, due_date, enabled FROM tasks WHERE id = ? AND family_id = ?",
			func(r SQLiteRow) error {
				curEmoji = r.String(0)
				curIconURL = r.String(1)
				curLabel = r.String(2)
				curPts = r.Int(3)
				curScope = r.String(4)
				curType = r.String(5)
				curDur = r.Int(6)
				curTarget = r.Int(7)
				curTimerMode = r.String(8)
				curRequirePhoto = r.Int(9)
				curRepeatMode = r.String(10)
				curRepeatDays = r.String(11)
				curDueDate = r.String(12)
				curEnabled = r.Int(13)
				return nil
			}, taskID, familyID,
		); err != nil {
			return errors.New("任务不存在")
		}
		if in.Label != nil {
			curLabel = strings.TrimSpace(*in.Label)
			if curLabel == "" {
				return errors.New("任务名称不能为空")
			}
		}
		if in.Pts != nil && *in.Pts > 0 {
			curPts = clampInt(*in.Pts, 1, 50)
		}
		if in.Scope != nil {
			if *in.Scope == "selected" {
				curScope = "selected"
			} else {
				curScope = "all"
			}
		}
		if in.TaskType != nil {
			if *in.TaskType == "rope_jump" {
				curType = "rope_jump"
			} else {
				curType = "normal"
			}
		}
		if in.DurationSec != nil && *in.DurationSec > 0 {
			curDur = clampInt(*in.DurationSec, 10, 7200)
		}
		if in.TargetCount != nil && *in.TargetCount > 0 {
			curTarget = clampInt(*in.TargetCount, 1, 500)
		}
		if in.TimerMode != nil {
			if *in.TimerMode == "countdown" || *in.TimerMode == "countup" {
				curTimerMode = *in.TimerMode
			} else {
				curTimerMode = "none"
			}
		}
		if in.RequirePhoto != nil {
			if *in.RequirePhoto {
				curRequirePhoto = 1
			} else {
				curRequirePhoto = 0
			}
		}
		if in.RepeatMode != nil && (*in.RepeatMode == "once" || *in.RepeatMode == "weekly" || *in.RepeatMode == "daily") {
			curRepeatMode = *in.RepeatMode
		}
		if in.RepeatDays != nil {
			curRepeatDays = strings.TrimSpace(*in.RepeatDays)
		}
		if in.DueDate != nil {
			curDueDate = strings.TrimSpace(*in.DueDate)
		}
		if curRepeatMode == "once" && curDueDate == "" {
			curDueDate = time.Now().Format("2006-01-02")
		}
		if in.Enabled != nil {
			if *in.Enabled {
				curEnabled = 1
			} else {
				curEnabled = 0
			}
		}
		if in.Emoji != nil {
			e := strings.TrimSpace(*in.Emoji)
			if e == "" {
				e = "✨"
			}
			curEmoji = e
		}
		if in.IconURL != nil {
			curIconURL = strings.TrimSpace(*in.IconURL)
		}
		if _, err := tx.Exec(
			"UPDATE tasks SET emoji = ?, icon_url = ?, label = ?, pts = ?, scope = ?, task_type = ?, duration_sec = ?, target_count = ?, timer_mode = ?, require_photo = ?, repeat_mode = ?, repeat_days = ?, due_date = ?, enabled = ? WHERE id = ? AND family_id = ?",
			curEmoji, curIconURL, curLabel, curPts, curScope, curType, curDur, curTarget, curTimerMode, curRequirePhoto, curRepeatMode, curRepeatDays, curDueDate, curEnabled, taskID, familyID,
		); err != nil {
			return err
		}
		if curScope == "all" {
			_, _ = tx.Exec("DELETE FROM task_assignments WHERE task_id = ?", taskID)
		} else if in.HasChildIDs {
			if len(in.ChildIDs) == 0 {
				return errors.New("指定任务请至少保留 1 位孩子")
			}
			_, _ = tx.Exec("DELETE FROM task_assignments WHERE task_id = ?", taskID)
			for _, cid := range in.ChildIDs {
				var ok string
				if e := tx.QueryRow("SELECT id FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
					ok = r.String(0)
					return nil
				}, cid, familyID); e == nil && ok != "" {
					_, _ = tx.Exec("INSERT OR IGNORE INTO task_assignments (task_id, child_id) VALUES (?, ?)", taskID, cid)
				}
			}
		}
		return nil
	})
}

func (s *Store) DeleteTask(familyID, taskID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		ch, err := tx.Exec("DELETE FROM tasks WHERE id = ? AND family_id = ?", taskID, familyID)
		if err != nil {
			return err
		}
		if ch == 0 {
			return errors.New("任务不存在")
		}
		return nil
	})
}

func (s *Store) GrantEgg(familyID, parentID, childID string, amount int, reason string) (map[string]any, error) {
	amt := clampInt(amount, 1, 10)
	if amount <= 0 {
		amt = 1
	}
	if strings.TrimSpace(reason) == "" {
		reason = "25积分兑换确认"
	}
	var out map[string]any
	err := s.db.WithTx(func(tx *Tx) error {
		var name string
		var coupons int
		if err := tx.QueryRow("SELECT name, coupons FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			name = r.String(0)
			coupons = r.Int(1)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("孩子档案不存在")
		}
		if _, err := tx.Exec("UPDATE children SET coupons = coupons + ? WHERE id = ?", amt, childID); err != nil {
			return err
		}
		if _, err := tx.Exec(
			"INSERT INTO egg_grants (id, family_id, child_id, parent_id, amount, reason, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			GenID("egg_"), familyID, childID, parentID, amt, reason, nowMs(),
		); err != nil {
			return err
		}
		out = map[string]any{"childName": name, "coupons": coupons + amt}
		return nil
	})
	return out, err
}

func (s *Store) CreateCustomSpecies(familyID string, sp Species) (map[string]any, error) {
	name := truncateRunes(strings.TrimSpace(sp.Name), 10)
	if name == "" {
		return nil, errors.New("请填写伙伴名字")
	}
	defStr := func(v, d string) string {
		if strings.TrimSpace(v) == "" {
			return d
		}
		return v
	}
	var out map[string]any
	err := s.db.WithTx(func(tx *Tx) error {
		if sp.TargetChildID != nil && *sp.TargetChildID != "" {
			var ok string
			if e := tx.QueryRow("SELECT id FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
				ok = r.String(0)
				return nil
			}, *sp.TargetChildID, familyID); e != nil || ok == "" {
				return errors.New("指定的孩子不存在")
			}
		} else {
			sp.TargetChildID = nil
		}
		id := GenID("sp_")
		if _, err := tx.Exec(
			"INSERT INTO custom_species (id, family_id, name, color, ear, acc, tail, mouth, base, hint, target_child_id, added_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			id, familyID, name,
			defStr(sp.Color, "#7fa8c9"), defStr(sp.Ear, "round"), defStr(sp.Acc, "none"),
			defStr(sp.Tail, "puff"), defStr(sp.Mouth, "smile"), defStr(sp.Base, "grass"),
			truncateRunes(strings.TrimSpace(sp.Hint), 20), sp.TargetChildID, nowMs(),
		); err != nil {
			return err
		}
		out = map[string]any{"id": id, "name": name}
		return nil
	})
	return out, err
}

func (s *Store) DeleteCustomSpecies(familyID, speciesID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		ch, err := tx.Exec("DELETE FROM custom_species WHERE id = ? AND family_id = ?", speciesID, familyID)
		if err != nil {
			return err
		}
		if ch == 0 {
			return ErrNotFound
		}
		_, _ = tx.Exec("DELETE FROM child_owned_species WHERE species_id = ?", speciesID)
		_, _ = tx.Exec("UPDATE children SET active_companion = NULL WHERE family_id = ? AND active_companion = ?", familyID, speciesID)
		return nil
	})
}

func (s *Store) ImportLegacyBackup(familyID, childID, rawBackupCode string) (map[string]any, error) {
	v := strings.TrimSpace(rawBackupCode)
	if !strings.HasPrefix(v, "GB1|") && !strings.HasPrefix(v, "GB2|") {
		return nil, errors.New("备份码格式不对，应以 GB1| 或 GB2| 开头")
	}
	decoded, err := base64.StdEncoding.DecodeString(v[4:])
	if err != nil {
		return nil, errors.New("备份码无法解析")
	}
	var parsed struct {
		XP              *int             `json:"xp"`
		Coupons         *int             `json:"coupons"`
		CouponsUsed     int              `json:"couponsUsed"`
		Name            string           `json:"name"`
		ActiveCompanion string           `json:"activeCompanion"`
		Species         []Species        `json:"species"`
		Owned           map[string]any   `json:"owned"`
		Log             []map[string]any `json:"log"`
		Wall            []map[string]any `json:"wall"`
	}
	if err := json.Unmarshal(decoded, &parsed); err != nil || parsed.XP == nil {
		return nil, errors.New("备份码内容无效")
	}
	xp := *parsed.XP
	coupons := 0
	if parsed.Coupons != nil {
		coupons = *parsed.Coupons
	} else {
		coupons = xp/20 - parsed.CouponsUsed
		if coupons < 0 {
			coupons = 0
		}
	}

	var out map[string]any
	err = s.db.WithTx(func(tx *Tx) error {
		var curPet string
		if err := tx.QueryRow("SELECT pet_name FROM children WHERE id = ? AND family_id = ?", func(r SQLiteRow) error {
			curPet = r.String(0)
			return nil
		}, childID, familyID); err != nil {
			return errors.New("目标孩子档案不存在")
		}
		petName := truncateRunes(parsed.Name, 12)
		if petName == "" {
			petName = curPet
		}
		var activeComp *string
		if parsed.ActiveCompanion != "" {
			ac := parsed.ActiveCompanion
			activeComp = &ac
		}
		if _, err := tx.Exec("UPDATE children SET xp = ?, coupons = ?, pet_name = ?, active_companion = ? WHERE id = ?",
			xp, coupons, petName, activeComp, childID); err != nil {
			return err
		}
		for _, sp := range parsed.Species {
			if sp.ID == "" || sp.Name == "" {
				continue
			}
			added := sp.AddedAt
			if added == 0 {
				added = nowMs()
			}
			_, _ = tx.Exec(
				"INSERT OR IGNORE INTO custom_species (id, family_id, name, color, ear, acc, tail, mouth, base, hint, target_child_id, added_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)",
				sp.ID, familyID, sp.Name, nonEmpty(sp.Color, "#7fa8c9"), nonEmpty(sp.Ear, "round"),
				nonEmpty(sp.Acc, "none"), nonEmpty(sp.Tail, "puff"), nonEmpty(sp.Mouth, "smile"),
				nonEmpty(sp.Base, "grass"), sp.Hint, added,
			)
		}
		for spID, dt := range parsed.Owned {
			if dt == nil || dt == false {
				continue
			}
			dtStr, ok := dt.(string)
			if !ok || dtStr == "" {
				dtStr = time.Now().Format("2006-01-02")
			}
			_, _ = tx.Exec("INSERT OR IGNORE INTO child_owned_species (child_id, species_id, adopted_date) VALUES (?, ?, ?)", childID, spID, dtStr)
		}
		for _, e := range parsed.Log {
			dt, _ := e["date"].(string)
			if dt == "" {
				continue
			}
			k := nonEmpty(asStr(e["k"]), "legacy")
			lbl := nonEmpty(asStr(e["label"]), "历史打卡")
			pts := asInt(e["pts"], 3)
			ts := asInt64(e["ts"], nowMs())
			_, _ = tx.Exec("INSERT OR IGNORE INTO checkin_logs (id, family_id, child_id, task_id, task_label, pts, checkin_date, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				GenID("log_"), familyID, childID, k, lbl, pts, dt, ts)
		}
		for _, w := range parsed.Wall {
			d, _ := w["d"].(string)
			if d == "" {
				continue
			}
			a := asInt(w["a"], 0)
			pet := nonEmpty(asStr(w["pet"]), "main")
			lv := asInt(w["lv"], LevelForXP(xp))
			note := asStr(w["note"])
			_, _ = tx.Exec("INSERT OR IGNORE INTO polaroids (id, family_id, child_id, avatar_idx, pet_ref, pet_lv, photo_date, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
				GenID("pho_"), familyID, childID, a, pet, lv, d, note, nowMs())
		}
		out = map[string]any{"xp": xp, "coupons": coupons}
		return nil
	})
	return out, err
}

/* ==================== Admin Operations ==================== */

func (s *Store) GetAdminByID(adminID string) *AdminDTO {
	var adm AdminDTO
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow("SELECT id, username, display_name FROM admins WHERE id = ?", func(r SQLiteRow) error {
			adm = AdminDTO{ID: r.String(0), Username: r.String(1), DisplayName: r.String(2)}
			return nil
		}, adminID)
	})
	if err != nil {
		return nil
	}
	return &adm
}

func (s *Store) GetAdminOverview(todayStr string) (map[string]any, error) {
	var out map[string]any
	err := s.db.WithLock(func(tx *Tx) error {
		families := make([]FamilyDTO, 0)
		_ = tx.Query("SELECT id, name, family_code, created_at FROM families ORDER BY created_at DESC", func(r SQLiteRow) error {
			families = append(families, FamilyDTO{ID: r.String(0), Name: r.String(1), FamilyCode: r.String(2), CreatedAt: r.Int64(3)})
			return nil
		})

		parentsByFam := make(map[string][]ParentDTO)
		parentCount := 0
		_ = tx.Query("SELECT id, family_id, username, display_name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, created_at FROM parents ORDER BY created_at ASC", func(r SQLiteRow) error {
			pid := r.String(0)
			p := ParentDTO{
				ID:          pid,
				FamilyID:    r.String(1),
				Username:    r.String(2),
				DisplayName: r.String(3),
				AvatarIdx:   clampAvatarIdx(r.Int(4)),
				AvatarURL:   avatarURL("parent", pid, r.Int(5) != 0, r.Int64(6)),
				CreatedAt:   r.Int64(7),
			}
			parentsByFam[p.FamilyID] = append(parentsByFam[p.FamilyID], p)
			parentCount++
			return nil
		})

		todayCountMap := make(map[string]int)
		totalToday := 0
		_ = tx.Query("SELECT child_id, COUNT(*) FROM checkin_logs WHERE checkin_date = ? GROUP BY child_id", func(r SQLiteRow) error {
			c := r.Int(1)
			todayCountMap[r.String(0)] = c
			totalToday += c
			return nil
		}, todayStr)

		childrenByFam := make(map[string][]map[string]any)
		childCount := 0
		_ = tx.Query("SELECT id, family_id, name, avatar_idx, LENGTH(avatar_data) > 0, avatar_updated_at, username, password_hash, pin, pet_name, xp, coupons, created_at FROM children ORDER BY created_at ASC", func(r SQLiteRow) error {
			cid := r.String(0)
			fid := r.String(1)
			xp := r.Int(10)
			childrenByFam[fid] = append(childrenByFam[fid], map[string]any{
				"id":             cid,
				"family_id":      fid,
				"name":           r.String(2),
				"avatar_idx":     clampAvatarIdx(r.Int(3)),
				"avatar_url":     avatarURL("child", cid, r.Int(4) != 0, r.Int64(5)),
				"username":       r.String(6),
				"has_password":   r.String(7) != "",
				"has_pin":        r.String(8) != "",
				"pet_name":       r.String(9),
				"xp":             xp,
				"level":          LevelForXP(xp),
				"coupons":        r.Int(11),
				"today_checkins": todayCountMap[cid],
				"created_at":     r.Int64(12),
			})
			childCount++
			return nil
		})

		taskCountMap := make(map[string]int)
		totalTasks := 0
		_ = tx.Query("SELECT family_id, COUNT(*) FROM tasks GROUP BY family_id", func(r SQLiteRow) error {
			n := r.Int(1)
			taskCountMap[r.String(0)] = n
			totalTasks += n
			return nil
		})

		famList := make([]map[string]any, 0, len(families))
		for _, f := range families {
			ps := parentsByFam[f.ID]
			if ps == nil {
				ps = make([]ParentDTO, 0)
			}
			cs := childrenByFam[f.ID]
			if cs == nil {
				cs = make([]map[string]any, 0)
			}
			famList = append(famList, map[string]any{
				"id":          f.ID,
				"name":        f.Name,
				"family_code": f.FamilyCode,
				"created_at":  f.CreatedAt,
				"task_count":  taskCountMap[f.ID],
				"parents":     ps,
				"children":    cs,
			})
		}

		out = map[string]any{
			"today": todayStr,
			"stats": map[string]any{
				"family_count":   len(families),
				"parent_count":   parentCount,
				"child_count":    childCount,
				"task_count":     totalTasks,
				"today_checkins": totalToday,
			},
			"families": famList,
		}
		return nil
	})
	return out, err
}

func (s *Store) FamilyExists(familyID string) bool {
	var ok string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow("SELECT id FROM families WHERE id = ?", func(r SQLiteRow) error {
			ok = r.String(0)
			return nil
		}, familyID)
	})
	return err == nil && ok != ""
}

func (s *Store) AdminUpdateFamily(familyID, name string) (map[string]any, error) {
	clean := truncateRunes(strings.TrimSpace(name), 24)
	if clean == "" {
		return nil, errors.New("家庭名称不能为空")
	}
	var out map[string]any
	err := s.db.WithTx(func(tx *Tx) error {
		ch, err := tx.Exec("UPDATE families SET name = ? WHERE id = ?", clean, familyID)
		if err != nil {
			return err
		}
		if ch == 0 {
			return ErrNotFound
		}
		out = map[string]any{"id": familyID, "name": clean}
		return nil
	})
	return out, err
}

func (s *Store) AdminDeleteFamily(familyID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		ch, err := tx.Exec("DELETE FROM families WHERE id = ?", familyID)
		if err != nil {
			return err
		}
		if ch == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) AdminUpdateParent(parentID string, username, displayName, password *string) (*ParentDTO, error) {
	var out *ParentDTO
	err := s.db.WithTx(func(tx *Tx) error {
		var curUser, curDisp, curHash string
		if err := tx.QueryRow("SELECT username, display_name, password_hash FROM parents WHERE id = ?", func(r SQLiteRow) error {
			curUser = r.String(0)
			curDisp = r.String(1)
			curHash = r.String(2)
			return nil
		}, parentID); err != nil {
			return ErrNotFound
		}
		if username != nil {
			u := strings.ToLower(strings.TrimSpace(*username))
			if len([]rune(u)) < 3 {
				return errors.New("家长账号至少 3 个字符")
			}
			var dup string
			if e := tx.QueryRow("SELECT id FROM parents WHERE username = ? AND id != ?", func(r SQLiteRow) error {
				dup = r.String(0)
				return nil
			}, u, parentID); e == nil && dup != "" {
				return errors.New("该家长账号已被占用")
			}
			curUser = u
		}
		if displayName != nil {
			if d := truncateRunes(strings.TrimSpace(*displayName), 16); d != "" {
				curDisp = d
			}
		}
		if password != nil && *password != "" {
			if len(*password) < 4 {
				return errors.New("密码至少 4 位")
			}
			curHash = HashPassword(*password)
		}
		if _, err := tx.Exec("UPDATE parents SET username = ?, display_name = ?, password_hash = ? WHERE id = ?", curUser, curDisp, curHash, parentID); err != nil {
			return err
		}
		out = &ParentDTO{ID: parentID, Username: curUser, DisplayName: curDisp}
		return nil
	})
	return out, err
}

func (s *Store) AdminDeleteParent(parentID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		var famID string
		if err := tx.QueryRow("SELECT family_id FROM parents WHERE id = ?", func(r SQLiteRow) error {
			famID = r.String(0)
			return nil
		}, parentID); err != nil {
			return ErrNotFound
		}
		var cnt int
		_ = tx.QueryRow("SELECT COUNT(*) FROM parents WHERE family_id = ?", func(r SQLiteRow) error {
			cnt = r.Int(0)
			return nil
		}, famID)
		if cnt <= 1 {
			return errors.New("每个家庭至少保留 1 位家长，若需清空请直接删除整个家庭")
		}
		_, err := tx.Exec("DELETE FROM parents WHERE id = ?", parentID)
		return err
	})
}

func (s *Store) AdminUpdateChild(childID string, in UpdateChildInput) (map[string]any, error) {
	var out map[string]any
	err := s.db.WithTx(func(tx *Tx) error {
		var famID string
		var curXP, curCoupons int
		if err := tx.QueryRow("SELECT family_id, xp, coupons FROM children WHERE id = ?", func(r SQLiteRow) error {
			famID = r.String(0)
			curXP = r.Int(1)
			curCoupons = r.Int(2)
			return nil
		}, childID); err != nil {
			return ErrNotFound
		}
		res, err := s.updateChildLocked(tx, famID, childID, in)
		if err != nil {
			return err
		}
		if in.XP != nil || in.Coupons != nil {
			xp := curXP
			if in.XP != nil {
				xp = *in.XP
				if xp < 0 {
					xp = 0
				}
			}
			cp := curCoupons
			if in.Coupons != nil {
				cp = *in.Coupons
				if cp < 0 {
					cp = 0
				}
			}
			if _, err := tx.Exec("UPDATE children SET xp = ?, coupons = ? WHERE id = ?", xp, cp, childID); err != nil {
				return err
			}
		}
		out = res
		return nil
	})
	return out, err
}

func (s *Store) AdminDeleteChild(childID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		var famID string
		if err := tx.QueryRow("SELECT family_id FROM children WHERE id = ?", func(r SQLiteRow) error {
			famID = r.String(0)
			return nil
		}, childID); err != nil {
			return ErrNotFound
		}
		var cnt int
		_ = tx.QueryRow("SELECT COUNT(*) FROM children WHERE family_id = ?", func(r SQLiteRow) error {
			cnt = r.Int(0)
			return nil
		}, famID)
		if cnt <= 1 {
			return errors.New("每个家庭至少保留 1 位儿童档案，若需清空请直接删除整个家庭")
		}
		_, err := tx.Exec("DELETE FROM children WHERE id = ?", childID)
		return err
	})
}

/* ==================== App OTA Releases ==================== */

func scanRelease(r SQLiteRow) AppReleaseDTO {
	return AppReleaseDTO{
		ID:           r.String(0),
		Platform:     r.String(1),
		VersionCode:  r.Int(2),
		VersionName:  r.String(3),
		ReleaseNotes: r.String(4),
		ForceUpdate:  r.Int(5) != 0,
		FileName:     r.String(6),
		FileSize:     r.Int64(7),
		SHA256:       r.String(8),
		DownloadURL:  r.String(9),
		CreatedAt:    r.Int64(10),
	}
}

func (s *Store) CreateRelease(rel AppReleaseDTO) (*AppReleaseDTO, error) {
	if rel.Platform == "" {
		rel.Platform = "android"
	}
	if rel.VersionCode <= 0 {
		return nil, errors.New("版本号（versionCode）必须为正整数")
	}
	rel.VersionName = strings.TrimSpace(rel.VersionName)
	if rel.VersionName == "" {
		return nil, errors.New("请填写版本名（如 2.1.0）")
	}
	if rel.ID == "" {
		rel.ID = GenID("rel_")
	}
	if rel.DownloadURL == "" {
		rel.DownloadURL = "/api/app/download/" + rel.ID
	}
	rel.CreatedAt = nowMs()
	forceInt := 0
	if rel.ForceUpdate {
		forceInt = 1
	}
	err := s.db.WithTx(func(tx *Tx) error {
		_, err := tx.Exec(
			"INSERT INTO app_releases (id, platform, version_code, version_name, release_notes, force_update, file_name, file_size, sha256, download_url, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			rel.ID, rel.Platform, rel.VersionCode, rel.VersionName, rel.ReleaseNotes, forceInt, rel.FileName, rel.FileSize, rel.SHA256, rel.DownloadURL, rel.CreatedAt,
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (s *Store) ListReleases(platform string) ([]AppReleaseDTO, error) {
	list := make([]AppReleaseDTO, 0)
	err := s.db.WithLock(func(tx *Tx) error {
		if platform != "" {
			return tx.Query(
				"SELECT id, platform, version_code, version_name, release_notes, force_update, file_name, file_size, sha256, download_url, created_at FROM app_releases WHERE platform = ? ORDER BY version_code DESC, created_at DESC",
				func(r SQLiteRow) error {
					list = append(list, scanRelease(r))
					return nil
				}, platform,
			)
		}
		return tx.Query(
			"SELECT id, platform, version_code, version_name, release_notes, force_update, file_name, file_size, sha256, download_url, created_at FROM app_releases ORDER BY version_code DESC, created_at DESC",
			func(r SQLiteRow) error {
				list = append(list, scanRelease(r))
				return nil
			},
		)
	})
	return list, err
}

func (s *Store) GetLatestRelease(platform string) (*AppReleaseDTO, error) {
	if platform == "" {
		platform = "android"
	}
	var rel AppReleaseDTO
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow(
			"SELECT id, platform, version_code, version_name, release_notes, force_update, file_name, file_size, sha256, download_url, created_at FROM app_releases WHERE platform = ? ORDER BY version_code DESC, created_at DESC LIMIT 1",
			func(r SQLiteRow) error {
				rel = scanRelease(r)
				return nil
			}, platform,
		)
	})
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (s *Store) GetReleaseByID(id string) (*AppReleaseDTO, error) {
	var rel AppReleaseDTO
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow(
			"SELECT id, platform, version_code, version_name, release_notes, force_update, file_name, file_size, sha256, download_url, created_at FROM app_releases WHERE id = ?",
			func(r SQLiteRow) error {
				rel = scanRelease(r)
				return nil
			}, id,
		)
	})
	if err != nil {
		return nil, err
	}
	return &rel, nil
}

func (s *Store) DeleteRelease(id string) error {
	var fileName string
	err := s.db.WithTx(func(tx *Tx) error {
		if err := tx.QueryRow("SELECT file_name FROM app_releases WHERE id = ?", func(r SQLiteRow) error {
			fileName = r.String(0)
			return nil
		}, id); err != nil {
			return ErrNotFound
		}
		_, err := tx.Exec("DELETE FROM app_releases WHERE id = ?", id)
		return err
	})
	if err != nil {
		return err
	}
	if fileName != "" && s.releasesDir != "" {
		_ = os.Remove(filepath.Join(s.releasesDir, id+".apk"))
	}
	return nil
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func asStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return def
	}
}

func asInt64(v any, def int64) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	default:
		return def
	}
}
