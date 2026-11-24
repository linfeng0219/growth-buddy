package com.growthbuddy.child

import android.content.res.ColorStateList
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.LinearLayout
import android.widget.TextView
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityAdventureBinding
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.Executors

data class QuizItem(
    val category: String,
    val question: String,
    val options: List<String>,
    val answerIdx: Int,
    val difficulty: String = "normal",
    val id: String = ""
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
        const val EXTRA_ADVENTURE_XP_TODAY = "adventure_xp_today"
        const val EXTRA_ADVENTURE_XP_LIMIT = "adventure_xp_limit"
    }

    private lateinit var binding: ActivityAdventureBinding
    private lateinit var sound: NativeSoundEngine
    private val io = Executors.newSingleThreadExecutor()

    private val questions = mutableListOf<QuizItem>()
    private val seenQuestionIds = LinkedHashSet<String>()
    private val seenQuestionTexts = LinkedHashSet<String>()
    private var qIndex = 0
    private var monsterHp = 3
    private val maxHp = 3
    private var petHp = 3
    private val maxPetHp = 3
    private var busy = false
    private var childAge: Int = 7
    private var currentDiff: String = "normal"
    private var roundsCleared: Int = 0
    private var wrongCountInRound: Int = 0
    private var fastCritCountInRound: Int = 0
    private var adventureXpToday: Int = 0
    private var adventureXpLimit: Int = 10
    private var serverAgeLabel: String = ""

    private val mainHandler = Handler(Looper.getMainLooper())
    private var questionShownAtMs: Long = 0L
    private var pausedAtMs: Long = 0L
    private var isResumed: Boolean = false

    private val chargeTicker = object : Runnable {
        override fun run() {
            if (!isResumed) return
            updateSlimeChargeBar()
            mainHandler.postDelayed(this, 100L)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityAdventureBinding.inflate(layoutInflater)
        setContentView(binding.root)

        sound = NativeSoundEngine(this)
        val petStage = intent.getIntExtra(EXTRA_PET_STAGE, 0).coerceIn(0, 5)
        val petAsleep = intent.getBooleanExtra(EXTRA_PET_ASLEEP, false)
        childAge = intent.getIntExtra(EXTRA_CHILD_AGE, 7).coerceIn(3, 14)
        currentDiff = normalizeDiff(intent.getStringExtra(EXTRA_QUIZ_DIFFICULTY))
        adventureXpToday = intent.getIntExtra(EXTRA_ADVENTURE_XP_TODAY, 0).coerceAtLeast(0)
        adventureXpLimit = intent.getIntExtra(EXTRA_ADVENTURE_XP_LIMIT, 10).coerceAtLeast(1)
        serverAgeLabel = "${childAge}岁"
        @Suppress("DEPRECATION")
        val speciesItem = intent.getSerializableExtra(EXTRA_SPECIES_ITEM) as? SpeciesItem
        binding.advBattleStage.setPet(speciesItem, petStage, petAsleep)
        binding.advBattleStage.setMonsterHp(monsterHp, maxHp)
        binding.advBattleStage.setPetHp(petHp, maxPetHp)

        binding.btnAdvExit.setOnClickListener { finish() }
        binding.btnOpt0.setOnClickListener { submitAnswer(0) }
        binding.btnOpt1.setOnClickListener { submitAnswer(1) }
        binding.btnOpt2.setOnClickListener { submitAnswer(2) }
        binding.btnAdvNextRound.setOnClickListener { resetRound() }

        binding.btnDiffEasy.setOnClickListener { switchDifficulty("easy") }
        binding.btnDiffNormal.setOnClickListener { switchDifficulty("normal") }
        binding.btnDiffHard.setOnClickListener { switchDifficulty("hard") }

        updateDifficultyChips()
        updateHpUi()
        applyResponsiveLayout()
        loadFallbackQuestions()
        showCurrentQuestion()
        fetchOnlineQuestions()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val dp = { v: Int -> ResponsiveUi.dp(this, v) }
        ResponsiveUi.applyCenteredMaxWidth(this, binding.advTopBar, 1140, horizontalReserveDp = 32)

        val leftLp = binding.advLeftPane.layoutParams as LinearLayout.LayoutParams
        val rightLp = binding.advRightPane.layoutParams as LinearLayout.LayoutParams
        val stageLp = binding.advBattleStage.layoutParams

        if (m.isDesktopSplit) {
            binding.advBodySplit.orientation = LinearLayout.HORIZONTAL
            binding.advBodySplit.gravity = Gravity.TOP
            ResponsiveUi.applyCenteredMaxWidth(this, binding.advBodySplit, 1140, horizontalReserveDp = 32)
            leftLp.width = 0
            leftLp.weight = 0.95f
            rightLp.width = 0
            rightLp.weight = 1.05f
            rightLp.marginStart = dp(18)
            rightLp.topMargin = 0
            stageLp.height = dp(248)
            binding.tvQuizQuestion.setTextSize(TypedValue.COMPLEX_UNIT_SP, 19f)
        } else {
            binding.advBodySplit.orientation = LinearLayout.VERTICAL
            ResponsiveUi.applyCenteredMaxWidth(
                this,
                binding.advBodySplit,
                if (m.isTablet) 680 else 1200,
                horizontalReserveDp = 32
            )
            leftLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            leftLp.weight = 0f
            rightLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            rightLp.weight = 0f
            rightLp.marginStart = 0
            rightLp.topMargin = dp(10)
            stageLp.height = dp(if (m.isTablet) 196 else if (m.isLandscape) 118 else 148)
            binding.tvQuizQuestion.setTextSize(TypedValue.COMPLEX_UNIT_SP, if (m.isTablet) 18f else 17f)
        }
        binding.advLeftPane.layoutParams = leftLp
        binding.advRightPane.layoutParams = rightLp
        binding.advBattleStage.layoutParams = stageLp
    }

    override fun onResume() {
        super.onResume()
        isResumed = true
        if (pausedAtMs > 0L && questionShownAtMs > 0L) {
            questionShownAtMs += (SystemClock.uptimeMillis() - pausedAtMs)
            pausedAtMs = 0L
        }
        mainHandler.removeCallbacks(chargeTicker)
        mainHandler.post(chargeTicker)
    }

    override fun onPause() {
        isResumed = false
        pausedAtMs = SystemClock.uptimeMillis()
        mainHandler.removeCallbacks(chargeTicker)
        super.onPause()
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
        resetChargeClock()
        binding.tvAdvStatus.text = "正在加载 ${childAge}岁 · ${diffLabel(currentDiff)} 难度在线题库..."
        fetchOnlineQuestions()
    }

    private fun updateDifficultyChips() {
        val baseLabel = serverAgeLabel.ifBlank { "${childAge}岁 · 在线题库" }
        binding.tvQuizLevelInfo.text = "🌐 $baseLabel · 今日战利品 $adventureXpToday/$adventureXpLimit"
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

    private fun fetchOnlineQuestions(appendMode: Boolean = false) {
        val baseUrl = ServerPrefs.getServerUrl(this).trimEnd('/')
        val token = ServerPrefs.getChildToken(this)
        if (baseUrl.isBlank() || token.isBlank()) return

        val reqAge = childAge
        val reqDiff = currentDiff
        val excludeParam = synchronized(seenQuestionIds) {
            seenQuestionIds.toList().takeLast(160).joinToString(",")
        }
        io.execute {
            try {
                val encodedExclude = URLEncoder.encode(excludeParam, "UTF-8")
                val url = "$baseUrl/api/child/questions?age=$reqAge&difficulty=$reqDiff&limit=40&exclude=$encodedExclude"
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
                    val fetchedAgeLabel = root.optString("age_label", "${reqAge}岁")
                    if (arr != null && arr.length() > 0) {
                        val loaded = mutableListOf<QuizItem>()
                        for (i in 0 until arr.length()) {
                            val o = arr.getJSONObject(i)
                            val optsArr = o.optJSONArray("options") ?: continue
                            val opts = (0 until optsArr.length()).map { optsArr.getString(it) }
                            val qText = o.optString("question", "")
                            if (opts.size >= 2 && qText.isNotBlank()) {
                                loaded.add(
                                    QuizItem(
                                        category = o.optString("category", "综合"),
                                        question = qText,
                                        options = opts,
                                        answerIdx = o.optInt("answer_idx", 0),
                                        difficulty = o.optString("difficulty", reqDiff),
                                        id = o.optString("id", "")
                                    )
                                )
                            }
                        }
                        if (loaded.isNotEmpty()) {
                            loaded.shuffle()
                            runOnUiThread {
                                if (currentDiff != reqDiff) return@runOnUiThread
                                this@AdventureActivity.serverAgeLabel = fetchedAgeLabel.ifBlank { "${reqAge}岁" }
                                updateDifficultyChips()
                                val existingTexts = questions.map { it.question }.toSet()
                                val unseen = loaded.filter {
                                    !seenQuestionTexts.contains(it.question) && !existingTexts.contains(it.question)
                                }
                                if (appendMode) {
                                    questions.addAll(if (unseen.isNotEmpty()) unseen else loaded)
                                } else {
                                    questions.clear()
                                    questions.addAll(if (unseen.isNotEmpty()) unseen else loaded)
                                    qIndex = 0
                                    if (monsterHp > 0) {
                                        showCurrentQuestion()
                                        val diffHint = if (reqDiff == "hard") "进阶挑战（含英语与数学拓展）" else "${diffLabel(reqDiff)}难度"
                                        binding.tvAdvStatus.text = "大题库已就绪（${diffHint}），答对充能击败怪兽！"
                                    }
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

    private fun fastCritThresholdMs(): Long = if (childAge <= 5) 10_000L else 8_000L

    private fun slimeChargeIntervalMs(): Long = when (currentDiff) {
        "easy" -> 0L // 简单模式不设超时压迫，仅在答错时史莱姆反击
        "hard" -> if (childAge <= 5) 28_000L else 22_000L
        else -> if (childAge <= 5) 35_000L else 28_000L
    }

    private fun resetChargeClock() {
        questionShownAtMs = SystemClock.uptimeMillis()
        binding.pbSlimeCharge.progress = 0
    }

    private fun updateHpUi() {
        val hearts = "💖".repeat(petHp) + "🤍".repeat((maxPetHp - petHp).coerceAtLeast(0))
        binding.tvPetHp.text = "$hearts 小伙伴元气 $petHp/$maxPetHp"
        binding.pbPetHp.progress = petHp
        binding.tvAdvHp.text = "史莱姆 $monsterHp/$maxHp"
        binding.tvSlimeHpBarLabel.text = "🫧 拖延史莱姆 $monsterHp/$maxHp"
        binding.pbMonsterHp.progress = monsterHp
    }

    private fun updateSlimeChargeBar() {
        if (busy || monsterHp <= 0 || petHp <= 0 || questions.isEmpty()) return
        val interval = slimeChargeIntervalMs()
        val fastSec = (fastCritThresholdMs() / 1000L).toInt()
        if (interval <= 0L) {
            binding.pbSlimeCharge.progress = 0
            binding.tvSlimeCharge.text = "🌱 简单模式不限时 · ${fastSec}秒内答对可触发 ⚡ 闪电暴击"
            return
        }
        val now = SystemClock.uptimeMillis()
        if (questionShownAtMs <= 0L) questionShownAtMs = now
        val elapsed = (now - questionShownAtMs).coerceAtLeast(0L)
        val pct = ((elapsed * 100L) / interval).toInt().coerceIn(0, 100)
        binding.pbSlimeCharge.progress = pct
        val remainSec = (((interval - elapsed) + 999L) / 1000L).coerceAtLeast(0L)
        if (elapsed <= fastCritThresholdMs()) {
            val fastRemain = (((fastCritThresholdMs() - elapsed) + 999L) / 1000L).coerceAtLeast(1L)
            binding.tvSlimeCharge.text = "⚡ ${fastRemain}s 内答对触发闪电暴击 · 史莱姆蓄力 ${remainSec}s"
        } else {
            binding.tvSlimeCharge.text = "🫧 史莱姆泡泡蓄力中（${remainSec}s 后吐泡泡攻击）"
        }
        if (elapsed >= interval) {
            triggerSlimeCounterAttack(attackLabel = "懒洋洋泡泡!", advanceQuestion = false, reasonText = "🫧 史莱姆泡泡蓄满啦！回答加快节奏就能打断它哦~")
        }
    }

    private fun showCurrentQuestion() {
        if (questions.isEmpty()) return
        val q = questions[qIndex % questions.size]
        if (q.id.isNotBlank()) {
            synchronized(seenQuestionIds) {
                seenQuestionIds.add(q.id)
            }
        }
        seenQuestionTexts.add(q.question)
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
        resetChargeClock()
    }

    private fun submitAnswer(choiceIdx: Int) {
        if (busy || monsterHp <= 0 || petHp <= 0 || questions.isEmpty()) return
        val q = questions[qIndex % questions.size]
        val answerCostMs = (SystemClock.uptimeMillis() - questionShownAtMs).coerceAtLeast(0L)
        if (choiceIdx == q.answerIdx) {
            busy = true
            val isFatal = (monsterHp - 1) <= 0
            val isFastCrit = answerCostMs <= fastCritThresholdMs()
            if (isFastCrit) fastCritCountInRound += 1
            binding.pbSlimeCharge.progress = 0
            binding.tvAdvStatus.text = if (isFastCrit) {
                "⚡ 反应神速！小伙伴发动「闪电暴击」！"
            } else {
                "⚡ 答对了！小伙伴蓄力发动元气暴击！"
            }
            binding.advBattleStage.triggerAttack(
                isFinishingBlow = isFatal,
                isLightningCrit = isFastCrit,
                onHit = {
                    sound.playFeed()
                    val previewHp = (monsterHp - 1).coerceAtLeast(0)
                    binding.advBattleStage.setMonsterHp(previewHp, maxHp)
                },
                onWin = {
                    monsterHp = (monsterHp - 1).coerceAtLeast(0)
                    binding.advBattleStage.setMonsterHp(monsterHp, maxHp)
                    updateHpUi()
                    busy = false
                    if (monsterHp == 0) {
                        roundsCleared += 1
                        sound.playVictoryFanfare()
                        sound.speak("太棒了，拖延怪被击败啦！")
                        binding.tvAdvStatus.text = "🎉 太棒了！拖延史莱姆被彻底击败啦！"
                        binding.btnOpt0.visibility = View.GONE
                        binding.btnOpt1.visibility = View.GONE
                        binding.btnOpt2.visibility = View.GONE
                        binding.btnAdvNextRound.visibility = View.VISIBLE
                        showVictoryDialog()
                    } else {
                        sound.playPat()
                        advanceToNextUnseenQuestion()
                        binding.tvAdvStatus.text = if (isFastCrit) {
                            "⚡ 闪电暴击命中！再答对 $monsterHp 题就能击败史莱姆！"
                        } else {
                            "命中！再答对 $monsterHp 题就能击败史莱姆！"
                        }
                    }
                }
            )
        } else {
            wrongCountInRound += 1
            triggerSlimeCounterAttack(
                attackLabel = "反击泡泡!",
                advanceQuestion = false,
                reasonText = "🫧 答错啦，史莱姆吐出了反击泡泡！别灰心，再仔细想想~"
            )
        }
    }

    private fun triggerSlimeCounterAttack(attackLabel: String, advanceQuestion: Boolean, reasonText: String) {
        if (busy || monsterHp <= 0 || petHp <= 0) return
        busy = true
        binding.pbSlimeCharge.progress = 0
        val isKnockout = (petHp - 1) <= 0
        binding.tvAdvStatus.text = "⚠️ 史莱姆蓄力吐出泡泡反击过来啦！"
        binding.advBattleStage.triggerEnemyAttack(
            isKnockout = isKnockout,
            attackLabel = attackLabel,
            onHit = {
                sound.playError()
                val nextPetHp = (petHp - 1).coerceAtLeast(0)
                binding.advBattleStage.setPetHp(nextPetHp, maxPetHp)
            },
            onDone = {
                petHp = (petHp - 1).coerceAtLeast(0)
                binding.advBattleStage.setPetHp(petHp, maxPetHp)
                updateHpUi()
                busy = false
                if (petHp <= 0) {
                    binding.tvAdvStatus.text = "🫧 小伙伴被懒洋洋泡泡包住啦，休息一下满血再战！"
                    showDefeatDialog()
                } else {
                    if (advanceQuestion) {
                        advanceToNextUnseenQuestion()
                    } else {
                        resetChargeClock()
                    }
                    binding.tvAdvStatus.text = reasonText
                }
            }
        )
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
        val accuracyHint = when {
            wrongCountInRound == 0 && fastCritCountInRound >= 2 -> "全对 + ${fastCritCountInRound}次闪电暴击，极速反应超神！"
            wrongCountInRound == 0 -> "全对通关，稳扎稳打真厉害！"
            else -> "坚持思考，成功逆袭击败史莱姆！"
        }
        val detailTv = TextView(this).apply {
            text = "第 $roundsCleared 关拖延史莱姆已被彻底打跑啦！\n$accuracyHint\n（适用题库：${childAge}岁 · ${diffLabel(currentDiff)}难度）"
            textSize = 14.5f
            setTextColor(Color.parseColor("#2F6B4F"))
            gravity = Gravity.CENTER
            setLineSpacing(6f, 1.15f)
            setPadding(0, (6 * dp).toInt(), 0, (4 * dp).toInt())
        }
        val rewardTv = TextView(this).apply {
            text = "⏳ 正在结算本场战斗成长值奖励..."
            textSize = 14f
            setTypeface(null, Typeface.BOLD)
            setTextColor(Color.parseColor("#B8621B"))
            gravity = Gravity.CENTER
            setPadding((10 * dp).toInt(), (8 * dp).toInt(), (10 * dp).toInt(), (8 * dp).toInt())
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#FFF4D6"))
                cornerRadius = 12 * dp
            }
        }
        val rewardLp = LinearLayout.LayoutParams(
            LinearLayout.LayoutParams.MATCH_PARENT,
            LinearLayout.LayoutParams.WRAP_CONTENT
        ).apply {
            topMargin = (8 * dp).toInt()
        }
        rewardTv.layoutParams = rewardLp

        card.addView(iconTv)
        card.addView(titleTv)
        card.addView(detailTv)
        card.addView(rewardTv)

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

        claimAdventureWinReward(rewardTv)
    }

    /**
     * 向后端结算本场冒险战斗胜利奖励（赢一场 +1 成长值，每日上限 10 点）。
     */
    private fun claimAdventureWinReward(rewardTv: TextView) {
        val baseUrl = ServerPrefs.getServerUrl(this).trimEnd('/')
        val token = ServerPrefs.getChildToken(this)
        if (baseUrl.isBlank() || token.isBlank()) {
            rewardTv.text = "⚠️ 离线练习模式，暂未结算成长值"
            return
        }
        val todayStr = SimpleDateFormat("yyyy-MM-dd", Locale.US).format(Date())
        io.execute {
            try {
                val url = "$baseUrl/api/child/adventure-win"
                val conn = (URL(url).openConnection() as HttpURLConnection).apply {
                    requestMethod = "POST"
                    doOutput = true
                    setRequestProperty("Authorization", "Bearer $token")
                    setRequestProperty("Content-Type", "application/json; charset=utf-8")
                    connectTimeout = 5000
                    readTimeout = 5000
                }
                val payload = JSONObject().put("date", todayStr).toString()
                conn.outputStream.use { it.write(payload.toByteArray(Charsets.UTF_8)) }
                val code = conn.responseCode
                val body = if (code in 200..299) {
                    conn.inputStream.bufferedReader().readText()
                } else {
                    conn.errorStream?.bufferedReader()?.readText().orEmpty()
                }
                conn.disconnect()

                if (code in 200..299) {
                    val obj = JSONObject(body)
                    val earned = obj.optInt("earned_xp", 0)
                    val todayXp = obj.optInt("today_adventure_xp", adventureXpToday)
                    val limit = obj.optInt("daily_limit", adventureXpLimit)
                    val remaining = obj.optInt("remaining", (limit - todayXp).coerceAtLeast(0))
                    val reachedLimit = obj.optBoolean("reached_limit", todayXp >= limit)
                    runOnUiThread {
                        if (isFinishing || isDestroyed) return@runOnUiThread
                        adventureXpToday = todayXp
                        adventureXpLimit = limit
                        updateDifficultyChips()
                        rewardTv.text = if (earned > 0) {
                            "🌟 成长值 +$earned！今日战斗已获 $todayXp/$limit 点（还可领 $remaining 点）"
                        } else if (reachedLimit) {
                            "🏅 今日战斗奖励已达上限（$todayXp/$limit 成长值），继续闯关巩固知识吧！"
                        } else {
                            "🎉 挑战成功！今日战斗已获 $todayXp/$limit 成长值"
                        }
                    }
                } else {
                    runOnUiThread {
                        if (!isFinishing && !isDestroyed) {
                            rewardTv.text = "⚠️ 奖励结算失败，请检查网络连接"
                        }
                    }
                }
            } catch (_: Exception) {
                runOnUiThread {
                    if (!isFinishing && !isDestroyed) {
                        rewardTv.text = "⚠️ 网络波动，成长值奖励结算失败"
                    }
                }
            }
        }
    }

    /**
     * 宠物元气心耗尽时的温和鼓励弹窗，支持立即满血复活再战。
     */
    private fun showDefeatDialog() {
        if (isFinishing || isDestroyed) return
        val dp = resources.displayMetrics.density
        val pad = (22 * dp).toInt()
        val card = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(pad, pad, pad, (16 * dp).toInt())
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#FAF5FF"))
                cornerRadius = 20 * dp
                setStroke((2 * dp).toInt(), Color.parseColor("#7B2CBF"))
            }
        }
        card.addView(TextView(this).apply {
            text = "👾"
            textSize = 44f
            gravity = Gravity.CENTER
        })
        card.addView(TextView(this).apply {
            text = "小伙伴被拖延粘液困住啦~"
            textSize = 20f
            setTypeface(null, Typeface.BOLD)
            setTextColor(Color.parseColor("#4C1D95"))
            gravity = Gravity.CENTER
            setPadding(0, (6 * dp).toInt(), 0, (4 * dp).toInt())
        })
        card.addView(TextView(this).apply {
            text = "深呼吸放松 1 秒钟，给小伙伴充满 3 颗元气心，咱们立刻继续打跑史莱姆！"
            textSize = 14f
            setTextColor(Color.parseColor("#2F6B4F"))
            gravity = Gravity.CENTER
            setLineSpacing(6f, 1.15f)
            setPadding(0, (6 * dp).toInt(), 0, (4 * dp).toInt())
        })

        AlertDialog.Builder(this)
            .setView(card)
            .setCancelable(false)
            .setPositiveButton("💖 元气满血再战") { _, _ ->
                sound.playPat()
                resetRound()
            }
            .setNegativeButton("🏠 返回主页") { _, _ ->
                finish()
            }
            .show()
    }

    private fun advanceToNextUnseenQuestion() {
        if (questions.isNotEmpty()) {
            var nextIdx = (qIndex + 1) % questions.size
            var checked = 0
            while (checked < questions.size && seenQuestionTexts.contains(questions[nextIdx].question)) {
                nextIdx = (nextIdx + 1) % questions.size
                checked++
            }
            qIndex = nextIdx
        }
        showCurrentQuestion()
        // 剩余未做题目不足 10 道时，静默续拉新一批去重后的在线+动态大题库追加到队尾
        val remaining = questions.count { !seenQuestionTexts.contains(it.question) }
        if (remaining < 10) {
            fetchOnlineQuestions(appendMode = true)
        }
    }

    private fun resetRound() {
        monsterHp = maxHp
        petHp = maxPetHp
        wrongCountInRound = 0
        fastCritCountInRound = 0
        binding.advBattleStage.setMonsterHp(maxHp, maxHp)
        binding.advBattleStage.setPetHp(maxPetHp, maxPetHp)
        updateHpUi()
        advanceToNextUnseenQuestion()
        binding.tvAdvStatus.text = "新的史莱姆拖延怪跳出来啦！答题充能击败它！"
    }

    override fun onDestroy() {
        mainHandler.removeCallbacks(chargeTicker)
        sound.release()
        io.shutdownNow()
        super.onDestroy()
    }
}
