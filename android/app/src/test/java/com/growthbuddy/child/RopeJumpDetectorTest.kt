package com.growthbuddy.child

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import kotlin.math.PI
import kotlin.math.sin

class RopeJumpDetectorTest {

    private fun makePose(
        shoulderY: Float,
        hipY: Float = shoulderY + 140f,
        noseY: Float = shoulderY - 55f,
        leftShoulderConf: Float = 0.95f,
        rightShoulderConf: Float = 0.95f,
        leftHipConf: Float = 0.95f,
        rightHipConf: Float = 0.95f
    ): RopeJumpDetector.PosePoints {
        return RopeJumpDetector.PosePoints(
            nose = RopeJumpDetector.Point2D(240f, noseY, 0.95f),
            leftShoulder = RopeJumpDetector.Point2D(195f, shoulderY, leftShoulderConf),
            rightShoulder = RopeJumpDetector.Point2D(285f, shoulderY, rightShoulderConf),
            leftHip = RopeJumpDetector.Point2D(205f, hipY, leftHipConf),
            rightHip = RopeJumpDetector.Point2D(275f, hipY, rightHipConf)
        )
    }

    @Test
    fun countsNormalAndFastContinuousSkippingWithoutMissing() {
        val detector = RopeJumpDetector()
        val w = 480
        val h = 640
        var t = 1000L

        // 1. 静止预热 15 帧
        repeat(15) {
            detector.onPoseFrame(makePose(220f), w, h, t, countingActive = false)
            t += 33L
        }

        // 2. 模拟高频连跳：180 次/分（周期 333ms，每周期约 10 帧，30fps），
        //    并且模拟孩子连跳时最低点比静止站立略高 4px（不完全回落基线）
        val targetJumps = 25
        val framesPerJump = 10
        val amplitudePx = 16f // ~11% 躯干高度
        for (j in 0 until targetJumps) {
            for (f in 0 until framesPerJump) {
                val phase = f.toDouble() / framesPerJump
                val lift = (sin(phase * PI) * amplitudePx).toFloat()
                val shoulderY = 216f - lift
                detector.onPoseFrame(makePose(shoulderY), w, h, t, countingActive = true)
                t += 33L
            }
        }
        // 结尾落地几帧让最后一跳回落确认
        repeat(5) {
            detector.onPoseFrame(makePose(216f), w, h, t, countingActive = true)
            t += 33L
        }

        assertEquals("高频 180次/分 连跳不应漏计", targetJumps, detector.count)
    }

    @Test
    fun countsLowAmplitudeAndBriefOcclusionJumps() {
        val detector = RopeJumpDetector()
        val w = 480
        val h = 640
        var t = 2000L

        repeat(10) {
            detector.onPoseFrame(makePose(220f), w, h, t, countingActive = false)
            t += 33L
        }

        // 模拟小幅跳（仅 9.5px ≈ 6.8% 躯干高度）且波峰附近单肩+单髋被绳子挡住 2 帧
        val targetJumps = 12
        val framesPerJump = 12
        for (j in 0 until targetJumps) {
            for (f in 0 until framesPerJump) {
                val phase = f.toDouble() / framesPerJump
                val lift = (sin(phase * PI) * 9.5).toFloat()
                val occluded = (f == 5 || f == 6)
                val pose = makePose(
                    shoulderY = 220f - lift,
                    leftShoulderConf = if (occluded) 0.10f else 0.95f,
                    rightHipConf = if (occluded) 0.10f else 0.95f
                )
                detector.onPoseFrame(pose, w, h, t, countingActive = true)
                t += 33L
            }
        }
        repeat(5) {
            detector.onPoseFrame(makePose(220f), w, h, t, countingActive = true)
            t += 33L
        }

        assertEquals("小幅跳且存在单侧短暂遮挡时不应漏计", targetJumps, detector.count)
    }

    @Test
    fun ignoresStandingJitterAndHeadNodding() {
        val detector = RopeJumpDetector()
        val w = 480
        val h = 640
        var t = 3000L

        repeat(10) {
            detector.onPoseFrame(makePose(220f), w, h, t, countingActive = false)
            t += 33L
        }

        // 模拟孩子站在原地微小抖动(±2px)且大幅度点头(nose ±18px)，肩髋不动
        for (i in 0 until 120) {
            val jitter = (sin(i * 0.7) * 2.0).toFloat()
            val headNod = (sin(i * 0.5) * 18.0).toFloat()
            val pose = makePose(
                shoulderY = 220f + jitter,
                hipY = 360f + jitter,
                noseY = 165f + headNod
            )
            detector.onPoseFrame(pose, w, h, t, countingActive = true)
            t += 33L
        }

        assertTrue("静止抖动与原地光点头不应误计，实际=${detector.count}", detector.count == 0)
    }
}
