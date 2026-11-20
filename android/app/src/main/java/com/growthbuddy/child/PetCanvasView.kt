package com.growthbuddy.child

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.Color
import android.graphics.Paint
import android.graphics.Path
import android.graphics.RadialGradient
import android.graphics.Rect
import android.graphics.RectF
import android.graphics.Shader
import android.os.SystemClock
import android.util.AttributeSet
import android.view.View
import kotlin.math.PI
import kotlin.math.cos
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sin
import kotlin.random.Random

/**
 * 原生 60fps 糖霜宠物绘图引擎（替代 WebView SVG，实现零迟滞互动动画与粒子爆发）。
 */
object PetArtCanvas {

    fun parseColorSafe(hex: String, fallback: Int = 0xFFEC9844.toInt()): Int {
        return runCatching { Color.parseColor(hex) }.getOrDefault(fallback)
    }

    fun drawPedestal(canvas: Canvas, base: String, cx: Float, cy: Float, scale: Float, paint: Paint) {
        val (fill, stroke) = when (base) {
            "snow" -> 0xFFE8EEF3.toInt() to 0xFFC3CFD9.toInt()
            "sand" -> 0xFFF2E3CC.toInt() to 0xFFD4B78C.toInt()
            "cloud" -> 0xFFEEF1F8.toInt() to 0xFFCCD4E6.toInt()
            else -> 0xFFEBE7D4.toInt() to 0xFFB6C79A.toInt()
        }
        paint.style = Paint.Style.FILL
        paint.shader = null
        paint.color = fill
        val rx = 68f * scale
        val ry = 12f * scale
        canvas.drawOval(RectF(cx - rx, cy - ry, cx + rx, cy + ry), paint)

        paint.style = Paint.Style.STROKE
        paint.color = stroke
        paint.strokeWidth = 2.2f * scale
        paint.strokeCap = Paint.Cap.ROUND
        val p = Path()
        p.moveTo(cx - 54f * scale, cy - 4f * scale)
        p.quadTo(cx - 50f * scale, cy - 13f * scale, cx - 46f * scale, cy - 4f * scale)
        p.moveTo(cx + 52f * scale, cy - 4f * scale)
        p.quadTo(cx + 56f * scale, cy - 13f * scale, cx + 60f * scale, cy - 4f * scale)
        canvas.drawPath(p, paint)
    }

    fun drawEyes(canvas: Canvas, x1: Float, x2: Float, y: Float, scale: Float, sleep: Boolean, paint: Paint) {
        if (sleep) {
            paint.style = Paint.Style.STROKE
            paint.shader = null
            paint.color = 0xFF3A2F22.toInt()
            paint.strokeWidth = 2.6f * scale
            paint.strokeCap = Paint.Cap.ROUND
            val p = Path()
            p.moveTo(x1 - 7f * scale, y)
            p.quadTo(x1, y + 6f * scale, x1 + 7f * scale, y)
            p.moveTo(x2 - 7f * scale, y)
            p.quadTo(x2, y + 6f * scale, x2 + 7f * scale, y)
            canvas.drawPath(p, paint)
            return
        }
        paint.style = Paint.Style.FILL
        for (ex in floatArrayOf(x1, x2)) {
            paint.shader = RadialGradient(
                ex - 2f * scale, y - 3f * scale, 12f * scale,
                intArrayOf(0xFF7C5630.toInt(), 0xFF4A3220.toInt(), 0xFF281B10.toInt()),
                floatArrayOf(0f, 0.5f, 1f),
                Shader.TileMode.CLAMP
            )
            canvas.drawOval(RectF(ex - 8f * scale, y - 10f * scale, ex + 8f * scale, y + 10f * scale), paint)
            paint.shader = null
            paint.color = Color.WHITE
            canvas.drawCircle(ex - 3.2f * scale, y - 4.0f * scale, 3.0f * scale, paint)
            paint.color = 0xE0FFFFFF.toInt()
            canvas.drawCircle(ex + 3.2f * scale, y + 4.0f * scale, 1.5f * scale, paint)
        }
    }

    fun drawMainPet(
        canvas: Canvas,
        lv: Int,
        cx: Float,
        cy: Float,
        scale: Float,
        sleep: Boolean,
        breath: Float,
        paint: Paint
    ) {
        drawPedestal(canvas, if (lv >= 4) "sand" else "grass", cx, cy + 72f * scale, scale, paint)

        if (lv >= 3) {
            paint.style = Paint.Style.FILL
            paint.shader = null
            paint.color = 0x29F3C653
            canvas.drawCircle(cx, cy + 6f * scale, 80f * scale, paint)
        }

        canvas.save()
        val bodyScaleY = 1f + breath * 0.025f
        val bodyScaleX = 1f - breath * 0.015f
        canvas.scale(bodyScaleX, bodyScaleY, cx, cy + 68f * scale)

        val bodyShader = RadialGradient(
            cx - 16f * scale, cy - 26f * scale, 95f * scale,
            if (lv >= 5) intArrayOf(0xFFFFE1AD.toInt(), 0xFFED8E32.toInt(), 0xFFC45F1C.toInt())
            else intArrayOf(0xFFFFDCA8.toInt(), 0xFFF39D48.toInt(), 0xFFD87526.toInt()),
            floatArrayOf(0f, 0.55f, 1f),
            Shader.TileMode.CLAMP
        )
        val eggShader = RadialGradient(
            cx - 12f * scale, cy - 18f * scale, 75f * scale,
            intArrayOf(0xFFFFF8EB.toInt(), 0xFFF3E5CB.toInt(), 0xFFDFD0B2.toInt()),
            floatArrayOf(0f, 0.65f, 1f),
            Shader.TileMode.CLAMP
        )

        when (lv) {
            0 -> {
                // 一颗蛋（柔和摇摆）
                canvas.rotate(breath * 4f, cx, cy + 56f * scale)
                paint.style = Paint.Style.FILL
                paint.shader = eggShader
                canvas.drawOval(RectF(cx - 44f * scale, cy - 34f * scale, cx + 44f * scale, cy + 72f * scale), paint)
                paint.shader = null
                paint.style = Paint.Style.STROKE
                paint.color = 0xFFD7C29E.toInt()
                paint.strokeWidth = 2.2f * scale
                canvas.drawOval(RectF(cx - 44f * scale, cy - 34f * scale, cx + 44f * scale, cy + 72f * scale), paint)
                paint.style = Paint.Style.FILL
                paint.color = 0x73FFFFFF
                canvas.drawOval(RectF(cx - 24f * scale, cy - 20f * scale, cx + 2f * scale, cy + 16f * scale), paint)
                paint.color = 0xFFE1CDA9.toInt()
                canvas.drawCircle(cx - 16f * scale, cy + 8f * scale, 4f * scale, paint)
                canvas.drawCircle(cx + 18f * scale, cy + 32f * scale, 3.5f * scale, paint)
            }

            1 -> {
                // 破壳啦
                paint.style = Paint.Style.FILL
                paint.shader = bodyShader
                canvas.drawOval(RectF(cx - 40f * scale, cy - 22f * scale, cx + 40f * scale, cy + 56f * scale), paint)
                drawEyes(canvas, cx - 16f * scale, cx + 16f * scale, cy + 12f * scale, scale, sleep, paint)
                // 腮红与微笑
                paint.shader = null
                paint.style = Paint.Style.FILL
                paint.color = 0x7AF28B82
                canvas.drawOval(RectF(cx - 34f * scale, cy + 20f * scale, cx - 20f * scale, cy + 29f * scale), paint)
                canvas.drawOval(RectF(cx + 20f * scale, cy + 20f * scale, cx + 34f * scale, cy + 29f * scale), paint)
                paint.style = Paint.Style.STROKE
                paint.color = 0xFF3A2F22.toInt()
                paint.strokeWidth = 2.4f * scale
                val m = Path().apply {
                    moveTo(cx - 7f * scale, cy + 26f * scale)
                    quadTo(cx, cy + 32f * scale, cx + 7f * scale, cy + 26f * scale)
                }
                canvas.drawPath(m, paint)
                // 下半蛋壳
                paint.style = Paint.Style.FILL
                paint.shader = eggShader
                val shell = Path().apply {
                    moveTo(cx - 42f * scale, cy + 36f * scale)
                    lineTo(cx - 26f * scale, cy + 48f * scale)
                    lineTo(cx - 10f * scale, cy + 36f * scale)
                    lineTo(cx + 6f * scale, cy + 48f * scale)
                    lineTo(cx + 22f * scale, cy + 36f * scale)
                    lineTo(cx + 42f * scale, cy + 36f * scale)
                    arcTo(RectF(cx - 42f * scale, cy + 8f * scale, cx + 42f * scale, cy + 72f * scale), 0f, 180f, false)
                    close()
                }
                canvas.drawPath(shell, paint)
            }

            else -> {
                // Lv.2 幼年体 / Lv.3 少年体 / Lv.4 成年体 / Lv.5 守护兽
                paint.shader = null
                paint.style = Paint.Style.FILL

                // 守护兽翅膀
                if (lv >= 5) {
                    paint.color = 0xFFD97224.toInt()
                    val lw = Path().apply {
                        moveTo(cx - 56f * scale, cy + 4f * scale)
                        quadTo(cx - 98f * scale, cy - 28f * scale, cx - 76f * scale, cy - 52f * scale)
                        quadTo(cx - 50f * scale, cy - 42f * scale, cx - 44f * scale, cy - 12f * scale)
                        close()
                    }
                    val rw = Path().apply {
                        moveTo(cx + 56f * scale, cy + 4f * scale)
                        quadTo(cx + 98f * scale, cy - 28f * scale, cx + 76f * scale, cy - 52f * scale)
                        quadTo(cx + 50f * scale, cy - 42f * scale, cx + 44f * scale, cy - 12f * scale)
                        close()
                    }
                    canvas.drawPath(lw, paint)
                    canvas.drawPath(rw, paint)
                }

                // 尾巴
                if (lv >= 3) {
                    paint.color = if (lv >= 4) 0xFFD87526.toInt() else 0xFFF8D29C.toInt()
                    canvas.drawCircle(cx + 52f * scale, cy + 40f * scale, 14f * scale, paint)
                    canvas.drawCircle(cx + 62f * scale, cy + 30f * scale, 10f * scale, paint)
                }

                // 耳朵
                paint.color = 0xFFEB9540.toInt()
                if (lv == 2) {
                    canvas.drawCircle(cx - 38f * scale, cy - 24f * scale, 15f * scale, paint)
                    canvas.drawCircle(cx + 38f * scale, cy - 24f * scale, 15f * scale, paint)
                    paint.color = 0xFFFBD2A2.toInt()
                    canvas.drawCircle(cx - 36f * scale, cy - 22f * scale, 7f * scale, paint)
                    canvas.drawCircle(cx + 36f * scale, cy - 22f * scale, 7f * scale, paint)
                } else {
                    val le = Path().apply {
                        moveTo(cx - 48f * scale, cy - 14f * scale)
                        lineTo(cx - 44f * scale, cy - 56f * scale)
                        lineTo(cx - 18f * scale, cy - 30f * scale)
                        close()
                    }
                    val re = Path().apply {
                        moveTo(cx + 48f * scale, cy - 14f * scale)
                        lineTo(cx + 44f * scale, cy - 56f * scale)
                        lineTo(cx + 18f * scale, cy - 30f * scale)
                        close()
                    }
                    canvas.drawPath(le, paint)
                    canvas.drawPath(re, paint)
                }

                // 成年体小角 / 守护兽王冠
                if (lv == 4) {
                    paint.color = 0xFFF0C24B.toInt()
                    val lh = Path().apply {
                        moveTo(cx - 26f * scale, cy - 42f * scale)
                        lineTo(cx - 34f * scale, cy - 72f * scale)
                        lineTo(cx - 12f * scale, cy - 48f * scale)
                        close()
                    }
                    val rh = Path().apply {
                        moveTo(cx + 26f * scale, cy - 42f * scale)
                        lineTo(cx + 34f * scale, cy - 72f * scale)
                        lineTo(cx + 12f * scale, cy - 48f * scale)
                        close()
                    }
                    canvas.drawPath(lh, paint)
                    canvas.drawPath(rh, paint)
                } else if (lv >= 5) {
                    paint.color = 0xFFF4C74E.toInt()
                    val crown = Path().apply {
                        moveTo(cx - 22f * scale, cy - 44f * scale)
                        lineTo(cx - 12f * scale, cy - 68f * scale)
                        lineTo(cx, cy - 50f * scale)
                        lineTo(cx + 12f * scale, cy - 68f * scale)
                        lineTo(cx + 22f * scale, cy - 44f * scale)
                        close()
                    }
                    canvas.drawPath(crown, paint)
                }

                // 主体
                val rx = (52f + min(lv - 2, 3) * 4f) * scale
                val ry = (44f + min(lv - 2, 3) * 3f) * scale
                paint.shader = bodyShader
                canvas.drawOval(RectF(cx - rx, cy - ry + 14f * scale, cx + rx, cy + ry + 14f * scale), paint)

                // 顶光糖霜高光
                paint.shader = null
                paint.color = 0x55FFF3D9
                canvas.drawOval(RectF(cx - rx * 0.72f, cy - ry + 20f * scale, cx + rx * 0.72f, cy + 4f * scale), paint)

                // 小脚丫
                paint.color = 0xFFD87526.toInt()
                canvas.drawOval(RectF(cx - 46f * scale, cy + 52f * scale, cx - 20f * scale, cy + 68f * scale), paint)
                canvas.drawOval(RectF(cx + 20f * scale, cy + 52f * scale, cx + 46f * scale, cy + 68f * scale), paint)

                // 暖白小肚皮
                paint.color = 0xF2FCE8C5.toInt()
                canvas.drawOval(RectF(cx - 34f * scale, cy + 12f * scale, cx + 34f * scale, cy + 56f * scale), paint)

                // 眼睛、腮红、猫嘴
                drawEyes(canvas, cx - 21f * scale, cx + 21f * scale, cy + 6f * scale, scale, sleep, paint)
                paint.shader = null
                paint.style = Paint.Style.FILL
                paint.color = 0x7AF28B82
                canvas.drawOval(RectF(cx - 48f * scale, cy + 16f * scale, cx - 30f * scale, cy + 28f * scale), paint)
                canvas.drawOval(RectF(cx + 30f * scale, cy + 16f * scale, cx + 48f * scale, cy + 28f * scale), paint)

                paint.style = Paint.Style.STROKE
                paint.color = 0xFF3A2F22.toInt()
                paint.strokeWidth = 2.4f * scale
                paint.strokeCap = Paint.Cap.ROUND
                val mouth = Path().apply {
                    moveTo(cx - 10f * scale, cy + 22f * scale)
                    quadTo(cx - 5f * scale, cy + 27f * scale, cx, cy + 22f * scale)
                    quadTo(cx + 5f * scale, cy + 27f * scale, cx + 10f * scale, cy + 22f * scale)
                }
                canvas.drawPath(mouth, paint)
            }
        }
        canvas.restore()
    }

    fun drawSpecies(
        canvas: Canvas,
        sp: SpeciesItem,
        cx: Float,
        cy: Float,
        scale: Float,
        silhouette: Boolean,
        paint: Paint
    ) {
        val bodyC = if (silhouette) 0xFFBFB8AC.toInt() else parseColorSafe(sp.color)
        if (!silhouette) {
            drawPedestal(canvas, sp.base, cx, cy + 66f * scale, scale, paint)
        }
        paint.shader = null
        paint.style = Paint.Style.FILL
        paint.color = bodyC

        // 尾巴
        when (sp.tail) {
            "puff" -> {
                canvas.drawCircle(cx + 52f * scale, cy + 36f * scale, 13f * scale, paint)
                canvas.drawCircle(cx + 60f * scale, cy + 28f * scale, 8f * scale, paint)
            }
            "fox", "dragon" -> {
                val p = Path().apply {
                    moveTo(cx + 42f * scale, cy + 38f * scale)
                    quadTo(cx + 76f * scale, cy + 42f * scale, cx + 70f * scale, cy + 12f * scale)
                    quadTo(cx + 56f * scale, cy + 14f * scale, cx + 44f * scale, cy + 26f * scale)
                    close()
                }
                canvas.drawPath(p, paint)
            }
            "wing" -> {
                val lw = Path().apply {
                    moveTo(cx - 50f * scale, cy + 8f * scale)
                    quadTo(cx - 82f * scale, cy - 16f * scale, cx - 64f * scale, cy - 36f * scale)
                    quadTo(cx - 46f * scale, cy - 28f * scale, cx - 42f * scale, cy - 4f * scale)
                    close()
                }
                val rw = Path().apply {
                    moveTo(cx + 50f * scale, cy + 8f * scale)
                    quadTo(cx + 82f * scale, cy - 16f * scale, cx + 64f * scale, cy - 36f * scale)
                    quadTo(cx + 46f * scale, cy - 28f * scale, cx + 42f * scale, cy - 4f * scale)
                    close()
                }
                canvas.drawPath(lw, paint)
                canvas.drawPath(rw, paint)
            }
        }

        // 耳朵
        when (sp.ear) {
            "tall" -> {
                canvas.drawOval(RectF(cx - 38f * scale, cy - 72f * scale, cx - 16f * scale, cy - 14f * scale), paint)
                canvas.drawOval(RectF(cx + 16f * scale, cy - 72f * scale, cx + 38f * scale, cy - 14f * scale), paint)
            }
            "pointy" -> {
                val le = Path().apply {
                    moveTo(cx - 46f * scale, cy - 14f * scale)
                    lineTo(cx - 32f * scale, cy - 64f * scale)
                    lineTo(cx - 12f * scale, cy - 24f * scale)
                    close()
                }
                val re = Path().apply {
                    moveTo(cx + 46f * scale, cy - 14f * scale)
                    lineTo(cx + 32f * scale, cy - 64f * scale)
                    lineTo(cx + 12f * scale, cy - 24f * scale)
                    close()
                }
                canvas.drawPath(le, paint)
                canvas.drawPath(re, paint)
            }
            "horn", "unicorn" -> {
                paint.color = if (silhouette) bodyC else 0xFFF0C24B.toInt()
                val h = Path().apply {
                    moveTo(cx - 12f * scale, cy - 36f * scale)
                    lineTo(cx, cy - 72f * scale)
                    lineTo(cx + 12f * scale, cy - 36f * scale)
                    close()
                }
                canvas.drawPath(h, paint)
                paint.color = bodyC
            }
            "none" -> {}
            else -> {
                canvas.drawCircle(cx - 36f * scale, cy - 28f * scale, 16f * scale, paint)
                canvas.drawCircle(cx + 36f * scale, cy - 28f * scale, 16f * scale, paint)
            }
        }

        // 身体
        paint.color = bodyC
        canvas.drawOval(RectF(cx - 52f * scale, cy - 36f * scale, cx + 52f * scale, cy + 56f * scale), paint)

        if (!silhouette) {
            // 高光 + 肚皮
            paint.color = 0x55FFFFFF
            canvas.drawOval(RectF(cx - 36f * scale, cy - 28f * scale, cx + 36f * scale, cy - 2f * scale), paint)
            paint.color = 0xB8FFFDF8.toInt()
            canvas.drawOval(RectF(cx - 30f * scale, cy + 10f * scale, cx + 30f * scale, cy + 48f * scale), paint)

            // 配饰
            when (sp.acc) {
                "scarf" -> {
                    paint.color = 0xFFC95742.toInt()
                    canvas.drawRoundRect(
                        RectF(cx - 38f * scale, cy + 28f * scale, cx + 38f * scale, cy + 40f * scale),
                        8f * scale, 8f * scale, paint
                    )
                }
                "crown" -> {
                    paint.color = 0xFFF2C44D.toInt()
                    val cr = Path().apply {
                        moveTo(cx - 18f * scale, cy - 36f * scale)
                        lineTo(cx - 10f * scale, cy - 56f * scale)
                        lineTo(cx, cy - 42f * scale)
                        lineTo(cx + 10f * scale, cy - 56f * scale)
                        lineTo(cx + 18f * scale, cy - 36f * scale)
                        close()
                    }
                    canvas.drawPath(cr, paint)
                }
                "bow" -> {
                    paint.color = 0xFFE2758E.toInt()
                    canvas.drawCircle(cx - 12f * scale, cy - 34f * scale, 8f * scale, paint)
                    canvas.drawCircle(cx + 12f * scale, cy - 34f * scale, 8f * scale, paint)
                    canvas.drawCircle(cx, cy - 34f * scale, 5f * scale, paint)
                }
                "star" -> {
                    paint.color = 0xFFF2C44D.toInt()
                    canvas.drawCircle(cx, cy - 40f * scale, 7f * scale, paint)
                }
            }

            drawEyes(canvas, cx - 19f * scale, cx + 19f * scale, cy + 4f * scale, scale, false, paint)
            paint.shader = null
            paint.style = Paint.Style.FILL
            paint.color = 0x7AF28B82
            canvas.drawOval(RectF(cx - 42f * scale, cy + 14f * scale, cx - 26f * scale, cy + 24f * scale), paint)
            canvas.drawOval(RectF(cx + 26f * scale, cy + 14f * scale, cx + 42f * scale, cy + 24f * scale), paint)
            paint.style = Paint.Style.STROKE
            paint.color = 0xFF3A2F22.toInt()
            paint.strokeWidth = 2.2f * scale
            val m = Path().apply {
                moveTo(cx - 8f * scale, cy + 18f * scale)
                quadTo(cx, cy + 25f * scale, cx + 8f * scale, cy + 18f * scale)
            }
            canvas.drawPath(m, paint)
        } else {
            paint.color = 0xFFFFFFFF.toInt()
            paint.textSize = 36f * scale
            paint.textAlign = Paint.Align.CENTER
            paint.isFakeBoldText = true
            canvas.drawText("?", cx, cy + 16f * scale, paint)
        }
    }

    fun drawChildAvatar(canvas: Canvas, avatarIdx: Int, cx: Float, cy: Float, radius: Float, paint: Paint) {
        val safeIdx = if (avatarIdx in GrowthMeta.AVATARS.indices) avatarIdx else 0
        val pal = GrowthMeta.AVATARS[safeIdx]
        val s = radius / 30f

        canvas.save()
        val clip = Path().apply { addCircle(cx, cy, radius, Path.Direction.CW) }
        canvas.clipPath(clip)

        paint.shader = null
        paint.style = Paint.Style.FILL
        paint.color = 0xFFFDF8EF.toInt()
        canvas.drawCircle(cx, cy, radius, paint)

        // 后层发型（丸子头、波波头、长发、双马尾）
        paint.color = pal.hair
        when (pal.style) {
            "bun" -> {
                canvas.drawCircle(cx - 13f * s, cy - 16f * s, 6f * s, paint)
                canvas.drawCircle(cx + 13f * s, cy - 16f * s, 6f * s, paint)
            }
            "bob" -> {
                canvas.drawRoundRect(
                    RectF(cx - 15f * s, cy - 10f * s, cx + 15f * s, cy + 11f * s),
                    7f * s, 7f * s, paint
                )
            }
            "long" -> {
                canvas.drawRoundRect(
                    RectF(cx - 16f * s, cy - 10f * s, cx + 16f * s, cy + 19f * s),
                    8f * s, 8f * s, paint
                )
            }
            "pigtails" -> {
                canvas.drawCircle(cx - 17f * s, cy - 5f * s, 6f * s, paint)
                canvas.drawCircle(cx + 17f * s, cy - 5f * s, 6f * s, paint)
                canvas.drawOval(RectF(cx - 22f * s, cy - 5f * s, cx - 14f * s, cy + 10f * s), paint)
                canvas.drawOval(RectF(cx + 14f * s, cy - 5f * s, cx + 22f * s, cy + 10f * s), paint)
            }
        }

        // 衣服 + 领口高光
        paint.color = pal.shirt
        canvas.drawOval(RectF(cx - 16f * s, cy + 9f * s, cx + 16f * s, cy + 35f * s), paint)
        paint.style = Paint.Style.STROKE
        paint.color = 0xC0FFFFFF.toInt()
        paint.strokeWidth = 2f * s
        paint.strokeCap = Paint.Cap.ROUND
        val collar = Path().apply {
            moveTo(cx - 7f * s, cy + 11f * s)
            quadTo(cx, cy + 16f * s, cx + 7f * s, cy + 11f * s)
        }
        canvas.drawPath(collar, paint)

        // 脸蛋
        paint.style = Paint.Style.FILL
        paint.color = pal.skin
        canvas.drawCircle(cx, cy - 6f * s, 13f * s, paint)

        // 头顶帽子或刘海
        if (pal.style == "cap") {
            paint.color = 0xFFE8913A.toInt()
            canvas.drawArc(
                RectF(cx - 14f * s, cy - 23f * s, cx + 14f * s, cy + 5f * s),
                180f, 180f, true, paint
            )
            paint.color = 0xFFCF7924.toInt()
            canvas.drawRoundRect(
                RectF(cx - 15f * s, cy - 11f * s, cx + 17f * s, cy - 7f * s),
                2f * s, 2f * s, paint
            )
        } else {
            paint.color = pal.hair
            val bangs = Path().apply {
                arcTo(RectF(cx - 13f * s, cy - 20f * s, cx + 13f * s, cy + 6f * s), 185f, 170f, true)
                quadTo(cx + 6f * s, cy - 15f * s, cx, cy - 12f * s)
                quadTo(cx - 6f * s, cy - 15f * s, cx - 13f * s, cy - 9f * s)
                close()
            }
            canvas.drawPath(bangs, paint)
        }

        // 眼睛 + 亮点
        paint.color = 0xFF3A2F22.toInt()
        canvas.drawCircle(cx - 4.5f * s, cy - 6f * s, 1.8f * s, paint)
        canvas.drawCircle(cx + 4.5f * s, cy - 6f * s, 1.8f * s, paint)
        paint.color = Color.WHITE
        canvas.drawCircle(cx - 4.0f * s, cy - 6.6f * s, 0.65f * s, paint)
        canvas.drawCircle(cx + 5.0f * s, cy - 6.6f * s, 0.65f * s, paint)

        // 眼镜
        if (pal.style == "glasses") {
            paint.style = Paint.Style.STROKE
            paint.color = 0xFF3A2F22.toInt()
            paint.strokeWidth = 1.4f * s
            canvas.drawCircle(cx - 4.5f * s, cy - 6f * s, 4.1f * s, paint)
            canvas.drawCircle(cx + 4.5f * s, cy - 6f * s, 4.1f * s, paint)
            canvas.drawLine(cx - 0.4f * s, cy - 6f * s, cx + 0.4f * s, cy - 6f * s, paint)
        }

        // 微笑
        paint.style = Paint.Style.STROKE
        paint.color = 0xFF3A2F22.toInt()
        paint.strokeWidth = 1.5f * s
        val smile = Path().apply {
            moveTo(cx - 4f * s, cy)
            quadTo(cx, cy + 3.2f * s, cx + 4f * s, cy)
        }
        canvas.drawPath(smile, paint)

        // 胡须
        if (pal.style == "beard") {
            paint.style = Paint.Style.FILL
            paint.color = pal.hair
            paint.alpha = 215
            val beard = Path().apply {
                moveTo(cx - 11f * s, cy - 4f * s)
                quadTo(cx - 11f * s, cy + 7f * s, cx, cy + 7f * s)
                quadTo(cx + 11f * s, cy + 7f * s, cx + 11f * s, cy - 4f * s)
                quadTo(cx + 7f * s, cy + 2f * s, cx, cy + 2f * s)
                quadTo(cx - 7f * s, cy + 2f * s, cx - 11f * s, cy - 4f * s)
                close()
            }
            canvas.drawPath(beard, paint)
            paint.alpha = 255
        }

        // 腮红
        paint.style = Paint.Style.FILL
        paint.color = 0x66E87968
        canvas.drawOval(RectF(cx - 12.5f * s, cy - 3.6f * s, cx - 7.1f * s, cy), paint)
        canvas.drawOval(RectF(cx + 7.1f * s, cy - 3.6f * s, cx + 12.5f * s, cy), paint)

        canvas.restore()

        // 精致外圈描边
        paint.style = Paint.Style.STROKE
        paint.color = 0xFFE4D8C4.toInt()
        paint.strokeWidth = max(1.5f, 1.4f * s)
        canvas.drawCircle(cx, cy, radius - paint.strokeWidth * 0.5f, paint)
    }

    fun drawCircleAvatarBitmap(canvas: Canvas, bmp: Bitmap, cx: Float, cy: Float, radius: Float, paint: Paint) {
        canvas.save()
        val clip = Path().apply { addCircle(cx, cy, radius, Path.Direction.CW) }
        canvas.clipPath(clip)

        val bw = bmp.width
        val bh = bmp.height
        val side = min(bw, bh)
        val sx = (bw - side) / 2
        val sy = (bh - side) / 2
        val src = Rect(sx, sy, sx + side, sy + side)
        val dst = RectF(cx - radius, cy - radius, cx + radius, cy + radius)
        paint.shader = null
        paint.style = Paint.Style.FILL
        paint.isFilterBitmap = true
        canvas.drawBitmap(bmp, src, dst, paint)
        canvas.restore()

        paint.style = Paint.Style.STROKE
        paint.color = 0xFFE4D8C4.toInt()
        paint.strokeWidth = max(2f, radius * 0.05f)
        canvas.drawCircle(cx, cy, radius - paint.strokeWidth * 0.5f, paint)
    }
}

class PetStageView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    private data class Particle(
        val kind: Int, // 0=food, 1=heart, 2=spark
        var x: Float,
        var y: Float,
        var vx: Float,
        var vy: Float,
        var tx: Float = 0f,
        var ty: Float = 0f,
        var progress: Float = 0f,
        var life: Float = 1f,
        var size: Float = 10f,
        val color: Int
    )

    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val particles = mutableListOf<Particle>()

    private var level: Int = 0
    private var asleep: Boolean = false
    private var companion: SpeciesItem? = null

    private var hopAnimStartMs: Long = 0L
    private var squishAnimStartMs: Long = 0L
    private var lastFrameMs: Long = SystemClock.uptimeMillis()

    fun setPetState(lv: Int, isSleep: Boolean, activeComp: SpeciesItem?) {
        level = lv
        asleep = isSleep
        companion = activeComp
        invalidate()
    }

    fun triggerPatAnimation() {
        squishAnimStartMs = SystemClock.uptimeMillis()
        spawnBurst(1, width * 0.5f, height * 0.36f, 7)
        invalidate()
    }

    fun triggerFeedAnimation() {
        hopAnimStartMs = SystemClock.uptimeMillis()
        val colors = intArrayOf(0xFFE8913A.toInt(), 0xFF6CB88C.toInt(), 0xFFF2C44D.toInt(), 0xFFE46D5B.toInt())
        val cx = width * 0.5f
        val cy = height * 0.55f
        for (i in 0 until 9) {
            particles.add(
                Particle(
                    kind = 0,
                    x = width * 0.12f + Random.nextFloat() * width * 0.76f,
                    y = height * 0.92f,
                    vx = 0f,
                    vy = 0f,
                    tx = cx + (Random.nextFloat() - 0.5f) * 28f,
                    ty = cy,
                    progress = -i * 0.08f,
                    size = 10f + Random.nextFloat() * 5f,
                    color = colors[i % colors.size]
                )
            )
        }
        invalidate()
    }

    fun triggerCelebration() {
        hopAnimStartMs = SystemClock.uptimeMillis()
        spawnBurst(2, width * 0.5f, height * 0.42f, 22)
        spawnBurst(1, width * 0.5f, height * 0.38f, 10)
        invalidate()
    }

    private fun spawnBurst(kind: Int, cx: Float, cy: Float, count: Int) {
        val sparkColors = intArrayOf(0xFFF0C24B.toInt(), 0xFFF7D98B.toInt(), 0xFFE8913A.toInt(), 0xFF6CB88C.toInt())
        for (i in 0 until count) {
            val angle = Random.nextFloat() * 2f * PI.toFloat()
            val speed = 2.4f + Random.nextFloat() * 6.5f
            particles.add(
                Particle(
                    kind = kind,
                    x = cx,
                    y = cy,
                    vx = cos(angle) * speed,
                    vy = sin(angle) * speed - 3.6f,
                    life = 1f,
                    size = if (kind == 1) 24f + Random.nextFloat() * 10f else 8f + Random.nextFloat() * 7f,
                    color = if (kind == 1) 0xFFE46D5B.toInt() else sparkColors[i % sparkColors.size]
                )
            )
        }
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val now = SystemClock.uptimeMillis()
        val dt = ((now - lastFrameMs).coerceIn(8L, 40L)) / 16f
        lastFrameMs = now

        val w = width.toFloat().coerceAtLeast(1f)
        val h = height.toFloat().coerceAtLeast(1f)
        val scale = min(w / 240f, h / 205f)
        val cx = w * 0.5f
        val cy = h * 0.50f
        val breath = sin(now / 520.0).toFloat()

        // 跳跃 & Q弹物理变换
        var offsetY = 0f
        var sx = 1f
        var sy = 1f
        val hopElapsed = now - hopAnimStartMs
        if (hopAnimStartMs > 0L && hopElapsed < 560L) {
            val p = hopElapsed / 560f
            offsetY = -sin(p * PI).toFloat() * 34f * scale
            sx = 1f + sin(p * 2 * PI).toFloat() * 0.06f
            sy = 1f - sin(p * 2 * PI).toFloat() * 0.06f
        }
        val sqElapsed = now - squishAnimStartMs
        if (squishAnimStartMs > 0L && sqElapsed < 400L) {
            val p = sqElapsed / 400f
            val wave = sin(p * 2 * PI).toFloat() * (1f - p)
            sx *= 1f + wave * 0.13f
            sy *= 1f - wave * 0.12f
        }

        // 绘制主宠
        canvas.save()
        canvas.translate(0f, offsetY)
        canvas.scale(sx, sy, cx, cy + 68f * scale)
        PetArtCanvas.drawMainPet(canvas, level, cx, cy, scale, asleep, breath, paint)
        canvas.restore()

        // 绘制身旁的陪伴伙伴
        companion?.let { sp ->
            val compScale = scale * 0.45f
            val compX = cx + 82f * scale
            val compY = cy + 36f * scale + sin(now / 430.0 + 1.1).toFloat() * 3f * scale
            PetArtCanvas.drawSpecies(canvas, sp, compX, compY, compScale, silhouette = false, paint = paint)
        }

        // 睡眠 Zzz
        if (asleep) {
            paint.shader = null
            paint.style = Paint.Style.FILL
            paint.color = 0xFFA79F92.toInt()
            paint.textSize = 24f * scale
            paint.textAlign = Paint.Align.CENTER
            val zy = cy - 50f * scale + sin(now / 420.0).toFloat() * 6f * scale
            canvas.drawText("💤", cx + 52f * scale, zy, paint)
        }

        // 更新并绘制粒子
        val iter = particles.listIterator()
        while (iter.hasNext()) {
            val p = iter.next()
            if (p.kind == 0) {
                p.progress += dt * 0.042f
                if (p.progress < 0f) continue
                if (p.progress >= 1f) {
                    iter.remove()
                    continue
                }
                val e = p.progress
                val ex = (1f - e) * p.x + e * p.tx
                val ey = (1f - e) * (1f - e) * p.y + (1f - (1f - e) * (1f - e)) * p.ty
                paint.shader = null
                paint.style = Paint.Style.FILL
                paint.color = p.color
                canvas.drawCircle(ex, ey, p.size, paint)
            } else {
                p.x += p.vx * dt
                p.y += p.vy * dt
                p.vy += 0.14f * dt
                p.life -= 0.022f * dt
                if (p.life <= 0f) {
                    iter.remove()
                    continue
                }
                paint.shader = null
                paint.style = Paint.Style.FILL
                paint.color = p.color
                paint.alpha = (max(0f, p.life) * 255).toInt()
                if (p.kind == 1) {
                    paint.textSize = p.size
                    paint.textAlign = Paint.Align.CENTER
                    canvas.drawText("❤", p.x, p.y, paint)
                } else {
                    canvas.drawCircle(p.x, p.y, p.size * 0.5f, paint)
                }
                paint.alpha = 255
            }
        }

        // 如果刚好有食物飞到终点，补一波爱心
        if (hopAnimStartMs > 0L && now - hopAnimStartMs in 340L..380L && particles.none { it.kind == 1 }) {
            spawnBurst(1, cx, cy - 10f * scale, 6)
            spawnBurst(2, cx, cy, 10)
        }

        postInvalidateOnAnimation()
    }
}

class SpeciesThumbView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    private var species: SpeciesItem? = null
    private var silhouette: Boolean = false
    private var avatarIndex: Int? = null
    private var avatarBitmap: Bitmap? = null
    private var mainPetLv: Int? = null

    fun setSpecies(sp: SpeciesItem, isSilhouette: Boolean) {
        species = sp
        silhouette = isSilhouette
        avatarIndex = null
        avatarBitmap = null
        mainPetLv = null
        invalidate()
    }

    fun setAvatar(idx: Int, bmp: Bitmap? = null) {
        avatarIndex = idx
        avatarBitmap = bmp
        species = null
        mainPetLv = null
        invalidate()
    }

    fun setMainPet(lv: Int) {
        mainPetLv = lv
        species = null
        avatarIndex = null
        avatarBitmap = null
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val w = width.toFloat().coerceAtLeast(1f)
        val h = height.toFloat().coerceAtLeast(1f)
        val cx = w * 0.5f
        val cy = h * 0.54f
        val scale = min(w / 185f, h / 170f)

        avatarBitmap?.let { bmp ->
            if (!bmp.isRecycled) {
                PetArtCanvas.drawCircleAvatarBitmap(canvas, bmp, cx, h * 0.5f, min(w, h) * 0.45f, paint)
                return
            }
        }
        avatarIndex?.let {
            PetArtCanvas.drawChildAvatar(canvas, it, cx, h * 0.5f, min(w, h) * 0.45f, paint)
            return
        }
        mainPetLv?.let {
            PetArtCanvas.drawMainPet(canvas, it, cx, cy, scale, sleep = false, breath = 0f, paint = paint)
            return
        }
        species?.let {
            PetArtCanvas.drawSpecies(canvas, it, cx, cy, scale, silhouette = silhouette, paint = paint)
        }
    }
}
