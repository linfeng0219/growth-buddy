package com.growthbuddy.child

import android.content.Intent
import android.content.res.Configuration
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
import android.view.ViewGroup
import android.view.inputmethod.EditorInfo
import android.widget.Button
import android.widget.EditText
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.PopupWindow
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.FileProvider
import com.growthbuddy.child.databinding.ActivityMainBinding
import org.json.JSONArray
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

    private val mailboxLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { _ ->
        loadChildState()
    }

    private val dexLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { _ ->
        loadChildState()
    }

    private val adventureLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { _ ->
        loadChildState()
    }

    private val parentPortalLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { res ->
        if (res.resultCode == ParentPortalActivity.RESULT_SWITCH_ACCOUNT) {
            showLoginView("已退出家长端，可直接下拉选择或输入账号切换")
        } else {
            finish()
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
        val isNewRecord = obj.optBoolean("isNewRecord", false)
        val photoPath = obj.optString("photoPath", "")

        if (completed && taskId.isNotEmpty()) {
            if (isNewRecord) {
                Toast.makeText(this, "🎆 太厉害了！跳绳 ${actualCount} 个打破历史最高纪录！", Toast.LENGTH_LONG).show()
            }
            val autoPhotoDataUrl = readRopePhotoDataUrl(photoPath)
            if (autoPhotoDataUrl != null) {
                submitCheckin(taskId, actualCount, actualDuration, autoPhotoDataUrl)
            } else {
                val taskItem = state?.tasks?.find { it.id == taskId }
                if (taskItem != null) {
                    showCheckinConfirmDialog(taskItem, actualCount, actualDuration)
                } else {
                    submitCheckin(taskId, actualCount, actualDuration, null)
                }
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

    private fun readRopePhotoDataUrl(photoPath: String): String? {
        if (photoPath.isBlank()) return null
        val file = File(photoPath)
        if (!file.exists()) return null
        return try {
            val bytes = file.readBytes()
            if (bytes.isEmpty()) null
            else "data:image/jpeg;base64," + Base64.encodeToString(bytes, Base64.NO_WRAP)
        } catch (_: Exception) {
            null
        } finally {
            runCatching { file.delete() }
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
        applyResponsiveLayout()

        val activeRole = ServerPrefs.getActiveRole(this)
        if (activeRole == "parent" && ServerPrefs.getParentToken(this).isNotEmpty()) {
            showLoginView()
            launchParentPortal("")
        } else if (ServerPrefs.getChildToken(this).isNotEmpty()) {
            showAppView()
            loadChildState()
        } else {
            showLoginView()
        }
        // 统一启动静默升级检测：无论是否已登录，有新版或强制升级包立即提醒，无新版不打扰
        updater.checkForUpdate(silent = true)
    }

    override fun onResume() {
        super.onResume()
        updater.resumePendingInstallIfAllowed()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)

        // 登录页：平板横屏左右双栏（左宠物萌态介绍 / 右登录表单），平板竖屏居中限宽 500dp
        ResponsiveUi.applyCenteredMaxWidth(this, binding.loginTopBar, 940, horizontalReserveDp = 40)
        val loginHeroLp = binding.loginHeroPane.layoutParams as LinearLayout.LayoutParams
        val loginFormLp = binding.loginFormPane.layoutParams as LinearLayout.LayoutParams
        val previewLp = binding.loginPetPreview.layoutParams
        if (m.isDesktopSplit) {
            binding.loginSplitRow.orientation = LinearLayout.HORIZONTAL
            ResponsiveUi.applyCenteredMaxWidth(this, binding.loginSplitRow, 920, horizontalReserveDp = 40)
            loginHeroLp.width = 0
            loginHeroLp.weight = 0.9f
            loginFormLp.width = 0
            loginFormLp.weight = 1.1f
            loginFormLp.marginStart = dp(28)
            previewLp.width = dp(244)
            previewLp.height = dp(204)
        } else {
            binding.loginSplitRow.orientation = LinearLayout.VERTICAL
            ResponsiveUi.applyCenteredMaxWidth(this, binding.loginSplitRow, if (m.isTablet) 500 else 1000, horizontalReserveDp = 40)
            loginHeroLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            loginHeroLp.weight = 0f
            loginFormLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            loginFormLp.weight = 0f
            loginFormLp.marginStart = 0
            previewLp.width = dp(if (m.isTablet) 230 else 210)
            previewLp.height = dp(if (m.isTablet) 190 else 175)
        }
        binding.loginHeroPane.layoutParams = loginHeroLp
        binding.loginFormPane.layoutParams = loginFormLp
        binding.loginPetPreview.layoutParams = previewLp

        // 首页 & 图鉴页：平板横屏左右双栏（左宠物舞台+快捷胶囊 / 右今日专属任务）
        val leftLp = binding.homeLeftPane.layoutParams as LinearLayout.LayoutParams
        val rightLp = binding.homeRightPane.layoutParams as LinearLayout.LayoutParams
        val stageLp = binding.petStageView.layoutParams
        if (m.isDesktopSplit) {
            binding.homeSection.orientation = LinearLayout.HORIZONTAL
            ResponsiveUi.applyCenteredMaxWidth(this, binding.homeSection, 1140, horizontalReserveDp = 28)
            leftLp.width = 0
            leftLp.weight = 0.92f
            rightLp.width = 0
            rightLp.weight = 1.08f
            rightLp.topMargin = 0
            rightLp.marginStart = dp(16)
            stageLp.height = dp(244)
        } else {
            binding.homeSection.orientation = LinearLayout.VERTICAL
            ResponsiveUi.applyCenteredMaxWidth(this, binding.homeSection, if (m.isTablet) 680 else 1200, horizontalReserveDp = 28)
            leftLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            leftLp.weight = 0f
            rightLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            rightLp.weight = 0f
            rightLp.topMargin = dp(14)
            rightLp.marginStart = 0
            stageLp.height = dp(if (m.isTablet) 232 else if (m.isLandscape) 156 else 196)
        }
        binding.homeLeftPane.layoutParams = leftLp
        binding.homeRightPane.layoutParams = rightLp
        binding.petStageView.layoutParams = stageLp
    }

    private fun setupLoginUi() {
        binding.loginPetPreview.setPetState(3, false, null)
        refreshSavedAccountPicker(autoFillIfBlank = true)
        binding.layoutSavedAccountPicker.setOnClickListener {
            showSavedAccountsDropdown()
        }
        refreshServerFooter()
        binding.tvServerFooter.setOnClickListener { showChangelogDialog() }

        binding.btnLoginUpdate.setOnClickListener { updater.checkForUpdate(silent = false) }
        binding.btnDoLogin.setOnClickListener { performUnifiedLogin() }
        binding.etLoginPass.setOnEditorActionListener { _, actionId, _ ->
            if (actionId == EditorInfo.IME_ACTION_DONE) {
                performUnifiedLogin()
                true
            } else false
        }
    }

    private fun setupAppUi() {
        binding.ivHeaderAvatar.setOnClickListener { showAvatarPickerDialog() }
        binding.btnMute.setOnClickListener { toggleMute() }
        binding.mainSwipeRefresh.setColorSchemeColors(0xFF2F6B4F.toInt(), 0xFFE8913A.toInt())
        binding.mainSwipeRefresh.setOnRefreshListener {
            sound.playPat()
            loadChildState()
        }
        binding.btnLogout.setOnClickListener { confirmLogout() }

        binding.tabHome.setOnClickListener {
            binding.mainScroll.smoothScrollTo(0, 0)
        }
        binding.tabMessages.setOnClickListener {
            sound.playPat()
            val c = state?.child
            val chatAndReq = (c?.unreadChat ?: 0) + (c?.pendingFriendRequests ?: 0)
            val mailUnread = c?.unreadLetters ?: 0
            val initMode = if (chatAndReq == 0 && mailUnread > 0) "mail" else "chat"
            val intent = Intent(this, FriendsActivity::class.java).apply {
                putExtra(FriendsActivity.EXTRA_INITIAL_MODE, initMode)
            }
            mailboxLauncher.launch(intent)
        }

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
            }
        }
        binding.btnRenamePet.setOnClickListener { showRenamePetDialog() }
        binding.btnUndo.setOnClickListener { performUndo() }
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
                putExtra(AdventureActivity.EXTRA_CHILD_AGE, st?.child?.age ?: 7)
                putExtra(AdventureActivity.EXTRA_QUIZ_DIFFICULTY, st?.child?.quizDifficulty ?: "normal")
                putExtra(AdventureActivity.EXTRA_ADVENTURE_XP_TODAY, st?.child?.adventureXpToday ?: 0)
                putExtra(AdventureActivity.EXTRA_ADVENTURE_XP_LIMIT, st?.child?.adventureXpLimit ?: 10)
                if (activeComp != null) putExtra(AdventureActivity.EXTRA_SPECIES_ITEM, activeComp)
            }
            adventureLauncher.launch(intent)
        }
        binding.btnOpenDex.setOnClickListener {
            sound.playPat()
            dexLauncher.launch(Intent(this, DexActivity::class.java))
        }
    }

    private fun refreshServerFooter() {
        binding.tvServerFooter.text = "版本 ${BuildConfig.VERSION_NAME} · 更新记录 ⓘ"
    }

    private fun formatChangelogJson(arr: JSONArray): String {
        val sb = StringBuilder()
        for (i in 0 until arr.length()) {
            val entry = arr.optJSONObject(i) ?: continue
            val dt = entry.optString("date", "")
            val appItems = entry.optJSONArray("app_items")
            val useItems = if (appItems != null && appItems.length() > 0) appItems else entry.optJSONArray("items")
            if (useItems == null || useItems.length() == 0) continue
            val title = entry.optString("app_title", "").ifBlank { entry.optString("title", "") }
            sb.append("📱 ").append(dt)
            if (title.isNotEmpty()) sb.append(" · ").append(title)
            sb.append("\n")
            for (j in 0 until useItems.length()) {
                sb.append("  • ").append(useItems.optString(j, "")).append("\n")
            }
            if (i + 1 < arr.length()) sb.append("\n")
        }
        return sb.toString().trim()
    }

    private fun showChangelogDialog() {
        val cachePrefs = getSharedPreferences("gb_changelog_cache", MODE_PRIVATE)
        val cachedText = runCatching {
            val raw = cachePrefs.getString("changelog_json", "").orEmpty()
            if (raw.isNotEmpty()) formatChangelogJson(JSONArray(raw)) else ""
        }.getOrDefault("")
        val fallback = cachedText.ifEmpty { "暂未缓存更新记录，请联网后点击查看最新更新内容。" }

        val tv = TextView(this).apply {
            text = cachedText.ifEmpty { "正在加载更新内容..." }
            textSize = 13.5f
            setTextColor(0xFF4A4238.toInt())
            setPadding(dp(20), dp(14), dp(20), dp(12))
            setLineSpacing(dp(4).toFloat(), 1f)
        }
        val scroll = ScrollView(this).apply {
            addView(tv)
        }
        val dlg = AlertDialog.Builder(this)
            .setTitle("📋 版本更新内容 (${BuildConfig.VERSION_NAME})")
            .setView(scroll)
            .setPositiveButton("知道了", null)
            .create()
        dlg.show()

        api.request("GET", "/api/changelog", null) { res ->
            val arr = res.json?.optJSONArray("changelog")
            if (res.ok && arr != null && arr.length() > 0) {
                cachePrefs.edit().putString("changelog_json", arr.toString()).apply()
                tv.text = formatChangelogJson(arr).ifEmpty { fallback }
            } else {
                tv.text = fallback
            }
        }
    }

    private fun showLoginView(message: String = "") {
        binding.loginContainer.visibility = View.VISIBLE
        binding.appContainer.visibility = View.GONE
        binding.tvLoginMsg.text = message
        refreshSavedAccountPicker(autoFillIfBlank = true)
        refreshServerFooter()
        binding.tvServerFooter.setOnClickListener { showChangelogDialog() }
    }

    private fun selectSavedAccountIntoForm(acc: SavedAccount) {
        binding.etLoginUser.setText(acc.username)
        binding.etLoginPass.setText(acc.password)
        bindAvatarView(binding.ivLoginSavedAvatar, acc.avatarIdx, acc.avatarUrl)
        val roleLabel = if (acc.role == "parent") "👨‍👩‍👧 家长端" else "👧 孩子端"
        binding.tvLoginSavedName.text = "${acc.displayName} · $roleLabel"
        binding.tvLoginSavedSub.text = "@${acc.username}"
    }

    private fun refreshSavedAccountPicker(autoFillIfBlank: Boolean) {
        val accounts = ServerPrefs.getSavedAccounts(this)
        if (accounts.isEmpty()) {
            binding.layoutSavedAccountPicker.visibility = View.GONE
            if (autoFillIfBlank && binding.etLoginUser.text.isNullOrBlank()) {
                binding.etLoginUser.setText(ServerPrefs.getSavedUsername(this))
            }
            return
        }
        binding.layoutSavedAccountPicker.visibility = View.VISIBLE
        val typedUser = binding.etLoginUser.text.toString().trim()
        val curAcc = accounts.firstOrNull { it.username.equals(typedUser, ignoreCase = true) } ?: accounts.first()
        if (autoFillIfBlank && typedUser.isEmpty()) {
            selectSavedAccountIntoForm(curAcc)
        } else {
            bindAvatarView(binding.ivLoginSavedAvatar, curAcc.avatarIdx, curAcc.avatarUrl)
            val roleLabel = if (curAcc.role == "parent") "👨‍👩‍👧 家长端" else "👧 孩子端"
            binding.tvLoginSavedName.text = "${curAcc.displayName} · $roleLabel"
            binding.tvLoginSavedSub.text = "@${curAcc.username}"
        }
    }

    private fun showSavedAccountsDropdown() {
        val accounts = ServerPrefs.getSavedAccounts(this)
        if (accounts.isEmpty()) return
        sound.playPat()

        val card = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(8), dp(8), dp(8), dp(8))
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#FFFDF9"))
                cornerRadius = dp(14).toFloat()
                setStroke(dp(1), Color.parseColor("#D8CFC0"))
            }
            elevation = dp(8).toFloat()
        }

        val listCol = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }
        val scroll = ScrollView(this).apply {
            addView(listCol)
        }
        card.addView(scroll)

        val anchorW = binding.layoutSavedAccountPicker.width.coerceAtLeast(dp(260))
        val popup = PopupWindow(
            card,
            anchorW,
            LinearLayout.LayoutParams.WRAP_CONTENT,
            true
        ).apply {
            isOutsideTouchable = true
            elevation = dp(10).toFloat()
        }

        accounts.forEachIndexed { idx, acc ->
            val isParent = acc.role == "parent"
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(10), dp(8), dp(8), dp(8))
                background = GradientDrawable().apply {
                    setColor(Color.parseColor(if (idx == 0) "#F5F9F6" else "#FFFFFF"))
                    cornerRadius = dp(10).toFloat()
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply {
                    if (idx < accounts.lastIndex) bottomMargin = dp(4)
                }
            }

            val avatar = SpeciesThumbView(this).apply {
                layoutParams = LinearLayout.LayoutParams(dp(36), dp(36)).apply {
                    rightMargin = dp(10)
                }
            }
            bindAvatarView(avatar, acc.avatarIdx, acc.avatarUrl)

            val infoCol = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val titleRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
            }
            titleRow.addView(TextView(this).apply {
                text = acc.displayName
                textSize = 14f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#233128"))
            })
            titleRow.addView(TextView(this).apply {
                text = if (isParent) "家长端" else "孩子端"
                textSize = 10.5f
                setTextColor(Color.parseColor(if (isParent) "#B8621B" else "#2F6B4F"))
                setPadding(dp(6), dp(1), dp(6), dp(1))
                background = GradientDrawable().apply {
                    setColor(Color.parseColor(if (isParent) "#FFF1DE" else "#E6F4EC"))
                    cornerRadius = dp(99).toFloat()
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { leftMargin = dp(6) }
            })
            infoCol.addView(titleRow)
            infoCol.addView(TextView(this).apply {
                text = "@${acc.username}" + if (acc.password.isNotEmpty()) " · 已记密码" else ""
                textSize = 11.5f
                setTextColor(Color.parseColor("#8A7E6E"))
            })

            val delBtn = TextView(this).apply {
                text = "✕"
                textSize = 14f
                gravity = Gravity.CENTER
                setTextColor(Color.parseColor("#B0A596"))
                setPadding(dp(8), dp(6), dp(8), dp(6))
                setOnClickListener {
                    ServerPrefs.removeSavedAccount(this@MainActivity, acc.username, acc.role)
                    popup.dismiss()
                    refreshSavedAccountPicker(autoFillIfBlank = false)
                }
            }

            row.addView(avatar)
            row.addView(infoCol)
            row.addView(delBtn)
            row.setOnClickListener {
                sound.playPat()
                selectSavedAccountIntoForm(acc)
                binding.tvLoginMsg.text = "已填入「${acc.displayName}」的账号与密码，点击登录即可进入"
                popup.dismiss()
            }
            listCol.addView(row)
        }

        popup.showAsDropDown(binding.layoutSavedAccountPicker, 0, dp(4))
    }

    private fun launchParentPortal(parentDisplayName: String) {
        val intent = Intent(this, ParentPortalActivity::class.java).apply {
            if (parentDisplayName.isNotBlank()) {
                putExtra(ParentPortalActivity.EXTRA_PARENT_NAME, parentDisplayName)
            }
        }
        parentPortalLauncher.launch(intent)
    }

    private fun showAppView() {
        binding.loginContainer.visibility = View.GONE
        binding.appContainer.visibility = View.VISIBLE
        binding.homeSection.visibility = View.VISIBLE
        binding.mainScroll.scrollTo(0, 0)
    }

    private fun performUnifiedLogin() {
        val username = binding.etLoginUser.text.toString().trim()
        val password = binding.etLoginPass.text.toString()
        if (username.isEmpty() || password.isEmpty()) {
            binding.tvLoginMsg.text = "请输入登录账号和密码"
            sound.playError()
            return
        }
        binding.btnDoLogin.isEnabled = false
        binding.btnDoLogin.text = "正在识别身份并登录..."
        binding.tvLoginMsg.text = ""

        val payload = JSONObject().apply {
            put("username", username)
            put("password", password)
        }
        api.request("POST", "/api/auth/login", payload, useAuth = false) { res ->
            binding.btnDoLogin.isEnabled = true
            binding.btnDoLogin.text = "登录进入 →"
            if (res.ok && res.json != null) {
                val role = res.json.optString("role", "")
                if (role == "parent") {
                    onParentLoginSuccess(username, password, res.json)
                } else {
                    onChildLoginSuccess(username, password, res.json)
                }
            } else {
                sound.playError()
                binding.tvLoginMsg.text = res.error ?: "账号或密码不正确"
            }
        }
    }

    private fun onChildLoginSuccess(username: String, password: String, json: JSONObject) {
        val token = json.optString("token", "")
        if (token.isEmpty()) return
        ServerPrefs.clearParentToken(this)
        ServerPrefs.setChildToken(this, token)
        ServerPrefs.saveOrUpdateAccount(
            this,
            SavedAccount(
                username = username,
                password = password,
                role = "child",
                displayName = json.optString("name", username),
                avatarIdx = json.optInt("avatar_idx", 0),
                avatarUrl = json.optString("avatar_url", "")
            )
        )
        binding.etLoginPass.setText("")
        sound.playWin()
        showAppView()
        loadChildState()
    }

    private fun onParentLoginSuccess(username: String, password: String, json: JSONObject) {
        val token = json.optString("token", "")
        if (token.isEmpty()) return
        val pObj = json.optJSONObject("parent")
        val displayName = pObj?.optString("display_name", username)?.ifBlank { username } ?: username
        val avatarIdx = pObj?.optInt("avatar_idx", 0) ?: 0
        val avatarUrl = pObj?.optString("avatar_url", "") ?: ""

        ServerPrefs.clearChildToken(this)
        ServerPrefs.setParentToken(this, token)
        ServerPrefs.saveOrUpdateAccount(
            this,
            SavedAccount(
                username = username,
                password = password,
                role = "parent",
                displayName = displayName,
                avatarIdx = avatarIdx,
                avatarUrl = avatarUrl
            )
        )
        binding.etLoginPass.setText("")
        sound.playWin()
        launchParentPortal(displayName)
    }

    private fun confirmLogout() {
        AlertDialog.Builder(this)
            .setTitle("退出当前账号？")
            .setMessage("退出后可在登录页下拉快速切换其他孩子或家长账号")
            .setPositiveButton("退出登录") { _, _ ->
                if (ServerPrefs.getChildToken(this).isNotEmpty()) {
                    api.request("POST", "/api/auth/logout", JSONObject()) {}
                }
                ServerPrefs.clearChildToken(this)
                ServerPrefs.clearParentToken(this)
                state = null
                binding.etLoginUser.setText("")
                binding.etLoginPass.setText("")
                showLoginView("已退出登录，可在上方下拉选择账号")
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
        if (!binding.mainSwipeRefresh.isRefreshing) {
            binding.topLoading.visibility = View.VISIBLE
        }
        api.request("GET", "/api/child/state?date=${todayStr()}") { res ->
            binding.topLoading.visibility = View.GONE
            binding.mainSwipeRefresh.isRefreshing = false
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
        ServerPrefs.updateAccountProfile(
            this,
            ServerPrefs.getSavedUsername(this),
            "child",
            c.name,
            c.avatarIdx,
            c.avatarUrl
        )
        binding.tvKidBanner.text = "${c.name}的小屋"
        binding.tvFamilySub.text = "${st.family.name} · 点击头像可更换 ✎"
        binding.btnMute.text = if (c.muted) "🔇" else "🔊"

        binding.tvStagePill.text = "Lv.${lv + 1} ${lvSpec.name}"
        binding.tvStreakPill.text = "🌟 连续 ${computeStreakDays(st.logs)} 天"
        when (c.petHealth) {
            "weak" -> {
                binding.tvHealthPill.visibility = View.VISIBLE
                binding.tvHealthPill.text = "💧 虚弱 ${c.healProgress}/${c.healTarget.coerceAtLeast(10)}"
                binding.tvHealthPill.setTextColor(0xFFB86511.toInt())
                binding.tvHealthPill.background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(0xFFFFF4E5.toInt())
                    setStroke(dp(1), 0xFFEDC68C.toInt())
                }
            }
            "sick" -> {
                binding.tvHealthPill.visibility = View.VISIBLE
                binding.tvHealthPill.text = "🤒 生病 ${c.healProgress}/${c.healTarget.coerceAtLeast(20)}"
                binding.tvHealthPill.setTextColor(0xFFC0392B.toInt())
                binding.tvHealthPill.background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(0xFFFDEAEA.toInt())
                    setStroke(dp(1), 0xFFE5A9A9.toInt())
                }
            }
            else -> binding.tvHealthPill.visibility = View.GONE
        }
        binding.petStageView.setPetState(lv, asleep, activeComp, c.petHealth)

        binding.tvBubble.text = when {
            c.petHealth == "sick" -> {
                val left = (c.healTarget.coerceAtLeast(20) - c.healProgress).coerceAtLeast(1)
                "🤒 呜呜，我生病了……还差 $left 点成长值就能康复！"
            }
            c.petHealth == "weak" -> {
                val left = (c.healTarget.coerceAtLeast(10) - c.healProgress).coerceAtLeast(1)
                "💧 我有点虚弱了……再攒 $left 点成长值就能恢复活力！"
            }
            asleep -> "呼呼……已经 3 天没见到你了，喂我一口就醒来。"
            st.logs.isEmpty() -> GrowthMeta.LEVELS[0].talk
            else -> lvSpec.talk
        }
        binding.tvPetName.text = GrowthMeta.resolvePetDisplayName(c, st.catalog)

        val msgBadge = c.unreadChat + c.pendingFriendRequests + c.unreadLetters
        if (msgBadge > 0) {
            binding.tabMessages.text = "💬 消息($msgBadge)"
            binding.tabMessages.setTextColor(0xFFB86511.toInt())
        } else {
            binding.tabMessages.text = "💬 消息"
            binding.tabMessages.setTextColor(0xFF2F6B4F.toInt())
        }

        if (c.coupons > 0) {
            binding.btnOpenDex.text = "📖 图鉴与合影 · ${c.coupons}蛋"
            binding.btnOpenDex.setTextColor(0xFFB86511.toInt())
            binding.btnOpenDex.setBackgroundColor(0xFFFFF7EA.toInt())
        } else {
            binding.btnOpenDex.text = "📖 图鉴与合影"
            binding.btnOpenDex.setTextColor(0xFF2F6B4F.toInt())
            binding.btnOpenDex.setBackgroundColor(0xFFFFFDF9.toInt())
        }

        val totalDays = st.logs.map { it.checkinDate }.toSet().size
        val doneCount = st.tasks.count { st.todayDoneIds.contains(it.id) }
        binding.tvStatsRow.text = "陪伴 $totalDays 天 · 今日已完成 $doneCount/${st.tasks.size} · 成长值 ${c.xp}"
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
            val titleRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
            }
            if (t.iconUrl.isNotEmpty() && !done) {
                val iconIv = ImageView(this).apply {
                    scaleType = ImageView.ScaleType.CENTER_CROP
                    layoutParams = LinearLayout.LayoutParams(dp(26), dp(26)).apply { marginEnd = dp(8) }
                    tag = t.iconUrl
                }
                api.fetchBitmap(t.iconUrl) { bmp ->
                    if (bmp != null && iconIv.tag == t.iconUrl) {
                        iconIv.setImageBitmap(bmp)
                    }
                }
                titleRow.addView(iconIv)
            }
            val titleTv = TextView(this).apply {
                text = if (t.iconUrl.isNotEmpty() && !done) t.label else "${if (done) "✅" else t.emoji} ${t.label}"
                textSize = 15f
                setTypeface(null, Typeface.BOLD)
                setTextColor(if (done) 0xFF2F6B4F.toInt() else 0xFF2B2620.toInt())
            }
            titleRow.addView(titleTv)
            leftCol.addView(titleRow)

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
                text = when {
                    done -> if (!photoUrl.isNullOrEmpty()) "📷 查看" else "已领"
                    !isRope && t.timerMode == "none" && !t.requirePhoto -> "✓ 打卡 +${t.pts}"
                    else -> "+${t.pts}"
                }
                textSize = 13f
                setTypeface(null, Typeface.BOLD)
                setTextColor(
                    when {
                        done -> 0xFF2F6B4F.toInt()
                        !isRope && t.timerMode == "none" && !t.requirePhoto -> Color.WHITE
                        else -> 0xFFC56C14.toInt()
                    }
                )
                setPadding(dp(12), dp(5), dp(12), dp(5))
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    when {
                        done -> setColor(Color.WHITE)
                        !isRope && t.timerMode == "none" && !t.requirePhoto -> setColor(0xFF2F6B4F.toInt())
                        else -> setColor(0xFFFDF0DF.toInt())
                    }
                }
            }

            card.addView(leftCol)
            card.addView(ptsTv)

            if (!done) {
                val launchTaskFlow = {
                    when {
                        isRope -> launchRopeChallenge(t)
                        t.timerMode == "countdown" || t.timerMode == "countup" -> launchTaskFocus(t)
                        t.requirePhoto -> showCheckinConfirmDialog(t, null, null)
                        else -> {
                            AlertDialog.Builder(this)
                                .setTitle("确认完成任务？")
                                .setMessage("已完成「${t.label}」，现在喂小伙伴并领取 +${t.pts} 成长值吗？")
                                .setPositiveButton("✓ 确认打卡") { _, _ -> submitCheckin(t.id, null, null, null) }
                                .setNegativeButton("再想想", null)
                                .show()
                        }
                    }
                }
                if (!isRope && t.timerMode == "none" && !t.requirePhoto) {
                    ptsTv.setOnClickListener { submitCheckin(t.id, null, null, null) }
                }
                card.setOnClickListener { launchTaskFlow() }
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
        val ropeLogs = (state?.logs ?: emptyList()).filter { (it.actualCount ?: 0) > 0 }
        val prevBestCount = ropeLogs
            .filter { it.actualDuration == null || it.actualDuration == t.durationSec }
            .maxOfOrNull { it.actualCount ?: 0 }
            ?: ropeLogs.maxOfOrNull { it.actualCount ?: 0 }
            ?: 0

        val taskJson = JSONObject().apply {
            put("taskId", t.id)
            put("label", t.label)
            put("durationSec", t.durationSec)
            put("targetCount", t.targetCount)
            put("childName", state?.child?.name ?: "")
            put("bestCount", prevBestCount)
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
                        val healUsed = res.json.optInt("healUsed", 0)
                        val grownXp = res.json.optInt("grownXp", res.json.optInt("pts", 0))
                        val tip = when {
                            healUsed > 0 && grownXp == 0 ->
                                "💚 已用 $healUsed 点成长值帮小伙伴恢复健康$photoNote"
                            healUsed > 0 && grownXp > 0 ->
                                "💚 小伙伴已恢复健康！剩余 +$grownXp 用于成长$photoNote"
                            else ->
                                "打卡成功！成长值 +${res.json.optInt("pts", 0)}$photoNote"
                        }
                        Toast.makeText(this, tip, Toast.LENGTH_SHORT).show()
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
        binding.btnUndo.isEnabled = false
        api.request("POST", "/api/child/undo", JSONObject().put("date", todayStr())) { res ->
            binding.btnUndo.isEnabled = true
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
        api.request("PATCH", "/api/child/mute", JSONObject().put("muted", nextMute)) { }
    }

    private fun populateQuickChips(container: LinearLayout, items: List<String>, onPick: (String) -> Unit) {
        container.removeAllViews()
        items.forEach { txt ->
            val tv = TextView(this).apply {
                text = txt
                textSize = 12f
                setTextColor(0xFF2F6B4F.toInt())
                setPadding(dp(10), dp(5), dp(10), dp(5))
                background = GradientDrawable().apply {
                    setColor(0xFFF3FAF6.toInt())
                    cornerRadius = dp(14).toFloat()
                    setStroke(dp(1), 0xFFC5DDD0.toInt())
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { rightMargin = dp(6) }
                setOnClickListener { onPick(txt) }
            }
            container.addView(tv)
        }
    }

    private fun showRenamePetDialog() {
        val wrap = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(12), dp(20), dp(8))
        }
        val tip = TextView(this).apply {
            text = "点选推荐名字（或手动修改）："
            textSize = 12.5f
            setTextColor(0xFF6E655A.toInt())
            setPadding(0, 0, 0, dp(6))
        }
        val row1 = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        val row2 = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            setPadding(0, dp(6), 0, dp(10))
        }
        val curState = state
        val defaultPetName = curState?.let {
            GrowthMeta.resolvePetDisplayName(it.child.copy(petName = ""), it.catalog)
        } ?: "小团子"
        val input = EditText(this).apply {
            hint = "留空默认：$defaultPetName（最多 10 字）"
            setText(curState?.child?.petName ?: "")
            setPadding(dp(12), dp(10), dp(12), dp(10))
        }
        populateQuickChips(row1, listOf("团子", "糯米", "豆包", "可乐")) { name ->
            sound.playPat()
            input.setText(name)
            input.setSelection(name.length)
        }
        populateQuickChips(row2, listOf("布丁", "皮皮", "雪球", "元宝")) { name ->
            sound.playPat()
            input.setText(name)
            input.setSelection(name.length)
        }
        wrap.addView(tip)
        wrap.addView(row1)
        wrap.addView(row2)
        wrap.addView(input)

        AlertDialog.Builder(this)
            .setTitle("给小伙伴起个名字")
            .setView(wrap)
            .setPositiveButton("保存") { _, _ ->
                val name = input.text.toString().trim().take(10)
                api.request("PATCH", "/api/child/pet-name", JSONObject().put("petName", name)) { res ->
                    if (res.ok) {
                        val updatedState = state?.let { s ->
                            s.copy(child = s.child.copy(petName = name))
                        }
                        if (updatedState != null) {
                            state = updatedState
                            binding.tvPetName.text = GrowthMeta.resolvePetDisplayName(updatedState.child, updatedState.catalog)
                        } else {
                            binding.tvPetName.text = name.ifEmpty { defaultPetName }
                        }
                        sound.playPat()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
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
            setOnClickListener {
                showAvatarPreviewDialog(api, "${c.name} 的头像", c.avatarIdx, c.avatarUrl)
            }
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

        // 版本与更新入口（无需退出登录即可查看更新记录与检查新版）
        root.addView(TextView(this).apply {
            text = "关于与版本（当前 ${BuildConfig.VERSION_NAME}）"
            textSize = 12.5f
            setTextColor(0xFF7D7263.toInt())
            setPadding(0, dp(10), 0, dp(8))
        })
        val verRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        }
        verRow.addView(Button(this).apply {
            text = "🔄 检查更新"
            textSize = 12.5f
            setAllCaps(false)
            setTextColor(0xFF2F6B4F.toInt())
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFFE3F2E8.toInt())
                setStroke(dp(1), 0xFFB8D4C3.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(0, dp(40), 1f).apply { marginEnd = dp(6) }
            setOnClickListener {
                sound.playPat()
                dialogRef?.dismiss()
                updater.checkForUpdate(silent = false)
            }
        })
        verRow.addView(Button(this).apply {
            text = "📋 更新记录"
            textSize = 12.5f
            setAllCaps(false)
            setTextColor(0xFF8A5A2B.toInt())
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(0xFFFFF4D6.toInt())
                setStroke(dp(1), 0xFFE6C994.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(0, dp(40), 1f).apply { marginStart = dp(6) }
            setOnClickListener {
                sound.playPat()
                dialogRef?.dismiss()
                showChangelogDialog()
            }
        })
        root.addView(verRow)

        val scroll = ScrollView(this).apply { addView(root) }
        dialogRef = AlertDialog.Builder(this)
            .setTitle("个人头像与版本设置")
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
