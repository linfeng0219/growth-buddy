package com.growthbuddy.child

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.RectF
import android.graphics.Typeface
import android.media.AudioManager
import android.os.Bundle
import android.os.CountDownTimer
import android.os.SystemClock
import android.util.Size
import android.view.View
import android.view.WindowManager
import android.widget.Toast
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.annotation.OptIn
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ExperimentalGetImage
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.ResolutionSelector
import androidx.camera.core.resolutionselector.ResolutionStrategy
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.core.content.ContextCompat
import com.google.mlkit.vision.common.InputImage
import com.google.mlkit.vision.pose.Pose
import com.google.mlkit.vision.pose.PoseDetection
import com.google.mlkit.vision.pose.PoseDetector
import com.google.mlkit.vision.pose.PoseLandmark
import com.google.mlkit.vision.pose.defaults.PoseDetectorOptions
import com.growthbuddy.child.databinding.ActivityRopeJumpBinding
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors

class RopeJumpActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_TASK_JSON = "extra_task_json"
        const val EXTRA_RESULT_JSON = "extra_result_json"
    }

    private enum class Phase { FRAMING, COUNTDOWN, RUNNING, FINISHED }

    private lateinit var binding: ActivityRopeJumpBinding
    private lateinit var cameraExecutor: ExecutorService
    private lateinit var poseDetector: PoseDetector
    private lateinit var sound: NativeSoundEngine
    private val detector = RopeJumpDetector()

    private var taskId: String = ""
    private var taskLabel: String = "定时跳绳挑战"
    private var durationSec: Int = 60
    private var targetCount: Int = 60
    private var childName: String = ""
    private var prevBestCount: Int = 0

    private var phase: Phase = Phase.FRAMING
    private var framedFramesStreak: Int = 0
    private var countDownTimer: CountDownTimer? = null
    private var lastSpokenMilestone: Int = 0
    private var usingFrontCamera: Boolean = true
    private var cameraProviderRef: ProcessCameraProvider? = null
    private var settlementPayload: JSONObject? = null
    private var settlementPreviewBmp: Bitmap? = null

    private val cameraPermLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) startCamera()
        else {
            Toast.makeText(this, "需要摄像头权限才能识别跳绳动作哦", Toast.LENGTH_LONG).show()
            finishWithCancel()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        volumeControlStream = AudioManager.STREAM_MUSIC
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        binding = ActivityRopeJumpBinding.inflate(layoutInflater)
        setContentView(binding.root)

        parseTaskExtras()
        binding.tvTaskTitle.text = "🪢 $taskLabel"
        binding.tvTargetHint.text = "目标：${durationSec} 秒跳够 ${targetCount} 个"
        binding.tvTimer.text = "${durationSec}s"
        binding.tvCount.text = "0"

        sound = NativeSoundEngine(this)

        val options = PoseDetectorOptions.Builder()
            .setDetectorMode(PoseDetectorOptions.STREAM_MODE)
            .build()
        poseDetector = PoseDetection.getClient(options)
        cameraExecutor = Executors.newSingleThreadExecutor()

        binding.btnExit.setOnClickListener { finishWithCancel() }
        binding.btnSwitchCam.setOnClickListener {
            usingFrontCamera = !usingFrontCamera
            bindCameraUseCases()
        }
        binding.btnAction.setOnClickListener {
            when (phase) {
                Phase.FRAMING -> startThreeSecCountdown()
                Phase.RUNNING -> finishChallenge()
                else -> {}
            }
        }

        binding.btnConfirmSettlement.setOnClickListener {
            completeSettlement()
        }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (settlementPayload != null) {
                    completeSettlement()
                } else {
                    finishWithCancel()
                }
            }
        })

        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
            startCamera()
        } else {
            cameraPermLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    private fun parseTaskExtras() {
        val raw = intent.getStringExtra(EXTRA_TASK_JSON) ?: "{}"
        val obj = runCatching { JSONObject(raw) }.getOrNull() ?: JSONObject()
        taskId = obj.optString("taskId", "")
        taskLabel = obj.optString("label", "定时跳绳挑战")
        durationSec = obj.optInt("durationSec", 60).coerceIn(10, 600)
        targetCount = obj.optInt("targetCount", 60).coerceIn(1, 500)
        childName = obj.optString("childName", "")
        prevBestCount = obj.optInt("bestCount", 0).coerceAtLeast(0)
    }

    private fun startCamera() {
        val providerFuture = ProcessCameraProvider.getInstance(this)
        providerFuture.addListener({
            cameraProviderRef = providerFuture.get()
            usingFrontCamera = cameraProviderRef?.hasCamera(CameraSelector.DEFAULT_FRONT_CAMERA) == true
            bindCameraUseCases()
        }, ContextCompat.getMainExecutor(this))
    }

    private fun bindCameraUseCases() {
        val cameraProvider = cameraProviderRef ?: return
        val selector = if (usingFrontCamera && cameraProvider.hasCamera(CameraSelector.DEFAULT_FRONT_CAMERA)) {
            CameraSelector.DEFAULT_FRONT_CAMERA
        } else {
            usingFrontCamera = false
            CameraSelector.DEFAULT_BACK_CAMERA
        }

        val preview = Preview.Builder().build().also {
            it.setSurfaceProvider(binding.previewView.surfaceProvider)
        }

        // 低分辨率 320x240/360x480 推理让 ML Kit 稳定跑满 25~30fps，显著降低快跳漏计
        val resSelector = ResolutionSelector.Builder()
            .setResolutionStrategy(
                ResolutionStrategy(
                    Size(360, 480),
                    ResolutionStrategy.FALLBACK_RULE_CLOSEST_HIGHER_THEN_LOWER
                )
            )
            .build()

        val analysis = ImageAnalysis.Builder()
            .setResolutionSelector(resSelector)
            .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
            .build()
            .also {
                it.setAnalyzer(cameraExecutor, ::analyzeImage)
            }

        try {
            cameraProvider.unbindAll()
            cameraProvider.bindToLifecycle(this, selector, preview, analysis)
        } catch (e: Exception) {
            Toast.makeText(this, "摄像头启动失败: ${e.message}", Toast.LENGTH_LONG).show()
            finishWithCancel()
        }
    }

    @OptIn(ExperimentalGetImage::class)
    private fun analyzeImage(imageProxy: ImageProxy) {
        val mediaImage = imageProxy.image
        if (mediaImage == null) {
            imageProxy.close()
            return
        }
        val rotation = imageProxy.imageInfo.rotationDegrees
        val inputImage = InputImage.fromMediaImage(mediaImage, rotation)
        val rotW = if (rotation == 90 || rotation == 270) imageProxy.height else imageProxy.width
        val rotH = if (rotation == 90 || rotation == 270) imageProxy.width else imageProxy.height

        poseDetector.process(inputImage)
            .addOnSuccessListener { pose ->
                handlePoseResult(pose, rotW, rotH)
            }
            .addOnCompleteListener {
                imageProxy.close()
            }
    }

    private fun handlePoseResult(pose: Pose, width: Int, height: Int) {
        fun lm(type: Int): RopeJumpDetector.Point2D? {
            val p = pose.getPoseLandmark(type) ?: return null
            return RopeJumpDetector.Point2D(p.position.x, p.position.y, p.inFrameLikelihood)
        }

        val points = RopeJumpDetector.PosePoints(
            nose = lm(PoseLandmark.NOSE),
            leftShoulder = lm(PoseLandmark.LEFT_SHOULDER),
            rightShoulder = lm(PoseLandmark.RIGHT_SHOULDER),
            leftHip = lm(PoseLandmark.LEFT_HIP),
            rightHip = lm(PoseLandmark.RIGHT_HIP),
            leftAnkle = lm(PoseLandmark.LEFT_ANKLE),
            rightAnkle = lm(PoseLandmark.RIGHT_ANKLE)
        )

        val out = detector.onPoseFrame(
            points = points,
            imageWidth = width,
            imageHeight = height,
            timestampMs = SystemClock.elapsedRealtime(),
            countingActive = (phase == Phase.RUNNING)
        )

        // 零延迟关键：计数增加瞬间直接在回调线程触发 SoundPool，不等 UI 重绘排队！
        if (phase == Phase.RUNNING && out.incremented) {
            sound.playJumpTick()
        }

        runOnUiThread {
            binding.poseOverlay.updatePose(pose, width, height, usingFrontCamera, out.framedOk)
            if (phase == Phase.FRAMING) {
                binding.tvStatus.text = out.hint
                if (out.framedOk) {
                    framedFramesStreak += 1
                    if (framedFramesStreak >= 10) {
                        startThreeSecCountdown()
                    }
                } else {
                    framedFramesStreak = 0
                }
            } else if (phase == Phase.RUNNING) {
                if (out.incremented) {
                    binding.tvCount.text = out.count.toString()
                    // 跳跃数字弹性放大动画
                    binding.tvCount.animate().cancel()
                    binding.tvCount.scaleX = 1.24f
                    binding.tvCount.scaleY = 1.24f
                    binding.tvCount.animate().scaleX(1f).scaleY(1f).setDuration(110).start()

                    if (out.speedPerMin > 0) {
                        binding.tvSpeed.text = "约 ${out.speedPerMin} 个/分"
                    }

                    if (out.count == targetCount) {
                        sound.playWin()
                        sound.speak("已达标，继续保持")
                        binding.tvStatus.text = "🎉 太棒了！已达到目标 ${targetCount} 个，可继续冲刺！"
                    } else if (out.count > 0 && out.count % 10 == 0 && out.count != lastSpokenMilestone) {
                        lastSpokenMilestone = out.count
                        sound.speak("${out.count}")
                    }
                } else if (!out.framedOk) {
                    binding.tvStatus.text = out.hint
                } else if (out.count < targetCount) {
                    binding.tvStatus.text = "加油！距离目标还差 ${targetCount - out.count} 个"
                }
            }
        }
    }

    private fun startThreeSecCountdown() {
        if (phase != Phase.FRAMING) return
        phase = Phase.COUNTDOWN
        binding.btnAction.isEnabled = false
        binding.btnAction.text = "准备开始..."
        binding.tvBigCountdown.visibility = View.VISIBLE

        countDownTimer?.cancel()
        countDownTimer = object : CountDownTimer(3200L, 1000L) {
            override fun onTick(millisUntilFinished: Long) {
                val sec = ((millisUntilFinished + 400L) / 1000L).toInt().coerceAtLeast(1)
                binding.tvBigCountdown.text = sec.toString()
                binding.tvStatus.text = "站好准备，$sec 秒后开始！"
                sound.playCountdown()
                sound.speak(sec.toString())
            }

            override fun onFinish() {
                binding.tvBigCountdown.visibility = View.GONE
                startActiveChallenge()
            }
        }.start()
    }

    private fun startActiveChallenge() {
        phase = Phase.RUNNING
        detector.resetCount()
        lastSpokenMilestone = 0
        binding.tvCount.text = "0"
        binding.tvSpeed.text = "已跳个数"
        binding.btnAction.isEnabled = true
        binding.btnAction.text = "提前结束并结算"
        binding.tvStatus.text = "开始跳绳！加油 ${childName}"
        sound.playStart()
        sound.speak("开始")

        countDownTimer?.cancel()
        countDownTimer = object : CountDownTimer(durationSec * 1000L, 1000L) {
            override fun onTick(millisUntilFinished: Long) {
                val rem = ((millisUntilFinished + 500L) / 1000L).toInt().coerceAtLeast(0)
                binding.tvTimer.text = "${rem}s"
                if (rem == 10) sound.speak("最后十秒加油")
                else if (rem in 1..3) sound.playCountdown()
            }

            override fun onFinish() {
                binding.tvTimer.text = "0s"
                finishChallenge()
            }
        }.start()
    }

    private fun finishChallenge() {
        if (phase == Phase.FINISHED) return
        phase = Phase.FINISHED
        countDownTimer?.cancel()
        val finalCount = detector.count
        val completed = finalCount >= targetCount

        if (completed) {
            sound.playWin()
            sound.speak("挑战成功，一共跳了${finalCount}个")
            val isNewRecord = finalCount > prevBestCount && finalCount > 0
            val displayBest = maxOf(finalCount, prevBestCount)
            val checkinTimeStr = SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault()).format(Date())
            val rawBmp = binding.previewView.bitmap
            runCatching { cameraProviderRef?.unbindAll() }

            val photoPath = captureRopeSummaryPhoto(
                previewBmp = rawBmp,
                finalCount = finalCount,
                checkinTimeStr = checkinTimeStr,
                isNewRecord = isNewRecord,
                displayBest = displayBest
            )
            showSettlementScreen(
                rawBmp = rawBmp,
                finalCount = finalCount,
                checkinTimeStr = checkinTimeStr,
                isNewRecord = isNewRecord,
                displayBest = displayBest,
                photoPath = photoPath
            )
        } else {
            sound.playError()
            sound.speak("时间到，跳了${finalCount}个")
            val payload = JSONObject().apply {
                put("taskId", taskId)
                put("actualCount", finalCount)
                put("actualDuration", durationSec)
                put("completed", false)
                put("cancelled", false)
            }
            setResult(RESULT_OK, Intent().putExtra(EXTRA_RESULT_JSON, payload.toString()))
            finish()
        }
    }

    private fun showSettlementScreen(
        rawBmp: Bitmap?,
        finalCount: Int,
        checkinTimeStr: String,
        isNewRecord: Boolean,
        displayBest: Int,
        photoPath: String
    ) {
        val who = if (childName.isNotBlank()) " · $childName" else ""
        binding.topHud.visibility = View.GONE
        binding.scoreCard.visibility = View.GONE
        binding.bottomBar.visibility = View.GONE
        binding.poseOverlay.visibility = View.GONE
        binding.tvBigCountdown.visibility = View.GONE

        settlementPreviewBmp = if (rawBmp != null && rawBmp.width > 0 && rawBmp.height > 0) {
            rawBmp.copy(Bitmap.Config.ARGB_8888, false)
        } else {
            Bitmap.createBitmap(720, 960, Bitmap.Config.ARGB_8888).apply {
                Canvas(this).drawColor(Color.parseColor("#1A2332"))
            }
        }
        binding.ivSettlementPhoto.setImageBitmap(settlementPreviewBmp)

        binding.tvSettleTask.text = "🪢 $taskLabel$who"
        binding.tvSettleTime.text = "打卡时间：$checkinTimeStr"
        binding.tvSettleCount.text = "${finalCount} 个"
        binding.tvSettleSub.text = "用时 ${durationSec} 秒 · 目标 ${targetCount} 个"
        binding.tvSettleRecordTag.visibility = if (isNewRecord) View.VISIBLE else View.GONE
        binding.tvSettleBest.text = if (isNewRecord) {
            if (prevBestCount > 0) "🎆 打破历史纪录！新最高纪录：${displayBest} 个（原 ${prevBestCount} 个）"
            else "🎆 创造首次最高纪录：${displayBest} 个！"
        } else {
            "🏆 历史最高纪录：${displayBest} 个"
        }

        if (isNewRecord) {
            binding.fireworksView.visibility = View.VISIBLE
            binding.fireworksView.start()
        } else {
            binding.fireworksView.stop()
            binding.fireworksView.visibility = View.GONE
        }
        binding.settlementOverlay.visibility = View.VISIBLE

        settlementPayload = JSONObject().apply {
            put("taskId", taskId)
            put("actualCount", finalCount)
            put("actualDuration", durationSec)
            put("completed", true)
            put("isNewRecord", isNewRecord)
            put("bestCount", displayBest)
            put("checkinTime", checkinTimeStr)
            put("cancelled", false)
            if (photoPath.isNotEmpty()) put("photoPath", photoPath)
        }
    }

    private fun completeSettlement() {
        val payload = settlementPayload ?: return
        binding.fireworksView.stop()
        setResult(RESULT_OK, Intent().putExtra(EXTRA_RESULT_JSON, payload.toString()))
        finish()
    }

    /**
     * 跳绳挑战达标结束时，以抓拍画面为底图，仅在顶部与底部安全边缘叠加打卡时间、最终数量与破纪录礼花/最高纪录，
     * 避开中央人脸区域，保存为临时 JPEG 文件供上传家长端照片墙。
     */
    private fun captureRopeSummaryPhoto(
        previewBmp: Bitmap?,
        finalCount: Int,
        checkinTimeStr: String,
        isNewRecord: Boolean,
        displayBest: Int
    ): String {
        return try {
            val targetW: Int
            val targetH: Int
            val outBmp: Bitmap
            if (previewBmp != null && previewBmp.width > 0 && previewBmp.height > 0) {
                val maxW = 720
                if (previewBmp.width > maxW) {
                    targetW = maxW
                    targetH = (previewBmp.height.toFloat() * maxW / previewBmp.width).toInt().coerceAtLeast(320)
                    outBmp = Bitmap.createScaledBitmap(previewBmp, targetW, targetH, true)
                        .copy(Bitmap.Config.ARGB_8888, true)
                } else {
                    targetW = previewBmp.width
                    targetH = previewBmp.height
                    outBmp = previewBmp.copy(Bitmap.Config.ARGB_8888, true)
                }
            } else {
                targetW = 720
                targetH = 960
                outBmp = Bitmap.createBitmap(targetW, targetH, Bitmap.Config.ARGB_8888).apply {
                    Canvas(this).drawColor(Color.parseColor("#1A2332"))
                }
            }

            val canvas = Canvas(outBmp)
            val s = targetW / 720f
            val pad = 20f * s

            val bgPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = Color.parseColor("#D4141E2E")
                style = Paint.Style.FILL
            }
            val borderPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = if (isNewRecord) Color.parseColor("#B8FFD166") else Color.parseColor("#6694A3B8")
                style = Paint.Style.STROKE
                strokeWidth = 2.5f * s
            }

            // 1. 顶部超薄信息条（位于人物头部上方安全区）
            val topBarH = 62f * s
            val topRect = RectF(pad, pad, targetW - pad, pad + topBarH)
            canvas.drawRoundRect(topRect, 16f * s, 16f * s, bgPaint)
            val topTitlePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = Color.parseColor("#FFFFFF")
                textSize = 22f * s
                typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
            }
            val timePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = Color.parseColor("#CBD5E1")
                textSize = 19f * s
                textAlign = Paint.Align.RIGHT
            }
            val who = if (childName.isNotBlank()) " · $childName" else ""
            canvas.drawText("🪢 $taskLabel$who", topRect.left + 18f * s, topRect.top + 40f * s, topTitlePaint)
            canvas.drawText(checkinTimeStr, topRect.right - 18f * s, topRect.top + 39f * s, timePaint)

            // 2. 如破历史纪录，在四角边缘安全区绘制礼花（完全避开中央人脸）
            if (isNewRecord) {
                FireworksView.drawCornerFireworksOnBitmap(canvas, targetW.toFloat(), targetH.toFloat())
            }

            // 3. 底部紧凑结算条（紧贴底边，不遮挡人脸与上半身）
            val bannerH = 148f * s
            val botTop = (targetH - bannerH - pad).coerceAtLeast(topRect.bottom + pad)
            val cardRect = RectF(pad, botTop, targetW - pad, botTop + bannerH)
            canvas.drawRoundRect(cardRect, 20f * s, 20f * s, bgPaint)
            canvas.drawRoundRect(cardRect, 20f * s, 20f * s, borderPaint)

            val countPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = Color.parseColor("#FFD166")
                textSize = 46f * s
                typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
            }
            val subPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = Color.parseColor("#F8FAFC")
                textSize = 21f * s
            }
            val recordPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                color = if (isNewRecord) Color.parseColor("#FDE68A") else Color.parseColor("#BAE6FD")
                textSize = 21f * s
                typeface = Typeface.create(Typeface.DEFAULT, Typeface.BOLD)
            }

            canvas.drawText("${finalCount} 个", cardRect.left + 22f * s, cardRect.top + 56f * s, countPaint)
            val countW = countPaint.measureText("${finalCount} 个")
            canvas.drawText(
                "用时 ${durationSec} 秒 · 目标 ${targetCount} 个",
                cardRect.left + 22f * s + countW + 18f * s,
                cardRect.top + 48f * s,
                subPaint
            )
            val bestText = if (isNewRecord) {
                if (prevBestCount > 0) "🎆 打破纪录！最高纪录：${displayBest} 个（原 ${prevBestCount} 个）"
                else "🎆 新最高纪录：${displayBest} 个！"
            } else {
                "🏆 历史最高纪录：${displayBest} 个"
            }
            canvas.drawText(bestText, cardRect.left + 22f * s, cardRect.top + 96f * s, recordPaint)
            canvas.drawText(
                "打卡时间：$checkinTimeStr",
                cardRect.left + 22f * s,
                cardRect.top + 128f * s,
                subPaint
            )

            val file = File(cacheDir, "rope_snap_${System.currentTimeMillis()}.jpg")
            FileOutputStream(file).use { fos ->
                outBmp.compress(Bitmap.CompressFormat.JPEG, 82, fos)
            }
            outBmp.recycle()
            file.absolutePath
        } catch (_: Exception) {
            ""
        }
    }

    private fun finishWithCancel() {
        countDownTimer?.cancel()
        val payload = JSONObject().apply {
            put("taskId", taskId)
            put("cancelled", true)
        }
        setResult(RESULT_CANCELED, Intent().putExtra(EXTRA_RESULT_JSON, payload.toString()))
        finish()
    }

    override fun onDestroy() {
        countDownTimer?.cancel()
        binding.fireworksView.stop()
        settlementPreviewBmp?.recycle()
        settlementPreviewBmp = null
        sound.release()
        poseDetector.close()
        cameraExecutor.shutdown()
        super.onDestroy()
    }
}
