# 成长小伙伴 · 孩子端纯原生安卓 App (`GrowthBuddyChild` v2.0)

配合阿里云后端（`growth-buddy-server`）使用的 **100% 纯安卓原生（Kotlin）** 孩子端 App（彻底移除 WebView）：
- **儿童独立账号密码直登**：孩子只需输入自己的账号和密码即可登录打卡，无需记住家庭码（默认服务器地址 `http://linuu.xyz`，支持界面随时切换）。
- **60fps 原生 Canvas 糖霜宠物**：完整实现主宠成长舞台、抚摸互动、食物飞入/庆祝彩纸粒子、领养图鉴（神秘蛋/伴宠）与拍立得纪念墙，无 WebView 卡顿。
- **零延迟高响度声效引擎 (`NativeSoundEngine`)**：启动时预合成 44.1kHz 高响度（峰值 0.94）WAV 音效载入 `SoundPool(USAGE_GAME)`（<20ms 响度清晰），搭配满音量快节奏 `TextToSpeech` 与低音量一键拉升按钮。
- **天天跳绳式防漏计 AI 跳绳 (`RopeJumpDetector`)**：CameraX 低分辨率高帧率推理（360×480）+ ML Kit Pose Detection，采用肩髋多点融合抗遮挡（62% 肩 + 38% 髋 + 鼻部短遮挡兜底）、轻量保峰平滑与动态波峰波谷反转状态机，支持 150–200+/分钟高速连跳与小幅起跳不漏计，并支持前后镜头切换。

## 目录结构

```text
growth-buddy-android/
├── settings.gradle.kts
├── build.gradle.kts
└── app/
    ├── build.gradle.kts                     # CameraX + ML Kit Pose Detection (arm64-v8a / armeabi-v7a)
    └── src/main/
        ├── AndroidManifest.xml              # INTERNET & CAMERA 权限
        ├── res/layout/
        │   ├── activity_main.xml            # 纯原生登录 / 主舞台 / 任务 / 图鉴 / 纪念墙
        │   └── activity_rope_jump.xml       # 摄像头 HUD + 跳速 + 倒计时 + 音量/镜头按钮
        └── java/com/growthbuddy/child/
            ├── MainActivity.kt              # 纯原生主界面控制器
            ├── ApiClient.kt                 # 原生 HTTP JSON 客户端
            ├── ChildModels.kt               # 数据模型与成长阶段元数据
            ├── PetCanvasView.kt             # 60fps Candy-Glaze 糖霜宠物与图鉴 Canvas 绘制
            ├── NativeSoundEngine.kt         # SoundPool 预合成 PCM WAV + 满音量 TTS + 音量提升
            ├── RopeJumpActivity.kt          # CameraX 高帧率采集 + 3-2-1 倒计时 + 实时跳绳反馈
            ├── RopeJumpDetector.kt          # 多点融合 + 动态波峰波谷反转计数状态机
            ├── PoseOverlayView.kt           # 站位绿框与骨骼线实时叠加
            └── ServerPrefs.kt               # 服务器地址、儿童登录 Token 与账号持久化
```

## 本地编译与测试

```bash
./gradlew testDebugUnitTest assembleDebug
```

产物位于 `app/build/outputs/apk/debug/app-debug.apk`（仅打包 `arm64-v8a` / `armeabi-v7a`，真机安装体积精简约一半）。
