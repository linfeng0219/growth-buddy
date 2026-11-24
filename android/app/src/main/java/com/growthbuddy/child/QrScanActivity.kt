package com.growthbuddy.child

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.graphics.BitmapFactory
import android.os.Build
import android.os.Bundle
import android.util.Size
import android.view.Surface
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.camera.core.CameraSelector
import androidx.camera.core.ImageAnalysis
import androidx.camera.core.ImageProxy
import androidx.camera.core.Preview
import androidx.camera.core.resolutionselector.ResolutionSelector
import androidx.camera.core.resolutionselector.ResolutionStrategy
import androidx.camera.lifecycle.ProcessCameraProvider
import androidx.core.content.ContextCompat
import com.google.zxing.BinaryBitmap
import com.google.zxing.DecodeHintType
import com.google.zxing.PlanarYUVLuminanceSource
import com.google.zxing.RGBLuminanceSource
import com.google.zxing.common.HybridBinarizer
import com.google.zxing.qrcode.QRCodeReader
import com.growthbuddy.child.databinding.ActivityQrScanBinding
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

/**
 * 离线纯原生二维码扫描页（CameraX + ZXing Core）：
 * - 支持实时扫描小伙伴的「成长好友二维码」；
 * - 也支持从手机相册选择二维码图片识别。
 */
class QrScanActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_QR_RESULT = "extra_qr_result"
    }

    private lateinit var binding: ActivityQrScanBinding
    private lateinit var cameraExecutor: ExecutorService
    private val qrReader = QRCodeReader()
    private val decodeHints = mapOf(DecodeHintType.TRY_HARDER to true)
    private val handled = AtomicBoolean(false)
    private var previewUseCase: Preview? = null
    private var analysisUseCase: ImageAnalysis? = null

    private val permLauncher = registerForActivityResult(
        ActivityResultContracts.RequestPermission()
    ) { granted ->
        if (granted) startCamera()
        else Toast.makeText(this, "需要摄像头权限才能扫码，也可点击右上角从相册选择图片", Toast.LENGTH_LONG).show()
    }

    private val galleryLauncher = registerForActivityResult(
        ActivityResultContracts.GetContent()
    ) { uri ->
        if (uri == null) return@registerForActivityResult
        try {
            val bmp = contentResolver.openInputStream(uri)?.use { BitmapFactory.decodeStream(it) }
            if (bmp == null) {
                Toast.makeText(this, "未能读取图片", Toast.LENGTH_SHORT).show()
                return@registerForActivityResult
            }
            val w = bmp.width
            val h = bmp.height
            val pixels = IntArray(w * h)
            bmp.getPixels(pixels, 0, w, 0, 0, w, h)
            bmp.recycle()
            val bin = BinaryBitmap(HybridBinarizer(RGBLuminanceSource(w, h, pixels)))
            val result = qrReader.decode(bin, decodeHints)
            deliverResult(result.text)
        } catch (_: Exception) {
            Toast.makeText(this, "未在图片中识别到有效的好友二维码", Toast.LENGTH_SHORT).show()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityQrScanBinding.inflate(layoutInflater)
        setContentView(binding.root)

        cameraExecutor = Executors.newSingleThreadExecutor()
        binding.btnQrClose.setOnClickListener {
            setResult(RESULT_CANCELED)
            finish()
        }
        binding.btnQrGallery.setOnClickListener {
            galleryLauncher.launch("image/*")
        }

        if (ContextCompat.checkSelfPermission(this, Manifest.permission.CAMERA) == PackageManager.PERMISSION_GRANTED) {
            startCamera()
        } else {
            permLauncher.launch(Manifest.permission.CAMERA)
        }
    }

    @Suppress("DEPRECATION")
    private fun currentRotation(): Int {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            display?.rotation ?: Surface.ROTATION_0
        } else {
            windowManager.defaultDisplay?.rotation ?: Surface.ROTATION_0
        }
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        val rot = currentRotation()
        previewUseCase?.targetRotation = rot
        analysisUseCase?.targetRotation = rot
    }

    private fun startCamera() {
        val future = ProcessCameraProvider.getInstance(this)
        future.addListener({
            val provider = future.get()
            val rot = currentRotation()
            val preview = Preview.Builder()
                .setTargetRotation(rot)
                .build().also {
                    it.setSurfaceProvider(binding.qrPreviewView.surfaceProvider)
                }
            val resSelector = ResolutionSelector.Builder()
                .setResolutionStrategy(
                    ResolutionStrategy(
                        Size(720, 960),
                        ResolutionStrategy.FALLBACK_RULE_CLOSEST_HIGHER_THEN_LOWER
                    )
                )
                .build()
            val analysis = ImageAnalysis.Builder()
                .setTargetRotation(rot)
                .setResolutionSelector(resSelector)
                .setBackpressureStrategy(ImageAnalysis.STRATEGY_KEEP_ONLY_LATEST)
                .build().also {
                    it.setAnalyzer(cameraExecutor, ::scanFrame)
                }
            previewUseCase = preview
            analysisUseCase = analysis
            try {
                provider.unbindAll()
                val selector = if (provider.hasCamera(CameraSelector.DEFAULT_BACK_CAMERA)) {
                    CameraSelector.DEFAULT_BACK_CAMERA
                } else {
                    CameraSelector.DEFAULT_FRONT_CAMERA
                }
                provider.bindToLifecycle(this, selector, preview, analysis)
            } catch (e: Exception) {
                Toast.makeText(this, "打开摄像头失败: ${e.message}", Toast.LENGTH_SHORT).show()
            }
        }, ContextCompat.getMainExecutor(this))
    }

    private fun scanFrame(image: ImageProxy) {
        if (handled.get()) {
            image.close()
            return
        }
        try {
            val yPlane = image.planes.firstOrNull() ?: return
            val yBuffer = yPlane.buffer
            val rowStride = yPlane.rowStride
            val width = image.width
            val height = image.height
            val data = ByteArray(width * height)
            if (rowStride == width) {
                yBuffer.get(data)
            } else {
                val row = ByteArray(rowStride)
                for (r in 0 until height) {
                    yBuffer.position(r * rowStride)
                    val readLen = minOf(rowStride, yBuffer.remaining())
                    yBuffer.get(row, 0, readLen)
                    System.arraycopy(row, 0, data, r * width, width)
                }
            }
            val src = PlanarYUVLuminanceSource(data, width, height, 0, 0, width, height, false)
            val res = qrReader.decode(BinaryBitmap(HybridBinarizer(src)), decodeHints)
            if (!res.text.isNullOrBlank()) {
                runOnUiThread { deliverResult(res.text) }
            }
        } catch (_: Exception) {
            qrReader.reset()
        } finally {
            image.close()
        }
    }

    private fun deliverResult(rawText: String) {
        if (!handled.compareAndSet(false, true)) return
        setResult(RESULT_OK, Intent().putExtra(EXTRA_QR_RESULT, rawText.trim()))
        finish()
    }

    override fun onDestroy() {
        super.onDestroy()
        if (::cameraExecutor.isInitialized) {
            cameraExecutor.shutdownNow()
        }
    }
}
