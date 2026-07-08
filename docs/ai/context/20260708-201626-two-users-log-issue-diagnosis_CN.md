# xinlise 与 3876129758 日志问题排查

## 结论

- 两个用户当前 active Key 都能正常请求 `gpt-5.5`；刚做的真实请求均返回 200 并正确落库。
- 日志中的问题不是 Key 无效或订阅不可用，而是上游账号池与流式终止事件问题。
- 两个套餐池当前都只绑定同一个上游账号 `account_id=1/cliproxy-local-openai`，所以一旦该账号返回 502/503/429，就没有第二个账号可 failover，日志会出现 `no available accounts`。

## 3876129758@qq.com

### 当前状态

- 用户 `users.id=56`，状态 `active`。
- 当前未删除 active Key：
  - `api_keys.id=105/Codex`，脱敏 `sk-bba5b55...5164`。
  - `api_keys.id=106/Codex++`，脱敏 `sk-6abb393...0cbd`。
- 当前未删除 active 订阅：`user_subscriptions.id=90`，`group_id=4/codex-pool-49-usd`。
- 旧订阅 `user_subscriptions.id=76/group_id=2` 已 `expired + deleted_at=2026-07-08 16:58:50.722897+08`。

### 日志问题

- 今日 `ops_system_logs` 中该用户有 `111` 条 `openai_chat_completions.forward_failed`：
  - 时间范围：`2026-07-08 14:32:40+08` 到 `2026-07-08 17:21:38+08`。
  - 模型：`gpt-5.5`。
  - 错误：`stream usage incomplete: missing terminal event`。
  - 其中 `api_key_id=75` 有 `100` 条，`api_key_id=104` 有 `11` 条。
- 该用户还有一次 `api_key_id=106` 请求 `gpt-4o-mini`：
  - 时间：`2026-07-08 18:34:52+08`。
  - 上游 `account_id=1` 返回 `502` 后尝试 failover。
  - 因 `group_id=4` 只绑定一个上游账号，最终 `account_select_failed: no available accounts`。

### 扣费归属

- 旧订阅删除前，`api_key_id=75` 有 `105` 条请求扣到 `subscription_id=76/group_id=2`，总费用 `15.8351032500`，时间到 `2026-07-08 14:50:36+08`。
- 旧订阅删除后，后续请求均扣到当前订阅 `subscription_id=90/group_id=4`：
  - `api_key_id=75`：`25` 条，`0.5493140000`。
  - `api_key_id=104`：`6` 条，`0.1751795000`。
  - `api_key_id=105`：`1` 条，`0.0019050000`。
  - `api_key_id=106`：`1` 条，`0.0039960000`。
- 当前不是旧订阅仍被选中的问题。

### 判断

- 主要问题是 `/v1/chat/completions` 流式请求上游没有给出完整 usage 终止事件，Sub2API 因 `missing terminal event` 返回失败。
- 次要问题是单上游账号池没有可用 failover，导致一次 `gpt-4o-mini` 上游 502 直接变成 `no available accounts`。

## xinlise@gmail.com

### 当前状态

- 用户 `users.id=69`，状态 `active`。
- 当前 active Key：
  - `api_keys.id=99/codex`，脱敏 `sk-f1acac8...9374`。
  - `api_keys.id=102/佳一老师`，脱敏 `sk-7c22887...9178`，未测试。
- 当前 active 订阅：`user_subscriptions.id=88`，`group_id=8/codex-pool-89-usd`。

### 日志问题

- 今日 `usage_logs` 显示 `api_keys.id=99` 共 `499` 次成功请求，其中 `494` 次是流式，全部扣在 `subscription_id=88/group_id=8`，总费用 `61.8332090000`。
- 日志中有明显高并发/大请求现象：
  - 多个分钟内有 5 到 10 个流式请求。
  - 应用日志中可见请求体从约 `1.4MB` 到 `12MB` 不等。
  - `18:51` 一分钟内有 `6` 个流式请求，最长 `duration_ms=687237`、`first_token_ms=657452`。
- `ops_system_logs` 中该用户有：
  - `61` 次 `openai.upstream_failover_switching`，上游状态 `503`。
  - `61` 次 `openai.account_select_failed`，错误 `no available accounts`。
  - 时间范围集中在 `2026-07-08 16:50:22+08` 到 `16:56:40+08`。
- 另有 `30` 次 `openai.forward_failed`：
  - 时间范围：`2026-07-08 09:02:53+08` 到 `09:05:29+08`。
  - 错误：`upstream error: 400 ... encrypted content ... could not be decrypted or parsed`。
  - 这更像上游对加密上下文/请求内容解析失败，不是 Sub2API Key 或订阅失败。
- 还出现 `pool_mode_same_account_retry`：
  - `3` 次上游状态 `429`，时间在 `16:50` 到 `16:51`。
  - `1` 次上游状态 `401`，时间 `04:50:51+08`。

### 判断

- 当前 Key 与订阅本身正常，成功请求很多。
- 实际风险是用量和并发过高，叠加 `group_id=8` 只有一个上游账号，导致上游 503/429 时没有备用账号。
- `encrypted content could not be decrypted or parsed` 是独立上游错误，集中短时间出现，不能归因为额度或 Key 绑定。

## 上游账号池状态

- `group_id=4/codex-pool-49-usd` 只绑定：
  - `account_id=1/cliproxy-local-openai`，状态 `active`。
- `group_id=8/codex-pool-89-usd` 只绑定：
  - `account_id=1/cliproxy-local-openai`，状态 `active`。

## 建议

- 不要继续围绕这两个用户的 Key 或订阅做修复；当前真实请求已证明 Key 与订阅可用。
- 若要减少失败，应优先处理上游池容量/冗余：给 `group_id=4` 和 `group_id=8` 增加可用账号，或优化 CLIProxyAPI 上游账号健康/重试策略。
- 对 `stream usage incomplete: missing terminal event` 需要单独定位 chat/completions 流式终止事件处理，尤其是上游异常关闭时是否可以更温和地处理，而不是混同为订阅问题。
- 对 `xinlise@gmail.com` 的高并发大请求，建议另设每 Key 并发/请求体大小/分钟请求数观察或限制，避免单用户把唯一上游账号打到 503/429。

## 本轮影响

- 本轮只读排查日志和数据库。
- 未手工修改数据库业务数据。
- 未构建镜像、未替换容器、未重启服务、未修改 nginx/Redis。
