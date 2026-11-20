# 成长小伙伴 · Go + SQLite 后端 (`growth-buddy-go`)

零第三方 Go 依赖的单二进制后端服务（内嵌 SQLite 静态库与全部三端 Web 页面），100% 兼容原有 `growth_buddy.db` 数据库文件、密码哈希（`scrypt`）与所有三端 API，并新增 **安卓儿童端 OTA 远程在线升级** 管理。

## 核心亮点
- **单文件极简部署**：预编译二进制 `growth-buddy-server` 仅 **8.4 MB**，内嵌 SQLite 与 `public/` 前端静态资源，待机内存 ~15 MB。
- **100% 无损平滑升级**：直接复用阿里云现有的 `./data/growth_buddy.db` 和 `.env`，家长/儿童/管理员账号密码与打卡数据零丢失。
- **三端架构**：
  - 系统管理端 Web：`/admin.html`（默认 `admin` / `admin1234`，管理所有家庭、家长、儿童及 App 版本发布）
  - 家长端 Web：`/parent.html`（创建家庭与管理多孩任务、账号密码、领养蛋）
  - 儿童端原生 App：账号密码直登、低延迟高响度音效、天天跳绳式防漏计 AI 跳绳、自动检测 OTA 远程升级
- **App 远程在线升级（OTA）**：
  - 管理员在 `/admin.html` → **「📱 App 远程升级发布」** 上传新版 `.apk`（自动计算 SHA-256 与文件大小，支持可选/强制升级）或填写外部直链。
  - 公开接口：`GET /api/app/version?platform=android&currentVersionCode=3`、`GET /api/app/download/{id}`、`GET /api/app/latest.apk`。

## 阿里云 / 1Panel 部署或覆盖更新

```bash
# 1. 解压覆盖到原部署目录（注意：压缩包不含 data/ 与 .env，不会覆盖现有数据库！）
tar -xzf growth-buddy-go-aliyun.tar.gz -C /你的部署目录

# 2. 一键启动或重建（支持 Docker Compose 或原生 systemd）
cd /你的部署目录
chmod +x deploy-aliyun.sh growth-buddy-server
./deploy-aliyun.sh
```

> **1Panel 反向代理提示**：请在站点反向代理配置中将 `client_max_body_size` 设为 `160M`（参考 `nginx.conf.example`），即可直接在 `/admin.html` 浏览器上传 50 MB 级别的安卓 APK 安装包。

## 本地开发与测试

```bash
go test -v ./...
CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic -lsqlite3 -Wl,-Bdynamic -lm -ldl -lpthread" \
  go build -trimpath -ldflags="-s -w" -o growth-buddy-server .
```
