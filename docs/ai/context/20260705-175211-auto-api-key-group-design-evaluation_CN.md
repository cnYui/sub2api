# API Key 自动分组与流量包用户订阅页边界评估

## 背景

用户反馈 `1930863755@qq.com` 购买了 GPT 流量包，但不显示在管理员 subscriptions 页面；同时希望新用户创建 API Key 时无需选择分组，让同一个 API Key 可以随着套餐升级、退款、重购自动按当前权益使用和扣费。

## 事实核对

- 当前公网入口仍以 18084 为准：nginx 将 `api.aaccx.pw` 和 `aaccx.pw` 的 Sub2API API 路由反代到 `127.0.0.1:18084`。
- `1930863755@qq.com` 在 18084：
  - `users.id=62`
  - 用户状态 active，未软删除。
  - OpenAI 流量包可用余额为 `5.0000000000` USD。
  - `user_subscriptions` 无记录。
  - `api_keys` 无记录。
- 因此该用户不显示在管理员 subscriptions 页面是当前模型下的预期：该页面只列 `user_subscriptions`，不是流量包资产页面。

## 对 subscriptions 页边界的判断

用户的想法是合理的：管理员 subscriptions 页面应只显示购买或分配套餐的用户，不应混入单独购买流量包的用户。

原因：

- 套餐订阅和 GPT 流量包是两类权益事实源。
- 套餐有 group、有效期、日/周/月限额和用量窗口。
- 流量包是用户级 OpenAI 平台余额，走 `user_traffic_credits` 和 `traffic_credit_ledger`。
- 混在 subscriptions 表格中会让管理员误以为流量包也是一个套餐订阅，后续退款、续期、撤销和限额操作都会变得含糊。

更合适的管理视图是新增或增强“流量包 / Traffic Credits”入口，支持按邮箱查看用户的流量包余额、流水、过期时间和相关订单；不要把它伪装成 subscription。

## 对“创建 API Key 不选择分组”的判断

方向是对的，但不能只删前端下拉框。当前 group 同时承担：

- 平台识别：OpenAI、Anthropic、Gemini、Antigravity。
- 上游账号池选择：网关依赖 group 选择可用账号。
- 价格和倍率：部分模型价格会按 group 或 channel 解析。
- 限额和 RPM：订阅限额、group RPM、用户专属覆盖都依赖 group。
- 可见模型和特殊路由：OpenAI Messages Dispatch、图片价格、fallback 等也可能依赖 group。

所以推荐把产品语义设计为“自动 API Key”，而不是“无分组 API Key”：

- API Key 创建页默认不让普通用户选择分组，创建 `group_id=NULL` 的自动 Key。
- 请求到达网关后，根据请求平台和用户当前权益解析一个临时有效分组 `effective_group`。
- 解析出的 `effective_group` 只用于本次请求的账号选择、计费、倍率、限流和日志记录。
- 用户升级、退款、重购套餐后，同一个自动 Key 下次请求自动解析到新的当前权益，不需要重建 Key。

## 推荐的运行时解析规则

OpenAI 请求优先级建议：

1. 如果用户有 active OpenAI subscription，选择当前有效订阅对应的套餐 group。
2. 如果用户没有 active OpenAI subscription，但有可用 OpenAI/GPT 流量包，选择一个受控的 OpenAI 流量包入口 group。
3. 如果两者都没有，再按现有余额/标准分组能力处理；当前售卖链路已弱化余额充值，后续可按实际需要保留或关闭。
4. 如果用户有多个 active OpenAI subscriptions，必须定义确定性规则，例如选择日限额最高、最新购买，或显式 active primary subscription。不能随机选择。

这能解决两个问题：

- 仅购买流量包的新用户可以创建并使用 API Key。
- 套餐升级或退款重购后，用户原有自动 Key 不需要重新创建。

## 不推荐的方案

- 不建议把流量包写成虚假的 `user_subscriptions`。这会污染订阅事实源，使套餐页、管理员订阅页、过期提醒和限额逻辑混乱。
- 不建议只在前端隐藏分组选择。后端现在仍依赖 group 路由和计费，只隐藏 UI 会导致创建出来的 Key 在网关不可用或行为不确定。
- 不建议在购买新套餐时批量把用户所有旧 Key 改绑到新套餐 group。这样会破坏用户可能有意保留的固定分组 Key，也无法自然处理多套餐并存。

## 需要进一步确认

最大设计问题是既有 API Key 如何处理：

- 方案 A：只让新建 Key 默认为自动 Key；旧 Key 保持原 group 行为。
- 方案 B：上线后将普通用户现有 OpenAI 套餐 Key 迁移为自动 Key；管理员或高级入口保留“固定分组 Key”。
- 方案 C：保留旧 Key，但在用户购买/退款/升级后提示“一键转换为自动 Key”。

推荐从 A 开始，随后给管理员提供单用户转换能力；是否全量迁移旧 Key 需要谨慎。
