# Codex Desktop 看不到 GPT 5.6 模型问题诊断

## 时间

2026-07-10 12:22 (北京时间)

## 背景

用户反馈：公网 Sub2API 已部署 GPT 5.6 三个模型后，Codex Desktop 当前模型下拉仍然只显示 `GPT-5.5`，看起来无法选择/使用 5.6。

截图现象：

- ChatGPT Codex 桌面端右下角模型下拉显示 `GPT-5.5`
- 输入框按钮上也显示 `5.5 极高`
- 未在下拉中看到 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`

## 已验证事实

### 1. `/v1/models` 对真实 Key 正常返回 GPT 5.6

使用真实 API Key 请求：

```bash
GET https://api.aaccx.pw/v1/models
```

返回 HTTP 200，并包含：

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`
- `gpt-5.5`

同时不包含裸 `gpt-5.6`，这是符合当前三模型命名方案的。

### 2. 裸 `/models` 当前返回 400

使用同一个真实 API Key 请求：

```bash
GET https://api.aaccx.pw/models
GET https://api.aaccx.pw/models?client_version=0.142.0
```

当前返回：

```text
HTTP 400
```

且没有返回任何模型列表。

这与历史上下文中 Codex Desktop 会请求裸 `/models?client_version=0.142.0` 的行为吻合。

### 3. Codex Desktop 实际已经能调用 `gpt-5.6-sol`

从 Sub2API 日志和数据库核对，用户当前 Codex Desktop 使用的 Key：

- `api_key_id=84`
- `user_id=65`
- API Key 名称：`oc`
- group：`codex-pool-89-usd`

近期已经产生多条真实成功请求：

- path：`/v1/responses`
- model：`gpt-5.6-sol`
- stream：true
- status_code：200

数据库 `usage_logs` 中也有对应记录，例如：

```text
79392 | user_id=65 | api_key_id=84 | subscription_id=83 | model=gpt-5.6-sol | total_cost=0.1830020000 | 2026-07-10 12:20:27+08
79391 | user_id=65 | api_key_id=84 | subscription_id=83 | model=gpt-5.6-sol | total_cost=0.2133750000 | 2026-07-10 12:19:42+08
79390 | user_id=65 | api_key_id=84 | subscription_id=83 | model=gpt-5.6-sol | total_cost=0.1198430000 | 2026-07-10 12:18:37+08
```

因此当前不是“API 层完全不能用 5.6”，而是“桌面端模型列表 UI 未展示 5.6”。

### 4. `gpt-5.6-terra` API 层真实可用

此前用用户提供的真实 Key 测试：

```bash
POST https://api.aaccx.pw/v1/responses
model = gpt-5.6-terra
```

结果：

- HTTP 200
- status：completed
- output：`ok`
- 新增 `usage_logs.id=79365`
- `total_cost=0.0019980000`
- `billing_type=1`
- `subscription_id=71`

### 5. `gpt-5.6-luna` 当前上游不可用

此前用真实 Key 测试：

```bash
POST https://api.aaccx.pw/v1/responses
model = gpt-5.6-luna
```

结果：

- HTTP 502
- 未新增 `usage_logs`
- 未扣费

日志显示：

- Sub2API 已通过鉴权、内容审核和账号选择
- 请求已转发到 CLIProxyAPI
- CLIProxyAPI 轮换多个 Codex OAuth 账号
- 多个上游账号返回 `Model not found gpt-5.6-luna`
- 期间有一次上游 500

判断：`gpt-5.6-luna` 是 CLIProxyAPI/上游账号池当前实际不可用，不是 Sub2API 鉴权、部署或计费问题。

## 当前根因判断

### 根因 A：Codex Desktop 模型下拉依赖裸 `/models?client_version=...`

当前 Sub2API：

- `/v1/models`：正常，返回 GPT 5.6 三个模型
- `/models?client_version=0.142.0`：返回 400

Codex Desktop 很可能使用裸 `/models?client_version=...` 刷新模型下拉，而不是使用 `/v1/models`。

因此 UI 下拉仍然只显示缓存/内置的 `GPT-5.5`，看不到 5.6。

### 根因 B：`gpt-5.6-luna` 即使进入列表，也会因上游不可用而失败

`luna` 当前的问题不在模型列表，而在 CLIProxyAPI/上游：上游真实返回 `Model not found gpt-5.6-luna`。

## 待修复项

### 1. Sub2API 兼容裸 `/models`

需要把：

```text
GET /models
GET /models?client_version=...
```

接入和：

```text
GET /v1/models
```

相同或兼容的 OpenAI models list 逻辑，至少应对 OpenAI/Codex Desktop 返回同一组可用模型。

当前代码位置线索：

- 路由：`backend/internal/server/routes/gateway.go`
- 当前存在：`gateway.GET("/models", h.Gateway.Models)` 对应 `/v1/models`
- 当前裸 `/models`：`r.Any("/models", invalidBaseURLHandler)`
- 相关测试：`backend/internal/server/routes/gateway_test.go` 已覆盖 `/models?client_version=0.142.5` 路由存在性

实现目标：

- 裸 `/models` 不应再返回 400
- 带 `client_version` query 也应返回 200
- 返回格式需要兼容 Codex Desktop 模型下拉
- 不影响 `/v1/models`
- 不影响真正错误 base URL 的提示逻辑，需谨慎区分 `/models` 用作 OpenAI models list 的合法场景

### 2. CLIProxyAPI/上游修复 `gpt-5.6-luna`

需要到 CLIProxyAPI 或上游账号池排查：

- 为什么模型注册表显示多个 auth 支持 `gpt-5.6-luna`
- 但真实请求时多个账号返回 `Model not found gpt-5.6-luna`
- 是否模型目录误标、账号权限不一致、别名映射错误或上游暂未放量

在 `luna` 修复前，即使 Sub2API 模型列表展示它，真实请求仍可能 502。

## 当前可用性结论

- `gpt-5.6-sol`：API 层真实可用；Codex Desktop 已有成功调用和扣费记录。
- `gpt-5.6-terra`：API 层真实可用；真实 Key smoke test 成功并扣费。
- `gpt-5.6-luna`：模型列表可见，但上游真实不可用；当前请求 502，不扣费。
- Codex Desktop UI 下拉不显示 5.6：主要因为裸 `/models?client_version=...` 返回 400，需要 Sub2API 增加兼容。

## 后续建议

优先顺序：

1. 先修 Sub2API 裸 `/models?client_version=...` 兼容，让 Codex Desktop 下拉能显示 5.6。
2. 再修 CLIProxyAPI/上游 `gpt-5.6-luna` 实际不可用问题。
3. 修复后重新用真实 Codex Desktop 刷新模型下拉并发起 `sol/terra/luna` 三次真实请求验证。
