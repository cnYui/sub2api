# DeepSeek 公网真实调用核验

## 结论

截至 2026-08-20 15:12（Asia/Tokyo），`https://api.aaccx.pw/v1` 的 DeepSeek 模型可正常使用，不存在整体不可用的问题。

## 核验范围

- 使用管理员提供的 API Key 调用 `GET /v1/models`，认证成功，仅返回两个 DeepSeek 模型：`deepseek-v4-flash`、`deepseek-v4-pro`。
- 对两个模型分别调用 `POST /v1/chat/completions`，请求内容为最小文本，最大输出限制为 8 tokens。
- 凭证仅在当前进程内使用，未写入仓库、命令输出或本记录。

## 结果

| 模型 | HTTP 状态 | 返回模型 | 返回内容 | 结束原因 | 用量 |
| --- | --- | --- | --- | --- | --- |
| `deepseek-v4-flash` | 200 | `deepseek-v4-flash` | `DS_OK` | `stop` | 10 输入 + 3 输出，13 tokens |
| `deepseek-v4-pro` | 200 | `deepseek-v4-pro` | `DS_OK` | `stop` | 10 输入 + 11 输出，21 tokens |

两次请求均完成路由、上游响应和网关返回链路验证；本次产生的实际用量合计为 34 tokens。
