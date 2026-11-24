package com.growthbuddy.child

import android.graphics.Bitmap
import android.graphics.Color
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.QRCodeWriter
import org.json.JSONArray
import org.json.JSONObject

data class SocialSelfInfo(
    val id: String,
    val name: String,
    val username: String,
    val friendCode: String,
    val qrPayload: String,
    val avatarIdx: Int,
    val avatarUrl: String
)

data class FriendItem(
    val peerType: String,
    val peerId: String,
    val name: String,
    val roleLabel: String,
    val groupName: String,
    val isFamily: Boolean,
    val avatarIdx: Int,
    val avatarUrl: String,
    val petName: String,
    val petLevel: Int,
    val unreadCount: Int,
    val lastMessage: String,
    val lastMessageAt: Long
)

data class FriendRequestItem(
    val id: String,
    val fromChildId: String,
    val toChildId: String,
    val name: String,
    val username: String,
    val avatarIdx: Int,
    val avatarUrl: String,
    val petName: String,
    val petLevel: Int,
    val message: String,
    val status: String,
    val createdAt: Long
)

data class FriendLookupCandidate(
    val childId: String,
    val name: String,
    val username: String,
    val avatarIdx: Int,
    val avatarUrl: String,
    val petName: String,
    val petLevel: Int,
    val relationStatus: String
)

data class ChatGroupMemberItem(
    val memberType: String,
    val memberId: String,
    val name: String,
    val roleLabel: String,
    val avatarIdx: Int,
    val avatarUrl: String,
    val role: String
)

data class ChatGroupItem(
    val id: String,
    val name: String,
    val isFamilyDefault: Boolean,
    val memberCount: Int,
    val membersPreview: String,
    val unreadCount: Int,
    val lastMessage: String,
    val lastMessageAt: Long,
    val members: List<ChatGroupMemberItem> = emptyList()
)

data class SocialOverview(
    val self: SocialSelfInfo,
    val groups: List<String>,
    val friends: List<FriendItem>,
    val chatGroups: List<ChatGroupItem>,
    val incomingRequests: List<FriendRequestItem>,
    val outgoingRequests: List<FriendRequestItem>
)

data class ChatMessageItem(
    val id: String,
    val senderType: String,
    val senderId: String,
    val senderName: String,
    val senderAvatarIdx: Int,
    val senderAvatarUrl: String,
    val receiverType: String,
    val receiverId: String,
    val isSelf: Boolean,
    val msgType: String,
    val text: String,
    val mediaUrl: String,
    val durationSec: Int,
    val isRead: Boolean,
    val createdAt: Long
)

object SocialParser {

    fun parseOverview(obj: JSONObject): SocialOverview {
        val sObj = obj.optJSONObject("self") ?: JSONObject()
        val self = SocialSelfInfo(
            id = sObj.optString("id", ""),
            name = sObj.optString("name", "小朋友"),
            username = sObj.optString("username", ""),
            friendCode = sObj.optString("friend_code", ""),
            qrPayload = sObj.optString("qr_payload", ""),
            avatarIdx = sObj.optInt("avatar_idx", 0),
            avatarUrl = sObj.optString("avatar_url", "")
        )

        val groups = mutableListOf<String>()
        val gArr = obj.optJSONArray("groups") ?: JSONArray()
        for (i in 0 until gArr.length()) groups.add(gArr.getString(i))
        if (groups.isEmpty()) {
            groups.add("家庭")
            groups.add("朋友")
        }

        val friends = mutableListOf<FriendItem>()
        val fArr = obj.optJSONArray("friends") ?: JSONArray()
        for (i in 0 until fArr.length()) {
            val f = fArr.getJSONObject(i)
            friends.add(
                FriendItem(
                    peerType = f.optString("peer_type", "child"),
                    peerId = f.optString("peer_id", ""),
                    name = f.optString("name", "好友"),
                    roleLabel = f.optString("role_label", ""),
                    groupName = f.optString("group_name", "朋友"),
                    isFamily = f.optBoolean("is_family", false),
                    avatarIdx = f.optInt("avatar_idx", 0),
                    avatarUrl = f.optString("avatar_url", ""),
                    petName = f.optString("pet_name", ""),
                    petLevel = f.optInt("pet_level", 0),
                    unreadCount = f.optInt("unread_count", 0),
                    lastMessage = f.optString("last_message", ""),
                    lastMessageAt = f.optLong("last_message_at", 0L)
                )
            )
        }

        val chatGroups = mutableListOf<ChatGroupItem>()
        val cgArr = obj.optJSONArray("chat_groups") ?: JSONArray()
        for (i in 0 until cgArr.length()) {
            chatGroups.add(parseGroupItem(cgArr.getJSONObject(i)))
        }

        val inc = parseRequests(obj.optJSONArray("incoming_requests") ?: JSONArray())
        val out = parseRequests(obj.optJSONArray("outgoing_requests") ?: JSONArray())
        return SocialOverview(self, groups, friends, chatGroups, inc, out)
    }

    fun parseGroupItem(g: JSONObject): ChatGroupItem {
        val mList = mutableListOf<ChatGroupMemberItem>()
        val mArr = g.optJSONArray("members")
        if (mArr != null) {
            for (j in 0 until mArr.length()) {
                val m = mArr.getJSONObject(j)
                mList.add(
                    ChatGroupMemberItem(
                        memberType = m.optString("member_type", "child"),
                        memberId = m.optString("member_id", ""),
                        name = m.optString("name", "成员"),
                        roleLabel = m.optString("role_label", ""),
                        avatarIdx = m.optInt("avatar_idx", 0),
                        avatarUrl = m.optString("avatar_url", ""),
                        role = m.optString("role", "member")
                    )
                )
            }
        }
        return ChatGroupItem(
            id = g.optString("id", ""),
            name = g.optString("name", "群聊"),
            isFamilyDefault = g.optBoolean("is_family_default", false),
            memberCount = g.optInt("member_count", mList.size),
            membersPreview = g.optString("members_preview", ""),
            unreadCount = g.optInt("unread_count", 0),
            lastMessage = g.optString("last_message", ""),
            lastMessageAt = g.optLong("last_message_at", 0L),
            members = mList
        )
    }

    private fun parseRequests(arr: JSONArray): List<FriendRequestItem> {
        val list = mutableListOf<FriendRequestItem>()
        for (i in 0 until arr.length()) {
            val r = arr.getJSONObject(i)
            list.add(
                FriendRequestItem(
                    id = r.optString("id", ""),
                    fromChildId = r.optString("from_child_id", ""),
                    toChildId = r.optString("to_child_id", ""),
                    name = r.optString("name", "小伙伴"),
                    username = r.optString("username", ""),
                    avatarIdx = r.optInt("avatar_idx", 0),
                    avatarUrl = r.optString("avatar_url", ""),
                    petName = r.optString("pet_name", ""),
                    petLevel = r.optInt("pet_level", 0),
                    message = r.optString("message", ""),
                    status = r.optString("status", "pending"),
                    createdAt = r.optLong("created_at", 0L)
                )
            )
        }
        return list
    }

    fun parseLookup(obj: JSONObject): FriendLookupCandidate {
        return FriendLookupCandidate(
            childId = obj.optString("child_id", ""),
            name = obj.optString("name", ""),
            username = obj.optString("username", ""),
            avatarIdx = obj.optInt("avatar_idx", 0),
            avatarUrl = obj.optString("avatar_url", ""),
            petName = obj.optString("pet_name", ""),
            petLevel = obj.optInt("pet_level", 0),
            relationStatus = obj.optString("relation_status", "none")
        )
    }

    fun parseMessages(arr: JSONArray): List<ChatMessageItem> {
        val list = mutableListOf<ChatMessageItem>()
        for (i in 0 until arr.length()) {
            val m = arr.getJSONObject(i)
            list.add(
                ChatMessageItem(
                    id = m.optString("id", ""),
                    senderType = m.optString("sender_type", "child"),
                    senderId = m.optString("sender_id", ""),
                    senderName = m.optString("sender_name", ""),
                    senderAvatarIdx = m.optInt("sender_avatar_idx", 0),
                    senderAvatarUrl = m.optString("sender_avatar_url", ""),
                    receiverType = m.optString("receiver_type", "child"),
                    receiverId = m.optString("receiver_id", ""),
                    isSelf = m.optBoolean("is_self", false),
                    msgType = m.optString("msg_type", "text"),
                    text = m.optString("text", ""),
                    mediaUrl = m.optString("media_url", ""),
                    durationSec = m.optInt("duration_sec", 0),
                    isRead = m.optBoolean("is_read", false),
                    createdAt = m.optLong("created_at", 0L)
                )
            )
        }
        return list
    }

    fun generateQrBitmap(content: String, sizePx: Int = 520): Bitmap? {
        if (content.isBlank()) return null
        return try {
            val hints = mapOf(
                EncodeHintType.CHARACTER_SET to "UTF-8",
                EncodeHintType.MARGIN to 2
            )
            val matrix = QRCodeWriter().encode(content, BarcodeFormat.QR_CODE, sizePx, sizePx, hints)
            val pixels = IntArray(sizePx * sizePx)
            val fg = Color.parseColor("#233128")
            val bg = Color.WHITE
            for (y in 0 until sizePx) {
                val offset = y * sizePx
                for (x in 0 until sizePx) {
                    pixels[offset + x] = if (matrix.get(x, y)) fg else bg
                }
            }
            Bitmap.createBitmap(pixels, sizePx, sizePx, Bitmap.Config.ARGB_8888)
        } catch (_: Exception) {
            null
        }
    }
}

fun SpeciesThumbView.loadAvatar(api: ApiClient, avatarIdx: Int, avatarUrl: String) {
    tag = avatarUrl
    setAvatar(avatarIdx, null)
    if (avatarUrl.isNotEmpty()) {
        api.fetchBitmap(avatarUrl) { bmp ->
            if (bmp != null && tag == avatarUrl) {
                setAvatar(avatarIdx, bmp)
            }
        }
    }
}
