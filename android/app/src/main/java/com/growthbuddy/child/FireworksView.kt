package com.growthbuddy.child

import android.content.Context
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.os.SystemClock
import android.util.AttributeSet
import android.view.View
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.sin
import kotlin.random.Random

/**
 * 破纪录结算页专属礼花特效视图：
 * 严格限定在画面边缘安全区（四角及左右窄边）燃放礼花，避开中央站位人脸区域。
 */
class FireworksView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    private data class Particle(
        var x: Float,
        var y: Float,
        var vx: Float,
        var vy: Float,
        val color: Int,
        val radius: Float,
        val bornMs: Long,
        val lifeMs: Long
    )

    private val particles = mutableListOf<Particle>()
    private val paint = Paint(Paint.ANTI_ALIAS_FLAG).apply { style = Paint.Style.FILL }
    private val linePaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
        style = Paint.Style.STROKE
        strokeCap = Paint.Cap.ROUND
    }
    private val rng = Random(SystemClock.uptimeMillis())
    private var running = false
    private var lastBurstMs = 0L

    fun start() {
        if (running) return
        running = true
        particles.clear()
        lastBurstMs = 0L
        postInvalidateOnAnimation()
    }

    fun stop() {
        running = false
        particles.clear()
    }

    override fun onDetachedFromWindow() {
        running = false
        super.onDetachedFromWindow()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        if (!running || width <= 0 || height <= 0) return

        val now = SystemClock.uptimeMillis()
        if (now - lastBurstMs >= 520L) {
            spawnSafeEdgeBurst(now)
            if (lastBurstMs == 0L) {
                // 首次连发两组左右角礼花，营造瞬时庆贺感
                spawnSafeEdgeBurst(now, forceRight = true)
            }
            lastBurstMs = now
        }

        val dt = 0.016f
        val gravity = height * 0.28f
        val it = particles.iterator()
        while (it.hasNext()) {
            val p = it.next()
            val age = now - p.bornMs
            if (age >= p.lifeMs) {
                it.remove()
                continue
            }
            val progress = age.toFloat() / p.lifeMs.toFloat()
            val prevX = p.x
            val prevY = p.y
            p.vy += gravity * dt
            p.vx *= 0.985f
            p.vy *= 0.985f
            p.x += p.vx * dt
            p.y += p.vy * dt

            val alpha = ((1f - progress) * 255f).toInt().coerceIn(0, 255)
            linePaint.color = Color.argb((alpha * 0.65f).toInt(), Color.red(p.color), Color.green(p.color), Color.blue(p.color))
            linePaint.strokeWidth = p.radius * 1.15f
            canvas.drawLine(prevX, prevY, p.x, p.y, linePaint)

            paint.color = Color.argb(alpha, Color.red(p.color), Color.green(p.color), Color.blue(p.color))
            canvas.drawCircle(p.x, p.y, p.radius * (1f - progress * 0.45f), paint)
        }

        if (running) {
            postInvalidateOnAnimation()
        }
    }

    private fun spawnSafeEdgeBurst(now: Long, forceRight: Boolean = false) {
        val w = width.toFloat()
        val h = height.toFloat()

        // 仅在四角/外边缘燃放，避开中间人脸区域 (x: 22%~78%, y: 14%~72%)
        val zone = if (forceRight) 1 else rng.nextInt(4)
        val cx: Float
        val cy: Float
        when (zone) {
            0 -> { // 左上角
                cx = w * (0.08f + rng.nextFloat() * 0.14f)
                cy = h * (0.10f + rng.nextFloat() * 0.11f)
            }
            1 -> { // 右上角
                cx = w * (0.78f + rng.nextFloat() * 0.14f)
                cy = h * (0.10f + rng.nextFloat() * 0.11f)
            }
            2 -> { // 左侧边中上部
                cx = w * (0.06f + rng.nextFloat() * 0.10f)
                cy = h * (0.28f + rng.nextFloat() * 0.30f)
            }
            else -> { // 右侧边中上部
                cx = w * (0.84f + rng.nextFloat() * 0.10f)
                cy = h * (0.28f + rng.nextFloat() * 0.30f)
            }
        }

        val palette = PALETTE
        val baseColor = palette[rng.nextInt(palette.size)]
        val count = 26
        val minDim = minOf(w, h)
        for (i in 0 until count) {
            val angle = (2.0 * PI * i / count) + (rng.nextFloat() - 0.5f) * 0.22
            val speed = minDim * (0.16f + rng.nextFloat() * 0.22f)
            particles.add(
                Particle(
                    x = cx,
                    y = cy,
                    vx = (cos(angle) * speed).toFloat(),
                    vy = (sin(angle) * speed).toFloat(),
                    color = if (i % 3 == 0) Color.parseColor("#FFD166") else baseColor,
                    radius = minDim * (0.008f + rng.nextFloat() * 0.006f),
                    bornMs = now,
                    lifeMs = 850L + rng.nextLong(350L)
                )
            )
        }
    }

    companion object {
        private val PALETTE = intArrayOf(
            Color.parseColor("#FFD166"),
            Color.parseColor("#FF6B9D"),
            Color.parseColor("#70D6FF"),
            Color.parseColor("#95F29E"),
            Color.parseColor("#FF9F43"),
            Color.parseColor("#C77DFF")
        )

        /**
         * 在静态结算合成图上绘制边角礼花（严格避开中央人脸区），用于上传家长端照片墙留念。
         */
        fun drawCornerFireworksOnBitmap(canvas: Canvas, w: Float, h: Float) {
            val rayPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                style = Paint.Style.STROKE
                strokeCap = Paint.Cap.ROUND
            }
            val dotPaint = Paint(Paint.ANTI_ALIAS_FLAG).apply {
                style = Paint.Style.FILL
            }
            val centers = listOf(
                Triple(w * 0.13f, h * 0.17f, minOf(w, h) * 0.095f),
                Triple(w * 0.87f, h * 0.17f, minOf(w, h) * 0.095f),
                Triple(w * 0.11f, h * 0.66f, minOf(w, h) * 0.068f),
                Triple(w * 0.89f, h * 0.66f, minOf(w, h) * 0.068f)
            )
            centers.forEachIndexed { idx, (cx, cy, maxR) ->
                val spokes = 14
                for (i in 0 until spokes) {
                    val angle = 2.0 * PI * i / spokes + idx * 0.21
                    val col = PALETTE[(i + idx) % PALETTE.size]
                    val rStart = maxR * 0.24f
                    val rEnd = maxR * (0.72f + (i % 3) * 0.14f)
                    rayPaint.color = Color.argb(215, Color.red(col), Color.green(col), Color.blue(col))
                    rayPaint.strokeWidth = maxOf(2.4f, minOf(w, h) * 0.0045f)
                    val sx = cx + (cos(angle) * rStart).toFloat()
                    val sy = cy + (sin(angle) * rStart).toFloat()
                    val ex = cx + (cos(angle) * rEnd).toFloat()
                    val ey = cy + (sin(angle) * rEnd).toFloat()
                    canvas.drawLine(sx, sy, ex, ey, rayPaint)
                    dotPaint.color = col
                    canvas.drawCircle(ex, ey, maxOf(3.2f, minOf(w, h) * 0.006f), dotPaint)
                }
                dotPaint.color = Color.parseColor("#FFF8D6")
                canvas.drawCircle(cx, cy, maxOf(4f, minOf(w, h) * 0.008f), dotPaint)
            }
        }
    }
}
