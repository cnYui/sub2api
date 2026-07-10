# 公网完整链路 HTTP 上游修复与真实 API 测试结果

## 背景

用户要求参考 CLIProxyAPI 文档 `20260709-1708-local-management-http-https-protocol-mismatch_CN.md` 重新跑通公网完整链路。

该文档确认 CLIProxyAPI `8317` 已从 TLS 模式切换为明文 HTTP，正确访问方式为：

- `http://127.0.0.1:8317/management.html`
- `http://127.0.0.1:8317/healthz`

## 初始问题

Sub2API 公网真实请求最初返回：

- `503 Service temporarily unavailable`
- 日志：`openai.account_select_failed: no available accounts`
- 账号临时不可调度原因：`Post "https://host.docker.internal:8317/v1/responses": dial tcp ... connect: connection refused`

进一步检查发现：

- CLIProxyAPI 8317 当前 HTTP 可达：`/healthz` 返回 `{"status":"ok"}`
- Sub2API 容器内 HTTP 可达：`http://host.docker.internal:8317/healthz`
- 但 Sub2API 上游账号 `cliproxy-local-openai` 的 `credentials.base_url` 仍是 `https://host.docker.internal:8317/v1`

## 修复步骤

### 1. 修正 Sub2API 上游账号协议

将公网运行态 DB 中 `accounts.id=1` 的：

- 原 `base_url`: `https://host.docker.internal:8317/v1`
- 改为：`http://host.docker.internal:8317/v1`

同时清理：

- `temp_unschedulable_until`
- `temp_unschedulable_reason`

### 2. 发现 Sub2API 安全配置仍拒绝 HTTP

修正账号协议后，真实公网请求进入 Sub2API 转发层，但返回：

- HTTP 502
- 日志：`invalid base_url: invalid url scheme: http`

原因：容器环境变量为：

- `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false`
- `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=false`

### 3. 重建应用容器以允许本机 HTTP 上游

只替换 `sub2api-candidate` 应用容器，不动 Postgres/Redis/nginx。

旧容器保留为：

- `sub2api-candidate-before-http-upstream-env-20260709-172301`

新容器仍使用原镜像：

- `sub2api-candidate:20260709-102735-d4fae0839-auto-refund`

新增/覆盖环境变量：

- `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true`
- `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true`

公网 health 验证：

- `18084/health = 200`
- `8080/health = 200`
- `api.aaccx.pw/health = 200`

## 真实公网 API 测试

### 测试 1：正常订阅用户

用户：`313398924@qq.com`

- API Key：真实 active 自动 Key（未记录完整明文）
- 订阅：`codex-pool-19-usd`
- 请求：`POST https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- 输入：`请只回复 ok`

结果：

- HTTP：`200`
- 响应 ID：`resp_0461d0a61cd4976c016a4f5a89388c8191bd6cab751bec694d`
- status：`completed`
- output：`ok`
- 订阅 `daily_usage_usd`：`0.0000000000 -> 0.0039960000`
- usage_logs：新增 `id=74160`，`user_id=9`，`api_key_id=6`，`group_id=2`，`account_id=1`，`subscription_id=5`，`billing_type=1`，`total_cost=0.0039960000`

### 测试 2：仅流量卡用户

用户：`1160503105@qq.com`

- API Key：真实 active 自动 Key（未记录完整明文）
- 无 active 订阅
- OpenAI/GPT 流量卡余额：`27.7477390000`
- 请求：`POST https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- 输入：`请只回复 ok`

结果：

- HTTP：`200`
- 响应 ID：`resp_071216e20d321668016a4f5ab583148191a2461b6b2ec94d50`
- status：`completed`
- output：`ok`
- 流量卡余额：`27.7477390000 -> 27.7437430000`
- usage_logs：新增 `id=74164`，`user_id=36`，`api_key_id=86`，`group_id=10`，`account_id=1`，`subscription_id=NULL`，`billing_type=0`，`total_cost=0.0039960000`
- traffic_credit_ledger：新增 `id=2222`，`entry_type=deduction`，`amount_usd=0.0039960000`，`balance_after_usd=7.7437430000`

## CLIProxyAPI 侧验证

CLIProxyAPI 日志显示 Sub2API 请求已实际进入本地聚合入口：

- `POST "/v1/responses"` 返回 `200`
- 真实使用 OAuth provider `codex` 的 auth files
- 同期也存在部分上游账号 `401/429`，但 CLIProxyAPI 内部账号池有成功 failover，最终两次公网 smoke test 都返回 200。

## 当前最终状态

- 公网入口：正常
- Sub2API：正常
- Sub2API -> CLIProxyAPI：已改为 HTTP 并跑通
- CLIProxyAPI -> 上游 OpenAI/Codex：真实请求成功
- 正常订阅计费：成功
- 仅流量卡计费：成功
- 旧 HTTPS 协议不匹配问题：已修复

## 注意事项

- 本次只改运行态 DB 中上游账号 `base_url` 和应用容器环境变量。
- 未重建 Postgres/Redis。
- 未修改 nginx/Cloudflare Tunnel。
- 旧容器 `sub2api-candidate-before-http-upstream-env-20260709-172301` 保留用于应用层回滚。
- 后续如果 CLIProxyAPI 再切回 TLS，则需要同时把 Sub2API 上游 `base_url` 改回 HTTPS，并关闭/调整 insecure HTTP 运行态配置。
