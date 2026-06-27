# Sub2API 候选预演误影响公网事故诊断

## 背景

- 目标：新增候选预演流程，用“新镜像 + 生产 DB 克隆 + 本地候选端口”先完成本地验证，再决定是否上公网。
- 明确约束：不得影响公网 `https://api.aaccx.pw/v1`，不得替换或写入公网运行态数据库。
- 公网主容器：`sub2api`、`sub2api-postgres`、`sub2api-redis`。

## 已确认影响

- 第一次候选预演脚本执行期间，公网 `sub2api`、`sub2api-postgres`、`sub2api-redis` 被误停并重建。
- 公网应用随后恢复启动，当前 `http://127.0.0.1:18080/health` 和 `https://api.aaccx.pw/health` 均返回 200。
- 公网 Redis 被重建，导致旧 refresh token / 登录会话丢失。应用日志出现：
  - `/api/v1/auth/me` 返回 401。
  - `/api/v1/auth/refresh` 返回 401。
  - `Refresh token not found, possible reuse attack`。
- 公网 `/v1/responses` 不是全局不可用：事故后仍有有效 API key 请求返回 200；也存在某些 IP/请求连续 401。

## 数据库状态

只读检查显示公网 Postgres 没有被候选清洗 SQL 批量改坏：

- `users`：32 条。
- 未删除用户：31 条。
- 活跃用户：31 条。
- 未删除且有密码哈希用户：31 条。
- TOTP 开启用户：0 条。
- `api_keys`：22 条。
- 活跃 API key：21 条。
- `payment_provider_instances`：0 条，此状态与 2026-06-26 12:42 运行态记录一致，不是本次候选预演新造成。
- `settings` 在事故窗口后无更新。
- `schema_migrations` 当前没有 `155_seed_codex_subscription_plans_baseline.sql` 记录，最新迁移记录停在 153 段；当前公网容器未运行含 155 迁移的新镜像。

补充验证：

- 使用公网容器环境里的管理员账号密码，对本地公网后端 `http://127.0.0.1:18080/api/v1/auth/login` 做不输出 token 的登录 smoke test，HTTP 返回 200。
- 这说明当前账号密码登录能力未整体损坏；旧登录态失效是已确认影响。

## 根因

候选预演脚本第一版没有固定 Docker Compose project name：

- 候选 compose 文件位于 `deploy/` 目录。
- Docker Compose 默认 project name 取目录名，因此候选 compose 和公网 compose 都落到了 `deploy` project。
- 脚本执行 `docker compose ... down --remove-orphans` 时，Compose 按 project 清理容器，误把同 project 下的公网 `sub2api`、`sub2api-postgres`、`sub2api-redis` 一并停掉并移除。

这是候选环境隔离设计失败，不是用户操作问题。

## 为什么会出现“所有人掉登录”

- Postgres 用户表/API key 表没有整体丢失。
- 但公网 Redis 容器被误重建。当前登录体系的 refresh token 存在 Redis，Redis 重建后旧 refresh token 查不到。
- 用户浏览器继续拿旧 access/refresh token 请求时，会先看到 `/auth/me` 401，再尝试 `/auth/refresh`，最终回到登录页。
- 因此“旧登录态全部失效”是本次事故直接后果。

## 当前不能做的事

- 不再执行候选预演真实运行。
- 不执行 `deploy/promote-sub2api-candidate.sh`。
- 不重启公网容器。
- 不对公网 `/v1` 做主动验证。
- 不写入公网 Postgres 或 Redis，除非用户明确授权处理某个具体账号。

## 已做的隔离修复

worktree：`.worktrees/codex-sub2api-candidate-rehearsal-20260626`

已有提交：

- `16e1af0ae deploy: add sub2api candidate rehearsal config`
- `05ab68016 deploy: add candidate rehearsal workflow`
- `2abac881f fix: isolate candidate compose project`

`2abac881f` 已新增：

- `docker-compose.candidate.yml` 顶层 `name: sub2api-candidate-rehearsal`。
- 脚本中所有候选 compose 命令显式 `-p sub2api-candidate-rehearsal`。

尚未提交的修复：

- `deploy/rehearse-sub2api-candidate.sh` 增加 `wait_candidate_db()`，用 `pg_isready` 等候选 Postgres ready 后再 `pg_restore`。
- `deploy/test-candidate-rehearsal-scripts.sh` 增加 dry-run 必须包含 `pg_isready` 的断言。

## 后续处理建议

1. 先冻结候选预演真实执行，只保留只读排查。
2. 把未提交的 `pg_isready` 修复提交。
3. 再加第二层防线：候选脚本启动前检查 `sub2api`、`sub2api-postgres`、`sub2api-redis` 的 compose project label，若与候选 project 相同立即退出。
4. 去掉候选脚本里的 `down --remove-orphans`，改为只停止明确的 candidate 容器，避免按 project 误伤。
5. 候选数据目录迁出 `deploy/` 下的公网运行目录层级，避免路径误删风险。
6. 只有通过 dry-run、脚本单测、compose config 审查和本地候选端口验证后，才允许重新进行候选预演。

## 用户恢复建议

- 已登录用户需要重新登录。
- 如果某个用户“重新输入正确账号密码仍然 401”，需要提供该用户邮箱；只读检查该用户 `status`、`deleted_at`、`password_hash` 是否正常，再决定是否重置密码或恢复登录方式。
- 当前不建议恢复 Redis 旧快照：refresh token 属于会话态，且现有可用快照无法保证包含事故前完整会话，会引入更大风险。
