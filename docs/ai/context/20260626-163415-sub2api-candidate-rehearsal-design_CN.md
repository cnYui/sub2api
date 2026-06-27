# Sub2API 候选预演环境设计

## 背景

2026-06-26 直接执行本地 `main` 重部署时，新镜像构建成功，但替换公网 `sub2api` 容器后启动失败。直接原因是生产 DB 中 `schema_migrations` 记录的 `155_seed_codex_subscription_plans_baseline.sql` checksum 与新镜像内迁移文件不一致，应用启动时拒绝继续，导致公网短暂不可用。当前运行态已恢复到旧镜像，公网 API 正常服务。

本设计目标是在不影响 `https://api.aaccx.pw/v1` 的前提下，增加一套本地候选预演流程：新镜像先在本地独立容器中连接“生产 DB 克隆”启动，通过 migration、健康检查和关键 API 验证后，才允许替换公网容器。

## 目标

- 公网旧容器 `sub2api` 持续运行，继续占用 `127.0.0.1:18080`。
- 新代码构建为候选镜像，例如 `sub2api-candidate:<git-sha>`，不直接覆盖 `weishaw/sub2api:latest`。
- 候选容器使用独立端口，例如 `127.0.0.1:18081->8080`，所有验证只打本地候选端口。
- 候选 PostgreSQL 来自生产 DB 克隆，能复现真实 `schema_migrations`、订阅、套餐、支付配置等运行态状态。
- 候选 Redis 独立，避免污染公网 Redis。
- 候选验证通过后，再执行一次短窗口公网替换。

## 非目标

- 不做完整 blue-green 流量切换。
- 不让候选容器连接当前生产 DB 写入数据。
- 不让候选验证请求进入 `https://api.aaccx.pw/v1`。
- 不在文档、日志或提交中记录完整 API Key、支付密钥、HMAC secret、SMTP 密码或 env 文件内容。

## 推荐方案

新增一套“候选预演”Docker Compose，而不是复用公网 compose 直接改端口。

推荐新增文件：

- `deploy/docker-compose.candidate.yml`
- `deploy/.env.candidate.local.example`
- `deploy/rehearse-sub2api-candidate.sh`

实际敏感配置只放在未提交的 `deploy/.env.candidate.local`，不要写入源码。

候选环境容器命名建议：

- `sub2api-candidate`
- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`

候选环境数据目录建议：

- `deploy/candidate/postgres_data`
- `deploy/candidate/redis_data`
- `deploy/candidate/dumps`
- `deploy/candidate/logs`

## 架构

当前公网链路保持不变：

```text
Cloudflare Tunnel
  -> nginx 127.0.0.1:8080
  -> sub2api 127.0.0.1:18080
  -> CLIProxyAPI 127.0.0.1:8317
```

新增候选预演链路：

```text
local only
  -> sub2api-candidate 127.0.0.1:18081
  -> sub2api-candidate-postgres
  -> sub2api-candidate-redis
```

候选容器默认不挂到 nginx，不暴露公网域名，不改变 Cloudflare Tunnel。

## 数据库克隆设计

候选 DB 必须来自生产 DB 的克隆，但候选容器不能直接连接生产 DB。

克隆流程建议：

1. 从生产 Postgres 执行一致性 `pg_dump`，生成本地 dump 文件。
2. 创建或清空候选 Postgres 数据目录。
3. 启动 `sub2api-candidate-postgres`。
4. 将 dump 恢复到候选 Postgres。
5. 启动 `sub2api-candidate`，让新镜像在候选 DB 上执行启动 migration 校验。

为什么要完整克隆：

- 只克隆 `schema_migrations` 能暴露 checksum 问题，但不能发现套餐、支付、分组、用户、账号池等业务数据兼容问题。
- 完整克隆更接近公网状态，能提前发现“迁移通过但业务页面/接口异常”的问题。

候选 DB 克隆后的必要脱敏/隔离：

- 候选环境不允许发送真实支付回调、邮件、短信或用户通知。
- 候选环境不应自动执行会影响真实上游的后台任务。
- 如当前代码缺少“禁用外部副作用”的配置，应优先补一个候选模式开关，例如 `RUN_MODE=candidate` 或 `EXTERNAL_SIDE_EFFECTS_ENABLED=false`，让候选环境只做启动、读接口和受控 smoke test。

## 镜像与标签策略

候选构建禁止直接使用 `weishaw/sub2api:latest`。

建议标签：

```text
sub2api-candidate:<short-sha>
sub2api-candidate:<timestamp>-<short-sha>
```

候选通过后再执行受控提升：

```text
docker tag sub2api-candidate:<short-sha> weishaw/sub2api:latest
```

然后使用现有公网 compose 只重建 `sub2api` 服务。

这样可以避免“失败镜像已经覆盖 latest，回滚时再找旧镜像”的混乱。

## 验证范围

候选验证分三层。

第一层：启动与迁移。

- `sub2api-candidate` 容器进入 `healthy`。
- 日志无 `checksum mismatch`、migration failed、panic。
- `schema_migrations` 中最新记录与候选镜像一致。

第二层：本地 HTTP。

- `GET http://127.0.0.1:18081/health` 返回 200。
- 首页、dashboard、purchase、usage-guide HTML 返回 200。
- HTML 引用的 JS/CSS 资源均可从 `127.0.0.1:18081/assets/*` 获取。

第三层：关键业务 API。

- 只用候选端口测试 `/api/v1/settings/public`、套餐列表、订阅状态、支付方式列表等只读接口。
- 默认不对 `/v1/responses` 发真实上游请求。
- 如果必须验证 OpenAI 网关链路，只使用专门的候选测试 Key、极小请求、明确的测试分组，并接受可能调用 CLIProxyAPI 的事实。

## 发布门禁

只有满足以下条件才允许替换公网：

- 候选镜像构建成功。
- 候选 DB 克隆恢复成功。
- 候选容器 healthy。
- 候选 migration 校验通过。
- 候选关键页面和静态资源通过。
- 候选只读业务 API 通过。
- 没有发现需要人工修复的 migration checksum、启动 panic 或配置缺失。

发布时只做两步：

1. 将已通过验证的候选镜像 tag 为 `weishaw/sub2api:latest`。
2. 使用公网 compose `up -d --no-deps --force-recreate sub2api` 替换应用容器。

发布后只验证低风险端点：

- `http://127.0.0.1:18080/health`
- `https://api.aaccx.pw/health`
- 前端 HTML 与静态资源 hash

不要用真实用户 Key 对公网 `/v1` 发验证流量。

## 失败处理

候选阶段失败：

- 不影响公网。
- 保留候选容器日志、候选 DB、候选镜像 tag。
- 记录失败原因到 `docs/ai/context/`。
- 不提升镜像，不替换公网。

公网替换失败：

- 优先用已知可用旧镜像 tag 回滚。
- 不在生产 DB 上手工更新 migration checksum，除非已经确认 DB 实际状态与目标 migration 语义一致。
- 回滚后记录镜像 ID、容器状态、健康检查和根因。

## 对本次 155 checksum 问题的覆盖

这套候选预演能提前捕获本次事故：

- 候选 DB 克隆会带着生产 DB 中 `155_seed_codex_subscription_plans_baseline.sql = 0e2d20c6...` 的记录。
- 新镜像启动时会在候选 DB 上执行同样的 checksum 校验。
- 如果新镜像内 155 文件 checksum 不兼容，候选容器会在 `18081` 失败，而公网 `18080` 继续服务。

因此，后续修复 155 的正确路径是在候选环境先验证：

- 新增迁移补齐业务数据差异；或
- 为 155 增加明确 checksum 兼容规则；或
- 找回并恢复生产 DB 已应用的旧 155 版本。

任何方案都必须先在候选 DB 上通过启动和关键 API 验证。

## 需要实现的最小改动

1. 新增候选 compose，使用独立容器名、网络、端口和数据目录。
2. 新增候选 env example，只列变量名和说明，不包含密钥。
3. 新增候选预演脚本：
   - 定位 `main` worktree；
   - 构建 `sub2api-candidate:<sha>`；
   - dump 生产 DB 到本地文件；
   - 恢复到候选 DB；
   - 启动候选应用；
   - 执行候选健康检查和只读 smoke test；
   - 输出是否允许提升到公网。
4. 修改公网重部署脚本或新增发布脚本，支持“只发布已验证候选镜像”，避免直接 build 到 `latest`。

## 待确认问题

- 候选 DB 是否每次全量重建，还是保留最近一次克隆用于快速复测。推荐默认全量重建，避免历史候选状态污染判断。
- 候选环境是否允许访问 CLIProxyAPI。推荐默认不测真实 `/v1`，必要时用显式参数开启。
- 代码是否已有可靠的“禁用外部副作用”配置。若没有，应作为实现前置项补齐。
