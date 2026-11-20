package com.growthbuddy.child

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
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

    private var phase: Phase = Phase.FRAMING
    private var framedFramesStreak: Int = 0
    private var countDownTimer: CountDownTimer? = null
    private var lastSpokenMilestone: Int = 0
    private var usingFrontCamera: Boolean = true
    private var cameraProviderRef: ProcessCameraProvider? = null

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
        if (sound.isMediaVolumeLow()) {
            binding.btnBoostVol.visibility = View.VISIBLE
        }
        binding.btnBoostVol.setOnClickListener {
            sound.boostMediaVolumeToRecommended()
            sound.playJumpTick()
            binding.btnBoostVol.visibility = View.GONE
        }

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

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                finishWithCancel()
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
        } else {
            sound.playError()
            sound.speak("时间到，跳了${finalCount}个")
        }

        val payload = JSONObject().apply {
            put("taskId", taskId)
            put("actualCount", finalCount)
            put("actualDuration", durationSec)
            put("completed", completed)
            put("cancelled", false)
        }
        setResult(RESULT_OK, Intent().putExtra(EXTRA_RESULT_JSON, payload.toString()))
        finish()
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
        sound.release()
        poseDetector.close()
        cameraExecutor.shutdown()
        super.onDestroy()
    }
}
