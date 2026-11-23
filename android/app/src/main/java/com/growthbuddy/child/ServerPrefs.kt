package com.growthbuddy.child

import android.content.Context
import org.json.JSONArray
import org.json.JSONObject

data class SavedAccount(
    val username: String,
    val password: String,
    val role: String, // "child" | "parent"
    val displayName: String,
    val avatarIdx: Int = 0,
    val avatarUrl: String = "",
    val lastLoginAt: Long = 0L
)

object ServerPrefs {
    private const val PREF_NAME = "gb_child_prefs"
    private const val KEY_SERVER_URL = "server_url"
    private const val KEY_CHILD_TOKEN = "child_token"
    private const val KEY_PARENT_TOKEN = "parent_token"
    private const val KEY_ACTIVE_ROLE = "active_role"
    private const val KEY_SAVED_USERNAME = "saved_username"
    private const val KEY_SAVED_ACCOUNTS = "saved_accounts_json"
    const val DEFAULT_SERVER_URL = "https://linuu.xyz"

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)

    fun getServerUrl(context: Context): String = DEFAULT_SERVER_URL

    fun setServerUrl(context: Context, url: String) {
        // 固定使用默认 HTTPS 服务器地址，不再允许用户修改
        prefs(context).edit().putString(KEY_SERVER_URL, DEFAULT_SERVER_URL).apply()
    }

    fun getActiveRole(context: Context): String {
        val role = prefs(context).getString(KEY_ACTIVE_ROLE, "") ?: ""
        if (role.isNotEmpty()) return role
        if (getChildToken(context).isNotEmpty()) return "child"
        if (getParentToken(context).isNotEmpty()) return "parent"
        return ""
    }

    fun setActiveRole(context: Context, role: String) {
        prefs(context).edit().putString(KEY_ACTIVE_ROLE, role).apply()
    }

    fun getChildToken(context: Context): String {
        return prefs(context).getString(KEY_CHILD_TOKEN, "") ?: ""
    }

    fun setChildToken(context: Context, token: String) {
        prefs(context).edit()
            .putString(KEY_CHILD_TOKEN, token)
            .putString(KEY_ACTIVE_ROLE, "child")
            .apply()
    }

    fun clearChildToken(context: Context) {
        val ed = prefs(context).edit().remove(KEY_CHILD_TOKEN)
        if ((prefs(context).getString(KEY_ACTIVE_ROLE, "") ?: "") == "child") {
            ed.remove(KEY_ACTIVE_ROLE)
        }
        ed.apply()
    }

    fun getParentToken(context: Context): String {
        return prefs(context).getString(KEY_PARENT_TOKEN, "") ?: ""
    }

    fun setParentToken(context: Context, token: String) {
        prefs(context).edit()
            .putString(KEY_PARENT_TOKEN, token)
            .putString(KEY_ACTIVE_ROLE, "parent")
            .apply()
    }

    fun clearParentToken(context: Context) {
        val ed = prefs(context).edit().remove(KEY_PARENT_TOKEN)
        if ((prefs(context).getString(KEY_ACTIVE_ROLE, "") ?: "") == "parent") {
            ed.remove(KEY_ACTIVE_ROLE)
        }
        ed.apply()
    }

    fun getSavedUsername(context: Context): String {
        return prefs(context).getString(KEY_SAVED_USERNAME, "") ?: ""
    }

    fun setSavedUsername(context: Context, username: String) {
        prefs(context).edit().putString(KEY_SAVED_USERNAME, username.trim()).apply()
    }

    fun getSavedAccounts(context: Context): List<SavedAccount> {
        val raw = prefs(context).getString(KEY_SAVED_ACCOUNTS, "[]") ?: "[]"
        val arr = runCatching { JSONArray(raw) }.getOrNull() ?: return emptyList()
        val list = mutableListOf<SavedAccount>()
        for (i in 0 until arr.length()) {
            val o = arr.optJSONObject(i) ?: continue
            val u = o.optString("username", "").trim()
            if (u.isEmpty()) continue
            list.add(
                SavedAccount(
                    username = u,
                    password = o.optString("password", ""),
                    role = o.optString("role", "child"),
                    displayName = o.optString("displayName", u),
                    avatarIdx = o.optInt("avatarIdx", 0),
                    avatarUrl = o.optString("avatarUrl", ""),
                    lastLoginAt = o.optLong("lastLoginAt", 0L)
                )
            )
        }
        return list.sortedByDescending { it.lastLoginAt }
    }

    private fun writeSavedAccounts(context: Context, list: List<SavedAccount>) {
        val arr = JSONArray()
        list.take(10).forEach { acc ->
            arr.put(JSONObject().apply {
                put("username", acc.username)
                put("password", acc.password)
                put("role", acc.role)
                put("displayName", acc.displayName)
                put("avatarIdx", acc.avatarIdx)
                put("avatarUrl", acc.avatarUrl)
                put("lastLoginAt", acc.lastLoginAt)
            })
        }
        prefs(context).edit().putString(KEY_SAVED_ACCOUNTS, arr.toString()).apply()
    }

    fun saveOrUpdateAccount(context: Context, account: SavedAccount) {
        val cleanUser = account.username.trim()
        if (cleanUser.isEmpty()) return
        val existing = getSavedAccounts(context).toMutableList()
        val idx = existing.indexOfFirst {
            it.username.equals(cleanUser, ignoreCase = true) && it.role == account.role
        }
        val merged = if (idx >= 0) {
            val old = existing[idx]
            existing.removeAt(idx)
            account.copy(
                username = cleanUser,
                password = account.password.ifEmpty { old.password },
                displayName = account.displayName.ifBlank { old.displayName },
                avatarUrl = account.avatarUrl.ifBlank { old.avatarUrl },
                lastLoginAt = System.currentTimeMillis()
            )
        } else {
            account.copy(username = cleanUser, lastLoginAt = System.currentTimeMillis())
        }
        existing.add(0, merged)
        writeSavedAccounts(context, existing)
        setSavedUsername(context, cleanUser)
    }

    fun updateAccountProfile(
        context: Context,
        username: String,
        role: String,
        displayName: String,
        avatarIdx: Int,
        avatarUrl: String
    ) {
        if (username.isBlank()) return
        val existing = getSavedAccounts(context).toMutableList()
        val idx = existing.indexOfFirst {
            it.username.equals(username.trim(), ignoreCase = true) && it.role == role
        }
        if (idx >= 0) {
            val old = existing[idx]
            existing[idx] = old.copy(
                displayName = displayName.ifBlank { old.displayName },
                avatarIdx = avatarIdx,
                avatarUrl = avatarUrl
            )
            writeSavedAccounts(context, existing)
        }
    }

    fun removeSavedAccount(context: Context, username: String, role: String) {
        val filtered = getSavedAccounts(context).filterNot {
            it.username.equals(username.trim(), ignoreCase = true) && it.role == role
        }
        writeSavedAccounts(context, filtered)
    }

    fun normalizeUrl(input: String): String {
        var s = input.trim()
        if (s.isEmpty()) return ""
        if (!s.startsWith("http://") && !s.startsWith("https://")) {
            s = "http://$s"
        }
        return s.trimEnd('/')
    }
}
