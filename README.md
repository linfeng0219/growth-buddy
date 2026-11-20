# 成长小伙伴（Growth Buddy）

面向多家庭习惯养成与运动打卡的三端协同平台：
- **系统管理端 Web（`/admin.html`）**：管理全平台家庭、家长与儿童账号，并内置 **App 远程升级（OTA）版本发布台**。
- **家长端 Web（`/parent.html`）**：管理家庭多孩档案、配置孩子独立登录账号密码、进入**任务中心**（当日任务/新增任务/所有任务，支持单次与周一至周日循环、正/倒计时、是否拍照、内置与私有图标库）、查看实拍照片墙与按天/周/月/年汇总打卡记录、配置冒险小剧场题库。
- **儿童端纯原生安卓 App（`android/`）**：零 WebView 纯 Kotlin 原生应用，包含 60fps 糖霜宠物舞台（含缺卡虚弱/生病状态与恢复进度）、低延迟 `SoundPool` 音效与 TTS 语音、沉浸式番茄专注页（`FocusActivity`）、独立冒险小剧场答题闯关（`AdventureActivity`）、「天天跳绳」式肩髋多点融合 AI 跳绳计数，以及 **OTA 远程在线升级**；主宠成熟（200 成长值）后富余成长值每 25 点可直接兑换图鉴伙伴。

---

## 目录结构

```text
growth-buddy/
├── .github/workflows/ci-release.yml   # GitHub Actions 自动测试、编译服务端二进制与 Release 发布脚本
├── server/                            # Go + SQLite 零外部依赖三端后端（内嵌 Web 静态资源）
│   ├── main.go / server.go / store.go
│   ├── quiz.go / health_icons.go      # 一年级冒险小剧场题库 + 宠物健康结算与家庭私有图标库
│   ├── sqlite.go / auth.go / scrypt.go
│   ├── server_test.go                 # 集成测试（含密码兼容与 OTA 生命周期测试）
│   ├── public/                        # 首页 / 管理端 / 家长端 / 儿童端 Web 页面
│   ├── Dockerfile                     # 多阶段 Go + SQLite 构建镜像
│   ├── docker-compose.yml
│   ├── nginx.conf.example             # 1Panel / OpenResty 反代配置（160M APK 上传限制）
│   └── deploy-aliyun.sh               # 阿里云一键部署脚本
└── android/                           # 纯原生安卓儿童端工程（Kotlin + CameraX + ML Kit）
    └── app/src/main/java/com/growthbuddy/child/
        ├── MainActivity.kt            # 纯原生登录 / 宠物舞台 / 任务 / 图鉴 / 纪念墙 / 头像编辑
        ├── FocusActivity.kt           # 沉浸式番茄专注页（自由专注 & 任务正/倒计时）
        ├── AdventureActivity.kt       # 独立冒险小剧场（一年级题库答题充能击败拖延怪）
        ├── PetCanvasView.kt           # 60fps Candy-Glaze 糖霜宠物、健康状态与小剧场舞台绘制
        ├── NativeSoundEngine.kt       # 预合成高响度 SoundPool WAV + 满音量 TTS（含中文回退）
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
# 或仅打 64 位瘦身 Release 包（约 13.6 MB）：
gradle assembleRelease -Parm64Only=true
```
产物位于 `android/app/build/outputs/apk/debug/app-debug.apk`（或 `release/app-release.apk`）。

---

## GitHub Actions 自动构建与发版

仓库已配置 [.github/workflows/ci-release.yml](.github/workflows/ci-release.yml)：
1. **每次推送到 `main` 或发起 PR**：自动在 `ubuntu-22.04` 环境运行 Go 后端单测、构建 Linux `amd64` 单二进制 `growth-buddy-server` 及部署包 `growth-buddy-go-aliyun.tar.gz`，产物保存在对应 Workflow Run 的 **Artifacts** 中可直接下载。
2. **推送版本标签（例如 `v2.4.0`）自动创建 GitHub Release**：
   ```bash
   git tag v2.4.0
   git push origin main --tags
   ```
   GitHub Actions 会自动编译并在 **Releases** 页面挂载 `growth-buddy-go-aliyun.tar.gz` 与 `SHA256SUMS.txt`。
3. **也可在 GitHub 网页端手动触发**：进入仓库 **Actions → Build & Release → Run workflow**。

---

## 最近更新记录

- **2026-11-20**：
  - 修复 `health_icons.go` 中调用 `parseDataURLImage` 名称笔误导致的 CI 编译报错。
  - 宠物健康机制：1 天未打卡变虚弱（需 10 成长值恢复），2 天及以上未打卡变生病（需 20 成长值恢复）。
  - 任务中心升级：支持「当日任务 / 新增任务 / 所有任务」Tab 切换、单次或周一至周日循环、正/倒计时番茄专注与是否拍照留证；内置生活/运动/学习/兴趣 4 类图标模板，并支持上传图片到家庭私有图标库。
  - 打卡统计与兑换：保留实拍照片墙，新增按天/周/月打卡明细与按天/周/月/年任务汇总；取消领养蛋人工审核，主宠成熟（200 成长值）后富余成长值每 25 点直接兑换伙伴。
  - 儿童端功能：修复点击宠物朗读与头像编辑，新增沉浸式番茄专注页（`FocusActivity`）与独立冒险小剧场答题闯关（`AdventureActivity`，内置一年级语/数/科/趣味题库）。

---

## 阿里云 / 1Panel 部署与 App 远程升级

1. 下载 Release 中的 `growth-buddy-go-aliyun.tar.gz` 到服务器目录并解压执行 `./deploy-aliyun.sh`（或在服务器直接 `git clone` 后进入 `server/` 运行 `./deploy-aliyun.sh`）。
2. 在 1Panel 站点反向代理配置中设置 `client_max_body_size 160M;`（详见 `server/nginx.conf.example`）。
3. 浏览器打开 `http://你的域名/admin.html`（默认管理员 `admin` / `admin1234`，请及时修改密码），在 **「📱 App 远程升级发布」** 上传新版 APK，孩子平板启动 App 即可自动检测并在线更新。
