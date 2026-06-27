# 公网图生图链路实测结果

时间：2026-06-24 16:17 JST

## 结论

- 已使用本机管理员自用 API Key 通过公网入口 `https://aaccx.pw/v1/images/edits` 成功完成一次真实图生图。
- 返回 HTTP `200`，响应头包含：
  - `x-client-request-id: b05e21a8-4131-4409-9e46-3954dea454cb`
  - `x-request-id: 5ecb0951-017a-48e6-9925-4bdc9c2b4b58`
  - `cf-ray: a109f1d7091c3bb3-NRT`
- 响应体包含有效 `data[0].b64_json`，解码后得到有效 PNG：
  - 文件：`/tmp/sub2api-image-edit-public.png`
  - 类型：`PNG image data`
  - 尺寸：`1254 x 1254`
- 视觉检查确认图片不是空白图，也不是错误占位图，输出内容与“黑白极简海报风格”的编辑提示一致。

## 请求摘要

- 入口：`POST https://aaccx.pw/v1/images/edits`
- 鉴权：管理员本机自用 API Key（不记录明文）
- 输入图：`frontend/public/logo.png`
- 表单参数：
  - `model=gpt-image-2`
  - `prompt=请把这张图改成黑白极简海报风格，保留主体轮廓，输出一张清晰图片`
  - `image=@frontend/public/logo.png`
  - `size=1024x1024`
  - `response_format=b64_json`

## 返回摘要

- `created`: `1782285380`
- `usage.input_tokens`: `1097`
  - `image_tokens`: `992`
  - `text_tokens`: `105`
- `usage.output_tokens`: `2058`
- `usage.total_tokens`: `3155`
- `revised_prompt` 明确把原图编辑为黑白、高对比、主体居中、线条更利落的极简海报风格。

## 观察

- 这次优先走 `aaccx.pw` 成功，未触发前面无 Key 探测时在 `api.aaccx.pw` 上遇到的 Cloudflare challenge。
- 请求执行阶段明显比文本接口慢，符合真实生图路径特征。
- curl 在本次请求里表现出“长时间无头部输出，结束时一次性返回”的现象，但最终文件与响应体均完整，说明公网链路本身可用。

## 产物

- 原始响应头：`/tmp/sub2api-image-edit-public.headers`
- 原始响应体：`/tmp/sub2api-image-edit-public.json`
- 解码图片：`/tmp/sub2api-image-edit-public.png`
