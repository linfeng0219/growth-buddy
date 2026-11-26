package com.growthbuddy.child

import android.Manifest
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.Matrix
import android.graphics.drawable.GradientDrawable
import android.media.AudioAttributes
import android.media.ExifInterface
import android.media.MediaPlayer
import android.media.MediaRecorder
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.Base64
import android.view.Gravity
import android.view.View
import android.view.inputmethod.EditorInfo
import android.widget.Button
import android.widget.CheckBox
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import com.growthbuddy.child.databinding.ViewChatPaneBinding
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.URLEncoder
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 可复用即时聊天控制器：
 * - 供独立 ChatActivity（手机/竖屏）及 FriendsActivity（平板横屏右侧窗格）共用，避免逻辑重复。
 */
class ChildChatPaneController(
    private val activity: AppCompatActivity,
    private val binding: ViewChatPaneBinding,
    private val api: ApiClient,
    private val sound: NativeSoundEngine,
    private val onRequestAudioPermission: () -> Unit,
    private val onRequestCameraPermission: () -> Unit,
    private val onLaunchGallery: () -> Unit,
    private val onLaunchCamera: (Uri) -> Unit,
    private val onBackClicked: () -> Unit = {},
    private val onMessagesChanged: () -> Unit = {},
    private val onGroupLeft: () -> Unit = {}
) {
    var peerType: String = ""
        private set
    var peerId: String = ""
        private set
    var peerName: String = "好友"
        private set
    var peerGroup: String = "朋友"
        private set
    var peerRole: String = ""
        private set
    var peerOnline: Boolean = false
        private set

    private var peerAvatarIdx: Int = 0
    private var peerAvatarUrl: String = ""

    private val renderedIds = LinkedHashSet<String>()
    private var lastMessageTs: Long = 0L
    private var isResumed = false
    private val mainHandler = Handler(Looper.getMainLooper())

    private var recorder: MediaRecorder? = null
    private var recordingFile: File? = null
    private var recordingStartMs: Long = 0L
    private var isRecording = false

    private var activePlayer: MediaPlayer? = null
    private var activeVoiceButton: Button? = null
    private var activeVoiceLabel: String = ""

    private var pendingCameraUri: Uri? = null

    private val pollTicker = object : Runnable {
        override fun run() {
            if (!isResumed || peerId.isBlank()) return
            fetchMessages(incremental = true)
            mainHandler.postDelayed(this, 3500L)
        }
    }

    private val recTicker = object : Runnable {
        override fun run() {
            if (!isRecording) return
            val sec = ((SystemClock.elapsedRealtime() - recordingStartMs) / 1000L).toInt().coerceAtLeast(1)
            binding.tvVoiceRecTimer.text = String.format(Locale.getDefault(), "🎙️ 正在录音 00:%02d...", sec)
            if (sec >= 60) {
                finishAndSendVoice()
            } else {
                mainHandler.postDelayed(this, 400L)
            }
        }
    }

    init {
        binding.btnChatBack.setOnClickListener { onBackClicked() }
        binding.btnChatSend.setOnClickListener { sendTextMessage() }
        binding.etChatInput.setOnEditorActionListener { _, actionId, _ ->
            if (actionId == EditorInfo.IME_ACTION_SEND) {
                sendTextMessage()
                true
            } else false
        }
        binding.btnChatImage.setOnClickListener { showImagePickChoice() }
        binding.btnChatVoice.setOnClickListener {
            if (peerId.isBlank()) return@setOnClickListener
            if (isRecording) {
                finishAndSendVoice()
            } else if (ContextCompat.checkSelfPermission(activity, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
                startVoiceRecording()
            } else {
                onRequestAudioPermission()
            }
        }
        binding.btnCancelVoice.setOnClickListener { cancelVoiceRecording() }
        binding.btnSendVoiceNow.setOnClickListener { finishAndSendVoice() }
        binding.btnChatGroupInfo.setOnClickListener { showGroupInfoDialog() }
        binding.ivChatPeerAvatar.setOnClickListener {
            if (peerId.isNotBlank() && peerType != "group") {
                activity.showAvatarPreviewDialog(api, "${peerName} 的头像", peerAvatarIdx, peerAvatarUrl)
            }
        }
    }

    private fun dp(v: Int): Int = ResponsiveUi.dp(activity, v)

    fun setBackButtonVisible(visible: Boolean) {
        binding.btnChatBack.visibility = if (visible) View.VISIBLE else View.GONE
    }

    fun openPeer(
        peerType: String,
        peerId: String,
        peerName: String,
        peerGroup: String,
        peerRole: String,
        avatarIdx: Int = 0,
        avatarUrl: String = "",
        isOnline: Boolean = false
    ) {
        val changed = this.peerType != peerType || this.peerId != peerId
        this.peerType = peerType
        this.peerId = peerId
        this.peerName = peerName
        this.peerGroup = peerGroup
        this.peerRole = peerRole
        this.peerAvatarIdx = avatarIdx
        this.peerAvatarUrl = avatarUrl
        this.peerOnline = isOnline

        renderPeerHeader()
        if (changed) {
            cancelVoiceRecording()
            stopVoicePlayback()
            renderedIds.clear()
            lastMessageTs = 0L
            binding.chatMessagesContainer.removeAllViews()
            binding.etChatInput.setText("")
            fetchMessages(incremental = false)
        }
        if (isResumed) {
            mainHandler.removeCallbacks(pollTicker)
            mainHandler.postDelayed(pollTicker, 3500L)
        }
    }

    fun updatePeerPresence(peerType: String, peerId: String, peerName: String, peerGroup: String, peerRole: String, isOnline: Boolean) {
        if (this.peerType != peerType || this.peerId != peerId) return
        this.peerName = peerName
        this.peerGroup = peerGroup
        this.peerRole = peerRole
        this.peerOnline = isOnline
        renderPeerHeader()
    }

    private fun renderPeerHeader() {
        binding.tvChatPeerName.text = peerName
        if (peerType == "group") {
            binding.ivChatPeerAvatar.visibility = View.GONE
            binding.tvChatGroupBadge.visibility = View.VISIBLE
            binding.tvChatGroupBadge.text = if (peerGroup == "家庭群") "👨‍👩‍👧‍👦" else "👥"
            binding.btnChatGroupInfo.visibility = View.VISIBLE
            binding.tvChatPeerMeta.text = "$peerGroup · $peerRole · 文字 / 语音 / 图片"
        } else {
            binding.tvChatGroupBadge.visibility = View.GONE
            binding.btnChatGroupInfo.visibility = View.GONE
            binding.ivChatPeerAvatar.visibility = View.VISIBLE
            val statusStr = if (peerOnline) "🟢 在线" else "⚪ 离线"
            val roleTag = if (peerRole.isNotBlank()) " · $peerRole" else ""
            binding.tvChatPeerMeta.text = "$statusStr · $peerGroup 分组$roleTag · 文字 / 语音 / 图片"
            binding.ivChatPeerAvatar.loadAvatar(api, peerAvatarIdx, peerAvatarUrl)
        }
    }

    fun setActive(active: Boolean) {
        isResumed = active
        mainHandler.removeCallbacks(pollTicker)
        if (active && peerId.isNotBlank()) {
            mainHandler.postDelayed(pollTicker, 3500L)
        }
    }

    fun onPause() {
        setActive(false)
    }

    fun onDestroy() {
        setActive(false)
        cancelVoiceRecording()
        stopVoicePlayback()
    }

    fun onAudioPermissionResult(granted: Boolean) {
        if (granted) startVoiceRecording()
        else Toast.makeText(activity, "需要麦克风权限才能发送语音消息", Toast.LENGTH_LONG).show()
    }

    fun onCameraPermissionResult(granted: Boolean) {
        if (granted) launchCameraPhoto()
        else Toast.makeText(activity, "需要摄像头权限才能拍照发图", Toast.LENGTH_SHORT).show()
    }

    fun onImagePicked(uri: Uri?) {
        if (uri != null) sendImageUri(uri)
    }

    fun onCameraCaptured(ok: Boolean) {
        val uri = pendingCameraUri
        pendingCameraUri = null
        if (ok && uri != null) sendImageUri(uri)
    }

    fun fetchMessages(incremental: Boolean) {
        if (peerId.isBlank()) return
        val reqPeerType = peerType
        val reqPeerId = peerId
        val pType = URLEncoder.encode(reqPeerType, "UTF-8")
        val pId = URLEncoder.encode(reqPeerId, "UTF-8")
        val after = if (incremental && lastMessageTs > 0) "&after=$lastMessageTs" else ""
        api.request("GET", "/api/child/chat/messages?peerType=$pType&peerId=$pId$after") { res ->
            if (!res.ok || res.json == null) return@request
            if (reqPeerType != peerType || reqPeerId != peerId) return@request
            val arr = res.json.optJSONArray("messages") ?: JSONArray()
            val msgs = SocialParser.parseMessages(arr)
            if (!incremental && renderedIds.isEmpty()) {
                binding.chatMessagesContainer.removeAllViews()
                if (msgs.isEmpty()) {
                    val emptyTv = TextView(activity).apply {
                        tag = "empty_hint"
                        text = "还没有聊天记录，给「$peerName」发第一条消息吧~"
                        gravity = Gravity.CENTER
                        textSize = 13.5f
                        setTextColor(Color.parseColor("#8A7E6E"))
                        setPadding(dp(16), dp(48), dp(16), dp(48))
                    }
                    binding.chatMessagesContainer.addView(emptyTv)
                }
            }
            var added = false
            for (m in msgs) {
                if (renderedIds.add(m.id)) {
                    if (m.createdAt > lastMessageTs) lastMessageTs = m.createdAt
                    if (binding.chatMessagesContainer.findViewWithTag<View>("empty_hint") != null) {
                        binding.chatMessagesContainer.removeAllViews()
                    }
                    appendMessageBubble(m)
                    added = true
                }
            }
            if (added) {
                binding.chatScroll.post { binding.chatScroll.fullScroll(View.FOCUS_DOWN) }
                onMessagesChanged()
            }
        }
    }

    private fun appendMessageBubble(msg: ChatMessageItem) {
        val wrap = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            gravity = if (msg.isSelf) Gravity.END else Gravity.START
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(10) }
        }

        if (peerType == "group" && !msg.isSelf) {
            val senderRow = LinearLayout(activity).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(2), 0, dp(2), dp(3))
            }
            val sav = SpeciesThumbView(activity).apply {
                loadAvatar(api, msg.senderAvatarIdx, msg.senderAvatarUrl)
                layoutParams = LinearLayout.LayoutParams(dp(20), dp(20)).apply { marginEnd = dp(5) }
                setOnClickListener {
                    activity.showAvatarPreviewDialog(api, "${msg.senderName.ifBlank { "成员" }} 的头像", msg.senderAvatarIdx, msg.senderAvatarUrl)
                }
            }
            val sname = TextView(activity).apply {
                text = msg.senderName.ifBlank { "成员" }
                textSize = 11.5f
                setTextColor(Color.parseColor("#7D7263"))
            }
            senderRow.addView(sav)
            senderRow.addView(sname)
            wrap.addView(senderRow)
        }

        val bubble = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(12), dp(9), dp(12), dp(9))
            background = GradientDrawable().apply {
                cornerRadius = dp(14).toFloat()
                setColor(Color.parseColor(if (msg.isSelf) "#DFF2E6" else "#FFFFFF"))
                setStroke(dp(1), Color.parseColor(if (msg.isSelf) "#B8D9C4" else "#E6DFD1"))
            }
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.WRAP_CONTENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            )
        }

        when (msg.msgType) {
            "image" -> {
                val iv = ImageView(activity).apply {
                    scaleType = ImageView.ScaleType.CENTER_CROP
                    layoutParams = LinearLayout.LayoutParams(dp(190), dp(190))
                    setBackgroundColor(Color.parseColor("#EFECE5"))
                }
                bubble.addView(iv)
                if (msg.mediaUrl.isNotEmpty()) {
                    api.fetchBitmap(msg.mediaUrl) { bmp ->
                        if (bmp != null) {
                            iv.setImageBitmap(bmp)
                            iv.setOnClickListener { showImagePreviewDialog(bmp) }
                        }
                    }
                }
            }
            "voice" -> {
                val dur = msg.durationSec.coerceAtLeast(1)
                val idleLabel = "▶ 🔊 ${dur}″ 点击听语音"
                val btnVoice = Button(activity).apply {
                    text = idleLabel
                    textSize = 13.5f
                    setAllCaps(false)
                    setTextColor(Color.parseColor("#233128"))
                    setBackgroundColor(Color.TRANSPARENT)
                    setPadding(dp(4), 0, dp(4), 0)
                    minWidth = 0
                    minimumWidth = 0
                    minHeight = 0
                    layoutParams = LinearLayout.LayoutParams(
                        LinearLayout.LayoutParams.WRAP_CONTENT,
                        dp(34)
                    )
                }
                btnVoice.setOnClickListener {
                    playVoiceMessage(msg, btnVoice, idleLabel)
                }
                bubble.addView(btnVoice)
            }
            else -> {
                val tv = TextView(activity).apply {
                    text = msg.text
                    textSize = 15f
                    maxWidth = dp(280)
                    setTextColor(Color.parseColor("#233128"))
                    setLineSpacing(5f, 1.15f)
                }
                bubble.addView(tv)
            }
        }

        val fmt = SimpleDateFormat("HH:mm", Locale.getDefault())
        val timeTv = TextView(activity).apply {
            text = fmt.format(Date(msg.createdAt))
            textSize = 10.5f
            setTextColor(Color.parseColor("#9A8E7D"))
            setPadding(dp(4), dp(3), dp(4), 0)
        }

        wrap.addView(bubble)
        wrap.addView(timeTv)
        binding.chatMessagesContainer.addView(wrap)
    }

    private fun showImagePreviewDialog(bmp: Bitmap) {
        val iv = ImageView(activity).apply {
            setImageBitmap(bmp)
            adjustViewBounds = true
            setPadding(dp(8), dp(8), dp(8), dp(8))
        }
        AlertDialog.Builder(activity)
            .setView(iv)
            .setPositiveButton("关闭", null)
            .show()
    }

    private fun playVoiceMessage(msg: ChatMessageItem, btn: Button, idleLabel: String) {
        if (activeVoiceButton === btn && activePlayer?.isPlaying == true) {
            stopVoicePlayback()
            return
        }
        stopVoicePlayback()
        if (msg.mediaUrl.isBlank()) return
        btn.text = "⏳ 加载中..."
        val dest = File(activity.cacheDir, "voice_${msg.id}.m4a")
        api.fetchBinaryToFile(msg.mediaUrl, dest) { ok ->
            if (!ok || !dest.exists()) {
                btn.text = idleLabel
                Toast.makeText(activity, "语音加载失败", Toast.LENGTH_SHORT).show()
                return@fetchBinaryToFile
            }
            try {
                val mp = MediaPlayer().apply {
                    setAudioAttributes(
                        AudioAttributes.Builder()
                            .setUsage(AudioAttributes.USAGE_MEDIA)
                            .setContentType(AudioAttributes.CONTENT_TYPE_SPEECH)
                            .build()
                    )
                    setDataSource(dest.absolutePath)
                    setOnCompletionListener {
                        activity.runOnUiThread {
                            if (activeVoiceButton === btn) {
                                stopVoicePlayback()
                            }
                        }
                    }
                    prepare()
                    start()
                }
                activePlayer = mp
                activeVoiceButton = btn
                activeVoiceLabel = idleLabel
                btn.text = "⏹ 正在播放 ${msg.durationSec.coerceAtLeast(1)}″"
            } catch (_: Exception) {
                btn.text = idleLabel
                Toast.makeText(activity, "语音播放失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun stopVoicePlayback() {
        activeVoiceButton?.let { b ->
            if (activeVoiceLabel.isNotEmpty()) b.text = activeVoiceLabel
        }
        activeVoiceButton = null
        activeVoiceLabel = ""
        runCatching { activePlayer?.stop() }
        runCatching { activePlayer?.release() }
        activePlayer = null
    }

    private fun sendTextMessage() {
        if (peerId.isBlank()) return
        val txt = binding.etChatInput.text.toString().trim()
        if (txt.isEmpty()) return
        binding.btnChatSend.isEnabled = false
        val payload = JSONObject()
            .put("peerType", peerType)
            .put("peerId", peerId)
            .put("msgType", "text")
            .put("text", txt)
        api.request("POST", "/api/child/chat/messages", payload) { res ->
            binding.btnChatSend.isEnabled = true
            if (res.ok) {
                binding.etChatInput.setText("")
                sound.playPat()
                fetchMessages(incremental = true)
            } else {
                sound.playError()
                Toast.makeText(activity, res.error ?: "发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showImagePickChoice() {
        if (peerId.isBlank()) return
        val opts = arrayOf("📷 拍照发送", "🖼️ 从相册选图")
        AlertDialog.Builder(activity)
            .setTitle("发送图片")
            .setItems(opts) { _, idx ->
                if (idx == 0) {
                    if (ContextCompat.checkSelfPermission(activity, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
                        launchCameraPhoto()
                    } else {
                        onRequestCameraPermission()
                    }
                } else {
                    onLaunchGallery()
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun launchCameraPhoto() {
        val dir = File(activity.cacheDir, "chat_photos").apply { mkdirs() }
        val photoFile = File(dir, "chat_${System.currentTimeMillis()}.jpg")
        val uri = FileProvider.getUriForFile(activity, "${activity.packageName}.fileprovider", photoFile)
        pendingCameraUri = uri
        onLaunchCamera(uri)
    }

    private fun sendImageUri(uri: Uri) {
        if (peerId.isBlank()) return
        val dataUrl = compressChatImageUri(uri) ?: run {
            Toast.makeText(activity, "读取图片失败", Toast.LENGTH_SHORT).show()
            return
        }
        binding.btnChatImage.isEnabled = false
        val payload = JSONObject()
            .put("peerType", peerType)
            .put("peerId", peerId)
            .put("msgType", "image")
            .put("mimeType", "image/jpeg")
            .put("mediaData", dataUrl)
        api.request("POST", "/api/child/chat/messages", payload) { res ->
            binding.btnChatImage.isEnabled = true
            if (res.ok) {
                sound.playPat()
                fetchMessages(incremental = true)
            } else {
                sound.playError()
                Toast.makeText(activity, res.error ?: "图片发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun compressChatImageUri(uri: Uri): String? {
        return try {
            val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            activity.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
            val maxSide = 1280
            var sample = 1
            while (bounds.outWidth / sample > maxSide * 2 || bounds.outHeight / sample > maxSide * 2) {
                sample *= 2
            }
            val opts = BitmapFactory.Options().apply { inSampleSize = sample }
            var bmp = activity.contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, opts) }
                ?: return null

            val orientation = runCatching {
                activity.contentResolver.openInputStream(uri)?.use {
                    ExifInterface(it).getAttributeInt(ExifInterface.TAG_ORIENTATION, ExifInterface.ORIENTATION_NORMAL)
                } ?: ExifInterface.ORIENTATION_NORMAL
            }.getOrDefault(ExifInterface.ORIENTATION_NORMAL)

            val deg = when (orientation) {
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
            if (bmp.width > maxSide || bmp.height > maxSide) {
                val (w, h) = if (bmp.width >= bmp.height) {
                    maxSide to ((bmp.height * maxSide) / bmp.width).coerceAtLeast(1)
                } else {
                    ((bmp.width * maxSide) / bmp.height).coerceAtLeast(1) to maxSide
                }
                val scaled = Bitmap.createScaledBitmap(bmp, w, h, true)
                if (scaled != bmp) bmp.recycle()
                bmp = scaled
            }
            val out = ByteArrayOutputStream()
            bmp.compress(Bitmap.CompressFormat.JPEG, 82, out)
            bmp.recycle()
            "data:image/jpeg;base64," + Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP)
        } catch (_: Exception) {
            null
        }
    }

    private fun startVoiceRecording() {
        stopVoicePlayback()
        cancelVoiceRecording()
        val f = File(activity.cacheDir, "rec_${System.currentTimeMillis()}.m4a")
        try {
            @Suppress("DEPRECATION")
            val r = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) MediaRecorder(activity) else MediaRecorder()
            r.apply {
                setAudioSource(MediaRecorder.AudioSource.MIC)
                setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
                setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
                setAudioSamplingRate(22050)
                setAudioEncodingBitRate(32000)
                setMaxDuration(60_000)
                setOutputFile(f.absolutePath)
                prepare()
                start()
            }
            recorder = r
            recordingFile = f
            recordingStartMs = SystemClock.elapsedRealtime()
            isRecording = true
            binding.voiceRecordingBar.visibility = View.VISIBLE
            binding.tvVoiceRecTimer.text = "🎙️ 正在录音 00:01..."
            mainHandler.removeCallbacks(recTicker)
            mainHandler.postDelayed(recTicker, 400L)
        } catch (e: Exception) {
            runCatching { f.delete() }
            Toast.makeText(activity, "无法开启录音: ${e.message}", Toast.LENGTH_SHORT).show()
        }
    }

    fun cancelVoiceRecording() {
        isRecording = false
        mainHandler.removeCallbacks(recTicker)
        binding.voiceRecordingBar.visibility = View.GONE
        runCatching { recorder?.stop() }
        runCatching { recorder?.release() }
        recorder = null
        recordingFile?.let { runCatching { it.delete() } }
        recordingFile = null
    }

    private fun finishAndSendVoice() {
        if (!isRecording) return
        val dur = ((SystemClock.elapsedRealtime() - recordingStartMs) / 1000L).toInt().coerceIn(1, 60)
        isRecording = false
        mainHandler.removeCallbacks(recTicker)
        binding.voiceRecordingBar.visibility = View.GONE

        val stoppedOk = runCatching { recorder?.stop() }.isSuccess
        runCatching { recorder?.release() }
        recorder = null

        val f = recordingFile
        recordingFile = null
        if (!stoppedOk || f == null || !f.exists() || f.length() < 128L) {
            f?.let { runCatching { it.delete() } }
            Toast.makeText(activity, "录音时间太短，请再录一次", Toast.LENGTH_SHORT).show()
            return
        }

        val b64 = try {
            val bytes = f.readBytes()
            "data:audio/mp4;base64," + Base64.encodeToString(bytes, Base64.NO_WRAP)
        } catch (_: Exception) {
            null
        } finally {
            runCatching { f.delete() }
        }
        if (b64 == null) return

        val payload = JSONObject()
            .put("peerType", peerType)
            .put("peerId", peerId)
            .put("msgType", "voice")
            .put("mimeType", "audio/mp4")
            .put("durationSec", dur)
            .put("mediaData", b64)
        api.request("POST", "/api/child/chat/messages", payload) { res ->
            if (res.ok) {
                sound.playPat()
                fetchMessages(incremental = true)
            } else {
                sound.playError()
                Toast.makeText(activity, res.error ?: "语音发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showGroupInfoDialog() {
        if (peerType != "group" || peerId.isBlank()) return
        api.request("GET", "/api/child/groups/$peerId") { res ->
            if (!res.ok || res.json == null) {
                Toast.makeText(activity, res.error ?: "读取群聊详情失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val gObj = res.json.optJSONObject("group") ?: return@request
            val grp = SocialParser.parseGroupItem(gObj)
            binding.tvChatPeerName.text = grp.name
            binding.tvChatPeerMeta.text = (if (grp.isFamilyDefault) "家庭群" else "群聊") + " · ${grp.memberCount}位成员 · 文字 / 语音 / 图片"

            val wrap = LinearLayout(activity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(18), dp(12), dp(18), dp(6))
            }
            val subTip = TextView(activity).apply {
                text = if (grp.isFamilyDefault) {
                    "👨‍👩‍👧‍👦 默认家庭群（群名与家庭名保持同步，家庭成员自动入群）"
                } else {
                    "👥 好友群聊（共 ${grp.memberCount} 位成员）"
                }
                textSize = 12.5f
                setTextColor(Color.parseColor("#6B6052"))
                setPadding(0, 0, 0, dp(10))
            }
            wrap.addView(subTip)

            val listCol = LinearLayout(activity).apply { orientation = LinearLayout.VERTICAL }
            grp.members.forEach { m ->
                val row = LinearLayout(activity).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(0, dp(6), 0, dp(6))
                }
                val av = SpeciesThumbView(activity).apply {
                    loadAvatar(api, m.avatarIdx, m.avatarUrl)
                    layoutParams = LinearLayout.LayoutParams(dp(34), dp(34)).apply { marginEnd = dp(10) }
                    setOnClickListener {
                        activity.showAvatarPreviewDialog(api, "${m.name} 的头像", m.avatarIdx, m.avatarUrl)
                    }
                }
                val nm = TextView(activity).apply {
                    val ownerTag = if (m.role == "owner" && !grp.isFamilyDefault) " · 群主" else ""
                    val onlineStr = if (m.isOnline) "🟢 在线" else "⚪ 离线"
                    text = "${m.name}（${m.roleLabel}$ownerTag） · $onlineStr"
                    textSize = 13.5f
                    setTextColor(Color.parseColor("#233128"))
                }
                row.addView(av)
                row.addView(nm)
                listCol.addView(row)
            }
            val sv = ScrollView(activity).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    dp(220)
                )
                addView(listCol)
            }
            wrap.addView(sv)

            val builder = AlertDialog.Builder(activity)
                .setTitle("群聊详情 · ${grp.name}")
                .setView(wrap)
                .setPositiveButton("关闭", null)
            if (!grp.isFamilyDefault) {
                builder.setNeutralButton("➕ 邀请好友") { _, _ ->
                    showInviteGroupMembersDialog(grp)
                }
                builder.setNegativeButton("退出群聊") { _, _ ->
                    confirmLeaveGroup(grp)
                }
            }
            builder.show()
        }
    }

    private fun showInviteGroupMembersDialog(grp: ChatGroupItem) {
        api.request("GET", "/api/child/friends") { res ->
            if (!res.ok || res.json == null) {
                Toast.makeText(activity, res.error ?: "读取好友列表失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val ov = SocialParser.parseOverview(res.json)
            val existingKeys = grp.members.map { "${it.memberType}:${it.memberId}" }.toSet()
            val candidates = ov.friends.filter { "${it.peerType}:${it.peerId}" !in existingKeys }
            if (candidates.isEmpty()) {
                Toast.makeText(activity, "所有好友和家人都已在群里啦~", Toast.LENGTH_SHORT).show()
                return@request
            }

            val checksWrap = LinearLayout(activity).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(18), dp(12), dp(18), dp(6))
            }
            val picks = mutableListOf<Pair<FriendItem, CheckBox>>()
            candidates.forEach { f ->
                val cb = CheckBox(activity).apply {
                    text = "${f.name}（${f.roleLabel.ifBlank { f.groupName }}）"
                    textSize = 14f
                    setTextColor(Color.parseColor("#233128"))
                }
                checksWrap.addView(cb)
                picks.add(f to cb)
            }
            val sv = ScrollView(activity).apply {
                layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(220))
                addView(checksWrap)
            }
            AlertDialog.Builder(activity)
                .setTitle("邀请好友加入「${grp.name}」")
                .setView(sv)
                .setPositiveButton("确定邀请") { _, _ ->
                    val arr = JSONArray()
                    picks.forEach { (f, cb) ->
                        if (cb.isChecked) {
                            arr.put(JSONObject().put("memberType", f.peerType).put("memberId", f.peerId))
                        }
                    }
                    if (arr.length() == 0) return@setPositiveButton
                    api.request(
                        "POST",
                        "/api/child/groups/${grp.id}/members",
                        JSONObject().put("members", arr)
                    ) { invRes ->
                        if (invRes.ok) {
                            sound.playWin()
                            Toast.makeText(activity, "已邀请好友加入群聊！", Toast.LENGTH_SHORT).show()
                            showGroupInfoDialog()
                        } else {
                            sound.playError()
                            Toast.makeText(activity, invRes.error ?: "邀请失败", Toast.LENGTH_SHORT).show()
                        }
                    }
                }
                .setNegativeButton("取消", null)
                .show()
        }
    }

    private fun confirmLeaveGroup(grp: ChatGroupItem) {
        AlertDialog.Builder(activity)
            .setTitle("退出群聊")
            .setMessage("确定要退出群聊「${grp.name}」吗？")
            .setPositiveButton("确定退出") { _, _ ->
                api.request("POST", "/api/child/groups/${grp.id}/leave", JSONObject()) { res ->
                    if (res.ok) {
                        Toast.makeText(activity, "已退出群聊", Toast.LENGTH_SHORT).show()
                        onGroupLeft()
                    } else {
                        Toast.makeText(activity, res.error ?: "退出失败", Toast.LENGTH_SHORT).show()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }
}
