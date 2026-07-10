# 961109198@qq.com Chat Completions 流式 502 排查结果

## 结论

`961109198@qq.com` 本次截图附近的 502 不是 API Key、订阅、额度、余额或源站整体不可用问题。根因是该用户的 `/v1/chat/completions` 流式请求经过唯一 OpenAI 上游 `account_id=1/cliproxy-local-openai` 后，上游流没有返回 Sub2API 计费所需的终止 usage 事件，Sub2API 按既有保护逻辑返回 502：

```text
stream usage incomplete: missing terminal event
```

客户端显示 `API failed after 3 retries`，与 nginx/app 日志中的三次重试完全对上。

## 用户与套餐状态

- 用户：`961109198@qq.com`
- `users.id=71`
- 当前 active Key：主要为 `api_keys.id=96/mac_hermes`，掩码 `sk-a642521...e010`
- 当前 active 订阅：`user_subscriptions.id=86`
- 分组：`group_id=8/codex-pool-89-usd`
- 订阅到期：`2026-08-06 20:50:54+08`
- 当日窗口：`2026-07-09 00:00:00+08`
- 当日日用量：`65.0942040000 / 89 USD`

以上状态正常，没有超额，也没有 Key 失效。

## 关键请求时间线

截图时间 `20:41` 与以下北京时间请求吻合。nginx 本机日志使用 `+0900`，所以对应 nginx access 时间为 `21:40:31/41/52 +0900`。

### 第一轮三次失败

- `2026-07-09 20:29:22+08`：同 Key `/v1/chat/completions` 成功，落库 `usage_logs.id=75724`，费用 `0.2702880000`
- `2026-07-09 20:29:28+08`：失败，`stream usage incomplete: missing terminal event`
- `2026-07-09 20:29:38+08`：失败，`stream usage incomplete: missing terminal event`
- `2026-07-09 20:29:50+08`：失败，`stream usage incomplete: missing terminal event`

### 截图附近三次失败

- `2026-07-09 20:40:26 -> 20:40:31+08`
  - request_id：`dea7effe-a019-48b0-80ce-b2d34ce1f83f`
  - client_request_id：`4ab8cda6-d326-4ab2-9269-c1826cecd639`
  - 结果：HTTP 502，`stream usage incomplete: missing terminal event`
- `2026-07-09 20:40:38 -> 20:40:41+08`
  - request_id：`89cc7259-b364-4740-9034-23c69a36fcf0`
  - client_request_id：`b4d776ca-9cc3-4285-9623-9ef65fc60cc8`
  - 结果：HTTP 502，`stream usage incomplete: missing terminal event`
- `2026-07-09 20:40:49 -> 20:40:52+08`
  - request_id：`885b37f9-6b68-47f9-9bf8-59823acbd195`
  - client_request_id：`c031c2ea-d36f-4c67-8679-76ed21f1d88f`
  - 结果：HTTP 502，`stream usage incomplete: missing terminal event`

三次请求共同特征：

- `user_id=71`
- `api_key_id=96`
- `group_id=8`
- `model=gpt-5.5`
- `stream=true`
- `endpoint=/v1/chat/completions`
- `protocol=openai_chat_completions`
- `body_bytes=961298`
- `client_ip=180.113.77.220`
- `User-Agent=OpenAI/Python 2.24.0`
- `account_id=1`

## 与 Cloudflare 502 的关系

截图错误体显示 Cloudflare `origin_bad_gateway`，但本次同窗口不是上午那类源站不可达：

- `2026-07-09 20:30-20:46+08` / `21:30-21:46+0900` nginx error.log 没有 upstream connect/refused/premature close 错误。
- 同窗口其它 `/v1/responses`、`/api/*` 请求大量 200。
- 应用容器在同窗口明确记录了这位用户三次 `/v1/chat/completions` 502，错误来自业务网关层。
- `ops_error_logs` 对该用户今天为 0 条，说明这类流式缺 usage 502 主要落在应用日志和 nginx access 中，而不是监控错误表。

因此这里不能按 Cloudflare Tunnel 源站挂了处理；Cloudflare 对外展示的 problem JSON 只是客户端看到的 502 包装，真正触发点是 Sub2API 对上游流式计费终止事件缺失的保护。

## 代码机制

相关代码：

- `backend/internal/service/openai_gateway_chat_completions.go`
  - 流式转发中，如果解析到 `[DONE]` 前没有拿到计费 terminal usage，返回 `stream usage incomplete: missing terminal event`
- `backend/internal/handler/openai_chat_completions.go`
  - handler 捕获 forward 错误，写 fallback 502，并记录 `openai_chat_completions.forward_failed`

这个保护是为了避免上游流没有完整 usage 时仍按成功响应返回，导致请求无法可靠计费。

## 本轮操作

- 只读排查。
- 未修改代码。
- 未修改数据库。
- 未重启容器。
- 未构建镜像。
- 未触发真实用户计费请求。

## 后续建议

短期可让用户重试；如果同一客户端持续出现，建议优先改用 `/v1/responses` + Responses 协议，或临时关闭该客户端的 stream。根治方向不是改 Key/套餐，而是增强 Chat Completions 流式兼容：对缺 terminal usage 的上游流进行更可控的错误暴露、降级策略或上游账号池冗余。
