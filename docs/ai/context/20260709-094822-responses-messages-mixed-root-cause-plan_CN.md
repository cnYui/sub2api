# `/v1/responses` 混入 `messages` 根因排查计划

## 背景

已确认 `/v1/responses` 使用 Responses 格式 `input` 可返回 200，而把 Chat Completions 格式 `messages` 发到 `/v1/responses` 会触发上游 `400 Unsupported parameter: messages`，Sub2API 当前包装成 502。

## 排查目标

- 找出当前为什么会出现 `messages -> /v1/responses` 的混用。
- 区分是客户端配置问题、Sub2API 网关转换问题、管理后台测试逻辑问题，还是上游账号策略问题。
- 给出最小修复方案；若需要改代码，后续按 TDD 增加失败测试后再实现。

## 排查步骤

1. 从 `ops_system_logs` 抽取最新混用请求的 `user_id`、`api_key_id`、`group_id`、`user_agent`、`client_ip`、`client_request_id`。
2. 检查混用请求是否来自本轮人工探测，还是来自真实客户端。
3. 回溯 Sub2API 代码中所有可能构造 `messages` 请求体并发送到 Responses 路径的位置。
4. 对比 `/v1/chat/completions` 成功路径与 `/v1/responses` 失败路径的转换逻辑。
5. 输出修复建议：配置修复、客户端修复、服务端防御性校验或兼容转换。
