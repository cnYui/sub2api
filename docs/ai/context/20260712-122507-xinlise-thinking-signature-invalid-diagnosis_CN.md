# xinlise 连接失败与 thinking signature 错误只读诊断

时间：2026-07-12 12:25 JST

## 结论

- `xinlise@gmail.com` 当前账号、API Key、订阅、额度、分组绑定和上游服务均正常，不是 Key 失效、套餐撤销、日额度耗尽或并发槽卡死。
- 用户今天真正的失败集中在一条旧的 `gpt-5.5` 会话：请求携带的加密 reasoning 内容无法被当前轮询到的上游 OAuth 账号验证，上游返回 `400 thinking_signature_invalid`，Sub2API 对客户端表现为 HTTP 502。
- 该问题只影响特定会话上下文。相同 Key 在失败窗口内仍有其他 `gpt-5.5`、`gpt-5.6-sol` 请求成功，失败结束后两把 Key 也持续成功，因此不能描述为账号整体无法连接。
- 立即恢复方式是新建 Codex task / conversation，不继续复用报错会话的历史上下文。重新生成 Key、调整套餐或清额度不能解决这类错误。

## 当前用户状态

- 用户：`users.id=69`，状态 `active`，未删除。
- API Key：
  - `api_keys.id=99/codex`，active，自动分组，最近使用 `2026-07-12 11:22:05+08`。
  - `api_keys.id=102/佳一老师`，active，自动分组，最近使用 `2026-07-12 11:17:43+08`。
- 当前订阅：`user_subscriptions.id=98/group_id=12/codex-pool-179-usd`，active，未删除，到期 `2026-08-09 16:43:25+08`。
- `2026-07-12 12:25 JST` 复核快照的今日订阅用量：`59.06009355 / 179 USD`，未达到日限额。
- `group_id=12` 已绑定 `account_id=1/cliproxy-local-openai`；账号 active、schedulable，并发为 100。
- Redis 当前 `concurrency:api_key:99/102` 与等待队列均为 0，没有残留并发槽。
- Sub2API 本地和公网 `/health` 均为 200；CLIProxyAPI 在 8317 监听 TLS，带鉴权前的 `/v1/models` 可正常返回 401，说明服务可达。

## 今日成功记录

- 截至 `2026-07-12 12:25 JST` 复核时，用户今天已有 485 条成功 `usage_logs`，其中 484 条为流式请求，总费用 `59.060094 USD`。
- 成功时间范围：`2026-07-12 00:01:37+08` 到 `11:25:06+08`。
- 最近请求主要来自两台 Codex Desktop 客户端：
  - `api_key_id=99`，主要请求 `gpt-5.5` 和 `gpt-5.6-sol`。
  - `api_key_id=102`，主要请求 `gpt-5.5`。
- 本次错误结束后：
  - `api_key_id=99` 又成功 297 次，首条为 `09:03:14+08`，最后到 `11:13:21+08`。
  - `api_key_id=102` 成功 64 次，最后到 `11:18:23+08`。

## 失败记录

- 时间：`2026-07-12 09:01:22+08` 到 `09:03:02+08`，即日本时间 `10:01:22` 到 `10:03:02`。
- 数量：连续 30 次。
- 请求：`api_key_id=99`、`model=gpt-5.5`、`POST /v1/responses`、流式。
- 上游返回：HTTP 400，错误码 `thinking_signature_invalid`。
- 错误正文：`The encrypted content ... could not be verified. Reason: Encrypted content could not be decrypted or parsed.`
- Sub2API 日志事件：`openai.forward_failed`，对客户端返回 HTTP 502。
- 同一客户端今天同时有 313 次 `/v1/responses` 200、227 次 `/v1/models` 200；另一台客户端有 63 次 `/v1/responses` 200。
- `ops_error_logs` 对该用户为 0，因为运行态 `OPS_ENABLED=false`；本次证据来自应用容器 stdout、CLIProxyAPI 日志和 `usage_logs`。

## 根因链路

1. 报错会话把历史 reasoning item 的 `encrypted_content` 再次发送给上游。
2. CLIProxyAPI 使用 `round-robin` 在约 30 个 OAuth 凭据之间轮询；日志显示这些失败请求实际落到多个不同凭据，但都返回相同的签名验证错误，排除单个凭据损坏。
3. 加密 reasoning 内容与生成它的上游账号或会话状态绑定；轮询到其他账号后无法解密，因此该旧会话持续失败。
4. CLIProxyAPI 当前 `/Users/wujianxiang/CodeSpace/CLIProxyAPI/internal/runtime/executor/codex_executor.go` 的错误分类只识别正文包含 `invalid signature in thinking block` 或 `invalid_encrypted_content`，没有直接识别上游 `error.code=thinking_signature_invalid`，也没有匹配当前 `could not be decrypted or parsed` 文案。
5. 因未分类成功，已有的 invalid signature 清理逻辑没有触发，请求直接以 400 结束；Sub2API 再包装为 502。

## 历史对照

- 2026-07-08，该用户同一把 `api_key_id=99` 也曾出现 30 次相同的 encrypted content 解析失败，时间为 `09:02:53+08` 到 `09:05:29+08`。
- 2026-07-10，新购 199 元套餐后的 503 是 `group_id=12` 未绑定上游账号导致，后续已完成绑定并真实请求 200；当前不是该问题复发。
- 2026-07-10 的退款失败、旧订阅撤销状态和误操作恢复也均已处理；当前唯一未删除 active 套餐是 `subscription_id=98`。

## 建议

- 用户侧：关闭报错 task，新建 task 后继续使用；不要把旧 task 的完整历史重新注入新 task。
- 管理侧：无需改 Key、订阅、额度、Redis 或账号绑定。
- 代码层长期修复应在 CLIProxyAPI 单独实施并按 TDD 验证：错误分类增加 `upstreamCode == "thinking_signature_invalid"` 和当前错误文案识别，触发清理无效 reasoning replay / encrypted item 后最多重试一次，避免无限重试。

## 本轮影响

- 只读查询 `sub2api-candidate-postgres`、Redis、Sub2API/CLIProxyAPI/nginx 日志与健康状态。
- 未发起真实模型扣费请求。
- 未修改用户、Key、订阅、订单、额度、DB、Redis、nginx、容器或 CLIProxyAPI 配置。
- 未修改业务代码、未重启、未部署。
