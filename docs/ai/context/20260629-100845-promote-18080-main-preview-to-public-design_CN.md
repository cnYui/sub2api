# 18080 main-preview 切换为公网入口设计

## 背景

当前公网入口仍由 18084 候选栈承接：

```text
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317
```

18080 `sub2api-main-preview` 已完成蓝绿测试，运行本地 `main` 分支合并后的新版前后端。新版包含新增数据库迁移，18080 预览库已到 194 个 migration，18084 公网候选库仍为 191 个 migration。

用户确认当前没有真实用户使用，可以接受短暂公网停写窗口，并确认以 18084 公网数据库作为事实源覆盖 18080 预览库中的测试数据。

## 当前只读观测

运行容器：

- `sub2api-candidate`：`127.0.0.1:18084->8080`，公网候选应用。
- `sub2api-candidate-postgres`：18084 候选数据库。
- `sub2api-candidate-redis`：18084 候选 Redis。
- `sub2api-main-preview`：`127.0.0.1:18080->8080`，本地 main-preview 应用。
- `sub2api-main-preview-postgres`：18080 预览数据库。
- `sub2api-main-preview-redis`：18080 预览 Redis。

数据差异：

- 18084 候选库：191 migrations，50 users，41 keys，57 payment orders，342 traffic ledger。
- 18080 预览库：194 migrations，49 users，41 keys，59 payment orders，282 traffic ledger。
- 18080 镜像标签为 `sub2api-main-preview:20260629-092226-ddd4fb9a9`；当前本地 `main` HEAD 为 `d42b93695`，相对该镜像只多了文档和 AGENTS 记录，不影响运行代码。

结论：18080 预览库不能直接作为公网事实源；必须先用 18084 公网库做最终同步，再让 18080 新版应用对同步后的数据执行 156-158 等新增迁移。

## 目标

- 将公网入口从 18084 切换到 18080。
- 使用 18084 公网库作为唯一事实源，覆盖 18080 预览库。
- 让 18080 新版应用在恢复后的真实数据上自动执行缺失迁移。
- 切换后公网域名仍走现有 Cloudflare Tunnel 和 nginx 8080，只修改 nginx upstream 目标端口。
- 保留可回滚路径，避免误删 volume、误提交 dump、误泄露密钥。

## 非目标

- 不把 18080 当前测试库中的额外订单或 ledger 合并回公网事实源。
- 不修改 CLIProxyAPI、Cloudflare Tunnel、上游账号池和用户 API Key。
- 不重建或替换 18084 候选数据库 volume。
- 不在文档、日志摘要、提交中记录完整 API Key、SMTP 密码、HMAC secret、内部 token 或 dump 内容。

## 推荐方案

采用“最终停写后重新克隆”的切换方式：

1. 在执行窗口开始前，分别备份 18084 候选库和 18080 预览库。
2. 对公网进行临时停写：停止 `sub2api-candidate` 应用容器，保留 `sub2api-candidate-postgres` 和 `sub2api-candidate-redis` 运行。
3. 停写后重新从 `sub2api-candidate-postgres` 导出最终 dump，确保不会丢失停写前最后一笔写入。
4. 用最终 dump 覆盖恢复到 `sub2api-main-preview-postgres`。
5. 清空 `sub2api-main-preview-redis`，重启 `sub2api-main-preview`，让新版应用自动执行缺失迁移到 194。
6. 在 18080 本地端口完成健康检查、登录、购买页、API 路由、SMTP/payment 非敏感配置和真实模型请求验证。
7. 修改 nginx，将 Sub2API upstream 从 `127.0.0.1:18084` 切换到 `127.0.0.1:18080`，reload nginx。
8. 验证 `127.0.0.1:8080`、`https://api.aaccx.pw`、`https://aaccx.pw` 的关键路径。
9. 保留 18084 候选 DB/Redis 和旧应用镜像作为短期回滚资产；观察稳定后再按用户确认停止旧 18084 DB/Redis。

## 停写语义

本次“临时停写”定义为停止 18084 的公网应用容器，而不是停止 18084 数据库：

- 停止：`sub2api-candidate`。
- 保留运行：`sub2api-candidate-postgres`、`sub2api-candidate-redis`。
- 原因：停止应用可以阻断公网写入；保留数据库运行才能做最终一致 dump。
- 影响：停写窗口内公网 API 和控制台会不可用，直到 nginx 切到 18080 并验证通过。

如果需要更优雅的用户体验，可以在 nginx 临时返回维护页，但当前用户已确认无人使用，最小方案是直接停止 18084 应用。

## 数据流

```text
停写前:
18084 candidate DB -> 公网事实源
18080 preview DB   -> 可覆盖测试库

执行窗口:
stop sub2api-candidate
18084 candidate DB --pg_dump--> backup dump --pg_restore--> 18080 preview DB
18080 main-preview app --auto migrate--> 194 migrations

切换后:
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-main-preview 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317
```

## 验证清单

切换前验证：

- `docker ps` 中仅确认目标 6 个容器状态。
- 18084 和 18080 的 `schema_migrations` 数量、最新 filename、关键表数据量。
- 18084 候选库 dump 文件存在且不提交。
- 18080 预览库备份文件存在且不提交。

恢复后、切 nginx 前验证：

- `http://127.0.0.1:18080/health` 返回 200。
- 18080 库 migration 数为 194，最新包含 `158_enable_affiliate_default.sql`。
- 关键表数量与 18084 停写后 dump 基本一致；允许迁移 seed 表导致套餐配置变化。
- 登录管理员账号和普通测试账号可用。
- `/purchase` 可访问，支付方式非敏感公开配置符合预期。
- `POST /api/v1/auth/send-verify-code` 仅在需要时验证，日志只确认成功或脱敏错误。
- 使用可控 API Key 请求 `/v1/responses` 或 `/v1/chat/completions` 返回 200。

切 nginx 后验证：

- `http://127.0.0.1:8080/health` 返回 200。
- `https://api.aaccx.pw/health` 返回 200。
- `https://aaccx.pw/purchase`、`/dashboard`、`/usage-guide` 返回 200。
- 裸 `/responses`、`/chat/completions`、`/embeddings`、`/images/*` 仍代理到 Sub2API。
- 真实 LLM 请求走 `Sub2API -> CLIProxyAPI -> 上游 OpenAI` 成功。

## 回滚方案

在 nginx reload 后如果发现 18080 不健康或关键业务失败：

1. 将 nginx upstream 改回 `127.0.0.1:18084`。
2. 启动或恢复 `sub2api-candidate` 应用容器。
3. reload nginx。
4. 验证 `18084/health`、`8080/health`、公网 health 和一个认证失败路径。

如果 18080 数据库恢复或迁移失败：

1. 不修改 nginx，公网仍停在 18084 或保持停写状态。
2. 使用执行前的 18080 预览库备份恢复 preview。
3. 启动 `sub2api-candidate` 并恢复 nginx 到 18084。
4. 重新排查迁移失败原因，不做带病切换。

## 风险与控制

- 风险：停写后恢复耗时导致公网不可用时间变长。
  - 控制：先备份、确认命令和目标容器名，再进入停写窗口。
- 风险：把 18080 测试库误当事实源。
  - 控制：最终 dump 只从 `sub2api-candidate-postgres` 读取，恢复目标只允许是 `sub2api-main-preview-postgres`。
- 风险：迁移执行后旧 18084 应用无法读取新版库。
  - 控制：不迁移 18084 候选库，只迁移 18080 恢复后的库。
- 风险：敏感 dump 或运行态密钥进入 git。
  - 控制：dump 放 `deploy/backups/`，执行前后检查 `git status --short`，不打印 env 文件内容。
- 风险：Redis 中旧会话或缓存污染切换结果。
  - 控制：恢复 18080 DB 后清空 18080 Redis，再启动 18080 应用。

## 审批点

执行前需要用户确认：

1. 允许停写窗口内停止 `sub2api-candidate` 应用容器。
2. 允许用 18084 候选库最终 dump 覆盖 18080 预览库。
3. nginx 切换目标为 `127.0.0.1:18080`。
4. 切换成功后先保留 18084 DB/Redis 作为回滚资产；是否立即停止旧 18084 DB/Redis 留到验证完成后再确认。
