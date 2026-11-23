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
    private var petHealth: String = "normal"
    private var companion: SpeciesItem? = null

    private var hopAnimStartMs: Long = 0L
    private var squishAnimStartMs: Long = 0L
    private var lastFrameMs: Long = SystemClock.uptimeMillis()

    fun setPetState(lv: Int, isSleep: Boolean, activeComp: SpeciesItem?, health: String = "normal") {
        level = lv
        asleep = isSleep
        companion = activeComp
        petHealth = health
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

        // 睡眠 Zzz 或 健康状态角标（虚弱 / 生病）
        if (asleep) {
            paint.shader = null
            paint.style = Paint.Style.FILL
            paint.color = 0xFFA79F92.toInt()
            paint.textSize = 24f * scale
            paint.textAlign = Paint.Align.CENTER
            val zy = cy - 50f * scale + sin(now / 420.0).toFloat() * 6f * scale
            canvas.drawText("💤", cx + 52f * scale, zy, paint)
        } else if (petHealth == "weak" || petHealth == "sick") {
            paint.shader = null
            paint.style = Paint.Style.FILL
            paint.textSize = 26f * scale
            paint.textAlign = Paint.Align.CENTER
            val hy = cy - 54f * scale + sin(now / 380.0).toFloat() * 5f * scale
            val icon = if (petHealth == "sick") "🤒" else "💧"
            canvas.drawText(icon, cx + 56f * scale, hy, paint)
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

/**
 * 冒险小剧场舞台：左侧己方宠物，右侧史莱姆风「拖延怪」，
 * 支持 Q 弹待机呼吸、蓄力光环、残影冲刺、冲击波与凝胶飞溅等丰富战斗演出。
 */
class BattleStageView @JvmOverloads constructor(
    context: Context,
    attrs: AttributeSet? = null
) : View(context, attrs) {

    private data class BurstParticle(
        val bornMs: Float,
        val lifeMs: Float,
        val originRelX: Float,
        val originRelY: Float,
        val vx: Float,
        val vy: Float,
        val gravity: Float,
        val size: Float,
        val color: Int,
        val kind: Int // 0=圆形光粒, 1=水滴飞沫, 2=四角星芒
    )

    private val paint = Paint(Paint.ANTI_ALIAS_FLAG)
    private val monsterPaint = Paint(Paint.ANTI_ALIAS_FLAG)
    private var species: SpeciesItem? = null
    private var stage: Int = 0
    private var asleep: Boolean = false

    private var monsterHp: Int = 3
    private var monsterMaxHp: Int = 3
    private var finishingBlow: Boolean = false

    private var busy = false
    private var animStartMs = 0L
    private val idleBaseMs = SystemClock.uptimeMillis()
    private var hitFired = false
    private var onHitCallback: (() -> Unit)? = null
    private var onWinCallback: (() -> Unit)? = null
    private var particles: List<BurstParticle> = emptyList()
    private var attached = false

    private val battleDurationMs = 1480L
    private val hitTriggerMs = 500L

    private val idleTicker = object : Runnable {
        override fun run() {
            if (!attached || busy) return
            invalidate()
            postDelayed(this, 33L)
        }
    }

    private val animator = object : Runnable {
        override fun run() {
            if (!busy) return
            val elapsed = SystemClock.uptimeMillis() - animStartMs
            if (elapsed >= hitTriggerMs && !hitFired) {
                hitFired = true
                onHitCallback?.invoke()
            }
            if (elapsed >= battleDurationMs) {
                busy = false
                invalidate()
                onWinCallback?.invoke()
                if (attached) {
                    removeCallbacks(idleTicker)
                    postDelayed(idleTicker, 33L)
                }
                return
            }
            invalidate()
            postOnAnimation(this)
        }
    }

    fun setPet(species: SpeciesItem?, stage: Int, asleep: Boolean = false) {
        this.species = species
        this.stage = stage.coerceIn(0, 5)
        this.asleep = asleep
        invalidate()
    }

    fun setMonsterHp(hp: Int, maxHp: Int = 3) {
        this.monsterMaxHp = maxHp.coerceAtLeast(1)
        this.monsterHp = hp.coerceIn(0, this.monsterMaxHp)
        invalidate()
    }

    fun triggerAttack(isFinishingBlow: Boolean = false, onHit: () -> Unit, onWin: () -> Unit) {
        if (busy) return
        busy = true
        hitFired = false
        finishingBlow = isFinishingBlow
        animStartMs = SystemClock.uptimeMillis()
        onHitCallback = onHit
        onWinCallback = onWin
        particles = buildBattleParticles(isFinishingBlow)
        removeCallbacks(idleTicker)
        postOnAnimation(animator)
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        attached = true
        if (!busy) {
            removeCallbacks(idleTicker)
            postDelayed(idleTicker, 33L)
        }
    }

    override fun onDetachedFromWindow() {
        attached = false
        super.onDetachedFromWindow()
        removeCallbacks(animator)
        removeCallbacks(idleTicker)
        busy = false
    }

    private fun buildBattleParticles(isKo: Boolean): List<BurstParticle> {
        val list = ArrayList<BurstParticle>(28)
        // 冲刺起跳尘埃粒子
        for (i in 0 until 5) {
            val angle = Math.toRadians(145.0 + i * 16.0)
            val spd = 48f + (i % 3) * 18f
            list.add(
                BurstParticle(
                    bornMs = 260f,
                    lifeMs = 420f,
                    originRelX = 0.24f,
                    originRelY = 0.76f,
                    vx = (cos(angle) * spd).toFloat(),
                    vy = (sin(angle) * spd - 20f).toFloat(),
                    gravity = 90f,
                    size = 6f + (i % 2) * 3f,
                    color = 0xFFD9CDB8.toInt(),
                    kind = 0
                )
            )
        }
        // 命中瞬间星芒 + 史莱姆凝胶飞沫
        val burstCount = if (isKo) 20 else 14
        val colors = intArrayOf(0xFFF6C445.toInt(), 0xFF4BE3D0.toInt(), 0xFFFF8577.toInt(), 0xFF7BE8FF.toInt(), 0xFFFFE57F.toInt())
        for (i in 0 until burstCount) {
            val angle = (2.0 * PI * i / burstCount) + (i % 3) * 0.17
            val speed = (if (isKo) 150f else 115f) + (i % 4) * 32f
            val kind = when {
                i % 3 == 0 -> 2
                i % 3 == 1 -> 1
                else -> 0
            }
            list.add(
                BurstParticle(
                    bornMs = 500f,
                    lifeMs = if (isKo) 760f else 600f,
                    originRelX = 0.74f,
                    originRelY = 0.56f,
                    vx = (cos(angle) * speed).toFloat(),
                    vy = (sin(angle) * speed - 55f).toFloat(),
                    gravity = 210f,
                    size = if (kind == 2) 11f else 7.5f + (i % 3) * 2.5f,
                    color = colors[i % colors.size],
                    kind = kind
                )
            )
        }
        return list
    }

    override fun onDraw(canvas: Canvas) {
        super.onDraw(canvas)
        val w = width.toFloat().coerceAtLeast(1f)
        val h = height.toFloat().coerceAtLeast(1f)
        val nowMs = SystemClock.uptimeMillis()
        val idleSec = (nowMs - idleBaseMs) / 1000f
        val elapsed = if (busy) (nowMs - animStartMs).toFloat() else -1f

        // 命中瞬间轻微震屏 (500..760ms)
        var shakeX = 0f
        var shakeY = 0f
        if (elapsed in 500f..760f) {
            val st = (elapsed - 500f) / 260f
            val amp = (1f - st) * (if (finishingBlow) 11f else 7.5f)
            shakeX = sin(st * PI * 7.0).toFloat() * amp
            shakeY = cos(st * PI * 5.0).toFloat() * (amp * 0.65f)
        }

        canvas.save()
        canvas.translate(shakeX, shakeY)

        drawArenaBackdrop(canvas, w, h, idleSec, elapsed)

        // 1. 己方宠物运动轨迹与形变
        val basePetX = w * 0.23f
        val basePetY = h * 0.59f
        val petScale = min(w / 320f, h / 145f) * 0.90f
        var fighterDx = 0f
        var fighterDy = 0f
        var petSx = 1f
        var petSy = 1f
        var petRot = 0f

        if (elapsed < 0f) {
            // 待机元气呼吸
            fighterDy = -kotlin.math.abs(sin(idleSec * 2.8f)) * (h * 0.025f)
        } else {
            when {
                elapsed < 260f -> {
                    // 蓄力压低蹲起
                    val t = elapsed / 260f
                    val s = sin(t * PI * 0.5).toFloat()
                    petSx = 1f + 0.14f * s
                    petSy = 1f - 0.14f * s
                    fighterDy = h * 0.05f * s
                    petRot = -6f * s
                }
                elapsed < 530f -> {
                    // 高速跃起冲向史莱姆
                    val t = (elapsed - 260f) / 270f
                    val ease = 1f - (1f - t) * (1f - t)
                    fighterDx = w * 0.34f * ease
                    fighterDy = -sin(t * PI).toFloat() * (h * 0.19f)
                    petSx = 1.12f - 0.08f * t
                    petSy = 0.92f + 0.08f * t
                    petRot = 12f * (1f - t * 0.5f)
                }
                elapsed < 980f -> {
                    // 命中后优雅后空翻/弹回落地
                    val t = (elapsed - 530f) / 450f
                    fighterDx = w * 0.34f * (1f - t)
                    fighterDy = -sin(t * PI).toFloat() * (h * 0.14f)
                    petRot = if (t < 0.7f) -10f * sin((t / 0.7f) * PI).toFloat() else 0f
                    if (t > 0.85f) {
                        val lt = (t - 0.85f) / 0.15f
                        petSx = 1f + 0.10f * sin(lt * PI).toFloat()
                        petSy = 1f - 0.10f * sin(lt * PI).toFloat()
                    }
                }
            }
        }

        // 蓄力金色光环 (40..320ms)
        if (elapsed in 40f..320f) {
            drawChargeAura(canvas, basePetX + fighterDx, basePetY + fighterDy, min(w, h) * 0.24f, (elapsed - 40f) / 280f)
        }

        // 冲刺速度线 & 残影拖尾 (260..540ms)
        if (elapsed in 260f..540f) {
            val st = (elapsed - 260f) / 280f
            drawSpeedLinesAndAfterimages(canvas, w, h, basePetX, basePetY, fighterDx, fighterDy, petScale, st)
        }

        // 绘制己方主角宠物
        val fighterCx = basePetX + fighterDx
        val fighterCy = basePetY + fighterDy
        canvas.save()
        canvas.translate(fighterCx, fighterCy)
        canvas.rotate(petRot)
        canvas.scale(petSx, petSy)
        val sp = species
        if (sp != null) {
            PetArtCanvas.drawSpecies(canvas, sp, 0f, 0f, petScale, silhouette = false, paint = paint)
        } else {
            PetArtCanvas.drawMainPet(canvas, stage.coerceIn(0, 5), 0f, 0f, petScale, sleep = asleep, breath = sin(idleSec * 2.5f) * 0.5f, paint = paint)
        }
        canvas.restore()

        // 2. 飞驰元气光弹 (310..520ms)
        if (elapsed in 310f..520f) {
            drawEnergyProjectile(canvas, w, h, (elapsed - 310f) / 210f)
        }

        // 3. 右侧史莱姆风「拖延怪」
        val foeCx = w * 0.77f
        val foeCy = h * 0.59f
        val r = min(w, h) * 0.255f
        drawSlimeMonster(canvas, foeCx, foeCy, r, idleSec, elapsed)

        // 4. 命中爆发冲击波 & 星芒斩击 (490..920ms)
        if (elapsed in 490f..920f) {
            drawImpactBurst(canvas, foeCx - r * 0.15f, foeCy - r * 0.12f, r, (elapsed - 490f) / 430f)
        }

        // 5. 物理飞溅粒子 (260..1350ms)
        if (elapsed >= 260f) {
            drawParticles(canvas, w, h, elapsed)
        }

        // 6. 战斗飘字与胜利星芒 (540..1480ms)
        if (elapsed in 540f..1480f) {
            drawFloatingCombatText(canvas, foeCx, foeCy - r * 1.15f, elapsed)
        }

        canvas.restore()
    }

    private fun drawArenaBackdrop(canvas: Canvas, w: Float, h: Float, idleSec: Float, elapsed: Float) {
        paint.reset()
        paint.isAntiAlias = true
        paint.style = Paint.Style.FILL

        // 主角脚下草坪阴影
        paint.color = 0xFFDDE5D0.toInt()
        canvas.drawOval(w * 0.11f, h * 0.77f, w * 0.35f, h * 0.87f, paint)

        // 史莱姆脚下黏液微光水洼阴影
        paint.color = 0xFFD5E9E8.toInt()
        canvas.drawOval(w * 0.63f, h * 0.77f, w * 0.91f, h * 0.88f, paint)

        // 舞台背景漂浮微光点
        for (i in 0 until 4) {
            val px = w * (0.38f + i * 0.08f)
            val py = h * (0.20f + 0.10f * sin(idleSec * 1.6f + i * 1.4f))
            val alpha = (70 + 45 * sin(idleSec * 2.2f + i)).toInt().coerceIn(25, 130)
            paint.color = (alpha shl 24) or 0x00F6C445
            canvas.drawCircle(px, py, 3.2f + (i % 2) * 1.5f, paint)
        }

        // 命中高光短暂闪烁背景
        if (elapsed in 495f..585f) {
            val alpha = ((1f - (elapsed - 495f) / 90f) * 85f).toInt().coerceIn(0, 85)
            paint.color = (alpha shl 24) or 0x00FFFFFF
            canvas.drawRect(0f, 0f, w, h, paint)
        }
    }

    private fun drawChargeAura(canvas: Canvas, cx: Float, cy: Float, radius: Float, t: Float) {
        paint.reset()
        paint.isAntiAlias = true
        val alpha = ((sin(t * PI).toFloat()) * 210).toInt().coerceIn(0, 210)
        paint.style = Paint.Style.STROKE
        paint.strokeWidth = 4.5f
        paint.color = (alpha shl 24) or 0x00F6C445
        val ringR = radius * (1.35f - 0.55f * t)
        canvas.drawCircle(cx, cy, ringR, paint)

        paint.style = Paint.Style.FILL
        for (i in 0 until 6) {
            val a = t * 4.5f + i * (PI.toFloat() * 2f / 6f)
            val dist = radius * (1.25f * (1f - t) + 0.25f)
            val ox = cx + cos(a) * dist
            val oy = cy + sin(a) * dist * 0.72f
            paint.color = (alpha shl 24) or (if (i % 2 == 0) 0x00FFE082 else 0x005CE1E6)
            canvas.drawCircle(ox, oy, 4.5f * (1f - t * 0.35f), paint)
        }
    }

    private fun drawSpeedLinesAndAfterimages(
        canvas: Canvas,
        w: Float,
        h: Float,
        baseX: Float,
        baseY: Float,
        dx: Float,
        dy: Float,
        petScale: Float,
        t: Float
    ) {
        // 3 个残影（仅对默认及通用绘制做半透明光影替代，零重度性能损耗）
        paint.reset()
        paint.isAntiAlias = true
        paint.style = Paint.Style.FILL
        for (idx in 1..3) {
            val trailFrac = idx * 0.24f
            val tx = baseX + dx * (1f - trailFrac)
            val ty = baseY + dy * (1f - trailFrac * 0.6f)
            val alpha = ((1f - t) * (120 - idx * 28)).toInt().coerceIn(0, 120)
            paint.color = (alpha shl 24) or 0x00F8D678
            canvas.drawCircle(tx, ty, 20f * petScale * (1f - idx * 0.12f), paint)
        }
        // 高速气流线
        paint.style = Paint.Style.STROKE
        paint.strokeCap = Paint.Cap.ROUND
        paint.strokeWidth = 3f
        val lineAlpha = ((1f - t) * 185).toInt().coerceIn(0, 185)
        paint.color = (lineAlpha shl 24) or 0x00FFFDF5
        for (k in 0..2) {
            val ly = baseY + (k - 1) * (h * 0.10f)
            val startX = baseX + dx * 0.15f - k * 14f
            val endX = startX + w * 0.18f
            canvas.drawLine(startX, ly, endX, ly, paint)
        }
    }

    private fun drawEnergyProjectile(canvas: Canvas, w: Float, h: Float, t: Float) {
        val sx = w * 0.36f
        val sy = h * 0.50f
        val ex = w * 0.72f
        val ey = h * 0.53f
        val cx = w * 0.54f
        val cy = h * 0.24f
        val u = 1f - t
        val px = u * u * sx + 2 * u * t * cx + t * t * ex
        val py = u * u * sy + 2 * u * t * cy + t * t * ey

        paint.reset()
        paint.isAntiAlias = true
        paint.style = Paint.Style.FILL
        // 拖尾光球
        for (tail in 1..4) {
            val tt = (t - tail * 0.06f).coerceAtLeast(0f)
            val tu = 1f - tt
            val tx = tu * tu * sx + 2 * tu * tt * cx + tt * tt * ex
            val ty = tu * tu * sy + 2 * tu * tt * cy + tt * tt * ey
            paint.color = ((180 - tail * 36) shl 24) or 0x00FFD54F
            canvas.drawCircle(tx, ty, (11f - tail * 1.8f), paint)
        }
        // 核心闪烁星光球
        paint.color = 0x88FFF59D.toInt()
        canvas.drawCircle(px, py, 16f, paint)
        paint.color = Color.WHITE
        drawFourPointStar(canvas, px, py, 13f, t * 180f, paint)
    }

    private fun drawSlimeMonster(
        canvas: Canvas,
        foeCx: Float,
        foeCy: Float,
        r: Float,
        idleSec: Float,
        elapsed: Float
    ) {
        val baseY = foeCy + r * 0.84f
        val hpRatio = monsterHp.toFloat() / monsterMaxHp.toFloat()
        val isDefeated = (monsterHp <= 0 && !busy) || (finishingBlow && elapsed >= 620f)

        // 计算史莱姆 Q 弹缩放、旋转、受击位移与颤抖
        var sx = 1f
        var sy = 1f
        var dx = 0f
        var dy = 0f
        var rot = 0f
        var hitWhiteAlpha = 0
        var isHitReact = false

        when {
            isDefeated -> {
                // 被彻底击败：软趴趴摊成水洼
                val settle = if (elapsed in 620f..1100f) {
                    val dt = (elapsed - 620f) / 480f
                    1f + 0.14f * sin(dt * PI * 3.0).toFloat() * (1f - dt)
                } else 1f
                sx = 1.46f * settle
                sy = 0.42f / settle
                dy = r * 0.04f
            }
            elapsed in 120f..499f -> {
                // 看到主角蓄力冲刺：紧张发抖、微微后仰
                val lt = (elapsed - 120f) / 380f
                dx = sin(lt * PI * 10.0).toFloat() * 2.5f
                sx = 0.96f
                sy = 1.04f
            }
            elapsed in 500f..1160f -> {
                // 命中后：先猛烈压扁回弹，再阻尼果冻晃动
                isHitReact = true
                val ht = (elapsed - 500f) / 660f
                if (ht < 0.22f) {
                    val p = ht / 0.22f
                    sx = 1f + 0.44f * sin(p * PI * 0.5).toFloat()
                    sy = 1f - 0.42f * sin(p * PI * 0.5).toFloat()
                    dx = r * 0.22f * p
                    rot = 14f * p
                    hitWhiteAlpha = ((1f - p * 0.4f) * 180).toInt()
                } else {
                    val wt = (ht - 0.22f) / 0.78f
                    val damp = (1f - wt) * (1f - wt)
                    val osc = sin(wt * PI * 4.5).toFloat() * damp
                    sx = 1f + 0.26f * osc
                    sy = 1f - 0.24f * osc
                    dx = r * 0.22f * (1f - wt)
                    rot = 12f * osc
                    hitWhiteAlpha = ((1f - wt * 2.2f).coerceAtLeast(0f) * 110).toInt()
                }
            }
            else -> {
                // Q 弹史莱姆待机：水滴形上下呼吸弹跳
                val phase = idleSec * 3.6f
                val bounce = sin(phase)
                sx = 1f - 0.07f * bounce
                sy = 1f + 0.08f * bounce
                if (monsterHp == 1 && monsterMaxHp > 1) {
                    // 残血微微发抖
                    dx = sin(idleSec * 28f) * 2.2f
                }
            }
        }

        // 地面半透明小水滴（史莱姆身旁晃动的小分身凝胶滴）
        monsterPaint.reset()
        monsterPaint.isAntiAlias = true
        monsterPaint.style = Paint.Style.FILL
        val dropBob = sin(idleSec * 3.2f) * 3.5f
        monsterPaint.color = 0xAA4BE3D0.toInt()
        canvas.drawCircle(foeCx - r * 1.12f + dx * 0.3f, baseY - r * 0.14f - dropBob, r * 0.13f, monsterPaint)
        canvas.drawCircle(foeCx + r * 1.06f + dx * 0.3f, baseY - r * 0.10f + dropBob * 0.7f, r * 0.10f, monsterPaint)

        canvas.save()
        canvas.translate(foeCx + dx, baseY + dy)
        canvas.rotate(rot, 0f, -r * 0.35f)
        canvas.scale(sx, sy, 0f, 0f)

        // 1. 史莱姆经典水滴尖顶轮廓（底部坐标 y=0）
        val tipSway = sin(idleSec * 4.2f) * r * 0.06f
        val bodyPath = Path().apply {
            moveTo(tipSway, -r * 1.82f)
            // 右侧上弧 -> 饱满右颊 -> 贴地右底
            cubicTo(
                r * 0.18f, -r * 1.46f,
                r * 1.06f, -r * 1.12f,
                r * 1.05f, -r * 0.42f
            )
            cubicTo(
                r * 1.05f, -r * 0.02f,
                r * 0.68f, 0f,
                0f, 0f
            )
            // 左侧贴地 -> 饱满左颊 -> 回到顶部水滴尖
            cubicTo(
                -r * 0.68f, 0f,
                -r * 1.05f, -r * 0.02f,
                -r * 1.05f, -r * 0.42f
            )
            cubicTo(
                -r * 1.06f, -r * 1.12f,
                -r * 0.18f, -r * 1.46f,
                tipSway, -r * 1.82f
            )
            close()
        }

        // 果冻渐变填充（残血略偏蓝紫，平时亮薄荷青）
        val topCol = if (hpRatio <= 0.4f && monsterMaxHp > 1) 0xFFE8E6FF.toInt() else 0xFFD9FFF9.toInt()
        val midCol = if (hpRatio <= 0.4f && monsterMaxHp > 1) 0xFF7FA8F8.toInt() else 0xFF54DEC9.toInt()
        val deepCol = if (hpRatio <= 0.4f && monsterMaxHp > 1) 0xFF3E61B8.toInt() else 0xFF1D949E.toInt()

        monsterPaint.shader = RadialGradient(
            -r * 0.25f, -r * 1.05f, r * 1.65f,
            intArrayOf(topCol, midCol, deepCol),
            floatArrayOf(0f, 0.56f, 1f),
            Shader.TileMode.CLAMP
        )
        canvas.drawPath(bodyPath, monsterPaint)
        monsterPaint.shader = null

        // 体内半透明上升气泡
        monsterPaint.color = 0x55FFFFFF
        val bShift = (idleSec * 18f) % (r * 0.5f)
        canvas.drawCircle(r * 0.48f, -r * 0.34f - bShift * 0.5f, r * 0.09f, monsterPaint)
        canvas.drawCircle(-r * 0.52f, -r * 0.58f + bShift * 0.3f, r * 0.06f, monsterPaint)
        canvas.drawCircle(r * 0.15f, -r * 1.18f, r * 0.055f, monsterPaint)

        // 果冻左上角弯月高光
        monsterPaint.color = 0xA8FFFFFF.toInt()
        val shine = Path().apply {
            moveTo(-r * 0.58f, -r * 0.88f)
            quadTo(-r * 0.42f, -r * 1.24f, -r * 0.16f, -r * 1.32f)
            quadTo(-r * 0.34f, -r * 1.06f, -r * 0.42f, -r * 0.80f)
            close()
        }
        canvas.drawPath(shine, monsterPaint)
        canvas.drawCircle(-r * 0.62f, -r * 0.68f, r * 0.065f, monsterPaint)

        // 外圈果冻描边
        monsterPaint.style = Paint.Style.STROKE
        monsterPaint.strokeWidth = 3.2f
        monsterPaint.color = 0xFF146572.toInt()
        canvas.drawPath(bodyPath, monsterPaint)

        // 命中瞬间闪白遮罩
        if (hitWhiteAlpha > 0) {
            monsterPaint.style = Paint.Style.FILL
            monsterPaint.color = (hitWhiteAlpha shl 24) or 0x00FFFFFF
            canvas.drawPath(bodyPath, monsterPaint)
        }

        // 2. 史莱姆表情（相对于身体锚点）
        val faceY = -r * 0.64f
        monsterPaint.style = Paint.Style.FILL
        monsterPaint.color = 0x66FF85A1
        canvas.drawOval(-r * 0.68f, faceY + r * 0.06f, -r * 0.42f, faceY + r * 0.20f, monsterPaint)
        canvas.drawOval(r * 0.42f, faceY + r * 0.06f, r * 0.68f, faceY + r * 0.20f, monsterPaint)

        if (isDefeated) {
            // 晕倒 × × 眼
            monsterPaint.style = Paint.Style.STROKE
            monsterPaint.strokeWidth = 4f
            monsterPaint.strokeCap = Paint.Cap.ROUND
            monsterPaint.color = 0xFF0E3942.toInt()
            for (side in intArrayOf(-1, 1)) {
                val ex = side * r * 0.32f
                val ey = faceY - r * 0.08f
                val d = r * 0.11f
                canvas.drawLine(ex - d, ey - d, ex + d, ey + d, monsterPaint)
                canvas.drawLine(ex + d, ey - d, ex - d, ey + d, monsterPaint)
            }
            // 波浪晕嘴
            val mouth = Path().apply {
                moveTo(-r * 0.20f, faceY + r * 0.18f)
                quadTo(-r * 0.08f, faceY + r * 0.08f, 0f, faceY + r * 0.18f)
                quadTo(r * 0.08f, faceY + r * 0.28f, r * 0.20f, faceY + r * 0.18f)
            }
            canvas.drawPath(mouth, monsterPaint)
        } else if (isHitReact) {
            // > < 委屈挤眼 + O 形惊讶嘴
            monsterPaint.style = Paint.Style.STROKE
            monsterPaint.strokeWidth = 4.2f
            monsterPaint.strokeCap = Paint.Cap.ROUND
            monsterPaint.color = 0xFF0E3942.toInt()
            val ey = faceY - r * 0.06f
            // 左 >
            canvas.drawLine(-r * 0.44f, ey - r * 0.11f, -r * 0.24f, ey, monsterPaint)
            canvas.drawLine(-r * 0.44f, ey + r * 0.11f, -r * 0.24f, ey, monsterPaint)
            // 右 <
            canvas.drawLine(r * 0.44f, ey - r * 0.11f, r * 0.24f, ey, monsterPaint)
            canvas.drawLine(r * 0.44f, ey + r * 0.11f, r * 0.24f, ey, monsterPaint)
            // 惊讶 O 嘴
            monsterPaint.style = Paint.Style.FILL
            monsterPaint.color = 0xFF144954.toInt()
            canvas.drawOval(-r * 0.12f, faceY + r * 0.06f, r * 0.12f, faceY + r * 0.28f, monsterPaint)
        } else {
            // 水汪汪黑亮大眼（每 ~3 秒眨眼一次）
            val blinkPhase = idleSec % 3.2f
            val blinking = blinkPhase > 3.04f
            val ey = faceY - r * 0.08f
            if (blinking) {
                monsterPaint.style = Paint.Style.STROKE
                monsterPaint.strokeWidth = 4f
                monsterPaint.strokeCap = Paint.Cap.ROUND
                monsterPaint.color = 0xFF0E3942.toInt()
                canvas.drawLine(-r * 0.44f, ey, -r * 0.20f, ey, monsterPaint)
                canvas.drawLine(r * 0.20f, ey, r * 0.44f, ey, monsterPaint)
            } else {
                monsterPaint.style = Paint.Style.FILL
                monsterPaint.color = 0xFF0E3942.toInt()
                canvas.drawOval(-r * 0.42f, ey - r * 0.18f, -r * 0.20f, ey + r * 0.12f, monsterPaint)
                canvas.drawOval(r * 0.20f, ey - r * 0.18f, r * 0.42f, ey + r * 0.12f, monsterPaint)
                monsterPaint.color = Color.WHITE
                canvas.drawCircle(-r * 0.34f, ey - r * 0.07f, r * 0.065f, monsterPaint)
                canvas.drawCircle(r * 0.28f, ey - r * 0.07f, r * 0.065f, monsterPaint)
                canvas.drawCircle(-r * 0.26f, ey + r * 0.04f, r * 0.03f, monsterPaint)
                canvas.drawCircle(r * 0.36f, ey + r * 0.04f, r * 0.03f, monsterPaint)
            }

            // 受伤贴纸：HP < MaxHP 时右颊多贴一块可爱创可贴
            if (monsterHp < monsterMaxHp) {
                canvas.save()
                canvas.translate(r * 0.48f, -r * 1.02f)
                canvas.rotate(-22f)
                monsterPaint.style = Paint.Style.FILL
                monsterPaint.color = 0xFFFFE4B5.toInt()
                canvas.drawRoundRect(-r * 0.18f, -r * 0.08f, r * 0.18f, r * 0.08f, 4f, 4f, monsterPaint)
                monsterPaint.color = 0xFFF3B773.toInt()
                canvas.drawRect(-r * 0.06f, -r * 0.06f, r * 0.06f, r * 0.06f, monsterPaint)
                canvas.restore()
            }

            // 搞怪小嘴：满血俏皮 w 嘴，受伤委屈波浪嘴
            monsterPaint.style = Paint.Style.STROKE
            monsterPaint.strokeWidth = 3.6f
            monsterPaint.strokeCap = Paint.Cap.ROUND
            monsterPaint.color = 0xFF0E3942.toInt()
            val mouth = Path()
            if (monsterHp < monsterMaxHp) {
                mouth.moveTo(-r * 0.22f, faceY + r * 0.22f)
                mouth.quadTo(0f, faceY + r * 0.08f, r * 0.22f, faceY + r * 0.22f)
            } else {
                mouth.moveTo(-r * 0.24f, faceY + r * 0.12f)
                mouth.quadTo(-r * 0.12f, faceY + r * 0.26f, 0f, faceY + r * 0.14f)
                mouth.quadTo(r * 0.12f, faceY + r * 0.26f, r * 0.24f, faceY + r * 0.12f)
            }
            canvas.drawPath(mouth, monsterPaint)
        }

        canvas.restore()

        // 战败或残血头顶绕圈晕眩金星
        if (isDefeated || (monsterHp == 1 && monsterMaxHp > 1 && !busy)) {
            paint.reset()
            paint.isAntiAlias = true
            paint.style = Paint.Style.FILL
            val orbitY = if (isDefeated) baseY - r * 1.05f else baseY - r * 1.98f
            for (s in 0 until 3) {
                val a = idleSec * 4.2f + s * (2f * PI.toFloat() / 3f)
                val sxPos = foeCx + cos(a) * (r * 0.65f)
                val syPos = orbitY + sin(a) * (r * 0.16f)
                paint.color = 0xFFF5B727.toInt()
                drawFourPointStar(canvas, sxPos, syPos, 7.5f, idleSec * 90f + s * 40f, paint)
            }
        }
    }

    private fun drawImpactBurst(canvas: Canvas, cx: Float, cy: Float, r: Float, t: Float) {
        paint.reset()
        paint.isAntiAlias = true
        val fade = (1f - t).coerceIn(0f, 1f)

        // 双层扩散冲击圆环
        paint.style = Paint.Style.STROKE
        paint.strokeWidth = 6f * fade + 1.5f
        paint.color = ((fade * 220).toInt().coerceIn(0, 220) shl 24) or 0x00FFD54F
        canvas.drawCircle(cx, cy, r * (0.35f + 1.25f * t), paint)

        if (t > 0.15f) {
            val t2 = ((t - 0.15f) / 0.85f).coerceIn(0f, 1f)
            val f2 = 1f - t2
            paint.strokeWidth = 4.5f * f2 + 1f
            paint.color = ((f2 * 190).toInt().coerceIn(0, 190) shl 24) or 0x004BE3D0
            canvas.drawCircle(cx, cy, r * (0.25f + 1.35f * t2), paint)
        }

        // 放射状漫画打击线
        paint.strokeCap = Paint.Cap.ROUND
        paint.strokeWidth = 4f * fade
        paint.color = ((fade * 240).toInt().coerceIn(0, 240) shl 24) or 0x00FFFFFF
        for (i in 0 until 8) {
            val a = i * (PI / 4.0) + 0.2
            val inner = r * (0.25f + 0.55f * t)
            val outer = r * (0.60f + 0.95f * t)
            canvas.drawLine(
                cx + (cos(a) * inner).toFloat(),
                cy + (sin(a) * inner).toFloat(),
                cx + (cos(a) * outer).toFloat(),
                cy + (sin(a) * outer).toFloat(),
                paint
            )
        }

        // 中心高光十字闪星
        if (t < 0.45f) {
            val sf = 1f - t / 0.45f
            paint.style = Paint.Style.FILL
            paint.color = ((sf * 255).toInt().coerceIn(0, 255) shl 24) or 0x00FFFDE7
            drawFourPointStar(canvas, cx, cy, r * (0.75f * sin(sf * PI * 0.5).toFloat()), t * 140f, paint)
        }
    }

    private fun drawParticles(canvas: Canvas, w: Float, h: Float, elapsed: Float) {
        paint.reset()
        paint.isAntiAlias = true
        paint.style = Paint.Style.FILL
        for (p in particles) {
            if (elapsed < p.bornMs || elapsed > p.bornMs + p.lifeMs) continue
            val t = (elapsed - p.bornMs) / p.lifeMs
            val dtSec = (elapsed - p.bornMs) / 1000f
            val px = w * p.originRelX + p.vx * dtSec
            val py = h * p.originRelY + p.vy * dtSec + 0.5f * p.gravity * dtSec * dtSec
            val alpha = ((1f - t * t) * 245).toInt().coerceIn(0, 245)
            paint.color = (alpha shl 24) or (p.color and 0x00FFFFFF)
            when (p.kind) {
                2 -> drawFourPointStar(canvas, px, py, p.size * (1f - t * 0.35f), t * 240f, paint)
                1 -> canvas.drawOval(px - p.size * 0.6f, py - p.size * 0.8f, px + p.size * 0.6f, py + p.size * 0.8f, paint)
                else -> canvas.drawCircle(px, py, p.size * (1f - t * 0.45f), paint)
            }
        }
    }

    private fun drawFloatingCombatText(canvas: Canvas, cx: Float, baseY: Float, elapsed: Float) {
        val t = ((elapsed - 540f) / 940f).coerceIn(0f, 1f)
        val alpha = ((if (t < 0.25f) t / 0.25f else 1f - (t - 0.25f) / 0.75f) * 255).toInt().coerceIn(0, 255)
        val popScale = if (t < 0.22f) {
            0.7f + 0.55f * sin((t / 0.22f) * PI * 0.5).toFloat()
        } else {
            1.25f - 0.22f * ((t - 0.22f) / 0.78f).coerceIn(0f, 1f)
        }
        val label = if (finishingBlow) "KO! 暴击!" else "元气暴击!"
        val yPos = baseY - t * 34f

        canvas.save()
        canvas.translate(cx, yPos)
        canvas.scale(popScale, popScale)

        paint.reset()
        paint.isAntiAlias = true
        paint.textAlign = Paint.Align.CENTER
        paint.textSize = 34f
        paint.isFakeBoldText = true

        // 描边
        paint.style = Paint.Style.STROKE
        paint.strokeWidth = 6.5f
        paint.strokeJoin = Paint.Join.ROUND
        paint.color = (alpha shl 24) or 0x003D2516
        canvas.drawText(label, 0f, 0f, paint)

        // 亮色填充
        paint.style = Paint.Style.FILL
        paint.color = (alpha shl 24) or (if (finishingBlow) 0x00FFD54F else 0x00FF7043)
        canvas.drawText(label, 0f, 0f, paint)

        canvas.restore()

        if (elapsed in 680f..1460f) {
            val st = (elapsed - 680f) / 780f
            val sAlpha = ((if (st < 0.35f) st / 0.35f else 1f - (st - 0.35f) / 0.65f) * 255).toInt().coerceIn(0, 255)
            paint.reset()
            paint.isAntiAlias = true
            paint.style = Paint.Style.FILL
            paint.textAlign = Paint.Align.CENTER
            paint.textSize = 28f
            paint.color = (sAlpha shl 24) or 0x00F6A821
            canvas.drawText("⭐", cx + 48f, baseY - st * 42f, paint)
            if (finishingBlow) {
                canvas.drawText("✨", cx - 50f, baseY + 10f - st * 48f, paint)
            }
        }
    }

    private fun drawFourPointStar(canvas: Canvas, cx: Float, cy: Float, radius: Float, deg: Float, p: Paint) {
        canvas.save()
        canvas.translate(cx, cy)
        canvas.rotate(deg)
        val inner = radius * 0.34f
        val path = Path().apply {
            moveTo(0f, -radius)
            lineTo(inner, -inner)
            lineTo(radius, 0f)
            lineTo(inner, inner)
            lineTo(0f, radius)
            lineTo(-inner, inner)
            lineTo(-radius, 0f)
            lineTo(-inner, -inner)
            close()
        }
        canvas.drawPath(path, p)
        canvas.restore()
    }
}
