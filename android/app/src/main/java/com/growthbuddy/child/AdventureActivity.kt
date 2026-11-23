package com.growthbuddy.child

import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityAdventureBinding
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors

data class QuizItem(
    val category: String,
    val question: String,
    val options: List<String>,
    val answerIdx: Int,
    val difficulty: String = "normal"
)

/**
 * 独立「冒险小剧场」页面：
 * 接入云端分龄分级在线题库（按孩子年龄与难度动态匹配），答对给宠物充能击败拖延怪，并在胜利时弹出祝贺弹窗与音效。
 */
class AdventureActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_PET_STAGE = "pet_stage"
        const val EXTRA_PET_ASLEEP = "pet_asleep"
        const val EXTRA_SPECIES_ITEM = "species_item"
        const val EXTRA_CHILD_AGE = "child_age"
        const val EXTRA_QUIZ_DIFFICULTY = "quiz_difficulty"
    }

    private lateinit var binding: ActivityAdventureBinding
    private lateinit var sound: NativeSoundEngine
    private val io = Executors.newSingleThreadExecutor()

    private val questions = mutableListOf<QuizItem>()
    private var qIndex = 0
    private var monsterHp = 3
    private val maxHp = 3
    private var busy = false
    private var childAge: Int = 7
    private var currentDiff: String = "normal"
    private var roundsCleared: Int = 0
    private var wrongCountInRound: Int = 0

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityAdventureBinding.inflate(layoutInflater)
        setContentView(binding.root)

        sound = NativeSoundEngine(this)
        val petStage = intent.getIntExtra(EXTRA_PET_STAGE, 0).coerceIn(0, 5)
        val petAsleep = intent.getBooleanExtra(EXTRA_PET_ASLEEP, false)
        childAge = intent.getIntExtra(EXTRA_CHILD_AGE, 7).coerceIn(3, 14)
        currentDiff = normalizeDiff(intent.getStringExtra(EXTRA_QUIZ_DIFFICULTY))
        @Suppress("DEPRECATION")
        val speciesItem = intent.getSerializableExtra(EXTRA_SPECIES_ITEM) as? SpeciesItem
        binding.advBattleStage.setPet(speciesItem, petStage, petAsleep)

        binding.btnAdvExit.setOnClickListener { finish() }
        binding.btnOpt0.setOnClickListener { submitAnswer(0) }
        binding.btnOpt1.setOnClickListener { submitAnswer(1) }
        binding.btnOpt2.setOnClickListener { submitAnswer(2) }
        binding.btnAdvNextRound.setOnClickListener { resetRound() }

        binding.btnDiffEasy.setOnClickListener { switchDifficulty("easy") }
        binding.btnDiffNormal.setOnClickListener { switchDifficulty("normal") }
        binding.btnDiffHard.setOnClickListener { switchDifficulty("hard") }

        updateDifficultyChips()
        loadFallbackQuestions()
        showCurrentQuestion()
        fetchOnlineQuestions()
    }

    private fun normalizeDiff(raw: String?): String = when (raw?.trim()?.lowercase()) {
        "easy" -> "easy"
        "hard" -> "hard"
        else -> "normal"
    }

    private fun diffLabel(diff: String): String = when (diff) {
        "easy" -> "简单"
        "hard" -> "挑战"
        else -> "适中"
    }

    private fun switchDifficulty(newDiff: String) {
        if (busy || currentDiff == newDiff) return
        currentDiff = newDiff
        sound.playPat()
        updateDifficultyChips()
        binding.tvAdvStatus.text = "正在加载 ${childAge}岁 · ${diffLabel(currentDiff)} 难度在线题库..."
        fetchOnlineQuestions()
    }

    private fun updateDifficultyChips() {
        binding.tvQuizLevelInfo.text = "🌐 ${childAge}岁 · 在线题库"
        val map = listOf(
            binding.btnDiffEasy to "easy",
            binding.btnDiffNormal to "normal",
            binding.btnDiffHard to "hard"
        )
        map.forEach { (btn, code) ->
            val active = code == currentDiff
            btn.backgroundTintList = ColorStateList.valueOf(
                Color.parseColor(if (active) "#2F6B4F" else "#E8EFEA")
            )
            btn.setTextColor(Color.parseColor(if (active) "#FFFFFF" else "#2F6B4F"))
        }
    }

    private fun loadFallbackQuestions() {
        val pool = when {
            childAge <= 5 -> listOf(
                QuizItem("生活", "过马路看到红灯亮起时，我们要怎么做？", listOf("停下来等绿灯", "赶紧跑过去", "闭上眼睛走"), 0, currentDiff),
                QuizItem("数学", "2 加 3 等于几？", listOf("4", "5", "6"), 1, currentDiff),
                QuizItem("认知", "熟了的大香蕉是什么颜色的？", listOf("黄色", "蓝色", "紫色"), 0, currentDiff),
                QuizItem("儿歌", "“白毛浮绿水，红掌拨清波”写的是哪种动物？", listOf("大白鹅", "小花猫", "长颈鹿"), 0, currentDiff)
            )
            childAge <= 7 -> listOf(
                QuizItem("语文", "“床前明月光”的下一句是什么？", listOf("疑是地上霜", "低头思故乡", "春眠不觉晓"), 0, currentDiff),
                QuizItem("数学", "8 + 5 等于多少？", listOf("12", "13", "14"), 1, currentDiff),
                QuizItem("科学", "太阳每天从哪个方向升起？", listOf("东边", "西边", "北边"), 0, currentDiff),
                QuizItem("趣味", "什么东西越洗越脏？", listOf("水", "肥皂", "毛巾"), 0, currentDiff)
            )
            childAge == 8 -> listOf(
                QuizItem("数学", "6 × 7 等于多少？", listOf("36", "42", "48"), 1, currentDiff),
                QuizItem("语文", "“白日依山尽”的下一句是？", listOf("黄河入海流", "欲穷千里目", "更上一层楼"), 0, currentDiff),
                QuizItem("科学", "1 米等于多少厘米？", listOf("10 厘米", "100 厘米", "1000 厘米"), 1, currentDiff),
                QuizItem("趣味", "把 18 个苹果平均分在 3 个盘子里，每盘几个？", listOf("6 个", "5 个", "9 个"), 0, currentDiff)
            )
            childAge <= 10 -> listOf(
                QuizItem("语文", "“飞流直下三千尺”描写的是哪座名山？", listOf("庐山", "泰山", "黄山"), 0, currentDiff),
                QuizItem("数学", "15 × 4 等于多少？", listOf("45", "60", "75"), 1, currentDiff),
                QuizItem("科学", "地球自转一周大约需要多长时间？", listOf("24 小时", "30 天", "365 天"), 0, currentDiff),
                QuizItem("英语", "英文 “Wednesday” 是星期几？", listOf("星期三", "星期二", "星期四"), 0, currentDiff)
            )
            else -> listOf(
                QuizItem("语文", "“落霞与孤鹜齐飞”出自王勃的哪篇作品？", listOf("《滕王阁序》", "《岳阳楼记》", "《醉翁亭记》"), 0, currentDiff),
                QuizItem("数学", "1 到 100 所有整数相加的和是多少？", listOf("5000", "5050", "4950"), 1, currentDiff),
                QuizItem("科学", "空气中体积占比最高的气体是哪种？", listOf("氮气（约 78%）", "氧气（约 21%）", "二氧化碳"), 0, currentDiff),
                QuizItem("英语", "“Practice makes perfect.” 意思是？", listOf("熟能生巧", "有志者事竟成", "千里之行始于足下"), 0, currentDiff)
            )
        }
        questions.clear()
        questions.addAll(pool.shuffled())
    }

    private fun fetchOnlineQuestions() {
        val baseUrl = ServerPrefs.getServerUrl(this).trimEnd('/')
        val token = ServerPrefs.getChildToken(this)
        if (baseUrl.isBlank() || token.isBlank()) return

        val reqAge = childAge
        val reqDiff = currentDiff
        io.execute {
            try {
                val url = "$baseUrl/api/child/questions?age=$reqAge&difficulty=$reqDiff"
                val conn = (URL(url).openConnection() as HttpURLConnection).apply {
                    requestMethod = "GET"
                    setRequestProperty("Authorization", "Bearer $token")
                    connectTimeout = 5000
                    readTimeout = 5000
                }
                if (conn.responseCode == 200) {
                    val body = conn.inputStream.bufferedReader().readText()
                    val root = JSONObject(body)
                    val arr = root.optJSONArray("questions")
                    val serverAgeLabel = root.optString("age_label", "${reqAge}岁")
                    if (arr != null && arr.length() > 0) {
                        val loaded = mutableListOf<QuizItem>()
                        for (i in 0 until arr.length()) {
                            val o = arr.getJSONObject(i)
                            val optsArr = o.optJSONArray("options") ?: continue
                            val opts = (0 until optsArr.length()).map { optsArr.getString(it) }
                            if (opts.size >= 2) {
                                loaded.add(
                                    QuizItem(
                                        category = o.optString("category", "综合"),
                                        question = o.optString("question", ""),
                                        options = opts,
                                        answerIdx = o.optInt("answer_idx", 0),
                                        difficulty = o.optString("difficulty", reqDiff)
                                    )
                                )
                            }
                        }
                        if (loaded.isNotEmpty()) {
                            loaded.shuffle()
                            runOnUiThread {
                                if (currentDiff != reqDiff) return@runOnUiThread
                                questions.clear()
                                questions.addAll(loaded)
                                qIndex = 0
                                binding.tvQuizLevelInfo.text = "🌐 $serverAgeLabel"
                                if (monsterHp > 0) {
                                    showCurrentQuestion()
                                    val diffHint = if (reqDiff == "hard") "进阶挑战（含英语与数学拓展）" else "${diffLabel(reqDiff)}难度"
                                    binding.tvAdvStatus.text = "在线题库已就绪（${diffHint}），答对充能击败怪兽！"
                                }
                            }
                        }
                    }
                }
                conn.disconnect()
            } catch (_: Exception) {
                // 离线时使用适配当前年龄的本地兜底题库
            }
        }
    }

    private fun showCurrentQuestion() {
        if (questions.isEmpty()) return
        val q = questions[qIndex % questions.size]
        binding.tvQuizCat.text = "${q.category} · ${diffLabel(q.difficulty)}"
        binding.tvQuizQuestion.text = q.question

        val btns = listOf(binding.btnOpt0, binding.btnOpt1, binding.btnOpt2)
        val labels = listOf("A", "B", "C")
        btns.forEachIndexed { idx, btn ->
            if (idx < q.options.size) {
                btn.visibility = View.VISIBLE
                btn.text = "${labels[idx]}. ${q.options[idx]}"
            } else {
                btn.visibility = View.GONE
            }
        }
        binding.btnAdvNextRound.visibility = View.GONE
    }

    private fun submitAnswer(choiceIdx: Int) {
        if (busy || monsterHp <= 0 || questions.isEmpty()) return
        val q = questions[qIndex % questions.size]
        if (choiceIdx == q.answerIdx) {
            busy = true
            binding.tvAdvStatus.text = "⚡ 答对了！小伙伴充能完毕，出猎一击！"
            binding.advBattleStage.triggerAttack(
                onHit = { sound.playFeed() },
                onWin = {
                    monsterHp = (monsterHp - 1).coerceAtLeast(0)
                    binding.pbMonsterHp.progress = monsterHp
                    binding.tvAdvHp.text = "怪兽体力 $monsterHp/$maxHp"
                    busy = false
                    if (monsterHp == 0) {
                        roundsCleared += 1
                        sound.playVictoryFanfare()
                        sound.speak("太棒了，拖延怪被击败啦！")
                        binding.tvAdvStatus.text = "🎉 太棒了！拖延怪被彻底击败啦！"
                        binding.btnOpt0.visibility = View.GONE
                        binding.btnOpt1.visibility = View.GONE
                        binding.btnOpt2.visibility = View.GONE
                        binding.btnAdvNextRound.visibility = View.VISIBLE
                        showVictoryDialog()
                    } else {
                        sound.playPat()
                        qIndex++
                        showCurrentQuestion()
                        binding.tvAdvStatus.text = "命中！再答对 $monsterHp 题就能击败拖延怪！"
                    }
                }
            )
        } else {
            wrongCountInRound += 1
            sound.playError()
            binding.tvAdvStatus.text = "差一点点，再仔细想想哦~"
        }
    }

    /**
     * 战斗胜利后弹出祝贺弹窗并展示本轮战绩。
     */
    private fun showVictoryDialog() {
        if (isFinishing || isDestroyed) return
        val dp = resources.displayMetrics.density
        val pad = (22 * dp).toInt()
        val card = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(pad, pad, pad, (16 * dp).toInt())
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#FFFDF7"))
                cornerRadius = 20 * dp
                setStroke((2 * dp).toInt(), Color.parseColor("#E69A2B"))
            }
        }

        val iconTv = TextView(this).apply {
            text = "🏆"
            textSize = 46f
            gravity = Gravity.CENTER
        }
        val titleTv = TextView(this).apply {
            text = "🎉 战斗大获全胜！"
            textSize = 21f
            setTypeface(null, Typeface.BOLD)
            setTextColor(Color.parseColor("#233128"))
            gravity = Gravity.CENTER
            setPadding(0, (6 * dp).toInt(), 0, (4 * dp).toInt())
        }
        val accuracyHint = if (wrongCountInRound == 0) "全对通关，反应超神！" else "坚持思考，成功逆袭击败怪兽！"
        val detailTv = TextView(this).apply {
            text = "第 $roundsCleared 关拖延怪已被彻底打跑啦！\n$accuracyHint\n（适用题库：${childAge}岁 · ${diffLabel(currentDiff)}难度）"
            textSize = 14.5f
            setTextColor(Color.parseColor("#2F6B4F"))
            gravity = Gravity.CENTER
            setLineSpacing(6f, 1.15f)
            setPadding(0, (6 * dp).toInt(), 0, (4 * dp).toInt())
        }
        card.addView(iconTv)
        card.addView(titleTv)
        card.addView(detailTv)

        AlertDialog.Builder(this)
            .setView(card)
            .setCancelable(false)
            .setPositiveButton("⚔️ 再来一关") { _, _ ->
                sound.playPat()
                resetRound()
            }
            .setNegativeButton("🏠 返回主页") { _, _ ->
                finish()
            }
            .show()
    }

    private fun resetRound() {
        monsterHp = maxHp
        wrongCountInRound = 0
        binding.pbMonsterHp.progress = maxHp
        binding.tvAdvHp.text = "怪兽体力 $maxHp/$maxHp"
        qIndex++
        questions.shuffle()
        showCurrentQuestion()
        binding.tvAdvStatus.text = "新的拖延怪出现啦！答题充能击败它！"
        // 如当前题库快答完，后台再刷一批最新在线题
        if (qIndex >= questions.size - 2) {
            fetchOnlineQuestions()
        }
    }

    override fun onDestroy() {
        sound.release()
        io.shutdownNow()
        super.onDestroy()
    }
}
