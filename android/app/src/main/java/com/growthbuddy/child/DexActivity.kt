package com.growthbuddy.child

import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityDexBinding
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 伙伴图鉴与拍立得纪念二级页（从「🐾 小屋」宠窝卡片的「📖 图鉴与合影」入口进入）：
 * - 领养神秘蛋 / 富余成长值兑换伙伴、切换陪伴伙伴；
 * - 拍立得今日合影留念与纪念墙放大预览。
 */
class DexActivity : AppCompatActivity() {

    private lateinit var binding: ActivityDexBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private var state: ChildState? = null
    private var hasChangedState = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityDexBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)

        binding.btnDexBack.setOnClickListener {
            if (hasChangedState) setResult(RESULT_OK)
            finish()
        }
        binding.btnDexRefresh.setOnClickListener { loadState() }
        binding.btnMysteryEgg.setOnClickListener { performAdopt(speciesId = null, mystery = true) }
        binding.btnSnap.setOnClickListener { performTakePolaroid() }

        populateMoodChips()
        applyResponsiveLayout()
        loadState()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
        state?.let {
            renderDex(it)
            renderPolaroids(it)
        }
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 1140 else if (m.isTablet) 760 else 1200
        ResponsiveUi.applyCenteredMaxWidth(this, binding.dexTopBar, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.dexSection, maxW, horizontalReserveDp = 28)
    }

    private fun todayStr(): String = SimpleDateFormat("yyyy-MM-dd", Locale.US).format(Date())

    private fun loadState() {
        binding.dexLoading.visibility = View.VISIBLE
        api.request("GET", "/api/child/state?date=${todayStr()}") { res ->
            binding.dexLoading.visibility = View.GONE
            if (res.ok && res.json != null) {
                val parsed = GrowthMeta.parseChildState(res.json)
                state = parsed
                renderDex(parsed)
                renderPolaroids(parsed)
            } else {
                Toast.makeText(this, res.error ?: "加载图鉴失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun populateMoodChips() {
        binding.snapMoodRow.removeAllViews()
        listOf("😄 今天超开心！", "💪 任务全部完成", "🌟 我又进步啦", "❤️ 最爱小伙伴").forEach { item ->
            val chip = Button(this).apply {
                text = item
                textSize = 12f
                setAllCaps(false)
                setTextColor(0xFF4B4339.toInt())
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(0xFFFFF8EC.toInt())
                    setStroke(dp(1), 0xFFE5D5BA.toInt())
                }
                setPadding(dp(10), 0, dp(10), 0)
                minWidth = 0
                minimumWidth = 0
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    dp(32)
                ).apply { rightMargin = dp(6) }
                setOnClickListener {
                    sound.playPat()
                    binding.etSnapNote.setText(item)
                }
            }
            binding.snapMoodRow.addView(chip)
        }
    }

    private fun renderDex(st: ChildState) {
        val c = st.child
        val matureXp = 200
        val surplus = (c.xp - matureXp).coerceAtLeast(0)
        binding.tvCouponTitle.text = if (c.xp < matureXp) {
            "成长值：${c.xp}/$matureXp（主宠成熟后，富余每 25 点可换宠物蛋）"
        } else {
            "主宠已成熟！富余成长值：$surplus 点（每 25 点可直兑 1 只）"
        } + if (c.coupons > 0) " · 领养蛋 ${c.coupons} 枚" else ""
        binding.dexContainer.removeAllViews()

        val cols = ResponsiveUi.gridColumns(this, phoneCols = 2, tabletPortCols = 3, tabletLandCols = 4)
        for (rowItems in st.catalog.chunked(cols)) {
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(10) }
            }
            for (idx in 0 until cols) {
                val sp = rowItems.getOrNull(idx)
                if (sp == null) {
                    row.addView(View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply {
                            if (idx > 0) marginStart = dp(10)
                        }
                    })
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
                        if (idx > 0) marginStart = dp(10)
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
                        surplus >= 25 -> {
                            text = "✨ 25 富余成长值兑换"
                            setBackgroundColor(0xFFE8913A.toInt())
                            setTextColor(Color.WHITE)
                            setOnClickListener { performAdopt(sp.id, mystery = false) }
                        }
                        c.xp < matureXp -> {
                            text = "主宠成熟后可兑换"
                            isEnabled = false
                            setBackgroundColor(0xFFEDE6DA.toInt())
                            setTextColor(0xFF8A7E6E.toInt())
                        }
                        else -> {
                            text = "富余还差 ${25 - surplus} 点"
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
                hasChangedState = true
                setResult(RESULT_OK)
                sound.playWin()
                val adoptedName = res.json.optJSONObject("adopted")?.optString("name", "新伙伴") ?: "新伙伴"
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderDex(ns)
                    renderPolaroids(ns)
                } ?: loadState()
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
                hasChangedState = true
                setResult(RESULT_OK)
                sound.playPat()
                loadState()
                Toast.makeText(this, "已更换陪伴伙伴！", Toast.LENGTH_SHORT).show()
            } else {
                Toast.makeText(this, res.error ?: "设置失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun renderPolaroids(st: ChildState) {
        val c = st.child
        binding.tvWallCount.text = "${st.polaroids.size} 张合影"

        binding.avatarPickRow.removeAllViews()
        val curAv = SpeciesThumbView(this).apply {
            loadAvatar(api, c.avatarIdx, c.avatarUrl)
            layoutParams = LinearLayout.LayoutParams(dp(42), dp(42))
            setOnClickListener { showAvatarPreviewDialog(api, "${c.name} 的头像", c.avatarIdx, c.avatarUrl) }
        }
        val infoTv = TextView(this).apply {
            text = "拍立得将使用当前头像（如需更换请回小屋点左上角头像）"
            textSize = 12f
            setTextColor(0xFF7D7263.toInt())
            setPadding(dp(10), 0, 0, 0)
            layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
        }
        binding.avatarPickRow.addView(curAv)
        binding.avatarPickRow.addView(infoTv)

        binding.wallContainer.removeAllViews()
        if (st.polaroids.isEmpty()) {
            binding.wallContainer.addView(TextView(this).apply {
                text = "还没有合影，快拍下第一张拍立得留念吧！"
                setTextColor(0xFF8A7E6E.toInt())
                gravity = Gravity.CENTER
                setPadding(dp(8), dp(14), dp(8), dp(14))
            })
            return
        }

        val wallCols = ResponsiveUi.gridColumns(this, phoneCols = 2, tabletPortCols = 3, tabletLandCols = 4)
        for (rowItems in st.polaroids.chunked(wallCols)) {
            val row = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(10) }
            }
            for (idx in 0 until wallCols) {
                val p = rowItems.getOrNull(idx)
                if (p == null) {
                    row.addView(View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply {
                            if (idx > 0) marginStart = dp(10)
                        }
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
                        if (idx > 0) marginStart = dp(10)
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
                    loadAvatar(api, p.avatarIdx, p.avatarUrl)
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
                    frame.addView(TextView(this).apply {
                        text = p.note
                        textSize = 11f
                        setTextColor(0xFF8A7E6E.toInt())
                    })
                }
                frame.setOnClickListener {
                    showPolaroidPreviewDialog(p, st.catalog)
                }
                row.addView(frame)
            }
            binding.wallContainer.addView(row)
        }
    }

    private fun showPolaroidPreviewDialog(p: PolaroidItem, catalog: List<SpeciesItem>) {
        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(dp(18), dp(16), dp(18), dp(12))
        }
        val frame = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(dp(12), dp(12), dp(12), dp(14))
            background = GradientDrawable().apply {
                cornerRadius = dp(12).toFloat()
                setColor(Color.WHITE)
                setStroke(dp(1), 0xFFE5DCCB.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(dp(276), LinearLayout.LayoutParams.WRAP_CONTENT)
        }
        val photoBox = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER
            setPadding(dp(8), dp(14), dp(8), dp(12))
            background = GradientDrawable().apply {
                cornerRadius = dp(8).toFloat()
                setColor(0xFFFBF2E3.toInt())
            }
            layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(196))
        }
        val kidView = SpeciesThumbView(this).apply {
            loadAvatar(api, p.avatarIdx, p.avatarUrl)
            layoutParams = LinearLayout.LayoutParams(dp(108), dp(108))
            setOnClickListener {
                showAvatarPreviewDialog(api, "合影中的头像", p.avatarIdx, p.avatarUrl)
            }
        }
        val petView = SpeciesThumbView(this).apply {
            val sp = catalog.find { it.id == p.petRef }
            if (sp != null) setSpecies(sp, isSilhouette = false)
            else setMainPet(p.petLv)
            layoutParams = LinearLayout.LayoutParams(dp(132), dp(124))
        }
        photoBox.addView(kidView)
        photoBox.addView(petView)
        frame.addView(photoBox)

        frame.addView(TextView(this).apply {
            text = "📅 ${p.photoDate}"
            textSize = 14f
            setTypeface(null, Typeface.BOLD)
            setTextColor(0xFF5C5347.toInt())
            setPadding(0, dp(10), 0, 0)
        })
        if (p.note.isNotEmpty()) {
            frame.addView(TextView(this).apply {
                text = p.note
                textSize = 13f
                setTextColor(0xFF8A7E6E.toInt())
                setPadding(0, dp(4), 0, 0)
            })
        }
        root.addView(frame)

        AlertDialog.Builder(this)
            .setTitle("📸 纪念墙合影放大预览")
            .setView(root)
            .setPositiveButton("关闭", null)
            .show()
    }

    private fun performTakePolaroid() {
        val payload = JSONObject().apply {
            put("note", binding.etSnapNote.text.toString().trim().take(16))
            put("date", todayStr())
        }
        api.request("POST", "/api/child/polaroid", payload) { res ->
            if (res.ok && res.json != null) {
                hasChangedState = true
                setResult(RESULT_OK)
                sound.playWin()
                binding.etSnapNote.setText("")
                res.json.optJSONObject("state")?.let {
                    val ns = GrowthMeta.parseChildState(it)
                    state = ns
                    renderDex(ns)
                    renderPolaroids(ns)
                } ?: loadState()
                Toast.makeText(this, "📷 今日合影已珍藏到纪念墙！", Toast.LENGTH_SHORT).show()
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "拍照失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun dp(v: Int): Int = ResponsiveUi.dp(this, v)

    override fun onDestroy() {
        sound.release()
        api.shutdown()
        super.onDestroy()
    }
}
