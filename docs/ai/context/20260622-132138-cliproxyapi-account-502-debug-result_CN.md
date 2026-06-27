# CLIProxyAPI 账号 502 排查结果

## 结论

`codex-xiaobianfuai@gmail.com-plus.json` 无法稳定反代的根因不是账号文件缺失，也不是 Sub2API 到 CLIProxyAPI 的网络不通，而是 CLIProxyAPI 运行时已经把 Codex 文本模型池判定为上游用量限制/冷却中。

截图里的额度卡片显示该账号仍有 5 小时/周额度剩余，但真实反代请求以 Codex 上游响应为准。运行日志显示，该账号在真实请求中被上游返回 `429 The usage limit has been reached`，随后 CLIProxyAPI 将对应模型标记为 quota 并暂停调度。

## 证据

- 当前运行链路仍是 `nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。
- CLIProxyAPI 进程正常监听 `127.0.0.1:8317`，Sub2API 正式容器正常监听 `127.0.0.1:18080`。
- CLIProxyAPI 账号池中存在 `auths/codex-xiaobianfuai@gmail.com-plus.json`，该文件 `disabled=false`，`type=codex`，并含 access/refresh token。
- CLIProxyAPI 启动日志显示该账号已注册到 Codex provider，模型包含 `gpt-5.4`、`gpt-5.4-mini`、`gpt-5.5`、`codex-auto-review`、`gpt-image-2` 等。
- 关键运行日志：
  - `2026-06-22 11:34:01`：`codex-xiaobianfuai@gmail.com-plus.json` 请求 `gpt-5.5`，上游返回 `429 The usage limit has been reached`，CLIProxyAPI 标记 `gpt-5.5` quota 并 suspend。
  - `2026-06-22 11:47:32`：同账号请求 `gpt-5.4`，上游返回同样的 `429 The usage limit has been reached`。
  - `2026-06-22 12:28:18`：同账号请求 `gpt-5.4-mini`，上游返回同样的 `429 The usage limit has been reached`。
- 本地直连 CLIProxyAPI 验证：
  - `GET /v1/models` 返回 200，说明模型列表和入站认证正常。
  - `POST /v1/responses` 使用 `gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`codex-auto-review` 均返回 `429 model_cooldown`。
  - 返回体为 `All credentials for model ... are cooling down via provider codex`，恢复时间约 `2026-06-25 07:51 JST`。
- Sub2API 容器日志显示，当 CLIProxyAPI 返回 503 时，Sub2API 因只有一个上游账号可选，重试/切换后记录 `no available accounts`，最终对客户端返回 502；当 CLIProxyAPI 返回 429 时，Sub2API 会在同账号内重试 3 次，最终可能返回 429。

## 为什么“有额度”仍会 502

截图额度面板和真实反代请求不是同一个判定层：

- 额度面板显示的是 CLIProxyAPI 管理侧读取到的账号额度摘要。
- 真正请求 `gpt-5.5/gpt-5.4/gpt-5.4-mini` 时，Codex 上游返回了 `The usage limit has been reached`。
- CLIProxyAPI 收到上游 429 后，会把对应账号/模型进入 cooldown。
- 当整个 Codex provider 对目标模型都处于 cooldown 时，CLIProxyAPI 不再向上游发真实请求，而是直接返回 `model_cooldown` 或 `503`。
- Sub2API 当前只有一个 OpenAI 聚合上游 `cliproxy-local-openai`，因此 CLIProxyAPI 不可用时，Sub2API 没有第二个上游可切，最终展示 502 或 429。

## 额外问题

CLIProxyAPI 当前以 TRAE 启动时，日志里反复出现：

`open /Users/wujianxiang/CodeSpace/CLIProxyAPI/auths/logs/request-body-*.tmp: operation not permitted`

以及：

`open logs/usage/usage-events-2026-06.jsonl: operation not permitted`

这些权限错误不是本次 502 的直接根因，但会影响请求错误日志和用量事件落盘，导致后续排查少细节。

## 后续建议

1. 不要只看额度卡片，先以 CLIProxyAPI `/v1/responses` 最小请求作为真实可用性判断。
2. 如果确认额度面板可信但上游仍返回 429，需要重新登录/重新授权该 Codex 账号，或等待 cooldown 到期后再验证。
3. 如果要立刻恢复服务，应补充至少一个真实可用的 Codex 账号到 CLIProxyAPI 池，或临时切换 Sub2API 上游到其它可用 provider。
4. 单纯“刷新额度”不会清除已经由真实上游 429 触发的运行时 cooldown。
5. 后续应修复 CLIProxyAPI 启动方式或目录权限，让 `auths/logs` 和 `logs/usage` 能正常写入。
