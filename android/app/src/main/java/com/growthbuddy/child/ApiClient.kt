package com.growthbuddy.child

import android.content.Context
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.os.Handler
import android.os.Looper
import android.util.LruCache
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.io.OutputStreamWriter
import java.net.HttpURLConnection
import java.net.URL
import java.util.concurrent.Executors

/**
 * 轻量零依赖原生 HTTP JSON 客户端：
 * 在后台线程池执行请求并在主线程回调结果，避免 WebView 开销。
 */
class ApiClient(private val context: Context) {

    private val executor = Executors.newCachedThreadPool()
    private val mainHandler = Handler(Looper.getMainLooper())
    private val bitmapCache = object : LruCache<String, Bitmap>(32) {}

    data class ApiResult(
        val ok: Boolean,
        val status: Int,
        val json: JSONObject?,
        val error: String?
    )

    fun request(
        method: String,
        path: String,
        body: JSONObject? = null,
        useAuth: Boolean = true,
        callback: (ApiResult) -> Unit
    ) {
        val baseUrl = ServerPrefs.getServerUrl(context)
        val token = if (useAuth) ServerPrefs.getChildToken(context) else ""

        executor.execute {
            var conn: HttpURLConnection? = null
            try {
                val url = URL(baseUrl + path)
                conn = (url.openConnection() as HttpURLConnection).apply {
                    requestMethod = method
                    connectTimeout = 8000
                    readTimeout = 15000
                    setRequestProperty("Accept", "application/json")
                    setRequestProperty("Content-Type", "application/json; charset=UTF-8")
                    setRequestProperty("X-App-Platform", "android")
                    setRequestProperty("X-App-VersionCode", BuildConfig.VERSION_CODE.toString())
                    if (token.isNotEmpty()) {
                        setRequestProperty("Authorization", "Bearer $token")
                    }
                    if (body != null && (method == "POST" || method == "PATCH" || method == "PUT")) {
                        doOutput = true
                        OutputStreamWriter(outputStream, Charsets.UTF_8).use { writer ->
                            writer.write(body.toString())
                        }
                    }
                }

                val code = conn.responseCode
                val stream = if (code in 200..299) conn.inputStream else conn.errorStream
                val rawText = if (stream != null) {
                    BufferedReader(InputStreamReader(stream, Charsets.UTF_8)).use { it.readText() }
                } else ""

                val json = runCatching { JSONObject(rawText) }.getOrNull()
                val ok = code in 200..299
                val errMsg = if (!ok) {
                    json?.optString("error", "")?.ifEmpty { null }
                        ?: "请求失败（HTTP $code）"
                } else null

                mainHandler.post { callback(ApiResult(ok, code, json, errMsg)) }
            } catch (e: Exception) {
                val msg = when {
                    e.message?.contains("Unable to resolve host", ignoreCase = true) == true ->
                        "无法解析服务器域名，请检查网络或服务器地址"
                    e.message?.contains("failed to connect", ignoreCase = true) == true ->
                        "连接服务器失败，请检查网络连接后重试"
                    else -> "网络请求出错：${e.localizedMessage ?: "请稍后重试"}"
                }
                mainHandler.post { callback(ApiResult(false, 0, null, msg)) }
            } finally {
                conn?.disconnect()
            }
        }
    }

    fun fetchBitmap(pathOrUrl: String, callback: (Bitmap?) -> Unit) {
        if (pathOrUrl.isEmpty()) {
            callback(null)
            return
        }
        val baseUrl = ServerPrefs.getServerUrl(context)
        val fullUrl = if (pathOrUrl.startsWith("http://") || pathOrUrl.startsWith("https://")) {
            pathOrUrl
        } else {
            baseUrl + pathOrUrl
        }
        bitmapCache.get(fullUrl)?.let { cached ->
            callback(cached)
            return
        }
        executor.execute {
            var conn: HttpURLConnection? = null
            val bmp = try {
                conn = (URL(fullUrl).openConnection() as HttpURLConnection).apply {
                    requestMethod = "GET"
                    connectTimeout = 8000
                    readTimeout = 10000
                }
                if (conn.responseCode in 200..299) {
                    conn.inputStream.use { BitmapFactory.decodeStream(it) }
                } else null
            } catch (_: Exception) {
                null
            } finally {
                conn?.disconnect()
            }
            if (bmp != null) {
                bitmapCache.put(fullUrl, bmp)
            }
            mainHandler.post { callback(bmp) }
        }
    }

    fun shutdown() {
        executor.shutdownNow()
    }
}
