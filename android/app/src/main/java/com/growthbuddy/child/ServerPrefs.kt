package com.growthbuddy.child

import android.content.Context

object ServerPrefs {
    private const val PREF_NAME = "gb_child_prefs"
    private const val KEY_SERVER_URL = "server_url"
    private const val KEY_CHILD_TOKEN = "child_token"
    private const val KEY_SAVED_USERNAME = "saved_username"
    const val DEFAULT_SERVER_URL = "http://linuu.xyz"

    private fun prefs(context: Context) =
        context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)

    fun getServerUrl(context: Context): String {
        val raw = prefs(context).getString(KEY_SERVER_URL, DEFAULT_SERVER_URL) ?: DEFAULT_SERVER_URL
        val clean = normalizeUrl(raw)
        return clean.ifEmpty { DEFAULT_SERVER_URL }
    }

    fun setServerUrl(context: Context, url: String) {
        val clean = normalizeUrl(url).ifEmpty { DEFAULT_SERVER_URL }
        prefs(context).edit().putString(KEY_SERVER_URL, clean).apply()
    }

    fun getChildToken(context: Context): String {
        return prefs(context).getString(KEY_CHILD_TOKEN, "") ?: ""
    }

    fun setChildToken(context: Context, token: String) {
        prefs(context).edit().putString(KEY_CHILD_TOKEN, token).apply()
    }

    fun clearChildToken(context: Context) {
        prefs(context).edit().remove(KEY_CHILD_TOKEN).apply()
    }

    fun getSavedUsername(context: Context): String {
        return prefs(context).getString(KEY_SAVED_USERNAME, "") ?: ""
    }

    fun setSavedUsername(context: Context, username: String) {
        prefs(context).edit().putString(KEY_SAVED_USERNAME, username.trim()).apply()
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
