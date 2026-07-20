# 错误契约统一结果

## 完成范围

- 在隔离分支 `codex/error-contract-unification` 实现 Sub2API 与 CLIProxyAPI 的上游失败分类契约，修复账号池全部冷却时被聚合为 HTTP 502 的问题。
- 新增公开规范 `docs/ERROR_CONTRACT.md`：稳定序号为 `S2A-xxxx`，英文程序代码使用大写下划线形式，公开提示固定为英文且不包含上游正文、账号信息、内部地址或实现错误。
- CLIProxyAPI 在全部候选账号冷却时返回 HTTP 429、`all_accounts_rate_limited`、`retry_after`、`Retry-After` 和 `X-CLIProxy-Error-Class`；无可用凭据返回 HTTP 503 和 `credentials_unavailable`。
- Sub2API 的 OpenAI、Anthropic、Gemini failover 端点将可信上游类别转换为公开事实。`429` 映射为 `S2A-5004 / UPSTREAM_RATE_LIMITED`，凭据不可用映射为 `503`，超时映射为 `504`，连接失败和无效响应映射为 `502`。
- OpenAI、Anthropic、通用 JSON 和 Gemini 错误外壳保持兼容，并补充错误 ID、英文代码、可重试标记、重试等待和 request ID。前端会归一化响应体和响应头，Toast 可复制错误 ID、英文代码和 request ID。
- 错误透传规则已收敛为分类和 `skip_monitoring` 标记。管理端不再展示、提交或依赖状态码覆写、上游正文透传和自定义公开消息；历史数据库列保留兼容，运行时不读取它们改写公开语义。

## 根因与约束

原链路只把多次 failover 后的聚合失败视为网关失败，丢失了账号池的全局冷却事实，导致真实限流被降级为 502。新链路只信任 CLIProxyAPI 白名单失败类别和 OpenAI 兼容字段，并优先保留 429 与 `Retry-After`。

`S2A-1001` 到 `S2A-9002` 是本项目的完整规范目录。当前代码已完整迁移模型 API 的上游失败路径；管理、认证、计费等普通 REST 接口仍大量使用旧 `response.*` 帮助函数，尚未逐端点映射到目录。后续迁移必须以端点域为单位，并兼顾既有前端文案和 API 兼容性，不能因为目录已发布而把所有接口误称为已统一。

## 验证

- `go test ./internal/domain/errorcontract ./internal/handler ./internal/service` 通过。
- 错误契约、OpenAI 503/429、Anthropic、Gemini、图片 SSE 回归、透传规则和前端归一化测试通过。
- `pnpm build` 通过，包含 TypeScript 检查。构建保留项目既有动态导入、包体大小和 Browserslist 警告。
- `pnpm test:run` 为 890/900 通过。10 个失败位于未改动的用量图片展示、模型/分组分布图和分页偏好模块；本次新增的 Toast、规则表单、客户端和错误解析测试均通过。
- CLIProxyAPI 的 `go test ./sdk/...` 与 `go build ./cmd/server` 通过。
- 两个仓库的 `git diff --check` 通过。

## 运行态边界

未修改数据库、Redis、容器、Nginx、Cloudflare Tunnel、公网流量或现有错误透传规则数据，未提交、未推送、未部署。
