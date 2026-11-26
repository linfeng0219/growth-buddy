package com.growthbuddy.child

import android.Manifest
import android.content.res.Configuration
import android.os.Bundle
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityChatBinding

/**
 * 好友即时聊天页（手机 / 平板竖屏独立页面；内部委托 ChildChatPaneController 渲染与收发）。
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
        const val EXTRA_PEER_ONLINE = "extra_peer_online"
    }

    private lateinit var binding: ActivityChatBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private lateinit var controller: ChildChatPaneController

    private val audioPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        controller.onAudioPermissionResult(granted)
    }

    private val cameraPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        controller.onCameraPermissionResult(granted)
    }

    private val galleryImgLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        controller.onImagePicked(uri)
    }

    private val cameraPhotoLauncher = registerForActivityResult(
        ActivityResultContracts.TakePicture()
    ) { ok ->
        controller.onCameraCaptured(ok)
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityChatBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)

        val peerType = intent.getStringExtra(EXTRA_PEER_TYPE) ?: "child"
        val peerId = intent.getStringExtra(EXTRA_PEER_ID) ?: ""
        val peerName = intent.getStringExtra(EXTRA_PEER_NAME) ?: "好友"
        val peerGroup = intent.getStringExtra(EXTRA_PEER_GROUP) ?: "朋友"
        val peerRole = intent.getStringExtra(EXTRA_PEER_ROLE) ?: ""
        val avIdx = intent.getIntExtra(EXTRA_PEER_AVATAR_IDX, 0)
        val avUrl = intent.getStringExtra(EXTRA_PEER_AVATAR_URL) ?: ""
        val isOnline = intent.getBooleanExtra(EXTRA_PEER_ONLINE, false)

        if (peerId.isBlank()) {
            finish()
            return
        }

        controller = ChildChatPaneController(
            activity = this,
            binding = binding.chatPane,
            api = api,
            sound = sound,
            onRequestAudioPermission = { audioPermLauncher.launch(Manifest.permission.RECORD_AUDIO) },
            onRequestCameraPermission = { cameraPermLauncher.launch(Manifest.permission.CAMERA) },
            onLaunchGallery = { galleryImgLauncher.launch("image/*") },
            onLaunchCamera = { uri -> cameraPhotoLauncher.launch(uri) },
            onBackClicked = { finish() },
            onGroupLeft = { finish() }
        )
        controller.setBackButtonVisible(true)
        controller.openPeer(
            peerType = peerType,
            peerId = peerId,
            peerName = peerName,
            peerGroup = peerGroup,
            peerRole = peerRole,
            avatarIdx = avIdx,
            avatarUrl = avUrl,
            isOnline = isOnline
        )

        applyResponsiveLayout()
    }

    override fun onResume() {
        super.onResume()
        controller.setActive(true)
        controller.fetchMessages(incremental = true)
    }

    override fun onPause() {
        controller.onPause()
        super.onPause()
    }

    override fun onDestroy() {
        controller.onDestroy()
        super.onDestroy()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 960 else if (m.isTablet) 720 else 1200
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatPane.chatTopBar, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatPane.chatScroll, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatPane.voiceRecordingBar, maxW, horizontalReserveDp = 24)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.chatPane.chatInputBar, maxW, horizontalReserveDp = 24)
    }
}
