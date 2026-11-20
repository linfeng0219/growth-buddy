package com.growthbuddy.child

import android.content.Intent
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.Matrix
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.media.AudioManager
import android.media.ExifInterface
import android.net.Uri
import android.os.Bundle
import android.util.Base64
import android.view.Gravity
import android.view.View
import android.view.inputmethod.EditorInfo
import android.widget.Button
import android.widget.EditText
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.FileProvider
import com.growthbuddy.child.databinding.ActivityMainBinding
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.text.SimpleDateFormat
import java.util.Calendar
import java.util.Date
import java.util.Locale
import kotlin.math.min

/**
 * 纯原生安卓儿童端主界面（彻底移除 WebView，零迟滞交互与原生 SoundPool 音效）：
 * - 登录页：凭家长分配的孩子账号与密码直接登录，无需记住家庭码。
 * - 主界面：60fps 原生 Canvas 糖霜宠物舞台、今日任务打卡（支持一键拍照留证供家长查验）、天天跳绳式 AI 跳绳挑战、伙伴图鉴、拍立得纪念墙。
 */
class MainActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMainBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private lateinit var updater: AppUpdater

    private var state: ChildState? = null
    private var currentTab: Int = 0 // 0=home, 1=dex
    private var patIndex: Int = 0

    // 打卡拍照与头像选取状态
    private var pendingCaptureFile: File? = null
    private var pendingPhotoCallback: ((Bitmap, String) -> Unit)? = null
    private var pendingGalleryAvatarCallback: ((Bitmap, String) -> Unit)? = null

    private val takePictureLauncher = registerForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { success ->
        val file = pendingCaptureFile
        if (!success || file == null || !file.exists() || file.length() == 0L) {
            return@registerForActivityResult
        }
        val processed = decodeAndCompressCapture(file)
        runCatching { file.delete() }
        if (processed != null) {
            sound.playPat()
            pendingPhotoCallback?.invoke(processed.first, processed.second)
        } else {
            Toast.makeText(this, "读取照片失败，请重试", Toast.LENGTH_SHORT).show()
        }
    }

    private val focusLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { res ->
        if (res.resultCode == RESULT_OK) {
            val taskId = res.data?.getStringExtra(FocusActivity.EXTRA_TASK_ID).orEmpty()
            val actualDur = res.data?.getIntExtra(FocusActivity.EXTRA_ACTUAL_DURATION, 0) ?: 0
            val mins = res.data?.getIntExtra(FocusActivity.EXTRA_DURATION_MIN, 15) ?: 15
            if (taskId.isNotEmpty()) {
                val taskItem = state?.tasks?.find { it.id == taskId }
                if (taskItem != null && taskItem.requirePhoto) {
                    showCheckinConfirmDialog(taskItem, null, actualDur.takeIf { it > 0 })
                } else {
                    submitCheckin(taskId, null, actualDur.takeIf { it > 0 }, null)
                }
            } else {
                sound.playWin()
                binding.petStageView.triggerPatAnimation()
                binding.tvBubble.text = "太棒了！刚完成了 $mins 分钟专注，继续保持哦~"
                Toast.makeText(this, "🎉 完成 $mins 分钟专注！", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private val pickAvatarLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri: Uri? ->
        if (uri == null) return@registerForActivityResult
        val processed = decodeUriToSquareAvatar(uri)
        if (processed != null) {
            sound.playPat()
            pendingGalleryAvatarCallback?.invoke(processed.first, processed.second)
        } else {
            Toast.makeText(this, "读取图片失败，请选择其他照片", Toast.LENGTH_SHORT).show()
        }
    }

    private val ropeChallengeLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        val raw = result.data?.getStringExtra(RopeJumpActivity.EXTRA_RESULT_JSON) ?: return@registerForActivityResult
        val obj = runCatching { JSONObject(raw) }.getOrNull() ?: return@registerForActivityResult
        if (obj.optBoolean("cancelled", true)) return@registerForActivityResult

        val taskId = obj.optString("taskId", "")
        val actualCount = obj.optInt("actualCount", 0)
        val actualDuration = obj.optInt("actualDuration", 60)
        val completed = obj.optBoolean("completed", false)

        if (completed && taskId.isNotEmpty()) {
            val taskItem = state?.tasks?.find { it.id == taskId }
            if (taskItem != null) {
                showCheckinConfirmDialog(taskItem, actualCount, actualDuration)
            } else {
                submitCheckin(taskId, actualCount, actualDuration, null)
            }
        } else {
            sound.playError()
            AlertDialog.Builder(this)
                .setTitle("差一点点就达标啦！")
                .setMessage("本次跳了 $actualCount 个，加油调整节奏再挑战一次吧！")
                .setPositiveButton("再试一次") { _, _ ->
                    state?.tasks?.find { it.id == taskId }?.let { launchRopeChallenge(it) }
                }
                .setNegativeButton("先休息一下", null)
                .show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        volumeControlStream = AudioManager.STREAM_MUSIC
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)
        updater = AppUpdater(this)

        setupLoginUi()
        setupAppUi()

        if (ServerPrefs.getChildToken(this).isNotEmpty()) {
            showAppView()
            loadChildState()
        } else {
            showLoginView()
        }

        // 启动时静默检查远程升级（有新版则自动弹窗提示）
        updater.checkForUpdate(silent = true)
    }

    override fun onResume() {
        super.onResume()
        updateVolumeBoostButton()
        updater.resumePendingInstallIfAllowed()
    }

    private fun updateVolumeBoostButton() {
        val low = !sound.isMuted() && sound.isMediaVolumeLow()
        binding.btnBoostVolMain.visibility = if (low) View.VISIBLE else View.GONE
    }

    private fun setupLoginUi() {
        binding.loginPetPreview.setPetState(3, false, null)
        binding.etLoginUser.setText(ServerPrefs.getSavedUsername(this))
        refreshServerFooter()

        binding.btnLoginUpdate.setOnClickListener { updater.checkForUpdate(silent = false) }
        binding.btnLoginServer.setOnClickListener { showServerDialog() }
        binding.btnDoLogin.setOnClickListener { performChildLogin() }
        binding.etLoginPass.setOnEditorActionListener { _, actionId, _ ->
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                performChildLogin()
                true
            } else false
        }
    }

    private fun setupAppUi() {
        binding.btnBoostVolMain.setOnClickListener {
            sound.boostMediaVolumeToRecommended()
            sound.playPat()
            updateVolumeBoostButton()
        }
        binding.ivHeaderAvatar.setOnClickListener { showAvatarPickerDialog() }
        binding.btnMute.setOnClickListener { toggleMute() }
        binding.btnAppUpdate.setOnClickListener { updater.checkForUpdate(silent = false) }
        binding.btnRefresh.setOnClickListener { loadChildState() }
        binding.btnLogout.setOnClickListener { confirmLogout() }

        binding.tabHome.setOnClickListener { switchTab(0) }
        binding.tabDex.setOnClickListener { switchTab(1) }

        binding.petStageView.setOnClickListener {
            sound.playPat()
            binding.petStageView.triggerPatAnimation()
            val line = GrowthMeta.PAT_LINES[patIndex % GrowthMeta.PAT_LINES.size]
            patIndex++
            binding.tvBubble.text = line
            if (!sound.isMuted()) {
                sound.speak(line)
            }
        }
        binding.btnSpeak.setOnClickListener {
            val txt = binding.tvBubble.text.toString()
            if (sound.isMuted()) {
                Toast.makeText(this, "当前已静音，点右上角声音图标即可开启", Toast.LENGTH_SHORT).show()
            } else if (!sound.speak(txt)) {
                sound.playPat()
                Toast.makeText(this, "系统语音引擎未就绪，请检查系统 TTS 设置", Toast.LENGTH_SHORT).show()
            }
        }
        binding.btnRenamePet.setOnClickListener { showRenamePetDialog() }
        binding.btnUndo.setOnClickListener { performUndo() }
        binding.btnMysteryEgg.setOnClickListener { performAdopt(speciesId = null, mystery = true) }
        binding.btnSnap.setOnClickListener { performTakePolaroid() }
        binding.btnFreeFocus.setOnClickListener {
            val st = state
            val lv = st?.let { GrowthMeta.levelForXp(it.child.xp) } ?: 0
            val asleep = st?.let { isAsleep(it.logs) } ?: false
            val activeComp = st?.let { s -> s.catalog.find { it.id == s.child.activeCompanion } }
            val intent = Intent(this, FocusActivity::class.java).apply {
                putExtra(FocusActivity.EXTRA_TASK_TITLE, "自由专注")
                putExtra(FocusActivity.EXTRA_DURATION_MIN, 15)
                putExtra(FocusActivity.EXTRA_PET_STAGE, lv)
                putExtra(FocusActivity.EXTRA_PET_ASLEEP, asleep)
                if (activeComp != null) putExtra(FocusActivity.EXTRA_SPECIES_ITEM, activeComp)
            }
            focusLauncher.launch(intent)
        }
        binding.btnToggleBattle.setOnClickListener {
            val st = state
            val lv = st?.let { GrowthMeta.levelForXp(it.child.xp) } ?: 0
            val asleep = st?.let { isAsleep(it.logs) } ?: false
            val activeComp = st?.let { s -> s.catalog.find { it.id == s.child.activeCompanion } }
            val intent = Intent(this, AdventureActivity::class.java).apply {
                putExtra(AdventureActivity.EXTRA_PET_STAGE, lv)
                putExtra(AdventureActivity.EXTRA_PET_ASLEEP, asleep)
                if (activeComp != null) putExtra(AdventureActivity.EXTRA_SPECIES_ITEM, activeComp)
            }
            startActivity(intent)
        }
        binding.btnRunBattle.setOnClickListener {
            binding.tvBattleMsg.text = ""
            binding.battleStageView.triggerAttack(
                onHit = { sound.playFeed() },
                onWin = {
                    sound.playWin()
                    binding.tvBattleMsg.text = "拖延怪被击退啦！继续保持打卡节奏~"
                }
            )
        }
    }

    private fun refreshServerFooter() {
        binding.tvServerFooter.text = "当前服务器：${ServerPrefs.getServerUrl(this)} · v${BuildConfig.VERSION_NAME}"
    }

    private fun showLoginView(message: String = "") {
        binding.loginContainer.visibility = View.VISIBLE
        binding.appContainer.visibility = View.GONE
        binding.tvLoginMsg.text = message
        refreshServerFooter()
    }

    private fun showAppView() {
        binding.loginContainer.visibility = View.GONE
        binding.appContainer.visibility = View.VISIBLE
        switchTab(currentTab)
    }

    private fun switchTab(idx: Int) {
        currentTab = idx
        binding.homeSection.visibility = if (idx == 0) View.VISIBLE else View.GONE
        binding.dexSection.visibility = if (idx == 1) View.VISIBLE else View.GONE
        binding.tabHome.setTextColor(if (idx == 0) 0xFF2F6B4F.toInt() else 0xFF8A7E6E.toInt())
        binding.tabHome.setTypeface(null, if (idx == 0) Typeface.BOLD else Typeface.NORMAL)
        binding.tabDex.setTextColor(if (idx == 1) 0xFF2F6B4F.toInt() else 0xFF8A7E6E.toInt())
        binding.tabDex.setTypeface(null, if (idx == 1) Typeface.BOLD else Typeface.NORMAL)
        binding.mainScroll.scrollTo(0, 0)
    }

    private fun performChildLogin() {
        val username = binding.etLoginUser.text.toString().trim()
        val password = binding.etLoginPass.text.toString()
        if (username.isEmpty() || password.isEmpty()) {
            binding.tvLoginMsg.text = "请输入孩子账号和密码"
            sound.playError()
            return
        }
        binding.btnDoLogin.isEnabled = false
        binding.btnDoLogin.text = "正在登录..."
        binding.tvLoginMsg.text = ""

        val payload = JSONObject().apply {
            put("username", username)
            put("password", password)
        }
        api.request("POST", "/api/auth/child-login", payload, useAuth = false) { res ->
            binding.btnDoLogin.isEnabled = true
            binding.btnDoLogin.text = "登录我的小伙伴 →"
            if (res.ok && res.json != null) {
                val token = res.json.optString("token", "")
                if (token.isNotEmpty()) {
                    ServerPrefs.setChildToken(this, token)
                    ServerPrefs.setSavedUsername(this, username)
                    binding.etLoginPass.setText("")
                    sound.playWin()
                    showAppView()
                    loadChildState()
                }
            } else {
                sound.playError()
                binding.tvLoginMsg.text = res.error ?: "登录失败"
            }
        }
    }

    private fun confirmLogout() {
        AlertDialog.Builder(this)
            .setTitle("退出当前账号？")
            .setMessage("退出后可以切换其他小朋友的账号登录")
            .setPositiveButton("退出登录") { _, _ ->
                ServerPrefs.clearChildToken(this)
                state = null
                showLoginView("已退出登录，请输入孩子账号和密码")
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun showServerDialog() {
        val input = EditText(this).apply {
            hint = "例如：http://linuu.xyz"
            setText(ServerPrefs.getServerUrl(this@MainActivity))
            setPadding(40, 28, 40, 28)
        }
        AlertDialog.Builder(this)
            .setTitle("设置服务器地址")
            .setMessage("默认已指向 http://linuu.xyz，如更换服务器可在此修改")
            .setView(input)
            .setPositiveButton("保存") { _, _ ->
                ServerPrefs.setServerUrl(this, input.text.toString())
                refreshServerFooter()
                Toast.makeText(this, "服务器地址已保存", Toast.LENGTH_SHORT).show()
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun todayStr(): String = SimpleDateFormat("yyyy-MM-dd", Locale.US).format(Date())

    private fun dayOffsetStr(offsetDays: Int): String {
        val cal = Calendar.getInstance()
        cal.add(Calendar.DAY_OF_YEAR, offsetDays)
        return SimpleDateFormat("yyyy-MM-dd", Locale.US).format(cal.time)
    }

    private fun loadChildState() {
        binding.topLoading.visibility = View.VISIBLE
        api.request("GET", "/api/child/state?date=${todayStr()}") { res ->
            binding.topLoading.visibility = View.GONE
            if (res.status == 401) {
                ServerPrefs.clearChildToken(this)
                state = null
                showLoginView("登录已过期，请重新输入孩子账号和密码")
                return@request
            }
            if (res.ok && res.json != null) {
                val parsed = GrowthMeta.parseChildState(res.json)
                state = parsed
                sound.setMuted(parsed.child.muted)
                renderAll(parsed)
                updateVolumeBoostButton()
            } else {
                Toast.makeText(this, res.error ?: "同步状态失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun computeStreakDays(logs: List<CheckinLog>): Int {
        val days = logs.map { it.checkinDate }.toSet()
        var cnt = 0
        var off = if (days.contains(todayStr())) 0 else -1
        while (days.contains(dayOffsetStr(off))) {
            cnt++
            off--
        }
        return cnt
    }

    private fun isAsleep(logs: List<CheckinLog>): Boolean {
        val lastDate = logs.lastOrNull()?.checkinDate ?: return false
        return runCatching {
            val sdf = SimpleDateFormat("yyyy-MM-dd", Locale.US)
            val d1 = sdf.parse(todayStr()) ?: return false
            val d2 = sdf.parse(lastDate) ?: return false
            (d1.time - d2.time) / 86_400_000L >= 3L
        }.getOrDefault(false)
    }

    private fun renderAll(st: ChildState) {
        val c = st.child
        val lv = GrowthMeta.levelForXp(c.xp)
        val lvSpec = GrowthMeta.LEVELS[lv]
        val asleep = isAsleep(st.logs)
        val activeComp = st.catalog.find { it.id == c.activeCompanion }

        bindAvatarView(binding.ivHeaderAvatar, c.avatarIdx, c.avatarUrl)
        binding.tvKidBanner.text = "${c.name}的小屋"
        binding.tvFamilySub.text = "${st.family.name} · 点击头像可更换 ✎"
        binding.btnMute.text = if (c.muted) "🔇" else "🔊"

        binding.tvStagePill.text = "Lv.${lv + 1} ${lvSpec.name}"
        binding.tvStreakPill.text = "🌟 连续 ${computeStreakDays(st.logs)} 天"
        binding.petStageView.setPetState(lv, asleep, activeComp)
        binding.battleStageView.setPet(activeComp, lv)

        binding.tvBubble.text = if (asleep) {
            "呼呼……已经 3 天没见到你了，喂我一口就醒来。"
        } else if (st.logs.isEmpty()) {
            GrowthMeta.LEVELS[0].talk
        } else {
            lvSpec.talk
        }
        binding.tvPetName.text = c.petName.ifEmpty { "${c.name}的小伙伴" }

        val totalDays = st.logs.map { it.checkinDate }.toSet().size
        val doneCount = st.tasks.count { st.todayDoneIds.contains(it.id) }
        binding.tvStatsRow.text = "陪伴 $totalDays 天 · 今日已完成 $doneCount/${st.tasks.size} · 领养蛋 ${c.coupons} 枚"
        binding.tvTodayBadge.text = "$doneCount/${st.tasks.size}"

        // XP 进度条
        binding.tvXpLeft.text = "成长值 ${c.xp}"
        val nextLv = GrowthMeta.LEVELS.getOrNull(lv + 1)
        if (nextLv != null) {
            val base = lvSpec.xp
            val span = (nextLv.xp - base).coerceAtLeast(1)
            val pct = (((c.xp - base) * 100) / span).coerceIn(0, 100)
            binding.xpProgress.progress = pct
            binding.tvXpRight.text = "距「${nextLv.name}」还差 ${(nextLv.xp - c.xp).coerceAtLeast(0)}"
        } else {
            binding.xpProgress.progress = 100
            binding.tvXpRight.text = "已达最高阶段守护兽！"
        }

        renderWeekStrip(st)
        renderTasks(st)
        renderDex(st)
        renderPolaroids(st)
    }

    private fun renderWeekStrip(st: ChildState) {
        val hitDays = st.logs.map { it.checkinDate }.toSet()
        binding.weekStrip.removeAllViews()
        val labels = arrayOf("日", "一", "二", "三", "四", "五", "六")

        for (i in -6..0) {
            val cal = Calendar.getInstance().apply { add(Calendar.DAY_OF_YEAR, i) }
            val ds = SimpleDateFormat("yyyy-MM-dd", Locale.US).format(cal.time)
            val hit = hitDays.contains(ds)
            val isToday = (i == 0)

            val col = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val dot = TextView(this).apply {
                text = if (hit) "✓" else "·"
                gravity = Gravity.CENTER
                textSize = 13f
                setTextColor(if (hit) 0xFF2F6B4F.toInt() else 0xFFB8AE9D.toInt())
                setTypeface(null, Typeface.BOLD)
                background = GradientDrawable().apply {
                    shape = GradientDrawable.OVAL
                    setColor(if (hit) 0xFFE3F2E8.toInt() else 0xFFF3EDE2.toInt())
                    setStroke(
                        dp(if (isToday) 2 else 1),
                        if (isToday) 0xFFE8913A.toInt() else if (hit) 0xFFB8D4C3.toInt() else 0xFFE5DCCB.toInt()
                    )
                }
                layoutParams = LinearLayout.LayoutParams(dp(28), dp(28))
            }
            val lbl = TextView(this).apply {
                text = if (isToday) "今" else labels[cal.get(Calendar.DAY_OF_WEEK) - 1]
                textSize = 11f
                setTextColor(if (isToday) 0xFFE8913A.toInt() else 0xFF8A7E6E.toInt())
                setPadding(0, dp(3), 0, 0)
            }
            col.addView(dot)
            col.addView(lbl)
            binding.weekStrip.addView(col)
        }
    }

    private fun renderTasks(st: ChildState) {
        binding.tasksContainer.removeAllViews()
        if (st.tasks.isEmpty()) {
            val empty = TextView(this).apply {
                text = "今天还没有分配任务，快请家长在家长端布置吧~"
                setTextColor(0xFF8A7E6E.toInt())
                gravity = Gravity.CENTER
                setPadding(dp(8), dp(18), dp(8), dp(18))
            }
            binding.tasksContainer.addView(empty)
            return
        }

        val todayPhotoByTask = mutableMapOf<String, String>()
        val todayDate = st.today.ifEmpty { todayStr() }
        for (l in st.logs) {
            if (l.checkinDate == todayDate && l.hasPhoto && l.photoUrl.isNotEmpty() && l.taskId.isNotEmpty()) {
                todayPhotoByTask[l.taskId] = l.photoUrl
            }
        }

        for (t in st.tasks) {
            val done = st.todayDoneIds.contains(t.id)
            val isRope = t.taskType == "rope_jump"
            val photoUrl = todayPhotoByTask[t.id]

            val card = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(14), dp(12), dp(14), dp(12))
                background = GradientDrawable().apply {
                    cornerRadius = dp(14).toFloat()
                    when {
                        done -> {
                            setColor(0xFFEAF5EE.toInt())
                            setStroke(dp(1), 0xFFB8D4C3.toInt())
                        }
                        isRope -> {
                            setColor(0xFFFFF7EA.toInt())
                            setStroke(dp(1), 0xFFEDC68C.toInt())
                        }
                        else -> {
                            setColor(Color.WHITE)
                            setStroke(dp(1), 0xFFEAE2D3.toInt())
                        }
                    }
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(9) }
            }

            val leftCol = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val titleTv = TextView(this).apply {
                text = "${if (done) "✅" else t.emoji} ${t.label}"
                textSize = 15f
                setTypeface(null, Typeface.BOLD)
                setTextColor(if (done) 0xFF2F6B4F.toInt() else 0xFF2B2620.toInt())
            }
            leftCol.addView(titleTv)

            val subText = when {
                done && !photoUrl.isNullOrEmpty() -> "📷 今日已完成并上传成果照片 · 点击查看照片"
                done -> "今日已完成！"
                isRope -> "🪢 AI 跳绳：${t.durationSec} 秒跳够 ${t.targetCount} 个 · 点击开启镜头"
                t.timerMode == "countdown" -> "⏱️ 倒计时 ${(t.durationSec / 60).coerceAtLeast(1)} 分钟${if (t.requirePhoto) " · 需拍照" else ""} · 点击开始专注"
                t.timerMode == "countup" -> "⏱️ 正计时专注${if (t.requirePhoto) " · 需拍照" else ""} · 点击开始计时"
                t.requirePhoto -> "📷 需拍照留证 · 点击拍照完成打卡"
                t.scope == "selected" -> "⭐ 专属任务 · 点击直接打卡喂宠"
                else -> "✨ 点击直接完成打卡喂宠"
            }
            val subTv = TextView(this).apply {
                text = subText
                textSize = 12f
                setTextColor(
                    when {
                        done && !photoUrl.isNullOrEmpty() -> 0xFF2F6B4F.toInt()
                        isRope && !done -> 0xFFB86511.toInt()
                        else -> 0xFF8A7E6E.toInt()
                    }
                )
                setPadding(0, dp(2), 0, 0)
            }
            leftCol.addView(subTv)

            val ptsTv = TextView(this).apply {
                text = if (done) (if (!photoUrl.isNullOrEmpty()) "📷 查看" else "已领") else "+${t.pts}"
                textSize = 13f
                setTypeface(null, Typeface.BOLD)
                setTextColor(if (done) 0xFF2F6B4F.toInt() else 0xFFC56C14.toInt())
                setPadding(dp(10), dp(4), dp(10), dp(4))
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(if (done) Color.WHITE else 0xFFFDF0DF.toInt())
                }
            }

            card.addView(leftCol)
            card.addView(ptsTv)

            if (!done) {
                card.setOnClickListener {
                    when {
                        isRope -> launchRopeChallenge(t)
                        t.timerMode == "countdown" || t.timerMode == "countup" -> launchTaskFocus(t)
                        t.requirePhoto -> showCheckinConfirmDialog(t, null, null)
                        else -> submitCheckin(t.id, null, null, null)
                    }
                }
            } else if (!photoUrl.isNullOrEmpty()) {
                card.setOnClickListener {
                    showCheckinPhotoViewDialog(t.label, photoUrl)
                }
            }
            binding.tasksContainer.addView(card)
        }
    }

    private fun launchTaskFocus(t: TaskItem) {
        val st = state
        val lv = st?.let { GrowthMeta.levelForXp(it.child.xp) } ?: 0
        val asleep = st?.let { isAsleep(it.logs) } ?: false
        val activeComp = st?.let { s -> s.catalog.find { it.id == s.child.activeCompanion } }
        val intent = Intent(this, FocusActivity::class.java).apply {
            putExtra(FocusActivity.EXTRA_TASK_ID, t.id)
            putExtra(FocusActivity.EXTRA_TASK_TITLE, t.label)
            putExtra(FocusActivity.EXTRA_TIMER_MODE, t.timerMode)
            putExtra(FocusActivity.EXTRA_DURATION_SEC, t.durationSec)
            putExtra(FocusActivity.EXTRA_PET_STAGE, lv)
            putExtra(FocusActivity.EXTRA_PET_ASLEEP, asleep)
            if (activeComp != null) putExtra(FocusActivity.EXTRA_SPECIES_ITEM, activeComp)
        }
        focusLauncher.launch(intent)
    }

    private fun decodeAndCompressCapture(file: File): Pair<Bitmap, String>? {
        return runCatching {
            val boundsOpts = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            BitmapFactory.decodeFile(file.absolutePath, boundsOpts)
            val srcW = boundsOpts.outWidth
            val srcH = boundsOpts.outHeight
            if (srcW <= 0 || srcH <= 0) return null

            var sample = 1
            while (srcW / sample > 1600 || srcH / sample > 1600) {
                sample *= 2
            }
            val decodeOpts = BitmapFactory.Options().apply { inSampleSize = sample }
            var bmp = BitmapFactory.decodeFile(file.absolutePath, decodeOpts) ?: return null

            // 读取 EXIF 旋转角并转正
            val exif = runCatching { ExifInterface(file.absolutePath) }.getOrNull()
            val orient = exif?.getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)
                ?: ExifInterface.ORIENTATION_NORMAL
            val deg = when (orient) {
                ExifInterface.ORIENTATION_ROTATE_90 -> 90f
                ExifInterface.ORIENTATION_ROTATE_180 -> 180f
                ExifInterface.ORIENTATION_ROTATE_270 -> 270f
                else -> 0f
            }
            if (deg != 0f) {
                val m = Matrix().apply { postRotate(deg) }
                val rotated = Bitmap.createBitmap(bmp, 0, 0, bmp.width, bmp.height, m, true)
                if (rotated != bmp) bmp.recycle()
                bmp = rotated
            }

            // 缩放到最长边 800px，兼顾清晰度与上传速度
            val maxSide = 800
            if (bmp.width > maxSide || bmp.height > maxSide) {
                val (dstW, dstH) = if (bmp.width >= bmp.height) {
                    maxSide to ((bmp.height * maxSide) / bmp.width).coerceAtLeast(1)
                } else {
                    ((bmp.width * maxSide) / bmp.height).coerceAtLeast(1) to maxSide
                }
                val scaled = Bitmap.createScaledBitmap(bmp, dstW, dstH, true)
                if (scaled != bmp) bmp.recycle()
                bmp = scaled
            }

            val baos = ByteArrayOutputStream()
            bmp.compress(Bitmap.CompressFormat.JPEG, 78, baos)
            val b64 = Base64.encodeToString(baos.toByteArray(), Base64.NO_WRAP)
            bmp to "data:image/jpeg;base64,$b64"
        }.getOrNull()
    }

    private fun launchCameraForCheckin(onCaptured: (Bitmap, String) -> Unit) {
        try {
            val dir = File(cacheDir, "captures").apply { mkdirs() }
            val file = File(dir, "checkin_${System.currentTimeMillis()}.jpg")
            pendingCaptureFile = file
            pendingPhotoCallback = onCaptured
            val uri = FileProvider.getUriForFile(this, "${packageName}.fileprovider", file)
            takePictureLauncher.launch(uri)
        } catch (e: Exception) {
            Toast.makeText(this, "无法打开相机：${e.localizedMessage ?: "请检查权限"}", Toast.LENGTH_SHORT).show()
        }
    }

    private fun showCheckinConfirmDialog(
        task: TaskItem,
        actualCount: Int?,
        actualDuration: Int?
    ) {
        var capturedPhotoData: String? = null

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(dp(20), dp(14), dp(20), dp(8))
        }

        val descTv = TextView(this).apply {
            text = if (actualCount != null) {
                "🎉 跳绳挑战达标（$actualCount 个）！\n可以拍一张照片留念，或直接完成打卡领 +${task.pts} 成长值。"
            } else {
                "拍一张作业或完成成果的照片，爸妈在手机上就能看到你的努力啦！"
            }
            textSize = 13.5f
            setTextColor(0xFF5C5347.toInt())
        }
        root.addView(descTv)

        val previewIv = ImageView(this).apply {
            visibility = View.GONE
            scaleType = ImageView.ScaleType.FIT_CENTER
            adjustViewBounds = true
            maxHeight = dp(210)
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFFF7F3EB.toInt())
                setStroke(dp(1), 0xFFE5DCCB.toInt())
            }
            setPadding(dp(6), dp(6), dp(6), dp(6))
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = dp(12) }
        }
        root.addView(previewIv)

        val btnCamera = Button(this).apply {
            text = "📷 拍照留证（拍成果给爸妈看）"
            textSize = 14f
            setAllCaps(false)
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFFE8913A.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                dp(46)
            ).apply { topMargin = dp(14) }
        }
        root.addView(btnCamera)

        val btnSubmit = Button(this).apply {
            text = if (task.requirePhoto) "📷 请先拍照再打卡 (+${task.pts})" else "✅ 直接完成打卡 (+${task.pts})"
            textSize = 14f
            setAllCaps(false)
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFF2F6B4F.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                dp(46)
            ).apply { topMargin = dp(10) }
        }
        root.addView(btnSubmit)

        val dialog = AlertDialog.Builder(this)
            .setTitle("${task.emoji} ${task.label}")
            .setView(root)
            .setNegativeButton("取消") { _, _ ->
                pendingPhotoCallback = null
            }
            .create()

        btnCamera.setOnClickListener {
            launchCameraForCheckin { bmp, dataUrl ->
                capturedPhotoData = dataUrl
                previewIv.setImageBitmap(bmp)
                previewIv.visibility = View.VISIBLE
                btnCamera.text = "🔄 重新拍一张"
                btnSubmit.text = "✅ 提交照片并打卡 (+${task.pts})"
            }
        }

        btnSubmit.setOnClickListener {
            if (task.requirePhoto && capturedPhotoData == null) {
                sound.playError()
                Toast.makeText(this, "这项任务要求先拍照留证哦！", Toast.LENGTH_SHORT).show()
                return@setOnClickListener
            }
            pendingPhotoCallback = null
            dialog.dismiss()
            submitCheckin(task.id, actualCount, actualDuration, capturedPhotoData)
        }

        dialog.show()
    }

    private fun showCheckinPhotoViewDialog(taskLabel: String, photoUrl: String) {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER
            setPadding(dp(16), dp(14), dp(16), dp(8))
        }
        val statusTv = TextView(this).apply {
            text = "正在加载打卡照片..."
            textSize = 13f
            setTextColor(0xFF8A7E6E.toInt())
            gravity = Gravity.CENTER
        }
        val iv = ImageView(this).apply {
            visibility = View.GONE
            scaleType = ImageView.ScaleType.FIT_CENTER
            adjustViewBounds = true
            maxHeight = dp(340)
        }
        root.addView(statusTv)
        root.addView(iv)

        AlertDialog.Builder(this)
            .setTitle("📷 $taskLabel · 今日打卡照片")
            .setView(root)
            .setPositiveButton("关闭", null)
            .show()

        api.fetchBitmap(photoUrl) { bmp ->
            if (bmp != null) {
                statusTv.visibility = View.GONE
                iv.setImageBitmap(bmp)
                iv.visibility = View.VISIBLE
            } else {
                statusTv.text = "加载照片失败，请稍后重试"
            }
        }
    }

    private fun launchRopeChallenge(t: TaskItem) {
        val taskJson = JSONObject().apply {
            put("taskId", t.id)
            put("label", t.label)
            put("durationSec", t.durationSec)
            put("targetCount", t.targetCount)
            put("childName", state?.child?.name ?: "")
        }.toString()
        val intent = Intent(this, RopeJumpActivity::class.java)
        intent.putExtra(RopeJumpActivity.EXTRA_TASK_JSON, taskJson)
        ropeChallengeLauncher.launch(intent)
    }

    private fun submitCheckin(
        taskId: String,
        actualCount: Int?,
        actualDuration: Int?,
        photoData: String?
    ) {
        val prevLv = state?.let { GrowthMeta.levelForXp(it.child.xp) } ?: 0
        // 零延迟即时反馈：点击瞬间先播 SoundPool + 食物粒子
        sound.playFeed()
        binding.petStageView.triggerFeedAnimation()

        val payload = JSONObject().apply {
            put("taskId", taskId)
            put("date", todayStr())
            if (actualCount != null) put("actualCount", actualCount)
            if (actualDuration != null) put("actualDuration", actualDuration)
            if (!photoData.isNullOrEmpty()) put("photoData", photoData)
        }
        api.request("POST", "/api/child/checkin", payload) { res ->
            if (res.ok && res.json != null) {
                val stObj = res.json.optJSONObject("state")
                if (stObj != null) {
                    val newState = GrowthMeta.parseChildState(stObj)
                    state = newState
                    renderAll(newState)
                    val newLv = GrowthMeta.levelForXp(newState.child.xp)
                    if (newLv > prevLv) {
                        sound.playWin()
                        binding.petStageView.triggerCelebration()
                        val lvName = GrowthMeta.LEVELS[newLv].name
                        sound.speak("太棒了，你的小伙伴进化成${lvName}啦")
                        AlertDialog.Builder(this)
                            .setTitle("🎉 进化啦！Lv.${newLv + 1} $lvName")
                            .setMessage(GrowthMeta.LEVELS[newLv].talk)
                            .setPositiveButton("太棒啦！", null)
                            .show()
                    } else if (actualCount != null) {
                        sound.playWin()
                        binding.petStageView.triggerCelebration()
                        Toast.makeText(this, "🪢 跳绳 ${actualCount} 个达标！喂宠成功！", Toast.LENGTH_LONG).show()
                    } else {
                        val photoNote = if (!photoData.isNullOrEmpty()) "（📷 照片已传给爸妈）" else ""
                        Toast.makeText(this, "打卡成功！成长值 +${res.json.optInt("pts", 0)}$photoNote", Toast.LENGTH_SHORT).show()
                    }
                } else {
                    loadChildState()
                }
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "打卡失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun performUndo() {
        api.request("POST", "/api/child/undo", JSONObject().put("date", todayStr())) { res ->
            if (res.ok && res.json != null) {
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderAll(ns)
                } ?: loadChildState()
                Toast.makeText(this, "已撤销刚才的打卡", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, res.error ?: "暂无可撤销的今日打卡", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun toggleMute() {
        val nextMute = !sound.isMuted()
        sound.setMuted(nextMute)
        binding.btnMute.text = if (nextMute) "🔇" else "🔊"
        if (!nextMute) sound.playPat()
        updateVolumeBoostButton()
        api.request("PATCH", "/api/child/mute", JSONObject().put("muted", nextMute)) { }
    }

    private fun showRenamePetDialog() {
        val input = EditText(this).apply {
            hint = "最多 10 个字"
            setText(state?.child?.petName ?: "")
            setPadding(40, 28, 40, 28)
        }
        AlertDialog.Builder(this)
            .setTitle("给小伙伴起个名字")
            .setView(input)
            .setPositiveButton("保存") { _, _ ->
                val name = input.text.toString().trim().take(10)
                api.request("PATCH", "/api/child/pet-name", JSONObject().put("petName", name)) { res ->
                    if (res.ok) {
                        binding.tvPetName.text = name.ifEmpty { "${state?.child?.name ?: ""}的小伙伴" }
                        sound.playPat()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun renderDex(st: ChildState) {
        val c = st.child
        binding.tvCouponTitle.text = "当前拥有领养蛋：${c.coupons} 枚"
        binding.dexContainer.removeAllViews()

        val pairs = st.catalog.chunked(2)
        for (rowItems in pairs) {
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(10) }
            }
            for (idx in 0 until 2) {
                val sp = rowItems.getOrNull(idx)
                if (sp == null) {
                    val spacer = View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply { marginStart = dp(10) }
                    }
                    row.addView(spacer)
                    continue
                }
                val isOwned = st.owned.containsKey(sp.id)
                val isActive = c.activeCompanion == sp.id

                val card = LinearLayout(this).apply {
                    orientation = LinearLayout.VERTICAL
                    gravity = Gravity.CENTER_HORIZONTAL
                    setPadding(dp(10), dp(12), dp(10), dp(12))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(16).toFloat()
                        setColor(if (isOwned) 0xFFFAFDFB.toInt() else Color.WHITE)
                        setStroke(dp(1), if (isOwned) 0xFFBED6C7.toInt() else 0xFFEAE2D3.toInt())
                    }
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f).apply {
                        if (idx == 1) marginStart = dp(10)
                    }
                }

                val thumb = SpeciesThumbView(this).apply {
                    setSpecies(sp, isSilhouette = !isOwned)
                    layoutParams = LinearLayout.LayoutParams(dp(106), dp(92))
                }
                val nameTv = TextView(this).apply {
                    text = if (isOwned) sp.name else "？？？"
                    textSize = 14.5f
                    setTypeface(null, Typeface.BOLD)
                    setTextColor(0xFF2B2620.toInt())
                    setPadding(0, dp(4), 0, 0)
                }
                val hintTv = TextView(this).apply {
                    text = if (isOwned) "已拥有 · ${st.owned[sp.id]}" else sp.hint.ifEmpty { "等待孵化" }
                    textSize = 11.5f
                    setTextColor(if (isOwned) 0xFF2F6B4F.toInt() else 0xFF8A7E6E.toInt())
                }
                val actionBtn = Button(this).apply {
                    textSize = 12f
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        dp(38)
                    ).apply { topMargin = dp(8) }
                    when {
                        isActive -> {
                            text = "✨ 陪伴中"
                            isEnabled = false
                            setBackgroundColor(0xFFE3F2E8.toInt())
                            setTextColor(0xFF2F6B4F.toInt())
                        }
                        isOwned -> {
                            text = "设为陪伴伙伴"
                            setBackgroundColor(0xFF2F6B4F.toInt())
                            setTextColor(Color.WHITE)
                            setOnClickListener { performSetCompanion(sp.id) }
                        }
                        c.coupons > 0 -> {
                            text = "🥚 用 1 枚蛋领养"
                            setBackgroundColor(0xFFE8913A.toInt())
                            setTextColor(Color.WHITE)
                            setOnClickListener { performAdopt(sp.id, mystery = false) }
                        }
                        else -> {
                            text = "需要 1 枚领养蛋"
                            isEnabled = false
                            setBackgroundColor(0xFFEDE6DA.toInt())
                            setTextColor(0xFF8A7E6E.toInt())
                        }
                    }
                }

                card.addView(thumb)
                card.addView(nameTv)
                card.addView(hintTv)
                card.addView(actionBtn)
                row.addView(card)
            }
            binding.dexContainer.addView(row)
        }
    }

    private fun performAdopt(speciesId: String?, mystery: Boolean) {
        val payload = JSONObject().apply {
            put("mystery", mystery)
            put("date", todayStr())
            if (speciesId != null) put("speciesId", speciesId)
        }
        api.request("POST", "/api/child/adopt", payload) { res ->
            if (res.ok && res.json != null) {
                sound.playWin()
                val adoptedName = res.json.optJSONObject("adopted")?.optString("name", "新伙伴") ?: "新伙伴"
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderAll(ns)
                } ?: loadChildState()
                sound.speak("恭喜你，领养了新伙伴${adoptedName}")
                AlertDialog.Builder(this)
                    .setTitle("🎉 新伙伴来啦！")
                    .setMessage("欢迎「$adoptedName」加入你的图鉴，现在可以带它出来陪你一起成长啦！")
                    .setPositiveButton("太棒啦！", null)
                    .show()
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "领养失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun performSetCompanion(speciesId: String) {
        api.request("POST", "/api/child/companion", JSONObject().put("speciesId", speciesId)) { res ->
            if (res.ok) {
                sound.playPat()
                loadChildState()
                Toast.makeText(this, "已更换陪伴伙伴！", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, res.error ?: "设置失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun renderPolaroids(st: ChildState) {
        val c = st.child
        binding.tvWallCount.text = "${st.polaroids.size} 张合影"

        // 显示当前头像，并提示拍立得直接使用该头像（可点击更换头像）
        binding.avatarPickRow.removeAllViews()
        val curAv = SpeciesThumbView(this).apply {
            bindAvatarView(this, c.avatarIdx, c.avatarUrl)
            layoutParams = LinearLayout.LayoutParams(dp(42), dp(42))
            setOnClickListener { showAvatarPickerDialog() }
        }
        val infoTv = TextView(this).apply {
            text = "拍立得将直接使用你当前的头像"
            textSize = 12.5f
            setTextColor(0xFF7D7263.toInt())
            setPadding(dp(10), 0, dp(8), 0)
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        val btnChange = Button(this).apply {
            text = "更换头像"
            textSize = 12f
            setAllCaps(false)
            setTextColor(0xFF2F6B4F.toInt())
            background = GradientDrawable().apply {
                cornerRadius = dp(99).toFloat()
                setColor(0xFFE3F2E8.toInt())
                setStroke(dp(1), 0xFFB8D4C3.toInt())
            }
            setPadding(dp(12), 0, dp(12), 0)
            minWidth = 0
            minimumWidth = 0
            layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(32))
            setOnClickListener { showAvatarPickerDialog() }
        }
        binding.avatarPickRow.addView(curAv)
        binding.avatarPickRow.addView(infoTv)
        binding.avatarPickRow.addView(btnChange)

        // 合影墙卡片
        binding.wallContainer.removeAllViews()
        if (st.polaroids.isEmpty()) {
            val empty = TextView(this).apply {
                text = "还没有合影，快拍下第一张拍立得留念吧！"
                setTextColor(0xFF8A7E6E.toInt())
                gravity = Gravity.CENTER
                setPadding(dp(8), dp(14), dp(8), dp(14))
            }
            binding.wallContainer.addView(empty)
            return
        }

        for (rowItems in st.polaroids.chunked(2)) {
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(10) }
            }
            for (idx in 0 until 2) {
                val p = rowItems.getOrNull(idx)
                if (p == null) {
                    row.addView(View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply { marginStart = dp(10) }
                    })
                    continue
                }
                val frame = LinearLayout(this).apply {
                    orientation = LinearLayout.VERTICAL
                    gravity = Gravity.CENTER_HORIZONTAL
                    setPadding(dp(8), dp(8), dp(8), dp(10))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(10).toFloat()
                        setColor(Color.WHITE)
                        setStroke(dp(1), 0xFFE5DCCB.toInt())
                    }
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f).apply {
                        if (idx == 1) marginStart = dp(10)
                    }
                }
                val photoBox = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER
                    setPadding(dp(4), dp(8), dp(4), dp(6))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(6).toFloat()
                        setColor(0xFFFBF2E3.toInt())
                    }
                    layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(88))
                }
                val kidView = SpeciesThumbView(this).apply {
                    bindAvatarView(this, p.avatarIdx, p.avatarUrl)
                    layoutParams = LinearLayout.LayoutParams(dp(56), dp(56))
                }
                val petView = SpeciesThumbView(this).apply {
                    val sp = st.catalog.find { it.id == p.petRef }
                    if (sp != null) setSpecies(sp, isSilhouette = false)
                    else setMainPet(p.petLv)
                    layoutParams = LinearLayout.LayoutParams(dp(72), dp(68))
                }
                photoBox.addView(kidView)
                photoBox.addView(petView)

                val dateTv = TextView(this).apply {
                    text = p.photoDate
                    textSize = 11.5f
                    setTextColor(0xFF5C5347.toInt())
                    setPadding(0, dp(6), 0, 0)
                }
                frame.addView(photoBox)
                frame.addView(dateTv)
                if (p.note.isNotEmpty()) {
                    val noteTv = TextView(this).apply {
                        text = p.note
                        textSize = 11f
                        setTextColor(0xFF8A7E6E.toInt())
                    }
                    frame.addView(noteTv)
                }
                row.addView(frame)
            }
            binding.wallContainer.addView(row)
        }
    }

    private fun performTakePolaroid() {
        val payload = JSONObject().apply {
            put("note", binding.etSnapNote.text.toString().trim().take(16))
            put("date", todayStr())
        }
        api.request("POST", "/api/child/polaroid", payload) { res ->
            if (res.ok && res.json != null) {
                sound.playWin()
                binding.etSnapNote.setText("")
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderAll(ns)
                } ?: loadChildState()
                Toast.makeText(this, "📷 今日合影已珍藏到纪念墙！", Toast.LENGTH_SHORT).show()
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "拍照失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun bindAvatarView(view: SpeciesThumbView, avatarIdx: Int, avatarUrl: String) {
        view.tag = avatarUrl
        view.setAvatar(avatarIdx, null)
        if (avatarUrl.isNotEmpty()) {
            api.fetchBitmap(avatarUrl) { bmp ->
                if (bmp != null && view.tag == avatarUrl) {
                    view.setAvatar(avatarIdx, bmp)
                }
            }
        }
    }

    private fun cropSquareAndEncodeAvatar(src: Bitmap): Pair<Bitmap, String> {
        val side = min(src.width, src.height).coerceAtLeast(1)
        val sx = (src.width - side) / 2
        val sy = (src.height - side) / 2
        val square = if (src.width == src.height) src else Bitmap.createBitmap(src, sx, sy, side, side)
        val scaled = if (square.width == 256 && square.height == 256) square
        else Bitmap.createScaledBitmap(square, 256, 256, true)
        if (square != src && square != scaled) square.recycle()
        val baos = ByteArrayOutputStream()
        scaled.compress(Bitmap.CompressFormat.JPEG, 85, baos)
        val b64 = Base64.encodeToString(baos.toByteArray(), Base64.NO_WRAP)
        return scaled to "data:image/jpeg;base64,$b64"
    }

    private fun decodeUriToSquareAvatar(uri: Uri): Pair<Bitmap, String>? {
        return runCatching {
            val boundsOpts = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            contentResolver.openInputStream(uri)?.use {
                BitmapFactory.decodeStream(it, null, boundsOpts)
            }
            val srcW = boundsOpts.outWidth
            val srcH = boundsOpts.outHeight
            if (srcW <= 0 || srcH <= 0) return null

            var sample = 1
            while (srcW / sample > 1024 || srcH / sample > 1024) {
                sample *= 2
            }
            val decodeOpts = BitmapFactory.Options().apply { inSampleSize = sample }
            var bmp = contentResolver.openInputStream(uri)?.use {
                BitmapFactory.decodeStream(it, null, decodeOpts)
            } ?: return null

            val orient = runCatching {
                contentResolver.openInputStream(uri)?.use { stream ->
                    ExifInterface(stream).getAttributeInt(
                        ExifInterface.TAG_ORIENTATION,
                        ExifInterface.ORIENTATION_NORMAL
                    )
                }
            }.getOrNull() ?: ExifInterface.ORIENTATION_NORMAL
            val deg = when (orient) {
                ExifInterface.ORIENTATION_ROTATE_90 -> 90f
                ExifInterface.ORIENTATION_ROTATE_180 -> 180f
                ExifInterface.ORIENTATION_ROTATE_270 -> 270f
                else -> 0f
            }
            if (deg != 0f) {
                val m = Matrix().apply { postRotate(deg) }
                val rotated = Bitmap.createBitmap(bmp, 0, 0, bmp.width, bmp.height, m, true)
                if (rotated != bmp) bmp.recycle()
                bmp = rotated
            }
            cropSquareAndEncodeAvatar(bmp)
        }.getOrNull()
    }

    private fun saveChildAvatar(avatarIdx: Int?, avatarData: String?, onDone: () -> Unit) {
        val payload = JSONObject().apply {
            if (avatarData != null) put("avatarData", avatarData)
            else if (avatarIdx != null) put("avatarIdx", avatarIdx)
        }
        api.request("POST", "/api/child/avatar", payload) { res ->
            if (res.ok && res.json != null) {
                sound.playWin()
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderAll(ns)
                } ?: loadChildState()
                Toast.makeText(this, "✨ 头像已更新！", Toast.LENGTH_SHORT).show()
                onDone()
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "头像更新失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showAvatarPickerDialog() {
        val c = state?.child ?: return
        var dialogRef: AlertDialog? = null

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(18), dp(14), dp(18), dp(10))
        }

        // 当前头像预览区
        val previewRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(dp(12), dp(10), dp(12), dp(10))
            background = GradientDrawable().apply {
                cornerRadius = dp(14).toFloat()
                setColor(0xFFFAF6EE.toInt())
                setStroke(dp(1), 0xFFEAE2D3.toInt())
            }
        }
        val previewAvatar = SpeciesThumbView(this).apply {
            bindAvatarView(this, c.avatarIdx, c.avatarUrl)
            layoutParams = LinearLayout.LayoutParams(dp(58), dp(58))
        }
        val previewText = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(12), 0, 0, 0)
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        previewText.addView(TextView(this).apply {
            text = c.name
            textSize = 15.5f
            setTypeface(null, Typeface.BOLD)
            setTextColor(0xFF2B2620.toInt())
        })
        previewText.addView(TextView(this).apply {
            text = "登录后将显示在左上角，拍立得也会使用此头像"
            textSize = 11.5f
            setTextColor(0xFF8A7E6E.toInt())
            setPadding(0, dp(2), 0, 0)
        })
        previewRow.addView(previewAvatar)
        previewRow.addView(previewText)
        root.addView(previewRow)

        // 上传照片按钮行（相册 + 拍照）
        val uploadRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = dp(12) }
        }
        val btnGallery = Button(this).apply {
            text = "📁 从相册选照片"
            textSize = 13f
            setAllCaps(false)
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFFE8913A.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(0, dp(42), 1f).apply { marginEnd = dp(6) }
            setOnClickListener {
                pendingGalleryAvatarCallback = { bmp, dataUrl ->
                    previewAvatar.tag = ""
                    previewAvatar.setAvatar(c.avatarIdx, bmp)
                    saveChildAvatar(null, dataUrl) {
                        dialogRef?.dismiss()
                    }
                }
                pickAvatarLauncher.launch("image/*")
            }
        }
        val btnCamera = Button(this).apply {
            text = "📷 拍照设为头像"
            textSize = 13f
            setAllCaps(false)
            setTextColor(Color.WHITE)
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFF2F6B4F.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(0, dp(42), 1f).apply { marginStart = dp(6) }
            setOnClickListener {
                launchCameraForCheckin { rawBmp, _ ->
                    val (sqBmp, dataUrl) = cropSquareAndEncodeAvatar(rawBmp)
                    previewAvatar.tag = ""
                    previewAvatar.setAvatar(c.avatarIdx, sqBmp)
                    saveChildAvatar(null, dataUrl) {
                        dialogRef?.dismiss()
                    }
                }
            }
        }
        uploadRow.addView(btnGallery)
        uploadRow.addView(btnCamera)
        root.addView(uploadRow)

        root.addView(TextView(this).apply {
            text = "或选择内置卡通头像（共 ${GrowthMeta.AVATARS.size} 款）"
            textSize = 12.5f
            setTextColor(0xFF7D7263.toInt())
            setPadding(0, dp(14), 0, dp(8))
        })

        // 4 列 x 3 行内置头像网格
        val hasCustom = c.avatarUrl.isNotEmpty()
        for (rowStart in GrowthMeta.AVATARS.indices step 4) {
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(8) }
            }
            for (col in 0 until 4) {
                val avIdx = rowStart + col
                if (avIdx >= GrowthMeta.AVATARS.size) {
                    row.addView(View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f)
                    })
                    continue
                }
                val cellWrap = LinearLayout(this).apply {
                    gravity = Gravity.CENTER
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                }
                val isCurrent = !hasCustom && (avIdx == c.avatarIdx)
                val avView = SpeciesThumbView(this).apply {
                    setAvatar(avIdx, null)
                    setPadding(dp(3), dp(3), dp(3), dp(3))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(14).toFloat()
                        setColor(if (isCurrent) 0xFFE3F2E8.toInt() else 0xFFFDFCFA.toInt())
                        setStroke(dp(if (isCurrent) 2 else 1), if (isCurrent) 0xFF2F6B4F.toInt() else 0xFFEAE2D3.toInt())
                    }
                    layoutParams = LinearLayout.LayoutParams(dp(56), dp(56))
                    setOnClickListener {
                        sound.playPat()
                        saveChildAvatar(avIdx, null) {
                            dialogRef?.dismiss()
                        }
                    }
                }
                cellWrap.addView(avView)
                row.addView(cellWrap)
            }
            root.addView(row)
        }

        val scroll = ScrollView(this).apply { addView(root) }
        dialogRef = AlertDialog.Builder(this)
            .setTitle("设置我的头像")
            .setView(scroll)
            .setNegativeButton("关闭", null)
            .create()
        dialogRef.show()
    }

    private fun dp(v: Int): Int = (v * resources.displayMetrics.density).toInt()

    override fun onDestroy() {
        updater.shutdown()
        sound.release()
        api.shutdown()
        super.onDestroy()
    }
}
