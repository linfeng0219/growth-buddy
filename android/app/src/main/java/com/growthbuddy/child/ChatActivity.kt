package com.growthbuddy.child

import android.Manifest
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.Matrix
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.media.AudioAttributes
import android.media.ExifInterface
import android.media.MediaPlayer
import android.media.MediaRecorder
import android.net.Uri
import android.os.Build
import android.os.Bundle
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
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import com.growthbuddy.child.databinding.ActivityChatBinding
import org.json.JSONArray
import org.json.JSONObject
import java.io.ByteArrayOutputStream
import java.io.File
import java.net.URLEncoder
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 好友即时聊天页（支持发送/接收 文字、语音、图片消息）：
 * - 每 3.5 秒自动增量拉取新消息，支持语音缓存播放、图片点击放大预览。
 */
class ChatActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_PEER_TYPE = "extra_peer_type"
        const val EXTRA_PEER_ID = "extra_peer_id"
        const val EXTRA_PEER_NAME = "extra_peer_name"
        const val EXTRA_PEER_GROUP = "extra_peer_group"
        const val EXTRA_PEER_ROLE = "extra_peer_role"
        const val EXTRA_PEER_AVATAR_IDX = "extra_peer_avatar_idx"
        const val EXTRA_PEER_AVATAR_URL = "extra_peer_avatar_url"
    }

    private lateinit var binding: ActivityChatBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine

    private var peerType: String = "child"
    private var peerId: String = ""
    private var peerName: String = "好友"
    private var peerGroup: String = "朋友"
    private var peerRole: String = ""

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

    private val audioPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) startVoiceRecording()
        else Toast.makeText(this, "需要麦克风权限才能发送语音消息", Toast.LENGTH_LONG).show()
    }

    private val cameraPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) launchCameraPhoto()
        else Toast.makeText(this, "需要摄像头权限才能拍照发图", Toast.LENGTH_SHORT).show()
    }

    private val galleryImgLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri != null) sendImageUri(uri)
    }

    private val cameraPhotoLauncher = registerForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { ok ->
        val uri = pendingCameraUri
        pendingCameraUri = null
        if (ok && uri != null) sendImageUri(uri)
    }

    private val pollTicker = object : Runnable {
        override fun run() {
            if (!isResumed) return
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

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityChatBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)

        peerType = intent.getStringExtra(EXTRA_PEER_TYPE) ?: "child"
        peerId = intent.getStringExtra(EXTRA_PEER_ID) ?: ""
        peerName = intent.getStringExtra(EXTRA_PEER_NAME) ?: "好友"
        peerGroup = intent.getStringExtra(EXTRA_PEER_GROUP) ?: "朋友"
        peerRole = intent.getStringExtra(EXTRA_PEER_ROLE) ?: ""
        val avIdx = intent.getIntExtra(EXTRA_PEER_AVATAR_IDX, 0)
        val avUrl = intent.getStringExtra(EXTRA_PEER_AVATAR_URL) ?: ""

        if (peerId.isBlank()) {
            finish()
            return
        }

        binding.tvChatPeerName.text = peerName
        if (peerType == "group") {
            binding.ivChatPeerAvatar.visibility = View.GONE
            binding.tvChatGroupBadge.visibility = View.VISIBLE
            binding.tvChatGroupBadge.text = if (peerGroup == "家庭群") "👨‍👩‍👧‍👦" else "👥"
            binding.btnChatGroupInfo.visibility = View.VISIBLE
            binding.btnChatGroupInfo.setOnClickListener { showGroupInfoDialog() }
            binding.tvChatPeerMeta.text = "$peerGroup · $peerRole · 文字 / 语音 / 图片"
        } else {
            val roleTag = if (peerRole.isNotBlank()) " · $peerRole" else ""
            binding.tvChatPeerMeta.text = "$peerGroup 分组$roleTag · 文字 / 语音 / 图片"
            binding.ivChatPeerAvatar.loadAvatar(api, avIdx, avUrl)
        }

        binding.btnChatBack.setOnClickListener { finish() }
        binding.btnChatSend.setOnClickListener { sendTextMessage() }
        binding.etChatInput.setOnEditorActionListener { _, actionId, _ ->
            if (actionId == EditorInfo.IME_ACTION_SEND) {
                sendTextMessage()
                true
            } else false
        }
        binding.btnChatImage.setOnClickListener { showImagePickChoice() }
        binding.btnChatVoice.setOnClickListener {
            if (isRecording) {
                finishAndSendVoice()
            } else if (ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) == PackageManager.PERMISSION_GRANTED) {
                startVoiceRecording()
            } else {
                audioPermLauncher.launch(Manifest.permission.RECORD_AUDIO)
            }
        }
        binding.btnCancelVoice.setOnClickListener { cancelVoiceRecording() }
        binding.btnSendVoiceNow.setOnClickListener { finishAndSendVoice() }

        applyResponsiveLayout()
        fetchMessages(incremental = false)
    }

    override fun onResume() {
        super.onResume()
        isResumed = true
        mainHandler.removeCallbacks(pollTicker)
        mainHandler.postDelayed(pollTicker, 3500L)
    }

    override fun onPause() {
        isResumed = false
        mainHandler.removeCallbacks(pollTicker)
        super.onPause()
    }

    override fun onDestroy() {
        cancelVoiceRecording()
        stopVoicePlayback()
        super.onDestroy()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun dp(v: Int): Int = ResponsiveUi.dp(this, v)

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 960 else if (m.isTablet) 720 else 1200
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatTopBar, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatScroll, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.voiceRecordingBar, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatInputBar, maxW, horizontalReserveDp = 24)
    }

    private fun fetchMessages(incremental: Boolean) {
        val pType = URLEncoder.encode(peerType, "UTF-8")
        val pId = URLEncoder.encode(peerId, "UTF-8")
        val after = if (incremental && lastMessageTs > 0) "&after=$lastMessageTs" else ""
        api.request("GET", "/api/child/chat/messages?peerType=$pType&peerId=$pId$after") { res ->
            if (!res.ok || res.json == null) return@request
            val arr = res.json.optJSONArray("messages") ?: JSONArray()
            val msgs = SocialParser.parseMessages(arr)
            if (!incremental && renderedIds.isEmpty()) {
                binding.chatMessagesContainer.removeAllViews()
                if (msgs.isEmpty()) {
                    val emptyTv = TextView(this).apply {
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
            }
        }
    }

    private fun appendMessageBubble(msg: ChatMessageItem) {
        val wrap = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = if (msg.isSelf) Gravity.END else Gravity.START
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { bottomMargin = dp(10) }
        }

        if (peerType == "group" && !msg.isSelf) {
            val senderRow = LinearLayout(this).apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER_VERTICAL
                setPadding(dp(2), 0, dp(2), dp(4))
            }
            val senderAv = SpeciesThumbView(this).apply {
                loadAvatar(api, msg.senderAvatarIdx, msg.senderAvatarUrl)
                layoutParams = LinearLayout.LayoutParams(dp(18), dp(18)).apply { marginEnd = dp(5) }
            }
            val senderTv = TextView(this).apply {
                text = msg.senderName.ifBlank { "群成员" }
                textSize = 11.5f
                setTextColor(Color.parseColor("#6B6052"))
                setTypeface(null, Typeface.BOLD)
            }
            senderRow.addView(senderAv)
            senderRow.addView(senderTv)
            wrap.addView(senderRow)
        }

        val bubble = LinearLayout(this).apply {
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
                val iv = ImageView(this).apply {
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
                val btnVoice = Button(this).apply {
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
                val tv = TextView(this).apply {
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
        val timeTv = TextView(this).apply {
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
        val iv = ImageView(this).apply {
            setImageBitmap(bmp)
            adjustViewBounds = true
            setPadding(dp(8), dp(8), dp(8), dp(8))
        }
        AlertDialog.Builder(this)
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
        val dest = File(cacheDir, "voice_${msg.id}.m4a")
        api.fetchBinaryToFile(msg.mediaUrl, dest) { ok ->
            if (!ok || !dest.exists()) {
                btn.text = idleLabel
                Toast.makeText(this, "语音加载失败", Toast.LENGTH_SHORT).show()
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
                        runOnUiThread {
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
                Toast.makeText(this, "语音播放失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun stopVoicePlayback() {
        activeVoiceButton?.let { b ->
            if (activeVoiceLabel.isNotEmpty()) b.text = activeVoiceLabel
        }
        activeVoiceButton = null
        activeVoiceLabel = ""
        runCatching {
            activePlayer?.stop()
        }
        runCatching {
            activePlayer?.release()
        }
        activePlayer = null
    }

    private fun sendTextMessage() {
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
                Toast.makeText(this, res.error ?: "发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showImagePickChoice() {
        val opts = arrayOf("📷 拍照发送", "🖼️ 从相册选图")
        AlertDialog.Builder(this)
            .setTitle("发送图片")
            .setItems(opts) { _, idx ->
                if (idx == 0) {
                    if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
                        launchCameraPhoto()
                    } else {
                        cameraPermLauncher.launch(Manifest.permission.CAMERA)
                    }
                } else {
                    galleryImgLauncher.launch("image/*")
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun launchCameraPhoto() {
        val dir = File(cacheDir, "chat_photos").apply { mkdirs() }
        val photoFile = File(dir, "chat_${System.currentTimeMillis()}.jpg")
        val uri = FileProvider.getUriForFile(this, "${packageName}.fileprovider", photoFile)
        pendingCameraUri = uri
        cameraPhotoLauncher.launch(uri)
    }

    private fun sendImageUri(uri: Uri) {
        val dataUrl = compressChatImageUri(uri) ?: run {
            Toast.makeText(this, "读取图片失败", Toast.LENGTH_SHORT).show()
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
                Toast.makeText(this, res.error ?: "图片发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun compressChatImageUri(uri: Uri): String? {
        return try {
            val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
            contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, bounds) }
            val maxSide = 1280
            var sample = 1
            while (bounds.outWidth / sample > maxSide * 2 || bounds.outHeight / sample > maxSide * 2) {
                sample *= 2
            }
            val opts = BitmapFactory.Options().apply { inSampleSize = sample }
            var bmp = contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it, null, opts) }
                ?: return null

            val orientation = runCatching {
                contentResolver.openInputStream(uri)?.use {
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
        val f = File(cacheDir, "rec_${System.currentTimeMillis()}.m4a")
        try {
            @Suppress("DEPRECATION")
            val r = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) MediaRecorder(this) else MediaRecorder()
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
            Toast.makeText(this, "无法开启录音: ${e.message}", Toast.LENGTH_SHORT).show()
        }
    }

    private fun cancelVoiceRecording() {
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
            Toast.makeText(this, "录音时间太短，请再录一次", Toast.LENGTH_SHORT).show()
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
                Toast.makeText(this, res.error ?: "语音发送失败", Toast.LENGTH_SHORT).show()
            }
        }
    }

    private fun showGroupInfoDialog() {
        api.request("GET", "/api/child/groups/$peerId") { res ->
            if (!res.ok || res.json == null) {
                Toast.makeText(this, res.error ?: "读取群聊详情失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val gObj = res.json.optJSONObject("group") ?: return@request
            val grp = SocialParser.parseGroupItem(gObj)
            binding.tvChatPeerName.text = grp.name
            binding.tvChatPeerMeta.text = (if (grp.isFamilyDefault) "家庭群" else "群聊") + " · ${grp.memberCount}位成员 · 文字 / 语音 / 图片"

            val wrap = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(18), dp(12), dp(18), dp(6))
            }
            val subTip = TextView(this).apply {
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

            val listCol = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
            grp.members.forEach { m ->
                val row = LinearLayout(this).apply {
                    orientation = LinearLayout.HORIZONTAL
                    gravity = Gravity.CENTER_VERTICAL
                    setPadding(0, dp(6), 0, dp(6))
                }
                val av = SpeciesThumbView(this).apply {
                    loadAvatar(api, m.avatarIdx, m.avatarUrl)
                    layoutParams = LinearLayout.LayoutParams(dp(34), dp(34)).apply { marginEnd = dp(10) }
                }
                val nm = TextView(this).apply {
                    val ownerTag = if (m.role == "owner" && !grp.isFamilyDefault) " · 群主" else ""
                    text = "${m.name}（${m.roleLabel}$ownerTag）"
                    textSize = 14f
                    setTextColor(Color.parseColor("#233128"))
                }
                row.addView(av)
                row.addView(nm)
                listCol.addView(row)
            }
            val sv = ScrollView(this).apply {
                layoutParams = LinearLayout.LayoutParams(
                    LinearLayout.LayoutParams.MATCH_PARENT,
                    dp(220)
                )
                addView(listCol)
            }
            wrap.addView(sv)

            val builder = AlertDialog.Builder(this)
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
                Toast.makeText(this, res.error ?: "读取好友列表失败", Toast.LENGTH_SHORT).show()
                return@request
            }
            val ov = SocialParser.parseOverview(res.json)
            val existingKeys = grp.members.map { "${it.memberType}:${it.memberId}" }.toSet()
            val candidates = ov.friends.filter { "${it.peerType}:${it.peerId}" !in existingKeys }
            if (candidates.isEmpty()) {
                Toast.makeText(this, "所有好友和家人都已在群里啦~", Toast.LENGTH_SHORT).show()
                return@request
            }

            val checksWrap = LinearLayout(this).apply {
                orientation = LinearLayout.VERTICAL
                setPadding(dp(18), dp(12), dp(18), dp(6))
            }
            val picks = mutableListOf<Pair<FriendItem, CheckBox>>()
            candidates.forEach { f ->
                val cb = CheckBox(this).apply {
                    text = "${f.name}（${f.roleLabel.ifBlank { f.groupName }}）"
                    textSize = 14f
                    setTextColor(Color.parseColor("#233128"))
                }
                checksWrap.addView(cb)
                picks.add(f to cb)
            }
            val sv = ScrollView(this).apply {
                layoutParams = LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(220))
                addView(checksWrap)
            }
            AlertDialog.Builder(this)
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
                            Toast.makeText(this, "已邀请好友加入群聊！", Toast.LENGTH_SHORT).show()
                            showGroupInfoDialog()
                        } else {
                            sound.playError()
                            Toast.makeText(this, invRes.error ?: "邀请失败", Toast.LENGTH_SHORT).show()
                        }
                    }
                }
                .setNegativeButton("取消", null)
                .show()
        }
    }

    private fun confirmLeaveGroup(grp: ChatGroupItem) {
        AlertDialog.Builder(this)
            .setTitle("退出群聊")
            .setMessage("确定要退出群聊「${grp.name}」吗？")
            .setPositiveButton("确定退出") { _, _ ->
                api.request("POST", "/api/child/groups/${grp.id}/leave", JSONObject()) { res ->
                    if (res.ok) {
                        Toast.makeText(this, "已退出群聊", Toast.LENGTH_SHORT).show()
                        finish()
                    } else {
                        Toast.makeText(this, res.error ?: "退出失败", Toast.LENGTH_SHORT).show()
                    }
                }
            }
            .setNegativeButton("取消", null)
            .show()
    }
}
