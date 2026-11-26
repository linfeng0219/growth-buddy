package com.growthbuddy.child

import android.Manifest
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
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
 * - 手机 / 竖屏单栏点击进入独立 ChatActivity；平板横屏左侧显示联系人分组，右侧直接内嵌聊天窗口。
 * - 支持点击分组名（群聊 / 家庭 / 朋友）展开或折叠组内元素，展示好友在线/离线状态。
 */
class FriendsActivity : AppCompatActivity() {

    private lateinit var binding: ActivityFriendsBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private lateinit var chatController: ChildChatPaneController

    private var overview: SocialOverview? = null
    private val collapsedGroups = mutableMapOf<String, Boolean>()
    private var selectedPeerType: String = ""
    private var selectedPeerId: String = ""
    private var isResumed = false
    private var hasChangedState = false
    private val mainHandler = Handler(Looper.getMainLooper())

    private val audioPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        chatController.onAudioPermissionResult(granted)
    }

    private val cameraPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        chatController.onCameraPermissionResult(granted)
    }

    private val galleryImgLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        chatController.onImagePicked(uri)
    }

    private val cameraPhotoLauncher = registerForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { ok ->
        chatController.onCameraCaptured(ok)
    }

    private val overviewPollTicker = object : Runnable {
        override fun run() {
            if (!isResumed) return
            loadOverview(silent = true)
            mainHandler.postDelayed(this, 10_000L)
        }
    }

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

        chatController = ChildChatPaneController(
            activity = this,
            binding = binding.friendsChatPane,
            api = api,
            sound = sound,
            onRequestAudioPermission = { audioPermLauncher.launch(Manifest.permission.RECORD_AUDIO) },
            onRequestCameraPermission = { cameraPermLauncher.launch(Manifest.permission.CAMERA) },
            onLaunchGallery = { galleryImgLauncher.launch("image/*") },
            onLaunchCamera = { uri -> cameraPhotoLauncher.launch(uri) },
            onMessagesChanged = { loadOverview(silent = true) },
            onGroupLeft = {
                selectedPeerType = ""
                selectedPeerId = ""
                loadOverview(silent = false)
            }
        )
        chatController.setBackButtonVisible(false)

        binding.btnFriendsBack.setOnClickListener {
            if (hasChangedState) setResult(RESULT_OK)
            finish()
        }
        binding.btnMyCard.setOnClickListener { showMyQrCardDialog() }
        binding.btnAddFriend.setOnClickListener { showAddFriendChoiceDialog() }

        applyResponsiveLayout()
        loadOverview(silent = false)
    }

    override fun onResume() {
        super.onResume()
        isResumed = true
        loadOverview(silent = false)
        chatController.setActive(ResponsiveUi.metrics(this).isDesktopSplit && selectedPeerId.isNotBlank())
        mainHandler.removeCallbacks(overviewPollTicker)
        mainHandler.postDelayed(overviewPollTicker, 10_000L)
    }

    override fun onPause() {
        isResumed = false
        mainHandler.removeCallbacks(overviewPollTicker)
        chatController.onPause()
        super.onPause()
    }

    override fun onDestroy() {
        chatController.onDestroy()
        super.onDestroy()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
        overview?.let {
            syncSplitSelectionAndPane(it)
            renderFriendsList(it)
        }
    }

    private fun dp(v: Int): Int = ResponsiveUi.dp(this, v)

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 1180 else if (m.isTablet) 740 else 1200
        binding.tvFriendsTitle.setTextSize(android.util.TypedValue.COMPLEX_UNIT_SP, if (m.isTablet) 18f else 15.5f)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.friendsTopBar, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.friendsBodySplit, maxW, horizontalReserveDp = 28)

        val listLp = binding.friendsListScroll.layoutParams as LinearLayout.LayoutParams
        val chatLp = binding.friendsChatPaneWrap.layoutParams as LinearLayout.LayoutParams
        val hasReqs = !overview?.incomingRequests.isNullOrEmpty()
        binding.requestsSection.visibility = if (hasReqs) View.VISIBLE else View.GONE

        if (m.isDesktopSplit) {
            binding.friendsBodySplit.orientation = LinearLayout.HORIZONTAL
            listLp.width = 0
            listLp.height = ViewGroup.LayoutParams.MATCH_PARENT
            listLp.weight = 0.88f
            chatLp.width = 0
            chatLp.height = ViewGroup.LayoutParams.MATCH_PARENT
            chatLp.weight = 1.18f
            chatLp.marginStart = dp(14)
            binding.friendsChatPaneWrap.visibility = View.VISIBLE
            chatController.setActive(isResumed && selectedPeerId.isNotBlank())
        } else {
            binding.friendsBodySplit.orientation = LinearLayout.VERTICAL
            listLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            listLp.height = ViewGroup.LayoutParams.MATCH_PARENT
            listLp.weight = 0f
            chatLp.width = 0
            chatLp.weight = 0f
            chatLp.marginStart = 0
            binding.friendsChatPaneWrap.visibility = View.GONE
            chatController.setActive(false)
        }
        binding.friendsListScroll.layoutParams = listLp
        binding.friendsChatPaneWrap.layoutParams = chatLp
    }

    private fun loadOverview(silent: Boolean = false) {
        if (!silent) binding.friendsLoading.visibility = View.VISIBLE
        api.request("GET", "/api/child/friends") { res ->
            binding.friendsLoading.visibility = View.GONE
            if (!res.ok || res.json == null) {
                if (!silent) Toast.makeText(this, res.error ?: "读取好友列表失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val parsed = SocialParser.parseOverview(res.json)
            overview = parsed
            renderRequests(parsed.incomingRequests)
            applyResponsiveLayout()
            syncSplitSelectionAndPane(parsed)
            renderFriendsList(parsed)
        }
    }

    private fun syncSplitSelectionAndPane(ov: SocialOverview) {
        val split = ResponsiveUi.metrics(this).isDesktopSplit
        if (!split) return

        val validGroup = selectedPeerType == "group" && ov.chatGroups.any { it.id == selectedPeerId }
        val validFriend = selectedPeerType != "group" && ov.friends.any { it.peerType == selectedPeerType && it.peerId == selectedPeerId }

        if (!validGroup && !validFriend) {
            val firstGrp = ov.chatGroups.firstOrNull()
            val firstFr = ov.friends.firstOrNull()
            when {
                firstGrp != null -> {
                    openGroupInSplit(firstGrp)
                    return
                }
                firstFr != null -> {
                    openFriendInSplit(firstFr)
                    return
                }
                else -> {
                    selectedPeerType = ""
                    selectedPeerId = ""
                }
            }
        } else if (validGroup) {
            val g = ov.chatGroups.first { it.id == selectedPeerId }
            chatController.updatePeerPresence(
                "group",
                g.id,
                g.name,
                if (g.isFamilyDefault) "家庭群" else "群聊",
                "${g.memberCount}位成员",
                false
            )
        } else if (validFriend) {
            val f = ov.friends.first { it.peerType == selectedPeerType && it.peerId == selectedPeerId }
            chatController.updatePeerPresence("child".takeIf { f.peerType == "child" } ?: f.peerType, f.peerId, f.name, f.groupName, f.roleLabel, f.isOnline)
        }

        val hasSel = selectedPeerId.isNotBlank()
        binding.friendsChatPane.root.visibility = if (hasSel) View.VISIBLE else View.GONE
        binding.tvSplitChatPlaceholder.visibility = if (hasSel) View.GONE else View.VISIBLE
        chatController.setActive(isResumed && hasSel)
    }

    private fun openGroupInSplit(g: ChatGroupItem) {
        hasChangedState = true
        selectedPeerType = "group"
        selectedPeerId = g.id
        binding.friendsChatPane.root.visibility = View.VISIBLE
        binding.tvSplitChatPlaceholder.visibility = View.GONE
        chatController.setActive(isResumed)
        chatController.openPeer(
            peerType = "group",
            peerId = g.id,
            peerName = g.name,
            peerGroup = if (g.isFamilyDefault) "家庭群" else "群聊",
            peerRole = "${g.memberCount}位成员",
            isOnline = false
        )
        overview?.let { renderFriendsList(it) }
    }

    private fun openFriendInSplit(f: FriendItem) {
        hasChangedState = true
        selectedPeerType = f.peerType
        selectedPeerId = f.peerId
        binding.friendsChatPane.root.visibility = View.VISIBLE
        binding.tvSplitChatPlaceholder.visibility = View.GONE
        chatController.setActive(isResumed)
        chatController.openPeer(
            peerType = f.peerType,
            peerId = f.peerId,
            peerName = f.name,
            peerGroup = f.groupName,
            peerRole = f.roleLabel,
            avatarIdx = f.avatarIdx,
            avatarUrl = f.avatarUrl,
            isOnline = f.isOnline
        )
        overview?.let { renderFriendsList(it) }
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
                setOnClickListener { showAvatarPreviewDialog(api, "${req.name} 的头像", req.avatarIdx, req.avatarUrl) }
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
        val isSplit = ResponsiveUi.metrics(this).isDesktopSplit

        // 1. 群聊分组（点击分组名可折叠/展开，右侧固定「➕ 建群」按钮）
        val grpCollapsed = collapsedGroups["群聊"] == true
        val chatHeaderRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(dp(4), dp(8), dp(2), dp(6))
            setOnClickListener {
                collapsedGroups["群聊"] = !grpCollapsed
                renderFriendsList(ov)
            }
        }
        val chatHeader = TextView(this).apply {
            val arrow = if (grpCollapsed) "▶ " else "▼ "
            text = "${arrow}👥 群聊（${ov.chatGroups.size}） · 同家庭默认成群"
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

        if (!grpCollapsed) {
            if (ov.chatGroups.isEmpty()) {
                val emptyGrp = TextView(this).apply {
                    text = "暂无群聊，点击右侧「➕ 建群」拉上小伙伴一起聊！"
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
                    val isSelected = isSplit && selectedPeerType == "group" && selectedPeerId == g.id
                    val row = LinearLayout(this).apply {
                        orientation = LinearLayout.HORIZONTAL
                        gravity = Gravity.CENTER_VERTICAL
                        setPadding(dp(12), dp(11), dp(12), dp(11))
                        background = GradientDrawable().apply {
                            cornerRadius = dp(14).toFloat()
                            setColor(Color.parseColor(if (isSelected) "#EAF5EE" else "#FFFFFF"))
                            setStroke(
                                dp(if (isSelected || g.unreadCount > 0) 2 else 1),
                                Color.parseColor(if (isSelected) "#2F6B4F" else if (g.unreadCount > 0) "#E8913A" else "#E5DCCB")
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

        // 2. 家庭分组 & 朋友分组（点击分组名展开或折叠，展示在线/离线状态）
        for (grp in listOf("家庭", "朋友")) {
            val list = ov.friends.filter { it.groupName == grp }
            val isCollapsed = collapsedGroups[grp] == true
            val arrow = if (isCollapsed) "▶ " else "▼ "
            val header = TextView(this).apply {
                text = if (grp == "家庭") {
                    "${arrow}👨‍👩‍👧 家庭分组（${list.size}） · 同家庭自动成为好友"
                } else {
                    "${arrow}🧒 朋友分组（${list.size}） · 互相验证通过的小伙伴"
                }
                textSize = 13.5f
                setTypeface(null, Typeface.BOLD)
                setTextColor(Color.parseColor("#4B4339"))
                setPadding(dp(4), dp(10), dp(4), dp(6))
                setOnClickListener {
                    collapsedGroups[grp] = !isCollapsed
                    renderFriendsList(ov)
                }
            }
            binding.friendsListContainer.addView(header)

            if (isCollapsed) continue

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
                val isSelected = isSplit && selectedPeerType == f.peerType && selectedPeerId == f.peerId
                val row = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(dp(12), dp(11), dp(12), dp(11))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(14).toFloat()
                        setColor(Color.parseColor(if (isSelected) "#EAF5EE" else "#FFFFFF"))
                        setStroke(
                            dp(if (isSelected || f.unreadCount > 0) 2 else 1),
                            Color.parseColor(if (isSelected) "#2F6B4F" else if (f.unreadCount > 0) "#E8913A" else "#E5DCCB")
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
                    setOnClickListener { showAvatarPreviewDialog(api, "${f.name} 的头像", f.avatarIdx, f.avatarUrl) }
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
                    maxLines = 1
                    ellipsize = TextUtils.TruncateAt.END
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
                val onlineTv = TextView(this).apply {
                    text = if (f.isOnline) "🟢 在线" else "⚪ 离线"
                    textSize = 10f
                    setTextColor(Color.parseColor(if (f.isOnline) "#1C7C46" else "#8A8378"))
                    setPadding(dp(6), dp(2), dp(6), dp(2))
                    background = GradientDrawable().apply {
                        cornerRadius = dp(99).toFloat()
                        setColor(Color.parseColor(if (f.isOnline) "#E6F6EC" else "#F1EEE8"))
                    }
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.WRAP_CONTENT,
                        LinearLayout.LayoutParams.WRAP_CONTENT
                    ).apply { marginStart = dp(5) }
                }
                nameRow.addView(nameTv)
                nameRow.addView(tagTv)
                nameRow.addView(onlineTv)

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
                                        if (selectedPeerType == f.peerType && selectedPeerId == f.peerId) {
                                            selectedPeerType = ""
                                            selectedPeerId = ""
                                        }
                                        Toast.makeText(this, "已移除好友", Toast.LENGTH_SHORT).show()
                                        loadOverview(silent = false)
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
        if (ResponsiveUi.metrics(this).isDesktopSplit) {
            openGroupInSplit(g)
            return
        }
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
        if (ResponsiveUi.metrics(this).isDesktopSplit) {
            openFriendInSplit(f)
            return
        }
        hasChangedState = true
        val it = Intent(this, ChatActivity::class.java).apply {
            putExtra(ChatActivity.EXTRA_PEER_TYPE, f.peerType)
            putExtra(ChatActivity.EXTRA_PEER_ID, f.peerId)
            putExtra(ChatActivity.EXTRA_PEER_NAME, f.name)
            putExtra(ChatActivity.EXTRA_PEER_GROUP, f.groupName)
            putExtra(ChatActivity.EXTRA_PEER_ROLE, f.roleLabel)
            putExtra(ChatActivity.EXTRA_PEER_AVATAR_IDX, f.avatarIdx)
            putExtra(ChatActivity.EXTRA_PEER_AVATAR_URL, f.avatarUrl)
            putExtra(ChatActivity.EXTRA_PEER_ONLINE, f.isOnline)
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
                setOnClickListener { showAvatarPreviewDialog(api, "${cand.name} 的头像", cand.avatarIdx, cand.avatarUrl) }
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
