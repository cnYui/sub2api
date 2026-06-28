# 18080 main-preview 重启结果

## 结论

- 已将本地 `18080` 的 `sub2api-main-preview` 应用容器切换到当前本机 `main` HEAD `399f278f3 merge: 合并注册重复邮箱预检拦截`。
- 新镜像：`sub2api-main-preview:20260628-205543-399f278f3`
- 镜像 digest：`sha256:46b73dddfcd4f916b54dfbcb584f5f665e084ffb3759d9b4d607b3d73e5002f9`
- 端口保持：`127.0.0.1:18080->8080`
- 保留并继续使用：`sub2api-main-preview-postgres`、`sub2api-main-preview-redis`、`sub2api-main-preview-data:/app/data`
- 18084 公网候选链路未停止、未重建、未写入；`sub2api-candidate` 仍为 `sub2api-candidate:20260627-221441-traffic-card-fix`，启动时间仍为 `2026-06-27T13:25:13.194989879Z`。

## 执行记录

- 替换前备份 18080 preview DB：
  - `deploy/backups/20260628-205543-18080-preview-before-399f278f-restart.dump`
  - 文件权限：`600`
- 只替换应用容器：
  - 停止并删除旧 `sub2api-main-preview`
  - 使用新镜像重建同名容器
  - 未停止、删除或重建 preview PostgreSQL/Redis
- 启动后已删除临时 env 文件：
  - `/tmp/sub2api-main-preview-env-20260628-205543`

## 构建修复

- Docker build 首次失败在前端 `pnpm install --frozen-lockfile`：lockfile 配置与当前 pnpm 安装上下文不一致。
- 根因收敛：
  - Dockerfile 原本只复制 `frontend/package.json` 与 `frontend/pnpm-lock.yaml`，没有复制 `frontend/.npmrc`。
  - 本地 pnpm v11 不再读取 `package.json#pnpm.overrides`，但 Docker 构建仍使用 `pnpm@9.15.9`，需要兼容旧配置。
- 已做最小修复：
  - `.dockerignore` 增加 `deploy/backups/`，避免 DB dump 进入 Docker build context。
  - Dockerfile 在前端依赖安装层复制 `frontend/.npmrc`。
  - 新增 `frontend/pnpm-workspace.yaml`，给本地 pnpm v11 提供 `allowBuilds`、`overrides` 与已知安全构建脚本配置。
  - 保留 `frontend/package.json#pnpm.overrides`，兼容 Docker 构建中的 `pnpm@9.15.9`。

## 验证

- `go test ./migrations ./internal/repository -run 'TestMigration15(6|7)|TestIsMigrationChecksumCompatible|TestAuthIdentityPaymentMigrationsRegression' -count=1`：通过
- `go test ./internal/service ./internal/server -run 'Register|Email|Precheck|PreCheck' -count=1`：通过
- `pnpm install`：通过，本地 pnpm v11 仍提示 `package.json#pnpm` 被忽略，这是为 Docker pnpm9 兼容保留的旧字段。
- `pnpm vitest run src/views/auth/__tests__/RegisterView.spec.ts`：3 tests 通过
- Docker build：通过，前端 `vue-tsc -b && vite build` 与后端 release build 均完成。
- `http://127.0.0.1:18080/health`：`{"status":"ok"}`
- `http://127.0.0.1:18084/health`：`{"status":"ok"}`
- `http://127.0.0.1:8080/health`：`{"status":"ok"}`
- 18080 前端资源：
  - `assets/index-BVRJ38ir.js`
  - `assets/index-nffSQZgD.css`
- 18080 `schema_migrations`：194 条，最新：
  - `158_enable_affiliate_default.sql`
  - `157_fix_codex_79_subscription_plan_base_price.sql`
  - `156_seed_codex_79_subscription_plan.sql`
- 18084 `schema_migrations` 只读确认仍为 191 条，最新：
  - `155_seed_codex_subscription_plans_baseline.sql`
  - `154_seed_codex_99_subscription_plan.sql`
  - `153_scheduler_outbox_pending_dedup_key_index_notx.sql`

## 注意

- `deploy/backups/20260628-205543-18080-preview-before-399f278f-restart.dump` 含运行态数据，不能提交。
- 本轮未提交代码；新增/修改的 pnpm/Docker 构建兼容改动需要后续纳入提交或按用户要求处理。
