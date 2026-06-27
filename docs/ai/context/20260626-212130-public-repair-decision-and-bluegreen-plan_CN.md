# Sub2API 公网修复判断与蓝绿发布计划

## 当前证据

- 时间：2026-06-26 21:21 JST。
- 公网应用容器 `sub2api` healthy，映射 `127.0.0.1:18080->8080`。
- 公网 Postgres `sub2api-postgres` healthy。
- 公网 Redis `sub2api-redis` healthy。
- `http://127.0.0.1:18080/health` 返回 200，`https://api.aaccx.pw/health` 返回 200。
- 候选环境 `sub2api-candidate` healthy，映射 `127.0.0.1:18084->8080`。
- 生产 DB 主数据未见被清空：
  - `users_total=32`
  - `api_keys_total=22`
  - `users.status=active:32`
  - `password_hash_nonempty=32`
  - `auth_identities=31`
  - `pending_auth_sessions=0`
- 生产 Redis 当前不是空库：
  - `dbsize=376`
  - session 相关 key 约 `157`
  - token 相关 key 约 `103`
- 最近生产日志中存在多次 `/api/v1/auth/login` 401，但随后同一入口出现 `/api/v1/auth/login` 200 和 `/api/v1/auth/me` 200。

## 判断

当前没有证据表明生产 Postgres 用户表、密码哈希、API key 表被批量损坏。

已确认的事故影响是第一次候选预演误重建公网 Redis，导致旧 refresh token / 登录会话态丢失。用户继续拿旧 token 访问会看到 401，需要重新登录；如果重新输入正确账号密码仍 401，应按具体账号做只读排查，不应全局改用户表。

生产 DB 当前 `schema_migrations` 为 188 条，未写入候选镜像已在克隆 DB 上跑过的 154/155 等新迁移；候选克隆启动后迁移到 191 条。因此候选预演没有把新迁移写进生产 DB。

## 当前公网修复顺序

1. 不恢复旧 Redis 快照。refresh token 属于短期会话态，恢复不完整快照可能造成 token family/reuse 状态混乱。
2. 不重建公网 Postgres 或 Redis。
3. 对用户侧先按会话失效处理：刷新页面、重新登录；若浏览器仍卡旧态，清理站点本地存储后再登录。
4. 对“重新输入正确密码仍然 401”的具体账号，只读检查 `users.status`、`deleted_at`、`password_hash`、`auth_identities`，再做单账号密码重置或登录方式修复。
5. API key 请求若 401，按具体 key 排查是否缺失/禁用/余额或分组问题；不要把 dashboard 登录态 401 和 `/v1` API key 401 混为同一个问题。

## 候选镜像是否可以修复公网

候选环境已证明：当前候选镜像可以在生产 DB 克隆上完成 build、migration、启动、health、登录 smoke 和关键 API 验证。

但它不能恢复已经丢失的旧 Redis 会话；切换新镜像只能修复代码/迁移层问题，不能让用户旧登录态自动回来。

如果要发布候选镜像，正确动作是只替换公网 `sub2api` 应用容器，不动公网 Postgres/Redis：

1. 发布前做生产 Postgres 只读 dump 备份。
2. 确认候选脚本安全修复已提交，dry-run 不包含 `down --remove-orphans`，且只触碰 `sub2api-candidate*` 容器。
3. 将已验证候选镜像 retag 为公网镜像。
4. 使用公网 compose project `deploy` 只重建 `sub2api`：

```bash
docker compose \
  -p deploy \
  --env-file /Users/wujianxiang/CodeSpace/sub2api/deploy/.env.scheme-a.local \
  -f /Users/wujianxiang/CodeSpace/sub2api/deploy/docker-compose.local.yml \
  up -d --no-deps --force-recreate sub2api
```

5. 切换后验证 `127.0.0.1:18080/health`、`https://api.aaccx.pw/health`、管理员登录、关键只读页面和少量受控 API key smoke。

## 关于“只切前端”

不能只切前端来修复当前问题。

当前 Docker 镜像同时包含后端和前端静态资源。若本地候选完整跑通，公网应切换的是已验证的应用镜像，而不是单独切前端。前端代码不能修复 Redis 会话丢失，也不能替代后端 migration 和 API 行为验证。

## 发布红线

- 不执行会按 project 清理容器的 `docker compose down --remove-orphans`。
- 不使用 `up` 重建 `postgres` 或 `redis`。
- 不执行 `--renew-anon-volumes`、`volume rm`、`rm -rf deploy/postgres_data`、`rm -rf deploy/redis_data`。
- 不直接更新生产 `schema_migrations.checksum` 来绕过迁移校验。
- 不在日志、文档或提交里记录完整 API key、JWT、支付密钥、SMTP 密码。
