package com.growthbuddy.child

import org.json.JSONArray
import org.json.JSONObject

data class LevelSpec(val xp: Int, val name: String, val talk: String)

data class AvatarPalette(val shirt: Int, val hair: Int, val skin: Int, val style: String)

data class FamilyInfo(val id: String, val name: String, val familyCode: String)

data class ChildInfo(
    val id: String,
    val name: String,
    val avatarIdx: Int,
    val avatarUrl: String,
    val petName: String,
    val xp: Int,
    val level: Int,
    val coupons: Int,
    val activeCompanion: String?,
    val muted: Boolean
)

data class TaskItem(
    val id: String,
    val emoji: String,
    val label: String,
    val pts: Int,
    val scope: String,
    val taskType: String,
    val durationSec: Int,
    val targetCount: Int
)

data class SpeciesItem(
    val id: String,
    val name: String,
    val color: String,
    val ear: String,
    val acc: String,
    val tail: String,
    val mouth: String,
    val base: String,
    val hint: String
)

data class CheckinLog(
    val id: String,
    val taskId: String,
    val taskLabel: String,
    val pts: Int,
    val actualCount: Int?,
    val actualDuration: Int?,
    val hasPhoto: Boolean,
    val photoUrl: String,
    val checkinDate: String
)

data class PolaroidItem(
    val id: String,
    val avatarIdx: Int,
    val avatarUrl: String,
    val petRef: String,
    val petLv: Int,
    val photoDate: String,
    val note: String
)

data class ChildState(
    val today: String,
    val family: FamilyInfo,
    val child: ChildInfo,
    val childCount: Int,
    val tasks: List<TaskItem>,
    val todayDoneIds: Set<String>,
    val logs: List<CheckinLog>,
    val catalog: List<SpeciesItem>,
    val owned: Map<String, String>,
    val polaroids: List<PolaroidItem>
)

object GrowthMeta {
    val LEVELS = listOf(
        LevelSpec(0, "一颗蛋", "抱抱我，完成一件小事我就会慢慢破壳哦。"),
        LevelSpec(10, "破壳啦", "哇，外面的世界好暖和！快带我去看看吧。"),
        LevelSpec(30, "幼年体", "我已经长出小耳朵啦！今天你完成了什么呀？"),
        LevelSpec(60, "少年体", "听说图鉴里有好多好朋友，带一位出来陪我玩吧！"),
        LevelSpec(120, "成年体", "你好呀老朋友！神秘蛋也解锁啦，一起去看看惊喜吧。"),
        LevelSpec(200, "守护兽", "守护兽达成！我们一起记录每一天的成长闪光吧！")
    )

    val AVATARS = listOf(
        AvatarPalette(0xFF5B8DEF.toInt(), 0xFF4A3227.toInt(), 0xFFF2C9A0.toInt(), "short"),
        AvatarPalette(0xFFE86A6A.toInt(), 0xFF2E2A26.toInt(), 0xFFF0C39A.toInt(), "bun"),
        AvatarPalette(0xFF58B368.toInt(), 0xFF7A5230.toInt(), 0xFFF4CDA6.toInt(), "short"),
        AvatarPalette(0xFF9A6FCA.toInt(), 0xFF242B4D.toInt(), 0xFFF2C9A0.toInt(), "bob"),
        AvatarPalette(0xFFE8913A.toInt(), 0xFF5A3A1E.toInt(), 0xFFF7D3AD.toInt(), "short"),
        AvatarPalette(0xFF4A9F9F.toInt(), 0xFFC9A344.toInt(), 0xFFF2C9A0.toInt(), "bob"),
        AvatarPalette(0xFFD9668F.toInt(), 0xFF3B2A20.toInt(), 0xFFF4CDA6.toInt(), "long"),
        AvatarPalette(0xFF3F6EA8.toInt(), 0xFF2B2420.toInt(), 0xFFEEC29A.toInt(), "glasses"),
        AvatarPalette(0xFF6C8F4E.toInt(), 0xFF1F1B18.toInt(), 0xFFE9B98F.toInt(), "beard"),
        AvatarPalette(0xFFC58B3D.toInt(), 0xFF9A948C.toInt(), 0xFFF2C9A0.toInt(), "bun"),
        AvatarPalette(0xFF5AA4D6.toInt(), 0xFF3A2A1C.toInt(), 0xFFF2C9A0.toInt(), "cap"),
        AvatarPalette(0xFFB57EDC.toInt(), 0xFFE0A03A.toInt(), 0xFFF7D3AD.toInt(), "pigtails")
    )

    val PAT_LINES = listOf(
        "嘿嘿，摸头好舒服呀！",
        "今天也要元气满满哦~",
        "你真棒，我会一直陪着你的！",
        "完成任务就能喂我吃零食啦！",
        "一起加油向下一阶进化吧！"
    )

    fun levelForXp(xp: Int): Int {
        var lv = 0
        for (i in LEVELS.indices) {
            if (xp >= LEVELS[i].xp) lv = i
        }
        return lv
    }

    fun parseChildState(obj: JSONObject): ChildState {
        val famObj = obj.getJSONObject("family")
        val childObj = obj.getJSONObject("child")

        val tasks = mutableListOf<TaskItem>()
        val tArr = obj.optJSONArray("tasks") ?: JSONArray()
        for (i in 0 until tArr.length()) {
            val t = tArr.getJSONObject(i)
            tasks.add(
                TaskItem(
                    id = t.getString("id"),
                    emoji = t.optString("emoji", "✨"),
                    label = t.optString("label", "习惯任务"),
                    pts = t.optInt("pts", 3),
                    scope = t.optString("scope", "all"),
                    taskType = t.optString("task_type", "normal"),
                    durationSec = t.optInt("duration_sec", 60),
                    targetCount = t.optInt("target_count", 60)
                )
            )
        }

        val todayStr = obj.optString("today", "")
        val doneSet = mutableSetOf<String>()
        val dArr = obj.optJSONArray("today_done_ids") ?: JSONArray()
        for (i in 0 until dArr.length()) doneSet.add(dArr.getString(i))

        val logs = mutableListOf<CheckinLog>()
        val lArr = obj.optJSONArray("logs") ?: JSONArray()
        for (i in 0 until lArr.length()) {
            val l = lArr.getJSONObject(i)
            val tid = l.optString("task_id", "")
            val cDate = l.optString("checkin_date", "")
            if (todayStr.isNotEmpty() && cDate == todayStr && tid.isNotEmpty()) {
                doneSet.add(tid)
            }
            logs.add(
                CheckinLog(
                    id = l.optString("id", ""),
                    taskId = tid,
                    taskLabel = l.optString("task_label", ""),
                    pts = l.optInt("pts", 0),
                    actualCount = if (l.isNull("actual_count")) null else l.optInt("actual_count"),
                    actualDuration = if (l.isNull("actual_duration")) null else l.optInt("actual_duration"),
                    hasPhoto = l.optBoolean("has_photo", false),
                    photoUrl = l.optString("photo_url", ""),
                    checkinDate = cDate
                )
            )
        }

        val catalog = mutableListOf<SpeciesItem>()
        val cArr = obj.optJSONArray("catalog") ?: JSONArray()
        for (i in 0 until cArr.length()) {
            val s = cArr.getJSONObject(i)
            catalog.add(
                SpeciesItem(
                    id = s.getString("id"),
                    name = s.optString("name", "伙伴"),
                    color = s.optString("color", "#ec9844"),
                    ear = s.optString("ear", "round"),
                    acc = s.optString("acc", "none"),
                    tail = s.optString("tail", "puff"),
                    mouth = s.optString("mouth", "smile"),
                    base = s.optString("base", "grass"),
                    hint = s.optString("hint", "")
                )
            )
        }

        val owned = mutableMapOf<String, String>()
        val oObj = obj.optJSONObject("owned")
        if (oObj != null) {
            val keys = oObj.keys()
            while (keys.hasNext()) {
                val k = keys.next()
                owned[k] = oObj.optString(k, "")
            }
        }

        val polaroids = mutableListOf<PolaroidItem>()
        val pArr = obj.optJSONArray("polaroids") ?: JSONArray()
        for (i in 0 until pArr.length()) {
            val p = pArr.getJSONObject(i)
            polaroids.add(
                PolaroidItem(
                    id = p.optString("id", ""),
                    avatarIdx = p.optInt("avatar_idx", 0),
                    avatarUrl = p.optString("avatar_url", ""),
                    petRef = p.optString("pet_ref", "main"),
                    petLv = p.optInt("pet_lv", 0),
                    photoDate = p.optString("photo_date", ""),
                    note = p.optString("note", "")
                )
            )
        }

        val comp = if (childObj.isNull("active_companion")) null
        else childObj.optString("active_companion", "").ifEmpty { null }

        return ChildState(
            today = obj.optString("today", ""),
            family = FamilyInfo(
                id = famObj.optString("id", ""),
                name = famObj.optString("name", "我的家"),
                familyCode = famObj.optString("family_code", "")
            ),
            child = ChildInfo(
                id = childObj.getString("id"),
                name = childObj.optString("name", "小朋友"),
                avatarIdx = childObj.optInt("avatar_idx", 0),
                avatarUrl = childObj.optString("avatar_url", ""),
                petName = childObj.optString("pet_name", ""),
                xp = childObj.optInt("xp", 0),
                level = childObj.optInt("level", levelForXp(childObj.optInt("xp", 0))),
                coupons = childObj.optInt("coupons", 0),
                activeCompanion = comp,
                muted = childObj.optBoolean("muted", false)
            ),
            childCount = obj.optInt("child_count", 1),
            tasks = tasks,
            todayDoneIds = doneSet,
            logs = logs,
            catalog = catalog,
            owned = owned,
            polaroids = polaroids
        )
    }
}
