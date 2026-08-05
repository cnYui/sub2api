# Claude 模型访问核验

## 核验时间

2026-08-05 10:04（Asia/Tokyo）

## 核验对象

- API 地址：`https://api.aaccx.pw/v1`
- API Key：用户提供的 `sk-LOCAL-...`，完整密钥不写入文档

## 实测结果

1. 使用该 Key 请求 `GET /v1/models` 成功，返回 20 个模型。
2. 返回模型全部为 GPT、Codex 或图像模型，没有任何包含 `claude` 的模型。
3. 使用 OpenAI 兼容接口请求 `POST /v1/chat/completions`，模型为 `claude-opus-4-5-20251101`，消息为最小测试文本，服务返回 HTTP 404：

```json
{
  "error": {
    "message": "Model \"claude-opus-4-5-20251101\" is not supported by any configured account in this group",
    "type": "model_not_found"
  }
}
```

## 结论

该 API Key 当前所属分组没有可路由的 Claude 账号，因此不能通过该地址请求 Claude 模型。价格文件或代码中存在 Claude 模型定义不代表该 Key 已配置 Claude 上游账号；需要将 Key 加入包含 Claude 账号的分组，或改用对应分组的 Key 后重新验证。

## 安全提示

该密钥已在聊天中明文提供，建议核验后立即撤销并重新生成。
