# 2026-06-28 main-preview 18080 蓝绿测试启动结果

## 执行结果

已用本机当前 `main` 工作树构建并启动隔离的 `sub2api-main-preview` 栈：

- 工作树：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- HEAD：`eb95da02c fix: 统一订阅缺失时的计费兜底路径`
- 镜像：`sub2api-main-preview:20260628-173235-eb95da02c`
- 镜像 ID：`sha256:9fc233e0c7a38647293d1c1eb3719529080eaa5996da2e7101ec291dd0b59745`
- 辅助 tag：`sub2api-main-preview:codex-main`

构建首次失败原因是直接调用 Docker 二进制时 PATH 缺少 `docker-credential-osxkeychain`；确认 helper 位于 `/Applications/Docker.app/Contents/Resources/bin/` 后，用包含该目录的 PATH 重新构建成功。

## 启动方式

- 启动既有 `sub2api-main-preview-postgres`。
- 启动既有 `sub2api-main-preview-redis`。
- 删除停止中的旧 `sub2api-main-preview` 应用容器，仅用于把端口从旧的 `18082` 改为 `18080`。
- 使用新镜像重建 `sub2api-main-preview`，保留：
  - 网络：`sub2api-main-preview-net`
  - 数据卷：`sub2api-main-preview-data`
  - DB 数据卷：`sub2api-main-preview-pgdata`
  - Redis 数据卷：`sub2api-main-preview-redisdata`
- 新端口：`127.0.0.1:18080->8080`

## 当前状态

```text
sub2api-main-preview            sub2api-main-preview:20260628-173235-eb95da02c       Up (healthy)   127.0.0.1:18080->8080/tcp
sub2api-main-preview-postgres   postgres:18-alpine                                   Up             5432/tcp
sub2api-main-preview-redis      redis:8-alpine                                       Up             6379/tcp
```

公网候选栈保持不变：

```text
sub2api-candidate               sub2api-candidate:20260627-221441-traffic-card-fix   Up (healthy)   127.0.0.1:18084->8080/tcp
sub2api-candidate-postgres      postgres:18-alpine                                   Up (healthy)   5432/tcp
sub2api-candidate-redis         redis:8-alpine                                       Up (healthy)   6379/tcp
```

`sub2api-candidate` 启动时间仍为 `2026-06-27T13:25:13Z`，本次没有停止、删除、重建或重命名 `sub2api-candidate*`，也没有修改 nginx、Cloudflare Tunnel 或公网 `8080 -> 18084` 链路。

## 验证

- `curl -fsS http://127.0.0.1:18080/health` 返回 `{"status":"ok"}`。
- `curl -fsS http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。
- `curl -fsS http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。
- `http://127.0.0.1:18080/purchase` 返回 `200 text/html; charset=utf-8`。
- 18080 前端资源指纹：
  - `assets/index-BO7oQkXG.js`
  - `assets/index-nffSQZgD.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-vue-BqGtxt06.js`
- 18080 公开 settings 非敏感状态：
  - `registration_enabled=false`
  - `email_verify_enabled=false`
  - `password_reset_enabled=false`
  - `purchase_subscription_enabled=false`
  - `promo_code_enabled=true`
  - `site_name=天才程序员小站`
  - `api_base_url=""`
- 18080 预览库 `schema_migrations` 数量为 `192`，最新应用迁移为 `156_seed_codex_79_subscription_plan.sql`。

## 结论

本地蓝绿测试栈已在 `http://127.0.0.1:18080` 启动，连接独立的 `sub2api-main-preview-postgres` 和 `sub2api-main-preview-redis`。公网 18084 候选链路未被触碰，仍保持 healthy。
