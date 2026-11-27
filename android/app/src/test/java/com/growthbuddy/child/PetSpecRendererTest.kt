package com.growthbuddy.child

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PetSpecRendererTest {

    private fun countShapes(list: List<PetSpecStore.ShapeSpec>): Int {
        var total = 0
        for (s in list) {
            total += if (s is PetSpecStore.ShapeSpec.Group) 1 + countShapes(s.children) else 1
        }
        return total
    }

    private fun assertAllPathsValid(list: List<PetSpecStore.ShapeSpec>, tag: String) {
        for (s in list) {
            when (s) {
                is PetSpecStore.ShapeSpec.Group -> assertAllPathsValid(s.children, "$tag.group")
                is PetSpecStore.ShapeSpec.VectorPath -> {
                    assertTrue("$tag path '${s.rawD}' produced empty cmds", s.cmds.isNotEmpty())
                    assertTrue("$tag path '${s.rawD}' bounds width < 0", s.bounds.width() >= 0f)
                }
                else -> {}
            }
        }
    }

    @Test
    fun specLoadsAllMainStagesAndTenBuiltinSpeciesOptions() {
        val doc = PetSpecStore.get()
        assertEquals(220f, doc.viewBox.w, 0.01f)
        assertEquals(200f, doc.viewBox.h, 0.01f)
        for (lv in 0..5) {
            val st = doc.mainPetStages[lv]
            assertNotNull("mainPetStages[$lv] missing", st)
            assertTrue("mainPetStages[$lv] empty", countShapes(st!!.shapes) > 0)
            assertAllPathsValid(st.shapes, "mainPet[$lv]")
        }
        val tpl = doc.speciesTemplate
        assertAllPathsValid(tpl.stage0Shapes, "species.stage0")
        assertAllPathsValid(tpl.body, "species.body")
        assertAllPathsValid(tpl.ahoge, "species.ahoge")
        for ((k, v) in tpl.ears) assertAllPathsValid(v, "ears.$k")
        for ((k, v) in tpl.tails) assertAllPathsValid(v, "tails.$k")
        for ((k, v) in tpl.accessories) assertAllPathsValid(v, "acc.$k")
        for ((k, v) in tpl.mouths) assertAllPathsValid(v, "mouth.$k")
        for ((k, v) in tpl.stage1.ears) assertAllPathsValid(v, "stage1.ears.$k")
        for ((k, v) in tpl.stage1.mouths) assertAllPathsValid(v, "stage1.mouth.$k")
        assertTrue("floppy ear spec missing", tpl.ears.containsKey("floppy"))
        assertTrue("stage1 floppy ear missing", tpl.stage1.ears.containsKey("floppy"))
    }

    @Test
    fun animationCurvesMatchWebSpecTiming() {
        PetSpecStore.get()
        // Breathe: 3000ms cosine easing
        assertEquals(0f, PetArtCanvas.computeBreathProgress(0L), 0.001f)
        assertEquals(1f, PetArtCanvas.computeBreathProgress(1500L), 0.001f)
        assertEquals(0f, PetArtCanvas.computeBreathProgress(3000L), 0.001f)

        // Blink: 4600ms cycle
        assertEquals(1f, PetArtCanvas.computeBlinkScaleY(0L), 0.001f)
        assertEquals(1f, PetArtCanvas.computeBlinkScaleY(4100L), 0.001f)
        val closed = PetArtCanvas.computeBlinkScaleY((4600f * 0.95f).toLong())
        assertTrue("expected eye nearly closed near 95%, got $closed", closed < 0.15f)
        assertEquals(1f, PetArtCanvas.computeBlinkScaleY(4600L), 0.001f)

        // Egg wobble: 3400ms keyframe wobble
        assertEquals(0f, PetArtCanvas.computeEggWobbleDeg(0L), 0.001f)
        val peakNeg = PetArtCanvas.computeEggWobbleDeg((3400f * 0.78f).toLong())
        assertTrue("expected negative wobble peak near 78%, got $peakNeg", peakNeg < -3.0f)
        val peakPos = PetArtCanvas.computeEggWobbleDeg((3400f * 0.84f).toLong())
        assertTrue("expected positive wobble peak near 84%, got $peakPos", peakPos > 3.0f)
    }

    @Test
    fun svgPathParserHandlesNegativeRelativeAndDecimalStarts() {
        val cmds = SvgPathParser.parse("M150 78 l2.5 5 5.4 .7 -3.9 3.6 .9 5.3 z")
        assertFalse(cmds.isEmpty())
        assertEquals(PetSpecStore.PathCmd.Move(150f, 78f), cmds[0])
        assertEquals(PetSpecStore.PathCmd.Line(152.5f, 83f), cmds[1])
        assertEquals(PetSpecStore.PathCmd.Line(157.9f, 83.7f), cmds[2])
        assertEquals(PetSpecStore.PathCmd.Close, cmds.last())
    }
}
