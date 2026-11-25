package com.growthbuddy.child

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.text.TextUtils
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.Button
import android.widget.CheckBox
import android.widget.EditText
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityFriendsBinding
import org.json.JSONArray
import org.json.JSONObject
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 好友与聊天列表中心：
 * - 同家庭成员（父母与同家庭小伙伴）直接进入「家庭」分组；
 * - 另默认拥有「朋友」分组，孩子可通过二维码扫码、好友分享码、或对方账号查找并发送申请，需对方同意后建立好友关系。
 */
class FriendsActivity : AppCompatActivity() {

    private lateinit var binding: ActivityFriendsBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine

    private var overview: SocialOverview? = null
    private var groupFilter: String = "all" // all | 家庭 | 朋友
    private var hasChangedState = false

    private val qrScanLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { res ->
        if (res.resultCode == RESULT_OK) {
            val raw = res.data?.getStringExtra(QrScanActivity.EXTRA_QR_RESULT)?.trim().orEmpty()
            if (raw.isNotEmpty()) {
                sound.playPat()
                lookupAndConfirmAddFriend(raw)
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityFriendsBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)

        binding.btnFriendsBack.setOnClickListener {
            if (hasChangedState) setResult(RESULT_OK)
            finish()
        }
        binding.btnMyCard.setOnClickListener { showMyQrCardDialog() }
        binding.btnAddFriend.setOnClickListener { showAddFriendChoiceDialog() }

        binding.btnGroupAll.setOnClickListener { selectGroup("all") }
        binding.btnGroupChats.setOnClickListener { selectGroup("群聊") }
        binding.btnGroupFamily.setOnClickListener { selectGroup("家庭") }
        binding.btnGroupFriends.setOnClickListener { selectGroup("朋友") }

        applyResponsiveLayout()
        loadOverview()
    }

    override fun onResume() {
        super.onResume()
        loadOverview()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun dp(v: Int): Int = ResponsiveUi.dp(this, v)

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 1140 else if (m.isTablet) 740 else 1200
        binding.tvFriendsTitle.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, if (m.isTablet) 18f else 15.5f)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.friendsTopBar, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.friendsScroll, maxW, horizontalReserveDp = 28)

        val leftLp = binding.friendsLeftPane.layoutParams as LinearLayout.LayoutParams
        val rightLp = binding.friendsRightPane.layoutParams as LinearLayout.LayoutParams
        val hasReqs = !overview?.incomingRequests.isNullOrEmpty()
        if (m.isDesktopSplit && hasReqs) {
            binding.friendsBodySplit.orientation = LinearLayout.HORIZONTAL
            binding.friendsLeftPane.visibility = View.VISIBLE
            leftLp.width = 0
            leftLp.weight = 0.85f
            rightLp.width = 0
            rightLp.weight = 1.15f
            rightLp.topMargin = 0
            rightLp.marginStart = dp(16)
        } else {
            binding.friendsBodySplit.orientation = LinearLayout.VERTICAL
            binding.friendsLeftPane.visibility = if (hasReqs) View.VISIBLE else View.GONE
            leftLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            leftLp.weight = 0f
            rightLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            rightLp.weight = 0f
            rightLp.topMargin = 0
            rightLp.marginStart = 0
        }
        binding.friendsLeftPane.layoutParams = leftLp
        binding.friendsRightPane.layoutParams = rightLp
    }

    private fun selectGroup(g: String) {
        groupFilter = g
        val green = Color.parseColor("#2F6B4F")
        val muted = Color.parseColor("#5C5347")
        binding.btnGroupAll.setTextColor(if (g == "all") green else muted)
        binding.btnGroupChats.setTextColor(if (g == "群聊") green else muted)
        binding.btnGroupFamily.setTextColor(if (g == "家庭") green else muted)
        binding.btnGroupFriends.setTextColor(if (g == "朋友") green else muted)
        overview?.let { renderFriendsList(it) }
    }

    private fun loadOverview() {
        binding.friendsLoading.visibility = View.VISIBLE
        api.request("GET", "/api/child/friends") { res ->
            binding.friendsLoading.visibility = View.GONE
            if (!res.ok || res.json == null) {
                Toast.makeText(this, res.error ?: "读取好友列表失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val parsed = SocialParser.parseOverview(res.json)
            overview = parsed
            renderRequests(parsed.incomingRequests)
            applyResponsiveLayout()
            renderFriendsList(parsed)
        }
    }

    private fun renderRequests(reqs: List<FriendRequestItem>) {
        binding.requestsContainer.removeAllViews()
        if (reqs.isEmpty()) {
            binding.requestsSection.visibility = View.GONE
            return
        }
        binding.requestsSection.visibility = View.VISIBLE
        binding.tvRequestsTitle.text = "🔔 新的好友申请（${reqs.size} 条待你同意）"

        reqs.forEach { req ->
            val card = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(10), dp(10), dp(10), dp(10))
                background = GradientDrawable().apply {
                    cornerRadius = dp(12).toFloat()
                    setColor(Color.parseColor("#FFFDF6"))
                    setStroke(dp(1), Color.parseColor("#E8C89B"))
                }
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                ).apply { bottomMargin = dp(8) }
            }

            val topRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
            }
            val av = SpeciesThumbView(this).apply {
                loadAvatar(api, req.avatarIdx, req.avatarUrl)
                layoutParams = LinearLayout.LayoutParams(dp(40), dp(40)).apply { marginEnd = dp(10) }
            }
            val infoCol = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val nameTv = TextView(this).apply {
                val lvName = GrowthMeta.LEVELS.getOrNull(req.petLevel)?.name ?: "成长蛋"
                text = "${req.name}（${req.petName.ifBlank { lvName }}）"
                textSize = 14.5f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#233128"))
            }
            val msgTv = TextView(this).apply {
                text = req.message.ifBlank { "想和你成为朋友！" }
                textSize = 12.5f
                setTextColor(Color.parseColor("#6B6052"))
            }
            infoCol.addView(nameTv)
            infoCol.addView(msgTv)
            topRow.addView(av)
            topRow.addView(infoCol)

            val btnRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.END
                setPadding(0, dp(8), 0, 0)
            }
            val btnReject = Button(this).apply {
                text = "✕ 拒绝"
                textSize = 12f
                setAllCaps(false)
                setTextColor(Color.parseColor("#9B5642"))
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(Color.parseColor("#FFF3F0"))
                    setStroke(dp(1), Color.parseColor("#E5BFB8"))
                }
                setPadding(dp(12), 0, dp(12), 0)
                minWidth = 0
                minimumWidth = 0
                layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(32)).apply {
                    marginEnd = dp(8)
                }
                setOnClickListener { respondRequest(req.id, "reject") }
            }
            val btnAccept = Button(this).apply {
                text = "✓ 同意加好友"
                textSize = 12f
                setAllCaps(false)
                setTextColor(Color.WHITE)
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(Color.parseColor("#2F6B4F"))
                }
                setPadding(dp(14), 0, dp(14), 0)
                minWidth = 0
                minimumWidth = 0
                layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(32))
                setOnClickListener { respondRequest(req.id, "accept") }
            }
            btnRow.addView(btnReject)
            btnRow.addView(btnAccept)

            card.addView(topRow)
            card.addView(btnRow)
            binding.requestsContainer.addView(card)
        }
    }

    private fun respondRequest(reqId: String, action: String) {
        api.request(
            "POST",
            "/api/child/friends/requests/respond",
            JSONObject().put("requestId", reqId).put("action", action)
        ) { res ->
            if (res.ok) {
                hasChangedState = true
                if (action == "accept") sound.playWin() else sound.playPat()
                Toast.makeText(this, res.json?.optString("message", "已处理") ?: "已处理", Toast.LENGTH_SHORT).show()
                loadOverview()
            } else {
                sound.playError()
                Toast.makeText(this, res.error ?: "处理失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun renderFriendsList(ov: SocialOverview) {
        binding.friendsListContainer.removeAllViews()
        val fmt = SimpleDateFormat("HH:mm", Locale.getDefault())

        if (groupFilter == "all" || groupFilter == "群聊") {
            val chatHeaderRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(2), dp(6), dp(2), dp(6))
            }
            val chatHeader = TextView(this).apply {
                text = "👥 群聊（${ov.chatGroups.size}） · 同家庭默认成群"
                textSize = 13.5f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#4B4339"))
                layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
            }
            val btnNewGroup = Button(this).apply {
                text = "➕ 建群"
                textSize = 11.5f
                setAllCaps(false)
                setTextColor(Color.parseColor("#2F6B4F"))
                background = GradientDrawable().apply {
                    cornerRadius = dp(99).toFloat()
                    setColor(Color.parseColor("#E3F2E8"))
                }
                setPadding(dp(10), 0, dp(10), 0)
                minWidth = 0
                minimumWidth = 0
                minHeight = 0
                layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(28))
                setOnClickListener { showCreateGroupDialog() }
            }
            chatHeaderRow.addView(chatHeader)
            chatHeaderRow.addView(btnNewGroup)
            binding.friendsListContainer.addView(chatHeaderRow)

            if (ov.chatGroups.isEmpty()) {
                val emptyGrp = TextView(this).apply {
                    text = "暂无群聊，点击右上角「➕ 建群」拉上小伙伴一起聊！"
                    textSize = 12.5f
                    setTextColor(Color.parseColor("#8A7E6E"))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(12).toFloat()
                        setColor(Color.WHITE)
                        setStroke(dp(1), Color.parseColor("#EAE2D3"))
                    }
                    setPadding(dp(14), dp(14), dp(14), dp(14))
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { bottomMargin = dp(8) }
                }
                binding.friendsListContainer.addView(emptyGrp)
            } else {
                ov.chatGroups.forEach { g ->
                    val row = LinearLayout(this).apply {
                        orientation = LinearLayout.HORIZONTAL
                        gravity = Gravity.CENTER_VERTICAL
                        setPadding(dp(12), dp(11), dp(12), dp(11))
                        background = GradientDrawable().apply {
                            cornerRadius = dp(14).toFloat()
                            setColor(Color.WHITE)
                            setStroke(
                                dp(if (g.unreadCount > 0) 2 else 1),
                                Color.parseColor(if (g.unreadCount > 0) "#E8913A" else "#E5DCCB")
                            )
                        }
                        layoutParams = LinearLayout.LayoutParams(
                            LinearLayout.LayoutParams.MATCH_PARENT,
                            LinearLayout.LayoutParams.WRAP_CONTENT
                        ).apply { bottomMargin = dp(8) }
                    }

                    val iconTv = TextView(this).apply {
                        text = if (g.isFamilyDefault) "👨‍👩‍👧‍👦" else "👥"
                        textSize = 20f
                        gravity = Gravity.CENTER
                        background = GradientDrawable().apply {
                            cornerRadius = dp(23).toFloat()
                            setColor(Color.parseColor(if (g.isFamilyDefault) "#E3F2E8" else "#F3E8FF"))
                        }
                        layoutParams = LinearLayout.LayoutParams(dp(46), dp(46)).apply { marginEnd = dp(12) }
                    }

                    val centerCol = LinearLayout(this).apply {
                        orientation = LinearLayout.VERTICAL
                        layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                    }
                    val nameRow = LinearLayout(this).apply {
                        orientation = LinearLayout.HORIZONTAL
                        gravity = Gravity.CENTER_VERTICAL
                    }
                    val nameTv = TextView(this).apply {
                        text = g.name
                        textSize = 15.5f
                        setTypeface(null, Typeface.BOLD)
                        setTextColor(Color.parseColor("#233128"))
                        maxLines = 1
                        ellipsize = TextUtils.TruncateAt.END
                        layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT)
                    }
                    val tagTv = TextView(this).apply {
                        text = (if (g.isFamilyDefault) "家庭群" else "群聊") + " · ${g.memberCount}人"
                        textSize = 10.5f
                        setTextColor(Color.parseColor(if (g.isFamilyDefault) "#2F6B4F" else "#6D28D9"))
                        setPadding(dp(6), dp(2), dp(6), dp(2))
                        background = GradientDrawable().apply {
                            cornerRadius = dp(99).toFloat()
                            setColor(Color.parseColor(if (g.isFamilyDefault) "#E3F2E8" else "#F3E8FF"))
                        }
                        layoutParams = LinearLayout.LayoutParams(
                            LinearLayout.LayoutParams.WRAP_CONTENT,
                            LinearLayout.LayoutParams.WRAP_CONTENT
                        ).apply { marginStart = dp(6) }
                    }
                    nameRow.addView(nameTv)
                    nameRow.addView(tagTv)

                    val subTv = TextView(this).apply {
                        val fallbackSub = if (g.membersPreview.isNotBlank()) "成员：${g.membersPreview}" else "点击进入群聊~"
                        text = g.lastMessage.ifBlank { fallbackSub }
                        textSize = 12.5f
                        maxLines = 1
                        ellipsize = TextUtils.TruncateAt.END
                        setTextColor(Color.parseColor("#7D7263"))
                        setPadding(0, dp(3), 0, 0)
                    }
                    centerCol.addView(nameRow)
                    centerCol.addView(subTv)

                    val rightCol = LinearLayout(this).apply {
                        orientation = LinearLayout.VERTICAL
                        gravity = Gravity.END or Gravity.CENTER_VERTICAL
                        layoutParams = LinearLayout.LayoutParams(
                            LinearLayout.LayoutParams.WRAP_CONTENT,
                            LinearLayout.LayoutParams.WRAP_CONTENT
                        ).apply { marginStart = dp(8) }
                    }
                    if (g.lastMessageAt > 0) {
                        val timeTv = TextView(this).apply {
                            text = fmt.format(Date(g.lastMessageAt))
                            textSize = 11f
                            setTextColor(Color.parseColor("#9A8E7D"))
                        }
                        rightCol.addView(timeTv)
                    }
                    if (g.unreadCount > 0) {
                        val badge = TextView(this).apply {
                            text = g.unreadCount.toString()
                            textSize = 11f
                            setTypeface(null, Typeface.BOLD)
                            setTextColor(Color.WHITE)
                            gravity = Gravity.CENTER
                            setPadding(dp(7), dp(2), dp(7), dp(2))
                            background = GradientDrawable().apply {
                                cornerRadius = dp(99).toFloat()
                                setColor(Color.parseColor("#E11D48"))
                            }
                            layoutParams = LinearLayout.LayoutParams(
                                LinearLayout.LayoutParams.WRAP_CONTENT,
                                LinearLayout.LayoutParams.WRAP_CONTENT
                            ).apply { topMargin = dp(4) }
                        }
                        rightCol.addView(badge)
                    }

                    row.addView(iconTv)
                    row.addView(centerCol)
                    row.addView(rightCol)
                    row.setOnClickListener { openGroupChat(g) }
                    binding.friendsListContainer.addView(row)
                }
            }
        }

        if (groupFilter == "群聊") return

        val groupsToShow = when (groupFilter) {
            "家庭" -> listOf("家庭")
            "朋友" -> listOf("朋友")
            else -> listOf("家庭", "朋友")
        }
        for (grp in groupsToShow) {
            val list = ov.friends.filter { it.groupName == grp }
            val header = TextView(this).apply {
                text = if (grp == "家庭") {
                    "👨‍👩‍👧 家庭分组（${list.size}） · 同家庭自动成为好友"
                } else {
                    "🧒 朋友分组（${list.size}） · 互相验证通过的小伙伴"
                }
                textSize = 13.5f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#4B4339"))
                setPadding(dp(2), dp(8), dp(2), dp(6))
            }
            binding.friendsListContainer.addView(header)

            if (list.isEmpty()) {
                val empty = TextView(this).apply {
                    text = if (grp == "家庭") {
                        "当前家庭暂无其他成员档案"
                    } else {
                        "「朋友」分组暂时空空如也，点上方「➕ 加好友」邀请好朋友吧！"
                    }
                    textSize = 12.5f
                    setTextColor(Color.parseColor("#8A7E6E"))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(12).toFloat()
                        setColor(Color.WHITE)
                        setStroke(dp(1), Color.parseColor("#EAE2D3"))
                    }
                    setPadding(dp(14), dp(14), dp(14), dp(14))
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { bottomMargin = dp(8) }
                }
                binding.friendsListContainer.addView(empty)
                continue
            }

            list.forEach { f ->
                val row = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(dp(12), dp(11), dp(12), dp(11))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(14).toFloat()
                        setColor(Color.WHITE)
                        setStroke(
                            dp(if (f.unreadCount > 0) 2 else 1),
                            Color.parseColor(if (f.unreadCount > 0) "#E8913A" else "#E5DCCB")
                        )
                    }
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.MATCH_PARENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { bottomMargin = dp(8) }
                }

                val av = SpeciesThumbView(this).apply {
                    loadAvatar(api, f.avatarIdx, f.avatarUrl)
                    layoutParams = LinearLayout.LayoutParams(dp(46), dp(46)).apply { marginEnd = dp(12) }
                }

                val centerCol = LinearLayout(this).apply {
                    orientation = LinearLayout.VERTICAL
                    layoutParams = LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1f)
                }

                val nameRow = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                }
                val nameTv = TextView(this).apply {
                    text = f.name
                    textSize = 15.5f
                    setTypeface(null, Typeface.BOLD)
                    setTextColor(Color.parseColor("#233128"))
                }
                val tagTv = TextView(this).apply {
                    text = f.roleLabel.ifBlank { grp }
                    textSize = 10.5f
                    setTextColor(Color.parseColor(if (f.isFamily) "#2F6B4F" else "#6D28D9"))
                    setPadding(dp(6), dp(2), dp(6), dp(2))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(99).toFloat()
                        setColor(Color.parseColor(if (f.isFamily) "#E3F2E8" else "#F3E8FF"))
                    }
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.WRAP_CONTENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { marginStart = dp(6) }
                }
                nameRow.addView(nameTv)
                nameRow.addView(tagTv)

                val subTv = TextView(this).apply {
                    text = f.lastMessage.ifBlank { "点击发送文字、语音或图片消息~" }
                    textSize = 12.5f
                    maxLines = 1
                    ellipsize = TextUtils.TruncateAt.END
                    setTextColor(Color.parseColor("#7D7263"))
                    setPadding(0, dp(3), 0, 0)
                }
                centerCol.addView(nameRow)
                centerCol.addView(subTv)

                val rightCol = LinearLayout(this).apply {
                    orientation = LinearLayout.VERTICAL
                    gravity = Gravity.END or Gravity.CENTER_VERTICAL
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.WRAP_CONTENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { marginStart = dp(8) }
                }
                if (f.lastMessageAt > 0) {
                    val timeTv = TextView(this).apply {
                        text = fmt.format(Date(f.lastMessageAt))
                        textSize = 11f
                        setTextColor(Color.parseColor("#9A8E7D"))
                    }
                    rightCol.addView(timeTv)
                }
                if (f.unreadCount > 0) {
                    val badge = TextView(this).apply {
                        text = f.unreadCount.toString()
                        textSize = 11f
                        setTypeface(null, Typeface.BOLD)
                        setTextColor(Color.WHITE)
                        gravity = Gravity.CENTER
                        setPadding(dp(7), dp(2), dp(7), dp(2))
                        background = GradientDrawable().apply {
                            cornerRadius = dp(99).toFloat()
                            setColor(Color.parseColor("#E11D48"))
                        }
                        layoutParams = LinearLayout.LayoutParams(
                            LinearLayout.LayoutParams.WRAP_CONTENT,
                            LinearLayout.LayoutParams.WRAP_CONTENT
                        ).apply { topMargin = dp(4) }
                    }
                    rightCol.addView(badge)
                }

                row.addView(av)
                row.addView(centerCol)
                row.addView(rightCol)

                row.setOnClickListener { openChat(f) }
                if (!f.isFamily) {
                    row.setOnLongClickListener {
                        AlertDialog.Builder(this)
                            .setTitle("解除好友关系")
                            .setMessage("确定要将「${f.name}」从朋友列表中移除吗？")
                            .setPositiveButton("确定移除") { _, _ ->
                                api.request("DELETE", "/api/child/friends/${f.peerId}") { r ->
                                    if (r.ok) {
                                        hasChangedState = true
                                        Toast.makeText(this, "已移除好友", Toast.LENGTH_SHORT).show()
                                        loadOverview()
                                    } else {
                                        Toast.makeText(this, r.error ?: "操作失败", Toast.LENGTH_SHORT).show()
                                    }
                                }
                            }
                            .setNegativeButton("取消", null)
                            .show()
                        true
                    }
                }
                binding.friendsListContainer.addView(row)
            }
        }
    }

    private fun openGroupChat(g: ChatGroupItem) {
        hasChangedState = true
        val it = Intent(this, ChatActivity::class.java).apply {
            putExtra(ChatActivity.EXTRA_PEER_TYPE, "group")
            putExtra(ChatActivity.EXTRA_PEER_ID, g.id)
            putExtra(ChatActivity.EXTRA_PEER_NAME, g.name)
            putExtra(ChatActivity.EXTRA_PEER_GROUP, if (g.isFamilyDefault) "家庭群" else "群聊")
            putExtra(ChatActivity.EXTRA_PEER_ROLE, "${g.memberCount}位成员")
        }
        startActivity(it)
    }

    private fun openChat(f: FriendItem) {
        hasChangedState = true
        val it = Intent(this, ChatActivity::class.java).apply {
            putExtra(ChatActivity.EXTRA_PEER_TYPE, f.peerType)
            putExtra(ChatActivity.EXTRA_PEER_ID, f.peerId)
            putExtra(ChatActivity.EXTRA_PEER_NAME, f.name)
            putExtra(ChatActivity.EXTRA_PEER_GROUP, f.groupName)
            putExtra(ChatActivity.EXTRA_PEER_ROLE, f.roleLabel)
            putExtra(ChatActivity.EXTRA_PEER_AVATAR_IDX, f.avatarIdx)
            putExtra(ChatActivity.EXTRA_PEER_AVATAR_URL, f.avatarUrl)
        }
        startActivity(it)
    }

    private fun launchQrScanner() {
        qrScanLauncher.launch(Intent(this, QrScanActivity::class.java))
    }

    private fun showAddFriendChoiceDialog() {
        val options = arrayOf(
            "📷 扫一扫好友二维码",
            "🔎 输入好友分享码或账号",
            "👥 发起群聊（选好友/家人）",
            "🪪 亮出我的好友二维码名片"
        )
        AlertDialog.Builder(this)
            .setTitle("添加好友与群聊")
            .setItems(options) { _, which ->
                when (which) {
                    0 -> launchQrScanner()
                    1 -> showSearchAccountOrCodeDialog()
                    2 -> showCreateGroupDialog()
                    3 -> showMyQrCardDialog()
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun showCreateGroupDialog() {
        val candidates = overview?.friends.orEmpty()
        if (candidates.isEmpty()) {
            Toast.makeText(this, "先添加至少一位好友或家人才能建群哦~", Toast.LENGTH_SHORT).show()
            return
        }
        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(20), dp(14), dp(20), dp(6))
        }
        val etName = EditText(this).apply {
            hint = "起个群名，例如：跳绳小分队"
            maxLines = 1
        }
        val tipTv = TextView(this).apply {
            text = "选择一起加入群聊的家人或小伙伴："
            textSize = 13f
            setTextColor(Color.parseColor("#5C5347"))
            setPadding(0, dp(12), 0, dp(6))
        }
        val checksWrap = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
        }
        val selected = mutableListOf<Pair<FriendItem, CheckBox>>()
        candidates.forEach { f ->
            val cb = CheckBox(this).apply {
                text = "${f.name}（${f.roleLabel.ifBlank { f.groupName }}）"
                textSize = 14f
                setTextColor(Color.parseColor("#233128"))
            }
            checksWrap.addView(cb)
            selected.add(f to cb)
        }
        val sv = ScrollView(this).apply {
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                dp(210)
            )
            addView(checksWrap)
        }
        box.addView(etName)
        box.addView(tipTv)
        box.addView(sv)

        AlertDialog.Builder(this)
            .setTitle("👥 发起新群聊")
            .setView(box)
            .setPositiveButton("创建群聊") { _, _ ->
                val gName = etName.text.toString().trim()
                val membersArr = JSONArray()
                selected.forEach { (f, cb) ->
                    if (cb.isChecked) {
                        membersArr.put(JSONObject().put("memberType", f.peerType).put("memberId", f.peerId))
                    }
                }
                if (gName.isEmpty()) {
                    Toast.makeText(this, "请输入群名称", Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }
                if (membersArr.length() == 0) {
                    Toast.makeText(this, "请至少选择一位好友或家人", Toast.LENGTH_SHORT).show()
                    return@setPositiveButton
                }
                val payload = JSONObject()
                    .put("name", gName)
                    .put("members", membersArr)
                api.request("POST", "/api/child/groups", payload) { res ->
                    if (res.ok && res.json != null) {
                        sound.playWin()
                        hasChangedState = true
                        val gObj = res.json.optJSONObject("group")
                        loadOverview()
                        if (gObj != null) {
                            openGroupChat(SocialParser.parseGroupItem(gObj))
                        }
                    } else {
                        sound.playError()
                        Toast.makeText(this, res.error ?: "创建群聊失败", Toast.LENGTH_SHORT).show()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun showMyQrCardDialog() {
        val self = overview?.self ?: return
        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            setPadding(dp(20), dp(16), dp(20), dp(8))
        }
        val qrBmp = SocialParser.generateQrBitmap(self.qrPayload, 520)
        val qrIv = ImageView(this).apply {
            setImageBitmap(qrBmp)
            layoutParams = LinearLayout.LayoutParams(dp(220), dp(220))
        }
        val codeTv = TextView(this).apply {
            text = "分享码：${self.friendCode}"
            textSize = 18f
            setTypeface(null, Typeface.BOLD)
            setTextColor(Color.parseColor("#2F6B4F"))
            setPadding(0, dp(12), 0, dp(4))
        }
        val userTv = TextView(this).apply {
            text = "登录账号：${self.username.ifBlank { "暂未设置" }}\n（朋友扫一扫或输入分享码/账号，均需你同意才可聊天）"
            textSize = 12.5f
            gravity = Gravity.CENTER
            setTextColor(Color.parseColor("#6B6052"))
        }
        box.addView(qrIv)
        box.addView(codeTv)
        box.addView(userTv)

        AlertDialog.Builder(this)
            .setTitle("🪪 ${self.name} 的好友名片")
            .setView(box)
            .setPositiveButton("📋 复制分享码") { _, _ ->
                copyText("我的成长好友分享码", self.friendCode)
            }
            .setNeutralButton("🔄 更换分享码") { _, _ ->
                api.request("POST", "/api/child/friends/code/reset", JSONObject()) { res ->
                    if (res.ok) {
                        Toast.makeText(this, "分享码已更新！", Toast.LENGTH_SHORT).show()
                        loadOverview()
                    } else {
                        Toast.makeText(this, res.error ?: "更新失败", Toast.LENGTH_SHORT).show()
                    }
                }
            }
            .setNegativeButton("关闭", null)
            .show()
    }

    private fun showSearchAccountOrCodeDialog() {
        val box = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(18), dp(10), dp(18), dp(4))
        }
        val et = EditText(this).apply {
            hint = "输入小伙伴的分享码（如 K7M9P2XQ）或账号"
            isSingleLine = true
        }
        box.addView(et)

        AlertDialog.Builder(this)
            .setTitle("🔎 通过分享码 / 账号加好友")
            .setView(box)
            .setPositiveButton("查找小伙伴") { _, _ ->
                val q = et.text.toString().trim()
                if (q.isEmpty()) {
                    Toast.makeText(this, "请输入分享码或账号", Toast.LENGTH_SHORT).show()
                } else {
                    lookupAndConfirmAddFriend(q)
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun lookupAndConfirmAddFriend(queryRaw: String) {
        val encoded = java.net.URLEncoder.encode(queryRaw, "UTF-8")
        api.request("GET", "/api/child/friends/lookup?q=$encoded") { res ->
            if (!res.ok || res.json == null) {
                sound.playError()
                Toast.makeText(this, res.error ?: "未找到该小伙伴", Toast.LENGTH_SHORT).show()
                return@request
            }
            val candObj = res.json.optJSONObject("candidate") ?: return@request
            val cand = SocialParser.parseLookup(candObj)
            when (cand.relationStatus) {
                "self" -> {
                    Toast.makeText(this, "这是你自己的名片哦~", Toast.LENGTH_SHORT).show()
                    return@request
                }
                "family" -> {
                    Toast.makeText(this, "${cand.name}和你在同一家庭，已在「家庭」分组中直接聊天！", Toast.LENGTH_LONG).show()
                    return@request
                }
                "friend" -> {
                    Toast.makeText(this, "你们已经是「朋友」分组的好友啦！", Toast.LENGTH_SHORT).show()
                    return@request
                }
            }

            val box = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                gravity = Gravity.CENTER_HORIZONTAL
                setPadding(dp(20), dp(12), dp(20), dp(6))
            }
            val av = SpeciesThumbView(this).apply {
                loadAvatar(api, cand.avatarIdx, cand.avatarUrl)
                layoutParams = LinearLayout.LayoutParams(dp(64), dp(64))
            }
            val titleTv = TextView(this).apply {
                val lvName = GrowthMeta.LEVELS.getOrNull(cand.petLevel)?.name ?: "成长蛋"
                text = "${cand.name}  ·  ${cand.petName.ifBlank { lvName }}"
                textSize = 16.5f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#233128"))
                setPadding(0, dp(8), 0, dp(4))
            }
            val subTv = TextView(this).apply {
                text = "账号：${cand.username.ifBlank { "-" }}"
                textSize = 12.5f
                setTextColor(Color.parseColor("#7D7263"))
                setPadding(0, 0, 0, dp(10))
            }
            val etGreet = EditText(this).apply {
                hint = "附一句打招呼的话（可选）"
                setText("你好，我是${overview?.self?.name ?: ""}，我们做朋友吧！")
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    LinearLayout.LayoutParams.WRAP_CONTENT
                )
            }
            box.addView(av)
            box.addView(titleTv)
            box.addView(subTv)
            box.addView(etGreet)

            val btnLabel = if (cand.relationStatus == "pending_incoming") "✓ 对方已申请，直接同意并加好友" else "发送好友申请"
            AlertDialog.Builder(this)
                .setTitle("确认添加小伙伴")
                .setView(box)
                .setPositiveButton(btnLabel) { _, _ ->
                    val payload = JSONObject()
                        .put("target", cand.childId)
                        .put("message", etGreet.text.toString().trim())
                    api.request("POST", "/api/child/friends/request", payload) { r ->
                        if (r.ok) {
                            hasChangedState = true
                            sound.playWin()
                            Toast.makeText(this, r.json?.optString("message", "已发送") ?: "已发送", Toast.LENGTH_LONG).show()
                            loadOverview()
                        } else {
                            sound.playError()
                            Toast.makeText(this, r.error ?: "发送失败", Toast.LENGTH_SHORT).show()
                        }
                    }
                }
                .setNegativeButton("取消", null)
                .show()
        }
    }

    private fun copyText(label: String, txt: String) {
        val cm = getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        cm.setPrimaryClip(ClipData.newPlainText(label, txt))
        sound.playPat()
        Toast.makeText(this, "已复制：$txt", Toast.LENGTH_SHORT).show()
    }
}
