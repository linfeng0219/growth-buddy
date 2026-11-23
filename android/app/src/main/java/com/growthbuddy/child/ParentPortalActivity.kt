package com.growthbuddy.child

import android.annotation.SuppressLint
import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.view.View
import android.webkit.JavascriptInterface
import android.webkit.JsResult
import android.webkit.ValueCallback
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import androidx.activity.OnBackPressedCallback
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import androidx.appcompat.app.AppCompatActivity
import com.growthbuddy.child.databinding.ActivityParentPortalBinding
import org.json.JSONObject

/**
 * 原生内嵌家长管理台：
 * - 自动注入家长登录态（gb_parent_token），与网页端同步支持全部功能（群作业导入、家长信箱、孩子总览、打卡照片查验等）；
 * - 支持图片文件上传（头像/私有图标）及无缝切换回 App 登录页（供家长与孩子在同一台设备上快速切换身份）。
 */
class ParentPortalActivity : AppCompatActivity() {

    companion object {
        const val EXTRA_PARENT_NAME = "parent_name"
        const val RESULT_SWITCH_ACCOUNT = RESULT_FIRST_USER + 1
    }

    private lateinit var binding: ActivityParentPortalBinding
    private var parentToken: String = ""
    private var tokenInjected = false
    private var fileChooserCallback: ValueCallback<Array<Uri>>? = null

    private val filePickerLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { res ->
        val uris = if (res.resultCode == RESULT_OK) {
            WebChromeClient.FileChooserParams.parseResult(res.resultCode, res.data)
        } else null
        fileChooserCallback?.onReceiveValue(uris)
        fileChooserCallback = null
    }

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityParentPortalBinding.inflate(layoutInflater)
        setContentView(binding.root)

        parentToken = ServerPrefs.getParentToken(this)
        if (parentToken.isEmpty()) {
            switchBackToLogin()
            return
        }

        binding.btnParentRetry.setOnClickListener {
            binding.parentErrorBar.visibility = View.GONE
            binding.webParentPortal.reload()
        }
        binding.btnParentSwitchFallback.setOnClickListener {
            confirmSwitchAccount()
        }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                if (binding.webParentPortal.canGoBack()) {
                    binding.webParentPortal.goBack()
                } else {
                    setResult(RESULT_OK)
                    finish()
                }
            }
        })

        val ws: WebSettings = binding.webParentPortal.settings
        ws.javaScriptEnabled = true
        ws.domStorageEnabled = true
        ws.allowFileAccess = true
        ws.useWideViewPort = true
        ws.loadWithOverviewMode = true
        // 令牌未确认就绪前先隐藏 WebView，杜绝闪现网页登录页
        binding.webParentPortal.alpha = 0f

        binding.webParentPortal.addJavascriptInterface(object {
            @JavascriptInterface
            fun onWebLogout() {
                runOnUiThread { switchBackToLogin() }
            }
        }, "AndroidBridge")

        binding.webParentPortal.webViewClient = object : WebViewClient() {
            override fun shouldOverrideUrlLoading(view: WebView?, request: WebResourceRequest?): Boolean {
                val u = request?.url?.toString() ?: return false
                // 如点击网页端返回首页或下载 APK 链接，保留在外部处理或跳过，避免跳出家长控制台
                if (u.endsWith(".apk") || u.contains("/api/app/")) {
                    return true
                }
                return false
            }

            override fun onReceivedError(
                view: WebView?,
                request: WebResourceRequest?,
                error: android.webkit.WebResourceError?
            ) {
                super.onReceivedError(view, request, error)
                if (request?.isForMainFrame == true) {
                    binding.parentWebLoading.visibility = View.GONE
                    binding.parentErrorBar.visibility = View.VISIBLE
                }
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                if (url == null || !url.contains("/parent.html")) {
                    binding.parentWebLoading.visibility = View.GONE
                    binding.webParentPortal.alpha = 1f
                    return
                }

                val quotedTok = JSONObject.quote(parentToken)
                val js = """
                    (function(){
                      try{
                        var desired = $quotedTok;
                        if(localStorage.getItem('gb_parent_token') !== desired){
                          localStorage.setItem('gb_parent_token', desired);
                          if(!window.__gbReloadedOnce){
                            window.__gbReloadedOnce = true;
                            location.reload();
                            return 'reloading';
                          }
                        }
                        var av = document.getElementById('authView');
                        if(av) av.style.display = 'none';
                        return 'ready';
                      }catch(e){ return 'ready'; }
                    })();
                """.trimIndent()
                view?.evaluateJavascript(js) { res ->
                    if (res?.contains("reloading") != true) {
                        binding.parentWebLoading.visibility = View.GONE
                        binding.webParentPortal.animate().alpha(1f).setDuration(120).start()
                        tokenInjected = true
                    }
                }
            }
        }

        binding.webParentPortal.webChromeClient = object : WebChromeClient() {
            override fun onProgressChanged(view: WebView?, newProgress: Int) {
                binding.parentWebLoading.visibility = if (newProgress < 100) View.VISIBLE else View.GONE
            }

            override fun onJsAlert(view: WebView?, url: String?, message: String?, result: JsResult?): Boolean {
                AlertDialog.Builder(this@ParentPortalActivity)
                    .setMessage(message ?: "")
                    .setPositiveButton("确定") { _, _ -> result?.confirm() }
                    .setOnCancelListener { result?.cancel() }
                    .show()
                return true
            }

            override fun onJsConfirm(view: WebView?, url: String?, message: String?, result: JsResult?): Boolean {
                AlertDialog.Builder(this@ParentPortalActivity)
                    .setMessage(message ?: "")
                    .setPositiveButton("确定") { _, _ -> result?.confirm() }
                    .setNegativeButton("取消") { _, _ -> result?.cancel() }
                    .setOnCancelListener { result?.cancel() }
                    .show()
                return true
            }

            override fun onShowFileChooser(
                webView: WebView?,
                filePathCallback: ValueCallback<Array<Uri>>?,
                fileChooserParams: FileChooserParams?
            ): Boolean {
                fileChooserCallback?.onReceiveValue(null)
                fileChooserCallback = filePathCallback
                return try {
                    val intent = fileChooserParams?.createIntent()
                        ?: Intent(Intent.ACTION_GET_CONTENT).apply {
                            type = "image/*"
                            addCategory(Intent.CATEGORY_OPENABLE)
                        }
                    filePickerLauncher.launch(intent)
                    true
                } catch (_: Exception) {
                    fileChooserCallback = null
                    false
                }
            }
        }

        val targetUrl = ServerPrefs.getServerUrl(this).trimEnd('/') +
            "/parent.html#gb_app_token=" + Uri.encode(parentToken)
        binding.parentWebLoading.visibility = View.VISIBLE
        binding.webParentPortal.loadUrl(targetUrl)
    }

    private fun confirmSwitchAccount() {
        AlertDialog.Builder(this)
            .setTitle("退出家长端切换账号？")
            .setMessage("退出后可在登录页选择其他孩子或家长账号快速登录")
            .setPositiveButton("切换账号") { _, _ -> switchBackToLogin() }
            .setNegativeButton("取消", null)
            .show()
    }

    private fun switchBackToLogin() {
        ServerPrefs.clearParentToken(this)
        setResult(RESULT_SWITCH_ACCOUNT)
        finish()
    }

    override fun onDestroy() {
        binding.webParentPortal.destroy()
        super.onDestroy()
    }
}
