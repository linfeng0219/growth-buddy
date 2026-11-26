package com.growthbuddy.child

import android.content.res.Configuration
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityMailboxBinding

/**
 * 成长信箱独立壳（兼容直接唤起信箱的场景，核心内容由 [ChildMailboxPaneController] 承载）。
 */
class MailboxActivity : AppCompatActivity() {

    private lateinit var binding: ActivityMailboxBinding
    private lateinit var api: ApiClient
    private lateinit var sound: NativeSoundEngine
    private lateinit var controller: ChildMailboxPaneController
    private var hasChangedState = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityMailboxBinding.inflate(layoutInflater)
        setContentView(binding.root)

        api = ApiClient(this)
        sound = NativeSoundEngine(this)
        controller = ChildMailboxPaneController(
            activity = this,
            binding = binding.mailboxPane,
            api = api,
            sound = sound,
            onChanged = {
                hasChangedState = true
                setResult(RESULT_OK)
            }
        )

        binding.btnMailBack.setOnClickListener {
            if (hasChangedState) setResult(RESULT_OK)
            finish()
        }
        binding.btnWriteLetter.setOnClickListener {
            sound.playPat()
            controller.showComposeDialog()
        }

        applyResponsiveLayout()
        controller.loadLetters()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
        controller.rerenderCached()
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val maxW = if (m.isDesktopSplit) 1040 else if (m.isTablet) 740 else 1200
        ResponsiveUi.applyCenteredMaxWidth(this, binding.mailTopBar, maxW, horizontalReserveDp = 28)
        ResponsiveUi.applyCenteredMaxWidth(this, binding.mailboxPane.root, maxW, horizontalReserveDp = 28)
    }

    override fun onDestroy() {
        sound.release()
        api.shutdown()
        super.onDestroy()
    }
}
