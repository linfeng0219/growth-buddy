package com.growthbuddy.child

import android.app.Activity
import android.content.Context
import android.content.res.Configuration
import android.util.DisplayMetrics
import android.view.Gravity
import android.view.View
import android.view.ViewGroup
import android.widget.LinearLayout

/**
 * 手机 / 平板（竖屏 & 横屏桌面支架）统一自适应布局辅助工具。
 * 结合 AndroidManifest 中的 configChanges，在不中断倒计时/战斗/AI 跳绳的前提下平滑切换分栏与居中宽度。
 */
object ResponsiveUi {

    data class Metrics(
        val widthDp: Int,
        val heightDp: Int,
        val smallestWidthDp: Int,
        val isLandscape: Boolean,
        val isTablet: Boolean,
        val isDesktopSplit: Boolean
    )

    fun dp(ctx: Context, v: Int): Int {
        return (v * ctx.resources.displayMetrics.density).toInt()
    }

    fun computeMetrics(widthDp: Int, heightDp: Int, smallestWidthDp: Int): Metrics {
        val isLand = widthDp > heightDp
        val isTab = smallestWidthDp >= 580 || (widthDp >= 720 && heightDp >= 480)
        val isSplit = isTab && isLand && widthDp >= 760
        return Metrics(
            widthDp = widthDp,
            heightDp = heightDp,
            smallestWidthDp = smallestWidthDp,
            isLandscape = isLand,
            isTablet = isTab,
            isDesktopSplit = isSplit
        )
    }

    fun columnsForWidth(
        widthDp: Int,
        phoneCols: Int = 2,
        tabletPortCols: Int = 3,
        tabletLandCols: Int = 4
    ): Int = when {
        widthDp >= 840 -> tabletLandCols
        widthDp >= 560 -> tabletPortCols
        else -> phoneCols
    }

    fun metrics(activity: Activity): Metrics {
        val cfg = activity.resources.configuration
        val dm: DisplayMetrics = activity.resources.displayMetrics
        val wDp = if (cfg.screenWidthDp != Configuration.SCREEN_WIDTH_DP_UNDEFINED) {
            cfg.screenWidthDp
        } else {
            (dm.widthPixels / dm.density).toInt()
        }
        val hDp = if (cfg.screenHeightDp != Configuration.SCREEN_HEIGHT_DP_UNDEFINED) {
            cfg.screenHeightDp
        } else {
            (dm.heightPixels / dm.density).toInt()
        }
        val swDp = if (cfg.smallestScreenWidthDp != Configuration.SMALLEST_SCREEN_WIDTH_DP_UNDEFINED) {
            cfg.smallestScreenWidthDp
        } else {
            minOf(wDp, hDp)
        }
        return computeMetrics(wDp, hDp, swDp)
    }

    /**
     * 让平板大屏下的内容区保持舒适阅读/操作宽度并水平居中，避免表单或卡片被拉得过宽。
     * 注意：目标 View 的直接父容器需为 LinearLayout 或 FrameLayout。
     */
    fun applyCenteredMaxWidth(
        activity: Activity,
        target: View,
        maxWidthDp: Int,
        horizontalReserveDp: Int = 28
    ) {
        val m = metrics(activity)
        val lp = target.layoutParams ?: return
        val availDp = (m.widthDp - horizontalReserveDp).coerceAtLeast(240)
        val desiredWidth = if (availDp > maxWidthDp) {
            dp(activity, maxWidthDp)
        } else {
            ViewGroup.LayoutParams.MATCH_PARENT
        }
        var changed = lp.width != desiredWidth
        lp.width = desiredWidth
        if (lp is LinearLayout.LayoutParams && lp.gravity != Gravity.CENTER_HORIZONTAL) {
            lp.gravity = Gravity.CENTER_HORIZONTAL
            changed = true
        }
        if (changed) {
            target.layoutParams = lp
        }
    }

    fun gridColumns(
        activity: Activity,
        phoneCols: Int = 2,
        tabletPortCols: Int = 3,
        tabletLandCols: Int = 4
    ): Int {
        val m = metrics(activity)
        return columnsForWidth(m.widthDp, phoneCols, tabletPortCols, tabletLandCols)
    }
}
