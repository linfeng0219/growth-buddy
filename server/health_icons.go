package main

import (
	"errors"
	"strings"
	"time"
)

type TaskIconDTO struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Label     string `json:"label"`
	ImageURL  string `json:"image_url"`
	CreatedAt int64  `json:"created_at"`
}

func healTargetForHealth(health string) int {
	switch health {
	case "weak":
		return 10
	case "sick":
		return 20
	default:
		return 0
	}
}

// syncChildHealthLocked 按自然日结算宠物健康状态：
// 1天未打卡 -> 虚弱（需10成长值恢复）；2天及以上未打卡 -> 生病（需20成长值恢复）。
func (s *Store) syncChildHealthLocked(tx *Tx, childID, todayStr string) {
	if todayStr == "" {
		return
	}
	todayDt, err := time.Parse("2006-01-02", todayStr)
	if err != nil {
		return
	}
	var health, lastSync string
	var healProg int
	var createdAt int64
	if err := tx.QueryRow(
		"SELECT pet_health, heal_progress, last_health_date, created_at FROM children WHERE id = ?",
		func(r SQLiteRow) error {
			health = r.String(0)
			healProg = r.Int(1)
			lastSync = r.String(2)
			createdAt = r.Int64(3)
			return nil
		}, childID,
	); err != nil {
		return
	}
	if lastSync == todayStr {
		return
	}

	var refDateStr string
	_ = tx.QueryRow(
		"SELECT checkin_date FROM checkin_logs WHERE child_id = ? AND checkin_date <= ? ORDER BY checkin_date DESC LIMIT 1",
		func(r SQLiteRow) error {
			refDateStr = r.String(0)
			return nil
		}, childID, todayStr,
	)
	if refDateStr == "" && createdAt > 0 {
		refDateStr = time.UnixMilli(createdAt).Format("2006-01-02")
	}
	if refDateStr != "" {
		if refDt, err := time.Parse("2006-01-02", refDateStr); err == nil {
			gapDays := int(todayDt.Sub(refDt).Hours() / 24)
			missedDays := gapDays - 1
			if missedDays >= 2 {
				if health != "sick" {
					health = "sick"
					healProg = 0
				}
			} else if missedDays == 1 {
				if health == "normal" {
					health = "weak"
					healProg = 0
				}
			}
		}
	}
	_, _ = tx.Exec(
		"UPDATE children SET pet_health = ?, heal_progress = ?, last_health_date = ? WHERE id = ?",
		health, healProg, todayStr, childID,
	)
}

// applyHealProgressLocked 在孩子打卡获得成长值时累计恢复进度。
func (s *Store) applyHealProgressLocked(tx *Tx, childID string, earnedXP int) {
	if earnedXP <= 0 {
		return
	}
	var health string
	var healProg int
	if err := tx.QueryRow(
		"SELECT pet_health, heal_progress FROM children WHERE id = ?",
		func(r SQLiteRow) error {
			health = r.String(0)
			healProg = r.Int(1)
			return nil
		}, childID,
	); err != nil || health == "normal" || health == "" {
		return
	}
	healProg += earnedXP
	target := healTargetForHealth(health)
	if target > 0 && healProg >= target {
		health = "normal"
		healProg = 0
	}
	_, _ = tx.Exec("UPDATE children SET pet_health = ?, heal_progress = ? WHERE id = ?", health, healProg, childID)
}

func (s *Store) ListFamilyTaskIcons(familyID string) ([]TaskIconDTO, error) {
	icons := make([]TaskIconDTO, 0)
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.Query(
			"SELECT id, category, label, created_at FROM family_task_icons WHERE family_id = ? ORDER BY created_at DESC",
			func(r SQLiteRow) error {
				id := r.String(0)
				icons = append(icons, TaskIconDTO{
					ID:        id,
					Category:  r.String(1),
					Label:     r.String(2),
					ImageURL:  "/api/task-icon/" + id,
					CreatedAt: r.Int64(3),
				})
				return nil
			}, familyID,
		)
	})
	return icons, err
}

func (s *Store) AddFamilyTaskIcon(familyID, label, category, imageData string) (*TaskIconDTO, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "自定义图标"
	}
	category = strings.TrimSpace(category)
	if category == "" {
		category = "自定义"
	}
	cleanImg := strings.TrimSpace(imageData)
	if !strings.HasPrefix(cleanImg, "data:image/") {
		return nil, errors.New("请上传有效的图片文件")
	}
	if len(cleanImg) > 512*1024 {
		return nil, errors.New("图标图片体积过大，请不超过 500KB")
	}
	id := GenID("ico_")
	ts := nowMs()
	err := s.db.WithTx(func(tx *Tx) error {
		_, err := tx.Exec(
			"INSERT INTO family_task_icons (id, family_id, category, label, image_data, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			id, familyID, category, label, cleanImg, ts,
		)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &TaskIconDTO{
		ID:        id,
		Category:  category,
		Label:     label,
		ImageURL:  "/api/task-icon/" + id,
		CreatedAt: ts,
	}, nil
}

func (s *Store) DeleteFamilyTaskIcon(familyID, iconID string) error {
	return s.db.WithTx(func(tx *Tx) error {
		n, err := tx.Exec("DELETE FROM family_task_icons WHERE id = ? AND family_id = ?", iconID, familyID)
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.New("图标不存在")
		}
		return nil
	})
}

func (s *Store) GetTaskIconData(iconID string) (string, []byte, error) {
	var raw string
	err := s.db.WithLock(func(tx *Tx) error {
		return tx.QueryRow("SELECT image_data FROM family_task_icons WHERE id = ?", func(r SQLiteRow) error {
			raw = r.String(0)
			return nil
		}, iconID)
	})
	if err != nil || raw == "" {
		return "", nil, ErrNotFound
	}
	return decodeDataURL(raw)
}
