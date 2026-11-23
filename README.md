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
产物位于 `android/app/build/outputs/apk/release/GrowthBuddyChild-<编译时间>-release.apk`（Debug 版为 `GrowthBuddyChild-<编译时间>-debug.apk`）。

---

## GitHub Actions 自动构建与发版

仓库已配置 [.github/workflows/ci-release.yml](.github/workflows/ci-release.yml)：
1. **每次推送到 `main`**：自动在 `ubuntu-22.04` 环境运行 Go 后端单测并编译 Linux `amd64` 单二进制，同时自动更新 GitHub **Releases → `latest`**，直接挂载未压缩的 `growth-buddy-server-YYYY.MM.DD-HHMM` 与 `growth-buddy-server`（网页点击即可下载原文件，无需解压可直接拖入服务器；提示：GitHub Workflow `Artifacts` 页面由平台强制打包 `.zip`，请从仓库右侧 **Releases** 下载原文件）。
2. **推送版本标签（例如 `v2.4.0`）自动创建正式版本 Release**：
   ```bash
   git tag v2.4.0
   git push origin main --tags
   ```
   GitHub Actions 会自动编译并在 **Releases** 页面挂载未压缩二进制 `growth-buddy-server-*`、`growth-buddy-go-aliyun.tar.gz` 与 `SHA256SUMS.txt`。
3. **也可在 GitHub 网页端手动触发**：进入仓库 **Actions → Build & Release → Run workflow**。

---

## 最近更新记录

- **2026-11-23**：
  - 精简安卓儿童端顶栏与音量交互：移除儿童主界面顶部「更新」按钮（保留启动静默检测升级）以及主界面/跳绳页「🔊 音量+」调节入口，仅保留「🔊 / 🔇」静音切换开关。
  - 精简统一家长端头部布局：移除家长端头部横幅说明与客户端下载入口，调整头像栏、标题及状态胶囊排版，并与孩子端及 App 内置家长页保持单一顶栏一致风格。
  - 安卓端支持「家长 / 孩子」双端统一登录与账号记忆下拉切换：新增统一识别登录接口 `/api/auth/login`，在 App 登录时自动识别家长或孩子身份，分别进入内置「家长管理台」（`ParentPortalActivity`）或儿童端主界面；登录页自动记住已登录过的多组账号、头像与角色，支持直接点击下拉窗选号填充，一键切换身份。
  - 群作业纯文本批量智能导入：家长端「任务中心 → 群作业导入」支持直接粘贴老师微信/QQ 群里的整段作业消息，自动切分条目、过滤通知寒暄、识别学科图标、套用书写拍照+正计时 / 朗读倒计时 / 跳绳参数，一键批量生成今日或明日单次打卡任务。
  - 家长信箱双向写信互动：新增家庭信箱（`family_letters`），家长端可挑选邮票给孩子写表扬鼓励信（可选附带 2~10 点拆信礼物成长值），安卓儿童端主页新增「📬 家长信箱」新信提醒，支持拆信领礼物以及选贴纸写回信给爸妈。
  - 升级内置离线语音为萌娃拟真感情音色：使用 `zh-CN-YunxiaNeural` 童声音色重制并扩展 42 段离线 MP3 语音包（覆盖宠物摸头/各进化阶段、睡着/虚弱/生病情绪语调、冒险胜利/进化/领养祝贺，以及跳绳 3-2-1 倒计时与整十报数），体积更小、感情更生动且无需依赖系统 TTS。
  - 题库升级为分龄分级在线题库：取消家长手动录入题库，后台新增覆盖 `3~6岁`、`7~8岁`、`9~10岁`、`11~14岁` 四个学段 × 简单/适中/挑战三档难度的精选在线题库 + 实时动态算术/逻辑题生成器（同时支持通过 `QUIZ_REMOTE_URL` 环境变量同步远程 JSON 题库）。
  - 儿童档案与家长端适配：家长端支持为每位孩子配置「年龄」和「默认答题难度」，并提供在线题库按年龄与难度分档预览；儿童端冒险小剧场支持按年龄/难度在线抽题并在战斗界面随时切换难度。
  - 冒险战斗胜利祝贺弹窗与音效：击败拖延怪后自动播放凯旋祝贺音效与语音，并弹出战斗胜利祝贺弹窗展示关卡战绩，支持「再来一关」或返回主页。

- **2026-11-20**：
  - GitHub CI 直接发布未压缩服务端二进制：每次推送 `main` 自动发布至 GitHub Releases `latest`（直接提供未压缩的 `growth-buddy-server-YYYY.MM.DD-HHMM` 与 `growth-buddy-server` 原文件，下载即可直接拖入服务器）。
  - 跳绳破纪录结算页升级：跳绳达标后以抓拍照片为全屏底图展示结算页，展示打卡时间、最终数量与历史最高纪录；若打破历史纪录将在四角边缘安全区燃放动态/水印礼花并高亮显示新最高纪录，所有文字与礼花避开中央站位人脸区。
  - 跳绳打卡自动抓拍上传：安卓儿童端完成 AI 跳绳挑战时，自动截取画面并叠加最终跳绳数量、时长与时间戳水印，直接随打卡记录上传至家长端照片墙。
  - 家长端新增任务智能默认值：输入书写类关键词（写、作业、练字、抄、听写、日记、口算、试卷等）时默认勾选「打卡需拍照留念」并设为「正计时」；输入时间类关键词（分钟、小时、阅读、练琴、背诵、朗读等）时默认设为「倒计时」并自动提取分钟数（如手动调整选项则保留手动设置）。
  - 内置离线中文语音包：将 20 段常用宠物对话、状态提醒与专注/跳绳/冒险语音直接内置进 APK（`assets/voices/*.wav`），不依赖手机系统 TTS 即可稳定发声。
  - 修复 Android 11+ 无法绑定系统 TTS 语音引擎的问题：在 `AndroidManifest.xml` 中补充 `<queries>` `TTS_SERVICE` 声明。
  - 优化安卓主界面布局：移除登录后头像旁版本号文本，修复宠物名字旁「✏️ 改名」按钮文字底部被裁切问题。
  - 安卓儿童端固定服务器地址为 `https://linuu.xyz`，移除登录页服务器地址编辑入口。
  - 修复管理端 OTA 上传 APK 时优先从文件名解析编译时间戳版本号（避免因文件写入耗时差 1 分钟导致页面版本号与 APK 内部版本号不一致）。
  - 修复华为浏览器等手机浏览器下载 APK 失败问题：后端 `/api/app/latest.apk` 与 `/api/app/download/` 补充支持 `HEAD` 预检请求与 `.apk` 路径后缀，移除 `no-store` 响应头以兼容系统下载管理器。
  - 统一更新日志弹窗：首页、家长端、管理端及安卓 App 均支持点击版本号查看更新内容（由 `/api/changelog` 统一提供）；优化家长端新增/编辑任务图标选择器为单框预览。
  - 支持 `docker-compose.yml` 自动识别带编译时间的后端二进制 `growth-buddy-server-YYYY.MM.DD-HHMM`（自动选取最新版本启动，无需手动重命名）。
  - 产物文件名携带编译时间：APK 自动输出 `GrowthBuddyChild-YYYY.MM.DD-HHMM-release.apk`，CI 服务端二进制同步输出 `growth-buddy-server-YYYY.MM.DD-HHMM`。
  - 统一编译时间版本号：后台服务端版本号自动取编译/更新时间（`YYYY.MM.DD-HHMM`），Android APK 版本号自动取最后编译时间，管理端 OTA 发布台不再区分 `versionCode` / `versionName`。
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
