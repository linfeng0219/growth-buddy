package com.growthbuddy.child

import org.junit.Assert.assertEquals
import org.junit.Test

class PetNameTest {

    private fun sampleChild(
        name: String = "柚子",
        petName: String = "",
        activeCompanion: String? = null
    ) = ChildInfo(
        id = "c1",
        name = name,
        avatarIdx = 0,
        avatarUrl = "",
        petName = petName,
        xp = 0,
        level = 0,
        coupons = 0,
        activeCompanion = activeCompanion,
        muted = false
    )

    private fun sampleSpecies(id: String, name: String) = SpeciesItem(
        id = id,
        name = name,
        color = "#ec9844",
        ear = "round",
        acc = "none",
        tail = "puff",
        mouth = "smile",
        base = "grass",
        hint = ""
    )

    @Test
    fun defaultNameUsesChildAndFirstCatalogPet() {
        val catalog = listOf(sampleSpecies("b1", "小团子"), sampleSpecies("b2", "雪球"))
        val child = sampleChild(name = "柚子", petName = "", activeCompanion = null)
        assertEquals("柚子的小团子", GrowthMeta.resolvePetDisplayName(child, catalog))
    }

    @Test
    fun defaultNameFollowsActiveCompanion() {
        val catalog = listOf(sampleSpecies("b1", "小团子"), sampleSpecies("b2", "雪球"))
        val child = sampleChild(name = "柚子", petName = "", activeCompanion = "b2")
        assertEquals("柚子的雪球", GrowthMeta.resolvePetDisplayName(child, catalog))
    }

    @Test
    fun customPetNameOverridesDefault() {
        val catalog = listOf(sampleSpecies("b1", "小团子"))
        val child = sampleChild(name = "柚子", petName = "糯米", activeCompanion = "b1")
        assertEquals("糯米", GrowthMeta.resolvePetDisplayName(child, catalog))
    }

    @Test
    fun blankChildNameFallsBackToSpeciesNameOnly() {
        val catalog = listOf(sampleSpecies("b1", "小团子"))
        val child = sampleChild(name = " ", petName = "", activeCompanion = null)
        assertEquals("小团子", GrowthMeta.resolvePetDisplayName(child, catalog))
    }
}
