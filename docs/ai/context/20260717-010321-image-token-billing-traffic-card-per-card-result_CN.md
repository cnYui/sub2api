# 生图实际 Token 计费与逐张流量卡耗尽提醒实现结果

## 结论

本地分支 `codex/image-token-billing-traffic-card-events` 已完成 OpenAI 生图实际 Token 计费与逐张流量卡耗尽提醒实现。未部署，未连接或修改运行态 PostgreSQL、Redis、Nginx、容器、公网服务或用户数据。

## 最终行为

- OpenAI Responses、Chat Completions 兼容入口、Images、图片编辑与 WS 路径统一按最终可见尝试的实际 usage Token 结算。
- 生图不再按尺寸、图片数量、文件大小或独立图片倍率收费；多图只按上游返回的整次 usage Token 结算。
- 主模型和图片模型拆成两个计费组件：
  - 主模型：`input_tokens - cache_read_tokens - image_input_tokens`、`output_tokens - image_output_tokens`、缓存创建、缓存读取。
  - 图片模型：`image_input_tokens`、`image_output_tokens`。
- 图片模型缺失时回退 `gpt-image-2`。
- 缺失 Token 类别按 0 计费，并在 `usage_logs.billing_incomplete` 与 usage fact 的 `openai_billing.missing_usage_components` 中记录。
- 最终成功、失败、不完整、取消响应只要对客户端可见且带 billable Token，就先持久化 usage fact 再释放终止帧或响应体；内部 failover 中间尝试不结算。

## Schema 与配置

- 新增 migration `168_image_token_billing_and_traffic_credit_events.sql`：
  - `usage_logs.image_input_tokens`
  - `usage_logs.image_input_cost`
  - `usage_logs.billing_incomplete`
  - `traffic_credit_exhaustion_events`
  - 删除 `groups.image_rate_independent/image_rate_multiplier/image_price_1k/image_price_2k/image_price_4k`
- Ent schema 与生成代码已同步。
- `billing.traffic_credit_minimum_reserve_usd` 是流量卡 `$0.01` 门槛唯一来源，默认值为 `0.01`。

## 预留与实际扣费

- Images 请求在选择上游账号前完成 `AuthorizeImagesRequest`，按有效请求体、图片输入/输出上限、模型价格和套餐倍率预留。
- Responses/Chat/WS 继续复用前置预授权路径。
- 预留价格快照记录请求体 hash、模型、图片模型、Token 上限、倍率和价格。
- 响应后按实际 usage Token 生成 usage fact；实际费用低于预留会释放差额，高于预留会尝试补扣可用单卡余额，不足部分进入 debt。
- 已派发或 unknown 的 reservation 不会被过期任务或 defer release 错误释放。

## 逐张流量卡耗尽提醒

- 流量卡按 `expires_at, credited_at, id` 顺序使用，继续逐张预留和扣费，不合并成单一额度池。
- 单张卡余额从 `> $0.01` 降到 `<= $0.01` 时，在同一扣费事务内按 `(user_id, credit_id)` 幂等写入 `traffic_credit_exhaustion_events`。
- 一次请求耗尽多张卡会逐卡写事件；同一张卡只写一次。
- `/auth/me` 返回 pending `traffic_credit_exhaustion_notice.event_ids`；查询失败不影响用户资料响应。
- `POST /api/v1/user/traffic-credit-exhaustion-events/ack` 会校验事件 owner，防止越权确认。
- 前端会话内按事件 ID 去重，首次发现新事件时复用全局右上角 Toast，文案固定为“流量卡已用完”；ack 失败不阻断且下次可重试。
- notice 不写入 `auth_user` localStorage；退出登录会清空会话内去重 Set。用户买新卡后，未来新卡耗尽可再次提醒。

## 删除旧计费入口

- 后端运行时代码、DTO、缓存快照、仓储、API contract 和套餐生成脚本已删除旧图片固定价与独立倍率字段。
- 前端管理端已删除独立图片倍率和 1K/2K/4K 固定价控件。
- 用户侧说明改为“图片生成按上游实际返回的 Token 用量和套餐有效倍率计费；图片数量和文件大小不作为单独收费单位。”

## 本地审查结论

- Token 不重复计费：主模型组件剔除图片输入/输出 Token，图片组件独立计费；图片输入在 `computeTokenBreakdown` 内从 input 中拆分。
- 价格可重放：usage fact 保存 OpenAI billing component、pricing snapshot 与成本；reservation 保存请求预算快照。
- 终止响应前落 fact：非流式响应体和 SSE 终止帧由 usage fact response gate 持有，持久化失败会返回 `billing_persistence_error`。
- Images 已在上游前预留：`AuthorizeImagesRequest` 在账号选择和 `ForwardImages` 前执行。
- reservation item 没有被错误门槛过滤：预留规划以单卡 `AvailableUSD` 和统一 policy 计算，已耗尽卡不参与，未耗尽卡可按剩余额度预留。
- 耗尽事件和余额扣减同事务：reservation settlement、直接扣流量卡和补扣路径均在扣减后同事务调用 `recordTrafficCreditExhaustion`。
- ack 不越权：确认前逐个查询事件 owner，非本人或不存在均返回 invalid input。
- notice 不入 localStorage：auth store 解构剥离 `traffic_credit_exhaustion_notice` 后才写入 `AUTH_USER_KEY`。

## 验证结果

后端：

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler ./internal/repository ./internal/server
```

结果：`internal/service`、`internal/handler`、`internal/repository`、`internal/server` 均 `ok`。

```bash
GOMAXPROCS=2 go test -p=1 -count=1 -tags=integration ./internal/repository -run 'Test(MigrationsRunner|TrafficPackRepository|TrafficCreditReservationRepository|TrafficCreditExhaustionRepository|UsageBillingRepository)'
```

结果：`ok github.com/Wei-Shaw/sub2api/internal/repository 4.764s`。其中 `TestMigrationsRunner_AutoAPIKeyEffectiveGroupSeed` 在 migration 168 已删除旧图片列后跳过历史 migration 159 重放场景；不修改历史 migration。

```bash
GOMAXPROCS=2 go test -p=1 -count=1 ./cmd/server
```

结果：`ok github.com/Wei-Shaw/sub2api/cmd/server 0.923s`。

前端：

```bash
pnpm --dir frontend exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts src/stores/__tests__/auth.spec.ts src/api/__tests__/user.trafficCreditExhaustion.spec.ts src/views/user/__tests__/AvailableChannelsView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts src/views/admin/__tests__/groupsImagePricingRemoval.spec.ts
```

结果：7 个测试文件、82 个用例全部通过。测试日志中的损坏 localStorage 和 ack 网络错误为用例刻意覆盖的分支。

```bash
pnpm --dir frontend typecheck
```

结果：退出码 0。

```bash
pnpm --dir frontend build
```

结果：退出码 0；Vite 仅输出既有动态/静态 import、Browserslist 和 chunk size 警告。

静态检查：

```bash
rg -n 'image_rate_independent|image_rate_multiplier|image_price_1k|image_price_2k|image_price_4k' backend frontend --glob '!backend/migrations/*.sql' --glob '!backend/migrations/*_test.go'
```

结果：仅命中 schema/API/前端删除断言测试。

```bash
rg -n 'image_rate_independent|image_rate_multiplier|image_price_1k|image_price_2k|image_price_4k' backend frontend --glob '!backend/migrations/*.sql' --glob '!**/*_test.go' --glob '!**/*.spec.ts'
```

结果：无业务代码命中。

```bash
rg -n 'ImageCount > 0' backend/internal/handler backend/internal/service
```

结果：仍有非失败计费门槛命中，语义为通用非 OpenAI 显式 per_request/image 渠道、日志计费模式、图片模型或图片尺寸元数据判断；OpenAI 错误后是否结算已改为 `HasBillableOpenAIUsage`。

```bash
rg -n 'CalculateImageCost|resolveImageRateMultiplier' backend/internal
git diff --check
```

结果：旧固定价 helper 无命中；`git diff --check` 无输出。

## 未做事项

- 未补扣历史生图费用。
- 未给历史已耗尽流量卡补建提醒事件。
- 未推送、未创建 PR、未合并、未部署。
