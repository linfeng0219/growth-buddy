package com.growthbuddy.child

import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min

/**
 * 参考「天天跳绳」视觉计数原理的防漏计跳绳检测器：
 * 1. 多部位加权融合（双肩 60% + 双髋 35% + 头部兜底），单点被绳子或摆臂短暂遮挡不丢帧；
 * 2. 躯干主导信号天然抑制“原地光点头/光晃手”造成的误计；
 * 3. 轻量保峰滤波（alpha=0.76）避免高频连跳（150~220个/分）时波峰被削平导致漏计；
 * 4. 动态波峰-波谷反转状态机（Peak-Valley Reversal），不依赖脆弱的静态站立基线，
 *    连续快跳未完全回落到静止高度也能逐个精准计数，并在越过顶点回落瞬间立刻出数。
 */
class RopeJumpDetector(
    private val riseThresholdRatio: Float = 0.038f,
    private val minJumpAmpRatio: Float = 0.048f,
    private val fallConfirmMinRatio: Float = 0.020f,
    private val fallConfirmFraction: Float = 0.30f,
    private val minIntervalMs: Long = 145L
) {
    data class Point2D(val x: Float, val y: Float, val likelihood: Float)

    data class PosePoints(
        val nose: Point2D? = null,
        val leftShoulder: Point2D? = null,
        val rightShoulder: Point2D? = null,
        val leftHip: Point2D? = null,
        val rightHip: Point2D? = null,
        val leftAnkle: Point2D? = null,
        val rightAnkle: Point2D? = null
    )

    data class FrameOutput(
        val framedOk: Boolean,
        val hint: String,
        val count: Int,
        val incremented: Boolean,
        val speedPerMin: Int = 0
    )

    private enum class MotionState { SEEK_TAKEOFF, AIRBORNE }

    var count: Int = 0
        private set

    private var state: MotionState = MotionState.SEEK_TAKEOFF
    private var smoothedY: Float? = null
    private var smoothedTorsoHeight: Float? = null
    private var valleyY: Float? = null
    private var valleyUpdatedMs: Long = 0L
    private var takeoffValleyY: Float = 0f
    private var peakY: Float = 0f
    private var airborneSinceMs: Long = 0L
    private var lastJumpTimeMs: Long = 0L
    private var occludedFrames: Int = 0

    // 用于单点短暂遮挡时的连续偏移补偿，避免切换关键点时产生阶跃跳变
    private var lastShoulderHalfDy: Float = 0f
    private var lastHipHalfDy: Float = 0f
    private var lastHipToShoulderOffset: Float = 0f
    private var lastNoseToShoulderOffset: Float = 0f

    // 最近 8 次跳跃时间戳，用于计算实时跳速（个/分）
    private val recentJumpTimes = ArrayDeque<Long>()

    fun resetCount() {
        count = 0
        state = MotionState.SEEK_TAKEOFF
        smoothedY = null
        valleyY = null
        valleyUpdatedMs = 0L
        takeoffValleyY = 0f
        peakY = 0f
        airborneSinceMs = 0L
        lastJumpTimeMs = 0L
        occludedFrames = 0
        recentJumpTimes.clear()
    }

    fun evaluateFraming(
        points: PosePoints,
        imageWidth: Int,
        imageHeight: Int
    ): Pair<Boolean, String> {
        val ls = points.leftShoulder
        val rs = points.rightShoulder
        val lh = points.leftHip
        val rh = points.rightHip

        if (ls == null || rs == null || lh == null || rh == null) {
            return false to "未检测到完整上半身，请面向摄像头退后站立"
        }
        val minConf = minOf(ls.likelihood, rs.likelihood, lh.likelihood, rh.likelihood)
        if (minConf < 0.42f) {
            return false to "光线不足或身体被遮挡，请正对镜头进入绿框"
        }

        val shoulderY = (ls.y + rs.y) * 0.5f
        val hipY = (lh.y + rh.y) * 0.5f
        val torsoHeight = hipY - shoulderY
        val torsoRatio = torsoHeight / imageHeight.coerceAtLeast(1)

        if (torsoRatio < 0.11f) {
            return false to "离镜头太远了，请往前走半步"
        }
        if (torsoRatio > 0.68f) {
            return false to "离镜头太近了，请退后 1.5~2 米"
        }

        val centerX = ((ls.x + rs.x) * 0.5f) / imageWidth.coerceAtLeast(1)
        if (centerX < 0.14f || centerX > 0.86f) {
            return false to "请站到画面中央绿框内"
        }

        return true to "站位完美！准备开始跳绳"
    }

    /** 兼容旧版四点签名 */
    fun onPoseFrame(
        leftShoulder: Point2D?,
        rightShoulder: Point2D?,
        leftHip: Point2D?,
        rightHip: Point2D?,
        imageWidth: Int,
        imageHeight: Int,
        timestampMs: Long,
        countingActive: Boolean
    ): FrameOutput = onPoseFrame(
        points = PosePoints(
            leftShoulder = leftShoulder,
            rightShoulder = rightShoulder,
            leftHip = leftHip,
            rightHip = rightHip
        ),
        imageWidth = imageWidth,
        imageHeight = imageHeight,
        timestampMs = timestampMs,
        countingActive = countingActive
    )

    fun onPoseFrame(
        points: PosePoints,
        imageWidth: Int,
        imageHeight: Int,
        timestampMs: Long,
        countingActive: Boolean
    ): FrameOutput {
        val (framedOk, frameHint) = evaluateFraming(points, imageWidth, imageHeight)

        // 准备/对框阶段：仅做严格站位评估与躯干基线预热
        if (!countingActive) {
            updateTorsoAndFusedY(points, imageHeight)?.let { (fusedY, _) ->
                smoothedY = fusedY
                valleyY = fusedY
                valleyUpdatedMs = timestampMs
            }
            state = MotionState.SEEK_TAKEOFF
            return FrameOutput(framedOk, frameHint, count, false, 0)
        }

        // 正式计数阶段：绝不因单点短暂遮挡或轻微偏离中心就整帧丢弃！
        val extracted = updateTorsoAndFusedY(points, imageHeight)
        if (extracted == null) {
            occludedFrames += 1
            val hint = if (occludedFrames > 8) "请回到画面中，保持全身或上半身可见" else frameHint
            return FrameOutput(false, hint, count, false, computeSpeedPerMin(timestampMs))
        }
        occludedFrames = 0

        val (rawY, torsoHeight) = extracted

        // 轻量保峰滤波（0.76 新帧 + 0.24 历史），极小相位延迟且不削平快跳波峰
        val prevSmooth = smoothedY
        val smoothY = when {
            prevSmooth == null -> rawY
            abs(rawY - prevSmooth) > torsoHeight * 0.58f -> prevSmooth // 过滤极端骨骼跳点
            else -> prevSmooth * 0.24f + rawY * 0.76f
        }
        smoothedY = smoothY

        val currentValley = valleyY
        if (currentValley == null) {
            valleyY = smoothY
            valleyUpdatedMs = timestampMs
            return FrameOutput(framedOk, frameHint, count, false, computeSpeedPerMin(timestampMs))
        }

        val riseThresh = torsoHeight * riseThresholdRatio
        val minAmp = torsoHeight * minJumpAmpRatio
        val fallMin = torsoHeight * fallConfirmMinRatio
        var incremented = false

        when (state) {
            MotionState.SEEK_TAKEOFF -> {
                if (smoothY >= currentValley) {
                    // 继续下落或站稳，刷新局部最低点（Y 最大）
                    valleyY = smoothY
                    valleyUpdatedMs = timestampMs
                } else {
                    val rise = currentValley - smoothY
                    if (rise >= riseThresh) {
                        // 检测到明显向上起跳
                        state = MotionState.AIRBORNE
                        takeoffValleyY = currentValley
                        peakY = smoothY
                        airborneSinceMs = timestampMs
                    } else if (timestampMs - valleyUpdatedMs > 420L) {
                        // 长时间静止且有轻微上移漂移时缓慢回收谷底，避免陈旧下蹲拉大阈值
                        valleyY = currentValley * 0.90f + smoothY * 0.10f
                    }
                }
            }

            MotionState.AIRBORNE -> {
                if (smoothY < peakY) {
                    // 仍在上升，更新最高点（Y 最小）
                    peakY = smoothY
                } else {
                    val amp = takeoffValleyY - peakY
                    val drop = smoothY - peakY
                    val fallConfirm = max(fallMin, amp * fallConfirmFraction)
                    if (drop >= fallConfirm) {
                        // 越过波峰明确回落 -> 立即结算本次跳跃
                        if (amp >= minAmp && (lastJumpTimeMs == 0L || timestampMs - lastJumpTimeMs >= minIntervalMs)) {
                            count += 1
                            lastJumpTimeMs = timestampMs
                            incremented = true
                            recordJumpTime(timestampMs)
                        }
                        // 以当前回落位置作为新一轮谷底起点，无缝承接下一次高频连跳
                        state = MotionState.SEEK_TAKEOFF
                        valleyY = smoothY
                        valleyUpdatedMs = timestampMs
                    } else if (timestampMs - airborneSinceMs > 950L) {
                        // 超时保护（如镜头被抬高或孩子站上台阶）
                        state = MotionState.SEEK_TAKEOFF
                        valleyY = smoothY
                        valleyUpdatedMs = timestampMs
                    }
                }
            }
        }

        return FrameOutput(
            framedOk = true,
            hint = if (framedOk) frameHint else "保持在画面内，计数持续进行中",
            count = count,
            incremented = incremented,
            speedPerMin = computeSpeedPerMin(timestampMs)
        )
    }

    private fun updateTorsoAndFusedY(points: PosePoints, imageHeight: Int): Pair<Float, Float>? {
        val minTrackConf = 0.28f
        val ls = points.leftShoulder?.takeIf { it.likelihood >= minTrackConf }
        val rs = points.rightShoulder?.takeIf { it.likelihood >= minTrackConf }
        val lh = points.leftHip?.takeIf { it.likelihood >= minTrackConf }
        val rh = points.rightHip?.takeIf { it.likelihood >= minTrackConf }
        val nose = points.nose?.takeIf { it.likelihood >= minTrackConf }

        val shoulderY: Float? = when {
            ls != null && rs != null -> {
                lastShoulderHalfDy = (rs.y - ls.y) * 0.5f
                (ls.y + rs.y) * 0.5f
            }
            ls != null -> ls.y + lastShoulderHalfDy
            rs != null -> rs.y - lastShoulderHalfDy
            else -> null
        }

        val hipY: Float? = when {
            lh != null && rh != null -> {
                lastHipHalfDy = (rh.y - lh.y) * 0.5f
                (lh.y + rh.y) * 0.5f
            }
            lh != null -> lh.y + lastHipHalfDy
            rh != null -> rh.y - lastHipHalfDy
            else -> null
        }

        // 更新平滑躯干高度（短暂遮挡时沿用上次值）
        if (shoulderY != null && hipY != null && hipY > shoulderY + imageHeight * 0.08f) {
            val measuredTorso = hipY - shoulderY
            smoothedTorsoHeight = smoothedTorsoHeight?.let { it * 0.85f + measuredTorso * 0.15f } ?: measuredTorso
            lastHipToShoulderOffset = lastHipToShoulderOffset.let {
                if (it == 0f) measuredTorso else it * 0.85f + measuredTorso * 0.15f
            }
        }
        if (shoulderY != null && nose != null) {
            val offset = shoulderY - nose.y
            lastNoseToShoulderOffset = if (lastNoseToShoulderOffset == 0f) offset
            else lastNoseToShoulderOffset * 0.85f + offset * 0.15f
        }

        val torsoHeight = (smoothedTorsoHeight ?: (imageHeight * 0.26f))
            .coerceIn(imageHeight * 0.10f, imageHeight * 0.72f)

        // 将所有可用关键点对齐到“肩部中心等价高度”进行加权融合：
        // - 优先使用肩 + 髋（天然抵抗原地光点头误判）
        // - 仅当肩和髋同时瞬间丢失时，才使用头部 nose + 偏移量无缝兜底
        val fusedY: Float = when {
            shoulderY != null && hipY != null -> {
                val hipAligned = hipY - lastHipToShoulderOffset
                shoulderY * 0.62f + hipAligned * 0.38f
            }
            shoulderY != null -> shoulderY
            hipY != null && lastHipToShoulderOffset > 0f -> hipY - lastHipToShoulderOffset
            nose != null && lastNoseToShoulderOffset != 0f -> nose.y + lastNoseToShoulderOffset
            else -> return null
        }

        return fusedY to torsoHeight
    }

    private fun recordJumpTime(timestampMs: Long) {
        recentJumpTimes.addLast(timestampMs)
        while (recentJumpTimes.size > 8) recentJumpTimes.removeFirst()
    }

    private fun computeSpeedPerMin(nowMs: Long): Int {
        if (recentJumpTimes.size < 2) return 0
        if (nowMs - recentJumpTimes.last() > 1600L) return 0
        val spanMs = (recentJumpTimes.last() - recentJumpTimes.first()).coerceAtLeast(1L)
        val intervals = recentJumpTimes.size - 1
        return min(300, ((intervals * 60_000L) / spanMs).toInt())
    }
}
