package com.growthbuddy.child

import android.os.Bundle
import android.view.View
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
    val answerIdx: Int
)

/**
 * 独立「冒险小剧场」页面：
 * 孩子通过回答一年级题库（家长端可自定义）给宠物充能，答对后自动发动攻击击败拖延怪。
 */
class AdventureActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_PET_STAGE = "pet_stage"
        const val EXTRA_PET_ASLEEP = "pet_asleep"
        const val EXTRA_SPECIES_ITEM = "species_item"
    }

    private lateinit var binding: ActivityAdventureBinding
    private lateinit var sound: NativeSoundEngine
    private val io = Executors.newSingleThreadExecutor()

    private val questions = mutableListOf<QuizItem>()
    private var qIndex = 0
    private var monsterHp = 3
    private val maxHp = 3
    private var busy = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityAdventureBinding.inflate(layoutInflater)
        setContentView(binding.root)

        sound = NativeSoundEngine(this)
        val petStage = intent.getIntExtra(EXTRA_PET_STAGE, 0).coerceIn(0, 5)
        val petAsleep = intent.getBooleanExtra(EXTRA_PET_ASLEEP, false)
        @Suppress("DEPRECATION")
        val speciesItem = intent.getSerializableExtra(EXTRA_SPECIES_ITEM) as? SpeciesItem
        binding.advBattleStage.setPet(speciesItem, petStage, petAsleep)

        binding.btnAdvExit.setOnClickListener { finish() }
        binding.btnOpt0.setOnClickListener { submitAnswer(0) }
        binding.btnOpt1.setOnClickListener { submitAnswer(1) }
        binding.btnOpt2.setOnClickListener { submitAnswer(2) }
        binding.btnAdvNextRound.setOnClickListener { resetRound() }

        loadDefaultQuestions()
        showCurrentQuestion()
        fetchServerQuestions()
    }

    private fun loadDefaultQuestions() {
        questions.clear()
        questions.addAll(
            listOf(
                QuizItem("语文", "“床前明月光”的下一句是什么？", listOf("疑是地上霜", "低头思故乡", "春眠不觉晓"), 0),
                QuizItem("数学", "8 + 5 等于多少？", listOf("12", "13", "14"), 1),
                QuizItem("科学常识", "太阳每天从哪个方向升起？", listOf("东边", "西边", "北边"), 0),
                QuizItem("语文", "“大”字加一笔可以变成哪个字？", listOf("天", "小", "人"), 0),
                QuizItem("数学", "16 - 7 等于多少？", listOf("8", "9", "10"), 1),
                QuizItem("科学常识", "一年有四个季节，春天的下一个季节是什么？", listOf("冬天", "夏天", "秋天"), 1),
                QuizItem("趣味问答", "什么东西越洗越脏？", listOf("水", "肥皂", "毛巾"), 0),
                QuizItem("趣味问答", "什么动物天天熬夜，黑眼圈最大？", listOf("大熊猫", "小兔子", "长颈鹿"), 0)
            ).shuffled()
        )
    }

    private fun fetchServerQuestions() {
        val baseUrl = ServerPrefs.getServerUrl(this).trimEnd('/')
        val token = ServerPrefs.getChildToken(this)
        if (baseUrl.isBlank() || token.isBlank()) return

        io.execute {
            try {
                val conn = (URL("$baseUrl/api/child/questions").openConnection() as HttpURLConnection).apply {
                    requestMethod = "GET"
                    setRequestProperty("Authorization", "Bearer $token")
                    connectTimeout = 5000
                    readTimeout = 5000
                }
                if (conn.responseCode == 200) {
                    val body = conn.inputStream.bufferedReader().readText()
                    val arr = JSONObject(body).optJSONArray("questions")
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
                                        answerIdx = o.optInt("answer_idx", 0)
                                    )
                                )
                            }
                        }
                        if (loaded.isNotEmpty()) {
                            loaded.shuffle()
                            runOnUiThread {
                                questions.clear()
                                questions.addAll(loaded)
                                qIndex = 0
                                showCurrentQuestion()
                            }
                        }
                    }
                }
                conn.disconnect()
            } catch (_: Exception) {
                // 离线时保留本地预设题库
            }
        }
    }

    private fun showCurrentQuestion() {
        if (questions.isEmpty()) return
        val q = questions[qIndex % questions.size]
        binding.tvQuizCat.text = q.category
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
                        sound.playWin()
                        sound.speak("太棒了，拖延怪被击败啦！")
                        binding.tvAdvStatus.text = "🎉 太棒了！拖延怪被彻底击败啦！"
                        binding.btnOpt0.visibility = View.GONE
                        binding.btnOpt1.visibility = View.GONE
                        binding.btnOpt2.visibility = View.GONE
                        binding.btnAdvNextRound.visibility = View.VISIBLE
                    } else {
                        sound.playPat()
                        qIndex++
                        showCurrentQuestion()
                        binding.tvAdvStatus.text = "命中！再答对 $monsterHp 题就能击败拖延怪！"
                    }
                }
            )
        } else {
            sound.playError()
            binding.tvAdvStatus.text = "差一点点，再仔细想想哦~"
        }
    }

    private fun resetRound() {
        monsterHp = maxHp
        binding.pbMonsterHp.progress = maxHp
        binding.tvAdvHp.text = "怪兽体力 $maxHp/$maxHp"
        qIndex++
        questions.shuffle()
        showCurrentQuestion()
        binding.tvAdvStatus.text = "新的拖延怪出现啦！答题充能击败它！"
    }

    override fun onDestroy() {
        sound.release()
        super.onDestroy()
    }
}
