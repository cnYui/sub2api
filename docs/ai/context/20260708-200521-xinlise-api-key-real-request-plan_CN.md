# xinlise@gmail.com API Key 真实模型请求测试计划

## 背景

- 用户要求使用 `xinlise@gmail.com` 的 API Key 真实测试是否能发起模型请求。
- 当前公网候选链路为 `sub2api-candidate`，本次测试只做真实请求和只读核对，不改库、不重启、不构建、不替换容器。
- 完整 API Key 不写入文档、回复或日志摘要。

## 已确认状态

- 用户：`xinlise@gmail.com`，`users.id=69`，状态 `active`。
- active Key：
  - `api_keys.id=99`，名称 `codex`，脱敏 `sk-f1acac8...9374`，最近使用时间 `2026-07-08 19:04:20+08`。
  - `api_keys.id=102`，名称 `佳一老师`，脱敏 `sk-7c22887...9178`，尚无最近使用时间。
- active 订阅：`user_subscriptions.id=88`，`group_id=8/codex-pool-89-usd`，有效期到 `2026-08-07 03:57:35+08`。
- 分组日限额：`89 USD`；订阅今日用量约 `61.459137 USD`。
- OpenAI 流量卡可用余额：`0 USD`。

## 测试方案

1. 优先使用最近使用过的 active Key `api_keys.id=99/codex`，避免两把 Key 都测试造成额外扣费。
2. 请求公网正式入口 `https://api.aaccx.pw/v1/responses`。
3. 模型使用当前主链路常用的 `gpt-5.5`，输入使用最小 ping 文本。
4. 记录 HTTP 状态码、响应是否成功、usage log 是否新增、扣费是否归属到 `subscription_id=88`。

## 风险与预期

- 真实请求会产生极小订阅用量。
- 按当前用量和限额判断，请求应走订阅扣费，不应触发流量卡。
- 若上游偶发失败，只记录失败原因，不做修复或重试风暴。
