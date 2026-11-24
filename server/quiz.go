package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var dynIDSeq uint64

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

func loadLocalExtraQuestions() []QuizQuestion {
	candidates := []string{strings.TrimSpace(os.Getenv("QUIZ_EXTRA_FILE")), "quiz_extra.json"}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil || len(raw) == 0 {
			continue
		}
		var list []QuizQuestion
		if err := json.Unmarshal(raw, &list); err != nil {
			var wrapper struct {
				Questions []QuizQuestion `json:"questions"`
			}
			if err2 := json.Unmarshal(raw, &wrapper); err2 != nil {
				continue
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
		if len(valid) > 0 {
			return valid
		}
	}
	return nil
}

// QueryOnlineQuestions 根据儿童年龄与难度返回在线分级题库 + 实时动态多学科题。
func QueryOnlineQuestions(age int, difficulty string, limit int) []QuizQuestion {
	return QueryOnlineQuestionsWithExclude(age, difficulty, limit, nil)
}

// QueryOnlineQuestionsWithExclude 支持传入前端已答题目 ID 集合 excludeIDs，优先返回全新未做过的题目。
func QueryOnlineQuestionsWithExclude(age int, difficulty string, limit int, excludeIDs map[string]bool) []QuizQuestion {
	normAge := NormalizeAge(age)
	ageGroup, _ := AgeGroupForAge(normAge)
	diff := NormalizeDifficulty(difficulty)
	if limit <= 0 {
		limit = 40
	} else if limit > 80 {
		limit = 80
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	remoteQs := loadRemoteQuestionsIfConfigured()
	localExtraQs := loadLocalExtraQuestions()
	allBank := make([]QuizQuestion, 0, len(onlineCuratedBank)+len(remoteQs)+len(localExtraQs))
	allBank = append(allBank, onlineCuratedBank...)
	allBank = append(allBank, localExtraQs...)
	allBank = append(allBank, remoteQs...)

	var exactFresh, exactSeen []QuizQuestion
	var nearbyFresh, nearbySeen []QuizQuestion
	var boostEnglishFresh, boostEnglishSeen []QuizQuestion
	var boostMathFresh, boostMathSeen []QuizQuestion
	seenID := make(map[string]bool)
	seenText := make(map[string]bool)

	for _, q := range allBank {
		q.IsOnline = true
		isExcluded := q.ID != "" && excludeIDs != nil && excludeIDs[q.ID]
		if q.AgeGroup == ageGroup {
			if q.Difficulty == diff {
				if isExcluded {
					exactSeen = append(exactSeen, q)
				} else {
					exactFresh = append(exactFresh, q)
				}
			} else if (diff == "normal" && q.Difficulty == "easy") ||
				(diff == "hard" && q.Difficulty == "normal") ||
				(diff == "easy" && q.Difficulty == "normal") {
				if isExcluded {
					nearbySeen = append(nearbySeen, q)
				} else {
					nearbyFresh = append(nearbyFresh, q)
				}
			}
		} else if diff == "hard" && isHigherGradeChallengeCandidate(ageGroup, q) {
			if q.Category == "英语" {
				if isExcluded {
					boostEnglishSeen = append(boostEnglishSeen, q)
				} else {
					boostEnglishFresh = append(boostEnglishFresh, q)
				}
			} else if strings.Contains(q.Category, "数学") || q.Category == "趣味思维" || q.Category == "图形数数" {
				if isExcluded {
					boostMathSeen = append(boostMathSeen, q)
				} else {
					boostMathFresh = append(boostMathFresh, q)
				}
			}
		}
	}

	shuffleQuizSlice(rng, exactFresh)
	shuffleQuizSlice(rng, exactSeen)
	shuffleQuizSlice(rng, nearbyFresh)
	shuffleQuizSlice(rng, nearbySeen)
	shuffleQuizSlice(rng, boostEnglishFresh)
	shuffleQuizSlice(rng, boostEnglishSeen)
	shuffleQuizSlice(rng, boostMathFresh)
	shuffleQuizSlice(rng, boostMathSeen)

	dynMathCount := 7
	dynVerbalCount := 7
	if limit >= 32 {
		dynMathCount = 10
		dynVerbalCount = 10
	}
	dynamicQs := generateDynamicMathQuestions(rng, ageGroup, diff, dynMathCount)
	dynamicQs = append(dynamicQs, generateDynamicVerbalAndScienceQuestions(rng, ageGroup, diff, dynVerbalCount)...)
	shuffleQuizSlice(rng, dynamicQs)

	selected := make([]QuizQuestion, 0, limit)
	staticQuota := limit - len(dynamicQs)
	if staticQuota < 1 {
		staticQuota = limit / 2
	}
	addUnique := func(q QuizQuestion, forceDiff bool) bool {
		if q.ID != "" {
			if seenID[q.ID] {
				return false
			}
			seenID[q.ID] = true
		}
		qt := strings.TrimSpace(q.Question)
		if qt == "" || seenText[qt] {
			return false
		}
		seenText[qt] = true
		if forceDiff {
			q.Difficulty = diff
		}
		selected = append(selected, shuffleQuestionOptions(rng, q))
		return true
	}

	// 选择「进阶挑战」时，先确保混入适量英语与高年级拓展题（各最多 4 道）
	if diff == "hard" {
		enAdded := 0
		for _, pool := range [][]QuizQuestion{exactFresh, boostEnglishFresh, exactSeen, boostEnglishSeen} {
			for _, q := range pool {
				if q.Category == "英语" && enAdded < 4 && len(selected) < staticQuota {
					if addUnique(q, q.AgeGroup != ageGroup) {
						enAdded++
					}
				}
			}
		}
		mathAdded := 0
		for _, pool := range [][]QuizQuestion{exactFresh, boostMathFresh, exactSeen, boostMathSeen} {
			for _, q := range pool {
				if (q.Category == "数学拓展" || q.Category == "数学") && mathAdded < 4 && len(selected) < staticQuota {
					if addUnique(q, q.AgeGroup != ageGroup) {
						mathAdded++
					}
				}
			}
		}
	}

	// 优先抽选未做过的本档静态题 -> 未做过的同龄相邻难度题
	for _, pool := range [][]QuizQuestion{exactFresh, nearbyFresh} {
		for _, q := range pool {
			if len(selected) >= staticQuota {
				break
			}
			addUnique(q, false)
		}
	}
	// 混入本次新生成的跨学科动态题（无限变体不重样）
	for _, dq := range dynamicQs {
		if len(selected) >= limit {
			break
		}
		addUnique(dq, false)
	}
	// 若动态题去重后仍未满 limit，先继续从未做静态题补足，再按需生成额外动态题，最后回填旧题
	for _, pool := range [][]QuizQuestion{exactFresh, nearbyFresh} {
		for _, q := range pool {
			if len(selected) >= limit {
				break
			}
			addUnique(q, false)
		}
	}
	if len(selected) < limit {
		extraDyn := generateDynamicMathQuestions(rng, ageGroup, diff, limit-len(selected))
		extraDyn = append(extraDyn, generateDynamicVerbalAndScienceQuestions(rng, ageGroup, diff, limit-len(selected))...)
		for _, dq := range extraDyn {
			if len(selected) >= limit {
				break
			}
			addUnique(dq, false)
		}
	}
	for _, pool := range [][]QuizQuestion{exactSeen, nearbySeen} {
		for _, q := range pool {
			if len(selected) >= limit {
				break
			}
			addUnique(q, false)
		}
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
		kind := i % 4

		switch ageGroup {
		case "3-5":
			// 幼儿园小班/中班：easy/normal 纯 5~10 以内数数与极简单加减；hard 适当融入一年级 10~20 加减衔接
			switch diff {
			case "easy":
				a := rng.Intn(3) + 1
				b := rng.Intn(2) + 1
				switch kind {
				case 0:
					text = fmt.Sprintf("%d 加 %d 等于几？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				case 1:
					text = fmt.Sprintf("小兔原本有 %d 根萝卜，又拿到 %d 根，一共有几根？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				case 2:
					text = fmt.Sprintf("桌上有 %d 个苹果，吃掉 1 个，还剩几个？", a+1)
					opts, ansIdx = makeIntOpts(a)
				default:
					n := rng.Intn(5) + 3
					text = fmt.Sprintf("数一数，%d 后面的下一个数字是几？", n)
					opts, ansIdx = makeIntOpts(n + 1)
				}
			case "hard":
				switch kind {
				case 0:
					a := rng.Intn(4) + 5
					text = fmt.Sprintf("%d + %d 等于多少？", a, 10-a)
					opts, ansIdx = makeIntOpts(10)
				case 1:
					a := rng.Intn(4) + 5
					b := rng.Intn(4) + 2
					text = fmt.Sprintf("盘子里有 %d 颗草莓，吃掉了 %d 颗，还剩几颗？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				case 2:
					cat = "数学拓展"
					a := rng.Intn(7) + 2
					text = fmt.Sprintf("【一年级进阶】10 + %d 等于多少？", a)
					opts, ansIdx = makeIntOpts(10 + a)
				default:
					cat = "数学拓展"
					a := rng.Intn(4) + 6
					b := rng.Intn(4) + 5
					text = fmt.Sprintf("【一年级进阶】%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			default: // normal
				a := rng.Intn(4) + 2
				b := rng.Intn(3) + 1
				switch kind {
				case 0:
					text = fmt.Sprintf("%d + %d 等于几？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				case 1:
					sum := a + b
					text = fmt.Sprintf("手里有 %d 块积木，拿走 %d 块，还剩几块？", sum, b)
					opts, ansIdx = makeIntOpts(a)
				case 2:
					c := rng.Intn(3) + 2
					text = fmt.Sprintf("（）+ %d = %d，括号里应填几？", c, c+b)
					opts, ansIdx = makeIntOpts(b)
				default:
					big := rng.Intn(4) + 5
					small := rng.Intn(3) + 1
					text = fmt.Sprintf("%d 和 %d 相比，哪个数更大？", big, small)
					opts, ansIdx = []string{fmt.Sprintf("%d", big), fmt.Sprintf("%d", small), "一样大"}, 0
				}
			}

		case "6-7":
			// 江苏小学一年级（苏教版）：easy/normal 严格不涉及乘除法；hard 适当增加二年级乘除法启蒙
			switch diff {
			case "easy":
				a := rng.Intn(8) + 4
				b := rng.Intn(5) + 2
				switch kind {
				case 0:
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				case 1:
					text = fmt.Sprintf("%d - %d 等于多少？", a+5, b)
					opts, ansIdx = makeIntOpts(a + 5 - b)
				case 2:
					ten := (rng.Intn(5) + 2) * 10
					one := rng.Intn(8) + 1
					text = fmt.Sprintf("%d 个十和 %d 个一合起来是多少？", ten/10, one)
					opts, ansIdx = makeIntOpts(ten + one)
				default:
					n := rng.Intn(12) + 5
					text = fmt.Sprintf("比 %d 多 3 的数是几？", n)
					opts, ansIdx = makeIntOpts(n + 3)
				}
			case "hard":
				switch kind {
				case 0:
					a := rng.Intn(6) + 12
					b := rng.Intn(5) + 4
					c := rng.Intn(6) + 2
					text = fmt.Sprintf("%d - %d + %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a - b + c)
				case 1:
					yuan := rng.Intn(3) + 1
					jiao := rng.Intn(8) + 1
					text = fmt.Sprintf("%d 元 %d 角一共是多少角？", yuan, jiao)
					opts, ansIdx = makeIntOpts(yuan*10 + jiao)
				case 2:
					cat = "数学拓展"
					a := rng.Intn(5) + 2
					b := rng.Intn(5) + 2
					text = fmt.Sprintf("【高年级数学拓展】%d 个 %d 相加（%d × %d）等于多少？", a, b, a, b)
					opts, ansIdx = makeIntOpts(a * b)
				default:
					cat = "数学拓展"
					b := rng.Intn(3) + 2
					q := rng.Intn(4) + 2
					text = fmt.Sprintf("【高年级数学拓展】把 %d 颗草莓平均分给 %d 个小朋友，每人分几颗？", b*q, b)
					opts, ansIdx = makeIntOpts(q)
				}
			default: // normal
				switch kind {
				case 0:
					a := rng.Intn(5) + 6
					b := rng.Intn(5) + 5
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				case 1:
					a := rng.Intn(6) + 13
					b := rng.Intn(5) + 5
					text = fmt.Sprintf("%d - %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				case 2:
					hr := rng.Intn(9) + 2
					opts = []string{fmt.Sprintf("%d 点", hr), fmt.Sprintf("%d 点半", hr), "12 点"}
					text = fmt.Sprintf("钟面上分针指向 12，时针指向 %d，现在是几点？", hr)
					ansIdx = 0
				default:
					base := rng.Intn(7) + 4
					miss := rng.Intn(8) + 3
					text = fmt.Sprintf("%d + （  ） = %d，括号里应填几？", base, base+miss)
					opts, ansIdx = makeIntOpts(miss)
				}
			}

		case "8":
			// 江苏小学二年级（苏教版）：表内乘除法；hard 适当增加三~四年级两位数乘法与周长
			switch diff {
			case "easy":
				a := rng.Intn(4) + 2
				b := rng.Intn(5) + 2
				switch kind {
				case 0:
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				case 1:
					a1 := rng.Intn(30) + 25
					b1 := rng.Intn(18) + 10
					text = fmt.Sprintf("%d + %d 等于多少？", a1, b1)
					opts, ansIdx = makeIntOpts(a1 + b1)
				default:
					b2 := rng.Intn(4) + 2
					q2 := rng.Intn(4) + 2
					text = fmt.Sprintf("%d ÷ %d 等于多少？", b2*q2, b2)
					opts, ansIdx = makeIntOpts(q2)
				}
			case "hard":
				switch kind {
				case 0:
					a := rng.Intn(7) + 3
					b := rng.Intn(7) + 3
					c := rng.Intn(9) + 2
					text = fmt.Sprintf("%d × %d + %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a*b + c)
				case 1:
					b := rng.Intn(6) + 3
					q := rng.Intn(6) + 3
					rem := rng.Intn(b-1) + 1
					total := b*q + rem
					text = fmt.Sprintf("把 %d 颗糖平均分给 %d 个小朋友，每人最多分几颗？", total, b)
					opts, ansIdx = makeIntOpts(q)
				case 2:
					cat = "数学拓展"
					a := rng.Intn(8) + 12
					b := rng.Intn(4) + 3
					text = fmt.Sprintf("【高年级数学拓展】%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				default:
					cat = "数学拓展"
					l := rng.Intn(6) + 5
					w := rng.Intn(3) + 2
					text = fmt.Sprintf("【高年级数学拓展】长 %d 厘米、宽 %d 厘米的长方形，周长是多少厘米？", l, w)
					opts, ansIdx = makeIntOpts((l + w) * 2)
				}
			default: // normal
				switch kind {
				case 0:
					a := rng.Intn(8) + 2
					b := rng.Intn(8) + 2
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				case 1:
					b := rng.Intn(7) + 2
					q := rng.Intn(8) + 2
					text = fmt.Sprintf("%d ÷ %d 等于多少？", b*q, b)
					opts, ansIdx = makeIntOpts(q)
				default:
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
				if kind == 1 {
					text = fmt.Sprintf("%d - %d 等于多少？", a+30, b)
					opts, ansIdx = makeIntOpts(a + 30 - b)
				} else {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			case "hard":
				switch kind {
				case 0:
					l := rng.Intn(12) + 6
					w := rng.Intn(7) + 3
					text = fmt.Sprintf("一个长方形长 %d 厘米、宽 %d 厘米，它的周长是多少厘米？", l, w)
					opts, ansIdx = makeIntOpts((l + w) * 2)
				case 1:
					a := rng.Intn(15) + 12
					b := rng.Intn(7) + 3
					c := rng.Intn(20) + 10
					text = fmt.Sprintf("%d × %d - %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a*b - c)
				case 2:
					cat = "数学拓展"
					x := rng.Intn(7) + 3
					a := rng.Intn(3) + 2
					b := rng.Intn(9) + 3
					text = fmt.Sprintf("【高年级数学拓展】解简单方程：%dx + %d = %d，x 等于多少？", a, b, a*x+b)
					opts, ansIdx = makeIntOpts(x)
				default:
					cat = "数学拓展"
					v := (rng.Intn(5) + 4) * 10
					t := rng.Intn(3) + 2
					text = fmt.Sprintf("【高年级数学拓展】汽车每小时行驶 %d 千米，%d 小时一共行驶多少千米？", v, t)
					opts, ansIdx = makeIntOpts(v * t)
				}
			default: // normal
				switch kind {
				case 0:
					a := rng.Intn(18) + 12
					b := rng.Intn(7) + 3
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				case 1:
					b := rng.Intn(7) + 3
					q := rng.Intn(12) + 6
					text = fmt.Sprintf("%d ÷ %d 等于多少？", b*q, b)
					opts, ansIdx = makeIntOpts(q)
				default:
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
				if kind == 1 {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				} else {
					text = fmt.Sprintf("%d - %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				}
			case "hard":
				cat = "数学拓展"
				switch kind {
				case 0:
					x := rng.Intn(9) + 4
					a := rng.Intn(5) + 3
					b := rng.Intn(15) + 5
					c := a*x + b
					text = fmt.Sprintf("解方程：%dx + %d = %d，x 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(x)
				case 1:
					v := (rng.Intn(6) + 4) * 10
					t := rng.Intn(4) + 2
					text = fmt.Sprintf("高铁平均速度是 %d 千米/时，行驶 %d 小时能走多少千米？", v, t)
					opts, ansIdx = makeIntOpts(v * t)
				case 2:
					price := (rng.Intn(8) + 4) * 20
					disc := []int{8, 7, 9}[rng.Intn(3)]
					text = fmt.Sprintf("原价 %d 元的书包打 %d 折出售，现价是多少元？", price, disc)
					opts, ansIdx = makeIntOpts(price * disc / 10)
				default:
					base := (rng.Intn(6) + 3) * 2
					h := rng.Intn(6) + 4
					text = fmt.Sprintf("一个三角形的底是 %d 厘米、高是 %d 厘米，面积是多少平方厘米？", base, h)
					opts, ansIdx = makeIntOpts(base * h / 2)
				}
			default: // normal
				switch kind {
				case 0:
					a := rng.Intn(20) + 15
					b := rng.Intn(14) + 11
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				case 1:
					a1 := rng.Intn(50) + 30
					a2 := rng.Intn(60) + 40
					text = fmt.Sprintf("一个三角形的两个内角分别是 %d° 和 %d°，第三个角是多少度？", a1, a2)
					opts, ansIdx = makeIntOpts(180 - a1 - a2)
				default:
					side := rng.Intn(10) + 5
					text = fmt.Sprintf("正方形边长是 %d 厘米，它的面积是多少平方厘米？", side)
					opts, ansIdx = makeIntOpts(side * side)
				}
			}
		}

		res = append(res, QuizQuestion{
			ID:         fmt.Sprintf("dyn_%s_%s_%d_%d", ageGroup, diff, atomic.AddUint64(&dynIDSeq, 1), i),
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

// generateDynamicVerbalAndScienceQuestions 跨学科动态组合生成器（古诗接龙、反义词、量词、地理百科、分龄成语及英语词库），每轮随机组合不重复。
func generateDynamicVerbalAndScienceQuestions(rng *rand.Rand, ageGroup, diff string, count int) []QuizQuestion {
	type poemPair struct{ upper, lower, poem string }
	poemsEasy := []poemPair{
		{"白毛浮绿水", "红掌拨清波", "《咏鹅》"},
		{"床前明月光", "疑是地上霜", "《静夜思》"},
		{"春眠不觉晓", "处处闻啼鸟", "《春晓》"},
		{"锄禾日当午", "汗滴禾下土", "《悯农》"},
		{"夜来风雨声", "花落知多少", "《春晓》"},
		{"举头望明月", "低头思故乡", "《静夜思》"},
		{"小时不识月", "呼作白玉盘", "《古朗月行》"},
		{"远看山有色", "近听水无声", "《画》"},
		{"谁知盘中餐", "粒粒皆辛苦", "《悯农》"},
		{"一寸光阴一寸金", "寸金难买寸光阴", "《增广贤文》"},
	}
	poemsMid := []poemPair{
		{"白日依山尽", "黄河入海流", "《登鹳雀楼》"},
		{"欲穷千里目", "更上一层楼", "《登鹳雀楼》"},
		{"飞流直下三千尺", "疑是银河落九天", "《望庐山瀑布》"},
		{"两个黄鹂鸣翠柳", "一行白鹭上青天", "《绝句》"},
		{"不知细叶谁裁出", "二月春风似剪刀", "《咏柳》"},
		{"小荷才露尖尖角", "早有蜻蜓立上头", "《小池》"},
		{"野火烧不尽", "春风吹又生", "《草》"},
		{"春种一粒粟", "秋收万颗子", "《悯农》"},
		{"孤舟蓑笠翁", "独钓寒江雪", "《江雪》"},
		{"千山鸟飞绝", "万径人踪灭", "《江雪》"},
		{"解落三秋叶", "能开二月花", "《风》"},
		{"朝辞白帝彩云间", "千里江陵一日还", "《早发白帝城》"},
	}
	poemsHigh := []poemPair{
		{"停车坐爱枫林晚", "霜叶红于二月花", "《山行》"},
		{"接天莲叶无穷碧", "映日荷花别样红", "《晓出净慈寺送林子方》"},
		{"横看成岭侧成峰", "远近高低各不同", "《题西林壁》"},
		{"不识庐山真面目", "只缘身在此山中", "《题西林壁》"},
		{"春风又绿江南岸", "明月何时照我还", "《泊船瓜洲》"},
		{"莫愁前路无知己", "天下谁人不识君", "《别董大》"},
		{"劝君更尽一杯酒", "西出阳关无故人", "《送元二使安西》"},
		{"醉卧沙场君莫笑", "古来征战几人回", "《凉州词》"},
		{"海内存知己", "天涯若比邻", "《送杜少府之任蜀州》"},
		{"山重水复疑无路", "柳暗花明又一村", "《游山西村》"},
		{"天生我材必有用", "千金散尽还复来", "《将进酒》"},
		{"落霞与孤鹜齐飞", "秋水共长天一色", "《滕王阁序》"},
	}

	antonyms := [][2]string{
		{"高大", "矮小"}, {"炎热", "寒冷"}, {"光明", "黑暗"}, {"快速", "缓慢"}, {"柔软", "坚硬"},
		{"仔细", "粗心"}, {"谦虚", "骄傲"}, {"宽阔", "狭窄"}, {"安静", "喧闹"}, {"清晰", "模糊"},
		{"强壮", "虚弱"}, {"勤劳", "懒惰"}, {"安全", "危险"}, {"轻松", "沉重"}, {"热情", "冷漠"},
		{"笔直", "弯曲"}, {"诚实", "虚伪"}, {"干燥", "潮湿"},
	}
	measurers := [][2]string{
		{"马", "匹"}, {"汽车", "辆"}, {"轮船", "艘"}, {"飞机", "架"}, {"书本", "本"},
		{"铅笔", "支"}, {"大树", "棵"}, {"花朵", "朵"}, {"小鱼", "条"}, {"彩虹", "道"},
		{"古诗", "首"}, {"星星", "颗"}, {"图画", "幅"}, {"大象", "头"}, {"桌子", "张"},
	}
	enWords := [][2]string{
		{"apple", "苹果"}, {"banana", "香蕉"}, {"orange", "橙子"}, {"watermelon", "西瓜"}, {"grape", "葡萄"},
		{"cat", "小猫"}, {"dog", "小狗"}, {"rabbit", "兔子"}, {"panda", "熊猫"}, {"tiger", "老虎"},
		{"elephant", "大象"}, {"monkey", "猴子"}, {"bird", "小鸟"}, {"fish", "鱼"}, {"duck", "鸭子"},
		{"red", "红色"}, {"blue", "蓝色"}, {"green", "绿色"}, {"yellow", "黄色"}, {"orange/橙色", "橙色"},
		{"morning", "早上"}, {"library", "图书馆"}, {"teacher", "老师"}, {"student", "学生"}, {"friend", "朋友"},
		{"family", "家庭"}, {"water", "水"}, {"milk", "牛奶"}, {"bread", "面包"}, {"breakfast", "早餐"},
		{"Sunday", "星期日"}, {"Monday", "星期一"}, {"Tuesday", "星期二"}, {"Wednesday", "星期三"}, {"Friday", "星期五"},
		{"spring", "春天"}, {"summer", "夏天"}, {"autumn", "秋天"}, {"winter", "冬天"}, {"science", "科学"},
	}
	capitals := [][2]string{
		{"江苏", "南京"}, {"浙江", "杭州"}, {"四川", "成都"}, {"广东", "广州"}, {"湖北", "武汉"},
		{"陕西", "西安"}, {"山东", "济南"}, {"湖南", "长沙"}, {"安徽", "合肥"}, {"河南", "郑州"},
		{"辽宁", "沈阳"}, {"云南", "昆明"}, {"福建", "福州"}, {"贵州", "贵阳"},
	}

	allowEnglish := ageGroup == "9-10" || ageGroup == "11-14" || diff == "hard"
	minA, maxA := ageBoundsForGroup(ageGroup)
	res := make([]QuizQuestion, 0, count)
	seedBase := atomic.AddUint64(&dynIDSeq, 1)

	pickDistractors := func(pool []string, ans string) []string {
		idxList := rng.Perm(len(pool))
		opts := []string{ans}
		seen := map[string]bool{ans: true}
		for _, idx := range idxList {
			v := pool[idx]
			if !seen[v] {
				seen[v] = true
				opts = append(opts, v)
				if len(opts) >= 3 {
					break
				}
			}
		}
		return opts
	}

	for i := 0; i < count; i++ {
		var cat, text string
		var opts []string
		mode := i % 5
		if !allowEnglish && mode == 3 {
			mode = 4
		}

		switch mode {
		case 0: // 古诗上下句接龙
			cat = "语文"
			var pPool []poemPair
			switch ageGroup {
			case "3-5":
				pPool = poemsEasy
			case "6-7":
				pPool = poemsEasy
				if diff == "hard" {
					pPool = append(append([]poemPair(nil), poemsEasy...), poemsMid...)
				}
			case "8":
				pPool = poemsMid
			default:
				pPool = poemsHigh
			}
			item := pPool[rng.Intn(len(pPool))]
			lowers := make([]string, len(pPool))
			for k, p := range pPool {
				lowers[k] = p.lower
			}
			text = fmt.Sprintf("诗句“%s”选自%s，它的下一句是？", item.upper, item.poem)
			opts = pickDistractors(lowers, item.lower)
		case 1: // 反义词 / 量词组合
			cat = "语文"
			if ageGroup == "3-5" || (ageGroup == "6-7" && i%2 == 0) {
				m := measurers[rng.Intn(len(measurers))]
				allM := []string{"匹", "辆", "艘", "架", "本", "支", "棵", "朵", "条", "道", "首", "颗", "幅", "头", "张"}
				text = fmt.Sprintf("量词搭配：一（  ）%s，括号里最准确的是哪个？", m[0])
				opts = pickDistractors(allM, m[1])
			} else {
				a := antonyms[rng.Intn(len(antonyms))]
				askLeft := rng.Intn(2) == 0
				pool := make([]string, len(antonyms))
				var ans, qWord string
				if askLeft {
					qWord = a[0]
					ans = a[1]
					for k, p := range antonyms {
						pool[k] = p[1]
					}
				} else {
					qWord = a[1]
					ans = a[0]
					for k, p := range antonyms {
						pool[k] = p[0]
					}
				}
				text = fmt.Sprintf("词语“%s”的反义词是下面哪一个？", qWord)
				opts = pickDistractors(pool, ans)
			}
		case 2: // 百科与地理省会组合
			cat = "科学百科"
			if ageGroup == "3-5" || ageGroup == "6-7" {
				// 幼儿与一年级保持浅显生活自然科普
				facts := [][2]string{
					{"火警报警电话", "119"},
					{"急救中心救护车电话", "120"},
					{"报警求助电话", "110"},
					{"交通安全故障电话", "122"},
				}
				f := facts[rng.Intn(len(facts))]
				text = fmt.Sprintf("当遇到紧急情况需要拨打「%s」时，正确的号码是？", f[0])
				opts = pickDistractors([]string{"110", "119", "120", "122", "114"}, f[1])
			} else {
				c := capitals[rng.Intn(len(capitals))]
				capList := make([]string, len(capitals))
				for k, p := range capitals {
					capList[k] = p[1]
				}
				text = fmt.Sprintf("中国地理小百科：%s省的省会城市是哪一座？", c[0])
				opts = pickDistractors(capList, c[1])
			}
		case 3: // 英语词汇双向随机组题（仅 9-14岁 或 hard）
			cat = "英语"
			idx := rng.Intn(len(enWords))
			w := enWords[idx]
			if w[0] == "orange/橙色" {
				w[0] = "orange"
			}
			enToZh := rng.Intn(2) == 0
			if enToZh {
				zhPool := make([]string, len(enWords))
				for k, e := range enWords {
					zhPool[k] = e[1]
				}
				text = fmt.Sprintf("英语单词 “%s” 对应的正确中文意思是？", w[0])
				opts = pickDistractors(zhPool, w[1])
			} else {
				enPool := make([]string, len(enWords))
				for k, e := range enWords {
					enPool[k] = strings.TrimSuffix(e[0], "/橙色")
				}
				text = fmt.Sprintf("中文“%s”用下面哪个英语单词表示最准确？", w[1])
				opts = pickDistractors(enPool, w[0])
			}
		default: // 分龄分级成语动态组题（填字补全 + 看释义猜成语）
			cat = "语文"
			text, opts = generateDynamicIdiomQuestion(rng, ageGroup, diff, pickDistractors)
		}

		if len(opts) >= 2 && text != "" {
			res = append(res, QuizQuestion{
				ID:         fmt.Sprintf("dynv_%s_%s_%d_%d", ageGroup, diff, seedBase, i),
				Category:   cat,
				AgeGroup:   ageGroup,
				MinAge:     minA,
				MaxAge:     maxA,
				Difficulty: diff,
				Question:   text,
				Options:    opts,
				AnswerIdx:  0,
				IsOnline:   true,
			})
		}
	}
	return res
}

type dynIdiomEntry struct {
	idiom   string
	blanked string
	ansChar string
	meaning string
	alts    [2]string
}

// generateDynamicIdiomQuestion 根据年龄段和难度动态生成成语填字补全或看释义猜成语题目。
func generateDynamicIdiomQuestion(rng *rand.Rand, ageGroup, diff string, pickDistractors func([]string, string) []string) (string, []string) {
	idioms35 := []dynIdiomEntry{
		{"五颜六色", "五（  ）六色", "颜", "颜色非常多、很好看", [2]string{"朵", "片"}},
		{"一心一意", "一（  ）一意", "心", "心思专注、不分散", [2]string{"头", "手"}},
		{"七上八下", "七（  ）八下", "上", "心里慌乱不踏实", [2]string{"左", "前"}},
		{"春暖花开", "春（  ）花开", "暖", "春天到来温暖开花", [2]string{"冷", "凉"}},
		{"鸟语花香", "鸟（  ）花香", "语", "小鸟鸣叫、花朵飘香", [2]string{"飞", "跑"}},
		{"三心二意", "三（  ）二意", "心", "做事不专心、常变主意", [2]string{"眼", "手"}},
		{"欢天喜地", "欢（  ）喜地", "天", "形容非常开心快乐", [2]string{"云", "山"}},
		{"十全十美", "十（  ）十美", "全", "样样都好没有缺点", [2]string{"半", "少"}},
		{"四面八方", "四（  ）八方", "面", "指周围各个方向", [2]string{"边", "角"}},
		{"东张西望", "东（  ）西望", "张", "向周围到处看、不专心", [2]string{"走", "跑"}},
	}
	idioms67 := []dynIdiomEntry{
		{"一举两得", "一（  ）两得", "举", "做一件事同时收获两个好处", [2]string{"走", "拿"}},
		{"千山万水", "千（  ）万水", "山", "经过很多高山大河、路途遥远", [2]string{"风", "云"}},
		{"手舞足蹈", "手（  ）足蹈", "舞", "高兴得拍手跳脚", [2]string{"拿", "摆"}},
		{"大步流星", "大（  ）流星", "步", "脚步迈得大、走得极快", [2]string{"跑", "飞"}},
		{"百花齐放", "百（  ）齐放", "花", "各种鲜花一齐盛开", [2]string{"草", "树"}},
		{"兴高采烈", "兴（  ）采烈", "高", "兴致极高、情绪热烈", [2]string{"低", "大"}},
		{"山清水秀", "山（  ）水秀", "清", "山水风景十分优美", [2]string{"高", "青"}},
		{"人山人海", "人（  ）人海", "山", "聚集的人非常多、很热闹", [2]string{"水", "林"}},
		{"津津有味", "津（  ）有味", "津", "觉得特别有趣味或很香甜", [2]string{"金", "进"}},
		{"左顾右盼", "左（  ）右盼", "顾", "向左右两边不停看", [2]string{"看", "跑"}},
		{"争先恐后", "争（  ）恐后", "先", "抢着向前、生怕落后", [2]string{"前", "早"}},
		{"无边无际", "无（  ）无际", "边", "极其宽广、望不到尽头", [2]string{"处", "头"}},
	}
	idioms8 := []dynIdiomEntry{
		{"名列前茅", "名列前（  ）", "茅", "成绩或名次排在最前列", [2]string{"矛", "毛"}},
		{"专心致志", "专心（  ）志", "致", "心思集中、非常专心", [2]string{"制", "智"}},
		{"风和日丽", "风（  ）日丽", "和", "微风温和、阳光明媚", [2]string{"高", "平"}},
		{"天寒地冻", "天寒地（  ）", "冻", "冬天非常寒冷、大地结冰", [2]string{"动", "冷"}},
		{"鸦雀无声", "（  ）雀无声", "鸦", "形容非常安静、一点声响都没有", [2]string{"鸭", "鸡"}},
		{"井井有条", "井井有（  ）", "条", "做事或物品整理得很有秩序", [2]string{"理", "序"}},
		{"迫不及待", "迫不（  ）待", "及", "心里急切得等不下去", [2]string{"急", "即"}},
		{"胸有成竹", "胸有成（  ）", "竹", "做事之前心里早就很有把握", [2]string{"足", "书"}},
		{"水滴石穿", "水（  ）石穿", "滴", "比喻坚持不断做小事就能攻克难关", [2]string{"流", "冲"}},
		{"画龙点睛", "画龙点（  ）", "睛", "关键处加上精彩一笔使内容更生动", [2]string{"晴", "眼"}},
		{"惊弓之鸟", "惊（  ）之鸟", "弓", "受过惊吓、一遇动静就害怕的人", [2]string{"风", "剑"}},
		{"锲而不舍", "锲而不（  ）", "舍", "比喻有恒心、坚持不懈地做下去", [2]string{"弃", "停"}},
	}
	idioms910 := []dynIdiomEntry{
		{"精益求精", "精（  ）求精", "益", "已经很好了还追求更好", [2]string{"意", "艺"}},
		{"络绎不绝", "络（  ）不绝", "绎", "行人车马前后接连不断", [2]string{"译", "驿"}},
		{"拾金不昧", "拾金不（  ）", "昧", "捡到别人钱物不私自藏起来", [2]string{"味", "妹"}},
		{"波澜壮阔", "波（  ）壮阔", "澜", "水面波浪巨大、气势壮观", [2]string{"兰", "栏"}},
		{"三顾茅庐", "三（  ）茅庐", "顾", "诚心诚意多次上门拜访人才", [2]string{"雇", "故"}},
		{"破釜沉舟", "破（  ）沉舟", "釜", "下定必胜决心、不留退路", [2]string{"斧", "父"}},
		{"韦编三绝", "（  ）编三绝", "韦", "形容勤奋读书反复研读", [2]string{"伟", "围"}},
		{"再接再厉", "再接再（  ）", "厉", "一次又一次继续努力不松懈", [2]string{"励", "历"}},
		{"完璧归赵", "完（  ）归赵", "璧", "把借用或托付的东西完好归还", [2]string{"壁", "碧"}},
		{"闻鸡起舞", "闻鸡起（  ）", "舞", "有志之人及时发奋努力", [2]string{"武", "午"}},
		{"老马识途", "老马识（  ）", "途", "有经验的人熟悉情况能作向导", [2]string{"图", "涂"}},
		{"塞翁失马", "（  ）翁失马", "塞", "坏事在一定条件下可能变成好事", [2]string{"赛", "寒"}},
	}
	idioms1114 := []dynIdiomEntry{
		{"循序渐进", "循序（  ）进", "渐", "按一定顺序步骤慢慢深入提高", [2]string{"剑", "建"}},
		{"实事求是", "实事求（  ）", "是", "从客观实际出发探求真相和规律", [2]string{"事", "实"}},
		{"不耻下问", "不（  ）下问", "耻", "不以向不如自己的人请教为羞耻", [2]string{"齿", "尺"}},
		{"见贤思齐", "见贤思（  ）", "齐", "见到德才兼备的人就想向他看齐", [2]string{"奇", "期"}},
		{"任重道远", "任（  ）道远", "重", "责任重大而且路途遥远需长期奋斗", [2]string{"众", "中"}},
		{"废寝忘食", "废（  ）忘食", "寝", "专心学习或工作忘记睡觉和吃饭", [2]string{"睡", "清"}},
		{"厚积薄发", "厚积（  ）发", "薄", "长期充分积累后再慢慢展现才华", [2]string{"博", "泊"}},
		{"杞人忧天", "（  ）人忧天", "杞", "为不必要或没根据的事情白发愁", [2]string{"起", "企"}},
		{"缘木求鱼", "（  ）木求鱼", "缘", "方法或方向不对头达不到目的", [2]string{"原", "圆"}},
		{"班门弄斧", "班门弄（  ）", "斧", "在行家面前卖弄自己不熟练的本领", [2]string{"父", "府"}},
		{"曲高和寡", "曲高（  ）寡", "和", "作品或言论越深奥理解附和的人越少", [2]string{"合", "河"}},
		{"汗牛充栋", "汗牛充（  ）", "栋", "形容藏书或著作数量极其多", [2]string{"冻", "洞"}},
		{"相濡以沫", "相（  ）以沫", "濡", "身处困境互相以微薄力量救助扶持", [2]string{"儒", "需"}},
		{"韬光养晦", "（  ）光养晦", "韬", "隐藏才能不轻易炫耀并积蓄力量", [2]string{"滔", "涛"}},
		{"美轮美奂", "美轮美（  ）", "奂", "形容高大华美的建筑物或装饰", [2]string{"换", "焕"}},
		{"巧夺天工", "巧夺天（  ）", "工", "人工制作极其精巧胜过天然", [2]string{"功", "公"}},
	}

	var pool []dynIdiomEntry
	switch ageGroup {
	case "3-5":
		pool = append(pool, idioms35...)
		if diff == "hard" {
			pool = append(pool, idioms67[:6]...)
		}
	case "6-7":
		pool = append(pool, idioms67...)
		if diff == "easy" {
			pool = append(pool, idioms35[5:]...)
		} else if diff == "hard" {
			pool = append(pool, idioms8[:6]...)
		}
	case "8":
		pool = append(pool, idioms8...)
		if diff == "easy" {
			pool = append(pool, idioms67[4:]...)
		} else if diff == "hard" {
			pool = append(pool, idioms910[:6]...)
		}
	case "9-10":
		pool = append(pool, idioms910...)
		if diff == "easy" {
			pool = append(pool, idioms8[4:]...)
		} else if diff == "hard" {
			pool = append(pool, idioms1114[:6]...)
		}
	default: // 11-14
		pool = append(pool, idioms1114...)
		if diff == "easy" {
			pool = append(pool, idioms910[4:]...)
		}
	}

	e := pool[rng.Intn(len(pool))]
	askBlank := rng.Intn(2) == 0
	if askBlank {
		text := fmt.Sprintf("动态成语填空：%s（意指：%s），括号里应填哪个字？", e.blanked, e.meaning)
		opts := []string{e.ansChar, e.alts[0], e.alts[1]}
		return text, opts
	}

	names := make([]string, len(pool))
	for i, item := range pool {
		names[i] = item.idiom
	}
	text := fmt.Sprintf("成语猜一猜：形容“%s”，最恰当的是哪个成语？", e.meaning)
	opts := pickDistractors(names, e.idiom)
	return text, opts
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

// onlineCuratedBank 按江苏教学进度划分学段（3-5岁小中班 / 6-7岁一年级 / 8岁二年级 / 9-10岁三四年级 / 11-14岁高年级）的精选在线中文题库。
var onlineCuratedBank = []QuizQuestion{
	// ==================== 3-5 岁 · 幼儿园小班 / 中班 ====================
	{ID: "k_e_1", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "过马路看到红灯亮起时，我们要怎么做？", Options: []string{"停下来等绿灯", "赶紧跑过去", "闭上眼睛走"}, AnswerIdx: 0},
	{ID: "k_e_2", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "小猫咪最喜欢吃什么食物呀？", Options: []string{"小鱼", "大石头", "木头"}, AnswerIdx: 0},
	{ID: "k_e_3", Category: "生活习惯", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "每天早上起床和晚上睡前都要认真做什么？", Options: []string{"刷牙洗脸", "不洗手就吃东西", "把玩具扔一地"}, AnswerIdx: 0},
	{ID: "k_e_4", Category: "颜色水果", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "熟了的大香蕉是什么颜色的？", Options: []string{"黄色", "蓝色", "紫色"}, AnswerIdx: 0},
	{ID: "k_e_5", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "哪种小动物有长长的耳朵，喜欢蹦蹦跳跳吃胡萝卜？", Options: []string{"小兔子", "小螃蟹", "小金鱼"}, AnswerIdx: 0},
	{ID: "k_e_6", Category: "生活习惯", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "吃饭之前我们必须先做什么再拿勺子筷子？", Options: []string{"把小手洗干净", "在地上打滚", "先吃十颗糖"}, AnswerIdx: 0},
	{ID: "k_e_7", Category: "图形颜色", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "晴朗的天空和大海通常是什么颜色的？", Options: []string{"蓝色", "紫色", "黑色"}, AnswerIdx: 0},
	{ID: "k_e_8", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "小鸭子在水里游的时候会怎么叫？", Options: []string{"嘎嘎嘎", "汪汪汪", "喵喵喵"}, AnswerIdx: 0},
	{ID: "k_e_9", Category: "生活认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "自行车有几个轮子？", Options: []string{"2 个", "4 个", "6 个"}, AnswerIdx: 0},
	{ID: "k_e_10", Category: "颜色水果", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "切开大西瓜，里面的瓜瓤通常是什么颜色？", Options: []string{"红色", "白色", "蓝色"}, AnswerIdx: 0},
	{ID: "k_e_11", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "家里带电的插座，小朋友可以用小手或铁丝去戳吗？", Options: []string{"绝对不可以", "可以戳一下", "用湿手摸"}, AnswerIdx: 0},
	{ID: "k_e_12", Category: "生活习惯", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "easy", Question: "玩完积木和玩具之后，好习惯是？", Options: []string{"自己把玩具收进盒子里", "扔在门口不管", "踩在玩具上跳"}, AnswerIdx: 0},
	{ID: "k_n_1", Category: "儿歌童谣", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "“白毛浮绿水，红掌拨清波”写的是哪种动物？", Options: []string{"大白鹅", "小花猫", "长颈鹿"}, AnswerIdx: 0},
	{ID: "k_n_2", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "下面哪一种小动物有翅膀会在天上飞？", Options: []string{"小燕子", "小乌龟", "小金鱼"}, AnswerIdx: 0},
	{ID: "k_n_3", Category: "礼貌礼仪", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "别人帮助了我们或者送礼物时，应该礼貌地说什么？", Options: []string{"谢谢", "走开", "不理你"}, AnswerIdx: 0},
	{ID: "k_n_4", Category: "生活认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "下雨天出门，我们需要带上什么来遮雨？", Options: []string{"雨伞", "太阳镜", "扇子"}, AnswerIdx: 0},
	{ID: "k_n_5", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "什么动物的脖子特别长，能够到高高的树叶？", Options: []string{"长颈鹿", "小刺猬", "小企鹅"}, AnswerIdx: 0},
	{ID: "k_n_6", Category: "四季常识", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "堆雪人、穿厚厚羽绒服通常是在哪个季节？", Options: []string{"冬天", "夏天", "秋天"}, AnswerIdx: 0},
	{ID: "k_n_7", Category: "图形颜色", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "圆圆的皮球和硬币都是什么形状的？", Options: []string{"圆形", "正方形", "三角形"}, AnswerIdx: 0},
	{ID: "k_n_8", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "什么小昆虫最勤劳，喜欢去花朵里采花蜜？", Options: []string{"小蜜蜂", "小蚂蚁", "小蚯蚓"}, AnswerIdx: 0},
	{ID: "k_n_9", Category: "生活认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "晚上天黑时，天空中常常会出现弯弯的什么？", Options: []string{"月亮和星星", "太阳和彩虹", "红绿灯"}, AnswerIdx: 0},
	{ID: "k_n_10", Category: "动物认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "哪种大动物鼻子长长的，像一根水管能吸水？", Options: []string{"大象", "狮子", "河马"}, AnswerIdx: 0},
	{ID: "k_n_11", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "在商场里如果不小心和爸爸妈妈走散了，应该找谁帮忙？", Options: []string{"穿制服的工作人员或警察", "随便跟陌生人走", "一个人躲进厕所哭"}, AnswerIdx: 0},
	{ID: "k_n_12", Category: "四季常识", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "normal", Question: "春天到了，树枝上新长出来的嫩芽通常是什么颜色？", Options: []string{"绿色", "雪白色", "黑褐色"}, AnswerIdx: 0},
	{ID: "k_h_1", Category: "图形数数", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "一个三角形一共有几条边？", Options: []string{"3 条", "4 条", "5 条"}, AnswerIdx: 0},
	{ID: "k_h_2", Category: "儿歌童谣", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "“春眠不觉晓”的下一句是什么？", Options: []string{"处处闻啼鸟", "床前明月光", "鹅鹅鹅"}, AnswerIdx: 0},
	{ID: "k_h_3", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "水里游的小蝌蚪长大后会变成什么？", Options: []string{"青蛙", "蝴蝶", "小鸟"}, AnswerIdx: 0},
	{ID: "k_h_4", Category: "趣味思维", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "哥哥 5 岁，妹妹 3 岁，谁的年龄大一些？", Options: []string{"哥哥", "妹妹", "一样大"}, AnswerIdx: 0},
	{ID: "k_h_5", Category: "生活安全", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "发现着火了需要拨打哪个急救报警电话？", Options: []string{"119", "120", "110"}, AnswerIdx: 0},
	{ID: "k_h_6", Category: "自然认知", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "毛毛虫睡在茧里长大后，会变成漂亮飞舞的什么？", Options: []string{"蝴蝶", "蜻蜓", "甲虫"}, AnswerIdx: 0},
	{ID: "k_h_7", Category: "趣味思维", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "天亮了太阳公公出来，那晚上大家睡着的时候是谁值班？", Options: []string{"月亮和星星", "彩虹和闪电", "大公鸡"}, AnswerIdx: 0},
	{ID: "k_h_en1", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "英语单词 “apple” 指的是哪种水果？", Options: []string{"苹果", "香蕉", "西瓜"}, AnswerIdx: 0},
	{ID: "k_h_en2", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "小猫用英语怎么说？", Options: []string{"cat", "dog", "bird"}, AnswerIdx: 0},
	{ID: "k_h_en3", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "英语 “Hello!” 在见面时表示什么？", Options: []string{"你好！", "再见！", "晚安！"}, AnswerIdx: 0},
	{ID: "k_h_en4", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "小狗的英文单词是哪一个？", Options: []string{"dog", "duck", "pig"}, AnswerIdx: 0},
	{ID: "k_h_en5", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "英语数字 “three” 表示数字几？", Options: []string{"3", "2", "5"}, AnswerIdx: 0},
	{ID: "k_h_en6", Category: "英语", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "跟好朋友挥手告别时，用英语怎么说？", Options: []string{"Bye-bye!", "Good morning!", "Thank you!"}, AnswerIdx: 0},
	{ID: "k_h_m1", Category: "数学拓展", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "按规律填数：2、4、6、（  ），括号里应填几？", Options: []string{"8", "7", "9"}, AnswerIdx: 0},
	{ID: "k_h_m2", Category: "数学拓展", AgeGroup: "3-5", MinAge: 3, MaxAge: 5, Difficulty: "hard", Question: "一个正方形手帕有几条一样长的边、几个角？", Options: []string{"4 条边、4 个角", "3 条边、3 个角", "5 条边、4 个角"}, AnswerIdx: 0},

	// ==================== 6-7 岁 · 江苏小学一年级（无乘除、无正式英语） ====================
	{ID: "g1_e_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“床前明月光”的下一句是什么？", Options: []string{"疑是地上霜", "低头思故乡", "春眠不觉晓"}, AnswerIdx: 0},
	{ID: "g1_e_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“大”字加一笔可以变成哪个字？", Options: []string{"天", "小", "人"}, AnswerIdx: 0},
	{ID: "g1_e_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "下面哪一对词语是“反义词”？", Options: []string{"高 — 矮", "大 — 巨大", "红 — 火红"}, AnswerIdx: 0},
	{ID: "g1_e_4", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“一（ ）苹果”括号里最恰当的量词是？", Options: []string{"个", "条", "辆"}, AnswerIdx: 0},
	{ID: "g1_e_5", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "太阳每天早上从哪个方向升起？", Options: []string{"东边", "西边", "北边"}, AnswerIdx: 0},
	{ID: "g1_e_6", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“江南可采莲，莲叶何田田”描写的是哪个季节的景色？", Options: []string{"夏天", "冬天", "秋天"}, AnswerIdx: 0},
	{ID: "g1_e_7", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "下面哪组词是一对“反义词”？", Options: []string{"前 — 后", "花 — 花朵", "跑 — 跑步"}, AnswerIdx: 0},
	{ID: "g1_e_8", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“一（ ）小鸟”括号里最恰当的量词是？", Options: []string{"只", "条", "头"}, AnswerIdx: 0},
	{ID: "g1_e_9", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“鹅，鹅，鹅，曲项向天歌”的下一句是？", Options: []string{"白毛浮绿水", "疑是地上霜", "春晓处处闻啼鸟"}, AnswerIdx: 0},
	{ID: "g1_e_10", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "早上起床面向太阳站立，我们的后面是哪个方向？", Options: []string{"西边", "东边", "南边"}, AnswerIdx: 0},
	{ID: "g1_e_11", Category: "生活常识", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "我国的国庆节是每年的哪一天？", Options: []string{"10 月 1 日", "1 月 1 日", "6 月 1 日"}, AnswerIdx: 0},
	{ID: "g1_e_12", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "easy", Question: "“春去花还在”的下一句是？", Options: []string{"人来鸟不惊", "红掌拨清波", "低头思故乡"}, AnswerIdx: 0},
	{ID: "g1_n_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "《静夜思》这首古诗的作者是谁？", Options: []string{"李白", "杜甫", "白居易"}, AnswerIdx: 0},
	{ID: "g1_n_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“河、海、湖、江”这些汉字左边都是三点水，大多和什么有关？", Options: []string{"水", "树林", "花草"}, AnswerIdx: 0},
	{ID: "g1_n_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“远看山有色，近听水无声”这首诗描写的谜底是什么？", Options: []string{"画", "风", "云"}, AnswerIdx: 0},
	{ID: "g1_n_4", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "下面哪种动物生活在水里，用鳃呼吸？", Options: []string{"金鱼", "小鸭", "小猫"}, AnswerIdx: 0},
	{ID: "g1_n_5", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "一年有春夏秋冬四个季节，春天的下一个季节是？", Options: []string{"夏天", "秋天", "冬天"}, AnswerIdx: 0},
	{ID: "g1_n_6", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "什么东西越洗越脏？", Options: []string{"水", "肥皂", "毛巾"}, AnswerIdx: 0},
	{ID: "g1_n_7", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "麻屋子，红帐子，里面住着白胖子（打一种食物）", Options: []string{"花生", "核桃", "板栗"}, AnswerIdx: 0},
	{ID: "g1_n_8", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“花、草、莲、苹”这些字的偏旁都是草字头，大多和什么有关？", Options: []string{"植物花草", "动物羽毛", "金属石头"}, AnswerIdx: 0},
	{ID: "g1_n_9", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "《雪地里的小画家》里，小马在雪地上画出了什么？", Options: []string{"月牙", "竹叶", "梅花"}, AnswerIdx: 0},
	{ID: "g1_n_10", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "为什么中午时太阳下的影子最短，早上和傍晚最长？", Options: []string{"太阳高度角不同", "树长高了", "地面变斜了"}, AnswerIdx: 0},
	{ID: "g1_n_11", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "一年中一共有多少个月？", Options: []string{"12 个月", "10 个月", "24 个月"}, AnswerIdx: 0},
	{ID: "g1_n_12", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "写字的时候，汉字“十”的正确笔顺是先写哪一笔？", Options: []string{"先横后竖", "先竖后横", "先撇后捺"}, AnswerIdx: 0},
	{ID: "g1_n_13", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "normal", Question: "“千条线，万条线，落到水里看不见”，这是哪种自然现象？", Options: []string{"下雨", "下雪", "刮风"}, AnswerIdx: 0},
	{ID: "g1_h_1", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“谁知盘中餐”的下一句是什么？", Options: []string{"粒粒皆辛苦", "汗滴禾下土", "处处闻啼鸟"}, AnswerIdx: 0},
	{ID: "g1_h_2", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“桃、林、树、桥”这些字都有“木”字旁，一般都和什么有关？", Options: []string{"树木和木头", "火光", "石块"}, AnswerIdx: 0},
	{ID: "g1_h_3", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "“小时不识月，呼作白玉盘”中“白玉盘”指的是什么？", Options: []string{"圆圆的月亮", "大西瓜", "白盘子"}, AnswerIdx: 0},
	{ID: "g1_h_4", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "绿色植物生长最离不开阳光、空气和什么？", Options: []string{"水", "糖果", "玻璃"}, AnswerIdx: 0},
	{ID: "g1_h_5", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "树上有 8 只小鸟，猎人开枪打响了一声（没有打中任何小鸟），树上还剩几只鸟？", Options: []string{"0 只（都吓飞了）", "8 只", "7 只"}, AnswerIdx: 0},
	{ID: "g1_h_6", Category: "趣味思维", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "一个星期有 7 天，一整个星期加上星期一和星期二一共有几天？", Options: []string{"9 天", "8 天", "10 天"}, AnswerIdx: 0},
	{ID: "g1_h_7", Category: "语文", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "《风》这首诗里“解落三秋叶，能开二月花”的后面一句是？", Options: []string{"过江千尺浪", "入竹万竿斜", "处处闻啼鸟"}, AnswerIdx: 0},
	{ID: "g1_h_8", Category: "科学百科", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "我们喝的水变成冰之后，是变冷了还是变热了？", Options: []string{"变冷了（0℃以下结冰）", "变热了", "温度不变"}, AnswerIdx: 0},
	{ID: "g1_h_en1", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "英语单词 “book” 是下面哪种学习用品？", Options: []string{"书本", "尺子", "橡皮"}, AnswerIdx: 0},
	{ID: "g1_h_en2", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "表示“红色”的英语单词是哪一个？", Options: []string{"red", "blue", "green"}, AnswerIdx: 0},
	{ID: "g1_h_en3", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "别人对你说 “Thank you!” 时，可以礼貌回复哪句英语？", Options: []string{"You're welcome!", "Good night!", "How old are you?"}, AnswerIdx: 0},
	{ID: "g1_h_en4", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "英语单词 “ruler” 是文具盒里的什么物品？", Options: []string{"尺子", "铅笔", "书包"}, AnswerIdx: 0},
	{ID: "g1_h_en5", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "英语 “Good evening!” 表示什么时候问好？", Options: []string{"晚上好", "早上好", "中午好"}, AnswerIdx: 0},
	{ID: "g1_h_en6", Category: "英语", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "英文 “banana” 和 “orange” 分别是哪两种水果？", Options: []string{"香蕉和橙子", "草莓和葡萄", "苹果和西瓜"}, AnswerIdx: 0},
	{ID: "g1_h_m1", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】4 个 3 相加（4 × 3）一共是多少？", Options: []string{"12", "7", "15"}, AnswerIdx: 0},
	{ID: "g1_h_m2", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】15 块积木平均分给 3 个小朋友，每人能分到几块？", Options: []string{"5 块", "4 块", "6 块"}, AnswerIdx: 0},
	{ID: "g1_h_m3", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】乘法口诀“五六（  ）”，括号里应该填？", Options: []string{"三十", "二十五", "三十五"}, AnswerIdx: 0},
	{ID: "g1_h_m4", Category: "数学拓展", AgeGroup: "6-7", MinAge: 6, MaxAge: 7, Difficulty: "hard", Question: "【高年级数学拓展】1 米一共等于多少厘米？", Options: []string{"100 厘米", "10 厘米", "50 厘米"}, AnswerIdx: 0},

	// ==================== 8 岁 · 江苏小学二年级（开始学表内乘除法） ====================
	{ID: "g2_e_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“白日依山尽”的下一句是？", Options: []string{"黄河入海流", "欲穷千里目", "更上一层楼"}, AnswerIdx: 0},
	{ID: "g2_e_2", Category: "数学", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“三七（   ）”这句乘法口诀的得数是？", Options: []string{"二十一", "二十四", "十八"}, AnswerIdx: 0},
	{ID: "g2_e_3", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "下面哪一个长度单位比“厘米”更长？", Options: []string{"米", "毫米", "一样长"}, AnswerIdx: 0},
	{ID: "g2_e_4", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“飞流直下三千尺，疑是银河落九天”出自哪位诗人？", Options: []string{"李白", "王之涣", "白居易"}, AnswerIdx: 0},
	{ID: "g2_e_5", Category: "数学", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "easy", Question: "“八九（   ）”这句乘法口诀的得数是？", Options: []string{"七十二", "六十四", "八十一"}, AnswerIdx: 0},
	{ID: "g2_n_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "“遥知不是雪，为有暗香来”描写的是哪种花？", Options: []string{"梅花", "荷花", "菊花"}, AnswerIdx: 0},
	{ID: "g2_n_2", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "用“部首查字法”查“树”字，应该先查哪个部首？", Options: []string{"木", "又", "寸"}, AnswerIdx: 0},
	{ID: "g2_n_3", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "指南针的指针停下来后，一端总是指向北方，另一端指向哪个方向？", Options: []string{"南方", "东方", "西方"}, AnswerIdx: 0},
	{ID: "g2_n_4", Category: "趣味思维", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "把 18 个苹果平均分在 3 个盘子里，每个盘子放几个？", Options: []string{"6 个", "5 个", "9 个"}, AnswerIdx: 0},
	{ID: "g2_n_5", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "下面哪个成语讲的是“说话算数，非常诚实”？", Options: []string{"一诺千金", "画蛇添足", "狐假虎威"}, AnswerIdx: 0},
	{ID: "g2_n_6", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "normal", Question: "在野外如果树叶茂密与稀疏不一致，面向太阳那面（树叶更茂盛）通常是？", Options: []string{"南方", "北方", "西方"}, AnswerIdx: 0},
	{ID: "g2_h_1", Category: "语文", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“野火烧不尽，春风吹又生”是赞美什么植物顽强的生命力？", Options: []string{"小草", "松树", "荷花"}, AnswerIdx: 0},
	{ID: "g2_h_2", Category: "科学百科", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "“坐井观天”故事里，小青蛙为什么觉得天只有井口那么大？", Options: []string{"井壁挡住了它的视线", "天空本来就这么小", "小青蛙近视了"}, AnswerIdx: 0},
	{ID: "g2_h_3", Category: "趣味思维", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "时钟 3 点敲 3 下用了 4 秒，那么 5 点敲 5 下需要几秒？", Options: []string{"8 秒", "6 秒", "10 秒"}, AnswerIdx: 0},
	{ID: "g2_h_en1", Category: "英语", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "英语单词 “pencil” 的中文意思是？", Options: []string{"铅笔", "书包", "黑板"}, AnswerIdx: 0},
	{ID: "g2_h_en2", Category: "英语", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "英文询问“你叫什么名字？”是哪一句？", Options: []string{"What's your name?", "How old are you?", "Where are you from?"}, AnswerIdx: 0},
	{ID: "g2_h_m1", Category: "数学拓展", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "【高年级数学拓展】一个正方形边长是 6 厘米，它的周长是多少厘米？", Options: []string{"24 厘米", "12 厘米", "36 厘米"}, AnswerIdx: 0},
	{ID: "g2_h_m2", Category: "数学拓展", AgeGroup: "8", MinAge: 8, MaxAge: 8, Difficulty: "hard", Question: "【高年级数学拓展】1 千米（公里）一共等于多少米？", Options: []string{"1000 米", "100 米", "10000 米"}, AnswerIdx: 0},

	// ==================== 9-10 岁 · 江苏小学三~四年级（译林英语起点 + 多位数乘除） ====================
	{ID: "g3_e_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "“飞流直下三千尺”描写的是哪座名山？", Options: []string{"庐山", "泰山", "黄山"}, AnswerIdx: 0},
	{ID: "g3_e_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "下面哪个成语表示读书非常勤奋刻苦？", Options: []string{"悬梁刺股", "守株待兔", "掩耳盗铃"}, AnswerIdx: 0},
	{ID: "g3_e_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "水在标准大气压下，多少摄氏度会开始结冰？", Options: []string{"0℃", "50℃", "100℃"}, AnswerIdx: 0},
	{ID: "g3_e_4", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "英文打招呼 “Good morning!” 的意思是？", Options: []string{"早上好！", "晚安！", "再见！"}, AnswerIdx: 0},
	{ID: "g3_e_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "译林版三年级英语中，单词 “library” 是指什么地方？", Options: []string{"图书馆", "动物园", "操场"}, AnswerIdx: 0},
	{ID: "g3_e_6", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "“欲穷千里目，更上一层楼”是王之涣写哪座名楼的诗句？", Options: []string{"鹳雀楼", "黄鹤楼", "岳阳楼"}, AnswerIdx: 0},
	{ID: "g3_e_7", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "英语单词 “breakfast” 指的是一天中的哪一顿饭？", Options: []string{"早餐", "午餐", "晚餐"}, AnswerIdx: 0},
	{ID: "g3_n_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“停车坐爱枫林晚”中的“坐”是什么意思？", Options: []string{"因为", "坐下", "座位"}, AnswerIdx: 0},
	{ID: "g3_n_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“但使龙城飞将在，不教胡马度阴山”的作者是唐代诗人谁？", Options: []string{"王昌龄", "王维", "孟浩然"}, AnswerIdx: 0},
	{ID: "g3_n_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "地球自转一周大约需要多长时间？", Options: []string{"24 小时（一天）", "30 天（一月）", "365 天（一年）"}, AnswerIdx: 0},
	{ID: "g3_n_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "植物进行光合作用制造养分时，会吸收空气中的什么并释放氧气？", Options: []string{"吸收二氧化碳", "吸收氮气", "吸收氢气"}, AnswerIdx: 0},
	{ID: "g3_n_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "英文单词 “Wednesday” 指的是星期几？", Options: []string{"星期三", "星期二", "星期四"}, AnswerIdx: 0},
	{ID: "g3_n_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "中国古代“四大发明”不包括下面哪一项？", Options: []string{"地动仪", "造纸术", "印刷术"}, AnswerIdx: 0},
	{ID: "g3_n_7", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "在英语课程名称中，“Science” 指的是哪门学科？", Options: []string{"科学", "美术", "音乐"}, AnswerIdx: 0},
	{ID: "g3_n_8", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "磁铁拥有两个磁极，同极相斥，异极会怎样？", Options: []string{"相吸", "排斥", "不变"}, AnswerIdx: 0},
	{ID: "g3_h_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "“横看成岭侧成峰，远近高低各不同”蕴含了什么哲理？", Options: []string{"从不同角度观察事物会有不同认识", "庐山特别难爬", "冬天景色最美"}, AnswerIdx: 0},
	{ID: "g3_h_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "我国第一部纪传体通史《史记》的作者是谁？", Options: []string{"司马迁", "司马光", "孔子"}, AnswerIdx: 0},
	{ID: "g3_h_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "为什么冬天呼出的气在空中能看到“白雾”？", Options: []string{"水蒸气遇冷液化成小水滴", "空气结成了冰块", "水变成了氧气"}, AnswerIdx: 0},
	{ID: "g3_h_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "声音在哪种介质中传播速度最快？", Options: []string{"钢铁等固体", "水等液体", "空气等气体"}, AnswerIdx: 0},
	{ID: "g3_h_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "句子 “Where is my pencil case?” 是在询问什么？", Options: []string{"我的铅笔盒在哪里？", "今天星期几？", "现在几点钟？"}, AnswerIdx: 0},
	{ID: "g3_h_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "一根木头锯成 4 段需要 6 分钟，照这样计算锯成 6 段需要几分钟？", Options: []string{"10 分钟", "9 分钟", "12 分钟"}, AnswerIdx: 0},
	{ID: "g3_h_en2", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "英语中询问“你几岁了？”怎么表达最准确？", Options: []string{"How old are you?", "How are you?", "What color is it?"}, AnswerIdx: 0},
	{ID: "g3_h_en3", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "英文句子 “Don't shout in the library.” 的含义是？", Options: []string{"不要在图书馆里大喊大叫。", "欢迎来图书馆看书。", "图书馆今天关门了。"}, AnswerIdx: 0},
	{ID: "g3_h_m1", Category: "数学拓展", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "【高年级数学拓展】一个长方形长 12 厘米、宽 5 厘米，它的面积是多少平方厘米？", Options: []string{"60 平方厘米", "34 平方厘米", "17 平方厘米"}, AnswerIdx: 0},
	{ID: "g3_h_m2", Category: "数学拓展", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "【高年级数学拓展】一个三角形的内角和一共是多少度？", Options: []string{"180°", "90°", "360°"}, AnswerIdx: 0},

	// ==================== 11-14 岁 · 小学高年级 / 初中 ====================
	{ID: "g5_e_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "我国四大名著中，以描写农民起义故事为主的是？", Options: []string{"《水浒传》", "《红楼梦》", "《西游记》"}, AnswerIdx: 0},
	{ID: "g5_e_2", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "太阳系八大行星中，体积最大的是哪颗行星？", Options: []string{"木星", "火星", "水星"}, AnswerIdx: 0},
	{ID: "g5_e_3", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "“Practice makes perfect.” 这句谚语最恰当的翻译是？", Options: []string{"熟能生巧", "千里之行，始于足下", "有志者事竟成"}, AnswerIdx: 0},
	{ID: "g5_e_4", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "easy", Question: "地球表面大约有多少比例的面积被海洋覆盖？", Options: []string{"约 71%", "约 29%", "约 50%"}, AnswerIdx: 0},
	{ID: "g5_n_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "“落霞与孤鹜齐飞，秋水共长天一色”出自唐代王勃的哪篇名作？", Options: []string{"《滕王阁序》", "《岳阳楼记》", "《醉翁亭记》"}, AnswerIdx: 0},
	{ID: "g5_n_2", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "“先天下之忧而忧，后天下之乐而乐”的作者是谁？", Options: []string{"范仲淹", "欧阳修", "苏轼"}, AnswerIdx: 0},
	{ID: "g5_n_3", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "空气中体积分数占比最高的气体是哪种？", Options: []string{"氮气（约 78%）", "氧气（约 21%）", "二氧化碳"}, AnswerIdx: 0},
	{ID: "g5_n_4", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "我国最长的河流是长江，它最终注入哪片海域？", Options: []string{"东海", "渤海", "南海"}, AnswerIdx: 0},
	{ID: "g5_n_5", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "下列单词中，“environment” 的中文意思是？", Options: []string{"环境", "发明", "鼓励"}, AnswerIdx: 0},
	{ID: "g5_n_6", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "标准国际象棋棋盘上一共有多少个黑白方格？", Options: []string{"64 个", "36 个", "81 个"}, AnswerIdx: 0},
	{ID: "g5_n_7", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "normal", Question: "短语 “look forward to” 后面接动词时，动词通常用什么形式？", Options: []string{"动名词（-ing）", "动词过去式", "动词原形"}, AnswerIdx: 0},
	{ID: "g5_h_1", Category: "语文", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“初唐四杰”是指王勃、杨炯、卢照邻和哪位诗人？", Options: []string{"骆宾王", "陈子昂", "贺知章"}, AnswerIdx: 0},
	{ID: "g5_h_2", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "光在真空中的传播速度大约是多少？", Options: []string{"每秒约 30 万千米", "每秒约 340 米", "每秒约 3 万千米"}, AnswerIdx: 0},
	{ID: "g5_h_3", Category: "科学百科", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "人体消化和吸收营养物质最主要的器官是？", Options: []string{"小肠", "胃", "大肠"}, AnswerIdx: 0},
	{ID: "g5_h_4", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“He has lived here ____ five years.” 横线上应填哪个介词？", Options: []string{"for", "since", "at"}, AnswerIdx: 0},
	{ID: "g5_h_5", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "鸡兔同笼共 10 个头，26 条腿，请问笼子里有几只兔子？", Options: []string{"3 只", "4 只", "7 只"}, AnswerIdx: 0},
	{ID: "g5_h_6", Category: "趣味思维", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "1 到 100 的所有整数相加，总和是多少？", Options: []string{"5050", "5000", "4950"}, AnswerIdx: 0},
	{ID: "g5_h_en2", Category: "英语", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "“Not only you but also he ____ interested in reading.” 横线处填？", Options: []string{"is", "are", "were"}, AnswerIdx: 0},
	{ID: "g5_h_m1", Category: "数学拓展", AgeGroup: "11-14", MinAge: 11, MaxAge: 14, Difficulty: "hard", Question: "圆的周长公式中，如果半径 r = 5 厘米（π ≈ 3.14），它的周长约为多少厘米？", Options: []string{"31.4 厘米", "15.7 厘米", "78.5 厘米"}, AnswerIdx: 0},
}
