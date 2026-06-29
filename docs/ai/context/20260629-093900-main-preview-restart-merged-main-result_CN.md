# 2026-06-29 18080 main-preview 重启到合并后 main 结果

## 目标

- 将本地蓝绿测试环境 `127.0.0.1:18080` 的 `sub2api-main-preview` 更新到合并后的本地 `main` 最新代码。
- 只替换 preview 应用容器，保留 `sub2api-main-preview-postgres`、`sub2api-main-preview-redis` 与 volume。
- 严格不触碰公网候选链路 `18084` 和 nginx `8080`。

## 输入状态

- 工作目录：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- HEAD：`ddd4fb9a9 fix: remove duplicate usage guide locale keys`
- 旧 preview 镜像：`sub2api-main-preview:20260628-205543-399f278f3`
- 旧 preview 端口：`127.0.0.1:18080->8080`
- 公网候选应用：`sub2api-candidate:20260627-221441-traffic-card-fix`
- 备份文件：`deploy/backups/20260629-092226-18080-preview-before-merged-main-restart.dump`，权限 `600`，不提交。

## 执行

1. 使用 `main@ddd4fb9a9` 构建镜像：
   - `sub2api-main-preview:20260629-092226-ddd4fb9a9`
   - `sub2api-main-preview:codex-main`
2. Docker build 成功完成；前端 `vue-tsc` 与 Vite build 通过，后端 Go 二进制构建通过。
3. 从旧 `sub2api-main-preview` 提取 env 到临时文件，未打印 env 内容。
4. 只停止并删除旧应用容器 `sub2api-main-preview`。
5. 使用新镜像重建应用容器，沿用：
   - 网络：`sub2api-main-preview-net`
   - 端口：`127.0.0.1:18080:8080`
   - volume：`sub2api-main-preview-data:/app/data`
   - env：旧容器提取的临时 env 文件

## 验证结果

- 新 preview 镜像 digest：`sha256:1cc424ad95735188f6c707c5c1b19af42b670a2c332c863ef8e8a9cc4ddb563b`
- `sub2api-main-preview`：`Up` 且 `healthy`
- `18080/health`：`{"status":"ok"}`
- `18084/health`：`{"status":"ok"}`
- `8080/health`：`{"status":"ok"}`
- 18080 前端资源：
  - JS：`/assets/index-DHHTg5y4.js`
  - CSS：`/assets/index-B--asykV.css`
- 18080 preview DB：
  - `schema_migrations` 数量：`194`
  - 最新迁移：`158_enable_affiliate_default.sql`
  - 后续最新文件：`157_fix_codex_79_subscription_plan_base_price.sql`、`156_seed_codex_79_subscription_plan.sql`
- 启动日志显示服务已监听 `0.0.0.0:8080`，未见迁移失败或启动失败。

## 18084 边界确认

- `sub2api-candidate` 未被替换或重启。
- 替换前后候选容器 image/start time 比对一致。
- 当前候选应用仍为：
  - image：`sub2api-candidate:20260627-221441-traffic-card-fix`
  - image id：`sha256:299560875687ba0fc7c9b9703a5bece639a832c35720fb6ce47f8dd222483e22`
  - started_at：`2026-06-27T13:25:13.194989879Z`
- `sub2api-candidate-postgres`、`sub2api-candidate-redis` 与 nginx `8080` 未执行停止、删除、重建或配置修改。

## 备注

- 本轮没有执行默认公网替换脚本 `deploy/redeploy-sub2api-image.sh`。
- 本轮临时 env 文件仅用于重建 preview 应用容器，未写入文档或 git。
- 本轮敏感 preview DB dump 位于 `deploy/backups/`，只用于回滚，不提交。
