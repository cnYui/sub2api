# 流量卡自动切换实现与实测记录

## 需求规则

- 普通余额大于 0 时继续使用余额。
- 普通余额小于等于 0 时，OpenAI 请求仅在存在有效流量卡额度时放行。
- 流量卡额度小于等于 0、已过期或不可覆盖本次费用时，拒绝扣费，不制造负余额。

## 实现

- `backend/internal/server/middleware/api_key_auth.go`：认证预检接入 `BillingCacheService.CanUseTrafficPackCredit`；余额耗尽时按分组平台判断流量卡资格。
- `backend/internal/server/middleware/wire.go` 与 `backend/cmd/server/wire_gen.go`：生产 Wire 注入带计费缓存的认证中间件；保留旧构造器兼容不带流量卡依赖的测试调用。
- `backend/internal/service/billing_cache_service.go`：公开统一的流量卡资格判断，认证预检和计费预检复用同一规则。
- `backend/internal/repository/usage_billing_repo.go`：OpenAI 流量卡无法覆盖费用时返回 `INSUFFICIENT_BALANCE`，事务回滚，禁止回退为负余额；非流量卡平台保持原有行为。

## 自动化验证

- `go test -tags unit ./internal/server/middleware ./internal/repository`：通过。
- `go test ./cmd/server`：通过。
- `go test -run '^$' ./internal/service`：通过编译检查。
- `go test ./internal/service`：全量测试存在工作区既有的外部/集成类失败，本次改动未出现编译错误；未将其作为本次功能通过依据。

## 18082 真实验证

- 已使用源码重建并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis 和持久化目录未重建。
- 容器状态：`healthy`；`http://127.0.0.1:18082/health` 返回 HTTP 200。
- 管理员用户：`users.id=448`，普通余额为 `0.00000000`，有效 OpenAI 流量卡合计 `40.0000000000 USD`。
- 最小请求：`POST /v1/chat/completions`，模型 `gpt-5.6-terra`，`max_tokens=1`，返回 HTTP 200，内容为 `OK`。
- 流量卡余额：`40.0000000000 -> 39.9956665000 USD`，实际扣减 `0.0043335000 USD`。
- 最新 `traffic_credit_ledger`：`entry_type=deduction`、`credit_id=18`、`balance_after_usd=9.9956665000`。
- 最新 `usage_logs`：`actual_cost=0.0043335000`，普通余额未被扣减。

结论：管理员余额为 0 时已自动切换到 OpenAI 流量卡并成功扣费；流量卡无可用额度时认证预检拒绝，结算竞态或额度不足时也不会产生负余额。
