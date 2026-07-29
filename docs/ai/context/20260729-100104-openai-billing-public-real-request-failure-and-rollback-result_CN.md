# OpenAI 计费公网真实请求失败与回滚结果

## 请求范围

- 使用用户在当前会话提供的 API Key，通过 `https://api.aaccx.pw` 进行真实公网验证；密钥不记录、不输出、不写入文件。
- 已认证 `GET /v1/models` 返回 200，确认公网域名、Cloudflare、Nginx 与应用认证链路可达。
- 随后发送单轮、非流式、无图片/附件/工具的 `gpt-5.4-mini` Chat Completions 请求，显式 `max_tokens=1`，用于最小化验证上游调度与计费落库。

## 结果

模型请求未成功，应用返回 `billing_persistence_error`（HTTP 500）：`Unable to persist usage record`。未重试该请求，以避免重复计费风险。

外层日志给出的底层 PostgreSQL 错误为：`pq: column f.reservation_id does not exist`。Usage-fact worker 因同一错误持续无法认领待处理事实。

## 根因证据

- `180_openai_billing_authorizations.sql` 在 `usage_facts.reservation_id` 存在时将其重命名为 `authorization_id`，并且迁移集成测试明确断言旧列不应存在。
- 生产数据库已执行迁移 180，`usage_facts` 不存在 `reservation_id`。
- `internal/repository/usage_fact_repo.go` 仍在 INSERT、SELECT 和 ClaimPending 的 `RETURNING` 中引用 `reservation_id`。

因此根因是迁移后的数据库 schema 与 usage-fact repository 查询不一致，不是公网 Nginx、Cloudflare 或 API Key 认证问题。

## 安全处置与当前状态

- 由于这是计费完整性故障，已立即停止 `sub2api-public-nginx-local`，使公网 `https://api.aaccx.pw/health` 回到 502。
- 外层 `127.0.0.1:18080/health` 与内层 `127.0.0.1:18086/health` 仍为 200；数据库、Redis、volume、备份和回滚镜像均未改动。
- 只读检查最近十分钟的 `billing_authorizations` 与 `usage_facts` 未发现新增记录。该事实说明没有新增可见的本地计费记录，但不能仅凭此推断外部上游绝未接收请求。

## 恢复前条件

在重新开放公网前，必须使 usage-fact repository 统一使用 `authorization_id`，补充覆盖迁移后 schema 的集成验证，并在隔离候选环境完成一次真实或等价的端到端落库验证。修复与重新部署需另行授权。
