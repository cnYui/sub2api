# gpt-image-2 120 秒超时修改点判断

## 背景

用户追问：如果要把这次生图失败里约 120 秒的超时变长，应该改哪里。

前置排查已确认：`441565547@qq.com` 的三次 `gpt-image-2` 生图失败不是提示词过长，请求体约 3.3KB；失败集中在约 125 秒后返回 `502/context canceled`。

## 判断

这次最像调用方 OpenAI SDK 客户端或外部 provider 的请求超时，而不是 Sub2API / CLIProxyAPI 的图片生成总超时。

依据：

- 失败日志里调用方是 `AsyncOpenAI/Python 2.38.0`，模块名为 `ylcraft.openai_sdk_image`。
- 请求头没有 `x-stainless-timeout`，但 OpenAI Python SDK 自身仍可能通过 `AsyncOpenAI(timeout=...)`、`client.with_options(timeout=...)` 或请求级 `timeout=...` 控制等待时间。
- CLIProxyAPI 的 `gpt-image-2` 生图路径 `/Users/wujianxiang/CodeSpace/CLIProxyAPI/internal/runtime/executor/codex_openai_images.go` 调用 `helps.NewProxyAwareHTTPClient(..., 0)`；`0` 在 `/Users/wujianxiang/CodeSpace/CLIProxyAPI/internal/runtime/executor/helps/proxy_helpers.go` 中表示不设置 HTTP client 总超时。
- Sub2API 当前相关配置 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT=900` 是图片流“数据间隔”超时，不是这次非流式请求约 120 秒的总等待超时。
- Sub2API 的 `GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT=0` 当前默认禁用本地 OpenAI/Codex 响应头超时，也不是 120 秒来源。

## 推荐修改位置

优先在调用方 `ylcraft.openai_sdk_image` 或对应 provider `aaccx-gpt-image-2` 的 OpenAI SDK 客户端初始化处修改：

```python
from openai import AsyncOpenAI

client = AsyncOpenAI(
    base_url="https://api.aaccx.pw/v1",
    api_key=api_key,
    timeout=300.0,
)
```

如果客户端是全局复用，也可以在单次生图调用前使用：

```python
client = client.with_options(timeout=300.0)
response = await client.images.generate(...)
```

建议先调到 `300` 秒；如果产品形态允许，长期更稳的是将生图改成异步任务/轮询，避免 HTTP 长连接等待跨越客户端、nginx、Cloudflare、上游多个超时边界。

## 不建议优先修改的项

- 不优先改 `GATEWAY_IMAGE_STREAM_DATA_INTERVAL_TIMEOUT`：它当前 900 秒，且语义是图片流数据间隔。
- 不优先改 CLIProxyAPI `codex_openai_images.go` 的 `NewProxyAwareHTTPClient(..., 0)`：这里没有 120 秒总超时，贸然加固定值反而可能引入新的截断点。
- 不优先改 `GATEWAY_OPENAI_RESPONSE_HEADER_TIMEOUT`：当前为 0，不是约 120 秒取消来源。
