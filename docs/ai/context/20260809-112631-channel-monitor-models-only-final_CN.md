# 渠道监控统一目录探测最终调整

## 需求口径

`/monitor` 的所有渠道只做低成本模型目录探测，不再发送 Chat Completions、Responses 或其它真实推理请求。现有监控统一按 30 分钟（1800 秒）运行。

## 实现

- checker 每个监控只发起一次带鉴权的 `GET /v1/models`，主模型和附加模型复用同一份目录响应。
- HTTP 非 2xx、网络错误或目录响应无法解析记为 `error`；HTTP 2xx 但目标模型缺失记为 `failed`；目标模型存在记为 `operational`。
- 不再发起监控用 HEAD 请求；历史 API 中的 `ping_latency_ms` 字段保留兼容，但新探测不会产生该请求或写入该延迟。
- `api_mode` 只允许 `models`，空值默认也归一为 `models`；旧的 `chat_completions` / `responses` 值由迁移统一转换并被数据库约束拒绝。
- 默认间隔、设置回退值和已有监控迁移值统一为 `1800` 秒。

## 验证

- `frontend`: `pnpm build` 通过。
- `backend`: 目录探测、模式校验、监控服务、管理端重复操作和仓储相关单元测试通过（需要 `-tags unit` 的测试已带标签执行）。
- 后端全量 `go test ./...` 仍受既有无关测试 `TestCheckBillingEligibility_SubscriptionMode_BypassesPlatformQuota` 的 nil 指针 panic 影响，未归因于本次监控改动。
