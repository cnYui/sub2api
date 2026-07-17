# 代码冗余重构：OpenAI HTTP failover 收敛结果

日期：2026-07-17

## 已完成

- 新增 OpenAI HTTP failover 状态推进器，统一处理同账号池模式重试、失败账号排除、切换次数、切换指标和 OAuth 账号 429 停止判断。
- Responses、Messages、Chat Completions、Images、Embeddings 五条 HTTP 链路切换到该推进器。
- 各协议仍由原 Handler 输出各自的错误格式：OpenAI 与 Anthropic 的终态响应保持原样。
- WebSocket failover 保持独立，未改变其连接关闭与重连逻辑。

## 验证

- `go test -p 1 -parallel 1 -count=1 ./internal/handler -run 'Test.*(OpenAI|Failover|Embedding|Images|Chat)'`
- `go test -p 1 -parallel 1 -count=1 ./internal/handler`
- `go test -tags=unit -p 1 -parallel 1 -count=1 ./internal/handler`
- `git diff --check`

## 边界

- 未修改数据库迁移和运行态环境。
- `backend/internal/repository/migrations_schema_integration_test.go` 是用户已有的未提交改动，本阶段未修改、未暂存、不会提交。
