package com.growthbuddy.child

import android.app.Activity
import android.content.Intent
import android.os.Bundle
import android.os.CountDownTimer
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.View
import android.view.WindowManager
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
        binding.tvFocusBubble.text = "嘘——小伙伴正在陪你一起认真专注，加油！"
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
            .setMessage("现在退出不会记录本次专注，小伙伴还在等你哦。")
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
