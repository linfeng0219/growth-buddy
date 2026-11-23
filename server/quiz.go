package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// QuizQuestion 表示在线分级题库中的一道战斗题目。
type QuizQuestion struct {
	ID         string   `json:"id"`
	Category   string   `json:"category"`   // 语文 / 数学速算 / 科学百科 / 英语 / 趣味思维
	AgeGroup   string   `json:"age_group"`  // 3-5 / 6-7 / 8 / 9-10 / 11-14
	MinAge     int      `json:"min_age"`    // 适用最小年龄
	MaxAge     int      `json:"max_age"`    // 适用最大年龄
	Difficulty string   `json:"difficulty"` // easy / normal / hard
	Question   string   `json:"question"`
	Options    []string `json:"options"`
	AnswerIdx  int      `json:"answer_idx"`
	IsOnline   bool     `json:"is_online,omitempty"`
}

// NormalizeAge 将儿童年龄规范在 3~14 岁之间（默认 7 岁）。
func NormalizeAge(age int) int {
	if age <= 0 {
		return 7
	}
	if age < 3 {
		return 3
	}
	if age > 14 {
		return 14
	}
	return age
}

// NormalizeDifficulty 将题目难度规范为 easy / normal / hard（默认 normal）。
func NormalizeDifficulty(d string) string {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "easy", "简单":
		return "easy"
	case "hard", "挑战", "困难":
		return "hard"
	default:
		return "normal"
	}
}

// AgeGroupForAge 按江苏现行中小学课程进度返回年龄对应分组与描述：
// - 3~5岁：幼儿园小/中班（认知习惯、10以内加减，无乘除与正式英语）
// - 6~7岁：小学一年级（苏教版20~100以内加减、钟表元角分与统编一年级语文，无乘除法、无正式英语）
// - 8岁：小学二年级（苏教版表内乘除法起步）
// - 9~10岁：小学三~四年级（译林版三年级起点英语正式起步 + 多位数乘除竖式）
// - 11~14岁：小学高年级 / 初中
func AgeGroupForAge(age int) (string, string) {
	a := NormalizeAge(age)
	switch {
	case a <= 5:
		return "3-5", "3~5岁（幼儿园小/中班）"
	case a <= 7:
		return "6-7", "6~7岁（小学一年级）"
	case a == 8:
		return "8", "8岁（小学二年级）"
	case a <= 10:
		return "9-10", "9~10岁（小学三~四年级）"
	default:
		return "11-14", "11~14岁（小学高年级/初中）"
	}
}

// DifficultyLabelZh 返回难度中文标签。
func DifficultyLabelZh(d string) string {
	switch NormalizeDifficulty(d) {
	case "easy":
		return "简单启蒙"
	case "hard":
		return "进阶挑战"
	default:
		return "适中闯关"
	}
}

var (
	remoteQuizMu        sync.RWMutex
	remoteQuizCache     []QuizQuestion
	remoteQuizFetchedAt time.Time
)

func loadRemoteQuestionsIfConfigured() []QuizQuestion {
	url := strings.TrimSpace(os.Getenv("QUIZ_REMOTE_URL"))
	if url == "" {
		return nil
	}

	remoteQuizMu.RLock()
	if len(remoteQuizCache) > 0 && time.Since(remoteQuizFetchedAt) < 30*time.Minute {
		out := append([]QuizQuestion(nil), remoteQuizCache...)
		remoteQuizMu.RUnlock()
		return out
	}
	remoteQuizMu.RUnlock()

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil
	}

	var list []QuizQuestion
	if err := json.Unmarshal(body, &list); err != nil {
		var wrapper struct {
			Questions []QuizQuestion `json:"questions"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil
		}
		list = wrapper.Questions
	}

	valid := make([]QuizQuestion, 0, len(list))
	for _, q := range list {
		if strings.TrimSpace(q.Question) == "" || len(q.Options) < 2 || q.AnswerIdx < 0 || q.AnswerIdx >= len(q.Options) {
			continue
		}
		q.Difficulty = NormalizeDifficulty(q.Difficulty)
		if q.AgeGroup == "" {
			q.AgeGroup, _ = AgeGroupForAge(q.MinAge)
		}
		q.IsOnline = true
		valid = append(valid, q)
	}

	remoteQuizMu.Lock()
	remoteQuizCache = valid
	remoteQuizFetchedAt = time.Now()
	remoteQuizMu.Unlock()
	return valid
}

// QueryOnlineQuestions 根据儿童年龄与难度返回在线分级题库 + 实时动态算术/逻辑题。
func QueryOnlineQuestions(age int, difficulty string, limit int) []QuizQuestion {
	normAge := NormalizeAge(age)
	ageGroup, _ := AgeGroupForAge(normAge)
	diff := NormalizeDifficulty(difficulty)
	if limit <= 0 || limit > 60 {
		limit = 24
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	allBank := make([]QuizQuestion, 0, len(onlineCuratedBank)+16)
	allBank = append(allBank, onlineCuratedBank...)
	allBank = append(allBank, loadRemoteQuestionsIfConfigured()...)

	var exact []QuizQuestion
	var sameAgeNearbyDiff []QuizQuestion
	var boostEnglish []QuizQuestion
	var boostMath []QuizQuestion
	seenID := make(map[string]bool)

	for _, q := range allBank {
		q.IsOnline = true
		if q.AgeGroup == ageGroup {
			if q.Difficulty == diff {
				exact = append(exact, q)
			} else if diff == "normal" && q.Difficulty == "easy" {
				sameAgeNearbyDiff = append(sameAgeNearbyDiff, q)
			} else if diff == "hard" && q.Difficulty == "normal" {
				sameAgeNearbyDiff = append(sameAgeNearbyDiff, q)
			} else if diff == "easy" && q.Difficulty == "normal" {
				sameAgeNearbyDiff = append(sameAgeNearbyDiff, q)
			}
		} else if diff == "hard" && isHigherGradeChallengeCandidate(ageGroup, q) {
			if q.Category == "英语" {
				boostEnglish = append(boostEnglish, q)
			} else if strings.Contains(q.Category, "数学") || q.Category == "趣味思维" || q.Category == "图形数数" {
				boostMath = append(boostMath, q)
			}
		}
	}

	shuffleQuizSlice(rng, exact)
	shuffleQuizSlice(rng, sameAgeNearbyDiff)
	shuffleQuizSlice(rng, boostEnglish)
	shuffleQuizSlice(rng, boostMath)

	dynCount := 7
	dynamicQs := generateDynamicMathQuestions(rng, ageGroup, diff, dynCount)

	selected := make([]QuizQuestion, 0, limit)
	staticQuota := limit - len(dynamicQs)
	addUnique := func(q QuizQuestion, forceDiff bool) bool {
		if q.ID != "" {
			if seenID[q.ID] {
				return false
			}
			seenID[q.ID] = true
		}
		if forceDiff {
			q.Difficulty = diff
		}
		selected = append(selected, shuffleQuestionOptions(rng, q))
		return true
	}

	// 选择「进阶挑战」时，先确保混入适量英语与高年级拓展题（各最多 3 道）
	if diff == "hard" {
		enAdded := 0
		for _, q := range exact {
			if q.Category == "英语" && enAdded < 3 && len(selected) < staticQuota {
				if addUnique(q, false) {
					enAdded++
				}
			}
		}
		for _, q := range boostEnglish {
			if enAdded < 3 && len(selected) < staticQuota {
				if addUnique(q, true) {
					enAdded++
				}
			}
		}
		mathAdded := 0
		for _, q := range exact {
			if (q.Category == "数学拓展" || q.Category == "数学") && mathAdded < 2 && len(selected) < staticQuota {
				if addUnique(q, false) {
					mathAdded++
				}
			}
		}
		for _, q := range boostMath {
			if mathAdded < 3 && len(selected) < staticQuota {
				if addUnique(q, true) {
					mathAdded++
				}
			}
		}
	}

	for _, q := range exact {
		if len(selected) >= staticQuota {
			break
		}
		addUnique(q, false)
	}
	for _, q := range sameAgeNearbyDiff {
		if len(selected) >= staticQuota {
			break
		}
		addUnique(q, false)
	}
	for _, dq := range dynamicQs {
		if len(selected) >= limit {
			break
		}
		selected = append(selected, shuffleQuestionOptions(rng, dq))
	}

	shuffleQuizSlice(rng, selected)
	return selected
}

func isHigherGradeChallengeCandidate(curGroup string, q QuizQuestion) bool {
	if q.Difficulty == "hard" {
		return false
	}
	switch curGroup {
	case "3-5":
		return q.AgeGroup == "6-7" || (q.AgeGroup == "9-10" && q.Category == "英语" && q.Difficulty == "easy")
	case "6-7":
		return q.AgeGroup == "8" || (q.AgeGroup == "9-10" && q.Category == "英语" && q.Difficulty == "easy")
	case "8":
		return q.AgeGroup == "9-10"
	case "9-10":
		return q.AgeGroup == "11-14"
	default:
		return false
	}
}

func shuffleQuizSlice(rng *rand.Rand, s []QuizQuestion) {
	for i := len(s) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func shuffleQuestionOptions(rng *rand.Rand, q QuizQuestion) QuizQuestion {
	if len(q.Options) < 2 || q.AnswerIdx < 0 || q.AnswerIdx >= len(q.Options) {
		return q
	}
	opts := append([]string(nil), q.Options...)
	correct := opts[q.AnswerIdx]
	for i := len(opts) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		opts[i], opts[j] = opts[j], opts[i]
	}
	newIdx := 0
	for i, o := range opts {
		if o == correct {
			newIdx = i
			break
		}
	}
	q.Options = opts
	q.AnswerIdx = newIdx
	return q
}

func generateDynamicMathQuestions(rng *rand.Rand, ageGroup, diff string, count int) []QuizQuestion {
	res := make([]QuizQuestion, 0, count)
	minA, maxA := ageBoundsForGroup(ageGroup)

	makeIntOpts := func(ans int) ([]string, int) {
		seen := map[int]bool{ans: true}
		cand := []int{ans}
		offsets := []int{-3, -2, -1, 1, 2, 3, 4, -4, 5, -5, 10, -10}
		for _, off := range offsets {
			if len(cand) >= 3 {
				break
			}
			v := ans + off
			if v >= 0 && !seen[v] && rng.Intn(2) == 0 {
				seen[v] = true
				cand = append(cand, v)
			}
		}
		for step := 1; len(cand) < 3; step++ {
			v := ans + step
			if !seen[v] {
				seen[v] = true
				cand = append(cand, v)
			}
		}
		opts := []string{fmt.Sprintf("%d", cand[0]), fmt.Sprintf("%d", cand[1]), fmt.Sprintf("%d", cand[2])}
		return opts, 0
	}

	for i := 0; i < count; i++ {
		var text string
		var opts []string
		var ansIdx int
		cat := "数学速算"
		kind := i % 3

		switch ageGroup {
		case "3-5":
			// 幼儿园小班/中班：easy/normal 纯 5~10 以内数数与极简单加减；hard 适当融入一年级 10~20 加法衔接
			switch diff {
			case "easy":
				a := rng.Intn(3) + 1
				b := rng.Intn(2) + 1
				if kind == 1 {
					text = fmt.Sprintf("小兔原本有 %d 根萝卜，又拿到 %d 根，一共有几根？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				} else {
					text = fmt.Sprintf("%d 加 %d 等于几？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			case "hard":
				if kind == 2 {
					cat = "数学拓展"
					if rng.Intn(2) == 0 {
						a := rng.Intn(7) + 2
						text = fmt.Sprintf("【一年级进阶】10 + %d 等于多少？", a)
						opts, ansIdx = makeIntOpts(10 + a)
					} else {
						a := rng.Intn(4) + 6
						b := rng.Intn(4) + 5
						text = fmt.Sprintf("【一年级进阶】%d + %d 等于多少？", a, b)
						opts, ansIdx = makeIntOpts(a + b)
					}
				} else {
					a := rng.Intn(4) + 5
					b := rng.Intn(4) + 2
					if kind == 1 {
						text = fmt.Sprintf("盘子里有 %d 颗草莓，吃掉了 %d 颗，还剩几颗？", a, b)
						opts, ansIdx = makeIntOpts(a - b)
					} else {
						text = fmt.Sprintf("%d + %d 等于多少？", a, 10-a)
						opts, ansIdx = makeIntOpts(10)
					}
				}
			default: // normal
				a := rng.Intn(4) + 2
				b := rng.Intn(3) + 1
				if kind == 1 {
					sum := a + b
					text = fmt.Sprintf("手里有 %d 块积木，拿走 %d 块，还剩几块？", sum, b)
					opts, ansIdx = makeIntOpts(a)
				} else {
					text = fmt.Sprintf("%d + %d 等于几？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			}

		case "6-7":
			// 江苏小学一年级（苏教版）：easy/normal 严格不涉及乘除法；hard 适当增加二年级乘除法启蒙
			switch diff {
			case "easy":
				a := rng.Intn(8) + 4
				b := rng.Intn(5) + 2
				if kind == 1 {
					text = fmt.Sprintf("%d - %d 等于多少？", a+5, b)
					opts, ansIdx = makeIntOpts(a + 5 - b)
				} else {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			case "hard":
				if kind == 0 {
					a := rng.Intn(6) + 12
					b := rng.Intn(5) + 4
					c := rng.Intn(6) + 2
					text = fmt.Sprintf("%d - %d + %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a - b + c)
				} else if kind == 1 {
					yuan := rng.Intn(3) + 1
					jiao := rng.Intn(8) + 1
					text = fmt.Sprintf("%d 元 %d 角一共是多少角？", yuan, jiao)
					opts, ansIdx = makeIntOpts(yuan*10 + jiao)
				} else {
					cat = "数学拓展"
					if rng.Intn(2) == 0 {
						a := rng.Intn(5) + 2
						b := rng.Intn(5) + 2
						text = fmt.Sprintf("【高年级数学拓展】%d 个 %d 相加（%d × %d）等于多少？", a, b, a, b)
						opts, ansIdx = makeIntOpts(a * b)
					} else {
						b := rng.Intn(3) + 2
						q := rng.Intn(4) + 2
						text = fmt.Sprintf("【高年级数学拓展】把 %d 颗草莓平均分给 %d 个小朋友，每人分几颗？", b*q, b)
						opts, ansIdx = makeIntOpts(q)
					}
				}
			default: // normal
				if kind == 0 {
					a := rng.Intn(5) + 6
					b := rng.Intn(5) + 5
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				} else if kind == 1 {
					a := rng.Intn(6) + 13
					b := rng.Intn(5) + 5
					text = fmt.Sprintf("%d - %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				} else {
					hr := rng.Intn(9) + 2
					opts = []string{fmt.Sprintf("%d 点", hr), fmt.Sprintf("%d 点半", hr), "12 点"}
					text = fmt.Sprintf("钟面上分针指向 12，时针指向 %d，现在是几点？", hr)
					ansIdx = 0
				}
			}

		case "8":
			// 江苏小学二年级（苏教版）：表内乘除法；hard 适当增加三~四年级两位数乘法与周长
			switch diff {
			case "easy":
				a := rng.Intn(4) + 2
				b := rng.Intn(5) + 2
				if kind == 1 {
					a1 := rng.Intn(30) + 25
					b1 := rng.Intn(18) + 10
					text = fmt.Sprintf("%d + %d 等于多少？", a1, b1)
					opts, ansIdx = makeIntOpts(a1 + b1)
				} else {
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				}
			case "hard":
				if kind == 0 {
					a := rng.Intn(7) + 3
					b := rng.Intn(7) + 3
					c := rng.Intn(9) + 2
					text = fmt.Sprintf("%d × %d + %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a*b + c)
				} else if kind == 1 {
					b := rng.Intn(6) + 3
					q := rng.Intn(6) + 3
					rem := rng.Intn(b-1) + 1
					total := b*q + rem
					text = fmt.Sprintf("把 %d 颗糖平均分给 %d 个小朋友，每人最多分几颗？", total, b)
					opts, ansIdx = makeIntOpts(q)
				} else {
					cat = "数学拓展"
					a := rng.Intn(8) + 12
					b := rng.Intn(4) + 3
					text = fmt.Sprintf("【高年级数学拓展】%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				}
			default: // normal
				if kind == 0 {
					a := rng.Intn(8) + 2
					b := rng.Intn(8) + 2
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				} else if kind == 1 {
					b := rng.Intn(7) + 2
					q := rng.Intn(8) + 2
					text = fmt.Sprintf("%d ÷ %d 等于多少？", b*q, b)
					opts, ansIdx = makeIntOpts(q)
				} else {
					a := rng.Intn(35) + 35
					b := rng.Intn(18) + 9
					text = fmt.Sprintf("%d - %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				}
			}

		case "9-10":
			// 三~四年级：多位数乘除、周长面积、时间换算；hard 适当增加五~六年级面积与方程
			switch diff {
			case "easy":
				a := rng.Intn(40) + 30
				b := rng.Intn(25) + 15
				text = fmt.Sprintf("%d + %d 等于多少？", a, b)
				opts, ansIdx = makeIntOpts(a + b)
			case "hard":
				if kind == 0 {
					l := rng.Intn(12) + 6
					w := rng.Intn(7) + 3
					text = fmt.Sprintf("一个长方形长 %d 厘米、宽 %d 厘米，它的周长是多少厘米？", l, w)
					opts, ansIdx = makeIntOpts((l + w) * 2)
				} else if kind == 1 {
					a := rng.Intn(15) + 12
					b := rng.Intn(7) + 3
					c := rng.Intn(20) + 10
					text = fmt.Sprintf("%d × %d - %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a*b - c)
				} else {
					cat = "数学拓展"
					x := rng.Intn(7) + 3
					a := rng.Intn(3) + 2
					b := rng.Intn(9) + 3
					text = fmt.Sprintf("【高年级数学拓展】解简单方程：%dx + %d = %d，x 等于多少？", a, b, a*x+b)
					opts, ansIdx = makeIntOpts(x)
				}
			default: // normal
				if kind == 0 {
					a := rng.Intn(18) + 12
					b := rng.Intn(7) + 3
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				} else if kind == 1 {
					b := rng.Intn(7) + 3
					q := rng.Intn(12) + 6
					text = fmt.Sprintf("%d ÷ %d 等于多少？", b*q, b)
					opts, ansIdx = makeIntOpts(q)
				} else {
					m := rng.Intn(6) + 2
					text = fmt.Sprintf("%d 千米等于多少米？", m)
					opts, ansIdx = makeIntOpts(m * 1000)
				}
			}

		default: // 11-14
			switch diff {
			case "easy":
				a := rng.Intn(80) + 110
				b := rng.Intn(40) + 25
				text = fmt.Sprintf("%d - %d 等于多少？", a, b)
				opts, ansIdx = makeIntOpts(a - b)
			case "hard":
				cat = "数学拓展"
				if kind == 0 {
					x := rng.Intn(9) + 4
					a := rng.Intn(5) + 3
					b := rng.Intn(15) + 5
					c := a*x + b
					text = fmt.Sprintf("解方程：%dx + %d = %d，x 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(x)
				} else if kind == 1 {
					v := (rng.Intn(6) + 4) * 10
					t := rng.Intn(4) + 2
					text = fmt.Sprintf("高铁平均速度是 %d 千米/时，行驶 %d 小时能走多少千米？", v, t)
					opts, ansIdx = makeIntOpts(v * t)
				} else {
					price := (rng.Intn(8) + 4) * 20
					disc := []int{8, 7, 9}[rng.Intn(3)]
					text = fmt.Sprintf("原价 %d 元的书包打 %d 折出售，现价是多少元？", price, disc)
					opts, ansIdx = makeIntOpts(price * disc / 10)
				}
			default: // normal
				if kind == 0 {
					a := rng.Intn(20) + 15
					b := rng.Intn(14) + 11
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				} else if kind == 1 {
					a1 := rng.Intn(50) + 30
					a2 := rng.Intn(60) + 40
					text = fmt.Sprintf("一个三角形的两个内角分别是 %d° 和 %d°，第三个角是多少度？", a1, a2)
					opts, ansIdx = makeIntOpts(180 - a1 - a2)
				} else {
					side := rng.Intn(10) + 5
					text = fmt.Sprintf("正方形边长是 %d 厘米，它的面积是多少平方厘米？", side)
					opts, ansIdx = makeIntOpts(side * side)
				}
			}
		}

		res = append(res, QuizQuestion{
			ID:         fmt.Sprintf("dyn_%s_%s_%d_%d", ageGroup, diff, time.Now().UnixNano()%100000, i),
			Category:   cat,
			AgeGroup:   ageGroup,
			MinAge:     minA,
			MaxAge:     maxA,
			Difficulty: diff,
			Question:   text,
			Options:    opts,
			AnswerIdx:  ansIdx,
			IsOnline:   true,
		})
	}
	return res
}

func ageBoundsForGroup(g string) (int, int) {
	switch g {
	case "3-5":
		return 3, 5
	case "6-7":
		return 6, 7
	case "8":
		return 8, 8
	case "9-10":
		return 9, 10
	default:
		return 11, 14
	}
}

// AddFamilyQuestion / DeleteFamilyQuestion 保留兼容签名，提示已切换为云端在线题库。
func (s *Store) AddFamilyQuestion(familyID string, q QuizQuestion) (QuizQuestion, error) {
	return QuizQuestion{}, errors.New("已升级为在线分级题库，无需家长手动录入题目")
}

func (s *Store) DeleteFamilyQuestion(familyID, qid string) error {
	return errors.New("已升级为在线分级题库，系统自动维护题目")
}

// onlineCuratedBank 按江苏教学进度划分学段（3-5岁小中班 / 6-7岁一年级 / 8岁二年级 / 9-10岁三四年级 / 11-14岁高年级）的精选在线中文题库。
var onlineCuratedBank = []QuizQuestion{
	// ==================== 3-5 岁 · 幼儿园小班 / 中班 ====================
	{ID: "k_e_1", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "过马路看到红灯亮起时，我们要怎么做？", Options: []string{"停下来等绿灯", "赶紧跑过去", "闭上眼睛走"}, AnswerIdx: 0},
	{ID: "k_e_2", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "小猫咪最喜欢吃什么食物呀？", Options: []string{"小鱼", "大石头", "木头"}, AnswerIdx: 0},
	{ID: "k_e_3", Category: "生活习惯", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "每天早上起床和晚上睡前都要认真做什么？", Options: []string{"刷牙洗脸", "不洗手就吃东西", "把玩具扔一地"}, AnswerIdx: 0},
	{ID: "k_e_4", Category: "颜色水果", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "熟了的大香蕉是什么颜色的？", Options: []string{"黄色", "蓝色", "紫色"}, AnswerIdx: 0},
	{ID: "k_e_5", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "哪种小动物有长长的耳朵，喜欢蹦蹦跳跳吃胡萝卜？", Options: []string{"小兔子", "小螃蟹", "小金鱼"}, AnswerIdx: 0},
	{ID: "k_e_6", Category: "生活习惯", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "吃饭之前我们必须先做什么再拿勺子筷子？", Options: []string{"把小手洗干净", "在地上打滚", "先吃十颗糖"}, AnswerIdx: 0},
	{ID: "k_n_1", Category: "儿歌童谣", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "“白毛浮绿水，红掌拨清波”写的是哪种动物？", Options: []string{"大白鹅", "小花猫", "长颈鹿"}, AnswerIdx: 0},
	{ID: "k_n_2", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "下面哪一种小动物有翅膀会在天上飞？", Options: []string{"小燕子", "小乌龟", "小金鱼"}, AnswerIdx: 0},
	{ID: "k_n_3", Category: "礼貌礼仪", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "别人帮助了我们或者送礼物时，应该礼貌地说什么？", Options: []string{"谢谢", "走开", "不理你"}, AnswerIdx: 0},
	{ID: "k_n_4", Category: "生活认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "下雨天出门，我们需要带上什么来遮雨？", Options: []string{"雨伞", "太阳镜", "扇子"}, AnswerIdx: 0},
	{ID: "k_n_5", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "什么动物的脖子特别长，能够到高高的树叶？", Options: []string{"长颈鹿", "小刺猬", "小企鹅"}, AnswerIdx: 0},
	{ID: "k_n_6", Category: "四季常识", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "堆雪人、穿厚厚羽绒服通常是在哪个季节？", Options: []string{"冬天", "夏天", "秋天"}, AnswerIdx: 0},
	{ID: "k_h_1", Category: "图形数数", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "一个三角形一共有几条边？", Options: []string{"3 条", "4 条", "5 条"}, AnswerIdx: 0},
	{ID: "k_h_2", Category: "儿歌童谣", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "“春眠不觉晓”的下一句是什么？", Options: []string{"处处闻啼鸟", "床前明月光", "鹅鹅鹅"}, AnswerIdx: 0},
	{ID: "k_h_3", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "水里游的小蝌蚪长大后会变成什么？", Options: []string{"青蛙", "蝴蝶", "小鸟"}, AnswerIdx: 0},
	{ID: "k_h_4", Category: "趣味思维", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "哥哥 5 岁，妹妹 3 岁，谁的年龄大一些？", Options: []string{"哥哥", "妹妹", "一样大"}, AnswerIdx: 0},
	{ID: "k_h_5", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "发现着火了需要拨打哪个急救报警电话？", Options: []string{"119", "120", "110"}, AnswerIdx: 0},
	{ID: "k_h_en1", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "英语单词 “apple” 指的是哪种水果？", Options: []string{"苹果", "香蕉", "西瓜"}, AnswerIdx: 0},
	{ID: "k_h_en2", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "小猫用英语怎么说？", Options: []string{"cat", "dog", "bird"}, AnswerIdx: 0},
	{ID: "k_h_en3", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "英语 “Hello!” 在见面时表示什么？", Options: []string{"你好！", "再见！", "晚安！"}, AnswerIdx: 0},
	{ID: "k_h_m1", Category: "数学拓展", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "按规律填数：2、4、6、（  ），括号里应填几？", Options: []string{"8", "7", "9"}, AnswerIdx: 0},

	// ==================== 6-7 岁 · 江苏小学一年级（无乘除、无正式英语） ====================
	{ID: "g1_e_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“床前明月光”的下一句是什么？", Options: []string{"疑是地上霜", "低头思故乡", "春眠不觉晓"}, AnswerIdx: 0},
	{ID: "g1_e_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“大”字加一笔可以变成哪个字？", Options: []string{"天", "小", "人"}, AnswerIdx: 0},
	{ID: "g1_e_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "下面哪一对词语是“反义词”？", Options: []string{"高 — 矮", "大 — 巨大", "红 — 火红"}, AnswerIdx: 0},
	{ID: "g1_e_4", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“一（ ）苹果”括号里最恰当的量词是？", Options: []string{"个", "条", "辆"}, AnswerIdx: 0},
	{ID: "g1_e_5", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "太阳每天早上从哪个方向升起？", Options: []string{"东边", "西边", "北边"}, AnswerIdx: 0},
	{ID: "g1_e_6", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“江南可采莲，莲叶何田田”描写的是哪个季节的景色？", Options: []string{"夏天", "冬天", "秋天"}, AnswerIdx: 0},
	{ID: "g1_n_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "《静夜思》这首古诗的作者是谁？", Options: []string{"李白", "杜甫", "白居易"}, AnswerIdx: 0},
	{ID: "g1_n_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“河、海、湖、江”这些汉字左边都是三点水，大多和什么有关？", Options: []string{"水", "树林", "花草"}, AnswerIdx: 0},
	{ID: "g1_n_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“远看山有色，近听水无声”这首诗描写的谜底是什么？", Options: []string{"画", "风", "云"}, AnswerIdx: 0},
	{ID: "g1_n_4", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "下面哪种动物生活在水里，用鳃呼吸？", Options: []string{"金鱼", "小鸭", "小猫"}, AnswerIdx: 0},
	{ID: "g1_n_5", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "一年有春夏秋冬四个季节，春天的下一个季节是？", Options: []string{"夏天", "秋天", "冬天"}, AnswerIdx: 0},
	{ID: "g1_n_6", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "什么东西越洗越脏？", Options: []string{"水", "肥皂", "毛巾"}, AnswerIdx: 0},
	{ID: "g1_n_7", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "麻屋子，红帐子，里面住着白胖子（打一种食物）", Options: []string{"花生", "核桃", "板栗"}, AnswerIdx: 0},
	{ID: "g1_h_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“谁知盘中餐”的下一句是什么？", Options: []string{"粒粒皆辛苦", "汗滴禾下土", "处处闻啼鸟"}, AnswerIdx: 0},
	{ID: "g1_h_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“桃、林、树、桥”这些字都有“木”字旁，一般都和什么有关？", Options: []string{"树木和木头", "火光", "石块"}, AnswerIdx: 0},
	{ID: "g1_h_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“小时不识月，呼作白玉盘”中“白玉盘”指的是什么？", Options: []string{"圆圆的月亮", "大西瓜", "白盘子"}, AnswerIdx: 0},
	{ID: "g1_h_4", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "绿色植物生长最离不开阳光、空气和什么？", Options: []string{"水", "糖果", "玻璃"}, AnswerIdx: 0},
	{ID: "g1_h_5", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "树上有 8 只小鸟，猎人开枪打响了一声（没有打中任何小鸟），树上还剩几只鸟？", Options: []string{"0 只（都吓飞了）", "8 只", "7 只"}, AnswerIdx: 0},
	{ID: "g1_h_6", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "一个星期有 7 天，一整个星期加上星期一和星期二一共有几天？", Options: []string{"9 天", "8 天", "10 天"}, AnswerIdx: 0},
	{ID: "g1_h_en1", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "英语单词 “book” 是下面哪种学习用品？", Options: []string{"书本", "尺子", "橡皮"}, AnswerIdx: 0},
	{ID: "g1_h_en2", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "表示“红色”的英语单词是哪一个？", Options: []string{"red", "blue", "green"}, AnswerIdx: 0},
	{ID: "g1_h_en3", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "别人对你说 “Thank you!” 时，可以礼貌回复哪句英语？", Options: []string{"You're welcome!", "Good night!", "How old are you?"}, AnswerIdx: 0},
	{ID: "g1_h_m1", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】4 个 3 相加（4 × 3）一共是多少？", Options: []string{"12", "7", "15"}, AnswerIdx: 0},
	{ID: "g1_h_m2", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】15 块积木平均分给 3 个小朋友，每人能分到几块？", Options: []string{"5 块", "4 块", "6 块"}, AnswerIdx: 0},

	// ==================== 8 岁 · 江苏小学二年级（开始学表内乘除法） ====================
	{ID: "g2_e_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“白日依山尽”的下一句是？", Options: []string{"黄河入海流", "欲穷千里目", "更上一层楼"}, AnswerIdx: 0},
	{ID: "g2_e_2", Category: "数学", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“三七（   ）”这句乘法口诀的得数是？", Options: []string{"二十一", "二十四", "十八"}, AnswerIdx: 0},
	{ID: "g2_e_3", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "下面哪一个长度单位比“厘米”更长？", Options: []string{"米", "毫米", "一样长"}, AnswerIdx: 0},
	{ID: "g2_n_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "“遥知不是雪，为有暗香来”描写的是哪种花？", Options: []string{"梅花", "荷花", "菊花"}, AnswerIdx: 0},
	{ID: "g2_n_2", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "用“部首查字法”查“树”字，应该先查哪个部首？", Options: []string{"木", "又", "寸"}, AnswerIdx: 0},
	{ID: "g2_n_3", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "指南针的指针停下来后，一端总是指向北方，另一端指向哪个方向？", Options: []string{"南方", "东方", "西方"}, AnswerIdx: 0},
	{ID: "g2_n_4", Category: "趣味思维", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "把 18 个苹果平均分在 3 个盘子里，每个盘子放几个？", Options: []string{"6 个", "5 个", "9 个"}, AnswerIdx: 0},
	{ID: "g2_h_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“野火烧不尽，春风吹又生”是赞美什么植物顽强的生命力？", Options: []string{"小草", "松树", "荷花"}, AnswerIdx: 0},
	{ID: "g2_h_2", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“坐井观天”故事里，小青蛙为什么觉得天只有井口那么大？", Options: []string{"井壁挡住了它的视线", "天空本来就这么小", "小青蛙近视了"}, AnswerIdx: 0},
	{ID: "g2_h_3", Category: "趣味思维", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "时钟 3 点敲 3 下用了 4 秒，那么 5 点敲 5 下需要几秒？", Options: []string{"8 秒", "6 秒", "10 秒"}, AnswerIdx: 0},
	{ID: "g2_h_en1", Category: "英语", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "英语单词 “pencil” 的中文意思是？", Options: []string{"铅笔", "书包", "黑板"}, AnswerIdx: 0},
	{ID: "g2_h_m1", Category: "数学拓展", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "【高年级数学拓展】一个正方形边长是 6 厘米，它的周长是多少厘米？", Options: []string{"24 厘米", "12 厘米", "36 厘米"}, AnswerIdx: 0},

	// ==================== 9-10 岁 · 江苏小学三~四年级（译林英语起点 + 多位数乘除） ====================
	{ID: "g3_e_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "“飞流直下三千尺”描写的是哪座名山？", Options: []string{"庐山", "泰山", "黄山"}, AnswerIdx: 0},
	{ID: "g3_e_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "下面哪个成语表示读书非常勤奋刻苦？", Options: []string{"悬梁刺股", "守株待兔", "掩耳盗铃"}, AnswerIdx: 0},
	{ID: "g3_e_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "水在标准大气压下，多少摄氏度会开始结冰？", Options: []string{"0℃", "50℃", "100℃"}, AnswerIdx: 0},
	{ID: "g3_e_4", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "英文打招呼 “Good morning!” 的意思是？", Options: []string{"早上好！", "晚安！", "再见！"}, AnswerIdx: 0},
	{ID: "g3_e_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "译林版三年级英语中，单词 “library” 是指什么地方？", Options: []string{"图书馆", "动物园", "操场"}, AnswerIdx: 0},
	{ID: "g3_n_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“停车坐爱枫林晚”中的“坐”是什么意思？", Options: []string{"因为", "坐下", "座位"}, AnswerIdx: 0},
	{ID: "g3_n_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“但使龙城飞将在，不教胡马度阴山”的作者是唐代诗人谁？", Options: []string{"王昌龄", "王维", "孟浩然"}, AnswerIdx: 0},
	{ID: "g3_n_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "地球自转一周大约需要多长时间？", Options: []string{"24 小时（一天）", "30 天（一月）", "365 天（一年）"}, AnswerIdx: 0},
	{ID: "g3_n_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "植物进行光合作用制造养分时，会吸收空气中的什么并释放氧气？", Options: []string{"吸收二氧化碳", "吸收氮气", "吸收氢气"}, AnswerIdx: 0},
	{ID: "g3_n_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "英文单词 “Wednesday” 指的是星期几？", Options: []string{"星期三", "星期二", "星期四"}, AnswerIdx: 0},
	{ID: "g3_n_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "中国古代“四大发明”不包括下面哪一项？", Options: []string{"地动仪", "造纸术", "印刷术"}, AnswerIdx: 0},
	{ID: "g3_h_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "“横看成岭侧成峰，远近高低各不同”蕴含了什么哲理？", Options: []string{"从不同角度观察事物会有不同认识", "庐山特别难爬", "冬天景色最美"}, AnswerIdx: 0},
	{ID: "g3_h_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "我国第一部纪传体通史《史记》的作者是谁？", Options: []string{"司马迁", "司马光", "孔子"}, AnswerIdx: 0},
	{ID: "g3_h_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "为什么冬天呼出的气在空中能看到“白雾”？", Options: []string{"水蒸气遇冷液化成小水滴", "空气结成了冰块", "水变成了氧气"}, AnswerIdx: 0},
	{ID: "g3_h_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "声音在哪种介质中传播速度最快？", Options: []string{"钢铁等固体", "水等液体", "空气等气体"}, AnswerIdx: 0},
	{ID: "g3_h_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "句子 “Where is my pencil case?” 是在询问什么？", Options: []string{"我的铅笔盒在哪里？", "今天星期几？", "现在几点钟？"}, AnswerIdx: 0},
	{ID: "g3_h_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "一根木头锯成 4 段需要 6 分钟，照这样计算锯成 6 段需要几分钟？", Options: []string{"10 分钟", "9 分钟", "12 分钟"}, AnswerIdx: 0},
	{ID: "g3_h_en2", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "英语中询问“你几岁了？”怎么表达最准确？", Options: []string{"How old are you?", "How are you?", "What color is it?"}, AnswerIdx: 0},
	{ID: "g3_h_m1", Category: "数学拓展", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "【高年级数学拓展】一个长方形长 12 厘米、宽 5 厘米，它的面积是多少平方厘米？", Options: []string{"60 平方厘米", "34 平方厘米", "17 平方厘米"}, AnswerIdx: 0},

	// ==================== 11-14 岁 · 小学高年级 / 初中 ====================
	{ID: "g5_e_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "我国四大名著中，以描写农民起义故事为主的是？", Options: []string{"《水浒传》", "《红楼梦》", "《西游记》"}, AnswerIdx: 0},
	{ID: "g5_e_2", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "太阳系八大行星中，体积最大的是哪颗行星？", Options: []string{"木星", "火星", "水星"}, AnswerIdx: 0},
	{ID: "g5_e_3", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "“Practice makes perfect.” 这句谚语最恰当的翻译是？", Options: []string{"熟能生巧", "千里之行，始于足下", "有志者事竟成"}, AnswerIdx: 0},
	{ID: "g5_n_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "“落霞与孤鹜齐飞，秋水共长天一色”出自唐代王勃的哪篇名作？", Options: []string{"《滕王阁序》", "《岳阳楼记》", "《醉翁亭记》"}, AnswerIdx: 0},
	{ID: "g5_n_2", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "“先天下之忧而忧，后天下之乐而乐”的作者是谁？", Options: []string{"范仲淹", "欧阳修", "苏轼"}, AnswerIdx: 0},
	{ID: "g5_n_3", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "空气中体积分数占比最高的气体是哪种？", Options: []string{"氮气（约 78%）", "氧气（约 21%）", "二氧化碳"}, AnswerIdx: 0},
	{ID: "g5_n_4", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "我国最长的河流是长江，它最终注入哪片海域？", Options: []string{"东海", "渤海", "南海"}, AnswerIdx: 0},
	{ID: "g5_n_5", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "下列单词中，“environment” 的中文意思是？", Options: []string{"环境", "发明", "鼓励"}, AnswerIdx: 0},
	{ID: "g5_n_6", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "标准国际象棋棋盘上一共有多少个黑白方格？", Options: []string{"64 个", "36 个", "81 个"}, AnswerIdx: 0},
	{ID: "g5_h_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“初唐四杰”是指王勃、杨炯、卢照邻和哪位诗人？", Options: []string{"骆宾王", "陈子昂", "贺知章"}, AnswerIdx: 0},
	{ID: "g5_h_2", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "光在真空中的传播速度大约是多少？", Options: []string{"每秒约 30 万千米", "每秒约 340 米", "每秒约 3 万千米"}, AnswerIdx: 0},
	{ID: "g5_h_3", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "人体消化和吸收营养物质最主要的器官是？", Options: []string{"小肠", "胃", "大肠"}, AnswerIdx: 0},
	{ID: "g5_h_4", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“He has lived here ____ five years.” 横线上应填哪个介词？", Options: []string{"for", "since", "at"}, AnswerIdx: 0},
	{ID: "g5_h_5", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "鸡兔同笼共 10 个头，26 条腿，请问笼子里有几只兔子？", Options: []string{"3 只", "4 只", "7 只"}, AnswerIdx: 0},
	{ID: "g5_h_6", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "1 到 100 的所有整数相加，总和是多少？", Options: []string{"5050", "5000", "4950"}, AnswerIdx: 0},
}
