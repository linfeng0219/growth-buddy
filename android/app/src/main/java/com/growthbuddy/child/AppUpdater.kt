package com.growthbuddy.child

import android.app.Activity
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import android.widget.Toast
import androidx.appcompat.app.AlertDialog
import androidx.core.content.FileProvider
import org.json.JSONObject
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.security.MessageDigest
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean

data class RemoteReleaseInfo(
    val id: String,
    val versionCode: Int,
    val versionName: String,
    val releaseNotes: String,
    val forceUpdate: Boolean,
    val fileName: String,
    val fileSize: Long,
    val sha256: String,
    val downloadUrl: String
)

/**
 * 原生 OTA 远程在线升级引擎：
 * - 查询 /api/app/version?platform=android&currentVersionCode=...
 * - 弹窗展示版本信息与更新日志（支持强制/可选升级）
 * - 后台流式下载 APK 并显示实时进度条，自动校验 SHA-256 完整性
 * - 兼容 Android 8~14 FileProvider 与「允许安装未知来源应用」引导
 */
class AppUpdater(private val activity: Activity) {

    private val executor = Executors.newSingleThreadExecutor()
    private val mainHandler = Handler(Looper.getMainLooper())
    private val checking = AtomicBoolean(false)
    private val downloading = AtomicBoolean(false)
    private var pendingInstallApk: File? = null

    init {
        cleanupOldApks()
    }

    private fun updatesDir(): File {
        val dir = File(activity.cacheDir, "updates")
        if (!dir.exists()) dir.mkdirs()
        return dir
    }

    private fun cleanupOldApks() {
        executor.execute {
            runCatching {
                updatesDir().listFiles()?.forEach { f ->
                    if (f.isFile && f.name.endsWith(".apk")) f.delete()
                }
            }
        }
    }

    fun checkForUpdate(silent: Boolean) {
        if (!checking.compareAndSet(false, true)) return
        val baseUrl = ServerPrefs.getServerUrl(activity).trimEnd('/')
        val currentCode = BuildConfig.VERSION_CODE
        val urlStr = "$baseUrl/api/app/version?platform=android&currentVersionCode=$currentCode"

        executor.execute {
            try {
                val conn = (URL(urlStr).openConnection() as HttpURLConnection).apply {
                    requestMethod = "GET"
                    connectTimeout = 8000
                    readTimeout = 8000
                    setRequestProperty("Accept", "application/json")
                }
                val code = conn.responseCode
                val body = (if (code in 200..299) conn.inputStream else conn.errorStream)
                    ?.bufferedReader(Charsets.UTF_8)?.use { it.readText() } ?: ""
                conn.disconnect()

                val json = JSONObject(body)
                val hasUpdate = json.optBoolean("hasUpdate", false)
                val relObj = json.optJSONObject("release")
                if (hasUpdate && relObj != null) {
                    val rawUrl = relObj.optString("download_url", "")
                    val fullUrl = when {
                        rawUrl.startsWith("http://") || rawUrl.startsWith("https://") -> rawUrl
                        rawUrl.startsWith("/") -> baseUrl + rawUrl
                        else -> "$baseUrl/api/app/latest.apk"
                    }
                    val info = RemoteReleaseInfo(
                        id = relObj.optString("id", ""),
                        versionCode = relObj.optInt("version_code", currentCode),
                        versionName = relObj.optString("version_name", ""),
                        releaseNotes = relObj.optString("release_notes", ""),
                        forceUpdate = json.optBoolean("forceUpdate", relObj.optBoolean("force_update", false)),
                        fileName = relObj.optString("file_name", "GrowthBuddyChild.apk"),
                        fileSize = relObj.optLong("file_size", 0L),
                        sha256 = relObj.optString("sha256", "").lowercase(),
                        downloadUrl = fullUrl
                    )
                    mainHandler.post {
                        if (!activity.isFinishing && !activity.isDestroyed) {
                            showUpdateDialog(info)
                        }
                    }
                } else if (!silent) {
                    mainHandler.post {
                        if (!activity.isFinishing) {
                            Toast.makeText(
                                activity,
                                "当前已是最新版本（v${BuildConfig.VERSION_NAME}）",
                                Toast.LENGTH_SHORT
                            ).show()
                        }
                    }
                }
            } catch (e: Exception) {
                if (!silent) {
                    mainHandler.post {
                        if (!activity.isFinishing) {
                            Toast.makeText(
                                activity,
                                "检查更新失败：${e.message ?: "请检查服务器连接"}",
                                Toast.LENGTH_SHORT
                            ).show()
                        }
                    }
                }
            } finally {
                checking.set(false)
            }
        }
    }

    private fun showUpdateDialog(info: RemoteReleaseInfo) {
        val sizeText = if (info.fileSize > 0) {
            String.format("安装包大小：%.1f MB\n", info.fileSize / (1024.0 * 1024.0))
        } else ""
        val notes = info.releaseNotes.ifBlank { "包含体验优化与问题修复" }
        val msg = "发现新版本 v${info.versionName}（当前 v${BuildConfig.VERSION_NAME}）\n${sizeText}\n更新内容：\n$notes"

        val builder = AlertDialog.Builder(activity)
            .setTitle(if (info.forceUpdate) "🚀 发现重要版本升级" else "🎉 发现新版本")
            .setMessage(msg)
            .setCancelable(!info.forceUpdate)
            .setPositiveButton("立即升级") { _, _ ->
                startDownloadAndInstall(info)
            }
        if (!info.forceUpdate) {
            builder.setNegativeButton("稍后再说", null)
        }
        builder.show()
    }

    private fun startDownloadAndInstall(info: RemoteReleaseInfo) {
        if (!downloading.compareAndSet(false, true)) return

        val cancelFlag = AtomicBoolean(false)
        val density = activity.resources.displayMetrics.density
        val pad = (20 * density).toInt()

        val container = LinearLayout(activity).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(pad, pad, pad, (12 * density).toInt())
        }
        val tvStatus = TextView(activity).apply {
            text = "正在连接服务器下载 v${info.versionName}..."
            textSize = 14f
            setTextColor(0xFF2B2620.toInt())
        }
        val progressBar = ProgressBar(activity, null, android.R.attr.progressBarStyleHorizontal).apply {
            max = 100
            progress = 0
            isIndeterminate = info.fileSize <= 0
            layoutParams = LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT
            ).apply { topMargin = (12 * density).toInt() }
        }
        container.addView(tvStatus)
        container.addView(progressBar)

        val dlgBuilder = AlertDialog.Builder(activity)
            .setTitle("正在下载更新")
            .setView(container)
            .setCancelable(false)
        if (!info.forceUpdate) {
            dlgBuilder.setNegativeButton("取消下载") { _, _ -> cancelFlag.set(true) }
        }
        val dialog = dlgBuilder.create()
        dialog.show()

        executor.execute {
            val outFile = File(updatesDir(), "GrowthBuddy-v${info.versionCode}.apk")
            try {
                if (outFile.exists()) outFile.delete()
                val conn = (URL(info.downloadUrl).openConnection() as HttpURLConnection).apply {
                    instanceFollowRedirects = true
                    connectTimeout = 15000
                    readTimeout = 30000
                }
                val code = conn.responseCode
                if (code !in 200..299) {
                    throw IllegalStateException("服务器返回 HTTP $code")
                }
                val total = if (conn.contentLengthLong > 0) conn.contentLengthLong else info.fileSize
                val digest = MessageDigest.getInstance("SHA-256")
                var downloaded = 0L
                var lastUiAt = 0L

                conn.inputStream.use { input ->
                    FileOutputStream(outFile).use { output ->
                        val buf = ByteArray(16 * 1024)
                        while (true) {
                            if (cancelFlag.get()) {
                                throw InterruptedException("已取消下载")
                            }
                            val n = input.read(buf)
                            if (n <= 0) break
                            output.write(buf, 0, n)
                            digest.update(buf, 0, n)
                            downloaded += n

                            val now = System.currentTimeMillis()
                            if (now - lastUiAt >= 120) {
                                lastUiAt = now
                                val cur = downloaded
                                mainHandler.post {
                                    if (total > 0) {
                                        progressBar.isIndeterminate = false
                                        val pct = ((cur * 100) / total).toInt().coerceIn(0, 100)
                                        progressBar.progress = pct
                                        tvStatus.text = String.format(
                                            "已下载 %.1f MB / %.1f MB（%d%%）",
                                            cur / (1024.0 * 1024.0),
                                            total / (1024.0 * 1024.0),
                                            pct
                                        )
                                    } else {
                                        tvStatus.text = String.format("已下载 %.1f MB...", cur / (1024.0 * 1024.0))
                                    }
                                }
                            }
                        }
                    }
                }
                conn.disconnect()

                // 校验 SHA-256（若服务端提供）
                if (info.sha256.isNotEmpty()) {
                    val actualSha = digest.digest().joinToString("") { "%02x".format(it) }
                    if (!actualSha.equals(info.sha256, ignoreCase = true)) {
                        outFile.delete()
                        throw IllegalStateException("安装包完整性校验（SHA-256）未通过，请重试")
                    }
                }

                mainHandler.post {
                    if (dialog.isShowing) dialog.dismiss()
                    if (!activity.isFinishing && !activity.isDestroyed) {
                        triggerApkInstall(outFile)
                    }
                }
            } catch (e: InterruptedException) {
                outFile.delete()
                mainHandler.post {
                    if (dialog.isShowing) dialog.dismiss()
                    Toast.makeText(activity, "已取消下载", Toast.LENGTH_SHORT).show()
                }
            } catch (e: Exception) {
                outFile.delete()
                mainHandler.post {
                    if (dialog.isShowing) dialog.dismiss()
                    if (!activity.isFinishing) {
                        AlertDialog.Builder(activity)
                            .setTitle("下载更新失败")
                            .setMessage(e.message ?: "请检查网络后重试")
                            .setPositiveButton("重试") { _, _ -> startDownloadAndInstall(info) }
                            .setNegativeButton(if (info.forceUpdate) "关闭" else "取消", null)
                            .show()
                    }
                }
            } finally {
                downloading.set(false)
            }
        }
    }

    private fun triggerApkInstall(apkFile: File) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O && !activity.packageManager.canRequestPackageInstalls()) {
            pendingInstallApk = apkFile
            AlertDialog.Builder(activity)
                .setTitle("需要开启「安装应用」权限")
                .setMessage("安装包已下载完成！请在接下来的系统设置中允许「成长小伙伴」安装更新包，返回后将自动继续安装。")
                .setPositiveButton("去开启") { _, _ ->
                    val intent = Intent(
                        Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,
                        Uri.parse("package:${activity.packageName}")
                    )
                    activity.startActivity(intent)
                }
                .setNegativeButton("取消") { _, _ -> pendingInstallApk = null }
                .show()
            return
        }

        pendingInstallApk = null
        val uri = FileProvider.getUriForFile(activity, "${activity.packageName}.fileprovider", apkFile)
        val installIntent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        try {
            activity.startActivity(installIntent)
        } catch (e: Exception) {
            Toast.makeText(activity, "拉起系统安装器失败：${e.message}", Toast.LENGTH_LONG).show()
        }
    }

    fun resumePendingInstallIfAllowed() {
        val apk = pendingInstallApk ?: return
        if (!apk.exists()) {
            pendingInstallApk = null
            return
        }
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O || activity.packageManager.canRequestPackageInstalls()) {
            triggerApkInstall(apk)
        }
    }

    fun shutdown() {
        executor.shutdownNow()
    }
}
