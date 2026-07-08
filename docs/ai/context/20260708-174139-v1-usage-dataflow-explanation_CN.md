# /v1/usage 触发方式与数据流说明

## 结论

- `/v1/usage` 是 API Key 用量查询接口，不是模型请求入口，不会主动扣费，也不会写入 `usage_logs`。
- 前端 Key Usage 页面在用户输入 Key 并点击查询时触发：`GET /v1/usage`，请求头带 `Authorization: Bearer <key>`。
- 真实扣费发生在 `/v1/responses`、`/v1/chat/completions`、`/v1/messages` 等模型请求完成之后，由 `RecordUsage` 写账。
- `/v1/usage` 返回的数据有两套来源：
  - `usage.today`、`usage.total`、`daily_usage`、`model_stats` 来自 `usage_logs` 聚合，按 `api_key_id` 过滤。
  - `subscription.daily_usage_usd`、`weekly_usage_usd`、`monthly_usage_usd` 来自 `user_subscriptions` 窗口字段，修复后返回前做只读归一化。
- 之前断层的根因是自动 Key 的 `group_id=NULL`，访问 `/v1/usage` 时需要经过 effective group 解析，但旧的自动 Key endpoint policy 没把 `/v1/usage` 列为支持端点；同时 `/v1/usage` 是 `skipBilling`，不会走计费入口去补订阅上下文。

## 什么时候触发

主要触发方式：

1. 用户进入前端 Key Usage 页面，输入 API Key，点击查询。
2. 前端执行 `fetch('/v1/usage?...', { Authorization: 'Bearer ' + key })`。
3. 也可以由外部客户端直接调用同一个接口。

它不是定时任务，不会因为东八区 00:00 自动触发，也不会因为用户有模型请求而自动调用。模型请求完成后的写账链路和 `/v1/usage` 查询链路是两条路径。

## 后端请求链路

路由位置：

- `backend/internal/server/routes/gateway.go`
- `/v1` gateway 下注册 `GET /usage`。

中间件顺序大意：

1. body limit / request id / ops logger / endpoint normalization。
2. API Key 鉴权。
3. 自动 Key effective group 解析。
4. group 可用性检查。
5. `GatewayHandler.Usage` 返回 JSON。

API Key 鉴权做的事：

- 从 `Authorization: Bearer`、`x-api-key`、`x-goog-api-key` 取 Key。
- 查 `api_keys`。
- 验 Key 是否存在、用户是否 active、IP ACL、固定 group 是否可用。
- 对 `/v1/usage` 设置 `skipBilling=true`，所以不拦截过期/配额耗尽 Key，也不执行订阅额度准入。
- 写入 request context：
  - `ContextKeyAPIKey`
  - `ContextKeyUser`
  - `ContextKeyUserRole`
  - 固定订阅 Key 场景下可能还有 `ContextKeySubscription`

自动 Key 解析做的事：

- 如果 `api_keys.group_id` 不为空，说明是固定 group Key，直接跳过自动解析。
- 如果 `group_id=NULL`，说明是自动 Key，需要根据当前 endpoint 判断能否解析 effective group。
- 现在 `/v1/usage` 被 policy 明确识别为 OpenAI 支持端点。
- 解析器优先找用户 active OpenAI 订阅；没有订阅再看 OpenAI/GPT 流量包。
- 解析成功后只在当前 request 内把 `apiKey.GroupID/apiKey.Group` 补上，并把订阅写到 `ContextKeySubscription`；不会把自动 Key 的 `group_id` 写回数据库。

## Handler 返回字段

`GatewayHandler.Usage` 从 context 取：

- `apiKey`
- `AuthSubject`，主要是 `user_id`

然后按 API Key 聚合用量：

- `usage.today.requests`
- `usage.today.input_tokens`
- `usage.today.output_tokens`
- `usage.today.cache_creation_tokens`
- `usage.today.cache_read_tokens`
- `usage.today.total_tokens`
- `usage.today.cost`
- `usage.today.actual_cost`
- `usage.total.*`
- `usage.average_duration_ms`
- `usage.rpm`
- `usage.tpm`

其中 `usage.today` 的 SQL 条件是：

- `api_key_id = 当前 Key`
- `created_at >= timezone.Today()`

`timezone.Today()` 是应用配置时区当天 00:00；当前目标是东八区，也就是东八区当天 00:00。

`daily_usage` 是最近 N 天每日趋势，仍然来自 `usage_logs`，按 `user_id + api_key_id + 时间范围` 聚合。

`model_stats` 是指定日期范围内按模型维度聚合，来自 `usage_logs`。

模式判断：

- 如果 Key 自身配置了 `quota` 或 rate limit，返回 `mode=quota_limited`，展示 Key 级额度。
- 否则返回 `mode=unrestricted`。

订阅型 unrestricted 返回：

- `planName`
- `remaining`
- `subscription.daily_usage_usd`
- `subscription.weekly_usage_usd`
- `subscription.monthly_usage_usd`
- `subscription.daily_limit_usd`
- `subscription.weekly_limit_usd`
- `subscription.monthly_limit_usd`
- `subscription.expires_at`

余额型 unrestricted 返回：

- `balance`
- `remaining`
- `planName=钱包余额`

## 前端展示链路

前端文件：

- `frontend/src/views/KeyUsageView.vue`

展示来源：

- 环形额度图读 `data.subscription.daily_usage_usd / daily_limit_usd` 等字段。
- 详情行也读同一组 `subscription` 字段。
- 今日请求数、今日 token、今日费用读 `data.usage.today`。
- 历史总计读 `data.usage.total`。

所以前端本身不判断窗口是否过期，它只消费后端返回的字段。后端如果把 stale 的 `daily_usage_usd` 返回给前端，前端就会展示昨天残留；后端如果没有返回 `subscription` 块，前端就只会展示统计区或余额区，订阅额度块会缺失。

## 为什么之前会断层

断层由两个条件叠加产生：

1. 自动 Key 没有固定 `group_id`

自动 Key 的 `api_keys.group_id=NULL`。普通模型请求可以根据路径如 `/v1/responses`、`/v1/chat/completions` 自动解析 effective group。但旧 policy 里没有 `/v1/usage`，所以自动 Key 查用量时会被认为“不支持自动 Key 的 endpoint”，常见结果是 `AUTO_KEY_UNSUPPORTED_ENDPOINT`，或者无法拿到 `apiKey.Group` 和 `ContextKeySubscription`。

2. `/v1/usage` 是 skipBilling

这是故意设计的：过期或额度耗尽的 Key 也应该能查询自身用量。所以 `/v1/usage` 不走计费准入，也不会借由计费检查去加载/刷新订阅上下文。

因此固定 group Key 可能没问题，自动 Key 会断。因为固定 group Key 在 API Key 鉴权时能按 `user_id + group_id` 直接加载订阅；自动 Key 必须依赖 effective group resolver。

## 为什么可能把昨天混进今天展示

`usage.today` 不会混昨天，因为它按 `usage_logs.created_at >= 今天 00:00` 聚合。

容易混昨天的是 `subscription.daily_usage_usd`：

- 它是 `user_subscriptions` 上的窗口缓存字段。
- 如果 `daily_window_start` 还是昨天 00:00，且 `daily_usage_usd` 还是昨天累计值，旧 handler 直接返回这个字段，前端就会显示昨天的订阅已用量。
- 这和真实 `usage_logs` 今日聚合不是同一套数据源，所以会出现“今日统计是 0，但订阅日额度环形图显示昨天用量”的错觉。

修复后 `/v1/usage` 返回前会做只读归一化：

- 如果 `daily_window_start < 今天 00:00`，响应里的 `daily_usage_usd` 置为 0。
- 如果 `weekly_window_start < 本周开始`，响应里的 `weekly_usage_usd` 置为 0。
- 如果 `monthly_window_start + 30 days <= now`，响应里的 `monthly_usage_usd` 置为 0。

这个归一化只影响返回 JSON，不写数据库。

## 真实扣费如何进入今天额度

模型请求完成后，handler 调用 `RecordUsage`：

1. 根据上游返回 usage 计算成本。
2. 构造 `usageLog`，包括：
   - `user_id`
   - `api_key_id`
   - `account_id`
   - `request_id`
   - `model/requested_model/upstream_model`
   - `group_id`
   - `subscription_id`
   - token 字段
   - `total_cost`
   - `actual_cost`
   - `billing_type`
   - `stream`
   - `created_at`
3. 构造 `UsageBillingCommand`。
4. `UsageBillingCommand.CompletedAt` 使用 `usageLog.CreatedAt`。
5. 订阅扣费时按 `CompletedAt` 计算东八区 day/week/month window。
6. 如果完成时间已过东八区 00:00，且订阅表还是昨天窗口，DB 更新会把 daily window 推到今天，并把本次 cost 作为今天的初始用量。

所以“23:59 发起，00:00 后完成”的请求按完成时计入今天。这符合当前定论。

## 后台校准如何补齐所有 active 订阅

后台 scheduler 启动时跑一次，之后每分钟跑一次：

- 取 `dailyStart = timezone.StartOfDay(now)`。
- 取 `upperBound = now`。
- 批量选出 stale active 订阅：
  - `deleted_at IS NULL`
  - `status = active`
  - `expires_at > now`
  - `daily_window_start IS NULL OR daily_window_start < dailyStart`
- 聚合今天 `usage_logs`：
  - `subscription_id = 当前订阅`
  - `created_at >= dailyStart`
  - `created_at < upperBound`
- 把 `user_subscriptions.daily_usage_usd` 覆盖为今天聚合值，把 `daily_window_start` 推到今天。
- 对更新过的订阅失效 billing cache。
- 如果一轮结束仍有 stale 订阅，打印 `ALERT` 日志。

这个任务是幂等的：重复运行会把同一个订阅覆盖成“今天 usage_logs 聚合值”，不会叠加。

## 当前防线

- `/v1/usage` 已纳入自动 Key endpoint policy。
- 自动 Key 查询 `/v1/usage` 时会解析 effective group，并把订阅上下文写入当前 request。
- `/v1/usage` 返回前做窗口只读归一化，避免展示昨天残留。
- 真实扣费按完成时间推进订阅窗口，避免 API 入口惰性刷新重复清零。
- 后台 scheduler 主动校准所有 active 订阅窗口，避免长期 stale。

## 本轮未改动

本轮只读核对并补充说明文档，未修改业务代码、未运行迁移、未部署。
