# /responses 与 /chat/completions 最终验证记录

## 验证命令结果

- `POST /responses`：HTTP 400，符合裸模型 API 禁用策略。
- `POST /chat/completions`：HTTP 400，符合裸模型 API 禁用策略。
- `POST /v1/responses` 无鉴权：HTTP 401，说明正式路由进入鉴权。
- `POST /v1/chat/completions` 无鉴权：HTTP 401，说明正式路由进入鉴权。
- `POST /v1/responses` 带 active Key，Responses 格式 `input`：HTTP 200，新增 `usage_logs.id=71744`，`total_cost=0.0040410000`。
- `POST /v1/responses` 带 active Key，但错误使用 Chat Completions 格式 `messages`：HTTP 502；对应 `ops_system_logs.id=476614`，真实错误为 `upstream error: 400 message=Unsupported parameter: messages`。

## 结论确认

当前问题不是 `/v1/responses` 端点整体不可用，也不是 `/v1/chat/completions` 独有可用；根因是路径和请求体协议要匹配：Responses 端点用 `input`，Chat Completions 端点用 `messages`。
