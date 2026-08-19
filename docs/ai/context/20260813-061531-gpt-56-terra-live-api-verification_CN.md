# GPT-5.6 Terra 公网真实请求验证

## 目标

按用户授权验证公网 OpenAI 兼容接口的真实模型转发链路。

## 执行

- 使用用户在对话中提供的 API Key 访问 `https://api.aaccx.pw/v1`。
- `GET /v1/models` 成功返回可用模型列表，确认认证与公网接口可用。
- 对 `POST /v1/chat/completions` 发起非流式最小请求，模型为 `gpt-5.6-terra`，输入要求精确返回 `OK`，最大输出为 5 token。

## 结果

- HTTP 请求成功。
- 返回内容：`OK`。
- `finish_reason`：`stop`。
- 用量：输入 `4686` token、输出 `5` token、合计 `4691` token。

## 安全边界

- API Key 仅在执行进程内使用，未写入仓库、环境文件、日志或本文档。
