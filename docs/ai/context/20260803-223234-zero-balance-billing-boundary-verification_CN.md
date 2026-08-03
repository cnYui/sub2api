# 零余额 API Key 与请求计费边界验证

## 验证范围

- 本地服务：`http://127.0.0.1:18082`
- 验证对象：用户提供的管理员 API Key（文档不保存密钥）
- 数据库中的用户余额：`0.00000000`
- API Key 状态：`active`
- API Key quota：`0`
- API Key 关联用户角色：`admin`

## 真实请求结果

使用该 Key 对本地服务发起真实请求：

- `GET /v1/models`：HTTP `403`，错误码 `INSUFFICIENT_BALANCE`
- `POST /v1/chat/completions`：HTTP `403`，错误码 `INSUFFICIENT_BALANCE`

请求未转发到上游；数据库最近 10 分钟没有新增 `usage_logs`。因此管理员角色和 API Key 的 active 状态不会绕过余额检查，余额为零时当前实现会在转发前阻止调用。

## 代码结论

### 请求准入

1. API Key 鉴权层对非订阅余额模式使用 `balance <= 0` 作为硬门槛，见 `backend/internal/server/middleware/api_key_auth.go:394-399`；命中后返回 `403 INSUFFICIENT_BALANCE`，见同文件 `:261-265`。
2. 网关在获取并发槽位后还会再次执行 `CheckBillingEligibility`，例如 `backend/internal/handler/gateway_handler_responses.go:143-152`。该检查通过缓存/数据库确认余额超过资格阈值；当前配置的 `minimum_balance_reserve` 只影响该计费预检，不会改变鉴权层的 `<= 0` 语义。

### 普通余额请求的结算

普通 token、聊天、Embedding、标准图片等请求不是预冻结预计费用，而是上游返回或流式转发结束后，根据实际 usage 计算 `ActualCost`，再调用统一计费仓储。`ActualCost` 会进入 `UsageBillingCommand.BalanceCost`，见 `backend/internal/service/gateway_usage_billing.go:268-287`。

余额扣减的 SQL 是两阶段：

1. 第一条 `UPDATE` 带 `balance >= amount`，余额足够时正常扣减，见 `backend/internal/repository/usage_billing_repo.go:245-270`。
2. 第一条因余额不足返回无行后，第二条只校验用户存在，不再校验余额，仍执行 `balance = balance - amount`，见同文件 `:275-304`。

因此单次请求如果在准入时余额仍为正，但实际 `ActualCost` 大于剩余余额，结算会允许余额变成负数，并将 `BalanceOverdrafted` 标记为 `true`，见同文件 `:181-188`。这不是“扣到某个 token 实时停止”，而是请求完成后一次性结算。

结算完成后，如果新余额低于资格阈值，系统会使余额缓存失效，见 `backend/internal/service/gateway_usage_billing.go:386-402`；下一次请求重新读取余额并被拦截。也就是说，普通余额模式的实际停止边界是“当前请求结束后结算，下一请求准入时阻止”，不是本次请求中途硬停。

### 流式与 WebSocket

- 普通 HTTP 流式请求在流结束后按已解析到的最终 usage 记录并扣费。
- OpenAI WebSocket 多轮会话按每个 turn 的结果异步调用 `RecordUsage`，见 `backend/internal/handler/openai_gateway_handler.go:2180-2200`；因此每个 turn 结束后结算，但不会按 token 实时中断当前 turn。

### 有预冻结的例外

批量图片任务使用独立的余额 hold：提交前按预计费用 `ReserveBatchImageBalance`，完成后 `CaptureBatchImageBalance`，失败则 `ReleaseBatchImageBalance`，见 `backend/internal/service/batch_image_billing_hold.go:58-110`。该路径会在执行前要求余额足够，不能用普通请求的“结算后允许透支”结论替代。

订阅模式也不扣用户余额，而是检查订阅的日/周/月用量并在请求结算后递增订阅 usage；API Key quota、账号 quota、RPM 等是独立边界。

## 结论

- 当前管理员 API Key 余额为 `0` 时，真实请求会被阻止，不会到达上游，也不会产生 usage log。
- 普通余额请求没有预冻结预计费用；通过正余额准入后，会先完成请求，再按真实 usage 一次性扣费。
- 普通余额扣费允许单次扣成负数；负余额在本次请求结束后生效，后续请求在准入阶段被拒绝。
- 只有批量图片 hold 等专用流程采用预冻结；不能据此推断所有模型请求都预冻结。
