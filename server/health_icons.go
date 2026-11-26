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

// syncChildHealthLocked 按自然日结算「当前激活宠物」的健康状态：
// 1天未打卡 -> 虚弱（需10成长值恢复）；2天及以上未打卡 -> 生病（需20成长值恢复）。
// 休息中（非激活）的图鉴宠物不参与衰减。
func (s *Store) syncChildHealthLocked(tx *Tx, childID, todayStr string) {
	if todayStr == "" {
		return
	}
	todayDt, err := time.Parse("2006-01-02", todayStr)
	if err != nil {
		return
	}
	var petID, health, lastSync string
	var healProg int
	var petCreatedAt int64
	if err := tx.QueryRow(
		"SELECT id, pet_health, heal_progress, last_health_date, created_at FROM child_pets WHERE child_id = ? ORDER BY is_active DESC, created_at ASC LIMIT 1",
		func(r SQLiteRow) error {
			petID = r.String(0)
			health = r.String(1)
			healProg = r.Int(2)
			lastSync = r.String(3)
			petCreatedAt = r.Int64(4)
			return nil
		}, childID,
	); err != nil || petID == "" {
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
	if refDateStr == "" && petCreatedAt > 0 {
		refDateStr = time.UnixMilli(petCreatedAt).Format("2006-01-02")
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
		"UPDATE child_pets SET pet_health = ?, heal_progress = ?, last_health_date = ? WHERE id = ?",
		health, healProg, todayStr, petID,
	)
}

// applyEarnedPointsLocked 在孩子获得成长值时按规则结算：
// 若当前宠物处于虚弱(weak)或生病(sick)状态，须先消耗成长值恢复到健康(normal)，
// 超出恢复所需的成长值才累计到孩子的成长 XP 中；健康状态或尚未领养初始宠时全部用于成长。
func (s *Store) applyEarnedPointsLocked(tx *Tx, childID string, earnedXP int) (newXP, healUsed, grownXP int, newHealth string, newHealProg int, err error) {
	var curXP int
	if err = tx.QueryRow("SELECT xp FROM children WHERE id = ?", func(r SQLiteRow) error {
		curXP = r.Int(0)
		return nil
	}, childID); err != nil {
		return 0, 0, 0, "", 0, err
	}
	var petID, health string
	var healProg int
	_ = tx.QueryRow(
		"SELECT id, pet_health, heal_progress FROM child_pets WHERE child_id = ? ORDER BY is_active DESC, created_at ASC LIMIT 1",
		func(r SQLiteRow) error {
			petID = r.String(0)
			health = r.String(1)
			healProg = r.Int(2)
			return nil
		}, childID,
	)
	if health == "" {
		health = "normal"
	}
	newXP = curXP
	newHealth = health
	newHealProg = healProg
	if earnedXP <= 0 {
		return newXP, 0, 0, newHealth, newHealProg, nil
	}

	target := healTargetForHealth(health)
	if petID != "" && (health == "weak" || health == "sick") && target > 0 {
		needed := target - healProg
		if needed < 0 {
			needed = 0
		}
		if earnedXP < needed {
			healUsed = earnedXP
			newHealProg = healProg + earnedXP
			grownXP = 0
		} else {
			healUsed = needed
			newHealth = "normal"
			newHealProg = 0
			grownXP = earnedXP - needed
			newXP = curXP + grownXP
		}
	} else {
		grownXP = earnedXP
		newXP = curXP + earnedXP
	}

	if _, err = tx.Exec("UPDATE children SET xp = ? WHERE id = ?", newXP, childID); err != nil {
		return 0, 0, 0, "", 0, err
	}
	if petID != "" {
		if _, err = tx.Exec("UPDATE child_pets SET pet_health = ?, heal_progress = ? WHERE id = ?", newHealth, newHealProg, petID); err != nil {
			return 0, 0, 0, "", 0, err
		}
	}
	return newXP, healUsed, grownXP, newHealth, newHealProg, nil
}

// revertEarnedPointsLocked 在撤销打卡时优先扣减当前宠物恢复进度，再扣减成长 XP。
func (s *Store) revertEarnedPointsLocked(tx *Tx, childID string, pts int) error {
	if pts <= 0 {
		return nil
	}
	var curXP int
	if err := tx.QueryRow("SELECT xp FROM children WHERE id = ?", func(r SQLiteRow) error {
		curXP = r.Int(0)
		return nil
	}, childID); err != nil {
		return err
	}
	var petID, health string
	var healProg int
	_ = tx.QueryRow(
		"SELECT id, pet_health, heal_progress FROM child_pets WHERE child_id = ? ORDER BY is_active DESC, created_at ASC LIMIT 1",
		func(r SQLiteRow) error {
			petID = r.String(0)
			health = r.String(1)
			healProg = r.Int(2)
			return nil
		}, childID,
	)
	rem := pts
	if petID != "" && (health == "weak" || health == "sick") && healProg > 0 {
		deductHeal := rem
		if deductHeal > healProg {
			deductHeal = healProg
		}
		healProg -= deductHeal
		rem -= deductHeal
	}
	newXP := curXP - rem
	if newXP < 0 {
		newXP = 0
	}
	if _, err := tx.Exec("UPDATE children SET xp = ? WHERE id = ?", newXP, childID); err != nil {
		return err
	}
	if petID != "" {
		if _, err := tx.Exec("UPDATE child_pets SET heal_progress = ? WHERE id = ?", healProg, petID); err != nil {
			return err
		}
	}
	return nil
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
	return parseDataURLImage(raw)
}
