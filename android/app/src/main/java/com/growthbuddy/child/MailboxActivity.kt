package com.growthbuddy.child

import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityMailboxBinding
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

data class LetterItem(
    val id: String,
    val senderRole: String,
    val senderName: String,
    val stamp: String,
    val title: String,
    val content: String,
    val bonusXp: Int,
    val isRead: Boolean,
    val createdAt: Long
)

/**
 * 成长信箱（家长 ↔ 孩子双向写信互动）：
 * - 孩子可以查看爸爸妈妈寄来的表扬信/鼓励卡，点击拆信并领取附带成长值礼物；
 * - 也可以挑选可爱贴纸与快捷暖心语，给爸爸妈妈写信寄回家长端。
 */
class MailboxActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMailboxBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private var hasChangedState = false
    private var cachedLetters: List<LetterItem> = emptyList()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMailboxBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)

        binding.btnMailBack.setOnClickListener {
            if (hasChangedState) setResult(RESULT_OK)
            finish()
        }
        binding.btnWriteLetter.setOnClickListener {
            sound.playPat()
            showComposeDialog()
        }

        applyResponsiveLayout()
        loadLetters()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
        if (cachedLetters.isNotEmpty()) {
            renderLetters(cachedLetters)
        }
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 1040 else if (m.isTablet) 740 else 1200
        ResponsiveUi.applyCenteredMaxWidth(this, binding.mailTopBar, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.tvMailHint, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.mailScroll, maxW, horizontalReserveDp = 28)
    }

    private fun loadLetters() {
        binding.mailLoading.visibility = View.VISIBLE
        api.request("GET", "/api/child/letters") { res ->
            binding.mailLoading.visibility = View.GONE
            if (!res.ok || res.json == null) {
                Toast.makeText(this, res.error ?: "读取信箱失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val unread = res.json.optInt("unread_count", 0)
            binding.tvMailHint.text = if (unread > 0) {
                "📬 收到 $unread 封来自爸爸妈妈的新信件！点击信封拆开看看吧~"
            } else {
                "💌 爸爸妈妈寄来的鼓励信和你的小回信都会保存在这里~"
            }
            val arr = res.json.optJSONArray("letters")
            val list = mutableListOf<LetterItem>()
            if (arr != null) {
                for (i in 0 until arr.length()) {
                    val o = arr.getJSONObject(i)
                    list.add(
                        LetterItem(
                            id = o.optString("id", ""),
                            senderRole = o.optString("sender_role", "parent"),
                            senderName = o.optString("sender_name", "爸爸妈妈"),
                            stamp = o.optString("stamp", "💌"),
                            title = o.optString("title", "一封信"),
                            content = o.optString("content", ""),
                            bonusXp = o.optInt("bonus_xp", 0),
                            isRead = o.optBoolean("is_read", false),
                            createdAt = o.optLong("created_at", 0L)
                        )
                    )
                }
            }
            renderLetters(list)
        }
    }

    private fun renderLetters(list: List<LetterItem>) {
        cachedLetters = list
        binding.lettersContainer.removeAllViews()
        if (list.isEmpty()) {
            val emptyTv = TextView(this).apply {
                text = "📭 信箱还是空的哦~\n点击右上角「✉️ 给爸妈写信」，给爸爸妈妈一个惊喜吧！"
                gravity = Gravity.CENTER
                textSize = 14.5f
                setTextColor(Color.parseColor("#8A7E6E"))
                setPadding(dp(16), dp(48), dp(16), dp(48))
                setLineSpacing(8f, 1.2f)
            }
            binding.lettersContainer.addView(emptyTv)
            return
        }

        val cols = if (ResponsiveUi.metrics(this).isDesktopSplit) 2 else 1
        val fmt = SimpleDateFormat("MM-dd HH:mm", Locale.getDefault())
        for (rowItems in list.chunked(cols)) {
            val row = if (cols > 1) {
                LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { bottomMargin = dp(10) }
                }
            } else null

            for (idx in 0 until cols) {
                val item = rowItems.getOrNull(idx)
                if (item == null) {
                    row?.addView(View(this).apply {
                        layoutParams = LinearLayout.LayoutParams(0, 1, 1f).apply {
                            if (idx > 0) marginStart = dp(12)
                        }
                    })
                    continue
                }
                val isParent = item.senderRole == "parent"
                val unreadParent = isParent && !item.isRead
                val card = LinearLayout(this).apply {
                    orientation = LinearLayout.VERTICAL
                    setPadding(dp(14), dp(12), dp(14), dp(12))
                    background = GradientDrawable().apply {
                        setColor(
                            Color.parseColor(
                                when {
                                    unreadParent -> "#FFFDF4"
                                    isParent -> "#F3F9F5"
                                    else -> "#FFFFFF"
                                }
                            )
                        )
                        cornerRadius = dp(14).toFloat()
                        setStroke(
                            dp(if (unreadParent) 2 else 1),
                            Color.parseColor(if (unreadParent) "#E8913A" else if (isParent) "#C5DFCD" else "#E5DCCB")
                        )
                    }
                    layoutParams = if (cols > 1) {
                        LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.MATCH_PARENT, 1f).apply {
                            if (idx > 0) marginStart = dp(12)
                        }
                    } else {
                        LinearLayout.LayoutParams(
                            LinearLayout.LayoutParams.MATCH_PARENT,
                            LinearLayout.LayoutParams.WRAP_CONTENT
                        ).apply { bottomMargin = dp(10) }
                    }
                }

                // 头部：邮票 + 标题 + 未读/礼物标签
                val headerRow = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                }
                val titleTv = TextView(this).apply {
                    text = "${item.stamp} ${item.title}"
                    textSize = 15.5f
                    setTypeface(null, Typeface.BOLD)
                    setTextColor(Color.parseColor("#233128"))
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                }
                val badgeTv = TextView(this).apply {
                    textSize = 11.5f
                    setPadding(dp(8), dp(3), dp(8), dp(3))
                    when {
                        unreadParent && item.bonusXp > 0 -> {
                            text = "🎁 新信件 +${item.bonusXp}XP"
                            setTextColor(Color.WHITE)
                            background = GradientDrawable().apply {
                                setColor(Color.parseColor("#E8913A"))
                                cornerRadius = dp(99).toFloat()
                            }
                        }
                        unreadParent -> {
                            text = "✨ 未拆封"
                            setTextColor(Color.WHITE)
                            background = GradientDrawable().apply {
                                setColor(Color.parseColor("#E11D48"))
                                cornerRadius = dp(99).toFloat()
                            }
                        }
                        isParent -> {
                            text = if (item.bonusXp > 0) "已领 +${item.bonusXp}XP" else "已读"
                            setTextColor(Color.parseColor("#2F6B4F"))
                            background = GradientDrawable().apply {
                                setColor(Color.parseColor("#E3F2E8"))
                                cornerRadius = dp(99).toFloat()
                            }
                        }
                        else -> {
                            text = if (item.isRead) "爸妈已读 ✓" else "已寄给爸妈"
                            setTextColor(Color.parseColor("#8A5A2B"))
                            background = GradientDrawable().apply {
                                setColor(Color.parseColor("#FFF4DF"))
                                cornerRadius = dp(99).toFloat()
                            }
                        }
                    }
                }
                headerRow.addView(titleTv)
                headerRow.addView(badgeTv)

                val metaTv = TextView(this).apply {
                    val timeStr = if (item.createdAt > 0) fmt.format(Date(item.createdAt)) else ""
                    text = if (isParent) "来自：${item.senderName} · $timeStr" else "我寄给爸爸妈妈 · $timeStr"
                    textSize = 12f
                    setTextColor(Color.parseColor("#8A7E6E"))
                    setPadding(0, dp(3), 0, dp(6))
                }

                val bodyTv = TextView(this).apply {
                    text = if (unreadParent) "点击拆开这封来自${item.senderName}的信件..." else item.content
                    textSize = 14f
                    setTextColor(Color.parseColor(if (unreadParent) "#B8621B" else "#3D372E"))
                    if (unreadParent) setTypeface(null, Typeface.ITALIC)
                    setLineSpacing(6f, 1.15f)
                }

                card.addView(headerRow)
                card.addView(metaTv)
                card.addView(bodyTv)

                if (isParent) {
                    card.setOnClickListener { openParentLetterDialog(item) }
                }
                if (row != null) row.addView(card) else binding.lettersContainer.addView(card)
            }
            if (row != null) binding.lettersContainer.addView(row)
        }
    }

    private fun openParentLetterDialog(item: LetterItem) {
        if (!item.isRead) {
            api.request("POST", "/api/child/letters/read", JSONObject().put("letterId", item.id)) { res ->
                if (res.ok && res.json != null) {
                    hasChangedState = true
                    setResult(RESULT_OK)
                    val claimed = res.json.optInt("claimed_xp", 0)
                    if (claimed > 0) {
                        sound.playVictoryFanfare()
                        Toast.makeText(this, "🎁 拆信收到爸爸妈妈送你的 +$claimed 成长值！", Toast.LENGTH_LONG).show()
                    } else {
                        sound.playPat()
                    }
                    loadLetters()
                }
            }
        } else {
            sound.playPat()
        }

        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(18), dp(20), dp(12))
            background = GradientDrawable().apply {
                setColor(Color.parseColor("#FFFDF7"))
                cornerRadius = dp(18).toFloat()
                setStroke(dp(2), Color.parseColor("#E6C994"))
            }
        }
        box.addView(TextView(this).apply {
            text = "${item.stamp} ${item.title}"
            textSize = 18f
            setTypeface(null, Typeface.BOLD)
            setTextColor(Color.parseColor("#233128"))
        })
        box.addView(TextView(this).apply {
            text = "寄信人：${item.senderName}" + if (item.bonusXp > 0) " · 🎁 附带 +${item.bonusXp} 成长值" else ""
            textSize = 12.5f
            setTextColor(Color.parseColor("#2F6B4F"))
            setPadding(0, dp(4), 0, dp(10))
        })
        box.addView(TextView(this).apply {
            text = item.content
            textSize = 15.5f
            setTextColor(Color.parseColor("#2B2620"))
            setLineSpacing(8f, 1.25f)
        })

        AlertDialog.Builder(this)
            .setView(box)
            .setPositiveButton("✉️ 给爸妈回一封") { _, _ ->
                showComposeDialog(replyTitle = "回复 ${item.senderName}：${item.title}")
            }
            .setNegativeButton("收好信件", null)
            .show()
    }

    private fun showComposeDialog(replyTitle: String = "") {
        val wrap = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(18), dp(14), dp(18), dp(8))
        }

        val stamps = listOf("🌟 想说的话", "💖 谢谢爸妈", "🎯 我的目标", "🙏 小小心愿")
        var chosenStamp = "🌟"
        val stampRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
        }
        val stampBtns = mutableListOf<Button>()
        stamps.forEach { s ->
            val icon = s.substringBefore(" ")
            val b = Button(this).apply {
                text = icon
                textSize = 18f
                minWidth = 0
                setPadding(dp(8), dp(4), dp(8), dp(4))
                layoutParams = LinearLayout.LayoutParams(0, dp(42), 1f).apply { rightMargin = dp(6) }
            }
            stampBtns.add(b)
            stampRow.addView(b)
        }
        fun refreshStampUi() {
            stampBtns.forEach { b ->
                val active = b.text.toString() == chosenStamp
                b.setBackgroundColor(Color.parseColor(if (active) "#E8913A" else "#EFE8DA"))
            }
        }
        stampBtns.forEach { b ->
            b.setOnClickListener {
                chosenStamp = b.text.toString()
                refreshStampUi()
            }
        }
        refreshStampUi()

        val etTitle = EditText(this).apply {
            hint = "标题（可点下方快捷选项免输入）"
            setText(replyTitle)
            setSingleLine(true)
            textSize = 14.5f
        }
        val titleChipScroll = android.widget.HorizontalScrollView(this).apply {
            isHorizontalScrollBarEnabled = false
            setPadding(0, dp(2), 0, dp(4))
        }
        val titleChipRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        listOf("💌 给爸妈的信", "🌟 我想说谢谢", "🎈 我的小愿望").forEach { tPreset ->
            titleChipRow.addView(Button(this).apply {
                text = tPreset
                textSize = 11.5f
                minWidth = 0
                setPadding(dp(8), dp(2), dp(8), dp(2))
                setBackgroundColor(Color.parseColor("#FFF4D6"))
                setTextColor(Color.parseColor("#9C5812"))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    dp(32)
                ).apply { rightMargin = dp(6) }
                setOnClickListener {
                    etTitle.setText(tPreset)
                    etTitle.setSelection(tPreset.length)
                }
            })
        }
        titleChipScroll.addView(titleChipRow)

        val etBody = EditText(this).apply {
            hint = "写下你想对爸爸妈妈说的话，或点下方快捷句~"
            minLines = 4
            gravity = Gravity.TOP or Gravity.START
            textSize = 14.5f
        }

        // 快捷暖心句（横向滚动多条，免手打）
        val quickScroll = android.widget.HorizontalScrollView(this).apply {
            isHorizontalScrollBarEnabled = false
            setPadding(0, dp(4), 0, dp(6))
        }
        val quickRow = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
        listOf(
            "今天我表现超棒！",
            "谢谢爸爸妈妈，我爱你们！",
            "我会按时完成作业的！",
            "周末想和你们一起玩！"
        ).forEach { tpl ->
            quickRow.addView(Button(this).apply {
                text = tpl
                textSize = 11.5f
                minWidth = 0
                setPadding(dp(8), dp(2), dp(8), dp(2))
                setBackgroundColor(Color.parseColor("#E8F2EC"))
                setTextColor(Color.parseColor("#2F6B4F"))
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.WRAP_CONTENT,
                    dp(34)
                ).apply { rightMargin = dp(6) }
                setOnClickListener {
                    val cur = etBody.text.toString().trim()
                    etBody.setText(if (cur.isEmpty()) tpl else "$cur\n$tpl")
                    etBody.setSelection(etBody.text.length)
                }
            })
        }
        quickScroll.addView(quickRow)

        wrap.addView(TextView(this).apply {
            text = "选一张贴纸邮票："
            textSize = 13f
            setTextColor(Color.parseColor("#5C5347"))
            setPadding(0, 0, 0, dp(6))
        })
        wrap.addView(stampRow)
        wrap.addView(etTitle)
        wrap.addView(titleChipScroll)
        wrap.addView(quickScroll)
        wrap.addView(etBody)

        AlertDialog.Builder(this)
            .setTitle("✉️ 写信给爸爸妈妈")
            .setView(wrap)
            .setPositiveButton("📮 寄出信件") { _, _ ->
                val content = etBody.text.toString().trim()
                if (content.isEmpty()) {
                    Toast.makeText(this, "请写一点内容再寄出哦~", Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }
                val payload = JSONObject().apply {
                    put("stamp", chosenStamp)
                    put("title", etTitle.text.toString().trim())
                    put("content", content)
                }
                api.request("POST", "/api/child/letters", payload) { res ->
                    if (res.ok) {
                        hasChangedState = true
                        setResult(RESULT_OK)
                        sound.playFeed()
                        Toast.makeText(this, "📮 信件已经投递给爸爸妈妈啦！", Toast.LENGTH_SHORT).show()
                        loadLetters()
                    } else {
                        Toast.makeText(this, res.error ?: "寄信失败", Toast.LENGTH_SHORT).show()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun dp(v: Int): Int = (v * resources.displayMetrics.density).toInt()

    override fun onDestroy() {
        sound.release()
        super.onDestroy()
    }
}
