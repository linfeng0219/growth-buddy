package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// QuizQuestion 表示冒险小剧场的一道答题充能题目。
type QuizQuestion struct {
	ID        string   `json:"id"`
	Category  string   `json:"category"` // 语文 / 数学 / 科学常识
	Question  string   `json:"question"`
	Options   []string `json:"options"`
	AnswerIdx int      `json:"answer_idx"`
	IsPreset  bool     `json:"is_preset,omitempty"`
}

// DefaultGrade1Questions 返回一年级预设题库（语文、数学、科学常识）。
func DefaultGrade1Questions() []QuizQuestion {
	return []QuizQuestion{
		// 语文
		{
			ID: "preset_cn_1", Category: "语文", IsPreset: true,
			Question:  "“床前明月光”的下一句是什么？",
			Options:   []string{"疑是地上霜", "低头思故乡", "春眠不觉晓"},
			AnswerIdx: 0,
		},
		{
			ID: "preset_cn_2", Category: "语文", IsPreset: true,
			Question:  "“大”字加一笔可以变成哪个字？",
			Options:   []string{"天", "小", "人"},
			AnswerIdx: 0,
		},
		{
			ID: "preset_cn_3", Category: "语文", IsPreset: true,
			Question:  "下面哪个词语是“反义词”？",
			Options:   []string{"高 — 矮", "大 — 巨大", "红 — 火红"},
			AnswerIdx: 0,
		},
		{
			ID: "preset_cn_4", Category: "语文", IsPreset: true,
			Question:  "“一（ ）苹果”括号里应该填哪个量词？",
			Options:   []string{"只", "个", "条"},
			AnswerIdx: 1,
		},
		// 数学
		{
			ID: "preset_math_1", Category: "数学", IsPreset: true,
			Question:  "8 + 5 等于多少？",
			Options:   []string{"12", "13", "14"},
			AnswerIdx: 1,
		},
		{
			ID: "preset_math_2", Category: "数学", IsPreset: true,
			Question:  "16 - 7 等于多少？",
			Options:   []string{"8", "9", "10"},
			AnswerIdx: 1,
		},
		{
			ID: "preset_math_3", Category: "数学", IsPreset: true,
			Question:  "小明有 10 颗糖，吃了 3 颗，又买了 4 颗，现在有几颗？",
			Options:   []string{"9 颗", "11 颗", "13 颗"},
			AnswerIdx: 1,
		},
		{
			ID: "preset_math_4", Category: "数学", IsPreset: true,
			Question:  "钟面上分针指向 12，时针指向 3，现在是几点？",
			Options:   []string{"12 点", "3 点", "6 点"},
			AnswerIdx: 1,
		},
		// 科学常识
		{
			ID: "preset_sci_1", Category: "科学常识", IsPreset: true,
			Question:  "太阳每天从哪个方向升起？",
			Options:   []string{"东边", "西边", "北边"},
			AnswerIdx: 0,
		},
		{
			ID: "preset_sci_2", Category: "科学常识", IsPreset: true,
			Question:  "一年有四个季节，春天的下一个季节是什么？",
			Options:   []string{"冬天", "夏天", "秋天"},
			AnswerIdx: 1,
		},
		{
			ID: "preset_sci_3", Category: "科学常识", IsPreset: true,
			Question:  "下列哪种动物是鱼类，生活在水里用鳃呼吸？",
			Options:   []string{"金鱼", "青蛙", "小鸭"},
			AnswerIdx: 0,
		},
		{
			ID: "preset_sci_4", Category: "科学常识", IsPreset: true,
			Question:  "植物生长最需要阳光、空气和什么？",
			Options:   []string{"水", "糖果", "石头"},
			AnswerIdx: 0,
		},
	}
}

func (s *Store) ensureQuizTableLocked() {
	_, _ = s.db.Exec(`CREATE TABLE IF NOT EXISTS quiz_questions (
		id TEXT PRIMARY KEY,
		family_id TEXT NOT NULL,
		category TEXT NOT NULL,
		question TEXT NOT NULL,
		options_json TEXT NOT NULL,
		answer_idx INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	)`)
}

// ListFamilyQuestions 返回家庭自定义题库 + 一年级预设题库。
func (s *Store) ListFamilyQuestions(familyID string) []QuizQuestion {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureQuizTableLocked()

	rows, err := s.db.Query(
		`SELECT id, category, question, options_json, answer_idx FROM quiz_questions WHERE family_id = ? ORDER BY created_at DESC`,
		familyID,
	)
	var list []QuizQuestion
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var q QuizQuestion
			var optJSON string
			if err := rows.Scan(&q.ID, &q.Category, &q.Question, &optJSON, &q.AnswerIdx); err == nil {
				_ = json.Unmarshal([]byte(optJSON), &q.Options)
				if len(q.Options) >= 2 {
					list = append(list, q)
				}
			}
		}
	}
	list = append(list, DefaultGrade1Questions()...)
	return list
}

// AddFamilyQuestion 为指定家庭添加一道自定义题目。
func (s *Store) AddFamilyQuestion(familyID string, q QuizQuestion) (QuizQuestion, error) {
	q.Category = strings.TrimSpace(q.Category)
	if q.Category == "" {
		q.Category = "综合"
	}
	q.Question = strings.TrimSpace(q.Question)
	if q.Question == "" {
		return QuizQuestion{}, errors.New("请输入题干")
	}
	var cleanOpts []string
	for _, o := range q.Options {
		o = strings.TrimSpace(o)
		if o != "" {
			cleanOpts = append(cleanOpts, o)
		}
	}
	if len(cleanOpts) < 2 {
		return QuizQuestion{}, errors.New("至少需要 2 个选项")
	}
	if q.AnswerIdx < 0 || q.AnswerIdx >= len(cleanOpts) {
		return QuizQuestion{}, errors.New("正确答案索引无效")
	}
	q.Options = cleanOpts
	q.ID = fmt.Sprintf("q_%d", time.Now().UnixNano())
	q.IsPreset = false

	optBytes, _ := json.Marshal(q.Options)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureQuizTableLocked()
	_, err := s.db.Exec(
		`INSERT INTO quiz_questions (id, family_id, category, question, options_json, answer_idx, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		q.ID, familyID, q.Category, q.Question, string(optBytes), q.AnswerIdx, time.Now().Unix(),
	)
	if err != nil {
		return QuizQuestion{}, err
	}
	return q, nil
}

// DeleteFamilyQuestion 删除家庭自定义题目（预设题不可删除）。
func (s *Store) DeleteFamilyQuestion(familyID, qid string) error {
	if strings.HasPrefix(qid, "preset_") {
		return errors.New("预设题库不可删除")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureQuizTableLocked()
	res, err := s.db.Exec(`DELETE FROM quiz_questions WHERE id = ? AND family_id = ?`, qid, familyID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("题目不存在")
	}
	return nil
}
