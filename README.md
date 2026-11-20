# 成长小伙伴（Growth Buddy）

面向多家庭习惯养成与运动打卡的三端协同平台：
- **系统管理端 Web（`/admin.html`）**：管理全平台家庭、家长与儿童账号，并内置 **App 远程升级（OTA）版本发布台**。
- **家长端 Web（`/parent.html`）**：管理家庭多孩档案、配置孩子独立登录账号密码、布置日常习惯与定时 AI 跳绳任务、核发领养蛋。
- **儿童端纯原生安卓 App（`android/`）**：零 WebView 纯 Kotlin 原生应用，包含 60fps 糖霜宠物舞台、低延迟高响度 `SoundPool` 音效、「天天跳绳」式肩髋多点融合防漏计 AI 跳绳计数，以及 **OTA 远程在线升级**。

---

## 目录结构

```text
growth-buddy/
├── .github/workflows/ci-release.yml   # GitHub Actions 自动测试、编译与 Release 发布脚本
├── server/                            # Go + SQLite 零外部依赖三端后端（内嵌 Web 静态资源）
│   ├── main.go / server.go / store.go
│   ├── sqlite.go / auth.go / scrypt.go
│   ├── server_test.go                 # 集成测试（含密码兼容与 OTA 生命周期测试）
│   ├── public/                        # 管理端 / 家长端 / 儿童端 Web 页面
│   ├── Dockerfile                     # 多阶段 Go + SQLite 构建镜像
│   ├── docker-compose.yml
│   ├── nginx.conf.example             # 1Panel / OpenResty 反代配置（160M APK 上传限制）
│   └── deploy-aliyun.sh               # 阿里云一键部署脚本
└── android/                           # 纯原生安卓儿童端工程（Kotlin + CameraX + ML Kit）
    └── app/src/main/java/com/growthbuddy/child/
        ├── MainActivity.kt            # 纯原生登录 / 宠物舞台 / 任务 / 图鉴 / 纪念墙
        ├── PetCanvasView.kt           # 60fps Candy-Glaze 糖霜宠物绘制与粒子特效
        ├── NativeSoundEngine.kt       # 预合成高响度 SoundPool WAV + 满音量 TTS
        ├── RopeJumpDetector.kt        # 肩髋多点融合 + 动态波峰波谷反转防漏计状态机
        ├── RopeJumpActivity.kt        # CameraX 高帧率识别 + 前后镜头切换
        └── AppUpdater.kt              # OTA 远程检测、SHA-256 校验与系统安装器拉起
```

---

## 快速开发与测试

### 1. Go + SQLite 后端
需要 Go 1.22+、gcc 与 `libsqlite3-dev`（零第三方 Go 模块依赖）：
```bash
cd server
go test -v ./...
CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic -lsqlite3 -Wl,-Bdynamic -lm -ldl -lpthread" \
  go build -trimpath -ldflags="-s -w" -o growth-buddy-server .
```

### 2. 纯原生安卓儿童端
需要 JDK 17 与 Android SDK 34（可用 Android Studio 直接打开 `android/` 目录，或命令行构建）：
```bash
cd android
gradle testDebugUnitTest assembleDebug
```
产物位于 `android/app/build/outputs/apk/debug/app-debug.apk`。

---

## GitHub Actions 自动构建与发版

仓库已配置 [.github/workflows/ci-release.yml](.github/workflows/ci-release.yml)：
1. **每次推送到 `main` 或发起 PR**：自动运行 Go 后端单测、构建 Linux `amd64` 部署包 `growth-buddy-go-aliyun.tar.gz`，并运行安卓跳绳单测与编译 APK，产物保存在对应 Workflow Run 的 **Artifacts** 中可直接下载。
2. **推送版本标签（例如 `v2.2.0`）自动创建 GitHub Release**：
   ```bash
   # 先在 android/app/build.gradle.kts 中递增 versionCode 和 versionName
   git add .
   git commit -m "release: v2.2.0"
   git tag v2.2.0
   git push origin main --tags
   ```
   GitHub Actions 会自动编译并在 **Releases** 页面挂载 `growth-buddy-go-aliyun.tar.gz`、`GrowthBuddyChild-v2.2.0.apk` 与 `SHA256SUMS.txt`。
3. **也可在 GitHub 网页端手动触发**：进入仓库 **Actions → Build & Release → Run workflow**。

---

## 阿里云 / 1Panel 部署与 App 远程升级

1. 下载 Release 中的 `growth-buddy-go-aliyun.tar.gz` 到服务器目录并解压执行 `./deploy-aliyun.sh`（或在服务器直接 `git clone` 后进入 `server/` 运行 `./deploy-aliyun.sh`）。
2. 在 1Panel 站点反向代理配置中设置 `client_max_body_size 160M;`（详见 `server/nginx.conf.example`）。
3. 浏览器打开 `http://你的域名/admin.html`（默认管理员 `admin` / `admin1234`，请及时修改密码），在 **「📱 App 远程升级发布」** 上传新版 APK，孩子平板启动 App 即可自动检测并在线更新。
