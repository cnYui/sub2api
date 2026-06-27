# Sub2API 本地重部署失败与恢复结果

## 当前结论

- 本次按 `sub2api-local-redeploy` 执行的构建本身完成，产物镜像曾生成于 2026-06-26 15:54 JST。
- 该新镜像启动后健康检查失败，发布脚本最终以退出码 1 结束。
- 失败直接原因是生产 DB 的 `schema_migrations` 中 `155_seed_codex_subscription_plans_baseline.sql` checksum 与新镜像内迁移文件 checksum 不一致，应用启动时拒绝继续。
- 当前公网运行态已经恢复到旧镜像：`weishaw/sub2api:latest` 指向 2026-06-21 创建的镜像 `sha256:423e0593979c421012eb6a3e524c87884f5bd009011cf8287b91a1f7df693c81`。
- 当前 `sub2api` 容器为 `healthy`，绑定 `127.0.0.1:18080->8080/tcp`，日志中已有 `/v1/responses` 200 请求记录。

## 关键证据

- 新构建镜像仍留在本地为无标签镜像：`sha256:f668024ff11979396d6d378e2fbd1eb7dee1dd109a9f7d8b00ee81ff368d7c76`，创建时间 2026-06-26 15:54:24 JST。
- 当前运行容器使用镜像：`sha256:423e0593979c421012eb6a3e524c87884f5bd009011cf8287b91a1f7df693c81`，创建时间 2026-06-21 11:43:47 JST。
- 当前生产 DB 只读查询到：
  - `schema_migrations.filename = 155_seed_codex_subscription_plans_baseline.sql`
  - `checksum = 0e2d20c620783bf91cbf5ffb524edb46730e998a10799f66dd03e988a32b0b8f`
- 当前本地 `main` 上该迁移文件仅有一次 git 历史提交：`fadfe9c7f feat: seed codex subscription baseline plans`。
- `backend/internal/repository/migrations_runner.go` 的 checksum 兼容白名单未包含 `155_seed_codex_subscription_plans_baseline.sql`。

## 根因判断

生产 DB 曾经应用过一个旧版本的 `155_seed_codex_subscription_plans_baseline.sql`，并记录 checksum `0e2d20c6...`。当前本地 `main` 的 155 迁移内容与这条记录不一致，且该迁移没有兼容白名单，所以新镜像启动时触发迁移不可变性保护并退出。

这不是 Docker build 失败，也不是 nginx、Cloudflare Tunnel、CLIProxyAPI 或 API Key 配置问题。

## 影响

- 新镜像未成功成为当前运行版本。
- 当前公网 API 已由旧镜像提供服务；不要在高峰期再次直接执行重部署，否则仍可能复现同类启动失败。
- 只读排查期间未重启、停止或重建容器，也未向 `https://api.aaccx.pw/v1` 发送验证请求。

## 后续建议

优先在非公网路径修复迁移一致性问题，再重新部署：

1. 明确 155 迁移的旧版本内容来源，确认生产 DB 真实应用了哪些种子数据。
2. 不要直接修改已应用迁移文件；新增迁移补齐差异，或在代码中为 155 加入明确的 checksum 兼容规则。
3. 在 preview/本地 existing DB 复现生产 checksum 场景，确认新镜像可启动后，再执行公网重部署。
