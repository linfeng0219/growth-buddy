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
	Category   string   `json:"category"`   // 语文 / 数学 / 科学百科 / 英语 / 趣味思维
	AgeGroup   string   `json:"age_group"`  // 3-6 / 7-8 / 9-10 / 11-14
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

// AgeGroupForAge 返回年龄对应的分组编码和中文描述。
func AgeGroupForAge(age int) (string, string) {
	a := NormalizeAge(age)
	switch {
	case a <= 6:
		return "3-6", "3~6岁（幼儿启蒙）"
	case a <= 8:
		return "7-8", "7~8岁（小学低年级）"
	case a <= 10:
		return "9-10", "9~10岁（小学中年级）"
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

	// 1. 汇集后台在线精选库 + 可选远程同步库
	allBank := make([]QuizQuestion, 0, len(onlineCuratedBank)+16)
	allBank = append(allBank, onlineCuratedBank...)
	allBank = append(allBank, loadRemoteQuestionsIfConfigured()...)

	// 2. 分层匹配：优先精确匹配同年龄段+同难度，其次同年龄段相近难度
	var exact []QuizQuestion
	var sameAgeNearbyDiff []QuizQuestion
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
		}
	}

	shuffleQuizSlice(rng, exact)
	shuffleQuizSlice(rng, sameAgeNearbyDiff)

	// 3. 实时生成 7 道动态数学/思维题，防止背答案和重复
	dynCount := 7
	dynamicQs := generateDynamicMathQuestions(rng, ageGroup, diff, dynCount)

	selected := make([]QuizQuestion, 0, limit)
	// 先放精确匹配的静态在线题（最多占约 2/3）
	staticQuota := limit - len(dynamicQs)
	for _, q := range exact {
		if len(selected) >= staticQuota {
			break
		}
		selected = append(selected, shuffleQuestionOptions(rng, q))
	}
	for _, q := range sameAgeNearbyDiff {
		if len(selected) >= staticQuota {
			break
		}
		selected = append(selected, shuffleQuestionOptions(rng, q))
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
		kind := i % 3

		switch ageGroup {
		case "3-6":
			switch diff {
			case "easy":
				a := rng.Intn(4) + 1
				b := rng.Intn(4) + 1
				if kind == 1 && a >= b {
					text = fmt.Sprintf("小兔子有 %d 根胡萝卜，吃掉 %d 根，还剩几根？", a+1, b)
					opts, ansIdx = makeIntOpts(a + 1 - b)
				} else {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			case "hard":
				a := rng.Intn(9) + 6
				b := rng.Intn(7) + 3
				if kind == 1 {
					text = fmt.Sprintf("树上原来有 %d 只小鸟，飞走了 %d 只，还剩几只？", a+5, b)
					opts, ansIdx = makeIntOpts(a + 5 - b)
				} else {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			default: // normal
				a := rng.Intn(6) + 2
				b := rng.Intn(5) + 1
				if kind == 1 {
					sum := a + b
					text = fmt.Sprintf("%d - %d 等于多少？", sum, b)
					opts, ansIdx = makeIntOpts(a)
				} else {
					text = fmt.Sprintf("盘子里有 %d 个草莓，又放进 %d 个，一共几个？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			}
		case "7-8":
			switch diff {
			case "easy":
				a := rng.Intn(10) + 5
				b := rng.Intn(9) + 2
				if kind == 1 {
					text = fmt.Sprintf("%d - %d 等于多少？", a+5, b)
					opts, ansIdx = makeIntOpts(a + 5 - b)
				} else {
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				}
			case "hard":
				if kind == 0 {
					a := rng.Intn(7) + 3
					b := rng.Intn(7) + 3
					c := rng.Intn(9) + 2
					text = fmt.Sprintf("%d × %d + %d 等于多少？", a, b, c)
					opts, ansIdx = makeIntOpts(a*b + c)
				} else if kind == 1 {
					a := rng.Intn(35) + 25
					b := rng.Intn(18) + 12
					text = fmt.Sprintf("妈妈买了 %d 颗水果糖，分给同学 %d 颗，还剩多少颗？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				} else {
					hr := rng.Intn(9) + 2
					opts = []string{fmt.Sprintf("%d 点", hr), fmt.Sprintf("%d 点半", hr), "12 点"}
					text = fmt.Sprintf("钟面上分针指向 12，时针指向 %d，现在是几点？", hr)
					ansIdx = 0
				}
			default: // normal
				if kind == 0 {
					a := rng.Intn(8) + 2
					b := rng.Intn(8) + 2
					text = fmt.Sprintf("%d × %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a * b)
				} else if kind == 1 {
					a := rng.Intn(30) + 20
					b := rng.Intn(18) + 8
					text = fmt.Sprintf("%d + %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a + b)
				} else {
					a := rng.Intn(25) + 35
					b := rng.Intn(15) + 9
					text = fmt.Sprintf("%d - %d 等于多少？", a, b)
					opts, ansIdx = makeIntOpts(a - b)
				}
			}
		case "9-10":
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
					hrs := rng.Intn(3) + 2
					mins := rng.Intn(35) + 10
					text = fmt.Sprintf("%d 小时 %d 分钟一共是多少分钟？", hrs, mins)
					opts, ansIdx = makeIntOpts(hrs*60 + mins)
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
					text = fmt.Sprintf("%d 米等于多少厘米？", m)
					opts, ansIdx = makeIntOpts(m * 100)
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
			Category:   "数学速算",
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
	case "3-6":
		return 3, 6
	case "7-8":
		return 7, 8
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

// onlineCuratedBank 按「年龄段（3-6 / 7-8 / 9-10 / 11-14）× 难度（easy / normal / hard）」精选的在线中文题库。
var onlineCuratedBank = []QuizQuestion{
	// ==================== 3-6 岁 · 幼儿启蒙 ====================
	{ID: "k_e_1", Category: "生活常识", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "红绿灯亮起红灯时，我们应该怎么做？", Options: []string{"停下来等一等", "快速跑过去", "闭上眼睛走"}, AnswerIdx: 0},
	{ID: "k_e_2", Category: "科学百科", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "小猫喜欢吃哪种食物？", Options: []string{"小鱼", "青草", "石头"}, AnswerIdx: 0},
	{ID: "k_e_3", Category: "生活常识", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "每天早晚都要认真做的事情是什么？", Options: []string{"刷牙洗脸", "吃很多糖果", "熬夜不睡觉"}, AnswerIdx: 0},
	{ID: "k_e_4", Category: "科学百科", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "香蕉成熟后是什么颜色的？", Options: []string{"黄色", "蓝色", "黑色"}, AnswerIdx: 0},
	{ID: "k_e_5", Category: "英语启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "英文单词 “Apple” 是什么水果？", Options: []string{"苹果", "西瓜", "葡萄"}, AnswerIdx: 0},
	{ID: "k_e_6", Category: "英语启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "easy", Question: "见到小朋友想说“你好”，英文怎么说？", Options: []string{"Hello", "Bye-bye", "Thank you"}, AnswerIdx: 0},
	{ID: "k_n_1", Category: "语文启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "“白毛浮绿水，红掌拨清波”写的是哪种动物？", Options: []string{"大白鹅", "小花猫", "长颈鹿"}, AnswerIdx: 0},
	{ID: "k_n_2", Category: "科学百科", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "下面哪一种动物会飞？", Options: []string{"小燕子", "小乌龟", "小金鱼"}, AnswerIdx: 0},
	{ID: "k_n_3", Category: "生活常识", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "别人帮助了我们，应该礼貌地说什么？", Options: []string{"谢谢", "不用管", "走开"}, AnswerIdx: 0},
	{ID: "k_n_4", Category: "英语启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "“Dog” 在中文里是指哪种小动物？", Options: []string{"小狗", "小猫", "小鸟"}, AnswerIdx: 0},
	{ID: "k_n_5", Category: "趣味思维", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "什么动物的脖子长长的，喜欢吃高树上的叶子？", Options: []string{"长颈鹿", "小刺猬", "小仓鼠"}, AnswerIdx: 0},
	{ID: "k_n_6", Category: "科学百科", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "normal", Question: "一年中下雪最多的季节通常是？", Options: []string{"冬天", "夏天", "春天"}, AnswerIdx: 0},
	{ID: "k_h_1", Category: "趣味思维", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "hard", Question: "哥哥今年 6 岁，弟弟今年 4 岁，谁大一些？", Options: []string{"哥哥", "弟弟", "一样大"}, AnswerIdx: 0},
	{ID: "k_h_2", Category: "语文启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "hard", Question: "“春眠不觉晓”的下一句是什么？", Options: []string{"处处闻啼鸟", "床前明月光", "白日依山尽"}, AnswerIdx: 0},
	{ID: "k_h_3", Category: "科学百科", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "hard", Question: "蝌蚪长大以后会变成什么？", Options: []string{"青蛙", "蝴蝶", "小鱼"}, AnswerIdx: 0},
	{ID: "k_h_4", Category: "英语启蒙", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "hard", Question: "英文 “Red, Yellow, Blue” 都是指什么？", Options: []string{"颜色", "数字", "水果"}, AnswerIdx: 0},
	{ID: "k_h_5", Category: "趣味思维", AgeGroup: "3-6", MinAge: 3, MaxAge: 6, Difficulty: "hard", Question: "三角形一共有几条边？", Options: []string{"3 条", "4 条", "5 条"}, AnswerIdx: 0},

	// ==================== 7-8 岁 · 小学低年级（1-2年级） ====================
	{ID: "g1_e_1", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "“床前明月光”的下一句是什么？", Options: []string{"疑是地上霜", "低头思故乡", "春眠不觉晓"}, AnswerIdx: 0},
	{ID: "g1_e_2", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "“大”字加一笔可以变成哪个字？", Options: []string{"天", "小", "人"}, AnswerIdx: 0},
	{ID: "g1_e_3", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "下面哪个词语是反义词？", Options: []string{"高 — 矮", "大 — 巨大", "红 — 火红"}, AnswerIdx: 0},
	{ID: "g1_e_4", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "“一（ ）苹果”括号里最恰当的量词是？", Options: []string{"个", "条", "辆"}, AnswerIdx: 0},
	{ID: "g1_e_5", Category: "科学百科", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "太阳每天从哪个方向升起？", Options: []string{"东边", "西边", "北边"}, AnswerIdx: 0},
	{ID: "g1_e_6", Category: "英语", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "easy", Question: "英文 “Good morning” 是什么意思？", Options: []string{"早上好", "晚安", "再见"}, AnswerIdx: 0},
	{ID: "g1_n_1", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "《静夜思》这首古诗的作者是谁？", Options: []string{"李白", "杜甫", "白居易"}, AnswerIdx: 0},
	{ID: "g1_n_2", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "“河、海、湖、江”这些汉字大多带有哪个偏旁，跟什么有关？", Options: []string{"三点水，与水有关", "木字旁，与树有关", "草字头，与草有关"}, AnswerIdx: 0},
	{ID: "g1_n_3", Category: "科学百科", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "下列哪种动物用鳃在水中呼吸？", Options: []string{"金鱼", "小鸭", "家鸽"}, AnswerIdx: 0},
	{ID: "g1_n_4", Category: "科学百科", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "一年有四个季节，春天的下一个季节是？", Options: []string{"夏天", "秋天", "冬天"}, AnswerIdx: 0},
	{ID: "g1_n_5", Category: "英语", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "“Book” 是哪一种学习用品？", Options: []string{"书本", "铅笔", "橡皮"}, AnswerIdx: 0},
	{ID: "g1_n_6", Category: "趣味思维", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "什么东西越洗越脏？", Options: []string{"水", "肥皂", "毛巾"}, AnswerIdx: 0},
	{ID: "g1_n_7", Category: "趣味思维", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "normal", Question: "麻屋子，红帐子，里面住着白胖子（打一食物）", Options: []string{"花生", "核桃", "栗子"}, AnswerIdx: 0},
	{ID: "g1_h_1", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "“谁知盘中餐”的下一句是什么？", Options: []string{"粒粒皆辛苦", "汗滴禾下土", "处处闻啼鸟"}, AnswerIdx: 0},
	{ID: "g1_h_2", Category: "语文", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "把“惊”字左边的竖心旁换成“鱼”字旁，组成的新字念什么？", Options: []string{"鲸（鲸鱼）", "凉（凉快）", "景（风景）"}, AnswerIdx: 0},
	{ID: "g1_h_3", Category: "科学百科", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "指南针的红色指针通常指着哪个方向？", Options: []string{"北方或南方（南北极）", "东方", "西方"}, AnswerIdx: 0},
	{ID: "g1_h_4", Category: "英语", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "“How old are you?” 是用来询问对方的什么？", Options: []string{"年龄", "名字", "天气"}, AnswerIdx: 0},
	{ID: "g1_h_5", Category: "趣味思维", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "一个星期有 7 天，两个星期零 3 天一共有几天？", Options: []string{"17 天", "14 天", "10 天"}, AnswerIdx: 0},
	{ID: "g1_h_6", Category: "趣味思维", AgeGroup: "7-8", MinAge: 7, MaxAge: 8, Difficulty: "hard", Question: "小朋友排成一队做操，小明前面有 4 人，后面有 5 人，这一队一共几人？", Options: []string{"10 人", "9 人", "11 人"}, AnswerIdx: 0},

	// ==================== 9-10 岁 · 小学中年级（3-4年级） ====================
	{ID: "g3_e_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "“飞流直下三千尺”描写的是哪座名山？", Options: []string{"庐山", "泰山", "黄山"}, AnswerIdx: 0},
	{ID: "g3_e_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "下面哪个成语表示读书非常勤奋？", Options: []string{"悬梁刺股", "守株待兔", "掩耳盗铃"}, AnswerIdx: 0},
	{ID: "g3_e_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "水在标准大气压下，多少摄氏度会开始结冰？", Options: []string{"0℃", "50℃", "100℃"}, AnswerIdx: 0},
	{ID: "g3_e_4", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "easy", Question: "英文单词 “Wednesday” 指的是星期几？", Options: []string{"星期三", "星期二", "星期四"}, AnswerIdx: 0},
	{ID: "g3_n_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“停车坐爱枫林晚”中的“坐”是什么意思？", Options: []string{"因为", "坐下", "座位"}, AnswerIdx: 0},
	{ID: "g3_n_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "“但使龙城飞将在，不教胡马度阴山”的作者是唐代诗人谁？", Options: []string{"王昌龄", "王维", "孟浩然"}, AnswerIdx: 0},
	{ID: "g3_n_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "地球自转一周大约需要多长时间？", Options: []string{"24 小时（一天）", "30 天（一月）", "365 天（一年）"}, AnswerIdx: 0},
	{ID: "g3_n_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "植物进行光合作用制造养分时，吸收气体并释放出哪种气体？", Options: []string{"吸收二氧化碳，释放氧气", "吸收氧气，释放二氧化碳", "吸收氮气，释放氢气"}, AnswerIdx: 0},
	{ID: "g3_n_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "句子 “Where is the library?” 的意思是？", Options: []string{"图书馆在哪里？", "今天星期几？", "你喜欢看书吗？"}, AnswerIdx: 0},
	{ID: "g3_n_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "normal", Question: "中国古代“四大发明”不包括下面哪一项？", Options: []string{"地动仪", "造纸术", "印刷术"}, AnswerIdx: 0},
	{ID: "g3_h_1", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "“横看成岭侧成峰，远近高低各不同”蕴含了什么哲理？", Options: []string{"从不同角度观察事物会有不同认识", "庐山特别难爬", "冬天景色最美"}, AnswerIdx: 0},
	{ID: "g3_h_2", Category: "语文", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "我国第一部纪传体通史《史记》的作者是谁？", Options: []string{"司马迁", "司马光", "孔子"}, AnswerIdx: 0},
	{ID: "g3_h_3", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "为什么冬天呼出的气在空中能看到“白雾”？", Options: []string{"水蒸气遇冷液化成小水滴", "空气结成了冰块", "水变成了氧气"}, AnswerIdx: 0},
	{ID: "g3_h_4", Category: "科学百科", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "声音在哪种介质中传播速度最快？", Options: []string{"钢铁等固体", "水等液体", "空气等气体"}, AnswerIdx: 0},
	{ID: "g3_h_5", Category: "英语", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "选出与 “elephant” 同属动物类的单词：", Options: []string{"kangaroo", "umbrella", "hospital"}, AnswerIdx: 0},
	{ID: "g3_h_6", Category: "趣味思维", AgeGroup: "9-10", MinAge: 9, MaxAge: 10, Difficulty: "hard", Question: "一根木头锯成 4 段需要 6 分钟，照这样计算锯成 6 段需要几分钟？", Options: []string{"10 分钟", "9 分钟", "12 分钟"}, AnswerIdx: 0},

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
