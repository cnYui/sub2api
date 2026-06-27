# Sub2API -> CLIProxyAPI 图生图公网调用调查

时间：2026-06-24 16:10 JST

## 结论

- 当前公网 OpenAI 兼容入口仍是 `https://api.aaccx.pw/v1`，备用同源入口是 `https://aaccx.pw/v1`。
- 图生图优先走 `POST /v1/images/edits`，也就是：
  - `https://api.aaccx.pw/v1/images/edits`
  - `https://aaccx.pw/v1/images/edits`
- 模型使用 `gpt-image-2`。不传 `model` 时，Sub2API 和 CLIProxyAPI 都会默认落到 `gpt-image-2`。
- 当前链路是 `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。
- 本机监听状态确认：
  - nginx 监听 `*:8080`
  - Sub2API 监听 `127.0.0.1:18080`
  - CLIProxyAPI 监听 `*:8317`
- `https://aaccx.pw/v1/images/edits` 未带 Key 返回 Sub2API 的 `API_KEY_REQUIRED`，说明公网路由已落到 Sub2API。
- `http://127.0.0.1:8317/v1/images/edits` 未带 Key 返回 CLIProxyAPI 的 `Missing API key`，说明 CLIProxyAPI 内部图片端点存在。
- `https://api.aaccx.pw/health` 返回 200；但本机 curl 未带 Key 请求 `https://api.aaccx.pw/v1/models` 被 Cloudflare 403 challenge 挡住。若客户端遇到同类 403，可优先用 `https://aaccx.pw/v1` 验证，或检查 Cloudflare/WAF 规则。

## 推荐调用方式

JSON 方式适合输入图已经是公网 URL 或 data URL：

```bash
curl -X POST 'https://api.aaccx.pw/v1/images/edits' \
  -H 'Authorization: Bearer sk-xxxx' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把图中的人物改成赛博朋克风格，保留姿势和构图",
    "images": [
      {
        "image_url": "https://example.com/input.png"
      }
    ],
    "size": "1024x1024",
    "response_format": "b64_json"
  }'
```

本地文件方式用 multipart，更适合用户从本地图片直接图生图：

```bash
curl -X POST 'https://api.aaccx.pw/v1/images/edits' \
  -H 'Authorization: Bearer sk-xxxx' \
  -F 'model=gpt-image-2' \
  -F 'prompt=把这张图改成水彩插画风格，保持主体一致' \
  -F 'image=@/absolute/path/input.png' \
  -F 'size=1024x1024' \
  -F 'response_format=b64_json'
```

带蒙版的编辑：

```bash
curl -X POST 'https://api.aaccx.pw/v1/images/edits' \
  -H 'Authorization: Bearer sk-xxxx' \
  -F 'model=gpt-image-2' \
  -F 'prompt=只替换蒙版区域为一束白色花，其他区域不变' \
  -F 'image=@/absolute/path/input.png' \
  -F 'mask=@/absolute/path/mask.png' \
  -F 'size=1024x1024' \
  -F 'response_format=b64_json'
```

可用参数按当前链路建议：

- `prompt`：必填。
- `model`：建议显式传 `gpt-image-2`。
- `images[].image_url`：JSON 图生图必填；`file_id` 当前不支持。
- `image` / `image[]`：multipart 图生图必填，可传多张。
- `mask`：可选，multipart 文件或 JSON 的 `mask.image_url`。
- `size`：建议用 `1024x1024`、`2048x2048` 或 4K 对应尺寸；Sub2API 会按 1K/2K/4K 档计费。
- `response_format`：建议 `b64_json`；`url` 在当前实现里可能变成 data URL。
- `stream` / `partial_images`：链路支持流式和部分图事件，但普通用户集成优先用非流式，客户端处理简单。
- `quality`、`background`、`output_format`、`output_compression`、`input_fidelity`、`moderation`：CLIProxyAPI 图生图分支会继续传给上游 `image_generation` 工具。

## 链路行为

Sub2API：

- 路由支持 `POST /v1/images/generations` 和 `POST /v1/images/edits`。
- 请求进入 `OpenAIGatewayHandler.Images` 后，会先验证用户 Key、分组、生图开关、内容审核、并发和余额资格。
- 分组必须允许 `allow_image_generation=true`。历史记录显示 2026-06-21 已对在售分组和本机分组开启。
- JSON 图生图只接受 `images[].image_url`；`images[].file_id` 和 `mask.file_id` 会被拒绝。
- APIKey 类型上游账号会原样转发到账号 `base_url + /v1/images/edits`；这正是 Sub2API -> CLIProxyAPI 链路应走的路径。
- OAuth 类型上游账号会把 Image API 转成 `/responses + image_generation`。当前实际链路不依赖这个分支。

CLIProxyAPI：

- 监听端口为 `8317`，`config.yaml` 未显式配置 `disable-image-generation`，因此按默认 `false`。
- 路由注册了 `/v1/images/generations` 和 `/v1/images/edits`。
- `gpt-image-2` 是内置图片模型；CLIProxyAPI 收到 `/v1/images/edits` 后，会构造 `model=gpt-5.4-mini` 的 Responses 请求，并挂载 `tools=[{"type":"image_generation","action":"edit","model":"gpt-image-2", ...}]`。
- multipart 文件会被转为 data URL 后放进 Responses 的 `input_image`。
- JSON 的 `images[].image_url` 会直接变成 Responses 的 `input_image.image_url`。

## 风险与注意

- 不要让用户直连 `127.0.0.1:8317`；公网唯一入口仍应是 Sub2API，避免绕过用户 Key、订阅和计费。
- 不要在文档、日志或提交中记录完整用户 API Key、CLIProxyAPI Key、OAuth token。
- `api.aaccx.pw` 本次本机无 Key curl 在 `/v1/models` 遇到 Cloudflare 403；生产客户端如遇同样问题，优先验证 `https://aaccx.pw/v1/images/edits`，再检查 Cloudflare 规则。
- Sub2API 的 OAuth 转换分支目前解析了 `input_fidelity`，但转换到 Responses 工具时未带该字段；实际 Sub2API->CLIProxyAPI APIKey 转发路径不受影响。

## 依据

- Sub2API 路由和权限：`backend/internal/handler/openai_images.go`
- Sub2API 图片请求解析和 APIKey 上游转发：`backend/internal/service/openai_images.go`
- CLIProxyAPI 图片路由：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/internal/api/server.go`
- CLIProxyAPI Image API 处理：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/sdk/api/handlers/openai/openai_images_handlers.go`
- CLIProxyAPI Codex 图片执行器：`/Users/wujianxiang/CodeSpace/CLIProxyAPI/internal/runtime/executor/codex_openai_images.go`
- OpenAI 官方图像指南：`https://developers.openai.com/api/docs/guides/image-generation`
