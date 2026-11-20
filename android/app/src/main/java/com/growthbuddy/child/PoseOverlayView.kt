package com.growthbuddy.child

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.DashPathEffect
import android.graphics.Paint
import android.graphics.RectF
import android.util.AttributeSet
import android.view.View
import com.google.mlkit.vision.pose.Pose
import com.google.mlkit.vision.pose.PoseLandmark

/**
 * 叠加在 CameraX Preview 上的站位引导框与简化骨骼预览图层。
 */
class PoseOverlayView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    private var pose: Pose? = null
    private var imgW: Int = 1
    private var imgH: Int = 1
    private var isFrontCamera: Boolean = true
    private var framedOk: Boolean = false

    private val boxPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 6f
        pathEffect = DashPathEffect(floatArrayOf(22f, 14f), 0f)
    }

    private val linePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeWidth = 7f
        strokeCap = Paint.Cap.ROUND
        color = Color.parseColor("#8EE3B0")
    }

    private val dotPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.FILL
        color = Color.parseColor("#F4B254")
    }

    fun updatePose(
        newPose: Pose?,
        imageWidth: Int,
        imageHeight: Int,
        frontCamera: Boolean,
        isFramedOk: Boolean
    ) {
        pose = newPose
        imgW = imageWidth.coerceAtLeast(1)
        imgH = imageHeight.coerceAtLeast(1)
        isFrontCamera = frontCamera
        framedOk = isFramedOk
        postInvalidate()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)

        // 绘制中央站位框
        boxPaint.color = if (framedOk) Color.parseColor("#6CB88C") else Color.parseColor("#E8913A")
        val marginX = width * 0.14f
        val topY = height * 0.18f
        val bottomY = height * 0.84f
        canvas.drawRoundRect(RectF(marginX, topY, width - marginX, bottomY), 36f, 36f, boxPaint)

        val p = pose ?: return
        val scale = maxOf(width.toFloat() / imgW, height.toFloat() / imgH)
        val dx = (width - imgW * scale) * 0.5f
        val dy = (height - imgH * scale) * 0.5f

        fun mapX(x: Float): Float {
            val sx = x * scale + dx
            return if (isFrontCamera) width - sx else sx
        }

        fun mapY(y: Float): Float = y * scale + dy

        val pairs = listOf(
            PoseLandmark.LEFT_SHOULDER to PoseLandmark.RIGHT_SHOULDER,
            PoseLandmark.LEFT_SHOULDER to PoseLandmark.LEFT_HIP,
            PoseLandmark.RIGHT_SHOULDER to PoseLandmark.RIGHT_HIP,
            PoseLandmark.LEFT_HIP to PoseLandmark.RIGHT_HIP,
            PoseLandmark.LEFT_SHOULDER to PoseLandmark.LEFT_ELBOW,
            PoseLandmark.LEFT_ELBOW to PoseLandmark.LEFT_WRIST,
            PoseLandmark.RIGHT_SHOULDER to PoseLandmark.RIGHT_ELBOW,
            PoseLandmark.RIGHT_ELBOW to PoseLandmark.RIGHT_WRIST
        )

        for ((aType, bType) in pairs) {
            val a = p.getPoseLandmark(aType) ?: continue
            val b = p.getPoseLandmark(bType) ?: continue
            if (a.inFrameLikelihood < 0.4f || b.inFrameLikelihood < 0.4f) continue
            canvas.drawLine(
                mapX(a.position.x), mapY(a.position.y),
                mapX(b.position.x), mapY(b.position.y),
                linePaint
            )
        }

        val dots = listOf(
            PoseLandmark.LEFT_SHOULDER,
            PoseLandmark.RIGHT_SHOULDER,
            PoseLandmark.LEFT_HIP,
            PoseLandmark.RIGHT_HIP
        )
        for (type in dots) {
            val lm = p.getPoseLandmark(type) ?: continue
            if (lm.inFrameLikelihood < 0.4f) continue
            canvas.drawCircle(mapX(lm.position.x), mapY(lm.position.y), 11f, dotPaint)
        }
    }
}
