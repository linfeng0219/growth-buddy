package com.growthbuddy.child

import android.app.Activity
import android.content.Intent
import android.content.res.Configuration
import android.os.Bundle
import android.os.CountDownTimer
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.LinearLayout
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityFocusBinding

/**
 * 沉浸式番茄专注页：
 * - 支持绑定任务（到点自动返回并触发打卡）与自由专注（10/15/25 分钟）。
 * - 保持屏幕常亮，返回时二次确认防止误触退出。
 */
class FocusActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_TASK_ID = "task_id"
        const val EXTRA_TASK_TITLE = "task_title"
        const val EXTRA_DURATION_MIN = "duration_min"
        const val EXTRA_DURATION_SEC = "duration_sec"
        const val EXTRA_TIMER_MODE = "timer_mode"
        const val EXTRA_ACTUAL_DURATION = "actual_duration"
        const val EXTRA_SPECIES_ITEM = "species_item"
        const val EXTRA_PET_STAGE = "pet_stage"
        const val EXTRA_PET_ASLEEP = "pet_asleep"
        const val EXTRA_PET_NAME = "pet_name"
    }

    private lateinit var binding: ActivityFocusBinding
    private lateinit var sound: NativeSoundEngine

    private var taskId: String? = null
    private var timerMode: String = "countdown"
    private var selectedMinutes: Int = 15
    private var totalMillis: Long = 15 * 60_000L
    private var remainingMillis: Long = totalMillis
    private var elapsedMillis: Long = 0L
    private var countupStartMs: Long = 0L

    private var timer: CountDownTimer? = null
    private val countupHandler = Handler(Looper.getMainLooper())
    private var isRunning = false
    private var isFinished = false
    private var petDisplayName: String = "小伙伴"

    private val countupTicker = object : Runnable {
        override fun run() {
            if (!isRunning || timerMode != "countup") return
            elapsedMillis = SystemClock.elapsedRealtime() - countupStartMs
            updateTimerUi()
            countupHandler.postDelayed(this, 250L)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        binding = ActivityFocusBinding.inflate(layoutInflater)
        setContentView(binding.root)

        sound = NativeSoundEngine(this)

        taskId = intent.getStringExtra(EXTRA_TASK_ID)?.takeIf { it.isNotBlank() }
        val title = intent.getStringExtra(EXTRA_TASK_TITLE)?.takeIf { it.isNotBlank() } ?: "自由专注"
        timerMode = intent.getStringExtra(EXTRA_TIMER_MODE)?.takeIf { it == "countup" } ?: "countdown"
        val extraSec = intent.getIntExtra(EXTRA_DURATION_SEC, 0)
        selectedMinutes = intent.getIntExtra(EXTRA_DURATION_MIN, 15).coerceIn(1, 120)
        totalMillis = if (extraSec > 0) extraSec * 1000L else selectedMinutes * 60_000L
        if (extraSec > 0) selectedMinutes = (extraSec / 60).coerceAtLeast(1)
        remainingMillis = totalMillis

        binding.tvFocusTitle.text = "⏱️ $title"

        val petStage = intent.getIntExtra(EXTRA_PET_STAGE, 0).coerceIn(0, 5)
        val petAsleep = intent.getBooleanExtra(EXTRA_PET_ASLEEP, false)
        petDisplayName = intent.getStringExtra(EXTRA_PET_NAME)?.takeIf { it.isNotBlank() } ?: "你的宠物"
        @Suppress("DEPRECATION")
        val speciesItem = intent.getSerializableExtra(EXTRA_SPECIES_ITEM) as? SpeciesItem
        binding.focusPetStage.setPetState(petStage, isSleep = petAsleep, activeComp = speciesItem)

        if (taskId != null || timerMode == "countup") {
            binding.focusDurationRow.visibility = View.GONE
        } else {
            binding.btnDur10.setOnClickListener { selectDuration(10) }
            binding.btnDur15.setOnClickListener { selectDuration(15) }
            binding.btnDur25.setOnClickListener { selectDuration(25) }
        }
        if (timerMode == "countup") {
            binding.pbFocusProgress.visibility = View.GONE
            binding.btnFocusAction.text = "▶ 开始正计时"
        }

        binding.btnFocusExit.setOnClickListener { handleExitRequest() }
        binding.btnFocusAction.setOnClickListener {
            when {
                isFinished -> finishWithSuccess()
                isRunning && timerMode == "countup" -> finishCountup()
                isRunning -> pauseFocus()
                else -> startFocus()
            }
        }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                handleExitRequest()
            }
        })

        updateTimerUi()
        applyResponsiveLayout()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyResponsiveLayout()
    }

    private fun applyResponsiveLayout() {
        val m = ResponsiveUi.metrics(this)
        val dp = { v: Int -> ResponsiveUi.dp(this, v) }
        ResponsiveUi.applyCenteredMaxWidth(this, binding.focusTopBar, 1080, horizontalReserveDp = 32)

        val leftLp = binding.focusLeftPane.layoutParams as LinearLayout.LayoutParams
        val rightLp = binding.focusRightPane.layoutParams as LinearLayout.LayoutParams
        val stageLp = binding.focusPetStage.layoutParams
        val spacerLp = binding.focusSpacer.layoutParams as LinearLayout.LayoutParams

        if (m.isDesktopSplit) {
            binding.focusBodySplit.orientation = LinearLayout.HORIZONTAL
            binding.focusBodySplit.gravity = Gravity.CENTER_VERTICAL
            ResponsiveUi.applyCenteredMaxWidth(this, binding.focusBodySplit, 1020, horizontalReserveDp = 32)
            leftLp.width = 0
            leftLp.weight = 0.95f
            rightLp.width = 0
            rightLp.weight = 1.05f
            rightLp.marginStart = dp(24)
            rightLp.topMargin = 0
            stageLp.height = dp(268)
            spacerLp.weight = 0f
            spacerLp.height = dp(18)
            binding.tvFocusTimer.setTextSize(TypedValue.COMPLEX_UNIT_SP, 72f)
        } else {
            binding.focusBodySplit.orientation = LinearLayout.VERTICAL
            binding.focusBodySplit.gravity = Gravity.CENTER_HORIZONTAL
            ResponsiveUi.applyCenteredMaxWidth(
                this,
                binding.focusBodySplit,
                if (m.isTablet) 600 else 1000,
                horizontalReserveDp = 32
            )
            leftLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            leftLp.weight = 0f
            rightLp.width = ViewGroup.LayoutParams.MATCH_PARENT
            rightLp.weight = 0f
            rightLp.marginStart = 0
            rightLp.topMargin = dp(6)
            stageLp.height = dp(if (m.isTablet) 240 else if (m.isLandscape) 136 else 200)
            spacerLp.weight = 1f
            spacerLp.height = 0
            binding.tvFocusTimer.setTextSize(
                TypedValue.COMPLEX_UNIT_SP,
                if (m.isTablet) 64f else if (m.isLandscape) 44f else 54f
            )
        }
        binding.focusLeftPane.layoutParams = leftLp
        binding.focusRightPane.layoutParams = rightLp
        binding.focusPetStage.layoutParams = stageLp
        binding.focusSpacer.layoutParams = spacerLp
    }

    private fun selectDuration(minutes: Int) {
        if (isRunning || isFinished) return
        selectedMinutes = minutes
        totalMillis = minutes * 60_000L
        remainingMillis = totalMillis
        sound.playPat()
        updateTimerUi()
    }

    private fun startFocus() {
        isRunning = true
        binding.focusDurationRow.visibility = View.GONE
        binding.tvFocusBubble.text = "嘘——${petDisplayName}正在陪你一起认真专注，加油！"
        sound.playStart()
        sound.speak("开始专注，加油")

        if (timerMode == "countup") {
            binding.btnFocusAction.text = "✅ 完成专注并打卡"
            countupStartMs = SystemClock.elapsedRealtime() - elapsedMillis
            countupHandler.removeCallbacks(countupTicker)
            countupHandler.post(countupTicker)
            return
        }

        binding.btnFocusAction.text = "⏸ 暂停专注"
        timer?.cancel()
        timer = object : CountDownTimer(remainingMillis, 250L) {
            override fun onTick(millisUntilFinished: Long) {
                remainingMillis = millisUntilFinished
                updateTimerUi()
            }

            override fun onFinish() {
                remainingMillis = 0L
                isRunning = false
                isFinished = true
                updateTimerUi()
                onFocusCompleted()
            }
        }.start()
    }

    private fun finishCountup() {
        countupHandler.removeCallbacks(countupTicker)
        elapsedMillis = SystemClock.elapsedRealtime() - countupStartMs
        isRunning = false
        isFinished = true
        updateTimerUi()
        onFocusCompleted()
    }

    private fun pauseFocus() {
        timer?.cancel()
        countupHandler.removeCallbacks(countupTicker)
        isRunning = false
        binding.btnFocusAction.text = "▶ 继续专注"
        binding.tvFocusBubble.text = "休息一下吗？准备好就点继续哦~"
    }

    private fun onFocusCompleted() {
        binding.pbFocusProgress.progress = 1000
        binding.focusPetStage.triggerPatAnimation()
        sound.playWin()
        sound.speak("太棒了，专注挑战完成！")
        val doneMins = if (timerMode == "countup") {
            ((elapsedMillis / 1000L) / 60L).toInt().coerceAtLeast(1)
        } else selectedMinutes
        binding.tvFocusBubble.text = "🎉 太棒了！你完成了专注挑战（约 $doneMins 分钟）！"
        binding.btnFocusAction.text = if (taskId != null) "✅ 完成并返回打卡" else "✅ 完成并返回"
    }

    private fun updateTimerUi() {
        if (timerMode == "countup") {
            val totalSec = (elapsedMillis / 1000L).toInt().coerceAtLeast(0)
            binding.tvFocusTimer.text = String.format("%02d:%02d", totalSec / 60, totalSec % 60)
            return
        }
        val totalSec = ((remainingMillis + 999L) / 1000L).toInt().coerceAtLeast(0)
        val min = totalSec / 60
        val sec = totalSec % 60
        binding.tvFocusTimer.text = String.format("%02d:%02d", min, sec)
        val progress = if (totalMillis > 0L) {
            (((totalMillis - remainingMillis).toFloat() / totalMillis.toFloat()) * 1000f).toInt().coerceIn(0, 1000)
        } else 0
        binding.pbFocusProgress.progress = progress
    }

    private fun handleExitRequest() {
        if (isFinished) {
            finishWithSuccess()
            return
        }
        if (!isRunning && ((timerMode == "countup" && elapsedMillis == 0L) || (timerMode != "countup" && remainingMillis == totalMillis))) {
            setResult(Activity.RESULT_CANCELED)
            finish()
            return
        }
        AlertDialog.Builder(this)
            .setTitle("确定要离开吗？")
            .setMessage("现在退出不会记录本次专注，${petDisplayName}还在等你哦。")
            .setPositiveButton("继续专注", null)
            .setNegativeButton("确定退出") { _, _ ->
                timer?.cancel()
                countupHandler.removeCallbacks(countupTicker)
                setResult(Activity.RESULT_CANCELED)
                finish()
            }
            .show()
    }

    private fun finishWithSuccess() {
        val actualSec = if (timerMode == "countup") {
            (elapsedMillis / 1000L).toInt().coerceAtLeast(1)
        } else {
            (totalMillis / 1000L).toInt().coerceAtLeast(1)
        }
        val data = Intent().apply {
            putExtra(EXTRA_TASK_ID, taskId ?: "")
            putExtra(EXTRA_DURATION_MIN, selectedMinutes)
            putExtra(EXTRA_ACTUAL_DURATION, actualSec)
        }
        setResult(Activity.RESULT_OK, data)
        finish()
    }

    override fun onDestroy() {
        timer?.cancel()
        countupHandler.removeCallbacks(countupTicker)
        sound.release()
        super.onDestroy()
    }
}
