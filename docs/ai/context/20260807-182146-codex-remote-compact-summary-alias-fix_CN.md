# Codex remote compact v2 `compaction_summary` 兼容问题

## 现象

用户 `3867878292@qq.com`（生产用户 ID `599`）在长上下文使用 Codex 时收到：

```text
Error running remote compact task: Fatal error: remote compaction v2 expected exactly one compaction output item, got 0 from 2 output items
```

## 根因

该用户的 `codex` API Key 为 ID `244`，绑定 `group_id=10`，请求使用 `gpt-5.5`，由上游账号 ID `1129` 承接。生产复现请求携带 `x-codex-beta-features: remote_compaction_v2` 和 `input.type=compaction_trigger`，上游返回：

- `response.output_item.done` 共 2 个 item；
- 第一个是普通 `message`；
- 第二个是 `type=compaction_summary`；
- `response.completed.response.object=response.compaction`。

Codex remote compact v2 客户端只把 `type=compaction` 识别为压缩结果，因此把上游的 `compaction_summary` 视为普通/未知 item，最终认为压缩 item 数量为 0。问题不是用户余额、API Key 状态或模型扣费失败。

## 实现

- 在 `backend/internal/service/openai_gateway_response_handling.go` 增加 compact 输出 item 类型归一化：
  - `compaction_summary` 写回客户端时转换为 `compaction`；
  - `encrypted_content`、`summary`、`opaque` 等其它字段保持不变；
  - 覆盖 `response.output_item.added`、`response.output_item.done`、终态 `response.output`。
- 原生 SSE、SSE 转 JSON、unary compact 转 SSE、path-based JSON 写回共用同一归一化逻辑。
- 新增/调整回归测试覆盖 API Key remote compact v2 和已有 compact bridge 路径。

## 验证

通过：

```text
go test ./internal/service -run 'Compact|Compaction|OpenAIGatewayServiceForward.*RemoteCompact' -count=1
go test ./internal/handler -run 'Compact|Compaction|OpenAI' -count=1
git diff --check
```

`go test ./internal/service -count=1` 仍有与本次改动无关的既有环境/外部依赖失败：内容审核缓存刷新时序、真实 OpenAI 凭证缺失的 count-tokens、Redis/scheduler 可用性相关用例；compact 相关用例未失败。

## 部署状态

本次代码修复已写入工作区，但尚未重建或替换生产 `sub2api-official-18082` 容器。生产要生效还需要按既有发布流程构建应用镜像并替换应用容器，PostgreSQL、Redis 和数据卷无需重建。
