# 441565547@qq.com gpt-image-2 生图失败排查结果

## 背景

- 用户：`441565547@qq.com`，公网 18084 候选库中 `users.id=37`。
- 使用 Key：`api_keys.id=44`，名称 `本地29usd`，`group_id=3`。
- 请求端点：`POST /v1/images/generations`。
- 请求模型：Sub2API 与 CLIProxyAPI 均识别为 `gpt-image-2`；外部日志里 `model=` 为空不是 Sub2API 实际转发模型为空。
- 排查只读日志与数据库，未重启容器、未修改运行态 DB、未触碰 Key 明文。

## 关键证据

1. 请求体不大，不像提示词过长：
   - Sub2API 日志 `body_bytes=3320`。
   - CLIProxyAPI 错误日志 `Content-Length=3367`。
   - 本地按用户给出的提示词估算：提示词约 `1181` 字符、`3283` UTF-8 字节，JSON 请求体约 `3374` 字节。
   - 当前 Sub2API 公网请求体上限为 256MB，这个请求远低于限制。

2. 三次失败均发生在固定时长附近：
   - `2026-07-03 21:26:36 +0800` 开始，`21:28:41 +0800` 结束，`125012ms`，HTTP 502。
   - `2026-07-03 21:28:41 +0800` 开始，`21:30:46 +0800` 结束，`125010ms`，HTTP 502。
   - `2026-07-03 21:30:48 +0800` 开始，`21:32:53 +0800` 结束，`125006ms`，HTTP 502。

3. Sub2API 的失败点在转发到 CLIProxyAPI 之后：
   - 错误为 `upstream request failed: Post "http://host.docker.internal:8317/v1/images/generations": context canceled`。
   - content moderation 明确放行：`allowed=true`，并且没有提示词长度或内容安全拒绝。

4. CLIProxyAPI 对应三条错误日志均为同一类错误：
   - `error-v1-images-generations-2026-07-03T222841-203ca759.log`
   - `error-v1-images-generations-2026-07-03T223046-2314ee55.log`
   - `error-v1-images-generations-2026-07-03T223253-cb6bc5e4.log`
   - 返回体均为 `{"error":{"message":"context canceled","type":"server_error","code":"internal_server_error"}}`。
   - 请求头没有 `x-stainless-timeout` 等显式超时头，`User-Agent` 为 `AsyncOpenAI/Python 2.38.0`。

5. 该用户和链路不是完全不能生图：
   - `usage_logs` 中该用户有成功的 `gpt-image-2` 生图记录，例如 `2026-07-03 00:03:09 +0800` 与 `2026-07-03 23:20:26 +0800`，均为 `image_count=1`。

## 结论

- 本次失败不是提示词太长导致的直接拒绝。
- 根因更符合“下游客户端或中间链路在约 125 秒处取消请求，导致 Sub2API 取消对 CLIProxyAPI 的上游请求，CLIProxyAPI 也记录 `context canceled`”。
- 该提示词属于复杂多姿态角色设定板，可能让上游生图耗时超过 2 分钟；一旦超过调用方或链路的约 125 秒等待上限，就会被取消。

## 后续建议

- 用户侧临时建议：缩短或拆分提示词，把“多姿态设定板”拆成 2 到 3 次生成，或减少姿态数量。
- 平台侧若要降低误失败：排查调用方 `ylcraft.openai_sdk_image` 或外部 provider 的 HTTP timeout 是否为 120 秒级；必要时将生图 timeout 调到 180 到 300 秒，或改成异步任务/轮询。
- 不建议把这个问题按“提示词过长”处理；更应按“生图请求超过客户端等待上限”处理。
