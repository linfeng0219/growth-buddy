package com.growthbuddy.child

import org.junit.Assert.assertEquals
import org.junit.Test

class PetNameTest {

    private fun sampleChild(
        name: String = "柚子",
        petName: String = "",
        activeCompanion: String? = null,
        activePetId: String = ""
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
        activePetId = activePetId,
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
    fun independentActivePetNameOverridesChildFallback() {
        val catalog = listOf(sampleSpecies("b1", "小团子"), sampleSpecies("b2", "雪球"))
        val child = sampleChild(name = "柚子", petName = "旧名字", activeCompanion = "b2")
        val pets = listOf(
            ChildPet("pet_1", "c1", "b1", "柚子的小团子", isActive = false, adoptedDate = "2026-11-26"),
            ChildPet("pet_2", "c1", "b2", "白雪", isActive = true, adoptedDate = "2026-11-26"),
            ChildPet("pet_3", "c1", "b3", "柚子的桔子", isActive = false, adoptedDate = "2026-11-26")
        )
        val activePet = GrowthMeta.findActivePet(pets, child.activePetId)
        assertEquals("pet_2", activePet?.id)
        assertEquals("白雪", GrowthMeta.resolvePetDisplayName(child, catalog, activePet))
    }

    @Test
    fun findActivePetFallsBackToActivePetIdWhenFlagUnset() {
        val pets = listOf(
            ChildPet("pet_1", "c1", "b1", "团子一号", isActive = false, adoptedDate = "2026-11-26"),
            ChildPet("pet_2", "c1", "b2", "团子二号", isActive = false, adoptedDate = "2026-11-26")
        )
        val activePet = GrowthMeta.findActivePet(pets, activePetId = "pet_2")
        assertEquals("团子二号", activePet?.petName)
    }

    @Test
    fun blankChildNameFallsBackToSpeciesNameOnly() {
        val catalog = listOf(sampleSpecies("b1", "小团子"))
        val child = sampleChild(name = " ", petName = "", activeCompanion = null)
        assertEquals("小团子", GrowthMeta.resolvePetDisplayName(child, catalog))
    }

    @Test
    fun waitingStarterPetPromptsForChoice() {
        val catalog = listOf(sampleSpecies("b1", "小团子"), sampleSpecies("b2", "雪球"))
        val child = sampleChild(name = "柚子", petName = "", activeCompanion = null).copy(needsStarterPet = true)
        assertEquals("🥚 待领养初始宠物蛋", GrowthMeta.resolvePetDisplayName(child, catalog, null))
    }
}
