# CLIProxyAPI Usage Event 回调计费修复设计与实施计划

## 背景

当前 Sub2API 通过数据库中的 OpenAI 上游账号连接本机 CLIProxyAPI：

```text
Sub2API 容器 -> https://host.docker.internal:8317/v1 -> CLIProxyAPI
```

该主链路配置是正确的。CLIProxyAPI 进程运行在宿主机，Sub2API 运行在 Docker 容器内，因此 Sub2API 访问宿主机服务必须使用 `host.docker.internal`，而不能使用容器内的 `127.0.0.1`。

当前异常集中在 CLIProxyAPI 反向回调 Sub2API 的 usage event 链路。CLIProxyAPI `.env` 中仍配置旧地址：

```text
YUI_USAGE_EVENT_URL=http://127.0.0.1:4173/api/internal/usage-events
SHOP_KEY_STATUS_URL=http://127.0.0.1:4173/api/internal/api-keys/status
```

当前候选 Sub2API 服务映射在宿主机 `127.0.0.1:18084`，并且现有代码没有 `/api/internal/usage-events` 接收接口。因此 CLIProxyAPI usage event 回调持续超时或 404，导致真实 token usage 无法通过该链路进入 Sub2API 内部计费。

## 目标

1. 在 Sub2API main 分支新增 CLIProxyAPI usage event 内部接收接口。
2. 保留 CLIProxyAPI usage event，不关闭 `USAGE_EVENTS_ENABLED`。
3. 对回调请求执行内部 token 与 HMAC 校验，避免公网或伪造请求写入计费事实。
4. 将 CLIProxyAPI 返回的真实 token usage 转换为 Sub2API 现有 `usage_facts` durable outbox 事实。
5. 保证 usage event 幂等，不因 CLIProxyAPI 重试重复扣费。
6. 上线前备份 PostgreSQL 与 Redis，使用 main 最新代码构建新容器启动，并重启 CLIProxyAPI。

## CLIProxyAPI 已有 usage event 协议

CLIProxyAPI 发送请求：

```http
POST {YUI_USAGE_EVENT_URL}
Content-Type: application/json
x-internal-token: {YUI_USAGE_EVENT_TOKEN}
x-usage-timestamp: {unix_timestamp}
x-usage-signature: {hex_hmac_sha256}
```

签名原文：

```text
{timestamp}\n{raw_body}
```

签名密钥：

```text
YUI_USAGE_EVENT_HMAC_SECRET
```

请求体：

```json
{
  "version": 1,
  "request_id": "string",
  "api_key_hash": "string",
  "api_key_preview": "string",
  "provider": "string",
  "model": "string",
  "endpoint": "string",
  "source": "string",
  "auth_index": "string",
  "success": true,
  "failed": false,
  "input_tokens": 0,
  "output_tokens": 0,
  "reasoning_tokens": 0,
  "cached_tokens": 0,
  "cache_hit_input_tokens": 0,
  "cache_miss_input_tokens": 0,
  "total_tokens": 0,
  "latency_ms": 0,
  "requested_at": "2026-07-18T00:00:00Z"
}
```

## 关键限制

CLIProxyAPI event 不携带 Sub2API 的 `user_id`、`api_key_id`、`group_id`、订阅/流量卡预留等完整计费上下文。当前可利用的关联字段主要是：

- `request_id`
- `api_key_hash`
- `api_key_preview`
- token 用量字段
- `model`
- `endpoint`
- `source` / `auth_index`

因此首版实现采用两层关联策略：

1. 优先通过 `request_id` 查找 Sub2API 已经创建的 pending `usage_facts`。
2. 若找不到 pending fact，则使用 `api_key_hash` 反查 Sub2API API Key，并根据 API Key 的用户、分组和当前有效计费资格创建新的 `usage_facts`。

如果 CLIProxyAPI 的 `request_id` 与 Sub2API 的请求 ID 不一致，则第一层关联不会命中，但第二层仍可基于 API Key hash 兜底。后续可增强为 Sub2API 转发到 CLIProxyAPI 时显式透传 correlation id，并由 CLIProxyAPI 原样带回。

## Sub2API 实现方案

### 1. 配置项

新增配置结构：

```yaml
internal_usage_event:
  enabled: true
  token: ""
  hmac_secret: ""
  max_skew_seconds: 300
```

环境变量可覆盖：

```text
INTERNAL_USAGE_EVENT_ENABLED=true
INTERNAL_USAGE_EVENT_TOKEN=...
INTERNAL_USAGE_EVENT_HMAC_SECRET=...
INTERNAL_USAGE_EVENT_MAX_SKEW_SECONDS=300
```

为兼容现有 CLIProxyAPI 命名，也支持直接读取：

```text
YUI_USAGE_EVENT_TOKEN
YUI_USAGE_EVENT_HMAC_SECRET
```

优先级：显式 `internal_usage_event.*` 配置优先；为空时回退到 `YUI_*` 环境变量。

### 2. 路由

新增无需 JWT/API Key 鉴权的内部路由：

```text
POST /api/internal/usage-events
```

该路由只允许通过 token + HMAC 校验访问。

### 3. 鉴权与防重放

接收逻辑：

1. 读取 raw body。
2. 校验 `x-internal-token` 是否等于配置 token。
3. 校验 `x-usage-timestamp` 是否存在且与当前时间偏差不超过 `max_skew_seconds`。
4. 使用配置 hmac secret 计算 `HMAC-SHA256(timestamp + "\n" + raw_body)`。
5. 使用常量时间比较校验 `x-usage-signature`。
6. 任一校验失败返回 `401`。
7. JSON 不合法或字段缺失返回 `400`。

### 4. DTO 映射

新增 `CLIProxyUsageEvent` DTO，字段与 CLIProxyAPI `internal/usage/event_types.go` 保持一致。

Token 映射到 Sub2API 计费字段：

| CLIProxyAPI 字段 | Sub2API 字段 |
|---|---|
| `input_tokens` | `InputTokens` |
| `output_tokens` | `OutputTokens` |
| `reasoning_tokens` | 暂存到 payload 的 upstream usage 元数据 |
| `cached_tokens` | 优先映射为 `CacheReadTokens` |
| `cache_hit_input_tokens` | `CacheReadTokens` |
| `cache_miss_input_tokens` | 输入 token 的明细元数据 |
| `total_tokens` | usage log total / metadata |

如果 `cache_hit_input_tokens > 0`，优先使用它作为 `CacheReadTokens`；否则使用 `cached_tokens`。`input_tokens` 保持原始输入 token，不在接收层自行扣减 cache hit，以避免破坏现有价格模型；后续价格模型如需区分命中/未命中，可从 payload 元数据读取。

### 5. 幂等策略

首版采用 `usage_facts` 现有唯一约束：

```text
(request_id, api_key_id)
```

`request_id` 构造：

```text
cliproxy:{event.request_id}
```

如果 CLIProxyAPI 重试同一 event，则相同 `request_id + api_key_id` 会命中 `CreatePending` 的 `ON CONFLICT DO NOTHING`，不会重复写入和重复扣费。

### 6. 关联策略

#### 6.1 优先关联已有 usage_fact

给 `UsageFactRepository` 增加查询能力：

```go
FindByRequestID(ctx, requestID string) ([]UsageFact, error)
```

查找候选：

- `request_id = event.request_id`
- 或 `request_id = "cliproxy:" + event.request_id`

如果命中 pending fact，则用 event token 更新或重建 payload，并进入后续 durable settlement。

#### 6.2 API Key hash 兜底

如果找不到已有 fact，则通过 API Key hash 找回 Sub2API API Key。

CLIProxyAPI 当前 `api_key_hash` 是对明文 key 的 SHA256 hex。Sub2API 数据库当前保存明文 API Key，因此可新增 repository 方法：

```go
FindActiveBySHA256Hash(ctx, hash string) (*APIKey, error)
```

实现上可先查询活跃 API Key 列表，在应用层 SHA256 比对；后续如需要优化，再增加数据库 hash 列或表达式索引。

### 7. 计费上下文构造

兜底创建 fact 时，需要重建 `UsageBillingCommand`：

- `UserID`：来自 API Key。
- `APIKeyID`：来自 API Key。
- `GroupID`：来自 API Key 当前 effective group。
- `AccountID`：当前 CLIProxyAPI 上游账号 ID，若无法准确反查具体 OAuth auth 文件，则使用 Sub2API 中 `cliproxy-local-openai` 这个 OpenAI APIKey 上游账号 ID。
- `Model`：event.model。
- `InputTokens` / `OutputTokens` / `CacheReadTokens`：来自 event。
- `CompletedAt`：优先使用 event.requested_at + latency，否则使用当前时间。
- `RequestPayloadHash`：对 event raw body 求 hash。
- `RequestFingerprint`：复用现有 `UsageBillingCommand.Normalize()` 自动生成。

对于订阅/流量卡来源，首版应复用现有 billing eligibility / authorization 服务，不应自行直接扣余额。若缺少预请求 reservation，traffic credit 预留字段为空，durable settlement 仍通过现有 `UsageBillingRepository.Apply()` 执行最终幂等扣费。

### 8. 与现有 OpenAI usage fact 的关系

当前 Sub2API 已经会在部分 OpenAI 成功响应中同步写入 `usage_facts`。为了避免双计费：

1. CLIProxyAPI event 的 `request_id` 加 `cliproxy:` 前缀，与普通 Sub2API 直接响应 fact 区分。
2. 只有当该请求没有被 Sub2API 自己完整解析并写入真实 usage 时，CLIProxyAPI usage event 才应作为补充事实。
3. 如果后续能让 Sub2API 与 CLIProxyAPI 共享同一个 correlation id，则应改为同一个 request id，并让 CLIProxyAPI event 更新 pending fact，而不是另建 fact。

首版上线时需要重点观察同一用户同一请求是否出现两条 usage log。如果出现双计费，应改为强制 request_id 关联和 pending fact 更新模式。

## CLIProxyAPI 配置修复

Sub2API 新接口上线后，将 CLIProxyAPI `.env` 调整为：

```text
USAGE_EVENTS_ENABLED=true
YUI_USAGE_EVENT_URL=http://127.0.0.1:18084/api/internal/usage-events
YUI_USAGE_EVENT_TOKEN=<与 Sub2API 配置一致>
YUI_USAGE_EVENT_HMAC_SECRET=<与 Sub2API 配置一致>
```

`SHOP_KEY_STATUS_URL` 当前 Sub2API 仍无对应接口，本次不作为计费主链路处理。若 CLIProxyAPI 需要该接口做 key 状态同步，后续单独补 `/api/internal/api-keys/status`。

## 实施步骤

1. 新增设计文档。
2. 新增 Sub2API config：`InternalUsageEventConfig`。
3. 新增 DTO 与 handler：`InternalUsageEventHandler`。
4. 新增 service：`InternalUsageEventService`，负责鉴权后业务处理。
5. 扩展 `UsageFactRepository`：支持按 request id 查询，必要时支持更新 pending payload。
6. 扩展 `APIKeyRepository` 或 `APIKeyService`：支持通过 SHA256 hash 反查 active API Key。
7. 注册路由：`POST /api/internal/usage-events`。
8. 补充单元测试：
   - HMAC 成功。
   - token 错误。
   - timestamp 超时。
   - signature 错误。
   - 重复 event 幂等。
9. 运行 Go 测试、格式化与构建检查。
10. 备份 PostgreSQL 与 Redis。
11. 基于 main 最新代码构建新 Sub2API 容器。
12. 更新 CLIProxyAPI `.env` usage event URL 到 `18084`。
13. 重启 CLIProxyAPI。
14. 验证日志不再出现 `usage event sync failed`，并确认新 usage fact / usage log 写入。

## 验证命令

建议本地代码验证：

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/backend
go test -count=1 ./internal/config ./internal/handler ./internal/service ./internal/repository
go test -count=1 ./cmd/server
go test -count=1 ./...
```

运行态验证：

```bash
curl -sS http://127.0.0.1:18084/health
curl -sS https://api.aaccx.pw/health
```

日志验证：

```bash
tail -f /Users/wujianxiang/Library/Logs/com.cliproxyapi.server.out.log
```

期望不再出现：

```text
usage event sync failed
Post "http://127.0.0.1:4173/api/internal/usage-events": context deadline exceeded
```

## 风险与回滚

### 风险

- 如果 Sub2API 已经从 OpenAI 响应中写入真实 usage，CLIProxyAPI event 可能造成双计费。
- 如果 CLIProxyAPI `api_key_hash` 与 Sub2API key hash 算法不一致，兜底关联会失败。
- 如果无法准确还原订阅/流量卡预授权上下文，首次版本可能只能完成余额/后置结算，流量卡超卖控制依赖现有 reservation 逻辑。

### 回滚

1. 恢复旧 Sub2API 容器。
2. 恢复 CLIProxyAPI `.env` 中的 usage event URL 或临时关闭回调。
3. 如产生误写 usage fact，可按 `request_id LIKE 'cliproxy:%'` 审计并人工处理。
4. 使用上线前 PostgreSQL/Redis 备份恢复数据。

## 后续增强

1. Sub2API 转发到 CLIProxyAPI 时显式传递 `X-Sub2API-Usage-Correlation-ID`。
2. CLIProxyAPI usage event 增加 `sub2api_request_id` 字段并原样回传。
3. Sub2API 接收 event 时优先更新既有 pending fact，而不是另建 `cliproxy:` fact。
4. 增加 `/api/internal/api-keys/status`，让 CLIProxyAPI 在发起请求前能确认 key 是否仍有效。
5. 为 API Key 增加持久化 hash 字段和索引，避免应用层扫描 active keys。
