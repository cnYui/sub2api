# 自动 API Key 与 effective_group 运行时解析设计

## 目标

把普通用户 API Key 从“创建时绑定固定 group”改为“创建时不选分组，运行时根据用户当前权益解析 effective_group”。

用户期望：

- 前端普通用户创建 API Key 时删除分组下拉框。
- 普通用户创建 Key 默认 `group_id=NULL`。
- 请求进来后，后端按用户当前权益实时解析 `effective_group`。
- 有 active OpenAI 套餐时，走当前套餐 group。
- 没有套餐但有 GPT/OpenAI 流量包时，走受控的 OpenAI 流量包入口 group。
- 用户升级、退款、重购套餐后，同一个 API Key 下次请求自动按新权益使用，不需要重建 Key。
- 旧的已创建 OpenAI API Key 也迁移为这种自动 Key。

非目标：

- 不把 GPT 流量包伪造成 `user_subscriptions`。
- 不让单独购买流量包的用户显示到管理员 subscriptions 页面。
- 不改变订阅、流量包、订单、用量日志的事实源边界。
- 不移除管理员后台固定分组管理能力。

## 当前约束

现有 `group_id` 不是普通 UI 字段，它同时承担：

- 平台识别：OpenAI、Anthropic、Gemini、Antigravity。
- 网关路由：`/responses`、`/chat/completions`、`/images/*` 等根据 `apiKey.Group.Platform` 分发到 OpenAI 或兼容网关。
- 上游账号池选择：账号调度通过 group 过滤账号。
- 价格与倍率：group 默认倍率、用户专属 group 倍率、渠道定价都依赖 group。
- 限额与 RPM：订阅额度、group RPM、用户 group RPM override 依赖 group。
- 日志与后扣：`usage_logs.group_id`、`billing_usage_entries` 需要记录本次请求实际使用的 group。

因此不能只删前端下拉框。后端必须在请求级别解析出一个临时 `effective_group`，并让后续路由、计费、调度看到它。

## 推荐方案

新增 `EffectiveGroupResolver`，把“普通用户自动 Key 的运行时分组选择”收敛到单一服务。

核心规则：

1. 如果 API Key 本身有固定 `group_id`，默认保留现有行为。
2. 如果 API Key 是自动 Key（`group_id=NULL`），根据本次请求目标平台解析 `effective_group`。
3. OpenAI 平台解析优先级：
   - 用户有 active OpenAI subscription：选择 active subscription 对应的 group。
   - 用户没有 active OpenAI subscription，但有可用 OpenAI/GPT 流量包：选择受控的 OpenAI 流量包入口 group。
   - 都没有：返回明确错误，不进入上游调度。
4. 解析出的 `effective_group` 只在当前请求上下文生效，不回写 `api_keys.group_id`。
5. 本次请求的用量日志、计费记录和账号选择都使用 `effective_group`。

### 多套餐选择规则

如果用户同时有多个 active OpenAI subscriptions，必须确定性选择，不允许依赖数据库返回顺序。

推荐规则：

1. 优先选择未过期且 group active 的 OpenAI subscription。
2. 按额度从高到低排序；无限额度或 `daily_limit_usd IS NULL` 视为最高。
3. 额度相同时选择最近创建或最近分配的 subscription。
4. 仍相同时按 subscription id 倒序。

这个规则符合升级场景：用户购买更高套餐后，自动 Key 优先使用更高权益。

## OpenAI 流量包入口 group

推荐新增一个明确的内部 OpenAI standard group，例如：

- name/code：`traffic-pack-openai`
- platform：`openai`
- subscription_type：`standard`
- is_exclusive：`true`
- status：`active`
- rate_multiplier：沿用 OpenAI 默认倍率
- account binding：绑定当前 OpenAI 上游账号池，例如现有 `cliproxy-local-openai`

为什么不用现有套餐 group：

- 现有 `codex-pool-19-usd`、`codex-pool-69-usd` 等是 subscription group，语义是套餐。
- 流量包是用户级平台余额，不应在日志和管理上伪装成某个套餐。
- 独立入口 group 可以让用量日志清楚显示“这次是流量包入口”，但实际扣费仍由 `traffic_credit_ledger` 决定。

为什么设为 exclusive：

- 普通用户不应在可选分组列表中直接看到或手动绑定它。
- 自动 resolver 可以内部选择它，不依赖 `user_allowed_groups`。
- 管理员仍能在后台看到并维护这个 group 和账号绑定。

## 中间件与请求数据流

推荐在 API Key 鉴权后、订阅加载和 `RequireGroupAssignment` 前执行 effective group 解析。

理想顺序：

1. API Key 鉴权：验证 Key、用户状态、Key 状态。
2. 如果 `apiKey.GroupID == nil`，调用 `EffectiveGroupResolver.Resolve(ctx, apiKey, request)`。
3. Resolver 根据路径、ForcePlatform 或协议推断目标平台；本期重点支持 OpenAI。
4. Resolver 返回：
   - `effective_group`
   - 可选 `effective_subscription`
   - `source`：`subscription` / `traffic_pack` / `fixed`
5. 中间件把请求上下文中的 API Key 浅拷贝为“带 effective group 的 API Key”，避免污染 auth cache。
6. 设置 `ctxkey.Group`、middleware API Key context、subscription context。
7. 后续 `RequireGroupAssignment`、路由分发、计费检查、账号调度、后扣记录继续读取 `apiKey.Group`，不需要各自重新发明规则。

注意：不能直接修改 auth cache 里复用的 APIKey 指针；必须 clone 或构造 request-scoped copy。

## OpenAI 端点覆盖范围

本期自动 group 解析应覆盖所有 OpenAI/GPT 用户实际使用入口：

- `/v1/responses`
- `/responses`
- `/backend-api/codex/responses`
- `/v1/chat/completions`
- `/chat/completions`
- `/v1/embeddings`
- `/embeddings`
- `/v1/images/generations`
- `/v1/images/edits`
- `/images/generations`
- `/images/edits`
- `/v1/messages`：如果用户有 OpenAI entitlement，解析为 OpenAI group 后继续走现有 OpenAI Messages Dispatch。

Gemini、Antigravity、Anthropic 的自动权益解析不在本期范围。旧的非 OpenAI 固定分组 Key 保持原行为。

## 计费与日志

订阅用户：

- Resolver 选择 active subscription 的 OpenAI group。
- subscription context 设置为该 active subscription。
- `BillingCacheService.CheckBillingEligibility()` 按订阅限额检查。
- 后扣仍记录 `subscription_id`，计费类型为 subscription。

流量包用户：

- Resolver 选择 `traffic-pack-openai` effective group。
- subscription context 为 nil。
- `BillingCacheService.CheckBillingEligibility()` 在余额不足时允许 OpenAI 流量包兜底。
- 后扣通过 `shouldBillWithTrafficPack()` 和 `TrafficPackService.Deduct()` 写 `traffic_credit_ledger`。
- `usage_logs.group_id` 记录 `traffic-pack-openai` group id，`subscription_id=NULL`。

无权益用户：

- 不进入上游调度。
- OpenAI 端点返回明确错误，例如 `NO_OPENAI_ENTITLEMENT`，提示需要购买套餐或 GPT 流量包。

## 前端设计

普通用户 `KeysView`：

- 创建/编辑 API Key 弹窗删除 group 下拉框。
- 删除创建时 `group_id` 必填校验。
- 创建请求不传 `group_id`。
- Key 列表里原本展示 group 的位置显示：
  - `自动` 或 `自动分组`
  - 辅助文案：`按当前套餐或 GPT 流量包自动使用`
- 删除普通用户侧快速切换 group 的下拉菜单。
- 分组筛选对普通用户不再有意义，建议隐藏或改为只读展示历史固定 Key。

管理员后台：

- 管理员 API Key 管理、用户 Key 详情、手动迁移、固定分组能力保留。
- 管理员可以看到 `group_id=NULL` 的自动 Key。
- 管理员可以按需把某个 Key 改成固定 group，但普通用户不能自己改。

## 普通用户 API 行为

`POST /api/v1/keys`：

- 普通用户创建 Key 时后端忽略或拒绝 `group_id` 入参。
- 推荐策略：忽略普通用户传入的 `group_id`，始终创建 `group_id=NULL`，返回结果里显示为自动 Key。这样旧前端或旧脚本不会因为仍传 group_id 而创建失败。

`PUT /api/v1/keys/:id`：

- 普通用户不再允许修改 `group_id`。
- 推荐策略：如果请求携带 `group_id`，忽略该字段，其他字段照常更新。

管理员 API：

- `/admin/api-keys/:id` 等后台接口保留固定 group 更新能力。

## 旧 OpenAI API Key 迁移

上线时需要一次性把旧 OpenAI Key 转为自动 Key：

```sql
UPDATE api_keys ak
SET group_id = NULL, updated_at = NOW()
FROM groups g
WHERE ak.group_id = g.id
  AND g.platform = 'openai'
  AND ak.deleted_at IS NULL;
```

要求：

- 执行前备份公网数据库。
- 只迁移当前仍存在的 OpenAI group Key。
- 不迁移非 OpenAI 固定 Key，例如 `anthropic/default`。
- 不修改历史 `usage_logs` 和 `billing_usage_entries`。
- 迁移后需要失效 API Key auth cache，避免旧 group 缓存在请求中继续生效。

灰度建议：

1. 先在 18080 或候选环境跑迁移和真实请求。
2. 验证已有套餐用户旧 Key 请求 `/v1/responses` 仍走当前 active subscription。
3. 验证仅流量包用户旧 Key 或新 Key 请求 `/v1/responses` 走 `traffic-pack-openai` 并扣流量包。
4. 再替换公网应用并执行公网库迁移。

## 错误处理

- 自动 Key 调 OpenAI 端点但无套餐、无流量包：返回 403 `NO_OPENAI_ENTITLEMENT`。
- 自动 Key 调 OpenAI 端点但 traffic-pack group 未配置或未绑定账号：返回 503 `OPENAI_TRAFFIC_GROUP_UNAVAILABLE`，并写 ops error。
- 自动 Key 解析 active subscription 时遇到 DB 错误：返回 500，不 fallback 到流量包，避免错误状态下误扣。
- active subscription 已过期但定时任务未更新 status：Resolver 以 `expires_at > now` 为准。
- traffic pack 余额存在但扣费时不足：按现有后扣失败处理策略记录告警，不应提前放过已知余额不足。

## 测试计划

后端单元测试：

- 自动 Key + active OpenAI subscription 解析到套餐 group。
- 自动 Key + 多个 active OpenAI subscriptions 选择最高额度/最新订阅。
- 自动 Key + 无订阅 + OpenAI 流量包解析到 `traffic-pack-openai`。
- 自动 Key + 无订阅 + 无流量包返回 `NO_OPENAI_ENTITLEMENT`。
- 固定 group Key 不走 resolver，保持现有行为。
- resolver 不污染 auth cache 中的 APIKey 指针。
- 旧 OpenAI Key 迁移 SQL 只清空 OpenAI group，不影响 Anthropic/Gemini/Antigravity Key。

后端集成测试：

- `/v1/responses`、`/v1/chat/completions`、`/v1/images/generations` 自动 Key 均能进入 OpenAI handler。
- 套餐用户请求后 `usage_logs.subscription_id` 非空。
- 流量包用户请求后 `usage_logs.subscription_id=NULL`，`traffic_credit_ledger` 新增 deduction。
- 无权益用户不会进入上游账号调度。

前端测试：

- 创建 Key 弹窗不再渲染 group 下拉框。
- 创建 Key 请求不传 `group_id`。
- group 必填校验被删除。
- Key 列表显示自动分组文案。
- 普通用户无法打开快速 group 切换下拉。

运行态验收：

- `1930863755@qq.com` 购买了 5 USD 流量包，无 subscription，无 API Key；创建自动 Key 后真实 `/v1/responses` 应返回 200 并扣流量包。
- 一个 active 79 元套餐用户使用旧迁移 Key 请求 `/v1/responses` 应走 `codex-pool-69-usd` subscription，不扣流量包。
- 一个已退款/重购用户使用同一 Key 能自动切到新套餐 group。

## 风险与取舍

- 这是跨鉴权、路由、计费、前端的共享行为变更，不能只做 UI 修改。
- 旧的 `allow_ungrouped_key_scheduling` 不是本方案的主能力；它表示未分组 Key 调度未分组账号，会绕开套餐 group 账号池，不适合 GPT 套餐和流量包。
- 运行时 resolver 增加一次订阅/流量包查询，需要缓存或复用现有 subscription L1、traffic pack 查询能力，避免热路径过慢。
- 迁移旧 OpenAI Key 后，用户不能再固定到某个旧套餐 group；这是本次产品目标要求，但管理员后台仍应保留固定 group 能力用于特殊情况。

## 实施边界

本设计文档只定义方案，不执行代码修改、数据库迁移、容器重启或公网发布。

进入实现前需要写实施计划，按 TDD 做后端 resolver、迁移和前端移除下拉框，再在候选环境真实验证。
