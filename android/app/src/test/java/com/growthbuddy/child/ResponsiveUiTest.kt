package com.growthbuddy.child

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ResponsiveUiTest {

    @Test
    fun phonePortraitStaysSingleColumnTwoCols() {
        val m = ResponsiveUi.computeMetrics(widthDp = 412, heightDp = 915, smallestWidthDp = 412)
        assertFalse(m.isTablet)
        assertFalse(m.isLandscape)
        assertFalse(m.isDesktopSplit)
        assertEquals(2, ResponsiveUi.columnsForWidth(m.widthDp))
    }

    @Test
    fun phoneLandscapeDoesNotSplitDesktopTwoPane() {
        val m = ResponsiveUi.computeMetrics(widthDp = 850, heightDp = 390, smallestWidthDp = 390)
        assertFalse(m.isTablet)
        assertTrue(m.isLandscape)
        assertFalse(m.isDesktopSplit)
    }

    @Test
    fun tabletPortraitUsesThreeColumnsWithoutDesktopSplit() {
        val m = ResponsiveUi.computeMetrics(widthDp = 800, heightDp = 1280, smallestWidthDp = 800)
        assertTrue(m.isTablet)
        assertFalse(m.isLandscape)
        assertFalse(m.isDesktopSplit)
        assertEquals(3, ResponsiveUi.columnsForWidth(m.widthDp))
    }

    @Test
    fun tabletLandscapeActivatesDesktopSplitAndFourColumns() {
        val m = ResponsiveUi.computeMetrics(widthDp = 1280, heightDp = 800, smallestWidthDp = 800)
        assertTrue(m.isTablet)
        assertTrue(m.isLandscape)
        assertTrue(m.isDesktopSplit)
        assertEquals(4, ResponsiveUi.columnsForWidth(m.widthDp))
    }
}
