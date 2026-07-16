# 生图实际 Token 计费与逐张流量卡耗尽提醒设计

## 状态

- 日期：2026-07-16
- 状态：设计已由用户口头确认，待用户审核本文档
- 范围：本地 `main` 代码设计，不部署、不修改运行态数据库、Redis、容器或配置
- 前置基础：本地 `main` 已包含 `usage_facts` durable outbox、流量卡 reservation/items、`reserved_usd` 和 debt gate

## 背景

当前 OpenAI 生图请求虽然已经能从部分上游响应中提取文本、缓存和图片 Token，但只要结果中存在图片，结算仍可能进入按图片数量和 `1K/2K/4K` 尺寸计算固定费用的路径。这会覆盖主模型 Token 成本，也无法准确反映 Responses 中“主模型推理 + 图片工具模型生成”的真实成本。

流量卡当前在数据库中按购买批次逐张保存，每张卡都有独立 `user_traffic_credits.id`、初始额度、剩余额度、预留额度、购买时间和到期时间。预留与结算也通过 `traffic_credit_reservation_items.credit_id` 分配到具体卡片，并按“先到期、同到期时先购买、最后按 ID”消费。当前用户页面却将所有有效流量卡汇总成一张 `GPT 流量包` 卡片，不能显示单张卡的消耗和耗尽状态。

本文设计同时解决以下问题：

1. 生图按上游实际返回 Token 结算，不再按尺寸或图片数量固定收费。
2. Responses 生图分别按主模型和图片工具模型价格计算。
3. 流量卡继续逐张预留、逐张扣费，并在用户页面逐张展示。
4. 单张流量卡余额降到 `$0.01` 时，即使请求成功，也按该卡 `credit_id` 记录一次耗尽事件。
5. 同一请求耗尽多张卡时，事件逐卡保存，但右上角只显示一次“流量卡已用完”。

## 已确认决策

### 生图计费

- 取消图片固定费、最低图片消费、按图片数量收费和 `1K/2K/4K` 客户价。
- 一次请求多图直接使用上游返回的整次 Token，不再乘图片数。
- 计费包含所有可确认 Token：主模型输入、缓存创建、缓存读取、文本输出、图片输入和图片输出。
- Responses 生图拆分模型定价：主模型 Token 按实际主模型价格，图片 Token 按实际图片工具模型价格；无法确认图片工具模型时回退 `gpt-image-2`。
- 所有 Token 使用现有统一有效倍率，不再存在独立生图倍率。
- 上游缺少某类 Token 时，该类按 0 计费，不估算、不使用固定价兜底，并记录 `billing_incomplete=true`。
- 失败请求只要返回可确认 Token，也按实际 Token 收费；完全没有 Token 才是 0 费用。
- 订阅、余额、流量卡、usage log 和账号统计继续复用普通请求的统一结算路径。
- 流量卡保留请求前预留，响应后按实际 Token 成本结算并释放差额；预留不是最终收费。

### 流量卡

- 每次购买形成独立流量卡记录和独立 `credit_id`，不把多张卡合并成一张持久化额度池。
- 保留当前消费顺序：先到期优先，同到期时先购买优先，最后按 `credit_id` 排序。
- 用户页面按单张流量卡展示，不再只展示汇总卡片。
- 单张卡实际剩余额度 `<= $0.01` 后视为耗尽，不再参与后续预留和扣费，并从用户页面消失。
- 流量卡自然过期只消失，不产生“流量卡已用完”提醒。
- 从未购买过流量卡的用户不会收到该提醒。

### 提醒

- 单张卡从 `> $0.01` 结算到 `<= $0.01` 时，即使 API 请求成功，也创建该 `credit_id` 的耗尽事件。
- 同一 `credit_id` 最多创建一次耗尽事件。
- 一次请求可以耗尽多张卡，后端逐卡记录事件，但前端合并为一个右上角错误 Toast。
- Toast 固定显示“流量卡已用完”，不显示 debt、请求预算或“请求超额”等信息。
- 用户购买的新卡有新的 `credit_id`，新卡耗尽后可以再次提醒。

## 目标架构

### 1. 归一化 Token 明细

扩展 OpenAI 用量归一化结果，使一次请求明确表达以下两类计费组件：

```text
主模型组件
  model
  input_tokens
  cache_creation_tokens
  cache_read_tokens
  text_output_tokens

图片工具组件
  model
  image_input_tokens
  image_output_tokens
```

现有 `OpenAIUsage` 和 `UsageTokens` 已包含大部分字段，但当前成本计算只能绑定一个 billing model。实现时应增加明确的计费组件结构，由统一计费器分别计算主模型与图片模型，再合并为一个 `CostBreakdown`。不要在 handler 中拼接金额，也不要继续使用 `ImageCount` 乘固定单价。

`output_tokens` 若包含 `image_output_tokens`，文本输出 Token 应按 `max(output_tokens - image_output_tokens, 0)` 计算，避免重复收费。图片输入 Token 同理需要从总输入中拆分，不能同时按主模型输入价和图片输入价重复收费。

### 2. 价格快照

请求前预授权和响应后结算必须使用同一套可重放价格事实。价格快照必须包含：

- 主模型名称及输入、输出、缓存价格；
- 图片工具模型名称及图片输入、图片输出价格；
- service tier；
- 用户/分组统一有效倍率；
- 请求预算、输出上限和最终出站 body 指纹；
- 哪些 Token 类别缺失，以及 `billing_incomplete` 原因。

流量卡 reservation 继续保存请求前快照。usage fact payload 保存最终 Token 明细、最终成本和完整性标记，durable worker 重放时不得重新读取已经变化的实时价格。

### 3. 统一结算

所有 OpenAI 入口最终构造同一种 `UsageFactPayload`：

```text
上游响应或终止事件
  -> 归一化 Token 组件
  -> 使用价格快照计算标准成本和实际成本
  -> 同步持久化 usage fact
  -> reservation/订阅/余额/流量卡统一结算
  -> 写 usage_logs 与结算副作用
```

Images、Responses image tool、Chat Completions 转换路径和图片编辑都不得再进入 `CalculateImageCost()`。失败响应只要能够提取 Token，也要生成 usage fact；无 Token 的失败响应不生成费用，但可以继续保留现有错误日志。

## 删除旧尺寸价格与独立倍率

用户已明确允许直接删除以下 `groups` 字段：

- `image_rate_independent`
- `image_rate_multiplier`
- `image_price_1k`
- `image_price_2k`
- `image_price_4k`

删除范围包括：

- Ent schema 与重新生成的 Ent 代码；
- service/domain/group DTO；
- API 请求和响应字段；
- API Key 鉴权缓存快照；
- 管理端分组创建、编辑和价格预览控件；
- 用户侧 `1K/2K/4K` 价格展示；
- OpenAI 生图固定价格 helper 和相关业务测试。

历史 migration 文件必须保持不变，不能删除其中的旧列创建或 seed 语句，否则已部署数据库会发生 migration checksum mismatch。实现时新增下一可用 migration，在所有历史 migration 之后执行 `DROP COLUMN`，并增加 schema integration 断言确认五个字段已经不存在。迁移编号必须避开并行任务已经计划使用的 `166/167`，以实现时仓库的下一可用编号为准。

通用渠道 token 定价中的 `ImageInputPricePerToken`、`ImageOutputPricePerToken` 必须保留，它们是本次新计费规则需要的 Token 单价，不属于被删除的尺寸固定价。非 OpenAI 平台仍在使用的通用 `per_request` 能力不在本次删除范围，但 OpenAI 生图路径不得再使用它。

## 逐张流量卡模型

### 查询与展示

在现有汇总之外增加用户自己的逐卡列表，单项包含：

```json
{
  "id": 123,
  "pack_id": 2,
  "order_id": 456,
  "initial_usd": 10,
  "remaining_usd": 6.42,
  "reserved_usd": 0,
  "available_usd": 6.42,
  "credited_at": "...",
  "expires_at": "..."
}
```

只返回当前用户自己的未过期卡片。`available_usd` 用于展示并扣除尚未结算的预留，但耗尽事件必须以事务结算后的 `remaining_usd` 为准，不能因为临时预留而误报耗尽。

前端 `SubscriptionsView` 按后端顺序逐张渲染 `SubscriptionUsageCard`。每张卡展示初始额度、已用额度、剩余额度和真实到期时间。`remaining_usd <= $0.01` 的卡不返回或不展示，不能继续显示为汇总中的零头。

### 预留与扣费

预留和结算继续按具体 `credit_id` 分配：

1. 按 `expires_at, credited_at, id` 锁定可用卡片。
2. 已经 `remaining_usd <= $0.01` 的卡不参与规划。
3. 一次请求预算可以跨多张卡预留，items 继续记录每张卡的预留金额。
4. 结算按 items 顺序消费，实际成本低于预留时逐卡释放差额。
5. 实际成本高于预留时，继续按相同顺序补扣其他可用卡；不足部分保留现有 debt 行为。
6. 只有事务提交后的实际 `remaining_usd` 跨过门槛，才产生耗尽事件。

`$0.01` 使用现有 `billing.traffic_credit_minimum_reserve_usd` 配置作为单一事实来源。repository/service/UI 不得分别硬编码第二套阈值。前端只消费后端返回的卡片状态和事件，不自行比较浮点金额。

## 耗尽事件与 Toast

### 数据表

新增 `traffic_credit_exhaustion_events`，字段如下：

- `id BIGSERIAL PRIMARY KEY`
- `user_id BIGINT NOT NULL`
- `credit_id BIGINT NOT NULL`
- `request_id VARCHAR(255) NOT NULL`
- `batch_key VARCHAR(255) NOT NULL`，同一次结算耗尽多张卡时相同
- `reason VARCHAR(32) NOT NULL DEFAULT 'depleted'`
- `created_at TIMESTAMPTZ NOT NULL`
- `acknowledged_at TIMESTAMPTZ NULL`

约束与索引：

- `UNIQUE(user_id, credit_id)`，保证单张卡只提醒一次；
- pending 查询索引覆盖 `user_id, acknowledged_at, created_at`；
- `credit_id` 外键指向 `user_traffic_credits.id`，卡片记录不能物理删除。

事件写入与流量卡结算必须在同一个数据库事务中完成。结算重试、usage fact 重放和并发 worker 都通过唯一约束实现幂等。

### 事件投递

`GET /auth/me` 在普通用户响应中增加可选字段 `traffic_credit_exhaustion_notice`。没有待确认事件时省略该字段；存在事件时返回固定结构：

```json
{
  "traffic_credit_exhaustion_notice": {
    "event_ids": [101, 102]
  }
}
```

完整流量卡财务明细仍由支付/订阅接口返回，不混入用户 DTO。前端现有 auth store 每 60 秒刷新，因此外部 CLI/API 请求产生的耗尽事件会在浏览器打开时最多约 60 秒内显示；用户重新登录时会立即获取。

确认接口固定为 `POST /api/v1/user/traffic-credit-exhaustion-events/ack`，请求体为：

```json
{
  "event_ids": [101, 102]
}
```

后端只确认属于当前登录用户的事件；空列表、非法 ID 或越权 ID 返回参数错误，不部分确认。

前端收到一个或多个待确认事件时：

1. 只调用一次 `appStore.showError('流量卡已用完')`；
2. 批量调用确认接口，将本次取得的全部事件标记为已确认；
3. 会话内按事件 ID 再做一次去重，避免确认请求失败导致同页重复 Toast；
4. 后续出现新的 `credit_id` 事件时再次显示。

如果用户在看到旧事件前已经购买新卡，购买履约事务应将当前旧的 pending 耗尽事件标记为已确认，避免购卡成功后再弹陈旧提示。新卡使用新的 `credit_id`，不影响未来提醒。

## `billing_incomplete`

`usage_logs` 增加：

- `image_input_tokens INTEGER NOT NULL DEFAULT 0`
- `billing_incomplete BOOLEAN NOT NULL DEFAULT FALSE`

usage fact payload额外保存缺失类别列表，例如 `missing_usage_components: ["image_input_tokens"]`。`billing_incomplete` 只表示成本可能偏低，不触发固定价补扣，也不阻断已成功响应。运营侧后续可以按该字段统计上游 usage 完整率。

## 错误处理

- 卡片临时被其他请求预留，不产生耗尽事件。
- 单次请求预算高于当前可用额度，但仍有单张卡余额明显高于 `$0.01` 时，保留现有 quota/preauthorization 错误，不显示“流量卡已用完”。
- debt gate 继续阻止新请求；如果导致 debt 的结算同时把相关卡片降到门槛，则耗尽事件已经在结算事务中创建，不需要等下一次 402。
- 流量卡自然过期不创建事件。
- 事件查询或确认失败不影响 API 计费、用户鉴权和页面其他数据，只记录错误并在下次刷新重试。

## 测试策略

### 后端单元测试

- 主模型 Token 和图片工具 Token 分别按不同模型价格计算。
- 缓存、文本输出、图片输入和图片输出不会重复计费。
- 多图只使用整次 Token，不乘图片数。
- 失败请求有 Token 时计费，无 Token 时费用为 0。
- 缺失 Token 类别按 0 并设置 `billing_incomplete`。
- OpenAI 生图不再调用尺寸固定价或独立倍率。
- `$0.01` 单卡边界：`> 0.01` 可用，`<= 0.01` 不再参与后续规划。
- 同一请求耗尽一张、多张和没有耗尽卡片的事件结果。
- 同一 `credit_id` 重放和并发写入只产生一条事件。

### 仓储与迁移集成测试

- 多张卡按 `expires_at, credited_at, id` 排序预留和结算。
- 结算跨多张卡时每张卡 ledger、reservation item 和余额正确。
- 事务回滚时余额与耗尽事件一起回滚。
- usage fact 重放不重复扣费、不重复建事件。
- 购卡履约关闭旧 pending 事件，新卡以后仍可生成新事件。
- 新 migration 后五个旧 group 字段不存在，新增事件表和 usage log 字段存在。

### 前端测试

- 多张流量卡逐张展示，顺序和后端一致。
- 单张卡显示真实初始额度、剩余额度和到期时间。
- 耗尽卡不显示，剩余卡继续显示。
- 一个 pending 事件显示一次 Toast。
- 同批或同时返回多个事件只显示一次 Toast，并批量确认。
- 同一事件重复刷新不再次显示，新事件会再次显示。
- 普通额度不足、自然过期和从未购卡不显示该 Toast。
- 管理端和用户侧不再出现尺寸固定价与独立倍率字段。

### 回归验证

- 后端 service、handler、repository 目标测试；
- migration schema integration；
- `./cmd/server` 编译；
- 前端目标 Vitest、typecheck、build；
- Ent 代码生成后工作区一致性检查；
- `git diff --check`；
- 不连接或修改运行态数据库、Redis、Nginx、容器和公网服务。

## 不在本次范围

- 不部署或启用生产 reservation 强制模式。
- 不补扣历史生图请求。
- 不修改流量卡售价、有效期、支付和退款规则。
- 不修改订阅和余额的优先级。
- 不为流量卡耗尽增加邮件、短信或站内消息中心。
- 不删除非 OpenAI 平台仍需要的通用 token 定价字段。

## 设计取舍

本设计选择“实际 Token + 价格快照 + durable usage fact”，而不是固定图片价，原因是 Responses 生图中主模型成本可能显著高于图片本身，固定价无法稳定覆盖真实成本。流量卡耗尽按单张 `credit_id` 记录，而不是按用户汇总池记录，是因为当前财务事实、预留 items、ledger 和消费顺序本来就以单张卡为边界。Toast 只做展示合并，不改变逐卡财务事实。

`docs/ai/context/20260716-142225-image-generation-pricing-research_CN.md` 是调研阶段建议，其中固定最低图片价、按图片数和缺失 Token 固定价兜底的方向已被本次用户确认决策取代；实施应以本文为准。
