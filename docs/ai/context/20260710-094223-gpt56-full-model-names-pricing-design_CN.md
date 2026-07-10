# GPT-5.6 完整模型名展示、调用与计费设计

## 目标

将 Sub2API OpenAI 链路永久支持以下三个完整模型名：

- `gpt-5.6-sol`
- `gpt-5.6-terra`
- `gpt-5.6-luna`

目标行为：

- `GET /v1/models` 展示完整模型名，不展示短别名 `gpt-5.6`。
- 用户请求 `/v1/responses`、`/v1/chat/completions` 时可直接使用完整模型名转发上游。
- 用量扣费按 OpenAI 官方 API 价格和真实 usage token 计算。
- 数据库继续用现有通用模型字段承载新模型，不按模型新增列。

本设计只写方案，不实施代码、不改数据库、不重启容器。

## 当前事实

- 上游 CLIProxyAPI `127.0.0.1:8317/v1/models` 已返回 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`。
- Sub2API 公网 `/v1/responses` 使用 `gpt-5.6-sol` 已返回 HTTP 200。
- Sub2API 公网 `/v1/models` 仍回落到 `backend/internal/pkg/openai/constants.go` 的静态 `openai.DefaultModels`，因此暂不显示 GPT-5.6。
- `gpt-5.6` 短别名当前真实请求返回 502；本次不把它作为可见模型名。
- 当前生产库无 `channels` 记录，默认扣费主要走 `PricingService -> BillingService fallback`，不是 DB channel pricing。

官方来源：

- OpenAI Models 文档：`https://developers.openai.com/api/docs/models`
- OpenAI Pricing 文档：`https://developers.openai.com/api/docs/pricing`
- 当前远程 price mirror 已含 `gpt-5.6/gpt-5.6-sol/gpt-5.6-terra/gpt-5.6-luna`，仅作实施交叉校验，不作为官方来源。

## 方案比较

### 方案 A：只改运行态 `model_mapping`

给 `cliproxy-local-openai` 配完整 identity `model_mapping`，让 `/v1/models` 从账号 mapping 聚合出 GPT-5.6。

优点：无需发版。

缺点：`model_mapping` 同时是账号白名单，必须补齐所有旧模型，否则会误拦截；也无法给代码默认列表和测试提供长期保障。

结论：只适合临时救火，不作为永久方案。

### 方案 B：只改 `openai.DefaultModels`

把三个模型加入静态默认列表。

优点：能立刻解决 `/v1/models` 展示。

缺点：计费仍可能命中 `pricing missing -> zero cost`，尤其在远程 pricing 未更新或网络不可用时。

结论：不完整。

### 方案 C：默认模型列表 + 内置价格资源 + fallback 计费一起更新

同时更新：

- `openai.DefaultModels`
- `backend/resources/model-pricing/model_prices_and_context_window.json`
- `BillingService` fallback 价格与 OpenAI 模型归一化
- 相关单测和公网验收

优点：展示、调用、价格、扣费、离线 fallback 一致；远程价格源不可用时也不会漏扣。

结论：推荐。

## 设计细节

### 模型展示

修改 `backend/internal/pkg/openai/constants.go`：

- 在 `DefaultModels` 中加入：
  - `ID: "gpt-5.6-sol"`, `DisplayName: "GPT-5.6 Sol"`
  - `ID: "gpt-5.6-terra"`, `DisplayName: "GPT-5.6 Terra"`
  - `ID: "gpt-5.6-luna"`, `DisplayName: "GPT-5.6 Luna"`
- 不加入 `gpt-5.6` alias，避免客户端看到可选项后用 alias 调用仍失败。
- `Created` 使用官方发布时间对应的 Unix 秒；实施时以官方文档或上游模型元数据复核。

这会让未配置 `model_mapping` 的 OpenAI 分组继续走默认模型列表，并自然显示三款 GPT-5.6。

### 调用路径

不增加额外 alias 映射。

原因：

- 用户要求“用完整名字展示并调用”。
- 当前 `gpt-5.6-sol` 已真实可用，说明 OpenAI APIKey passthrough 路径可以原样转发完整模型名。
- `gpt-5.6` alias 当前 502，贸然对外展示或自动映射会掩盖上游真实能力差异。

如果未来要支持 alias，单独设计 `gpt-5.6 -> gpt-5.6-sol`，并在模型列表、调用、usage log 的 `requested_model/upstream_model/model_mapping_chain` 中显式记录映射。

### 官方价格

实施前必须再次以 OpenAI 官方 pricing 页复核。当前设计采用以下价格矩阵，单位为 USD / 1M tokens：

| 模型 | 输入 | 缓存输入 | 输出 | 长上下文输入 | 长上下文缓存输入 | 长上下文输出 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `gpt-5.6-sol` | 5.00 | 0.50 | 30.00 | 10.00 | 1.00 | 45.00 |
| `gpt-5.6-terra` | 2.50 | 0.25 | 15.00 | 5.00 | 0.50 | 22.50 |
| `gpt-5.6-luna` | 1.00 | 0.10 | 6.00 | 2.00 | 0.20 | 9.00 |

Service tier：

- `flex` 按当前 OpenAI 价格与现有代码规则：输入、缓存输入、输出为标准价 50%。
- `priority` 按当前 OpenAI 价格与现有代码规则：输入、缓存输入、输出为标准价 200%。
- `batch` 价格可写入内置 pricing JSON 以保持数据完整，但当前网关没有正式 `/v1/batch` 用户入口，本次不新增 batch 扣费路径。

换算为代码中的 per-token：

| 模型 | input | cache_read | output |
| --- | ---: | ---: | ---: |
| `gpt-5.6-sol` | `5e-6` | `5e-7` | `3e-5` |
| `gpt-5.6-terra` | `2.5e-6` | `2.5e-7` | `1.5e-5` |
| `gpt-5.6-luna` | `1e-6` | `1e-7` | `6e-6` |

### 计费实现

现有 `OpenAIGatewayService.RecordUsage()` 已按真实 usage 生成：

- `InputTokens = usage.input_tokens - cache_read_input_tokens`
- `OutputTokens = usage.output_tokens`
- `CacheReadTokens = usage.cache_read_input_tokens`
- `CacheCreationTokens = usage.cache_creation_input_tokens`
- `ImageOutputTokens = usage.image_output_tokens`

本次不改扣费主流程，只补价格解析：

1. `backend/resources/model-pricing/model_prices_and_context_window.json`
   - 增加 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`。
   - 可增加 `gpt-5.6` alias 定价用于防漏扣，但不放进 `/v1/models`。
   - 写入 `input_cost_per_token`、`output_cost_per_token`、`cache_read_input_token_cost`、`*_flex`、`*_priority`、`*_above_272k_tokens`。

2. `backend/internal/service/billing_service.go`
   - 新增三款模型的 fallback `ModelPricing`。
   - 让 `normalizeKnownOpenAICodexModel()` / `getFallbackPricing()` 识别 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`。
   - 将长上下文策略从 `isOpenAIGPT54Model()` 扩展为 OpenAI 272K+ 定价策略，覆盖 GPT-5.6 三款模型。
   - 保持未知 OpenAI 模型不回退，避免误计价。

3. `backend/internal/service/pricing_service.go`
   - 如果远程 JSON 已含 GPT-5.6，精确匹配优先。
   - 如果远程 JSON 缺失，fallback 价格仍能工作。
   - 不能把 `gpt-5.6-sol` 归一成 `gpt-5.6` 后丢失 sol/terra/luna 价差；基础版本归一只用于没有精确价格时的候选，不应覆盖精确匹配。

### 数据库设计

不新增“每个模型一个字段”的列。

原因：

- 模型是高频变化的业务数据，不是稳定 schema。
- 现有数据库已经有通用字段：
  - `usage_logs.model`
  - `usage_logs.requested_model`
  - `usage_logs.upstream_model`
  - `usage_logs.input_tokens/output_tokens/cache_read_tokens/cache_creation_tokens`
  - `usage_logs.input_cost/output_cost/cache_read_cost/cache_creation_cost/total_cost`
  - `channel_model_pricing.models`
  - `channel_model_pricing.input_price/output_price/cache_read_price/cache_write_price`
  - `channel_pricing_intervals.min_tokens/max_tokens/...`
- 这些字段足够表达新模型、真实使用量和长上下文区间价格。

本次数据库侧设计：

- 不做 schema 变更。
- 不创建默认 channel，避免改变当前所有 OpenAI 分组的扣费来源。
- 如后续管理员创建 channel，可用现有 `channel_model_pricing.models` 放入 `["gpt-5.6-sol"]` 等模型名，用 `channel_pricing_intervals` 表达 0-272K 与 272K+ 两段价格。
- 本次实现只需要增加迁移级测试或 repository/service 测试，证明现有 DB 字段可存储三款新模型名及其 usage/cost。

如果业务强制要求“数据库里有默认价格记录”，应单独设计默认价格表或开启 channel 体系；这会改变当前默认计费架构，不应混在本次 GPT-5.6 支持里。

### 测试设计

必须先补失败测试，再实现：

- `backend/internal/pkg/openai`：`DefaultModels` 包含三款完整模型名，不包含 `gpt-5.6` alias。
- `backend/internal/handler`：OpenAI 分组 `/v1/models` 默认 fallback 返回三款 GPT-5.6。
- `backend/internal/service`：
  - `GetModelPricing("gpt-5.6-sol")` 返回官方 sol 价。
  - `GetModelPricing("gpt-5.6-terra")` 返回官方 terra 价。
  - `GetModelPricing("gpt-5.6-luna")` 返回官方 luna 价。
  - 三款模型 272K+ 触发长上下文倍率，cache read 同步按输入侧倍率计费。
  - `service_tier=flex/priority` 按现有 tier 规则计费。
  - 未知 `gpt-5.6-unknown` 不应误回退到 sol。
- 资源文件测试：内置 `model_prices_and_context_window.json` 可解析，且三款模型存在。
- 公网验收：
  - `/v1/models` 包含 `gpt-5.6-sol/terra/luna`。
  - `/v1/models` 不包含 `gpt-5.6`。
  - 三款模型各做一次 `/v1/responses` 最小真实请求，HTTP 200 且 usage log `requested_model/model/total_cost` 正常。

### 发布与回滚

发布方式：

- 按永久方案构建新镜像并替换 `sub2api-candidate` 应用容器。
- 不重建 Postgres/Redis/nginx/Cloudflare Tunnel。
- 发布前备份数据库，虽然本方案默认不写库，但真实请求验收会新增 usage log。

回滚方式：

- 回滚到发布前应用容器镜像即可。
- 若仅代码回滚，已产生的 GPT-5.6 usage log 保留，作为历史账单记录，不需要删除。

## 风险与取舍

- 官方价格可能在发布后调整：实施前必须再次核 OpenAI 官方 pricing 页。
- `gpt-5.6` alias 不展示可能让部分用户困惑，但这是为了保证“列表里出现的模型都能直接调用”。
- 如果远程 pricing 已更新但内置 fallback 未更新，离线场景会漏扣；因此本次必须同步更新 fallback。
- 如果后续要支持 batch 价格，需要先正式接入 `/v1/batch`，不能只写价格字段。

## 推荐结论

采用方案 C：完整模型名展示 + 原样调用 + 内置价格资源 + fallback 计费一起更新。

数据库不新增每模型字段；现有通用 usage 与 pricing 字段足够承载。若未来要把默认价格全部迁入 DB，应作为独立计费架构迁移处理。
